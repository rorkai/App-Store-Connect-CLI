package apps

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSelectLatestAppStoreVersion(t *testing.T) {
	versions := []asc.Resource[asc.AppStoreVersionAttributes]{
		{
			ID: "old",
			Attributes: asc.AppStoreVersionAttributes{
				CreatedDate: "2024-01-01T00:00:00Z",
			},
		},
		{
			ID: "new",
			Attributes: asc.AppStoreVersionAttributes{
				CreatedDate: "2025-01-01T00:00:00Z",
			},
		},
	}

	selected := selectLatestAppStoreVersion(versions)
	if selected.ID != "new" {
		t.Fatalf("expected latest version to be %q, got %q", "new", selected.ID)
	}
}

func TestSelectLatestAppStoreVersionFallsBackToFirst(t *testing.T) {
	versions := []asc.Resource[asc.AppStoreVersionAttributes]{
		{
			ID: "first",
			Attributes: asc.AppStoreVersionAttributes{
				CreatedDate: "invalid-date",
			},
		},
		{
			ID: "second",
			Attributes: asc.AppStoreVersionAttributes{
				CreatedDate: "",
			},
		},
	}

	selected := selectLatestAppStoreVersion(versions)
	if selected.ID != "first" {
		t.Fatalf("expected fallback to the first version, got %q", selected.ID)
	}
}

func TestResolveAppStoreVersionForAppInfoPaginatesBeforeSelectingLatest(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=page-2"

	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		token, ok := strings.CutPrefix(req.Header.Get("Authorization"), "Bearer ")
		if !ok || strings.TrimSpace(token) == "" {
			t.Errorf("request is missing bearer authorization: %s %s", req.Method, req.URL.String())
			http.Error(w, "missing authorization", http.StatusUnauthorized)
			return
		}
		requests = append(requests, req.Method+" "+req.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Query().Get("cursor") {
		case "":
			_, _ = io.WriteString(w, `{"data":[{"type":"appStoreVersions","id":"old","attributes":{"createdDate":"2026-01-01T00:00:00Z"}}],"links":{"next":"`+nextURL+`"}}`)
		case "page-2":
			_, _ = io.WriteString(w, `{"data":[{"type":"appStoreVersions","id":"new","attributes":{"createdDate":"2026-02-01T00:00:00Z"}}]}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	client := newAppInfoTestServerClient(t, server)
	requestCtx, cancel := shared.ContextWithTimeout(context.Background())
	defer cancel()

	selected, err := resolveAppStoreVersionForAppInfo(requestCtx, client, "app-1", "", "", nil, nil)
	if err != nil {
		t.Fatalf("resolveAppStoreVersionForAppInfo() error: %v", err)
	}
	if selected.ID != "new" {
		t.Fatalf("expected latest version %q, got %q", "new", selected.ID)
	}
	wantRequests := []string{
		"GET /v1/apps/app-1/appStoreVersions?limit=200",
		"GET /v1/apps/app-1/appStoreVersions?cursor=page-2",
	}
	if !slices.Equal(requests, wantRequests) {
		t.Fatalf("request sequence = %v, want %v", requests, wantRequests)
	}
}

func TestResolveAppStoreVersionForAppInfoRejectsPartialResults(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=page-2"

	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Query().Get("cursor") {
		case "":
			_, _ = io.WriteString(w, `{"data":[{"type":"appStoreVersions","id":"old","attributes":{"createdDate":"2026-01-01T00:00:00Z"}}],"links":{"next":"`+nextURL+`"}}`)
		case "page-2":
			_, _ = io.WriteString(w, `{`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	client := newAppInfoTestServerClient(t, server)
	requestCtx, cancel := shared.ContextWithTimeout(context.Background())
	defer cancel()

	selected, err := resolveAppStoreVersionForAppInfo(requestCtx, client, "app-1", "", "", nil, nil)
	if err == nil {
		t.Fatal("expected continuation error, got nil")
	}
	if selected.ID != "" {
		t.Fatalf("expected no version from partial results, got %q", selected.ID)
	}
	if !strings.Contains(err.Error(), "page 2") {
		t.Fatalf("expected continuation error to identify page 2, got %v", err)
	}
	wantRequests := []string{
		"GET /v1/apps/app-1/appStoreVersions?limit=200",
		"GET /v1/apps/app-1/appStoreVersions?cursor=page-2",
	}
	if !slices.Equal(requests, wantRequests) {
		t.Fatalf("request sequence = %v, want %v", requests, wantRequests)
	}
}

func TestWarnAppInfoSetSubmitIncompleteLocaleMentionsCanonicalPublishFlow(t *testing.T) {
	stderr := captureAppsCreateOutput(t, func() {
		warnAppInfoSetSubmitIncompleteLocale("en-US", asc.AppStoreVersionLocalizationAttributes{})
	})

	if !strings.Contains(stderr, "`asc publish appstore --submit`") {
		t.Fatalf("expected canonical publish guidance in warning, got %q", stderr)
	}
	if strings.Contains(stderr, "release run") {
		t.Fatalf("expected warning to avoid removed compatibility guidance, got %q", stderr)
	}
}

func TestRunAppInfoSetSingleLocaleRefetchesAndUpdatesAfterCreateConflict(t *testing.T) {
	requests := make([]string, 0)
	client := newAppInfoTestClient(t, appInfoRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch len(requests) {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected initial request: %s %s", req.Method, req.URL.String())
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":[]}`), nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.String())
			}
			return appInfoJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Conflict","detail":"localization already exists"}]}`), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected refetch request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[locale]"); got != "en-US" {
				t.Fatalf("expected refetch filter[locale]=en-US, got %q", got)
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Old description"}}]}`), nil
		case 4:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
				t.Fatalf("unexpected update request: %s %s", req.Method, req.URL.String())
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read update body: %v", err)
			}
			if !strings.Contains(string(body), `"description":"New description"`) {
				t.Fatalf("expected update body to contain new description, got %s", string(body))
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"New description"}}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	outputFormat := "json"
	pretty := false
	output := shared.OutputFlags{Output: &outputFormat, Pretty: &pretty}
	stderr := captureAppsCreateOutput(t, func() {
		err := runAppInfoSetSingleLocale(context.Background(), client, "version-1", "en-US", "", asc.AppStoreVersionLocalizationAttributes{
			Description: "New description",
		}, shared.SubmitReadinessOptions{RequireWhatsNew: true}, output)
		if err != nil {
			t.Fatalf("runAppInfoSetSingleLocale() error: %v", err)
		}
	})

	if !strings.Contains(stderr, "whatsNew") {
		t.Fatalf("expected RequireWhatsNew warning after conflict fallback, got %q", stderr)
	}
	if got, want := strings.Join(requests, ","), "GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations,POST /v1/appStoreVersionLocalizations,GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations,PATCH /v1/appStoreVersionLocalizations/loc-1"; got != want {
		t.Fatalf("request sequence = %s, want %s", got, want)
	}
}

