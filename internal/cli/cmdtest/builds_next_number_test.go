package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBuildsNextBuildNumberUsesUploadsAndBuilds(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"builds","id":"build-1","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]
			}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"builds","id":"build-1","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]
			}`))
	reg.Register(asctest.GET("/v1/apps/100000001/buildUploads", "filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"101"}}],
				"links":{"next":""}
			}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "next-build-number", "--app", "100000001"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestProcessedBuildNumber *string  `json:"latestProcessedBuildNumber"`
		LatestUploadBuildNumber    *string  `json:"latestUploadBuildNumber"`
		LatestObservedBuildNumber  *string  `json:"latestObservedBuildNumber"`
		NextBuildNumber            string   `json:"nextBuildNumber"`
		SourcesConsidered          []string `json:"sourcesConsidered"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "100" {
		t.Fatalf("expected latestProcessedBuildNumber=100, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "101" {
		t.Fatalf("expected latestUploadBuildNumber=101, got %v", out.LatestUploadBuildNumber)
	}
	if out.LatestObservedBuildNumber == nil || *out.LatestObservedBuildNumber != "101" {
		t.Fatalf("expected latestObservedBuildNumber=101, got %v", out.LatestObservedBuildNumber)
	}
	if out.NextBuildNumber != "102" {
		t.Fatalf("expected nextBuildNumber=102, got %q", out.NextBuildNumber)
	}
	if len(out.SourcesConsidered) != 2 {
		t.Fatalf("expected two sources considered, got %v", out.SourcesConsidered)
	}
}

func TestBuildsNextBuildNumberRejectsInvalidInitialBuildNumber(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "next-build-number", "--app", "100000001", "--initial-build-number", "0"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --initial-build-number must be >= 1") {
		t.Fatalf("expected initial build number validation error, got %q", stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected validation before auth resolution, got %q", stderr)
	}
}

func TestBuildsNextBuildNumberExplainsUnavailableUploadHistory(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		paginate     bool
		wantCause    error
		wantExitCode int
	}{
		{
			name:         "initial request forbidden",
			status:       http.StatusForbidden,
			responseBody: `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`,
			wantCause:    asc.ErrForbidden,
			wantExitCode: cmd.ExitAuth,
		},
		{
			name:         "initial request not found",
			status:       http.StatusNotFound,
			responseBody: `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`,
			wantCause:    asc.ErrNotFound,
			wantExitCode: cmd.ExitNotFound,
		},
		{
			name:         "next page forbidden",
			status:       http.StatusForbidden,
			responseBody: `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`,
			paginate:     true,
			wantCause:    asc.ErrForbidden,
			wantExitCode: cmd.ExitAuth,
		},
		{
			name:         "next page not found",
			status:       http.StatusNotFound,
			responseBody: `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`,
			paginate:     true,
			wantCause:    asc.ErrNotFound,
			wantExitCode: cmd.ExitNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			// Build Upload API 404s are retried; exhaust the budget quickly so
			// the terminal exit code and guidance are asserted after retries.
			t.Setenv("ASC_MAX_RETRIES", "1")
			t.Setenv("ASC_BASE_DELAY", "1ms")
			t.Setenv("ASC_MAX_DELAY", "1ms")

			reg := asctest.NewRegistry(t)
			reg.Install()
			builds := asctest.GET("/v1/builds", "filter[app]=100000001", "limit=200", "sort=-uploadedDate")
			const buildsBody = `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`
			reg.Register(builds, asctest.JSON(http.StatusOK, buildsBody))
			reg.Register(builds, asctest.JSON(http.StatusOK, buildsBody))
			uploads := asctest.GET("/v1/apps/100000001/buildUploads", "filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE", "limit=200")
			if tt.paginate {
				reg.Register(uploads, asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/100000001/buildUploads?cursor=next-page"}}`))
				uploads = asctest.GET("/v1/apps/100000001/buildUploads", "cursor=next-page")
			}
			reg.Register(uploads, asctest.JSON(tt.status, tt.responseBody))
			if tt.status == http.StatusNotFound {
				reg.Register(asctest.GET("/v1/apps/100000001"), asctest.JSON(http.StatusNotFound, tt.responseBody))
			}

			stdout, stderr, runErr := runCommand(t, []string{"builds", "next-build-number", "--app", "100000001"})

			if runErr == nil {
				t.Fatal("expected unavailable upload history to fail")
			}
			if !errors.Is(runErr, tt.wantCause) {
				t.Fatalf("expected %v in error chain, got %v", tt.wantCause, runErr)
			}
			if got := cmd.ExitCodeFromError(runErr); got != tt.wantExitCode {
				t.Fatalf("exit code = %d, want %d", got, tt.wantExitCode)
			}
			for _, want := range []string{
				`build upload history is unavailable for app "100000001"`,
				"refusing to guess because an in-flight upload may already use the next number",
				`asc builds uploads list --app "100000001" --paginate`,
			} {
				if !strings.Contains(runErr.Error(), want) {
					t.Fatalf("expected error to contain %q, got %v", want, runErr)
				}
			}
			if stdout != "" || stderr != "" {
				t.Fatalf("expected no partial output, got stdout=%q stderr=%q", stdout, stderr)
			}
			wantAppChecks := 0
			if tt.status == http.StatusNotFound {
				wantAppChecks = 1
			}
			appChecks := 0
			for _, req := range reg.Requests() {
				if req.URL.Path == "/v1/apps/100000001" {
					appChecks++
				}
			}
			if appChecks != wantAppChecks {
				t.Fatalf("app existence checks = %d, want %d", appChecks, wantAppChecks)
			}
		})
	}
}

