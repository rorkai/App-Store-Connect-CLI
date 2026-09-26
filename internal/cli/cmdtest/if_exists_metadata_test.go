package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Apple's 409 body for a duplicate locale on POST /v1/appStoreVersionLocalizations,
// recorded verbatim against disposable app 6759231657 on 2026-09-25. metadata
// push only reaches it when the locale appeared after the plan read, or when a
// previous attempt created it and the CLI never saw the response.
const metadataVersionLocaleDuplicate409 = `{"errors":[{"id":"ca643fcd-402d-4120-9e8b-db867fc15a83","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","title":"The provided entity includes an attribute with a value that has already been used","detail":"Entity with locale: ja already exists. Try updating.","source":{"pointer":"/data/attributes/locale"}}]}`

// Apple's 409 body for a duplicate locale on POST /v1/appInfoLocalizations,
// recorded verbatim against disposable app 6759231657 on 2026-09-25. The code
// and pointer match the version scope; the detail text differs.
const metadataAppInfoLocaleDuplicate409 = `{"errors":[{"id":"922f3559-8b92-46da-a2cd-96001fbaab5d","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","title":"The provided entity includes an attribute with a value that has already been used","detail":"An 'appInfoLocalizations' with a 'locale' of 'ja' already exists.","source":{"pointer":"/data/attributes/locale"}}]}`

// A 409 that is not an existence conflict and must keep failing without a
// read-back. Representative rather than recorded: the disposable app's only
// version is editable, so a state conflict cannot be triggered there.
const metadataLocalizationState409 = `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"The version is not editable in its current state."}]}`

const metadataPushVersionsList = `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":""}}`

const metadataPushAppInfosList = `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":""}}`

const metadataPushEmptyList = `{"data":[],"links":{"next":""}}`

// metadataFixtureLocale is the locale every fixture in this file uses; it is
// the locale named in the replayed Apple 409 bodies.
const metadataFixtureLocale = "ja"

// writeMetadataVersionFixture writes the version-scope localization file for
// metadataFixtureLocale and returns the metadata root directory.
func writeMetadataVersionFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	localeDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localeDir, metadataFixtureLocale+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write version localization file: %v", err)
	}
	return dir
}

// writeMetadataAppInfoFixture writes the app-info-scope localization file for
// metadataFixtureLocale and returns the metadata root directory.
func writeMetadataAppInfoFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	appInfoDir := filepath.Join(dir, "app-info")
	if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
		t.Fatalf("mkdir app-info dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appInfoDir, metadataFixtureLocale+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write app-info localization file: %v", err)
	}
	return dir
}

