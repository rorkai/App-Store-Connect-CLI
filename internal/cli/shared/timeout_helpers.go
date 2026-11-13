package shared

import (
	"context"
	"errors"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// ErrUploadDeadline marks an error caused by the deadline that
// ASC_UPLOAD_TIMEOUT controls, so the error hint names that setting even when
// the message itself does not mention an upload.
var ErrUploadDeadline = errors.New("upload deadline reached")

type uploadDeadlineError struct{ error }

func (e uploadDeadlineError) Unwrap() []error { return []error{e.error, ErrUploadDeadline} }

// MarkUploadDeadline marks a deadline error as an upload deadline. Callers use
// it only after checking that their upload context, not a request context,
// expired. Other errors are returned unchanged.
func MarkUploadDeadline(err error) error {
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return uploadDeadlineError{err}
}

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
