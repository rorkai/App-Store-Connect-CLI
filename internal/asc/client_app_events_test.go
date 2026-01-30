package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func decodeRequestBody(t *testing.T, req *http.Request) map[string]interface{} {
	t.Helper()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return payload
}

func requireMap(t *testing.T, value interface{}, name string) map[string]interface{} {
	t.Helper()
	m, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected %s to be object, got %T", name, value)
	}
	return m
}

func requireString(t *testing.T, value interface{}, name string) string {
	t.Helper()
	str, ok := value.(string)
	if !ok {
		t.Fatalf("expected %s to be string, got %T", name, value)
	}
	return str
}

func requireBool(t *testing.T, value interface{}, name string) bool {
	t.Helper()
	boolean, ok := value.(bool)
	if !ok {
		t.Fatalf("expected %s to be bool, got %T", name, value)
	}
	return boolean
}

func requireFloat(t *testing.T, value interface{}, name string) float64 {
	t.Helper()
	number, ok := value.(float64)
	if !ok {
		t.Fatalf("expected %s to be number, got %T", name, value)
	}
	return number
}

func TestGetAppEvents_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/appEvents" {
			t.Fatalf("expected path /v1/apps/app-123/appEvents, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEvents(context.Background(), "app-123", WithAppEventsLimit(10)); err != nil {
		t.Fatalf("GetAppEvents() error: %v", err)
	}
}

func TestGetAppEvents_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps/app-123/appEvents?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEvents(context.Background(), "app-123", WithAppEventsNextURL(next)); err != nil {
		t.Fatalf("GetAppEvents() error: %v", err)
	}
}

func TestGetAppEvent(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Summer","badge":"CHALLENGE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEvents/event-1" {
			t.Fatalf("expected path /v1/appEvents/event-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEvent(context.Background(), "event-1"); err != nil {
		t.Fatalf("GetAppEvent() error: %v", err)
	}
}

func TestCreateAppEvent(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"PREMIERE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEvents" {
			t.Fatalf("expected path /v1/appEvents, got %s", req.URL.Path)
		}
		payload := decodeRequestBody(t, req)
		data := requireMap(t, payload["data"], "data")
		if requireString(t, data["type"], "data.type") != "appEvents" {
			t.Fatalf("unexpected type %v", data["type"])
		}
		attrs := requireMap(t, data["attributes"], "data.attributes")
		if requireString(t, attrs["referenceName"], "referenceName") != "Launch" {
			t.Fatalf("unexpected referenceName %v", attrs["referenceName"])
		}
		rels := requireMap(t, data["relationships"], "data.relationships")
		app := requireMap(t, rels["app"], "relationships.app")
		appData := requireMap(t, app["data"], "app.data")
		if requireString(t, appData["id"], "app.data.id") != "app-123" {
			t.Fatalf("unexpected app id %v", appData["id"])
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppEventCreateAttributes{
		ReferenceName: "Launch",
		Badge:         "PREMIERE",
	}
	if _, err := client.CreateAppEvent(context.Background(), "app-123", attrs); err != nil {
		t.Fatalf("CreateAppEvent() error: %v", err)
	}
}

func TestUpdateAppEvent(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEvents/event-1" {
			t.Fatalf("expected path /v1/appEvents/event-1, got %s", req.URL.Path)
		}
		payload := decodeRequestBody(t, req)
		data := requireMap(t, payload["data"], "data")
		if requireString(t, data["id"], "data.id") != "event-1" {
			t.Fatalf("unexpected id %v", data["id"])
		}
		attrs := requireMap(t, data["attributes"], "data.attributes")
		if requireString(t, attrs["referenceName"], "referenceName") != "Updated" {
			t.Fatalf("unexpected referenceName %v", attrs["referenceName"])
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	if _, err := client.UpdateAppEvent(context.Background(), "event-1", AppEventUpdateAttributes{ReferenceName: &name}); err != nil {
		t.Fatalf("UpdateAppEvent() error: %v", err)
	}
}

func TestDeleteAppEvent(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEvents/event-1" {
			t.Fatalf("expected path /v1/appEvents/event-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppEvent(context.Background(), "event-1"); err != nil {
		t.Fatalf("DeleteAppEvent() error: %v", err)
	}
}

func TestGetAppEventLocalizations_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEvents/event-1/localizations" {
			t.Fatalf("expected path /v1/appEvents/event-1/localizations, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEventLocalizations(context.Background(), "event-1", WithAppEventLocalizationsLimit(5)); err != nil {
		t.Fatalf("GetAppEventLocalizations() error: %v", err)
	}
}

