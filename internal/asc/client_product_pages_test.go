package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestGetAppCustomProductPages_SendsQuerySurface(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/appCustomProductPages" {
			t.Fatalf("expected custom product pages path, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		want := map[string]string{
			"filter[visible]":                      "false,true",
			"fields[appCustomProductPages]":        "name,visible,app,appCustomProductPageVersions",
			"fields[apps]":                         "name,bundleId",
			"fields[appCustomProductPageVersions]": "version,state",
			"include":                              "app,appCustomProductPageVersions",
			"limit[appCustomProductPageVersions]":  "25",
			"limit":                                "10",
		}
		for key, expected := range want {
			if got := query.Get(key); got != expected {
				t.Fatalf("query[%q] = %q, want %q (full query %s)", key, got, expected, query.Encode())
			}
		}
	}, jsonResponse(http.StatusOK, `{"data":[{"type":"appCustomProductPages","id":"page-1"}]}`))

	_, err := client.GetAppCustomProductPages(
		context.Background(),
		"app-1",
		WithAppCustomProductPagesVisible([]string{"false", "true"}),
		WithAppCustomProductPagesFields([]string{"name", "visible", "app"}),
		WithAppCustomProductPagesAppFields([]string{"name", "bundleId"}),
		WithAppCustomProductPagesVersionFields([]string{"version", "state"}),
		WithAppCustomProductPagesInclude([]string{"app", "appCustomProductPageVersions"}),
		WithAppCustomProductPagesVersionsLimit(25),
		Limit[AppCustomProductPagesOption](10),
	)
	if err != nil {
		t.Fatalf("GetAppCustomProductPages() error: %v", err)
	}
}

