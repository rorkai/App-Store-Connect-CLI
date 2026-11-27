package bundleids

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestReconcileReportsEveryIdentifierWithoutInferringLinks(t *testing.T) {
	for _, format := range []string{"json", "table", "markdown"} {
		for _, present := range []bool{false, true} {
			t.Run(format+"/"+map[bool]string{false: "no capabilities", true: "capabilities enabled"}[present], func(t *testing.T) {
				client := newReconcileClient(t, func(req *http.Request) *http.Response {
					if req.Method != http.MethodGet {
						t.Fatalf("unexpected mutation %s %s", req.Method, req.URL.Path)
					}
					switch req.URL.Path {
					case "/v1/bundleIds/bundle-1":
						return reconcileJSON(200, `{"data":{"id":"bundle-1","type":"bundleIds","attributes":{"identifier":"com.example.app"}}}`)
					case "/v1/bundleIds/bundle-1/bundleIdCapabilities":
						if !present {
							return reconcileJSON(200, `{"data":[]}`)
						}
						return reconcileJSON(200, `{"data":[{"id":"groups","attributes":{"capabilityType":"APP_GROUPS"}},{"id":"pay","attributes":{"capabilityType":"APPLE_PAY"}},{"id":"cloud","attributes":{"capabilityType":"ICLOUD"}}]}`)
					default:
						t.Fatalf("unexpected lookup %s", req.URL.Path)
						return nil
					}
				})
				d, out, _ := newReconcileDeps(func() (*asc.Client, error) { return client, nil })
				path := writeEntitlements(t, map[string]any{
					"com.apple.security.application-groups":              []string{"group.example.shared", "group.example.shared"},
					"com.apple.developer.in-app-payments":                []string{"merchant.example"},
					"com.apple.developer.icloud-container-identifiers":   []string{"iCloud.example.main"},
					"com.apple.developer.icloud-services":                []string{"CloudKit"},
					"com.apple.developer.ubiquity-container-identifiers": []string{"TEAM.iCloud.example.legacy"},
				})
				cmd := capabilityReconcileCommand(d, "plan", false)
				if err := cmd.ParseAndRun(context.Background(), []string{"--bundle", "bundle-1", "--entitlements", path, "--output", format}); err != nil {
					t.Fatal(err)
				}
				want := map[string]string{"com.apple.security.application-groups": "group.example.shared", "com.apple.developer.in-app-payments": "merchant.example", "com.apple.developer.icloud-container-identifiers": "iCloud.example.main", "com.apple.developer.ubiquity-container-identifiers": "TEAM.iCloud.example.legacy"}
				if format != "json" {
					text := out.String()
					for _, identifier := range want {
						if !strings.Contains(text, identifier) {
							t.Fatalf("missing identifier %q in %s: %s", identifier, format, text)
						}
					}
					if !strings.Contains(text, "unverified") || !strings.Contains(strings.ToLower(text), "identifier") {
						t.Fatalf("missing linkage evidence in %s: %s", format, text)
					}
					return
				}

				var result struct {
					IdentifierLinks []struct{ Entitlement, Identifier, Status, Reason, Command string } `json:"identifierLinks"`
					Actions         []struct{ Capability string }                                       `json:"actions"`
				}
				if err := json.Unmarshal(out.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				got := map[string]string{}
				for _, link := range result.IdentifierLinks {
					if link.Status != "unverified" || link.Reason == "" || link.Command != "asc web bundle-ids list --output json" {
						t.Fatalf("unsupported linkage claim: %+v", link)
					}
					got[link.Entitlement] = link.Identifier
				}
				if len(result.IdentifierLinks) != 4 || !reflect.DeepEqual(got, want) || len(result.Actions) != 3 {
					t.Fatalf("identifiers=%#v links=%d actions=%#v", got, len(result.IdentifierLinks), result.Actions)
				}
			})
		}
	}
}
