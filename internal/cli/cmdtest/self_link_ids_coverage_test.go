package cmdtest

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// These tests close the gaps left after the first two self-link passes: the
// localization media-set commands, flags built by the shared command
// builders, and the bare --id flags. The guard test at the bottom walks the
// real command tree so a resource ID flag registered later cannot silently
// skip the normalizer.

func TestSelfLinkCoverageSendsExtractedID(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
		wantID   string
		bodies   map[string]string
	}{
		{
			name:     "localizations preview-sets list",
			args:     []string{"localizations", "preview-sets", "list", "--localization-id", selfLinkTestBase + "/v1/appStoreVersionLocalizations/loc-1"},
			wantPath: "GET /v1/appStoreVersionLocalizations/loc-1/appPreviewSets",
			wantID:   "preview-set-1",
			bodies: map[string]string{
				"/v1/appStoreVersionLocalizations/loc-1/appPreviewSets": `{"data":[{"type":"appPreviewSets","id":"preview-set-1","attributes":{"previewType":"IPHONE_67"}}],"links":{}}`,
			},
		},
		{
			name:     "localizations preview-sets links",
			args:     []string{"localizations", "preview-sets", "links", "--localization-id", selfLinkTestBase + "/v1/appStoreVersionLocalizations/loc-1"},
			wantPath: "GET /v1/appStoreVersionLocalizations/loc-1/relationships/appPreviewSets",
			wantID:   "preview-set-1",
			bodies: map[string]string{
				"/v1/appStoreVersionLocalizations/loc-1/relationships/appPreviewSets": `{"data":[{"type":"appPreviewSets","id":"preview-set-1"}],"links":{}}`,
			},
		},
		{
			name:     "localizations preview-sets view",
			args:     []string{"localizations", "preview-sets", "view", "--id", selfLinkTestBase + "/v1/appPreviewSets/preview-set-1"},
			wantPath: "GET /v1/appPreviewSets/preview-set-1",
			wantID:   "preview-set-1",
			bodies: map[string]string{
				"/v1/appPreviewSets/preview-set-1": `{"data":{"type":"appPreviewSets","id":"preview-set-1","attributes":{"previewType":"IPHONE_67"}}}`,
			},
		},
		{
			name:     "localizations screenshot-sets list",
			args:     []string{"localizations", "screenshot-sets", "list", "--localization-id", selfLinkTestBase + "/v1/appStoreVersionLocalizations/loc-1"},
			wantPath: "GET /v1/appStoreVersionLocalizations/loc-1/appScreenshotSets",
			wantID:   "screenshot-set-1",
			bodies: map[string]string{
				"/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets": `{"data":[{"type":"appScreenshotSets","id":"screenshot-set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_67"}}],"links":{}}`,
			},
		},
		{
			name:     "localizations screenshot-sets view",
			args:     []string{"localizations", "screenshot-sets", "view", "--id", selfLinkTestBase + "/v1/appScreenshotSets/screenshot-set-1"},
			wantPath: "GET /v1/appScreenshotSets/screenshot-set-1",
			wantID:   "screenshot-set-1",
			bodies: map[string]string{
				"/v1/appScreenshotSets/screenshot-set-1": `{"data":{"type":"appScreenshotSets","id":"screenshot-set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_67"}}}`,
			},
		},
		{
			name:     "localizations screenshot-sets delete",
			args:     []string{"localizations", "screenshot-sets", "delete", "--id", selfLinkTestBase + "/v1/appScreenshotSets/screenshot-set-1", "--confirm"},
			wantPath: "DELETE /v1/appScreenshotSets/screenshot-set-1",
			wantID:   "screenshot-set-1",
			bodies: map[string]string{
				"/v1/appScreenshotSets/screenshot-set-1": ``,
			},
		},
		{
			name:     "video-previews list",
			args:     []string{"video-previews", "list", "--version-localization", selfLinkTestBase + "/v1/appStoreVersionLocalizations/loc-1"},
			wantPath: "GET /v1/appStoreVersionLocalizations/loc-1/appPreviewSets",
			wantID:   "preview-set-1",
			bodies: map[string]string{
				"/v1/appStoreVersionLocalizations/loc-1/appPreviewSets":      `{"data":[{"type":"appPreviewSets","id":"preview-set-1","attributes":{"previewType":"IPHONE_67"}}],"links":{}}`,
				"/v1/appPreviewSets/preview-set-1/appPreviews":               `{"data":[],"links":{}}`,
				"/v1/appPreviewSets/preview-set-1/relationships/appPreviews": `{"data":[],"links":{}}`,
			},
		},
		{
			name:     "screenshots list",
			args:     []string{"screenshots", "list", "--version-localization", selfLinkTestBase + "/v1/appStoreVersionLocalizations/loc-1"},
			wantPath: "GET /v1/appStoreVersionLocalizations/loc-1/appScreenshotSets",
			wantID:   "screenshot-set-1",
			bodies: map[string]string{
				"/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":            `{"data":[{"type":"appScreenshotSets","id":"screenshot-set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_67"}}],"links":{}}`,
				"/v1/appScreenshotSets/screenshot-set-1/appScreenshots":               `{"data":[],"links":{}}`,
				"/v1/appScreenshotSets/screenshot-set-1/relationships/appScreenshots": `{"data":[],"links":{}}`,
			},
		},
		{
			name:     "localizations supported-locales",
			args:     []string{"localizations", "supported-locales", "--version", selfLinkTestBase + "/v1/appStoreVersions/version-1"},
			wantPath: "GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations",
			wantID:   "en-US",
			bodies: map[string]string{
				"/v1/appStoreVersions/version-1/appStoreVersionLocalizations": `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{}}`,
			},
		},
		{
			name:     "subscriptions groups view",
			args:     []string{"subscriptions", "groups", "view", "--id", selfLinkTestBase + "/v1/subscriptionGroups/group-1"},
			wantPath: "GET /v1/subscriptionGroups/group-1",
			wantID:   "group-1",
			bodies: map[string]string{
				"/v1/subscriptionGroups/group-1": `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`,
			},
		},
		{
			name:     "game-center leaderboards view",
			args:     []string{"game-center", "leaderboards", "view", "--id", selfLinkTestBase + "/v1/gameCenterLeaderboards/leaderboard-1"},
			wantPath: "GET /v1/gameCenterLeaderboards/leaderboard-1",
			wantID:   "leaderboard-1",
			bodies: map[string]string{
				"/v1/gameCenterLeaderboards/leaderboard-1": `{"data":{"type":"gameCenterLeaderboards","id":"leaderboard-1","attributes":{"referenceName":"High score"}}}`,
			},
		},
		{
			name:     "xcode-cloud products view",
			args:     []string{"xcode-cloud", "products", "view", "--id", selfLinkTestBase + "/v1/ciProducts/product-1"},
			wantPath: "GET /v1/ciProducts/product-1",
			wantID:   "product-1",
			bodies: map[string]string{
				"/v1/ciProducts/product-1": `{"data":{"type":"ciProducts","id":"product-1","attributes":{"name":"App"}}}`,
			},
		},
		{
			name:     "xcode-cloud scm providers view",
			args:     []string{"xcode-cloud", "scm", "providers", "view", "--provider-id", selfLinkTestBase + "/v1/scmProviders/provider-1"},
			wantPath: "GET /v1/scmProviders/provider-1",
			wantID:   "provider-1",
			bodies: map[string]string{
				"/v1/scmProviders/provider-1": `{"data":{"type":"scmProviders","id":"provider-1","attributes":{}}}`,
			},
		},
		{
			name:     "subscriptions offers offer-codes custom-codes view",
			args:     []string{"subscriptions", "offers", "offer-codes", "custom-codes", "view", "--custom-code-id", selfLinkTestBase + "/v1/subscriptionOfferCodeCustomCodes/custom-1"},
			wantPath: "GET /v1/subscriptionOfferCodeCustomCodes/custom-1",
			wantID:   "custom-1",
			bodies: map[string]string{
				"/v1/subscriptionOfferCodeCustomCodes/custom-1": `{"data":{"type":"subscriptionOfferCodeCustomCodes","id":"custom-1","attributes":{"customCode":"SPRING"}}}`,
			},
		},
		{
			name:     "users view",
			args:     []string{"users", "view", "--id", selfLinkTestBase + "/v1/users/user-1"},
			wantPath: "GET /v1/users/user-1",
			wantID:   "user-1",
			bodies: map[string]string{
				"/v1/users/user-1": `{"data":{"type":"users","id":"user-1","attributes":{"username":"dev@example.com"}}}`,
			},
		},
		{
			name:     "reviews view",
			args:     []string{"reviews", "view", "--id", selfLinkTestBase + "/v1/customerReviews/review-1"},
			wantPath: "GET /v1/customerReviews/review-1",
			wantID:   "review-1",
			bodies: map[string]string{
				"/v1/customerReviews/review-1": `{"data":{"type":"customerReviews","id":"review-1","attributes":{"rating":5}}}`,
			},
		},
		{
			name:     "testflight testers view",
			args:     []string{"testflight", "testers", "view", "--id", selfLinkTestBase + "/v1/betaTesters/tester-1"},
			wantPath: "GET /v1/betaTesters/tester-1",
			wantID:   "tester-1",
			bodies: map[string]string{
				"/v1/betaTesters/tester-1": `{"data":{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}}`,
			},
		},
		{
			name:     "profiles view",
			args:     []string{"profiles", "view", "--id", selfLinkTestBase + "/v1/profiles/profile-1"},
			wantPath: "GET /v1/profiles/profile-1",
			wantID:   "profile-1",
			bodies: map[string]string{
				"/v1/profiles/profile-1": `{"data":{"type":"profiles","id":"profile-1","attributes":{"name":"Dev"}}}`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")

			recorder := &selfLinkRemainingServer{bodies: test.bodies}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				recorder.record(req.Method + " " + req.URL.Path)
				body, ok := test.bodies[req.URL.Path]
				w.Header().Set("Content-Type", "application/json")
				if !ok {
					w.WriteHeader(http.StatusNotFound)
					_, _ = io.WriteString(w, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"not found"}]}`)
					return
				}
				if body == "" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				_, _ = io.WriteString(w, body)
			}))
			t.Cleanup(server.Close)
			installDefaultTransportForServer(t, server)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append(append([]string(nil), test.args...), "--output", "json")
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v (requests %q)", err, recorder.requested())
				}
			})
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, fmt.Sprintf("%q", test.wantID)) {
				t.Fatalf("stdout = %q, want id %q", stdout, test.wantID)
			}
			requested := recorder.requested()
			sort.Strings(requested)
			found := false
			for _, request := range requested {
				if strings.Contains(request, "api.appstoreconnect.apple.com") || strings.Contains(request, "%2F") {
					t.Fatalf("request path leaked the self-link: %q", request)
				}
				if request == test.wantPath {
					found = true
				}
			}
			if !found {
				t.Fatalf("requests = %q, want one equal to %q", requested, test.wantPath)
			}
		})
	}
}

// TestSelfLinkCoverageAppFlagsSendExtractedID covers the --app flags that read
// the raw flag value instead of going through shared.ResolveAppID. The stub
// answers every request with 404, so only the first request path matters.
func TestSelfLinkCoverageAppFlagsSendExtractedID(t *testing.T) {
	appLink := selfLinkTestBase + "/v1/apps/123456789"
	tests := []struct {
		name     string
		args     []string
		wantPath string
	}{
		{
			name:     "performance download",
			args:     []string{"performance", "download", "--app", appLink, "--output", "metrics.json"},
			wantPath: "GET /v1/apps/123456789/perfPowerMetrics",
		},
		{
			name:     "signing fetch",
			args:     []string{"signing", "fetch", "--app", appLink, "--bundle-id", "com.example.app", "--profile-type", "IOS_APP_STORE"},
			wantPath: "GET /v1/apps/123456789",
		},
		{
			name:     "app-setup info set",
			args:     []string{"app-setup", "info", "set", "--app", appLink, "--primary-locale", "en-US"},
			wantPath: "PATCH /v1/apps/123456789",
		},
		{
			name:     "app-setup categories set",
			args:     []string{"app-setup", "categories", "set", "--app", appLink, "--primary", "GAMES"},
			wantPath: "GET /v1/apps/123456789/appInfos",
		},
		{
			name:     "categories set",
			args:     []string{"categories", "set", "--app", appLink, "--primary", "GAMES"},
			wantPath: "GET /v1/apps/123456789/appInfos",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")
			t.Chdir(t.TempDir())

			recorder := &selfLinkRemainingServer{}
			stubTransport(t, func(req *http.Request) (*http.Response, error) {
				recorder.record(req.Method + " " + req.URL.Path)
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404","code":"NOT_FOUND","title":"not found"}]}`)),
					Request:    req,
				}, nil
			})

			_, stderr := captureOutput(t, func() {
				_ = rootcmd.Run(test.args, "1.2.3")
			})
			requested := recorder.requested()
			if len(requested) == 0 {
				t.Fatalf("expected a request, got none (stderr %q)", stderr)
			}
			for _, request := range requested {
				if strings.Contains(request, "https:") || strings.Contains(request, "api.appstoreconnect.apple.com") {
					t.Fatalf("request path leaked the self-link: %q", request)
				}
			}
			if requested[0] != test.wantPath {
				t.Fatalf("first request = %q, want %q (all %q)", requested[0], test.wantPath, requested)
			}
		})
	}
}

