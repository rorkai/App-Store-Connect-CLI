package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

const includedFieldsNextURL = "https://api.appstoreconnect.apple.com/v1/next?cursor=abc"

type includedFieldsCase struct {
	name      string
	operation string
	command   []string
	selector  []string
	include   string
	path      string
	query     map[string]string
	flags     map[string]string
	// resource marks a single-resource read.
	resource bool
	// noNext marks a command without a --next flag.
	noNext bool
	// nextSelector lists selector flags that --next does not replace.
	nextSelector []string
	// alwaysIncluded lists flags whose relationship the command always requests.
	alwaysIncluded []string
}

func openAPIFieldValues(t *testing.T, operation string) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "openapi", "latest.json"))
	if err != nil {
		t.Fatalf("read OpenAPI snapshot: %v", err)
	}
	var spec struct {
		Paths map[string]struct {
			Get struct {
				OperationID string `json:"operationId"`
				Parameters  []struct {
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
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("decode OpenAPI snapshot: %v", err)
	}
	for _, item := range spec.Paths {
		if item.Get.OperationID != operation {
			continue
		}
		values := map[string][]string{}
		for _, parameter := range item.Get.Parameters {
			if resource, ok := strings.CutPrefix(parameter.Name, "fields["); ok {
				values[strings.TrimSuffix(resource, "]")] = parameter.Schema.Items.Enum
			}
		}
		return values
	}
	t.Fatalf("operation %s is not in the OpenAPI snapshot", operation)
	return nil
}

var buildIncludedFieldFlags = map[string]string{
	"pre-release-version-fields":        "preReleaseVersions",
	"tester-fields":                     "betaTesters",
	"beta-group-fields":                 "betaGroups",
	"beta-build-localization-fields":    "betaBuildLocalizations",
	"encryption-declaration-fields":     "appEncryptionDeclarations",
	"beta-app-review-submission-fields": "betaAppReviewSubmissions",
	"app-fields":                        "apps",
	"build-beta-detail-fields":          "buildBetaDetails",
	"app-store-version-fields":          "appStoreVersions",
	"build-icon-fields":                 "buildIcons",
	"build-bundle-fields":               "buildBundles",
	"build-upload-fields":               "buildUploads",
}

var versionIncludedFieldFlags = map[string]string{
	"app-fields":                              "apps",
	"version-localization-fields":             "appStoreVersionLocalizations",
	"build-fields":                            "builds",
	"phased-release-fields":                   "appStoreVersionPhasedReleases",
	"game-center-app-version-fields":          "gameCenterAppVersions",
	"routing-app-coverage-fields":             "routingAppCoverages",
	"review-detail-fields":                    "appStoreReviewDetails",
	"version-submission-fields":               "appStoreVersionSubmissions",
	"app-clip-default-experience-fields":      "appClipDefaultExperiences",
	"experiment-fields":                       "appStoreVersionExperiments",
	"alternative-distribution-package-fields": "alternativeDistributionPackages",
}

var nominationIncludedFieldFlags = map[string]string{
	"app-fields":       "apps",
	"actor-fields":     "actors",
	"app-event-fields": "appEvents",
	"territory-fields": "territories",
}

var encryptionIncludedFieldFlags = map[string]string{"app-fields": "apps", "build-fields": "builds"}

var reviewSubmissionIncludedFieldFlags = map[string]string{
	"app-fields":               "apps",
	"app-store-version-fields": "appStoreVersions",
	"actor-fields":             "actors",
}

var iapListIncludedFieldFlags = map[string]string{
	"iap-localization-fields":      "inAppPurchaseLocalizations",
	"iap-content-fields":           "inAppPurchaseContents",
	"iap-review-screenshot-fields": "inAppPurchaseAppStoreReviewScreenshots",
	"promoted-purchase-fields":     "promotedPurchases",
	"iap-price-schedule-fields":    "inAppPurchasePriceSchedules",
	"iap-availability-fields":      "inAppPurchaseAvailabilities",
	"iap-image-fields":             "inAppPurchaseImages",
	"iap-offer-code-fields":        "inAppPurchaseOfferCodes",
}

const (
	buildIncludes      = "preReleaseVersion,individualTesters,betaGroups,betaBuildLocalizations,appEncryptionDeclaration,betaAppReviewSubmission,app,buildBetaDetail,appStoreVersion,icons,buildBundles,buildUpload"
	versionIncludes    = "app,appStoreVersionLocalizations,build,appStoreVersionPhasedRelease,gameCenterAppVersion,routingAppCoverage,appStoreReviewDetail,appStoreVersionSubmission,appClipDefaultExperience,appStoreVersionExperiments,alternativeDistributionPackage"
	nominationIncludes = "relatedApps,createdByActor,inAppEvents,supportedTerritories"
	iapListIncludes    = "inAppPurchaseLocalizations,content,appStoreReviewScreenshot,promotedPurchase,iapPriceSchedule,inAppPurchaseAvailability,images,offerCodes"
)

func withFlag(flags map[string]string, name, resource string) map[string]string {
	merged := map[string]string{name: resource}
	for key, value := range flags {
		merged[key] = value
	}
	return merged
}

func includedFieldsCases() []includedFieldsCase {
	return []includedFieldsCase{
		{
			name: "builds list", operation: "builds_getCollection",
			command: []string{"builds", "list"}, selector: []string{"--app", "123456789"},
			include: buildIncludes, path: "/v1/builds", query: map[string]string{"filter[app]": "123456789"},
			flags: buildIncludedFieldFlags, alwaysIncluded: []string{"pre-release-version-fields"},
		},
		{
			name: "builds info", operation: "builds_getInstance", resource: true, noNext: true,
			command: []string{"builds", "info"}, selector: []string{"--build-id", "build-1"},
			include: buildIncludes, path: "/v1/builds/build-1",
			flags: buildIncludedFieldFlags,
		},
		{
			name: "versions list", operation: "apps_appStoreVersions_getToManyRelated",
			command: []string{"versions", "list"}, selector: []string{"--app", "app-1"},
			include: versionIncludes, path: "/v1/apps/app-1/appStoreVersions",
			flags: versionIncludedFieldFlags,
		},
		{
			name: "versions view", operation: "appStoreVersions_getInstance", resource: true, noNext: true,
			command: []string{"versions", "view"}, selector: []string{"--version-id", "version-1"},
			include: versionIncludes, path: "/v1/appStoreVersions/version-1",
			flags: versionIncludedFieldFlags,
		},
		{
			name: "certificates view", operation: "certificates_getInstance", resource: true, noNext: true,
			command: []string{"certificates", "view"}, selector: []string{"--id", "certificate-1"},
			include: "passTypeId", path: "/v1/certificates/certificate-1",
			flags: map[string]string{"pass-type-id-fields": "passTypeIds"},
		},
		{
			name: "profiles view", operation: "profiles_getInstance", resource: true, noNext: true,
			command: []string{"profiles", "view"}, selector: []string{"--id", "profile-1"},
			include: "bundleId,certificates,devices", path: "/v1/profiles/profile-1",
			flags: map[string]string{"bundle-id-fields": "bundleIds", "certificate-fields": "certificates", "device-fields": "devices"},
		},
		{
			name: "users view", operation: "users_getInstance", resource: true, noNext: true,
			command: []string{"users", "view"}, selector: []string{"--id", "user-1"},
			include: "visibleApps", path: "/v1/users/user-1",
			flags: map[string]string{"app-fields": "apps"},
		},
		{
			name: "app clip default experiences view", operation: "appClipDefaultExperiences_getInstance", resource: true, noNext: true,
			command: []string{"app-clips", "default-experiences", "view"}, selector: []string{"--experience-id", "experience-1"},
			include: "appClip,releaseWithAppStoreVersion,appClipDefaultExperienceLocalizations,appClipAppStoreReviewDetail",
			path:    "/v1/appClipDefaultExperiences/experience-1",
			flags: map[string]string{
				"app-clip-fields":                        "appClips",
				"app-store-version-fields":               "appStoreVersions",
				"default-experience-localization-fields": "appClipDefaultExperienceLocalizations",
				"app-clip-review-detail-fields":          "appClipAppStoreReviewDetails",
			},
		},
		{
			name: "iap price points list", operation: "inAppPurchasesV2_pricePoints_getToManyRelated",
			command: []string{"iap", "pricing", "price-points", "list"}, selector: []string{"--iap-id", "123456789"},
			include: "territory", path: "/v2/inAppPurchases/123456789/pricePoints",
			flags: map[string]string{"territory-fields": "territories"},
		},
		{
			name: "iap price point equalizations", operation: "inAppPurchasePricePoints_equalizations_getToManyRelated",
			command: []string{"iap", "pricing", "price-points", "equalizations"}, selector: []string{"--id", "point-1"},
			include: "territory", path: "/v1/inAppPurchasePricePoints/point-1/equalizations",
			flags: map[string]string{"territory-fields": "territories"},
		},
		{
			name: "app price points", operation: "apps_appPricePoints_getToManyRelated",
			command: []string{"pricing", "price-points"}, selector: []string{"--app", "app-1"},
			include: "territory", path: "/v1/apps/app-1/appPricePoints",
			flags: map[string]string{"territory-fields": "territories"},
		},
		{
			name: "app price point equalizations", operation: "appPricePointsV3_equalizations_getToManyRelated",
			command: []string{"pricing", "price-points", "equalizations"}, selector: []string{"--price-point", "point-1"},
			include: "territory", path: "/v3/appPricePoints/point-1/equalizations",
			flags: map[string]string{"territory-fields": "territories"},
		},
		{
			name: "nominations list", operation: "nominations_getCollection",
			command: []string{"nominations", "list"}, selector: []string{"--status", "DRAFT"},
			include: nominationIncludes, path: "/v1/nominations", query: map[string]string{"filter[state]": "DRAFT"},
			flags: nominationIncludedFieldFlags,
		},
		{
			name: "nominations view", operation: "nominations_getInstance", resource: true, noNext: true,
			command: []string{"nominations", "view"}, selector: []string{"--id", "nomination-1"},
			include: nominationIncludes, path: "/v1/nominations/nomination-1",
			flags: nominationIncludedFieldFlags,
		},
		{
			name: "testflight testers list", operation: "betaTesters_getCollection",
			command: []string{"testflight", "testers", "list"}, selector: []string{"--app", "app-1"},
			include: "apps,betaGroups,builds", path: "/v1/betaTesters", query: map[string]string{"filter[apps]": "app-1"},
			flags: map[string]string{"app-fields": "apps", "beta-group-fields": "betaGroups", "build-fields": "builds"},
		},
		{
			name: "encryption declarations list", operation: "appEncryptionDeclarations_getCollection",
			command: []string{"encryption", "declarations", "list"}, selector: []string{"--app", "app-1"},
			include: "app,builds", path: "/v1/appEncryptionDeclarations", query: map[string]string{"filter[app]": "app-1"},
			flags: encryptionIncludedFieldFlags,
		},
		{
			name: "encryption declarations view", operation: "appEncryptionDeclarations_getInstance", resource: true, noNext: true,
			command: []string{"encryption", "declarations", "view"}, selector: []string{"--id", "declaration-1"},
			include: "app,builds", path: "/v1/appEncryptionDeclarations/declaration-1",
			flags: encryptionIncludedFieldFlags,
		},
		{
			name: "apps app encryption declarations list", operation: "appEncryptionDeclarations_getCollection",
			command: []string{"apps", "app-encryption-declarations", "list"}, selector: []string{"--id", "app-1"},
			include: "app,builds", path: "/v1/appEncryptionDeclarations", query: map[string]string{"filter[app]": "app-1"},
			flags: encryptionIncludedFieldFlags,
		},
		{
			name: "apps info view", operation: "appInfos_getInstance", resource: true,
			command: []string{"apps", "info", "view"}, selector: []string{"--info-id", "info-1"}, nextSelector: []string{"--info-id", "info-1"},
			include: "app,appInfoLocalizations,primaryCategory", path: "/v1/appInfos/info-1",
			flags: map[string]string{"app-fields": "apps", "app-info-localization-fields": "appInfoLocalizations", "category-fields": "appCategories"},
		},
		{
			name: "review submissions list", operation: "apps_reviewSubmissions_getToManyRelated",
			command: []string{"review", "submissions-list"}, selector: []string{"--app", "app-1"},
			include: "app,appStoreVersionForReview,submittedByActor", path: "/v1/apps/app-1/reviewSubmissions",
			flags: reviewSubmissionIncludedFieldFlags,
		},
		{
			name: "review submissions get", operation: "reviewSubmissions_getInstance", resource: true, noNext: true,
			command: []string{"review", "submissions-get"}, selector: []string{"--id", "submission-1"},
			include: "app,appStoreVersionForReview,lastUpdatedByActor", path: "/v1/reviewSubmissions/submission-1",
			flags: reviewSubmissionIncludedFieldFlags,
		},
		{
			name: "review items list", operation: "reviewSubmissions_items_getToManyRelated",
			command: []string{"review", "items", "list"}, selector: []string{"--submission", "submission-1"},
			include: "appStoreVersion,appCustomProductPageVersion,appStoreVersionExperiment,appEvent,appAssetLibraryImage,appAssetLibraryVideo,backgroundAssetVersion,gameCenterAchievementVersion,gameCenterActivityVersion,gameCenterChallengeVersion,gameCenterLeaderboardSetVersion,gameCenterLeaderboardVersion",
			path:    "/v1/reviewSubmissions/submission-1/items",
			flags: map[string]string{
				"app-store-version-fields":                   "appStoreVersions",
				"custom-product-page-version-fields":         "appCustomProductPageVersions",
				"experiment-fields":                          "appStoreVersionExperiments",
				"app-event-fields":                           "appEvents",
				"asset-image-fields":                         "appAssetLibraryImages",
				"asset-video-fields":                         "appAssetLibraryVideos",
				"background-asset-version-fields":            "backgroundAssetVersions",
				"game-center-achievement-version-fields":     "gameCenterAchievementVersions",
				"game-center-activity-version-fields":        "gameCenterActivityVersions",
				"game-center-challenge-version-fields":       "gameCenterChallengeVersions",
				"game-center-leaderboard-set-version-fields": "gameCenterLeaderboardSetVersions",
				"game-center-leaderboard-version-fields":     "gameCenterLeaderboardVersions",
			},
		},
		{
			name: "testflight crashes list", operation: "apps_betaFeedbackCrashSubmissions_getToManyRelated",
			command: []string{"testflight", "crashes", "list"}, selector: []string{"--app", "123456789"},
			include: "build,tester", path: "/v1/apps/123456789/betaFeedbackCrashSubmissions", query: map[string]string{"fields[builds]": "version,preReleaseVersion"},
			flags: map[string]string{"build-fields": "builds", "tester-fields": "betaTesters"},
		},
		{
			name: "testflight feedback list", operation: "apps_betaFeedbackScreenshotSubmissions_getToManyRelated",
			command: []string{"testflight", "feedback", "list"}, selector: []string{"--app", "123456789"},
			include: "build,tester", path: "/v1/apps/123456789/betaFeedbackScreenshotSubmissions", query: map[string]string{"fields[builds]": "version,preReleaseVersion"},
			flags: map[string]string{"build-fields": "builds", "tester-fields": "betaTesters"},
		},
		{
			name: "localization placements list", operation: "appStoreVersionLocalizations_placements_getToManyRelated",
			command: []string{"localizations", "placements", "list"}, selector: []string{"--localization-id", "localization-1"},
			include: "image,video", path: "/v1/appStoreVersionLocalizations/localization-1/placements",
			flags: map[string]string{"asset-image-fields": "appAssetLibraryImages", "asset-video-fields": "appAssetLibraryVideos"},
		},
		{
			name: "iap list", operation: "apps_inAppPurchasesV2_getToManyRelated",
			command: []string{"iap", "list"}, selector: []string{"--app", "app-1"},
			include: iapListIncludes, path: "/v1/apps/app-1/inAppPurchasesV2",
			flags: iapListIncludedFieldFlags,
		},
		{
			name: "iap view", operation: "inAppPurchasesV2_getInstance", resource: true, noNext: true,
			command: []string{"iap", "view"}, selector: []string{"--id", "iap-1"},
			include: iapListIncludes + ",pricePoints", path: "/v2/inAppPurchases/iap-1",
			flags: withFlag(iapListIncludedFieldFlags, "iap-price-point-fields", "inAppPurchasePricePoints"),
		},
		{
			name: "localizations list", operation: "appStoreVersions_appStoreVersionLocalizations_getToManyRelated",
			command: []string{"localizations", "list"}, selector: []string{"--version", "version-1"},
			include: "appStoreVersion,appScreenshotSets,appPreviewSets", path: "/v1/appStoreVersions/version-1/appStoreVersionLocalizations",
			flags: map[string]string{"app-store-version-fields": "appStoreVersions", "screenshot-set-fields": "appScreenshotSets", "preview-set-fields": "appPreviewSets"},
		},
		{
			name: "localization screenshot sets list", operation: "appStoreVersionLocalizations_appScreenshotSets_getToManyRelated",
			command: []string{"localizations", "screenshot-sets", "list"}, selector: []string{"--localization-id", "localization-1"},
			include: "appScreenshots", path: "/v1/appStoreVersionLocalizations/localization-1/appScreenshotSets", query: map[string]string{"limit[appScreenshots]": "50"},
			flags: map[string]string{"screenshot-fields": "appScreenshots"},
		},
	}
}

func TestIncludedResourceFieldFlags(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var requests []*url.URL
	var resource bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests = append(requests, req.URL)
		w.Header().Set("Content-Type", "application/json")
		if resource {
			_, _ = io.WriteString(w, `{"data":{"type":"resources","id":"resource-1"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	t.Cleanup(server.Close)
	installDefaultTransportForServer(t, server)

	run := func(t *testing.T, args []string) (*url.URL, error) {
		t.Helper()
		requests = nil
		_, stderr, err := runRootCommand(t, append(args, "--output", "json"))
		if err != nil {
			return nil, err
		}
		if len(requests) != 1 {
			t.Fatalf("requests = %v, want one (stderr: %s)", requests, stderr)
		}
		return requests[0], nil
	}
	assertUsageError := func(t *testing.T, args []string, want string) {
		t.Helper()
		requests = nil
		_, _, err := runRootCommand(t, append(args, "--output", "json"))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want it to mention %q", err, want)
		}
		if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d (error: %v)", got, rootcmd.ExitUsage, err)
		}
		if len(requests) != 0 {
			t.Fatalf("usage error must not reach the API, got %v", requests)
		}
	}

	for _, tc := range includedFieldsCases() {
		t.Run(tc.name, func(t *testing.T) {
			resource = tc.resource
			allowed := openAPIFieldValues(t, tc.operation)
			selected := slices.Concat(tc.command, tc.selector)

			omitted, err := run(t, slices.Concat(selected, []string{"--include", tc.include}))
			if err != nil {
				t.Fatalf("run without field flags: %v", err)
			}
			wantBase := url.Values{}
			for key, value := range tc.query {
				wantBase.Set(key, value)
			}
			gotBase := omitted.Query()
			gotInclude := gotBase.Get("include")
			gotBase.Del("include")
			if omitted.Path != tc.path || gotBase.Encode() != wantBase.Encode() {
				t.Fatalf("omitted flags sent %s?%s, want %s?%s", omitted.Path, gotBase.Encode(), tc.path, wantBase.Encode())
			}
			if strings.Join(sortedCSV(gotInclude), ",") != strings.Join(sortedCSV(tc.include), ",") {
				t.Fatalf("include = %q, want %q", gotInclude, tc.include)
			}

			args := slices.Concat(selected, []string{"--include", tc.include})
			want := omitted.Query()
			for flagName, resourceType := range tc.flags {
				values := allowed[resourceType]
				if len(values) == 0 {
					t.Fatalf("%s: %s accepts no fields[%s]", flagName, tc.operation, resourceType)
				}
				args = append(args, "--"+flagName, strings.Join(values, ","))
				want.Set("fields["+resourceType+"]", strings.Join(values, ","))
			}
			got, err := run(t, args)
			if err != nil {
				t.Fatalf("run with field flags: %v", err)
			}
			if got.Path != tc.path || got.Query().Encode() != want.Encode() {
				t.Fatalf("query = %s\nwant    %s", got.Query().Encode(), want.Encode())
			}

			for flagName, resourceType := range tc.flags {
				value := allowed[resourceType][0]
				if !tc.noNext {
					assertUsageError(t, slices.Concat(tc.command, tc.nextSelector, []string{"--next", includedFieldsNextURL, "--" + flagName, value}), "--next cannot be combined with --"+flagName)
				}
				if !slices.Contains(tc.alwaysIncluded, flagName) {
					assertUsageError(t, slices.Concat(selected, []string{"--" + flagName, value}), "--"+flagName+" requires --include")
				}
				assertUsageError(t, slices.Concat(selected, []string{"--include", tc.include, "--" + flagName, "notAField"}), "--"+flagName+" must be one of")
				assertUsageError(t, slices.Concat(selected, []string{"--include", tc.include, "--" + flagName, " , "}), "--"+flagName+" must not be empty")
			}
		})
	}
}

func sortedCSV(value string) []string {
	values := strings.Split(value, ",")
	slices.Sort(values)
	return values
}
