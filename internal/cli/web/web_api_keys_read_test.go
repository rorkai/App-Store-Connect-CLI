package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAPIKeysListPrintsMetadataWithoutKeyMaterial(t *testing.T) {
	restore := installWebAPIKeyReadFakes(t)
	t.Cleanup(restore)

	cmd := WebAPIKeysListCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := captureOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr != nil {
		t.Fatalf("expected success, got %v", execErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.WebAPIKeysListResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode JSON: %v\nstdout: %s", err, stdout)
	}
	if len(result.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %#v", result.Keys)
	}
	if result.Keys[0].KeyID != "ABC123XYZ" || result.Keys[0].Name != "Release automation" || result.Keys[0].Kind != "team" {
		t.Fatalf("unexpected first key: %#v", result.Keys[0])
	}
	if len(result.Keys[0].Roles) != 1 || result.Keys[0].Roles[0] != "ADMIN" || !result.Keys[0].Active {
		t.Fatalf("unexpected first key roles/state: %#v", result.Keys[0])
	}
	if result.Keys[1].KeyID != "IND456ABC" || result.Keys[1].Kind != "individual" || result.Keys[1].Active {
		t.Fatalf("unexpected second key: %#v", result.Keys[1])
	}
	assertNoKeyMaterial(t, apiKeyTestP8(t), stdout, stderr)
}

func TestWebAPIKeysListRejectsInvalidOutputBeforeSession(t *testing.T) {
	resolveCalled := false
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalled = true
		return &webcore.AuthSession{}, "cache", nil
	})
	t.Cleanup(restoreSession)

	cmd := WebAPIKeysListCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "xml"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	_, stderr := captureOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(execErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", execErr)
	}
	if !strings.Contains(stderr, `--output must be one of`) && !strings.Contains(execErr.Error(), `--output must be one of`) {
		t.Fatalf("expected invalid-output stderr, got %q / %v", stderr, execErr)
	}
	if resolveCalled {
		t.Fatal("did not expect session resolution for invalid --output")
	}
}

func TestWebAPIKeysViewRequiresKeyIDBeforeSession(t *testing.T) {
	resolveCalled := false
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalled = true
		return &webcore.AuthSession{}, "cache", nil
	})
	t.Cleanup(restoreSession)

	cmd := WebAPIKeysViewCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	_, stderr := captureOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(execErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", execErr)
	}
	if !strings.Contains(stderr, "Error: --key-id is required") {
		t.Fatalf("expected missing --key-id stderr, got %q", stderr)
	}
	if resolveCalled {
		t.Fatal("did not expect session resolution before --key-id validation")
	}
}

func TestWebAPIKeysViewRejectsBlankKeyIDBeforeSession(t *testing.T) {
	resolveCalled := false
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalled = true
		return &webcore.AuthSession{}, "cache", nil
	})
	t.Cleanup(restoreSession)

	cmd := WebAPIKeysViewCommand()
	if err := cmd.FlagSet.Parse([]string{"--key-id", "   "}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	_, stderr := captureOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(execErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", execErr)
	}
	if !strings.Contains(stderr, "Error: --key-id is required") {
		t.Fatalf("expected blank --key-id stderr, got %q", stderr)
	}
	if resolveCalled {
		t.Fatal("did not expect session resolution for a blank --key-id")
	}
}

