package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMetadataPreviewPlanIncludesPrerequisites(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	for _, localeExists := range []bool{false, true} {
		t.Run(fmt.Sprintf("locale-exists=%v", localeExists), func(t *testing.T) {
			setupAuth(t)
			t.Chdir(t.TempDir())
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			root := t.TempDir()
			for _, device := range []string{"iphone_65", "iphone_67"} {
				dir := filepath.Join(root, "app_previews", "en-US", device)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(dir, "preview.mp4"), "fixture-video")
			}
			installValidStoreAssetProbe(t)
			mutations := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					mutations++
					return nil, fmt.Errorf("unexpected mutation %s %s", req.Method, req.URL.Path)
				}
				switch req.URL.Path {
				case "/v1/apps/APP_ID/appStoreVersions":
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`), nil
				case "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
					if !localeExists {
						return migrateJSONResponse(200, `{"data":[]}`), nil
					}
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersionLocalizations","id":"LOC","attributes":{"locale":"en-US"}}]}`), nil
				case "/v1/appStoreVersionLocalizations/LOC/appPreviewSets":
					return migrateJSONResponse(200, `{"data":[]}`), nil
				default:
					return nil, fmt.Errorf("unexpected request %s", req.URL.Path)
				}
			}))
			type planResult struct {
				Adds []struct{ Key, Reason, To string }
			}
			check := func(plan planResult) {
				t.Helper()
				localizations, sets, uploads := 0, 0, 0
				for _, item := range plan.Adds {
					switch {
					case strings.HasPrefix(item.Key, "store-assets/version_localization/"):
						localizations++
						if item.Reason != "create" || !strings.Contains(item.To, "VERSION_ID") || !strings.Contains(item.To, "en-US") {
							t.Errorf("localization creation lacks target identity: %+v", item)
						}
					case strings.HasPrefix(item.Key, "store-assets/preview_set/"):
						sets++
						parent := "VERSION_ID"
						if localeExists {
							parent = "LOC"
						}
						if item.Reason != "create" || !strings.Contains(item.To, parent) || (!strings.Contains(item.To, "IPHONE_65") && !strings.Contains(item.To, "IPHONE_67")) {
							t.Errorf("set creation lacks parent/device identity: %+v", item)
						}
					case strings.HasPrefix(item.Key, "store-assets/preview/"):
						uploads++
					}
				}
				wantLocalizations := 1
				if localeExists {
					wantLocalizations = 0
				}
				if localizations != wantLocalizations || sets != 2 || uploads != 2 {
					t.Fatalf("planned localizations=%d sets=%d uploads=%d; want %d/2/2: %+v", localizations, sets, uploads, wantLocalizations, plan.Adds)
				}
			}
			var runErr error
			stdout, _ := captureOutput(t, func() {
				runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "push", "--app", "APP_ID", "--version", "1.0", "--dir", root, "--include", "previews", "--dry-run", "--output", "json"})
			})
			if runErr != nil {
				t.Fatal(runErr)
			}
			var dryRun planResult
			if err := json.Unmarshal([]byte(stdout), &dryRun); err != nil {
				t.Fatal(err)
			}
			check(dryRun)
			reviewDir := t.TempDir()
			stdout = runMetadataReviewCommand(t, []string{"metadata", "plan", "--app", "APP_ID", "--version", "1.0", "--dir", root, "--include", "previews", "--review-dir", reviewDir, "--output", "json"})
			var review struct {
				PlanHash string
				Plan     planResult
			}
			if err := json.Unmarshal([]byte(stdout), &review); err != nil {
				t.Fatal(err)
			}
			if review.PlanHash == "" {
				t.Fatal("review has no hash")
			}
			check(review.Plan)
			artifact, err := os.ReadFile(filepath.Join(reviewDir, "plan.json"))
			if err != nil {
				t.Fatal(err)
			}
			var saved struct {
				PlanHash string
				Plan     planResult
			}
			if err := json.Unmarshal(artifact, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.PlanHash != review.PlanHash {
				t.Fatal("saved review hash differs")
			}
			check(saved.Plan)
			if mutations != 0 {
				t.Fatalf("planning mutated remote resources %d times", mutations)
			}
		})
	}
}
