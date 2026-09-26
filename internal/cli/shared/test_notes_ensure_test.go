package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type recordedTestNotesRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   string
}

// testNotesRoute answers one recorded request. A zero status marks the request
// as unexpected so the test fails instead of silently passing.
type testNotesRoute func(request recordedTestNotesRequest) (int, string)

type testNotesRecorder struct {
	mu       sync.Mutex
	requests []recordedTestNotesRequest
}

func (r *testNotesRecorder) record(request recordedTestNotesRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, request)
}

func (r *testNotesRecorder) matching(method, path string) []recordedTestNotesRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	matches := make([]recordedTestNotesRequest, 0, len(r.requests))
	for _, request := range r.requests {
		if request.Method == method && request.Path == path {
			matches = append(matches, request)
		}
	}
	return matches
}

func (r *testNotesRecorder) count(method, path string) int {
	return len(r.matching(method, path))
}

func newTestNotesServerClient(t *testing.T, recorder *testNotesRecorder, route testNotesRoute) *asc.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		request := recordedTestNotesRequest{
			Method: req.Method,
			Path:   req.URL.Path,
			Query:  req.URL.Query(),
			Body:   string(body),
		}
		recorder.record(request)

		status, responseBody := route(request)
		if status == 0 {
			t.Errorf("unexpected request: %s %s", request.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if _, err := io.WriteString(w, responseBody); err != nil {
			t.Errorf("WriteString() error: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return newBuildUploadsServerTestClient(t, server)
}

const (
	testNotesBuildAppBody  = `{"data":{"type":"apps","id":"app-9"}}`
	testNotesNotesCreated  = `{"data":{"type":"betaBuildLocalizations","id":"bbl-1","attributes":{"locale":"en-US","whatsNew":"Check the new tab"}}}`
	testNotesAppLocCreated = `{"data":{"type":"betaAppLocalizations","id":"bal-1","attributes":{"locale":"en-US"}}}`
)

func TestUpsertBetaBuildLocalizationCreatesMissingBetaAppLocalization(t *testing.T) {
	recorder := &testNotesRecorder{}
	client := newTestNotesServerClient(t, recorder, func(request recordedTestNotesRequest) (int, string) {
		switch {
		case request.Method == http.MethodGet && request.Path == "/v1/builds/build-1/app":
			return http.StatusOK, testNotesBuildAppBody
		case request.Method == http.MethodGet && request.Path == "/v1/betaAppLocalizations":
			return http.StatusOK, `{"data":[{"type":"betaAppLocalizations","id":"bal-ja","attributes":{"locale":"ja"}}],"links":{}}`
		case request.Method == http.MethodPost && request.Path == "/v1/betaAppLocalizations":
			return http.StatusCreated, testNotesAppLocCreated
		case request.Method == http.MethodGet && request.Path == "/v1/builds/build-1/betaBuildLocalizations":
			return http.StatusOK, `{"data":[],"links":{}}`
		case request.Method == http.MethodPost && request.Path == "/v1/betaBuildLocalizations":
			return http.StatusCreated, testNotesNotesCreated
		default:
			return 0, ""
		}
	})

	resp, err := UpsertBetaBuildLocalization(
		context.Background(), client,
		"build-1", "en-US", "Check the new tab",
		UpsertBetaBuildLocalizationOptions{},
	)
	if err != nil {
		t.Fatalf("UpsertBetaBuildLocalization() error: %v", err)
	}
	if resp == nil || resp.Data.ID != "bbl-1" {
		t.Fatalf("expected the created build localization, got %#v", resp)
	}

	creates := recorder.matching(http.MethodPost, "/v1/betaAppLocalizations")
	if len(creates) != 1 {
		t.Fatalf("expected exactly one betaAppLocalizations create, got %d", len(creates))
	}
	assertBetaAppLocalizationCreateBody(t, creates[0].Body, "en-US", "app-9")

	listRequests := recorder.matching(http.MethodGet, "/v1/betaAppLocalizations")
	if len(listRequests) != 1 {
		t.Fatalf("expected exactly one betaAppLocalizations list, got %d", len(listRequests))
	}
	if got := listRequests[0].Query.Get("filter[app]"); got != "app-9" {
		t.Fatalf("list filter[app] = %q, want app-9", got)
	}
	if count := recorder.count(http.MethodPatch, "/v1/betaAppLocalizations/bal-ja"); count != 0 {
		t.Fatalf("expected no update to an existing beta app localization, got %d", count)
	}
	if count := recorder.count(http.MethodPost, "/v1/betaBuildLocalizations"); count != 1 {
		t.Fatalf("expected exactly one What to Test write, got %d", count)
	}
}

func TestUpsertBetaBuildLocalizationSkipsExistingBetaAppLocalization(t *testing.T) {
	recorder := &testNotesRecorder{}
	client := newTestNotesServerClient(t, recorder, func(request recordedTestNotesRequest) (int, string) {
		switch {
		case request.Method == http.MethodGet && request.Path == "/v1/betaAppLocalizations":
			return http.StatusOK, `{"data":[{"type":"betaAppLocalizations","id":"bal-1","attributes":{"locale":"en-us","description":"Existing"}}],"links":{}}`
		case request.Method == http.MethodGet && request.Path == "/v1/builds/build-1/betaBuildLocalizations":
			return http.StatusOK, `{"data":[],"links":{}}`
		case request.Method == http.MethodPost && request.Path == "/v1/betaBuildLocalizations":
			return http.StatusCreated, testNotesNotesCreated
		default:
			return 0, ""
		}
	})

	if _, err := UpsertBetaBuildLocalization(
		context.Background(), client,
		"build-1", "en-US", "Check the new tab",
		UpsertBetaBuildLocalizationOptions{AppID: "app-9"},
	); err != nil {
		t.Fatalf("UpsertBetaBuildLocalization() error: %v", err)
	}

	if count := recorder.count(http.MethodPost, "/v1/betaAppLocalizations"); count != 0 {
		t.Fatalf("expected no betaAppLocalizations create, got %d", count)
	}
	if count := recorder.count(http.MethodPatch, "/v1/betaAppLocalizations/bal-1"); count != 0 {
		t.Fatalf("expected no betaAppLocalizations update, got %d", count)
	}
	if count := recorder.count(http.MethodGet, "/v1/builds/build-1/app"); count != 0 {
		t.Fatalf("expected the caller-supplied app ID to skip the build app lookup, got %d requests", count)
	}
	if count := recorder.count(http.MethodPost, "/v1/betaBuildLocalizations"); count != 1 {
		t.Fatalf("expected exactly one What to Test write, got %d", count)
	}
}

func TestUpsertBetaBuildLocalizationLooksUpOnlyTheRequestedLocale(t *testing.T) {
	recorder := &testNotesRecorder{}
	client := newTestNotesServerClient(t, recorder, func(request recordedTestNotesRequest) (int, string) {
		switch {
		case request.Method == http.MethodGet && request.Path == "/v1/betaAppLocalizations":
			return http.StatusOK, `{"data":[{"type":"betaAppLocalizations","id":"bal-2","attributes":{"locale":"en-US"}}],"links":{}}`
		case request.Method == http.MethodGet && request.Path == "/v1/builds/build-1/betaBuildLocalizations":
			return http.StatusOK, `{"data":[],"links":{}}`
		case request.Method == http.MethodPost && request.Path == "/v1/betaBuildLocalizations":
			return http.StatusCreated, testNotesNotesCreated
		default:
			return 0, ""
		}
	})

	if _, err := UpsertBetaBuildLocalization(
		context.Background(), client,
		"build-1", "en-US", "Check the new tab",
		UpsertBetaBuildLocalizationOptions{AppID: "app-9"},
	); err != nil {
		t.Fatalf("UpsertBetaBuildLocalization() error: %v", err)
	}

	lists := recorder.matching(http.MethodGet, "/v1/betaAppLocalizations")
	if len(lists) != 1 {
		t.Fatalf("expected one filtered localization lookup, got %d requests", len(lists))
	}
	if got := lists[0].Query.Get("filter[locale]"); got != "en-US" {
		t.Fatalf("lookup filter[locale] = %q, want en-US", got)
	}
	if got := lists[0].Query.Get("filter[app]"); got != "app-9" {
		t.Fatalf("lookup filter[app] = %q, want app-9", got)
	}
	if count := recorder.count(http.MethodPost, "/v1/betaAppLocalizations"); count != 0 {
		t.Fatalf("expected no betaAppLocalizations create, got %d", count)
	}
}

func TestUpsertBetaBuildLocalizationStopsWhenBetaAppLocalizationCreateFails(t *testing.T) {
	recorder := &testNotesRecorder{}
	client := newTestNotesServerClient(t, recorder, func(request recordedTestNotesRequest) (int, string) {
		switch {
		case request.Method == http.MethodGet && request.Path == "/v1/betaAppLocalizations":
			return http.StatusOK, `{"data":[],"links":{}}`
		case request.Method == http.MethodPost && request.Path == "/v1/betaAppLocalizations":
			return http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"Forbidden","detail":"insufficient permission"}]}`
		default:
			return 0, ""
		}
	})

	resp, err := UpsertBetaBuildLocalization(
		context.Background(), client,
		"build-1", "en-US", "Check the new tab",
		UpsertBetaBuildLocalizationOptions{AppID: "app-9"},
	)
	if err == nil {
		t.Fatalf("expected the failed localization create to fail the write, got %#v", resp)
	}
	if resp != nil {
		t.Fatalf("expected no response when the localization create fails, got %#v", resp)
	}
	if !errors.Is(err, asc.ErrForbidden) {
		t.Fatalf("expected the API error to be preserved, got %v", err)
	}
	if count := recorder.count(http.MethodPost, "/v1/betaBuildLocalizations"); count != 0 {
		t.Fatalf("expected no What to Test write after a failed localization create, got %d", count)
	}
	if count := recorder.count(http.MethodGet, "/v1/builds/build-1/betaBuildLocalizations"); count != 0 {
		t.Fatalf("expected no What to Test read after a failed localization create, got %d", count)
	}
}

func TestUpsertBetaBuildLocalizationTreatsConcurrentCreateConflictAsEnsured(t *testing.T) {
	recorder := &testNotesRecorder{}
	client := newTestNotesServerClient(t, recorder, func(request recordedTestNotesRequest) (int, string) {
		switch {
		case request.Method == http.MethodGet && request.Path == "/v1/betaAppLocalizations":
			// The second lookup sees the localization a concurrent writer created.
			if recorder.count(http.MethodPost, "/v1/betaAppLocalizations") > 0 {
				return http.StatusOK, `{"data":[{"type":"betaAppLocalizations","id":"bal-race","attributes":{"locale":"en-US"}}],"links":{}}`
			}
			return http.StatusOK, `{"data":[],"links":{}}`
		case request.Method == http.MethodPost && request.Path == "/v1/betaAppLocalizations":
			return http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"The provided entity includes an attribute with an invalid value","detail":"There is an entity with same 'locale'"}]}`
		case request.Method == http.MethodGet && request.Path == "/v1/builds/build-1/betaBuildLocalizations":
			return http.StatusOK, `{"data":[],"links":{}}`
		case request.Method == http.MethodPost && request.Path == "/v1/betaBuildLocalizations":
			return http.StatusCreated, testNotesNotesCreated
		default:
			return 0, ""
		}
	})

	resp, err := UpsertBetaBuildLocalization(
		context.Background(), client,
		"build-1", "en-US", "Check the new tab",
		UpsertBetaBuildLocalizationOptions{AppID: "app-9"},
	)
	if err != nil {
		t.Fatalf("UpsertBetaBuildLocalization() error: %v", err)
	}
	if resp == nil || resp.Data.ID != "bbl-1" {
		t.Fatalf("expected the created build localization, got %#v", resp)
	}
	if count := recorder.count(http.MethodPost, "/v1/betaBuildLocalizations"); count != 1 {
		t.Fatalf("expected exactly one What to Test write, got %d", count)
	}
	if count := recorder.count(http.MethodGet, "/v1/betaAppLocalizations"); count != 2 {
		t.Fatalf("expected the conflict to be confirmed with a second lookup, got %d lookups", count)
	}
}

