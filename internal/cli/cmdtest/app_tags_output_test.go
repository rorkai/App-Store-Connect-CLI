package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAppTagsListOutputAndQueryOptions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appTags", "fields[appTags]=name,visibleInAppStore", "fields[territories]=currency", "filter[visibleInAppStore]=true,false", "include=territories", "limit=5", "limit[territories]=2", "sort=-name"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"appTags",
					"id":"tag-1",
					"attributes":{"name":"Featured","visibleInAppStore":true}
				}
			],
			"included":[
				{
					"type":"territories",
					"id":"USA",
					"attributes":{"currency":"USD"}
				}
			],
			"links":{"next":""}
		}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-tags", "list",
			"--app", "app-1",
			"--visible-in-app-store", "true,false",
			"--sort", "-name",
			"--fields", "name,visibleInAppStore",
			"--include", "territories",
			"--territory-fields", "currency",
			"--territory-limit", "2",
			"--limit", "5",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "Warning: App-tag territories are deprecated in API 4.5; remove territory selections and lookups. Requests are still forwarded for compatibility.\n" {
		t.Fatalf("expected one deprecation warning, got %q", stderr)
	}

	var out struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Name              string `json:"name"`
				VisibleInAppStore bool   `json:"visibleInAppStore"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if len(out.Data) != 1 {
		t.Fatalf("expected 1 app tag, got %d", len(out.Data))
	}
	if out.Data[0].ID != "tag-1" || out.Data[0].Attributes.Name != "Featured" || !out.Data[0].Attributes.VisibleInAppStore {
		t.Fatalf("unexpected app tag output: %+v", out.Data[0])
	}
}

func TestAppTagsListReturnsFetchFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appTags"),
		asctest.JSON(http.StatusForbidden, `{"errors":[{"status":"403","title":"Forbidden","detail":"not allowed"}]}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"app-tags", "list", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "app-tags list: failed to fetch") {
		t.Fatalf("expected fetch failure, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestAppTagsListPaginateRejectsRepeatedNextURL(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	const repeatedNextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?cursor=AQ"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appTags", "limit=200"), asctest.JSON(http.StatusOK, `{
				"data":[{"type":"appTags","id":"tag-1"}],
				"links":{"next":"`+repeatedNextURL+`"}
			}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appTags", "cursor=AQ"), asctest.JSON(http.StatusOK, `{
				"data":[{"type":"appTags","id":"tag-2"}],
				"links":{"next":"`+repeatedNextURL+`"}
			}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"app-tags", "list", "--app", "app-1", "--paginate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "detected repeated pagination URL") {
		t.Fatalf("expected repeated pagination URL error, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "app-tags list:") {
		t.Fatalf("expected app-tags context, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestAppTagsListOutputErrors(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	asctest.NewRegistry(t).Install()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unsupported output",
			args:    []string{"app-tags", "list", "--app", "app-1", "--output", "yaml"},
			wantErr: `(got "yaml")`,
		},
		{
			name:    "pretty with markdown",
			args:    []string{"app-tags", "list", "--app", "app-1", "--output", "markdown", "--pretty"},
			wantErr: "--pretty is only valid with JSON output",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !isUsageClassError(runErr) {
				t.Fatalf("expected help error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppTagsListPaginateWithFiltersUsesQueryOptions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?cursor=BQ&limit=200"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appTags", "fields[appTags]=name,visibleInAppStore", "fields[territories]=currency", "filter[visibleInAppStore]=true,false", "include=territories", "limit=200", "limit[territories]=2", "sort=-name"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"appTags","id":"tag-page-1"}],
				"links":{"next":"`+secondURL+`"}
			}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appTags", "cursor=BQ", "limit=200"), asctest.JSON(http.StatusOK, `{
				"data":[{"type":"appTags","id":"tag-page-2"}],
				"links":{"next":""}
			}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-tags", "list",
			"--app", "app-1",
			"--visible-in-app-store", "true,false",
			"--sort", "-name",
			"--fields", "name,visibleInAppStore",
			"--include", "territories",
			"--territory-fields", "currency",
			"--territory-limit", "2",
			"--paginate",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "Warning: App-tag territories are deprecated in API 4.5; remove territory selections and lookups. Requests are still forwarded for compatibility.\n" {
		t.Fatalf("expected one deprecation warning, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"tag-page-1"`) || !strings.Contains(stdout, `"id":"tag-page-2"`) {
		t.Fatalf("expected both paginated tags in output, got %q", stdout)
	}
}

func TestAppTagsListValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
		isHelp  bool
	}{
		{
			name:    "invalid fields",
			args:    []string{"app-tags", "list", "--app", "app-1", "--paginate", "--fields", "name,bad"},
			wantErr: "app-tags list: --fields must be one of:",
			isHelp:  true,
		},
		{
			name:    "territory fields require include",
			args:    []string{"app-tags", "list", "--app", "app-1", "--paginate", "--territory-fields", "currency"},
			wantErr: "Error: --territory-fields requires --include territories",
			isHelp:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if test.isHelp {
				if !errors.Is(runErr, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", runErr)
				}
				if !strings.Contains(stderr, test.wantErr) {
					t.Fatalf("expected stderr %q, got %q", test.wantErr, stderr)
				}
			} else {
				if errors.Is(runErr, flag.ErrHelp) {
					t.Fatalf("expected non-help error, got %v", runErr)
				}
				if !strings.Contains(runErr.Error(), test.wantErr) {
					t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
				}
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestAppTagsListMarkdownOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appTags"), asctest.JSON(http.StatusOK, `{
			"data":[{"type":"appTags","id":"tag-md-1","attributes":{"name":"Featured","visibleInAppStore":true}}],
			"links":{"next":""}
		}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"app-tags", "list", "--app", "app-1", "--output", "markdown"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "tag-md-1") {
		t.Fatalf("expected markdown output to contain app tag id, got %q", stdout)
	}
}
