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
	id       string
	name     string
	bundleID string
}

func appsResponseFromApps(apps []appFixture) *asc.AppsResponse {
	resp := &asc.AppsResponse{
		Data: make([]asc.Resource[asc.AppAttributes], 0, len(apps)),
	}
	for _, app := range apps {
		resp.Data = append(resp.Data, asc.Resource[asc.AppAttributes]{
			ID: app.id,
			Attributes: asc.AppAttributes{
				Name: app.name, BundleID: app.bundleID,
			},
		})
	}
	return resp
}

func appsResponseFromAppsWithNext(apps []appFixture) *asc.AppsResponse {
	resp := appsResponseFromApps(apps)
	resp.Links.Next = "https://api.appstoreconnect.apple.com/v1/apps?cursor=next"
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

func TestIsNumericAppID(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty", value: "", want: false},
		{name: "numeric", value: "123456789", want: true},
		{name: "whitespace", value: " 123456789 ", want: false},
		{name: "bundle id", value: "com.example.app", want: false},
		{name: "mixed", value: "123abc", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsNumericAppID(test.value); got != test.want {
				t.Fatalf("IsNumericAppID(%q) = %t, want %t", test.value, got, test.want)
			}
		})
	}
}

func TestResolveAppIDWithLookup_NumericPassthroughSkipsClientLookup(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	stub := &sequenceAppLookupStub{
		responses: []*asc.AppsResponse{
			appsResponseFromApps([]appFixture{{id: "unexpected", name: "Unexpected"}}),
		},
	}

	got, err := ResolveAppIDWithLookup(context.Background(), stub, "123456789")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "123456789" {
		t.Fatalf("expected numeric app id passthrough, got %q", got)
	}
	if stub.calls != 0 {
		t.Fatalf("expected numeric app id passthrough to skip client lookup, got %d calls", stub.calls)
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
			appsResponseFromApps([]appFixture{{id: "app-bundle", name: "Bundle App", bundleID: "com.example.app"}}),
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
	if !strings.Contains(err.Error(), `2 apps match "My App"; pass --app with one of:`) {
		t.Fatalf("expected ambiguous name error, got %v", err)
	}
}

func TestResolveAppIDWithLookupRejectsUnfinishedBundleSearch(t *testing.T) {
	page := appsResponseFromAppsWithNext([]appFixture{
		{id: "app-1", bundleID: "com.example.app"},
	})
	stub := &sequenceAppLookupStub{responses: []*asc.AppsResponse{page, page}}
	got, err := ResolveAppIDWithLookup(context.Background(), stub, "com.example.app")
	if got != "" || err == nil || !strings.Contains(err.Error(), "pagination") {
		t.Fatalf("incomplete lookup resolved %q or failed unexpectedly: %v", got, err)
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

func TestResolveAppIDWithLookupRejectsUniqueFuzzyName(t *testing.T) {
	candidate := appsResponseFromApps([]appFixture{{id: "app-fuzzy", name: "Example Pro"}})
	stub := &sequenceAppLookupStub{responses: []*asc.AppsResponse{appsResponseFromApps(nil), candidate, candidate, candidate}}
	got, err := ResolveAppIDWithLookup(context.Background(), stub, "Example")
	if got != "" || err == nil || !strings.Contains(err.Error(), "asc apps list") {
		t.Fatalf("fuzzy lookup resolved %q or lacked discovery guidance: %v", got, err)
	}
}

func TestResolveAppIDWithExactLookup_NumericPassthroughTakesPriorityOverExactName(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stub := &sequenceAppLookupStub{
		responses: []*asc.AppsResponse{
			appsResponseFromApps(nil),
			appsResponseFromApps([]appFixture{
				{id: "app-name", name: "2048"},
			}),
		},
	}

	got, err := ResolveAppIDWithExactLookup(context.Background(), stub, "2048")
	if err != nil {
		t.Fatalf("ResolveAppIDWithExactLookup() error: %v", err)
	}
	if got != "2048" {
		t.Fatalf("expected numeric passthrough app id 2048, got %q", got)
	}
	if stub.calls != 0 {
		t.Fatalf("expected numeric passthrough without lookup calls, got %d", stub.calls)
	}
}

func TestResolveAppIDWithLookup_ExactBundleIDWinsOverPrefixSiblings(t *testing.T) {
	page := func(hasNext bool, apps ...[2]string) *asc.AppsResponse {
		resp := &asc.AppsResponse{}
		for _, app := range apps {
			resp.Data = append(resp.Data, asc.Resource[asc.AppAttributes]{ID: app[0], Attributes: asc.AppAttributes{BundleID: app[1]}})
		}
		if hasNext {
			resp.Links.Next = "https://api.appstoreconnect.apple.com/v1/apps?cursor=next"
		}
		return resp
	}
	exact, dev, beta := [2]string{"app-exact", "com.example.app"}, [2]string{"app-dev", "com.example.app.dev"}, [2]string{"app-beta", "com.example.app.beta"}

	for _, test := range []struct {
		name      string
		input     string
		pages     []*asc.AppsResponse
		want      string
		wantErr   string
		wantCalls int
	}{
		{name: "exact match with a longer sibling", pages: []*asc.AppsResponse{page(false, dev, exact)}, want: "app-exact", wantCalls: 1},
		{name: "exact match in another letter case", input: "com.example.APP", pages: []*asc.AppsResponse{page(false, exact, dev)}, want: "app-exact", wantCalls: 1},
		{name: "two siblings and no exact match", pages: []*asc.AppsResponse{page(false, dev, beta)}, wantErr: "not found", wantCalls: 3},
		{name: "one result that is not exact", pages: []*asc.AppsResponse{page(false, dev)}, wantErr: "not found", wantCalls: 3},
		{name: "two exact apps", pages: []*asc.AppsResponse{page(false, exact, [2]string{"duplicate", "com.example.app"})}, wantErr: "2 apps match", wantCalls: 1},
		{name: "exact match beyond the first page", pages: []*asc.AppsResponse{page(true, dev, beta), page(false, dev, beta, exact)}, want: "app-exact", wantCalls: 2},
		{name: "exact match after an empty first page", pages: []*asc.AppsResponse{page(true), page(false, exact)}, want: "app-exact", wantCalls: 2},
		{name: "no exact match on a truncated wider page", pages: []*asc.AppsResponse{page(true, dev, beta), page(true, dev, beta)}, wantErr: "pagination", wantCalls: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &sequenceAppLookupStub{responses: test.pages}
			input := test.input
			if input == "" {
				input = "com.example.app"
			}
			got, err := ResolveAppIDWithLookup(context.Background(), stub, input)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, test.wantErr)
				}
			} else if err != nil || got != test.want {
				t.Fatalf("resolved %q, %v, want %q", got, err, test.want)
			}
			if stub.calls != test.wantCalls {
				t.Fatalf("lookups = %d, want %d", stub.calls, test.wantCalls)
			}
		})
	}
}

