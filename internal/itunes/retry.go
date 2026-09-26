package itunes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// do executes a public App Store request and hands a successful response to
// handle.
//
// Apple's public endpoints rate limit aggressively, so rate-limited and
// server-side failures are retried with the shared exponential backoff engine.
// Only idempotent reads are replayed. Terminal failures keep the original
// *httpStatusError so callers, output, and telemetry continue to observe the
// public storefront status.
func (c *Client) do(ctx context.Context, operation string, req *http.Request, handle func(*http.Response) error) error {
	retrySafe := req.Method == http.MethodGet || req.Method == http.MethodHead

	// A zero value disables retries, which keeps non-idempotent requests on the
	// single-attempt path.
	var options asc.RetryOptions
	if retrySafe {
		options = asc.ResolveRetryOptions()
	}

	_, err := asc.WithRetry(ctx, func() (struct{}, error) {
		return struct{}{}, c.doOnce(ctx, operation, req, handle, retrySafe, options.MaxDelay)
	}, options)
	return unwrapRetryableError(err)
}

func (c *Client) doOnce(
	ctx context.Context,
	operation string,
	req *http.Request,
	handle func(*http.Response) error,
	retrySafe bool,
	maxDelay time.Duration,
) error {
	resp, err := c.httpClient().Do(req.Clone(ctx))
	if err != nil {
		return fmt.Errorf("%s request failed: %w", operation, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		statusErr := &httpStatusError{operation: operation, statusCode: resp.StatusCode}
		if retrySafe && isRetryablePublicStatus(resp.StatusCode) {
			drainPublicRetryResponse(resp.Body)
			retryAfter := publicRetryDelay(resp.Header, time.Now(), maxDelay)
			return &asc.RetryableError{
				Err:                     statusErr,
				RetryAfter:              retryAfter,
				PreserveErrorOnDeadline: true,
			}
		}
		return statusErr
	}

	return handle(resp)
}

const maxPublicRetryResponseDrain = 64 << 10

// drainPublicRetryResponse lets the transport reuse connections after the
// small error bodies returned by public endpoints without trusting an
// unbounded response body.
func drainPublicRetryResponse(body io.Reader) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, maxPublicRetryResponseDrain))
}

// unwrapRetryableError keeps retry bookkeeping internal so an exhausted retry
// budget surfaces the same error a single attempt would have returned.
func unwrapRetryableError(err error) error {
	if err == nil {
		return nil
	}
	// A retry delay that cannot fit the request context is already a terminal
	// diagnosis. Preserve its terminal marker and retryable provider status for
	// callers that use the error to cancel queued storefront work, while keeping
	// the public error text compatible with a direct storefront status failure.
	if asc.IsRetryDelayExceeded(err) {
		var statusErr *httpStatusError
		if errors.As(err, &statusErr) {
			return &publicRetryDelayExceededError{status: statusErr, cause: err}
		}
		return err
	}
	// An explicit cancellation is operator intent, not an exhausted storefront
	// retry. Keep the shared cancellation wrapper so errors.Is still observes
	// context.Canceled instead of reducing it to Apple's last HTTP status.
	if errors.Is(err, context.Canceled) {
		return err
	}
	if retryErr, ok := errors.AsType[*asc.RetryableError](err); ok {
		return retryErr.Err
	}
	return err
}

// publicRetryDelayExceededError exposes both the provider status and the
// terminal marker of a retry diagnosis. Its Error method
// intentionally stays the same as the historical storefront status error so
// direct public reads do not gain internal retry wording.
type publicRetryDelayExceededError struct {
	status *httpStatusError
	cause  error
}

func (e *publicRetryDelayExceededError) Error() string {
	return e.status.Error()
}

func (e *publicRetryDelayExceededError) Unwrap() []error {
	return []error{e.status, e.cause}
}

// isRetryablePublicStatus reports whether replaying the request could succeed:
// 429 for storefront rate limiting and any 5xx for storefront-side failures.
func isRetryablePublicStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || (statusCode >= 500 && statusCode <= 599)
}

// publicRetryDelay reads the Retry-After hint, which Apple sends either as a
// delay in seconds or as an HTTP date, and caps it at maxDelay.
func publicRetryDelay(headers http.Header, now time.Time, maxDelay time.Duration) time.Duration {
	value := strings.TrimSpace(headers.Get("Retry-After"))
	if value == "" {
		return 0
	}

	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return publicRetryDelayFromSeconds(seconds, maxDelay)
	}
	// ParseInt rejects positive values above MaxInt64, but Retry-After's
	// delay-seconds grammar has no such bound. Treat an all-digit positive
	// overflow as an arbitrarily long delay and apply the configured cap.
	if isPositiveDecimal(value) {
		const maxDuration = time.Duration(1<<63 - 1)
		return capPublicRetryDelay(maxDuration, maxDelay)
	}
	if deadline, err := http.ParseTime(value); err == nil {
		if delay := deadline.Sub(now); delay > 0 {
			return capPublicRetryDelay(delay, maxDelay)
		}
	}
	return 0
}

func isPositiveDecimal(value string) bool {
	value = strings.TrimPrefix(value, "+")
	if value == "" {
		return false
	}
	positive := false
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
		positive = positive || value[i] != '0'
	}
	return positive
}

func publicRetryDelayFromSeconds(seconds int64, maxDelay time.Duration) time.Duration {
	if seconds <= 0 {
		return 0
	}

	const maxDuration = time.Duration(1<<63 - 1)
	if seconds > int64(maxDuration/time.Second) {
		return capPublicRetryDelay(maxDuration, maxDelay)
	}
	return capPublicRetryDelay(time.Duration(seconds)*time.Second, maxDelay)
}

func capPublicRetryDelay(delay, maxDelay time.Duration) time.Duration {
	if delay > 0 && maxDelay > 0 && delay > maxDelay {
		return maxDelay
	}
	return delay
}
