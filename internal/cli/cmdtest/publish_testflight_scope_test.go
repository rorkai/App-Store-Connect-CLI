package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestPublishTestflightBindsCollectionPagesAndResourceTypes(t *testing.T) {
	for _, tc := range []struct {
		name               string
		internalOnly       bool
		wrongApp           bool
		wrongOwnerType     bool
		wrongBuild         bool
		byNumber           bool
		wrongBuildNumber   bool
		discoveryNext      string
		discoveryType      string
		discoveryID        string
		membershipNext     string
		membershipType     string
		membershipNoData   bool
		paginateDiscovery  bool
		paginateMembership bool
		absent             bool
		failure            bool
	}{
		{name: "internal-only exact build number", internalOnly: true, byNumber: true},
		{name: "internal-only wrong build number", internalOnly: true, byNumber: true, wrongBuildNumber: true, failure: true},
		{name: "internal-only build number wrong owner", internalOnly: true, byNumber: true, wrongApp: true, failure: true},
		{name: "internal-only membership retained", internalOnly: true},
		{name: "internal-only missing membership added", internalOnly: true, absent: true},
		{name: "internal-only wrong app before notes or wait", internalOnly: true, wrongApp: true, failure: true},
		{name: "internal-only wrong owner type", internalOnly: true, wrongOwnerType: true, failure: true},
		{name: "internal-only wrong build identity", internalOnly: true, wrongBuild: true, failure: true},
		{name: "scoped absolute pages", paginateDiscovery: true, paginateMembership: true},
		{name: "reject alternate API origin", failure: true, paginateMembership: true, membershipNext: asc.EnterpriseBaseURL + "/v1/betaGroups/g1/relationships/builds?cursor=next"},
		{name: "invalid group resource path", discoveryID: "../foreign", failure: true},
		{name: "missing membership data", membershipNoData: true, failure: true},
		{name: "scoped relative membership", paginateMembership: true, membershipNext: "/v1/betaGroups/g1/relationships/builds?cursor=next"},
		{name: "missing membership is added", absent: true},
		{name: "discovery rejects alternate API origin", paginateDiscovery: true, discoveryNext: asc.EnterpriseBaseURL + "/v1/apps/123/betaGroups?cursor=next", failure: true},
		{name: "discovery foreign collection", paginateDiscovery: true, discoveryNext: "https://api.appstoreconnect.apple.com/v1/apps/other/betaGroups?cursor=next", failure: true},
		{name: "discovery changed query", paginateDiscovery: true, discoveryNext: "https://api.appstoreconnect.apple.com/v1/apps/123/betaGroups?cursor=next&filter[id]=g1", failure: true},
		{name: "discovery wrong resource type", discoveryType: "apps", failure: true},
		{name: "discovery next wrong resource type", paginateDiscovery: true, discoveryType: "apps", failure: true},
		{name: "membership foreign collection", paginateMembership: true, membershipNext: "https://api.appstoreconnect.apple.com/v1/betaGroups/other/relationships/builds?cursor=next", failure: true},
		{name: "membership escaped path", paginateMembership: true, membershipNext: "https://api.appstoreconnect.apple.com/v1/betaGroups/%671/relationships/builds?cursor=next", failure: true},
		{name: "membership wrong relationship type", membershipType: "apps", failure: true},
		{name: "membership next wrong relationship type", paginateMembership: true, membershipType: "apps", failure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes, foreignReads := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				page := r.URL.Query().Get("cursor") == "next"
				var payload any
				switch r.Method + " " + r.URL.Path {
				case "GET /v1/apps/123/betaGroups":
					groupType := tc.discoveryType
					if groupType == "" {
						groupType = "betaGroups"
					}
					groupID := tc.discoveryID
					if groupID == "" {
						groupID = "g1"
					}
					payload = map[string]any{"data": []any{map[string]any{"type": groupType, "id": groupID, "attributes": map[string]any{"name": "External", "isInternalGroup": tc.internalOnly}}}}
					if tc.paginateDiscovery && !page {
						next := tc.discoveryNext
						if next == "" {
							next = "https://api.appstoreconnect.apple.com/v1/apps/123/betaGroups?cursor=next&limit=200"
						}
						payload = map[string]any{"data": []any{}, "links": map[string]string{"next": next}}
					}
				case "GET /v1/builds":
					if r.URL.Query().Get("filter[app]") != "123" || r.URL.Query().Get("filter[version]") != "42" || r.URL.Query().Get("filter[preReleaseVersion.platform]") != "IOS" {
						t.Errorf("build lookup lost its app, number or platform: %s", r.URL)
					}
					number := "42"
					if tc.wrongBuildNumber {
						number = "43"
					}
					payload = map[string]any{"data": []any{map[string]any{"type": "builds", "id": "b1", "attributes": map[string]string{"version": number, "processingState": "VALID"}}}}
				case "GET /v1/builds/b1/app":
					ownerType, ownerID := "apps", "123"
					if tc.wrongApp {
						ownerID = "foreign"
					}
					if tc.wrongOwnerType {
						ownerType = "builds"
					}
					payload = map[string]any{"data": map[string]string{"type": ownerType, "id": ownerID}}
				case "GET /v1/builds/b1/preReleaseVersion":
					if !tc.byNumber || tc.wrongApp || tc.wrongBuildNumber {
						t.Error("unexpected prerelease read for an unselected build")
					}
					payload = map[string]any{"data": map[string]any{"type": "preReleaseVersions", "id": "pr-1", "attributes": map[string]string{"platform": "IOS"}}}
				case "GET /v1/builds/b1":
					if tc.wrongApp || tc.wrongOwnerType {
						t.Error("foreign build content was read")
					}
					buildID := "b1"
					if tc.wrongBuild {
						buildID = "foreign"
					}
					payload = map[string]any{"data": map[string]any{"type": "builds", "id": buildID, "attributes": map[string]string{"version": "42", "processingState": "VALID"}}}
				case "GET /v1/betaGroups/g1/relationships/builds":
					relationshipType := tc.membershipType
					if relationshipType == "" {
						relationshipType = "builds"
					}
					payload = map[string]any{"data": []any{map[string]string{"type": relationshipType, "id": "b1"}}}
					if tc.membershipNoData {
						payload = map[string]any{}
					} else if tc.absent {
						payload = map[string]any{"data": []any{}}
					} else if tc.paginateMembership && !page {
						next := tc.membershipNext
						if next == "" {
							next = "https://api.appstoreconnect.apple.com/v1/betaGroups/g1/relationships/builds?cursor=next&limit=200"
						}
						payload = map[string]any{"data": []any{}, "links": map[string]string{"next": next}}
					}
				case "POST /v1/builds/b1/relationships/betaGroups":
					writes++
					w.WriteHeader(http.StatusNoContent)
					return
				default:
					foreignReads++
					t.Errorf("request escaped selected collections: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if err := json.NewEncoder(w).Encode(payload); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			args := []string{"publish", "testflight", "--app", "123", "--build-id", "b1", "--group", "External"}
			if tc.byNumber {
				args = []string{"publish", "testflight", "--app", "123", "--build-number", "42", "--group", "External"}
			}
			if tc.wrongApp || tc.wrongOwnerType {
				args = append(args, "--wait", "--test-notes", "Original instructions", "--locale", "en-US")
			}
			stdout, stderr, err := runRootWithDeps(t, publishBoundOriginFixtureClient(t, server), args...)
			wantWrites := 0
			if tc.absent {
				wantWrites = 1
			}
			if (err != nil) != tc.failure || writes != wantWrites || foreignReads != 0 {
				t.Fatalf("writes=%d foreign=%d stdout=%s stderr=%s err=%v", writes, foreignReads, stdout, stderr, err)
			}
			if tc.failure {
				var result asc.TestFlightPublishResult
				if decodeErr := json.Unmarshal([]byte(stdout), &result); decodeErr != nil {
					t.Fatal(decodeErr)
				}
				if result.Status != "failed" || result.FailureStage == "" || result.FailureCode == "" || len(result.Failure) > 2048 {
					t.Fatalf("invalid structured preflight failure: %s", stdout)
				}
			}
		})
	}
}

func TestPublishTestflightReportsLocaleCreationBeforeBoundedNotesFailure(t *testing.T) {
	localeCreated := false
	noteWrites, groupWrites := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var payload string
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/apps/123/betaGroups":
			payload = `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"External"}}]}`
		case "GET /v1/builds/b1/app":
			payload = `{"data":{"type":"apps","id":"123"}}`
		case "GET /v1/builds/b1":
			payload = `{"data":{"type":"builds","id":"b1","attributes":{"version":"42","processingState":"VALID"}}}`
		case "GET /v1/betaGroups/g1/relationships/builds":
			payload = `{"data":[]}`
		case "GET /v1/betaAppLocalizations":
			payload = `{"data":[]}`
		case "POST /v1/betaAppLocalizations":
			localeCreated = true
			payload = `{"data":{"type":"betaAppLocalizations","id":"locale1","attributes":{"locale":"en-US"}}}`
			w.WriteHeader(http.StatusCreated)
		case "GET /v1/builds/b1/betaBuildLocalizations":
			payload = `{"data":[]}`
		case "POST /v1/betaBuildLocalizations":
			noteWrites++
			w.WriteHeader(http.StatusUnprocessableEntity)
			detail, _ := json.Marshal("PRIVATE-CANARY user@example.invalid https://private.invalid/?token=private-secret /Users/private/provider rejected notes: " + strings.Repeat("界", 4096) + "\u001b[31m")
			payload = fmt.Sprintf(`{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Notes rejected","detail":%s}]}`, detail)
		case "POST /v1/builds/b1/relationships/betaGroups", "POST /v1/buildBetaNotifications":
			groupWrites++
			t.Error("distribution continued after notes failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, err := w.Write([]byte(payload)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	stdout, stderr, err := runRootWithDeps(t, publishBoundOriginFixtureClient(t, server),
		"publish", "testflight", "--app", "123", "--build-id", "b1", "--group", "g1", "--test-notes", "Original instructions", "--locale", "en-US")
	if err == nil || !localeCreated || noteWrites != 1 || groupWrites != 0 {
		t.Fatalf("locale=%t notes=%d groups=%d stdout=%s stderr=%s err=%v", localeCreated, noteWrites, groupWrites, stdout, stderr, err)
	}
	var result asc.TestFlightPublishResult
	if decodeErr := json.Unmarshal([]byte(stdout), &result); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if result.Status != "partial" || result.FailureStage != "test_notes" || result.FailureCode != "test_notes_failed" ||
		!slices.Contains(result.CompletedStages, "beta_app_localization") || slices.Contains(result.CompletedStages, "test_notes") ||
		result.Recovery == nil || result.Recovery.Locale != "en-US" || result.Recovery.SubmittedNotes != "Original instructions" ||
		result.Failure != "Test notes could not be saved." || strings.Contains(stdout, "PRIVATE-CANARY") || strings.Contains(stdout, "private-secret") || len(err.Error()) > 2048 || !strings.Contains(err.Error(), "PRIVATE-CANARY") {
		t.Fatalf("partial locale completion or bounded diagnostic missing: %s err=%v", stdout, err)
	}
}

func publishBoundOriginFixtureClient(t *testing.T, server *httptest.Server) func() (*asc.Client, error) {
	t.Helper()
	destination, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	origin, err := url.Parse(asc.BaseURL)
	if err != nil {
		t.Fatal(err)
	}
	return stubClient(t, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Scheme != origin.Scheme || request.URL.Host != origin.Host {
			t.Errorf("request changed the selected API origin: %s", request.URL)
			return nil, fmt.Errorf("unexpected API origin")
		}
		cloned := request.Clone(request.Context())
		cloned.URL.Scheme, cloned.URL.Host = destination.Scheme, destination.Host
		return server.Client().Transport.RoundTrip(cloned)
	}))
}

