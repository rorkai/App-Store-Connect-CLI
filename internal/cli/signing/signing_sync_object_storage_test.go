package signing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing/objectstoretest"
)

const signingObjectTestBucket = "team-certs"

func newSigningObjectTestServer(t *testing.T) *objectstoretest.Server {
	t.Helper()
	fake := objectstoretest.New(t, signingObjectTestBucket)
	fake.UseAsAWSEnvironment(t)
	return fake
}

func signingObjectStorageArgs(fake *objectstoretest.Server) []string {
	return []string{
		"--storage", signingSyncStorageObject,
		"--object-bucket", fake.Bucket,
		"--object-prefix", "asc/",
		"--object-region", "us-east-1",
		"--object-endpoint", fake.HTTP.URL,
	}
}

func publishSigningObjectSeed(t *testing.T, fake *objectstoretest.Server, seed *signingpkg.GitStore) {
	t.Helper()
	backend, err := signingpkg.NewObjectStorageStore(context.Background(), signingpkg.ObjectStorageOptions{
		Bucket: fake.Bucket, Prefix: "asc", Region: "us-east-1", Endpoint: fake.HTTP.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Publish(context.Background(), seed); err != nil {
		t.Fatal(err)
	}
}

func TestSigningSyncObjectStorageFlagsArePresent(t *testing.T) {
	d, _, _ := newSigningTestDeps()
	for _, command := range []*ffcli.Command{syncPushCommand(d), syncPullCommand(d), syncRotatePasswordCommand(d)} {
		for _, name := range signingSyncObjectFlagNames {
			found := command.FlagSet.Lookup(name)
			if found == nil || strings.TrimSpace(found.Usage) == "" {
				t.Fatalf("%s: expected a documented --%s flag", command.Name, name)
			}
		}
		if !strings.Contains(command.FlagSet.Lookup("storage").Usage, signingSyncStorageObject) {
			t.Fatalf("%s: --storage usage does not name object storage", command.Name)
		}
		for _, secret := range []string{"object-access-key", "object-secret-key", "object-session-token"} {
			if command.FlagSet.Lookup(secret) != nil {
				t.Fatalf("%s: credentials must not be accepted as flags (--%s)", command.Name, secret)
			}
		}
	}
}

func TestSigningSyncObjectStorageRejectsMixedBackendFlagsBeforeSideEffects(t *testing.T) {
	object := []string{"--storage", signingSyncStorageObject, "--object-bucket", signingObjectTestBucket}
	tests := []struct {
		name       string
		newCommand func(*shared.Deps) *ffcli.Command
		args       []string
		want       string
	}{
		{"pull requires bucket", syncPullCommand, []string{"--storage", signingSyncStorageObject}, "--object-bucket is required with --storage object"},
		{"pull rejects repo", syncPullCommand, append(append([]string{}, object...), "--repo", "git@github.com:team/certs.git"), "--repo is only supported with --storage git, not --storage object"},
		{"pull rejects branch", syncPullCommand, append(append([]string{}, object...), "--branch", "release"), "--branch is only supported with --storage git, not --storage object"},
		{"pull rejects shared prefix", syncPullCommand, append(append([]string{}, object...), "--prefix", "asc"), "--prefix is not supported with --storage object; use --object-prefix"},
		{"pull rejects shared region", syncPullCommand, append(append([]string{}, object...), "--region", "us-east-1"), "--region is not supported with --storage object; use --object-region"},
		{"pull rejects gitlab flags", syncPullCommand, append(append([]string{}, object...), "--gitlab-project", "42"), "--gitlab-project requires --storage gitlab-secure-files"},
		{"git rejects bucket", syncPullCommand, []string{"--repo", "git@github.com:team/certs.git", "--object-bucket", signingObjectTestBucket}, "--object-bucket requires --storage object"},
		{"aws rejects object region", syncPullCommand, []string{"--storage", signingSyncStorageAWS, "--prefix", "asc", "--region", "us-east-1", "--object-region", "us-east-1"}, "--object-region requires --storage object"},
		{"gitlab rejects object prefix", syncPullCommand, []string{"--storage", signingSyncStorageGitLab, "--gitlab-project", "42", "--prefix", "asc", "--gitlab-token-file", "token", "--object-prefix", "asc"}, "--object-prefix requires --storage object"},
		{"invalid bucket", syncPullCommand, []string{"--storage", signingSyncStorageObject, "--object-bucket", "Team_Certs"}, "object storage bucket must be 3-63 lowercase letters, digits, dots, or hyphens, starting and ending with a letter or digit"},
		{"http endpoint", syncPullCommand, append(append([]string{}, object...), "--object-endpoint", "http://127.0.0.1:9000"), "object storage endpoint must be an HTTPS origin such as https://s3.example.com"},
		{"traversal prefix", syncPullCommand, append(append([]string{}, object...), "--object-prefix", "../escape"), `object storage prefix component ".." must start with a letter or digit and use only letters, digits, dot, underscore, or hyphen`},
		{"push rejects repo", syncPushCommand, append([]string{"--bundle-id", "com.example.app", "--profile-type", "IOS_APP_STORE", "--repo", "git@github.com:team/certs.git"}, object...), "--repo is only supported with --storage git, not --storage object"},
		{"push requires bucket", syncPushCommand, []string{"--bundle-id", "com.example.app", "--profile-type", "IOS_APP_STORE", "--storage", signingSyncStorageObject}, "--object-bucket is required with --storage object"},
		{"rotate requires bucket", syncRotatePasswordCommand, []string{"--storage", signingSyncStorageObject, "--password-file", "current", "--new-password-file", "next", "--confirm"}, "--object-bucket is required with --storage object"},
		{"rotate rejects repo", syncRotatePasswordCommand, append(append([]string{}, object...), "--repo", "git@github.com:team/certs.git", "--password-file", "current", "--new-password-file", "next", "--confirm"), "--repo is only supported with --storage git, not --storage object"},
		{"rotate git rejects bucket", syncRotatePasswordCommand, []string{"--repo", "git@github.com:team/certs.git", "--object-bucket", signingObjectTestBucket, "--password-file", "current", "--new-password-file", "next", "--confirm"}, "--object-bucket requires --storage object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(signingSyncPasswordEnvVar, "repository-password")
			clientCalls := 0
			clientFactory := func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("client must not be created")
			}
			stdout, stderr, err := runSigningSyncCommandForStorage(t, tt.newCommand, clientFactory, tt.args)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want a usage error (exit 2)", err)
			}
			if clientCalls != 0 || stdout != "" {
				t.Fatalf("side effects before validation: client calls=%d stdout=%q", clientCalls, stdout)
			}
			if strings.Contains(stderr, "Fetching") || strings.Contains(stderr, "Cloning") {
				t.Fatalf("stderr shows storage side effects: %q", stderr)
			}
		})
	}
}

