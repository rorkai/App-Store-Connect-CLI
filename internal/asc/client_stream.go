package asc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// doStreamingRequest sends a request whose response body is consumed by the
// caller. Client.Timeout also covers that body copy, which aborts a large
// download mid-transfer, so the configured timeout is applied to every phase
// before the response headers (dial, TLS handshake, request write and server
// think time) while the body copy is bounded by the request context.
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

	ctx, cancel := context.WithCancel(req.Context())
	watchdog := &headerTimeout{cancel: cancel}
	watchdog.timer = time.AfterFunc(timeout, watchdog.fire)

	resp, err := streaming.Do(req.WithContext(ctx))
	expired := watchdog.settle()
	switch {
	case err != nil:
		cancel()
		if expired && req.Context().Err() == nil {
			return nil, fmt.Errorf("timed out after %s awaiting response headers: %w", timeout, err)
		}
		return nil, err
	case expired:
		// The watchdog cancelled the request as the headers arrived, so this
		// body is no longer readable.
		_ = resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("timed out after %s awaiting response headers", timeout)
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
	defer h.mu.Unlock()
	if h.settled {
		return
	}
	h.expired = true
	h.cancel()
}

func (h *headerTimeout) settle() bool {
	h.timer.Stop()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.settled = true
	return h.expired
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
