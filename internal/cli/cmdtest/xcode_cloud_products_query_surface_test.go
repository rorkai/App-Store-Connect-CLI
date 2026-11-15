package cmdtest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type ciProductsListQueryCapture struct {
	requests []ciProductsListQueryRequest
	client   func() (*asc.Client, error)
}

type ciProductsListQueryRequest struct {
	path  string
	query url.Values
}

func ciProductsListQueryStub(t *testing.T, handler http.HandlerFunc) *ciProductsListQueryCapture {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	captured := &ciProductsListQueryCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		captured.requests = append(captured.requests, ciProductsListQueryRequest{
			path:  req.URL.Path,
			query: req.URL.Query(),
		})
		handler(w, req)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Scheme + "://" + req.URL.Host; got != asc.BaseURL {
			t.Errorf("request origin = %s, want %s", got, asc.BaseURL)
		}
		routed := req.Clone(req.Context())
		routed.URL.Scheme = serverURL.Scheme
		routed.URL.Host = serverURL.Host
		routed.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(routed)
	})
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	captured.client = func() (*asc.Client, error) {
		return client, nil
	}

	return captured
}

func (r *ciProductsListQueryCapture) assertNoRequest(t *testing.T) {
	t.Helper()
	if len(r.requests) != 0 {
		t.Fatalf("expected no request, got %d (%s?%s)", len(r.requests), r.requests[0].path, r.requests[0].query.Encode())
	}
}

func runCiProductsListQuery(t *testing.T, captured *ciProductsListQueryCapture, args ...string) (string, string, error) {
	t.Helper()

	return runRootWithDeps(t, captured.client, args...)
}

func writeCiProductsResponse(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

func TestXcodeCloudProductsListQuerySurface(t *testing.T) {
	captured := ciProductsListQueryStub(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/ciProducts" {
			t.Errorf("path = %q, want /v1/ciProducts", req.URL.Path)
		}
		writeCiProductsResponse(w, `{"data":[{"type":"ciProducts","id":"product-1"}],"links":{"next":""}}`)
	})

	stdout, stderr, err := runCiProductsListQuery(
		t,
		captured,
		"xcode-cloud", "products", "list",
		"--app", "123456789",
		"--product-type", "app,FRAMEWORK",
		"--fields", "name,productType,primaryRepositories",
		"--app-fields", "name,bundleId",
		"--bundle-id-fields", "identifier,platform",
		"--scm-repository-fields", "repositoryName,ownerName",
		"--include", "app,bundleId,primaryRepositories",
		"--primary-repositories-limit", "25",
		"--limit", "10",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}
	if len(captured.requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(captured.requests))
	}

	got := captured.requests[0]
	want := url.Values{
		"filter[app]":                {"123456789"},
		"filter[productType]":        {"APP,FRAMEWORK"},
		"fields[ciProducts]":         {"name,productType,primaryRepositories,app,bundleId"},
		"fields[apps]":               {"name,bundleId"},
		"fields[bundleIds]":          {"identifier,platform"},
		"fields[scmRepositories]":    {"repositoryName,ownerName"},
		"include":                    {"app,bundleId,primaryRepositories"},
		"limit[primaryRepositories]": {"25"},
		"limit":                      {"10"},
	}
	if got.query.Encode() != want.Encode() {
		t.Fatalf("query = %q, want %q", got.query.Encode(), want.Encode())
	}

	var response asc.CiProductsResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode stdout: %v (stdout=%q)", err, stdout)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "product-1" {
		t.Fatalf("response = %+v, want product-1", response.Data)
	}
}

func TestXcodeCloudProductsListQueryFlagsAreRegistered(t *testing.T) {
	for _, path := range [][]string{
		{"xcode-cloud", "products"},
		{"xcode-cloud", "products", "list"},
	} {
		cmd := findCommandByPath(t, path...)
		for _, name := range []string{
			"product-type",
			"fields",
			"app-fields",
			"bundle-id-fields",
			"scm-repository-fields",
			"include",
			"primary-repositories-limit",
		} {
			flagValue := cmd.FlagSet.Lookup(name)
			if flagValue == nil {
				t.Fatalf("%s: missing --%s flag", strings.Join(path, " "), name)
			}
		}
	}
}

