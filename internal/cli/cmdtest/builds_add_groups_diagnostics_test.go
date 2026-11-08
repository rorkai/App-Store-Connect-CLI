package cmdtest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type addGroupsDiagnosticsFixture struct {
	t *testing.T

	external              bool
	groupInput            string
	groupsBody            string
	postStatus            int
	postBody              string
	buildStatus           int
	buildBody             string
	preflightBuildStatus  int
	preflightBuildBody    string
	detailStatus          int
	detailBody            string
	preflightDetailStatus int
	preflightDetailBody   string

	postCount   int
	buildReads  int
	detailReads int
	requests    []string
}

func (f *addGroupsDiagnosticsFixture) transport() roundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		request := req.Method + " " + req.URL.Path
		f.requests = append(f.requests, request)
		switch request {
		case "GET /v1/builds/build-1/app":
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`)
		case "GET /v1/apps/app-1/betaGroups":
			if req.URL.Query().Get("cursor") == "UNNEEDED" {
				return nil, io.ErrUnexpectedEOF
			}
			if f.groupsBody != "" {
				return jsonResponse(http.StatusOK, f.groupsBody)
			}
			isInternal := !f.external
			return jsonResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"QA","isInternalGroup":`+boolString(isInternal)+`}}]}`)
		case "POST /v1/builds/build-1/relationships/betaGroups":
			f.postCount++
			status := f.postStatus
			if status == 0 {
				status = http.StatusNoContent
			}
			return jsonResponse(status, f.postBody)
		case "GET /v1/builds/build-1":
			f.buildReads++
			status := f.buildStatus
			body := f.buildBody
			if f.preflightBuildBody != "" && f.buildReads == 1 {
				status = f.preflightBuildStatus
				body = f.preflightBuildBody
			}
			if status == 0 {
				status = http.StatusOK
			}
			if body == "" {
				body = `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"VALID","expired":false,"buildAudienceType":"APP_STORE_ELIGIBLE","usesNonExemptEncryption":false}}}`
			}
			return jsonResponse(status, body)
		case "GET /v1/builds/build-1/buildBetaDetail":
			f.detailReads++
			status := f.detailStatus
			body := f.detailBody
			if f.preflightDetailBody != "" && f.detailReads == 1 {
				status = f.preflightDetailStatus
				body = f.preflightDetailBody
			}
			if status == 0 {
				status = http.StatusOK
			}
			if body == "" {
				body = `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"externalBuildState":"READY_FOR_BETA_TESTING"}}}`
			}
			return jsonResponse(status, body)
		default:
			f.t.Fatalf("unexpected request %s", request)
			return nil, nil
		}
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func runAddGroupsDiagnostics(t *testing.T, fixture *addGroupsDiagnosticsFixture) (string, string, error) {
	return runAddGroupsDiagnosticsArgs(t, fixture)
}

func runAddGroupsDiagnosticsArgs(t *testing.T, fixture *addGroupsDiagnosticsFixture, extraArgs ...string) (string, string, error) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = fixture.transport()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		groupInput := fixture.groupInput
		if groupInput == "" {
			groupInput = "group-1"
		}
		args := []string{
			"builds", "add-groups",
			"--build-id", "build-1",
			"--group", groupInput,
		}
		args = append(args, extraArgs...)
		args = append(args, "--output", "json")
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func TestBuildsAddGroupsExactIDDoesNotFetchUnneededGroupPage(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")
	fixture := &addGroupsDiagnosticsFixture{
		t:          t,
		groupsBody: `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"QA","isInternalGroup":true}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/betaGroups?cursor=UNNEEDED"}}`,
	}
	stdout, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr != nil {
		t.Fatalf("exact group ID failed: %v (stderr=%q)", runErr, stderr)
	}
	wantRequests := []string{
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
		"GET /v1/builds/build-1",
		"POST /v1/builds/build-1/relationships/betaGroups",
	}
	if !reflect.DeepEqual(fixture.requests, wantRequests) || !strings.Contains(stdout, `"groupIds":["group-1"]`) {
		t.Fatalf("requests = %v, stdout = %q", fixture.requests, stdout)
	}
}

