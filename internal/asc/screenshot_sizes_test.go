package asc

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestValidateScreenshotDimensionsValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "valid.png")
	writePNG(t, path, 640, 960)

	if err := ValidateScreenshotDimensions(path, "APP_IPHONE_35"); err != nil {
		t.Fatalf("expected valid dimensions, got %v", err)
	}
}

func TestValidateScreenshotDimensionsInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.png")
	writePNG(t, path, 100, 100)

	err := ValidateScreenshotDimensions(path, "APP_IPHONE_35")
	if err == nil {
		t.Fatal("expected dimension validation error, got nil")
	}
	message := err.Error()
	if !strings.Contains(message, "100x100") {
		t.Fatalf("expected actual size in error, got %q", message)
	}
	if !strings.Contains(message, "640x960") {
		t.Fatalf("expected allowed size in error, got %q", message)
	}
	if !strings.Contains(message, "asc screenshots sizes") {
		t.Fatalf("expected hint in error, got %q", message)
	}
}

func TestValidateScreenshotDimensionsSuggestsMatchingDisplayType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known-wrong-type.png")
	writePNG(t, path, 1206, 2622)

	err := ValidateScreenshotDimensions(path, "APP_IPHONE_67")
	if err == nil {
		t.Fatal("expected dimension validation error, got nil")
	}
	message := err.Error()
	if !strings.Contains(message, "This size matches: APP_IPHONE_61") {
		t.Fatalf("expected display type suggestion in error, got %q", message)
	}
}

func TestValidateScreenshotDimensionsAcceptsLatestIPhone67Sizes(t *testing.T) {
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "1260x2736 portrait", width: 1260, height: 2736},
		{name: "2736x1260 landscape", width: 2736, height: 1260},
		{name: "1320x2868 portrait", width: 1320, height: 2868},
		{name: "2868x1320 landscape", width: 2868, height: 1320},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "latest-large.png")
			writePNG(t, path, tc.width, tc.height)

			if err := ValidateScreenshotDimensions(path, "APP_IPHONE_67"); err != nil {
				t.Fatalf("expected dimensions %dx%d to be valid for APP_IPHONE_67, got %v", tc.width, tc.height, err)
			}
		})
	}
}

func TestValidateScreenshotDimensionsAcceptsLatestIPhone61Sizes(t *testing.T) {
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "1206x2622 portrait", width: 1206, height: 2622},
		{name: "2622x1206 landscape", width: 2622, height: 1206},
		{name: "1179x2556 portrait", width: 1179, height: 2556},
		{name: "2556x1179 landscape", width: 2556, height: 1179},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "latest-61.png")
			writePNG(t, path, tc.width, tc.height)

			if err := ValidateScreenshotDimensions(path, "APP_IPHONE_61"); err != nil {
				t.Fatalf("expected dimensions %dx%d to be valid for APP_IPHONE_61, got %v", tc.width, tc.height, err)
			}
		})
	}
}

func TestValidateScreenshotDimensionsAcceptsIPhone65ConsolidatedSlotSizes(t *testing.T) {
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "1242x2688 portrait", width: 1242, height: 2688},
		{name: "2688x1242 landscape", width: 2688, height: 1242},
		{name: "1284x2778 portrait", width: 1284, height: 2778},
		{name: "2778x1284 landscape", width: 2778, height: 1284},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "iphone65-consolidated.png")
			writePNG(t, path, tc.width, tc.height)

			if err := ValidateScreenshotDimensions(path, "APP_IPHONE_65"); err != nil {
				t.Fatalf("expected dimensions %dx%d to be valid for APP_IPHONE_65, got %v", tc.width, tc.height, err)
			}
		})
	}
}

func TestValidateScreenshotDimensionsAcceptsLatestIPhone58Sizes(t *testing.T) {
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "1170x2532 portrait", width: 1170, height: 2532},
		{name: "2532x1170 landscape", width: 2532, height: 1170},
		{name: "1125x2436 portrait", width: 1125, height: 2436},
		{name: "2436x1125 landscape", width: 2436, height: 1125},
		{name: "1080x2340 portrait", width: 1080, height: 2340},
		{name: "2340x1080 landscape", width: 2340, height: 1080},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "latest-58.png")
			writePNG(t, path, tc.width, tc.height)

			if err := ValidateScreenshotDimensions(path, "APP_IPHONE_58"); err != nil {
				t.Fatalf("expected dimensions %dx%d to be valid for APP_IPHONE_58, got %v", tc.width, tc.height, err)
			}
		})
	}
}

