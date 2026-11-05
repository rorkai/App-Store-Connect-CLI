package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
)

func TestNormalizeCreateAttrsDefaults(t *testing.T) {
	attrs, err := normalizeCreateAttrs(AppCreateAttributes{
		Name:     "My App",
		BundleID: "com.example.app",
		SKU:      "SKU123",
	})
	if err != nil {
		t.Fatalf("normalizeCreateAttrs error: %v", err)
	}
	if attrs.PrimaryLocale != defaultPrimaryLocale {
		t.Fatalf("expected default locale %q, got %q", defaultPrimaryLocale, attrs.PrimaryLocale)
	}
	if attrs.Platform != defaultPlatform {
		t.Fatalf("expected default platform %q, got %q", defaultPlatform, attrs.Platform)
	}
	if attrs.VersionString != defaultVersion {
		t.Fatalf("expected default version %q, got %q", defaultVersion, attrs.VersionString)
	}
}

func TestRestoreAndPermissionCapturedContracts(t *testing.T) {
	for _, tc := range []struct{ name, access, operation string }{
		{"full", "full", "GRANT"},
		{"limited", "limited", "REVOKE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := handlertest.New(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				body, _ := io.ReadAll(r.Body)
				if calls == 1 {
					if r.Method != http.MethodPatch || r.URL.Path != "/apps/123" || string(body) != `{"data":{"attributes":{"removed":false},"id":"123","type":"apps"}}` {
						fixture.Respond(w, "bad patch")
						return
					}
					_, _ = w.Write([]byte(`{"data":{"id":"123","type":"apps"}}`))
					return
				}
				want := fmt.Sprintf(`{"data":{"attributes":{"appAdamId":"123","operationType":"%s","userOperationType":"ALL_SILOABLE_USERS"},"type":"userAppPermissions"}}`, tc.operation)
				if r.Method != http.MethodPost || r.URL.Path != "/userAppPermissions" || string(body) != want {
					fixture.Respond(w, "bad permission")
					return
				}
				_, _ = w.Write([]byte(`{"data":{}}`))
			}))
			defer server.Close()
			c := &Client{httpClient: server.Client(), baseURL: server.URL}
			if _, err := c.RestoreApp(context.Background(), "123"); err != nil {
				t.Fatal(err)
			}
			if err := c.SetUserAppPermission(context.Background(), "123", tc.access); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSetUserAppPermissionRejectsInvalid(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	c := &Client{httpClient: server.Client(), baseURL: server.URL}
	for _, tc := range []struct{ appID, access, want string }{
		{"", "full", "app id is required"},
		{"123", "other", "access must be limited or full"},
	} {
		if err := c.SetUserAppPermission(context.Background(), tc.appID, tc.access); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("expected %q, got %v", tc.want, err)
		}
	}
	if requests != 0 {
		t.Fatalf("invalid arguments sent %d requests", requests)
	}
}

func TestNormalizeCreateAttrsRejectsInvalidPlatform(t *testing.T) {
	_, err := normalizeCreateAttrs(AppCreateAttributes{
		Name:     "My App",
		BundleID: "com.example.app",
		SKU:      "SKU123",
		Platform: "WATCH_OS",
	})
	if err == nil {
		t.Fatal("expected invalid platform error")
	}
}

// frozenAppCreateRequestBody is the captured POST /apps contract without
// access-level or user-selection fields. Apple's public New App form fields for
// Full/Limited access are not present in this snapshot; do not add guessed
// names to the create body.
const frozenAppCreateRequestBody = `{"data":{"type":"apps","attributes":{"sku":"SKU123","primaryLocale":"en-US","bundleId":"com.example.app"},"relationships":{"appStoreVersions":{"data":[{"type":"appStoreVersions","id":"${new-appStoreVersion}"}]},"appInfos":{"data":[{"type":"appInfos","id":"${new-appInfo}"}]}}},"included":[{"type":"appStoreVersions","id":"${new-appStoreVersion}","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"appStoreVersionLocalizations":{"data":[{"type":"appStoreVersionLocalizations","id":"${new-appStoreVersionLocalization}"}]}}},{"type":"appStoreVersionLocalizations","id":"${new-appStoreVersionLocalization}","attributes":{"locale":"en-US"}},{"type":"appInfos","id":"${new-appInfo}","relationships":{"appInfoLocalizations":{"data":[{"type":"appInfoLocalizations","id":"${new-appInfoLocalization}"}]}}},{"type":"appInfoLocalizations","id":"${new-appInfoLocalization}","attributes":{"locale":"en-US","name":"My App"}}]}`

