package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionsListReadyForDistributionUsesAppVersionStateFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/123456789/appStoreVersions" {
			t.Fatalf("expected app versions path, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if got := query.Get("filter[appVersionState]"); got != "READY_FOR_DISTRIBUTION" {
			t.Fatalf("expected filter[appVersionState]=READY_FOR_DISTRIBUTION, got %q", got)
		}
		if got := query.Get("filter[appStoreState]"); got != "" {
			t.Fatalf("expected no deprecated app store state filter, got %q", got)
		}
		body := `{"data":[{"type":"appStoreVersions","id":"version-live","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"READY_FOR_DISTRIBUTION"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "list", "--app", "123456789", "--state", "READY_FOR_DISTRIBUTION"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"version-live"`) {
		t.Fatalf("expected version output, got %q", stdout)
	}
}

func TestVersionsListMixedReadyStatesUsesSingleAppVersionStateFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		if got := query.Get("filter[appVersionState]"); got != "READY_FOR_REVIEW,READY_FOR_DISTRIBUTION" {
			t.Fatalf("expected filter[appVersionState]=READY_FOR_REVIEW,READY_FOR_DISTRIBUTION, got %q", got)
		}
		if got := query.Get("filter[appStoreState]"); got != "" {
			t.Fatalf("expected no deprecated app store state filter, got %q", got)
		}
		body := `{"data":[{"type":"appStoreVersions","id":"version-review","attributes":{"versionString":"1.1","platform":"IOS","appVersionState":"READY_FOR_REVIEW"}},{"type":"appStoreVersions","id":"version-live","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"READY_FOR_DISTRIBUTION"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "list", "--app", "123456789", "--state", "READY_FOR_REVIEW,READY_FOR_DISTRIBUTION"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"version-review"`) || !strings.Contains(stdout, `"id":"version-live"`) {
		t.Fatalf("expected both versions in output, got %q", stdout)
	}
}

func TestVersionsListRejectsMixedAppStoreAndAppVersionOnlyStates(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{"versions", "list", "--app", "123456789", "--state", "READY_FOR_SALE,READY_FOR_DISTRIBUTION"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if err == nil {
		t.Fatal("expected mixed state filter error")
	}
	if !strings.Contains(err.Error(), "cannot mix appVersionState-only values with appStoreState-only values") {
		t.Fatalf("expected mixed state filter error, got %q", err.Error())
	}
}
