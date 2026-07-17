package subscriptions

import (
	"flag"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func normalizeSparseFieldsFlag(fs *flag.FlagSet, next, name, value string, allowed []string) ([]string, error) {
	if strings.TrimSpace(next) != "" && flagWasProvided(fs, name) {
		return nil, shared.UsageErrorf("--next cannot be combined with --%s", name)
	}
	return normalizeSelectionFlag(fs, value, "--"+name, allowed)
}

func validateNextExclusiveFlags(fs *flag.FlagSet, next string, names ...string) error {
	if strings.TrimSpace(next) == "" {
		return nil
	}
	for _, name := range names {
		if flagWasProvided(fs, name) {
			return shared.UsageErrorf("--next cannot be combined with --%s", name)
		}
	}
	return nil
}

func includeRelationshipForFields(fields []string, relationship string) []string {
	if len(fields) == 0 {
		return nil
	}
	return []string{relationship}
}

func appendIncludeForFields(include []string, fields []string, relationship string) []string {
	if len(fields) == 0 || containsString(include, relationship) {
		return include
	}
	return append(include, relationship)
}

func subscriptionGroupFieldsList() []string {
	return []string{"referenceName", "subscriptions", "subscriptionGroupLocalizations", "versions"}
}

func subscriptionPricePointFieldsList() []string {
	return []string{"customerPrice", "proceeds", "proceedsYear2", "territory", "equalizations", "adjustedEqualizations"}
}
