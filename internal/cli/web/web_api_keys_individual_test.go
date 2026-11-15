package web

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAPIKeysRegistersIndividualCreateCommand(t *testing.T) {
	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCommand(d)
	for _, subcommand := range command.Subcommands {
		if subcommand.Name == "create-individual" {
			if subcommand.UsageFunc == nil {
				t.Fatal("create-individual command must define UsageFunc")
			}
			return
		}
	}
	t.Fatal("web api-keys is missing create-individual")
}

func TestWebAPIKeysCreateIndividualRequiresConfirmBeforeSession(t *testing.T) {
	resolveCalled := false
	restore := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalled = true
		return &webcore.AuthSession{}, "cache", nil
	})
	t.Cleanup(restore)

	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{
		"--user-id", "USER-UUID",
		"--output-dir", t.TempDir(),
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = command.Exec(context.Background(), nil)
	})
	if !errors.Is(execErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", execErr)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected confirmation error, got %q", stderr)
	}
	if resolveCalled {
		t.Fatal("did not expect session resolution before confirmation validation")
	}
}

func TestWebAPIKeysCreateIndividualUnsupportedPublicationDoesNotCreate(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	outputDir := t.TempDir()
	publicationErr := errors.New("atomic no-replace rename unsupported")
	publicationCalls := 0
	postCount := 0
	patchCount := 0
	resolveCalled := false
	restorePublication := setCheckIndividualAPIKeyOutputPublicationForTest(func(rootfs.Root, string) error {
		publicationCalls++
		return publicationErr
	})
	t.Cleanup(restorePublication)
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalled = true
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"owner@example.com"}}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				_, _ = w.Write([]byte(`{"data":[]}`))
			case r.Method == http.MethodPost && r.URL.Path == "/iris/v2/apiKeys":
				postCount++
				http.Error(w, "unexpected create", http.StatusInternalServerError)
			case r.Method == http.MethodPatch && r.URL.Path == "/iris/v2/apiKeys/IND-1":
				patchCount++
				http.Error(w, "unexpected patch", http.StatusInternalServerError)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{
		"--user-id", "USER-UUID",
		"--output-dir", outputDir,
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if !errors.Is(err, publicationErr) {
		t.Fatalf("expected publication preflight error, got %v", err)
	}
	if publicationCalls != 1 {
		t.Fatalf("publication probe calls = %d, want 1", publicationCalls)
	}
	if resolveCalled {
		t.Fatal("did not expect session resolution after publication preflight failure")
	}
	if postCount != 0 || patchCount != 0 {
		t.Fatalf("remote mutations = POST %d, PATCH %d; want zero", postCount, patchCount)
	}
}

func TestWebAPIKeysCreateIndividualGeneratesAndRegistersP8(t *testing.T) {
	testWebAPIKeysCreateIndividualGeneratesAndRegistersP8(t, t.TempDir())
}

func testWebAPIKeysCreateIndividualGeneratesAndRegistersP8(t *testing.T, outputDir string) {
	t.Helper()
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	requestCount := 0
	postCreated := false
	var registeredPublicKey string

	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"owner@example.com"}}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				if !postCreated {
					_, _ = w.Write([]byte(`{"data":[]}`))
				} else {
					publicKey := any(nil)
					if registeredPublicKey != "" {
						publicKey = registeredPublicKey
					}
					response := map[string]any{"data": []any{map[string]any{
						"type": "apiKeys", "id": "IND-1",
						"attributes": map[string]any{"isActive": true, "publicKey": publicKey},
					}}}
					_ = json.NewEncoder(w).Encode(response)
				}
			case r.Method == http.MethodPost && r.URL.Path == "/iris/v2/apiKeys":
				postCreated = true
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPatch && r.URL.Path == "/iris/v2/apiKeys/IND-1":
				var request struct {
					Data struct {
						Attributes struct {
							PublicKey string `json:"publicKey"`
						} `json:"attributes"`
					} `json:"data"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatalf("decode patch: %v", err)
				}
				registeredPublicKey = request.Data.Attributes.PublicKey
				w.WriteHeader(http.StatusNoContent)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{
		"--user-id", "USER-UUID",
		"--output-dir", outputDir,
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = command.Exec(context.Background(), nil)
	})
	if execErr != nil {
		t.Fatalf("expected success, got %v (stderr %q)", execErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"keyId":"IND-1"`) || !strings.Contains(stdout, `"registered":true`) {
		t.Fatalf("unexpected receipt: %q", stdout)
	}
	if strings.Contains(stdout, "BEGIN PRIVATE KEY") || strings.Contains(stdout, "BEGIN PUBLIC KEY") {
		t.Fatalf("receipt leaked key material: %q", stdout)
	}

	path := filepath.Join(outputDir, "ApiKey_IND-1.p8")
	privatePEM, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat private key: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %04o, want 0600", info.Mode().Perm())
	}
	if len(privatePEM) == 0 || registeredPublicKey == "" {
		t.Fatal("expected private artifact and registered public key")
	}
	if requestCount != 6 {
		t.Fatalf("request count = %d, want identity/preflight/create/list/patch/postread", requestCount)
	}
}

