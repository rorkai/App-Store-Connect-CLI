package analytics

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAnalyticsDownloadRetries(t *testing.T) {
	for _, allSegments := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			status  bool
			cancel  bool
			corrupt bool
		}{
			{name: "server delay", status: true}, {name: "truncated body"}, {name: "cancelled body", cancel: true}, {name: "checksum mismatch", corrupt: true},
		} {
			if !allSegments && tc.corrupt {
				continue
			}
			t.Run(tc.name+map[bool]string{false: " single", true: " staged"}[allSegments], func(t *testing.T) {
				t.Setenv("ASC_MAX_RETRIES", "2")
				t.Setenv("ASC_BASE_DELAY", "1ms")
				t.Setenv("ASC_MAX_DELAY", "2s")
				t.Setenv("ASC_RETRY_LOG", "")
				synctest.Test(t, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					payload := compareGzipFixture(t, "Date\tCount\n2026-11-23\t12\n")
					digest := md5.Sum(payload)
					checksum := hex.EncodeToString(digest[:])
					if tc.corrupt {
						checksum = strings.Repeat("0", 32)
					}
					calls := 0
					start := time.Now()
					client := newCompareTransportClient(t, compareRoundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method != http.MethodGet || req.Header.Get("Authorization") != "" || req.URL.Host != "mzstatic.com" {
							t.Fatalf("unsafe download request: %s %s auth=%t", req.Method, req.URL.Host, req.Header.Get("Authorization") != "")
						}
						calls++
						resp := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(payload))}
						if calls == 1 && !tc.corrupt {
							if tc.status {
								resp.StatusCode = 503
								resp.Header.Set("Retry-After", "1")
								resp.Body = io.NopCloser(strings.NewReader("private-value"))
							} else {
								readErr := error(io.ErrUnexpectedEOF)
								if tc.cancel {
									cancel()
									readErr = context.Canceled
								}
								resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(payload[:10]), iotest.ErrReader(readErr)))
							}
						}
						return resp, nil
					}))
					dir := t.TempDir()
					target := filepath.Join(dir, "report.gz")
					d := shared.DefaultDeps()
					var err error
					var size int64
					if allSegments {
						staging, openErr := os.OpenRoot(dir)
						if openErr != nil {
							t.Fatal(openErr)
						}
						defer staging.Close()
						segment := asc.Resource[asc.AnalyticsReportSegmentAttributes]{ID: "segment-1", Attributes: asc.AnalyticsReportSegmentAttributes{URL: "https://mzstatic.com/report.gz?token=private-value", Checksum: checksum, SizeInBytes: int64(len(payload))}}
						result, stageErr := stageAnalyticsSegment(ctx, d, client, staging, dir, "report", segment, true)
						err = stageErr
						size = result.FileSize
					} else {
						size, err = downloadAnalyticsReportToFile(ctx, d, client, "https://mzstatic.com/report.gz?token=private-value", target)
					}
					if tc.cancel {
						if !errors.Is(err, context.Canceled) || calls != 1 {
							t.Fatalf("cancellation = %v after %d requests", err, calls)
						}
						if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
							t.Fatalf("partial output left: %v, %v", entries, err)
						}
						return
					}
					if tc.corrupt {
						if err == nil || !strings.Contains(err.Error(), "MD5 checksum mismatch") || calls != 1 {
							t.Fatalf("checksum failure = %v after %d requests", err, calls)
						}
						return
					}
					if err != nil || calls != 2 || size != int64(len(payload)) {
						t.Fatalf("download = %d bytes, %d requests, %v", size, calls, err)
					}
					if tc.status && time.Since(start) != time.Second {
						t.Fatalf("retry wait = %s", time.Since(start))
					}
					got, readErr := os.ReadFile(target)
					if readErr != nil || !bytes.Equal(got, payload) {
						t.Fatalf("downloaded bytes differ: %v", readErr)
					}
					if allSegments {
						got, readErr := os.ReadFile(filepath.Join(dir, "report"))
						if readErr != nil || string(got) != "Date\tCount\n2026-11-23\t12\n" {
							t.Fatalf("decompressed report = %q, %v", got, readErr)
						}
					}
				})
			})
		}
	}
}
