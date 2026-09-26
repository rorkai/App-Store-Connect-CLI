package asc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDoStreamNoAuthUsesRequestContextInsteadOfClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("arti"))
		w.(http.Flusher).Flush()
		// A large download keeps the body open well past the client timeout.
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte("fact"))
	}))
	t.Cleanup(server.Close)

	client := &Client{httpClient: &http.Client{Timeout: 40 * time.Millisecond}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	resp, err := client.doStreamNoAuth(ctx, server.URL, "")
	if err != nil {
		t.Fatalf("doStreamNoAuth() error = %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "artifact" {
		t.Fatalf("body = %q", body)
	}
}

func TestDoStreamNoAuthStillHonorsCallerContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := &Client{httpClient: &http.Client{Timeout: time.Second}}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	t.Cleanup(cancel)

	_, err := client.doStreamNoAuth(ctx, server.URL, "")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("doStreamNoAuth() error = %v, want context deadline exceeded", err)
	}
}

func TestDoStreamNoAuthBoundsResponseHeadersByClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := &Client{httpClient: &http.Client{Timeout: 40 * time.Millisecond}}

	_, err := client.doStreamNoAuth(context.Background(), server.URL, "")
	if err == nil {
		t.Fatal("expected the client timeout to bound response headers without a caller deadline")
	}
}

func TestDoStreamingRequestClientTimeoutBoundsBodyWithoutEarlierCallerDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("arti"))
		w.(http.Flusher).Flush()
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte("fact"))
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Timeout: 40 * time.Millisecond}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := doStreamingRequest(client, req)
	if err != nil {
		t.Fatalf("doStreamingRequest() error = %v", err)
	}
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read body error = %v, want context deadline exceeded", err)
	}
}

func TestDoStreamingRequestBoundsCustomTransportBeforeHeaders(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		select {
		case <-time.After(300 * time.Millisecond):
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}, nil
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	})
	client := &http.Client{Timeout: 40 * time.Millisecond, Transport: transport}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/artifact", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	start := time.Now()
	if _, err := doStreamingRequest(client, req); err == nil {
		t.Fatal("expected the client timeout to bound a custom transport")
	}
	if elapsed := time.Since(start); elapsed >= 300*time.Millisecond {
		t.Fatalf("custom transport was not bounded before headers: elapsed = %s", elapsed)
	}
}

func TestDoStreamingRequestHeaderTimeoutWrapsDeadlineExceeded(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	client := &http.Client{Timeout: 40 * time.Millisecond, Transport: transport}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/artifact", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	_, err = doStreamingRequest(client, req)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("doStreamingRequest() error = %v, want context deadline exceeded", err)
	}
}

func TestDoStreamingRequestCallerCancellationWins(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	client := &http.Client{Timeout: time.Second, Transport: transport}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/artifact", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	_, err = doStreamingRequest(client, req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("doStreamingRequest() error = %v, want context canceled", err)
	}
}

func TestHeaderTimeoutSettledBeforeExpiryLeavesBodyReadable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	watchdog := &headerTimeout{cancel: cancel, timer: time.NewTimer(time.Hour)}

	if expired := watchdog.settle(); expired {
		t.Fatal("settle() reported an expiry that never happened")
	}
	watchdog.fire()

	if err := ctx.Err(); err != nil {
		t.Fatalf("a settled watchdog must not cancel the body copy: %v", err)
	}
}

func TestHeaderTimeoutExpiryAtSettleBoundaryIsReported(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	watchdog := &headerTimeout{cancel: cancel, timer: time.NewTimer(time.Hour)}

	watchdog.fire()

	if expired := watchdog.settle(); !expired {
		t.Fatal("settle() must report an expiry that already cancelled the request")
	}
	if ctx.Err() == nil {
		t.Fatal("expected the watchdog to cancel the request")
	}
}

func TestDoStreamingRequestClosingBodyReleasesContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("artifact"))
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Timeout: time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := doStreamingRequest(client, req)
	if err != nil {
		t.Fatalf("doStreamingRequest() error = %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "artifact" {
		t.Fatalf("body = %q", body)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close body: %v", err)
	}
	if err := resp.Request.Context().Err(); err == nil {
		t.Fatal("expected closing the body to cancel the derived request context")
	}
}