func TestCreateAppCustomProductPage_SendsRequest(t *testing.T) {
	appResponse := jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}}}`)
	createResponse := jsonResponse(http.StatusCreated, `{"data":{"type":"appCustomProductPages","id":"page-1","attributes":{"name":"Summer"}}}`)
	sawAppGet := false
	client := newTestClient(t, func(req *http.Request) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1" {
			sawAppGet = true
			assertAuthorized(t, req)
			return
		}
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCustomProductPages" {
			t.Fatalf("expected path /v1/appCustomProductPages, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AppCustomProductPageCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppCustomProductPages {
			t.Fatalf("expected type appCustomProductPages, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Summer" {
			t.Fatalf("expected name Summer, got %q", payload.Data.Attributes.Name)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship")
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app ID app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		versions := payload.Data.Relationships.AppCustomProductPageVersions.Data
		if len(versions) != 1 {
			t.Fatalf("expected one inline appCustomProductPageVersions linkage, got %#v", versions)
		}
		version := versions[0]
		if version.Type != ResourceTypeAppCustomProductPageVersions || version.ID != "${new-appCustomProductPageVersion-id}" {
			t.Fatalf("unexpected inline version linkage: %#v", version)
		}
		if len(payload.Included) != 2 {
			t.Fatalf("expected an included version and localization, got %#v", payload.Included)
		}
		includedVersion := payload.Included[0]
		if includedVersion.Type != version.Type || includedVersion.ID != version.ID {
			t.Fatalf("included version does not match relationship linkage: %#v", includedVersion)
		}
		if includedVersion.Relationships == nil {
			t.Fatal("expected inline version relationships")
		}
		localizations := includedVersion.Relationships.AppCustomProductPageLocalizations.Data
		if len(localizations) != 1 {
			t.Fatalf("expected one inline localization linkage, got %#v", localizations)
		}
		localization := localizations[0]
		if localization.Type != ResourceTypeAppCustomProductPageLocalizations || localization.ID != "${new-appCustomProductPageLocalization-id}" {
			t.Fatalf("unexpected inline localization linkage: %#v", localization)
		}
		includedLocalization := payload.Included[1]
		if includedLocalization.Type != localization.Type || includedLocalization.ID != localization.ID {
			t.Fatalf("included localization does not match relationship linkage: %#v", includedLocalization)
		}
		if includedLocalization.Attributes == nil || includedLocalization.Attributes.Locale != "en-US" {
			t.Fatalf("expected localization to use app primary locale en-US, got %#v", includedLocalization.Attributes)
		}
		assertAuthorized(t, req)
	}, appResponse, createResponse)

	if _, err := client.CreateAppCustomProductPage(context.Background(), "app-1", "Summer", AppCustomProductPageTemplate{}); err != nil {
		t.Fatalf("CreateAppCustomProductPage() error: %v", err)
	}
	if !sawAppGet {
		t.Fatal("expected app primary locale lookup")
	}
}

func TestCreateAppCustomProductPage_RequiresAppPrimaryLocale(t *testing.T) {
	appResponse := jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1" {
			t.Fatalf("expected app lookup, got %s %s", req.Method, req.URL.Path)
		}
		assertAuthorized(t, req)
	}, appResponse)

	_, err := client.CreateAppCustomProductPage(context.Background(), "app-1", "Summer", AppCustomProductPageTemplate{})
	if err == nil || err.Error() != "app primary locale is required" {
		t.Fatalf("expected app primary locale error, got %v", err)
	}
}

func TestUpdateAppCustomProductPage_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appCustomProductPages","id":"page-1","attributes":{"name":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCustomProductPages/page-1" {
			t.Fatalf("expected path /v1/appCustomProductPages/page-1, got %s", req.URL.Path)
		}
		var payload AppCustomProductPageUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppCustomProductPages {
			t.Fatalf("expected type appCustomProductPages, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil {
			t.Fatalf("expected name attribute")
		}
		if *payload.Data.Attributes.Name != "Updated" {
			t.Fatalf("expected name Updated, got %q", *payload.Data.Attributes.Name)
		}
		if payload.Data.Attributes.Visible == nil || *payload.Data.Attributes.Visible != true {
			t.Fatalf("expected visible true")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppCustomProductPageUpdateAttributes{
		Name:    new("Updated"),
		Visible: new(true),
	}
	if _, err := client.UpdateAppCustomProductPage(context.Background(), "page-1", attrs); err != nil {
		t.Fatalf("UpdateAppCustomProductPage() error: %v", err)
	}
}

func TestCreateAppCustomProductPageVersion_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appCustomProductPageVersions","id":"version-1","attributes":{"version":"1.0"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCustomProductPageVersions" {
			t.Fatalf("expected path /v1/appCustomProductPageVersions, got %s", req.URL.Path)
		}
		var payload AppCustomProductPageVersionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppCustomProductPageVersions {
			t.Fatalf("expected type appCustomProductPageVersions, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.DeepLink != "https://example.com/deeplink" {
			t.Fatalf("expected deepLink value, got %q", payload.Data.Attributes.DeepLink)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppCustomProductPage == nil {
			t.Fatalf("expected appCustomProductPage relationship")
		}
		if payload.Data.Relationships.AppCustomProductPage.Data.ID != "page-1" {
			t.Fatalf("expected custom page ID page-1, got %q", payload.Data.Relationships.AppCustomProductPage.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppCustomProductPageVersion(context.Background(), "page-1", "https://example.com/deeplink"); err != nil {
		t.Fatalf("CreateAppCustomProductPageVersion() error: %v", err)
	}
}

func TestUpdateAppCustomProductPageVersion_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appCustomProductPageVersions","id":"version-1","attributes":{"deepLink":"https://example.com/deeplink"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCustomProductPageVersions/version-1" {
			t.Fatalf("expected path /v1/appCustomProductPageVersions/version-1, got %s", req.URL.Path)
		}
		var payload AppCustomProductPageVersionUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppCustomProductPageVersions {
			t.Fatalf("expected type appCustomProductPageVersions, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.DeepLink == nil {
			t.Fatalf("expected deepLink attribute")
		}
		if *payload.Data.Attributes.DeepLink != "https://example.com/deeplink" {
			t.Fatalf("expected deepLink value, got %q", *payload.Data.Attributes.DeepLink)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppCustomProductPageVersionUpdateAttributes{
		DeepLink: new("https://example.com/deeplink"),
	}
	if _, err := client.UpdateAppCustomProductPageVersion(context.Background(), "version-1", attrs); err != nil {
		t.Fatalf("UpdateAppCustomProductPageVersion() error: %v", err)
	}
}

func TestGetAppCustomProductPageLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appCustomProductPageLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCustomProductPageLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appCustomProductPageLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppCustomProductPageLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetAppCustomProductPageLocalization() error: %v", err)
	}
}

func TestAddAppCustomProductPageLocalizationSearchKeywords_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCustomProductPageLocalizations/loc-1/relationships/searchKeywords" {
			t.Fatalf("expected path /v1/appCustomProductPageLocalizations/loc-1/relationships/searchKeywords, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 keywords, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeAppKeywords {
			t.Fatalf("expected type appKeywords, got %q", payload.Data[0].Type)
		}
		if payload.Data[0].ID != "kw-1" {
			t.Fatalf("expected keyword kw-1, got %q", payload.Data[0].ID)
		}
		if payload.Data[1].ID != "kw-2" {
			t.Fatalf("expected keyword kw-2, got %q", payload.Data[1].ID)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AddAppCustomProductPageLocalizationSearchKeywords(context.Background(), "loc-1", []string{"kw-1", "kw-2"}); err != nil {
		t.Fatalf("AddAppCustomProductPageLocalizationSearchKeywords() error: %v", err)
	}
}

func TestDeleteAppCustomProductPageLocalizationSearchKeywords_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCustomProductPageLocalizations/loc-1/relationships/searchKeywords" {
			t.Fatalf("expected path /v1/appCustomProductPageLocalizations/loc-1/relationships/searchKeywords, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if len(payload.Data) != 1 {
			t.Fatalf("expected 1 keyword, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeAppKeywords {
			t.Fatalf("expected type appKeywords, got %q", payload.Data[0].Type)
		}
		if payload.Data[0].ID != "kw-1" {
			t.Fatalf("expected keyword kw-1, got %q", payload.Data[0].ID)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppCustomProductPageLocalizationSearchKeywords(context.Background(), "loc-1", []string{"kw-1"}); err != nil {
		t.Fatalf("DeleteAppCustomProductPageLocalizationSearchKeywords() error: %v", err)
	}
}

func TestAppCustomProductPageEndpoints_ValidateRequiredArguments(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "GetAppCustomProductPages requires app ID",
			call: func() error {
				_, err := client.GetAppCustomProductPages(ctx, "")
				return err
			},
		},
		{
			name: "GetAppCustomProductPage requires ID",
			call: func() error {
				_, err := client.GetAppCustomProductPage(ctx, "")
				return err
			},
		},
		{
			name: "GetAppCustomProductPageVersions requires page ID",
			call: func() error {
				_, err := client.GetAppCustomProductPageVersions(ctx, "")
				return err
			},
		},
		{
			name: "GetAppCustomProductPageVersion requires ID",
			call: func() error {
				_, err := client.GetAppCustomProductPageVersion(ctx, "")
				return err
			},
		},
		{
			name: "GetAppCustomProductPageLocalizations requires version ID",
			call: func() error {
				_, err := client.GetAppCustomProductPageLocalizations(ctx, "")
				return err
			},
		},
		{
			name: "GetAppCustomProductPageLocalization requires ID",
			call: func() error {
				_, err := client.GetAppCustomProductPageLocalization(ctx, "")
				return err
			},
		},
		{
			name: "GetAppCustomProductPageLocalizationSearchKeywords requires localization ID",
			call: func() error {
				_, err := client.GetAppCustomProductPageLocalizationSearchKeywords(ctx, "")
				return err
			},
		},
		{
			name: "AddAppCustomProductPageLocalizationSearchKeywords requires localization ID",
			call: func() error {
				return client.AddAppCustomProductPageLocalizationSearchKeywords(ctx, "", []string{"kw-1"})
			},
		},
		{
			name: "AddAppCustomProductPageLocalizationSearchKeywords requires keywords",
			call: func() error {
				return client.AddAppCustomProductPageLocalizationSearchKeywords(ctx, "loc-1", nil)
			},
		},
		{
			name: "DeleteAppCustomProductPageLocalizationSearchKeywords requires localization ID",
			call: func() error {
				return client.DeleteAppCustomProductPageLocalizationSearchKeywords(ctx, "", []string{"kw-1"})
			},
		},
		{
			name: "DeleteAppCustomProductPageLocalizationSearchKeywords requires keywords",
			call: func() error {
				return client.DeleteAppCustomProductPageLocalizationSearchKeywords(ctx, "loc-1", nil)
			},
		},
		{
			name: "GetAppCustomProductPageLocalizationPreviewSets requires localization ID",
			call: func() error {
				_, err := client.GetAppCustomProductPageLocalizationPreviewSets(ctx, "")
				return err
			},
		},
		{
			name: "GetAppCustomProductPageLocalizationScreenshotSets requires localization ID",
			call: func() error {
				_, err := client.GetAppCustomProductPageLocalizationScreenshotSets(ctx, "")
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestCreateAppCustomProductPageLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appCustomProductPageLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCustomProductPageLocalizations" {
			t.Fatalf("expected path /v1/appCustomProductPageLocalizations, got %s", req.URL.Path)
		}
		var payload AppCustomProductPageLocalizationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppCustomProductPageLocalizations {
			t.Fatalf("expected type appCustomProductPageLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Locale != "en-US" {
			t.Fatalf("expected locale en-US, got %q", payload.Data.Attributes.Locale)
		}
		if payload.Data.Attributes.PromotionalText != "Promo" {
			t.Fatalf("expected promotional text, got %q", payload.Data.Attributes.PromotionalText)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppCustomProductPageVersion == nil {
			t.Fatalf("expected appCustomProductPageVersion relationship")
		}
		if payload.Data.Relationships.AppCustomProductPageVersion.Data.ID != "version-1" {
			t.Fatalf("expected version ID version-1, got %q", payload.Data.Relationships.AppCustomProductPageVersion.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppCustomProductPageLocalization(context.Background(), "version-1", "en-US", "Promo"); err != nil {
		t.Fatalf("CreateAppCustomProductPageLocalization() error: %v", err)
	}
}

func TestUpdateAppCustomProductPageLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appCustomProductPageLocalizations","id":"loc-1","attributes":{"promotionalText":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCustomProductPageLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appCustomProductPageLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload AppCustomProductPageLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppCustomProductPageLocalizations {
			t.Fatalf("expected type appCustomProductPageLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.PromotionalText == nil {
			t.Fatalf("expected promotionalText attribute")
		}
		if *payload.Data.Attributes.PromotionalText != "Updated" {
			t.Fatalf("expected promotionalText Updated, got %q", *payload.Data.Attributes.PromotionalText)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppCustomProductPageLocalizationUpdateAttributes{
		PromotionalText: new("Updated"),
	}
	if _, err := client.UpdateAppCustomProductPageLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateAppCustomProductPageLocalization() error: %v", err)
	}
}

func TestGetAppStoreVersionExperiments_WithState(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionExperiments" {
			t.Fatalf("expected path /v1/appStoreVersions/version-1/appStoreVersionExperiments, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("filter[state]") != "IN_REVIEW" {
			t.Fatalf("expected filter[state]=IN_REVIEW, got %q", req.URL.Query().Get("filter[state]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersionExperiments(context.Background(), "version-1", WithAppStoreVersionExperimentsState([]string{"IN_REVIEW"})); err != nil {
		t.Fatalf("GetAppStoreVersionExperiments() error: %v", err)
	}
}

func TestGetAppStoreVersionExperimentsV2_WithState(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/appStoreVersionExperimentsV2" {
			t.Fatalf("expected path /v1/apps/app-1/appStoreVersionExperimentsV2, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" {
			t.Fatalf("expected filter[state]=READY_FOR_REVIEW, got %q", req.URL.Query().Get("filter[state]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersionExperimentsV2(context.Background(), "app-1", WithAppStoreVersionExperimentsV2State([]string{"READY_FOR_REVIEW"})); err != nil {
		t.Fatalf("GetAppStoreVersionExperimentsV2() error: %v", err)
	}
}

func TestCreateAppStoreVersionExperiment_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionExperiments","id":"exp-1","attributes":{"name":"Icon Test"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionExperiments" {
			t.Fatalf("expected path /v1/appStoreVersionExperiments, got %s", req.URL.Path)
		}
		var payload AppStoreVersionExperimentCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppStoreVersionExperiments {
			t.Fatalf("expected type appStoreVersionExperiments, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Icon Test" {
			t.Fatalf("expected name Icon Test, got %q", payload.Data.Attributes.Name)
		}
		if payload.Data.Attributes.TrafficProportion != 25 {
			t.Fatalf("expected trafficProportion 25, got %d", payload.Data.Attributes.TrafficProportion)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppStoreVersion == nil {
			t.Fatalf("expected appStoreVersion relationship")
		}
		if payload.Data.Relationships.AppStoreVersion.Data.ID != "version-1" {
			t.Fatalf("expected version ID version-1, got %q", payload.Data.Relationships.AppStoreVersion.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppStoreVersionExperiment(context.Background(), "version-1", "Icon Test", 25); err != nil {
		t.Fatalf("CreateAppStoreVersionExperiment() error: %v", err)
	}
}

func TestCreateAppStoreVersionExperimentV2_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionExperiments","id":"exp-2","attributes":{"name":"Icon Test V2","platform":"IOS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/appStoreVersionExperiments" {
			t.Fatalf("expected path /v2/appStoreVersionExperiments, got %s", req.URL.Path)
		}
		var payload AppStoreVersionExperimentV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppStoreVersionExperiments {
			t.Fatalf("expected type appStoreVersionExperiments, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Platform != PlatformIOS {
			t.Fatalf("expected platform IOS, got %q", payload.Data.Attributes.Platform)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship")
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app ID app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppStoreVersionExperimentV2(context.Background(), "app-1", PlatformIOS, "Icon Test V2", 40); err != nil {
		t.Fatalf("CreateAppStoreVersionExperimentV2() error: %v", err)
	}
}

func TestUpdateAppStoreVersionExperiment_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionExperiments","id":"exp-1","attributes":{"name":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionExperiments/exp-1" {
			t.Fatalf("expected path /v1/appStoreVersionExperiments/exp-1, got %s", req.URL.Path)
		}
		var payload AppStoreVersionExperimentUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil {
			t.Fatalf("expected name attribute")
		}
		if *payload.Data.Attributes.Name != "Updated" {
			t.Fatalf("expected name Updated, got %q", *payload.Data.Attributes.Name)
		}
		if payload.Data.Attributes.Started == nil || *payload.Data.Attributes.Started != true {
			t.Fatalf("expected started true")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppStoreVersionExperimentUpdateAttributes{
		Name:    new("Updated"),
		Started: new(true),
	}
	if _, err := client.UpdateAppStoreVersionExperiment(context.Background(), "exp-1", attrs); err != nil {
		t.Fatalf("UpdateAppStoreVersionExperiment() error: %v", err)
	}
}

func TestUpdateAppStoreVersionExperimentV2_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionExperiments","id":"exp-2","attributes":{"name":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/appStoreVersionExperiments/exp-2" {
			t.Fatalf("expected path /v2/appStoreVersionExperiments/exp-2, got %s", req.URL.Path)
		}
		var payload AppStoreVersionExperimentV2UpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil {
			t.Fatalf("expected name attribute")
		}
		if *payload.Data.Attributes.Name != "Updated" {
			t.Fatalf("expected name Updated, got %q", *payload.Data.Attributes.Name)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppStoreVersionExperimentV2UpdateAttributes{
		Name: new("Updated"),
	}
	if _, err := client.UpdateAppStoreVersionExperimentV2(context.Background(), "exp-2", attrs); err != nil {
		t.Fatalf("UpdateAppStoreVersionExperimentV2() error: %v", err)
	}
}

func TestCreateAppStoreVersionExperimentTreatment_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionExperimentTreatments","id":"treat-1","attributes":{"name":"Variant A"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionExperimentTreatments" {
			t.Fatalf("expected path /v1/appStoreVersionExperimentTreatments, got %s", req.URL.Path)
		}
		var payload AppStoreVersionExperimentTreatmentCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppStoreVersionExperimentTreatments {
			t.Fatalf("expected type appStoreVersionExperimentTreatments, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Variant A" {
			t.Fatalf("expected name Variant A, got %q", payload.Data.Attributes.Name)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppStoreVersionExperiment == nil {
			t.Fatalf("expected appStoreVersionExperiment relationship")
		}
		if payload.Data.Relationships.AppStoreVersionExperiment.Data.ID != "exp-1" {
			t.Fatalf("expected experiment ID exp-1, got %q", payload.Data.Relationships.AppStoreVersionExperiment.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppStoreVersionExperimentTreatment(context.Background(), "exp-1", "Variant A", "Icon A"); err != nil {
		t.Fatalf("CreateAppStoreVersionExperimentTreatment() error: %v", err)
	}
}

func TestUpdateAppStoreVersionExperimentTreatment_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionExperimentTreatments","id":"treat-1","attributes":{"name":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionExperimentTreatments/treat-1" {
			t.Fatalf("expected path /v1/appStoreVersionExperimentTreatments/treat-1, got %s", req.URL.Path)
		}
		var payload AppStoreVersionExperimentTreatmentUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil {
			t.Fatalf("expected name attribute")
		}
		if *payload.Data.Attributes.Name != "Updated" {
			t.Fatalf("expected name Updated, got %q", *payload.Data.Attributes.Name)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppStoreVersionExperimentTreatmentUpdateAttributes{
		Name: new("Updated"),
	}
	if _, err := client.UpdateAppStoreVersionExperimentTreatment(context.Background(), "treat-1", attrs); err != nil {
		t.Fatalf("UpdateAppStoreVersionExperimentTreatment() error: %v", err)
	}
}

func TestGetAppStoreVersionExperimentTreatmentLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionExperimentTreatmentLocalizations","id":"tloc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionExperimentTreatmentLocalizations/tloc-1" {
			t.Fatalf("expected path /v1/appStoreVersionExperimentTreatmentLocalizations/tloc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersionExperimentTreatmentLocalization(context.Background(), "tloc-1"); err != nil {
		t.Fatalf("GetAppStoreVersionExperimentTreatmentLocalization() error: %v", err)
	}
}

func TestCreateAppStoreVersionExperimentTreatmentLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionExperimentTreatmentLocalizations","id":"tloc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionExperimentTreatmentLocalizations" {
			t.Fatalf("expected path /v1/appStoreVersionExperimentTreatmentLocalizations, got %s", req.URL.Path)
		}
		var payload AppStoreVersionExperimentTreatmentLocalizationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppStoreVersionExperimentTreatmentLocalizations {
			t.Fatalf("expected type appStoreVersionExperimentTreatmentLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Locale != "en-US" {
			t.Fatalf("expected locale en-US, got %q", payload.Data.Attributes.Locale)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppStoreVersionExperimentTreatment == nil {
			t.Fatalf("expected treatment relationship")
		}
		if payload.Data.Relationships.AppStoreVersionExperimentTreatment.Data.ID != "treat-1" {
			t.Fatalf("expected treatment ID treat-1, got %q", payload.Data.Relationships.AppStoreVersionExperimentTreatment.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppStoreVersionExperimentTreatmentLocalization(context.Background(), "treat-1", "en-US"); err != nil {
		t.Fatalf("CreateAppStoreVersionExperimentTreatmentLocalization() error: %v", err)
	}
}
