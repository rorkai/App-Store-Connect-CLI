package signing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

func TestSigningSyncCommandLongHelpUsesOutputDirExample(t *testing.T) {
	cmd := SigningSyncCommand()
	if !strings.Contains(cmd.LongHelp, "--output-dir ./signing") {
		t.Fatalf("expected long help to document --output-dir, got %q", cmd.LongHelp)
	}
	if strings.Contains(cmd.LongHelp, "--output ./signing") {
		t.Fatalf("expected long help to avoid --output path example, got %q", cmd.LongHelp)
	}
}

func TestSigningSyncPushHelpPairsDeviceWithCreateMissing(t *testing.T) {
	deviceFlag := syncPushCommand().FlagSet.Lookup("device")
	if deviceFlag == nil {
		t.Fatal("expected --device flag")
	}
	if !strings.Contains(deviceFlag.Usage, "--create-missing") {
		t.Fatalf("--device usage = %q, want it to name --create-missing", deviceFlag.Usage)
	}
	if strings.Contains(deviceFlag.Usage, "deprecated") || strings.Contains(deviceFlag.Usage, "5.0.0") {
		t.Fatalf("--device usage = %q, must not describe a finished transition", deviceFlag.Usage)
	}
}

func TestSigningSyncPushHelpDocumentsTargetsFile(t *testing.T) {
	bundleFlag := syncPushCommand().FlagSet.Lookup("bundle-id")
	if bundleFlag == nil {
		t.Fatal("expected --bundle-id flag")
	}
	if !strings.Contains(bundleFlag.Usage, "required unless --targets-file") {
		t.Fatalf("--bundle-id usage = %q, want the alternative selector contract", bundleFlag.Usage)
	}

	flag := syncPushCommand().FlagSet.Lookup("targets-file")
	if flag == nil {
		t.Fatal("expected --targets-file flag")
	}
	if !strings.Contains(flag.Usage, "1-32") || !strings.Contains(flag.Usage, "--bundle-id") || !strings.Contains(flag.Usage, "root-relative") {
		t.Fatalf("--targets-file usage = %q, want root boundary, bounds, and mutual exclusion", flag.Usage)
	}
}

func TestSigningSyncPreparesRepositoryOnceInAssetOrder(t *testing.T) {
	tests := []struct {
		name       string
		hasProfile bool
		wantEvents []string
	}{
		{
			name:       "existing profile",
			hasProfile: true,
			wantEvents: []string{
				"GET /v1/bundleIds/bundle-main/profiles",
				"GET /v1/profiles/profile-main/certificates",
				"clone repository",
			},
		},
		{
			name: "missing profile",
			wantEvents: []string{
				"GET /v1/bundleIds/bundle-main/profiles",
				"GET /v1/certificates",
				"clone repository",
				"POST /v1/profiles",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := []string{}
			client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
				events = append(events, req.Method+" "+req.URL.Path)
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
					if tt.hasProfile {
						return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"profile-main","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}]}`)
					}
					return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/profiles/profile-main/certificates":
					return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION"}}]}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
					return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
					return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"profiles","id":"profile-created","attributes":{"profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}}`)
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
				}
			})

			cloneCount := 0
			prepareRepository := onceAfterSuccess(func() error {
				cloneCount++
				events = append(events, "clone repository")
				return nil
			})
			_, _, _, err := resolveSigningAssets(
				context.Background(),
				client,
				signingAssetsOptions{
					BundleIDResourceID: "bundle-main",
					BundleIdentifier:   "com.example.signing.profile",
					ProfileType:        "IOS_APP_STORE",
					CreateMissing:      !tt.hasProfile,
					BeforeCreate: func(profileCreatePlan) error {
						return prepareRepository()
					},
				},
			)
			if err != nil {
				t.Fatalf("resolveSigningAssets() error: %v", err)
			}
			if err := prepareRepository(); err != nil {
				t.Fatalf("prepareRepository() error: %v", err)
			}
			if cloneCount != 1 {
				t.Fatalf("repository clone count = %d, want 1", cloneCount)
			}
			if strings.Join(events, ",") != strings.Join(tt.wantEvents, ",") {
				t.Fatalf("unexpected operation order: got %v, want %v", events, tt.wantEvents)
			}
		})
	}
}

