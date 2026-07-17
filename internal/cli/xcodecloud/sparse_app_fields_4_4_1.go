package xcodecloud

import (
	"flag"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var ciProductAppInAppPurchaseSparseFields441 = []string{"versions"}

var ciProductAppSubscriptionGroupSparseFields441 = []string{"versions"}

var ciProductAppInfoSparseFields441 = []string{"kidsAgeBand"}

func normalizeCiProductAppSparseField(fs *flag.FlagSet, value string, allowed []string, flagName string) ([]string, error) {
	values, err := shared.NormalizeSelection(value, allowed, flagName)
	if err != nil {
		return nil, err
	}
	provided := false
	fs.Visit(func(f *flag.Flag) { provided = provided || f.Name == strings.TrimPrefix(flagName, "--") })
	if provided && len(values) == 0 {
		return nil, fmt.Errorf("%s must not be empty", flagName)
	}
	return values, nil
}

func addCiProductAppInclude(values []string, include string) []string {
	if !shared.HasInclude(values, include) {
		values = append(values, include)
	}
	return values
}