func TestXcodeCloudProductsListRejectsInvalidQueryValuesBeforeAuth(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "product type", args: []string{"--product-type", "APP,UNKNOWN"}, want: "--product-type must be one of"},
		{name: "product fields", args: []string{"--fields", "notAField"}, want: "--fields must be one of"},
		{name: "app fields", args: []string{"--app-fields", "notAField"}, want: "--app-fields must be one of"},
		{name: "bundle ID fields", args: []string{"--bundle-id-fields", "notAField"}, want: "--bundle-id-fields must be one of"},
		{name: "SCM repository fields", args: []string{"--scm-repository-fields", "notAField"}, want: "--scm-repository-fields must be one of"},
		{name: "include", args: []string{"--include", "notARelationship"}, want: "--include must be one of"},
		{name: "primary repositories zero limit", args: []string{"--primary-repositories-limit", "0"}, want: "--primary-repositories-limit must be between 1 and 50"},
		{name: "primary repositories limit", args: []string{"--primary-repositories-limit", "51"}, want: "--primary-repositories-limit must be between 1 and 50"},
		{name: "app fields prerequisite", args: []string{"--app-fields", "name"}, want: "--app-fields requires --include app"},
		{name: "bundle ID fields prerequisite", args: []string{"--bundle-id-fields", "identifier"}, want: "--bundle-id-fields requires --include bundleId"},
		{name: "SCM repository fields prerequisite", args: []string{"--scm-repository-fields", "repositoryName"}, want: "--scm-repository-fields requires --include primaryRepositories"},
		{name: "primary repositories limit prerequisite", args: []string{"--primary-repositories-limit", "10"}, want: "--primary-repositories-limit requires --include primaryRepositories"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			captured := ciProductsListQueryStub(t, func(w http.ResponseWriter, req *http.Request) {
				writeCiProductsResponse(w, `{"data":[]}`)
			})

			_, stderr, err := runCiProductsListQuery(t, captured, append([]string{"xcode-cloud", "products", "list"}, test.args...)...)
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			captured.assertNoRequest(t)
		})
	}
}

func TestXcodeCloudProductsListRejectsQueryFlagsCombinedWithNext(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=PAGE2"
	tests := []struct {
		name     string
		flagName string
		args     []string
	}{
		{name: "product type", flagName: "product-type", args: []string{"--product-type", "APP"}},
		{name: "product fields", flagName: "fields", args: []string{"--fields", "name"}},
		{name: "app fields", flagName: "app-fields", args: []string{"--app-fields", "name"}},
		{name: "bundle ID fields", flagName: "bundle-id-fields", args: []string{"--bundle-id-fields", "identifier"}},
		{name: "SCM repository fields", flagName: "scm-repository-fields", args: []string{"--scm-repository-fields", "repositoryName"}},
		{name: "include", flagName: "include", args: []string{"--include", "app"}},
		{name: "primary repositories limit", flagName: "primary-repositories-limit", args: []string{"--primary-repositories-limit", "10"}},
		{name: "limit", flagName: "limit", args: []string{"--limit", "10"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			captured := ciProductsListQueryStub(t, func(w http.ResponseWriter, req *http.Request) {
				writeCiProductsResponse(w, `{"data":[]}`)
			})
			factoryCalled := false
			captured.client = func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("client factory must not run during --next validation")
			}

			args := append([]string{"xcode-cloud", "products", "list", "--next", nextURL}, test.args...)
			_, stderr, err := runCiProductsListQuery(t, captured, args...)
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			want := ": --next cannot be combined with --" + test.flagName
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if factoryCalled {
				t.Fatal("client factory ran before --next conflict validation")
			}
			captured.assertNoRequest(t)
		})
	}
}

