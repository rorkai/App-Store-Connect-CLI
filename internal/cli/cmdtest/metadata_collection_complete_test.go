package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// A default-only push must not silently omit locales advertised by Apple.
func TestMetadataPushRequiresCompleteLocalizationCollections(t *testing.T) {
	for _, scope := range []struct{ name, dir, collection, resource, field string }{
		{"app-info", "app-info", "/v1/appInfos/appinfo-1/appInfoLocalizations", "appInfoLocalizations", "subtitle"},
		{"version", "version/1.0", "/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "appStoreVersionLocalizations", "keywords"},
	} {
		for _, sample := range []struct {
			name, meta, lastMeta       string
			paginated, duplicate, fail bool
			wantWrites                 int
		}{
			{"missing total", "", "", false, false, false, 1},
			{"complete total", `,"meta":{"paging":{"total":1,"limit":200}}`, "", false, false, false, 1},
			{"missing next", `,"meta":{"paging":{"total":2,"limit":200}}`, "", false, false, true, 0},
			{"complete pagination", `,"meta":{"paging":{"total":2,"limit":200}}`, "", true, false, false, 2},
			{"duplicate page", `,"meta":{"paging":{"total":2,"limit":200}}`, "", true, true, true, 0},
			{"later total", "", `,"meta":{"paging":{"total":3,"limit":200}}`, true, false, true, 0},
		} {
			t.Run(scope.name+"/"+sample.name, func(t *testing.T) {
				dir := t.TempDir()
				target := filepath.Join(dir, scope.dir, "default.json")
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte(fmt.Sprintf(`{%q:"fresh"}`, scope.field)), 0o600); err != nil {
					t.Fatal(err)
				}
				var writes atomic.Int32
				pages := 0
				client := stubClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == http.MethodPatch {
						writes.Add(1)
						var body struct {
							Data struct {
								ID         string
								Attributes map[string]string
							}
						}
						if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
							t.Fatal(err)
						}
						if !strings.HasPrefix(req.URL.Path, "/v1/"+scope.resource+"/loc-") || len(body.Data.Attributes) != 1 || body.Data.Attributes[scope.field] != "fresh" {
							t.Fatalf("unexpected mutation %s %+v", req.URL.Path, body)
						}
						return jsonResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":%q,"id":%q,"attributes":{"locale":"en-US",%q:"fresh"}}}`, scope.resource, body.Data.ID, scope.field))
					}
					if req.Method != http.MethodGet {
						t.Fatalf("unexpected request %s %s", req.Method, req.URL)
					}
					switch req.URL.Path {
					case "/v1/apps/123456789/appStoreVersions":
						return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}],"links":{}}`)
					case "/v1/apps/123456789/appInfos":
						return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}],"links":{}}`)
					case scope.collection:
						pages++
						locale, id, next := "en-US", "loc-en", ""
						if sample.paginated && pages == 1 {
							next = scope.collection + "?cursor=next"
						}
						meta := sample.meta
						if pages == 2 {
							meta = sample.lastMeta
							if !sample.duplicate {
								locale, id = "fr-FR", "loc-fr"
							}
						}
						return jsonResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":%q,"id":%q,"attributes":{"locale":%q,%q:"old"}}],"links":{"next":%q}%s}`, scope.resource, id, locale, scope.field, next, meta))
					default:
						t.Fatalf("unexpected request %s", req.URL)
						return nil, nil
					}
				}))
				stdout, stderr, code := runWithDeps(t, client, "metadata", "push", "--app", "123456789", "--version", "1.0", "--dir", dir, "--output", "json")
				if sample.fail {
					if code == 0 || stdout != "" || !strings.Contains(stderr, "incomplete "+scope.name+" localizations: received ") {
						t.Errorf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
					}
				} else {
					var result struct {
						Applied          bool
						Total, Succeeded int
					}
					if err := json.Unmarshal([]byte(stdout), &result); err != nil {
						t.Fatalf("receipt: %v; stdout=%s stderr=%s", err, stdout, stderr)
					}
					if code != 0 || stderr != "" || !result.Applied || result.Total != sample.wantWrites || result.Succeeded != sample.wantWrites {
						t.Errorf("exit=%d receipt=%+v stderr=%s", code, result, stderr)
					}
				}
				if int(writes.Load()) != sample.wantWrites || pages != map[bool]int{false: 1, true: 2}[sample.paginated] {
					t.Errorf("writes=%d pages=%d", writes.Load(), pages)
				}
			})
		}
	}
}
