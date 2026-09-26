package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Apple's real 409 body for a duplicate versionString on POST /v1/appStoreVersions,
// captured live against app 6759231657 on 2026-09-15. Apple reports two errors
// and the duplicate is the second one, so the conflict matcher has to inspect
// every entry in errors[] rather than only the first.
const versionsDuplicate409 = `{"errors":[{"id":"b068c5c0-b3fa-4d12-aa89-f1b9aa061b28","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"You cannot create a new version of the App in the current state.","source":{"pointer":"/data/relationships/app"}},{"id":"eb1884a7-e427-42db-ac95-26c49c84a5c2","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","title":"The provided entity includes an attribute with a value that has already been used","detail":"The version number has been previously used.","source":{"pointer":"/data/attributes/versionString"}}]}`

// Apple's 409 body when an appStoreReviewDetail already exists for the version,
// captured live against app 6759231657 on 2026-09-15. Apple answers this
// existence conflict with STATE_ERROR.ALREADY_EXISTS.
const reviewDetailExists409 = `{"errors":[{"id":"a48854d3-ef2e-4eea-9451-228c907bf1ad","status":"409","code":"STATE_ERROR.ALREADY_EXISTS","title":"Resource already exists.","detail":"The given app version already has an existing review."}]}`

// Apple's 409 on POST /v1/appStoreVersions when the app cannot take a new
// version yet; it is not an existence conflict and must keep failing.
const versionsRelationship409 = `{"errors":[{"id":"7c2d9e1f-4b3a-4c6d-8e5f-1a2b3c4d5e6f","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"You cannot create a new version of the App in the current state.","source":{"pointer":"/data/relationships/app"}}]}`

// A 409 that is not an existence conflict and must keep failing.
const versionsState409 = `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"You cannot create a new version while another version is in review."}]}`

const existingVersionsList = `{"data":[{"type":"appStoreVersions","id":"version-existing","attributes":{"versionString":"2.0.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION","copyright":"old"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions"}}`

const existingReviewDetail = `{"data":{"type":"appStoreReviewDetails","id":"detail-existing","attributes":{"contactFirstName":"Old","notes":"old notes","demoAccountRequired":false}}}`

const sourceVersionsList = `{"data":[{"type":"appStoreVersions","id":"version-source","attributes":{"versionString":"1.9.0","platform":"IOS","appStoreState":"READY_FOR_SALE"}}]}`

const sourceLocalizationsList = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-source-en","attributes":{"locale":"en-US","description":"Source description","keywords":"one,two"}}]}`

const destinationLocalizationsList = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-dest-en","attributes":{"locale":"en-US"}}]}`

type ifExistsRequest struct {
	Method string
	Path   string
	Query  string
	Body   string
}

func runIfExistsCommand(t *testing.T, args []string, handler func(req ifExistsRequest) (*http.Response, error)) (string, string, []ifExistsRequest, error) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	log := newRequestLog(4)
	var seen []ifExistsRequest
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := ""
		if req.Body != nil {
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			body = string(payload)
		}
		entry := ifExistsRequest{Method: req.Method, Path: req.URL.Path, Query: req.URL.RawQuery, Body: body}
		log.Add(req.Method + " " + req.URL.Path)
		seen = append(seen, entry)
		return handler(entry)
	}))

	root := RootCommand("test")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, seen, runErr
}

