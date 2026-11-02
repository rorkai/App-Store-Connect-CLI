package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAppFieldsAcceptEveryOpenAPIValue(t *testing.T) {
	tests := []struct {
		name        string
		specPath    string
		requestPath string
		args        []string
	}{
		{
			name:        "bundle-ids list",
			specPath:    "/v1/bundleIds",
			requestPath: "/v1/bundleIds",
			args:        []string{"bundle-ids", "list", "--include", "app"},
		},
		{
			name:        "product-pages custom-pages list",
			specPath:    "/v1/apps/{id}/appCustomProductPages",
			requestPath: "/v1/apps/app-1/appCustomProductPages",
			args:        []string{"product-pages", "custom-pages", "list", "--app", "app-1", "--include", "app"},
		},
		{
			name:        "testflight groups list",
			specPath:    "/v1/betaGroups",
			requestPath: "/v1/betaGroups",
			args:        []string{"testflight", "groups", "list", "--app", "app-1", "--include", "app"},
		},
		{
			name:        "users list",
			specPath:    "/v1/users",
			requestPath: "/v1/users",
			args:        []string{"users", "list", "--include", "visibleApps"},
		},
		{
			name:        "xcode-cloud products list",
			specPath:    "/v1/ciProducts",
			requestPath: "/v1/ciProducts",
			args:        []string{"xcode-cloud", "products", "list", "--app", "123456789", "--include", "app"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			enum := openAPIAppFieldsEnum(t, test.specPath)
			if got := shared.AppFieldsList(); !slices.Equal(got, enum) {
				t.Fatalf("shared.AppFieldsList() = %v, want %v", got, enum)
			}
			appFields := strings.Join(enum, ",")
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests.Add(1)
				if req.URL.Path != test.requestPath {
					t.Errorf("path = %q, want %q", req.URL.Path, test.requestPath)
				}
				if got := req.URL.Query().Get("fields[apps]"); got != appFields {
					t.Errorf("fields[apps] = %q, want %q", got, appFields)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":[],"links":{"next":""}}`)
			}))
			t.Cleanup(server.Close)
			installDefaultTransportForServer(t, server)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append(append([]string{}, test.args...), "--app-fields", appFields, "--output", "json")
			var runErr error
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if runErr != nil {
				t.Fatalf("run error: %v; stderr=%q", runErr, stderr)
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("request count = %d, want 1", got)
			}
		})
	}
}

func openAPIAppFieldsEnum(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "openapi", "latest.json"))
	if err != nil {
		t.Fatalf("read OpenAPI schema: %v", err)
	}
	var schema struct {
		Paths map[string]struct {
			Get struct {
				Parameters []struct {
					Name   string `json:"name"`
					Schema struct {
						Items struct {
							Enum []string `json:"enum"`
						} `json:"items"`
					} `json:"schema"`
				} `json:"parameters"`
			} `json:"get"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode OpenAPI schema: %v", err)
	}
	for _, parameter := range schema.Paths[path].Get.Parameters {
		if parameter.Name == "fields[apps]" {
			return parameter.Schema.Items.Enum
		}
	}
	t.Fatalf("GET %s has no fields[apps] parameter", path)
	return nil
}