func TestBuildsAddGroupsDryRunDoesNotPostAndReportsAdvisoryState(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:          t,
		external:   true,
		buildBody:  `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"PROCESSING","expired":false}}}`,
		detailBody: `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"externalBuildState":"MISSING_EXPORT_COMPLIANCE"}}}`,
	}
	stdout, stderr, runErr := runAddGroupsDiagnosticsArgs(t, fixture, "--dry-run")
	if runErr != nil {
		t.Fatalf("unexpected error: %v (stderr=%q)", runErr, stderr)
	}
	wantRequests := []string{
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
		"GET /v1/builds/build-1",
		"GET /v1/builds/build-1/buildBetaDetail",
	}
	if !reflect.DeepEqual(fixture.requests, wantRequests) {
		t.Fatalf("requests = %v, want %v", fixture.requests, wantRequests)
	}
	if fixture.postCount != 0 {
		t.Fatalf("postCount = %d, want zero", fixture.postCount)
	}
	for _, want := range []string{`"groupIds":["group-1"]`, `"action":"would-add"`, `"dryRun":true`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	}
	for _, want := range []string{
		"Dry run: no beta groups were added to build build-1",
		"Current build state: processingState=PROCESSING",
		"externalBuildState=MISSING_EXPORT_COMPLIANCE",
		"readiness is advisory",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	if strings.Contains(stderr, "Successfully added") {
		t.Fatalf("dry-run stderr claims mutation: %q", stderr)
	}
}

func TestBuildsAddGroupsDryRunSkipsInternalAndPlansExternal(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:          t,
		groupInput: "group-internal,group-external",
		groupsBody: `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true}},{"type":"betaGroups","id":"group-external","attributes":{"name":"External","isInternalGroup":false}}]}`,
		buildBody:  `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"VALID","expired":false}}}`,
	}
	stdout, stderr, runErr := runAddGroupsDiagnosticsArgs(t, fixture, "--skip-internal", "--dry-run")
	if runErr != nil {
		t.Fatalf("unexpected error: %v (stderr=%q)", runErr, stderr)
	}
	wantRequests := []string{
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
		"GET /v1/builds/build-1",
		"GET /v1/builds/build-1/buildBetaDetail",
	}
	if !reflect.DeepEqual(fixture.requests, wantRequests) {
		t.Fatalf("requests = %v, want %v", fixture.requests, wantRequests)
	}
	for _, want := range []string{`"groupIds":["group-external"]`, `"action":"would-add"`, `"dryRun":true`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	}
	if !strings.Contains(stderr, `Skipped internal group "Internal"`) || !strings.Contains(stderr, "Dry run: no beta groups were added") {
		t.Fatalf("stderr = %q, want skip and no-mutation messages", stderr)
	}
}

func TestBuildsAddGroupsDryRunAllInternalIsNoOp(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:          t,
		groupsBody: `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"Internal","isInternalGroup":true}}]}`,
	}
	stdout, stderr, runErr := runAddGroupsDiagnosticsArgs(t, fixture, "--skip-internal", "--dry-run")
	if runErr != nil {
		t.Fatalf("unexpected error: %v (stderr=%q)", runErr, stderr)
	}
	wantRequests := []string{
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
	}
	if !reflect.DeepEqual(fixture.requests, wantRequests) {
		t.Fatalf("requests = %v, want %v", fixture.requests, wantRequests)
	}
	for _, want := range []string{`"groupIds":[]`, `"action":"no-op"`, `"dryRun":true`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	}
	if !strings.Contains(stderr, "No groups to add for build build-1 after applying filters") || strings.Contains(stderr, "GET /v1/builds/build-1") {
		t.Fatalf("stderr = %q, want no-op message", stderr)
	}
}