func fixtureAppCreateAttributes() AppCreateAttributes {
	return AppCreateAttributes{
		Name:          "My App",
		BundleID:      "com.example.app",
		SKU:           "SKU123",
		PrimaryLocale: "en-US",
		Platform:      "IOS",
		VersionString: "1.0",
	}
}

func TestCreateAppSendsCapturedContractBody(t *testing.T) {
	fixture := handlertest.New(t)
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/apps" {
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.Path)
			return
		}
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"app-123","type":"apps","attributes":{}}}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}
	if _, err := client.CreateApp(context.Background(), fixtureAppCreateAttributes()); err != nil {
		t.Fatalf("CreateApp error: %v", err)
	}
	if gotBody != frozenAppCreateRequestBody {
		t.Fatalf("create request body changed\ngot:  %s\nwant: %s", gotBody, frozenAppCreateRequestBody)
	}
}

func TestFindAppEscapesBundleIDQuery(t *testing.T) {
	var gotRawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}
	_, err := client.FindApp(context.Background(), "com.example/app?x=1")
	if err != nil {
		t.Fatalf("FindApp error: %v", err)
	}
	if strings.Contains(gotRawQuery, "com.example/app?x=1") {
		t.Fatalf("expected escaped query value, got raw query %q", gotRawQuery)
	}
	if !strings.Contains(gotRawQuery, "filter%5BbundleId%5D=") && !strings.Contains(gotRawQuery, "filter[bundleId]=") {
		t.Fatalf("expected bundleId filter query, got %q", gotRawQuery)
	}
}

func TestDeleteAppSendsRemovedPatch(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"1234567890","attributes":{"name":"Throwaway","bundleId":"com.example.throwaway","removed":true}}}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}
	app, err := client.DeleteApp(context.Background(), "1234567890")
	if err != nil {
		t.Fatalf("DeleteApp error: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("expected PATCH, got %s", gotMethod)
	}
	if gotPath != "/apps/1234567890" {
		t.Fatalf("expected /apps/1234567890, got %s", gotPath)
	}
	for _, want := range []string{
		`"type":"apps"`,
		`"id":"1234567890"`,
		`"removed":true`,
	} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("expected request body to contain %s, got %s", want, gotBody)
		}
	}
	if app == nil || app.Data.ID != "1234567890" {
		t.Fatalf("expected app response id, got %+v", app)
	}
}

func TestDeleteAppEmptyResponseDoesNotSynthesizeID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}
	app, err := client.DeleteApp(context.Background(), "1234567890")
	if err != nil {
		t.Fatalf("DeleteApp error: %v", err)
	}
	if app == nil {
		t.Fatal("expected parsed response")
	}
	if strings.TrimSpace(app.Data.ID) != "" {
		t.Fatalf("empty PATCH payload must not synthesize data.id from the request, got %q", app.Data.ID)
	}
}

