package reviews

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"

// SetReviewItemsClientFactory replaces the review-items client factory for tests.
func SetReviewItemsClientFactory(fn func() (*asc.Client, error)) func() {
	previous := reviewItemsClientFactory
	if fn != nil {
		reviewItemsClientFactory = fn
	}
	return func() {
		reviewItemsClientFactory = previous
	}
}