func TestVersionsCreateIfExistsSkipReturnsExistingVersion(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--platform", "IOS",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			if !strings.Contains(req.Query, "filter%5BversionString%5D=2.0.0") || !strings.Contains(req.Query, "filter%5Bplatform%5D=IOS") {
				t.Fatalf("read-back query = %q, want versionString and platform filters", req.Query)
			}
			return jsonResponse(http.StatusOK, existingVersionsList)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %d, want POST then read-back GET: %+v", len(seen), seen)
	}
	var result struct {
		ID            string `json:"id"`
		VersionString string `json:"versionString"`
		AlreadyExists bool   `json:"alreadyExists"`
		Action        string `json:"action"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.ID != "version-existing" || result.VersionString != "2.0.0" {
		t.Fatalf("receipt = %+v, want existing version", result)
	}
	if !result.AlreadyExists || result.Action != "skipped" {
		t.Fatalf("receipt = %+v, want alreadyExists=true action=skipped", result)
	}
	if !strings.Contains(stderr, "already exists") || !strings.Contains(stderr, "version-existing") || !strings.Contains(stderr, "--if-exists skip") {
		t.Fatalf("stderr = %q, want already-exists diagnostic", stderr)
	}
}

func TestVersionsCreateIfExistsUpdatePatchesExistingVersion(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0",
		"--copyright", "2026 Example", "--release-type", "MANUAL",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, existingVersionsList)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersions/version-existing":
			if !strings.Contains(req.Body, `"copyright":"2026 Example"`) || !strings.Contains(req.Body, `"releaseType":"MANUAL"`) {
				t.Fatalf("PATCH body = %s, want copyright and releaseType", req.Body)
			}
			if strings.Contains(req.Body, "versionString") || strings.Contains(req.Body, "platform") {
				t.Fatalf("PATCH body = %s, must not resend create-only attributes", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-existing","attributes":{"versionString":"2.0.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION","copyright":"2026 Example","releaseType":"MANUAL"}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, GET, PATCH", seen)
	}
	var result struct {
		ID            string `json:"id"`
		AlreadyExists bool   `json:"alreadyExists"`
		Action        string `json:"action"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.ID != "version-existing" || !result.AlreadyExists || result.Action != "updated" {
		t.Fatalf("receipt = %+v, want existing id, alreadyExists=true, action=updated", result)
	}
	if !strings.Contains(stderr, "already exists") || !strings.Contains(stderr, "--if-exists update") {
		t.Fatalf("stderr = %q, want already-exists diagnostic", stderr)
	}
}

func TestVersionsCreateIfExistsUpdateWithoutUpdatableFlagsSkips(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, existingVersionsList)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want no PATCH when nothing is updatable", seen)
	}
	if !strings.Contains(stdout, `"action":"skipped"`) {
		t.Fatalf("stdout = %q, want action=skipped", stdout)
	}
}

// --if-exists update carries --copy-metadata-from onto the existing version even
// when there is nothing to PATCH on the version itself, matching the documented
// behavior: skip leaves the version untouched, update still copies metadata.
func TestVersionsCreateIfExistsUpdateCopiesMetadataWithoutUpdatableFlags(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--platform", "IOS",
		"--copy-metadata-from", "1.9.0", "--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			if strings.Contains(req.Query, "filter%5BversionString%5D=1.9.0") {
				return jsonResponse(http.StatusOK, sourceVersionsList)
			}
			return jsonResponse(http.StatusOK, existingVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-source/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, sourceLocalizationsList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-existing/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, destinationLocalizationsList)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-dest-en":
			if !strings.Contains(req.Body, "Source description") {
				t.Fatalf("copy PATCH body = %s, want the source description", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-dest-en","attributes":{"locale":"en-US"}}}`)
		default:
			t.Fatalf("unexpected request %s %s?%s", req.Method, req.Path, req.Query)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	var result struct {
		Action       string `json:"action"`
		MetadataCopy *struct {
			SourceVersion      string `json:"sourceVersion"`
			CopiedLocales      int    `json:"copiedLocales"`
			CopiedFieldUpdates int    `json:"copiedFieldUpdates"`
		} `json:"metadataCopy"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.MetadataCopy == nil {
		t.Fatalf("receipt = %q, want metadataCopy on --if-exists update", stdout)
	}
	if result.MetadataCopy.SourceVersion != "1.9.0" || result.MetadataCopy.CopiedLocales != 1 {
		t.Fatalf("metadataCopy = %+v, want the 1.9.0 copy to have run", *result.MetadataCopy)
	}
	// The metadata copy PATCHes the existing version's localizations, so the
	// receipt must report a mutation even though the version resource itself
	// had nothing to PATCH.
	if result.Action != "updated" {
		t.Fatalf("action = %q, want updated when the metadata copy ran against the existing version", result.Action)
	}
	if !strings.Contains(stderr, "updated it in place") {
		t.Fatalf("stderr = %q, want the update diagnostic rather than left unchanged", stderr)
	}
	sawCopyPatch := false
	for _, req := range seen {
		if req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-dest-en" {
			sawCopyPatch = true
		}
	}
	if !sawCopyPatch {
		t.Fatalf("requests = %+v, want the metadata copy PATCH", seen)
	}
}

// --if-exists skip must leave the existing version completely untouched, so the
// metadata copy does not run.
func TestVersionsCreateIfExistsSkipDoesNotCopyMetadata(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--platform", "IOS",
		"--copy-metadata-from", "1.9.0", "--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, existingVersionsList)
		default:
			t.Fatalf("unexpected request %s %s?%s", req.Method, req.Path, req.Query)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want only the POST and the read-back", seen)
	}
	if strings.Contains(stdout, "metadataCopy") {
		t.Fatalf("stdout = %q, want no metadataCopy on skip", stdout)
	}
}

func TestVersionsCreateDefaultIfExistsFailPreservesConflict(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions" {
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the 409 conflict", runErr)
	}
	// Apple's own errors[0] detail is what the CLI has always surfaced for this
	// body; --if-exists fail must keep printing it verbatim.
	if !strings.Contains(runErr.Error(), "You cannot create a new version of the App in the current state.") {
		t.Fatalf("run error = %v, want Apple detail preserved", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want only the POST", seen)
	}
}

func TestVersionsCreateDefaultIfExistsFailPreservesSuccessfulOutput(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "default", args: nil},
		{name: "explicit fail", args: []string{"--if-exists", "fail"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"versions", "create", "--app", "app-1", "--version", "2.0.0", "--platform", "IOS", "--output", "json"}
			args = append(args, tt.args...)
			stdout, stderr, seen, runErr := runIfExistsCommand(t, args, func(req ifExistsRequest) (*http.Response, error) {
				if req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions" {
					return jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreVersions","id":"version-created","attributes":{"versionString":"2.0.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}}`)
				}
				t.Fatalf("unexpected request %s %s", req.Method, req.Path)
				return nil, nil
			})
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			const want = `{"id":"version-created","versionString":"2.0.0","platform":"IOS","state":"PREPARE_FOR_SUBMISSION"}` + "\n"
			if stdout != want {
				t.Fatalf("stdout = %q, want legacy output %q", stdout, want)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if len(seen) != 1 {
				t.Fatalf("requests = %+v, want only the POST", seen)
			}
		})
	}
}