func TestGetAppEventLocalizationsRelationships_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEvents/event-1/relationships/localizations" {
			t.Fatalf("expected path /v1/appEvents/event-1/relationships/localizations, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "3" {
			t.Fatalf("expected limit=3, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEventLocalizationsRelationships(context.Background(), "event-1", WithLinkagesLimit(3)); err != nil {
		t.Fatalf("GetAppEventLocalizationsRelationships() error: %v", err)
	}
}

func TestGetAppEventLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appEventLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appEventLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEventLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetAppEventLocalization() error: %v", err)
	}
}

func TestCreateAppEventLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appEventLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventLocalizations" {
			t.Fatalf("expected path /v1/appEventLocalizations, got %s", req.URL.Path)
		}
		payload := decodeRequestBody(t, req)
		data := requireMap(t, payload["data"], "data")
		attrs := requireMap(t, data["attributes"], "data.attributes")
		if requireString(t, attrs["locale"], "locale") != "en-US" {
			t.Fatalf("unexpected locale %v", attrs["locale"])
		}
		rels := requireMap(t, data["relationships"], "data.relationships")
		appEvent := requireMap(t, rels["appEvent"], "relationships.appEvent")
		appData := requireMap(t, appEvent["data"], "appEvent.data")
		if requireString(t, appData["id"], "appEvent.data.id") != "event-1" {
			t.Fatalf("unexpected event id %v", appData["id"])
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppEventLocalizationCreateAttributes{Locale: "en-US"}
	if _, err := client.CreateAppEventLocalization(context.Background(), "event-1", attrs); err != nil {
		t.Fatalf("CreateAppEventLocalization() error: %v", err)
	}
}

func TestUpdateAppEventLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appEventLocalizations","id":"loc-1","attributes":{"name":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appEventLocalizations/loc-1, got %s", req.URL.Path)
		}
		payload := decodeRequestBody(t, req)
		data := requireMap(t, payload["data"], "data")
		if requireString(t, data["id"], "data.id") != "loc-1" {
			t.Fatalf("unexpected id %v", data["id"])
		}
		attrs := requireMap(t, data["attributes"], "data.attributes")
		if requireString(t, attrs["name"], "name") != "Updated" {
			t.Fatalf("unexpected name %v", attrs["name"])
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	if _, err := client.UpdateAppEventLocalization(context.Background(), "loc-1", AppEventLocalizationUpdateAttributes{Name: &name}); err != nil {
		t.Fatalf("UpdateAppEventLocalization() error: %v", err)
	}
}

func TestDeleteAppEventLocalization(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appEventLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppEventLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteAppEventLocalization() error: %v", err)
	}
}

func TestGetAppEventScreenshots_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventLocalizations/loc-1/appEventScreenshots" {
			t.Fatalf("expected path /v1/appEventLocalizations/loc-1/appEventScreenshots, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "25" {
			t.Fatalf("expected limit=25, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEventScreenshots(context.Background(), "loc-1", WithAppEventScreenshotsLimit(25)); err != nil {
		t.Fatalf("GetAppEventScreenshots() error: %v", err)
	}
}

