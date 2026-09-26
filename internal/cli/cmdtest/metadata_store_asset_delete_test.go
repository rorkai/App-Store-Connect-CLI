package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Replacing a header uses DELETE before reservation; --confirm alone must not
// bypass the deletion authorization offered by metadata push/apply.
func TestMetadataHeaderReplacementRequiresAllowDeletes(t *testing.T) {
	for _, tc := range []struct {
		command              string
		allowDeletes, dryRun bool
	}{{"push", false, false}, {"apply", false, false}, {"push", true, false}, {"push", false, true}} {
		t.Run(fmt.Sprintf("%s/deletes=%t/dry=%t", tc.command, tc.allowDeletes, tc.dryRun), func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			t.Chdir(t.TempDir())
			root := t.TempDir()
			dir := filepath.Join(root, "en-US", "app_clip")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			writePNGForMigrate(t, filepath.Join(dir, "header_image.png"), 1800, 1200)
			var mutations []string
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				key := req.Method + " " + req.URL.Path
				if req.Method != http.MethodGet {
					mutations = append(mutations, key)
				}
				switch key {
				case "GET /v1/apps/APP_ID/appStoreVersions":
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`), nil
				case "GET /v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
					return migrateJSONResponse(200, `{"data":[]}`), nil
				case "GET /v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
					return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"EXP","attributes":{"action":"OPEN"}}}`), nil
				case "GET /v1/appClipDefaultExperiences/EXP/appClipDefaultExperienceLocalizations":
					return migrateJSONResponse(200, `{"data":[{"type":"appClipDefaultExperienceLocalizations","id":"LOC","attributes":{"locale":"en-US"}}]}`), nil
				case "GET /v1/appClipDefaultExperienceLocalizations/LOC/appClipHeaderImage":
					return migrateJSONResponse(200, `{"data":{"type":"appClipHeaderImages","id":"OLD_HEADER","attributes":{"sourceFileChecksum":"old"}}}`), nil
				case "DELETE /v1/appClipHeaderImages/OLD_HEADER":
					return migrateJSONResponse(204, ``), nil
				case "POST /v1/appClipHeaderImages":
					return migrateJSONResponse(400, `{"errors":[{"status":"400","code":"INVALID","title":"reservation rejected"}]}`), nil
				default:
					return nil, fmt.Errorf("unexpected %s", key)
				}
			}))
			args := []string{"metadata", tc.command, "--app", "APP_ID", "--version", "1.0", "--dir", root, "--include", "app-clip", "--output", "json"}
			if tc.dryRun {
				args = append(args, "--dry-run")
			} else {
				args = append(args, "--confirm")
			}
			if tc.allowDeletes {
				args = append(args, "--allow-deletes")
			}
			var runErr error
			stdout, _ := captureOutput(t, func() { runErr = RootCommand("test").ParseAndRun(context.Background(), args) })
			if tc.dryRun {
				if runErr != nil {
					t.Fatal(runErr)
				}
				if len(mutations) > 0 {
					t.Fatalf("dry-run mutated: %v", mutations)
				}
				var plan struct {
					Deletes []struct {
						Scope  string `json:"scope"`
						Reason string `json:"reason"`
						From   string `json:"from"`
					} `json:"deletes"`
				}
				if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
					t.Fatal(err)
				}
				if len(plan.Deletes) != 1 || plan.Deletes[0].Scope != "store-assets" || plan.Deletes[0].Reason != "replace" || !strings.Contains(plan.Deletes[0].From, "OLD_HEADER") {
					t.Fatalf("dry-run concealed replacement deletion: %s", stdout)
				}
				return
			}
			if !tc.allowDeletes {
				if runErr == nil || !strings.Contains(runErr.Error(), "--allow-deletes") {
					t.Errorf("expected deletion authorization error, got %v", runErr)
				}
				if len(mutations) != 0 {
					t.Errorf("mutated without deletion authorization: %v", mutations)
				}
				return
			}
			if runErr == nil {
				t.Fatal("expected reservation failure")
			}
			if len(mutations) != 2 || mutations[0] != "DELETE /v1/appClipHeaderImages/OLD_HEADER" || mutations[1] != "POST /v1/appClipHeaderImages" {
				t.Fatalf("authorized replacement mutations=%v", mutations)
			}
			var receipt struct {
				AssetResults []struct {
					PreviousID      string `json:"previousId"`
					PreviousDeleted bool   `json:"previousDeleted"`
					Status          string `json:"status"`
				} `json:"assetResults"`
			}
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatalf("decode partial receipt: %v; %s", err, stdout)
			}
			if len(receipt.AssetResults) != 1 || receipt.AssetResults[0].PreviousID != "OLD_HEADER" || !receipt.AssetResults[0].PreviousDeleted || receipt.AssetResults[0].Status != "failed" {
				t.Fatalf("missing authorized replacement receipt: %s", stdout)
			}
		})
	}
}