func TestBuildsAddGroupsSkippedInternalDiagnosticsSanitizeProviderID(t *testing.T) {
	const groupID = "group-control-\x1b[31mID\nNEXT"

	for _, testCase := range []struct {
		name   string
		dryRun bool
	}{
		{name: "normal", dryRun: false},
		{name: "dry-run", dryRun: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := &addGroupsDiagnosticsFixture{
				t:          t,
				groupInput: groupID,
				groupsBody: `{"data":[{"type":"betaGroups","id":"group-control-\u001b[31mID\nNEXT","attributes":{"name":"QA-\u001b[31mNAME\nINJECT","isInternalGroup":true}}]}`,
			}
			args := []string{"--skip-internal"}
			if testCase.dryRun {
				args = append(args, "--dry-run")
			}

			stdout, stderr, runErr := runAddGroupsDiagnosticsArgs(t, fixture, args...)
			if runErr != nil {
				t.Fatalf("unexpected error: %v (stdout=%q stderr=%q)", runErr, stdout, stderr)
			}

			wantSkippedLine := `Skipped internal group "QA-\x1b[31mNAME\nINJECT" (group-control-[31mID NEXT) because --skip-internal was set` + "\n"
			if !strings.Contains(stderr, wantSkippedLine) {
				t.Fatalf("stderr = %q, want sanitized skipped-group diagnostic %q", stderr, wantSkippedLine)
			}
			if strings.Contains(stderr, "\x1b") {
				t.Fatalf("stderr contains raw ESC: %q", stderr)
			}
			if strings.Contains(stderr, groupID) {
				t.Fatalf("stderr contains unsanitized provider ID: %q", stderr)
			}

			if testCase.dryRun {
				if fixture.postCount != 0 {
					t.Fatalf("postCount = %d, want zero for dry-run", fixture.postCount)
				}
				if !strings.Contains(stdout, `"action":"no-op"`) || !strings.Contains(stdout, `"dryRun":true`) {
					t.Fatalf("stdout = %q, want dry-run no-op receipt", stdout)
				}
			}
		})
	}
}

func TestBuildsAddGroupsDryRunRejectsSubmitBeforeNetwork(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{t: t, external: true}
	_, stderr, runErr := runAddGroupsDiagnosticsArgs(t, fixture, "--submit", "--confirm", "--dry-run")
	if runErr == nil {
		t.Fatal("expected --submit/--dry-run conflict")
	}
	if rootcmd.ExitCodeFromError(runErr) != 2 {
		t.Fatalf("exit code = %d, want usage exit 2", rootcmd.ExitCodeFromError(runErr))
	}
	if len(fixture.requests) != 0 {
		t.Fatalf("requests = %v, want no network requests", fixture.requests)
	}
	if !strings.Contains(stderr, "--submit cannot be used with --dry-run") {
		t.Fatalf("stderr = %q, want conflict message", stderr)
	}
}

func TestBuildsAddGroupsDryRunKeepsPreviewSuccessfulWhenStateReadFails(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:           t,
		external:    false,
		buildStatus: http.StatusInternalServerError,
		buildBody:   `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","detail":"Unavailable."}]}`,
	}
	stdout, stderr, runErr := runAddGroupsDiagnosticsArgs(t, fixture, "--dry-run")
	if runErr != nil {
		t.Fatalf("unexpected error: %v (stderr=%q)", runErr, stderr)
	}
	if fixture.postCount != 0 || fixture.buildReads != 1 || fixture.detailReads != 0 {
		t.Fatalf("requests = %v, want no POST and one best-effort build read", fixture.requests)
	}
	if !strings.Contains(stdout, `"dryRun":true`) || !strings.Contains(stderr, "readiness is unknown") {
		t.Fatalf("stdout=%q stderr=%q, want dry-run receipt and unknown readiness", stdout, stderr)
	}
}

