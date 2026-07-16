package iap

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"

// SetVersionClientFactory replaces the IAP version ASC client factory for tests.
func SetVersionClientFactory(fn func() (*asc.Client, error)) func() {
	previousVersion := iapVersionClientFactory
	previousQuery := iapQueryClientFactory
	if fn == nil {
		iapVersionClientFactory = previousVersion
		iapQueryClientFactory = previousQuery
	} else {
		iapVersionClientFactory = fn
		iapQueryClientFactory = fn
	}
	return func() {
		iapVersionClientFactory = previousVersion
		iapQueryClientFactory = previousQuery
	}
}
