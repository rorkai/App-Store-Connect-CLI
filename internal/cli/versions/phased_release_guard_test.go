package versions

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type guardRoundTrip func(*http.Request) (*http.Response, error)

func (fn guardRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

type guardCheckCall struct {
	argv []string
	env  []string
}

type guardHarness struct {
	t            *testing.T
	mu           sync.Mutex
	states       []string
	days         []int
	getErrs      int
	getErrStart  int
	getErrStatus int
	retryAfter   time.Duration
	cancelGet    int
	cancelDelay  time.Duration
	gets         int
	getTimes     []time.Time
	nullGet      int
	patches      []string
	checks       []guardCheckCall
	results      []phasedReleaseCheckResult
	checkErr     error
	timeouts     []time.Duration
	// cancelAfterCheck cancels the run once this many checks have passed.
	cancelAfterCheck int
	cancel           context.CancelFunc
	// d is the process deps with the client replaced by one served by h.
	d *shared.Deps
}

func newGuardHarness(t *testing.T) *guardHarness {
	t.Helper()
	h := &guardHarness{t: t}

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	t.Setenv("ASC_MAX_RETRIES", "0")
	client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, &http.Client{Transport: guardRoundTrip(h.serve)})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	h.d = guardDeps(func() (*asc.Client, error) { return client, nil })

	original := phasedReleaseGuardRunCheck
	phasedReleaseGuardRunCheck = h.runCheck
	t.Cleanup(func() { phasedReleaseGuardRunCheck = original })
	return h
}

func guardJSON(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func guardPhasedReleaseBody(state string, day int) string {
	return fmt.Sprintf(`{"data":{"type":"appStoreVersionPhasedReleases","id":"PR-1","attributes":{"phasedReleaseState":%q,"currentDayNumber":%d}},"links":{}}`, state, day)
}

func (h *guardHarness) serve(req *http.Request) (*http.Response, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION-1/appStoreVersionPhasedRelease":
		h.gets++
		h.getTimes = append(h.getTimes, time.Now())
		if h.gets == h.cancelGet {
			if h.cancelDelay > 0 {
				time.AfterFunc(h.cancelDelay, h.cancel)
			} else {
				h.cancel()
			}
		}
		if h.gets == h.nullGet {
			return guardJSON(http.StatusOK, `{"data":null,"links":{}}`), nil
		}
		if h.getErrs > 0 && h.gets >= h.getErrStart {
			h.getErrs--
			status := h.getErrStatus
			if status == 0 {
				status = http.StatusInternalServerError
			}
			response := guardJSON(status, fmt.Sprintf(`{"errors":[{"status":"%d","code":"TEST_ERROR","title":%q}]}`, status, http.StatusText(status)))
			if h.retryAfter > 0 {
				response.Header.Set("Retry-After", fmt.Sprint(int(h.retryAfter/time.Second)))
			}
			return response, nil
		}
		state := h.states[min(len(h.states)-1, h.gets-1)]
		return guardJSON(http.StatusOK, guardPhasedReleaseBody(state, h.days[min(len(h.days)-1, h.gets-1)])), nil
	case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionPhasedReleases/PR-1":
		var body asc.AppStoreVersionPhasedReleaseUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			h.t.Errorf("decode patch body: %v", err)
		}
		if body.Data.ID != "PR-1" || body.Data.Type != asc.ResourceTypeAppStoreVersionPhasedReleases {
			h.t.Errorf("patch data = %+v, want PR-1 appStoreVersionPhasedReleases", body.Data)
		}
		state := string(body.Data.Attributes.PhasedReleaseState)
		h.patches = append(h.patches, state)
		return guardJSON(http.StatusOK, guardPhasedReleaseBody(state, h.days[min(len(h.days)-1, h.gets-1)])), nil
	default:
		h.t.Errorf("unexpected request %s %s", req.Method, req.URL.String())
		return guardJSON(http.StatusNotFound, `{"errors":[]}`), nil
	}
}

