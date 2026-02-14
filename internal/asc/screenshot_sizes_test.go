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

func TestValidateScreenshotDimensionsAcceptsLatestLargeIPhoneSizes(t *testing.T) {
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

func TestScreenshotSizeEntryIncludesLatestLargeIPhoneDimensions(t *testing.T) {
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
