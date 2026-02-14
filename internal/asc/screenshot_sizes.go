package asc

import (
	"fmt"
	"sort"
	"strings"
)

// ScreenshotDimension represents a single allowed screenshot size.
type ScreenshotDimension struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (d ScreenshotDimension) String() string {
	return fmt.Sprintf("%dx%d", d.Width, d.Height)
}

// ScreenshotSizeEntry describes allowed sizes for a display type.
type ScreenshotSizeEntry struct {
	DisplayType string                `json:"displayType"`
	Family      string                `json:"family"`
	Dimensions  []ScreenshotDimension `json:"dimensions"`
}

// ScreenshotSizesResult is the output container for sizes command.
type ScreenshotSizesResult struct {
	Sizes []ScreenshotSizeEntry `json:"sizes"`
}

func portraitLandscape(width, height int) []ScreenshotDimension {
	return []ScreenshotDimension{
		{Width: width, Height: height},
		{Width: height, Height: width},
	}
}

func singleOrientation(width, height int) []ScreenshotDimension {
	return []ScreenshotDimension{
		{Width: width, Height: height},
	}
}

func combineDimensions(groups ...[]ScreenshotDimension) []ScreenshotDimension {
	var combined []ScreenshotDimension
	for _, group := range groups {
		combined = append(combined, group...)
	}
	return uniqueSortedDimensions(combined)
}

func uniqueSortedDimensions(dims []ScreenshotDimension) []ScreenshotDimension {
	unique := make([]ScreenshotDimension, 0, len(dims))
	seen := make(map[ScreenshotDimension]struct{}, len(dims))
	for _, dim := range dims {
		if _, ok := seen[dim]; ok {
			continue
		}
		seen[dim] = struct{}{}
		unique = append(unique, dim)
	}
	sort.Slice(unique, func(i, j int) bool {
		if unique[i].Width == unique[j].Width {
			return unique[i].Height < unique[j].Height
		}
		return unique[i].Width < unique[j].Width
	})
	return unique
}

var (
	iphone69Dimensions = combineDimensions(
		portraitLandscape(1260, 2736),
		portraitLandscape(1290, 2796),
		portraitLandscape(1320, 2868),
		portraitLandscape(1284, 2778),
	)
	iphone67Dimensions = combineDimensions(
		portraitLandscape(1260, 2736),
		portraitLandscape(1290, 2796),
		portraitLandscape(1320, 2868),
		portraitLandscape(1284, 2778),
	)
	iphone61Dimensions = combineDimensions(
		portraitLandscape(1179, 2556),
		portraitLandscape(1170, 2532),
	)
	iphone65Dimensions = portraitLandscape(1242, 2688)
	iphone58Dimensions = portraitLandscape(1125, 2436)
	iphone55Dimensions = portraitLandscape(1242, 2208)
	iphone47Dimensions = portraitLandscape(750, 1334)
	iphone40Dimensions = portraitLandscape(640, 1136)
	iphone35Dimensions = portraitLandscape(640, 960)

	ipadPro129Dimensions = portraitLandscape(2048, 2732)
	ipadPro11Dimensions  = portraitLandscape(1668, 2388)
	ipad105Dimensions    = portraitLandscape(1668, 2224)
	ipad97Dimensions     = portraitLandscape(1536, 2048)
	desktopDimensions    = combineDimensions(
		singleOrientation(1280, 800),
		singleOrientation(1440, 900),
		singleOrientation(2560, 1600),
		singleOrientation(2880, 1800),
	)
	appleTVDimensions    = combineDimensions(singleOrientation(1920, 1080), singleOrientation(3840, 2160))
	visionProDimensions  = singleOrientation(3840, 2160)
	watchUltraDimensions = combineDimensions(
		singleOrientation(422, 514),
		singleOrientation(410, 502),
	)
	watchSeries10Dimensions = singleOrientation(416, 496)
	watchSeries7Dimensions  = singleOrientation(396, 484)
	watchSeries4Dimensions  = singleOrientation(368, 448)
	watchSeries3Dimensions  = singleOrientation(312, 390)
)

