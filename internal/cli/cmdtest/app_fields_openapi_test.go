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

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// Apple production rejects these spec values (400 "is not a valid field name"
// or HTTP 500), so the CLI must reject them before sending a request.
var appFieldsRejectedEverywhere = []string{"appEncryptionDeclarations"}

func TestAppFieldsAcceptEveryOpenAPIValue(t *testing.T) {
	tests := []struct {
		name        string
		specPath    string
		requestPath string
		args        []string
		rejected    []string
		deps        bool // the command is built on shared.Deps: stub its client and read its deps streams
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
			deps:        true,
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
			rejected: []string{
				"appStoreIcon",
				"performanceOverviews",
				"customerReviewSummarizations",
				"analyticsReportRequests",
				"betaFeedbackScreenshotSubmissions",
				"betaFeedbackCrashSubmissions",
				"webhooks",
				"androidToIosAppMappingDetails",
			},
		},
		{
			name:        "xcode-cloud products list",
			specPath:    "/v1/ciProducts",
			requestPath: "/v1/ciProducts",
			args:        []string{"xcode-cloud", "products", "list", "--app", "123456789", "--include", "app"},
			rejected:    []string{"assetLibrary"},
		},
	}

	for _, test := range tests {
		rejected := append(append([]string{}, appFieldsRejectedEverywhere...), test.rejected...)
		accepted := slices.DeleteFunc(openAPIAppFieldsEnum(t, test.specPath), func(value string) bool {
			return slices.Contains(rejected, value)
		})

		t.Run(test.name, func(t *testing.T) {
			appFields := strings.Join(accepted, ",")
			stderr, requests, runErr := runAppFieldsCommand(t, test.args, appFields, test.requestPath, test.deps)
			if runErr != nil {
				t.Fatalf("run error: %v; stderr=%q", runErr, stderr)
			}
			if requests != 1 {
				t.Fatalf("request count = %d, want 1", requests)
			}
		})

		for _, value := range rejected {
			t.Run(test.name+" rejects "+value, func(t *testing.T) {
				stderr, requests, runErr := runAppFieldsCommand(t, test.args, value, test.requestPath, test.deps)
				if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d; err=%v stderr=%q", got, rootcmd.ExitUsage, runErr, stderr)
				}
				if requests != 0 {
					t.Fatalf("request count = %d, want 0", requests)
				}
			})
		}
	}
}

func runAppFieldsCommand(t *testing.T, commandArgs []string, appFields, requestPath string, deps bool) (string, int32, error) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests.Add(1)
		if req.URL.Path != requestPath {
			t.Errorf("path = %q, want %q", req.URL.Path, requestPath)
		}
		if got := req.URL.Query().Get("fields[apps]"); got != appFields {
			t.Errorf("fields[apps] = %q, want %q", got, appFields)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"links":{"next":""}}`)
	}))
	t.Cleanup(server.Close)

	args := append(append([]string{}, commandArgs...), "--app-fields", appFields, "--output", "json")
	if deps {
		_, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server), args...)
		return stderr, requests.Load(), runErr
	}
	installDefaultTransportForServer(t, server)
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stderr, requests.Load(), runErr
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
