package app_events

import (
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SetClientFactory replaces the ASC client factory for tests.
// It returns a restore function to reset the previous handler.
func SetClientFactory(fn func() (*asc.Client, error)) func() {
	previous := appEventsClientFactory
	if fn == nil {
		appEventsClientFactory = shared.GetASCClient
	} else {
		appEventsClientFactory = fn
	}
	return func() {
		appEventsClientFactory = previous
	}
}
