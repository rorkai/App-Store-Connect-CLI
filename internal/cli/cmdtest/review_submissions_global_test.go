package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewSubmissionsListSelectorRequests(t *testing.T) {
	tests := []struct {
		name       string
		envAppID   string
		args       []string
		wantPath   string
		wantQuery  map[string]string
		body       string
		wantStdout string
	}{
		{
			name:       "global with app",
			args:       []string{"review", "submissions-list", "--global", "--app", "app-1"},
			wantPath:   "/v1/reviewSubmissions",
			wantQuery:  map[string]string{"filter[app]": "app-1"},
			body:       `{"data":[{"type":"reviewSubmissions","id":"rs-1","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW"}}],"links":{"self":"/v1/reviewSubmissions"}}`,
			wantStdout: `"id":"rs-1"`,
		},
		{
			name:       "global with ASC_APP_ID",
			envAppID:   "app-from-env",
			args:       []string{"review", "submissions-list", "--global"},
			wantPath:   "/v1/reviewSubmissions",
			wantQuery:  map[string]string{"filter[app]": "app-from-env"},
			body:       `{"data":[],"links":{"self":"/v1/reviewSubmissions"}}`,
			wantStdout: `"data":[]`,
		},
		{
			name:     "global with filters",
			args:     []string{"review", "submissions-list", "--global", "--app", "app-1", "--platform", "IOS", "--state", "READY_FOR_REVIEW"},
			wantPath: "/v1/reviewSubmissions",
			wantQuery: map[string]string{
				"filter[app]":      "app-1",
				"filter[platform]": "IOS",
				"filter[state]":    "READY_FOR_REVIEW",
			},
			body: `{"data":[],"links":{"self":"/v1/reviewSubmissions"}}`,
		},
		{
			name:       "app scoped",
			args:       []string{"review", "submissions-list", "--app", "app-1"},
			wantPath:   "/v1/apps/app-1/reviewSubmissions",
			body:       `{"data":[{"type":"reviewSubmissions","id":"rs-scoped","attributes":{"platform":"IOS"}}],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`,
			wantStdout: `"id":"rs-scoped"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", test.envAppID)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != test.wantPath {
					t.Fatalf("request = %s %s, want GET %s", req.Method, req.URL.Path, test.wantPath)
				}
				for key, want := range test.wantQuery {
					if got := req.URL.Query().Get(key); got != want {
						t.Fatalf("%s = %q, want %q", key, got, want)
					}
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(test.body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
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
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("expected stdout to contain %q, got %q", test.wantStdout, stdout)
			}
		})
	}
}

func TestReviewSubmissionsItemFieldsAutomaticallyIncludeItems(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		path       string
		wantFields string
	}{
		{
			name:       "global list",
			args:       []string{"review", "submissions-list", "--global", "--app", "app-1", "--item-fields", "subscriptionVersion"},
			path:       "/v1/reviewSubmissions",
			wantFields: "subscriptionVersion",
		},
		{
			name:       "detail",
			args:       []string{"review", "submissions-get", "--id", "submission-1", "--item-fields", "subscriptionGroupVersion"},
			path:       "/v1/reviewSubmissions/submission-1",
			wantFields: "subscriptionGroupVersion",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					t.Fatalf("request = %s %s, want GET %s", req.Method, req.URL.Path, test.path)
				}
				if got := req.URL.Query().Get("include"); got != "items" {
					t.Fatalf("include = %q, want items", got)
				}
				if got := req.URL.Query().Get("fields[reviewSubmissionItems]"); got != test.wantFields {
					t.Fatalf("fields[reviewSubmissionItems] = %q, want %q", got, test.wantFields)
				}
				body := `{"data":{"type":"reviewSubmissions","id":"submission-1"}}`
				if strings.Contains(test.name, "list") {
					body = `{"data":[],"links":{"self":"/v1/reviewSubmissions"}}`
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
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
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestReviewItemsVersionFieldsAutomaticallyIncludeRelatedResources(t *testing.T) {
	tests := []struct {
		name           string
		flag           string
		field          string
		itemFields     string
		include        string
		wantInclude    string
		wantQueryName  string
		wantItemFields string
	}{
		{
			name:           "in-app purchase version",
			flag:           "--iap-version-fields",
			field:          "inAppPurchase",
			itemFields:     "state",
			wantInclude:    "inAppPurchaseVersion",
			wantQueryName:  "fields[inAppPurchaseVersions]",
			wantItemFields: "state,inAppPurchaseVersion",
		},
		{
			name:           "subscription version",
			flag:           "--subscription-version-fields",
			field:          "subscription",
			itemFields:     "state",
			wantInclude:    "subscriptionVersion",
			wantQueryName:  "fields[subscriptionVersions]",
			wantItemFields: "state,subscriptionVersion",
		},
		{
			name:           "subscription group version",
			flag:           "--subscription-group-version-fields",
			field:          "subscriptionGroup",
			itemFields:     "state",
			wantInclude:    "subscriptionGroupVersion",
			wantQueryName:  "fields[subscriptionGroupVersions]",
			wantItemFields: "state,subscriptionGroupVersion",
		},
		{
			name:           "without item sparse fields",
			flag:           "--subscription-version-fields",
			field:          "version",
			wantInclude:    "subscriptionVersion",
			wantQueryName:  "fields[subscriptionVersions]",
			wantItemFields: "",
		},
		{
			name:           "preserves explicit includes without duplicates",
			flag:           "--iap-version-fields",
			field:          "version",
			itemFields:     "state",
			include:        "appStoreVersion,inAppPurchaseVersion",
			wantInclude:    "appStoreVersion,inAppPurchaseVersion",
			wantQueryName:  "fields[inAppPurchaseVersions]",
			wantItemFields: "state,inAppPurchaseVersion",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/reviewSubmissions/submission-1/items" {
					t.Fatalf("request = %s %s, want GET /v1/reviewSubmissions/submission-1/items", req.Method, req.URL.Path)
				}
				if got := req.URL.Query().Get("include"); got != test.wantInclude {
					t.Fatalf("include = %q, want %q", got, test.wantInclude)
				}
				if got := req.URL.Query().Get(test.wantQueryName); got != test.field {
					t.Fatalf("%s = %q, want %q", test.wantQueryName, got, test.field)
				}
				if got := req.URL.Query().Get("fields[reviewSubmissionItems]"); got != test.wantItemFields {
					t.Fatalf("fields[reviewSubmissionItems] = %q, want %q", got, test.wantItemFields)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"links":{"self":"/v1/reviewSubmissions/submission-1/items"}}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			_, stderr := captureOutput(t, func() {
				args := []string{"review", "items", "list", "--submission", "submission-1"}
				if test.itemFields != "" {
					args = append(args, "--fields", test.itemFields)
				}
				if test.include != "" {
					args = append(args, "--include", test.include)
				}
				args = append(args, test.flag, test.field)
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestReviewSubmissionsListGlobalPaginate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.URL.Path != "/v1/reviewSubmissions" {
				t.Fatalf("expected path /v1/reviewSubmissions, got %s", req.URL.Path)
			}
			if req.URL.Query().Get("filter[app]") != "app-1" {
				t.Fatalf("expected filter[app]=app-1, got %q", req.URL.Query().Get("filter[app]"))
			}
			body := `{"data":[{"type":"reviewSubmissions","id":"rs-1","attributes":{"platform":"IOS"}}],"links":{"self":"/v1/reviewSubmissions","next":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions?cursor=page2"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			body := `{"data":[{"type":"reviewSubmissions","id":"rs-2","attributes":{"platform":"MAC_OS"}}],"links":{"self":"/v1/reviewSubmissions"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "submissions-list", "--global", "--app", "app-1", "--paginate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"rs-1"`) {
		t.Fatalf("expected first page data in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"rs-2"`) {
		t.Fatalf("expected second page data in output, got %q", stdout)
	}
}
