package web

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	apiKeyFixtureP8     []byte
	apiKeyFixtureP8Once sync.Once
)

func apiKeyTestP8(t *testing.T) []byte {
	t.Helper()
	apiKeyFixtureP8Once.Do(func() {
		apiKeyFixtureP8 = generateP256PKCS8PEM(t)
	})
	return apiKeyFixtureP8
}

func TestWebAPIKeysCreateSavesP8AndPrintsMetadata(t *testing.T) {
	restore := installWebAPIKeyCreateFakes(t)
	t.Cleanup(restore)

	outputDir := t.TempDir()
	cmd := WebAPIKeysCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--name", "Release automation",
		"--role", "APP_MANAGER",
		"--output-dir", outputDir,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{
		`"keyId":"ABC123XYZ"`,
		`"issuerId":"69a6de00-aaaa-bbbb-cccc-123456789abc"`,
		`"roles":["APP_MANAGER"]`,
		`"p8Path":`,
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected stdout to contain %s, got %q", want, stdout)
		}
	}
	p8 := apiKeyTestP8(t)
	assertNoKeyMaterial(t, p8, stdout, stderr)

	path := filepath.Join(outputDir, "AuthKey_ABC123XYZ.p8")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved P8: %v", err)
	}
	if string(contents) != string(p8) {
		t.Fatalf("unexpected P8 contents length %d, want %d", len(contents), len(p8))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved P8: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600, got %04o", info.Mode().Perm())
	}
}

func TestWebAPIKeysCreateHelpDocumentsCaseInsensitiveRole(t *testing.T) {
	help := WebAPIKeysCreateCommand().LongHelp
	if strings.Contains(help, "must\nbe an uppercase identifier") || strings.Contains(help, "must be an uppercase identifier") {
		t.Fatalf("help still claims --role must be uppercase even though lowercase input is accepted:\n%s", help)
	}
	if !strings.Contains(help, "case-insensitive") {
		t.Fatalf("help should say --role is case-insensitive:\n%s", help)
	}
}

func TestWebAPIKeysCreateUpperCasesLowercaseRole(t *testing.T) {
	restore := installWebAPIKeyCreateFakes(t)
	t.Cleanup(restore)

	originalCreate := createWebAPIKeyFn
	var sentRole string
	createWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, attrs webcore.APIKeyCreateAttributes) (*webcore.APIKey, error) {
		sentRole = attrs.Role
		return originalCreate(ctx, client, attrs)
	}
	t.Cleanup(func() { createWebAPIKeyFn = originalCreate })

	cmd := WebAPIKeysCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--name", "Release automation",
		"--role", "app_manager",
		"--output-dir", t.TempDir(),
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("expected lowercase role to be accepted, got %v", err)
		}
	})
	if sentRole != "APP_MANAGER" {
		t.Fatalf("expected role sent as APP_MANAGER, got %q", sentRole)
	}
}

func TestWebAPIKeysCreateValidatesBeforeResolvingSession(t *testing.T) {
	resolveCalled := false
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalled = true
		return &webcore.AuthSession{}, "cache", nil
	})
	t.Cleanup(restoreSession)

	cmd := WebAPIKeysCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--name", "Release automation", "--role", "not-a-role"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(err.Error(), "--role must be a role identifier such as ADMIN or APP_MANAGER") {
		t.Fatalf("expected identifier-shape stderr, got %v", err)
	}
	if resolveCalled {
		t.Fatal("did not expect session resolution before role validation")
	}
}

func TestWebAPIKeysCreateRejectsEmptyRoleBeforeResolvingSession(t *testing.T) {
	resolveCalled := false
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalled = true
		return &webcore.AuthSession{}, "cache", nil
	})
	t.Cleanup(restoreSession)

	cmd := WebAPIKeysCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--name", "Release automation", "--role", "   "}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(err.Error(), "--role is required") {
		t.Fatalf("expected required-role stderr, got %v", err)
	}
	if resolveCalled {
		t.Fatal("did not expect session resolution before role validation")
	}
}

