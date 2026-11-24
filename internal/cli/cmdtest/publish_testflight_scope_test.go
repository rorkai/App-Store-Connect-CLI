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
		{name: "scoped absolute pages", paginateDiscovery: true, paginateMembership: true},
		{name: "alternate API origin stays selected", paginateMembership: true, membershipNext: asc.EnterpriseBaseURL + "/v1/betaGroups/g1/relationships/builds?cursor=next"},
		{name: "invalid group resource path", discoveryID: "../foreign", failure: true},
		{name: "missing membership data", membershipNoData: true, failure: true},
		{name: "scoped relative membership", paginateMembership: true, membershipNext: "/v1/betaGroups/g1/relationships/builds?cursor=next"},
		{name: "missing membership is added", absent: true},
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
					payload = map[string]any{"data": []any{map[string]any{"type": groupType, "id": groupID, "attributes": map[string]any{"name": "External"}}}}
					if tc.paginateDiscovery && !page {
						next := tc.discoveryNext
						if next == "" {
							next = "https://api.appstoreconnect.apple.com/v1/apps/123/betaGroups?cursor=next&limit=200"
						}
						payload = map[string]any{"data": []any{}, "links": map[string]string{"next": next}}
					}
				case "GET /v1/builds/b1/app":
					payload = map[string]any{"data": map[string]string{"type": "apps", "id": "123"}}
				case "GET /v1/builds/b1":
					payload = map[string]any{"data": map[string]any{"type": "builds", "id": "b1", "attributes": map[string]string{"version": "42", "processingState": "VALID"}}}
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
			stdout, stderr, err := runRootWithDeps(t, publishBoundOriginFixtureClient(t, server),
				"publish", "testflight", "--app", "123", "--build-id", "b1", "--group", "External")
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
			detail, _ := json.Marshal("provider rejected notes: " + strings.Repeat("界", 4096) + "\u001b[31m")
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
		len(result.Failure) > 2048 || len(err.Error()) > 2048 || strings.Contains(result.Failure, "\x1b") {
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
		{name: "alternate API origin stays selected", next: asc.EnterpriseBaseURL + "/v1/betaAppLocalizations?filter[app]=123&cursor=next"},
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