func metadataPushActions(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var payload struct {
		Actions []map[string]any `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode push result: %v (stdout %q)", err, stdout)
	}
	return payload.Actions
}

func metadataPushResult(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode push result: %v (stdout %q)", err, stdout)
	}
	return payload
}

func countRequests(seen []ifExistsRequest, method, path string) int {
	count := 0
	for _, req := range seen {
		if req.Method == method && req.Path == path {
			count++
		}
	}
	return count
}

const metadataPushVersionLocalizationsRemote = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Remote JA description"}}],"links":{"next":""}}`

const metadataPushVersionLocalizationsPlanned = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Planned JA description","whatsNew":"Planned JA release notes"}}],"links":{"next":""}}`

// metadataPushVersionConflictHandler replays a metadata push whose version
// localization create for "ja" fails with conflictBody. The locale is absent
// from the plan read and present afterwards, which is exactly the retry shape
// the telemetry shows: the reconciliation read-back finds the locale with
// different content, so the conflict survives into --if-exists handling.
func metadataPushVersionConflictHandler(t *testing.T, conflictBody string, patched *bool) func(ifExistsRequest) (*http.Response, error) {
	t.Helper()
	posts := 0
	return func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			switch {
			case patched != nil && *patched:
				return jsonResponse(http.StatusOK, metadataPushVersionLocalizationsPlanned)
			case posts == 0:
				return jsonResponse(http.StatusOK, metadataPushEmptyList)
			default:
				return jsonResponse(http.StatusOK, metadataPushVersionLocalizationsRemote)
			}
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			posts++
			return jsonResponse(http.StatusConflict, conflictBody)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			if patched != nil {
				*patched = true
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Planned JA description","whatsNew":"Planned JA release notes"}}}`)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	}
}

func TestMetadataPushIfExistsSkipRecordsSkippedAction(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","whatsNew":"Planned JA release notes"}`)
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "skip", "--output", "json",
	}, metadataPushVersionConflictHandler(t, metadataVersionLocaleDuplicate409, nil))

	if runErr != nil {
		t.Fatalf("expected exit 0 with --if-exists skip, got %v (stderr %q)", runErr, stderr)
	}
	if countRequests(seen, http.MethodPatch, "/v1/appStoreVersionLocalizations/loc-ja") != 0 {
		t.Fatalf("--if-exists skip must not update the existing localization: %v", seen)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 {
		t.Fatalf("actions = %v, want exactly one", actions)
	}
	action := actions[0]
	if action["status"] != "skipped" {
		t.Fatalf("action status = %v, want skipped", action["status"])
	}
	if action["alreadyExists"] != true {
		t.Fatalf("action alreadyExists = %v, want true", action["alreadyExists"])
	}
	if action["ifExists"] != "skip" {
		t.Fatalf("action ifExists = %v, want skip", action["ifExists"])
	}
	if action["localizationId"] != "loc-ja" {
		t.Fatalf("action localizationId = %v, want loc-ja", action["localizationId"])
	}
	result := metadataPushResult(t, stdout)
	if result["skipped"] != float64(1) {
		t.Fatalf("result skipped = %v, want 1", result["skipped"])
	}
	if result["failed"] != nil {
		t.Fatalf("result failed = %v, want omitted", result["failed"])
	}
	if result["applied"] != true {
		t.Fatalf("result applied = %v, want true", result["applied"])
	}
	if !strings.Contains(stderr, "already exists") || !strings.Contains(stderr, "--if-exists skip") {
		t.Fatalf("stderr = %q, want an --if-exists skip diagnostic", stderr)
	}
}

func TestMetadataPushIfExistsUpdateRoutesToLocalizationPatch(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","whatsNew":"Planned JA release notes"}`)
	patched := false
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "update", "--output", "json",
	}, metadataPushVersionConflictHandler(t, metadataVersionLocaleDuplicate409, &patched))

	if runErr != nil {
		t.Fatalf("expected exit 0 with --if-exists update, got %v (stderr %q)", runErr, stderr)
	}
	if !patched {
		t.Fatalf("--if-exists update must PATCH the existing localization: %v", seen)
	}
	var patchBody string
	for _, req := range seen {
		if req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-ja" {
			patchBody = req.Body
		}
	}
	if !strings.Contains(patchBody, `"description":"Planned JA description"`) {
		t.Fatalf("PATCH body = %q, want the planned description", patchBody)
	}
	if strings.Contains(patchBody, `"locale"`) {
		t.Fatalf("PATCH body = %q, must not carry the immutable locale", patchBody)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 {
		t.Fatalf("actions = %v, want exactly one", actions)
	}
	action := actions[0]
	if action["status"] != "succeeded" || action["action"] != "update" {
		t.Fatalf("action = %v, want succeeded update", action)
	}
	if action["alreadyExists"] != true || action["ifExists"] != "update" {
		t.Fatalf("action = %v, want alreadyExists with ifExists update", action)
	}
	if !strings.Contains(stderr, "--if-exists update") {
		t.Fatalf("stderr = %q, want an --if-exists update diagnostic", stderr)
	}
}