func TestWebAPIKeysCreateRejectsKnownNonSelectableRole(t *testing.T) {
	resolveCalled := false
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalled = true
		return &webcore.AuthSession{}, "cache", nil
	})
	t.Cleanup(restoreSession)

	cmd := WebAPIKeysCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--name", "Release automation", "--role", "ACCOUNT_HOLDER"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(err.Error(), "ACCOUNT_HOLDER") || !strings.Contains(err.Error(), "not a selectable team API key role") {
		t.Fatalf("expected known non-selectable role stderr, got %v", err)
	}
	if resolveCalled {
		t.Fatal("did not expect session resolution for a documented non-selectable role")
	}
}

func TestWebAPIKeysCreateWarnsAndContinuesForUnknownRole(t *testing.T) {
	restore := installWebAPIKeyCreateFakes(t)
	t.Cleanup(restore)
	createCalled := false
	createWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, attrs webcore.APIKeyCreateAttributes) (*webcore.APIKey, error) {
		createCalled = true
		if attrs.Role != "FUTURE_TEAM_ROLE" {
			t.Fatalf("unexpected role %q", attrs.Role)
		}
		return &webcore.APIKey{
			KeyID:          "ABC123XYZ",
			Name:           attrs.Nickname,
			Roles:          []string{attrs.Role},
			Active:         true,
			AllAppsVisible: true,
			CanDownload:    true,
			KeyType:        "PUBLIC_API",
		}, nil
	}
	getWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKey, error) {
		return &webcore.APIKey{
			KeyID:          keyID,
			Name:           "Release automation",
			IssuerID:       "69a6de00-aaaa-bbbb-cccc-123456789abc",
			Roles:          []string{"FUTURE_TEAM_ROLE"},
			Active:         true,
			AllAppsVisible: true,
			KeyType:        "PUBLIC_API",
		}, nil
	}

	outputDir := t.TempDir()
	cmd := WebAPIKeysCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--name", "Release automation",
		"--role", "FUTURE_TEAM_ROLE",
		"--output-dir", outputDir,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := captureOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr != nil {
		t.Fatalf("expected unknown session-accepted role to succeed, got %v", execErr)
	}
	if !createCalled {
		t.Fatal("expected create to run for an unknown-but-well-formed role")
	}
	if !strings.Contains(stderr, "Warning:") || !strings.Contains(stderr, "FUTURE_TEAM_ROLE") {
		t.Fatalf("expected unknown-role warning on stderr, got %q", stderr)
	}
	p8 := apiKeyTestP8(t)
	assertNoKeyMaterial(t, p8, stdout, stderr, execErrString(execErr))
	if _, err := os.Stat(filepath.Join(outputDir, "AuthKey_ABC123XYZ.p8")); err != nil {
		t.Fatalf("expected P8 to be written: %v", err)
	}
}

func TestDownloadWebAPIKeyWithRetryRetriesPropagationErrors(t *testing.T) {
	originalDownload := downloadWebAPIKeyFn
	originalWait := waitWebAPIKeyRetryFn
	t.Cleanup(func() {
		downloadWebAPIKeyFn = originalDownload
		waitWebAPIKeyRetryFn = originalWait
	})

	attempts := 0
	downloadWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) ([]byte, error) {
		attempts++
		if attempts < 3 {
			return nil, &webcore.APIError{Status: 404}
		}
		return apiKeyTestP8(t), nil
	}
	var waits []time.Duration
	waitWebAPIKeyRetryFn = func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}

	got, err := downloadWebAPIKeyWithRetry(context.Background(), &webcore.Client{}, "ABC123XYZ")
	if err != nil {
		t.Fatalf("downloadWebAPIKeyWithRetry() error: %v", err)
	}
	if string(got) != string(apiKeyTestP8(t)) {
		t.Fatalf("unexpected P8 contents length %d, want %d", len(got), len(apiKeyTestP8(t)))
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if len(waits) != 2 || waits[0] != time.Second || waits[1] != 2*time.Second {
		t.Fatalf("unexpected waits: %#v", waits)
	}
}