func TestUpsertBetaBuildLocalizationFailsOnConflictWhenLocaleStillMissing(t *testing.T) {
	// App Store Connect also answers 409 for an invalid locale, so a conflict
	// alone does not prove the localization exists.
	recorder := &testNotesRecorder{}
	client := newTestNotesServerClient(t, recorder, func(request recordedTestNotesRequest) (int, string) {
		switch {
		case request.Method == http.MethodGet && request.Path == "/v1/betaAppLocalizations":
			return http.StatusOK, `{"data":[],"links":{}}`
		case request.Method == http.MethodPost && request.Path == "/v1/betaAppLocalizations":
			return http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"The provided entity includes an attribute with an invalid value","detail":"The 'locale' value is invalid."}]}`
		default:
			return 0, ""
		}
	})

	var diagnostics bytes.Buffer
	resp, err := UpsertBetaBuildLocalization(
		context.Background(), client,
		"build-1", "xx-YY", "Check the new tab",
		UpsertBetaBuildLocalizationOptions{AppID: "app-9", Diagnostics: &diagnostics},
	)
	if err == nil {
		t.Fatalf("expected a non-duplicate conflict to fail the write, got %#v", resp)
	}
	if !errors.Is(err, asc.ErrConflict) {
		t.Fatalf("expected the original conflict to be preserved, got %v", err)
	}
	if !strings.Contains(err.Error(), "The 'locale' value is invalid.") {
		t.Fatalf("expected Apple's conflict detail in the error, got %v", err)
	}
	if count := recorder.count(http.MethodGet, "/v1/betaAppLocalizations"); count != 2 {
		t.Fatalf("expected the conflict to be checked with a second lookup, got %d lookups", count)
	}
	if count := recorder.count(http.MethodPost, "/v1/betaBuildLocalizations"); count != 0 {
		t.Fatalf("expected no What to Test write after an unconfirmed conflict, got %d", count)
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("expected no creation notice after a failed create, got %q", diagnostics.String())
	}
}

func TestUpsertBetaBuildLocalizationReportsCreatedBetaAppLocalization(t *testing.T) {
	recorder := &testNotesRecorder{}
	client := newTestNotesServerClient(t, recorder, func(request recordedTestNotesRequest) (int, string) {
		switch {
		case request.Method == http.MethodGet && request.Path == "/v1/betaAppLocalizations":
			return http.StatusOK, `{"data":[],"links":{}}`
		case request.Method == http.MethodPost && request.Path == "/v1/betaAppLocalizations":
			return http.StatusCreated, testNotesAppLocCreated
		case request.Method == http.MethodGet && request.Path == "/v1/builds/build-1/betaBuildLocalizations":
			return http.StatusOK, `{"data":[],"links":{}}`
		case request.Method == http.MethodPost && request.Path == "/v1/betaBuildLocalizations":
			return http.StatusCreated, testNotesNotesCreated
		default:
			return 0, ""
		}
	})

	var diagnostics bytes.Buffer
	if _, err := UpsertBetaBuildLocalization(
		context.Background(), client,
		"build-1", "en-US", "Check the new tab",
		UpsertBetaBuildLocalizationOptions{AppID: "app-9", Diagnostics: &diagnostics},
	); err != nil {
		t.Fatalf("UpsertBetaBuildLocalization() error: %v", err)
	}
	want := "Notice: created TestFlight app localization for locale \"en-US\" on app \"app-9\" so What to Test notes can be saved.\n"
	if diagnostics.String() != want {
		t.Fatalf("diagnostics = %q, want %q", diagnostics.String(), want)
	}
}

func assertBetaAppLocalizationCreateBody(t *testing.T, body, wantLocale, wantAppID string) {
	t.Helper()

	var payload struct {
		Data struct {
			Type          string         `json:"type"`
			Attributes    map[string]any `json:"attributes"`
			Relationships struct {
				App struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"app"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("parse create body %q: %v", body, err)
	}
	if payload.Data.Type != "betaAppLocalizations" {
		t.Fatalf("create type = %q, want betaAppLocalizations", payload.Data.Type)
	}
	if got := payload.Data.Relationships.App.Data.ID; got != wantAppID {
		t.Fatalf("create app relationship = %q, want %q", got, wantAppID)
	}
	if got := payload.Data.Relationships.App.Data.Type; got != "apps" {
		t.Fatalf("create app relationship type = %q, want apps", got)
	}
	if got, ok := payload.Data.Attributes["locale"].(string); !ok || got != wantLocale {
		t.Fatalf("create locale = %v, want %q", payload.Data.Attributes["locale"], wantLocale)
	}
	if len(payload.Data.Attributes) != 1 {
		keys := make([]string, 0, len(payload.Data.Attributes))
		for key := range payload.Data.Attributes {
			keys = append(keys, key)
		}
		t.Fatalf("create attributes must contain only the locale, got %s", strings.Join(keys, ","))
	}
}

func TestUpsertBetaBuildLocalizationRejectsRefusedCharactersBeforeAnyRequest(t *testing.T) {
	recorder := &testNotesRecorder{}
	client := newTestNotesServerClient(t, recorder, func(recordedTestNotesRequest) (int, string) {
		return 0, ""
	})

	var diagnostics bytes.Buffer
	resp, err := UpsertBetaBuildLocalization(
		context.Background(), client,
		"build-1", "en-US", "Roll the die \u2764\ufe0f",
		UpsertBetaBuildLocalizationOptions{Diagnostics: &diagnostics},
	)
	if err == nil {
		t.Fatalf("UpsertBetaBuildLocalization() = %#v, want a usage failure", resp)
	}
	if got := ClassifyUsageError(err); got != UsageErrorInvalidValue {
		t.Fatalf("ClassifyUsageError() = %q, want %q", got, UsageErrorInvalidValue)
	}
	recorder.mu.Lock()
	requests := len(recorder.requests)
	recorder.mu.Unlock()
	if requests != 0 {
		t.Fatalf("requests = %d, want none before the notes are accepted locally", requests)
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("diagnostics = %q, want no localization side-effect notice", diagnostics.String())
	}
}
