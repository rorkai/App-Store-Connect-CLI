package xcodecloud

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestArtifactsDownloadRetries(t *testing.T) {
	for _, tc := range []struct {
		name         string
		status       int
		partial      bool
		cancel       bool
		permanent    bool
		wantAttempts int
	}{
		{name: "server delay", status: 503, wantAttempts: 2},
		{name: "truncated body", partial: true, wantAttempts: 2},
		{name: "cancelled body", partial: true, cancel: true, wantAttempts: 1},
		{name: "retry budget", status: 500, permanent: true, wantAttempts: 3},
		{name: "forbidden", status: 403, wantAttempts: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ASC_MAX_RETRIES", "2")
			t.Setenv("ASC_BASE_DELAY", "1ms")
			t.Setenv("ASC_MAX_DELAY", "2s")
			t.Setenv("ASC_RETRY_LOG", "")
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				target := filepath.Join(t.TempDir(), "artifact.zip")
				if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
					t.Fatal(err)
				}
				calls := 0
				start := time.Now()
				client := newDoctorLogTestClient(t, doctorLogRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet {
						t.Fatalf("unexpected method %s", req.Method)
					}
					resp := &http.Response{StatusCode: 200, Header: make(http.Header)}
					if req.URL.Path == "/v1/ciArtifacts/artifact-1" {
						resp.Body = io.NopCloser(strings.NewReader(`{"data":{"id":"artifact-1","type":"ciArtifacts","attributes":{"downloadUrl":"https://mzstatic.com/artifact.zip?token=private-value","fileName":"artifact.zip"}}}`))
						return resp, nil
					}
					if req.URL.Host != "mzstatic.com" || req.Header.Get("Authorization") != "" {
						t.Fatalf("unsafe download request: host=%s auth=%t", req.URL.Host, req.Header.Get("Authorization") != "")
					}
					calls++
					if got, err := os.ReadFile(target); err != nil || string(got) != "original" {
						t.Fatalf("destination changed before completion: %q, %v", got, err)
					}
					resp.Body = io.NopCloser(strings.NewReader("complete"))
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
							resp.Header.Set("Retry-After", "1")
							resp.Body = io.NopCloser(strings.NewReader("private-value"))
						}
					}
					return resp, nil
				}))
				d := shared.DefaultDeps()
				d.Client = func() (*asc.Client, error) { return client, nil }
				var out bytes.Buffer
				d.IO = &shared.IO{Out: &out, ErrOut: io.Discard}
				err := newXcodeCloudArtifactsDownloadCommand(d).ParseAndRun(ctx, []string{"--id", "artifact-1", "--path", target, "--overwrite", "--output", "json"})
				success := tc.wantAttempts == 2
				if success && err != nil {
					t.Fatal(err)
				}
				if !success && err == nil {
					t.Fatal("expected failure")
				}
				if tc.cancel && !errors.Is(err, context.Canceled) {
					t.Fatalf("lost cancellation: %v", err)
				}
				if err != nil && strings.Contains(err.Error(), "private-value") {
					t.Fatal("download URL or response body leaked")
				}
				if calls != tc.wantAttempts {
					t.Fatalf("GET count = %d, want %d", calls, tc.wantAttempts)
				}
				if tc.status == 503 && time.Since(start) != time.Second {
					t.Fatalf("retry wait = %s", time.Since(start))
				}
				want := "original"
				if success {
					want = "complete"
					var receipt asc.CiArtifactDownloadResult
					if err := json.Unmarshal(out.Bytes(), &receipt); err != nil || receipt.BytesWritten != 8 || receipt.OutputPath != target {
						t.Fatalf("receipt = %+v, %v", receipt, err)
					}
				} else if out.Len() != 0 {
					t.Fatalf("failure printed receipt: %s", out.String())
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
