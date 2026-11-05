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

func TestCategoriesRejectsNextQueryFlagsBeforeAuth(t *testing.T) {
	const listNextURL = "https://api.appstoreconnect.apple.com/v1/appCategories?cursor=next"
	const nextURL = "https://api.appstoreconnect.apple.com/v1/appCategories/GAMES/subcategories?cursor=next"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "category before next",
			args:    []string{"categories", "subcategories", "--category-id", "BUSINESS", "--next", nextURL},
			wantErr: "categories subcategories: --next cannot be combined with --category-id",
		},
		{
			name:    "explicit empty category after next",
			args:    []string{"categories", "subcategories", "--next", nextURL, "--category-id", ""},
			wantErr: "categories subcategories: --next cannot be combined with --category-id",
		},
		{
			name:    "limit before next",
			args:    []string{"categories", "subcategories", "--limit", "25", "--next", nextURL},
			wantErr: "categories subcategories: --next cannot be combined with --limit",
		},
		{
			name:    "explicit zero limit after next",
			args:    []string{"categories", "subcategories", "--next", nextURL, "--limit", "0"},
			wantErr: "categories subcategories: --next cannot be combined with --limit",
		},
		{
			name:    "out of range limit after next",
			args:    []string{"categories", "subcategories", "--next", nextURL, "--limit", "201"},
			wantErr: "categories subcategories: --next cannot be combined with --limit",
		},
		{
			name:    "list limit before next",
			args:    []string{"categories", "list", "--limit", "25", "--next", listNextURL},
			wantErr: "categories list: --next cannot be combined with --limit",
		},
		{
			name:    "list explicit default limit after next",
			args:    []string{"categories", "list", "--next", listNextURL, "--limit", "200"},
			wantErr: "categories list: --next cannot be combined with --limit",
		},
		{
			name:    "list out of range limit after next",
			args:    []string{"categories", "list", "--next", listNextURL, "--limit", "201"},
			wantErr: "categories list: --next cannot be combined with --limit",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			})
			defer restore()

			assertUsageExit(t, test.args, test.wantErr)
			if clientFactoryCalled {
				t.Fatal("client factory ran before --next conflict validation")
			}
		})
	}
}

func TestCategoriesInvalidNextPrecedesLimitConflict(t *testing.T) {
	for _, test := range []struct {
		command string
		next    string
	}{
		{command: "list", next: "http://api.appstoreconnect.apple.com/v1/appCategories?cursor=next"},
		{command: "subcategories", next: "http://api.appstoreconnect.apple.com/v1/appCategories/GAMES/subcategories?cursor=next"},
	} {
		t.Run(test.command, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				code := rootcmd.Run([]string{"categories", test.command, "--next", test.next, "--limit", "201"}, "1.2.3")
				if code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			// The usage page ffcli renders after a usage error lists every flag, so the
			// precedence assertion is scoped to the diagnostic line itself.
			diagnostic, _, _ := strings.Cut(stderr, "\n")
			if want := "Error: categories " + test.command + ": --next must be an App Store Connect URL"; diagnostic != want {
				t.Fatalf("stderr diagnostic = %q, want %q", diagnostic, want)
			}
		})
	}
}

func TestCategoriesNextOnlyUsesCursorURL(t *testing.T) {
	for _, test := range []struct {
		command string
		path    string
	}{
		{command: "list", path: "/v1/appCategories"},
		{command: "subcategories", path: "/v1/appCategories/GAMES/subcategories"},
	} {
		t.Run(test.command, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			nextURL := "https://api.appstoreconnect.apple.com" + test.path + "?cursor=page-2&limit=5"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				want := url.Values{"cursor": {"page-2"}, "limit": {"5"}}
				if got := req.URL.Query(); got.Encode() != want.Encode() {
					t.Errorf("query = %q, want %q", got.Encode(), want.Encode())
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":[{"type":"appCategories","id":"category-next"}],"links":{"next":""}}`)
			}))
			t.Cleanup(server.Close)
			installDefaultTransportForServer(t, server)

			root := RootCommand("1.2.3")
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"categories", test.command, "--next", nextURL, "--output", "json"}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			assertCategoryResponseID(t, stdout, "category-next")
		})
	}
}

func TestCategoriesSubcategoriesCategoryOnlyBuildsLimitQuery(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appCategories/GAMES/subcategories" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		want := url.Values{"limit": {"25"}}
		if got := req.URL.Query(); got.Encode() != want.Encode() {
			t.Errorf("query = %q, want %q", got.Encode(), want.Encode())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"appCategories","id":"subcategory-filtered"}],"links":{"next":""}}`)
	}))
	t.Cleanup(server.Close)
	installDefaultTransportForServer(t, server)

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		args := []string{
			"categories", "subcategories",
			"--category-id", "GAMES",
			"--limit", "25",
			"--output", "json",
		}
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	assertCategoryResponseID(t, stdout, "subcategory-filtered")
}

func assertCategoryResponseID(t *testing.T, stdout, wantID string) {
	t.Helper()

	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse JSON output: %v\noutput: %s", err, stdout)
	}
	if len(response.Data) != 1 || response.Data[0].ID != wantID {
		t.Fatalf("response data = %#v, want one resource with id %q", response.Data, wantID)
	}
}