func TestWebAPIKeysCreateIndividualRejectsAmbiguousPostList(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	outputDir := t.TempDir()
	postCreated := false
	patchCalled := false
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"owner@example.com"}}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				if !postCreated {
					_, _ = w.Write([]byte(`{"data":[]}`))
					return
				}
				response := map[string]any{"data": []any{
					map[string]any{"type": "apiKeys", "id": "IND-1", "attributes": map[string]any{"isActive": true, "publicKey": nil}},
					map[string]any{"type": "apiKeys", "id": "IND-2", "attributes": map[string]any{"isActive": true, "publicKey": nil}},
				}}
				_ = json.NewEncoder(w).Encode(response)
			case r.Method == http.MethodPost && r.URL.Path == "/iris/v2/apiKeys":
				postCreated = true
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPatch:
				patchCalled = true
				http.Error(w, "unexpected patch", http.StatusInternalServerError)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{"--user-id", "USER-UUID", "--output-dir", outputDir, "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous post-create list error, got %v", err)
	}
	if patchCalled {
		t.Fatal("did not expect PATCH when more than one new active key was returned")
	}
	if !hasIndividualAPIKeyStagingArtifact(t, outputDir) {
		t.Fatal("expected staged private artifact to remain after ambiguous resolution")
	}
}

func TestWebAPIKeysCreateIndividualRejectsAlreadyRegisteredNewKey(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	outputDir := t.TempDir()
	postCreated := false
	patchCalled := false
	registeredPublicKey := generateTestPublicKeyPEM(t)
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"owner@example.com"}}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				if !postCreated {
					_, _ = w.Write([]byte(`{"data":[]}`))
					return
				}
				response := map[string]any{"data": []any{map[string]any{
					"type": "apiKeys", "id": "IND-1",
					"attributes": map[string]any{"isActive": true, "publicKey": registeredPublicKey},
				}}}
				_ = json.NewEncoder(w).Encode(response)
			case r.Method == http.MethodPost && r.URL.Path == "/iris/v2/apiKeys":
				postCreated = true
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPatch:
				patchCalled = true
				http.Error(w, "unexpected patch", http.StatusInternalServerError)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{"--user-id", "USER-UUID", "--output-dir", outputDir, "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "already has a registered public key") {
		t.Fatalf("expected concurrent registration refusal, got %v", err)
	}
	if patchCalled {
		t.Fatal("did not expect PATCH when the resolved new key already has a public key")
	}
	if !hasIndividualAPIKeyStagingArtifact(t, outputDir) {
		t.Fatal("expected staged private artifact to remain after concurrent registration")
	}
}

