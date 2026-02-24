package asc

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshotcatalog"
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

// ScreenshotDisplayTypes returns the supported display types in stable order.
func ScreenshotDisplayTypes() []string {
	return screenshotcatalog.DisplayTypes()
}

// ScreenshotDimensions returns a copy of allowed dimensions for a display type.
func ScreenshotDimensions(displayType string) ([]ScreenshotDimension, bool) {
	dims, ok := screenshotcatalog.Dimensions(displayType)
	if !ok {
		return nil, false
	}
	converted := make([]ScreenshotDimension, 0, len(dims))
	for _, dim := range dims {
		converted = append(converted, ScreenshotDimension{
			Width:  dim.Width,
			Height: dim.Height,
		})
	}
	return converted, true
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

// CanonicalScreenshotDisplayTypeForAPI converts local aliases to API-supported display types.
func CanonicalScreenshotDisplayTypeForAPI(displayType string) string {
	return screenshotcatalog.CanonicalDisplayTypeForAPI(displayType)
}

func suggestDisplayTypesForDimensions(width, height int, currentDisplayType string) []string {
	allDisplayTypes := screenshotcatalog.DisplayTypes()
	suggestions := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)

	normalizedCurrent := strings.ToUpper(strings.TrimSpace(currentDisplayType))
	currentFamily := "ANY"
	if strings.HasPrefix(normalizedCurrent, "APP_") {
		currentFamily = "APP"
	}
	if strings.HasPrefix(normalizedCurrent, "IMESSAGE_") {
		currentFamily = "IMESSAGE"
	}
	currentCanonical := CanonicalScreenshotDisplayTypeForAPI(normalizedCurrent)

	for _, displayType := range allDisplayTypes {
		switch currentFamily {
		case "APP":
			if !strings.HasPrefix(displayType, "APP_") {
				continue
			}
		case "IMESSAGE":
			if !strings.HasPrefix(displayType, "IMESSAGE_") {
				continue
			}
		}

		dims, ok := screenshotcatalog.Dimensions(displayType)
		if !ok {
			continue
		}

		for _, dim := range dims {
			if dim.Width != width || dim.Height != height {
				continue
			}
			canonical := CanonicalScreenshotDisplayTypeForAPI(displayType)
			if canonical == currentCanonical {
				break
			}
			if _, exists := seen[canonical]; exists {
				break
			}
			seen[canonical] = struct{}{}
			suggestions = append(suggestions, canonical)
			break
		}
	}

	sort.Strings(suggestions)
	return suggestions
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

	suggestions := suggestDisplayTypesForDimensions(dims.Width, dims.Height, displayType)
	suggestionMessage := ""
	if len(suggestions) > 0 {
		suggestionMessage = fmt.Sprintf(" This size matches: %s.", strings.Join(suggestions, ", "))
	}

	return fmt.Errorf(
		"screenshot %q has unsupported size %dx%d for %s (allowed: %s). See \"asc screenshots sizes --display-type %s\".%s",
		path,
		dims.Width,
		dims.Height,
		displayType,
		formatScreenshotDimensions(allowed),
		displayType,
		suggestionMessage,
	)
}
