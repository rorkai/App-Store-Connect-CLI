package bundleids

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestReconcileKeepsKnownMemoryRequestsUnmanaged(t *testing.T) {
	for _, item := range []struct{ key, capability string }{
		{"com.apple.developer.kernel.increased-memory-limit", "INCREASED_MEMORY_LIMIT"},
		{"com.apple.developer.kernel.increased-debugging-memory-limit", "INCREASED_MEMORY_LIMIT_DEBUGGING"},
	} {
		for _, mode := range []string{"plan", "ignore-unknown", "apply"} {
			t.Run(item.capability+"/"+mode, func(t *testing.T) {
				writes := 0
				client := newReconcileClient(t, func(req *http.Request) *http.Response {
					if req.Method != http.MethodGet {
						writes++
						t.Fatalf("unmanaged request allowed mutation: %s %s", req.Method, req.URL.Path)
					}
					switch req.URL.Path {
					case "/v1/bundleIds/bundle-1":
						return reconcileJSON(200, `{"data":{"type":"bundleIds","id":"bundle-1"}}`)
					case "/v1/bundleIds/bundle-1/bundleIdCapabilities":
						return reconcileJSON(200, `{"data":[]}`)
					default:
						t.Fatalf("unexpected read %s", req.URL.Path)
						return nil
					}
				})
				d, out, _ := newReconcileDeps(func() (*asc.Client, error) { return client, nil })
				path := writeEntitlements(t, map[string]any{item.key: true, "aps-environment": "development"})
				apply := mode == "apply"
				name := "plan"
				if apply {
					name = "apply"
				}
				args := []string{"--bundle", "bundle-1", "--entitlements", path, "--output", "json"}
				if apply {
					args = append(args, "--confirm")
				}
				if mode == "ignore-unknown" {
					args = append(args, "--ignore-unknown")
				}
				err := capabilityReconcileCommand(d, name, apply).ParseAndRun(context.Background(), args)
				if apply {
					if err == nil || !strings.Contains(err.Error(), "unmanaged") {
						t.Fatalf("apply error=%v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				var result struct {
					Actions []asc.CapabilityReconcileAction `json:"actions"`
				}
				if err := json.Unmarshal(out.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				found := false
				for _, action := range result.Actions {
					if action.Entitlement != item.key {
						continue
					}
					found = true
					if action.Action != "unmanaged" || action.Capability != item.capability || action.Status != "unverified" || action.Error == "" || action.Command != "" {
						t.Fatalf("unsupported operation claimed: %+v", action)
					}
				}
				if !found || writes != 0 {
					t.Fatalf("found=%v writes=%d receipt=%s", found, writes, out)
				}
			})
		}
	}
}
