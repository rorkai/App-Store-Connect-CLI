package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSparseAppFieldFlagsValidateBeforeAuth(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"apps list iap", []string{"apps", "list", "--iap-fields", "name"}, "--iap-fields must be one of: versions"},
		{"apps view group", []string{"apps", "view", "--id", "app-1", "--subscription-group-fields", "subscriptions"}, "--subscription-group-fields must be one of: versions"},
		{"app infos list fields", []string{"apps", "info", "list", "--app", "app-1", "--fields", "state"}, "--fields must be one of: kidsAgeBand"},
		{"app info view age rating", []string{"apps", "info", "view", "--info-id", "info-1", "--age-rating-fields", "gambling"}, "--age-rating-fields must be one of"},
		{"app info fields conflict with version", []string{"apps", "info", "view", "--app", "app-1", "--version-id", "version-1", "--fields", "kidsAgeBand"}, "--fields cannot be used with version localization flags"},
		{"app info age rating fields conflict with next", []string{"apps", "info", "view", "--app", "app-1", "--next", "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations?cursor=next", "--age-rating-fields", "socialMedia"}, "--age-rating-fields cannot be used with version localization flags"},
		{"age rating view", []string{"age-rating", "view", "--app-info-id", "info-1", "--fields", "gambling"}, "--fields must be one of"},
		{"xcode cloud product app", []string{"xcode-cloud", "products", "app", "--id", "product-1", "--iap-fields", "name"}, "--iap-fields must be one of: versions"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v, want flag.ErrHelp", err)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
		})
	}
}

func TestAppsListSparseFieldsConflictWithNextBeforeAuth(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps?cursor=next"
	for _, flagName := range []string{"--iap-fields", "--subscription-group-fields"} {
		t.Run(flagName, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"apps", "list", "--next", next, flagName, ""}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v, want flag.ErrHelp", err)
				}
			})
			if !strings.Contains(stderr, "--next cannot be combined with "+flagName) {
				t.Fatalf("stderr = %q", stderr)
			}
		})
	}
}

func TestSparseAppFieldCommandsSendExactQueries(t *testing.T) {
	setupAuth(t)
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantQuery map[string]string
		response  string
	}{
		{
			name: "apps list", args: []string{"apps", "list", "--iap-fields", "versions", "--subscription-group-fields", "versions", "--output", "json"},
			wantPath: "/v1/apps", response: `{"data":[]}`,
			wantQuery: map[string]string{"fields[inAppPurchases]": "versions", "fields[subscriptionGroups]": "versions", "include": "inAppPurchases,subscriptionGroups"},
		},
		{
			name: "apps view", args: []string{"apps", "view", "--id", "app-1", "--iap-fields", "versions", "--subscription-group-fields", "versions", "--output", "json"},
			wantPath: "/v1/apps/app-1", response: `{"data":{"type":"apps","id":"app-1"}}`,
			wantQuery: map[string]string{"fields[inAppPurchases]": "versions", "fields[subscriptionGroups]": "versions", "include": "inAppPurchases,subscriptionGroups"},
		},
		{
			name: "app infos list", args: []string{"apps", "info", "list", "--app", "app-1", "--fields", "kidsAgeBand", "--age-rating-fields", "socialMedia,socialMediaAgeRestricted", "--output", "json"},
			wantPath: "/v1/apps/app-1/appInfos", response: `{"data":[]}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "fields[ageRatingDeclarations]": "socialMedia,socialMediaAgeRestricted", "include": "ageRatingDeclaration"},
		},
		{
			name: "app info view", args: []string{"apps", "info", "view", "--info-id", "info-1", "--fields", "kidsAgeBand", "--age-rating-fields", "socialMedia", "--output", "json"},
			wantPath: "/v1/appInfos/info-1", response: `{"data":{"type":"appInfos","id":"info-1"}}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "fields[ageRatingDeclarations]": "socialMedia", "include": "ageRatingDeclaration"},
		},
		{
			name: "age rating view", args: []string{"age-rating", "view", "--app-info-id", "info-1", "--fields", "socialMedia,socialMediaAgeRestricted", "--output", "json"},
			wantPath: "/v1/appInfos/info-1/ageRatingDeclaration", response: `{"data":{"type":"ageRatingDeclarations","id":"age-1"}}`,
			wantQuery: map[string]string{"fields[ageRatingDeclarations]": "socialMedia,socialMediaAgeRestricted"},
		},
		{
			name: "xcode cloud product app", args: []string{"xcode-cloud", "products", "app", "--id", "product-1", "--iap-fields", "versions", "--subscription-group-fields", "versions", "--output", "json"},
			wantPath: "/v1/ciProducts/product-1/app", response: `{"data":{"type":"apps","id":"app-1"}}`,
			wantQuery: map[string]string{"fields[inAppPurchases]": "versions", "fields[subscriptionGroups]": "versions", "include": "inAppPurchases,subscriptionGroups"},
		},
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Method != http.MethodGet || req.URL.Path != test.wantPath {
					t.Fatalf("request = %s %s, want GET %s", req.Method, req.URL.Path, test.wantPath)
				}
				query := req.URL.Query()
				for key, want := range test.wantQuery {
					if got := query.Get(key); got != want {
						t.Errorf("query %s = %q, want %q", key, got, want)
					}
				}
				if len(query) != len(test.wantQuery) {
					t.Errorf("query = %v, want exactly %v", query, test.wantQuery)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(test.response)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
		})
	}
}
