package shared

import (
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
)

// WarnDeprecatedAppInfoFields warns only for the legacy AppInfo selector.
// AgeRatingDeclaration.kidsAgeBand is a separate, supported attribute.
func WarnDeprecatedAppInfoFields(fields []string, nextURL string) {
	deprecated := slices.Contains(fields, "kidsAgeBand")
	if !deprecated && strings.TrimSpace(nextURL) != "" {
		if parsed, err := url.Parse(nextURL); err == nil {
			for _, selection := range parsed.Query()["fields[appInfos]"] {
				for _, field := range strings.Split(selection, ",") {
					deprecated = deprecated || strings.TrimSpace(field) == "kidsAgeBand"
				}
			}
		}
	}
	if deprecated {
		fmt.Fprintln(os.Stderr, "Warning: AppInfo.kidsAgeBand is deprecated and was removed from the API 4.5 schema; use asc age-rating view --app-info-id ID. This selector is still forwarded for compatibility.")
	}
}
