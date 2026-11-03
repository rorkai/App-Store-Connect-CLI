package shared

import (
	"flag"
	"strings"
)

const maxPlacementsLimit = 50

// LocalizationIncludeFlags holds the --include and --placements-limit flags of
// a localization read.
type LocalizationIncludeFlags struct {
	fs              *flag.FlagSet
	allowed         []string
	include         *OnceCSVValue
	placementsLimit *int
}

// BindLocalizationIncludeFlags registers --include and --placements-limit.
func BindLocalizationIncludeFlags(fs *flag.FlagSet, usage string, allowed []string) *LocalizationIncludeFlags {
	return &LocalizationIncludeFlags{
		fs:              fs,
		allowed:         allowed,
		include:         BindOnceCSVFlag(fs, "include", usage+", comma-separated: "+strings.Join(allowed, ", ")),
		placementsLimit: fs.Int("placements-limit", 0, "Maximum included placements (1-50); requires --include placements"),
	}
}

// Provided reports whether --include was set.
func (f *LocalizationIncludeFlags) Provided() bool {
	return f.include.Provided()
}

// Resolve validates the flags and returns the include list and placements limit.
func (f *LocalizationIncludeFlags) Resolve() ([]string, int, error) {
	include, err := NormalizeSelection(f.include.String(), f.allowed, "--include")
	if err != nil {
		return nil, 0, UsageError(err.Error())
	}
	if f.include.Provided() && len(include) == 0 {
		return nil, 0, UsageError("--include must not be empty")
	}
	limitProvided := false
	f.fs.Visit(func(fl *flag.Flag) {
		limitProvided = limitProvided || fl.Name == "placements-limit"
	})
	if limitProvided {
		if *f.placementsLimit < 1 || *f.placementsLimit > maxPlacementsLimit {
			return nil, 0, UsageError("--placements-limit must be between 1 and 50")
		}
		if !HasInclude(include, "placements") {
			return nil, 0, UsageError("--placements-limit requires --include placements")
		}
	}
	return include, *f.placementsLimit, nil
}