func TestMetadataPushIfExistsUpdateContinuesWhenOptionalMatchReadFails(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","keywords":"planned,keywords","supportUrl":"https://example.com/support","whatsNew":"Planned release notes"}`)
	localizationReads := 0
	patched := false
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			localizationReads++
			switch localizationReads {
			case 1:
				return jsonResponse(http.StatusOK, metadataPushEmptyList)
			case 2, 4:
				return jsonResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"PARAMETER_ERROR.INVALID","title":"Invalid request"}]}`)
			default:
				return jsonResponse(http.StatusOK, metadataPushVersionLocalizationsRemote)
			}
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusConflict, metadataVersionLocaleDuplicate409)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			patched = true
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Planned JA description"}}}`)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})

	if runErr != nil {
		t.Fatalf("expected update to proceed after optional read failure, got %v (stderr %q)", runErr, stderr)
	}
	if !patched || countRequests(seen, http.MethodPatch, "/v1/appStoreVersionLocalizations/loc-ja") != 1 {
		t.Fatalf("expected one PATCH despite the optional comparison read failure, requests: %v", seen)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 || actions[0]["action"] != "update" || actions[0]["status"] != "succeeded" {
		t.Fatalf("actions = %v, want one successful update", actions)
	}
}

func TestMetadataPushIfExistsFailKeepsConflictFailure(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","whatsNew":"Planned JA release notes"}`)
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--output", "json",
	}, metadataPushVersionConflictHandler(t, metadataVersionLocaleDuplicate409, nil))

	if runErr == nil {
		t.Fatalf("expected the default --if-exists fail to keep failing (stderr %q)", stderr)
	}
	if countRequests(seen, http.MethodPatch, "/v1/appStoreVersionLocalizations/loc-ja") != 0 {
		t.Fatalf("default mode must not PATCH: %v", seen)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 || actions[0]["status"] != "failed" {
		t.Fatalf("actions = %v, want one failed action", actions)
	}
	if actions[0]["alreadyExists"] != nil || actions[0]["ifExists"] != nil {
		t.Fatalf("actions = %v, want no --if-exists fields on the default path", actions)
	}
}

func TestMetadataPushIfExistsSkipKeepsNonExistenceConflictFailing(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","whatsNew":"Planned JA release notes"}`)
	stdout, stderr, _, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "skip", "--output", "json",
	}, metadataPushVersionConflictHandler(t, metadataLocalizationState409, nil))

	if runErr == nil {
		t.Fatalf("expected a STATE_ERROR 409 to keep failing under --if-exists skip (stderr %q)", stderr)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 || actions[0]["status"] != "failed" {
		t.Fatalf("actions = %v, want one failed action", actions)
	}
}

func TestMetadataPushIfExistsSkipFailsWhenReadBackFindsNothing(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","whatsNew":"Planned JA release notes"}`)
	stdout, stderr, _, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusConflict, metadataVersionLocaleDuplicate409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})

	if runErr == nil {
		t.Fatalf("expected failure when the read-back finds no locale (stderr %q)", stderr)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 || actions[0]["status"] != "failed" {
		t.Fatalf("actions = %v, want one failed action", actions)
	}
}

func TestMetadataPushRejectsUnknownIfExistsValue(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","whatsNew":"Planned JA release notes"}`)
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "merge", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s before flag validation", req.Method, req.Path)
		return nil, nil
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("runErr = %v, want a usage error (exit 2)", runErr)
	}
	if len(seen) != 0 {
		t.Fatalf("expected no HTTP request, got %v", seen)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--if-exists must be one of fail, skip, update") {
		t.Fatalf("stderr = %q, want the supported --if-exists values", stderr)
	}
}

