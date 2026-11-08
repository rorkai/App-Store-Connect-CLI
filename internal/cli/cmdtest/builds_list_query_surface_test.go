package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// buildsListQuerySurfaceRequest records what the CLI sent, so tests can assert
// both the emitted query and that rejected invocations sent nothing at all.
type buildsListQuerySurfaceRequest struct {
	calls int
	path  string
	query url.Values
}

// buildsListQuerySurfaceStub installs a server-backed ASC client that captures
// the builds request the CLI issues and replies with a minimal envelope.
func buildsListQuerySurfaceStub(t *testing.T) *buildsListQuerySurfaceRequest {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	captured := &buildsListQuerySurfaceRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		captured.calls++
		captured.path = req.URL.Path
		captured.query = req.URL.Query()
		body := `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"42"}}]}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Scheme + "://" + req.URL.Host; got != asc.BaseURL {
			t.Errorf("request origin = %s, want %s", got, asc.BaseURL)
		}
		routed := req.Clone(req.Context())
		routed.URL.Scheme = serverURL.Scheme
		routed.URL.Host = serverURL.Host
		routed.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(routed)
	})
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	return captured
}

// assertNoRequest fails when a rejected invocation reached the network, proving
// validation runs before any side effect.
func (r *buildsListQuerySurfaceRequest) assertNoRequest(t *testing.T) {
	t.Helper()

	if r.calls != 0 {
		t.Fatalf("expected validation to short-circuit before any request, got %d call(s) to %s?%s", r.calls, r.path, r.query.Encode())
	}
}

func runBuildsListQuerySurface(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	return stdout, stderr, runErr
}

func TestBuildsListBetaReviewStateEmitsFilter(t *testing.T) {
	captured := buildsListQuerySurfaceStub(t)

	stdout, stderr, err := runBuildsListQuerySurface(
		t,
		"builds", "list",
		"--app", "123456789",
		"--beta-review-state", "WAITING_FOR_REVIEW,IN_REVIEW",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}

	if captured.path != "/v1/builds" {
		t.Fatalf("expected /v1/builds path, got %q", captured.path)
	}
	if got := captured.query.Get("filter[betaAppReviewSubmission.betaReviewState]"); got != "WAITING_FOR_REVIEW,IN_REVIEW" {
		t.Fatalf("expected beta review state filter, got %q", got)
	}
	if got := captured.query.Get("filter[app]"); got != "123456789" {
		t.Fatalf("expected filter[app]=123456789, got %q", got)
	}
	if !strings.Contains(stdout, `"id":"build-1"`) {
		t.Fatalf("expected build envelope, got %q", stdout)
	}
}

func TestBuildsListBetaReviewStateNormalizesCase(t *testing.T) {
	captured := buildsListQuerySurfaceStub(t)

	_, stderr, err := runBuildsListQuerySurface(
		t,
		"builds", "list",
		"--app", "123456789",
		"--beta-review-state", "approved",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}

	if got := captured.query.Get("filter[betaAppReviewSubmission.betaReviewState]"); got != "APPROVED" {
		t.Fatalf("expected normalized APPROVED, got %q", got)
	}
}

func TestBuildsListBetaReviewStateRejectsUnknownValue(t *testing.T) {
	captured := buildsListQuerySurfaceStub(t)

	_, stderr, err := runBuildsListQuerySurface(
		t,
		"builds", "list",
		"--app", "123456789",
		"--beta-review-state", "WAITING_FOR_REVIEW,PENDING",
	)

	if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
	}
	for _, want := range []string{"--beta-review-state", "WAITING_FOR_REVIEW", "IN_REVIEW", "REJECTED", "APPROVED"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to mention %q, got %q", want, stderr)
		}
	}
	captured.assertNoRequest(t)
}

func TestBuildsBetaReviewStateRejectsBlankCSV(t *testing.T) {
	for _, command := range []string{"list", "count"} {
		t.Run(command, func(t *testing.T) {
			captured := buildsListQuerySurfaceStub(t)

			_, stderr, err := runBuildsListQuerySurface(
				t,
				"builds", command,
				"--app", "123456789",
				"--beta-review-state", ",",
			)

			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			if !strings.Contains(stderr, "--beta-review-state must be a comma-separated list of") {
				t.Fatalf("expected beta review state validation error, got %q", stderr)
			}
			captured.assertNoRequest(t)
		})
	}
}

func TestBuildsListIncludeAlwaysCarriesPreReleaseVersion(t *testing.T) {
	tests := []struct {
		name        string
		include     []string
		wantInclude string
	}{
		{name: "default", wantInclude: "preReleaseVersion"},
		{name: "union", include: []string{"--include", "app,buildBetaDetail"}, wantInclude: "preReleaseVersion,app,buildBetaDetail"},
		{name: "deduplicated", include: []string{"--include", "preReleaseVersion,betaGroups,preReleaseVersion"}, wantInclude: "preReleaseVersion,betaGroups"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			captured := buildsListQuerySurfaceStub(t)

			args := append([]string{"builds", "list", "--app", "123456789"}, test.include...)
			_, stderr, err := runBuildsListQuerySurface(t, args...)
			if err != nil {
				t.Fatalf("run error: %v (stderr=%q)", err, stderr)
			}

			if got := captured.query.Get("include"); got != test.wantInclude {
				t.Fatalf("include = %q, want %q", got, test.wantInclude)
			}
		})
	}
}

func TestBuildsListIncludeRejectsUnknownValue(t *testing.T) {
	captured := buildsListQuerySurfaceStub(t)

	_, stderr, err := runBuildsListQuerySurface(
		t,
		"builds", "list",
		"--app", "123456789",
		"--include", "app,buildBundle",
	)

	if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
	}
	for _, want := range []string{
		"--include",
		"preReleaseVersion",
		"individualTesters",
		"betaGroups",
		"betaBuildLocalizations",
		"appEncryptionDeclaration",
		"betaAppReviewSubmission",
		"app",
		"buildBetaDetail",
		"appStoreVersion",
		"icons",
		"buildBundles",
		"buildUpload",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to mention %q, got %q", want, stderr)
		}
	}
	captured.assertNoRequest(t)
}

func TestBuildsListIncludeRejectsBlankCSV(t *testing.T) {
	captured := buildsListQuerySurfaceStub(t)

	_, stderr, err := runBuildsListQuerySurface(
		t,
		"builds", "list",
		"--app", "123456789",
		"--include", ",",
	)

	if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
	}
	if !strings.Contains(stderr, "--include must be a comma-separated list of") {
		t.Fatalf("expected include validation error, got %q", stderr)
	}
	captured.assertNoRequest(t)
}

func TestBuildsListRejectsQueryFlagsCombinedWithNext(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/builds?cursor=PAGE2&filter%5Bapp%5D=123456789&include=preReleaseVersion"

	for _, testCase := range []struct {
		name string
		flag string
		args []string
	}{
		{name: "app scope", flag: "--app", args: []string{"--app", "987654321"}},
		{name: "beta review state", flag: "--beta-review-state", args: []string{"--beta-review-state", "APPROVED"}},
		{name: "include", flag: "--include", args: []string{"--include", "betaGroups"}},
		{name: "sort", flag: "--sort", args: []string{"--sort", "version"}},
		{name: "platform", flag: "--platform", args: []string{"--platform", "IOS"}},
		{name: "processing state", flag: "--processing-state", args: []string{"--processing-state", "VALID"}},
		{name: "version", flag: "--version", args: []string{"--version", "1.2.3"}},
		{name: "build number", flag: "--build-number", args: []string{"--build-number", "77"}},
		{name: "limit", flag: "--limit", args: []string{"--limit", "10"}},
		{name: "limit explicit zero", flag: "--limit", args: []string{"--limit", "0"}},
		{name: "exclude expired", flag: "--exclude-expired", args: []string{"--exclude-expired"}},
		{name: "not expired alias", flag: "--not-expired", args: []string{"--not-expired"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			captured := buildsListQuerySurfaceStub(t)

			args := append([]string{"builds", "list", "--next", nextURL}, testCase.args...)
			stdout, stderr, err := runBuildsListQuerySurface(t, args...)

			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			wantError := "builds list: --next cannot be combined with " + testCase.flag
			if err == nil || !errors.Is(err, flag.ErrHelp) || err.Error() != wantError {
				t.Fatalf("error = %v, want usage error %q", err, wantError)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if firstLine := strings.SplitN(stderr, "\n", 2)[0]; firstLine != "Error: "+wantError {
				t.Fatalf("expected concise first stderr line %q, got %q", "Error: "+wantError, firstLine)
			}
			captured.assertNoRequest(t)
		})
	}
}

func TestBuildsListNextAloneStillPaginates(t *testing.T) {
	captured := buildsListQuerySurfaceStub(t)

	_, stderr, err := runBuildsListQuerySurface(
		t,
		"builds", "list",
		"--next", "https://api.appstoreconnect.apple.com/v1/builds?cursor=PAGE2&filter%5Bapp%5D=123456789&include=preReleaseVersion",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}

	if captured.path != "/v1/builds" {
		t.Fatalf("expected /v1/builds path, got %q", captured.path)
	}
	// The whole next URL must survive, not just the cursor: dropping filter[app]
	// or include here would silently widen the page or break the table renderer,
	// and adding anything would mean the URL was rebuilt rather than followed.
	wantQuery := map[string]string{
		"cursor":      "PAGE2",
		"filter[app]": "123456789",
		"include":     "preReleaseVersion",
	}
	for param, want := range wantQuery {
		if got := captured.query.Get(param); got != want {
			t.Fatalf("expected %s=%q on the followed next URL, got %q (full query %s)", param, want, got, captured.query.Encode())
		}
	}
	if len(captured.query) != len(wantQuery) {
		t.Fatalf("expected exactly the next URL's %d parameters, got %s", len(wantQuery), captured.query.Encode())
	}
}

func TestBuildsListSortAcceptsEveryDocumentedKey(t *testing.T) {
	for _, sortValue := range []string{
		"version",
		"-version",
		"uploadedDate",
		"-uploadedDate",
		"preReleaseVersion",
		"-preReleaseVersion",
	} {
		t.Run(sortValue, func(t *testing.T) {
			captured := buildsListQuerySurfaceStub(t)

			_, stderr, err := runBuildsListQuerySurface(
				t,
				"builds", "list",
				"--app", "123456789",
				"--sort", sortValue,
			)
			if err != nil {
				t.Fatalf("run error: %v (stderr=%q)", err, stderr)
			}

			if got := captured.query.Get("sort"); got != sortValue {
				t.Fatalf("expected sort=%q, got %q", sortValue, got)
			}
		})
	}
}

// builds count documents filter parity with builds list, so the beta review
// state filter has to reach the count request too.
func TestBuildsCountBetaReviewStateEmitsFilter(t *testing.T) {
	captured := buildsListQuerySurfaceStub(t)

	_, stderr, err := runBuildsListQuerySurface(
		t,
		"builds", "count",
		"--app", "123456789",
		"--beta-review-state", "rejected",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}

	if captured.path != "/v1/builds" {
		t.Fatalf("expected /v1/builds path, got %q", captured.path)
	}
	if got := captured.query.Get("filter[betaAppReviewSubmission.betaReviewState]"); got != "REJECTED" {
		t.Fatalf("expected normalized REJECTED filter, got %q", got)
	}
}

func TestBuildsCountBetaReviewStateRejectsUnknownValue(t *testing.T) {
	captured := buildsListQuerySurfaceStub(t)

	_, stderr, err := runBuildsListQuerySurface(
		t,
		"builds", "count",
		"--app", "123456789",
		"--beta-review-state", "PENDING",
	)

	if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
	}
	wantError := "--beta-review-state must be a comma-separated list of: WAITING_FOR_REVIEW, IN_REVIEW, REJECTED, APPROVED"
	if err == nil || !errors.Is(err, flag.ErrHelp) || err.Error() != wantError {
		t.Fatalf("error = %v, want usage error %q (stderr=%q)", err, wantError, stderr)
	}
	if !strings.Contains(stderr, wantError) {
		t.Fatalf("expected stderr to contain %q, got %q", wantError, stderr)
	}
	captured.assertNoRequest(t)
}

func TestBuildsListSortRejectsUnknownKey(t *testing.T) {
	captured := buildsListQuerySurfaceStub(t)

	_, stderr, err := runBuildsListQuerySurface(
		t,
		"builds", "list",
		"--app", "123456789",
		"--sort", "expirationDate",
	)

	wantError := "builds: --sort must be one of: version, -version, uploadedDate, -uploadedDate, preReleaseVersion, -preReleaseVersion"
	if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
	}
	if err == nil || !errors.Is(err, flag.ErrHelp) || err.Error() != wantError {
		t.Fatalf("error = %v, want usage error %q (stderr=%q)", err, wantError, stderr)
	}
	if !strings.Contains(stderr, wantError) {
		t.Fatalf("expected stderr to contain %q, got %q", wantError, stderr)
	}
	captured.assertNoRequest(t)
}