func (h *guardHarness) runCheck(_ context.Context, timeout time.Duration, argv, env []string) (phasedReleaseCheckResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.timeouts = append(h.timeouts, timeout)
	h.checks = append(h.checks, guardCheckCall{argv: argv, env: env})
	if h.checkErr != nil {
		return phasedReleaseCheckResult{}, h.checkErr
	}
	if h.cancelAfterCheck > 0 && len(h.checks) == h.cancelAfterCheck {
		h.cancel()
	}
	index := min(len(h.results)-1, len(h.checks)-1)
	return h.results[index], nil
}

func captureGuardOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	stdoutResult, stderrResult := make(chan string, 1), make(chan string, 1)
	go func() { data, _ := io.ReadAll(stdoutReader); stdoutResult <- string(data) }()
	go func() { data, _ := io.ReadAll(stderrReader); stderrResult <- string(data) }()

	originalStdout, originalStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	defer func() { os.Stdout, os.Stderr = originalStdout, originalStderr }()
	fn()
	os.Stdout, os.Stderr = originalStdout, originalStderr
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	return <-stdoutResult, <-stderrResult
}

func runGuardCommand(t *testing.T, ctx context.Context, d *shared.Deps, args ...string) (string, string, error) {
	t.Helper()
	var runErr error
	stdout, stderr := captureGuardOutput(t, func() {
		cmd := newPhasedReleaseGuardCommand(d)
		if runErr = cmd.FlagSet.Parse(args); runErr == nil {
			runErr = cmd.Exec(ctx, cmd.FlagSet.Args())
		}
	})
	return stdout, stderr, runErr
}

func (h *guardHarness) run(args ...string) (string, string, error) {
	var runErr error
	stdout, stderr := captureGuardOutput(h.t, func() {
		synctest.Test(h.t, func(*testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			h.cancel = cancel
			cmd := newPhasedReleaseGuardCommand(h.d)
			if runErr = cmd.FlagSet.Parse(args); runErr == nil {
				runErr = cmd.Exec(ctx, cmd.FlagSet.Args())
			}
		})
	})
	return stdout, stderr, runErr
}

// guardDeps returns the process deps with client as their client, so the
// guard's own lines, the check's output and the poller's notices all reach the
// captured process streams in order.
func guardDeps(client func() (*asc.Client, error)) *shared.Deps {
	d := shared.DefaultDeps()
	d.Client = client
	return d
}

func decodeGuardResult(t *testing.T, stdout string) asc.PhasedReleaseGuardResult {
	t.Helper()
	var result asc.PhasedReleaseGuardResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode receipt %q: %v", stdout, err)
	}
	return result
}

var guardBaseArgs = []string{"--version-id", "VERSION-1", "--check-timeout", "2m", "--poll-interval", "1ms", "--output", "json"}

func guardArgs(extra ...string) []string {
	args := append(slices.Clone(guardBaseArgs), extra...)
	return append(args, "--", "./health-check", "--threshold", "$(touch pwned)")
}

func TestPhasedReleaseGuardValidationRejectsBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"missing version", []string{"--confirm", "--", "./check"}, "Error: --version-id is required\n"},
		{"missing command", []string{"--version-id", "VERSION-1", "--confirm"}, "Error: a check command is required after --\n"},
		{"neither confirm nor dry run", []string{"--version-id", "VERSION-1", "--", "./check"}, "Error: --confirm is required unless --dry-run is set\n"},
		{"confirm and dry run", []string{"--version-id", "VERSION-1", "--confirm", "--dry-run", "--", "./check"}, "Error: --confirm and --dry-run are mutually exclusive\n"},
		{"zero poll interval", []string{"--version-id", "VERSION-1", "--confirm", "--poll-interval", "0s", "--", "./check"}, "Error: --poll-interval must be greater than 0\n"},
		{"zero check timeout", []string{"--version-id", "VERSION-1", "--confirm", "--check-timeout", "0s", "--", "./check"}, "Error: --check-timeout must be greater than 0\n"},
		{"negative max polls", []string{"--version-id", "VERSION-1", "--confirm", "--max-polls", "-1", "--", "./check"}, "Error: --max-polls must be greater than or equal to 0\n"},
		{"complete day out of range", []string{"--version-id", "VERSION-1", "--confirm", "--complete-on-day", "8", "--", "./check"}, "Error: --complete-on-day must be between 1 and 7\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := guardDeps(func() (*asc.Client, error) {
				t.Fatal("client created before validation finished")
				return nil, nil
			})
			stdout, stderr, err := runGuardCommand(t, context.Background(), d, test.args...)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
		})
	}
}

