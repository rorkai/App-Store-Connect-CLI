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
)

func TestAppTagsListOutputAndQueryOptions(t *testing.T) {
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
		if req.URL.Path != "/v1/apps/app-1/appTags" {
			t.Fatalf("expected path /v1/apps/app-1/appTags, got %s", req.URL.Path)
		}

		query := req.URL.Query()
		if query.Get("filter[visibleInAppStore]") != "true,false" {
			t.Fatalf("expected visibility filter true,false, got %q", query.Get("filter[visibleInAppStore]"))
		}
		if query.Get("sort") != "-name" {
			t.Fatalf("expected sort -name, got %q", query.Get("sort"))
		}
		if query.Get("fields[appTags]") != "name,visibleInAppStore" {
			t.Fatalf("expected fields[appTags], got %q", query.Get("fields[appTags]"))
		}
		if query.Get("include") != "territories" {
			t.Fatalf("expected include territories, got %q", query.Get("include"))
		}
		if query.Get("fields[territories]") != "currency" {
			t.Fatalf("expected territory fields currency, got %q", query.Get("fields[territories]"))
		}
		if query.Get("limit[territories]") != "2" {
			t.Fatalf("expected territory limit 2, got %q", query.Get("limit[territories]"))
		}
		if query.Get("limit") != "5" {
			t.Fatalf("expected limit 5, got %q", query.Get("limit"))
		}

		body := `{
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
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

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

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/appTags" {
			t.Fatalf("expected path /v1/apps/app-1/appTags, got %s", req.URL.Path)
		}
		body := `{"errors":[{"status":"403","title":"Forbidden","detail":"not allowed"}]}`
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appTags" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit 200 for paginate first page, got %q", req.URL.Query().Get("limit"))
			}
			body := `{
				"data":[{"type":"appTags","id":"tag-1"}],
				"links":{"next":"` + repeatedNextURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != repeatedNextURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"type":"appTags","id":"tag-2"}],
				"links":{"next":"` + repeatedNextURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/appTags" {
			t.Fatalf("expected path /v1/apps/app-1/appTags, got %s", req.URL.Path)
		}
		body := `{"data":[{"type":"appTags","id":"tag-1"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unsupported output",
			args:    []string{"app-tags", "list", "--app", "app-1", "--output", "yaml"},
			wantErr: "unsupported format: yaml",
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

			if !errors.Is(runErr, flag.ErrHelp) {
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

func TestAppTagsListRejectsInvalidNextURL(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-tags", "list",
			"--next", "http://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?cursor=AQ",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "app-tags list: --next must be an App Store Connect URL") {
		t.Fatalf("expected invalid --next error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestAppTagsListPaginateWithFiltersUsesQueryOptions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?cursor=BQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appTags" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[visibleInAppStore]") != "true,false" {
				t.Fatalf("expected visibility filter true,false, got %q", query.Get("filter[visibleInAppStore]"))
			}
			if query.Get("sort") != "-name" {
				t.Fatalf("expected sort -name, got %q", query.Get("sort"))
			}
			if query.Get("fields[appTags]") != "name,visibleInAppStore" {
				t.Fatalf("expected app tag fields, got %q", query.Get("fields[appTags]"))
			}
			if query.Get("include") != "territories" {
				t.Fatalf("expected include territories, got %q", query.Get("include"))
			}
			if query.Get("fields[territories]") != "currency" {
				t.Fatalf("expected territory fields currency, got %q", query.Get("fields[territories]"))
			}
			if query.Get("limit[territories]") != "2" {
				t.Fatalf("expected territory limit 2, got %q", query.Get("limit[territories]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected paginate first-page limit=200, got %q", query.Get("limit"))
			}

			body := `{
				"data":[{"type":"appTags","id":"tag-page-1"}],
				"links":{"next":"` + secondURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"type":"appTags","id":"tag-page-2"}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

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

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
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

func TestAppTagsListPaginateFromNextWithoutApp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?cursor=BQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"type":"appTags","id":"tag-next-1"}],
				"links":{"next":"` + secondURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"type":"appTags","id":"tag-next-2"}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"app-tags", "list", "--paginate", "--next", firstURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"tag-next-1"`) || !strings.Contains(stdout, `"id":"tag-next-2"`) {
		t.Fatalf("expected both paginated tags in output, got %q", stdout)
	}
}

func TestAppTagsListMarkdownOutput(t *testing.T) {
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
		if req.URL.Path != "/v1/apps/app-1/appTags" {
			t.Fatalf("expected path /v1/apps/app-1/appTags, got %s", req.URL.Path)
		}
		body := `{
			"data":[{"type":"appTags","id":"tag-md-1","attributes":{"name":"Featured","visibleInAppStore":true}}],
			"links":{"next":""}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

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
