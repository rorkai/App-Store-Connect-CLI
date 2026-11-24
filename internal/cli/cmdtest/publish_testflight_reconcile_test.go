package cmdtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestPublishTestflightPreflightsAndReconcilesExternalRelease(t *testing.T) {
	for _, tc := range []struct {
		name           string
		submit         bool
		failure        string
		mixed          bool
		wrongApp       bool
		wrongOwnerType bool
		invalidBuild   string
	}{
		{name: "submit", submit: true},
		{name: "without submit"},
		{name: "mixed groups", submit: true, mixed: true},
		{name: "mixed groups without submit", mixed: true},
		{name: "without submit wrong app", wrongApp: true},
		{name: "submit wrong app", submit: true, wrongApp: true},
		{name: "wrong owner type", wrongOwnerType: true},
		{name: "wrong build id", invalidBuild: "id"},
		{name: "wrong build type", invalidBuild: "type"},
		{name: "notification send unknown", failure: "send"},
		{name: "notification not attempted", failure: "read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ready, assigned, submitted := false, false, false
			writes, buildReads, ownerReads := 0, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var payload string
				switch r.Method + " " + r.URL.Path {
				case "GET /v1/apps/123/betaGroups":
					payload = `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"External","isInternalGroup":false}}]}`
					if tc.mixed {
						payload = `{"data":[{"type":"betaGroups","id":"g1","attributes":{"name":"External","isInternalGroup":false}},{"type":"betaGroups","id":"g2","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}}]}`
					}
				case "GET /v1/builds/b1":
					buildReads++
					if tc.wrongApp || tc.wrongOwnerType {
						t.Error("build content read before ownership was verified")
					}
					payload = `{"data":{"type":"builds","id":"b1","attributes":{"version":"42","processingState":"VALID"}}}`
					switch tc.invalidBuild {
					case "id":
						payload = `{"data":{"type":"builds","id":"other-build","attributes":{"version":"42","processingState":"VALID"}}}`
					case "type":
						payload = `{"data":{"type":"apps","id":"b1","attributes":{"version":"42","processingState":"VALID"}}}`
					}
				case "GET /v1/builds/b1/app":
					ownerReads++
					if ownerReads > 1 {
						t.Error("build ownership was read more than once")
					}
					payload = `{"data":{"type":"apps","id":"123"}}`
					if tc.wrongApp {
						payload = `{"data":{"type":"apps","id":"other-app"}}`
					}
					if tc.wrongOwnerType {
						payload = `{"data":{"type":"builds","id":"123"}}`
					}
				case "GET /v1/builds/b1/betaAppReviewSubmission":
					if submitted {
						payload = `{"data":{"type":"betaAppReviewSubmissions","id":"s1"}}`
					} else {
						w.WriteHeader(404)
						payload = `{"errors":[{"status":"404","code":"NOT_FOUND","title":"missing"}]}`
					}
				case "GET /v1/betaAppLocalizations":
					if ready {
						payload = `{"data":[{"type":"betaAppLocalizations","id":"l1","attributes":{"locale":"en-US","description":"Test app","feedbackEmail":"test@example.com"}}]}`
					} else {
						payload = `{"data":[]}`
					}
				case "GET /v1/betaAppReviewDetails":
					payload = `{"data":[{"type":"betaAppReviewDetails","id":"d1","attributes":{"contactFirstName":"Dev","contactLastName":"Tester","contactPhone":"+1 408 555 0100","contactEmail":"test@example.com"}}]}`
				case "GET /v1/builds/b1/buildBetaDetail":
					if tc.failure == "read" {
						w.WriteHeader(http.StatusForbidden)
						payload = `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Notification settings unavailable"}]}`
					} else {
						payload = `{"data":{"type":"buildBetaDetails","id":"bd1","attributes":{"externalBuildState":"READY_FOR_BETA_SUBMISSION"}}}`
					}
				case "GET /v1/betaGroups/g1/relationships/builds", "GET /v1/betaGroups/g2/relationships/builds":
					if tc.wrongApp {
						t.Error("group membership read before build ownership was verified")
					}
					if assigned {
						payload = `{"data":[{"type":"builds","id":"b1"}]}`
					} else {
						payload = `{"data":[]}`
					}
				case "POST /v1/builds/b1/relationships/betaGroups":
					var body struct {
						Data []struct {
							ID string `json:"id"`
						} `json:"data"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					ids := make([]string, 0, len(body.Data))
					for _, group := range body.Data {
						ids = append(ids, group.ID)
					}
					wantIDs := []string{"g1"}
					if tc.mixed {
						wantIDs = append(wantIDs, "g2")
					}
					slices.Sort(ids)
					if !slices.Equal(ids, wantIDs) {
						t.Errorf("assigned groups=%v, want %v", ids, wantIDs)
					}
					assigned = true
					writes++
					w.WriteHeader(204)
					return
				case "GET /v1/builds/b1/betaBuildLocalizations":
					if tc.wrongApp {
						t.Error("test notes read before build ownership was verified")
					}
					payload = `{"data":[{"type":"betaBuildLocalizations","id":"bl1","attributes":{"locale":"en-US","whatsNew":"Existing instructions"}}]}`
				case "POST /v1/buildBetaNotifications":
					writes++
					if tc.failure == "send" {
						w.WriteHeader(http.StatusForbidden)
						payload = `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Notification failed"}]}`
					} else {
						payload = `{"data":{"type":"buildBetaNotifications","id":"n1"}}`
					}
				case "POST /v1/betaAppReviewSubmissions":
					if !tc.submit {
						t.Error("review submission was not requested")
					}
					submitted = true
					writes++
					payload = `{"data":{"type":"betaAppReviewSubmissions","id":"s1"}}`
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(500)
					return
				}
				if _, err := w.Write([]byte(payload)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client := stubServerClient(t, server)
			run := func() (string, string, error) {
				buildReads, ownerReads = 0, 0
				groupNames := "External"
				if tc.mixed {
					groupNames = "External,Internal"
				}
				args := []string{"publish", "testflight", "--app", "123", "--build-id", "b1", "--group", groupNames, "--notify", "--test-notes", "Existing instructions", "--locale", "en-US"}
				if tc.wrongApp || tc.wrongOwnerType {
					args = append(args, "--wait")
				}
				if tc.submit {
					args = append(args, "--submit", "--confirm")
				}
				return runRootWithDeps(t, client, args...)
			}
			if tc.submit && !tc.wrongApp {
				stdout, stderr, err := run()
				if err == nil || writes != 0 || !strings.Contains(err.Error()+stderr, "feedback email") {
					t.Fatalf("writes=%d stdout=%s stderr=%s err=%v", writes, stdout, stderr, err)
				}
			}
			ready = true
			stdout, stderr, err := run()
			if tc.wrongApp || tc.wrongOwnerType {
				if err == nil || writes != 0 || buildReads != 0 || ownerReads != 1 || !strings.Contains(err.Error()+stderr, "does not belong to app 123") {
					t.Fatalf("wrong-app writes=%d stdout=%s stderr=%s err=%v", writes, stdout, stderr, err)
				}
				return
			}
			if tc.invalidBuild != "" {
				if err == nil || writes != 0 || buildReads != 1 || !strings.Contains(err.Error()+stderr, "does not match requested build b1") {
					t.Fatalf("invalid build: reads=%d writes=%d stdout=%s stderr=%s err=%v", buildReads, writes, stdout, stderr, err)
				}
				return
			}
			wantWrites := 2
			if tc.submit {
				wantWrites = 3
			}
			if tc.failure == "read" {
				wantWrites = 1
			}
			if (err != nil) != (tc.failure != "") || writes != wantWrites {
				t.Fatalf("writes=%d stdout=%s stderr=%s err=%v", writes, stdout, stderr, err)
			}
			var output asc.TestFlightPublishResult
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatal(err)
			}
			if tc.submit && (output.BetaReviewSubmitted == nil || !*output.BetaReviewSubmitted) {
				t.Fatalf("output=%s", stdout)
			}
			if tc.failure != "" {
				state := "unknown"
				if tc.failure == "read" {
					state = "not_attempted"
				}
				recovery := output.NotificationRecovery
				if output.Status != "partial" || output.FailureStage != "notification" || recovery == nil || recovery.State != state ||
					recovery.BuildID != "b1" || recovery.Command != "asc" || !slices.Equal(recovery.Arguments, []string{"testflight", "notifications", "send", "--build-id", "b1"}) {
					t.Fatalf("notification recovery=%s", stdout)
				}
			}
			stdout, stderr, err = run()
			if err != nil || writes != wantWrites {
				t.Fatalf("repeat writes=%d stdout=%s stderr=%s err=%v", writes, stdout, stderr, err)
			}
			output = asc.TestFlightPublishResult{}
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatal(err)
			}
			if output.NotificationRecovery == nil || output.NotificationRecovery.State != "unknown" || !strings.Contains(output.NotificationRecovery.Message, "do not replay") {
				t.Fatalf("repeat must not imply notification delivery is known: %s", stdout)
			}
		})
	}
}
