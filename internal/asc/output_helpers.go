package asc

import "strings"

func compactWhitespace(input string) string {
	clean := sanitizeTerminal(input)
	return strings.Join(strings.Fields(clean), " ")
}

// platformDisplayNames maps API enum values to human-readable names for table/markdown output.
var platformDisplayNames = map[string]string{
	"IOS":       "iOS",
	"MAC_OS":    "macOS",
	"TV_OS":     "tvOS",
	"VISION_OS": "visionOS",
}

// displayPlatform returns a human-readable platform name for table/markdown output.
// Unknown values pass through unchanged.
func displayPlatform(raw string) string {
	if display, ok := platformDisplayNames[raw]; ok {
		return display
	}
	return raw
}
