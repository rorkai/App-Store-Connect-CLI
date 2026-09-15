package shared

import (
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestParseIfExistsModeAcceptsSupportedValuesOnly(t *testing.T) {
	// The flag defaults to "fail", so an empty raw value can only come from an
	// explicit --if-exists "" (or an all-whitespace value). Accepting it would
	// silently ignore an unsupported value.
	for _, raw := range []string{"", "   "} {
		_, err := ParseIfExistsMode(raw, IfExistsSkip)
		if err == nil {
			t.Fatalf("ParseIfExistsMode(%q) = nil error; want usage error", raw)
		}
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("ParseIfExistsMode(%q) = %v; want usage-class error", raw, err)
		}
		if !strings.Contains(err.Error(), "fail, skip") {
			t.Fatalf("ParseIfExistsMode(%q) = %v; want the supported modes listed", raw, err)
		}
	}
	if mode, err := ParseIfExistsMode(" Skip ", IfExistsSkip); err != nil || mode != IfExistsSkip {
		t.Fatalf("skip = %q, %v; want skip", mode, err)
	}
	_, err := ParseIfExistsMode("update", IfExistsSkip)
	if err == nil || !strings.Contains(err.Error(), "fail, skip") || strings.Contains(err.Error(), "update (got") {
		t.Fatalf("unsupported update = %v; want usage error listing fail, skip", err)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage-class error, got %v", err)
	}
}

func TestIsIfExistsConflictKeysOnStatusAndExactCode(t *testing.T) {
	codes := []string{"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE"}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"duplicate 409", &asc.APIError{Code: "ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE", StatusCode: http.StatusConflict}, true},
		{"case-insensitive code", &asc.APIError{Code: "entity_error.attribute.invalid.duplicate", StatusCode: http.StatusConflict}, true},
		{"state error 409", &asc.APIError{Code: "STATE_ERROR", StatusCode: http.StatusConflict}, false},
		{"relationship 409", &asc.APIError{Code: "ENTITY_ERROR.RELATIONSHIP.INVALID", StatusCode: http.StatusConflict}, false},
		{"longer code with matching prefix", &asc.APIError{Code: "ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.DIFFERENT_ACCOUNT", StatusCode: http.StatusConflict}, false},
		{"matching code but 400", &asc.APIError{Code: "ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE", StatusCode: http.StatusBadRequest}, false},
		{"plain error", errors.New("network down"), false},
	}
	for _, tc := range cases {
		if got := IsIfExistsConflict(tc.err, codes); got != tc.want {
			t.Errorf("%s: IsIfExistsConflict = %t, want %t", tc.name, got, tc.want)
		}
	}
}

func TestResolveIfExistsConflictRequiresConflictAndReadBack(t *testing.T) {
	codes := []string{"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE"}
	conflict := &asc.APIError{Code: "ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE", StatusCode: http.StatusConflict}
	stateConflict := &asc.APIError{Code: "STATE_ERROR", StatusCode: http.StatusConflict}
	other := &asc.APIError{Code: "BAD_REQUEST", StatusCode: http.StatusBadRequest}
	found := func() (string, bool, error) { return "existing", true, nil }
	missing := func() (string, bool, error) { return "", false, nil }
	notFound := func() (string, bool, error) {
		return "", false, &asc.APIError{Code: "NOT_FOUND", StatusCode: http.StatusNotFound}
	}
	broken := func() (string, bool, error) { return "", false, errors.New("network down") }
	unexpectedLookup := func() (string, bool, error) {
		t.Fatal("lookup must not run for a non-existence conflict")
		return "", false, nil
	}

	if _, handled, err := ResolveIfExistsConflict(IfExistsFail, conflict, codes, unexpectedLookup); handled || !errors.Is(err, conflict) {
		t.Fatalf("fail mode = handled %t, %v; want the conflict unchanged", handled, err)
	}
	if _, handled, err := ResolveIfExistsConflict(IfExistsSkip, other, codes, unexpectedLookup); handled || !errors.Is(err, other) {
		t.Fatalf("non-409 = handled %t, %v; want the error unchanged", handled, err)
	}
	if _, handled, err := ResolveIfExistsConflict(IfExistsSkip, stateConflict, codes, unexpectedLookup); handled || !errors.Is(err, stateConflict) {
		t.Fatalf("409 with other code = handled %t, %v; want the error unchanged without read-back", handled, err)
	}
	if existing, handled, err := ResolveIfExistsConflict(IfExistsSkip, conflict, codes, found); !handled || err != nil || existing != "existing" {
		t.Fatalf("read-back hit = %q, %t, %v; want existing", existing, handled, err)
	}
	if _, handled, err := ResolveIfExistsConflict(IfExistsUpdate, conflict, codes, missing); handled || !errors.Is(err, conflict) {
		t.Fatalf("read-back miss = handled %t, %v; want the conflict", handled, err)
	}
	if _, handled, err := ResolveIfExistsConflict(IfExistsSkip, conflict, codes, notFound); handled || !errors.Is(err, conflict) {
		t.Fatalf("read-back 404 = handled %t, %v; want the conflict", handled, err)
	}
	_, handled, err := ResolveIfExistsConflict(IfExistsSkip, conflict, codes, broken)
	if handled || !errors.Is(err, asc.ErrConflict) || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("read-back failure = handled %t, %v; want conflict with read-back cause", handled, err)
	}
}

func TestIsIfExistsConflictMatchesCodeBeyondTheFirstError(t *testing.T) {
	codes := []string{"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE"}

	// Apple's live 409 for a duplicate versionString reports the
	// not-in-this-state relationship error first and the duplicate second, so
	// matching only the first code misses the existence conflict.
	duplicateSecond := &asc.APIError{
		Code:       "ENTITY_ERROR.RELATIONSHIP.INVALID",
		StatusCode: http.StatusConflict,
		AllCodes:   []string{"ENTITY_ERROR.RELATIONSHIP.INVALID", "ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE"},
	}
	if !IsIfExistsConflict(duplicateSecond, codes) {
		t.Fatal("IsIfExistsConflict = false, want true for a duplicate code carried by a later error")
	}

	// A 409 whose every code is unrelated is still not an existence conflict.
	stateOnly := &asc.APIError{
		Code:       "ENTITY_ERROR.RELATIONSHIP.INVALID",
		StatusCode: http.StatusConflict,
		AllCodes:   []string{"ENTITY_ERROR.RELATIONSHIP.INVALID", "STATE_ERROR"},
	}
	if IsIfExistsConflict(stateOnly, codes) {
		t.Fatal("IsIfExistsConflict = true, want false when no error carries a listed code")
	}
}