func TestWebAPIKeysViewPrintsMetadataWithoutKeyMaterial(t *testing.T) {
	restore := installWebAPIKeyReadFakes(t)
	t.Cleanup(restore)

	cmd := WebAPIKeysViewCommand()
	if err := cmd.FlagSet.Parse([]string{"--key-id", "ABC123XYZ", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := captureOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr != nil {
		t.Fatalf("expected success, got %v", execErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.WebAPIKeyGetResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode JSON: %v\nstdout: %s", err, stdout)
	}
	if result.KeyID != "ABC123XYZ" || result.Name != "Release automation" || result.IssuerID != "69a6de00-aaaa-bbbb-cccc-123456789abc" {
		t.Fatalf("unexpected key: %#v", result)
	}
	if len(result.Roles) != 1 || result.Roles[0] != "ADMIN" || !result.Active {
		t.Fatalf("unexpected roles/state: %#v", result)
	}
	assertNoKeyMaterial(t, apiKeyTestP8(t), stdout, stderr)
}

func TestWebAPIKeysListHTTPFixtureNeverPrintsPrivateKey(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	p8 := apiKeyTestP8(t)
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{TeamID: "TEAM123"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	originalClient := newWebAPIKeyClientFn
	t.Cleanup(func() { newWebAPIKeyClientFn = originalClient })

	newWebAPIKeyClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/apiKeys":
				_, _ = w.Write([]byte(`{"data":[{"id":"ABC123XYZ","attributes":{"nickname":"Release automation","roles":["ADMIN"],"isActive":true,"keyType":"PUBLIC_API","privateKey":"` + encodeAPIKeyP8(p8) + `"}}],"included":[]}`))
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
				_, _ = w.Write([]byte(`{"data":[]}`))
			default:
				http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
			}
		}))
	}

	cmd := WebAPIKeysListCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := captureOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr != nil {
		t.Fatalf("expected success, got %v", execErr)
	}
	assertNoKeyMaterial(t, p8, stdout, stderr, execErrString(execErr))
}

func TestWebAPIKeysViewHTTPFixtureNeverPrintsPrivateKey(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	p8 := apiKeyTestP8(t)
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{TeamID: "TEAM123"}, "cache", nil
	})
	t.Cleanup(restoreSession)
	originalClient := newWebAPIKeyClientFn
	t.Cleanup(func() { newWebAPIKeyClientFn = originalClient })

	newWebAPIKeyClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return newCLIAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method != http.MethodGet || r.URL.Path != "/iris/v1/apiKeys/ABC123XYZ" {
				http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"type":"apiKeys","id":"ABC123XYZ","attributes":{"nickname":"Release automation","roles":["ADMIN"],"isActive":true,"allAppsVisible":true,"keyType":"PUBLIC_API","privateKey":"` + encodeAPIKeyP8(p8) + `"},"relationships":{"provider":{"data":{"type":"contentProviders","id":"69a6de00-aaaa-bbbb-cccc-123456789abc"}}}}}`))
		}))
	}

	cmd := WebAPIKeysViewCommand()
	if err := cmd.FlagSet.Parse([]string{"--key-id", "ABC123XYZ", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := captureOutput(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if execErr != nil {
		t.Fatalf("expected success, got %v", execErr)
	}
	assertNoKeyMaterial(t, p8, stdout, stderr, execErrString(execErr))
}

func installWebAPIKeyReadFakes(t *testing.T) func() {
	t.Helper()
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{TeamID: "TEAM123"}, "cache", nil
	})
	originalNewClient := newWebAPIKeyClientFn
	originalList := listWebAPIKeysFn
	originalGet := getWebAPIKeyFn

	newWebAPIKeyClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	listWebAPIKeysFn = func(ctx context.Context, client *webcore.Client) ([]webcore.APIKeyListItem, error) {
		return []webcore.APIKeyListItem{
			{
				KeyID:  "ABC123XYZ",
				Name:   "Release automation",
				Kind:   webcore.APIKeyKindTeam,
				Roles:  []string{"ADMIN"},
				Active: true,
			},
			{
				KeyID:  "IND456ABC",
				Name:   "Personal",
				Kind:   webcore.APIKeyKindIndividual,
				Roles:  []string{"APP_MANAGER"},
				Active: false,
			},
		}, nil
	}
	getWebAPIKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKey, error) {
		if keyID != "ABC123XYZ" {
			t.Fatalf("unexpected key id %q", keyID)
		}
		return &webcore.APIKey{
			KeyID:          keyID,
			Name:           "Release automation",
			IssuerID:       "69a6de00-aaaa-bbbb-cccc-123456789abc",
			Roles:          []string{"ADMIN"},
			Active:         true,
			AllAppsVisible: true,
			KeyType:        "PUBLIC_API",
		}, nil
	}

	return func() {
		restoreSession()
		newWebAPIKeyClientFn = originalNewClient
		listWebAPIKeysFn = originalList
		getWebAPIKeyFn = originalGet
	}
}

func encodeAPIKeyP8(p8 []byte) string {
	return base64.StdEncoding.EncodeToString(p8)
}