func TestScreenshotDisplayTypesMatchOpenAPI(t *testing.T) {
	specTypes := openAPIScreenshotDisplayTypes(t)
	codeTypes := ScreenshotDisplayTypes()
	sort.Strings(codeTypes)

	missingFromCode := differenceStrings(specTypes, codeTypes)
	if len(missingFromCode) > 0 {
		t.Fatalf("missing screenshot display types from OpenAPI: %v", missingFromCode)
	}

	extrasInCode := differenceStrings(codeTypes, specTypes)
	allowedExtras := []string{"APP_IPHONE_69", "IMESSAGE_APP_IPHONE_69"}
	unexpectedExtras := differenceStrings(extrasInCode, allowedExtras)
	if len(unexpectedExtras) > 0 {
		t.Fatalf("unexpected screenshot display types not in OpenAPI: %v", unexpectedExtras)
	}
}

func TestCanonicalScreenshotDisplayTypeForAPI(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "iphone 69 canonicalizes to 67", input: "APP_IPHONE_69", want: "APP_IPHONE_67"},
		{name: "imessage iphone 69 canonicalizes to 67", input: "IMESSAGE_APP_IPHONE_69", want: "IMESSAGE_APP_IPHONE_67"},
		{name: "existing API type unchanged", input: "APP_IPHONE_61", want: "APP_IPHONE_61"},
		{name: "normalizes case and whitespace", input: "  app_iphone_61 ", want: "APP_IPHONE_61"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := CanonicalScreenshotDisplayTypeForAPI(tc.input)
			if got != tc.want {
				t.Fatalf("CanonicalScreenshotDisplayTypeForAPI(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestCanonicalIPhone69HasSameDimensionsAsIPhone67(t *testing.T) {
	iphone69, ok := ScreenshotDimensions("APP_IPHONE_69")
	if !ok {
		t.Fatal("expected APP_IPHONE_69 dimensions")
	}
	iphone67, ok := ScreenshotDimensions("APP_IPHONE_67")
	if !ok {
		t.Fatal("expected APP_IPHONE_67 dimensions")
	}

	if len(iphone69) != len(iphone67) {
		t.Fatalf("expected APP_IPHONE_69 and APP_IPHONE_67 to have same number of dimensions, got %d and %d", len(iphone69), len(iphone67))
	}
	for i := range iphone67 {
		if iphone69[i] != iphone67[i] {
			t.Fatalf("expected APP_IPHONE_69 dimensions to match APP_IPHONE_67, got %v and %v", iphone69, iphone67)
		}
	}
}

func TestScreenshotSizeEntryIncludesLatestIPhone67Dimensions(t *testing.T) {
	entry, ok := ScreenshotSizeEntryForDisplayType("APP_IPHONE_67")
	if !ok {
		t.Fatal("expected APP_IPHONE_67 entry in screenshot size catalog")
	}

	expected := []ScreenshotDimension{
		{Width: 1260, Height: 2736},
		{Width: 2736, Height: 1260},
		{Width: 1290, Height: 2796},
		{Width: 2796, Height: 1290},
		{Width: 1320, Height: 2868},
		{Width: 2868, Height: 1320},
	}
	for _, dim := range expected {
		if !containsScreenshotDimension(entry.Dimensions, dim) {
			t.Fatalf("expected APP_IPHONE_67 to include %s, got %v", dim.String(), entry.Dimensions)
		}
	}
}

func TestScreenshotSizeEntryIncludesLatestIPhone61Dimensions(t *testing.T) {
	entry, ok := ScreenshotSizeEntryForDisplayType("APP_IPHONE_61")
	if !ok {
		t.Fatal("expected APP_IPHONE_61 entry in screenshot size catalog")
	}

	expected := []ScreenshotDimension{
		{Width: 1206, Height: 2622},
		{Width: 2622, Height: 1206},
		{Width: 1179, Height: 2556},
		{Width: 2556, Height: 1179},
	}
	for _, dim := range expected {
		if !containsScreenshotDimension(entry.Dimensions, dim) {
			t.Fatalf("expected APP_IPHONE_61 to include %s, got %v", dim.String(), entry.Dimensions)
		}
	}
}

func TestScreenshotSizeEntryIncludesIPhone65ConsolidatedDimensions(t *testing.T) {
	entry, ok := ScreenshotSizeEntryForDisplayType("APP_IPHONE_65")
	if !ok {
		t.Fatal("expected APP_IPHONE_65 entry in screenshot size catalog")
	}

	expected := []ScreenshotDimension{
		{Width: 1242, Height: 2688},
		{Width: 2688, Height: 1242},
		{Width: 1284, Height: 2778},
		{Width: 2778, Height: 1284},
	}
	for _, dim := range expected {
		if !containsScreenshotDimension(entry.Dimensions, dim) {
			t.Fatalf("expected APP_IPHONE_65 to include %s, got %v", dim.String(), entry.Dimensions)
		}
	}
}

func TestScreenshotSizeEntryIncludesLatestIPhone58Dimensions(t *testing.T) {
	entry, ok := ScreenshotSizeEntryForDisplayType("APP_IPHONE_58")
	if !ok {
		t.Fatal("expected APP_IPHONE_58 entry in screenshot size catalog")
	}

	expected := []ScreenshotDimension{
		{Width: 1170, Height: 2532},
		{Width: 2532, Height: 1170},
		{Width: 1125, Height: 2436},
		{Width: 2436, Height: 1125},
		{Width: 1080, Height: 2340},
		{Width: 2340, Height: 1080},
	}
	for _, dim := range expected {
		if !containsScreenshotDimension(entry.Dimensions, dim) {
			t.Fatalf("expected APP_IPHONE_58 to include %s, got %v", dim.String(), entry.Dimensions)
		}
	}
}

func TestValidateScreenshotDimensionsAcceptsIPadPro11LatestSizes(t *testing.T) {
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "1488x2266 portrait", width: 1488, height: 2266},
		{name: "2266x1488 landscape", width: 2266, height: 1488},
		{name: "1668x2388 portrait", width: 1668, height: 2388},
		{name: "2388x1668 landscape", width: 2388, height: 1668},
		{name: "1668x2420 portrait", width: 1668, height: 2420},
		{name: "2420x1668 landscape", width: 2420, height: 1668},
		{name: "1640x2360 portrait", width: 1640, height: 2360},
		{name: "2360x1640 landscape", width: 2360, height: 1640},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "ipad-pro-11-latest.png")
			writePNG(t, path, tc.width, tc.height)

			if err := ValidateScreenshotDimensions(path, "APP_IPAD_PRO_3GEN_11"); err != nil {
				t.Fatalf("expected dimensions %dx%d to be valid for APP_IPAD_PRO_3GEN_11, got %v", tc.width, tc.height, err)
			}
		})
	}
}

