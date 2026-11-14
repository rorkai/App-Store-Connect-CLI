package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestGetAppClips_WithFiltersAndLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appClips","id":"clip-1","attributes":{"bundleId":"com.example.clip"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/appClips" {
			t.Fatalf("expected path /v1/apps/app-1/appClips, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[bundleId]") != "com.example.clip" {
			t.Fatalf("expected filter[bundleId]=com.example.clip, got %q", values.Get("filter[bundleId]"))
		}
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClips(context.Background(), "app-1", WithAppClipsBundleIDs([]string{"com.example.clip"}), Limit[AppClipsOption](10)); err != nil {
		t.Fatalf("GetAppClips() error: %v", err)
	}
}

func TestGetAppClip(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClips","id":"clip-1","attributes":{"bundleId":"com.example.clip"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClips/clip-1" {
			t.Fatalf("expected path /v1/appClips/clip-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClip(context.Background(), "clip-1"); err != nil {
		t.Fatalf("GetAppClip() error: %v", err)
	}
}

func TestCreateAppClipDefaultExperience(t *testing.T) {
	action := AppClipActionOpen
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appClipDefaultExperiences","id":"exp-1","attributes":{"action":"OPEN"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperiences" {
			t.Fatalf("expected path /v1/appClipDefaultExperiences, got %s", req.URL.Path)
		}
		var payload AppClipDefaultExperienceCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Relationships.AppClip.Data.ID != "clip-1" {
			t.Fatalf("expected appClip id clip-1, got %s", payload.Data.Relationships.AppClip.Data.ID)
		}
		if payload.Data.Relationships.ReleaseWithAppStoreVersion.Data.ID != "version-1" {
			t.Fatalf("expected releaseWithAppStoreVersion id version-1, got %s", payload.Data.Relationships.ReleaseWithAppStoreVersion.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Action == nil || *payload.Data.Attributes.Action != action {
			t.Fatalf("expected action OPEN, got %#v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := &AppClipDefaultExperienceCreateAttributes{Action: &action}
	if _, err := client.CreateAppClipDefaultExperience(context.Background(), "clip-1", attrs, "version-1", ""); err != nil {
		t.Fatalf("CreateAppClipDefaultExperience() error: %v", err)
	}
}

func TestUpdateAppClipDefaultExperience(t *testing.T) {
	action := AppClipActionView
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClipDefaultExperiences","id":"exp-1","attributes":{"action":"VIEW"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperiences/exp-1" {
			t.Fatalf("expected path /v1/appClipDefaultExperiences/exp-1, got %s", req.URL.Path)
		}
		var payload AppClipDefaultExperienceUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Action == nil || *payload.Data.Attributes.Action != action {
			t.Fatalf("expected action VIEW, got %#v", payload.Data.Attributes)
		}
		if payload.Data.Relationships.ReleaseWithAppStoreVersion.Data.ID != "version-2" {
			t.Fatalf("expected releaseWithAppStoreVersion id version-2, got %s", payload.Data.Relationships.ReleaseWithAppStoreVersion.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := &AppClipDefaultExperienceUpdateAttributes{Action: &action}
	if _, err := client.UpdateAppClipDefaultExperience(context.Background(), "exp-1", attrs, "version-2"); err != nil {
		t.Fatalf("UpdateAppClipDefaultExperience() error: %v", err)
	}
}

func TestDeleteAppClipDefaultExperience(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperiences/exp-1" {
			t.Fatalf("expected path /v1/appClipDefaultExperiences/exp-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppClipDefaultExperience(context.Background(), "exp-1"); err != nil {
		t.Fatalf("DeleteAppClipDefaultExperience() error: %v", err)
	}
}

func TestGetAppClipDefaultExperienceLocalizations_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperiences/exp-1/appClipDefaultExperienceLocalizations" {
			t.Fatalf("expected path /v1/appClipDefaultExperiences/exp-1/appClipDefaultExperienceLocalizations, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[locale]") != "en-US,fr-FR" {
			t.Fatalf("expected filter[locale]=en-US,fr-FR, got %q", values.Get("filter[locale]"))
		}
		if values.Get("limit") != "20" {
			t.Fatalf("expected limit=20, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClipDefaultExperienceLocalizations(
		context.Background(), "exp-1",
		WithAppClipDefaultExperienceLocalizationsLocales([]string{"en-US", "fr-FR"}),
		Limit[AppClipDefaultExperienceLocalizationsOption](20),
	); err != nil {
		t.Fatalf("GetAppClipDefaultExperienceLocalizations() error: %v", err)
	}
}

func TestCreateAppClipDefaultExperienceLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appClipDefaultExperienceLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperienceLocalizations" {
			t.Fatalf("expected path /v1/appClipDefaultExperienceLocalizations, got %s", req.URL.Path)
		}
		var payload AppClipDefaultExperienceLocalizationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Relationships.AppClipDefaultExperience.Data.ID != "exp-1" {
			t.Fatalf("expected default experience id exp-1, got %s", payload.Data.Relationships.AppClipDefaultExperience.Data.ID)
		}
		if payload.Data.Attributes.Locale != "en-US" {
			t.Fatalf("expected locale en-US, got %s", payload.Data.Attributes.Locale)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppClipDefaultExperienceLocalizationCreateAttributes{Locale: "en-US"}
	if _, err := client.CreateAppClipDefaultExperienceLocalization(context.Background(), "exp-1", attrs); err != nil {
		t.Fatalf("CreateAppClipDefaultExperienceLocalization() error: %v", err)
	}
}

func TestUpdateAppClipDefaultExperienceLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClipDefaultExperienceLocalizations","id":"loc-1","attributes":{"subtitle":"Try it"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperienceLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appClipDefaultExperienceLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload AppClipDefaultExperienceLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Attributes.Subtitle == nil || *payload.Data.Attributes.Subtitle != "Try it" {
			t.Fatalf("expected subtitle Try it, got %#v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	subtitle := "Try it"
	attrs := &AppClipDefaultExperienceLocalizationUpdateAttributes{Subtitle: &subtitle}
	if _, err := client.UpdateAppClipDefaultExperienceLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateAppClipDefaultExperienceLocalization() error: %v", err)
	}
}

func TestDeleteAppClipDefaultExperienceLocalization(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperienceLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appClipDefaultExperienceLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppClipDefaultExperienceLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteAppClipDefaultExperienceLocalization() error: %v", err)
	}
}

func TestGetAppClipDefaultExperience_WithInclude(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClipDefaultExperiences","id":"exp-1","attributes":{"action":"OPEN"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperiences/exp-1" {
			t.Fatalf("expected path /v1/appClipDefaultExperiences/exp-1, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("include") != "releaseWithAppStoreVersion,appClipAppStoreReviewDetail" {
			t.Fatalf("expected include=releaseWithAppStoreVersion,appClipAppStoreReviewDetail, got %q", values.Get("include"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClipDefaultExperience(
		context.Background(), "exp-1",
		WithAppClipDefaultExperienceInclude([]string{"releaseWithAppStoreVersion", "appClipAppStoreReviewDetail"}),
	); err != nil {
		t.Fatalf("GetAppClipDefaultExperience() error: %v", err)
	}
}

func TestGetAppClipDefaultExperiencesRelationships(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appClipDefaultExperiences","id":"exp-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClips/clip-1/relationships/appClipDefaultExperiences" {
			t.Fatalf("expected path /v1/appClips/clip-1/relationships/appClipDefaultExperiences, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClipDefaultExperiencesRelationships(context.Background(), "clip-1", Limit[LinkagesOption](10)); err != nil {
		t.Fatalf("GetAppClipDefaultExperiencesRelationships() error: %v", err)
	}
}

func TestGetAppClipAdvancedExperiencesRelationships(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appClipAdvancedExperiences","id":"adv-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClips/clip-1/relationships/appClipAdvancedExperiences" {
			t.Fatalf("expected path /v1/appClips/clip-1/relationships/appClipAdvancedExperiences, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "20" {
			t.Fatalf("expected limit=20, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClipAdvancedExperiencesRelationships(context.Background(), "clip-1", Limit[LinkagesOption](20)); err != nil {
		t.Fatalf("GetAppClipAdvancedExperiencesRelationships() error: %v", err)
	}
}

func TestGetAppClipDefaultExperienceReviewDetailRelationship(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClipAppStoreReviewDetails","id":"detail-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperiences/exp-1/relationships/appClipAppStoreReviewDetail" {
			t.Fatalf("expected path /v1/appClipDefaultExperiences/exp-1/relationships/appClipAppStoreReviewDetail, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClipDefaultExperienceReviewDetailRelationship(context.Background(), "exp-1"); err != nil {
		t.Fatalf("GetAppClipDefaultExperienceReviewDetailRelationship() error: %v", err)
	}
}

func TestGetAppClipDefaultExperienceReleaseWithAppStoreVersionRelationship(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperiences/exp-1/relationships/releaseWithAppStoreVersion" {
			t.Fatalf("expected path /v1/appClipDefaultExperiences/exp-1/relationships/releaseWithAppStoreVersion, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClipDefaultExperienceReleaseWithAppStoreVersionRelationship(context.Background(), "exp-1"); err != nil {
		t.Fatalf("GetAppClipDefaultExperienceReleaseWithAppStoreVersionRelationship() error: %v", err)
	}
}

func TestGetAppClipDefaultExperienceReviewDetail(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClipAppStoreReviewDetails","id":"detail-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperiences/exp-1/appClipAppStoreReviewDetail" {
			t.Fatalf("expected path /v1/appClipDefaultExperiences/exp-1/appClipAppStoreReviewDetail, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClipDefaultExperienceReviewDetail(context.Background(), "exp-1"); err != nil {
		t.Fatalf("GetAppClipDefaultExperienceReviewDetail() error: %v", err)
	}
}

func TestGetAppClipDefaultExperienceReleaseWithAppStoreVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperiences/exp-1/releaseWithAppStoreVersion" {
			t.Fatalf("expected path /v1/appClipDefaultExperiences/exp-1/releaseWithAppStoreVersion, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClipDefaultExperienceReleaseWithAppStoreVersion(context.Background(), "exp-1"); err != nil {
		t.Fatalf("GetAppClipDefaultExperienceReleaseWithAppStoreVersion() error: %v", err)
	}
}

func TestGetAppClipDefaultExperienceLocalizationHeaderImageRelationship(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClipHeaderImages","id":"header-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperienceLocalizations/loc-1/relationships/appClipHeaderImage" {
			t.Fatalf("expected path /v1/appClipDefaultExperienceLocalizations/loc-1/relationships/appClipHeaderImage, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClipDefaultExperienceLocalizationHeaderImageRelationship(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetAppClipDefaultExperienceLocalizationHeaderImageRelationship() error: %v", err)
	}
}

func TestGetAppClipDefaultExperienceLocalizationHeaderImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClipHeaderImages","id":"header-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipDefaultExperienceLocalizations/loc-1/appClipHeaderImage" {
			t.Fatalf("expected path /v1/appClipDefaultExperienceLocalizations/loc-1/appClipHeaderImage, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClipDefaultExperienceLocalizationHeaderImage(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetAppClipDefaultExperienceLocalizationHeaderImage() error: %v", err)
	}
}

func TestGetAppClipAdvancedExperiences_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClips/clip-1/appClipAdvancedExperiences" {
			t.Fatalf("expected path /v1/appClips/clip-1/appClipAdvancedExperiences, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[action]") != "OPEN,VIEW" {
			t.Fatalf("expected filter[action]=OPEN,VIEW, got %q", values.Get("filter[action]"))
		}
		if values.Get("filter[status]") != "ACTIVE" {
			t.Fatalf("expected filter[status]=ACTIVE, got %q", values.Get("filter[status]"))
		}
		if values.Get("filter[placeStatus]") != "ACTIVE" {
			t.Fatalf("expected filter[placeStatus]=ACTIVE, got %q", values.Get("filter[placeStatus]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppClipAdvancedExperiences(
		context.Background(), "clip-1",
		WithAppClipAdvancedExperiencesActions([]string{"OPEN", "VIEW"}),
		WithAppClipAdvancedExperiencesStatuses([]string{"ACTIVE"}),
		WithAppClipAdvancedExperiencesPlaceStatuses([]string{"ACTIVE"}),
	); err != nil {
		t.Fatalf("GetAppClipAdvancedExperiences() error: %v", err)
	}
}

func TestCreateAppClipAdvancedExperience(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appClipAdvancedExperiences","id":"adv-1","attributes":{"link":"https://example.com","version":1793204321770}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipAdvancedExperiences" {
			t.Fatalf("expected path /v1/appClipAdvancedExperiences, got %s", req.URL.Path)
		}
		var payload AppClipAdvancedExperienceCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Attributes.Link != "https://example.com" {
			t.Fatalf("expected link, got %s", payload.Data.Attributes.Link)
		}
		if payload.Data.Attributes.DefaultLanguage != AppClipAdvancedExperienceLanguageEN {
			t.Fatalf("expected defaultLanguage EN, got %s", payload.Data.Attributes.DefaultLanguage)
		}
		if !payload.Data.Attributes.IsPoweredBy {
			t.Fatalf("expected isPoweredBy true")
		}
		if payload.Data.Relationships.AppClip.Data.ID != "clip-1" {
			t.Fatalf("expected appClip id clip-1, got %s", payload.Data.Relationships.AppClip.Data.ID)
		}
		if payload.Data.Relationships.AppClip.Data.Type != ResourceTypeAppClips {
			t.Fatalf("expected appClip type %s, got %s", ResourceTypeAppClips, payload.Data.Relationships.AppClip.Data.Type)
		}
		if payload.Data.Relationships.HeaderImage.Data.ID != "img-1" {
			t.Fatalf("expected headerImage id img-1, got %s", payload.Data.Relationships.HeaderImage.Data.ID)
		}
		if payload.Data.Relationships.HeaderImage.Data.Type != ResourceTypeAppClipAdvancedExperienceImages {
			t.Fatalf("expected headerImage type %s, got %s", ResourceTypeAppClipAdvancedExperienceImages, payload.Data.Relationships.HeaderImage.Data.Type)
		}
		if len(payload.Data.Relationships.Localizations.Data) != 2 {
			t.Fatalf("expected 2 localizations, got %d", len(payload.Data.Relationships.Localizations.Data))
		}
		wantLocalizationIDs := []string{"loc-1", "loc-2"}
		for i, localization := range payload.Data.Relationships.Localizations.Data {
			if localization.Type != ResourceTypeAppClipAdvancedExperienceLocalizations {
				t.Fatalf("expected localization type %s, got %s", ResourceTypeAppClipAdvancedExperienceLocalizations, localization.Type)
			}
			if localization.ID != wantLocalizationIDs[i] {
				t.Fatalf("expected localization id %s, got %s", wantLocalizationIDs[i], localization.ID)
			}
		}
		if len(payload.Included) != 0 {
			t.Fatalf("expected no included localizations, got %d", len(payload.Included))
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppClipAdvancedExperienceCreateAttributes{
		Link:            "https://example.com",
		DefaultLanguage: AppClipAdvancedExperienceLanguageEN,
		IsPoweredBy:     true,
	}
	if _, err := client.CreateAppClipAdvancedExperience(context.Background(), "clip-1", attrs, "img-1", []string{"loc-1", "loc-2"}, nil); err != nil {
		t.Fatalf("CreateAppClipAdvancedExperience() error: %v", err)
	}
}

func TestCreateAppClipAdvancedExperienceWithInlineLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appClipAdvancedExperiences","id":"adv-1","attributes":{"link":"https://example.com"}},"links":{}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipAdvancedExperiences" {
			t.Fatalf("expected path /v1/appClipAdvancedExperiences, got %s", req.URL.Path)
		}

		var payload AppClipAdvancedExperienceCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Relationships.AppClip.Data.Type != ResourceTypeAppClips || payload.Data.Relationships.AppClip.Data.ID != "clip-1" {
			t.Fatalf("unexpected app clip linkage: %#v", payload.Data.Relationships.AppClip.Data)
		}
		if payload.Data.Relationships.HeaderImage.Data.Type != ResourceTypeAppClipAdvancedExperienceImages || payload.Data.Relationships.HeaderImage.Data.ID != "img-1" {
			t.Fatalf("unexpected header image linkage: %#v", payload.Data.Relationships.HeaderImage.Data)
		}
		if len(payload.Data.Relationships.Localizations.Data) != 1 {
			t.Fatalf("expected one localization relationship, got %d", len(payload.Data.Relationships.Localizations.Data))
		}
		localizationLinkage := payload.Data.Relationships.Localizations.Data[0]
		if localizationLinkage.Type != ResourceTypeAppClipAdvancedExperienceLocalizations || localizationLinkage.ID != "${localization-1}" {
			t.Fatalf("unexpected inline localization linkage: %#v", localizationLinkage)
		}
		if len(payload.Included) != 1 {
			t.Fatalf("expected one included localization, got %d", len(payload.Included))
		}
		included := payload.Included[0]
		if included.Type != ResourceTypeAppClipAdvancedExperienceLocalizations || included.ID != localizationLinkage.ID {
			t.Fatalf("included localization must match linkage: %#v", included)
		}
		if included.Attributes.Language != AppClipAdvancedExperienceLanguageEN || included.Attributes.Title != "Order ahead" || included.Attributes.Subtitle != "Ready when you arrive" {
			t.Fatalf("unexpected included localization attributes: %#v", included.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppClipAdvancedExperienceCreateAttributes{
		Link:            "https://example.com",
		DefaultLanguage: AppClipAdvancedExperienceLanguageEN,
		IsPoweredBy:     true,
	}
	inline := []AppClipAdvancedExperienceLocalizationCreateAttributes{{
		Language: AppClipAdvancedExperienceLanguageEN,
		Title:    "Order ahead",
		Subtitle: "Ready when you arrive",
	}}
	if _, err := client.CreateAppClipAdvancedExperience(context.Background(), "clip-1", attrs, "img-1", nil, inline); err != nil {
		t.Fatalf("CreateAppClipAdvancedExperience() error: %v", err)
	}
}

func TestCreateAppClipAdvancedExperienceRequiresCreateRelationships(t *testing.T) {
	attrs := AppClipAdvancedExperienceCreateAttributes{
		Link:            "https://example.com",
		DefaultLanguage: AppClipAdvancedExperienceLanguageEN,
		IsPoweredBy:     true,
	}

	tests := []struct {
		name                string
		headerImageID       string
		localizationIDs     []string
		inlineLocalizations []AppClipAdvancedExperienceLocalizationCreateAttributes
		want                string
	}{
		{name: "header image", localizationIDs: []string{"loc-1"}, want: "headerImageID is required"},
		{name: "localizations", headerImageID: "img-1", want: "at least one localization is required"},
		{name: "blank localizations", headerImageID: "img-1", localizationIDs: []string{" ", ""}, want: "at least one localization is required"},
		{
			name:                "inline localization language",
			headerImageID:       "img-1",
			inlineLocalizations: []AppClipAdvancedExperienceLocalizationCreateAttributes{{Title: "Order ahead"}},
			want:                "inline localization 1: language is required",
		},
		{
			name:          "inline localization title",
			headerImageID: "img-1",
			inlineLocalizations: []AppClipAdvancedExperienceLocalizationCreateAttributes{{
				Language: AppClipAdvancedExperienceLanguageEN,
			}},
			want: "inline localization 1: title is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			}, jsonResponse(http.StatusCreated, `{}`))

			_, err := client.CreateAppClipAdvancedExperience(context.Background(), "clip-1", attrs, test.headerImageID, test.localizationIDs, test.inlineLocalizations)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("CreateAppClipAdvancedExperience() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestUpdateAppClipAdvancedExperience(t *testing.T) {
	action := AppClipActionPlay
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClipAdvancedExperiences","id":"adv-1","attributes":{"action":"PLAY","version":1793204321770}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipAdvancedExperiences/adv-1" {
			t.Fatalf("expected path /v1/appClipAdvancedExperiences/adv-1, got %s", req.URL.Path)
		}
		var payload AppClipAdvancedExperienceUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Attributes.Action == nil || *payload.Data.Attributes.Action != action {
			t.Fatalf("expected action PLAY, got %#v", payload.Data.Attributes)
		}
		if payload.Data.Relationships.HeaderImage.Data.ID != "img-2" {
			t.Fatalf("expected headerImage id img-2, got %s", payload.Data.Relationships.HeaderImage.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := &AppClipAdvancedExperienceUpdateAttributes{Action: &action}
	if _, err := client.UpdateAppClipAdvancedExperience(context.Background(), "adv-1", attrs, "", "img-2", nil); err != nil {
		t.Fatalf("UpdateAppClipAdvancedExperience() error: %v", err)
	}
}

func TestDeleteAppClipAdvancedExperience(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClipAdvancedExperiences","id":"adv-1","attributes":{"status":"REMOVED"}},"links":{}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipAdvancedExperiences/adv-1" {
			t.Fatalf("expected path /v1/appClipAdvancedExperiences/adv-1, got %s", req.URL.Path)
		}
		var payload AppClipAdvancedExperienceUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Removed == nil || !*payload.Data.Attributes.Removed {
			t.Fatalf("expected removed=true, got %#v", payload.Data.Attributes)
		}
		if payload.Data.Type != ResourceTypeAppClipAdvancedExperiences || payload.Data.ID != "adv-1" {
			t.Fatalf("expected advanced experience adv-1, got type=%s id=%s", payload.Data.Type, payload.Data.ID)
		}
		if payload.Data.Relationships != nil {
			t.Fatalf("expected no relationships, got %#v", payload.Data.Relationships)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppClipAdvancedExperience(context.Background(), "adv-1"); err != nil {
		t.Fatalf("DeleteAppClipAdvancedExperience() error: %v", err)
	}
}

func TestCreateAppClipAdvancedExperienceImage(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appClipAdvancedExperienceImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipAdvancedExperienceImages" {
			t.Fatalf("expected path /v1/appClipAdvancedExperienceImages, got %s", req.URL.Path)
		}
		var payload AppClipAdvancedExperienceImageCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Attributes.FileName != "image.png" {
			t.Fatalf("expected fileName image.png, got %s", payload.Data.Attributes.FileName)
		}
		if payload.Data.Attributes.FileSize != 12 {
			t.Fatalf("expected fileSize 12, got %d", payload.Data.Attributes.FileSize)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppClipAdvancedExperienceImage(context.Background(), "image.png", 12); err != nil {
		t.Fatalf("CreateAppClipAdvancedExperienceImage() error: %v", err)
	}
}

func TestCreateAppClipHeaderImage(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appClipHeaderImages","id":"hdr-1","attributes":{"fileName":"header.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appClipHeaderImages" {
			t.Fatalf("expected path /v1/appClipHeaderImages, got %s", req.URL.Path)
		}
		var payload AppClipHeaderImageCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Relationships.AppClipDefaultExperienceLocalization.Data.ID != "loc-1" {
			t.Fatalf("expected localization id loc-1, got %s", payload.Data.Relationships.AppClipDefaultExperienceLocalization.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppClipHeaderImage(context.Background(), "loc-1", "header.png", 12); err != nil {
		t.Fatalf("CreateAppClipHeaderImage() error: %v", err)
	}
}

func TestGetBetaAppClipInvocation(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaAppClipInvocations","id":"inv-1","attributes":{"url":"https://example.com/clip"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppClipInvocations/inv-1" {
			t.Fatalf("expected path /v1/betaAppClipInvocations/inv-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaAppClipInvocation(context.Background(), "inv-1"); err != nil {
		t.Fatalf("GetBetaAppClipInvocation() error: %v", err)
	}
}

func TestCreateBetaAppClipInvocation(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"betaAppClipInvocations","id":"inv-1","attributes":{"url":"https://example.com/clip"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppClipInvocations" {
			t.Fatalf("expected path /v1/betaAppClipInvocations, got %s", req.URL.Path)
		}
		var payload BetaAppClipInvocationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Relationships.BuildBundle.Data.ID != "bundle-1" {
			t.Fatalf("expected buildBundle id bundle-1, got %s", payload.Data.Relationships.BuildBundle.Data.ID)
		}
		if payload.Data.Relationships.BetaAppClipInvocationLocalizations == nil {
			t.Fatal("expected localization relationships to be present")
		}
		if len(payload.Data.Relationships.BetaAppClipInvocationLocalizations.Data) != 1 {
			t.Fatalf("expected one localization relationship, got %d", len(payload.Data.Relationships.BetaAppClipInvocationLocalizations.Data))
		}
		if payload.Data.Relationships.BetaAppClipInvocationLocalizations.Data[0].ID != "loc-1" {
			t.Fatalf("expected localization id loc-1, got %s", payload.Data.Relationships.BetaAppClipInvocationLocalizations.Data[0].ID)
		}
		if payload.Data.Attributes.URL != "https://example.com/clip" {
			t.Fatalf("expected url https://example.com/clip, got %s", payload.Data.Attributes.URL)
		}
		if len(payload.Included) != 0 {
			t.Fatalf("expected no included localizations, got %d", len(payload.Included))
		}
		assertAuthorized(t, req)
	}, response)

	attrs := BetaAppClipInvocationCreateAttributes{URL: "https://example.com/clip"}
	if _, err := client.CreateBetaAppClipInvocation(context.Background(), "bundle-1", attrs, []string{"loc-1"}, nil); err != nil {
		t.Fatalf("CreateBetaAppClipInvocation() error: %v", err)
	}
}

func TestCreateBetaAppClipInvocationWithInlineLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"betaAppClipInvocations","id":"inv-1","attributes":{"url":"https://example.com/clip"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppClipInvocations" {
			t.Fatalf("expected path /v1/betaAppClipInvocations, got %s", req.URL.Path)
		}
		var payload BetaAppClipInvocationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Relationships.BetaAppClipInvocationLocalizations == nil {
			t.Fatal("expected localization relationships to be present")
		}
		if len(payload.Data.Relationships.BetaAppClipInvocationLocalizations.Data) != 1 {
			t.Fatalf("expected one localization relationship, got %d", len(payload.Data.Relationships.BetaAppClipInvocationLocalizations.Data))
		}
		relationship := payload.Data.Relationships.BetaAppClipInvocationLocalizations.Data[0]
		if relationship.ID != "${localization-1}" {
			t.Fatalf("expected inline localization id ${localization-1}, got %s", relationship.ID)
		}
		if len(payload.Included) != 1 {
			t.Fatalf("expected one included localization, got %d", len(payload.Included))
		}
		included := payload.Included[0]
		if included.ID != relationship.ID {
			t.Fatalf("expected included id %s, got %s", relationship.ID, included.ID)
		}
		if included.Attributes.Locale != "en-US" {
			t.Fatalf("expected locale en-US, got %s", included.Attributes.Locale)
		}
		if included.Attributes.Title != "Try it" {
			t.Fatalf("expected title Try it, got %s", included.Attributes.Title)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := BetaAppClipInvocationCreateAttributes{URL: "https://example.com/clip"}
	inline := []BetaAppClipInvocationLocalizationCreateAttributes{{
		Locale: "en-US",
		Title:  "Try it",
	}}
	if _, err := client.CreateBetaAppClipInvocation(context.Background(), "bundle-1", attrs, nil, inline); err != nil {
		t.Fatalf("CreateBetaAppClipInvocation() error: %v", err)
	}
}

func TestCreateBetaAppClipInvocationRequiresLocalization(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		t.Fatalf("did not expect request, got %s %s", req.Method, req.URL.Path)
	}, jsonResponse(http.StatusCreated, `{"data":{"type":"betaAppClipInvocations","id":"inv-1"}}`))

	attrs := BetaAppClipInvocationCreateAttributes{URL: "https://example.com/clip"}
	_, err := client.CreateBetaAppClipInvocation(context.Background(), "bundle-1", attrs, nil, nil)
	if err == nil {
		t.Fatal("expected localization validation error, got nil")
	}
	if !strings.Contains(err.Error(), "at least one localization is required") {
		t.Fatalf("expected localization validation error, got %v", err)
	}
}

func TestUpdateBetaAppClipInvocation(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaAppClipInvocations","id":"inv-1","attributes":{"url":"https://example.com/new"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppClipInvocations/inv-1" {
			t.Fatalf("expected path /v1/betaAppClipInvocations/inv-1, got %s", req.URL.Path)
		}
		var payload BetaAppClipInvocationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Attributes.URL == nil || *payload.Data.Attributes.URL != "https://example.com/new" {
			t.Fatalf("expected url https://example.com/new, got %#v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	urlValue := "https://example.com/new"
	attrs := &BetaAppClipInvocationUpdateAttributes{URL: &urlValue}
	if _, err := client.UpdateBetaAppClipInvocation(context.Background(), "inv-1", attrs); err != nil {
		t.Fatalf("UpdateBetaAppClipInvocation() error: %v", err)
	}
}

func TestDeleteBetaAppClipInvocation(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppClipInvocations/inv-1" {
			t.Fatalf("expected path /v1/betaAppClipInvocations/inv-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteBetaAppClipInvocation(context.Background(), "inv-1"); err != nil {
		t.Fatalf("DeleteBetaAppClipInvocation() error: %v", err)
	}
}

func TestCreateBetaAppClipInvocationLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"betaAppClipInvocationLocalizations","id":"loc-1","attributes":{"title":"Try it","locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppClipInvocationLocalizations" {
			t.Fatalf("expected path /v1/betaAppClipInvocationLocalizations, got %s", req.URL.Path)
		}
		var payload BetaAppClipInvocationLocalizationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Relationships.BetaAppClipInvocation.Data.ID != "inv-1" {
			t.Fatalf("expected invocation id inv-1, got %s", payload.Data.Relationships.BetaAppClipInvocation.Data.ID)
		}
		if payload.Data.Attributes.Locale != "en-US" || payload.Data.Attributes.Title != "Try it" {
			t.Fatalf("unexpected localization attributes: %#v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := BetaAppClipInvocationLocalizationCreateAttributes{Locale: "en-US", Title: "Try it"}
	if _, err := client.CreateBetaAppClipInvocationLocalization(context.Background(), "inv-1", attrs); err != nil {
		t.Fatalf("CreateBetaAppClipInvocationLocalization() error: %v", err)
	}
}

func TestUpdateBetaAppClipInvocationLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaAppClipInvocationLocalizations","id":"loc-1","attributes":{"title":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppClipInvocationLocalizations/loc-1" {
			t.Fatalf("expected path /v1/betaAppClipInvocationLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload BetaAppClipInvocationLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Attributes.Title == nil || *payload.Data.Attributes.Title != "Updated" {
			t.Fatalf("expected title Updated, got %#v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	titleValue := "Updated"
	attrs := &BetaAppClipInvocationLocalizationUpdateAttributes{Title: &titleValue}
	if _, err := client.UpdateBetaAppClipInvocationLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateBetaAppClipInvocationLocalization() error: %v", err)
	}
}

func TestDeleteBetaAppClipInvocationLocalization(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppClipInvocationLocalizations/loc-1" {
			t.Fatalf("expected path /v1/betaAppClipInvocationLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteBetaAppClipInvocationLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteBetaAppClipInvocationLocalization() error: %v", err)
	}
}
