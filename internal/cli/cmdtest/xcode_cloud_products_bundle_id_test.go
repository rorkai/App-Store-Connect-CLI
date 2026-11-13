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

func TestXcodeCloudProductsTableHydratesBundleIDsAcrossPages(t *testing.T) {
	type step struct {
		path     string
		rawQuery string
		body     string
	}
	const firstPage = `{"data":[{"type":"ciProducts","id":"prod-1","attributes":{"name":"First","productType":"APP"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}],` +
		`"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"com.example.first"}}],`
	tests := []struct {
		name          string
		args          []string
		steps         []step
		wantBundleIDs []string
	}{
		{
			name: "continuation page falls back to related app",
			args: []string{"--next", "https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=PAGE2"},
			steps: []step{
				{"/v1/ciProducts", "cursor=PAGE2", `{"data":[{"type":"ciProducts","id":"prod-1","attributes":{"name":"FoundationLab","productType":"APP"}}]}`},
				{"/v1/ciProducts/prod-1/app", "", `{"data":{"type":"apps","id":"app-1","attributes":{"bundleId":"com.rudrankriyam.foundationlab"}}}`},
			},
			wantBundleIDs: []string{"com.rudrankriyam.foundationlab"},
		},
		{
			name: "paginated pages hydrate from included apps",
			args: []string{"--paginate"},
			steps: []step{
				{"/v1/ciProducts", "fields%5Bapps%5D=bundleId&include=app&limit=200", firstPage + `"links":{"next":"https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=PAGE2&include=app&fields%5Bapps%5D=bundleId"}}`},
				{"/v1/ciProducts", "cursor=PAGE2&include=app&fields%5Bapps%5D=bundleId", `{"data":[{"type":"ciProducts","id":"prod-2","attributes":{"name":"Second","productType":"APP"},"relationships":{"app":{"data":{"type":"apps","id":"app-2"}}}}],` +
					`"included":[{"type":"apps","id":"app-2","attributes":{"bundleId":"com.example.second"}}],"links":{"next":""}}`},
			},
			wantBundleIDs: []string{"com.example.first", "com.example.second"},
		},
		{
			name: "paginated continuation without include falls back to related app",
			args: []string{"--paginate"},
			steps: []step{
				{"/v1/ciProducts", "fields%5Bapps%5D=bundleId&include=app&limit=200", firstPage + `"links":{"next":"https://api.appstoreconnect.apple.com/v1/ciProducts?cursor=PAGE2"}}`},
				{"/v1/ciProducts", "cursor=PAGE2", `{"data":[{"type":"ciProducts","id":"prod-2","attributes":{"name":"Second","productType":"APP"},"relationships":{"app":{"data":{"type":"apps","id":"app-2"}}}}],"links":{"next":""}}`},
				{"/v1/ciProducts/prod-2/app", "", `{"data":{"type":"apps","id":"app-2","attributes":{"bundleId":"com.example.second"}}}`},
			},
			wantBundleIDs: []string{"com.example.first", "com.example.second"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			callCount := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount++
				if callCount > len(test.steps) {
					t.Fatalf("unexpected request %d: %s %s", callCount, req.Method, req.URL.String())
				}
				want := test.steps[callCount-1]
				if req.Method != http.MethodGet || req.URL.Path != want.path || req.URL.RawQuery != want.rawQuery {
					t.Fatalf("request %d = %s %s?%s, want GET %s?%s", callCount, req.Method, req.URL.Path, req.URL.RawQuery, want.path, want.rawQuery)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(want.body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			args := append([]string{"xcode-cloud", "products", "list"}, test.args...)
			args = append(args, "--output", "table")
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
			if callCount != len(test.steps) {
				t.Fatalf("requests = %d, want %d", callCount, len(test.steps))
			}
			for _, bundleID := range test.wantBundleIDs {
				if !strings.Contains(stdout, bundleID) {
					t.Fatalf("expected %q in output, got %q", bundleID, stdout)
				}
			}
		})
	}
}
