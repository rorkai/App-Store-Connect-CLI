package cmdtest

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	legacyPKCS12 "github.com/bitrise-io/go-pkcs12"
	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"go.mozilla.org/pkcs7"
	"howett.net/plist"
	modernPKCS12 "software.sslmate.com/src/go-pkcs12"
)

func TestSigningSyncPush_RejectsConflictingIdentityInputsBeforeAuth(t *testing.T) {
	root := RootCommand("test")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"signing", "sync", "push",
			"--bundle-id", "com.example.app",
			"--profile-type", "IOS_APP_ADHOC",
			"--repo", "git@example.com:team/signing.git",
			"--identity", "identity.p12",
			"--private-key", "identity-key.pem",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("error = %v, want usage error", err)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Error: --identity and --private-key are mutually exclusive") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestSigningSyncPushIdentityLoadFailuresUseOperationalExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_SIGNING_SYNC_PASSWORD", "repository-password")
	clientCalls := 0
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalls++
		return nil, errors.New("client must not be created")
	}))

	fixtureDir := t.TempDir()
	badPKCS12 := filepath.Join(fixtureDir, "bad.p12")
	writeSigningSyncProtectedFile(t, badPKCS12, []byte("not a PKCS#12 identity"))
	badKey := filepath.Join(fixtureDir, "bad-key.pem")
	writeSigningSyncProtectedFile(t, badKey, []byte("not a PEM private key"))
	permissiveIdentity := filepath.Join(fixtureDir, "permissive.p12")
	writeSigningSyncProtectedFile(t, permissiveIdentity, []byte("not a PKCS#12 identity"))
	if err := os.Chmod(permissiveIdentity, 0o644); err != nil {
		t.Fatal(err)
	}

	base := []string{
		"signing", "sync", "push",
		"--bundle-id", "com.example.app",
		"--profile-type", "IOS_APP_STORE",
		"--repo", "git@example.com:team/signing.git",
	}
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "missing identity",
			args:       []string{"--identity", filepath.Join(fixtureDir, "missing.p12")},
			wantStderr: "PKCS#12 identity file does not exist",
		},
		{
			name:       "bad PKCS12",
			args:       []string{"--identity", badPKCS12},
			wantStderr: "decode PKCS#12 identity",
		},
		{
			name:       "bad private key",
			args:       []string{"--private-key", badKey, "--identity-sha256", strings.Repeat("A", 64)},
			wantStderr: "private key is not PEM encoded",
		},
		{
			name:       "missing identity password file",
			args:       []string{"--identity", badPKCS12, "--identity-password-file", filepath.Join(fixtureDir, "missing-password")},
			wantStderr: "identity password file does not exist",
		},
	}
	if runtime.GOOS != "windows" {
		tests = append(tests, struct {
			name       string
			args       []string
			wantStderr string
		}{
			name:       "unreadable identity permissions",
			args:       []string{"--identity", permissiveIdentity},
			wantStderr: "PKCS#12 identity file permissions must be 0600 or more restrictive",
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var exitCode int
			stdout, stderr := captureOutput(t, func() {
				exitCode = rootcmd.Run(append(append([]string{}, base...), tt.args...), "test")
			})
			if exitCode != rootcmd.ExitError {
				t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, rootcmd.ExitError, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.wantStderr)
			}
		})
	}
	if clientCalls != 0 {
		t.Fatalf("client factory calls = %d, want 0", clientCalls)
	}
}

func TestSigningSyncPushInvalidIdentityCombinationUsesUsageExitCode(t *testing.T) {
	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run([]string{
			"signing", "sync", "push",
			"--bundle-id", "com.example.app",
			"--profile-type", "IOS_APP_STORE",
			"--repo", "git@example.com:team/signing.git",
			"--identity", "identity.p12",
			"--private-key", "identity-key.pem",
		}, "test")
	})
	if exitCode != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--identity and --private-key are mutually exclusive") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestSigningSyncPushReportsIdentityFilterCause(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_SIGNING_SYNC_PASSWORD", "repository-password")
	localKey, localCertificate, _ := signingSyncIdentityFixture(t)
	_, ascCertificate, _ := signingSyncIdentityFixture(t)
	identityFile := filepath.Join(t.TempDir(), "identity.p12")
	identity, err := legacyPKCS12.Encode(rand.Reader, localKey, localCertificate, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	writeSigningSyncProtectedFile(t, identityFile, identity)

	certificateContent := base64.StdEncoding.EncodeToString(ascCertificate.Raw)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/bundleIds":
			writeSigningSyncJSON(t, w, `{"data":[{"type":"bundleIds","id":"bundle-main","attributes":{"identifier":"com.example.app"}}]}`)
		case "/v1/bundleIds/bundle-main/profiles":
			writeSigningSyncJSON(t, w, `{"data":[]}`)
		case "/v1/certificates":
			writeSigningSyncJSON(t, w, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-main","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":%q,"certificateContent":%q}}]}`,
				ascCertificate.NotAfter.Format(time.RFC3339), certificateContent))
		default:
			t.Errorf("unexpected ASC request %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := server.Client().Transport
	client, err := asc.NewClientWithHTTPClient(os.Getenv("ASC_KEY_ID"), os.Getenv("ASC_ISSUER_ID"), os.Getenv("ASC_PRIVATE_KEY_PATH"), &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return transport.RoundTrip(cloned)
	})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run([]string{
			"signing", "sync", "push",
			"--bundle-id", "com.example.app",
			"--profile-type", "IOS_APP_ADHOC",
			"--repo", "git@example.com:team/signing.git",
			"--identity", identityFile,
			"--create-missing",
			"--device", "DEVICE1",
		}, "test")
	})
	if exitCode != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, rootcmd.ExitError, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "no App Store Connect certificate matches the local signing identity") {
		t.Fatalf("stderr = %q", stderr)
	}
	if strings.Contains(stderr, "no active, unexpired certificates available") {
		t.Fatalf("stderr reports the wrong cause: %q", stderr)
	}
}

func TestSigningSyncIdentityPushPullPublicRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("local Git fixture setup uses POSIX-compatible file permission assertions")
	}
	setupAuth(t)
	t.Setenv("ASC_SIGNING_SYNC_PASSWORD", "")
	t.Setenv("ASC_MATCH_PASSWORD", "")
	t.Setenv("GIT_AUTHOR_NAME", "ASC Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "asc-test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "ASC Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "asc-test@example.invalid")

	privateKey, certificate, profile := signingSyncIdentityFixture(t)
	certificateContent := base64.StdEncoding.EncodeToString(certificate.Raw)
	profileContent := base64.StdEncoding.EncodeToString(profile)
	profileCreateCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds":
			writeSigningSyncJSON(t, w, `{"data":[{"type":"bundleIds","id":"bundle-main","attributes":{"identifier":"com.example.app"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			if profileCreateCount == 0 {
				writeSigningSyncJSON(t, w, `{"data":[]}`)
				return
			}
			writeSigningSyncJSON(t, w, fmt.Sprintf(`{"data":[{"type":"profiles","id":"profile-main","attributes":{"name":"Ad Hoc","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","profileContent":%q}}]}`, profileContent))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			writeSigningSyncJSON(t, w, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-main","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"SERIAL","activated":true,"expirationDate":%q,"certificateContent":%q}}]}`,
				certificate.NotAfter.Format(time.RFC3339), certificateContent))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			profileCreateCount++
			writeSigningSyncJSON(t, w, fmt.Sprintf(`{"data":{"type":"profiles","id":"profile-main","attributes":{"name":"Ad Hoc","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","profileContent":%q}}}`, profileContent))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/profiles/profile-main/certificates":
			writeSigningSyncJSON(t, w, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-main","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"SERIAL","certificateContent":%q}}]}`,
				certificateContent))
		default:
			t.Errorf("unexpected ASC request %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := server.Client().Transport
	client, err := asc.NewClientWithHTTPClient(os.Getenv("ASC_KEY_ID"), os.Getenv("ASC_ISSUER_ID"), os.Getenv("ASC_PRIVATE_KEY_PATH"), &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return transport.RoundTrip(cloned)
	})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	fixtureDir := t.TempDir()
	repository := filepath.Join(fixtureDir, "signing.git")
	runSigningSyncGit(t, "init", "--bare", "--initial-branch=main", repository)
	repositoryPassword := "CANARY-REPOSITORY-PASSWORD"
	sourcePassword := "CANARY-SOURCE-PASSWORD"
	repositoryPasswordFile := filepath.Join(fixtureDir, "repository-password")
	sourcePasswordFile := filepath.Join(fixtureDir, "source-password")
	identityFile := filepath.Join(fixtureDir, "source.p12")
	writeSigningSyncProtectedFile(t, repositoryPasswordFile, []byte(repositoryPassword+"\n"))
	writeSigningSyncProtectedFile(t, sourcePasswordFile, []byte(sourcePassword+"\n"))
	inputIdentity, err := legacyPKCS12.Encode(rand.Reader, privateKey, certificate, nil, sourcePassword)
	if err != nil {
		t.Fatal(err)
	}
	writeSigningSyncProtectedFile(t, identityFile, inputIdentity)

	runPush := func() map[string]any {
		t.Helper()
		root := RootCommand("test")
		root.FlagSet.SetOutput(io.Discard)
		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse([]string{
				"signing", "sync", "push",
				"--bundle-id", "com.example.app",
				"--profile-type", "IOS_APP_ADHOC",
				"--repo", repository,
				"--password-file", repositoryPasswordFile,
				"--identity", identityFile,
				"--identity-password-file", sourcePasswordFile,
				"--create-missing",
				"--device", "DEVICE1",
				"--output", "json",
			}); err != nil {
				t.Fatal(err)
			}
			runErr = root.Run(context.Background())
		})
		if runErr != nil {
			t.Fatalf("push failed: %v\nstderr=%s", runErr, stderr)
		}
		for _, canary := range []string{repositoryPassword, sourcePassword} {
			if strings.Contains(stdout, canary) || strings.Contains(stderr, canary) {
				t.Fatalf("secret canary leaked: stdout=%q stderr=%q", stdout, stderr)
			}
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("decode push JSON %q: %v", stdout, err)
		}
		if result["operation"] != "push" || result["identityPresent"] != true {
			t.Fatalf("push result = %#v", result)
		}
		return result
	}
	first := runPush()
	second := runPush()
	if first["identitySha256"] != second["identitySha256"] {
		t.Fatalf("idempotent identity fingerprints differ: first=%#v second=%#v", first, second)
	}
	if profileCreateCount != 1 {
		t.Fatalf("profile create requests = %d, want 1", profileCreateCount)
	}

	outDir := filepath.Join(fixtureDir, "pulled")
	root := RootCommand("test")
	root.FlagSet.SetOutput(io.Discard)
	var pullErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"signing", "sync", "pull", "--repo", repository,
			"--password-file", repositoryPasswordFile, "--output-dir", outDir, "--output", "json",
		}); err != nil {
			t.Fatal(err)
		}
		pullErr = root.Run(context.Background())
	})
	if pullErr != nil {
		t.Fatalf("pull failed: %v\nstderr=%s", pullErr, stderr)
	}
	if strings.Contains(stdout, repositoryPassword) || strings.Contains(stderr, repositoryPassword) {
		t.Fatalf("repository password leaked: stdout=%q stderr=%q", stdout, stderr)
	}
	var pullResult struct {
		IdentityPresent bool     `json:"identityPresent"`
		SensitiveFiles  []string `json:"sensitiveFiles"`
	}
	if err := json.Unmarshal([]byte(stdout), &pullResult); err != nil {
		t.Fatal(err)
	}
	if !pullResult.IdentityPresent || len(pullResult.SensitiveFiles) != 1 {
		t.Fatalf("pull result = %#v", pullResult)
	}
	pulledIdentityPath := filepath.Join(outDir, filepath.FromSlash(pullResult.SensitiveFiles[0]))
	pulledIdentity, err := os.ReadFile(pulledIdentityPath)
	if err != nil {
		t.Fatal(err)
	}
	pulledKey, pulledCertificate, err := modernPKCS12.Decode(pulledIdentity, repositoryPassword)
	if err != nil {
		t.Fatalf("decode modern pulled PKCS#12: %v", err)
	}
	pulledSigner, ok := pulledKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatal("pulled identity does not contain an EC private key")
	}
	wantPublicKey, marshalErr := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	gotPublicKey, marshalErr := x509.MarshalPKIXPublicKey(&pulledSigner.PublicKey)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if !bytes.Equal(gotPublicKey, wantPublicKey) || !pulledCertificate.Equal(certificate) {
		t.Fatal("pulled identity does not match source key/certificate")
	}
	info, err := os.Stat(pulledIdentityPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("pulled identity mode = %o, want 600", info.Mode().Perm())
	}

	clone := filepath.Join(fixtureDir, "inspect")
	runSigningSyncGit(t, "clone", "--quiet", repository, clone)
	fingerprint, _ := first["identitySha256"].(string)
	if _, err := os.Stat(filepath.Join(clone, "identities", "distribution", fingerprint+".p12.enc")); err != nil {
		t.Fatalf("encrypted identity artifact missing: %v", err)
	}
}

func signingSyncIdentityFixture(t *testing.T) (*ecdsa.PrivateKey, *x509.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(101),
		Subject:      pkix.Name{CommonName: "Apple Distribution Test", OrganizationalUnit: []string{"TEAM123"}},
		NotBefore:    now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID":           "01234567-89ab-cdef-0123-456789abcdef",
		"TeamIdentifier": []string{"TEAM123"}, "ApplicationIdentifierPrefix": []string{"SEED456"},
		"ExpirationDate": now.Add(12 * time.Hour), "DeveloperCertificates": [][]byte{certificate.Raw},
		"ProvisionedDevices": []string{"DEVICE1"},
		"Entitlements":       map[string]any{"application-identifier": "SEED456.com.example.app", "get-task-allow": false},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := pkcs7.NewSignedData(profilePlist)
	if err != nil {
		t.Fatal(err)
	}
	if err := signed.AddSigner(certificate, key, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatal(err)
	}
	profile, err := signed.Finish()
	if err != nil {
		t.Fatal(err)
	}
	return key, certificate, profile
}

func writeSigningSyncProtectedFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeSigningSyncJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, body); err != nil {
		t.Error(err)
	}
}

func runSigningSyncGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Env = cleanGitRepoEnv(os.Environ())
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
