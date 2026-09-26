package cmdtest

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestXcodeCloudProductsTableHydratesBundleIDFromIncludedApp(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantQuery url.Values
	}{
		{
			name: "default selection",
			args: []string{"--limit", "1"},
			wantQuery: url.Values{
				"include":      {"app"},
				"fields[apps]": {"bundleId"},
				"limit":        {"1"},
			},
		},
		{
			name: "sparse product fields keep the app relationship",
			args: []string{"--fields", "name"},
			wantQuery: url.Values{
				"fields[ciProducts]": {"name,app"},
				"include":            {"app"},
				"fields[apps]":       {"bundleId"},
			},
		},
		{
			name: "requested app fields are preserved",
			args: []string{"--include", "app", "--app-fields", "name"},
			wantQuery: url.Values{
				"include":      {"app"},
				"fields[apps]": {"name,bundleId"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})

			var requested []url.Values
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/ciProducts" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				requested = append(requested, req.URL.Query())
				body := `{"data":[{"type":"ciProducts","id":"prod-1","attributes":{"name":"FoundationLab","createdDate":"2025-06-25T08:55:49.429Z","productType":"APP"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}],` +
					`"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"com.rudrankriyam.foundationlab"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			args := append([]string{"xcode-cloud", "products", "list", "--output", "table"}, test.args...)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, "Bundle ID") {
				t.Fatalf("expected Bundle ID header, got %q", stdout)
			}
			if !strings.Contains(stdout, "com.rudrankriyam.foundationlab") {
				t.Fatalf("expected bundle ID value in output, got %q", stdout)
			}
			if len(requested) != 1 {
				t.Fatalf("expected exactly 1 request, got %d", len(requested))
			}
			if got := requested[0].Encode(); got != test.wantQuery.Encode() {
				t.Fatalf("query = %q, want %q", got, test.wantQuery.Encode())
			}
		})
	}
}

func TestXcodeCloudProductsTableFallsBackToRelatedAppForContinuationPages(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var requestedPaths []string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestedPaths = append(requestedPaths, req.URL.Path)

		var body string
		switch req.URL.Path {
		case "/v1/ciProducts":
			if req.URL.RawQuery != "cursor=PAGE2" {
				t.Fatalf("continuation query = %q, want %q", req.URL.RawQuery, "cursor=PAGE2")
			}
			body = `{"data":[{"type":"ciProducts","id":"prod-1","attributes":{"name":"FoundationLab","productType":"APP"}}]}`
		case "/v1/ciProducts/prod-1/app":
			body = `{"data":{"type":"apps","id":"app-1","attributes":{"bundleId":"com.rudrankriyam.foundationlab"}}}`
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}

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
			"xcode-cloud", "products", "list",
			"--next", "https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=PAGE2",
			"--output", "table",
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
	if !strings.Contains(stdout, "com.rudrankriyam.foundationlab") {
		t.Fatalf("expected bundle ID value in output, got %q", stdout)
	}
	want := []string{"/v1/ciProducts", "/v1/ciProducts/prod-1/app"}
	if strings.Join(requestedPaths, ",") != strings.Join(want, ",") {
		t.Fatalf("requests = %v, want %v", requestedPaths, want)
	}
}

func TestXcodeCloudProductsPaginatedTableHydratesBundleIDsFromIncludedApps(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	const nextURL = "https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=PAGE2&include=app&fields%5Bapps%5D=bundleId"
	var requested []url.Values
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/ciProducts" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		requested = append(requested, req.URL.Query())

		var body string
		switch len(requested) {
		case 1:
			body = `{"data":[{"type":"ciProducts","id":"prod-1","attributes":{"name":"First","productType":"APP"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}],` +
				`"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"com.example.first"}}],` +
				`"links":{"next":"` + nextURL + `"}}`
		case 2:
			body = `{"data":[{"type":"ciProducts","id":"prod-2","attributes":{"name":"Second","productType":"APP"},"relationships":{"app":{"data":{"type":"apps","id":"app-2"}}}}],` +
				`"included":[{"type":"apps","id":"app-2","attributes":{"bundleId":"com.example.second"}}],` +
				`"links":{"next":""}}`
		default:
			t.Fatalf("unexpected request count %d", len(requested))
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "products", "list", "--paginate", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if len(requested) != 2 {
		t.Fatalf("expected exactly 2 requests, got %d (stderr=%q)", len(requested), stderr)
	}
	if got := requested[0].Get("include"); got != "app" {
		t.Fatalf("include = %q, want %q", got, "app")
	}
	for _, want := range []string{"com.example.first", "com.example.second"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in output, got %q", want, stdout)
		}
	}
}

func TestXcodeCloudProductsPaginatedTableFallsBackWhenContinuationOmitsInclude(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	const nextURL = "https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=PAGE2"
	var requested []string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.Path+"?"+req.URL.RawQuery)

		var body string
		switch req.URL.Path {
		case "/v1/ciProducts":
			if req.URL.Query().Get("cursor") == "" {
				body = `{"data":[{"type":"ciProducts","id":"prod-1","attributes":{"name":"First","productType":"APP"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}],` +
					`"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"com.example.first"}}],` +
					`"links":{"next":"` + nextURL + `"}}`
			} else {
				body = `{"data":[{"type":"ciProducts","id":"prod-2","attributes":{"name":"Second","productType":"APP"},"relationships":{"app":{"data":{"type":"apps","id":"app-2"}}}}],"links":{"next":""}}`
			}
		case "/v1/ciProducts/prod-2/app":
			body = `{"data":{"type":"apps","id":"app-2","attributes":{"bundleId":"com.example.second"}}}`
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "products", "list", "--paginate", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if len(requested) != 3 {
		t.Fatalf("requests = %v, want the list, the continuation page, and a fallback for prod-2 (stderr=%q)", requested, stderr)
	}
	if !strings.HasPrefix(requested[0], "/v1/ciProducts?") || strings.Contains(requested[0], "cursor=") {
		t.Fatalf("first request = %q, want the list query", requested[0])
	}
	if !strings.Contains(requested[1], "/v1/ciProducts?cursor=PAGE2") {
		t.Fatalf("continuation request = %q, want cursor=PAGE2", requested[1])
	}
	if !strings.HasPrefix(requested[2], "/v1/ciProducts/prod-2/app?") {
		t.Fatalf("fallback request = %q, want prod-2 related app", requested[2])
	}
	for _, bundleID := range []string{"com.example.first", "com.example.second"} {
		if !strings.Contains(stdout, bundleID) {
			t.Fatalf("expected %q in output, got %q", bundleID, stdout)
		}
	}
}