func TestPhasedReleaseGuardFailedCheckPausesRelease(t *testing.T) {
	h := newGuardHarness(t)
	h.states, h.days = []string{"ACTIVE"}, []int{3}
	h.results = []phasedReleaseCheckResult{{ExitCode: 2}}

	stdout, stderr, err := h.run(guardArgs("--confirm")...)
	if err == nil || errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want reported check failure", err)
	}
	var reported shared.ReportedError
	if !errors.As(err, &reported) {
		t.Fatalf("error = %T %v, want reported error after receipt", err, err)
	}
	if !slices.Equal(h.patches, []string{"PAUSED"}) {
		t.Fatalf("patches = %v, want [PAUSED]", h.patches)
	}
	if !slices.Equal(h.timeouts, []time.Duration{2 * time.Minute}) {
		t.Fatalf("check timeouts = %v, want one 2m check", h.timeouts)
	}
	if got, want := h.checks[0].argv, []string{"./health-check", "--threshold", "$(touch pwned)"}; !slices.Equal(got, want) {
		t.Fatalf("argv = %q, want verbatim %q", got, want)
	}
	for _, want := range []string{
		"ASC_PHASED_RELEASE_VERSION_ID=VERSION-1",
		"ASC_PHASED_RELEASE_ID=PR-1",
		"ASC_PHASED_RELEASE_STATE=ACTIVE",
		"ASC_PHASED_RELEASE_DAY=3",
		"ASC_PHASED_RELEASE_CHECK=1",
	} {
		if !slices.Contains(h.checks[0].env, want) {
			t.Fatalf("check env %v missing %q", h.checks[0].env, want)
		}
	}
	result := decodeGuardResult(t, stdout)
	if result.Outcome != "paused" || result.PhasedReleaseState != asc.PhasedReleaseStatePaused || result.DryRun {
		t.Fatalf("receipt = %+v, want paused PAUSED non-dry-run", result)
	}
	if len(result.Checks) != 1 || result.Checks[0] != (asc.PhasedReleaseGuardCheck{CurrentDayNumber: 3, ExitCode: 2, Action: "pause"}) {
		t.Fatalf("checks = %+v, want one pause on day 3 exit 2", result.Checks)
	}
	if !strings.Contains(stderr, "check 1 failed with exit status 2") {
		t.Fatalf("stderr = %q, want check failure progress line", stderr)
	}
}

func TestPhasedReleaseGuardDryRunReportsWithoutPatching(t *testing.T) {
	tests := []struct {
		name        string
		days        []int
		results     []phasedReleaseCheckResult
		extra       []string
		wantOutcome string
		wantCheck   asc.PhasedReleaseGuardCheck
		wantErr     bool
	}{
		{
			name:        "timed out check would pause",
			days:        []int{2},
			results:     []phasedReleaseCheckResult{{ExitCode: -1, TimedOut: true}},
			wantOutcome: "would-pause",
			wantCheck:   asc.PhasedReleaseGuardCheck{CurrentDayNumber: 2, ExitCode: -1, TimedOut: true, Action: "would-pause"},
			wantErr:     true,
		},
		{
			name:        "passing check on completion day would complete",
			days:        []int{5},
			results:     []phasedReleaseCheckResult{{ExitCode: 0}},
			extra:       []string{"--complete-on-day", "5"},
			wantOutcome: "would-complete",
			wantCheck:   asc.PhasedReleaseGuardCheck{CurrentDayNumber: 5, Action: "would-complete"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newGuardHarness(t)
			h.states, h.days, h.results = []string{"ACTIVE"}, test.days, test.results

			stdout, _, err := h.run(guardArgs(append([]string{"--dry-run"}, test.extra...)...)...)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, test.wantErr)
			}
			if len(h.patches) != 0 {
				t.Fatalf("dry run sent patches %v", h.patches)
			}
			result := decodeGuardResult(t, stdout)
			if result.Outcome != test.wantOutcome || !result.DryRun || result.PhasedReleaseState != asc.PhasedReleaseStateActive {
				t.Fatalf("receipt = %+v, want %s dry run still ACTIVE", result, test.wantOutcome)
			}
			if len(result.Checks) != 1 || result.Checks[0] != test.wantCheck {
				t.Fatalf("checks = %+v, want [%+v]", result.Checks, test.wantCheck)
			}
		})
	}
}