func TestVersionsCreateIfExistsSkipStillFailsWhenReadBackFindsNothing(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the original 409", runErr)
	}
	if !strings.Contains(runErr.Error(), "You cannot create a new version of the App in the current state.") {
		t.Fatalf("run error = %v, want the original conflict", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want POST then read-back", seen)
	}
}

func TestVersionsCreateIfExistsOnlyHandlesDuplicateVersionCode(t *testing.T) {
	for name, body := range map[string]string{
		"state error":           versionsState409,
		"relationship rejected": versionsRelationship409,
	} {
		t.Run(name, func(t *testing.T) {
			stdout, _, seen, runErr := runIfExistsCommand(t, []string{
				"versions", "create", "--app", "app-1", "--version", "2.0.0", "--if-exists", "skip", "--output", "json",
			}, func(req ifExistsRequest) (*http.Response, error) {
				if req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions" {
					return jsonResponse(http.StatusConflict, body)
				}
				t.Fatalf("unexpected request %s %s", req.Method, req.Path)
				return nil, nil
			})
			if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
				t.Fatalf("run error = %v, want the original 409", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if len(seen) != 1 {
				t.Fatalf("requests = %+v, want only the POST (no read-back for a non-existence 409)", seen)
			}
		})
	}
}

func TestVersionsCreateRejectsInvalidIfExistsBeforeHTTP(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--if-exists", "merge",
	}, func(req ifExistsRequest) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})
	if !isUsageClassError(runErr) {
		t.Fatalf("run error = %v, want usage-class error", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--if-exists") || !strings.Contains(stderr, "fail, skip, update") {
		t.Fatalf("stderr = %q, want --if-exists allowed values", stderr)
	}
	if len(seen) != 0 {
		t.Fatalf("requests = %+v, want none", seen)
	}
}

func TestReviewDetailsCreateIfExistsSkipReturnsExistingDetail(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"review", "details-create", "--version-id", "version-1", "--notes", "new notes",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreReviewDetails":
			return jsonResponse(http.StatusConflict, reviewDetailExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreReviewDetail":
			return jsonResponse(http.StatusOK, existingReviewDetail)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want POST then read-back GET", seen)
	}
	if !strings.Contains(stdout, `"id":"detail-existing"`) || !strings.Contains(stdout, `"notes":"old notes"`) {
		t.Fatalf("stdout = %q, want the existing detail envelope unchanged", stdout)
	}
	if !strings.Contains(stderr, "detail-existing") || !strings.Contains(stderr, "already exists") || !strings.Contains(stderr, "--if-exists skip") {
		t.Fatalf("stderr = %q, want already-exists diagnostic", stderr)
	}
}

