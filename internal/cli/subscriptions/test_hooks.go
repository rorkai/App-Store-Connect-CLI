package subscriptions

import (
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SetSetupClientFactory replaces the setup ASC client factory for tests.
// It returns a restore function to reset the previous factory.
func SetSetupClientFactory(fn func() (*asc.Client, error)) func() {
	previous := subscriptionsSetupClientFactory
	if fn == nil {
		subscriptionsSetupClientFactory = shared.GetASCClient
	} else {
		subscriptionsSetupClientFactory = fn
	}
	return func() {
		subscriptionsSetupClientFactory = previous
	}
}
