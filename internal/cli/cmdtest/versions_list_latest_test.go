package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestVersionsListLatestKeepsNewestVersionPerPlatform(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	pageTwoPath := "/v1/apps/app-1/appStoreVersions"
	pageTwoQuery := "cursor=page2"
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == pageTwoPath && req.URL.RawQuery == pageTwoQuery:
			return jsonHTTPResponse(http.StatusOK, `{"data":[
				{"type":"appStoreVersions","id":"ver-ios-new","attributes":{"platform":"IOS","versionString":"2.4.1","appStoreState":"READY_FOR_SALE","createdDate":"2026-02-20T00:30:00Z"}},
				{"type":"appStoreVersions","id":"ver-mac-old","attributes":{"platform":"MAC_OS","versionString":"1.0.0","appStoreState":"READY_FOR_SALE","createdDate":"2020-01-01T00:00:00-07:00"}}
			],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == pageTwoPath:
			if got := req.URL.Query().Get("limit"); got != "10" {
				return nil, fmt.Errorf("first-page limit = %q, want 10", got)
			}
			return jsonHTTPResponse(http.StatusOK, fmt.Sprintf(`{"data":[
				{"type":"appStoreVersions","id":"ver-ios-old","attributes":{"platform":"IOS","versionString":"2.3.2","appStoreState":"READY_FOR_SALE","createdDate":"2026-02-20T01:00:00+01:00"}},
				{"type":"appStoreVersions","id":"ver-mac-new","attributes":{"platform":"MAC_OS","versionString":"2.6.2","appStoreState":"READY_FOR_SALE","createdDate":"2020-10-26T09:49:56-07:00"}}
			],"links":{"next":"https://api.appstoreconnect.apple.com%s?%s"},"meta":{"paging":{"total":14,"limit":200}}}`, pageTwoPath, pageTwoQuery)), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "list", "--app", "app-1", "--state", "READY_FOR_SALE", "--latest", "--limit", "10", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result struct {
		Items []struct {
			ID         string `json:"id"`
			Attributes struct {
				Platform      string `json:"platform"`
				VersionString string `json:"versionString"`
			} `json:"attributes"`
		} `json:"items"`
		TotalCount int  `json:"totalCount"`
		HasMore    bool `json:"hasMore"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items length = %d, want 2 (newest per platform)", len(result.Items))
	}
	if result.TotalCount != 2 || result.HasMore {
		t.Fatalf("computed paging = totalCount %d, hasMore %t; want 2, false", result.TotalCount, result.HasMore)
	}
	byPlatform := map[string]string{}
	for _, version := range result.Items {
		byPlatform[version.Attributes.Platform] = version.Attributes.VersionString
	}
	if byPlatform["IOS"] != "2.4.1" {
		t.Fatalf("IOS latest = %q, want 2.4.1 (newest across pages)", byPlatform["IOS"])
	}
	if byPlatform["MAC_OS"] != "2.6.2" {
		t.Fatalf("MAC_OS latest = %q, want 2.6.2", byPlatform["MAC_OS"])
	}
}

func TestVersionsListLatestRejectsInvalidCreatedDate(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-invalid","attributes":{"platform":"IOS","versionString":"2.4.1","createdDate":"not-a-date"}}],"links":{}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "list", "--app", "app-1", "--latest", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), `version "ver-invalid" has invalid createdDate`) {
			t.Fatalf("expected invalid createdDate error, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected no computed output for invalid data, got %q", stdout)
	}
}

func TestVersionsListLatestKeepsOnlySelectedIncludesAndRedactsSecrets(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if got := req.URL.Query().Get("include"); got != "build,appStoreReviewDetail" {
			return nil, fmt.Errorf("include = %q, want build,appStoreReviewDetail", got)
		}
		return jsonHTTPResponse(http.StatusOK, `{
			"data":[
				{"type":"appStoreVersions","id":"old","attributes":{"platform":"IOS","versionString":"1.0","createdDate":"2025-01-01T00:00:00Z"},"relationships":{"build":{"data":{"type":"builds","id":"build-old"}},"appStoreReviewDetail":{"data":{"type":"appStoreReviewDetails","id":"review-old"}}}},
				{"type":"appStoreVersions","id":"new","attributes":{"platform":"IOS","versionString":"2.0","createdDate":"2026-01-01T00:00:00Z"},"relationships":{"build":{"data":{"type":"builds","id":"build-new"}},"appStoreReviewDetail":{"data":{"type":"appStoreReviewDetails","id":"review-new"}}}}
			],
			"included":[
				{"type":"builds","id":"build-old","attributes":{"version":"1"}},
				{"type":"appStoreReviewDetails","id":"review-old","attributes":{"demoAccountPassword":"old-secret"}},
				{"type":"builds","id":"build-new","attributes":{"version":"2"}},
				{"type":"appStoreReviewDetails","id":"review-new","attributes":{"demoAccountPassword":"new-secret"}}
			],
			"links":{}
		}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "list", "--app", "app-1", "--latest", "--include", "build,appStoreReviewDetail", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		Included []struct {
			ID         string `json:"id"`
			Attributes struct {
				DemoAccountPassword string `json:"demoAccountPassword"`
			} `json:"attributes"`
		} `json:"included"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Items) != 1 || result.Items[0].ID != "new" {
		t.Fatalf("items = %#v, want only new version", result.Items)
	}
	if len(result.Included) != 2 {
		t.Fatalf("included = %#v, want only resources linked to new version", result.Included)
	}
	byID := map[string]string{}
	for _, resource := range result.Included {
		byID[resource.ID] = resource.Attributes.DemoAccountPassword
	}
	if _, ok := byID["build-old"]; ok {
		t.Fatalf("included retained old build: %#v", result.Included)
	}
	if _, ok := byID["review-old"]; ok {
		t.Fatalf("included retained old review detail: %#v", result.Included)
	}
	if _, ok := byID["build-new"]; !ok {
		t.Fatalf("included omitted selected build: %#v", result.Included)
	}
	if got := byID["review-new"]; got != "(redacted)" {
		t.Fatalf("review password = %q, want redacted", got)
	}
}

func TestVersionsListLatestRejectsNextURL(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"versions", "list", "--latest", "--next", "https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=x"}, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--latest fetches all pages itself and cannot be combined with --next") {
		t.Fatalf("expected latest/next conflict diagnostic, got %q", stderr)
	}
}

// The help recommends --state READY_FOR_DISTRIBUTION --latest for live versions
// that Apple reports only through appVersionState; it must filter on that
// attribute and return such a version.
func TestVersionsListLatestFindsVersionReportedOnlyAsReadyForDistribution(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		query := req.URL.Query()
		if got := query.Get("filter[appVersionState]"); got != "READY_FOR_DISTRIBUTION" {
			return nil, fmt.Errorf("filter[appVersionState] = %q, want READY_FOR_DISTRIBUTION", got)
		}
		if got := query.Get("filter[appStoreState]"); got != "" {
			return nil, fmt.Errorf("filter[appStoreState] = %q, want unset", got)
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[
			{"type":"appStoreVersions","id":"ver-live","attributes":{"platform":"IOS","versionString":"3.0","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-09-01T00:00:00Z"}}
		],"links":{}}`), nil
	})

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"versions", "list", "--app", "app-1", "--state", "READY_FOR_DISTRIBUTION", "--latest", "--output", "json"}, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if !strings.Contains(stdout, `"id":"ver-live"`) {
		t.Fatalf("expected the READY_FOR_DISTRIBUTION version, got stdout=%q stderr=%q", stdout, stderr)
	}
}