func TestMetadataPushRejectsExplicitlyEmptyIfExistsValue(t *testing.T) {
	// The flag defaults to fail, so --if-exists "" is an explicitly supplied
	// unsupported value and must not be read as fail. Internal callers that
	// leave PushExecutionOptions.IfExists unset still mean fail; that path goes
	// through shared.ParseOptionalIfExistsMode instead.
	//
	// Each value runs as a subtest: runIfExistsCommand installs a package-wide
	// default transport that is only released by the test's own cleanup, so two
	// calls in one test scope would deadlock.
	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "whitespace", raw: "   "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","whatsNew":"Planned JA release notes"}`)
			stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
				"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
				"--if-exists", test.raw, "--output", "json",
			}, func(req ifExistsRequest) (*http.Response, error) {
				t.Fatalf("unexpected request %s %s before flag validation", req.Method, req.Path)
				return nil, nil
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("runErr = %v, want a usage error (exit 2)", runErr)
			}
			if len(seen) != 0 {
				t.Fatalf("expected no HTTP request, got %v", seen)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "--if-exists must be one of fail, skip, update") {
				t.Fatalf("stderr = %q, want the supported --if-exists values", stderr)
			}
		})
	}
}

func TestMetadataPlanRejectsExplicitlyEmptyIfExistsValue(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","whatsNew":"Planned JA release notes"}`)
	_, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "plan", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s before flag validation", req.Method, req.Path)
		return nil, nil
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("runErr = %v, want a usage error (exit 2)", runErr)
	}
	if len(seen) != 0 {
		t.Fatalf("expected no HTTP request, got %v", seen)
	}
	if !strings.Contains(stderr, "--if-exists must be one of fail, skip, update") {
		t.Fatalf("stderr = %q, want the supported --if-exists values", stderr)
	}
}

func TestMetadataPushIfExistsUpdateRoutesAppInfoConflictToPatch(t *testing.T) {
	dir := writeMetadataAppInfoFixture(t, `{"name":"Planned JA name"}`)
	patched := false
	posts := 0
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			if patched {
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-ja","attributes":{"locale":"ja","name":"Planned JA name"}}],"links":{"next":""}}`)
			}
			if posts == 0 {
				return jsonResponse(http.StatusOK, metadataPushEmptyList)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-ja","attributes":{"locale":"ja","name":"Remote JA name"}}],"links":{"next":""}}`)
		case req.Method == http.MethodPost && req.Path == "/v1/appInfoLocalizations":
			posts++
			return jsonResponse(http.StatusConflict, metadataAppInfoLocaleDuplicate409)
		case req.Method == http.MethodPatch && req.Path == "/v1/appInfoLocalizations/loc-ja":
			patched = true
			return jsonResponse(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-ja","attributes":{"locale":"ja","name":"Planned JA name"}}}`)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})

	if runErr != nil {
		t.Fatalf("expected exit 0 with --if-exists update, got %v (stderr %q)", runErr, stderr)
	}
	if !patched {
		t.Fatalf("--if-exists update must PATCH the existing app-info localization: %v", seen)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 {
		t.Fatalf("actions = %v, want exactly one", actions)
	}
	if actions[0]["scope"] != "app-info" || actions[0]["alreadyExists"] != true {
		t.Fatalf("action = %v, want an app-info action resolved as already existing", actions[0])
	}
}

func TestMetadataPushIfExistsUpdateRoutesPatchOnlyAppInfoLocaleToPatch(t *testing.T) {
	dir := writeMetadataAppInfoFixture(t, `{"subtitle":"Planned JA subtitle"}`)
	patched := false
	localizationReads := 0
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			localizationReads++
			if localizationReads == 1 {
				return jsonResponse(http.StatusOK, metadataPushEmptyList)
			}
			if patched {
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-ja","attributes":{"locale":"ja","name":"Remote JA name","subtitle":"Planned JA subtitle"}}],"links":{"next":""}}`)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-ja","attributes":{"locale":"ja","name":"Remote JA name","subtitle":"Remote JA subtitle"}}],"links":{"next":""}}`)
		case req.Method == http.MethodPatch && req.Path == "/v1/appInfoLocalizations/loc-ja":
			if !strings.Contains(req.Body, `"subtitle":"Planned JA subtitle"`) || strings.Contains(req.Body, `"name"`) {
				t.Fatalf("PATCH body = %q, want only the planned subtitle and no name", req.Body)
			}
			patched = true
			return jsonResponse(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-ja","attributes":{"locale":"ja","name":"Remote JA name","subtitle":"Planned JA subtitle"}}}`)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})

	if runErr != nil {
		t.Fatalf("expected exit 0 with --if-exists update, got %v (stderr %q)", runErr, stderr)
	}
	if !patched {
		t.Fatalf("--if-exists update must PATCH a late-created patch-only app-info localization: %v", seen)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 || actions[0]["action"] != "update" || actions[0]["status"] != "succeeded" || actions[0]["alreadyExists"] != true {
		t.Fatalf("actions = %v, want one successful update resolved as already existing", actions)
	}
}