func TestScreenshotSizeEntryIncludesIPadPro11Dimensions(t *testing.T) {
	entry, ok := ScreenshotSizeEntryForDisplayType("APP_IPAD_PRO_3GEN_11")
	if !ok {
		t.Fatal("expected APP_IPAD_PRO_3GEN_11 entry in screenshot size catalog")
	}

	expected := []ScreenshotDimension{
		{Width: 1488, Height: 2266},
		{Width: 2266, Height: 1488},
		{Width: 1668, Height: 2388},
		{Width: 2388, Height: 1668},
		{Width: 1668, Height: 2420},
		{Width: 2420, Height: 1668},
		{Width: 1640, Height: 2360},
		{Width: 2360, Height: 1640},
	}
	for _, dim := range expected {
		if !containsScreenshotDimension(entry.Dimensions, dim) {
			t.Fatalf("expected APP_IPAD_PRO_3GEN_11 to include %s, got %v", dim.String(), entry.Dimensions)
		}
	}
}

func TestScreenshotSizeEntryIncludesIPhone69Dimensions(t *testing.T) {
	entry, ok := ScreenshotSizeEntryForDisplayType("APP_IPHONE_69")
	if !ok {
		t.Fatal("expected APP_IPHONE_69 entry in screenshot size catalog")
	}

	expected := []ScreenshotDimension{
		{Width: 1260, Height: 2736},
		{Width: 2736, Height: 1260},
		{Width: 1290, Height: 2796},
		{Width: 2796, Height: 1290},
		{Width: 1320, Height: 2868},
		{Width: 2868, Height: 1320},
	}
	for _, dim := range expected {
		if !containsScreenshotDimension(entry.Dimensions, dim) {
			t.Fatalf("expected APP_IPHONE_69 to include %s, got %v", dim.String(), entry.Dimensions)
		}
	}
}