func TestDownloadWebAPIKeyWithRetryDoesNotRetryTransportFailure(t *testing.T) {
	originalDownload := downloadWebAPIKeyFn
	originalWait := waitWebAPIKeyRetryFn
	t.Cleanup(func() {
		downloadWebAPIKeyFn = originalDownload
		waitWebAPIKeyRetryFn = originalWait
	})

	attempts := 0
	downloadWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) ([]byte, error) {
		attempts++
		return nil, fmt.Errorf("read body: connection reset")
	}
	waits := 0
	waitWebAPIKeyRetryFn = func(ctx context.Context, delay time.Duration) error {
		waits++
		return nil
	}

	_, err := downloadWebAPIKeyWithRetry(context.Background(), &webcore.Client{}, "ABC123XYZ")
	if err == nil || !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("expected transport error, got %v", err)
	}
	if attempts != 1 || waits != 0 {
		t.Fatalf("expected one attempt and no waits, got %d attempts and %d waits", attempts, waits)
	}
}

func TestDownloadWebAPIKeyWithRetryDoesNotRetryInvalidResponse(t *testing.T) {
	originalDownload := downloadWebAPIKeyFn
	originalWait := waitWebAPIKeyRetryFn
	t.Cleanup(func() {
		downloadWebAPIKeyFn = originalDownload
		waitWebAPIKeyRetryFn = originalWait
	})

	attempts := 0
	downloadWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) ([]byte, error) {
		attempts++
		return nil, fmt.Errorf("decode failed: %w", webcore.ErrAPIKeyResponseInvalid)
	}
	waits := 0
	waitWebAPIKeyRetryFn = func(ctx context.Context, delay time.Duration) error {
		waits++
		return nil
	}

	_, err := downloadWebAPIKeyWithRetry(context.Background(), &webcore.Client{}, "ABC123XYZ")
	if !errors.Is(err, webcore.ErrAPIKeyResponseInvalid) {
		t.Fatalf("expected invalid response error, got %v", err)
	}
	if attempts != 1 || waits != 0 {
		t.Fatalf("expected one attempt and no waits, got %d attempts and %d waits", attempts, waits)
	}
}

func TestWebAPIKeysCreateDoesNotReplaceExistingP8(t *testing.T) {
	restore := installWebAPIKeyCreateFakes(t)
	t.Cleanup(restore)
	downloadCalls := 0
	downloadWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) ([]byte, error) {
		downloadCalls++
		return apiKeyTestP8(t), nil
	}

	outputDir := t.TempDir()
	path := filepath.Join(outputDir, "AuthKey_ABC123XYZ.p8")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	cmd := WebAPIKeysCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--name", "Release automation", "--output-dir", outputDir, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := captureOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr == nil {
		t.Fatal("expected existing destination error")
	}
	assertNoKeyMaterial(t, apiKeyTestP8(t), stdout, stderr, execErr.Error())
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing file: %v", err)
	}
	if string(contents) != "existing" {
		t.Fatalf("existing file was replaced: %q", string(contents))
	}
	if downloadCalls != 0 {
		t.Fatalf("expected destination collision before one-time download, got %d download calls", downloadCalls)
	}
}

func TestWebAPIKeysCreateRemovesReservationWhenDownloadFails(t *testing.T) {
	restore := installWebAPIKeyCreateFakes(t)
	t.Cleanup(restore)

	downloadErr := &webcore.APIError{Status: 403}
	downloadWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) ([]byte, error) {
		return nil, downloadErr
	}

	outputDir := t.TempDir()
	cmd := WebAPIKeysCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--name", "Release automation", "--output-dir", outputDir}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, downloadErr) {
		t.Fatalf("expected download error, got %v", err)
	}
	path := filepath.Join(outputDir, "AuthKey_ABC123XYZ.p8")
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected failed-download reservation to be removed, stat error = %v", statErr)
	}
}

