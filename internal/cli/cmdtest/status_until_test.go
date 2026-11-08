package cmdtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// installStatusUntilTransport serves each path's bodies in order, repeating
// the last one.
func installStatusUntilTransport(t *testing.T, sequences map[string][]string) {
	t.Helper()

	var mu sync.Mutex
	calls := map[string]int{}
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("filter[betaAppReviewSubmission.betaReviewState]") != "" {
			return statusJSONResponse(`{"data":[],"links":{"next":""}}`), nil
		}
		mu.Lock()
		defer mu.Unlock()
		bodies, ok := sequences[req.URL.Path]
		if !ok {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		}
		index := min(calls[req.URL.Path], len(bodies)-1)
		calls[req.URL.Path]++
		return statusJSONResponse(bodies[index]), nil
	}))
}

func runStatusUntil(t *testing.T, args ...string) (string, string, int) {
	t.Helper()

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run(append([]string{"status", "--app", "123456789", "--poll-interval", "1ms"}, args...), "1.0.0")
	})
	return stdout, stderr, code
}

func setupStatusUntilTest(t *testing.T) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_MAX_RETRIES", "0")
	t.Setenv("ASC_READ_ONLY", "")
}

func statusUntilVersionsBody(state string) string {
	return fmt.Sprintf(`{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.2.3","appVersionState":%q,"createdDate":"2026-03-15T00:00:00Z"}}],"links":{"next":""}}`, state)
}

func statusUntilBuildsBody(processingState, internalState string) string {
	return fmt.Sprintf(`{
		"data":[{
			"type":"builds",
			"id":"build-9",
			"attributes":{"version":"9","uploadedDate":"2026-03-15T00:00:00Z","processingState":%q},
			"relationships":{
				"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-1"}},
				"buildBetaDetail":{"data":{"type":"buildBetaDetails","id":"bbd-9"}}
			}
		}],
		"included":[
			{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}},
			{"type":"buildBetaDetails","id":"bbd-9","attributes":{"internalBuildState":%q}}
		],
		"links":{"next":""}
	}`, processingState, internalState)
}

func statusUntilResult(t *testing.T, stdout string) asc.StatusUntilResult {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	var result asc.StatusUntilResult
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &result); err != nil {
		t.Fatalf("unmarshal final result %q: %v", lines[len(lines)-1], err)
	}
	return result
}