func TestSigningSyncIdentityConflictFailsBeforeProfileCreatePOST(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 19)
	active := true
	certificateJSON := fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":%t,"expirationDate":%q,"certificateContent":%q}}]}`,
		active, time.Now().Add(time.Hour).Format(time.RFC3339), base64.StdEncoding.EncodeToString(certificate.Raw))
	postCalled := false
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, certificateJSON)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			postCalled = true
			return signingFetchJSONResponse(http.StatusCreated, `{}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	identity := &signingIdentity{PrivateKey: key}
	_, _, _, err := resolveSigningAssets(context.Background(), client, signingAssetsOptions{
		BundleIDResourceID: "bundle-main",
		BundleIdentifier:   "com.example.app",
		ProfileType:        "IOS_APP_ADHOC",
		CreateMissing:      true,
		CertificateFilter:  identityCertificateFilter(identity),
		BeforeCreate: func(plan profileCreatePlan) error {
			if err := preflightIdentityForProfileCreate(identity, plan, "repository-password", time.Now()); err != nil {
				return err
			}
			artifacts, err := prepareSigningIdentityArtifacts(identity, "repository-password", "com.example.app", "IOS_APP_ADHOC")
			if err != nil {
				return err
			}
			path := filepath.Join(store.LocalDir, artifacts.IdentityPath+".enc")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte("conflicting-artifact"), 0o600); err != nil {
				return err
			}
			return preflightSigningIdentityArtifacts(store, artifacts, "repository-password")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "before creating profile") {
		t.Fatalf("resolveSigningAssets() error = %v", err)
	}
	if postCalled {
		t.Fatal("profile POST occurred after local identity conflict")
	}
}

func TestSigningSyncLegacyDestinationConflictFailsBeforeProfileCreatePOST(t *testing.T) {
	active := true
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-1","attributes":{"serialNumber":"SERIAL","certificateType":"IOS_DISTRIBUTION","activated":%t,"expirationDate":%q}}]}`,
				active, time.Now().Add(time.Hour).Format(time.RFC3339)))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			t.Fatal("profile POST occurred after local signing destination conflict")
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
		return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
	})
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	outside := t.TempDir()
	_, _, _, err := resolveSigningAssets(context.Background(), client, signingAssetsOptions{
		BundleIDResourceID: "bundle-main", BundleIdentifier: "com.example.app", ProfileType: "IOS_APP_ADHOC", CreateMissing: true,
		BeforeCreate: func(plan profileCreatePlan) error {
			conflict := filepath.Join(store.LocalDir, "profiles", "adhoc", safeFileName(plan.ProfileName, "profile")+".mobileprovision.enc")
			if err := os.MkdirAll(filepath.Dir(conflict), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(filepath.Join(outside, "profile.enc"), conflict); err != nil {
				return err
			}
			return preflightSigningAssetDestinations(store, plan, "IOS_APP_ADHOC")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "preflight profile destination") {
		t.Fatalf("legacy destination preflight error = %v", err)
	}
}

func TestSigningSyncCaseCollisionFailsBeforeProfileCreatePOST(t *testing.T) {
	active := true
	postCalls := 0
	client := newSigningFetchTestClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-1","attributes":{"serialNumber":"SERIAL","certificateType":"IOS_DISTRIBUTION","activated":%t,"expirationDate":%q}}]}`,
				active, time.Now().Add(time.Hour).Format(time.RFC3339)))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			postCalls++
			return signingFetchJSONResponse(http.StatusCreated, `{}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	_, _, _, err := resolveSigningAssets(context.Background(), client, signingAssetsOptions{
		BundleIDResourceID: "bundle-main", BundleIdentifier: "com.example.app", ProfileType: "IOS_APP_ADHOC", CreateMissing: true,
		BeforeCreate: func(plan profileCreatePlan) error {
			existing := filepath.Join("profiles", "adhoc", strings.ToLower(safeFileName(plan.ProfileName, "profile"))+".mobileprovision")
			path := filepath.Join(store.LocalDir, existing+".enc")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
				return err
			}
			profilePath := filepath.Join("profiles", profileDirectoryName("IOS_APP_ADHOC"), safeFileName(plan.ProfileName, "profile")+shared.ProvisioningProfileExtension("", "IOS_APP_ADHOC"))
			planned := signingAssetRepositoryPathsForProfile(plan.Certificates, "IOS_APP_ADHOC", profilePath, nil)
			return store.CheckEncryptedRepositoryPaths(planned)
		},
	})
	if err == nil || err.Error() != "preflight before creating profile: encrypted repository paths collide under Windows Unicode case folding" {
		t.Fatalf("case collision error = %v", err)
	}
	if postCalls != 0 {
		t.Fatalf("profile POST calls = %d, want 0", postCalls)
	}
	entries, readErr := store.ListEncryptedFiles()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("repository files = %v, want only existing collision", entries)
	}
}

func TestSigningSyncPushRejectsDeviceWithoutCreateMissing(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "repository-password")
	clientCalls := 0
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalls++
		return nil, errors.New("client must not be created")
	}))

	cmd := syncPushCommand()
	cmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Parse([]string{
			"--bundle-id", "com.example.app",
			"--profile-type", "IOS_APP_DEVELOPMENT",
			"--repo", "git@github.com:team/certs.git",
			"--device", "DEVICE1",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = cmd.Run(context.Background())
	})

	if runErr == nil || runErr.Error() != deviceWithoutCreateMissingError {
		t.Fatalf("error = %v, want %q", runErr, deviceWithoutCreateMissingError)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("error = %v, want usage error", runErr)
	}
	if clientCalls != 0 {
		t.Fatalf("client factory calls = %d, want 0", clientCalls)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "Error: "+deviceWithoutCreateMissingError+"\n" {
		t.Fatalf("stderr = %q", stderr)
	}
	if strings.Contains(stderr, "Cloning signing repo") {
		t.Fatalf("stderr shows repository side effects: %q", stderr)
	}
}

func TestSanitizeRepoURLForOutputRedactsCredentials(t *testing.T) {
	raw := "https://token:secret@example.com/org/repo.git?access_token=abc123"
	got := sanitizeRepoURLForOutput(raw)

	if strings.Contains(got, "token:secret@") || strings.Contains(got, "secret") || strings.Contains(got, "abc123") {
		t.Fatalf("expected credentials to be redacted, got %q", got)
	}
	if !strings.Contains(got, "%5BREDACTED%5D") {
		t.Fatalf("expected sanitized marker, got %q", got)
	}
}

func TestSigningCommandLongHelpUsesOutputDirForSyncPull(t *testing.T) {
	cmd := SigningCommand()
	if !strings.Contains(cmd.LongHelp, "asc signing sync pull --repo git@github.com:team/certs.git --output-dir ./signing") {
		t.Fatalf("expected top-level help to use --output-dir for sync pull, got %q", cmd.LongHelp)
	}
	if strings.Contains(cmd.LongHelp, "asc signing sync pull --repo git@github.com:team/certs.git --output ./signing") {
		t.Fatalf("expected top-level help to avoid --output for sync pull, got %q", cmd.LongHelp)
	}
}

func TestSigningSyncCommandLongHelpDocumentsSelectivePull(t *testing.T) {
	cmd := SigningSyncCommand()
	if !strings.Contains(cmd.LongHelp, "asc signing sync pull --repo git@github.com:team/certs.git --bundle-id com.example.app") ||
		!strings.Contains(cmd.LongHelp, "--profile-type IOS_APP_STORE") {
		t.Fatalf("expected pull example to document target selection, got %q", cmd.LongHelp)
	}
}

func TestWriteDecryptedOutputFileWritesPlaintext(t *testing.T) {
	outDir := t.TempDir()
	relPath := filepath.Join("profiles", "appstore", "app.mobileprovision")
	plaintext := []byte("profile-data")

	if err := writeDecryptedOutputFile(outDir, relPath, plaintext, false); err != nil {
		t.Fatalf("writeDecryptedOutputFile: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outDir, relPath))
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("output mismatch: got %q, want %q", got, plaintext)
	}
	info, err := os.Stat(filepath.Join(outDir, relPath))
	if err != nil {
		t.Fatalf("stat output file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("output mode = %o, want 600", info.Mode().Perm())
	}
}

func TestWriteDecryptedOutputFilePreservesExistingNonSensitiveMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows mode bits do not model Unix file permissions")
	}
	outDir := t.TempDir()
	relPath := filepath.Join("profiles", "appstore", "app.mobileprovision")
	destination := filepath.Join(outDir, relPath)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := writeDecryptedOutputFile(outDir, relPath, []byte("new"), false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
}

func TestResolveSyncPasswordPrefersProtectedFileThenEnvironment(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "environment-password")
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("file-password\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	password, err := resolveSyncPassword(path)
	if err != nil {
		t.Fatalf("resolveSyncPassword(file) error = %v", err)
	}
	if password != "file-password" {
		t.Fatalf("file result = %q", password)
	}

	password, err = resolveSyncPassword("")
	if err != nil {
		t.Fatalf("resolveSyncPassword(env) error = %v", err)
	}
	if password != "environment-password" {
		t.Fatalf("environment result = %q", password)
	}

	t.Setenv(signingSyncPasswordEnvVar, "")
	_, err = resolveSyncPassword("")
	if err == nil || err.Error() != "--password-file is required (or set ASC_SIGNING_SYNC_PASSWORD)" {
		t.Fatalf("resolveSyncPassword(missing) error = %v, want missing --password-file usage error", err)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want usage error", err)
	}
}

func TestResolveSyncPasswordPreservesEnvironmentWhitespace(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "  environment password  ")
	password, err := resolveSyncPassword("")
	if err != nil {
		t.Fatal(err)
	}
	if password != "  environment password  " {
		t.Fatalf("password = %q", password)
	}

	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("file password \n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	password, err = resolveSyncPassword(path)
	if err != nil {
		t.Fatal(err)
	}
	if password != "file password \n" {
		t.Fatalf("file password = %q", password)
	}
}

func TestResolveSyncPasswordIgnoresLegacyMatchPasswordEnvironment(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "")
	t.Setenv("ASC_MATCH_PASSWORD", "legacy-environment-password")

	password, err := resolveSyncPassword("")
	if err == nil {
		t.Fatalf("resolveSyncPassword() = %q, want missing --password-file usage error", password)
	}
	if err.Error() != "--password-file is required (or set ASC_SIGNING_SYNC_PASSWORD)" {
		t.Fatalf("error = %q, want error naming only the supported sources", err.Error())
	}
	if strings.Contains(err.Error(), "ASC_MATCH_PASSWORD") || strings.Contains(err.Error(), "--password ") {
		t.Fatalf("error = %q mentions removed legacy sources", err.Error())
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want usage error", err)
	}
}

func TestSigningSyncIgnoresLegacyMatchPasswordEnvironmentWithoutWarning(t *testing.T) {
	tests := []struct {
		name string
		cmd  *ffcli.Command
		args []string
	}{
		{
			name: "push",
			cmd:  syncPushCommand(),
			args: []string{
				"--bundle-id", "com.example.app",
				"--profile-type", "IOS_APP_STORE",
				"--repo", "git@example.com:team/signing.git",
			},
		},
		{
			name: "pull",
			cmd:  syncPullCommand(),
			args: []string{
				"--repo", "git@example.com:team/signing.git",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(signingSyncPasswordEnvVar, "")
			t.Setenv("ASC_MATCH_PASSWORD", "legacy-environment-password")
			clientCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("client must not be created")
			}))
			if err := tt.cmd.Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				runErr = tt.cmd.Run(context.Background())
			})
			if runErr == nil || runErr.Error() != "--password-file is required (or set ASC_SIGNING_SYNC_PASSWORD)" {
				t.Fatalf("error = %v, want missing --password-file usage error", runErr)
			}
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, "Error: --password-file is required (or set ASC_SIGNING_SYNC_PASSWORD)\n") {
				t.Fatalf("stderr = %q", stderr)
			}
			if strings.Contains(stderr, "Warning") || strings.Contains(stderr, "ASC_MATCH_PASSWORD") || strings.Contains(stderr, "deprecated") {
				t.Fatalf("stderr mentions removed legacy sources: %q", stderr)
			}
			if strings.Contains(stderr, "Cloning signing repo") {
				t.Fatalf("stderr shows repository side effects: %q", stderr)
			}
			if clientCalls != 0 {
				t.Fatalf("client factory calls = %d, want 0", clientCalls)
			}
		})
	}
}

func TestSigningSyncDoesNotDefineLegacyPasswordFlag(t *testing.T) {
	for _, tt := range []struct {
		name string
		cmd  *ffcli.Command
	}{
		{name: "push", cmd: syncPushCommand()},
		{name: "pull", cmd: syncPullCommand()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cmd.FlagSet.Lookup("password") != nil {
				t.Fatalf("signing sync %s still defines the removed --password flag", tt.name)
			}
			if tt.cmd.FlagSet.Lookup("password-file") == nil {
				t.Fatalf("signing sync %s is missing --password-file", tt.name)
			}
		})
	}
}

func TestClassifySigningFileKeepsIdentitySeparateFromOtherSensitiveFiles(t *testing.T) {
	sensitive, identity, err := classifySigningFile("future/secret.bin", nil, signingpkg.EncryptedFileMetadata{Kind: "future-secret", Sensitive: true}, "password")
	if err != nil || !sensitive || identity {
		t.Fatalf("future secret classified as sensitive=%v identity=%v", sensitive, identity)
	}
	_, _, err = classifySigningFile("certs/distribution/cert.cer", nil, signingpkg.EncryptedFileMetadata{Kind: "pkcs12-identity", Sensitive: true}, "password")
	if err == nil {
		t.Fatal("PKCS#12 identity metadata outside identity path was accepted")
	}
}

func TestPrepareDecryptedSigningFilesRejectsLegacyCiphertextAtIdentityPathWithoutWriting(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	identityPath := filepath.Join("identities", "distribution", strings.Repeat("A", 64)+".p12")
	if err := store.WriteEncryptedFile(identityPath, []byte("not-an-identity"), "password"); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	destination := filepath.Join(outDir, identityPath)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareDecryptedSigningFiles(store, []string{identityPath}, "password", outDir); err == nil || !strings.Contains(err.Error(), "versioned sensitive") {
		t.Fatalf("legacy identity substitution error = %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "existing" {
		t.Fatalf("existing identity changed to %q", got)
	}
}

func TestPrepareDecryptedSigningFilesRejectsLegacyCiphertextAtArbitraryP12Path(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	identityPath := filepath.Join("nested", "Renamed.P12")
	if err := store.WriteEncryptedFile(identityPath, []byte("not-an-identity"), "password"); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareDecryptedSigningFiles(store, []string{identityPath}, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "versioned sensitive") {
		t.Fatalf("arbitrary P12 substitution error = %v", err)
	}
}

func TestPrepareDecryptedSigningFilesRejectsIdentityMetadataMismatchWithoutWriting(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 20)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	normalized, err := normalizeSigningIdentity(identity, "password")
	if err != nil {
		t.Fatal(err)
	}
	identityPath := filepath.Join("identities", "distribution", identity.CertificateSHA256+".p12")
	if err := store.WriteEncryptedFileWithMetadata(identityPath, normalized, "password", signingpkg.EncryptedFileMetadata{
		Kind: "pkcs12-identity", Sensitive: true, CertificateSHA256: identity.CertificateSHA256, TeamID: "WRONGTEAM",
	}); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	if _, err := prepareDecryptedSigningFiles(store, []string{identityPath}, "password", outDir); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("identity metadata mismatch error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, identityPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("identity output exists after mismatch: %v", err)
	}
}

func TestPrepareDecryptedSigningFilesRequiresIdentityCoreContextIntegrity(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 24)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	artifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	profilePath, profileContent := bindTestSigningIdentityArtifacts(t, artifacts, certificate, key, "com.example.app", "IOS_APP_ADHOC", "profile-context")

	t.Run("core without context", func(t *testing.T) {
		store := &signingpkg.GitStore{LocalDir: t.TempDir()}
		if err := store.WriteEncryptedFileWithMetadata(artifacts.IdentityPath, artifacts.IdentityData, "password", artifacts.IdentityMetadata); err != nil {
			t.Fatal(err)
		}
		decrypted, err := prepareDecryptedSigningFiles(store, []string{artifacts.IdentityPath}, "password", t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if len(decrypted) != 0 {
			t.Fatalf("unreferenced core remained active: %#v", decrypted)
		}
	})

	t.Run("orphan context", func(t *testing.T) {
		store := &signingpkg.GitStore{LocalDir: t.TempDir()}
		if err := store.WriteEncryptedFileWithMetadata(artifacts.BindingPath, artifacts.BindingData, "password", artifacts.BindingMetadata); err != nil {
			t.Fatal(err)
		}
		if _, err := prepareDecryptedSigningFiles(store, []string{artifacts.BindingPath}, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "no matching core") {
			t.Fatalf("orphan context error = %v", err)
		}
	})

	t.Run("matched graph", func(t *testing.T) {
		store := &signingpkg.GitStore{LocalDir: t.TempDir()}
		if err := store.WriteEncryptedFile(profilePath, profileContent, "password"); err != nil {
			t.Fatal(err)
		}
		if err := writeOrReuseSigningIdentityArtifacts(store, artifacts, "password"); err != nil {
			t.Fatal(err)
		}
		if _, err := prepareDecryptedSigningFiles(store, []string{profilePath, artifacts.IdentityPath, artifacts.BindingPath}, "password", t.TempDir()); err != nil {
			t.Fatalf("matched identity graph rejected: %v", err)
		}
	})
}

func TestIdentityContextCurrentPointerReplacesOldProfileBinding(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 27)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}

	oldArtifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	oldProfilePath, oldProfileContent := bindTestSigningIdentityArtifacts(t, oldArtifacts, certificate, key, "com.example.app", "IOS_APP_ADHOC", "old-profile")
	if err := store.WriteEncryptedFile(oldProfilePath, oldProfileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, oldArtifacts, "password"); err != nil {
		t.Fatal(err)
	}

	newArtifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	newProfilePath, newProfileContent := bindTestSigningIdentityArtifacts(t, newArtifacts, certificate, key, "com.example.app", "IOS_APP_ADHOC", "new-profile")
	if oldArtifacts.BindingPath != newArtifacts.BindingPath {
		t.Fatalf("current context paths differ: old=%q new=%q", oldArtifacts.BindingPath, newArtifacts.BindingPath)
	}
	if err := preflightSigningIdentityArtifactsForContextUpdate(store, newArtifacts, "password"); err != nil {
		t.Fatalf("pre-create renewal preflight: %v", err)
	}
	if err := store.WriteEncryptedFile(newProfilePath, newProfileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := preflightSigningIdentityArtifactsForContextUpdate(store, newArtifacts, "password"); err != nil {
		t.Fatalf("post-create renewal preflight: %v", err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, newArtifacts, "password"); err != nil {
		t.Fatalf("replace current context: %v", err)
	}
	files := []string{oldProfilePath, newProfilePath, newArtifacts.IdentityPath, newArtifacts.BindingPath}
	if _, err := prepareDecryptedSigningFiles(store, files, "password", t.TempDir()); err != nil {
		t.Fatalf("current context graph with retained old profile rejected: %v", err)
	}
}

func TestIdentityContextCurrentPointerSupportsCertificateRotationWithRetainedOldCore(t *testing.T) {
	oldKey := mustECKey(t)
	oldCertificate := mustSigningCertificate(t, oldKey, 29)
	oldIdentity := &signingIdentity{PrivateKey: oldKey, Certificate: oldCertificate, CertificateSHA256: certificateSHA256(oldCertificate)}
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	oldArtifacts, err := prepareSigningIdentityArtifacts(oldIdentity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	oldProfilePath, oldProfileContent := bindTestSigningIdentityArtifacts(t, oldArtifacts, oldCertificate, oldKey, "com.example.app", "IOS_APP_ADHOC", "old-cert-profile")
	if err := store.WriteEncryptedFile(oldProfilePath, oldProfileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, oldArtifacts, "password"); err != nil {
		t.Fatal(err)
	}

	newKey := mustECKey(t)
	newCertificate := mustSigningCertificate(t, newKey, 30)
	newIdentity := &signingIdentity{PrivateKey: newKey, Certificate: newCertificate, CertificateSHA256: certificateSHA256(newCertificate)}
	newArtifacts, err := prepareSigningIdentityArtifacts(newIdentity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	if oldArtifacts.BindingPath != newArtifacts.BindingPath {
		t.Fatalf("rotation context paths differ: old=%q new=%q", oldArtifacts.BindingPath, newArtifacts.BindingPath)
	}
	if err := preflightSigningIdentityArtifactsForContextUpdate(store, newArtifacts, "password"); err != nil {
		t.Fatalf("rotation preflight: %v", err)
	}
	newProfilePath, newProfileContent := bindTestSigningIdentityArtifacts(t, newArtifacts, newCertificate, newKey, "com.example.app", "IOS_APP_ADHOC", "new-cert-profile")
	if err := store.WriteEncryptedFile(newProfilePath, newProfileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, newArtifacts, "password"); err != nil {
		t.Fatalf("rotation write: %v", err)
	}
	files := []string{oldProfilePath, newProfilePath, oldArtifacts.IdentityPath, newArtifacts.IdentityPath, newArtifacts.BindingPath}
	decrypted, err := prepareDecryptedSigningFiles(store, files, "password", t.TempDir())
	if err != nil {
		t.Fatalf("rotation graph rejected: %v", err)
	}
	for _, file := range decrypted {
		if file.RelativePath == oldArtifacts.IdentityPath {
			t.Fatal("retained old identity core was selected for output")
		}
	}
}

func TestPrepareDecryptedSigningFilesIgnoresRetainedExpiredUnreferencedCore(t *testing.T) {
	expiredKey := mustECKey(t)
	expiredCertificate := mustSigningCertificateWithValidity(t, expiredKey, 31, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	expiredIdentity := &signingIdentity{PrivateKey: expiredKey, Certificate: expiredCertificate, CertificateSHA256: certificateSHA256(expiredCertificate)}
	expiredArtifacts, err := prepareSigningIdentityArtifacts(expiredIdentity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	if err := store.WriteEncryptedFileWithMetadata(expiredArtifacts.IdentityPath, expiredArtifacts.IdentityData, "password", expiredArtifacts.IdentityMetadata); err != nil {
		t.Fatal(err)
	}

	activeKey := mustECKey(t)
	activeCertificate := mustSigningCertificate(t, activeKey, 32)
	activeIdentity := &signingIdentity{PrivateKey: activeKey, Certificate: activeCertificate, CertificateSHA256: certificateSHA256(activeCertificate)}
	activeArtifacts, err := prepareSigningIdentityArtifacts(activeIdentity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	profilePath, profileContent := bindTestSigningIdentityArtifacts(t, activeArtifacts, activeCertificate, activeKey, "com.example.app", "IOS_APP_ADHOC", "active-profile")
	if err := store.WriteEncryptedFile(profilePath, profileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, activeArtifacts, "password"); err != nil {
		t.Fatal(err)
	}
	files := []string{profilePath, expiredArtifacts.IdentityPath, activeArtifacts.IdentityPath, activeArtifacts.BindingPath}
	decrypted, err := prepareDecryptedSigningFiles(store, files, "password", t.TempDir())
	if err != nil {
		t.Fatalf("retained expired unreferenced core rejected: %v", err)
	}
	for _, file := range decrypted {
		if file.RelativePath == expiredArtifacts.IdentityPath {
			t.Fatal("retained expired identity core was selected for output")
		}
	}
}

func TestPrepareDecryptedSigningFilesRejectsInvalidActiveIdentityCoreValidity(t *testing.T) {
	tests := []struct {
		name      string
		notBefore time.Time
		notAfter  time.Time
	}{
		{name: "expired", notBefore: time.Now().Add(-48 * time.Hour), notAfter: time.Now().Add(-24 * time.Hour)},
		{name: "not yet valid", notBefore: time.Now().Add(24 * time.Hour), notAfter: time.Now().Add(48 * time.Hour)},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := mustECKey(t)
			certificate := mustSigningCertificateWithValidity(t, key, int64(33+index), test.notBefore, test.notAfter)
			identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
			artifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_ADHOC")
			if err != nil {
				t.Fatal(err)
			}
			signerKey := mustECKey(t)
			signerCertificate := mustSigningCertificate(t, signerKey, int64(35+index))
			profilePath, profileContent := bindTestSigningIdentityArtifactsWithSigner(t, artifacts, certificate, signerCertificate, signerKey, "com.example.app", "IOS_APP_ADHOC", "invalid-core-profile")
			store := &signingpkg.GitStore{LocalDir: t.TempDir()}
			if err := store.WriteEncryptedFile(profilePath, profileContent, "password"); err != nil {
				t.Fatal(err)
			}
			if err := writeOrReuseSigningIdentityArtifacts(store, artifacts, "password"); err != nil {
				t.Fatal(err)
			}
			if _, err := prepareDecryptedSigningFiles(store, []string{profilePath, artifacts.IdentityPath, artifacts.BindingPath}, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "not currently valid") {
				t.Fatalf("invalid active identity error = %v", err)
			}
		})
	}
}

func TestIdentityContextRejectsProfileDistributionTypeMismatch(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 28)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	artifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_STORE")
	if err != nil {
		t.Fatal(err)
	}
	profilePath, profileContent := bindTestSigningIdentityArtifacts(t, artifacts, certificate, key, "com.example.app", "IOS_APP_STORE", "store-profile")
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	if err := store.WriteEncryptedFile(profilePath, profileContent, "password"); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, artifacts, "password"); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareDecryptedSigningFiles(store, []string{profilePath, artifacts.IdentityPath, artifacts.BindingPath}, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "distribution type") {
		t.Fatalf("profile type mismatch error = %v", err)
	}
}

func TestValidateIdentityArtifactGraphRejectsConflictingContextScope(t *testing.T) {
	binding := func(certificate string) []byte {
		data, err := json.Marshal(identityContextBinding{CertificateSHA256: certificate, TeamID: "TEAM123", BundleID: "com.example.app", ProfileType: "IOS_APP_ADHOC"})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	files := []decryptedSigningFile{
		{RelativePath: "identities/distribution/AAA.p12", Metadata: signingpkg.EncryptedFileMetadata{Kind: "pkcs12-identity", TeamID: "TEAM123"}},
		{RelativePath: "identities/distribution/BBB.p12", Metadata: signingpkg.EncryptedFileMetadata{Kind: "pkcs12-identity", TeamID: "TEAM123"}},
		{RelativePath: "identity-contexts/AAA.json", Plaintext: binding("AAA"), Metadata: signingpkg.EncryptedFileMetadata{Kind: "identity-context"}},
		{RelativePath: "identity-contexts/BBB.json", Plaintext: binding("BBB"), Metadata: signingpkg.EncryptedFileMetadata{Kind: "identity-context"}},
	}
	if _, err := validateIdentityArtifactGraph(files); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflicting graph error = %v", err)
	}
}

func TestIdentityArtifactGraphRejectsCaseFoldedRepositoryPaths(t *testing.T) {
	for name, paths := range map[string][]string{
		"profile": {
			"profiles/adhoc/Release.mobileprovision",
			"profiles/adhoc/release.mobileprovision",
		},
		"certificate": {
			"certs/distribution/ABC.cer",
			"certs/distribution/abc.cer",
		},
		"identity": {
			"identities/distribution/ABC.p12",
			"identities/distribution/abc.p12",
		},
		"context": {
			"identity-contexts/ABC.json",
			"identity-contexts/abc.json",
		},
	} {
		t.Run(name, func(t *testing.T) {
			files := []decryptedSigningFile{{RelativePath: paths[0]}, {RelativePath: paths[1]}}
			if _, err := validateIdentityArtifactGraph(files); err == nil || err.Error() != "encrypted repository paths collide under Windows Unicode case folding" {
				t.Fatalf("validateIdentityArtifactGraph() error = %v", err)
			}
		})
	}
}

func TestPrepareDecryptedSigningFilesRejectsCaseCollisionBeforeOutput(t *testing.T) {
	outDir := t.TempDir()
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	_, err := prepareDecryptedSigningFiles(store, []string{
		"profiles/adhoc/Release.mobileprovision",
		"profiles/adhoc/release.mobileprovision",
	}, "password", outDir)
	if err == nil || err.Error() != "encrypted repository paths collide under Windows Unicode case folding" {
		t.Fatalf("prepareDecryptedSigningFiles() error = %v", err)
	}
	entries, readErr := os.ReadDir(outDir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("collision created output entries: %v", entries)
	}
}

func TestValidateIdentityArtifactGraphRejectsCrossTeamContext(t *testing.T) {
	binding, err := json.Marshal(identityContextBinding{CertificateSHA256: "AAA", TeamID: "TEAM-B", BundleID: "com.example.app", ProfileType: "IOS_APP_ADHOC"})
	if err != nil {
		t.Fatal(err)
	}
	files := []decryptedSigningFile{
		{RelativePath: "identities/distribution/AAA.p12", Metadata: signingpkg.EncryptedFileMetadata{Kind: "pkcs12-identity", TeamID: "TEAM-A"}},
		{RelativePath: "identity-contexts/AAA.json", Plaintext: binding, Metadata: signingpkg.EncryptedFileMetadata{Kind: "identity-context"}},
	}
	if _, err := validateIdentityArtifactGraph(files); err == nil || !strings.Contains(err.Error(), "team") {
		t.Fatalf("cross-team graph error = %v", err)
	}
}

func TestPrepareDecryptedSigningFilesBoundsRepositoryBeforeDecrypt(t *testing.T) {
	paths := make([]string, maxEncryptedSigningFiles+1)
	for i := range paths {
		paths[i] = fmt.Sprintf("certs/distribution/%03d.cer", i)
	}
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	if _, err := prepareDecryptedSigningFiles(store, paths, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "files; limit") {
		t.Fatalf("file count error = %v", err)
	}

	paths = nil
	for i := 0; i < 4; i++ {
		relPath := fmt.Sprintf("certs/distribution/large-%d.cer", i)
		path := filepath.Join(store.LocalDir, relPath+".enc")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(33 << 20); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, relPath)
	}
	if _, err := prepareDecryptedSigningFiles(store, paths, "password", t.TempDir()); err == nil || !strings.Contains(err.Error(), "cumulative size limit") {
		t.Fatalf("cumulative size error = %v", err)
	}
}

func TestSigningSyncPushRejectsIdentityFlagConflictsBeforeSecretsOrClient(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "identity and private key",
			args: []string{"--identity", "identity.p12", "--private-key", "key.pem"},
			want: "--identity and --private-key are mutually exclusive",
		},
		{
			name: "fingerprint without identity",
			args: []string{"--identity-sha256", strings.Repeat("A", 64)},
			want: "--identity-sha256 requires --identity or --private-key",
		},
		{
			name: "identity password without identity",
			args: []string{"--identity-password-file", "password"},
			want: "--identity-password-file requires --identity",
		},
		{
			name: "private key without fingerprint",
			args: []string{"--private-key", "key.pem"},
			want: "--identity-sha256 is required with --private-key to select one App Store Connect certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := syncPushCommand()
			base := []string{
				"--bundle-id", "com.example.app",
				"--profile-type", "IOS_APP_STORE",
				"--repo", "git@example.com:team/signing.git",
			}
			if err := cmd.Parse(append(base, tt.args...)); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
		})
	}
}

func TestSigningSyncRejectsBlankPasswordFile(t *testing.T) {
	tests := []struct {
		name string
		cmd  *ffcli.Command
		args []string
	}{
		{
			name: "push",
			cmd:  syncPushCommand(),
			args: []string{
				"--bundle-id", "com.example.app",
				"--profile-type", "IOS_APP_STORE",
				"--repo", "git@example.com:team/signing.git",
				"--password-file", " \t ",
			},
		},
		{
			name: "pull",
			cmd:  syncPullCommand(),
			args: []string{
				"--repo", "git@example.com:team/signing.git",
				"--password-file", " \t ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(signingSyncPasswordEnvVar, "environment-fallback-must-not-be-used")
			clientCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("client must not be created")
			}))
			if err := tt.cmd.Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				runErr = tt.cmd.Run(context.Background())
			})
			err := runErr
			if err == nil || err.Error() != "--password-file must not be empty" {
				t.Fatalf("error = %v, want blank password-file usage error", err)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, "Error: --password-file must not be empty\n") {
				t.Fatalf("stderr = %q", stderr)
			}
			if strings.Contains(stderr, "Cloning signing repo") {
				t.Fatalf("stderr shows repository side effects: %q", stderr)
			}
			if clientCalls != 0 {
				t.Fatalf("client factory calls = %d, want 0", clientCalls)
			}
		})
	}
}

func TestSigningSyncPushDirectDistributionIdentityLoadFailureIsOperational(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "repository-password")
	for _, profileType := range []string{"MAC_APP_DIRECT", "MAC_CATALYST_APP_DIRECT"} {
		t.Run(profileType, func(t *testing.T) {
			cmd := syncPushCommand()
			if err := cmd.Parse([]string{
				"--bundle-id", "com.example.app",
				"--profile-type", profileType,
				"--repo", "git@example.com:team/signing.git",
				"--identity", filepath.Join(t.TempDir(), "missing.p12"),
			}); err != nil {
				t.Fatal(err)
			}
			err := cmd.Run(context.Background())
			if err == nil || !strings.Contains(err.Error(), "signing sync push: signing identity") {
				t.Fatalf("error = %v, want operational identity-load failure", err)
			}
			if errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want operational error", err)
			}
		})
	}
}

func TestIdentityProfileTypeMatchesDirectRequiresAllDeviceClaim(t *testing.T) {
	profile := &identityMobileProvision{
		Entitlements: map[string]any{"get-task-allow": false},
		Platform:     []string{"OSX"},
	}
	for _, profileType := range []string{"MAC_APP_DIRECT", "MAC_CATALYST_APP_DIRECT"} {
		if identityProfileTypeMatches(profile, profileType) {
			t.Fatalf("%s matched without the all-device claim", profileType)
		}
		profile.ProvisionsAllDevices = true
		if !identityProfileTypeMatches(profile, profileType) {
			t.Fatalf("%s did not match its all-device claim", profileType)
		}
		profile.ProvisionedDevices = []string{"DEVICE1"}
		if identityProfileTypeMatches(profile, profileType) {
			t.Fatalf("%s matched with an explicit device list", profileType)
		}
		profile.ProvisionedDevices = nil
		profile.ProvisionsAllDevices = false
	}
	profile.ProvisionsAllDevices = true
	profile.Entitlements["get-task-allow"] = true
	if identityProfileTypeMatches(profile, "MAC_APP_DIRECT") {
		t.Fatal("direct profile matched with debugging enabled")
	}
}

func TestIdentityProfileTypeMatchesDirectRequiresMacOSPlatformClaim(t *testing.T) {
	profile := &identityMobileProvision{
		Entitlements:         map[string]any{"get-task-allow": false},
		ProvisionsAllDevices: true,
	}
	for _, platform := range [][]string{nil, {}, {"iOS"}, {"OSX", "iOS"}} {
		profile.Platform = platform
		if identityProfileTypeMatches(profile, "MAC_APP_DIRECT") {
			t.Fatalf("direct profile matched platform claim %v", platform)
		}
	}
	profile.Platform = []string{"OSX"}
	if !identityProfileTypeMatches(profile, "MAC_APP_DIRECT") {
		t.Fatal("direct profile did not match its macOS platform claim")
	}
}

func TestSigningSyncPushIdentityLoadFailureIsOperational(t *testing.T) {
	t.Setenv(signingSyncPasswordEnvVar, "repository-password")
	cmd := syncPushCommand()
	if err := cmd.Parse([]string{
		"--bundle-id", "com.example.app",
		"--profile-type", "IOS_APP_STORE",
		"--repo", "git@example.com:team/signing.git",
		"--identity", filepath.Join(t.TempDir(), "missing.p12"),
	}); err != nil {
		t.Fatal(err)
	}

	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "signing sync push: signing identity") {
		t.Fatalf("error = %v, want signing identity load failure", err)
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want operational error", err)
	}
}

func TestWriteDecryptedSensitiveOutputRefusesExistingFile(t *testing.T) {
	outDir := t.TempDir()
	relPath := filepath.Join("identities", "distribution", "ABC.p12")
	destination := filepath.Join(outDir, relPath)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeDecryptedOutputFile(outDir, relPath, []byte("identity"), true); err == nil || !errors.Is(err, os.ErrExist) {
		t.Fatalf("error = %v, want existing-file refusal", err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("existing output mode = %o, want unchanged 644", info.Mode().Perm())
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("existing output = %q, want unchanged", data)
	}
}

func TestPrepareDecryptedSigningFilesPreflightsAllDestinationsBeforeWriting(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	password := "repository-password"
	certificatePath := filepath.Join("certs", "distribution", "certificate.cer")
	identityPath := filepath.Join("future-secrets", "credential.bin")
	if err := store.WriteEncryptedFile(certificatePath, []byte("new-certificate"), password); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteEncryptedFileWithMetadata(identityPath, []byte("identity"), password, signingpkg.EncryptedFileMetadata{
		Kind: "future-secret", Sensitive: true,
	}); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	certificateDestination := filepath.Join(outDir, certificatePath)
	identityDestination := filepath.Join(outDir, identityPath)
	if err := os.MkdirAll(filepath.Dir(certificateDestination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certificateDestination, []byte("existing-certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(identityDestination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(identityDestination, []byte("existing-identity"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := prepareDecryptedSigningFiles(store, []string{certificatePath, identityPath}, password, outDir)
	if err == nil || !errors.Is(err, os.ErrExist) {
		t.Fatalf("prepareDecryptedSigningFiles() error = %v, want identity collision", err)
	}
	certificate, readErr := os.ReadFile(certificateDestination)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(certificate) != "existing-certificate" {
		t.Fatalf("earlier destination changed to %q", certificate)
	}
}

func TestPrepareDecryptedSigningFilesDecryptsAllBeforeDestinationPreflight(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	password := "repository-password"
	certificatePath := filepath.Join("certs", "distribution", "certificate.cer")
	corruptPath := filepath.Join("profiles", "adhoc", "app.mobileprovision")
	if err := store.WriteEncryptedFile(certificatePath, []byte("new-certificate"), password); err != nil {
		t.Fatal(err)
	}
	corruptDestination := filepath.Join(store.LocalDir, corruptPath+".enc")
	if err := os.MkdirAll(filepath.Dir(corruptDestination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corruptDestination, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	certificateDestination := filepath.Join(outDir, certificatePath)
	if err := os.MkdirAll(filepath.Dir(certificateDestination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certificateDestination, []byte("existing-certificate"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := prepareDecryptedSigningFiles(store, []string{certificatePath, corruptPath}, password, outDir); err == nil {
		t.Fatal("corrupt later artifact was accepted")
	}
	certificate, readErr := os.ReadFile(certificateDestination)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(certificate) != "existing-certificate" {
		t.Fatalf("earlier destination changed to %q", certificate)
	}
}

func TestWriteDecryptedOutputFileRejectsSymlinkTarget(t *testing.T) {
	outDir := t.TempDir()
	targetDir := t.TempDir()
	relPath := filepath.Join("profiles", "appstore", "app.mobileprovision")
	destPath := filepath.Join(outDir, relPath)
	targetPath := filepath.Join(targetDir, "app.mobileprovision")

	if err := os.WriteFile(targetPath, []byte("original"), 0o600); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		t.Fatalf("mkdir output parent: %v", err)
	}
	if err := os.Symlink(targetPath, destPath); err != nil {
		t.Fatalf("create destination symlink: %v", err)
	}

	err := writeDecryptedOutputFile(outDir, relPath, []byte("updated"), false)
	if err == nil {
		t.Fatal("expected symlink rejection error, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}

	got, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("read target file: %v", readErr)
	}
	if string(got) != "original" {
		t.Fatalf("did not expect write through symlink target, got %q", got)
	}
}