func TestWebAPIKeysCreatePreservesP8WhenIssuerLookupFails(t *testing.T) {
	restore := installWebAPIKeyCreateFakes(t)
	t.Cleanup(restore)

	lookupErr := errors.New("lookup failed")
	getWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKey, error) {
		return nil, lookupErr
	}
	outputDir := t.TempDir()
	cmd := WebAPIKeysCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--name", "Release automation", "--output-dir", outputDir}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected issuer lookup error, got %v", err)
	}
	path := filepath.Join(outputDir, "AuthKey_ABC123XYZ.p8")
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("expected recovery path in error, got %v", err)
	}
	contents, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("expected saved P8 after lookup failure: %v", readErr)
	}
	if string(contents) != string(apiKeyTestP8(t)) {
		t.Fatalf("unexpected saved P8 length %d, want %d", len(contents), len(apiKeyTestP8(t)))
	}
	assertNoKeyMaterial(t, apiKeyTestP8(t), err.Error())
}

func TestWebAPIKeysCreateRejectsInvalidP8WithoutWritingFile(t *testing.T) {
	valid := generateP256PKCS8PEM(t)
	tests := []struct {
		name string
		id   string
		p8   []byte
	}{
		{name: "truncated", id: "ABC123XYZ", p8: truncatedPKCS8PEM(t, valid)},
		{name: "non-pkcs8", id: "ABC123XYZ", p8: []byte("-----BEGIN PRIVATE KEY-----\nfixture-secret\n-----END PRIVATE KEY-----\n")},
		{name: "wrong-key-type", id: "ABC123XYZ", p8: generateRSAPKCS8PEM(t)},
		{name: "multi-block", id: "ABC123XYZ", p8: append(append([]byte{}, valid...), valid...)},
		{name: "leading-data", id: "ABC123XYZ", p8: append(append([]byte{}, []byte("leading-junk\n")...), valid...)},
		{name: "mismatched-id", id: "OTHERKEY", p8: valid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := installWebAPIKeyCreateFakes(t)
			t.Cleanup(restore)

			downloadWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) ([]byte, error) {
				return client.DownloadAPIKey(ctx, keyID)
			}
			newWebAPIKeyClientFn = func(session *webcore.AuthSession) *webcore.Client {
				return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(cliAPIKeyDownloadJSON(tt.id, tt.p8))
				}))
			}

			outputDir := t.TempDir()
			cmd := WebAPIKeysCreateCommand()
			if err := cmd.FlagSet.Parse([]string{"--name", "Release automation", "--output-dir", outputDir}); err != nil {
				t.Fatalf("parse error: %v", err)
			}

			var execErr error
			stdout, stderr := captureOutput(t, func() {
				execErr = cmd.Exec(context.Background(), nil)
			})
			if execErr == nil {
				t.Fatal("expected invalid P8 to fail")
			}
			if !errors.Is(execErr, webcore.ErrAPIKeyResponseInvalid) {
				t.Fatalf("expected invalid P8 error, got %v", execErr)
			}
			if !strings.Contains(execErr.Error(), "P8") && !strings.Contains(execErr.Error(), "key") {
				t.Fatalf("expected clear stderr about the P8, got %v", execErr)
			}
			assertNoKeyMaterial(t, valid, stdout, stderr, execErr.Error())
			assertNoKeyMaterial(t, tt.p8, stdout, stderr, execErr.Error())
			path := filepath.Join(outputDir, "AuthKey_ABC123XYZ.p8")
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("expected no P8 file after invalid download, stat error = %v", statErr)
			}
		})
	}
}

