package shared

import (
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

func TestUpsertBetaBuildLocalizationFindsExistingLocaleOnLaterPage(t *testing.T) {
	recorder := &testNotesRecorder{}
	client := newTestNotesServerClient(t, recorder, func(request recordedTestNotesRequest) (int, string) {
		switch {
		case request.Method == http.MethodGet && request.Path == "/v1/betaAppLocalizations":
			if request.Query.Get("cursor") == "page-2" {
				return http.StatusOK, `{"data":[{"type":"betaAppLocalizations","id":"bal-2","attributes":{"locale":"en-US"}}],"links":{}}`
			}
			return http.StatusOK, `{"data":[{"type":"betaAppLocalizations","id":"bal-1","attributes":{"locale":"ja"}}],"links":{"next":"/v1/betaAppLocalizations?cursor=page-2"}}`
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

	if count := recorder.count(http.MethodGet, "/v1/betaAppLocalizations"); count != 2 {
		t.Fatalf("expected both localization pages to be read, got %d requests", count)
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
			return http.StatusOK, `{"data":[],"links":{}}`
		case request.Method == http.MethodPost && request.Path == "/v1/betaAppLocalizations":
			return http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"Conflict","detail":"The locale already exists."}]}`
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