func TestWebAPIKeysCreateIndividualRejectsDifferentPostReadPublicKey(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	outputDir := t.TempDir()
	wrongPublicKey := generateTestPublicKeyPEM(t)
	postCreated := false
	registeredPublicKey := ""
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"owner@example.com"}}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				if !postCreated {
					_, _ = w.Write([]byte(`{"data":[]}`))
				} else {
					publicKey := any(nil)
					if registeredPublicKey != "" {
						publicKey = registeredPublicKey
					}
					response := map[string]any{"data": []any{map[string]any{
						"type": "apiKeys", "id": "IND-1",
						"attributes": map[string]any{"isActive": true, "publicKey": publicKey},
					}}}
					_ = json.NewEncoder(w).Encode(response)
				}
			case r.Method == http.MethodPost && r.URL.Path == "/iris/v2/apiKeys":
				postCreated = true
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPatch && r.URL.Path == "/iris/v2/apiKeys/IND-1":
				registeredPublicKey = wrongPublicKey
				w.WriteHeader(http.StatusNoContent)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{"--user-id", "USER-UUID", "--output-dir", outputDir, "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "generated public key") {
		t.Fatalf("expected generated-public-key verification failure, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "ApiKey_IND-1.p8")); statErr != nil {
		t.Fatalf("expected retained private artifact: %v", statErr)
	}
	if strings.Contains(err.Error(), "BEGIN PUBLIC KEY") {
		t.Fatalf("error leaked public key material: %v", err)
	}
}

