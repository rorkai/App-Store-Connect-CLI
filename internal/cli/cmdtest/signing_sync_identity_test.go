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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/bundleIds":
			writeSigningSyncJSON(t, w, `{"data":[{"type":"bundleIds","id":"bundle-main","attributes":{"identifier":"com.example.app"}}]}`)
		case "/v1/bundleIds/bundle-main/profiles":
			writeSigningSyncJSON(t, w, fmt.Sprintf(`{"data":[{"type":"profiles","id":"profile-main","attributes":{"name":"Ad Hoc","uuid":"profile-main","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","profileContent":%q}}]}`, profileContent))
		case "/v1/profiles/profile-main/certificates":
			writeSigningSyncJSON(t, w, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-main","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"SERIAL","expirationDate":%q,"certificateContent":%q}}]}`,
				certificate.NotAfter.Format(time.RFC3339), certificateContent))
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
		"UUID":           "profile-main",
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