func TestXcodeCloudProductsListPreservesServerNextQueryDuringPagination(t *testing.T) {
	firstQuery := url.Values{
		"filter[app]":                {"123456789"},
		"filter[productType]":        {"APP"},
		"fields[ciProducts]":         {"name,primaryRepositories,app"},
		"fields[apps]":               {"name"},
		"include":                    {"app,primaryRepositories"},
		"limit[primaryRepositories]": {"25"},
		"limit":                      {"200"},
	}
	nextQuery := url.Values{
		"cursor":                     {"PAGE2"},
		"filter[app]":                {"123456789"},
		"filter[productType]":        {"APP"},
		"fields[ciProducts]":         {"name,primaryRepositories,app"},
		"fields[apps]":               {"name"},
		"include":                    {"app,primaryRepositories"},
		"limit[primaryRepositories]": {"25"},
		"limit":                      {"200"},
	}
	nextURL := "https://api.appstoreconnect.apple.com/v1/ciProducts?" + nextQuery.Encode()

	requestNumber := 0
	captured := ciProductsListQueryStub(t, func(w http.ResponseWriter, req *http.Request) {
		requestNumber++
		switch requestNumber {
		case 1:
			if req.URL.RawQuery != firstQuery.Encode() {
				t.Errorf("first query = %q, want %q", req.URL.RawQuery, firstQuery.Encode())
			}
			writeCiProductsResponse(w, `{"data":[{"type":"ciProducts","id":"product-1"}],"included":[{"type":"apps","id":"app-1","attributes":{"name":"Example"}}],"links":{"next":"`+nextURL+`"}}`)
		case 2:
			if req.URL.Path != "/v1/ciProducts" || req.URL.RawQuery != nextQuery.Encode() {
				t.Errorf("continuation request = %s?%s, want /v1/ciProducts?%s", req.URL.Path, req.URL.RawQuery, nextQuery.Encode())
			}
			writeCiProductsResponse(w, `{"data":[{"type":"ciProducts","id":"product-2"}],"included":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app"}}],"links":{"next":""}}`)
		default:
			t.Errorf("unexpected request count %d", requestNumber)
			writeCiProductsResponse(w, `{"data":[]}`)
		}
	})

	stdout, stderr, err := runCiProductsListQuery(
		t,
		captured,
		"xcode-cloud", "products", "list",
		"--app", "123456789",
		"--product-type", "APP",
		"--fields", "name,primaryRepositories",
		"--app-fields", "name",
		"--include", "app,primaryRepositories",
		"--primary-repositories-limit", "25",
		"--paginate",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}
	if len(captured.requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(captured.requests))
	}

	var response asc.CiProductsResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode stdout: %v (stdout=%q)", err, stdout)
	}
	if len(response.Data) != 2 || response.Data[0].ID != "product-1" || response.Data[1].ID != "product-2" {
		t.Fatalf("response IDs = %+v, want product-1 and product-2", response.Data)
	}
	var included []struct {
		Type asc.ResourceType `json:"type"`
		ID   string           `json:"id"`
	}
	if err := json.Unmarshal(response.Included, &included); err != nil {
		t.Fatalf("decode included: %v (included=%s)", err, response.Included)
	}
	if len(included) != 2 || included[0].Type != asc.ResourceTypeApps || included[0].ID != "app-1" || included[1].Type != asc.ResourceTypeBundleIds || included[1].ID != "bundle-1" {
		t.Fatalf("included = %+v, want apps/app-1 and bundleIds/bundle-1", included)
	}
}

func TestXcodeCloudProductsListAllowsExistingAppWithNextURL(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=PAGE2&limit=17"
	captured := ciProductsListQueryStub(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/ciProducts" || req.URL.RawQuery != "cursor=PAGE2&limit=17" {
			t.Errorf("request URL = %s?%s, want /v1/ciProducts?cursor=PAGE2&limit=17", req.URL.Path, req.URL.RawQuery)
		}
		writeCiProductsResponse(w, `{"data":[{"type":"ciProducts","id":"product-next"}],"links":{"next":""}}`)
	})

	stdout, stderr, err := runCiProductsListQuery(
		t,
		captured,
		"xcode-cloud", "products", "list",
		"--next", nextURL,
		"--app", "123456789",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}
	if len(captured.requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(captured.requests))
	}
	if !strings.Contains(stdout, `"id":"product-next"`) {
		t.Fatalf("stdout = %q, want product-next", stdout)
	}
}

func TestXcodeCloudProductsListRejectsBlankQuerySelections(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "product type", args: []string{"--product-type", ","}, want: "--product-type must not be empty"},
		{name: "product fields", args: []string{"--fields", ","}, want: "--fields must not be empty"},
		{name: "include", args: []string{"--include", ","}, want: "--include must not be empty"},
	} {
		t.Run(test.name, func(t *testing.T) {
			captured := ciProductsListQueryStub(t, func(w http.ResponseWriter, req *http.Request) {
				writeCiProductsResponse(w, `{"data":[]}`)
			})
			_, stderr, err := runCiProductsListQuery(t, captured, append([]string{"xcode-cloud", "products", "list"}, test.args...)...)
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			captured.assertNoRequest(t)
		})
	}
}