func TestSigningSyncObjectTransportMirrorsGitLayoutAndDecryptsIdentically(t *testing.T) {
	fake := newSigningObjectTestServer(t)
	selection := signingSyncStorageSelection{
		kind:   signingSyncStorageObject,
		object: signingpkg.ObjectStorageOptions{Bucket: fake.Bucket, Prefix: "asc/", Region: "us-east-1", Endpoint: fake.HTTP.URL},
	}
	transport, err := selection.transport(context.Background(), testSigningDeps())
	if err != nil {
		t.Fatalf("transport() error: %v", err)
	}
	if transport.Kind() != signingSyncStorageObject || transport.Locator() != "s3://team-certs/asc" {
		t.Fatalf("transport kind=%q locator=%q", transport.Kind(), transport.Locator())
	}

	const password = "repository-password"
	plaintexts := map[string][]byte{
		"certs/distribution/serial.cer":                     []byte("certificate-plaintext"),
		"profiles/appstore/com.example.app.mobileprovision": []byte("profile-plaintext"),
	}
	gitTree := newSigningSyncStore(signingSyncGitTransport{stderr: io.Discard}, t.TempDir(), "", "")
	for relPath, plaintext := range plaintexts {
		if err := gitTree.WriteEncryptedFile(relPath, plaintext, password); err != nil {
			t.Fatal(err)
		}
	}
	if err := transport.Fetch(context.Background(), gitTree, true); err != nil {
		t.Fatalf("Fetch() of an empty prefix error: %v", err)
	}
	if err := transport.Publish(context.Background(), gitTree, "unused"); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	pulled := newSigningSyncStore(transport, t.TempDir(), "", "")
	if err := transport.Fetch(context.Background(), pulled, false); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	for relPath, plaintext := range plaintexts {
		gitBytes, err := os.ReadFile(filepath.Join(gitTree.LocalDir, filepath.FromSlash(relPath)+".enc"))
		if err != nil {
			t.Fatal(err)
		}
		objectBytes, ok := fake.Object("asc/" + relPath + ".enc")
		if !ok || !bytes.Equal(objectBytes, gitBytes) {
			t.Fatalf("object for %s is not byte-identical to the git tree ciphertext", relPath)
		}
		fromGit, err := gitTree.ReadEncryptedFile(relPath, password)
		if err != nil {
			t.Fatal(err)
		}
		fromObject, err := pulled.ReadEncryptedFile(relPath, password)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(fromGit, fromObject) || !bytes.Equal(fromObject, plaintext) {
			t.Fatalf("decrypted %s differs between git and object storage", relPath)
		}
	}
}

