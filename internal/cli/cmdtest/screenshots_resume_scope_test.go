package cmdtest

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRunScreenshotsResumeRequiresIndependentScopeAndLiveMembership(t *testing.T) {
	for _, tc := range []struct {
		name                                         string
		pending, upload, drift, ownerDrift, unscoped bool
		change                                       string
		pagination, nextURL                          string
		relative                                     bool
		wantWrites                                   int
	}{
		{name: "scoped reorder", wantWrites: 1},
		{name: "scoped pending commit", pending: true, wantWrites: 2},
		{name: "unscoped receipt", unscoped: true},
		{name: "wrong selected app", change: "app"},
		{name: "changed receipt localization", change: "localization"},
		{name: "changed receipt display", change: "display"},
		{name: "changed receipt set", change: "set"},
		{name: "foreign pending asset", pending: true, change: "pending"},
		{name: "foreign order member", change: "order"},
		{name: "omitted order member", change: "omitted"},
		{name: "duplicate order member", change: "duplicate"},
		{name: "membership drift before reorder", drift: true},
		{name: "membership drift before pending read", pending: true, drift: true},
		{name: "membership drift before reservation", upload: true, drift: true},
		{name: "ownership drift before reorder", ownerDrift: true},
		{name: "owned absolute pagination", pagination: "all", wantWrites: 1},
		{name: "owned relative pagination", pagination: "all", relative: true, wantWrites: 1},
		{name: "foreign localization continuation", pagination: "sets", nextURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/other/appScreenshotSets?cursor=outside"},
		{name: "foreign inventory continuation", pagination: "inventory", nextURL: "https://api.appstoreconnect.apple.com/v1/appScreenshotSets/other/appScreenshots?cursor=outside"},
		{name: "foreign ordering continuation", pagination: "order", nextURL: "https://api.appstoreconnect.apple.com/v1/appScreenshotSets/other/relationships/appScreenshots?cursor=outside"},
		{name: "inventory changed collection", pagination: "inventory", nextURL: "https://api.appstoreconnect.apple.com/v1/appScreenshotSets/set-1/relationships/appScreenshots?cursor=outside"},
		{name: "ordering changed collection", pagination: "order", nextURL: "https://api.appstoreconnect.apple.com/v1/appScreenshotSets/set-1/appScreenshots?cursor=outside"},
		{name: "encoded collection alias", pagination: "inventory", nextURL: "https://api.appstoreconnect.apple.com/v1/appScreenshotSets/%73et-1/appScreenshots?cursor=outside"},
		{name: "query-only continuation", pagination: "sets", nextURL: "?cursor=outside"},
		{name: "untrusted continuation origin", pagination: "inventory", nextURL: "https://invalid.example/v1/appScreenshotSets/set-1/appScreenshots?cursor=outside"},
		{name: "continuation userinfo", pagination: "order", nextURL: "https://wrong@api.appstoreconnect.apple.com/v1/appScreenshotSets/set-1/relationships/appScreenshots?cursor=outside"},
		{name: "alternate supported origin sets", pagination: "sets", nextURL: "https://api.enterprise.developer.apple.com/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets?cursor=outside"},
		{name: "alternate supported origin inventory", pagination: "inventory", nextURL: "https://api.enterprise.developer.apple.com/v1/appScreenshotSets/set-1/appScreenshots?cursor=outside"},
		{name: "alternate supported origin ordering", pagination: "order", nextURL: "https://api.enterprise.developer.apple.com/v1/appScreenshotSets/set-1/relationships/appScreenshots?cursor=outside"},
		{name: "continuation fragment", pagination: "order", nextURL: "https://api.appstoreconnect.apple.com/v1/appScreenshotSets/set-1/relationships/appScreenshots?cursor=outside#foreign"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			dir := t.TempDir()
			file := writeCmdtestScreenshotPNG(t, dir, "01-home.png")
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			checksum := fmt.Sprintf("%x", md5.Sum(data))
			members := []string{"two", "one"}
			ordered := []string{"one", "two"}
			if tc.pending {
				members = []string{"pending", "one"}
				ordered = []string{"one"}
			}
			if tc.upload {
				members = []string{"one"}
				ordered = []string{"one"}
			}
			record := map[string]any{"versionLocalizationId": "LOC_123", "displayType": "APP_IPHONE_65", "setId": "set-1", "orderedIds": ordered}
			if tc.pending || tc.upload {
				record["rootPath"] = dir
				record["files"] = []string{file}
				record["pendingFiles"] = []string{file}
			}
			if tc.pending {
				assetID := "pending"
				if tc.change == "pending" {
					assetID = "foreign"
				}
				record["pendingAssets"] = []map[string]string{{"assetId": assetID, "fileName": filepath.Base(file), "filePath": file, "checksum": checksum, "state": "UPLOADED"}}
			}
			switch tc.change {
			case "localization":
				record["versionLocalizationId"] = "other-localization"
			case "display":
				record["displayType"] = "APP_IPAD_PRO_3GEN_129"
			case "set":
				record["setId"] = "other-set"
			case "order":
				record["orderedIds"] = []string{"one", "foreign"}
			case "omitted":
				record["orderedIds"] = []string{"one"}
			case "duplicate":
				record["orderedIds"] = []string{"one", "one", "two"}
			}
			payload, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			artifact := filepath.Join(dir, "resume.json")
			if err := os.WriteFile(artifact, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			writes, detailReads, inventoryReads, ownerReads := 0, 0, 0, 0
			pageReads, offScopeReads := 0, 0
			committed := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method != http.MethodGet {
					writes++
				}
				if r.URL.Query().Get("cursor") == "outside" {
					offScopeReads++
					if err := json.NewEncoder(w).Encode(map[string]any{"data": []any{}}); err != nil {
						t.Error(err)
					}
					return
				}
				collection := func(kind string, data []map[string]any) map[string]any {
					response := map[string]any{"data": data}
					if tc.pagination != "all" && tc.pagination != kind {
						return response
					}
					if r.URL.Query().Get("cursor") == "page-2" {
						pageReads++
						if kind != "sets" {
							response["data"] = data[1:]
						}
						return response
					}
					next := tc.nextURL
					if next == "" {
						next = r.URL.Path + "?cursor=page-2&limit=1"
						if !tc.relative {
							next = "https://api.appstoreconnect.apple.com" + next
						}
						if kind == "sets" {
							response["data"] = []map[string]any{}
						} else {
							response["data"] = data[:1]
						}
					}
					response["links"] = map[string]string{"next": next}
					return response
				}
				var response any
				switch r.Method + " " + r.URL.Path {
				case "GET /v1/appStoreVersionLocalizations/LOC_123":
					q := r.URL.Query()
					if q.Get("include") != "appStoreVersion" || q.Get("fields[appStoreVersionLocalizations]") != "appStoreVersion" || q.Get("fields[appStoreVersions]") != "app" {
						t.Errorf("unexpected localization query: %s", r.URL)
					}
					response = map[string]any{"data": map[string]any{"type": "appStoreVersionLocalizations", "id": "LOC_123", "relationships": map[string]any{"appStoreVersion": map[string]any{"data": map[string]string{"type": "appStoreVersions", "id": "VERSION_123"}}}}}
				case "GET /v1/appStoreVersions/VERSION_123":
					ownerReads++
					appID := "100"
					if tc.ownerDrift && ownerReads > 1 {
						appID = "200"
					}
					response = map[string]any{"data": map[string]any{"type": "appStoreVersions", "id": "VERSION_123", "attributes": map[string]string{"platform": "IOS"}, "relationships": map[string]any{"app": map[string]any{"data": map[string]string{"type": "apps", "id": appID}}}}}
				case "GET /v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
					response = collection("sets", []map[string]any{{"type": "appScreenshotSets", "id": "set-1", "attributes": map[string]string{"screenshotDisplayType": "APP_IPHONE_65"}}})
				case "GET /v1/appScreenshotSets/set-1/appScreenshots":
					inventoryReads++
					if tc.drift && inventoryReads > 1 && !slices.Contains(members, "intruder") {
						members = append(members, "intruder")
					}
					resources := make([]map[string]any, 0, len(members))
					for _, id := range members {
						item := map[string]any{"type": "appScreenshots", "id": id}
						if id == "pending" && committed {
							item["attributes"] = map[string]any{"sourceFileChecksum": checksum, "assetDeliveryState": map[string]string{"state": "COMPLETE"}}
						}
						resources = append(resources, item)
					}
					response = collection("inventory", resources)
				case "GET /v1/appScreenshotSets/set-1/relationships/appScreenshots":
					resources := make([]map[string]any, 0, len(members))
					for _, id := range members {
						resources = append(resources, map[string]any{"type": "appScreenshots", "id": id})
					}
					response = collection("order", resources)
				case "GET /v1/appScreenshots/pending":
					detailReads++
					state := "AWAITING_UPLOAD"
					if committed {
						state = "COMPLETE"
					}
					response = map[string]any{"data": map[string]any{"type": "appScreenshots", "id": "pending", "attributes": map[string]any{"sourceFileChecksum": checksum, "assetDeliveryState": map[string]string{"state": state}}}}
				case "PATCH /v1/appScreenshots/pending":
					committed = true
					response = map[string]any{"data": map[string]any{"type": "appScreenshots", "id": "pending", "attributes": map[string]any{"sourceFileChecksum": checksum, "assetDeliveryState": map[string]string{"state": "COMPLETE"}}}}
				case "PATCH /v1/appScreenshotSets/set-1/relationships/appScreenshots":
					var body struct{ Data []struct{ ID string } }
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					ids := make([]string, 0, len(body.Data))
					for _, item := range body.Data {
						ids = append(ids, item.ID)
					}
					expected := []string{"one", "two"}
					if tc.pending {
						expected = []string{"one", "pending"}
					}
					if !slices.Equal(ids, expected) {
						t.Errorf("unexpected proposed order: %v", ids)
					}
					w.WriteHeader(http.StatusNoContent)
					return
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			args := []string{"screenshots", "upload", "--resume", artifact, "--output", "json"}
			if !tc.unscoped {
				appID := "100"
				if tc.change == "app" {
					appID = "200"
				}
				args = append(args, "--app", appID, "--version-localization", "LOC_123", "--device-type", "IPHONE_65")
			}
			stdout, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server), args...)
			if (runErr == nil) != (tc.wantWrites > 0) || writes != tc.wantWrites {
				t.Fatalf("writes=%d want=%d stdout=%s stderr=%s err=%v", writes, tc.wantWrites, stdout, stderr, runErr)
			}
			if offScopeReads != 0 {
				t.Fatalf("sent %d off-scope continuation reads", offScopeReads)
			}
			if tc.pagination == "all" && pageReads != 6 {
				t.Fatalf("continuation reads = %d, want 6", pageReads)
			}
			if tc.nextURL != "" && !strings.Contains(stderr+fmt.Sprint(runErr), "screenshot pagination") {
				t.Fatalf("missing pagination refusal: %s %v", stderr, runErr)
			}
			if tc.wantWrites == 0 && detailReads != 0 {
				t.Fatalf("denied resume read %d pending asset details", detailReads)
			}
			if tc.unscoped && !strings.Contains(stderr+fmt.Sprint(runErr), "--resume requires --app") {
				t.Fatalf("missing scope diagnostic: %s %v", stderr, runErr)
			}
		})
	}
}
