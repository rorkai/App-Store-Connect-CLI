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
		{"apps list app info", []string{"apps", "list", "--app-info-fields", "state"}, "--app-info-fields must be one of: kidsAgeBand"},
		{"apps view group", []string{"apps", "view", "--id", "app-1", "--subscription-group-fields", "subscriptions"}, "--subscription-group-fields must be one of: versions"},
		{"app infos list fields", []string{"apps", "info", "list", "--app", "app-1", "--fields", "state"}, "--fields must be one of: kidsAgeBand"},
		{"app info view age rating", []string{"apps", "info", "view", "--info-id", "info-1", "--age-rating-fields", "gambling"}, "--age-rating-fields must be one of"},
		{"app info fields conflict with version", []string{"apps", "info", "view", "--app", "app-1", "--version-id", "version-1", "--fields", "kidsAgeBand"}, "--fields cannot be used with version localization flags"},
		{"app info age rating fields conflict with next", []string{"apps", "info", "view", "--app", "app-1", "--next", "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations?cursor=next", "--age-rating-fields", "socialMedia"}, "--next cannot be combined with --age-rating-fields"},
		{"age rating view", []string{"age-rating", "view", "--app-info-id", "info-1", "--fields", "gambling"}, "--fields must be one of"},
		{"app info localizations", []string{"localizations", "list", "--type", "app-info", "--app", "app-1", "--app-info", "info-1", "--app-info-fields", "state"}, "--app-info-fields must be one of: kidsAgeBand"},
		{"xcode cloud product app", []string{"xcode-cloud", "products", "app", "--id", "product-1", "--iap-fields", "name"}, "--iap-fields must be one of: versions"},
		{"xcode cloud product app app info", []string{"xcode-cloud", "products", "app", "--id", "product-1", "--app-info-fields", "state"}, "--app-info-fields must be one of: kidsAgeBand"},
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

func TestSparseAppFieldFlagsRejectExplicitEmptyBeforeAuth(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	tests := []struct {
		name string
		args []string
		want string
	}{
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); !errors.Is(err, flag.ErrHelp) {
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

func TestAppInfoLocalizationSparseFieldsRejectNextAndOtherTypesBeforeAuth(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/appInfos/info-1/appInfoLocalizations?cursor=next"
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "next conflict",
			args: []string{"localizations", "list", "--type", "app-info", "--app", "app-1", "--next", next, "--app-info-fields", ""},
			want: "--next cannot be combined with --app-info-fields",
		},
		{
			name: "version type",
			args: []string{"localizations", "list", "--version", "version-1", "--app-info-fields", "kidsAgeBand"},
			want: "--app-info-fields requires --type app-info",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); !errors.Is(err, flag.ErrHelp) {
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
	for _, flagName := range []string{"--app-info-fields", "--iap-fields", "--subscription-group-fields"} {
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

func TestAppInfoSparseFieldsConflictWithNextBeforeAuth(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations?cursor=next"
	for _, flagName := range []string{"--fields", "--age-rating-fields"} {
		t.Run(flagName, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"apps", "info", "view", "--app", "app-1", "--next", next, flagName, ""}); err != nil {
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
			name: "apps list", args: []string{"apps", "list", "--app-info-fields", "kidsAgeBand", "--iap-fields", "versions", "--subscription-group-fields", "versions", "--output", "json"},
			wantPath: "/v1/apps", response: `{"data":[]}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "fields[inAppPurchases]": "versions", "fields[subscriptionGroups]": "versions", "include": "appInfos,inAppPurchases,subscriptionGroups"},
		},
		{
			name: "apps view", args: []string{"apps", "view", "--id", "app-1", "--app-info-fields", "kidsAgeBand", "--iap-fields", "versions", "--subscription-group-fields", "versions", "--output", "json"},
			wantPath: "/v1/apps/app-1", response: `{"data":{"type":"apps","id":"app-1"}}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "fields[inAppPurchases]": "versions", "fields[subscriptionGroups]": "versions", "include": "appInfos,inAppPurchases,subscriptionGroups"},
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
			name: "app info localizations", args: []string{"localizations", "list", "--type", "app-info", "--app", "app-1", "--app-info", "info-1", "--app-info-fields", "kidsAgeBand", "--output", "json"},
			wantPath: "/v1/appInfos/info-1/appInfoLocalizations", response: `{"data":[]}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "include": "appInfo"},
		},
		{
			name: "xcode cloud product app", args: []string{"xcode-cloud", "products", "app", "--id", "product-1", "--app-info-fields", "kidsAgeBand", "--iap-fields", "versions", "--subscription-group-fields", "versions", "--output", "json"},
			wantPath: "/v1/ciProducts/product-1/app", response: `{"data":{"type":"apps","id":"app-1"}}`,
			wantQuery: map[string]string{"fields[appInfos]": "kidsAgeBand", "fields[inAppPurchases]": "versions", "fields[subscriptionGroups]": "versions", "include": "appInfos,inAppPurchases,subscriptionGroups"},
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

func TestSparseAppFieldHelp441(t *testing.T) {
	root := RootCommand("1.2.3")
	for _, path := range [][]string{
		{"apps", "list"},
		{"apps", "view"},
		{"xcode-cloud", "products", "app"},
	} {
		command := findSubcommand(root, path...)
		if command == nil {
			t.Fatalf("command %v not found", path)
		}
		usage := command.UsageFunc(command)
		for _, flagName := range []string{"--app-info-fields", "--iap-fields", "--subscription-group-fields"} {
			if !strings.Contains(usage, flagName) {
				t.Errorf("help for %v does not contain %s: %q", path, flagName, usage)
			}
		}
	}

	localizations := findSubcommand(root, "localizations", "list")
	if localizations == nil {
		t.Fatal("command [localizations list] not found")
	}
	if usage := localizations.UsageFunc(localizations); !strings.Contains(usage, "--app-info-fields") {
		t.Fatalf("help for [localizations list] does not contain --app-info-fields: %q", usage)
	}
}