func TestWebAPIKeysCreateIndividualAcceptsEquivalentPostReadPublicKeyPEM(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	outputDir := t.TempDir()
	postCreated := false
	registeredPublicKey := ""
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"owner@example.com"}}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				if !postCreated {
					_, _ = w.Write([]byte(`{"data":[]}`))
				} else {
					publicKey := any(nil)
					if registeredPublicKey != "" {
						publicKey = registeredPublicKey
					}
					response := map[string]any{"data": []any{map[string]any{
						"type": "apiKeys", "id": "IND-1",
						"attributes": map[string]any{"isActive": true, "publicKey": publicKey},
					}}}
					_ = json.NewEncoder(w).Encode(response)
				}
			case r.Method == http.MethodPost && r.URL.Path == "/iris/v2/apiKeys":
				postCreated = true
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPatch && r.URL.Path == "/iris/v2/apiKeys/IND-1":
				var request struct {
					Data struct {
						Attributes struct {
							PublicKey string `json:"publicKey"`
						} `json:"attributes"`
					} `json:"data"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatalf("decode patch: %v", err)
				}
				registeredPublicKey = strings.ReplaceAll(request.Data.Attributes.PublicKey, "\n", "\r\n")
				w.WriteHeader(http.StatusNoContent)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{"--user-id", "USER-UUID", "--output-dir", outputDir, "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := command.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected equivalent public key PEM to verify, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "ApiKey_IND-1.p8")); statErr != nil {
		t.Fatalf("expected private artifact: %v", statErr)
	}
}

func TestWebAPIKeysCreateIndividualStagesPrivateKeyBeforeRemoteCreate(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	outputDir := t.TempDir()
	stageSeenAtPost := false
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"owner@example.com"}}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				_, _ = w.Write([]byte(`{"data":[]}`))
			case r.Method == http.MethodPost && r.URL.Path == "/iris/v2/apiKeys":
				entries, readErr := os.ReadDir(outputDir)
				if readErr != nil {
					t.Fatalf("read output directory at POST: %v", readErr)
				}
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), ".asc-api-key-") && strings.HasSuffix(entry.Name(), ".p8") {
						stageSeenAtPost = true
					}
				}
				http.Error(w, "ambiguous create response", http.StatusBadGateway)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{"--user-id", "USER-UUID", "--output-dir", outputDir, "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected ambiguous create failure")
	}
	if !stageSeenAtPost {
		t.Fatal("expected private key staging before remote POST")
	}
	if !strings.Contains(err.Error(), ".asc-api-key-") {
		t.Fatalf("expected retained staging diagnostic, got %v", err)
	}
}

func TestWebAPIKeysCreateIndividualFinalRenamePreservesExistingCanonicalFile(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	outputDir := t.TempDir()
	const canonicalContent = "pre-existing canonical"
	postCreated := false
	var patchCalled bool
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"owner@example.com"}}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				if !postCreated {
					_, _ = w.Write([]byte(`{"data":[]}`))
					return
				}
				_, _ = w.Write([]byte(`{"data":[{"type":"apiKeys","id":"IND-1","attributes":{"isActive":true,"publicKey":null}}]}`))
			case r.Method == http.MethodPost && r.URL.Path == "/iris/v2/apiKeys":
				if err := os.WriteFile(filepath.Join(outputDir, "ApiKey_IND-1.p8"), []byte(canonicalContent), 0o600); err != nil {
					t.Fatalf("create canonical replacement: %v", err)
				}
				postCreated = true
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPatch:
				patchCalled = true
				http.Error(w, "unexpected patch", http.StatusInternalServerError)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{"--user-id", "USER-UUID", "--output-dir", outputDir, "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || patchCalled {
		t.Fatalf("expected final rename failure before patch, got %v (patchCalled=%v)", err, patchCalled)
	}
	for _, want := range []string{
		`individual API key "IND-1" was created`,
		"public key has not been registered",
		"inspect or revoke key",
		filepath.Join(outputDir, "ApiKey_IND-1.p8"),
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("rename failure missing %q: %v", want, err)
		}
	}
	if got, readErr := os.ReadFile(filepath.Join(outputDir, "ApiKey_IND-1.p8")); readErr != nil || string(got) != canonicalContent {
		t.Fatalf("canonical file changed: contents=%q err=%v", got, readErr)
	}
	entries, readErr := os.ReadDir(outputDir)
	if readErr != nil {
		t.Fatalf("read output directory: %v", readErr)
	}
	stageRetained := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".asc-api-key-") && strings.HasSuffix(entry.Name(), ".p8") {
			stageRetained = true
		}
	}
	if !stageRetained {
		t.Fatal("expected staged private artifact to remain after final rename failure")
	}
}

func TestWebAPIKeysCreateIndividualVerifiedOutputFailurePreservesReceipt(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	outputDir := t.TempDir()
	postCount := 0
	patchCount := 0
	requestCount := 0
	postCreated := false
	registeredPublicKey := ""
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"owner@example.com"}}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				if !postCreated {
					_, _ = w.Write([]byte(`{"data":[]}`))
					return
				}
				publicKey := any(nil)
				if registeredPublicKey != "" {
					publicKey = registeredPublicKey
				}
				response := map[string]any{"data": []any{map[string]any{
					"type": "apiKeys", "id": "IND-1",
					"attributes": map[string]any{"isActive": true, "publicKey": publicKey},
				}}}
				_ = json.NewEncoder(w).Encode(response)
			case r.Method == http.MethodPost && r.URL.Path == "/iris/v2/apiKeys":
				postCount++
				postCreated = true
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPatch && r.URL.Path == "/iris/v2/apiKeys/IND-1":
				patchCount++
				var request struct {
					Data struct {
						Attributes struct {
							PublicKey string `json:"publicKey"`
						} `json:"attributes"`
					} `json:"data"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatalf("decode patch: %v", err)
				}
				registeredPublicKey = request.Data.Attributes.PublicKey
				w.WriteHeader(http.StatusNoContent)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{
		"--user-id", "USER-UUID",
		"--output-dir", outputDir,
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	closedStdout, err := os.CreateTemp(t.TempDir(), "closed-stdout-*")
	if err != nil {
		t.Fatalf("create closed stdout fixture: %v", err)
	}
	if err := closedStdout.Close(); err != nil {
		t.Fatalf("close stdout fixture: %v", err)
	}
	d.IO.Out = closedStdout
	execErr := command.Exec(context.Background(), nil)
	if execErr == nil {
		t.Fatal("expected output failure after verified registration")
	}
	for _, want := range []string{
		`individual API key "IND-1" was created`,
		"public key was registered",
		filepath.Join(outputDir, "ApiKey_IND-1.p8"),
		"do not retry automatically",
	} {
		if !strings.Contains(execErr.Error(), want) {
			t.Fatalf("output failure missing %q: %v", want, execErr)
		}
	}
	if postCount != 1 || patchCount != 1 || requestCount != 6 {
		t.Fatalf("request counts = post %d, patch %d, total %d; want 1, 1, 6", postCount, patchCount, requestCount)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "ApiKey_IND-1.p8")); statErr != nil {
		t.Fatalf("expected saved private artifact: %v", statErr)
	}
}