func TestPhasedReleaseGuardCompletesAfterPassingChecksReachDay(t *testing.T) {
	h := newGuardHarness(t)
	h.states, h.days = []string{"ACTIVE", "ACTIVE"}, []int{3, 4}
	h.results = []phasedReleaseCheckResult{{ExitCode: 0}}

	stdout, _, err := h.run(guardArgs("--confirm", "--complete-on-day", "4")...)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !slices.Equal(h.patches, []string{"COMPLETE"}) {
		t.Fatalf("patches = %v, want [COMPLETE]", h.patches)
	}
	result := decodeGuardResult(t, stdout)
	if result.Outcome != "completed" || result.PhasedReleaseState != asc.PhasedReleaseStateComplete {
		t.Fatalf("receipt = %+v, want completed COMPLETE", result)
	}
	wantChecks := []asc.PhasedReleaseGuardCheck{{CurrentDayNumber: 3, Action: "none"}, {CurrentDayNumber: 4, Action: "complete"}}
	if !slices.Equal(result.Checks, wantChecks) {
		t.Fatalf("checks = %+v, want %+v", result.Checks, wantChecks)
	}
}

func TestPhasedReleaseGuardStopsCleanlyWhenReleaseLeavesActive(t *testing.T) {
	tests := []struct {
		name        string
		states      []string
		exitCode    int
		extra       []string
		wantOutcome string
		wantChecks  int
		getErrs     int
	}{
		{"already complete", []string{"COMPLETE"}, 0, nil, "already-complete", 0, 0},
		{"inactive", []string{"INACTIVE"}, 0, nil, "inactive", 0, 0},
		{"paused by someone else mid run", []string{"ACTIVE", "PAUSED"}, 0, nil, "already-paused", 1, 0},
		{"max polls reached", []string{"ACTIVE"}, 0, []string{"--max-polls", "2"}, "max-polls", 2, 0},
		{"paused during check is never completed", []string{"ACTIVE", "PAUSED"}, 0, []string{"--complete-on-day", "1"}, "already-paused", 1, 0},
		{"completed during failing check is not paused", []string{"ACTIVE", "COMPLETE"}, 1, nil, "already-complete", 1, 0},
		{"paused while safety read recovers", []string{"ACTIVE", "PAUSED"}, 0, []string{"--complete-on-day", "1"}, "already-paused", 1, 1},
		{"completed while safety read recovers", []string{"ACTIVE", "COMPLETE"}, 1, nil, "already-complete", 1, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newGuardHarness(t)
			h.states, h.days = test.states, []int{2}
			h.results = []phasedReleaseCheckResult{{ExitCode: test.exitCode}}
			h.getErrStart, h.getErrs = 2, test.getErrs

			stdout, _, err := h.run(guardArgs(append([]string{"--confirm"}, test.extra...)...)...)
			if err != nil {
				t.Fatalf("error = %v, want clean exit", err)
			}
			if len(h.patches) != 0 {
				t.Fatalf("patches = %v, want none", h.patches)
			}
			if len(h.checks) != test.wantChecks {
				t.Fatalf("checks run = %d, want %d", len(h.checks), test.wantChecks)
			}
			result := decodeGuardResult(t, stdout)
			if result.Outcome != test.wantOutcome || len(result.Checks) != test.wantChecks {
				t.Fatalf("receipt = %+v, want %s with %d checks", result, test.wantOutcome, test.wantChecks)
			}
		})
	}
}

