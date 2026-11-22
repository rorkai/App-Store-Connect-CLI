package screenshotcatalog

import (
	"sort"
	"strings"
)

// Dimension represents one allowed screenshot size.
type Dimension struct {
	Width  int
	Height int
}

func portraitLandscape(width, height int) []Dimension {
	return []Dimension{
		{Width: width, Height: height},
		{Width: height, Height: width},
	}
}

func singleOrientation(width, height int) []Dimension {
	return []Dimension{
		{Width: width, Height: height},
	}
}

func combineDimensions(groups ...[]Dimension) []Dimension {
	var combined []Dimension
	for _, group := range groups {
		combined = append(combined, group...)
	}
	return uniqueSortedDimensions(combined)
}

func uniqueSortedDimensions(dims []Dimension) []Dimension {
	unique := make([]Dimension, 0, len(dims))
	seen := make(map[Dimension]struct{}, len(dims))
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
	iphoneDuoDimensions = combineDimensions(
		portraitLandscape(1398, 2034),
		portraitLandscape(2007, 2853),
	)
	iphone69Dimensions = combineDimensions(
		portraitLandscape(1260, 2736),
		portraitLandscape(1290, 2796),
		portraitLandscape(1320, 2868),
	)
	iphone67Dimensions = combineDimensions(
		portraitLandscape(1260, 2736),
		portraitLandscape(1290, 2796),
		portraitLandscape(1320, 2868),
	)
	iphone61Dimensions = combineDimensions(
		portraitLandscape(1206, 2622),
		portraitLandscape(1179, 2556),
	)
	iphone65Dimensions = combineDimensions(
		portraitLandscape(1242, 2688),
		portraitLandscape(1284, 2778),
	)
	iphone58Dimensions = combineDimensions(
		portraitLandscape(1170, 2532),
		portraitLandscape(1125, 2436),
		portraitLandscape(1080, 2340),
	)
	iphone55Dimensions = portraitLandscape(1242, 2208)
	iphone47Dimensions = portraitLandscape(750, 1334)
	iphone40Dimensions = portraitLandscape(640, 1136)
	iphone35Dimensions = portraitLandscape(640, 960)

	ipadPro3Gen129Dimensions = combineDimensions(
		portraitLandscape(2048, 2732),
		portraitLandscape(2064, 2752),
	)
	ipadPro129Dimensions = portraitLandscape(2048, 2732)
	ipadPro11Dimensions  = combineDimensions(
		portraitLandscape(1488, 2266),
		portraitLandscape(1668, 2388),
		portraitLandscape(1668, 2420),
		portraitLandscape(1640, 2360),
	)
	ipad105Dimensions = portraitLandscape(1668, 2224)
	ipad97Dimensions  = portraitLandscape(1536, 2048)
	desktopDimensions = combineDimensions(
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

var registry = map[string][]Dimension{
	"APP_IPHONE_DUO":                 iphoneDuoDimensions,
	"APP_IPHONE_69":                  iphone69Dimensions,
	"APP_IPHONE_67":                  iphone67Dimensions,
	"APP_IPHONE_61":                  iphone61Dimensions,
	"APP_IPHONE_65":                  iphone65Dimensions,
	"APP_IPHONE_58":                  iphone58Dimensions,
	"APP_IPHONE_55":                  iphone55Dimensions,
	"APP_IPHONE_47":                  iphone47Dimensions,
	"APP_IPHONE_40":                  iphone40Dimensions,
	"APP_IPHONE_35":                  iphone35Dimensions,
	"APP_IPAD_PRO_3GEN_129":          ipadPro3Gen129Dimensions,
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
	"IMESSAGE_APP_IPHONE_DUO":        iphoneDuoDimensions,
	"IMESSAGE_APP_IPHONE_69":         iphone69Dimensions,
	"IMESSAGE_APP_IPHONE_67":         iphone67Dimensions,
	"IMESSAGE_APP_IPHONE_61":         iphone61Dimensions,
	"IMESSAGE_APP_IPHONE_65":         iphone65Dimensions,
	"IMESSAGE_APP_IPHONE_58":         iphone58Dimensions,
	"IMESSAGE_APP_IPHONE_55":         iphone55Dimensions,
	"IMESSAGE_APP_IPHONE_47":         iphone47Dimensions,
	"IMESSAGE_APP_IPHONE_40":         iphone40Dimensions,
	"IMESSAGE_APP_IPAD_PRO_3GEN_129": ipadPro3Gen129Dimensions,
	"IMESSAGE_APP_IPAD_PRO_3GEN_11":  ipadPro11Dimensions,
	"IMESSAGE_APP_IPAD_PRO_129":      ipadPro129Dimensions,
	"IMESSAGE_APP_IPAD_105":          ipad105Dimensions,
	"IMESSAGE_APP_IPAD_97":           ipad97Dimensions,
}

var apiAliases = map[string]string{
	"APP_IPHONE_69":          "APP_IPHONE_67",
	"IMESSAGE_APP_IPHONE_69": "IMESSAGE_APP_IPHONE_67",
}

// supersededDisplayTypes maps a retired display type to the display type App
// Store Connect now serves the same screenshot size from. Both remain valid API
// values, so this is not an alias: it only decides which slot a screenshot
// belongs in when its size matches the retired slot and its successor.
var supersededDisplayTypes = map[string]string{
	"APP_IPAD_PRO_129":          "APP_IPAD_PRO_3GEN_129",
	"IMESSAGE_APP_IPAD_PRO_129": "IMESSAGE_APP_IPAD_PRO_3GEN_129",
}

// SupersededDisplayType returns the display type that replaces a retired one.
func SupersededDisplayType(displayType string) (string, bool) {
	successor, ok := supersededDisplayTypes[strings.ToUpper(strings.TrimSpace(displayType))]
	return successor, ok
}

// PreferCurrentDisplayTypes drops retired display types whose successor is
// already present, preserving input order. Inferring display types from a
// screenshot size can match a retired slot and its successor, but the
// screenshot belongs in exactly one of them.
func PreferCurrentDisplayTypes(displayTypes []string) []string {
	present := make(map[string]struct{}, len(displayTypes))
	for _, displayType := range displayTypes {
		present[strings.ToUpper(strings.TrimSpace(displayType))] = struct{}{}
	}

	preferred := make([]string, 0, len(displayTypes))
	for _, displayType := range displayTypes {
		if successor, ok := SupersededDisplayType(displayType); ok {
			if _, exists := present[successor]; exists {
				continue
			}
		}
		preferred = append(preferred, displayType)
	}
	return preferred
}

// DisplayTypes returns all supported display types in stable order.
func DisplayTypes() []string {
	types := make([]string, 0, len(registry))
	for key := range registry {
		types = append(types, key)
	}
	sort.Strings(types)
	return types
}

// Dimensions returns a copy of dimensions for a display type.
func Dimensions(displayType string) ([]Dimension, bool) {
	dims, ok := registry[displayType]
	if !ok {
		return nil, false
	}
	return append([]Dimension(nil), dims...), true
}

// CanonicalDisplayTypeForAPI converts local aliases to API-supported display types.
func CanonicalDisplayTypeForAPI(displayType string) string {
	normalized := strings.ToUpper(strings.TrimSpace(displayType))
	if mapped, ok := apiAliases[normalized]; ok {
		return mapped
	}
	return normalized
}