func TestWebAPIKeysCreateIndividualIdentityMismatchDoesNotCreate(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	createCalled := false
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"someone-else@example.com"}}}`))
			case r.Method == http.MethodPost:
				createCalled = true
				http.Error(w, "unexpected create", http.StatusInternalServerError)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{"--user-id", "USER-UUID", "--output-dir", t.TempDir(), "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "does not match authenticated web session") {
		t.Fatalf("expected identity mismatch, got %v", err)
	}
	if createCalled {
		t.Fatal("did not expect POST after identity mismatch")
	}
}

func TestWebAPIKeysCreateIndividualActiveKeyDoesNotCreate(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	createCalled := false
	activePublicKey := generateTestPublicKeyPEM(t)
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"owner@example.com"}}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				response := map[string]any{"data": []any{map[string]any{
					"type": "apiKeys", "id": "IND-OLD",
					"attributes": map[string]any{"isActive": true, "publicKey": activePublicKey},
				}}}
				_ = json.NewEncoder(w).Encode(response)
			case r.Method == http.MethodPost:
				createCalled = true
				http.Error(w, "unexpected create", http.StatusInternalServerError)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{"--user-id", "USER-UUID", "--output-dir", t.TempDir(), "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "active individual API key") {
		t.Fatalf("expected active-key preflight error, got %v", err)
	}
	if createCalled {
		t.Fatal("did not expect POST when an active key already exists")
	}
}

func TestWebAPIKeysCreateIndividualRetainsArtifactWhenPatchFails(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	outputDir := t.TempDir()
	postCreated := false
	patchCalled := false
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{UserEmail: "owner@example.com"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	restoreClient := setNewWebAPIKeyClientForTest(func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/users/USER-UUID":
				_, _ = w.Write([]byte(`{"data":{"type":"users","id":"USER-UUID","attributes":{"username":"owner@example.com"}}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				if !postCreated {
					_, _ = w.Write([]byte(`{"data":[]}`))
					return
				}
				_, _ = w.Write([]byte(`{"data":[{"type":"apiKeys","id":"IND-1","attributes":{"isActive":true,"publicKey":null}}]}`))
			case r.Method == http.MethodPost && r.URL.Path == "/iris/v2/apiKeys":
				postCreated = true
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPatch && r.URL.Path == "/iris/v2/apiKeys/IND-1":
				patchCalled = true
				http.Error(w, "temporary upstream failure", http.StatusBadGateway)
			default:
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}
		}))
	})
	t.Cleanup(restoreClient)

	d, _, _ := newWebTestDeps()
	command := newWebAPIKeysCreateIndividualCommand(d)
	if err := command.FlagSet.Parse([]string{"--user-id", "USER-UUID", "--output-dir", outputDir, "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !patchCalled {
		t.Fatalf("expected patch failure, got %v (patchCalled=%v)", err, patchCalled)
	}
	if !strings.Contains(err.Error(), filepath.Join(outputDir, "ApiKey_IND-1.p8")) {
		t.Fatalf("error should identify retained artifact path, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "ApiKey_IND-1.p8")); statErr != nil {
		t.Fatalf("expected retained private artifact: %v", statErr)
	}
	if strings.Contains(err.Error(), "BEGIN PRIVATE KEY") || strings.Contains(err.Error(), "BEGIN PUBLIC KEY") {
		t.Fatalf("error leaked key material: %v", err)
	}
}

func setNewWebAPIKeyClientForTest(fn func(*webcore.AuthSession) *webcore.Client) func() {
	previous := newWebAPIKeyClientFn
	newWebAPIKeyClientFn = fn
	return func() { newWebAPIKeyClientFn = previous }
}

func setCheckIndividualAPIKeyOutputPublicationForTest(fn func(rootfs.Root, string) error) func() {
	previous := checkIndividualAPIKeyOutputPublicationFn
	checkIndividualAPIKeyOutputPublicationFn = fn
	return func() { checkIndividualAPIKeyOutputPublicationFn = previous }
}

func generateTestPublicKeyPEM(t *testing.T) string {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal test public key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func hasIndividualAPIKeyStagingArtifact(t *testing.T, outputDir string) bool {
	t.Helper()
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".asc-api-key-") && strings.HasSuffix(entry.Name(), ".p8") {
			return true
		}
	}
	return false
}
