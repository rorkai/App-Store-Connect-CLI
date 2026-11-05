package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"slices"
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
		{"apps list iap", []string{"apps", "list", "--iap-fields", "bogus"}, "--iap-fields must be one of: referenceName"},
		{"apps list app info", []string{"apps", "list", "--app-info-fields", "bogus"}, "--app-info-fields must be one of: appStoreState"},
		{"apps view group", []string{"apps", "view", "--id", "app-1", "--subscription-group-fields", "bogus"}, "--subscription-group-fields must be one of: referenceName"},
		{"apps list retention message", []string{"apps", "list", "--retention-message-fields", "bogus"}, "--retention-message-fields must be one of: referenceName"},
		{"app infos list fields", []string{"apps", "info", "list", "--app", "app-1", "--fields", "bogus"}, "--fields must be one of: appStoreState"},
		{"app info view age rating", []string{"apps", "info", "view", "--info-id", "info-1", "--age-rating-fields", "bogus"}, "--age-rating-fields must be one of: advertising"},
		{"app info view unsupported include", []string{"apps", "info", "view", "--info-id", "info-1", "--include", "territoryAgeRatings"}, "--include must be one of: app, ageRatingDeclaration, appInfoLocalizations, primaryCategory"},
		{"app info fields conflict with version", []string{"apps", "info", "view", "--app", "app-1", "--version-id", "version-1", "--fields", "kidsAgeBand"}, "--fields cannot be used with version localization flags"},
		{"app info age rating fields conflict with next", []string{"apps", "info", "view", "--app", "app-1", "--next", "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations?cursor=next", "--age-rating-fields", "socialMedia"}, "--next cannot be combined with --age-rating-fields"},
		{"age rating view", []string{"age-rating", "view", "--app-info-id", "info-1", "--fields", "bogus"}, "--fields must be one of: advertising"},
		{"app info localizations", []string{"localizations", "list", "--type", "app-info", "--app", "app-1", "--app-info", "info-1", "--app-info-fields", "bogus"}, "--app-info-fields must be one of: appStoreState"},
		{"xcode cloud product app", []string{"xcode-cloud", "products", "app", "--id", "product-1", "--iap-fields", "bogus"}, "--iap-fields must be one of: referenceName"},
		{"xcode cloud product app app info", []string{"xcode-cloud", "products", "app", "--id", "product-1", "--app-info-fields", "bogus"}, "--app-info-fields must be one of: appStoreState"},
		{"apps list iap empty", []string{"apps", "list", "--iap-fields", ""}, "--iap-fields must not be empty"},
		{"apps list app info empty", []string{"apps", "list", "--app-info-fields", ""}, "--app-info-fields must not be empty"},
		{"apps list group whitespace", []string{"apps", "list", "--subscription-group-fields", " \t"}, "--subscription-group-fields must not be empty"},
		{"apps view iap whitespace", []string{"apps", "view", "--id", "app-1", "--iap-fields", " \t"}, "--iap-fields must not be empty"},
		{"apps view group empty", []string{"apps", "view", "--id", "app-1", "--subscription-group-fields", ""}, "--subscription-group-fields must not be empty"},
		{"apps view app info empty", []string{"apps", "view", "--id", "app-1", "--app-info-fields", ""}, "--app-info-fields must not be empty"},
		{"app infos list fields empty", []string{"apps", "info", "list", "--app", "app-1", "--fields", ""}, "--fields must not be empty"},
		{"app infos list age rating whitespace", []string{"apps", "info", "list", "--app", "app-1", "--age-rating-fields", " \t"}, "--age-rating-fields must not be empty"},
		{"app info view fields whitespace", []string{"apps", "info", "view", "--info-id", "info-1", "--fields", " \t"}, "--fields must not be empty"},
		{"app info view age rating empty", []string{"apps", "info", "view", "--info-id", "info-1", "--age-rating-fields", ""}, "--age-rating-fields must not be empty"},
		{"age rating view fields empty", []string{"age-rating", "view", "--app-info-id", "info-1", "--fields", ""}, "--fields must not be empty"},
		{"app info localizations fields empty", []string{"localizations", "list", "--type", "app-info", "--app", "app-1", "--app-info", "info-1", "--app-info-fields", ""}, "--app-info-fields must not be empty"},
		{"xcode cloud product app iap empty", []string{"xcode-cloud", "products", "app", "--id", "product-1", "--iap-fields", ""}, "--iap-fields must not be empty"},
		{"xcode cloud product app group whitespace", []string{"xcode-cloud", "products", "app", "--id", "product-1", "--subscription-group-fields", " \t"}, "--subscription-group-fields must not be empty"},
		{"xcode cloud product app app info empty", []string{"xcode-cloud", "products", "app", "--id", "product-1", "--app-info-fields", ""}, "--app-info-fields must not be empty"},
		{"app info localizations next conflict", []string{"localizations", "list", "--type", "app-info", "--app", "app-1", "--next", "https://api.appstoreconnect.apple.com/v1/appInfos/info-1/appInfoLocalizations?cursor=next", "--app-info-fields", ""}, "--next cannot be combined with --app-info-fields"},
		{"app info localizations version type", []string{"localizations", "list", "--version", "version-1", "--app-info-fields", "kidsAgeBand"}, "--app-info-fields requires --type app-info"},
		{"apps list next app info", []string{"apps", "list", "--next", "https://api.appstoreconnect.apple.com/v1/apps?cursor=next", "--app-info-fields", ""}, "--next cannot be combined with --app-info-fields"},
		{"apps list next iap", []string{"apps", "list", "--next", "https://api.appstoreconnect.apple.com/v1/apps?cursor=next", "--iap-fields", ""}, "--next cannot be combined with --iap-fields"},
		{"apps list next group", []string{"apps", "list", "--next", "https://api.appstoreconnect.apple.com/v1/apps?cursor=next", "--subscription-group-fields", ""}, "--next cannot be combined with --subscription-group-fields"},
		{"apps list next retention message", []string{"apps", "list", "--next", "https://api.appstoreconnect.apple.com/v1/apps?cursor=next", "--retention-message-fields", ""}, "--next cannot be combined with --retention-message-fields"},
		{"app info view next fields", []string{"apps", "info", "view", "--app", "app-1", "--next", "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations?cursor=next", "--fields", ""}, "--next cannot be combined with --fields"},
		{"app info view next age rating", []string{"apps", "info", "view", "--app", "app-1", "--next", "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations?cursor=next", "--age-rating-fields", ""}, "--next cannot be combined with --age-rating-fields"},
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
			name: "apps shorthand", args: []string{"apps", "--app-info-fields", "kidsAgeBand", "--output", "json"},
			wantPath: "/v1/apps", response: `{"data":[]}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "include": "appInfos"},
		},
		{
			name: "apps continuation", args: []string{"apps", "list", "--next", "https://api.appstoreconnect.apple.com/v1/apps?fields%5BappInfos%5D=kidsAgeBand&cursor=next", "--output", "json"},
			wantPath: "/v1/apps", response: `{"data":[]}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "cursor": "next"},
		},

		{
			name: "localizations continuation", args: []string{"localizations", "list", "--type", "app-info", "--app", "app-1", "--app-info", "info-1", "--next", "https://api.appstoreconnect.apple.com/v1/appInfos/info-1/appInfoLocalizations?fields%5BappInfos%5D=kidsAgeBand&cursor=next", "--output", "json"},
			wantPath: "/v1/appInfos/info-1/appInfoLocalizations", response: `{"data":[]}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "cursor": "next"},
		},
		{
			name: "apps list", args: []string{"apps", "list", "--app-info-fields", "kidsAgeBand", "--iap-fields", "versions", "--subscription-group-fields", "versions", "--retention-message-fields", "referenceName", "--output", "json"},
			wantPath: "/v1/apps", response: `{"data":[]}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "fields[inAppPurchases]": "versions", "fields[subscriptionGroups]": "versions", "fields[retentionMessages]": "referenceName", "include": "appInfos,inAppPurchases,subscriptionGroups,retentionMessages"},
		},
		{
			name: "apps view", args: []string{"apps", "view", "--id", "app-1", "--app-info-fields", "kidsAgeBand", "--iap-fields", "versions", "--subscription-group-fields", "versions", "--retention-message-fields", "referenceName", "--output", "json"},
			wantPath: "/v1/apps/app-1", response: `{"data":{"type":"apps","id":"app-1"}}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "fields[inAppPurchases]": "versions", "fields[subscriptionGroups]": "versions", "fields[retentionMessages]": "referenceName", "include": "appInfos,inAppPurchases,subscriptionGroups,retentionMessages"},
		},
		{
			name: "app infos list", args: []string{"apps", "info", "list", "--app", "app-1", "--fields", "kidsAgeBand", "--age-rating-fields", "socialMedia,socialMediaAgeRestricted", "--output", "json"},
			wantPath: "/v1/apps/app-1/appInfos", response: `{"data":[]}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand,ageRatingDeclaration", "fields[ageRatingDeclarations]": "socialMedia,socialMediaAgeRestricted", "include": "ageRatingDeclaration"},
		},
		{
			name: "app info view", args: []string{"apps", "info", "view", "--info-id", "info-1", "--fields", "kidsAgeBand", "--age-rating-fields", "socialMedia", "--output", "json"},
			wantPath: "/v1/appInfos/info-1", response: `{"data":{"type":"appInfos","id":"info-1"}}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand,ageRatingDeclaration", "fields[ageRatingDeclarations]": "socialMedia", "include": "ageRatingDeclaration"},
		},
		{
			name: "age rating kids age band remains valid", args: []string{"age-rating", "view", "--app-info-id", "info-1", "--output", "json"},
			wantPath: "/v1/appInfos/info-1/ageRatingDeclaration", response: `{"data":{"type":"ageRatingDeclarations","id":"age-1","attributes":{"kidsAgeBand":"FIVE_AND_UNDER"}}}`,
			wantQuery: map[string]string{},
		},
		{
			name: "age rating view", args: []string{"age-rating", "view", "--app-info-id", "info-1", "--fields", "socialMedia,socialMediaAgeRestricted", "--output", "json"},
			wantPath: "/v1/appInfos/info-1/ageRatingDeclaration", response: `{"data":{"type":"ageRatingDeclarations","id":"age-1"}}`,
			wantQuery: map[string]string{"fields[ageRatingDeclarations]": "socialMedia,socialMediaAgeRestricted"},
		},
		{
			name: "app info localizations", args: []string{"localizations", "list", "--type", "app-info", "--app", "app-1", "--app-info", "info-1", "--app-info-fields", "kidsAgeBand", "--output", "json"},
			wantPath: "/v1/appInfos/info-1/appInfoLocalizations", response: `{"data":[]}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "include": "appInfo"},
		},
		{
			name: "xcode cloud product app", args: []string{"xcode-cloud", "products", "app", "--id", "product-1", "--app-info-fields", "kidsAgeBand", "--iap-fields", "versions", "--subscription-group-fields", "versions", "--retention-message-fields", "referenceName", "--output", "json"},
			wantPath: "/v1/ciProducts/product-1/app", response: `{"data":{"type":"apps","id":"app-1"}}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "fields[inAppPurchases]": "versions", "fields[subscriptionGroups]": "versions", "fields[retentionMessages]": "referenceName", "include": "appInfos,inAppPurchases,subscriptionGroups,retentionMessages"},
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
					got, ok := query[key]
					if !ok {
						t.Errorf("query is missing %s", key)
						continue
					}
					if len(got) != 1 || got[0] != want {
						t.Errorf("query %s = %q, want exactly [%q]", key, got, want)
					}
				}
				if len(query) != len(test.wantQuery) {
					t.Errorf("query = %v, want exactly %v", query, test.wantQuery)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(test.response)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			wantStderr := ""
			if strings.Contains(test.wantQuery["fields[appInfos]"], "kidsAgeBand") {
				wantStderr = "Warning: AppInfo.kidsAgeBand is deprecated and was removed from the API 4.5 schema; use asc age-rating view --app-info-id ID. This selector is still forwarded for compatibility.\n"
			}
			if stderr != wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
			}
			if test.name == "age rating kids age band remains valid" && !strings.Contains(stdout, `"kidsAgeBand":"FIVE_AND_UNDER"`) {
				t.Fatalf("age rating kidsAgeBand lost: %s", stdout)
			}

			if calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
		})
	}
}

// A narrower allowlist rejects values Apple documents, and a value removed from
// the spec leaves the flag unable to express a valid request.
func TestSparseAppFieldFlagsForwardEveryOpenAPIValue(t *testing.T) {
	setupAuth(t)
	raw, err := os.ReadFile("../../../docs/openapi/latest.json")
	if err != nil {
		t.Fatalf("read OpenAPI snapshot: %v", err)
	}
	var spec struct {
		Paths map[string]struct {
			Get struct {
				Parameters []struct {
					Name   string `json:"name"`
					Schema struct {
						Items struct {
							Enum []string `json:"enum"`
						} `json:"items"`
					} `json:"schema"`
				} `json:"parameters"`
			} `json:"get"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse OpenAPI snapshot: %v", err)
	}

	const (
		appInfos   = "fields[appInfos]"
		ageRatings = "fields[ageRatingDeclarations]"
		iaps       = "fields[inAppPurchases]"
		groups     = "fields[subscriptionGroups]"
		retention  = "fields[retentionMessages]"
		list       = `{"data":[]}`
		app        = `{"data":{"type":"apps","id":"app-1"}}`
		appInfo    = `{"data":{"type":"appInfos","id":"info-1"}}`
		ageRating  = `{"data":{"type":"ageRatingDeclarations","id":"age-1"}}`
	)
	appsList := []string{"apps", "list"}
	appsView := []string{"apps", "view", "--id", "app-1"}
	infosList := []string{"apps", "info", "list", "--app", "app-1"}
	infoView := []string{"apps", "info", "view", "--info-id", "info-1"}
	ageRatingView := []string{"age-rating", "view", "--app-info-id", "info-1"}
	localizations := []string{"localizations", "list", "--type", "app-info", "--app", "app-1", "--app-info", "info-1"}
	productApp := []string{"xcode-cloud", "products", "app", "--id", "product-1"}
	tests := []struct {
		command             []string
		flagName, parameter string
		specPath, path      string
		response            string
	}{
		{appsList, "--app-info-fields", appInfos, "/v1/apps", "/v1/apps", list},
		{appsList, "--iap-fields", iaps, "/v1/apps", "/v1/apps", list},
		{appsList, "--subscription-group-fields", groups, "/v1/apps", "/v1/apps", list},
		{appsList, "--retention-message-fields", retention, "/v1/apps", "/v1/apps", list},
		{appsView, "--app-info-fields", appInfos, "/v1/apps/{id}", "/v1/apps/app-1", app},
		{appsView, "--iap-fields", iaps, "/v1/apps/{id}", "/v1/apps/app-1", app},
		{appsView, "--subscription-group-fields", groups, "/v1/apps/{id}", "/v1/apps/app-1", app},
		{appsView, "--retention-message-fields", retention, "/v1/apps/{id}", "/v1/apps/app-1", app},
		{infosList, "--fields", appInfos, "/v1/apps/{id}/appInfos", "/v1/apps/app-1/appInfos", list},
		{infosList, "--age-rating-fields", ageRatings, "/v1/apps/{id}/appInfos", "/v1/apps/app-1/appInfos", list},
		{infoView, "--fields", appInfos, "/v1/appInfos/{id}", "/v1/appInfos/info-1", appInfo},
		{infoView, "--age-rating-fields", ageRatings, "/v1/appInfos/{id}", "/v1/appInfos/info-1", appInfo},
		{ageRatingView, "--fields", ageRatings, "/v1/appInfos/{id}/ageRatingDeclaration", "/v1/appInfos/info-1/ageRatingDeclaration", ageRating},
		{localizations, "--app-info-fields", appInfos, "/v1/appInfos/{id}/appInfoLocalizations", "/v1/appInfos/info-1/appInfoLocalizations", list},
		{productApp, "--app-info-fields", appInfos, "/v1/ciProducts/{id}/app", "/v1/ciProducts/product-1/app", app},
		{productApp, "--iap-fields", iaps, "/v1/ciProducts/{id}/app", "/v1/ciProducts/product-1/app", app},
		{productApp, "--subscription-group-fields", groups, "/v1/ciProducts/{id}/app", "/v1/ciProducts/product-1/app", app},
		{productApp, "--retention-message-fields", retention, "/v1/ciProducts/{id}/app", "/v1/ciProducts/product-1/app", app},
	}

	for _, test := range tests {
		t.Run(test.specPath+" "+test.flagName, func(t *testing.T) {
			var want []string
			for _, parameter := range spec.Paths[test.specPath].Get.Parameters {
				if parameter.Name == test.parameter {
					want = slices.Sorted(slices.Values(parameter.Schema.Items.Enum))
				}
			}
			if len(want) == 0 {
				t.Fatalf("OpenAPI snapshot has no %s on GET %s", test.parameter, test.specPath)
			}

			calls := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.URL.Path != test.path {
					t.Fatalf("path = %s, want %s", req.URL.Path, test.path)
				}
				got := slices.Sorted(slices.Values(strings.Split(req.URL.Query().Get(test.parameter), ",")))
				if !slices.Equal(got, want) {
					t.Errorf("%s = %v, want %v", test.parameter, got, want)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(test.response)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
			}))

			args := append(slices.Clone(test.command), test.flagName, strings.Join(want, ","), "--output", "json")
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Errorf("run error: %v", err)
				}
			})
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			if calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
		})
	}
}
