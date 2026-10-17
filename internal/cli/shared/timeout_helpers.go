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