func TestSigningSyncPullFromObjectStorageWritesDecryptedFiles(t *testing.T) {
	fake := newSigningObjectTestServer(t)
	const password = "repository-password"
	seed := &signingpkg.GitStore{LocalDir: t.TempDir()}
	certificate := []byte("certificate-plaintext")
	if err := seed.WriteEncryptedFile("certs/distribution/serial.cer", certificate, password); err != nil {
		t.Fatal(err)
	}
	publishSigningObjectSeed(t, fake, seed)
	t.Setenv(signingSyncPasswordEnvVar, password)

	outputDir := filepath.Join(t.TempDir(), "signing")
	args := append(signingObjectStorageArgs(fake), "--output-dir", outputDir, "--output", "json")
	stdout, stderr, err := runSigningSyncCommandForStorage(t, syncPullCommand, nil, args)
	if err != nil {
		t.Fatalf("pull error = %v\nstderr=%s", err, stderr)
	}
	if !strings.Contains(stderr, "Fetching encrypted signing artifacts from object storage") {
		t.Fatalf("stderr = %q, want object storage progress", stderr)
	}
	var result SyncResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if result.Operation != "pull" || result.RepoURL != "s3://team-certs/asc" || len(result.Files) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(stdout, `"storage":{"kind":"object","location":"s3://team-certs/asc"}`) {
		t.Fatalf("stdout = %s, want the object storage receipt", stdout)
	}
	written, err := os.ReadFile(filepath.Join(outputDir, "certs", "distribution", "serial.cer"))
	if err != nil || !bytes.Equal(written, certificate) {
		t.Fatalf("decrypted output = %q, %v", written, err)
	}
	if mutations := fake.Mutations(); len(mutations) != 1 {
		t.Fatalf("pull wrote to the bucket: %+v", mutations)
	}
}

func TestSigningSyncObjectTransportReportsConcurrentPushAsOperationalError(t *testing.T) {
	fake := newSigningObjectTestServer(t)
	selection := signingSyncStorageSelection{
		kind:   signingSyncStorageObject,
		object: signingpkg.ObjectStorageOptions{Bucket: fake.Bucket, Prefix: "asc", Region: "us-east-1", Endpoint: fake.HTTP.URL},
	}
	transport, err := selection.transport(context.Background(), testSigningDeps())
	if err != nil {
		t.Fatal(err)
	}
	store := newSigningSyncStore(transport, t.TempDir(), "", "")
	if err := transport.Fetch(context.Background(), store, true); err != nil {
		t.Fatal(err)
	}
	fake.Put("asc/certs/distribution/serial.cer.enc", []byte("pushed by another runner"))
	if err := store.WriteEncryptedFile("certs/distribution/serial.cer", []byte("local"), "repository-password"); err != nil {
		t.Fatal(err)
	}

	err = transport.Publish(context.Background(), store, "unused")
	if !errors.Is(err, signingpkg.ErrObjectStorageConflict) || errors.Is(err, flag.ErrHelp) {
		t.Fatalf("Publish() error = %v, want an operational (exit 1) conflict", err)
	}
	if !strings.Contains(err.Error(), "pull the current artifacts and retry") {
		t.Fatalf("error = %q, want a clear retry instruction", err)
	}
	if stored, _ := fake.Object("asc/certs/distribution/serial.cer.enc"); string(stored) != "pushed by another runner" {
		t.Fatal("concurrent push was overwritten")
	}
}

// seedSigningObjectRotationStore publishes a complete, valid signing store
// encrypted with oldPassword to the fake bucket.
func seedSigningObjectRotationStore(t *testing.T, fake *objectstoretest.Server, oldPassword string) (string, string) {
	t.Helper()
	seed := &signingpkg.GitStore{LocalDir: t.TempDir()}
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 92)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	artifacts, err := prepareSigningIdentityArtifacts(identity, oldPassword, "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	profilePath, profileContent := bindTestSigningIdentityArtifacts(t, artifacts, certificate, key, "com.example.app", "IOS_APP_ADHOC", "profile-object-rotation")
	certificatePath := filepath.ToSlash(filepath.Join("certs", "distribution", "certificate.cer"))
	if err := seed.WriteEncryptedFile(certificatePath, certificate.Raw, oldPassword); err != nil {
		t.Fatal(err)
	}
	if err := seed.WriteEncryptedFile(profilePath, profileContent, oldPassword); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(seed, artifacts, oldPassword); err != nil {
		t.Fatal(err)
	}
	publishSigningObjectSeed(t, fake, seed)
	return certificatePath, artifacts.IdentityPath
}