func TestBuildsNextBuildNumberWithFiltersUsesCanonicalQueryShape(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=100000001", "filter[platform]=IOS", "filter[version]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[expired]=false", "filter[preReleaseVersion]=prv-1", "filter[processingState]=VALID", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-new-50","attributes":{"version":"50","uploadedDate":"2026-02-02T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[expired]=false", "filter[preReleaseVersion]=prv-1", "filter[processingState]=VALID", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[
					{"type":"builds","id":"build-new-50","attributes":{"version":"50","uploadedDate":"2026-02-02T00:00:00Z"}},
					{"type":"builds","id":"build-old-100","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}
				]}`))
	reg.Register(asctest.GET("/v1/apps/100000001/buildUploads", "filter[cfBundleShortVersionString]=1.2.3", "filter[platform]=IOS", "filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"90"}}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"builds", "next-build-number",
		"--app", "100000001",
		"--version", "1.2.3",
		"--platform", "ios",
		"--processing-state", "valid",
		"--exclude-expired",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestProcessedBuildNumber *string `json:"latestProcessedBuildNumber"`
		LatestUploadBuildNumber    *string `json:"latestUploadBuildNumber"`
		LatestObservedBuildNumber  *string `json:"latestObservedBuildNumber"`
		NextBuildNumber            string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "50" {
		t.Fatalf("expected latestProcessedBuildNumber=50, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "90" {
		t.Fatalf("expected latestUploadBuildNumber=90, got %v", out.LatestUploadBuildNumber)
	}
	if out.LatestObservedBuildNumber == nil || *out.LatestObservedBuildNumber != "100" {
		t.Fatalf("expected latestObservedBuildNumber=100, got %v", out.LatestObservedBuildNumber)
	}
	if out.NextBuildNumber != "101" {
		t.Fatalf("expected nextBuildNumber=101, got %q", out.NextBuildNumber)
	}
	chronologicalBuildRequests, maximumBuildRequests := 0, 0
	for _, req := range reg.Requests() {
		if req.URL.Path != "/v1/builds" {
			continue
		}
		switch req.URL.Query().Get("limit") {
		case "1":
			chronologicalBuildRequests++
		case "200":
			maximumBuildRequests++
		}
	}
	if chronologicalBuildRequests != 1 || maximumBuildRequests != 1 {
		t.Fatalf("build requests = chronological:%d maximum:%d, want 1 each", chronologicalBuildRequests, maximumBuildRequests)
	}
}

func TestBuildsNextBuildNumberScansEveryEquivalentVersionUpload(t *testing.T) {
	setupAuth(t)
	shared.ResetEquivalentVersionNotesForTest()
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=100000001", "filter[platform]=IOS", "filter[version]=76.54.0", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=100000001", "filter[platform]=IOS", "filter[version]=76.54", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-equivalent","attributes":{"version":"76.54","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[preReleaseVersion]=prv-equivalent", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-10","attributes":{"version":"10","uploadedDate":"2026-02-01T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[preReleaseVersion]=prv-equivalent", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-10","attributes":{"version":"10","uploadedDate":"2026-02-01T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/apps/100000001/buildUploads", "filter[cfBundleShortVersionString]=76.54.0,76.54", "filter[platform]=IOS", "filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-20","attributes":{"cfBundleShortVersionString":"76.54","cfBundleVersion":"20"}},{"type":"buildUploads","id":"upload-30","attributes":{"cfBundleShortVersionString":"76.54.0","cfBundleVersion":"30"}}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"builds", "next-build-number",
		"--app", "100000001",
		"--version", "76.54.0",
		"--platform", "ios",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	preReleaseRequests := 0
	for _, req := range reg.Requests() {
		if req.URL.Path == "/v1/preReleaseVersions" {
			preReleaseRequests++
		}
	}
	if preReleaseRequests != 2 {
		t.Fatalf("pre-release requests = %d, want 2", preReleaseRequests)
	}
	var out struct {
		LatestUploadBuildNumber *string `json:"latestUploadBuildNumber"`
		NextBuildNumber         string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "30" {
		t.Fatalf("latestUploadBuildNumber = %v, want 30", out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "31" {
		t.Fatalf("nextBuildNumber = %q, want 31", out.NextBuildNumber)
	}
	if !strings.Contains(stderr, `matched version "76.54" for requested "76.54.0"`) {
		t.Fatalf("expected equivalent-version note, got %q", stderr)
	}
}

func TestBuildsNextBuildNumberSkipsNonPositiveBuildUploadNumber(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=100000001", "filter[version]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[expired]=false", "filter[preReleaseVersion]=prv-1", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[expired]=false", "filter[preReleaseVersion]=prv-1", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/100000001/buildUploads", "filter[cfBundleShortVersionString]=1.2.3", "filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"buildUploads","id":"expired-upload","attributes":{"cfBundleVersion":"0"}},
					{"type":"buildUploads","id":"spaced-zero-upload","attributes":{"cfBundleVersion":"0 .1"}}
				],
				"links":{"next":""}
			}`))

	stdout, stderr, err := runCommand(t, []string{
		"builds", "next-build-number",
		"--app", "100000001",
		"--version", "1.2.3",
		"--exclude-expired",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestUploadBuildNumber *string  `json:"latestUploadBuildNumber"`
		NextBuildNumber         string   `json:"nextBuildNumber"`
		SourcesConsidered       []string `json:"sourcesConsidered"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestUploadBuildNumber != nil {
		t.Fatalf("expected invalid upload number to be ignored, got %v", *out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "1" {
		t.Fatalf("expected nextBuildNumber=1, got %q", out.NextBuildNumber)
	}
	if len(out.SourcesConsidered) != 0 {
		t.Fatalf("expected no sources considered, got %v", out.SourcesConsidered)
	}
}

func TestBuildsNextBuildNumberVersionFilterCollectsEquivalentPlatforms(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=100000001", "filter[version]=1.1,1.1.0", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"MAC_OS"}},{"type":"preReleaseVersions","id":"prv-near","attributes":{"version":"1.1.0","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[preReleaseVersion]=prv-exact", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-exact","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[preReleaseVersion]=prv-near", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[preReleaseVersion]=prv-exact,prv-near", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-exact","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/apps/100000001/buildUploads", "filter[cfBundleShortVersionString]=1.1,1.1.0", "filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"101"}}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "next-build-number", "--app", "100000001", "--version", "1.1"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestProcessedBuildNumber *string `json:"latestProcessedBuildNumber"`
		LatestUploadBuildNumber    *string `json:"latestUploadBuildNumber"`
		NextBuildNumber            string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "100" {
		t.Fatalf("expected latestProcessedBuildNumber=100, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "101" {
		t.Fatalf("expected latestUploadBuildNumber=101, got %v", out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "102" {
		t.Fatalf("expected nextBuildNumber=102, got %q", out.NextBuildNumber)
	}
	var buildFilters []string
	for _, req := range reg.Requests() {
		if req.URL.Path == "/v1/builds" {
			buildFilters = append(buildFilters, req.URL.Query().Get("filter[preReleaseVersion]"))
		}
	}
	if len(buildFilters) != 3 || buildFilters[0] != "prv-exact" || buildFilters[1] != "prv-near" || buildFilters[2] != "prv-exact,prv-near" {
		t.Fatalf("expected both equivalent platform trains in latest and history lookups, got %v", buildFilters)
	}
}

func TestBuildsNextBuildNumberVersionFilterKeepsServerMatchedPreReleaseVersionsWithoutAttributes(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=100000001", "filter[version]=1.1,1.1.0", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-server","attributes":{}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[preReleaseVersion]=prv-server", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-server","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[preReleaseVersion]=prv-server", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-server","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/apps/100000001/buildUploads", "filter[cfBundleShortVersionString]=1.1,1.1.0", "filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"101"}}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "next-build-number", "--app", "100000001", "--version", "1.1"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestProcessedBuildNumber *string `json:"latestProcessedBuildNumber"`
		LatestUploadBuildNumber    *string `json:"latestUploadBuildNumber"`
		NextBuildNumber            string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "100" {
		t.Fatalf("expected latestProcessedBuildNumber=100, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "101" {
		t.Fatalf("expected latestUploadBuildNumber=101, got %v", out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "102" {
		t.Fatalf("expected nextBuildNumber=102, got %q", out.NextBuildNumber)
	}
}

func TestBuildsNextBuildNumberVersionAndPlatformPaginatesPastNearMatches(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextURL = "https://api.appstoreconnect.apple.com/v1/preReleaseVersions?cursor=page-2"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=100000001", "filter[platform]=IOS", "filter[version]=1.1", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-near","attributes":{"version":"1.1.0","platform":"IOS"}}],"links":{"next":"`+nextURL+`"}}`))
	reg.Register(asctest.GET("/v1/preReleaseVersions", "cursor=page-2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"IOS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[preReleaseVersion]=prv-exact", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-exact","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "filter[preReleaseVersion]=prv-exact", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-exact","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/apps/100000001/buildUploads", "filter[cfBundleShortVersionString]=1.1,1.1.0", "filter[platform]=IOS", "filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"101"}}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "next-build-number", "--app", "100000001", "--version", "1.1", "--platform", "IOS"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestProcessedBuildNumber *string `json:"latestProcessedBuildNumber"`
		LatestUploadBuildNumber    *string `json:"latestUploadBuildNumber"`
		NextBuildNumber            string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "100" {
		t.Fatalf("expected latestProcessedBuildNumber=100, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "101" {
		t.Fatalf("expected latestUploadBuildNumber=101, got %v", out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "102" {
		t.Fatalf("expected nextBuildNumber=102, got %q", out.NextBuildNumber)
	}
}

func TestBuildsHelpShowsNextBuildNumberAndHidesLatestAlias(t *testing.T) {
	usage := usageForCommand(t, "builds")
	if !strings.Contains(usage, "\n  next-build-number") {
		t.Fatalf("expected builds help to list next-build-number, got %q", usage)
	}
	if strings.Contains(usage, "\n  latest\t") || strings.Contains(usage, "\n  latest ") {
		t.Fatalf("expected deprecated latest alias to stay hidden from builds help, got %q", usage)
	}
}

func TestBuildsNextBuildNumberRetriesBuildUploadsNotFoundForExistingApp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_MAX_RETRIES", "3")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=100000001", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/apps/100000001/buildUploads", "filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE", "limit=200"),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id '100000001'"}]}`))
	reg.Register(asctest.GET("/v1/apps/100000001"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"100000001"}}`))
	reg.Register(asctest.GET("/v1/apps/100000001/buildUploads", "filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE", "limit=200"),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id '100000001'"}]}`))
	reg.Register(asctest.GET("/v1/apps/100000001/buildUploads", "filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"101"}}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "next-build-number", "--app", "100000001"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	uploadAttempts, appChecks := 0, 0
	for _, req := range reg.Requests() {
		switch req.URL.Path {
		case "/v1/apps/100000001/buildUploads":
			uploadAttempts++
		case "/v1/apps/100000001":
			appChecks++
		}
	}
	if uploadAttempts != 3 {
		t.Fatalf("expected 3 buildUploads attempts (404, 404, 200), got %d", uploadAttempts)
	}
	if appChecks != 1 {
		t.Fatalf("expected one app existence check, got %d", appChecks)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var out struct {
		LatestUploadBuildNumber *string `json:"latestUploadBuildNumber"`
		NextBuildNumber         string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "101" {
		t.Fatalf("expected latestUploadBuildNumber=101, got %v", out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "102" {
		t.Fatalf("expected nextBuildNumber=102, got %q", out.NextBuildNumber)
	}
}