func TestStatusUntilStopsAtConditionWithExitCode(t *testing.T) {
	versionsPath := "/v1/apps/123456789/appStoreVersions"
	tests := []struct {
		name      string
		until     string
		include   string
		path      string
		bodies    []string
		wantCode  int
		want      asc.StatusUntilResult
		wantNotes string
	}{
		{
			name:    "review-done approved",
			until:   "review-done",
			include: "appstore",
			path:    versionsPath,
			bodies: []string{
				statusUntilVersionsBody("WAITING_FOR_REVIEW"),
				statusUntilVersionsBody("IN_REVIEW"),
				statusUntilVersionsBody("PENDING_DEVELOPER_RELEASE"),
			},
			wantCode: cmd.ExitSuccess,
			want:     asc.StatusUntilResult{Until: "review-done", Reached: true, Outcome: "approved", State: "PENDING_DEVELOPER_RELEASE", Polls: 3},
		},
		{
			name:    "review-done rejected",
			until:   "review-done",
			include: "appstore",
			path:    versionsPath,
			bodies: []string{
				statusUntilVersionsBody("IN_REVIEW"),
				statusUntilVersionsBody("REJECTED"),
			},
			wantCode:  cmd.ExitError,
			want:      asc.StatusUntilResult{Until: "review-done", Reached: true, Outcome: "rejected", State: "REJECTED", Polls: 2},
			wantNotes: "status: --until review-done ended with outcome rejected (state REJECTED)",
		},
		{
			name:    "ready-for-sale waits past approval",
			until:   "ready-for-sale",
			include: "appstore",
			path:    versionsPath,
			bodies: []string{
				statusUntilVersionsBody("PENDING_DEVELOPER_RELEASE"),
				statusUntilVersionsBody("PROCESSING_FOR_DISTRIBUTION"),
				statusUntilVersionsBody("READY_FOR_DISTRIBUTION"),
			},
			wantCode: cmd.ExitSuccess,
			want:     asc.StatusUntilResult{Until: "ready-for-sale", Reached: true, Outcome: "ready-for-sale", State: "READY_FOR_DISTRIBUTION", Polls: 3},
		},
		{
			name:    "processed valid",
			until:   "processed",
			include: "builds",
			path:    "/v1/builds",
			bodies: []string{
				statusUntilBuildsBody("PROCESSING", ""),
				statusUntilBuildsBody("VALID", ""),
			},
			wantCode: cmd.ExitSuccess,
			want:     asc.StatusUntilResult{Until: "processed", Reached: true, Outcome: "processed", State: "VALID", Polls: 2},
		},
		{
			name:    "processed failed",
			until:   "processed",
			include: "builds",
			path:    "/v1/builds",
			bodies: []string{
				statusUntilBuildsBody("PROCESSING", ""),
				statusUntilBuildsBody("FAILED", ""),
			},
			wantCode:  cmd.ExitError,
			want:      asc.StatusUntilResult{Until: "processed", Reached: true, Outcome: "failed", State: "FAILED", Polls: 2},
			wantNotes: "status: --until processed ended with outcome failed (state FAILED)",
		},
		{
			name:    "testflight-ready",
			until:   "testflight-ready",
			include: "testflight",
			path:    "/v1/builds",
			bodies: []string{
				statusUntilBuildsBody("VALID", "PROCESSING"),
				statusUntilBuildsBody("VALID", "READY_FOR_BETA_TESTING"),
			},
			wantCode: cmd.ExitSuccess,
			want:     asc.StatusUntilResult{Until: "testflight-ready", Reached: true, Outcome: "testflight-ready", State: "READY_FOR_BETA_TESTING", Polls: 2},
		},
		{
			name:    "testflight-ready blocked on export compliance",
			until:   "testflight-ready",
			include: "testflight",
			path:    "/v1/builds",
			bodies: []string{
				statusUntilBuildsBody("VALID", "MISSING_EXPORT_COMPLIANCE"),
			},
			wantCode:  cmd.ExitError,
			want:      asc.StatusUntilResult{Until: "testflight-ready", Reached: true, Outcome: "blocked", State: "MISSING_EXPORT_COMPLIANCE", Polls: 1},
			wantNotes: "status: --until testflight-ready ended with outcome blocked (state MISSING_EXPORT_COMPLIANCE)",
		},
		{
			name:    "change after unchanged polls",
			until:   "change",
			include: "appstore",
			path:    versionsPath,
			bodies: []string{
				statusUntilVersionsBody("WAITING_FOR_REVIEW"),
				statusUntilVersionsBody("WAITING_FOR_REVIEW"),
				statusUntilVersionsBody("IN_REVIEW"),
			},
			wantCode: cmd.ExitSuccess,
			want:     asc.StatusUntilResult{Until: "change", Reached: true, Outcome: "changed", Polls: 3},
		},
		{
			name:    "max-polls ends pending",
			until:   "review-done",
			include: "appstore",
			path:    versionsPath,
			bodies: []string{
				statusUntilVersionsBody("WAITING_FOR_REVIEW"),
			},
			wantCode:  cmd.ExitPending,
			want:      asc.StatusUntilResult{Until: "review-done", Reached: false, Outcome: "pending", State: "WAITING_FOR_REVIEW", Polls: 3},
			wantNotes: "status: --until review-done not reached after 3 polls (state WAITING_FOR_REVIEW)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupStatusUntilTest(t)
			sequences := map[string][]string{test.path: test.bodies}
			if test.include == "testflight" {
				sequences["/v1/betaAppReviewSubmissions"] = []string{`{"data":[],"links":{"next":""}}`}
			}
			installStatusUntilTransport(t, sequences)

			stdout, stderr, code := runStatusUntil(t, "--include", test.include, "--until", test.until, "--max-polls", "3", "--output", "json")

			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, test.wantCode, stdout, stderr)
			}
			if strings.TrimSpace(stderr) != test.wantNotes {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantNotes)
			}
			result := statusUntilResult(t, stdout)
			if result != test.want {
				t.Fatalf("result = %+v, want %+v", result, test.want)
			}
		})
	}
}

func TestStatusUntilTimeoutEndsPending(t *testing.T) {
	setupStatusUntilTest(t)
	installStatusUntilTransport(t, map[string][]string{
		"/v1/apps/123456789/appStoreVersions": {statusUntilVersionsBody("IN_REVIEW")},
	})

	stdout, stderr, code := runStatusUntil(t, "--include", "appstore", "--until", "review-done", "--timeout", "50ms", "--output", "json")

	if code != cmd.ExitPending {
		t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitPending, stdout, stderr)
	}
	result := statusUntilResult(t, stdout)
	if result.Reached || result.Outcome != "pending" || result.State != "IN_REVIEW" || result.Polls < 1 {
		t.Fatalf("unexpected pending result %+v", result)
	}
	if !strings.Contains(stderr, "status: --until review-done not reached after") {
		t.Fatalf("expected pending notice on stderr, got %q", stderr)
	}
}

