package cmdtest

import (
	"net/http"
	"strings"
	"testing"
)

// Recorded duplicate-locale 409. Apple's detail is "Entity with locale: 'ja'
// already exists. Try updating." and the code is
// ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.
const localizationUpdateDuplicate409 = `{"errors":[{"id":"2f8a6d41-7c3b-4e59-8a12-9b0d6e4c3f77","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","title":"The provided entity includes an attribute with a value that has already been used","detail":"Entity with locale: 'ja' already exists. Try updating.","source":{"pointer":"/data/attributes/locale"}}]}`

const localizationUpdateState409 = `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"The version is not editable in its current state."}]}`

const localizationUpdateExisting = `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"old description"}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-ja"}}`

const localizationUpdatePatched = `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"new description"}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-ja"}}`

func TestLocalizationsUpdateIfExistsSkipReturnsExistingLocalization(t *testing.T) {
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "update", "--id", "loc-ja", "--description", "new description",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			return jsonResponse(http.StatusConflict, localizationUpdateDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			return jsonResponse(http.StatusOK, localizationUpdateExisting)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %+v, want PATCH then GET", seen)
	}
	if !strings.Contains(stdout, "old description") || strings.Contains(stdout, "alreadyExists") {
		t.Fatalf("stdout = %q, want the unmodified existing envelope", stdout)
	}
	if !strings.Contains(stderr, "loc-ja") || !strings.Contains(stderr, "left unchanged") || !strings.Contains(stderr, "--if-exists skip") {
		t.Fatalf("stderr = %q, want the skip diagnostic", stderr)
	}
}

func TestLocalizationsUpdateIfExistsUpdateRetriesTheFieldWrite(t *testing.T) {
	patches := 0
	stdout, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "update", "--id", "loc-ja", "--description", "new description",
		"--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			patches++
			if !strings.Contains(req.Body, `"description":"new description"`) {
				t.Fatalf("PATCH body = %s", req.Body)
			}
			if patches == 1 {
				return jsonResponse(http.StatusConflict, localizationUpdateDuplicate409)
			}
			return jsonResponse(http.StatusOK, localizationUpdatePatched)
		case req.Method == http.MethodGet && req.Path == "/v1/appStoreVersionLocalizations/loc-ja":
			return jsonResponse(http.StatusOK, localizationUpdateExisting)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if patches != 2 || len(seen) != 3 {
		t.Fatalf("patches = %d requests = %+v, want conflict, read-back, then the field write", patches, seen)
	}
	if !strings.Contains(stdout, "new description") {
		t.Fatalf("stdout = %q, want the updated envelope", stdout)
	}
	if !strings.Contains(stderr, "updated") || !strings.Contains(stderr, "--if-exists update") {
		t.Fatalf("stderr = %q, want the update diagnostic", stderr)
	}
}

func TestLocalizationsUpdateDefaultIfExistsFailPreservesConflict(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "update", "--id", "loc-ja", "--description", "new description", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPatch {
			return jsonResponse(http.StatusConflict, localizationUpdateDuplicate409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil {
		t.Fatal("expected the 409 to fail")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want only the PATCH", seen)
	}
}

func TestLocalizationsUpdateIfExistsKeepsStateConflict(t *testing.T) {
	_, _, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "update", "--id", "loc-ja", "--description", "new description",
		"--if-exists", "skip", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		if req.Method == http.MethodPatch {
			return jsonResponse(http.StatusConflict, localizationUpdateState409)
		}
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil {
		t.Fatal("expected the state 409 to fail")
	}
	if len(seen) != 1 {
		t.Fatalf("requests = %+v, want no read-back", seen)
	}
}

func TestLocalizationsUpdateRejectsInvalidIfExistsBeforeHTTP(t *testing.T) {
	_, stderr, seen, runErr := runIfExistsCommand(t, []string{
		"localizations", "update", "--id", "loc-ja", "--description", "new description",
		"--if-exists", "merge", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s", req.Method, req.Path)
		return nil, nil
	})
	if runErr == nil {
		t.Fatal("expected a usage error")
	}
	if len(seen) != 0 {
		t.Fatalf("requests = %+v, want none", seen)
	}
	if !strings.Contains(stderr, "--if-exists") || !strings.Contains(stderr, "fail, skip, update") {
		t.Fatalf("stderr = %q, want the allowed modes", stderr)
	}
}
