package reference

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	v, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if v.LastVerified == "" {
		t.Fatal("expected last_verified")
	}
	if len(v.Roles) == 0 {
		t.Fatal("expected roles")
	}
	if len(v.Groups) == 0 {
		t.Fatal("expected capability groups")
	}
	if len(v.Limitations) == 0 {
		t.Fatal("expected limitations")
	}
}

func TestResolveTeam(t *testing.T) {
	view, err := Resolve("team", []string{"APP_MANAGER", "FINANCE"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(view.RoleDetails) != 2 {
		t.Fatalf("expected two role details, got %d", len(view.RoleDetails))
	}
	if len(view.Capabilities) == 0 {
		t.Fatal("expected capabilities")
	}
	if view.Capabilities[0].ID != "all_apps_access" && view.Capabilities[0].ID != "app_pricing_and_store_info" {
		t.Fatalf("unexpected first capability: %#v", view.Capabilities[0])
	}
	if view.Scope == nil || !view.Scope.AppliesToAllApps {
		t.Fatalf("expected team scope, got %#v", view.Scope)
	}
	if view.KeyNotes == nil || view.KeyNotes.Kind != "team" {
		t.Fatalf("expected team key notes, got %#v", view.KeyNotes)
	}
	if len(view.DocumentedAccess) == 0 {
		t.Fatal("expected documented access")
	}
	if len(view.Sources) == 0 {
		t.Fatal("expected sources")
	}
	if len(view.Limitations) == 0 {
		t.Fatal("expected limitations")
	}
}

func TestResolveIndividual(t *testing.T) {
	view, err := Resolve("individual", []string{"APP_MANAGER"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if view.Scope != nil {
		t.Fatalf("expected no explicit individual scope, got %#v", view.Scope)
	}
	if view.KeyNotes == nil || view.KeyNotes.Kind != "individual" {
		t.Fatalf("expected individual key notes, got %#v", view.KeyNotes)
	}
	if view.KeyNotes.OneActiveKeyPerUser == nil || !*view.KeyNotes.OneActiveKeyPerUser {
		t.Fatalf("expected one-active-key note, got %#v", view.KeyNotes)
	}
}

func TestResolveAccountHolderIncludesBroadAccess(t *testing.T) {
	view, err := Resolve("individual", []string{"ACCOUNT_HOLDER"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	ids := make(map[string]struct{}, len(view.Capabilities))
	for _, item := range view.Capabilities {
		ids[item.ID] = struct{}{}
	}
	for _, want := range []string{
		"all_apps_access",
		"app_pricing_and_store_info",
		"app_development_and_delivery",
		"payments_financial_reports_and_tax",
		"customer_reviews",
	} {
		if _, ok := ids[want]; !ok {
			t.Fatalf("expected account holder capabilities to include %q, got %#v", want, view.Capabilities)
		}
	}
}

func TestResolveAdminIncludesBroadAccess(t *testing.T) {
	view, err := Resolve("team", []string{"ADMIN"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	ids := make(map[string]struct{}, len(view.Capabilities))
	for _, item := range view.Capabilities {
		ids[item.ID] = struct{}{}
	}
	for _, want := range []string{
		"all_apps_access",
		"app_pricing_and_store_info",
		"app_development_and_delivery",
		"marketing_and_promotional_artwork",
		"customer_reviews",
		"app_analytics",
		"sales_and_trends",
		"payments_financial_reports_and_tax",
	} {
		if _, ok := ids[want]; !ok {
			t.Fatalf("expected admin capabilities to include %q, got %#v", want, view.Capabilities)
		}
	}
}

func TestResolveUnknownRole(t *testing.T) {
	view, err := Resolve("team", []string{"NOPE", "APP_MANAGER"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(view.UnknownRoles) != 1 || view.UnknownRoles[0] != "NOPE" {
		t.Fatalf("unexpected unknown roles: %#v", view.UnknownRoles)
	}
	if len(view.RoleDetails) != 1 || view.RoleDetails[0].Code != "APP_MANAGER" {
		t.Fatalf("unexpected role details: %#v", view.RoleDetails)
	}
}

func TestValidateRejectsUnknownCapabilityReferences(t *testing.T) {
	err := validateSnapshot(&Snapshot{
		Groups: []CapabilityGroup{
			{ID: "known"},
		},
		Roles: []Role{
			{Code: "ADMIN", Capabilities: []string{"known", "missing"}},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected missing capability id in error, got %v", err)
	}
}
