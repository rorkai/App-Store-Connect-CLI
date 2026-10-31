package asc

import "strings"

// AssetLibraryProfile identifies the placement profile that serves a legacy
// screenshot or preview slot. Apple publishes group IDs in reference data and
// does not promise their names, so callers look the group up by this pair.
type AssetLibraryProfile struct {
	DisplayClass string
	Platform     string
}

var assetLibraryProfilesBySlot = map[string]AssetLibraryProfile{
	"IPHONE_DUO":        {"IPHONE_DUO", "IPHONE_APP_STORE"},
	"IPHONE_67":         {"IPHONE_DYNAMIC_ISLAND_LARGE_DISPLAY", "IPHONE_APP_STORE"},
	"IPHONE_61":         {"IPHONE_DYNAMIC_ISLAND_MEDIUM_DISPLAY", "IPHONE_APP_STORE"},
	"IPHONE_65":         {"IPHONE_FACE_ID_LARGE_DISPLAY", "IPHONE_APP_STORE"},
	"IPHONE_58":         {"IPHONE_FACE_ID_MEDIUM_DISPLAY", "IPHONE_APP_STORE"},
	"IPHONE_55":         {"IPHONE_HOME_BUTTON_LARGE_DISPLAY", "IPHONE_APP_STORE"},
	"IPHONE_47":         {"IPHONE_HOME_BUTTON_MEDIUM_DISPLAY", "IPHONE_APP_STORE"},
	"IPHONE_40":         {"IPHONE_HOME_BUTTON_40_DISPLAY", "IPHONE_APP_STORE"},
	"IPHONE_35":         {"IPHONE_HOME_BUTTON_35_DISPLAY", "IPHONE_APP_STORE"},
	"IPAD_PRO_3GEN_129": {"IPAD_13_DISPLAY", "IPAD_APP_STORE"},
	"IPAD_PRO_3GEN_11":  {"IPAD_11_DISPLAY", "IPAD_APP_STORE"},
	"IPAD_PRO_129":      {"IPAD_129_DISPLAY", "IPAD_APP_STORE"},
	"IPAD_105":          {"IPAD_105_DISPLAY", "IPAD_APP_STORE"},
	"IPAD_97":           {"IPAD_97_DISPLAY", "IPAD_APP_STORE"},
	"WATCH_ULTRA":       {"WATCH_ULTRA", "WATCH_APP_STORE"},
	"WATCH_SERIES_10":   {"WATCH_SERIES_10", "WATCH_APP_STORE"},
	"WATCH_SERIES_7":    {"WATCH_SERIES_7", "WATCH_APP_STORE"},
	"WATCH_SERIES_4":    {"WATCH_SERIES_4", "WATCH_APP_STORE"},
	"WATCH_SERIES_3":    {"WATCH_SERIES_3", "WATCH_APP_STORE"},
	"DESKTOP":           {"DEFAULT", "MAC_APP_STORE"},
	"APPLE_TV":          {"DEFAULT", "TV_APP_STORE"},
	"APPLE_VISION_PRO":  {"DEFAULT", "VISION_PRO_APP_STORE"},
}

// AssetLibraryProfileForSlot maps a screenshot display type or preview type to
// its Asset Library placement profile.
func AssetLibraryProfileForSlot(slot string) (AssetLibraryProfile, bool) {
	canonical := CanonicalScreenshotDisplayTypeForAPI(slot)
	key, iMessage := strings.CutPrefix(canonical, "IMESSAGE_")
	profile, ok := assetLibraryProfilesBySlot[strings.TrimPrefix(key, "APP_")]
	if ok && iMessage {
		profile.Platform = "IMESSAGE_APP_STORE"
	}
	return profile, ok
}
