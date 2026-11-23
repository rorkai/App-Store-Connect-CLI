package validate

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

// A first App Store version can carry any version number (issue #4043), so
// the What's New warning must follow the app's release history.
func TestBuildReadinessReport_WhatsNewFollowsReleaseHistory(t *testing.T) {
	tests := []struct {
		name             string
		releasedVersions string
		wantWarning      bool
	}{
		{name: "first release numbered 1.3", releasedVersions: `{"data":[]}`, wantWarning: false},
		{name: "update after a released version", releasedVersions: `{"data":[{"type":"appStoreVersions","id":"ver-0","attributes":{"platform":"IOS","versionString":"1.2","appStoreState":"READY_FOR_SALE"}}]}`, wantWarning: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var mu sync.Mutex
			historyReads := 0
			client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/appStoreVersions/ver-1":
					return buildsJSONResponse(http.StatusOK, `{
						"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.3","appVersionState":"PREPARE_FOR_SUBMISSION"},"relationships":{
							"app":{"data":{"type":"apps","id":"app-1"}},
							"appStoreVersionLocalizations":{"data":[{"type":"appStoreVersionLocalizations","id":"ver-loc-1"}],"meta":{"paging":{"total":1,"limit":50}}},
							"build":{"data":null},
							"appStoreReviewDetail":{"data":null}
						}},
						"included":[{"type":"appStoreVersionLocalizations","id":"ver-loc-1","attributes":{"locale":"en-US","description":"desc","keywords":"kw","supportUrl":"https://example.com"}}]
					}`)
				case "/v1/apps/app-1/appInfos":
					return buildsJSONResponse(http.StatusOK, `{
						"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"},"relationships":{
							"app":{"data":{"type":"apps","id":"app-1"}},
							"ageRatingDeclaration":{"data":null},
							"appInfoLocalizations":{"data":[],"meta":{"paging":{"total":0,"limit":50}}},
							"primaryCategory":{"data":null}
						}}],
						"included":[{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}}]
					}`)
				case "/v1/apps/app-1/appStoreVersions":
					mu.Lock()
					historyReads++
					mu.Unlock()
					return buildsJSONResponse(http.StatusOK, test.releasedVersions)
				case "/v1/apps/app-1/appPriceSchedule", "/v1/apps/app-1/appAvailabilityV2":
					return buildsJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`)
				default:
					return buildsJSONResponse(http.StatusOK, `{"data":[]}`)
				}
			}))

			report, err := BuildReadinessReportWith(context.Background(), readinessTestDeps(client), ReadinessOptions{AppID: "app-1", VersionID: "ver-1"})
			if err != nil {
				t.Fatalf("BuildReadinessReport() error = %v", err)
			}

			gotWarning := false
			for _, check := range report.Checks {
				if check.ID == "metadata.required.whats_new" {
					gotWarning = true
				}
			}
			if gotWarning != test.wantWarning {
				t.Fatalf("whatsNew warning = %v, want %v", gotWarning, test.wantWarning)
			}
			mu.Lock()
			defer mu.Unlock()
			if historyReads == 0 {
				t.Fatal("release history was not read")
			}
		})
	}
}
