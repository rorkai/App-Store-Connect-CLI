package gamecenter

import (
	"flag"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// validateGameCenterReplacementConfirm enforces --confirm on relationship
// replacements, which can drop existing relationships.
func validateGameCenterReplacementConfirm(fs *flag.FlagSet, confirmed bool) error {
	if confirmed {
		return nil
	}
	if flagPassed(fs, "confirm") {
		return shared.UsageError("--confirm must be true when specified")
	}
	return shared.RequiredFlag("--confirm")
}
