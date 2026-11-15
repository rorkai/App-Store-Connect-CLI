package apps

import (
	"flag"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func normalizeSparseField(fs *flag.FlagSet, value string, allowed []string, flagName string) ([]string, error) {
	values, err := shared.NormalizeSelection(value, allowed, flagName)
	if err != nil {
		return nil, err
	}
	if appFlagWasProvided(fs, strings.TrimPrefix(flagName, "--")) && len(values) == 0 {
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

func appFlagWasProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
}