func TestMetadataPushIfExistsSkipRefreshesPatchOnlyAppInfoLocalesOnce(t *testing.T) {
	dir := t.TempDir()
	appInfoDir := filepath.Join(dir, "app-info")
	if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
		t.Fatalf("mkdir app-info dir: %v", err)
	}
	for locale, body := range map[string]string{
		"fr-FR": `{"subtitle":"Planned FR subtitle"}`,
		"ja":    `{"subtitle":"Planned JA subtitle"}`,
	} {
		if err := os.WriteFile(filepath.Join(appInfoDir, locale+".json"), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s app-info localization: %v", locale, err)
		}
	}

	localizationReads := 0
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			localizationReads++
			if localizationReads == 1 {
				return jsonResponse(http.StatusOK, metadataPushEmptyList)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-ja","attributes":{"locale":"ja","name":"Remote JA name"}},{"type":"appInfoLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","name":"Remote FR name"}}],"links":{"next":""}}`)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})

	if runErr != nil {
		t.Fatalf("expected late-existing locales to be skipped, got %v (stderr %q)", runErr, stderr)
	}
	if localizationReads != 2 || countRequests(seen, http.MethodGet, "/v1/appInfos/appinfo-1/appInfoLocalizations") != 2 {
		t.Fatalf("localization reads = %d, requests = %+v; want initial read plus one shared refresh", localizationReads, seen)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 2 || actions[0]["status"] != "skipped" || actions[1]["status"] != "skipped" {
		t.Fatalf("actions = %v, want both patch-only locales skipped", actions)
	}
}

func TestMetadataPushIfExistsUpdateRejectsMissingPatchOnlyAppInfoLocaleBeforeMutation(t *testing.T) {
	dir := writeMetadataAppInfoFixture(t, `{"subtitle":"Planned JA subtitle"}`)
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations", req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		}
		t.Fatalf("unexpected mutation request %s %s", req.Method, req.Path)
		return nil, nil
	})

	if runErr == nil || !strings.Contains(stderr, "requires name when creating a new locale") {
		t.Fatalf("runErr = %v, stderr = %q; want a missing-name error", runErr, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want no receipt before preflight succeeds", stdout)
	}
	for _, req := range seen {
		if req.Method == http.MethodPost || req.Method == http.MethodPatch || req.Method == http.MethodDelete {
			t.Fatalf("unexpected mutation before missing-name validation: %+v", req)
		}
	}
}

func TestMetadataPushIfExistsUpdateEmitsNoCreateReadinessWarning(t *testing.T) {
	// No whatsNew locally, so metadata push resolves the submit-readiness
	// context and would warn about a newly created locale. Nothing was
	// created here, so the warning must not be emitted.
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description"}`)
	patched := false
	stdout, stderr, _, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "update", "--output", "json",
	}, metadataPushVersionConflictHandler(t, metadataVersionLocaleDuplicate409, &patched))

	if runErr != nil {
		t.Fatalf("expected exit 0 with --if-exists update, got %v (stderr %q)", runErr, stderr)
	}
	if !patched {
		t.Fatalf("--if-exists update must PATCH the existing localization (stdout %q)", stdout)
	}
	if strings.Contains(stderr, "created locale ja") || strings.Contains(stderr, "creating locale ja") {
		t.Fatalf("stderr = %q, want no submit-readiness create warning for a resolved duplicate", stderr)
	}
}