func TestBuildsAddGroupsSuccessPreflightsStateBeforePost(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{t: t, external: true}
	stdout, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr != nil {
		t.Fatalf("unexpected error: %v (stderr=%q)", runErr, stderr)
	}
	if fixture.postCount != 1 || fixture.buildReads != 1 || fixture.detailReads != 1 {
		t.Fatalf("requests = %v, want one preflight build/detail read and one POST", fixture.requests)
	}
	if !strings.Contains(stdout, `"groupIds":["group-1"]`) {
		t.Fatalf("stdout = %q, want assignment receipt", stdout)
	}
}

func TestBuildsAddGroupsSubmitBlocksInFlightReviewBeforeAnyPost(t *testing.T) {
	for _, state := range []string{"WAITING_FOR_BETA_REVIEW", "IN_BETA_REVIEW"} {
		t.Run(state, func(t *testing.T) {
			fixture := &addGroupsDiagnosticsFixture{
				t:          t,
				external:   true,
				detailBody: `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"externalBuildState":"` + state + `"}}}`,
			}

			_, stderr, runErr := runAddGroupsDiagnosticsArgs(t, fixture, "--submit", "--confirm")
			if runErr == nil {
				t.Fatal("expected in-flight review state to block assignment")
			}
			if fixture.postCount != 0 {
				t.Fatalf("postCount = %d, want zero relationship or submission POSTs", fixture.postCount)
			}
			wantRequests := []string{
				"GET /v1/builds/build-1/app",
				"GET /v1/apps/app-1/betaGroups",
				"GET /v1/builds/build-1",
				"GET /v1/builds/build-1/buildBetaDetail",
			}
			if !reflect.DeepEqual(fixture.requests, wantRequests) {
				t.Fatalf("requests = %v, want read-only preflight requests %v", fixture.requests, wantRequests)
			}
			if !strings.Contains(stderr, state) || !strings.Contains(stderr, "awaiting beta app review") {
				t.Fatalf("stderr = %q, want state-specific preflight diagnostic", stderr)
			}
		})
	}
}

func TestBuildsAddGroupsDiagnoses422AfterAssignment(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:                   t,
		external:            true,
		postStatus:          http.StatusUnprocessableEntity,
		postBody:            `{"errors":[{"status":"422","code":"STATE_ERROR.ENTITY_STATE_INVALID","detail":"The build is not ready.","meta":{"associatedErrors":{"betaGroups":[{"code":"BETA_GROUP_INVALID","detail":"The selected beta group is not eligible."}]}}}]}`,
		buildBody:           `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"FAILED","expired":false}}}`,
		detailBody:          `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"externalBuildState":"MISSING_EXPORT_COMPLIANCE"}}}`,
		preflightBuildBody:  `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"VALID","expired":false,"buildAudienceType":"APP_STORE_ELIGIBLE","usesNonExemptEncryption":false}}}`,
		preflightDetailBody: `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"externalBuildState":"READY_FOR_BETA_TESTING"}}}`,
	}
	_, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected HTTP 422 failure")
	}
	wantRequests := []string{
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
		"GET /v1/builds/build-1",
		"GET /v1/builds/build-1/buildBetaDetail",
		"POST /v1/builds/build-1/relationships/betaGroups",
		"GET /v1/builds/build-1",
		"GET /v1/builds/build-1/buildBetaDetail",
	}
	if !reflect.DeepEqual(fixture.requests, wantRequests) {
		t.Fatalf("requests = %v, want %v", fixture.requests, wantRequests)
	}
	if rootcmd.ExitCodeFromError(runErr) != rootcmd.HTTPStatusToExitCode(http.StatusUnprocessableEntity) {
		t.Fatalf("exit code = %d, want HTTP 422 mapping", rootcmd.ExitCodeFromError(runErr))
	}
	var apiErr *asc.APIError
	if !errors.As(runErr, &apiErr) || apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("error chain lost original API error: %v", runErr)
	}
	for _, want := range []string{
		"The build is not ready.",
		"The selected beta group is not eligible.",
		"Current build state: processingState=FAILED",
		"externalBuildState=MISSING_EXPORT_COMPLIANCE",
		`--ipa "PATH_TO_IPA"`,
		`--pkg "PATH_TO_PKG"`,
		"If the app does not use non-exempt encryption",
		`--uses-non-exempt-encryption=false`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	if strings.Contains(stderr, "--file") {
		t.Fatalf("stderr contains unsupported --file guidance: %q", stderr)
	}
	if codeAt, associatedAt := strings.Index(stderr, "(STATE_ERROR.ENTITY_STATE_INVALID)"), strings.Index(stderr, "Associated errors"); codeAt < 0 || associatedAt < 0 || codeAt > associatedAt {
		t.Fatalf("top-level code must precede associated errors: %q", stderr)
	}
}