func TestScreenshotSizeEntryIncludesIPadPro129M5Dimensions(t *testing.T) {
	entry, ok := ScreenshotSizeEntryForDisplayType("APP_IPAD_PRO_3GEN_129")
	if !ok {
		t.Fatal("expected APP_IPAD_PRO_3GEN_129 entry in screenshot size catalog")
	}

	expected := []ScreenshotDimension{
		{Width: 2048, Height: 2732},
		{Width: 2732, Height: 2048},
		{Width: 2064, Height: 2752},
		{Width: 2752, Height: 2064},
	}
	for _, dim := range expected {
		if !containsScreenshotDimension(entry.Dimensions, dim) {
			t.Fatalf("expected APP_IPAD_PRO_3GEN_129 to include %s, got %v", dim.String(), entry.Dimensions)
		}
	}
}

func TestScreenshotSizeEntryIncludesMacDesktopDimensions(t *testing.T) {
	entry, ok := ScreenshotSizeEntryForDisplayType("APP_DESKTOP")
	if !ok {
		t.Fatal("expected APP_DESKTOP entry in screenshot size catalog")
	}

	expected := []ScreenshotDimension{
		{Width: 1280, Height: 800},
		{Width: 1440, Height: 900},
		{Width: 2560, Height: 1600},
		{Width: 2880, Height: 1800},
	}
	for _, dim := range expected {
		if !containsScreenshotDimension(entry.Dimensions, dim) {
			t.Fatalf("expected APP_DESKTOP to include %s, got %v", dim.String(), entry.Dimensions)
		}
	}
}

func TestScreenshotSizeEntryIncludesAppleWatchDimensions(t *testing.T) {
	testCases := []struct {
		displayType string
		expected    []ScreenshotDimension
	}{
		{
			displayType: "APP_WATCH_ULTRA",
			expected: []ScreenshotDimension{
				{Width: 422, Height: 514},
				{Width: 410, Height: 502},
			},
		},
		{
			displayType: "APP_WATCH_SERIES_10",
			expected: []ScreenshotDimension{
				{Width: 416, Height: 496},
			},
		},
		{
			displayType: "APP_WATCH_SERIES_7",
			expected: []ScreenshotDimension{
				{Width: 396, Height: 484},
			},
		},
		{
			displayType: "APP_WATCH_SERIES_4",
			expected: []ScreenshotDimension{
				{Width: 368, Height: 448},
			},
		},
		{
			displayType: "APP_WATCH_SERIES_3",
			expected: []ScreenshotDimension{
				{Width: 312, Height: 390},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.displayType, func(t *testing.T) {
			entry, ok := ScreenshotSizeEntryForDisplayType(tc.displayType)
			if !ok {
				t.Fatalf("expected %s entry in screenshot size catalog", tc.displayType)
			}
			for _, dim := range tc.expected {
				if !containsScreenshotDimension(entry.Dimensions, dim) {
					t.Fatalf("expected %s to include %s, got %v", tc.displayType, dim.String(), entry.Dimensions)
				}
			}
		})
	}
}

func TestScreenshotSizeEntryIncludesAppleTVAndVisionDimensions(t *testing.T) {
	tv, ok := ScreenshotSizeEntryForDisplayType("APP_APPLE_TV")
	if !ok {
		t.Fatal("expected APP_APPLE_TV entry in screenshot size catalog")
	}
	tvExpected := []ScreenshotDimension{
		{Width: 1920, Height: 1080},
		{Width: 3840, Height: 2160},
	}
	for _, dim := range tvExpected {
		if !containsScreenshotDimension(tv.Dimensions, dim) {
			t.Fatalf("expected APP_APPLE_TV to include %s, got %v", dim.String(), tv.Dimensions)
		}
	}

	vision, ok := ScreenshotSizeEntryForDisplayType("APP_APPLE_VISION_PRO")
	if !ok {
		t.Fatal("expected APP_APPLE_VISION_PRO entry in screenshot size catalog")
	}
	if !containsScreenshotDimension(vision.Dimensions, ScreenshotDimension{Width: 3840, Height: 2160}) {
		t.Fatalf("expected APP_APPLE_VISION_PRO to include 3840x2160, got %v", vision.Dimensions)
	}
}

func TestValidateScreenshotDimensionsAcceptsMacWatchTVVisionSizes(t *testing.T) {
	testCases := []struct {
		name        string
		displayType string
		width       int
		height      int
	}{
		{name: "mac desktop 2880x1800", displayType: "APP_DESKTOP", width: 2880, height: 1800},
		{name: "watch ultra 422x514", displayType: "APP_WATCH_ULTRA", width: 422, height: 514},
		{name: "watch series 10 416x496", displayType: "APP_WATCH_SERIES_10", width: 416, height: 496},
		{name: "watch series 7 396x484", displayType: "APP_WATCH_SERIES_7", width: 396, height: 484},
		{name: "watch series 4 368x448", displayType: "APP_WATCH_SERIES_4", width: 368, height: 448},
		{name: "watch series 3 312x390", displayType: "APP_WATCH_SERIES_3", width: 312, height: 390},
		{name: "apple tv 3840x2160", displayType: "APP_APPLE_TV", width: 3840, height: 2160},
		{name: "vision pro 3840x2160", displayType: "APP_APPLE_VISION_PRO", width: 3840, height: 2160},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "platform-size.png")
			writePNG(t, path, tc.width, tc.height)

			if err := ValidateScreenshotDimensions(path, tc.displayType); err != nil {
				t.Fatalf("expected dimensions %dx%d to be valid for %s, got %v", tc.width, tc.height, tc.displayType, err)
			}
		})
	}
}