func installWebAPIKeyCreateFakes(t *testing.T) func() {
	t.Helper()
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{TeamID: "TEAM123"}, "cache", nil
	})
	originalNewClient := newWebAPIKeyClientFn
	originalCreate := createWebAPIKeyFn
	originalDownload := downloadWebAPIKeyFn
	originalGet := getWebAPIKeyFn
	originalWait := waitWebAPIKeyRetryFn

	newWebAPIKeyClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	createWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, attrs webcore.APIKeyCreateAttributes) (*webcore.APIKey, error) {
		if attrs.Nickname != "Release automation" {
			t.Fatalf("unexpected nickname %q", attrs.Nickname)
		}
		return &webcore.APIKey{
			KeyID:          "ABC123XYZ",
			Name:           attrs.Nickname,
			Roles:          []string{attrs.Role},
			Active:         true,
			AllAppsVisible: true,
			CanDownload:    true,
			KeyType:        "PUBLIC_API",
		}, nil
	}
	downloadWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) ([]byte, error) {
		return apiKeyTestP8(t), nil
	}
	getWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKey, error) {
		return &webcore.APIKey{
			KeyID:          keyID,
			Name:           "Release automation",
			IssuerID:       "69a6de00-aaaa-bbbb-cccc-123456789abc",
			Roles:          []string{"APP_MANAGER"},
			Active:         true,
			AllAppsVisible: true,
			KeyType:        "PUBLIC_API",
		}, nil
	}
	waitWebAPIKeyRetryFn = func(ctx context.Context, delay time.Duration) error { return nil }

	return func() {
		restoreSession()
		newWebAPIKeyClientFn = originalNewClient
		createWebAPIKeyFn = originalCreate
		downloadWebAPIKeyFn = originalDownload
		getWebAPIKeyFn = originalGet
		waitWebAPIKeyRetryFn = originalWait
	}
}

func generateP256PKCS8PEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-256 key: %v", err)
	}
	return marshalPKCS8PEM(t, key)
}

func generateRSAPKCS8PEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return marshalPKCS8PEM(t, key)
}

func marshalPKCS8PEM(t *testing.T, key any) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal PKCS#8: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func truncatedPKCS8PEM(t *testing.T, valid []byte) []byte {
	t.Helper()
	block, _ := pem.Decode(valid)
	if block == nil || len(block.Bytes) < 8 {
		t.Fatal("expected a decodable PKCS#8 PEM fixture")
	}
	truncated := append([]byte{}, block.Bytes[:len(block.Bytes)/2]...)
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: truncated})
}

func cliAPIKeyDownloadJSON(id string, p8 []byte) []byte {
	encoded := base64.StdEncoding.EncodeToString(p8)
	return []byte(`{"data":{"type":"apiKeys","id":"` + id + `","attributes":{"privateKey":"` + encoded + `"}}}`)
}

type cliAPIKeyRewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t cliAPIKeyRewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	requestURL := *request.URL
	requestURL.Scheme = t.target.Scheme
	requestURL.Host = t.target.Host
	clone.URL = &requestURL
	return t.base.RoundTrip(clone)
}

func newCLIAPIKeyHTTPTestClient(t *testing.T, handler http.Handler) *webcore.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	return webcore.NewClient(&webcore.AuthSession{
		Client: &http.Client{Transport: cliAPIKeyRewriteTransport{
			target: target,
			base:   server.Client().Transport,
		}},
	})
}

func execErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func assertNoKeyMaterial(t *testing.T, p8 []byte, outputs ...string) {
	t.Helper()
	if len(p8) == 0 {
		return
	}
	full := strings.TrimSpace(string(p8))
	block, _ := pem.Decode(p8)
	for _, out := range outputs {
		if out == "" {
			continue
		}
		if full != "" && strings.Contains(out, full) {
			t.Fatal("output contained P8 contents")
		}
		if strings.Contains(out, "-----BEGIN PRIVATE KEY-----") || strings.Contains(out, "-----END PRIVATE KEY-----") {
			t.Fatal("output contained PEM boundary")
		}
		if block != nil && len(block.Bytes) > 0 {
			if strings.Contains(out, base64.StdEncoding.EncodeToString(block.Bytes)) {
				t.Fatal("output contained PKCS#8 DER")
			}
		}
	}
}
