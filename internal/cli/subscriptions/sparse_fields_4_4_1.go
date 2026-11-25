package subscriptions

import (
	"flag"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func normalizeSparseFieldsFlag(d *shared.Deps, fs *flag.FlagSet, next, name, value string, allowed []string) ([]string, error) {
	if strings.TrimSpace(next) != "" && flagWasProvided(fs, name) {
		return nil, d.UsageErrorf("--next cannot be combined with --%s", name)
	}
	return normalizeSelectionFlag(d, fs, value, "--"+name, allowed)
}

func includeRelationshipForFields(fields []string, relationship string) []string {
	if len(fields) == 0 {
		return nil
	}
	return []string{relationship}
}

func appendIncludeForFields(include []string, fields []string, relationship string) []string {
	if len(fields) == 0 || slices.Contains(include, relationship) {
		return include
	}
	return append(include, relationship)
}

func subscriptionPricePointFieldsList() []string {
	return []string{"customerPrice", "proceeds", "proceedsYear2", "territory", "equalizations", "adjustedEqualizations"}
}

func subscriptionPriceFieldsList() []string {
	return shared.IncludedResourceFields("subscriptionPrices")
}