func TestGetAppRemovalStateRequestsCapturedFields(t *testing.T) {
	var gotPath, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{
			"data": {
				"type": "apps",
				"id": "1234567890",
				"attributes": {
					"name": "Throwaway",
					"bundleId": "com.example.throwaway",
					"removed": false,
					"appStoreLegacyStatus": "PREPARE_FOR_SUBMISSION",
					"marketplace": "APP_STORE"
				},
				"relationships": {
					"displayableVersions": {
						"data": [{"type": "appStoreVersions", "id": "version-1"}]
					}
				}
			},
			"included": [{
				"type": "appStoreVersions",
				"id": "version-1",
				"attributes": {
					"platform": "IOS",
					"versionString": "1.0",
					"appStoreState": "PREPARE_FOR_SUBMISSION"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}
	state, err := client.GetAppRemovalState(context.Background(), "1234567890")
	if err != nil {
		t.Fatalf("GetAppRemovalState error: %v", err)
	}
	if gotPath != "/apps/1234567890" {
		t.Fatalf("expected /apps/1234567890, got %s", gotPath)
	}
	if !strings.Contains(gotQuery, "removed") || !strings.Contains(gotQuery, "appStoreLegacyStatus") || !strings.Contains(gotQuery, "marketplace") {
		t.Fatalf("expected captured app fields in query, got %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "displayableVersions") {
		t.Fatalf("expected displayableVersions include, got %q", gotQuery)
	}
	if state.ID != "1234567890" || state.Name != "Throwaway" || state.BundleID != "com.example.throwaway" {
		t.Fatalf("unexpected state identity: %+v", state)
	}
	if state.Removed || !state.RemovedKnown {
		t.Fatalf("expected removed=false from server, got %+v", state)
	}
	if state.AppStoreLegacyStatus != "PREPARE_FOR_SUBMISSION" || state.Marketplace != "APP_STORE" {
		t.Fatalf("unexpected status fields: %+v", state)
	}
	if len(state.VersionStates) != 1 || state.VersionStates[0] != "PREPARE_FOR_SUBMISSION" {
		t.Fatalf("unexpected version states: %+v", state.VersionStates)
	}
	if !state.DisplayableVersionsLoaded {
		t.Fatal("expected displayableVersions linkage to be complete")
	}
}

func TestGetAppRemovalStatePreservesBothVersionStateFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{
			"data": {
				"type": "apps",
				"id": "1234567890",
				"attributes": {
					"name": "Throwaway",
					"bundleId": "com.example.throwaway",
					"removed": false,
					"appStoreLegacyStatus": "PREPARE_FOR_SUBMISSION",
					"marketplace": "APP_STORE"
				},
				"relationships": {
					"displayableVersions": {
						"data": [{"type": "appStoreVersions", "id": "version-1"}]
					}
				}
			},
			"included": [{
				"type": "appStoreVersions",
				"id": "version-1",
				"attributes": {
					"platform": "IOS",
					"versionString": "1.0",
					"appStoreState": "READY_FOR_SALE",
					"appVersionState": "WAITING_FOR_REVIEW"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}
	state, err := client.GetAppRemovalState(context.Background(), "1234567890")
	if err != nil {
		t.Fatalf("GetAppRemovalState error: %v", err)
	}
	got := strings.Join(state.VersionStates, ",")
	if !strings.Contains(got, "READY_FOR_SALE") {
		t.Fatalf("expected appStoreState READY_FOR_SALE, got %+v", state.VersionStates)
	}
	if !strings.Contains(got, "WAITING_FOR_REVIEW") {
		t.Fatalf("expected appVersionState WAITING_FOR_REVIEW, got %+v", state.VersionStates)
	}
}

func TestGetAppRemovalStateLeavesVersionsUnloadedWithoutCompleteVersionData(t *testing.T) {
	const appData = `"type": "apps",
		"id": "1234567890",
		"attributes": {
			"name": "Throwaway",
			"bundleId": "com.example.throwaway",
			"removed": false,
			"appStoreLegacyStatus": "PREPARE_FOR_SUBMISSION",
			"marketplace": "APP_STORE"
		}`
	const relationships = `,
		"relationships": {
			"displayableVersions": {
				"data": [{"type": "appStoreVersions", "id": "version-1"}]
			}
		}`
	tests := []struct {
		name string
		body string
	}{
		{name: "relationship omitted", body: `{"data": {` + appData + `}}`},
		{name: "include missing", body: `{"data": {` + appData + relationships + `}}`},
		{name: "state fields missing", body: `{"data": {` + appData + relationships + `},
			"included": [{
				"type": "appStoreVersions",
				"id": "version-1",
				"attributes": {"platform": "IOS", "versionString": "1.0"}
			}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			client := &Client{httpClient: server.Client(), baseURL: server.URL}
			state, err := client.GetAppRemovalState(context.Background(), "1234567890")
			if err != nil {
				t.Fatalf("GetAppRemovalState error: %v", err)
			}
			if state.DisplayableVersionsLoaded {
				t.Fatal("incomplete displayableVersions data must not count as loaded")
			}
		})
	}
}

func TestGetAppRemovalStateRequiresMatchingID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{
			"data": {
				"type": "apps",
				"id": "9999999999",
				"attributes": {
					"name": "Other",
					"bundleId": "com.example.other",
					"removed": false
				}
			}
		}`))
	}))
	defer server.Close()

	client := &Client{httpClient: server.Client(), baseURL: server.URL}
	_, err := client.GetAppRemovalState(context.Background(), "1234567890")
	if err == nil {
		t.Fatal("expected mismatched app id error")
	}
	if !strings.Contains(err.Error(), "1234567890") || !strings.Contains(err.Error(), "9999999999") {
		t.Fatalf("expected both ids in error, got %v", err)
	}
}

func TestGetAppRemovalStateRequiresID(t *testing.T) {
	client := &Client{}
	if _, err := client.GetAppRemovalState(context.Background(), "  "); err == nil {
		t.Fatal("expected missing app id error")
	}
}

func TestDeleteAppRequiresID(t *testing.T) {
	client := &Client{}
	if _, err := client.DeleteApp(context.Background(), "  "); err == nil {
		t.Fatal("expected missing app id error")
	}
}

func TestListRemovedAppsUsesRemovedFilterAndDecodesVersions(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/iris/v1/apps" {
			t.Fatalf("expected /iris/v1/apps, got %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		query := r.URL.Query()
		if query.Get("filter[removed]") != "true" {
			t.Fatalf("expected removed filter, got query %q", r.URL.RawQuery)
		}
		if query.Get("limit") != "48" {
			t.Fatalf("expected limit=48, got query %q", r.URL.RawQuery)
		}
		if query.Get("include") != "appStoreIcon,displayableVersions" {
			t.Fatalf("expected displayable versions include, got query %q", r.URL.RawQuery)
		}
		if !strings.Contains(query.Get("fields[apps]"), "removed") {
			t.Fatalf("expected removed app fields, got query %q", r.URL.RawQuery)
		}
		if query.Get("limit[displayableVersions]") != "20" {
			t.Fatalf("expected displayable version limit, got query %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"type": "apps",
				"id": "1234567890",
				"attributes": {
					"name": "Throwaway",
					"bundleId": "com.example.throwaway",
					"primaryLocale": "en-US",
					"sku": "THROWAWAY",
					"removed": true,
					"appStoreLegacyStatus": "PREPARE_FOR_SUBMISSION",
					"marketplace": "APP_STORE"
				},
				"relationships": {
					"displayableVersions": {
						"data": [{"type": "appStoreVersions", "id": "version-1"}]
					}
				}
			}],
			"included": [{
				"type": "appStoreVersions",
				"id": "version-1",
				"attributes": {
					"platform": "IOS",
					"versionString": "1.0",
					"appStoreState": "PREPARE_FOR_SUBMISSION",
					"createdDate": "2026-07-06T10:00:00Z",
					"isWatchOnly": false
				}
			}],
			"links": {"self": "https://appstoreconnect.apple.com/iris/v1/apps?filter[removed]=true"}
		}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL + "/iris/v1",
	}
	result, err := client.ListRemovedApps(context.Background(), RemovedAppsListOptions{Limit: 48})
	if err != nil {
		t.Fatalf("ListRemovedApps error: %v", err)
	}
	if !strings.Contains(gotQuery, "filter%5Bremoved%5D=true") && !strings.Contains(gotQuery, "filter[removed]=true") {
		t.Fatalf("expected removed filter in raw query, got %q", gotQuery)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 app, got %d", len(result.Data))
	}
	app := result.Data[0]
	if app.ID != "1234567890" || app.Name != "Throwaway" || app.BundleID != "com.example.throwaway" || !app.Removed {
		t.Fatalf("unexpected app result: %+v", app)
	}
	if app.Status != "PREPARE_FOR_SUBMISSION" {
		t.Fatalf("expected displayable version status, got %q", app.Status)
	}
	if len(app.DisplayableVersions) != 1 {
		t.Fatalf("expected 1 displayable version, got %d", len(app.DisplayableVersions))
	}
	version := app.DisplayableVersions[0]
	if version.ID != "version-1" || version.Platform != "IOS" || version.VersionString != "1.0" {
		t.Fatalf("unexpected version result: %+v", version)
	}
}

func TestListRemovedAppsPaginatesNextLinks(t *testing.T) {
	requests := 0
	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch requests {
		case 1:
			if r.URL.Query().Get("filter[removed]") != "true" {
				t.Fatalf("expected first page to use removed filter, got query %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{
				"data": [{
					"type": "apps",
					"id": "app-1",
					"attributes": {"name": "First", "removed": true}
				}],
				"links": {"next": "` + serverURL + `/iris/v1/apps?filter[removed]=true&page=2"}
			}`))
		case 2:
			if r.URL.Path != "/iris/v1/apps" || r.URL.Query().Get("filter[removed]") != "true" || r.URL.Query().Get("page") != "2" {
				t.Fatalf("expected second request to follow next link, got %s?%s", r.URL.Path, r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{
				"data": [{
					"type": "apps",
					"id": "app-2",
					"attributes": {"name": "Second", "removed": true}
				}],
				"links": {}
			}`))
		default:
			t.Fatalf("unexpected request %d", requests)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL + "/iris/v1",
	}
	result, err := client.ListRemovedApps(context.Background(), RemovedAppsListOptions{Limit: 48, Paginate: true})
	if err != nil {
		t.Fatalf("ListRemovedApps error: %v", err)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(result.Data))
	}
	if result.Data[0].ID != "app-1" || result.Data[1].ID != "app-2" {
		t.Fatalf("unexpected app ids: %+v", result.Data)
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests, got %d", requests)
	}
}

func TestListRemovedAppsRejectsNextWithoutRemovedFilter(t *testing.T) {
	client := &Client{
		httpClient: http.DefaultClient,
		baseURL:    "https://appstoreconnect.apple.com/iris/v1",
	}
	_, err := client.ListRemovedApps(context.Background(), RemovedAppsListOptions{
		Next: "https://appstoreconnect.apple.com/iris/v1/apps?limit=48",
	})
	if err == nil {
		t.Fatal("expected missing removed filter error")
	}
	if !strings.Contains(err.Error(), "filter[removed]=true") {
		t.Fatalf("expected removed filter error, got %v", err)
	}
}
