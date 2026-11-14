package asc

import (
	"encoding/json"
	"testing"
)

// Attributes the App Store Connect API returns must survive decoding into the
// response type and encoding back, or JSON output silently drops them.
func TestResponseKeepsAttributesFromOpenAPI46(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		response  any
		attribute string
		value     string
	}{
		{
			name:      "app preview frame image",
			response:  &AppPreviewResponse{},
			attribute: "previewFrameImage",
			value:     `{"image":{"templateUrl":"https://example.test/{w}x{h}.{f}","width":1290,"height":2796},"state":{"state":"COMPLETE"}}`,
		},
		{
			name:      "experiment treatment app icon",
			response:  &AppStoreVersionExperimentTreatmentResponse{},
			attribute: "appIcon",
			value:     `{"templateUrl":"https://example.test/{w}x{h}.{f}","width":1024,"height":1024}`,
		},
		{
			name:      "subscription review screenshot asset type",
			response:  &SubscriptionAppStoreReviewScreenshotResponse{},
			attribute: "assetType",
			value:     `"SCREENSHOT"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := `{"data":{"type":"resources","id":"1","attributes":{"` + tt.attribute + `":` + tt.value + `}}}`
			if err := json.Unmarshal([]byte(body), tt.response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			encoded, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatalf("encode response: %v", err)
			}
			var payload struct {
				Data struct {
					Attributes map[string]json.RawMessage `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatalf("decode encoded response: %v", err)
			}
			if got := string(payload.Data.Attributes[tt.attribute]); got != tt.value {
				t.Fatalf("%s after a round trip = %q, want %s (encoded: %s)", tt.attribute, got, tt.value, encoded)
			}
		})
	}
}
