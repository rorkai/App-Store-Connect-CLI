package localizations

import (
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// LocalizationsPlacementsCommand exposes localized header and search placements.
func newLocalizationsPlacementsCommand(d *shared.Deps) *ffcli.Command {
	return d.CreativePlacementsCommand("appStoreVersionLocalizations", "localizations", "an App Store version localization")
}
