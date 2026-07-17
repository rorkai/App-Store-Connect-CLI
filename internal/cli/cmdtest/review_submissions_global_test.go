package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewSubmissionsListGlobalSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/reviewSubmissions" {
			t.Fatalf("expected path /v1/reviewSubmissions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("filter[app]") != "app-1" {
			t.Fatalf("expected filter[app]=app-1, got %q", req.URL.Query().Get("filter[app]"))
		}
		body := `{"data":[{"type":"reviewSubmissions","id":"rs-1","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "submissions-list", "--global", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"rs-1"`) {
		t.Fatalf("expected submission id in output, got %q", stdout)
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
					body = `{"data":[]}`
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
		name          string
		flag          string
		field         string
		wantInclude   string
		wantQueryName string
		wantItemField string
	}{
		{
			name:          "in-app purchase version",
			flag:          "--iap-version-fields",
			field:         "inAppPurchase",
			wantInclude:   "inAppPurchaseVersion",
			wantQueryName: "fields[inAppPurchaseVersions]",
			wantItemField: "inAppPurchaseVersion",
		},
		{
			name:          "subscription version",
			flag:          "--subscription-version-fields",
			field:         "subscription",
			wantInclude:   "subscriptionVersion",
			wantQueryName: "fields[subscriptionVersions]",
			wantItemField: "subscriptionVersion",
		},
		{
			name:          "subscription group version",
			flag:          "--subscription-group-version-fields",
			field:         "subscriptionGroup",
			wantInclude:   "subscriptionGroupVersion",
			wantQueryName: "fields[subscriptionGroupVersions]",
			wantItemField: "subscriptionGroupVersion",
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
				if got, want := req.URL.Query().Get("fields[reviewSubmissionItems]"), "state,"+test.wantItemField; got != want {
					t.Fatalf("fields[reviewSubmissionItems] = %q, want %q", got, want)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"review", "items", "list", "--submission", "submission-1", "--fields", "state", test.flag, test.field}); err != nil {
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

func TestReviewSubmissionsListGlobalWithASCAppIDSet(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "app-from-env")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/reviewSubmissions" {
			t.Fatalf("expected global path /v1/reviewSubmissions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("filter[app]") != "app-from-env" {
			t.Fatalf("expected filter[app]=app-from-env, got %q", req.URL.Query().Get("filter[app]"))
		}
		body := `{"data":[]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "submissions-list", "--global"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("expected empty data output, got %q", stdout)
	}
}

func TestReviewSubmissionsListGlobalWithFilters(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/reviewSubmissions" {
			t.Fatalf("expected path /v1/reviewSubmissions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[app]") != "app-1" {
			t.Fatalf("expected filter[app]=app-1, got %q", values.Get("filter[app]"))
		}
		if values.Get("filter[platform]") != "IOS" {
			t.Fatalf("expected filter[platform]=IOS, got %q", values.Get("filter[platform]"))
		}
		if values.Get("filter[state]") != "READY_FOR_REVIEW" {
			t.Fatalf("expected filter[state]=READY_FOR_REVIEW, got %q", values.Get("filter[state]"))
		}
		body := `{"data":[]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "submissions-list", "--global", "--app", "app-1", "--platform", "IOS", "--state", "READY_FOR_REVIEW"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestReviewSubmissionsListGlobalMissingAppError(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "submissions-list", "--global"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "Error: --app is required with --global") {
		t.Fatalf("expected missing app for global error, got %q", stderr)
	}
}

func TestReviewSubmissionsListMissingSelectorError(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "submissions-list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "Error: --app or --global is required") {
		t.Fatalf("expected missing selector error, got %q", stderr)
	}
}

func TestReviewSubmissionsListScopedStillWorks(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/apps/app-1/reviewSubmissions" {
			t.Fatalf("expected path /v1/apps/app-1/reviewSubmissions, got %s", req.URL.Path)
		}
		body := `{"data":[{"type":"reviewSubmissions","id":"rs-scoped","attributes":{"platform":"IOS"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "submissions-list", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"rs-scoped"`) {
		t.Fatalf("expected scoped submission in output, got %q", stdout)
	}
}

func TestReviewSubmissionsListNextSkipsSelector(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	nextURL := "https://api.appstoreconnect.apple.com/v1/reviewSubmissions?cursor=page2"
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != nextURL {
			t.Fatalf("expected next URL %q, got %q", nextURL, req.URL.String())
		}
		body := `{"data":[]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "submissions-list", "--next", nextURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
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
			body := `{"data":[{"type":"reviewSubmissions","id":"rs-1","attributes":{"platform":"IOS"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions?cursor=page2"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			body := `{"data":[{"type":"reviewSubmissions","id":"rs-2","attributes":{"platform":"MAC_OS"}}]}`
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