func TestRunAppInfoSetBatchRefetchesAndUpdatesAfterCreateConflict(t *testing.T) {
	requests := make([]string, 0)
	client := newAppInfoTestClient(t, appInfoRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch len(requests) {
		case 1:
			return appInfoJSONResponse(http.StatusOK, `{"data":[]}`), nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.String())
			}
			return appInfoJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Conflict","detail":"localization already exists"}]}`), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected refetch request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[locale]"); got != "en-US" {
				t.Fatalf("expected refetch filter[locale]=en-US, got %q", got)
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Old description"}}]}`), nil
		case 4:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
				t.Fatalf("unexpected update request: %s %s", req.Method, req.URL.String())
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read update body: %v", err)
			}
			if !strings.Contains(string(body), `"description":"New description"`) {
				t.Fatalf("expected update body to contain new description, got %s", string(body))
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"New description"}}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	result, warnings, err := runAppInfoSetBatch(context.Background(), client, "app-1", "version-1", map[string]asc.AppStoreVersionLocalizationAttributes{
		"en-US": {Description: "New description"},
	}, shared.SubmitReadinessOptions{RequireWhatsNew: true}, false)
	if err != nil {
		t.Fatalf("runAppInfoSetBatch() error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %+v", warnings)
	}
	if warnings[0].Locale != "en-US" || warnings[0].Mode != shared.SubmitReadinessCreateModeApplied {
		t.Fatalf("unexpected warning identity: %+v", warnings[0])
	}
	if !slices.Equal(warnings[0].MissingFields, []string{"keywords", "supportUrl", "whatsNew"}) {
		t.Fatalf("unexpected warning missing fields: %+v", warnings[0].MissingFields)
	}
	if result == nil || result.Failed != 0 || result.Succeeded != 1 {
		t.Fatalf("unexpected batch result: %+v", result)
	}
	if len(result.Results) != 1 || result.Results[0].Action != "update" || result.Results[0].LocalizationID != "loc-1" {
		t.Fatalf("unexpected locale result: %+v", result.Results)
	}
}

