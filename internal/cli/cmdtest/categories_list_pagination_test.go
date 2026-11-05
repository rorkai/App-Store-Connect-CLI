package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestCategoriesListRejectsInvalidLimit(t *testing.T) {
	clientFactoryCalled := false
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientFactoryCalled = true
		return nil, errors.New("client factory must not run during validation")
	})
	defer restore()

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"categories", "list", "--limit", "0"}, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "categories list: --limit must be between 1 and 200") {
		t.Fatalf("stderr = %q, want invalid --limit error", stderr)
	}
	if clientFactoryCalled {
		t.Fatal("client factory ran before --limit validation")
	}
}

func TestCategoriesListDefaultsToMaximumPageSize(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appCategories" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		want := url.Values{"limit": {"200"}}
		if got := req.URL.Query(); got.Encode() != want.Encode() {
			t.Errorf("query = %q, want %q", got.Encode(), want.Encode())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"appCategories","id":"category-default"}],"links":{"next":""}}`)
	}))
	t.Cleanup(server.Close)
	installDefaultTransportForServer(t, server)

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"categories", "list", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	assertCategoryResponseID(t, stdout, "category-default")
}

func TestCategoriesListPaginateAggregatesPages(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const secondURL = "https://api.appstoreconnect.apple.com/v1/appCategories?cursor=BQ&limit=200"

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appCategories" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch requestCount {
		case 1:
			want := url.Values{"limit": {"200"}}
			if got := req.URL.Query(); got.Encode() != want.Encode() {
				t.Errorf("first query = %q, want %q", got.Encode(), want.Encode())
			}
			_, _ = io.WriteString(w, `{"data":[{"type":"appCategories","id":"category-page-1"}],"links":{"next":"`+secondURL+`"}}`)
		case 2:
			want := url.Values{"cursor": {"BQ"}, "limit": {"200"}}
			if got := req.URL.Query(); got.Encode() != want.Encode() {
				t.Errorf("second query = %q, want %q", got.Encode(), want.Encode())
			}
			_, _ = io.WriteString(w, `{"data":[{"type":"appCategories","id":"category-page-2"}],"links":{"next":""}}`)
		default:
			t.Errorf("unexpected extra request: %s", req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	installDefaultTransportForServer(t, server)

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"categories", "list", "--paginate", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}

	var response struct {
		Data []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"data"`
		Links struct {
			Next string `json:"next"`
		} `json:"links"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse JSON output: %v\noutput: %s", err, stdout)
	}
	if len(response.Data) != 2 {
		t.Fatalf("data = %#v, want two aggregated resources", response.Data)
	}
	if response.Data[0].ID != "category-page-1" || response.Data[1].ID != "category-page-2" {
		t.Fatalf("data ids = %q, %q, want page order", response.Data[0].ID, response.Data[1].ID)
	}
	if response.Data[0].Type != "appCategories" {
		t.Fatalf("data type = %q, want appCategories envelope preserved", response.Data[0].Type)
	}
	if response.Links.Next != "" {
		t.Fatalf("links.next = %q, want cleared after aggregation", response.Links.Next)
	}
}

func TestCategoriesListPaginateFromNextCursor(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const firstURL = "https://api.appstoreconnect.apple.com/v1/appCategories?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appCategories?cursor=BQ&limit=200"

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		switch requestCount {
		case 1:
			if got, want := req.URL.RequestURI(), "/v1/appCategories?cursor=AQ&limit=200"; got != want {
				t.Errorf("first request = %q, want %q", got, want)
			}
			_, _ = io.WriteString(w, `{"data":[{"type":"appCategories","id":"category-cursor-1"}],"links":{"next":"`+secondURL+`"}}`)
		case 2:
			if got, want := req.URL.RequestURI(), "/v1/appCategories?cursor=BQ&limit=200"; got != want {
				t.Errorf("second request = %q, want %q", got, want)
			}
			_, _ = io.WriteString(w, `{"data":[{"type":"appCategories","id":"category-cursor-2"}],"links":{"next":""}}`)
		default:
			t.Errorf("unexpected extra request: %s", req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	installDefaultTransportForServer(t, server)

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"categories", "list", "--paginate", "--next", firstURL, "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
	for _, id := range []string{"category-cursor-1", "category-cursor-2"} {
		if !strings.Contains(stdout, `"id":"`+id+`"`) {
			t.Fatalf("stdout = %q, want to contain %q", stdout, id)
		}
	}
}
