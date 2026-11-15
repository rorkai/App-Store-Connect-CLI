package apps

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func appTagTerritoriesRequested(fields, includes []string, nextURL string) bool {
	if slices.Contains(fields, "territories") || slices.Contains(includes, "territories") {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(nextURL))
	if err != nil {
		return false
	}
	if strings.HasSuffix(parsed.Path, "/territories") {
		return true
	}
	query := parsed.Query()
	if query.Has("fields[territories]") || query.Has("limit[territories]") {
		return true
	}
	for _, key := range []string{"fields[appTags]", "include"} {
		for _, selection := range query[key] {
			for _, value := range strings.Split(selection, ",") {
				if strings.TrimSpace(value) == "territories" {
					return true
				}
			}
		}
	}
	return false
}

func warnAppTagTerritoryDeprecation(d *shared.Deps) {
	fmt.Fprintln(d.IO.ErrOut, "Warning: App-tag territories are deprecated in API 4.5; remove territory selections and lookups. Requests are still forwarded for compatibility.")
}
