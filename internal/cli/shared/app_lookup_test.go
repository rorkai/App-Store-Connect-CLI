package shared

import (
	"context"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type sequenceAppLookupStub struct {
	responses []*asc.AppsResponse
	calls     int
}

func (s *sequenceAppLookupStub) GetApps(_ context.Context, _ ...asc.AppsOption) (*asc.AppsResponse, error) {
	if s.calls >= len(s.responses) {
		s.calls++
		return &asc.AppsResponse{}, nil
	}
	resp := s.responses[s.calls]
	s.calls++
	if resp == nil {
		return &asc.AppsResponse{}, nil
	}
	return resp, nil
}

type appFixture struct {
	id   string
	name string
}

func appsResponseFromApps(apps []appFixture) *asc.AppsResponse {
	resp := &asc.AppsResponse{
		Data: make([]asc.Resource[asc.AppAttributes], 0, len(apps)),
	}
	for _, app := range apps {
		resp.Data = append(resp.Data, asc.Resource[asc.AppAttributes]{
			ID: app.id,
			Attributes: asc.AppAttributes{
				Name: app.name,
			},
		})
	}
	return resp
}

func TestResolveAppIDWithLookup_NumericPassthrough(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	got, err := ResolveAppIDWithLookup(context.Background(), nil, "123456789")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "123456789" {
		t.Fatalf("expected numeric app id passthrough, got %q", got)
	}
}

func TestResolveAppIDWithLookup_DoesNotReResolveFromEnv(t *testing.T) {
	t.Setenv("ASC_APP_ID", "999888777")
	got, err := ResolveAppIDWithLookup(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty when input is empty (should not re-resolve from env), got %q", got)
	}
}

func TestResolveAppIDWithLookup_ResolvesByBundleThenName(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	bundleOnly := &sequenceAppLookupStub{
		responses: []*asc.AppsResponse{
			appsResponseFromApps([]appFixture{{id: "app-bundle", name: "Bundle App"}}),
		},
	}
	got, err := ResolveAppIDWithLookup(context.Background(), bundleOnly, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-bundle" {
		t.Fatalf("expected bundle match app-bundle, got %q", got)
	}

	nameOnly := &sequenceAppLookupStub{
		responses: []*asc.AppsResponse{
			appsResponseFromApps(nil),
			appsResponseFromApps([]appFixture{{id: "app-name", name: "Example App"}}),
		},
	}
	got, err = ResolveAppIDWithLookup(context.Background(), nameOnly, "Example App")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-name" {
		t.Fatalf("expected name match app-name, got %q", got)
	}
}

func TestResolveAppIDWithLookup_NotFound(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stub := &sequenceAppLookupStub{
		responses: []*asc.AppsResponse{
			appsResponseFromApps(nil),
			appsResponseFromApps([]appFixture{{id: "app-fuzzy", name: "Missing App Pro"}}),
			appsResponseFromApps([]appFixture{{id: "app-fuzzy", name: "Missing App Pro"}}),
			appsResponseFromApps(nil),
		},
	}
	_, err := ResolveAppIDWithLookup(context.Background(), stub, "missing-app")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestResolveAppIDWithLookup_AmbiguousName(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stub := &sequenceAppLookupStub{
		responses: []*asc.AppsResponse{
			appsResponseFromApps(nil),
			appsResponseFromApps([]appFixture{
				{id: "app-1", name: "My App"},
				{id: "app-2", name: "My App"},
			}),
		},
	}
	_, err := ResolveAppIDWithLookup(context.Background(), stub, "My App")
	if err == nil {
		t.Fatal("expected ambiguous name error")
	}
	if !strings.Contains(err.Error(), "multiple apps found for name") {
		t.Fatalf("expected ambiguous name error, got %v", err)
	}
}

func TestResolveAppIDWithLookup_PrefersExactNameOverFuzzyMatches(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stub := &sequenceAppLookupStub{
		responses: []*asc.AppsResponse{
			appsResponseFromApps(nil),
			appsResponseFromApps([]appFixture{
				{id: "app-fuzzy", name: "Musadora Labs"},
				{id: "app-exact", name: "Musadora"},
			}),
		},
	}

	got, err := ResolveAppIDWithLookup(context.Background(), stub, "Musadora")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-exact" {
		t.Fatalf("expected exact-name app id app-exact, got %q", got)
	}
}

func TestResolveAppIDWithLookup_FallsBackToFullScanWhenNameFilterMissesExact(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stub := &sequenceAppLookupStub{
		responses: []*asc.AppsResponse{
			appsResponseFromApps(nil),
			appsResponseFromApps([]appFixture{
				{id: "app-fuzzy", name: "Unwindr: Mind & Sleep Aid"},
			}),
			appsResponseFromApps([]appFixture{
				{id: "app-exact", name: "Unwind: Meditate, Sleep, Relax"},
			}),
		},
	}

	got, err := ResolveAppIDWithLookup(context.Background(), stub, "Unwind: Meditate, Sleep, Relax")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-exact" {
		t.Fatalf("expected fallback exact-name app id app-exact, got %q", got)
	}
}

func TestResolveAppIDWithLookup_FallsBackToUniqueFuzzyMatchWhenNoExact(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stub := &sequenceAppLookupStub{
		responses: []*asc.AppsResponse{
			appsResponseFromApps(nil),
			appsResponseFromApps([]appFixture{
				{id: "app-fuzzy", name: "Relax: Sleep + Focus"},
			}),
			appsResponseFromApps(nil),
			appsResponseFromApps([]appFixture{
				{id: "app-fuzzy", name: "Relax: Sleep + Focus"},
			}),
		},
	}

	got, err := ResolveAppIDWithLookup(context.Background(), stub, "Relax")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-fuzzy" {
		t.Fatalf("expected legacy fuzzy fallback app id app-fuzzy, got %q", got)
	}
}
