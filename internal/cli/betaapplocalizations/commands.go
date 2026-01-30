package betaapplocalizations

import "github.com/peterbourgon/ff/v3/ffcli"

// Command returns the beta app localizations command group.
func Command() *ffcli.Command {
	return BetaAppLocalizationsCommand()
}
