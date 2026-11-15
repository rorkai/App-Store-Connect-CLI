package gamecenter

import (
	"flag"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// validateGameCenterReplacementConfirm enforces --confirm on relationship
// replacements, which can drop existing relationships.
func validateGameCenterReplacementConfirm(d *shared.Deps, fs *flag.FlagSet, confirmed bool) error {
	if !confirmed && flagPassed(fs, "confirm") {
		return d.UsageError("--confirm must be true when specified")
	}
	return d.RequireConfirm(confirmed)
}
