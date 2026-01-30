package betabuildlocalizations

import "github.com/peterbourgon/ff/v3/ffcli"

// Command returns the beta build localizations command group.
func Command() *ffcli.Command {
	return BetaBuildLocalizationsCommand()
}
