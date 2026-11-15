package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func TestReviewDoctorLiveVersionReadiness(t *testing.T) {
	for _, test := range []struct {
		name            string
		state           string
		missingKeywords bool
		wantBlocker     string
	}{
		{name: "modern live state", state: "READY_FOR_DISTRIBUTION"},
		{name: "legacy live state", state: "READY_FOR_SALE"},
		{name: "live metadata blocker", state: "READY_FOR_DISTRIBUTION", missingKeywords: true, wantBlocker: "metadata.required.keywords"},
		{name: "non-live state remains blocked", state: "IN_REVIEW", wantBlocker: "version.state.editable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			fixture := validValidateFixture()
			fixture.version = strings.ReplaceAll(fixture.version, "PREPARE_FOR_SUBMISSION", test.state)
			if test.state == "READY_FOR_SALE" {
				fixture.version = strings.ReplaceAll(fixture.version, "appVersionState", "appStoreState")
			}
			if test.missingKeywords {
				fixture.versionLocs = strings.ReplaceAll(fixture.versionLocs, `"keywords":"keyword"`, `"keywords":""`)
			}
			readinessTransport := validateTestTransport(t, fixture)

			// Use the real readiness builder and its existing complete API fixture.
			// This server supplies only the review snapshot consumed before it.
			assert := handlertest.New(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet {
					assert.Respond(w, "unexpected method: %s", req.Method)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				switch req.URL.Path {
				case "/v1/apps":
					_, _ = io.WriteString(w, `{"data":[{"type":"apps","id":"app-1","attributes":{"bundleId":"app-1"}}]}`)
				case "/v1/appStoreVersions/ver-1":
					_, _ = io.WriteString(w, fixture.version)
				case "/v1/appStoreVersions/ver-1/appStoreReviewDetail":
					_, _ = io.WriteString(w, fixture.reviewDetails)
				case "/v1/apps/app-1/reviewSubmissions":
					_, _ = io.WriteString(w, `{"data":[]}`)
				default:
					assert.Respond(w, "unexpected request: %s", req.URL)
				}
			}))
			t.Cleanup(server.Close)
			client := stubClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/apps", "/v1/appStoreVersions/ver-1", "/v1/appStoreVersions/ver-1/appStoreReviewDetail", "/v1/apps/app-1/reviewSubmissions":
					forwarded := req.Clone(req.Context())
					forwarded.URL.Scheme = "http"
					forwarded.URL.Host = server.Listener.Addr().String()
					return server.Client().Transport.RoundTrip(forwarded)
				default:
					return readinessTransport(req)
				}
			}))

			stdout, stderr, err := runRootWithDeps(t, client, "review", "doctor", "--app", "app-1", "--version-id", "ver-1", "--output", "json")
			if err != nil {
				t.Fatal(err)
			}
			if stderr != "" {
				t.Fatalf("unexpected stderr: %s", stderr)
			}
			var result struct {
				NextAction     string                   `json:"nextAction"`
				Summary        validation.Summary       `json:"summary"`
				BlockingChecks []validation.CheckResult `json:"blockingChecks"`
				WarningChecks  []validation.CheckResult `json:"warningChecks"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if test.wantBlocker == "" {
				if len(result.BlockingChecks) != 0 || result.NextAction != "No action needed." {
					t.Fatalf("expected live version to need no action, got %s", stdout)
				}
			} else if len(result.BlockingChecks) != 1 || result.BlockingChecks[0].ID != test.wantBlocker || result.NextAction != result.BlockingChecks[0].Remediation {
				t.Fatalf("expected %s blocker and remediation, got %s", test.wantBlocker, stdout)
			}
			if result.Summary.Errors != len(result.BlockingChecks) || result.Summary.Blocking != len(result.BlockingChecks) || result.Summary.Warnings != len(result.WarningChecks) {
				t.Fatalf("summary does not match retained checks: %s", stdout)
			}
		})
	}
}