func signingObjectRotationArgs(t *testing.T, fake *objectstoretest.Server, oldPassword, newPassword string) []string {
	t.Helper()
	currentPasswordFile := filepath.Join(t.TempDir(), "current")
	newPasswordFile := filepath.Join(t.TempDir(), "next")
	writePrivateTestFile(t, currentPasswordFile, []byte(oldPassword+"\n"))
	writePrivateTestFile(t, newPasswordFile, []byte(newPassword+"\n"))
	return append(
		signingObjectStorageArgs(fake),
		"--password-file", currentPasswordFile,
		"--new-password-file", newPasswordFile,
		"--confirm",
		"--output", "json",
	)
}

func TestSigningSyncRotatePasswordReencryptsObjectStorage(t *testing.T) {
	const oldPassword = "old-repository-password"
	const newPassword = "new-repository-password"
	fake := newSigningObjectTestServer(t)
	certificatePath, identityPath := seedSigningObjectRotationStore(t, fake, oldPassword)
	liveKeys := fake.Keys()

	stdout, stderr, err := runSigningSyncCommandForStorage(t, syncRotatePasswordCommand, nil, signingObjectRotationArgs(t, fake, oldPassword, newPassword))
	if err != nil {
		t.Fatalf("rotate-password error = %v\nstderr=%s", err, stderr)
	}
	var result SyncResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if result.Operation != "rotate-password" || result.RepoURL != "s3://team-certs/asc" || !result.IdentityPresent || len(result.Files) != len(liveKeys) {
		t.Fatalf("result = %#v", result)
	}
	if result.Storage == nil || *result.Storage != (asc.SigningSyncStorage{Kind: "object", Location: "s3://team-certs/asc"}) {
		t.Fatalf("rotation storage = %#v", result.Storage)
	}
	if keys := fake.Keys(); strings.Join(keys, ",") != strings.Join(liveKeys, ",") {
		t.Fatalf("keys after rotation = %v, want only %v", keys, liveKeys)
	}

	verify := &signingpkg.GitStore{LocalDir: t.TempDir()}
	backend, err := signingpkg.NewObjectStorageStore(context.Background(), signingpkg.ObjectStorageOptions{
		Bucket: fake.Bucket, Prefix: "asc", Region: "us-east-1", Endpoint: fake.HTTP.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Fetch(context.Background(), verify); err != nil {
		t.Fatal(err)
	}
	encryptedFiles, err := verify.ListEncryptedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepareDecryptedSigningFiles(testSigningDeps(), verify, encryptedFiles, newPassword, t.TempDir()); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
	if _, err := verify.ReadEncryptedFile(certificatePath, oldPassword); err == nil {
		t.Fatal("old password still decrypts a rotated artifact")
	}
	found := false
	for _, file := range encryptedFiles {
		found = found || file == identityPath
	}
	if !found {
		t.Fatalf("identity %s missing after rotation: %v", identityPath, encryptedFiles)
	}
}

func TestSigningSyncRotatePasswordObjectStorageConflictExitsOperationally(t *testing.T) {
	const oldPassword = "old-repository-password"
	const newPassword = "new-repository-password"
	fake := newSigningObjectTestServer(t)
	certificatePath, _ := seedSigningObjectRotationStore(t, fake, oldPassword)
	liveKeys := fake.Keys()
	before := make(map[string][]byte, len(liveKeys))
	for _, key := range liveKeys {
		before[key], _ = fake.Object(key)
	}
	concurrentKey := "asc/" + certificatePath + ".enc"
	var staged atomic.Int32
	fake.AfterRequest = func(method, key string) {
		if method == http.MethodPut && strings.Contains(key, ".asc-rotation-") && staged.Add(1) == int32(len(liveKeys)) {
			fake.Put(concurrentKey, []byte("concurrent push"))
		}
	}

	_, stderr, err := runSigningSyncCommandForStorage(t, syncRotatePasswordCommand, nil, signingObjectRotationArgs(t, fake, oldPassword, newPassword))
	if err == nil || errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want an operational (exit 1) failure", err)
	}
	if !errors.Is(err, signingpkg.ErrObjectStorageConflict) || !strings.Contains(err.Error(), "no live artifact was changed") {
		t.Fatalf("error = %q, want a conflict that leaves live artifacts unchanged", err)
	}
	if strings.Contains(stderr, "Done") {
		t.Fatalf("stderr reports success: %q", stderr)
	}
	for _, key := range fake.Keys() {
		stored, _ := fake.Object(key)
		if key == concurrentKey {
			if string(stored) != "concurrent push" {
				t.Fatal("concurrent push was overwritten")
			}
			continue
		}
		if !bytes.Equal(stored, before[key]) {
			t.Fatalf("%s changed although the rotation aborted", key)
		}
	}
}

func TestSigningSyncReceiptPinsStorageForEveryBackend(t *testing.T) {
	fake := newSigningObjectTestServer(t)
	gitLab, err := signingpkg.NewGitLabSecureFilesStore(signingpkg.GitLabSecureFilesOptions{
		ProjectID: "42",
		Prefix:    "asc-signing",
		Token:     "glpat-token-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	awsTransport, err := signingSyncStorageSelection{kind: signingSyncStorageAWS, prefix: "asc-signing", region: "us-east-1"}.transport(context.Background(), testSigningDeps())
	if err != nil {
		t.Fatal(err)
	}
	objectTransport, err := signingSyncStorageSelection{
		kind:   signingSyncStorageObject,
		object: signingpkg.ObjectStorageOptions{Bucket: fake.Bucket, Prefix: "asc/", Endpoint: fake.HTTP.URL, Region: "us-east-1"},
	}.transport(context.Background(), testSigningDeps())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		transport signingSyncTransport
		want      string
	}{
		{
			name:      "git",
			transport: signingSyncGitTransport{repoURL: "https://token:secret@example.com/team/certs.git", branch: "release", stderr: io.Discard},
			want:      `{"operation":"pull","repoUrl":"https://%5BREDACTED%5D@example.com/team/certs.git","storage":{"kind":"git","location":"https://%5BREDACTED%5D@example.com/team/certs.git","branch":"release"},"bundleId":"","profileType":"","files":[],"identityPresent":false}`,
		},
		{
			name:      "gitlab-secure-files",
			transport: signingSyncRemoteTransport{kind: signingSyncStorageGitLab, label: "GitLab Secure Files", backend: gitLab, stderr: io.Discard},
			want:      `{"operation":"pull","repoUrl":"gitlab-secure-files://gitlab.com/projects/42/asc-signing","storage":{"kind":"gitlab-secure-files","location":"gitlab-secure-files://gitlab.com/projects/42/asc-signing"},"bundleId":"","profileType":"","files":[],"identityPresent":false}`,
		},
		{
			name:      "aws-secrets-manager",
			transport: awsTransport,
			want:      `{"operation":"pull","repoUrl":"aws-secrets-manager://us-east-1/asc-signing","storage":{"kind":"aws-secrets-manager","location":"aws-secrets-manager://us-east-1/asc-signing"},"bundleId":"","profileType":"","files":[],"identityPresent":false}`,
		},
		{
			name:      "object",
			transport: objectTransport,
			want:      `{"operation":"pull","repoUrl":"s3://team-certs/asc","storage":{"kind":"object","location":"s3://team-certs/asc"},"bundleId":"","profileType":"","files":[],"identityPresent":false}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(&SyncResult{
				Operation: "pull",
				RepoURL:   tt.transport.Locator(),
				Storage:   tt.transport.Storage(),
				Files:     []string{},
			})
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.want {
				t.Fatalf("receipt = %s\nwant      %s", data, tt.want)
			}
		})
	}
}

func TestSigningSyncBatchPartialResultDescribesDefaultGitStorage(t *testing.T) {
	result := signingSyncBatchPartialResult(signingSyncBatchOptions{
		RepoURL: "https://token:secret@example.com/team/certs.git",
		Branch:  "main",
	}, []string{"com.example.app"}, nil, nil)
	want := asc.SigningSyncStorage{Kind: "git", Location: "https://%5BREDACTED%5D@example.com/team/certs.git", Branch: "main"}
	if result.Storage == nil || *result.Storage != want || result.RepoURL != want.Location {
		t.Fatalf("partial result repoUrl=%q storage=%#v, want %#v", result.RepoURL, result.Storage, want)
	}
}
