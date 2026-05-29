package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAppCompatibilityBuildsExpectedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/apps/app-123" {
			t.Fatalf("expected app path, got %s", r.URL.Path)
		}
		include := r.URL.Query().Get("include")
		if !strings.Contains(include, macCompatibilityRelationship) || !strings.Contains(include, visionCompatibilityRelationship) {
			t.Fatalf("expected compatibility includes, got %q", include)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"type": "apps",
				"id": "app-123",
				"relationships": {
					"iosAppToMacAppStoreOptInSetting": {"data": {"type": "iosAppToMacAppStoreOptInSettings", "id": "mac-1"}},
					"iosAppToAppStoreOnVisionOsOptInSetting": {"data": {"type": "iosAppToAppStoreOnVisionOsOptInSettings", "id": "vision-1"}}
				}
			},
			"included": [
				{"type": "iosAppToMacAppStoreOptInSettings", "id": "mac-1", "attributes": {"isOptedInToDistributeIosAppOnMacAppStore": false}},
				{"type": "iosAppToAppStoreOnVisionOsOptInSettings", "id": "vision-1", "attributes": {"isOptedInToDistributeIosAppToAppStoreOnVisionOs": true}}
			]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)

	got, err := client.GetAppCompatibility(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("GetAppCompatibility() error = %v", err)
	}
	if got.AppID != "app-123" || got.MacSettingID != "mac-1" || got.VisionProSettingID != "vision-1" {
		t.Fatalf("unexpected compatibility ids: %+v", got)
	}
	if got.IOSAppOnMac == nil || *got.IOSAppOnMac {
		t.Fatalf("expected mac=false, got %+v", got.IOSAppOnMac)
	}
	if got.IOSAppOnVisionPro == nil || !*got.IOSAppOnVisionPro {
		t.Fatalf("expected vision=true, got %+v", got.IOSAppOnVisionPro)
	}
}

func TestUpdateAppCompatibilityPatchesSelectedSettings(t *testing.T) {
	type patchRequest struct {
		path string
		body string
	}
	var patchRequests []patchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/apps/app-123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"type": "apps",
					"id": "app-123",
					"relationships": {
						"iosAppToMacAppStoreOptInSetting": {"data": {"type": "iosAppToMacAppStoreOptInSettings", "id": "mac-1"}},
						"iosAppToAppStoreOnVisionOsOptInSetting": {"data": {"type": "iosAppToAppStoreOnVisionOsOptInSettings", "id": "vision-1"}}
					}
				},
				"included": [
					{"type": "iosAppToMacAppStoreOptInSettings", "id": "mac-1", "attributes": {"isOptedInToDistributeIosAppOnMacAppStore": true}},
					{"type": "iosAppToAppStoreOnVisionOsOptInSettings", "id": "vision-1", "attributes": {"isOptedInToDistributeIosAppToAppStoreOnVisionOs": true}}
				]
			}`))
		case r.Method == http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read patch body: %v", err)
			}
			patchRequests = append(patchRequests, patchRequest{
				path: r.URL.Path,
				body: string(body),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"` + strings.TrimPrefix(r.URL.Path, "/") + `"}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client := testWebClient(server)

	value := false
	got, err := client.UpdateAppCompatibility(context.Background(), "app-123", &value, &value)
	if err != nil {
		t.Fatalf("UpdateAppCompatibility() error = %v", err)
	}
	if got.IOSAppOnMac == nil || *got.IOSAppOnMac {
		t.Fatalf("expected updated mac=false, got %+v", got.IOSAppOnMac)
	}
	if got.IOSAppOnVisionPro == nil || *got.IOSAppOnVisionPro {
		t.Fatalf("expected updated vision=false, got %+v", got.IOSAppOnVisionPro)
	}
	if len(patchRequests) != 2 {
		t.Fatalf("expected 2 patch requests, got %d", len(patchRequests))
	}

	expectedByPath := map[string]string{
		"/iosAppToMacAppStoreOptInSettings/mac-1":           `"isOptedInToDistributeIosAppOnMacAppStore":false`,
		"/iosAppToAppStoreOnVisionOsOptInSettings/vision-1": `"isOptedInToDistributeIosAppToAppStoreOnVisionOs":false`,
	}
	for _, req := range patchRequests {
		want, ok := expectedByPath[req.path]
		if !ok {
			t.Fatalf("unexpected patch path: %s", req.path)
		}
		if !strings.Contains(req.body, want) {
			t.Fatalf("unexpected patch payload for %s: %s", req.path, req.body)
		}
		delete(expectedByPath, req.path)
	}
	if len(expectedByPath) != 0 {
		t.Fatalf("missing expected patch requests: %+v", expectedByPath)
	}
}