func TestSelfLinkCoverageRejectsWrongType(t *testing.T) {
	resetCmdtestState()
	setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	appsLink := selfLinkTestBase + "/v1/apps/123"
	tests := []struct {
		args     []string
		flag     string
		wantType string
	}{
		// Localization media sets and the asset commands that read them.
		{args: []string{"localizations", "preview-sets", "list"}, flag: "localization-id", wantType: "appStoreVersionLocalizations"},
		{args: []string{"localizations", "preview-sets", "links"}, flag: "localization-id", wantType: "appStoreVersionLocalizations"},
		{args: []string{"localizations", "preview-sets", "view"}, flag: "id", wantType: "appPreviewSets"},
		{args: []string{"localizations", "screenshot-sets", "list"}, flag: "localization-id", wantType: "appStoreVersionLocalizations"},
		{args: []string{"localizations", "screenshot-sets", "links"}, flag: "localization-id", wantType: "appStoreVersionLocalizations"},
		{args: []string{"localizations", "screenshot-sets", "view"}, flag: "id", wantType: "appScreenshotSets"},
		{args: []string{"localizations", "screenshot-sets", "delete"}, flag: "id", wantType: "appScreenshotSets"},
		{args: []string{"localizations", "create"}, flag: "version", wantType: "appStoreVersions"},
		{args: []string{"localizations", "update"}, flag: "version", wantType: "appStoreVersions"},
		{args: []string{"localizations", "update"}, flag: "app-info", wantType: "appInfos"},
		{args: []string{"localizations", "supported-locales"}, flag: "version", wantType: "appStoreVersions"},
		{args: []string{"screenshots", "list"}, flag: "version-localization", wantType: "appStoreVersionLocalizations"},
		{args: []string{"screenshots", "upload"}, flag: "version-localization", wantType: "appStoreVersionLocalizations"},
		{args: []string{"screenshots", "download"}, flag: "version-localization", wantType: "appStoreVersionLocalizations"},
		{args: []string{"screenshots", "download"}, flag: "id", wantType: "appScreenshots"},
		{args: []string{"screenshots", "delete"}, flag: "id", wantType: "appScreenshots"},
		{args: []string{"video-previews", "list"}, flag: "version-localization", wantType: "appStoreVersionLocalizations"},
		{args: []string{"video-previews", "upload"}, flag: "version-localization", wantType: "appStoreVersionLocalizations"},
		{args: []string{"video-previews", "download"}, flag: "id", wantType: "appPreviews"},
		{args: []string{"video-previews", "delete"}, flag: "id", wantType: "appPreviews"},
		{args: []string{"video-previews", "set-poster-frame"}, flag: "id", wantType: "appPreviews"},
		{args: []string{"build-localizations", "view"}, flag: "id", wantType: "appStoreVersionLocalizations"},
		{args: []string{"diff", "localizations"}, flag: "from-version", wantType: "appStoreVersions"},
		{args: []string{"diff", "localizations"}, flag: "to-version", wantType: "appStoreVersions"},
		{args: []string{"app-setup", "localizations", "upload"}, flag: "version", wantType: "appStoreVersions"},
		{args: []string{"app-setup", "categories", "set"}, flag: "app-info", wantType: "appInfos"},
		{args: []string{"app-setup", "categories", "set"}, flag: "primary", wantType: "appCategories"},
		{args: []string{"categories", "set"}, flag: "secondary-subcategory-two", wantType: "appCategories"},
		{args: []string{"metadata", "keywords", "audit"}, flag: "app-info", wantType: "appInfos"},
		// Shared command builders.
		{args: []string{"xcode-cloud", "products", "view"}, flag: "id", wantType: "ciProducts"},
		{args: []string{"xcode-cloud", "products", "workflows"}, flag: "id", wantType: "ciProducts"},
		{args: []string{"xcode-cloud", "workflows", "delete"}, flag: "id", wantType: "ciWorkflows"},
		{args: []string{"xcode-cloud", "actions", "view"}, flag: "id", wantType: "ciBuildActions"},
		{args: []string{"xcode-cloud", "build-runs", "view"}, flag: "id", wantType: "ciBuildRuns"},
		{args: []string{"xcode-cloud", "artifacts", "view"}, flag: "id", wantType: "ciArtifacts"},
		{args: []string{"xcode-cloud", "issues", "view"}, flag: "id", wantType: "ciIssues"},
		{args: []string{"xcode-cloud", "test-results", "view"}, flag: "id", wantType: "ciTestResults"},
		{args: []string{"xcode-cloud", "macos-versions", "view"}, flag: "id", wantType: "ciMacOsVersions"},
		{args: []string{"xcode-cloud", "xcode-versions", "view"}, flag: "id", wantType: "ciXcodeVersions"},
		{args: []string{"xcode-cloud", "scm", "providers", "view"}, flag: "provider-id", wantType: "scmProviders"},
		{args: []string{"xcode-cloud", "scm", "repositories", "git-references"}, flag: "repo-id", wantType: "scmRepositories"},
		{args: []string{"xcode-cloud", "scm", "git-references", "view"}, flag: "id", wantType: "scmGitReferences"},
		{args: []string{"xcode-cloud", "scm", "pull-requests", "view"}, flag: "id", wantType: "scmPullRequests"},
		{args: []string{"subscriptions", "offers", "offer-codes", "custom-codes", "list"}, flag: "offer-code-id", wantType: "subscriptionOfferCodes"},
		{args: []string{"subscriptions", "offers", "offer-codes", "custom-codes", "view"}, flag: "custom-code-id", wantType: "subscriptionOfferCodeCustomCodes"},
		{args: []string{"subscriptions", "offers", "win-back", "view"}, flag: "id", wantType: "winBackOffers"},
		{args: []string{"pricing", "schedule", "automatic-prices"}, flag: "schedule", wantType: "appPriceSchedules"},
		{args: []string{"pre-orders", "list"}, flag: "availability", wantType: "appAvailabilities"},
		// Bare --id flags and other single-resource flags.
		{args: []string{"accessibility", "view"}, flag: "id", wantType: "accessibilityDeclarations"},
		{args: []string{"actors", "view"}, flag: "id", wantType: "actors"},
		{args: []string{"age-rating", "edit"}, flag: "id", wantType: "ageRatingDeclarations"},
		{args: []string{"agreements", "territories", "list"}, flag: "id", wantType: "endUserLicenseAgreements"},
		{args: []string{"app-clips", "view"}, flag: "id", wantType: "appClips"},
		{args: []string{"app-clips", "header-images", "view"}, flag: "id", wantType: "appClipHeaderImages"},
		{args: []string{"app-clips", "review-details", "view"}, flag: "id", wantType: "appClipAppStoreReviewDetails"},
		{args: []string{"app-clips", "advanced-experiences", "images", "view"}, flag: "id", wantType: "appClipAdvancedExperienceImages"},
		{args: []string{"app-tags", "view"}, flag: "id", wantType: "appTags"},
		{args: []string{"apps", "ci-product", "view"}, flag: "id", wantType: "apps"},
		{args: []string{"background-assets", "view"}, flag: "id", wantType: "backgroundAssets"},
		{args: []string{"background-assets", "app-store-releases", "view"}, flag: "id", wantType: "backgroundAssetVersionAppStoreReleases"},
		{args: []string{"background-assets", "external-beta-releases", "view"}, flag: "id", wantType: "backgroundAssetVersionExternalBetaReleases"},
		{args: []string{"background-assets", "internal-beta-releases", "view"}, flag: "id", wantType: "backgroundAssetVersionInternalBetaReleases"},
		{args: []string{"builds", "uploads", "view"}, flag: "id", wantType: "buildUploads"},
		{args: []string{"builds", "uploads", "files", "view"}, flag: "id", wantType: "buildUploadFiles"},
		{args: []string{"builds", "uploads", "files", "list"}, flag: "upload", wantType: "buildUploads"},
		{args: []string{"bundle-ids", "capabilities", "update"}, flag: "id", wantType: "bundleIdCapabilities"},
		{args: []string{"bundle-ids", "capabilities", "reconcile", "plan"}, flag: "bundle", wantType: "bundleIds"},
		{args: []string{"certificates", "view"}, flag: "id", wantType: "certificates"},
		{args: []string{"devices", "view"}, flag: "id", wantType: "devices"},
		{args: []string{"encryption", "declarations", "view"}, flag: "id", wantType: "appEncryptionDeclarations"},
		{args: []string{"encryption", "documents", "view"}, flag: "id", wantType: "appEncryptionDeclarationDocuments"},
		{args: []string{"encryption", "documents", "upload"}, flag: "declaration", wantType: "appEncryptionDeclarations"},
		{args: []string{"eula", "view"}, flag: "id", wantType: "endUserLicenseAgreements"},
		{args: []string{"game-center", "achievements", "view"}, flag: "id", wantType: "gameCenterAchievements"},
		{args: []string{"game-center", "achievements", "v2", "localizations", "view"}, flag: "id", wantType: "gameCenterAchievementLocalizations"},
		{args: []string{"game-center", "activities", "releases", "delete"}, flag: "id", wantType: "gameCenterActivityVersionReleases"},
		{args: []string{"game-center", "app-versions", "view"}, flag: "id", wantType: "gameCenterAppVersions"},
		{args: []string{"game-center", "challenges", "images", "view"}, flag: "id", wantType: "gameCenterChallengeImages"},
		{args: []string{"game-center", "details", "view"}, flag: "id", wantType: "gameCenterDetails"},
		{args: []string{"game-center", "groups", "view"}, flag: "id", wantType: "gameCenterGroups"},
		{args: []string{"game-center", "leaderboard-sets", "member-localizations", "view"}, flag: "id", wantType: "gameCenterLeaderboardSetMemberLocalizations"},
		{args: []string{"game-center", "leaderboards", "view"}, flag: "id", wantType: "gameCenterLeaderboards"},
		{args: []string{"game-center", "matchmaking", "teams", "update"}, flag: "id", wantType: "gameCenterMatchmakingTeams"},
		{args: []string{"iap", "pricing", "availabilities", "view"}, flag: "id", wantType: "inAppPurchaseAvailabilities"},
		{args: []string{"iap", "versions", "submit"}, flag: "submission", wantType: "reviewSubmissions"},
		{args: []string{"nominations", "view"}, flag: "id", wantType: "nominations"},
		{args: []string{"pricing", "availability", "view"}, flag: "id", wantType: "appAvailabilities"},
		{args: []string{"pricing", "schedule", "view"}, flag: "id", wantType: "appPriceSchedules"},
		{args: []string{"profiles", "view"}, flag: "id", wantType: "profiles"},
		{args: []string{"profiles", "create"}, flag: "bundle", wantType: "bundleIds"},
		{args: []string{"review", "submissions-get"}, flag: "id", wantType: "reviewSubmissions"},
		{args: []string{"review", "details-get"}, flag: "id", wantType: "appStoreReviewDetails"},
		{args: []string{"review", "attachments-get"}, flag: "id", wantType: "appStoreReviewAttachments"},
		{args: []string{"review", "attachments-list"}, flag: "review-detail", wantType: "appStoreReviewDetails"},
		{args: []string{"review", "items", "update"}, flag: "id", wantType: "reviewSubmissionItems"},
		{args: []string{"review", "items", "add"}, flag: "submission", wantType: "reviewSubmissions"},
		{args: []string{"reviews", "view"}, flag: "id", wantType: "customerReviews"},
		{args: []string{"reviews", "response", "view"}, flag: "id", wantType: "customerReviewResponses"},
		{args: []string{"routing-coverage", "info"}, flag: "id", wantType: "routingAppCoverages"},
		{args: []string{"sandbox", "view"}, flag: "id", wantType: "sandboxTesters"},
		{args: []string{"submit", "status"}, flag: "id", wantType: "reviewSubmissions"},
		{args: []string{"subscriptions", "groups", "view"}, flag: "id", wantType: "subscriptionGroups"},
		{args: []string{"subscriptions", "groups", "versions", "localizations", "view"}, flag: "id", wantType: "subscriptionGroupLocalizations"},
		{args: []string{"subscriptions", "grace-periods", "view"}, flag: "id", wantType: "subscriptionGracePeriods"},
		{args: []string{"subscriptions", "offers", "introductory", "view"}, flag: "id", wantType: "subscriptionIntroductoryOffers"},
		{args: []string{"subscriptions", "offers", "introductory", "create"}, flag: "price-point", wantType: "subscriptionPricePoints"},
		{args: []string{"subscriptions", "offers", "promotional", "view"}, flag: "id", wantType: "subscriptionPromotionalOffers"},
		{args: []string{"subscriptions", "versions", "view"}, flag: "id", wantType: "subscriptionVersions"},
		{args: []string{"subscriptions", "versions", "images", "view"}, flag: "id", wantType: "subscriptionImages"},
		{args: []string{"subscriptions", "versions", "localizations", "view"}, flag: "id", wantType: "subscriptionLocalizations"},
		{args: []string{"testflight", "agreements", "view"}, flag: "id", wantType: "betaLicenseAgreements"},
		{args: []string{"testflight", "app-localizations", "view"}, flag: "id", wantType: "betaAppLocalizations"},
		{args: []string{"testflight", "distribution", "edit"}, flag: "id", wantType: "buildBetaDetails"},
		{args: []string{"testflight", "metrics", "public-link"}, flag: "group", wantType: "betaGroups"},
		{args: []string{"testflight", "metrics", "app-testers"}, flag: "filter-tester", wantType: "betaTesters"},
		{args: []string{"testflight", "pre-release", "view"}, flag: "id", wantType: "preReleaseVersions"},
		{args: []string{"testflight", "recruitment", "delete"}, flag: "id", wantType: "betaRecruitmentCriteria"},
		{args: []string{"testflight", "recruitment", "set"}, flag: "group", wantType: "betaGroups"},
		{args: []string{"testflight", "review", "edit"}, flag: "id", wantType: "betaAppReviewDetails"},
		{args: []string{"testflight", "review", "submissions", "view"}, flag: "id", wantType: "betaAppReviewSubmissions"},
		{args: []string{"testflight", "testers", "view"}, flag: "id", wantType: "betaTesters"},
		{args: []string{"testflight", "testers", "list"}, flag: "group", wantType: "betaGroups"},
		{args: []string{"users", "view"}, flag: "id", wantType: "users"},
		{args: []string{"users", "invites", "view"}, flag: "id", wantType: "userInvitations"},
		{args: []string{"versions", "phased-release", "update"}, flag: "id", wantType: "appStoreVersionPhasedReleases"},
		// --app flags that bypassed the shared app resolver.
		{args: []string{"performance", "download"}, flag: "app", wantType: "apps"},
		{args: []string{"signing", "fetch"}, flag: "app", wantType: "apps"},
		{args: []string{"app-setup", "info", "set"}, flag: "app", wantType: "apps"},
		{args: []string{"app-setup", "categories", "set"}, flag: "app", wantType: "apps"},
		{args: []string{"categories", "set"}, flag: "app", wantType: "apps"},
	}

	for _, test := range tests {
		name := strings.Join(test.args, " ") + " --" + test.flag
		t.Run(name, func(t *testing.T) {
			value := appsLink
			wantErr := fmt.Sprintf("expected a self-link of type %s, got apps", test.wantType)
			if test.wantType == "apps" {
				value = selfLinkTestBase + "/v1/builds/build-1"
				wantErr = "expected a self-link of type apps, got builds"
			}
			args := append(append([]string(nil), test.args...), "--"+test.flag, value)
			stdout, stderr := captureOutput(t, func() {
				code := rootcmd.Run(args, "1.2.3")
				if code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			firstLine, _, _ := strings.Cut(stderr, "\n")
			wantPrefix := fmt.Sprintf("Error: invalid value %q for flag -%s: ", value, test.flag)
			if !strings.HasPrefix(firstLine, wantPrefix) || !strings.Contains(firstLine, wantErr) {
				t.Fatalf("first stderr line = %q, want prefix %q containing %q", firstLine, wantPrefix, wantErr)
			}
		})
	}
}

// selfLinkIDFlagExclusions lists every flag named "id" or "*-id" that is
// deliberately not an App Store Connect API resource ID. Each entry is keyed
// by "<command path> --<flag>" and states why; docs/design/self-link-ids.md
// carries the same exclusions for readers.
var selfLinkIDFlagExclusions = map[string]string{
	// Type depends on another flag.
	"asc localizations update --id":  "appStoreVersionLocalizations or appInfoLocalizations, chosen by --type",
	"asc localizations view --id":    "appStoreVersionLocalizations or appInfoLocalizations, chosen by --type",
	"asc review items add --item-id": "one of five collections, chosen by --item-type",
	"asc review items-add --item-id": "one of five collections, chosen by --item-type",
	// No single-resource /v<n>/<type>/<id> path exists.
	"asc app-clips domain-status cache --build-bundle-id":       "buildBundles has no single-resource path",
	"asc app-clips domain-status debug --build-bundle-id":       "buildBundles has no single-resource path",
	"asc app-clips invocations create --build-bundle-id":        "buildBundles has no single-resource path",
	"asc app-clips invocations list --build-bundle-id":          "buildBundles has no single-resource path",
	"asc build-bundles app-clip cache-status view --id":         "buildBundles has no single-resource path",
	"asc build-bundles app-clip debug-status view --id":         "buildBundles has no single-resource path",
	"asc build-bundles app-clip invocations list --id":          "buildBundles has no single-resource path",
	"asc build-bundles file-sizes list --id":                    "buildBundles has no single-resource path",
	"asc game-center enabled-versions compatible-versions --id": "gameCenterEnabledVersions has no single-resource path",
	"asc iap pricing price-points equalizations --id":           "inAppPurchasePricePoints has no single-resource path",
	"asc iap setup --price-point-id":                            "inAppPurchasePricePoints has no single-resource path",
	"asc performance diagnostics view --id":                     "diagnosticSignatures has no single-resource path",
	"asc performance download --diagnostic-id":                  "diagnosticSignatures has no single-resource path",
	"asc webhooks deliveries redeliver --delivery-id":           "webhookDeliveries has no single-resource path",
	"asc screenshots review-approve --id":                       "local review manifest screenshot name",
	"asc game-center achievements submit --scoped-player-id":    "Game Center player identifier",
	"asc game-center leaderboards submit --scoped-player-id":    "Game Center player identifier",
	"asc subscriptions offers win-back create --offer-id":       "offerId attribute, not a resource ID",
	"asc subscriptions offers retention create --offer-id":      "offerId attribute, not a resource ID",
	"asc validate --apple-id":                                   "cached Apple Account for the web session",
}

// selfLinkIdentifierFlagNames are "-id" flags that always carry an identifier
// or credential rather than an App Store Connect API resource ID.
var selfLinkIdentifierFlagNames = map[string]bool{
	"bundle-id": true, "product-id": true, "vendor-id": true, "team-id": true,
	"key-id": true, "issuer-id": true, "seed-id": true, "adam-id": true,
	"client-id": true, "org-id": true,
}

// selfLinkExcludedCommandTrees name command subtrees that do not address the
// public App Store Connect API: private web-session endpoints (including the
// web-session rating-reset commands), Apple Ads, StoreKit server APIs, the
// notary service, credentials, and local Xcode tooling.
var selfLinkExcludedCommandTrees = []string{
	"asc web", "asc versions rating-reset", "asc ads", "asc storekit",
	"asc notarization", "asc xcode", "asc auth",
}

func TestSelfLinkEveryResourceIDFlagIsNormalized(t *testing.T) {
	root := RootCommand("1.2.3")
	seen := map[string]bool{}
	var missing []string
	var walk func(path string, command *ffcli.Command)
	walk = func(path string, command *ffcli.Command) {
		for _, tree := range selfLinkExcludedCommandTrees {
			if path == tree || strings.HasPrefix(path, tree+" ") {
				return
			}
		}
		if command.FlagSet != nil {
			command.FlagSet.VisitAll(func(f *flag.Flag) {
				if f.Name != "id" && !strings.HasSuffix(f.Name, "-id") {
					return
				}
				key := path + " --" + f.Name
				if _, ok := selfLinkIDFlagExclusions[key]; ok {
					seen[key] = true
					return
				}
				if selfLinkIdentifierFlagNames[f.Name] || isCommaSeparatedIDFlag(f) {
					return
				}
				if resourceType, ok := selfLinkFlagResourceType(f); !ok || resourceType == "" {
					missing = append(missing, key)
				}
			})
		}
		for _, sub := range command.Subcommands {
			walk(path+" "+sub.Name, sub)
		}
	}
	walk("asc", root)

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("resource ID flags without a typed self-link normalizer (bind with shared.BindResourceIDFlag or add a reasoned exclusion):\n%s", strings.Join(missing, "\n"))
	}
	var stale []string
	for key := range selfLinkIDFlagExclusions {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Fatalf("stale self-link exclusions (flag no longer exists):\n%s", strings.Join(stale, "\n"))
	}
}

func isCommaSeparatedIDFlag(f *flag.Flag) bool {
	if strings.Contains(strings.ToLower(f.Usage), "comma-separated") {
		return true
	}
	return fmt.Sprintf("%T", f.Value) == "*shared.OnceCSVValue"
}

// selfLinkFlagResourceType reports the resource type bound to a flag created
// by shared.BindResourceIDFlag.
func selfLinkFlagResourceType(f *flag.Flag) (string, bool) {
	if fmt.Sprintf("%T", f.Value) != "*shared.resourceIDValue" {
		return "", false
	}
	field := reflect.ValueOf(f.Value).Elem().FieldByName("resourceType")
	if !field.IsValid() {
		return "", false
	}
	return field.String(), true
}
