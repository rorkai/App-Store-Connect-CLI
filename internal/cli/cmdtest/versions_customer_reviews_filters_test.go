package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const versionCustomerReviewsPayload = `{"data":[{"type":"customerReviews","id":"review-1","attributes":{"rating":2,"title":"Slow","body":"Laggy","reviewerNickname":"Tester","createdDate":"2026-01-20T00:00:00Z","territory":"USA"}}]}`

const (
	reviewFieldsUsage          = "--fields must be a comma-separated list of: rating,title,body,reviewerNickname,createdDate,territory,response,reviewTerritory"
	reviewIncludeUsage         = "--include must be a comma-separated list of: response,reviewTerritory"
	reviewTerritoryFieldsUsage = "--territory-fields must be a comma-separated list of: currency"
)

func TestVersionsCustomerReviewsListEmitsReviewFilters(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/customerReviews" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		query := req.URL.Query()
		for _, want := range []struct {
			key   string
			value string
		}{
			{key: "filter[rating]", value: "1,2"},
			{key: "filter[territory]", value: "USA,GBR"},
			{key: "sort", value: "-createdDate"},
			{key: "exists[publishedResponse]", value: "false"},
			{key: "include", value: "response"},
			{key: "fields[customerReviewResponses]", value: "responseBody,state"},
			{key: "limit", value: "5"},
		} {
			if got := query.Get(want.key); got != want.value {
				t.Errorf("%s = %q, want %q (raw query %q)", want.key, got, want.value, req.URL.RawQuery)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, versionCustomerReviewsPayload)
	}))
	t.Cleanup(server.Close)

	client := newReviewTestServerClient(t, server)
	restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"versions", "customer-reviews", "list",
			"--version-id", "version-1",
			"--stars", "1,2",
			"--territory", "us,GB",
			"--sort", "-createdDate",
			"--response-state", "unreplied",
			"--response-fields", "responseBody,state",
			"--limit", "5",
			"--output", "json",
		}, "1.2.3")
		if code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"review-1"`) {
		t.Fatalf("expected review envelope on stdout, got %q", stdout)
	}
	if requests != 1 {
		t.Fatalf("request count = %d, want 1", requests)
	}
}

func TestVersionsCustomerReviewsListOnlyUnrespondedAndIncludeResponse(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		query := req.URL.Query()
		if got := query.Get("exists[publishedResponse]"); got != "false" {
			t.Errorf("exists[publishedResponse] = %q, want false", got)
		}
		if got := query.Get("include"); got != "response" {
			t.Errorf("include = %q, want response", got)
		}
		if _, ok := query["fields[customerReviewResponses]"]; ok {
			t.Errorf("fields[customerReviewResponses] must be absent, got %q", req.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, versionCustomerReviewsPayload)
	}))
	t.Cleanup(server.Close)

	client := newReviewTestServerClient(t, server)
	restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	_, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"versions", "customer-reviews", "list",
			"--version-id", "version-1",
			"--only-unresponded",
			"--include-response",
			"--output", "json",
		}, "1.2.3")
		if code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestVersionsCustomerReviewsListRejectsInvalidStars(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, versionCustomerReviewsPayload)
	}))
	t.Cleanup(server.Close)

	client := newReviewTestServerClient(t, server)
	restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"versions", "customer-reviews", "list",
			"--version-id", "version-1",
			"--stars", "9",
			"--output", "json",
		}, "1.2.3")
		if code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--stars must be a comma-separated list of star ratings: 1, 2, 3, 4, 5") {
		t.Fatalf("expected --stars guidance listing valid values, got %q", stderr)
	}
	if requests != 0 {
		t.Fatalf("request count = %d, want 0 (validation must precede the request)", requests)
	}
}

// The version-scoped listing shares Apple's customer review filter surface with
// the app-level listing, so identical bad input must fail identically on both.
func TestVersionsCustomerReviewsFilterValidationMatchesAppLevelReviews(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantMessage string
	}{
		{
			name:        "explicitly empty star rating",
			args:        []string{"--stars", ""},
			wantMessage: "--stars must be a comma-separated list of star ratings: 1, 2, 3, 4, 5",
		},
		{
			name:        "explicitly empty territory",
			args:        []string{"--territory", ""},
			wantMessage: "--territory must be a valid App Store territory code",
		},
		{
			name:        "explicitly empty sort",
			args:        []string{"--sort", ""},
			wantMessage: "--sort must be one of: rating, -rating, createdDate, -createdDate",
		},
		{
			name:        "explicitly empty response state",
			args:        []string{"--response-state", ""},
			wantMessage: "--response-state must be one of: any, unresponded, unreplied, responded, replied",
		},
		{
			name:        "whitespace response fields",
			args:        []string{"--response-fields", "   "},
			wantMessage: "--response-fields must be a comma-separated list of: responseBody,lastModifiedDate,state,review",
		},
		{
			name:        "star rating out of range",
			args:        []string{"--stars", "9"},
			wantMessage: "--stars must be a comma-separated list of star ratings: 1, 2, 3, 4, 5",
		},
		{
			name:        "star rating not numeric",
			args:        []string{"--stars", "five"},
			wantMessage: "--stars must be a comma-separated list of star ratings: 1, 2, 3, 4, 5",
		},
		{
			name:        "unsupported sort",
			args:        []string{"--sort", "bogus"},
			wantMessage: "--sort must be one of: rating, -rating, createdDate, -createdDate",
		},
		{
			name:        "unsupported response state",
			args:        []string{"--response-state", "maybe"},
			wantMessage: "--response-state must be one of: any, unresponded, unreplied, responded, replied",
		},
		{
			name:        "unsupported response fields",
			args:        []string{"--response-fields", "bogus"},
			wantMessage: "--response-fields must be a comma-separated list of: responseBody,lastModifiedDate,state,review",
		},
		{
			name:        "unsupported territory",
			args:        []string{"--territory", "not-a-territory"},
			wantMessage: "--territory must be a valid App Store territory code",
		},
		{
			name:        "response fields with a repeated comma",
			args:        []string{"--response-fields", "responseBody,,state"},
			wantMessage: "--response-fields must be a comma-separated list of: responseBody,lastModifiedDate,state,review",
		},
		{
			name:        "response fields with no usable elements",
			args:        []string{"--response-fields", ","},
			wantMessage: "--response-fields must be a comma-separated list of: responseBody,lastModifiedDate,state,review",
		},
		{
			name:        "conflicting response filters",
			args:        []string{"--only-unresponded", "--response-state", "responded"},
			wantMessage: "--only-unresponded cannot be combined with --response-state responded",
		},
		{
			name:        "repeated star filters",
			args:        []string{"--stars", "1", "--stars", "2"},
			wantMessage: `--stars specified multiple times; pass one comma-separated list, for example --stars "1,2"`,
		},
		{
			name:        "repeated territory filters",
			args:        []string{"--territory", "USA", "--territory", "GBR"},
			wantMessage: `--territory specified multiple times; pass one comma-separated list, for example --territory "USA,GBR"`,
		},
		{
			name:        "explicitly empty review territory",
			args:        []string{"--review-territory", ""},
			wantMessage: "--review-territory must be a valid App Store territory code",
		},
		{
			name:        "unsupported review territory",
			args:        []string{"--review-territory", "USA,not-a-territory"},
			wantMessage: "--review-territory must be a valid App Store territory code",
		},
		{
			name:        "review territory with a trailing comma",
			args:        []string{"--review-territory", "USA,"},
			wantMessage: "--review-territory must be a valid App Store territory code",
		},
		{
			name:        "repeated review territory filters",
			args:        []string{"--review-territory", "USA", "--review-territory", "GBR"},
			wantMessage: `--review-territory specified multiple times; pass one comma-separated list, for example --review-territory "USA,GBR"`,
		},
		{
			name:        "unsupported fields",
			args:        []string{"--fields", "rating,bogus"},
			wantMessage: reviewFieldsUsage,
		},
		{
			name:        "explicitly empty fields",
			args:        []string{"--fields", ""},
			wantMessage: reviewFieldsUsage,
		},
		{
			name:        "unsupported include",
			args:        []string{"--include", "territory"},
			wantMessage: reviewIncludeUsage,
		},
		{
			name:        "include with a repeated comma",
			args:        []string{"--include", "response,,reviewTerritory"},
			wantMessage: reviewIncludeUsage,
		},
		{
			name:        "unsupported territory fields",
			args:        []string{"--territory-fields", "name"},
			wantMessage: reviewTerritoryFieldsUsage,
		},
		{
			name:        "whitespace territory fields",
			args:        []string{"--territory-fields", "   "},
			wantMessage: reviewTerritoryFieldsUsage,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appCode, appStderr := runReviewFilterFailure(t, append([]string{"reviews", "list", "--app", "app-1"}, test.args...))
			versionCode, versionStderr := runReviewFilterFailure(t, append([]string{"versions", "customer-reviews", "list", "--version-id", "version-1"}, test.args...))

			if appCode == rootcmd.ExitSuccess {
				t.Fatalf("app-level reviews list unexpectedly succeeded")
			}
			if versionCode != appCode {
				t.Fatalf("version-scoped exit code = %d, app-level exit code = %d; they must match", versionCode, appCode)
			}
			if !strings.Contains(appStderr, test.wantMessage) {
				t.Fatalf("app-level stderr = %q, want it to contain %q", appStderr, test.wantMessage)
			}
			if !strings.Contains(versionStderr, test.wantMessage) {
				t.Fatalf("version-scoped stderr = %q, want it to contain %q", versionStderr, test.wantMessage)
			}
		})
	}
}

func TestCustomerReviewListsRejectQueryFlagsWithNext(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/customerReviews?cursor=next"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "version stars",
			args:    []string{"versions", "customer-reviews", "list", "--next", nextURL, "--stars", "1"},
			wantErr: "--next cannot be combined with --stars",
		},
		{
			name:    "app response fields",
			args:    []string{"reviews", "list", "--next", nextURL, "--response-fields", "responseBody"},
			wantErr: "--next cannot be combined with --response-fields",
		},
		{
			name:    "explicit empty territory",
			args:    []string{"versions", "customer-reviews", "list", "--next", nextURL, "--territory", ""},
			wantErr: "--next cannot be combined with --territory",
		},
		{
			name:    "app review territory",
			args:    []string{"reviews", "list", "--next", nextURL, "--review-territory", "USA"},
			wantErr: "--next cannot be combined with --review-territory",
		},
		{
			name:    "version fields",
			args:    []string{"versions", "customer-reviews", "list", "--next", nextURL, "--fields", "rating"},
			wantErr: "--next cannot be combined with --fields",
		},
		{
			name:    "app include",
			args:    []string{"reviews", "--next", nextURL, "--include", "reviewTerritory"},
			wantErr: "--next cannot be combined with --include",
		},
		{
			name:    "version territory fields",
			args:    []string{"versions", "customer-reviews", "list", "--next", nextURL, "--territory-fields", "currency"},
			wantErr: "--next cannot be combined with --territory-fields",
		},
		{
			name:    "explicit zero limit",
			args:    []string{"reviews", "--next", nextURL, "--limit", "0"},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "version owner",
			args:    []string{"versions", "customer-reviews", "list", "--next", nextURL, "--version-id", "version-1"},
			wantErr: "--next cannot be combined with --version-id",
		},
		{
			name:    "app owner",
			args:    []string{"reviews", "list", "--next", nextURL, "--app", "app-1"},
			wantErr: "--next cannot be combined with --app",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, stderr := runReviewFilterFailure(t, test.args)
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantErr)
			}
		})
	}
}

// Every customer review read shares Apple's selection surface, and the older
// --territory filter must stay on filter[territory] next to the new one.
func TestCustomerReviewReadsSendRequestedQuery(t *testing.T) {
	const listPayload = `{"data":[]}`
	const viewPayload = `{"data":{"type":"customerReviews","id":"review-1","attributes":{"rating":5}}}`
	selection := []string{
		"--fields", "rating,reviewTerritory",
		"--include", "response,reviewTerritory",
		"--response-fields", "responseBody",
		"--territory-fields", "currency",
	}
	selectionQuery := url.Values{
		"fields[customerReviews]":         {"rating,reviewTerritory"},
		"include":                         {"response,reviewTerritory"},
		"fields[customerReviewResponses]": {"responseBody"},
		"fields[territories]":             {"currency"},
	}
	withFilters := func(values url.Values) url.Values {
		merged := url.Values{"filter[territory]": {"USA"}, "filter[reviewTerritory]": {"USA,GBR"}}
		for key, value := range values {
			merged[key] = value
		}
		return merged
	}
	filters := []string{"--territory", "US", "--review-territory", "us,GBR"}

	tests := []struct {
		name     string
		args     []string
		path     string
		payload  string
		wantRead url.Values
	}{
		{
			name:     "app list",
			args:     append(append([]string{"reviews", "list", "--app", "app-1"}, filters...), selection...),
			path:     "/v1/apps/app-1/customerReviews",
			payload:  listPayload,
			wantRead: withFilters(selectionQuery),
		},
		{
			name:     "reviews root list",
			args:     append(append([]string{"reviews", "--app", "app-1"}, filters...), selection...),
			path:     "/v1/apps/app-1/customerReviews",
			payload:  listPayload,
			wantRead: withFilters(selectionQuery),
		},
		{
			name:     "version list",
			args:     append(append([]string{"versions", "customer-reviews", "list", "--version-id", "version-1"}, filters...), selection...),
			path:     "/v1/appStoreVersions/version-1/customerReviews",
			payload:  listPayload,
			wantRead: withFilters(selectionQuery),
		},
		{
			name:     "view",
			args:     append([]string{"reviews", "view", "--id", "review-1"}, selection...),
			path:     "/v1/customerReviews/review-1",
			payload:  viewPayload,
			wantRead: selectionQuery,
		},
		{
			name:     "app list without new flags keeps the older territory filter alone",
			args:     []string{"reviews", "list", "--app", "app-1", "--territory", "US"},
			path:     "/v1/apps/app-1/customerReviews",
			payload:  listPayload,
			wantRead: url.Values{"filter[territory]": {"USA"}},
		},
		{
			name:     "view without flags sends no query",
			args:     []string{"reviews", "view", "--id", "review-1"},
			path:     "/v1/customerReviews/review-1",
			payload:  viewPayload,
			wantRead: url.Values{},
		},
		{
			name: "field flags imply their include once",
			args: []string{
				"reviews", "list", "--app", "app-1", "--include-response", "--include", "response",
				"--response-fields", "state", "--territory-fields", "currency",
			},
			path:    "/v1/apps/app-1/customerReviews",
			payload: listPayload,
			wantRead: url.Values{
				"include":                         {"response,reviewTerritory"},
				"fields[customerReviewResponses]": {"state"},
				"fields[territories]":             {"currency"},
			},
		},
		{
			name:     "view territory fields imply the include",
			args:     []string{"reviews", "view", "--id", "review-1", "--territory-fields", "currency"},
			path:     "/v1/customerReviews/review-1",
			payload:  viewPayload,
			wantRead: url.Values{"include": {"reviewTerritory"}, "fields[territories]": {"currency"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				if got := req.URL.Query(); !reflect.DeepEqual(got, test.wantRead) {
					t.Errorf("query = %v, want %v", got, test.wantRead)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, test.payload)
			}))
			t.Cleanup(server.Close)

			client := newReviewTestServerClient(t, server)
			restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
			t.Cleanup(restoreClient)

			_, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(append(test.args, "--output", "json"), "1.2.3"); code != rootcmd.ExitSuccess {
					t.Errorf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
				}
			})
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requests != 1 {
				t.Fatalf("request count = %d, want 1", requests)
			}
		})
	}
}

// A sparse fieldset must not print attributes Apple did not return, and the
// sideloaded territory must survive in the envelope.
func TestReviewsViewKeepsSparseEnvelope(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"type":"customerReviews","id":"review-1","attributes":{"rating":5},"relationships":{"reviewTerritory":{"data":{"type":"territories","id":"USA"}}}},"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}]}`)
	}))
	t.Cleanup(server.Close)

	client := newReviewTestServerClient(t, server)
	restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	stdout, _ := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"reviews", "view", "--id", "review-1",
			"--fields", "rating", "--include", "reviewTerritory",
			"--output", "json",
		}, "1.2.3")
		if code != rootcmd.ExitSuccess {
			t.Errorf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})

	var envelope struct {
		Data struct {
			Attributes    map[string]any `json:"attributes"`
			Relationships map[string]any `json:"relationships"`
		} `json:"data"`
		Included []struct {
			ID string `json:"id"`
		} `json:"included"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("stdout is not JSON: %v (%q)", err, stdout)
	}
	if want := map[string]any{"rating": float64(5)}; !reflect.DeepEqual(envelope.Data.Attributes, want) {
		t.Fatalf("attributes = %v, want %v", envelope.Data.Attributes, want)
	}
	if _, ok := envelope.Data.Relationships["reviewTerritory"]; !ok {
		t.Fatalf("relationships = %v, want reviewTerritory", envelope.Data.Relationships)
	}
	if len(envelope.Included) != 1 || envelope.Included[0].ID != "USA" {
		t.Fatalf("included = %+v, want the USA territory", envelope.Included)
	}
}

// Read output replays the attributes Apple returned: an empty title on a full
// read stays, and a key a sparse read left out stays out. Tables read the same
// response through the typed fields.
func TestCustomerReviewReadsKeepAppleAttributes(t *testing.T) {
	const fullAttributes = `{"rating":3,"title":"","body":"","reviewerNickname":"Tester","createdDate":"2026-01-20T00:00:00Z","territory":"USA"}`
	const sparseAttributes = `{"title":"Slow"}`
	resource := func(attributes string) string {
		return `{"type":"customerReviews","id":"review-1","attributes":` + attributes + `}`
	}

	tests := []struct {
		name       string
		args       []string
		attributes string
	}{
		{
			name:       "app list full read",
			args:       []string{"reviews", "list", "--app", "app-1"},
			attributes: fullAttributes,
		},
		{
			name:       "version list full read",
			args:       []string{"versions", "customer-reviews", "list", "--version-id", "version-1"},
			attributes: fullAttributes,
		},
		{
			name:       "view full read",
			args:       []string{"reviews", "view", "--id", "review-1"},
			attributes: fullAttributes,
		},
		{
			name:       "app list sparse read",
			args:       []string{"reviews", "list", "--app", "app-1", "--fields", "title"},
			attributes: sparseAttributes,
		},
		{
			name:       "view sparse read",
			args:       []string{"reviews", "view", "--id", "review-1", "--fields", "title"},
			attributes: sparseAttributes,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			isView := test.args[1] == "view"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if isView {
					_, _ = io.WriteString(w, `{"data":`+resource(test.attributes)+`}`)
					return
				}
				_, _ = io.WriteString(w, `{"data":[`+resource(test.attributes)+`]}`)
			}))
			t.Cleanup(server.Close)

			client := newReviewTestServerClient(t, server)
			restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
			t.Cleanup(restoreClient)

			stdout, _ := captureOutput(t, func() {
				if code := rootcmd.Run(append(test.args, "--output", "json"), "1.2.3"); code != rootcmd.ExitSuccess {
					t.Errorf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
				}
			})

			var got map[string]any
			var want map[string]any
			if err := json.Unmarshal([]byte(test.attributes), &want); err != nil {
				t.Fatal(err)
			}
			if isView {
				var envelope struct {
					Data struct {
						Attributes map[string]any `json:"attributes"`
					} `json:"data"`
				}
				if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
					t.Fatalf("stdout is not JSON: %v (%q)", err, stdout)
				}
				got = envelope.Data.Attributes
			} else {
				var envelope struct {
					Data []struct {
						Attributes map[string]any `json:"attributes"`
					} `json:"data"`
				}
				if err := json.Unmarshal([]byte(stdout), &envelope); err != nil || len(envelope.Data) != 1 {
					t.Fatalf("stdout = %q, want one review (err %v)", stdout, err)
				}
				got = envelope.Data[0].Attributes
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("attributes = %v, want %v", got, want)
			}

			table, _ := captureOutput(t, func() {
				if code := rootcmd.Run(append(test.args, "--output", "table"), "1.2.3"); code != rootcmd.ExitSuccess {
					t.Errorf("table exit code = %d, want %d", code, rootcmd.ExitSuccess)
				}
			})
			wantCell := "USA"
			if test.attributes == sparseAttributes {
				wantCell = "Slow"
			}
			if !strings.Contains(table, wantCell) {
				t.Fatalf("table = %q, want it to contain %q", table, wantCell)
			}
		})
	}
}

func TestReviewsViewRejectsInvalidSelection(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantMessage string
	}{
		{name: "fields", args: []string{"--fields", "bogus"}, wantMessage: reviewFieldsUsage},
		{name: "empty include", args: []string{"--include", ""}, wantMessage: reviewIncludeUsage},
		{name: "include", args: []string{"--include", "bogus"}, wantMessage: reviewIncludeUsage},
		{name: "territory fields", args: []string{"--territory-fields", "bogus"}, wantMessage: reviewTerritoryFieldsUsage},
		{
			name:        "response fields",
			args:        []string{"--response-fields", "bogus"},
			wantMessage: "--response-fields must be a comma-separated list of: responseBody,lastModifiedDate,state,review",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, stderr := runReviewFilterFailure(t, append([]string{"reviews", "view", "--id", "review-1"}, test.args...))
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantMessage)
			}
		})
	}
}

func runReviewFilterFailure(t *testing.T, args []string) (int, string) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Errorf("unexpected request %s %s: validation must precede the request", req.Method, req.URL.String())
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, versionCustomerReviewsPayload)
	}))
	t.Cleanup(server.Close)

	client := newReviewTestServerClient(t, server)
	restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	code := 0
	_, stderr := captureOutput(t, func() {
		code = rootcmd.Run(append(args, "--output", "json"), "1.2.3")
	})
	return code, stderr
}