func TestPhasedReleaseGuardMissingReleaseStopsBeforeCheckOrMutation(t *testing.T) {
	for _, test := range []struct {
		name       string
		nullGet    int
		wantChecks int
		getErrs    int
	}{
		{"initial read", 1, 0, 0},
		{"recheck before pause", 2, 1, 0},
		{"recheck after transient error", 3, 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newGuardHarness(t)
			h.states, h.days = []string{"ACTIVE"}, []int{2}
			h.results = []phasedReleaseCheckResult{{ExitCode: 1}}
			h.nullGet = test.nullGet
			h.getErrStart, h.getErrs = 2, test.getErrs

			stdout, _, err := h.run(guardArgs("--confirm")...)
			if err == nil || err.Error() != "phased-release guard: version VERSION-1 has no phased release" {
				t.Fatalf("error = %v, want missing phased release error", err)
			}
			if h.gets != test.nullGet || len(h.checks) != test.wantChecks || len(h.patches) != 0 {
				t.Fatalf("reads = %d checks = %d patches = %v, want %d reads, %d checks, and no mutation", h.gets, len(h.checks), h.patches, test.nullGet, test.wantChecks)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want no receipt after a missing release", stdout)
			}
		})
	}
}

func TestPhasedReleaseGuardInterruptedRunNeverChangesState(t *testing.T) {
	startErr := errors.New("exec: \"./health-check\": file does not exist")
	tests := []struct {
		name             string
		checkErr         error
		cancelAfterCheck int
		want             error
	}{
		{"canceled during check", context.Canceled, 0, context.Canceled},
		{"canceled while waiting for next poll", nil, 1, context.Canceled},
		{"check cannot start", startErr, 0, startErr},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newGuardHarness(t)
			h.states, h.days = []string{"ACTIVE"}, []int{2}
			h.results = []phasedReleaseCheckResult{{ExitCode: 0}}
			h.checkErr, h.cancelAfterCheck = test.checkErr, test.cancelAfterCheck

			stdout, _, err := h.run(guardArgs("--confirm")...)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if len(h.patches) != 0 {
				t.Fatalf("patches = %v, want none", h.patches)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want no receipt", stdout)
			}
		})
	}
}

