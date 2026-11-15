package cmdtest

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestBetaGroupsListQuerySurfacePropagatesOpenAPIFiltersAndIncludes(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/betaGroups" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}

		want := url.Values{
			"filter[app]":                     {"app-1"},
			"filter[id]":                      {"group-1,group-2"},
			"filter[publicLinkEnabled]":       {"true"},
			"filter[publicLinkLimitEnabled]":  {"false"},
			"filter[publicLink]":              {"https://example.com/public"},
			"fields[betaGroups]":              {"name,publicLink,app,builds,betaTesters,betaRecruitmentCriteria"},
			"fields[apps]":                    {"name,bundleId"},
			"fields[builds]":                  {"version"},
			"fields[betaTesters]":             {"email"},
			"fields[betaRecruitmentCriteria]": {"lastModifiedDate"},
			"include":                         {"app,builds,betaTesters,betaRecruitmentCriteria"},
			"limit[betaTesters]":              {"25"},
			"limit[builds]":                   {"100"},
		}
		if got := req.URL.Query(); got.Encode() != want.Encode() {
			t.Errorf("query = %q, want %q", got.Encode(), want.Encode())
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"QA"}}]}`)
	}))
	t.Cleanup(server.Close)

	stdout, stderr, err := runRootWithDeps(t, stubServerClient(t, server),
		"testflight", "groups", "list", "--app", "app-1",
		"--id", "group-1,group-2",
		"--public-link-enabled", "true",
		"--public-link-limit-enabled", "false",
		"--public-link", "https://example.com/public",
		"--fields", "name,publicLink",
		"--app-fields", "name,bundleId",
		"--build-fields", "version",
		"--tester-fields", "email",
		"--recruitment-criteria-fields", "lastModifiedDate",
		"--include", "app,builds,betaTesters,betaRecruitmentCriteria",
		"--testers-limit", "25",
		"--builds-limit", "100",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"id":"group-1"`) {
		t.Fatalf("expected group response, got %q", stdout)
	}
}

func TestBetaGroupsListNextRejectsQuerySurfaceFlagsBeforeAuth(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/betaGroups?cursor=next"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "id", args: []string{"--id", "group-1"}, wantErr: "--next cannot be combined with --id"},
		{name: "public link enabled", args: []string{"--public-link-enabled", "true"}, wantErr: "--next cannot be combined with --public-link-enabled"},
		{name: "public link limit enabled", args: []string{"--public-link-limit-enabled", "true"}, wantErr: "--next cannot be combined with --public-link-limit-enabled"},
		{name: "public link", args: []string{"--public-link", "link"}, wantErr: "--next cannot be combined with --public-link"},
		{name: "fields", args: []string{"--fields", "name"}, wantErr: "--next cannot be combined with --fields"},
		{name: "app fields", args: []string{"--app-fields", "name"}, wantErr: "--next cannot be combined with --app-fields"},
		{name: "build fields", args: []string{"--build-fields", "version"}, wantErr: "--next cannot be combined with --build-fields"},
		{name: "tester fields", args: []string{"--tester-fields", "email"}, wantErr: "--next cannot be combined with --tester-fields"},
		{name: "recruitment criteria fields", args: []string{"--recruitment-criteria-fields", "lastModifiedDate"}, wantErr: "--next cannot be combined with --recruitment-criteria-fields"},
		{name: "include", args: []string{"--include", "app"}, wantErr: "--next cannot be combined with --include"},
		{name: "testers limit", args: []string{"--testers-limit", "10"}, wantErr: "--next cannot be combined with --testers-limit"},
		{name: "builds limit", args: []string{"--builds-limit", "10"}, wantErr: "--next cannot be combined with --builds-limit"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			client := func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			}

			assertUsageExitWithDeps(t, client, append([]string{"testflight", "groups", "list", "--next", nextURL}, test.args...), test.wantErr)
			if clientFactoryCalled {
				t.Fatal("client factory ran before --next conflict validation")
			}
		})
	}
}

func TestBetaGroupsListQuerySurfaceRejectsInvalidValuesBeforeAuth(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "public link enabled", args: []string{"--public-link-enabled", "maybe"}, want: "--public-link-enabled must be true or false"},
		{name: "beta group fields", args: []string{"--fields", "notAField"}, want: "--fields must be one of"},
		{name: "app fields", args: []string{"--app-fields", "notAField"}, want: "--app-fields must be one of"},
		{name: "build fields", args: []string{"--build-fields", "notAField"}, want: "--build-fields must be one of"},
		{name: "tester fields", args: []string{"--tester-fields", "notAField"}, want: "--tester-fields must be one of"},
		{name: "recruitment criteria fields", args: []string{"--recruitment-criteria-fields", "notAField"}, want: "--recruitment-criteria-fields must be one of"},
		{name: "explicit zero testers limit", args: []string{"--testers-limit", "0"}, want: "--testers-limit must be between 1 and 50"},
		{name: "explicit zero builds limit", args: []string{"--builds-limit", "0"}, want: "--builds-limit must be between 1 and 1000"},
		{name: "public link limit", args: []string{"--testers-limit", "51"}, want: "--testers-limit must be between 1 and 50"},
		{name: "builds limit", args: []string{"--builds-limit", "1001"}, want: "--builds-limit must be between 1 and 1000"},
		{name: "include", args: []string{"--include", "invalid"}, want: "--include must be one of"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			client := func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			}

			assertUsageExitWithDeps(t, client, append([]string{"testflight", "groups", "list", "--global"}, test.args...), test.want)
			if clientFactoryCalled {
				t.Fatal("client factory ran before query validation")
			}
		})
	}
}