func TestGetAppEventScreenshotsRelationships_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventLocalizations/loc-1/relationships/appEventScreenshots" {
			t.Fatalf("expected path /v1/appEventLocalizations/loc-1/relationships/appEventScreenshots, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "8" {
			t.Fatalf("expected limit=8, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEventScreenshotsRelationships(context.Background(), "loc-1", WithLinkagesLimit(8)); err != nil {
		t.Fatalf("GetAppEventScreenshotsRelationships() error: %v", err)
	}
}

func TestGetAppEventScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appEventScreenshots","id":"shot-1","attributes":{"fileName":"event.png"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventScreenshots/shot-1" {
			t.Fatalf("expected path /v1/appEventScreenshots/shot-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEventScreenshot(context.Background(), "shot-1"); err != nil {
		t.Fatalf("GetAppEventScreenshot() error: %v", err)
	}
}

func TestCreateAppEventScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appEventScreenshots","id":"shot-1","attributes":{"fileName":"event.png"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventScreenshots" {
			t.Fatalf("expected path /v1/appEventScreenshots, got %s", req.URL.Path)
		}
		payload := decodeRequestBody(t, req)
		data := requireMap(t, payload["data"], "data")
		attrs := requireMap(t, data["attributes"], "data.attributes")
		if requireString(t, attrs["fileName"], "fileName") != "event.png" {
			t.Fatalf("unexpected fileName %v", attrs["fileName"])
		}
		if requireFloat(t, attrs["fileSize"], "fileSize") != 123 {
			t.Fatalf("unexpected fileSize %v", attrs["fileSize"])
		}
		if requireString(t, attrs["appEventAssetType"], "appEventAssetType") != "EVENT_CARD" {
			t.Fatalf("unexpected asset type %v", attrs["appEventAssetType"])
		}
		rels := requireMap(t, data["relationships"], "data.relationships")
		loc := requireMap(t, rels["appEventLocalization"], "relationships.appEventLocalization")
		locData := requireMap(t, loc["data"], "appEventLocalization.data")
		if requireString(t, locData["id"], "appEventLocalization.data.id") != "loc-1" {
			t.Fatalf("unexpected localization id %v", locData["id"])
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppEventScreenshot(context.Background(), "loc-1", "event.png", 123, "EVENT_CARD"); err != nil {
		t.Fatalf("CreateAppEventScreenshot() error: %v", err)
	}
}

func TestUpdateAppEventScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appEventScreenshots","id":"shot-1","attributes":{"fileName":"event.png"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventScreenshots/shot-1" {
			t.Fatalf("expected path /v1/appEventScreenshots/shot-1, got %s", req.URL.Path)
		}
		payload := decodeRequestBody(t, req)
		data := requireMap(t, payload["data"], "data")
		attrs := requireMap(t, data["attributes"], "data.attributes")
		if !requireBool(t, attrs["uploaded"], "uploaded") {
			t.Fatalf("expected uploaded true")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.UpdateAppEventScreenshot(context.Background(), "shot-1", true); err != nil {
		t.Fatalf("UpdateAppEventScreenshot() error: %v", err)
	}
}

func TestDeleteAppEventScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventScreenshots/shot-1" {
			t.Fatalf("expected path /v1/appEventScreenshots/shot-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppEventScreenshot(context.Background(), "shot-1"); err != nil {
		t.Fatalf("DeleteAppEventScreenshot() error: %v", err)
	}
}

func TestGetAppEventVideoClips_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventLocalizations/loc-1/appEventVideoClips" {
			t.Fatalf("expected path /v1/appEventLocalizations/loc-1/appEventVideoClips, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "7" {
			t.Fatalf("expected limit=7, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEventVideoClips(context.Background(), "loc-1", WithAppEventVideoClipsLimit(7)); err != nil {
		t.Fatalf("GetAppEventVideoClips() error: %v", err)
	}
}