func TestReviewDetailsCreateIfExistsUpdatePatchesExistingDetail(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "details-create", "--version-id", "version-1",
		"--contact-first-name", "Dev", "--notes", "new notes",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreReviewDetails":
			return jsonResponse(http.StatusConflict, reviewDetailExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreReviewDetail":
			return jsonResponse(http.StatusOK, existingReviewDetail)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreReviewDetails/detail-existing":
			if !strings.Contains(req.Body, `"notes":"new notes"`) || !strings.Contains(req.Body, `"contactFirstName":"Dev"`) {
				t.Fatalf("PATCH body = %s, want the same attributes", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-existing","attributes":{"contactFirstName":"Dev","notes":"new notes"}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, GET, PATCH", seen)
	}
	if !strings.Contains(stdout, `"notes":"new notes"`) {
		t.Fatalf("stdout = %q, want PATCH response", stdout)
	}
}

func TestReviewDetailsCreateIfExistsUpdateReusesExistingDemoCredentials(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "details-create", "--version-id", "version-1",
		"--demo-account-required=true",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreReviewDetail":
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-existing","attributes":{"demoAccountName":"reviewer@example.com","demoAccountPassword":"existing-password","demoAccountRequired":true}}}`)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreReviewDetails/detail-existing":
			if !strings.Contains(req.Body, `"demoAccountRequired":true`) {
				t.Fatalf("PATCH body = %s, want demoAccountRequired=true", req.Body)
			}
			if strings.Contains(req.Body, "demoAccountName") || strings.Contains(req.Body, "demoAccountPassword") {
				t.Fatalf("PATCH body = %s, must not resend existing credentials", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-existing","attributes":{"demoAccountRequired":true}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want read-back GET then PATCH", seen)
	}
	if !strings.Contains(stdout, `"id":"detail-existing"`) {
		t.Fatalf("stdout = %q, want updated detail", stdout)
	}
}

func TestReviewDetailsCreateDefaultIfExistsFailPreservesConflict(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "details-create", "--version-id", "version-1", "--notes", "new notes", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPost && req.Path == "/v1/appStoreReviewDetails" {
			return jsonResponse(http.StatusConflict, reviewDetailExists409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the 409 conflict", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want only the POST", seen)
	}
}

func TestReviewDetailsCreateIfExistsStillFailsForStateConflict(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "details-create", "--version-id", "version-1", "--notes", "new notes",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPost && req.Path == "/v1/appStoreReviewDetails" {
			return jsonResponse(http.StatusConflict, versionsState409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the original 409", runErr)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want only the POST", seen)
	}
}

func TestReviewDetailsCreateIfExistsSkipStillFailsWhenReadBackFindsNothing(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"review", "details-create", "--version-id", "version-1", "--notes", "new notes",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreReviewDetails":
			return jsonResponse(http.StatusConflict, reviewDetailExists409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreReviewDetail":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the original 409", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want POST then read-back", seen)
	}
}

// A metadata copy that changes nothing leaves the existing version untouched,
// so the receipt must stay skipped rather than claim an update.
func TestVersionsCreateIfExistsUpdateKeepsSkippedWhenMetadataCopyChangesNothing(t *testing.T) {
	stdout, stderr, _, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0", "--platform", "IOS",
		"--copy-metadata-from", "1.9.0", "--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			if strings.Contains(req.Query, "filter%5BversionString%5D=1.9.0") {
				return jsonResponse(http.StatusOK, sourceVersionsList)
			}
			return jsonResponse(http.StatusOK, existingVersionsList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-source/appStoreVersionLocalizations":
			// The source locale carries none of the copyable fields.
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-source-en","attributes":{"locale":"en-US"}}]}`)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-existing/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, destinationLocalizationsList)
		default:
			t.Fatalf("unexpected request %s %s?%s", req.Method, req.Path, req.Query)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	var result struct {
		Action       string `json:"action"`
		MetadataCopy *struct {
			CopiedFieldUpdates int `json:"copiedFieldUpdates"`
		} `json:"metadataCopy"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.MetadataCopy == nil || result.MetadataCopy.CopiedFieldUpdates != 0 {
		t.Fatalf("metadataCopy = %+v, want a copy that applied no field updates", result.MetadataCopy)
	}
	if result.Action != "skipped" {
		t.Fatalf("action = %q, want skipped when the copy changed nothing", result.Action)
	}
	if !strings.Contains(stderr, "left unchanged") {
		t.Fatalf("stderr = %q, want the left-unchanged diagnostic", stderr)
	}
}