func TestPhasedReleaseGuardTransientReadFailures(t *testing.T) {
	for _, test := range []struct {
		name        string
		start       int
		failures    int
		status      int
		cancelGet   int
		wantGets    int
		wantChecks  int
		wantErr     string
		retryAfter  time.Duration
		cancelDelay time.Duration
	}{
		{"initial read recovers", 1, 1, 500, 0, 3, 1, "", 0, 0},
		{"safety read recovers after server error", 2, 1, 500, 0, 3, 1, "", 0, 0},
		{"safety read recovers after rate limit", 2, 1, 429, 0, 3, 1, "", 0, 0},
		{"initial read exhausts tolerance", 1, 100, 500, 0, 6, 0, "giving up after 6 consecutive transient App Store Connect errors", 0, 0},
		{"safety read exhausts tolerance", 2, 100, 500, 0, 7, 1, "giving up after 6 consecutive transient App Store Connect errors", 0, 0},
		{"safety read is forbidden", 2, 1, 403, 0, 2, 1, "Forbidden", 0, 0},
		{"canceled during safety read", 2, 1, 500, 2, 2, 1, "context canceled", 0, 0},
		{"initial read honors exhausted client retry after", 1, 2, 429, 0, 4, 1, "", 10 * time.Second, 0},
		{"canceled while initial read honors retry after", 1, 2, 429, 2, 2, 0, "context canceled", 10 * time.Second, 2 * time.Second},
		{"safety read honors exhausted client retry after", 2, 2, 429, 0, 4, 1, "", 10 * time.Second, 0},
		{"canceled while honoring retry after", 2, 2, 429, 3, 3, 1, "context canceled", 10 * time.Second, 2 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newGuardHarness(t)
			h.states, h.days = []string{"ACTIVE"}, []int{2}
			h.results = []phasedReleaseCheckResult{{ExitCode: 2}, {ExitCode: 0}}
			h.getErrStart, h.getErrs, h.getErrStatus, h.cancelGet = test.start, test.failures, test.status, test.cancelGet
			h.retryAfter, h.cancelDelay = test.retryAfter, test.cancelDelay
			if test.retryAfter > 0 {
				t.Setenv("ASC_MAX_RETRIES", "1")
			}

			pollInterval := "15m"
			if test.start == 1 {
				pollInterval = "1s"
			}
			stdout, stderr, err := h.run(guardArgs("--confirm", "--max-polls", "1", "--poll-interval", pollInterval)...)
			if h.gets != test.wantGets || len(h.checks) != test.wantChecks {
				t.Fatalf("reads = %d checks = %d, want %d reads and %d checks; error = %v", h.gets, len(h.checks), test.wantGets, test.wantChecks, err)
			}
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
				if len(h.patches) != 0 || stdout != "" {
					t.Fatalf("patches = %v stdout = %q, want no mutation or receipt", h.patches, stdout)
				}
			} else {
				var reported shared.ReportedError
				if !errors.As(err, &reported) || !slices.Equal(h.patches, []string{"PAUSED"}) {
					t.Fatalf("error = %v patches = %v, want reported failure and one PAUSED mutation", err, h.patches)
				}
				result := decodeGuardResult(t, stdout)
				wantChecks := []asc.PhasedReleaseGuardCheck{{CurrentDayNumber: 2, ExitCode: 2, Action: "pause"}}
				if result.Outcome != "paused" || !slices.Equal(result.Checks, wantChecks) {
					t.Fatalf("receipt = %+v, want paused with original failed check %+v", result, wantChecks)
				}
				if test.start == 2 && h.getTimes[2].Sub(h.getTimes[1]) >= time.Minute {
					t.Fatalf("safety retry took %s, want less than a minute despite 15m health-check interval", h.getTimes[2].Sub(h.getTimes[1]))
				}
				recoveryRead := len(h.getTimes) - 1
				if test.start == 1 {
					recoveryRead = test.failures
				}
				if delay := h.getTimes[recoveryRead].Sub(h.getTimes[recoveryRead-1]); delay < test.retryAfter {
					t.Fatalf("recovery read took %s, must honor Retry-After %s after the client retry budget is exhausted", delay, test.retryAfter)
				}
			}
			if test.status != 403 && test.cancelGet == 0 && !strings.Contains(stderr, "transient App Store Connect error while waiting (1/5)") {
				t.Fatalf("stderr = %q, want tolerated failure progress", stderr)
			}
		})
	}
}

func TestRunPhasedReleaseCheckSendsOutputToStderrAndReportsStatus(t *testing.T) {
	if os.Getenv("ASC_GUARD_HELPER_PROCESS") == "1" {
		fmt.Fprintf(os.Stdout, "day=%s\n", os.Getenv("ASC_PHASED_RELEASE_DAY"))
		os.Exit(3)
	}

	var result phasedReleaseCheckResult
	var runErr error
	stdout, stderr := captureGuardOutput(t, func() {
		result, runErr = runPhasedReleaseCheck(context.Background(), time.Hour,
			[]string{os.Args[0], "-test.run=^TestRunPhasedReleaseCheckSendsOutputToStderrAndReportsStatus$"},
			[]string{"ASC_GUARD_HELPER_PROCESS=1", "ASC_PHASED_RELEASE_DAY=4"})
	})

	if runErr != nil {
		t.Fatalf("run error = %v", runErr)
	}
	if result != (phasedReleaseCheckResult{ExitCode: 3}) {
		t.Fatalf("result = %+v, want exit 3", result)
	}
	if len(stdout) != 0 {
		t.Fatalf("check output leaked to stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "day=4") {
		t.Fatalf("stderr = %q, want check output with env", stderr)
	}

	expired, err := runPhasedReleaseCheck(context.Background(), 0, []string{os.Args[0]}, nil)
	if err != nil || !expired.TimedOut {
		t.Fatalf("expired check = %+v, %v; want timed out failure", expired, err)
	}
}
