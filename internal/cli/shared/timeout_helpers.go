package shared

import (
	"context"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// ContextWithResolvedTimeout returns a context with ASC timeout resolution and
// a package-provided default fallback duration.
func ContextWithResolvedTimeout(ctx context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	return withTimeoutContext(ctx, asc.ResolveTimeoutWithDefault(defaultTimeout))
}

// ContextWithResolvedUploadTimeout returns a context bounded by
// ASC_UPLOAD_TIMEOUT when configured, or by defaultTimeout otherwise.
func ContextWithResolvedUploadTimeout(ctx context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	return withTimeoutContext(ctx, asc.ResolveUploadTimeoutWithDefault(defaultTimeout))
}

// The default media upload deadline grows with the files it covers, from the
// base for no recorded work up to the maximum. ASC_UPLOAD_TIMEOUT replaces it.
const (
	mediaUploadBaseTimeout    = 10 * time.Minute
	mediaUploadTimeoutPerFile = time.Minute
	mediaUploadTimeoutPerMiB  = 30 * time.Second
	mediaUploadMaxTimeout     = time.Hour
)

// MediaUploadTimeoutForWork returns the default deadline for uploading files
// totaling bytes and waiting for App Store Connect to process them.
func MediaUploadTimeoutForWork(files int, bytes int64) time.Duration {
	mib := (bytes + 1<<20 - 1) >> 20
	timeout := mediaUploadBaseTimeout +
		time.Duration(files)*mediaUploadTimeoutPerFile +
		time.Duration(mib)*mediaUploadTimeoutPerMiB
	return min(timeout, mediaUploadMaxTimeout)
}

// ContextWithMediaUploadTimeout bounds one media file's upload and processing
// wait by ASC_UPLOAD_TIMEOUT, or by MediaUploadTimeoutForWork when it is unset.
func ContextWithMediaUploadTimeout(ctx context.Context, bytes int64) (context.Context, context.CancelFunc) {
	return ContextWithResolvedUploadTimeout(ctx, MediaUploadTimeoutForWork(1, bytes))
}