var screenshotSizeRegistry = map[string][]ScreenshotDimension{
	"APP_IPHONE_69":                  iphone69Dimensions,
	"APP_IPHONE_67":                  iphone67Dimensions,
	"APP_IPHONE_61":                  iphone61Dimensions,
	"APP_IPHONE_65":                  iphone65Dimensions,
	"APP_IPHONE_58":                  iphone58Dimensions,
	"APP_IPHONE_55":                  iphone55Dimensions,
	"APP_IPHONE_47":                  iphone47Dimensions,
	"APP_IPHONE_40":                  iphone40Dimensions,
	"APP_IPHONE_35":                  iphone35Dimensions,
	"APP_IPAD_PRO_3GEN_129":          ipadPro129Dimensions,
	"APP_IPAD_PRO_3GEN_11":           ipadPro11Dimensions,
	"APP_IPAD_PRO_129":               ipadPro129Dimensions,
	"APP_IPAD_105":                   ipad105Dimensions,
	"APP_IPAD_97":                    ipad97Dimensions,
	"APP_DESKTOP":                    desktopDimensions,
	"APP_WATCH_ULTRA":                watchUltraDimensions,
	"APP_WATCH_SERIES_10":            watchSeries10Dimensions,
	"APP_WATCH_SERIES_7":             watchSeries7Dimensions,
	"APP_WATCH_SERIES_4":             watchSeries4Dimensions,
	"APP_WATCH_SERIES_3":             watchSeries3Dimensions,
	"APP_APPLE_TV":                   appleTVDimensions,
	"APP_APPLE_VISION_PRO":           visionProDimensions,
	"IMESSAGE_APP_IPHONE_69":         iphone69Dimensions,
	"IMESSAGE_APP_IPHONE_67":         iphone67Dimensions,
	"IMESSAGE_APP_IPHONE_61":         iphone61Dimensions,
	"IMESSAGE_APP_IPHONE_65":         iphone65Dimensions,
	"IMESSAGE_APP_IPHONE_58":         iphone58Dimensions,
	"IMESSAGE_APP_IPHONE_55":         iphone55Dimensions,
	"IMESSAGE_APP_IPHONE_47":         iphone47Dimensions,
	"IMESSAGE_APP_IPHONE_40":         iphone40Dimensions,
	"IMESSAGE_APP_IPAD_PRO_3GEN_129": ipadPro129Dimensions,
	"IMESSAGE_APP_IPAD_PRO_3GEN_11":  ipadPro11Dimensions,
	"IMESSAGE_APP_IPAD_PRO_129":      ipadPro129Dimensions,
	"IMESSAGE_APP_IPAD_105":          ipad105Dimensions,
	"IMESSAGE_APP_IPAD_97":           ipad97Dimensions,
}

// ScreenshotDisplayTypes returns the supported display types in stable order.
func ScreenshotDisplayTypes() []string {
	types := make([]string, 0, len(screenshotSizeRegistry))
	for key := range screenshotSizeRegistry {
		types = append(types, key)
	}
	sort.Strings(types)
	return types
}

// ScreenshotDimensions returns a copy of allowed dimensions for a display type.
func ScreenshotDimensions(displayType string) ([]ScreenshotDimension, bool) {
	dims, ok := screenshotSizeRegistry[displayType]
	if !ok {
		return nil, false
	}
	return append([]ScreenshotDimension(nil), dims...), true
}

// ScreenshotSizeEntryForDisplayType returns a catalog entry for a display type.
func ScreenshotSizeEntryForDisplayType(displayType string) (ScreenshotSizeEntry, bool) {
	dims, ok := ScreenshotDimensions(displayType)
	if !ok {
		return ScreenshotSizeEntry{}, false
	}
	return ScreenshotSizeEntry{
		DisplayType: displayType,
		Family:      screenshotFamily(displayType),
		Dimensions:  dims,
	}, true
}

// ScreenshotSizeCatalog returns all display types with their allowed sizes.
func ScreenshotSizeCatalog() []ScreenshotSizeEntry {
	types := ScreenshotDisplayTypes()
	sizes := make([]ScreenshotSizeEntry, 0, len(types))
	for _, displayType := range types {
		if entry, ok := ScreenshotSizeEntryForDisplayType(displayType); ok {
			sizes = append(sizes, entry)
		}
	}
	return sizes
}

func screenshotFamily(displayType string) string {
	switch {
	case strings.HasPrefix(displayType, "IMESSAGE_"):
		return "IMESSAGE"
	case strings.HasPrefix(displayType, "APP_"):
		return "APP"
	default:
		return "UNKNOWN"
	}
}

func formatScreenshotDimensions(dims []ScreenshotDimension) string {
	if len(dims) == 0 {
		return ""
	}
	parts := make([]string, 0, len(dims))
	for _, dim := range dims {
		parts = append(parts, dim.String())
	}
	return strings.Join(parts, ", ")
}

// ValidateScreenshotDimensions checks that the image matches an allowed size.
func ValidateScreenshotDimensions(path, displayType string) error {
	dims, err := ReadImageDimensions(path)
	if err != nil {
		return err
	}
	allowed, ok := ScreenshotDimensions(displayType)
	if !ok {
		return fmt.Errorf("unsupported screenshot display type %q", displayType)
	}
	for _, dim := range allowed {
		if dim.Width == dims.Width && dim.Height == dims.Height {
			return nil
		}
	}
	return fmt.Errorf(
		"screenshot %q has unsupported size %dx%d for %s (allowed: %s). See \"asc screenshots sizes --display-type %s\"",
		path,
		dims.Width,
		dims.Height,
		displayType,
		formatScreenshotDimensions(allowed),
		displayType,
	)
}
