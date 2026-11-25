package assets

import (
	"encoding/json"
	"net/http"
	"testing"
)

func assetsScreenshotResumeTarget() screenshotResumeTarget {
	return screenshotResumeTarget{AppID: "100", LocalizationID: "LOC_123", DisplayType: "APP_IPHONE_65"}
}

func screenshotResumeScopeTestPayload(t *testing.T, req *http.Request, members []string) (string, bool) {
	t.Helper()
	if req.Method != http.MethodGet {
		return "", false
	}
	switch req.URL.Path {
	case "/v1/appStoreVersionLocalizations/LOC_123":
		query := req.URL.Query()
		if query.Get("include") != "appStoreVersion" || query.Get("fields[appStoreVersionLocalizations]") != "appStoreVersion" || query.Get("fields[appStoreVersions]") != "app" {
			t.Fatalf("unexpected ownership query: %s", req.URL)
		}
		return `{"data":{"type":"appStoreVersionLocalizations","id":"LOC_123","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_123"}}}}}`, true
	case "/v1/appStoreVersions/VERSION_123":
		return `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"100"}}}}}`, true
	case "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
		return `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`, true
	case "/v1/appScreenshotSets/set-1/appScreenshots", "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
		data := make([]map[string]any, 0, len(members))
		for _, id := range members {
			item := map[string]any{"type": "appScreenshots", "id": id}
			if req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots" {
				item["attributes"] = map[string]any{"sourceFileChecksum": "settled", "assetDeliveryState": map[string]string{"state": "COMPLETE"}}
			}
			data = append(data, item)
		}
		payload, err := json.Marshal(map[string]any{"data": data})
		if err != nil {
			t.Fatal(err)
		}
		return string(payload), true
	default:
		return "", false
	}
}
