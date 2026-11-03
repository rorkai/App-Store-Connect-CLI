package cmdtest

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCustomPagesCreateSendsTemplateRelationship(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want map[string]any
	}{
		{"app store version", []string{"--from-version", "version-1"}, map[string]any{"appStoreVersionTemplate": map[string]any{"data": map[string]any{"type": "appStoreVersions", "id": "version-1"}}}},
		{"custom product page", []string{"--from-custom-page", "page-1"}, map[string]any{"customProductPageTemplate": map[string]any{"data": map[string]any{"type": "appCustomProductPages", "id": "page-1"}}}},
		{"no template", nil, map[string]any{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1" {
					return jsonResponse(200, `{"data":{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}}}`)
				}
				if req.Method != http.MethodPost || req.URL.Path != "/v1/appCustomProductPages" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				var body struct {
					Data struct {
						Relationships map[string]any `json:"relationships"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				got := map[string]any{}
				for _, name := range []string{"appStoreVersionTemplate", "customProductPageTemplate"} {
					if value, ok := body.Data.Relationships[name]; ok {
						got[name] = value
					}
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Fatalf("template relationships = %v, want %v", got, tt.want)
				}
				return jsonResponse(201, `{"data":{"type":"appCustomProductPages","id":"new-page","attributes":{"name":"Summer"}}}`)
			})
			root := RootCommand("test")
			args := append([]string{"product-pages", "custom-pages", "create", "--app", "app-1", "--name", "Summer", "--output", "json"}, tt.args...)
			var err error
			stdout, stderr := captureOutput(t, func() { err = root.ParseAndRun(context.Background(), args) })
			if err != nil || !strings.Contains(stdout, `"id":"new-page"`) {
				t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout, stderr)
			}
		})
	}
}

func TestCustomPagesCreateRejectsTwoTemplatesBeforeHTTP(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("two templates caused HTTP")
		return nil, nil
	})
	root := RootCommand("test")
	var err error
	stdout, stderr := captureOutput(t, func() {
		err = root.ParseAndRun(context.Background(), []string{"product-pages", "custom-pages", "create", "--app", "app-1", "--name", "Summer", "--from-version", "version-1", "--from-custom-page", "page-1"})
	})
	if !isUsageClassError(err) || stdout != "" || !strings.Contains(stderr, "--from-version and --from-custom-page are mutually exclusive") {
		t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
}