func TestValidateScreenshotDimensionsRejectsLegacyWatchSizes(t *testing.T) {
	testCases := []struct {
		name        string
		displayType string
		width       int
		height      int
	}{
		{name: "legacy series 10 374x446", displayType: "APP_WATCH_SERIES_10", width: 374, height: 446},
		{name: "legacy series 7 352x430", displayType: "APP_WATCH_SERIES_7", width: 352, height: 430},
		{name: "legacy series 4 324x394", displayType: "APP_WATCH_SERIES_4", width: 324, height: 394},
		{name: "legacy series 3 272x340", displayType: "APP_WATCH_SERIES_3", width: 272, height: 340},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "legacy-watch.png")
			writePNG(t, path, tc.width, tc.height)

			err := ValidateScreenshotDimensions(path, tc.displayType)
			if err == nil {
				t.Fatalf("expected dimensions %dx%d to be rejected for %s", tc.width, tc.height, tc.displayType)
			}
		})
	}
}

func TestValidateScreenshotDimensionsAcceptsIPadPro129M5Size(t *testing.T) {
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "2064x2752 portrait", width: 2064, height: 2752},
		{name: "2752x2064 landscape", width: 2752, height: 2064},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "ipad-pro-129-m5.png")
			writePNG(t, path, tc.width, tc.height)

			if err := ValidateScreenshotDimensions(path, "APP_IPAD_PRO_3GEN_129"); err != nil {
				t.Fatalf("expected dimensions %dx%d to be valid for APP_IPAD_PRO_3GEN_129, got %v", tc.width, tc.height, err)
			}
		})
	}
}

func openAPIScreenshotDisplayTypes(t *testing.T) []string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	path := filepath.Join(root, "docs", "openapi", "latest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}

	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Enum []string `json:"enum"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse openapi: %v", err)
	}
	entry, ok := spec.Components.Schemas["ScreenshotDisplayType"]
	if !ok || len(entry.Enum) == 0 {
		t.Fatal("missing ScreenshotDisplayType enum in OpenAPI")
	}
	enum := append([]string(nil), entry.Enum...)
	sort.Strings(enum)
	return enum
}

func writePNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

func containsScreenshotDimension(dims []ScreenshotDimension, target ScreenshotDimension) bool {
	for _, dim := range dims {
		if dim == target {
			return true
		}
	}
	return false
}

func differenceStrings(left, right []string) []string {
	if len(left) == 0 {
		return nil
	}
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range right {
		rightSet[value] = struct{}{}
	}
	var diff []string
	for _, value := range left {
		if _, ok := rightSet[value]; !ok {
			diff = append(diff, value)
		}
	}
	return diff
}
