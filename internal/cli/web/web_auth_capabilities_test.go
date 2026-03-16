package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAuthCapabilitiesRejectsPositionalArgs(t *testing.T) {
	cmd := WebAuthCapabilitiesCommand()
	if err := cmd.FlagSet.Parse([]string{"extra"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{"extra"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestWebAuthCapabilitiesKeyIDBypassesLocalAuthResolution(t *testing.T) {
	origResolveAuth := resolveWebAuthCredentialsFn
	origResolveSession := resolveSessionFn
	origNewClient := newWebAuthClientFn
	origLookup := lookupWebAuthKeyFn
	t.Cleanup(func() {
		resolveWebAuthCredentialsFn = origResolveAuth
		resolveSessionFn = origResolveSession
		newWebAuthClientFn = origNewClient
		lookupWebAuthKeyFn = origLookup
	})

	resolveWebAuthCredentialsFn = func(profile string) (shared.ResolvedAuthCredentials, error) {
		t.Fatal("did not expect local auth resolution when --key-id is provided")
		return shared.ResolvedAuthCredentials{}, nil
	}
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebAuthClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	lookupWebAuthKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKeyRoleLookup, error) {
		if keyID != "39MX87M9Y4" {
			t.Fatalf("expected key-id override, got %q", keyID)
		}
		return &webcore.APIKeyRoleLookup{
			KeyID:      "39MX87M9Y4",
			Kind:       "team",
			Roles:      []string{"APP_MANAGER"},
			RoleSource: "key",
			Active:     true,
			Lookup:     "team_keys",
		}, nil
	}

	cmd := WebAuthCapabilitiesCommand()
	if err := cmd.FlagSet.Parse([]string{"--key-id", "39MX87M9Y4", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("Exec() error: %v", err)
	}
}

func TestWebAuthCapabilitiesResolvesCurrentAuthKeyID(t *testing.T) {
	origResolveAuth := resolveWebAuthCredentialsFn
	origResolveSession := resolveSessionFn
	origNewClient := newWebAuthClientFn
	origLookup := lookupWebAuthKeyFn
	t.Cleanup(func() {
		resolveWebAuthCredentialsFn = origResolveAuth
		resolveSessionFn = origResolveSession
		newWebAuthClientFn = origNewClient
		lookupWebAuthKeyFn = origLookup
	})

	resolveWebAuthCredentialsFn = func(profile string) (shared.ResolvedAuthCredentials, error) {
		return shared.ResolvedAuthCredentials{
			KeyID:   "ENVKEY",
			Profile: "client",
		}, nil
	}
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebAuthClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	lookupWebAuthKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKeyRoleLookup, error) {
		if keyID != "ENVKEY" {
			t.Fatalf("expected resolved key id, got %q", keyID)
		}
		return &webcore.APIKeyRoleLookup{
			KeyID:      keyID,
			Kind:       "team",
			Roles:      []string{"APP_MANAGER"},
			RoleSource: "key",
			Active:     true,
			Lookup:     "team_keys",
		}, nil
	}

	cmd := WebAuthCapabilitiesCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("Exec() error: %v", err)
	}
}

func TestWrapWebAuthCapabilitiesErrorFormatsLookupFailures(t *testing.T) {
	err := wrapWebAuthCapabilitiesError("missing", webcore.ErrAPIKeyNotFound)
	if err == nil || !strings.Contains(err.Error(), "not found in App Store Connect web key lists") {
		t.Fatalf("unexpected not-found error: %v", err)
	}

	err = wrapWebAuthCapabilitiesError("missing", webcore.ErrAPIKeyNotVisible)
	if err == nil || !strings.Contains(err.Error(), "not visible in the accessible App Store Connect web key lists") {
		t.Fatalf("unexpected not-visible error: %v", err)
	}

	err = wrapWebAuthCapabilitiesError("missing", webcore.ErrAPIKeyRolesUnresolved)
	if err == nil || !strings.Contains(err.Error(), "exact roles could not be resolved") {
		t.Fatalf("unexpected unresolved error: %v", err)
	}
}

func TestWebAuthCapabilitiesMissingLocalAuthReturnsUsageError(t *testing.T) {
	origResolveAuth := resolveWebAuthCredentialsFn
	t.Cleanup(func() {
		resolveWebAuthCredentialsFn = origResolveAuth
	})

	resolveWebAuthCredentialsFn = func(profile string) (shared.ResolvedAuthCredentials, error) {
		return shared.ResolvedAuthCredentials{}, errors.New("missing authentication")
	}

	cmd := WebAuthCapabilitiesCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestWebAuthCapabilitiesRejectsPrettyForTableOutput(t *testing.T) {
	cmd := WebAuthCapabilitiesCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "table", "--pretty"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestWebAuthCapabilitiesRows(t *testing.T) {
	rows := webAuthCapabilitiesRows(webAuthCapabilitiesResult{
		KeyID:        "39MX87M9Y4",
		Kind:         "team",
		Active:       true,
		Roles:        []string{"APP_MANAGER", "FINANCE"},
		Name:         "asc_cli",
		Lookup:       "team_keys",
		ResolvedFrom: "auth",
		Profile:      "client",
	})
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	if rows[0][3] != "APP_MANAGER, FINANCE" {
		t.Fatalf("unexpected role join output: %#v", rows[0])
	}
}

func TestWebAuthCapabilitiesKeyIDOutputsJSON(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveAuth := resolveWebAuthCredentialsFn
	origResolveSession := resolveSessionFn
	origNewClient := newWebAuthClientFn
	origLookup := lookupWebAuthKeyFn
	t.Cleanup(func() {
		resolveWebAuthCredentialsFn = origResolveAuth
		resolveSessionFn = origResolveSession
		newWebAuthClientFn = origNewClient
		lookupWebAuthKeyFn = origLookup
	})

	resolveWebAuthCredentialsFn = func(profile string) (shared.ResolvedAuthCredentials, error) {
		t.Fatal("did not expect local auth resolution when --key-id is provided")
		return shared.ResolvedAuthCredentials{}, nil
	}
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebAuthClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	lookupWebAuthKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKeyRoleLookup, error) {
		return &webcore.APIKeyRoleLookup{
			KeyID:      keyID,
			Name:       "asc_cli",
			Kind:       "team",
			Roles:      []string{"APP_MANAGER", "FINANCE"},
			RoleSource: "key",
			Active:     true,
			KeyType:    "PUBLIC_API",
			LastUsed:   "2026-03-16T00:00:00Z",
			Lookup:     "team_keys",
			GeneratedBy: &webcore.KeyActor{
				ID:   "user-1",
				Name: "Jane Admin",
			},
		}, nil
	}

	cmd := WebAuthCapabilitiesCommand()
	if err := cmd.FlagSet.Parse([]string{"--key-id", "39MX87M9Y4", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var got webAuthCapabilitiesResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if got.KeyID != "39MX87M9Y4" || got.ResolvedFrom != "flag" || got.Profile != "" {
		t.Fatalf("unexpected json payload: %+v", got)
	}
	if len(got.Roles) != 2 || got.Roles[1] != "FINANCE" {
		t.Fatalf("unexpected roles: %#v", got.Roles)
	}
	if got.GeneratedBy == nil || got.GeneratedBy.Name != "Jane Admin" {
		t.Fatalf("unexpected generatedBy: %#v", got.GeneratedBy)
	}
	if len(*labels) != 1 || (*labels)[0] != "Loading exact API key roles" {
		t.Fatalf("unexpected progress labels: %#v", *labels)
	}
}

func TestWebAuthCapabilitiesAuthResolutionOutputsJSON(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveAuth := resolveWebAuthCredentialsFn
	origResolveSession := resolveSessionFn
	origNewClient := newWebAuthClientFn
	origLookup := lookupWebAuthKeyFn
	t.Cleanup(func() {
		resolveWebAuthCredentialsFn = origResolveAuth
		resolveSessionFn = origResolveSession
		newWebAuthClientFn = origNewClient
		lookupWebAuthKeyFn = origLookup
	})

	resolveWebAuthCredentialsFn = func(profile string) (shared.ResolvedAuthCredentials, error) {
		return shared.ResolvedAuthCredentials{
			KeyID:   "ENVKEY",
			Profile: "client",
		}, nil
	}
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebAuthClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	lookupWebAuthKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKeyRoleLookup, error) {
		return &webcore.APIKeyRoleLookup{
			KeyID:      keyID,
			Kind:       "team",
			Roles:      []string{"APP_MANAGER"},
			RoleSource: "key",
			Active:     true,
			Lookup:     "team_keys",
		}, nil
	}

	cmd := WebAuthCapabilitiesCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var got webAuthCapabilitiesResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if got.KeyID != "ENVKEY" || got.ResolvedFrom != "auth" || got.Profile != "client" {
		t.Fatalf("unexpected json payload: %+v", got)
	}
	if len(got.Roles) != 1 || got.Roles[0] != "APP_MANAGER" {
		t.Fatalf("unexpected roles: %#v", got.Roles)
	}
	if len(*labels) != 1 || (*labels)[0] != "Loading exact API key roles" {
		t.Fatalf("unexpected progress labels: %#v", *labels)
	}
}

func TestWebAuthCapabilitiesUnauthorizedLookupGetsWebHint(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	origNewClient := newWebAuthClientFn
	origLookup := lookupWebAuthKeyFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebAuthClientFn = origNewClient
		lookupWebAuthKeyFn = origLookup
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebAuthClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	lookupWebAuthKeyFn = func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKeyRoleLookup, error) {
		return nil, &webcore.APIError{Status: 401}
	}

	cmd := WebAuthCapabilitiesCommand()
	if err := cmd.FlagSet.Parse([]string{"--key-id", "39MX87M9Y4", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "web session is unauthorized or expired") {
		t.Fatalf("expected web auth hint, got %v", err)
	}
	if !strings.Contains(err.Error(), "asc web auth login") {
		t.Fatalf("expected login guidance, got %v", err)
	}
	if len(*labels) != 1 || (*labels)[0] != "Loading exact API key roles" {
		t.Fatalf("unexpected progress labels: %#v", *labels)
	}
}

func TestWebAuthCapabilitiesAuthResolutionFailureIsUsageError(t *testing.T) {
	origResolveAuth := resolveWebAuthCredentialsFn
	t.Cleanup(func() {
		resolveWebAuthCredentialsFn = origResolveAuth
	})

	resolveWebAuthCredentialsFn = func(profile string) (shared.ResolvedAuthCredentials, error) {
		return shared.ResolvedAuthCredentials{}, fmt.Errorf("mixed authentication sources detected")
	}

	cmd := WebAuthCapabilitiesCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "unable to resolve current API key ID") {
		t.Fatalf("expected auth resolution prefix, got %q", stderr)
	}
	if !strings.Contains(stderr, "mixed authentication sources detected") {
		t.Fatalf("expected wrapped auth resolution cause, got %q", stderr)
	}
}

func TestWebAuthCapabilitiesHelpContrastsPublicCapabilities(t *testing.T) {
	cmd := WebAuthCapabilitiesCommand()
	usage := cmd.UsageFunc(cmd)

	if !strings.Contains(usage, `Unlike "asc auth capabilities", which probes effective public-API access`) {
		t.Fatalf("expected usage to contrast public auth capabilities, got %q", usage)
	}
	if !strings.Contains(usage, "--key-id") {
		t.Fatalf("expected usage to describe --key-id, got %q", usage)
	}
}
