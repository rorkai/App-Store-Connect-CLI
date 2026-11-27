package asc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// RetryDownload retries a read-only download, including consumption of its body.
// Each attempt must discard incomplete output and reset any accumulated state.
func RetryDownload[T any](ctx context.Context, rawURL string, attempt func() (T, error), opts RetryOptions) (T, error) {
	return withRetry(ctx, func() (T, error) {
		if err := ctx.Err(); err != nil {
			var zero T
			return zero, err
		}
		result, err := attempt()
		var networkError net.Error
		if errors.As(err, &networkError) {
			err = newSanitizedNoAuthStreamError("download", rawURL, err)
		}
		return result, err
	}, opts, func(err error) bool {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || IsRetryBudgetExhausted(err) || IsRetryDelayExceeded(err) {
			return false
		}
		return IsRetryable(err) || isTransientTransportError(err)
	})
}

// DownloadResponseError retains the HTTP status and server delay without
// exposing a signed download URL or an untrusted response body.
func DownloadResponseError(status int, retryAfter string) error {
	err := &APIError{Code: apiErrorCodeFromStatus(status), Title: fmt.Sprintf("download request failed with status %d", status), StatusCode: status}
	if isRetryableHTTPStatus(status) {
		return &RetryableError{Err: err, RetryAfter: parseRetryAfterHeader(retryAfter)}
	}
	return err
}

// ParseRetryAfter returns the server delay expressed as seconds or an HTTP date.
func ParseRetryAfter(value string) time.Duration {
	return parseRetryAfterHeader(value)
}
