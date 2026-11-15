package subscriptions

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/winbackoffers"
)

// newSubscriptionsWinBackOffersCommand returns the canonical nested win-back offers tree.
func newSubscriptionsWinBackOffersCommand(d *shared.Deps) *ffcli.Command {
	return shared.RewriteCommandTreePath(
		winbackoffers.NewWinBackOffersCommand(d),
		"asc win-back-offers",
		"asc subscriptions win-back-offers",
	)
}
