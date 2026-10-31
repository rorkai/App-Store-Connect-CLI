package asc

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestEverySlotHasAnAssetLibraryProfileFromTheSpec(t *testing.T) {
	raw, err := os.ReadFile("../../docs/openapi/latest.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Enum []string `json:"enum"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	classes := spec.Components.Schemas["AppAssetLibraryDisplayClass"].Enum
	platforms := spec.Components.Schemas["AppAssetLibraryPlacementPlatform"].Enum
	for _, slot := range append(ScreenshotDisplayTypes(), ValidPreviewTypes...) {
		profile, ok := AssetLibraryProfileForSlot(slot)
		if !ok {
			t.Errorf("%s has no Asset Library profile, so --asset-library cannot place it", slot)
			continue
		}
		if !slices.Contains(classes, profile.DisplayClass) || !slices.Contains(platforms, profile.Platform) {
			t.Errorf("%s maps to %+v, which is not in the published enums", slot, profile)
		}
	}
}

func TestAssetLibraryProfileForSlotSeparatesIMessageFromApp(t *testing.T) {
	app, _ := AssetLibraryProfileForSlot("APP_IPHONE_69")
	iMessage, _ := AssetLibraryProfileForSlot("IMESSAGE_APP_IPHONE_69")
	if app.DisplayClass != iMessage.DisplayClass || app.Platform != "IPHONE_APP_STORE" || iMessage.Platform != "IMESSAGE_APP_STORE" {
		t.Fatalf("app = %+v, iMessage = %+v", app, iMessage)
	}
}
