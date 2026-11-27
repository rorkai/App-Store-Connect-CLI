package assets

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"testing/synctest"
	"time"
)

func TestDownloadURLToFileRetries(t *testing.T) {
	for _, tc := range []struct {
		name         string
		status       int
		retryAfter   string
		partial      bool
		cancel       bool
		permanent    bool
		wantAttempts int
		wantWait     time.Duration
		transportErr error
		maxDelay     string
		noRetries    bool
		noOverwrite  bool
	}{
		{name: "server error", status: 500, wantAttempts: 2},
		{name: "delivery forbidden", status: 403, retryAfter: "1", wantAttempts: 2, wantWait: time.Second},
		{name: "transient transport", transportErr: &net.DNSError{IsTimeout: true}, wantAttempts: 2},
		{name: "transport deadline", transportErr: context.DeadlineExceeded, wantAttempts: 1},
		{name: "permanent TLS failure", transportErr: &tls.CertificateVerificationError{Err: errors.New("private-value")}, wantAttempts: 1},
		{name: "local destination exists", status: 200, noOverwrite: true, wantAttempts: 1},
		{name: "configured cap", status: 503, retryAfter: "3", maxDelay: "5s", wantAttempts: 2, wantWait: 3 * time.Second},
		{name: "retries disabled", status: 500, noRetries: true, wantAttempts: 1},
		{name: "retry after seconds", status: 429, retryAfter: "1", wantAttempts: 2, wantWait: time.Second},
		{name: "retry after date", status: 503, retryAfter: "date", wantAttempts: 2, wantWait: time.Second},
		{name: "retry after exceeds cap", status: 503, retryAfter: "60", wantAttempts: 1},
		{name: "truncated body", partial: true, wantAttempts: 2},
		{name: "cancelled transfer", partial: true, cancel: true, wantAttempts: 1},
		{name: "retry budget", partial: true, permanent: true, wantAttempts: 4},
		{name: "not found", status: 404, wantAttempts: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ASC_MAX_RETRIES", "3")
			t.Setenv("ASC_BASE_DELAY", "200ms")
			t.Setenv("ASC_MAX_DELAY", "2s")
			if tc.maxDelay != "" {
				t.Setenv("ASC_MAX_DELAY", tc.maxDelay)
			}
			if tc.noRetries {
				t.Setenv("ASC_MAX_RETRIES", "0")
			}
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				target := filepath.Join(t.TempDir(), "image.png")
				if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				calls := 0
				d := newAssetsUploadTestDeps()
				d.HTTPTransport = func() http.RoundTripper {
					return assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method != http.MethodGet {
							t.Fatalf("unexpected method %s", req.Method)
						}
						calls++
						if calls == 1 && tc.transportErr != nil {
							return nil, tc.transportErr
						}
						if calls > 1 {
							if got, err := os.ReadFile(target); err != nil || string(got) != "original" {
								t.Fatalf("incomplete transfer replaced destination: %q, %v", got, err)
							}
						}
						resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(strings.NewReader("complete"))}
						if calls == 1 || tc.permanent {
							if tc.partial {
								readErr := error(io.ErrUnexpectedEOF)
								if tc.cancel {
									cancel()
									readErr = context.Canceled
								}
								resp.Body = io.NopCloser(io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(readErr)))
							} else {
								resp.StatusCode = tc.status
								hint := tc.retryAfter
								if hint == "date" {
									hint = start.Add(time.Second).UTC().Format(http.TimeFormat)
								}
								resp.Header.Set("Retry-After", hint)
								resp.Body = io.NopCloser(strings.NewReader("private-value"))
							}
						}
						return resp, nil
					})
				}
				n, _, err := downloadURLToFile(ctx, d, "https://mzstatic.com/image.png?token=private-value", target, !tc.noOverwrite)
				success := tc.wantAttempts == 2
				if success && (err != nil || n != 8) {
					t.Fatalf("download = %d, %v, want complete file", n, err)
				}
				if err != nil && strings.Contains(err.Error(), "private-value") {
					t.Fatal("download URL or response body leaked")
				}
				if !success && err == nil {
					t.Fatal("expected terminal failure")
				}
				if tc.transportErr != nil && !success && !errors.Is(err, tc.transportErr) {
					t.Fatalf("lost transport cause: %v", err)
				}
				if tc.noOverwrite && !errors.Is(err, os.ErrExist) {
					t.Fatalf("lost destination error: %v", err)
				}
				if err != nil && tc.status >= 400 {
					var status interface{ HTTPStatusCode() int }
					if !errors.As(err, &status) || status.HTTPStatusCode() != tc.status {
						t.Fatalf("lost HTTP status: %v", err)
					}
				}
				if tc.cancel && !errors.Is(err, context.Canceled) {
					t.Fatalf("lost cancellation: %v", err)
				}
				if calls != tc.wantAttempts {
					t.Fatalf("GET count = %d, want %d", calls, tc.wantAttempts)
				}
				if tc.wantWait > 0 && time.Since(start) != tc.wantWait {
					t.Fatalf("wait = %s, want %s", time.Since(start), tc.wantWait)
				}
				want := "original"
				if success {
					want = "complete"
				}
				got, readErr := os.ReadFile(target)
				if readErr != nil || string(got) != want {
					t.Fatalf("saved bytes = %q, %v; want %q", got, readErr, want)
				}
				entries, readErr := os.ReadDir(filepath.Dir(target))
				if readErr != nil || len(entries) != 1 {
					t.Fatalf("staged files leaked: %v, %v", entries, readErr)
				}
			})
		})
	}
}
