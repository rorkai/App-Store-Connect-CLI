package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type ScreenshotPlan struct {
	Locale      string   `json:"locale"`
	DisplayType string   `json:"displayType"`
	Files       []string `json:"files"`
}

type ScreenshotUploadResult struct {
	Locale      string                      `json:"locale"`
	DisplayType string                      `json:"displayType"`
	Uploaded    []asc.AssetUploadResultItem `json:"uploaded,omitempty"`
	Skipped     []SkippedItem               `json:"skipped,omitempty"`
}

func discoverScreenshotPlan(screenshotsDir string) ([]ScreenshotPlan, []SkippedItem, error) {
	entries, err := os.ReadDir(screenshotsDir)
	if err != nil {
		return nil, nil, err
	}

	type planKey struct {
		locale      string
		displayType string
	}
	plans := make(map[planKey][]string)
	var skipped []SkippedItem

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		localeName := entry.Name()
		if localeName == "default" {
			continue
		}
		locale, err := normalizeLocale(localeName)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid locale directory %q: %w", localeName, err)
		}
		localeDir := filepath.Join(screenshotsDir, entry.Name())
		files, err := collectScreenshotFiles(localeDir)
		if err != nil {
			return nil, nil, err
		}
		for _, filePath := range files {
			dimensions, err := asc.ReadImageDimensions(filePath)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid screenshot file %q: %w", filePath, err)
			}
			displayType, err := inferScreenshotDisplayTypeFromDimensions(filePath, dimensions.Width, dimensions.Height)
			if err != nil {
				return nil, nil, err
			}
			key := planKey{locale: locale, displayType: displayType}
			plans[key] = append(plans[key], filePath)
		}
	}

	result := make([]ScreenshotPlan, 0, len(plans))
	for key, files := range plans {
		sort.Strings(files)
		result = append(result, ScreenshotPlan{
			Locale:      key.locale,
			DisplayType: key.displayType,
			Files:       files,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Locale == result[j].Locale {
			return result[i].DisplayType < result[j].DisplayType
		}
		return result[i].Locale < result[j].Locale
	})

	return result, skipped, nil
}

func collectScreenshotFiles(localeDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(localeDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to read symlink %q", path)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no screenshots found in %q", localeDir)
	}
	sort.Strings(files)
	return files, nil
}

func inferScreenshotDisplayType(path string) (string, error) {
	width, height, err := readImageDimensions(path)
	if err != nil {
		return "", fmt.Errorf("unable to read screenshot dimensions for %q: %w", path, err)
	}
	return inferScreenshotDisplayTypeFromDimensions(path, width, height)
}

func inferScreenshotDisplayTypeFromDimensions(path string, width, height int) (string, error) {
	hint := inferDisplayTypeFromFilename(path)
	if hint != "" {
		if !asc.IsValidScreenshotDisplayType(hint) {
			return "", fmt.Errorf("unsupported screenshot display type %q for %s", hint, path)
		}
		return hint, nil
	}

	if displayType := inferDisplayTypeFromDimensions(width, height); displayType != "" {
		if !asc.IsValidScreenshotDisplayType(displayType) {
			return "", fmt.Errorf("unsupported screenshot display type %q for %s", displayType, path)
		}
		return displayType, nil
	}

	return "", fmt.Errorf("unable to infer screenshot display type for %q", path)
}

func readImageDimensions(path string) (int, int, error) {
	dimensions, err := asc.ReadImageDimensions(path)
	if err != nil {
		return 0, 0, err
	}
	return dimensions.Width, dimensions.Height, nil
}