func TestBuildsAddGroupsNon422DoesNotReadPostFailureDiagnostics(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:          t,
		external:   true,
		postStatus: http.StatusConflict,
		postBody:   `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","detail":"Conflict."}]}`,
	}
	_, _, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected HTTP 409 failure")
	}
	if fixture.postCount != 1 || fixture.buildReads != 1 || fixture.detailReads != 1 {
		t.Fatalf("requests = %v, want one preflight build/detail read and one POST", fixture.requests)
	}
	var apiErr *asc.APIError
	if !errors.As(runErr, &apiErr) || apiErr.StatusCode != http.StatusConflict {
		t.Fatalf("error chain lost original API error: %v", runErr)
	}
}

func TestBuildsAddGroupsDiagnosticReadFailurePreserves422(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:                  t,
		external:           false,
		postStatus:         http.StatusUnprocessableEntity,
		postBody:           `{"errors":[{"status":"422","code":"STATE_ERROR.ENTITY_STATE_INVALID","detail":"The build is not ready."}]}`,
		buildStatus:        http.StatusInternalServerError,
		buildBody:          `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","detail":"Unavailable."}]}`,
		preflightBuildBody: `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"VALID","expired":false}}}`,
	}
	_, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected HTTP 422 failure")
	}
	if fixture.postCount != 1 || fixture.buildReads != 2 || fixture.detailReads != 0 {
		t.Fatalf("requests = %v, want one preflight build read, one POST, and one diagnostic build read", fixture.requests)
	}
	if rootcmd.ExitCodeFromError(runErr) != rootcmd.HTTPStatusToExitCode(http.StatusUnprocessableEntity) {
		t.Fatalf("exit code = %d, want original HTTP 422 mapping (stderr=%q)", rootcmd.ExitCodeFromError(runErr), stderr)
	}
	var apiErr *asc.APIError
	if !errors.As(runErr, &apiErr) || apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("error chain lost original API error: %v", runErr)
	}
}

func TestBuildsAddGroupsPreflightBlocksProcessingBeforePost(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:          t,
		external:   true,
		buildBody:  `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"PROCESSING","expired":false,"buildAudienceType":"APP_STORE_ELIGIBLE","usesNonExemptEncryption":false}}}`,
		detailBody: `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"externalBuildState":"PROCESSING"}}}`,
	}
	_, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected processing preflight failure")
	}
	if fixture.postCount != 0 {
		t.Fatalf("postCount = %d, want zero", fixture.postCount)
	}
	if rootcmd.ExitCodeFromError(runErr) != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want validation exit", rootcmd.ExitCodeFromError(runErr))
	}
	if !strings.Contains(stderr, "processingState PROCESSING") || !strings.Contains(stderr, "asc builds wait") {
		t.Fatalf("stderr = %q, want processing state and remediation", stderr)
	}
}