func TestRunAppInfoSetBatchConflictWarningsUseRefetchedFields(t *testing.T) {
	requests := make([]string, 0)
	client := newAppInfoTestClient(t, appInfoRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch len(requests) {
		case 1:
			return appInfoJSONResponse(http.StatusOK, `{"data":[]}`), nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.String())
			}
			return appInfoJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Conflict","detail":"localization already exists"}]}`), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected refetch request: %s %s", req.Method, req.URL.String())
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Existing description","keywords":"existing,keywords","supportUrl":"https://example.com/support"}}]}`), nil
		case 4:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
				t.Fatalf("unexpected update request: %s %s", req.Method, req.URL.String())
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read update body: %v", err)
			}
			if !strings.Contains(string(body), `"whatsNew":"Bug fixes"`) {
				t.Fatalf("expected update body to contain whatsNew, got %s", string(body))
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Bug fixes"}}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	result, warnings, err := runAppInfoSetBatch(context.Background(), client, "app-1", "version-1", map[string]asc.AppStoreVersionLocalizationAttributes{
		"en-US": {WhatsNew: "Bug fixes"},
	}, shared.SubmitReadinessOptions{RequireWhatsNew: true}, false)
	if err != nil {
		t.Fatalf("runAppInfoSetBatch() error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings from refetched complete fields, got %+v", warnings)
	}
	if result == nil || result.Failed != 0 || result.Succeeded != 1 {
		t.Fatalf("unexpected batch result: %+v", result)
	}
}

