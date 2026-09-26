package asc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// doStreamingRequest sends a request whose response body is consumed by the
// caller. Client.Timeout covers the whole exchange, including that body copy.
// The derived context preserves the caller's earlier cancellation or deadline
// while ensuring a request with no earlier deadline cannot remain open
// indefinitely after the client timeout is exceeded.
func doStreamingRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	if client == nil {
		client = newDefaultHTTPClient(ResolveTimeout())
	}
	timeout := client.Timeout
	streaming := *client
	streaming.Timeout = 0
	if timeout <= 0 {
		return streaming.Do(req)
	}

	if deadline, ok := req.Context().Deadline(); !ok || time.Until(deadline) <= timeout {
		return doStreamingWithExchangeTimeout(&streaming, req, timeout)
	}

	return doStreamingWithHeaderTimeout(&streaming, req, timeout)
}

func doStreamingWithExchangeTimeout(client *http.Client, req *http.Request, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), timeout)
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		callerErr := req.Context().Err()
		cancel()
		if callerErr != nil {
			return nil, callerErr
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("timed out after %s awaiting response headers: %w", timeout, context.DeadlineExceeded)
		}
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		_ = resp.Body.Close()
		cancel()
		if callerErr := req.Context().Err(); callerErr != nil {
			return nil, callerErr
		}
		return nil, fmt.Errorf("timed out after %s awaiting response headers: %w", timeout, err)
	}

	resp.Body = &streamingResponseBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func doStreamingWithHeaderTimeout(client *http.Client, req *http.Request, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	watchdog := &headerTimeout{cancel: cancel}
	watchdog.timer = time.AfterFunc(timeout, watchdog.fire)

	resp, err := client.Do(req.WithContext(ctx))
	expired := watchdog.settle()
	if err != nil {
		callerErr := req.Context().Err()
		cancel()
		if callerErr != nil {
			return nil, callerErr
		}
		if expired {
			return nil, fmt.Errorf("timed out after %s awaiting response headers: %w", timeout, context.DeadlineExceeded)
		}
		return nil, err
	}
	if expired {
		// The watchdog cancelled the request as the headers arrived, so this
		// body is no longer readable.
		_ = resp.Body.Close()
		cancel()
		if callerErr := req.Context().Err(); callerErr != nil {
			return nil, callerErr
		}
		return nil, fmt.Errorf("timed out after %s awaiting response headers: %w", timeout, context.DeadlineExceeded)
	}

	// The derived context has to outlive this call so the body stays readable;
	// closing the body releases it.
	resp.Body = &streamingResponseBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// headerTimeout cancels a streaming request that has not produced response
// headers in time. settle reports whether that already happened and keeps the
// timer from cancelling a body copy that has since started.
type headerTimeout struct {
	cancel context.CancelFunc
	timer  *time.Timer

	mu      sync.Mutex
	settled bool
	expired bool
}

func (h *headerTimeout) fire() {
	h.mu.Lock()
	if h.settled {
		h.mu.Unlock()
		return
	}
	h.expired = true
	h.mu.Unlock()
	h.cancel()
}

func (h *headerTimeout) settle() bool {
	h.mu.Lock()
	h.settled = true
	expired := h.expired
	h.mu.Unlock()
	h.timer.Stop()
	return expired
}

type streamingResponseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *streamingResponseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}
