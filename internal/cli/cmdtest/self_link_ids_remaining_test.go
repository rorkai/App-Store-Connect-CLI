package cmdtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// The remaining single-value resource ID flags accept an API self-link too, so
// a links.self value from JSON output or a webhook delivery pipes straight
// back into asc. These tests pin both halves of that contract for the resource
// families wired here: a valid link reaches Apple as the bare ID, and a link
// naming another resource type fails flag parsing before any request.

// selfLinkRemainingServer serves the canned bodies keyed by request path and
// records every path it is asked for.
type selfLinkRemainingServer struct {
	mu     sync.Mutex
	paths  []string
	bodies map[string]string
}

func (s *selfLinkRemainingServer) record(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paths = append(s.paths, path)
}

func (s *selfLinkRemainingServer) requested() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.paths...)
}

func TestSelfLinkRemainingFlagsSendExtractedID(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
		wantID   string
		bodies   map[string]string
	}{
		{
			name:     "webhooks view",
			args:     []string{"webhooks", "view", "--webhook-id", selfLinkTestBase + "/v1/webhooks/webhook-1"},
			wantPath: "GET /v1/webhooks/webhook-1",
			wantID:   "webhook-1",
			bodies: map[string]string{
				"/v1/webhooks/webhook-1": `{"data":{"type":"webhooks","id":"webhook-1","attributes":{"name":"Build updates","url":"https://example.com/hook","enabled":true}}}`,
			},
		},
		{
			name:     "app-events view",
			args:     []string{"app-events", "view", "--event-id", selfLinkTestBase + "/v1/appEvents/event-1"},
			wantPath: "GET /v1/appEvents/event-1",
			wantID:   "event-1",
			bodies: map[string]string{
				"/v1/appEvents/event-1": `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","eventState":"DRAFT"}}}`,
			},
		},
		{
			name:     "categories view",
			args:     []string{"categories", "view", "--category-id", selfLinkTestBase + "/v1/appCategories/GAMES"},
			wantPath: "GET /v1/appCategories/GAMES",
			wantID:   "GAMES",
			bodies: map[string]string{
				"/v1/appCategories/GAMES": `{"data":{"type":"appCategories","id":"GAMES","attributes":{"platforms":["IOS"]}}}`,
			},
		},
		{
			name:     "product-pages custom-pages view",
			args:     []string{"product-pages", "custom-pages", "view", "--custom-page-id", selfLinkTestBase + "/v1/appCustomProductPages/page-1"},
			wantPath: "GET /v1/appCustomProductPages/page-1",
			wantID:   "page-1",
			bodies: map[string]string{
				"/v1/appCustomProductPages/page-1": `{"data":{"type":"appCustomProductPages","id":"page-1","attributes":{"name":"Winter","visible":true}}}`,
			},
		},
		{
			name:     "android-ios-mapping view",
			args:     []string{"android-ios-mapping", "view", "--mapping-id", selfLinkTestBase + "/v1/androidToIosAppMappingDetails/mapping-1"},
			wantPath: "GET /v1/androidToIosAppMappingDetails/mapping-1",
			wantID:   "mapping-1",
			bodies: map[string]string{
				"/v1/androidToIosAppMappingDetails/mapping-1": `{"data":{"type":"androidToIosAppMappingDetails","id":"mapping-1","attributes":{"androidPackageName":"com.example.android"}}}`,
			},
		},
		{
			name:     "alternative-distribution domains view",
			args:     []string{"alternative-distribution", "domains", "view", "--domain-id", selfLinkTestBase + "/v1/alternativeDistributionDomains/domain-1"},
			wantPath: "GET /v1/alternativeDistributionDomains/domain-1",
			wantID:   "domain-1",
			bodies: map[string]string{
				"/v1/alternativeDistributionDomains/domain-1": `{"data":{"type":"alternativeDistributionDomains","id":"domain-1","attributes":{"domain":"example.com"}}}`,
			},
		},
		{
			name:     "merchant-ids view",
			args:     []string{"merchant-ids", "view", "--merchant-id", selfLinkTestBase + "/v1/merchantIds/merchant-1"},
			wantPath: "GET /v1/merchantIds/merchant-1",
			wantID:   "merchant-1",
			bodies: map[string]string{
				"/v1/merchantIds/merchant-1": `{"data":{"type":"merchantIds","id":"merchant-1","attributes":{"name":"Store","identifier":"merchant.com.example"}}}`,
			},
		},
		{
			name:     "analytics reports view",
			args:     []string{"analytics", "reports", "view", "--report-id", selfLinkTestBase + "/v1/analyticsReports/report-1"},
			wantPath: "GET /v1/analyticsReports/report-1",
			wantID:   "report-1",
			bodies: map[string]string{
				"/v1/analyticsReports/report-1": `{"data":{"type":"analyticsReports","id":"report-1","attributes":{"name":"App Store Installs","category":"APP_USAGE"}}}`,
			},
		},
		{
			name:     "game-center achievements localizations list",
			args:     []string{"game-center", "achievements", "localizations", "list", "--achievement-id", selfLinkTestBase + "/v1/gameCenterAchievements/achievement-1"},
			wantPath: "GET /v1/gameCenterAchievements/achievement-1/localizations",
			wantID:   "localization-1",
			bodies: map[string]string{
				"/v1/gameCenterAchievements/achievement-1/localizations": `{"data":[{"type":"gameCenterAchievementLocalizations","id":"localization-1","attributes":{"locale":"en-US","name":"First Win"}}]}`,
			},
		},
		{
			name:     "builds test-notes view",
			args:     []string{"builds", "test-notes", "view", "--localization-id", selfLinkTestBase + "/v1/betaBuildLocalizations/localization-1"},
			wantPath: "GET /v1/betaBuildLocalizations/localization-1",
			wantID:   "localization-1",
			bodies: map[string]string{
				"/v1/betaBuildLocalizations/localization-1": `{"data":{"type":"betaBuildLocalizations","id":"localization-1","attributes":{"locale":"en-US","whatsNew":"Fixes"}}}`,
			},
		},
		{
			name:     "testflight testers apps list",
			args:     []string{"testflight", "testers", "apps", "list", "--tester-id", selfLinkTestBase + "/v1/betaTesters/tester-1"},
			wantPath: "GET /v1/betaTesters/tester-1/apps",
			wantID:   "app-1",
			bodies: map[string]string{
				"/v1/betaTesters/tester-1/apps": `{"data":[{"type":"apps","id":"app-1","attributes":{"name":"Demo","bundleId":"com.example.demo"}}]}`,
			},
		},
		{
			name:     "app-clips default-experiences localizations list",
			args:     []string{"app-clips", "default-experiences", "localizations", "list", "--experience-id", selfLinkTestBase + "/v1/appClipDefaultExperiences/experience-1"},
			wantPath: "GET /v1/appClipDefaultExperiences/experience-1/appClipDefaultExperienceLocalizations",
			wantID:   "clip-localization-1",
			bodies: map[string]string{
				"/v1/appClipDefaultExperiences/experience-1/appClipDefaultExperienceLocalizations": `{"data":[{"type":"appClipDefaultExperienceLocalizations","id":"clip-localization-1","attributes":{"locale":"en-US","subtitle":"Order"}}]}`,
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

func TestSelfLinkRemainingFlagsRejectWrongType(t *testing.T) {
	resetCmdtestState()
	setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	tests := []struct {
		name    string
		args    []string
		flag    string
		value   string
		wantErr string
	}{
		{
			name:    "webhooks view rejects apps link",
			args:    []string{"webhooks", "view"},
			flag:    "webhook-id",
			value:   selfLinkTestBase + "/v1/apps/123",
			wantErr: "expected a self-link of type webhooks, got apps",
		},
		{
			name:    "webhooks view rejects relationship path",
			args:    []string{"webhooks", "view"},
			flag:    "webhook-id",
			value:   selfLinkTestBase + "/v1/webhooks/webhook-1/relationships/app",
			wantErr: "/v1/<type>/<id>",
		},
		{
			name:    "marketplace webhooks view rejects webhooks link",
			args:    []string{"marketplace", "webhooks", "view"},
			flag:    "webhook-id",
			value:   selfLinkTestBase + "/v1/webhooks/webhook-1",
			wantErr: "expected a self-link of type marketplaceWebhooks, got webhooks",
		},
		{
			name:    "merchant-ids view rejects apps link",
			args:    []string{"merchant-ids", "view"},
			flag:    "merchant-id",
			value:   selfLinkTestBase + "/v1/apps/123",
			wantErr: "expected a self-link of type merchantIds, got apps",
		},
		{
			name:    "pass-type-ids certificates list rejects certificates link",
			args:    []string{"pass-type-ids", "certificates", "list"},
			flag:    "pass-type-id",
			value:   selfLinkTestBase + "/v1/certificates/certificate-1",
			wantErr: "expected a self-link of type passTypeIds, got certificates",
		},
		{
			name:    "reviews respond rejects apps link",
			args:    []string{"reviews", "respond"},
			flag:    "review-id",
			value:   selfLinkTestBase + "/v1/apps/123",
			wantErr: "expected a self-link of type customerReviews, got apps",
		},
		{
			name:    "categories view rejects apps link",
			args:    []string{"categories", "view"},
			flag:    "category-id",
			value:   selfLinkTestBase + "/v1/apps/123",
			wantErr: "expected a self-link of type appCategories, got apps",
		},
		{
			name:    "analytics reports view rejects request link",
			args:    []string{"analytics", "reports", "view"},
			flag:    "report-id",
			value:   selfLinkTestBase + "/v1/analyticsReportRequests/request-1",
			wantErr: "expected a self-link of type analyticsReports, got analyticsReportRequests",
		},
		{
			name:    "xcode-cloud status rejects workflow link",
			args:    []string{"xcode-cloud", "status"},
			flag:    "run-id",
			value:   selfLinkTestBase + "/v1/ciWorkflows/workflow-1",
			wantErr: "expected a self-link of type ciBuildRuns, got ciWorkflows",
		},
		{
			name:    "alternative-distribution domains view rejects other host",
			args:    []string{"alternative-distribution", "domains", "view"},
			flag:    "domain-id",
			value:   "https://appstoreconnect.apple.com/v1/alternativeDistributionDomains/domain-1",
			wantErr: "api.appstoreconnect.apple.com",
		},
		{
			name:    "background-assets versions list rejects version link",
			args:    []string{"background-assets", "versions", "list"},
			flag:    "background-asset-id",
			value:   selfLinkTestBase + "/v1/backgroundAssetVersions/version-1",
			wantErr: "expected a self-link of type backgroundAssets, got backgroundAssetVersions",
		},
		{
			name:    "game-center achievements localizations list rejects leaderboard link",
			args:    []string{"game-center", "achievements", "localizations", "list"},
			flag:    "achievement-id",
			value:   selfLinkTestBase + "/v1/gameCenterLeaderboards/leaderboard-1",
			wantErr: "expected a self-link of type gameCenterAchievements, got gameCenterLeaderboards",
		},
		{
			name:    "game-center achievements v2 list rejects details link",
			args:    []string{"game-center", "achievements", "v2", "list"},
			flag:    "group-id",
			value:   selfLinkTestBase + "/v1/gameCenterDetails/detail-1",
			wantErr: "expected a self-link of type gameCenterGroups, got gameCenterDetails",
		},
		{
			name:    "game-center matchmaking queues update rejects queue link",
			args:    []string{"game-center", "matchmaking", "queues", "update"},
			flag:    "rule-set-id",
			value:   selfLinkTestBase + "/v1/gameCenterMatchmakingQueues/queue-1",
			wantErr: "expected a self-link of type gameCenterMatchmakingRuleSets, got gameCenterMatchmakingQueues",
		},
		{
			name:    "app-clips default-experiences localizations list rejects app-clips link",
			args:    []string{"app-clips", "default-experiences", "localizations", "list"},
			flag:    "experience-id",
			value:   selfLinkTestBase + "/v1/appClips/clip-1",
			wantErr: "expected a self-link of type appClipDefaultExperiences, got appClips",
		},
		{
			name:    "product-pages custom-pages view rejects version link",
			args:    []string{"product-pages", "custom-pages", "view"},
			flag:    "custom-page-id",
			value:   selfLinkTestBase + "/v1/appCustomProductPageVersions/version-1",
			wantErr: "expected a self-link of type appCustomProductPages, got appCustomProductPageVersions",
		},
		{
			name:    "product-pages experiments view rejects version link",
			args:    []string{"product-pages", "experiments", "view"},
			flag:    "experiment-id",
			value:   selfLinkTestBase + "/v1/appStoreVersions/version-1",
			wantErr: "expected a self-link of type appStoreVersionExperiments, got appStoreVersions",
		},
		{
			name:    "app-events view rejects localization link",
			args:    []string{"app-events", "view"},
			flag:    "event-id",
			value:   selfLinkTestBase + "/v1/appEventLocalizations/localization-1",
			wantErr: "expected a self-link of type appEvents, got appEventLocalizations",
		},
		{
			name:    "subscriptions offers offer-codes view rejects iap offer code link",
			args:    []string{"subscriptions", "offers", "offer-codes", "view"},
			flag:    "offer-code-id",
			value:   selfLinkTestBase + "/v1/inAppPurchaseOfferCodes/offer-1",
			wantErr: "expected a self-link of type subscriptionOfferCodes, got inAppPurchaseOfferCodes",
		},
		{
			name:    "iap offer-codes custom-codes list rejects subscription offer code link",
			args:    []string{"iap", "offer-codes", "custom-codes", "list"},
			flag:    "offer-code-id",
			value:   selfLinkTestBase + "/v1/subscriptionOfferCodes/offer-1",
			wantErr: "expected a self-link of type inAppPurchaseOfferCodes, got subscriptionOfferCodes",
		},
		{
			name:    "testflight testers apps list rejects apps link",
			args:    []string{"testflight", "testers", "apps", "list"},
			flag:    "tester-id",
			value:   selfLinkTestBase + "/v1/apps/123",
			wantErr: "expected a self-link of type betaTesters, got apps",
		},
		{
			name:    "testflight testers apps list rejects apps link on the --id alias",
			args:    []string{"testflight", "testers", "apps", "list"},
			flag:    "id",
			value:   selfLinkTestBase + "/v1/apps/123",
			wantErr: "expected a self-link of type betaTesters, got apps",
		},
		{
			name:    "builds test-notes view rejects builds link",
			args:    []string{"builds", "test-notes", "view"},
			flag:    "localization-id",
			value:   selfLinkTestBase + "/v1/builds/build-1",
			wantErr: "expected a self-link of type betaBuildLocalizations, got builds",
		},
		{
			name:    "android-ios-mapping view rejects apps link",
			args:    []string{"android-ios-mapping", "view"},
			flag:    "mapping-id",
			value:   selfLinkTestBase + "/v1/apps/123",
			wantErr: "expected a self-link of type androidToIosAppMappingDetails, got apps",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string(nil), test.args...), "--"+test.flag, test.value)
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
			wantPrefix := fmt.Sprintf("Error: invalid value %q for flag -%s: ", test.value, test.flag)
			if !strings.HasPrefix(firstLine, wantPrefix) || !strings.Contains(firstLine, test.wantErr) {
				t.Fatalf("first stderr line = %q, want prefix %q containing %q", firstLine, wantPrefix, test.wantErr)
			}
		})
	}
}