func TestRunAppInfoSetSingleLocaleConflictRechecksCopyFromAfterRefetch(t *testing.T) {
	requests := make([]string, 0)
	client := newAppInfoTestClient(t, appInfoRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch len(requests) {
		case 1:
			if got := req.URL.Query().Get("filter[locale]"); got != "en-US,fr-FR" {
				t.Fatalf("expected initial filter[locale]=en-US,fr-FR, got %q", got)
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-source","attributes":{"locale":"fr-FR","description":"Source description","keywords":"source,keywords","supportUrl":"https://example.com/source"}}]}`), nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.String())
			}
			return appInfoJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Conflict","detail":"localization already exists"}]}`), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected refetch request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[locale]"); got != "en-US" {
				t.Fatalf("expected refetch filter[locale]=en-US, got %q", got)
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Target description","keywords":"target,keywords","supportUrl":"https://example.com/target"}}]}`), nil
		case 4:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
				t.Fatalf("unexpected update request: %s %s", req.Method, req.URL.String())
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read update body: %v", err)
			}
			bodyString := string(body)
			if !strings.Contains(bodyString, `"whatsNew":"Bug fixes"`) {
				t.Fatalf("expected update body to contain explicit whatsNew, got %s", bodyString)
			}
			if strings.Contains(bodyString, "Source description") || strings.Contains(bodyString, "source,keywords") || strings.Contains(bodyString, "https://example.com/source") {
				t.Fatalf("copy-from values should not overwrite refetched target fields, got %s", bodyString)
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Bug fixes"}}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	outputFormat := "json"
	pretty := false
	output := shared.OutputFlags{Output: &outputFormat, Pretty: &pretty}
	captureAppsCreateOutput(t, func() {
		err := runAppInfoSetSingleLocale(context.Background(), client, "version-1", "en-US", "fr-FR", asc.AppStoreVersionLocalizationAttributes{
			WhatsNew: "Bug fixes",
		}, shared.SubmitReadinessOptions{RequireWhatsNew: true}, output)
		if err != nil {
			t.Fatalf("runAppInfoSetSingleLocale() error: %v", err)
		}
	})

	if got, want := strings.Join(requests, ","), "GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations,POST /v1/appStoreVersionLocalizations,GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations,PATCH /v1/appStoreVersionLocalizations/loc-1"; got != want {
		t.Fatalf("request sequence = %s, want %s", got, want)
	}
}

func TestRunAppInfoSetSingleLocaleConflictBackfillsMissingRefetchedTarget(t *testing.T) {
	requests := make([]string, 0)
	client := newAppInfoTestClient(t, appInfoRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch len(requests) {
		case 1:
			if got := req.URL.Query().Get("filter[locale]"); got != "en-US,fr-FR" {
				t.Fatalf("expected initial filter[locale]=en-US,fr-FR, got %q", got)
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-source","attributes":{"locale":"fr-FR","description":"Source description","keywords":"source,keywords","supportUrl":"https://example.com/source"}}]}`), nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.String())
			}
			return appInfoJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Conflict","detail":"localization already exists"}]}`), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected refetch request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[locale]"); got != "en-US" {
				t.Fatalf("expected refetch filter[locale]=en-US, got %q", got)
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`), nil
		case 4:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
				t.Fatalf("unexpected update request: %s %s", req.Method, req.URL.String())
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read update body: %v", err)
			}
			bodyString := string(body)
			for _, want := range []string{`"description":"Source description"`, `"keywords":"source,keywords"`, `"supportUrl":"https://example.com/source"`, `"whatsNew":"Bug fixes"`} {
				if !strings.Contains(bodyString, want) {
					t.Fatalf("expected update body to contain %s, got %s", want, bodyString)
				}
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Source description","keywords":"source,keywords","supportUrl":"https://example.com/source","whatsNew":"Bug fixes"}}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	outputFormat := "json"
	pretty := false
	output := shared.OutputFlags{Output: &outputFormat, Pretty: &pretty}
	captureAppsCreateOutput(t, func() {
		err := runAppInfoSetSingleLocale(context.Background(), client, "version-1", "en-US", "fr-FR", asc.AppStoreVersionLocalizationAttributes{
			WhatsNew: "Bug fixes",
		}, shared.SubmitReadinessOptions{RequireWhatsNew: true}, output)
		if err != nil {
			t.Fatalf("runAppInfoSetSingleLocale() error: %v", err)
		}
	})

	if got, want := strings.Join(requests, ","), "GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations,POST /v1/appStoreVersionLocalizations,GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations,PATCH /v1/appStoreVersionLocalizations/loc-1"; got != want {
		t.Fatalf("request sequence = %s, want %s", got, want)
	}
}

func TestRunAppInfoSetSingleLocaleConflictBackfillsBlankRefetchedTarget(t *testing.T) {
	requests := make([]string, 0)
	client := newAppInfoTestClient(t, appInfoRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch len(requests) {
		case 1:
			if got := req.URL.Query().Get("filter[locale]"); got != "en-US,fr-FR" {
				t.Fatalf("expected initial filter[locale]=en-US,fr-FR, got %q", got)
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-source","attributes":{"locale":"fr-FR","description":"Source description","keywords":"source,keywords","supportUrl":"https://example.com/source"}}]}`), nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.String())
			}
			return appInfoJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Conflict","detail":"localization already exists"}]}`), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected refetch request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[locale]"); got != "en-US" {
				t.Fatalf("expected refetch filter[locale]=en-US, got %q", got)
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"","keywords":"","supportUrl":""}}]}`), nil
		case 4:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
				t.Fatalf("unexpected update request: %s %s", req.Method, req.URL.String())
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read update body: %v", err)
			}
			bodyString := string(body)
			for _, want := range []string{
				`"description":"Source description"`,
				`"keywords":"source,keywords"`,
				`"supportUrl":"https://example.com/source"`,
				`"whatsNew":"Bug fixes"`,
			} {
				if !strings.Contains(bodyString, want) {
					t.Fatalf("expected update body to contain %s, got %s", want, bodyString)
				}
			}
			return appInfoJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Source description","keywords":"source,keywords","supportUrl":"https://example.com/source","whatsNew":"Bug fixes"}}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	outputFormat := "json"
	pretty := false
	output := shared.OutputFlags{Output: &outputFormat, Pretty: &pretty}
	captureAppsCreateOutput(t, func() {
		err := runAppInfoSetSingleLocale(context.Background(), client, "version-1", "en-US", "fr-FR", asc.AppStoreVersionLocalizationAttributes{
			WhatsNew: "Bug fixes",
		}, shared.SubmitReadinessOptions{RequireWhatsNew: true}, output)
		if err != nil {
			t.Fatalf("runAppInfoSetSingleLocale() error: %v", err)
		}
	})

	if got, want := strings.Join(requests, ","), "GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations,POST /v1/appStoreVersionLocalizations,GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations,PATCH /v1/appStoreVersionLocalizations/loc-1"; got != want {
		t.Fatalf("request sequence = %s, want %s", got, want)
	}
}

type appInfoRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn appInfoRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func appInfoJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newAppInfoTestClient(t *testing.T, transport http.RoundTripper) *asc.Client {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func newAppInfoTestServerClient(t *testing.T, server *httptest.Server) *asc.Client {
	t.Helper()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := appInfoRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "api.appstoreconnect.apple.com" {
			return nil, fmt.Errorf("unexpected App Store Connect URL %q", req.URL.String())
		}
		routed := req.Clone(req.Context())
		routed.URL.Scheme = serverURL.Scheme
		routed.URL.Host = serverURL.Host
		routed.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(routed)
	})
	return newAppInfoTestClient(t, transport)
}