func inferDisplayTypeFromFilename(path string) string {
	name := strings.ToLower(filepath.Base(path))
	replacements := map[string]string{
		"iphone 6.9":      "APP_IPHONE_69",
		"iphone6.9":       "APP_IPHONE_69",
		"iphone 6.7":      "APP_IPHONE_67",
		"iphone6.7":       "APP_IPHONE_67",
		"iphone 6.5":      "APP_IPHONE_65",
		"iphone6.5":       "APP_IPHONE_65",
		"iphone 6.1":      "APP_IPHONE_61",
		"iphone6.1":       "APP_IPHONE_61",
		"iphone 5.8":      "APP_IPHONE_58",
		"iphone5.8":       "APP_IPHONE_58",
		"iphone 5.5":      "APP_IPHONE_55",
		"iphone5.5":       "APP_IPHONE_55",
		"iphone 4.7":      "APP_IPHONE_47",
		"iphone4.7":       "APP_IPHONE_47",
		"iphone 4.0":      "APP_IPHONE_40",
		"iphone4.0":       "APP_IPHONE_40",
		"iphone 3.5":      "APP_IPHONE_35",
		"iphone3.5":       "APP_IPHONE_35",
		"ipad 12.9":       "APP_IPAD_PRO_129",
		"ipad12.9":        "APP_IPAD_PRO_129",
		"ipad 11":         "APP_IPAD_PRO_3GEN_11",
		"ipad11":          "APP_IPAD_PRO_3GEN_11",
		"ipad 10.5":       "APP_IPAD_105",
		"ipad10.5":        "APP_IPAD_105",
		"ipad 9.7":        "APP_IPAD_97",
		"ipad9.7":         "APP_IPAD_97",
		"apple tv":        "APP_APPLE_TV",
		"appletv":         "APP_APPLE_TV",
		"vision pro":      "APP_APPLE_VISION_PRO",
		"desktop":         "APP_DESKTOP",
		"mac":             "APP_DESKTOP",
		"watch ultra":     "APP_WATCH_ULTRA",
		"watch series 10": "APP_WATCH_SERIES_10",
		"watch series 7":  "APP_WATCH_SERIES_7",
		"watch series 4":  "APP_WATCH_SERIES_4",
		"watch series 3":  "APP_WATCH_SERIES_3",
	}
	for key, value := range replacements {
		if strings.Contains(name, key) {
			return value
		}
	}
	return ""
}

func inferDisplayTypeFromDimensions(width, height int) string {
	maxDim := width
	minDim := height
	if height > width {
		maxDim = height
		minDim = width
	}
	switch {
	case maxDim == 2688 && minDim == 1242:
		return "APP_IPHONE_65"
	case maxDim == 2778 && minDim == 1284:
		return "APP_IPHONE_65"
	case maxDim == 2868 && minDim == 1320:
		return "APP_IPHONE_69"
	case maxDim == 2736 && minDim == 1260:
		return "APP_IPHONE_69"
	case maxDim == 2796 && minDim == 1290:
		return "APP_IPHONE_67"
	case maxDim == 2622 && minDim == 1206:
		return "APP_IPHONE_61"
	case maxDim == 2556 && minDim == 1179:
		return "APP_IPHONE_61"
	case maxDim == 2532 && minDim == 1170:
		return "APP_IPHONE_58"
	case maxDim == 2436 && minDim == 1125:
		return "APP_IPHONE_58"
	case maxDim == 2340 && minDim == 1080:
		return "APP_IPHONE_58"
	case maxDim == 2208 && minDim == 1242:
		return "APP_IPHONE_55"
	case maxDim == 1334 && minDim == 750:
		return "APP_IPHONE_47"
	case maxDim == 1136 && minDim == 640:
		return "APP_IPHONE_40"
	case maxDim == 960 && minDim == 640:
		return "APP_IPHONE_35"
	case maxDim == 2732 && minDim == 2048:
		return "APP_IPAD_PRO_129"
	case maxDim == 2420 && minDim == 1668:
		return "APP_IPAD_PRO_3GEN_11"
	case maxDim == 2388 && minDim == 1668:
		return "APP_IPAD_PRO_3GEN_11"
	case maxDim == 2360 && minDim == 1640:
		return "APP_IPAD_PRO_3GEN_11"
	case maxDim == 2266 && minDim == 1488:
		return "APP_IPAD_PRO_3GEN_11"
	case maxDim == 2224 && minDim == 1668:
		return "APP_IPAD_105"
	case maxDim == 2048 && minDim == 1536:
		return "APP_IPAD_97"
	case maxDim == 1920 && minDim == 1080:
		return "APP_APPLE_TV"
	default:
		return ""
	}
}
