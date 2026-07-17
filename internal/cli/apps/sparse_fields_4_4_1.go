package apps

import (
	"flag"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var appInfoSparseFields441 = []string{"kidsAgeBand"}

var ageRatingSparseFields441 = []string{
	"socialMedia",
	"socialMediaAgeRestricted",
}

var appInAppPurchaseSparseFields441 = []string{"versions"}

var appSubscriptionGroupSparseFields441 = []string{"versions"}

func normalizeSparseField(fs *flag.FlagSet, value string, allowed []string, flagName string) ([]string, error) {
	values, err := shared.NormalizeSelection(value, allowed, flagName)
	if err != nil {
		return nil, err
	}
	if _, provided := appFlagWasProvided(fs, strings.TrimPrefix(flagName, "--")); provided && len(values) == 0 {
		return nil, fmt.Errorf("%s must not be empty", flagName)
	}
	return values, nil
}

func addInclude(values []string, include string) []string {
	if !shared.HasInclude(values, include) {
		values = append(values, include)
	}
	return values
}

func appFlagWasProvided(fs *flag.FlagSet, names ...string) (string, bool) {
	provided := make(map[string]bool, len(names))
	fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	for _, name := range names {
		if provided[name] {
			return "--" + strings.TrimPrefix(name, "--"), true
		}
	}
	return "", false
}
