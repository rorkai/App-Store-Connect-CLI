package validation

import "testing"

func TestScreenshotChecks_Mismatch(t *testing.T) {
	sets := []ScreenshotSet{
		{
			ID:          "set-1",
			DisplayType: "APP_IPHONE_65",
			Locale:      "en-US",
			Screenshots: []Screenshot{
				{ID: "shot-1", FileName: "shot.png", Width: 100, Height: 100},
			},
		},
	}

	checks := screenshotChecks("IOS", sets)
	if !hasCheckID(checks, "screenshots.dimension_mismatch") {
		t.Fatalf("expected dimension mismatch check")
	}
}

func TestScreenshotChecks_Pass(t *testing.T) {
	sets := []ScreenshotSet{
		{
			ID:          "set-1",
			DisplayType: "APP_IPHONE_65",
			Locale:      "en-US",
			Screenshots: []Screenshot{
				{ID: "shot-1", FileName: "shot.png", Width: 1242, Height: 2688},
			},
		},
	}

	checks := screenshotChecks("IOS", sets)
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d", len(checks))
	}
}

func TestScreenshotChecks_PassIPhone65ConsolidatedSlot(t *testing.T) {
	sets := []ScreenshotSet{
		{
			ID:          "set-1",
			DisplayType: "APP_IPHONE_65",
			Locale:      "en-US",
			Screenshots: []Screenshot{
				{ID: "shot-1", FileName: "shot-1.png", Width: 1242, Height: 2688},
				{ID: "shot-2", FileName: "shot-2.png", Width: 1284, Height: 2778},
			},
		},
	}

	checks := screenshotChecks("IOS", sets)
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestScreenshotChecks_PassLatestLargeIPhoneSizes(t *testing.T) {
	sets := []ScreenshotSet{
		{
			ID:          "set-1",
			DisplayType: "APP_IPHONE_67",
			Locale:      "en-US",
			Screenshots: []Screenshot{
				{ID: "shot-1", FileName: "shot-1.png", Width: 1260, Height: 2736},
				{ID: "shot-2", FileName: "shot-2.png", Width: 1320, Height: 2868},
			},
		},
	}

	checks := screenshotChecks("IOS", sets)
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestScreenshotChecks_PassLatestIPhone61Size(t *testing.T) {
	sets := []ScreenshotSet{
		{
			ID:          "set-1",
			DisplayType: "APP_IPHONE_61",
			Locale:      "en-US",
			Screenshots: []Screenshot{
				{ID: "shot-1", FileName: "shot-1.png", Width: 1206, Height: 2622},
			},
		},
	}

	checks := screenshotChecks("IOS", sets)
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestScreenshotChecks_PassLatestIPhone58And65AndIPad11Sizes(t *testing.T) {
	sets := []ScreenshotSet{
		{
			ID:          "set-58",
			DisplayType: "APP_IPHONE_58",
			Locale:      "en-US",
			Screenshots: []Screenshot{
				{ID: "shot-58", FileName: "shot-58.png", Width: 1170, Height: 2532},
			},
		},
		{
			ID:          "set-65",
			DisplayType: "APP_IPHONE_65",
			Locale:      "en-US",
			Screenshots: []Screenshot{
				{ID: "shot-65", FileName: "shot-65.png", Width: 1284, Height: 2778},
			},
		},
		{
			ID:          "set-ipad11",
			DisplayType: "APP_IPAD_PRO_3GEN_11",
			Locale:      "en-US",
			Screenshots: []Screenshot{
				{ID: "shot-ipad11", FileName: "shot-ipad11.png", Width: 1488, Height: 2266},
			},
		},
	}

	checks := screenshotChecks("IOS", sets)
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestScreenshotChecks_PassIPadPro129M5Size(t *testing.T) {
	sets := []ScreenshotSet{
		{
			ID:          "set-1",
			DisplayType: "APP_IPAD_PRO_3GEN_129",
			Locale:      "en-US",
			Screenshots: []Screenshot{
				{ID: "shot-1", FileName: "shot-1.png", Width: 2064, Height: 2752},
				{ID: "shot-2", FileName: "shot-2.png", Width: 2752, Height: 2064},
			},
		},
	}

	checks := screenshotChecks("IOS", sets)
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestScreenshotChecks_PassDesktopAndWatchUltraNewestSizes(t *testing.T) {
	sets := []ScreenshotSet{
		{
			ID:          "set-mac",
			DisplayType: "APP_DESKTOP",
			Locale:      "en-US",
			Screenshots: []Screenshot{
				{ID: "shot-mac", FileName: "mac.png", Width: 2880, Height: 1800},
			},
		},
		{
			ID:          "set-watch",
			DisplayType: "APP_WATCH_ULTRA",
			Locale:      "en-US",
			Screenshots: []Screenshot{
				{ID: "shot-watch", FileName: "watch.png", Width: 422, Height: 514},
			},
		},
	}

	checks := screenshotChecks("IOS", sets)
	if len(checks) != 1 {
		t.Fatalf("expected one platform mismatch check for APP_DESKTOP under IOS, got %d (%v)", len(checks), checks)
	}
	if checks[0].ID != "screenshots.display_type_platform_mismatch" {
		t.Fatalf("expected platform mismatch check, got %s", checks[0].ID)
	}

	iosOnly := screenshotChecks("IOS", []ScreenshotSet{sets[1]})
	if len(iosOnly) != 0 {
		t.Fatalf("expected no checks for watch ultra IOS set, got %d (%v)", len(iosOnly), iosOnly)
	}

	macOnly := screenshotChecks("MAC_OS", []ScreenshotSet{sets[0]})
	if len(macOnly) != 0 {
		t.Fatalf("expected no checks for desktop MAC_OS set, got %d (%v)", len(macOnly), macOnly)
	}
}

func TestScreenshotPresenceChecks_NoSets(t *testing.T) {
	versionLocs := []VersionLocalization{
		{ID: "ver-loc-1", Locale: "en-US"},
	}

	checks := screenshotPresenceChecks("en-US", versionLocs, nil)
	if !hasCheckID(checks, "screenshots.required.any") {
		t.Fatalf("expected screenshots.required.any check")
	}
}

func TestScreenshotPresenceChecks_MissingSetsForLocalization(t *testing.T) {
	versionLocs := []VersionLocalization{
		{ID: "ver-loc-en", Locale: "en-US"},
		{ID: "ver-loc-fr", Locale: "fr-FR"},
	}
	sets := []ScreenshotSet{
		{
			ID:             "set-fr-1",
			DisplayType:    "APP_IPHONE_65",
			Locale:         "fr-FR",
			LocalizationID: "ver-loc-fr",
			Screenshots: []Screenshot{
				{ID: "shot-1", FileName: "shot.png", Width: 1242, Height: 2688},
			},
		},
	}

	checks := screenshotPresenceChecks("en-US", versionLocs, sets)
	if !hasCheckID(checks, "screenshots.required.localization_missing_sets") {
		t.Fatalf("expected screenshots.required.localization_missing_sets check")
	}

	foundEN := false
	for _, c := range checks {
		if c.ID == "screenshots.required.localization_missing_sets" && c.Locale == "en-US" {
			foundEN = true
			break
		}
	}
	if !foundEN {
		t.Fatalf("expected missing-sets check for en-US, got %v", checks)
	}
}

func TestScreenshotPresenceChecks_EmptySet(t *testing.T) {
	versionLocs := []VersionLocalization{
		{ID: "ver-loc-1", Locale: "en-US"},
	}
	sets := []ScreenshotSet{
		{
			ID:             "set-1",
			DisplayType:    "APP_IPHONE_65",
			Locale:         "en-US",
			LocalizationID: "ver-loc-1",
			Screenshots:    nil,
		},
	}

	checks := screenshotPresenceChecks("en-US", versionLocs, sets)
	if !hasCheckID(checks, "screenshots.required.set_nonempty") {
		t.Fatalf("expected screenshots.required.set_nonempty check")
	}
}

func TestScreenshotPresenceChecks_Pass(t *testing.T) {
	versionLocs := []VersionLocalization{
		{ID: "ver-loc-1", Locale: "en-US"},
	}
	sets := []ScreenshotSet{
		{
			ID:             "set-1",
			DisplayType:    "APP_IPHONE_65",
			Locale:         "en-US",
			LocalizationID: "ver-loc-1",
			Screenshots: []Screenshot{
				{ID: "shot-1", FileName: "shot.png", Width: 1242, Height: 2688},
			},
		},
	}

	checks := screenshotPresenceChecks("en-US", versionLocs, sets)
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}
