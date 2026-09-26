package cmdtest

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// Apple's 409 body for a duplicate locale on POST /v1/appStoreVersionLocalizations.
const localizationDuplicate409 = `{"errors":[{"id":"2f8a6d41-7c3b-4e59-8a12-9b0d6e4c3f77","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","title":"The provided entity includes an attribute with a value that has already been used","detail":"Entity with locale: 'ja' already exists. Try updating.","source":{"pointer":"/data/attributes/locale"}}]}`

// A 409 that is not an existence conflict and must keep failing.
const localizationState409 = `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"The version is not editable in its current state."}]}`

const existingLocalizationsList = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"old description","whatsNew":"old whats new"}},{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"english"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations"}}`

const existingLocalizationDetail = `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"old description","whatsNew":"old whats new"},"relationships":{"appStoreVersion":{"links":{"related":"https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-ja/appStoreVersion"}}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-ja"}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-ja"}}`

const noLocalizationsList = `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations"}}`

func TestLocalizationsCreateIfExistsSkipReturnsExistingLocalization(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "create", "--version", "version-1", "--locale", "ja",
		"--description", "new description", "--whats-new", "new notes", "--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusConflict, localizationDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, existingLocalizationsList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			return jsonResponse(http.StatusOK, existingLocalizationDetail)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, the locale read-back, then the detail re-read", seen)
	}
	// Apple's own single-resource envelope is printed unmodified, including the
	// top-level links the collection response does not carry per resource.
	if !strings.Contains(stdout, `"id":"loc-ja"`) || !strings.Contains(stdout, "old description") {
		t.Fatalf("stdout = %q, want Apple's envelope for loc-ja", stdout)
	}
	if !strings.Contains(stdout, "appStoreVersionLocalizations/loc-ja") {
		t.Fatalf("stdout = %q, want Apple's detail envelope links preserved, not a synthesized envelope", stdout)
	}
	if strings.Contains(stdout, "alreadyExists") || strings.Contains(stdout, `"action"`) {
		t.Fatalf("stdout = %q, must not decorate Apple's envelope", stdout)
	}
	if !strings.Contains(stderr, "already exists") || !strings.Contains(stderr, "loc-ja") || !strings.Contains(stderr, "--if-exists skip") {
		t.Fatalf("stderr = %q, want the already-exists diagnostic", stderr)
	}
}

func TestLocalizationsCreateIfExistsUpdatePatchesExistingLocalization(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "create", "--version", "version-1", "--locale", "ja",
		"--description", "new description", "--keywords", "one,two", "--whats-new", "new notes",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusConflict, localizationDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, existingLocalizationsList)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			if !strings.Contains(req.Body, `"description":"new description"`) || !strings.Contains(req.Body, `"keywords":"one,two"`) || !strings.Contains(req.Body, `"whatsNew":"new notes"`) {
				t.Fatalf("PATCH body = %s, want the create fields", req.Body)
			}
			if strings.Contains(req.Body, `"locale"`) {
				t.Fatalf("PATCH body = %s, must not resend the immutable locale", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"new description","keywords":"one,two"}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, read-back GET, PATCH", seen)
	}
	if !strings.Contains(stdout, `"id":"loc-ja"`) || !strings.Contains(stdout, "new description") {
		t.Fatalf("stdout = %q, want the PATCH response envelope", stdout)
	}
	if !strings.Contains(stderr, "--if-exists update") {
		t.Fatalf("stderr = %q, want the already-exists diagnostic", stderr)
	}
}

// A handled duplicate created nothing, so the create-readiness warning (which
// describes a locale just created from these attributes) must not be emitted and
// its version lookup must not run. Without --whats-new a real create would both
// look the version up and warn.
func TestLocalizationsCreateIfExistsSkipEmitsNoCreateReadinessWarning(t *testing.T) {
	_, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "create", "--version", "version-1", "--locale", "ja",
		"--description", "new description", "--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusConflict, localizationDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, existingLocalizationsList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			return jsonResponse(http.StatusOK, existingLocalizationDetail)
		default:
			t.Fatalf("unexpected request %s %s; no submit-readiness lookup may run for a handled duplicate", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, the locale read-back, and the detail re-read only", seen)
	}
	if strings.Contains(stderr, "was created") || strings.Contains(stderr, "whatsNew") {
		t.Fatalf("stderr = %q, must not claim the locale was created", stderr)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Fatalf("stderr = %q, want the already-exists diagnostic", stderr)
	}
}

// A locale-only create has nothing the PATCH can carry, so --if-exists update
// resolves it like skip rather than sending an empty PATCH that a non-editable
// localization could reject. This mirrors versions create --if-exists update.
func TestLocalizationsCreateIfExistsUpdateWithoutUpdatableFieldsDoesNotPatch(t *testing.T) {
	_, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "create", "--version", "version-1", "--locale", "ja",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusConflict, localizationDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, existingLocalizationsList)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			return jsonResponse(http.StatusOK, existingLocalizationDetail)
		default:
			t.Fatalf("unexpected request %s %s; an empty PATCH must not be sent", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, the locale read-back, and the detail re-read only", seen)
	}
	for _, req := range seen {
		if req.Method == http.MethodPatch {
			t.Fatalf("unexpected PATCH %+v; nothing was updatable", req)
		}
	}
	if !strings.Contains(stderr, "left unchanged") {
		t.Fatalf("stderr = %q, want the left-unchanged outcome", stderr)
	}
}

func TestLocalizationsCreateDefaultIfExistsFailPreservesConflict(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "create", "--version", "version-1", "--locale", "ja",
		"--description", "new description", "--whats-new", "new notes", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations" {
			return jsonResponse(http.StatusConflict, localizationDuplicate409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the 409 conflict", runErr)
	}
	if !strings.Contains(runErr.Error(), "already exists. Try updating.") {
		t.Fatalf("run error = %v, want Apple's detail preserved", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want only the POST", seen)
	}
}

func TestLocalizationsCreateIfExistsStillFailsForStateConflict(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "create", "--version", "version-1", "--locale", "ja",
		"--description", "new description", "--whats-new", "new notes", "--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations" {
			return jsonResponse(http.StatusConflict, localizationState409)
		}
		t.Fatalf("unexpected request %s %s; a non-existence 409 must not trigger a read-back", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the state conflict", runErr)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want only the POST", seen)
	}
}

func TestLocalizationsCreateIfExistsSkipStillFailsWhenReadBackFindsNothing(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "create", "--version", "version-1", "--locale", "ja",
		"--description", "new description", "--whats-new", "new notes", "--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusConflict, localizationDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, noLocalizationsList)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr == nil || !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("run error = %v, want the original conflict when the read-back finds nothing", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want POST then the read-back GET", seen)
	}
}

func TestLocalizationsCreateRejectsInvalidIfExistsBeforeHTTP(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "create", "--version", "version-1", "--locale", "ja",
		"--description", "new description", "--whats-new", "new notes", "--if-exists", "replace", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s; the usage error must precede any HTTP request", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil {
		t.Fatal("run error = nil, want a usage error")
	}
	if !strings.Contains(runErr.Error(), "--if-exists must be one of fail, skip, update") {
		t.Fatalf("run error = %v, want the supported-modes usage error", runErr)
	}
	if len(seen) != 0 {
		t.Fatalf("requests = %+v, want none", seen)
	}
}