func TestResolveAppIDWithLookupRejectsForeignContinuationPath(t *testing.T) {
	foreignPage := func(path string) *asc.AppsResponse {
		resp := appsResponseFromApps(nil)
		resp.Links.Next = "https://api.appstoreconnect.apple.com" + path + "?cursor=next"
		return resp
	}
	for _, test := range []struct {
		name      string
		selector  string
		responses []*asc.AppsResponse
		wantCalls int
	}{
		{
			name:      "bundle lookup",
			selector:  "com.example.app",
			responses: []*asc.AppsResponse{foreignPage("/v1/subscriptionGroups")},
			wantCalls: 1,
		},
		{
			name:     "name lookup",
			selector: "Example App",
			responses: []*asc.AppsResponse{
				appsResponseFromApps(nil),
				foreignPage("/v1/subscriptionGroups"),
			},
			wantCalls: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &sequenceAppLookupStub{responses: test.responses}
			_, err := ResolveAppIDWithLookup(context.Background(), stub, test.selector)
			if err == nil || !strings.Contains(err.Error(), "pagination URL") {
				t.Fatalf("expected foreign continuation to fail closed, got %v", err)
			}
			if stub.calls != test.wantCalls {
				t.Fatalf("lookups = %d, want %d proof calls and no foreign-page fetch", stub.calls, test.wantCalls)
			}
		})
	}
}

func TestScopedAppLookupsRequireExactIdentity(t *testing.T) {
	for _, lookup := range []struct {
		name    string
		resolve func(context.Context, appLookupClient, string) (string, error)
	}{
		{name: "default", resolve: ResolveAppIDWithLookup},
		{name: "exact", resolve: ResolveAppIDWithExactLookup},
	} {
		t.Run(lookup.name, func(t *testing.T) {
			for _, test := range []struct {
				name     string
				selector string
				filter   *asc.AppsResponse
			}{
				{
					name: "bundle prefix", selector: "com.example.demo",
					filter: &asc.AppsResponse{Data: []asc.Resource[asc.AppAttributes]{
						{ID: "other", Attributes: asc.AppAttributes{BundleID: "com.example.demo.preview"}},
					}},
				},
				{
					name: "bundle attribute absent", selector: "com.example.demo",
					filter: &asc.AppsResponse{Data: []asc.Resource[asc.AppAttributes]{{ID: "other"}}},
				},
			} {
				t.Run(test.name, func(t *testing.T) {
					stub := &sequenceAppLookupStub{responses: []*asc.AppsResponse{test.filter}}
					got, err := lookup.resolve(context.Background(), stub, test.selector)
					if got != "" || err == nil {
						t.Fatalf("lookup resolved an unverified app: id=%q error=%v", got, err)
					}
				})
			}
		})
	}
}