func TestPublishTestflightBindsReviewMetadataPages(t *testing.T) {
	for _, tc := range []struct {
		name          string
		next          string
		wrongResource string
		failure       bool
	}{
		{name: "scoped metadata pages"},
		{name: "reject alternate API origin", failure: true, next: asc.EnterpriseBaseURL + "/v1/betaAppLocalizations?filter[app]=123&cursor=next"},
		{name: "foreign app filter", next: asc.BaseURL + "/v1/betaAppLocalizations?filter[app]=other&cursor=next", failure: true},
		{name: "missing app filter", next: asc.BaseURL + "/v1/betaAppLocalizations?cursor=next", failure: true},
		{name: "different metadata collection", next: asc.BaseURL + "/v1/betaAppReviewDetails?filter[app]=123&cursor=next", failure: true},
		{name: "wrong localization type", wrongResource: "localization", failure: true},
		{name: "wrong review detail type", wrongResource: "review detail", failure: true},
		{name: "wrong build detail type", wrongResource: "build detail", failure: true},
		{name: "wrong submission type", wrongResource: "submission", failure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes, foreignReads := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				payload := ""
				switch r.Method + " " + r.URL.Path {
				case "GET /v1/apps/123/betaGroups":
					payload = `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"External"}}]}`
				case "GET /v1/builds/b1/app":
					payload = `{"data":{"type":"apps","id":"123"}}`
				case "GET /v1/builds/b1":
					payload = `{"data":{"type":"builds","id":"b1","attributes":{"version":"42","processingState":"VALID"}}}`
				case "GET /v1/builds/b1/betaAppReviewSubmission":
					if tc.wrongResource == "submission" {
						payload = `{"data":{"type":"apps","id":"submission1"}}`
					} else {
						w.WriteHeader(http.StatusNotFound)
						payload = `{"errors":[{"status":"404","code":"NOT_FOUND","title":"missing"}]}`
					}
				case "GET /v1/betaAppLocalizations":
					if r.URL.Query().Get("filter[app]") != "123" {
						foreignReads++
						t.Error("metadata read escaped selected app")
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if r.URL.Query().Get("cursor") == "" {
						next := tc.next
						if next == "" {
							next = asc.BaseURL + "/v1/betaAppLocalizations?filter[app]=123&cursor=next"
						}
						encoded, _ := json.Marshal(next)
						payload = fmt.Sprintf(`{"data":[],"links":{"next":%s}}`, encoded)
					} else {
						resourceType := "betaAppLocalizations"
						if tc.wrongResource == "localization" {
							resourceType = "apps"
						}
						payload = fmt.Sprintf(`{"data":[{"type":%q,"id":"locale1","attributes":{"locale":"en-US","description":"Beta","feedbackEmail":"beta@example.com"}}]}`, resourceType)
					}
				case "GET /v1/betaAppReviewDetails":
					if r.URL.Query().Get("cursor") != "" {
						foreignReads++
						t.Error("unexpected review detail continuation")
					}
					resourceType := "betaAppReviewDetails"
					if tc.wrongResource == "review detail" {
						resourceType = "apps"
					}
					payload = fmt.Sprintf(`{"data":[{"type":%q,"id":"detail1","attributes":{"contactFirstName":"Beta","contactLastName":"Tester","contactPhone":"+1 408 555 0100","contactEmail":"beta@example.com"}}]}`, resourceType)
				case "GET /v1/builds/b1/buildBetaDetail":
					resourceType := "buildBetaDetails"
					if tc.wrongResource == "build detail" {
						resourceType = "apps"
					}
					payload = fmt.Sprintf(`{"data":{"type":%q,"id":"detail1","attributes":{"externalBuildState":"READY_FOR_BETA_SUBMISSION"}}}`, resourceType)
				case "GET /v1/betaGroups/g1/relationships/builds":
					payload = `{"data":[]}`
				case "POST /v1/builds/b1/relationships/betaGroups":
					writes++
					w.WriteHeader(http.StatusNoContent)
					return
				case "POST /v1/betaAppReviewSubmissions":
					writes++
					payload = `{"data":{"type":"betaAppReviewSubmissions","id":"submission1"}}`
				default:
					foreignReads++
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if _, err := w.Write([]byte(payload)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			stdout, stderr, err := runRootWithDeps(t, publishBoundOriginFixtureClient(t, server),
				"publish", "testflight", "--app", "123", "--build-id", "b1", "--group", "External", "--submit", "--confirm")
			expectedWrites := 2
			if tc.failure {
				expectedWrites = 0
			}
			if (err != nil) != tc.failure || writes != expectedWrites || foreignReads != 0 {
				t.Fatalf("writes=%d foreign=%d stdout=%s stderr=%s err=%v", writes, foreignReads, stdout, stderr, err)
			}
			if tc.failure {
				var result asc.TestFlightPublishResult
				if decodeErr := json.Unmarshal([]byte(stdout), &result); decodeErr != nil {
					t.Fatal(decodeErr)
				}
				if result.Status != "failed" || result.FailureStage != "beta_review_submission" || result.FailureCode != "beta_review_submission_failed" {
					t.Fatalf("unexpected metadata failure receipt: %s", stdout)
				}
			}
		})
	}
}

func TestPublishTestflightScopedReadsDoNotFollowRedirects(t *testing.T) {
	for _, tc := range []struct {
		name, path                  string
		next, notes, submit, notify bool
		wantWrites                  int
	}{
		{name: "group discovery", path: "/v1/apps/123/betaGroups"},
		{name: "group discovery continuation", path: "/v1/apps/123/betaGroups", next: true},
		{name: "build ownership", path: "/v1/builds/b1/app"},
		{name: "build content", path: "/v1/builds/b1"},
		{name: "group membership", path: "/v1/betaGroups/g1/relationships/builds"},
		{name: "group membership continuation", path: "/v1/betaGroups/g1/relationships/builds", next: true},
		{name: "review submission", path: "/v1/builds/b1/betaAppReviewSubmission", submit: true},
		{name: "review localization", path: "/v1/betaAppLocalizations", submit: true},
		{name: "review localization continuation", path: "/v1/betaAppLocalizations", submit: true, next: true},
		{name: "review details", path: "/v1/betaAppReviewDetails", submit: true},
		{name: "review build details", path: "/v1/builds/b1/buildBetaDetail", submit: true},
		{name: "notes app localization", path: "/v1/betaAppLocalizations", notes: true},
		{name: "notes build localization", path: "/v1/builds/b1/betaBuildLocalizations", notes: true},
		{name: "notification settings", path: "/v1/builds/b1/buildBetaDetail", notify: true, wantWrites: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			foreignReads, writes := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/foreign-scope" {
					foreignReads++
					http.Error(w, "PRIVATE-REDIRECT-CANARY", http.StatusBadRequest)
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == tc.path && (r.URL.Query().Get("cursor") == "next") == tc.next {
					http.Redirect(w, r, asc.BaseURL+"/foreign-scope", http.StatusTemporaryRedirect)
					return
				}
				if tc.next && r.URL.Path == tc.path {
					values := r.URL.Query()
					values.Set("cursor", "next")
					if err := json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "links": map[string]string{"next": asc.BaseURL + tc.path + "?" + values.Encode()}}); err != nil {
						t.Error(err)
					}
					return
				}
				var payload string
				switch r.Method + " " + r.URL.Path {
				case "GET /v1/apps/123/betaGroups":
					payload = `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"External"}}]}`
				case "GET /v1/builds/b1/app":
					payload = `{"data":{"type":"apps","id":"123"}}`
				case "GET /v1/builds/b1":
					payload = `{"data":{"type":"builds","id":"b1","attributes":{"version":"42","processingState":"VALID"}}}`
				case "GET /v1/betaGroups/g1/relationships/builds":
					payload = `{"data":[]}`
				case "GET /v1/builds/b1/betaAppReviewSubmission":
					w.WriteHeader(http.StatusNotFound)
					payload = `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Missing"}]}`
				case "GET /v1/betaAppLocalizations":
					payload = `{"data":[{"type":"betaAppLocalizations","id":"l1","attributes":{"locale":"en-US","description":"Beta app","feedbackEmail":"beta@example.invalid"}}]}`
				case "GET /v1/betaAppReviewDetails":
					payload = `{"data":[{"type":"betaAppReviewDetails","id":"d1","attributes":{"contactFirstName":"Beta","contactLastName":"Tester","contactPhone":"+1 408 555 0100","contactEmail":"beta@example.invalid"}}]}`
				case "GET /v1/builds/b1/buildBetaDetail":
					payload = `{"data":{"type":"buildBetaDetails","id":"bd1","attributes":{"externalBuildState":"READY_FOR_BETA_SUBMISSION","autoNotifyEnabled":false}}}`
				case "GET /v1/builds/b1/betaBuildLocalizations":
					payload = `{"data":[{"type":"betaBuildLocalizations","id":"bl1","attributes":{"locale":"en-US","whatsNew":"Original instructions"}}]}`
				case "POST /v1/builds/b1/relationships/betaGroups":
					writes++
					w.WriteHeader(http.StatusNoContent)
					return
				default:
					if r.Method != http.MethodGet {
						writes++
					}
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if _, err := w.Write([]byte(payload)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			args := []string{"publish", "testflight", "--app", "123", "--build-id", "b1", "--group", "External"}
			if tc.notes {
				args = append(args, "--test-notes", "Original instructions", "--locale", "en-US")
			}
			if tc.submit {
				args = append(args, "--submit", "--confirm")
			}
			if tc.notify {
				args = append(args, "--notify", "--confirm-notify-all-testers")
			}
			stdout, stderr, err := runRootWithDeps(t, publishBoundOriginFixtureClient(t, server), args...)
			var result asc.TestFlightPublishResult
			if decodeErr := json.Unmarshal([]byte(stdout), &result); decodeErr != nil {
				t.Fatalf("missing receipt: stdout=%s stderr=%s err=%v", stdout, stderr, err)
			}
			if err == nil || foreignReads != 0 || writes != tc.wantWrites || result.FailureStage == "" || result.FailureCode == "" || strings.Contains(stdout, "PRIVATE-REDIRECT-CANARY") {
				t.Fatalf("redirect escaped scope: reads=%d writes=%d stdout=%s stderr=%s err=%v", foreignReads, writes, stdout, stderr, err)
			}
			if tc.notify && (result.NotificationRecovery == nil || result.NotificationRecovery.State != "unknown" || strings.Contains(stdout, `"command"`) || strings.Contains(stdout, `"arguments"`)) {
				t.Fatalf("unknown delivery offered replay: %s", stdout)
			}
		})
	}
}

func TestPublishTestflightKeepsConfiguredBundleProvenance(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		explicit, exactBundle bool
	}{
		{name: "configured bundle cannot resolve by display name"},
		{name: "configured exact bundle", exactBundle: true},
		{name: "explicit display name", explicit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const selector = "com.example.fixture"
			t.Setenv("ASC_APP_ID", selector)
			nameReads, writes, scopedReads := 0, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var payload string
				switch r.Method + " " + r.URL.Path {
				case "GET /v1/apps":
					switch {
					case r.URL.Query().Get("filter[bundleId]") == selector:
						payload = `{"data":[{"type":"apps","id":"prefix-app","attributes":{"name":"Other","bundleId":"com.example.fixture.beta"}}]}`
						if tc.exactBundle {
							payload = `{"data":[{"type":"apps","id":"123","attributes":{"name":"Fixture","bundleId":"com.example.fixture"}}]}`
						}
					case r.URL.Query().Get("filter[name]") == selector:
						nameReads++
						payload = `{"data":[{"type":"apps","id":"123","attributes":{"name":"com.example.fixture","bundleId":"com.other.fixture"}}]}`
					default:
						t.Errorf("unexpected app lookup %s", r.URL)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
				case "GET /v1/apps/123/betaGroups":
					scopedReads++
					payload = `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"Internal","isInternalGroup":true}}]}`
				case "GET /v1/builds/b1/app":
					scopedReads++
					payload = `{"data":{"type":"apps","id":"123"}}`
				case "GET /v1/builds/b1":
					scopedReads++
					payload = `{"data":{"type":"builds","id":"b1","attributes":{"version":"42","processingState":"VALID"}}}`
				case "GET /v1/betaGroups/g1/relationships/builds":
					scopedReads++
					payload = `{"data":[]}`
				case "POST /v1/builds/b1/relationships/betaGroups":
					writes++
					w.WriteHeader(http.StatusNoContent)
					return
				default:
					t.Errorf("unexpected scoped request %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if _, err := w.Write([]byte(payload)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			args := []string{"publish", "testflight", "--build-id", "b1", "--group", "Internal"}
			if tc.explicit {
				args = append(args, "--app", selector)
			}
			stdout, stderr, err := runRootWithDeps(t, publishBoundOriginFixtureClient(t, server), args...)
			if tc.explicit || tc.exactBundle {
				if err != nil || writes != 1 || scopedReads != 4 {
					t.Fatalf("scoped positive failed: reads=%d writes=%d stdout=%s stderr=%s err=%v", scopedReads, writes, stdout, stderr, err)
				}
				return
			}
			var result asc.TestFlightPublishResult
			if decodeErr := json.Unmarshal([]byte(stdout), &result); decodeErr != nil {
				t.Fatalf("missing safe failure receipt: stdout=%s err=%v", stdout, err)
			}
			if err == nil || nameReads != 0 || scopedReads != 0 || writes != 0 || result.FailureCode != "app_resolution_failed" || result.Failure != "The selected app could not be verified." {
				t.Fatalf("configured bundle lost provenance: names=%d reads=%d writes=%d stdout=%s stderr=%s err=%v", nameReads, scopedReads, writes, stdout, stderr, err)
			}
		})
	}
}
