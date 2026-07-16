package subscriptions

import (
	"context"
	"os"
	"sync"

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

// SetPricePointsClientFactory replaces the price-points ASC client factory for tests.
// It returns a restore function to reset the previous factory.
func SetPricePointsClientFactory(fn func() (*asc.Client, error)) func() {
	previous := subscriptionPricePointsClientFactory
	if fn == nil {
		subscriptionPricePointsClientFactory = shared.GetASCClient
	} else {
		subscriptionPricePointsClientFactory = fn
	}
	return func() {
		subscriptionPricePointsClientFactory = previous
	}
}

var (
	subscriptionVersionImageUploaderMu sync.RWMutex
	subscriptionVersionImageUploader   = asc.UploadAssetFromFile
)

func uploadSubscriptionVersionImage(ctx context.Context, file *os.File, fileSize int64, operations []asc.UploadOperation) error {
	subscriptionVersionImageUploaderMu.RLock()
	uploader := subscriptionVersionImageUploader
	subscriptionVersionImageUploaderMu.RUnlock()
	return uploader(ctx, file, fileSize, operations)
}

// SetSubscriptionVersionImageUploaderForTesting replaces the version-image uploader.
// It returns a restore function for test isolation.
func SetSubscriptionVersionImageUploaderForTesting(fn func(context.Context, *os.File, int64, []asc.UploadOperation) error) func() {
	subscriptionVersionImageUploaderMu.Lock()
	previous := subscriptionVersionImageUploader
	if fn == nil {
		subscriptionVersionImageUploader = asc.UploadAssetFromFile
	} else {
		subscriptionVersionImageUploader = fn
	}
	subscriptionVersionImageUploaderMu.Unlock()
	return func() {
		subscriptionVersionImageUploaderMu.Lock()
		subscriptionVersionImageUploader = previous
		subscriptionVersionImageUploaderMu.Unlock()
	}
}