func TestMetadataPushSuppressesCreateWarningForReconciledDuplicateVersionLocalization(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description"}`)
	posts := 0
	stdout, stderr, _, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			if posts == 0 {
				return jsonResponse(http.StatusOK, metadataPushEmptyList)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Planned JA description"}}],"links":{"next":""}}`)
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			posts++
			return jsonResponse(http.StatusConflict, metadataVersionLocaleDuplicate409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})

	if runErr != nil {
		t.Fatalf("expected the field-matching read-back to reconcile the duplicate, got %v (stderr %q)", runErr, stderr)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 || actions[0]["action"] != "reconcile" || actions[0]["status"] != "succeeded" {
		t.Fatalf("actions = %v, want one reconciled successful existing create", actions)
	}
	if _, exists := actions[0]["alreadyExists"]; exists {
		t.Fatalf("default-mode receipt changed unexpectedly: %v", actions[0])
	}
	if strings.Contains(stderr, "created locale ja") || strings.Contains(stderr, "creating locale ja") {
		t.Fatalf("stderr = %q, want no create-readiness warning for an existing duplicate", stderr)
	}
}

func TestMetadataPushRetainsCreateWarningAfterAmbiguousCreateReplayDuplicate(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description"}`)
	localizationReads := 0
	posts := 0
	createdAfterAmbiguousPost := false
	stdout, stderr, _, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, metadataPushVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
			return jsonResponse(http.StatusOK, metadataPushAppInfosList)
		case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonResponse(http.StatusOK, metadataPushEmptyList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			localizationReads++
			if localizationReads <= 3 {
				return jsonResponse(http.StatusOK, metadataPushEmptyList)
			}
			if !createdAfterAmbiguousPost {
				t.Fatal("final read-back happened before the ambiguous POST created the locale")
			}
			return jsonResponse(http.StatusOK, metadataPushVersionLocalizationsPlanned)
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			posts++
			if posts == 1 {
				// Simulate a server-side create whose response is lost, followed by
				// temporarily stale read-backs before the client replays the POST.
				createdAfterAmbiguousPost = true
				return jsonResponse(http.StatusBadGateway, `{"errors":[{"status":"502","code":"SERVICE_ERROR","title":"Temporary service failure"}]}`)
			}
			return jsonResponse(http.StatusConflict, metadataVersionLocaleDuplicate409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})

	if runErr != nil {
		t.Fatalf("expected the replayed create to reconcile, got %v (stderr %q)", runErr, stderr)
	}
	if posts != 2 {
		t.Fatalf("create POSTs = %d, want first ambiguous POST and one duplicate replay", posts)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 || actions[0]["action"] != "reconcile" || actions[0]["status"] != "succeeded" {
		t.Fatalf("actions = %v, want one successful reconciliation", actions)
	}
	if _, exists := actions[0]["alreadyExists"]; exists {
		t.Fatalf("ambiguous create replay must not be classified as pre-existing: %v", actions[0])
	}
	if !strings.Contains(stderr, "created locale ja now participates in submission validation") {
		t.Fatalf("stderr = %q, want the create-readiness warning because this run may have created the locale", stderr)
	}
}

func TestMetadataPushIfExistsReusesMatchingDuplicateReadBack(t *testing.T) {
	for _, mode := range []string{"skip", "update"} {
		t.Run(mode, func(t *testing.T) {
			dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description"}`)
			localizationReads := 0
			stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
				"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
				"--if-exists", mode, "--output", "json",
			}, func(req ifExistsRequest) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
					return jsonResponse(http.StatusOK, metadataPushVersionsList)
				case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appInfos":
					return jsonResponse(http.StatusOK, metadataPushAppInfosList)
				case req.Method == http.MethodGet && req.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
					return jsonResponse(http.StatusOK, metadataPushEmptyList)
				case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					localizationReads++
					if localizationReads == 1 {
						return jsonResponse(http.StatusOK, metadataPushEmptyList)
					}
					if localizationReads == 2 {
						return jsonResponse(http.StatusOK, metadataPushVersionLocalizationsPlanned)
					}
					t.Fatalf("unexpected redundant localization read %d after the matching read-back", localizationReads)
				case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
					return jsonResponse(http.StatusConflict, metadataVersionLocaleDuplicate409)
				}
				t.Fatalf("unexpected request %s %s", req.Method, req.Path)
				return nil, nil
			})

			if runErr != nil {
				t.Fatalf("expected the matching read-back to resolve the duplicate, got %v (stderr %q)", runErr, stderr)
			}
			if localizationReads != 2 || countRequests(seen, http.MethodPatch, "/v1/appStoreVersionLocalizations/loc-ja") != 0 {
				t.Fatalf("localization reads=%d, requests=%+v; want plan read, one matching read-back, and no PATCH", localizationReads, seen)
			}

			actions := metadataPushActions(t, stdout)
			if len(actions) != 1 || actions[0]["alreadyExists"] != true || actions[0]["ifExists"] != mode {
				t.Fatalf("actions = %v, want one existing-resource action for --if-exists %s", actions, mode)
			}
			if mode == "skip" && actions[0]["status"] != "skipped" {
				t.Fatalf("actions = %v, want skipped status", actions)
			}
			if mode == "update" && (actions[0]["status"] != "succeeded" || actions[0]["action"] != "reconcile") {
				t.Fatalf("actions = %v, want successful reconcile without PATCH", actions)
			}
		})
	}
}