func TestGetAppEventVideoClipsRelationships_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventLocalizations/loc-1/relationships/appEventVideoClips" {
			t.Fatalf("expected path /v1/appEventLocalizations/loc-1/relationships/appEventVideoClips, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "6" {
			t.Fatalf("expected limit=6, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEventVideoClipsRelationships(context.Background(), "loc-1", WithLinkagesLimit(6)); err != nil {
		t.Fatalf("GetAppEventVideoClipsRelationships() error: %v", err)
	}
}

func TestGetAppEventVideoClip(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appEventVideoClips","id":"clip-1","attributes":{"fileName":"clip.mov"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventVideoClips/clip-1" {
			t.Fatalf("expected path /v1/appEventVideoClips/clip-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEventVideoClip(context.Background(), "clip-1"); err != nil {
		t.Fatalf("GetAppEventVideoClip() error: %v", err)
	}
}

func TestCreateAppEventVideoClip(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appEventVideoClips","id":"clip-1","attributes":{"fileName":"clip.mov"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventVideoClips" {
			t.Fatalf("expected path /v1/appEventVideoClips, got %s", req.URL.Path)
		}
		payload := decodeRequestBody(t, req)
		data := requireMap(t, payload["data"], "data")
		attrs := requireMap(t, data["attributes"], "data.attributes")
		if requireString(t, attrs["fileName"], "fileName") != "clip.mov" {
			t.Fatalf("unexpected fileName %v", attrs["fileName"])
		}
		if requireFloat(t, attrs["fileSize"], "fileSize") != 256 {
			t.Fatalf("unexpected fileSize %v", attrs["fileSize"])
		}
		if requireString(t, attrs["appEventAssetType"], "appEventAssetType") != "EVENT_DETAILS_PAGE" {
			t.Fatalf("unexpected asset type %v", attrs["appEventAssetType"])
		}
		if requireString(t, attrs["previewFrameTimeCode"], "previewFrameTimeCode") != "00:00:05.000" {
			t.Fatalf("unexpected previewFrameTimeCode %v", attrs["previewFrameTimeCode"])
		}
		rels := requireMap(t, data["relationships"], "data.relationships")
		loc := requireMap(t, rels["appEventLocalization"], "relationships.appEventLocalization")
		locData := requireMap(t, loc["data"], "appEventLocalization.data")
		if requireString(t, locData["id"], "appEventLocalization.data.id") != "loc-1" {
			t.Fatalf("unexpected localization id %v", locData["id"])
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppEventVideoClip(context.Background(), "loc-1", "clip.mov", 256, "EVENT_DETAILS_PAGE", "00:00:05.000"); err != nil {
		t.Fatalf("CreateAppEventVideoClip() error: %v", err)
	}
}

func TestUpdateAppEventVideoClip(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appEventVideoClips","id":"clip-1","attributes":{"fileName":"clip.mov"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventVideoClips/clip-1" {
			t.Fatalf("expected path /v1/appEventVideoClips/clip-1, got %s", req.URL.Path)
		}
		payload := decodeRequestBody(t, req)
		data := requireMap(t, payload["data"], "data")
		attrs := requireMap(t, data["attributes"], "data.attributes")
		if requireString(t, attrs["previewFrameTimeCode"], "previewFrameTimeCode") != "00:00:10.000" {
			t.Fatalf("unexpected previewFrameTimeCode %v", attrs["previewFrameTimeCode"])
		}
		if !requireBool(t, attrs["uploaded"], "uploaded") {
			t.Fatalf("expected uploaded true")
		}
		assertAuthorized(t, req)
	}, response)

	preview := "00:00:10.000"
	uploaded := true
	attrs := AppEventVideoClipUpdateAttributes{
		PreviewFrameTimeCode: &preview,
		Uploaded:             &uploaded,
	}
	if _, err := client.UpdateAppEventVideoClip(context.Background(), "clip-1", attrs); err != nil {
		t.Fatalf("UpdateAppEventVideoClip() error: %v", err)
	}
}

