package iap

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"

// SetVersionClientFactory replaces the IAP version ASC client factory for tests.
func SetVersionClientFactory(fn func() (*asc.Client, error)) func() {
	previousVersion := iapVersionClientFactory
	if fn != nil {
		iapVersionClientFactory = fn
	}
	return func() {
		iapVersionClientFactory = previousVersion
	}
}