func TestBuildsAddGroupsPreflightBlocksExpiredInternalBuildWithoutDetailRead(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:         t,
		buildBody: `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"VALID","expired":true}}}`,
	}
	_, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected expired preflight failure")
	}
	if fixture.postCount != 0 || fixture.detailReads != 0 {
		t.Fatalf("postCount=%d detailReads=%d, want zero post and detail reads", fixture.postCount, fixture.detailReads)
	}
	if !strings.Contains(stderr, "has expired") {
		t.Fatalf("stderr = %q, want expiration diagnostic", stderr)
	}
}

func TestBuildsAddGroupsPreflightBlocksExternalReviewNotReady(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:          t,
		external:   true,
		buildBody:  `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"VALID","expired":false,"buildAudienceType":"APP_STORE_ELIGIBLE","usesNonExemptEncryption":false}}}`,
		detailBody: `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"externalBuildState":"WAITING_FOR_BETA_REVIEW"}}}`,
	}
	_, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected beta review preflight failure")
	}
	if fixture.postCount != 0 {
		t.Fatalf("postCount = %d, want zero", fixture.postCount)
	}
	if !strings.Contains(stderr, "WAITING_FOR_BETA_REVIEW") || !strings.Contains(stderr, "asc builds info") {
		t.Fatalf("stderr = %q, want review state and remediation", stderr)
	}
}

func TestBuildsAddGroupsPreflightBlocksMissingEncryptionEvidence(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:          t,
		external:   true,
		buildBody:  `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"VALID","expired":false,"buildAudienceType":"APP_STORE_ELIGIBLE"}}}`,
		detailBody: `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"externalBuildState":"READY_FOR_BETA_TESTING"}}}`,
	}
	_, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected missing encryption preflight failure")
	}
	if fixture.postCount != 0 {
		t.Fatalf("postCount = %d, want zero", fixture.postCount)
	}
	if !strings.Contains(stderr, "usesNonExemptEncryption") || !strings.Contains(stderr, "asc builds update") {
		t.Fatalf("stderr = %q, want encryption evidence and remediation", stderr)
	}
}

func TestBuildsAddGroupsPreflightFailsClosedWhenBuildStateUnavailable(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:           t,
		external:    false,
		buildStatus: http.StatusServiceUnavailable,
		buildBody:   `{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","detail":"temporarily unavailable"}]}`,
	}
	_, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected unavailable preflight failure")
	}
	if fixture.postCount != 0 {
		t.Fatalf("postCount = %d, want zero", fixture.postCount)
	}
	if rootcmd.ExitCodeFromError(runErr) != rootcmd.HTTPStatusToExitCode(http.StatusServiceUnavailable) {
		t.Fatalf("exit code = %d, want provider HTTP 503 exit", rootcmd.ExitCodeFromError(runErr))
	}
	if !strings.Contains(stderr, "cannot verify build state for build build-1") {
		t.Fatalf("stderr = %q, want fail-closed diagnostic", stderr)
	}
	if diagnostic, ok := shared.DiagnosticFromError(runErr); !ok || diagnostic.Code != shared.DiagnosticRequestFailed {
		t.Fatalf("diagnostic = %+v (ok=%v), want request_failed", diagnostic, ok)
	}
}

func TestBuildsAddGroupsPreflightFailsClosedWhenExternalStateUnavailable(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:            t,
		external:     true,
		buildBody:    `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"VALID","expired":false,"buildAudienceType":"APP_STORE_ELIGIBLE","usesNonExemptEncryption":false}}}`,
		detailStatus: http.StatusServiceUnavailable,
		detailBody:   `{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","detail":"temporarily unavailable"}]}`,
	}
	_, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected unavailable external-state preflight failure")
	}
	if fixture.postCount != 0 {
		t.Fatalf("postCount = %d, want zero", fixture.postCount)
	}
	if !strings.Contains(stderr, "cannot verify external beta state for build build-1") {
		t.Fatalf("stderr = %q, want fail-closed external-state diagnostic", stderr)
	}
}
