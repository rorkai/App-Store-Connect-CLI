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

func appsResponseFromIDs(ids []string) *asc.AppsResponse {
	resp := &asc.AppsResponse{
		Data: make([]asc.Resource[asc.AppAttributes], 0, len(ids)),
	}
	for _, id := range ids {
		resp.Data = append(resp.Data, asc.Resource[asc.AppAttributes]{ID: id})
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

func TestResolveAppIDWithLookup_DoesNotResolveFromEnv(t *testing.T) {
	t.Setenv("ASC_APP_ID", "123456789")
	got, err := ResolveAppIDWithLookup(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty app id when input is empty, got %q", got)
	}
}

func TestResolveAppIDWithLookup_ResolvesByBundleThenName(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	bundleOnly := &sequenceAppLookupStub{
		responses: []*asc.AppsResponse{
			appsResponseFromIDs([]string{"app-bundle"}),
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
			appsResponseFromIDs(nil),
			appsResponseFromIDs([]string{"app-name"}),
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
			appsResponseFromIDs(nil),
			appsResponseFromIDs(nil),
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
			appsResponseFromIDs(nil),
			appsResponseFromIDs([]string{"app-1", "app-2"}),
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