// Apple can report several causes for one 409: a live duplicate-locale create
// with a too-short description returned the duplicate code followed by
// ENTITY_ERROR.ATTRIBUTE.INVALID.TOO_SHORT. This synthetic body puts the
// duplicate code after another cause, which only the shared matcher's walk over
// every errors[] entry catches; the duplicate entry is the recorded one.
const metadataVersionLocaleDuplicateSecondEntry409 = `{"errors":[{"id":"4d2b8e17-3a95-4c60-b1f7-5e8c9a0d2b43","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"The relationship 'appStoreVersion' is not valid for this request.","source":{"pointer":"/data/relationships/appStoreVersion"}},{"id":"ca643fcd-402d-4120-9e8b-db867fc15a83","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","title":"The provided entity includes an attribute with a value that has already been used","detail":"Entity with locale: ja already exists. Try updating.","source":{"pointer":"/data/attributes/locale"}}]}`

func TestMetadataPushIfExistsSkipMatchesDuplicateCodeAfterTheFirstError(t *testing.T) {
	dir := writeMetadataVersionFixture(t, `{"description":"Planned JA description","whatsNew":"Planned JA release notes"}`)
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--platform", "IOS", "--dir", dir,
		"--if-exists", "skip", "--output", "json",
	}, metadataPushVersionConflictHandler(t, metadataVersionLocaleDuplicateSecondEntry409, nil))

	if runErr != nil {
		t.Fatalf("expected exit 0 when the duplicate code is not errors[0], got %v (stderr %q)", runErr, stderr)
	}
	if countRequests(seen, http.MethodPatch, "/v1/appStoreVersionLocalizations/loc-ja") != 0 {
		t.Fatalf("--if-exists skip must not update the existing localization: %v", seen)
	}
	actions := metadataPushActions(t, stdout)
	if len(actions) != 1 || actions[0]["status"] != "skipped" || actions[0]["alreadyExists"] != true {
		t.Fatalf("actions = %v, want one skipped action resolved as already existing", actions)
	}
}