func TestDeleteAppEventVideoClip(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEventVideoClips/clip-1" {
			t.Fatalf("expected path /v1/appEventVideoClips/clip-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppEventVideoClip(context.Background(), "clip-1"); err != nil {
		t.Fatalf("DeleteAppEventVideoClip() error: %v", err)
	}
}

func TestAppEventListMethods_InvalidNextURL(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		t.Fatalf("unexpected request %s", req.URL.String())
	}, jsonResponse(http.StatusOK, `{"data":[]}`))

	next := "https://example.com/steal"
	if _, err := client.GetAppEvents(context.Background(), "app-123", WithAppEventsNextURL(next)); err == nil {
		t.Fatal("expected error for invalid next URL")
	}
	if _, err := client.GetAppEventLocalizations(context.Background(), "event-1", WithAppEventLocalizationsNextURL(next)); err == nil {
		t.Fatal("expected error for invalid next URL")
	}
	if _, err := client.GetAppEventScreenshots(context.Background(), "loc-1", WithAppEventScreenshotsNextURL(next)); err == nil {
		t.Fatal("expected error for invalid next URL")
	}
	if _, err := client.GetAppEventVideoClips(context.Background(), "loc-1", WithAppEventVideoClipsNextURL(next)); err == nil {
		t.Fatal("expected error for invalid next URL")
	}
	if _, err := client.GetAppEventLocalizationsRelationships(context.Background(), "event-1", WithLinkagesNextURL(next)); err == nil {
		t.Fatal("expected error for invalid next URL")
	}
	if _, err := client.GetAppEventScreenshotsRelationships(context.Background(), "loc-1", WithLinkagesNextURL(next)); err == nil {
		t.Fatal("expected error for invalid next URL")
	}
	if _, err := client.GetAppEventVideoClipsRelationships(context.Background(), "loc-1", WithLinkagesNextURL(next)); err == nil {
		t.Fatal("expected error for invalid next URL")
	}
}

