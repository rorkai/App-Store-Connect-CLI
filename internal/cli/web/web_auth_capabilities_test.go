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
	webref "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web/reference"
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

func TestWebAuthCapabilitiesKeyIDOutputsJSON(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveAuth := resolveWebAuthCredentialsFn
	origResolveSession := resolveSessionFn
	origNewClient := newWebAuthClientFn
	origLookup := lookupWebAuthKeyFn
	origResolveRef := resolveWebAuthRefFn
	t.Cleanup(func() {
		resolveWebAuthCredentialsFn = origResolveAuth
		resolveSessionFn = origResolveSession
		newWebAuthClientFn = origNewClient
		lookupWebAuthKeyFn = origLookup
		resolveWebAuthRefFn = origResolveRef
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
	resolveWebAuthRefFn = func(kind string, codes []string) (*webref.View, error) {
		return &webref.View{
			LastVerified: "2026-03-16",
			Purpose:      "Reference snapshot of Apple-documented App Store Connect role capabilities.",
			Sources: []webref.Source{
				{Title: "Apple Developer Program Roles", URL: "https://developer.apple.com/help/account/access/roles/"},
			},
			Scope: &webref.Scope{
				AppliesToAllApps: true,
				Summary:          "Team API keys apply across all apps.",
			},
			KeyNotes: &webref.KeyNotes{
				Kind:                  "team",
				SelectableRoles:       []string{"ADMIN", "APP_MANAGER"},
				EditableAfterCreation: boolPtr(false),
			},
			RoleDetails: []webref.Role{
				{
					Code:         "APP_MANAGER",
					Label:        "App Manager",
					Capabilities: []string{"app_pricing_and_store_info", "app_development_and_delivery"},
				},
				{
					Code:         "FINANCE",
					Label:        "Finance",
					Capabilities: []string{"payments_financial_reports_and_tax"},
				},
			},
			Capabilities: []webref.CapabilityGroup{
				{ID: "app_pricing_and_store_info", Label: "Manage app pricing and App Store information"},
				{ID: "app_development_and_delivery", Label: "Manage app development and delivery"},
				{ID: "payments_financial_reports_and_tax", Label: "Payments, financial reports, and tax forms"},
			},
			DocumentedAccess: []webref.DocumentedAccess{
				{
					ID:         "app_pricing_and_store_info",
					Label:      "Manage app pricing and App Store information",
					Roles:      []string{"APP_MANAGER"},
					RoleLabels: []string{"App Manager"},
				},
				{
					ID:         "payments_financial_reports_and_tax",
					Label:      "Payments, financial reports, and tax forms",
					Roles:      []string{"FINANCE"},
					RoleLabels: []string{"Finance"},
				},
			},
			Limitations: []string{
				"Exact role lookup comes from the live App Store Connect web session, but the expanded capabilities below come from this bundled Apple documentation snapshot.",
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
	if len(got.RoleDetails) != 2 || got.RoleDetails[0].Label != "App Manager" {
		t.Fatalf("unexpected roleDetails: %#v", got.RoleDetails)
	}
	if len(got.Capabilities) != 3 || got.Capabilities[2].ID != "payments_financial_reports_and_tax" {
		t.Fatalf("unexpected capabilities: %#v", got.Capabilities)
	}
	if len(got.DocumentedAccess) != 2 || got.DocumentedAccess[1].Roles[0] != "FINANCE" {
		t.Fatalf("unexpected documentedAccess: %#v", got.DocumentedAccess)
	}
	if len(got.Sources) != 1 || got.Sources[0].Title != "Apple Developer Program Roles" {
		t.Fatalf("unexpected sources: %#v", got.Sources)
	}
	if got.Scope == nil || !got.Scope.AppliesToAllApps {
		t.Fatalf("unexpected scope: %#v", got.Scope)
	}
	if got.KeyNotes == nil || got.KeyNotes.Kind != "team" || got.KeyNotes.EditableAfterCreation == nil || *got.KeyNotes.EditableAfterCreation {
		t.Fatalf("unexpected keyNotes: %#v", got.KeyNotes)
	}
	if got.ReferencePurpose == "" || got.ReferenceLastVerified != "2026-03-16" {
		t.Fatalf("unexpected reference metadata: %+v", got)
	}
	if len(got.Limitations) != 1 {
		t.Fatalf("unexpected limitations: %#v", got.Limitations)
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
	origResolveRef := resolveWebAuthRefFn
	t.Cleanup(func() {
		resolveWebAuthCredentialsFn = origResolveAuth
		resolveSessionFn = origResolveSession
		newWebAuthClientFn = origNewClient
		lookupWebAuthKeyFn = origLookup
		resolveWebAuthRefFn = origResolveRef
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
	resolveWebAuthRefFn = func(kind string, codes []string) (*webref.View, error) {
		return &webref.View{
			LastVerified: "2026-03-16",
			Purpose:      "Reference snapshot",
			RoleDetails: []webref.Role{{
				Code:  "APP_MANAGER",
				Label: "App Manager",
			}},
			Capabilities: []webref.CapabilityGroup{{
				ID:    "app_development_and_delivery",
				Label: "Manage app development and delivery",
			}},
			KeyNotes: &webref.KeyNotes{
				Kind:                "individual",
				OneActiveKeyPerUser: boolPtr(true),
			},
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
	if len(got.RoleDetails) != 1 || got.RoleDetails[0].Label != "App Manager" {
		t.Fatalf("unexpected roleDetails: %#v", got.RoleDetails)
	}
	if len(got.Capabilities) != 1 || got.Capabilities[0].Label != "Manage app development and delivery" {
		t.Fatalf("unexpected capabilities: %#v", got.Capabilities)
	}
	if got.KeyNotes == nil || got.KeyNotes.Kind != "individual" || got.KeyNotes.OneActiveKeyPerUser == nil || !*got.KeyNotes.OneActiveKeyPerUser {
		t.Fatalf("unexpected keyNotes: %#v", got.KeyNotes)
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
	origResolveRef := resolveWebAuthRefFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebAuthClientFn = origNewClient
		lookupWebAuthKeyFn = origLookup
		resolveWebAuthRefFn = origResolveRef
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
	resolveWebAuthRefFn = func(kind string, codes []string) (*webref.View, error) {
		t.Fatal("did not expect reference resolution on failed lookup")
		return nil, nil
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
	if !strings.Contains(usage, `asc web auth capabilities --apple-id "user@example.com"`) {
		t.Fatalf("expected usage to recommend --apple-id like other web commands, got %q", usage)
	}
	if !strings.Contains(usage, "documented role capabilities") {
		t.Fatalf("expected usage to mention documented capabilities, got %q", usage)
	}
	if !strings.Contains(usage, "flattened documented access with role provenance") {
		t.Fatalf("expected usage to mention agent-facing json metadata, got %q", usage)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