func TestStatusUntilTableOutputEndsWithResultRow(t *testing.T) {
	setupStatusUntilTest(t)
	installStatusUntilTransport(t, map[string][]string{
		"/v1/apps/123456789/appStoreVersions": {
			statusUntilVersionsBody("IN_REVIEW"),
			statusUntilVersionsBody("PENDING_DEVELOPER_RELEASE"),
		},
	})

	stdout, stderr, code := runStatusUntil(t, "--include", "appstore", "--until", "review-done", "--output", "table")

	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	tail := strings.Join(lines[len(lines)-4:], "\n")
	for _, want := range []string{"Until", "Outcome", "review-done", "approved", "PENDING_DEVELOPER_RELEASE"} {
		if !strings.Contains(tail, want) {
			t.Fatalf("expected final table rows to contain %q, got:\n%s", want, tail)
		}
	}
}

func TestStatusUntilUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unknown condition",
			args: []string{"--until", "approved"},
			want: "--until must be one of: review-done, ready-for-sale, processed, testflight-ready, change",
		},
		{
			name: "condition section excluded",
			args: []string{"--until", "review-done", "--include", "builds"},
			want: "--until review-done requires the appstore section in --include",
		},
		{
			name: "timeout without until",
			args: []string{"--watch", "--timeout", "1m"},
			want: "--timeout requires --until",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupStatusUntilTest(t)
			installStatusUntilTransport(t, map[string][]string{})

			stdout, stderr, code := runStatusUntil(t, test.args...)

			if code != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitUsage, stdout, stderr)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.want)
			}
		})
	}
}

func TestStatusJSONIncludesNextCommands(t *testing.T) {
	setupStatusUntilTest(t)
	installStatusUntilTransport(t, map[string][]string{
		"/v1/apps/123456789/appStoreVersions": {statusUntilVersionsBody("PENDING_DEVELOPER_RELEASE")},
	})

	stdout, stderr, code := runStatusUntil(t, "--include", "appstore", "--output", "json")
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr)
	}

	var payload struct {
		Summary struct {
			NextCommands []asc.StatusNextCommand `json:"nextCommands"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	want := []asc.StatusNextCommand{{Command: "asc versions release --version-id ver-1 --confirm", Reason: "Release the approved version.", Mutates: true}}
	if !slices.Equal(payload.Summary.NextCommands, want) {
		t.Fatalf("nextCommands = %+v, want %+v", payload.Summary.NextCommands, want)
	}
}

func TestStatusSuggestsReviewSubmissionOnlyForEligibleBuildAudience(t *testing.T) {
	for _, audience := range []string{"APP_STORE_ELIGIBLE", "INTERNAL_ONLY"} {
		t.Run(audience, func(t *testing.T) {
			setupStatusUntilTest(t)
			builds := strings.Replace(statusUntilBuildsBody("VALID", ""), `"processingState":"VALID"`, `"processingState":"VALID","buildAudienceType":"`+audience+`"`, 1)
			installStatusUntilTransport(t, map[string][]string{
				"/v1/builds":                          {builds},
				"/v1/apps/123456789/appStoreVersions": {statusUntilVersionsBody("PREPARE_FOR_SUBMISSION")},
			})

			stdout, stderr, code := runStatusUntil(t, "--include", "builds,appstore", "--output", "json")
			if code != cmd.ExitSuccess {
				t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
			}
			var payload struct {
				Summary struct {
					NextCommands []asc.StatusNextCommand `json:"nextCommands"`
				} `json:"summary"`
			}
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("decode status: %v", err)
			}
			want := []asc.StatusNextCommand{{Command: "asc validate --app 123456789 --version-id ver-1", Reason: "Check App Store submission readiness."}}
			if audience == "APP_STORE_ELIGIBLE" {
				want = append(want, asc.StatusNextCommand{
					Command: "asc review submit --app 123456789 --version-id ver-1 --build-id build-9 --confirm",
					Reason:  "Attach the latest build and submit the version for App Review.",
					Mutates: true,
				})
			}
			if !slices.Equal(payload.Summary.NextCommands, want) {
				t.Fatalf("nextCommands = %+v, want %+v", payload.Summary.NextCommands, want)
			}
		})
	}
}