func TestAppEventMethodsRequireIDs(t *testing.T) {
	client := &Client{}
	attrs := AppEventCreateAttributes{ReferenceName: "Launch"}
	updateName := "Updated"
	upload := true
	preview := "00:00:01.000"

	tests := []struct {
		name string
		call func() error
	}{
		{"GetAppEvents", func() error {
			_, err := client.GetAppEvents(context.Background(), "")
			return err
		}},
		{"GetAppEvent", func() error {
			_, err := client.GetAppEvent(context.Background(), "")
			return err
		}},
		{"CreateAppEvent", func() error {
			_, err := client.CreateAppEvent(context.Background(), "", attrs)
			return err
		}},
		{"UpdateAppEvent", func() error {
			_, err := client.UpdateAppEvent(context.Background(), "", AppEventUpdateAttributes{ReferenceName: &updateName})
			return err
		}},
		{"DeleteAppEvent", func() error {
			return client.DeleteAppEvent(context.Background(), "")
		}},
		{"GetAppEventLocalizations", func() error {
			_, err := client.GetAppEventLocalizations(context.Background(), "")
			return err
		}},
		{"GetAppEventLocalizationsRelationships", func() error {
			_, err := client.GetAppEventLocalizationsRelationships(context.Background(), "")
			return err
		}},
		{"GetAppEventLocalization", func() error {
			_, err := client.GetAppEventLocalization(context.Background(), "")
			return err
		}},
		{"CreateAppEventLocalization", func() error {
			_, err := client.CreateAppEventLocalization(context.Background(), "", AppEventLocalizationCreateAttributes{Locale: "en-US"})
			return err
		}},
		{"UpdateAppEventLocalization", func() error {
			_, err := client.UpdateAppEventLocalization(context.Background(), "", AppEventLocalizationUpdateAttributes{Name: &updateName})
			return err
		}},
		{"DeleteAppEventLocalization", func() error {
			return client.DeleteAppEventLocalization(context.Background(), "")
		}},
		{"GetAppEventScreenshots", func() error {
			_, err := client.GetAppEventScreenshots(context.Background(), "")
			return err
		}},
		{"GetAppEventScreenshotsRelationships", func() error {
			_, err := client.GetAppEventScreenshotsRelationships(context.Background(), "")
			return err
		}},
		{"GetAppEventScreenshot", func() error {
			_, err := client.GetAppEventScreenshot(context.Background(), "")
			return err
		}},
		{"CreateAppEventScreenshot", func() error {
			_, err := client.CreateAppEventScreenshot(context.Background(), "", "file.png", 1, "EVENT_CARD")
			return err
		}},
		{"UpdateAppEventScreenshot", func() error {
			_, err := client.UpdateAppEventScreenshot(context.Background(), "", true)
			return err
		}},
		{"DeleteAppEventScreenshot", func() error {
			return client.DeleteAppEventScreenshot(context.Background(), "")
		}},
		{"GetAppEventVideoClips", func() error {
			_, err := client.GetAppEventVideoClips(context.Background(), "")
			return err
		}},
		{"GetAppEventVideoClipsRelationships", func() error {
			_, err := client.GetAppEventVideoClipsRelationships(context.Background(), "")
			return err
		}},
		{"GetAppEventVideoClip", func() error {
			_, err := client.GetAppEventVideoClip(context.Background(), "")
			return err
		}},
		{"CreateAppEventVideoClip", func() error {
			_, err := client.CreateAppEventVideoClip(context.Background(), "", "file.mov", 1, "EVENT_CARD", preview)
			return err
		}},
		{"UpdateAppEventVideoClip", func() error {
			_, err := client.UpdateAppEventVideoClip(context.Background(), "", AppEventVideoClipUpdateAttributes{Uploaded: &upload})
			return err
		}},
		{"DeleteAppEventVideoClip", func() error {
			return client.DeleteAppEventVideoClip(context.Background(), "")
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestAppEventMethodsReturnAPIError(t *testing.T) {
	response := jsonResponse(http.StatusInternalServerError, `{"errors":[{"title":"Server error","detail":"boom"}]}`)
	tests := []struct {
		name string
		call func(client *Client) error
	}{
		{"GetAppEvents", func(client *Client) error {
			_, err := client.GetAppEvents(context.Background(), "app-1")
			return err
		}},
		{"GetAppEvent", func(client *Client) error {
			_, err := client.GetAppEvent(context.Background(), "event-1")
			return err
		}},
		{"CreateAppEvent", func(client *Client) error {
			_, err := client.CreateAppEvent(context.Background(), "app-1", AppEventCreateAttributes{ReferenceName: "Launch"})
			return err
		}},
		{"UpdateAppEvent", func(client *Client) error {
			name := "Updated"
			_, err := client.UpdateAppEvent(context.Background(), "event-1", AppEventUpdateAttributes{ReferenceName: &name})
			return err
		}},
		{"DeleteAppEvent", func(client *Client) error {
			return client.DeleteAppEvent(context.Background(), "event-1")
		}},
		{"GetAppEventLocalizations", func(client *Client) error {
			_, err := client.GetAppEventLocalizations(context.Background(), "event-1")
			return err
		}},
		{"GetAppEventLocalizationsRelationships", func(client *Client) error {
			_, err := client.GetAppEventLocalizationsRelationships(context.Background(), "event-1")
			return err
		}},
		{"GetAppEventLocalization", func(client *Client) error {
			_, err := client.GetAppEventLocalization(context.Background(), "loc-1")
			return err
		}},
		{"CreateAppEventLocalization", func(client *Client) error {
			_, err := client.CreateAppEventLocalization(context.Background(), "event-1", AppEventLocalizationCreateAttributes{Locale: "en-US"})
			return err
		}},
		{"UpdateAppEventLocalization", func(client *Client) error {
			name := "Updated"
			_, err := client.UpdateAppEventLocalization(context.Background(), "loc-1", AppEventLocalizationUpdateAttributes{Name: &name})
			return err
		}},
		{"DeleteAppEventLocalization", func(client *Client) error {
			return client.DeleteAppEventLocalization(context.Background(), "loc-1")
		}},
		{"GetAppEventScreenshots", func(client *Client) error {
			_, err := client.GetAppEventScreenshots(context.Background(), "loc-1")
			return err
		}},
		{"GetAppEventScreenshotsRelationships", func(client *Client) error {
			_, err := client.GetAppEventScreenshotsRelationships(context.Background(), "loc-1")
			return err
		}},
		{"GetAppEventScreenshot", func(client *Client) error {
			_, err := client.GetAppEventScreenshot(context.Background(), "shot-1")
			return err
		}},
		{"CreateAppEventScreenshot", func(client *Client) error {
			_, err := client.CreateAppEventScreenshot(context.Background(), "loc-1", "event.png", 1, "EVENT_CARD")
			return err
		}},
		{"UpdateAppEventScreenshot", func(client *Client) error {
			_, err := client.UpdateAppEventScreenshot(context.Background(), "shot-1", true)
			return err
		}},
		{"DeleteAppEventScreenshot", func(client *Client) error {
			return client.DeleteAppEventScreenshot(context.Background(), "shot-1")
		}},
		{"GetAppEventVideoClips", func(client *Client) error {
			_, err := client.GetAppEventVideoClips(context.Background(), "loc-1")
			return err
		}},
		{"GetAppEventVideoClipsRelationships", func(client *Client) error {
			_, err := client.GetAppEventVideoClipsRelationships(context.Background(), "loc-1")
			return err
		}},
		{"GetAppEventVideoClip", func(client *Client) error {
			_, err := client.GetAppEventVideoClip(context.Background(), "clip-1")
			return err
		}},
		{"CreateAppEventVideoClip", func(client *Client) error {
			_, err := client.CreateAppEventVideoClip(context.Background(), "loc-1", "clip.mov", 1, "EVENT_CARD", "")
			return err
		}},
		{"UpdateAppEventVideoClip", func(client *Client) error {
			uploaded := true
			_, err := client.UpdateAppEventVideoClip(context.Background(), "clip-1", AppEventVideoClipUpdateAttributes{Uploaded: &uploaded})
			return err
		}},
		{"DeleteAppEventVideoClip", func(client *Client) error {
			return client.DeleteAppEventVideoClip(context.Background(), "clip-1")
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, nil, response)
			if err := test.call(client); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestAppEventListMethods_UseNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/appEventScreenshots?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEventScreenshots(context.Background(), "loc-1", WithAppEventScreenshotsNextURL(next)); err != nil {
		t.Fatalf("GetAppEventScreenshots() error: %v", err)
	}
}

func TestAppEventVideoClipCreateOmitsPreviewFrame(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appEventVideoClips","id":"clip-2","attributes":{"fileName":"clip.mov"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		payload := decodeRequestBody(t, req)
		data := requireMap(t, payload["data"], "data")
		attrs := requireMap(t, data["attributes"], "data.attributes")
		if _, ok := attrs["previewFrameTimeCode"]; ok {
			t.Fatalf("expected previewFrameTimeCode to be omitted")
		}
	}, response)

	if _, err := client.CreateAppEventVideoClip(context.Background(), "loc-1", "clip.mov", 1, "EVENT_CARD", ""); err != nil {
		t.Fatalf("CreateAppEventVideoClip() error: %v", err)
	}
}

func TestAppEventLocalizationsNextURLBypassesAppID(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/appEvents/app-1/localizations?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if !strings.HasSuffix(req.URL.String(), "cursor=abc") {
			t.Fatalf("expected next URL, got %q", req.URL.String())
		}
	}, response)

	if _, err := client.GetAppEventLocalizations(context.Background(), "", WithAppEventLocalizationsNextURL(next)); err != nil {
		t.Fatalf("GetAppEventLocalizations() error: %v", err)
	}
}
