package builds

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveOneDSYMUsesDownloadBudget(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "40ms")
	t.Setenv("ASC_UPLOAD_TIMEOUT", "2s")
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "download", true: "verify existing"}[existing], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "ds")
				w.(http.Flusher).Flush()
				select {
				case <-time.After(150 * time.Millisecond):
					_, _ = io.WriteString(w, "ym")
				case <-r.Context().Done():
				}
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "test.dSYM.zip")
			if existing {
				if err := os.WriteFile(path, []byte("dsym"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			size, _, skipped, err := saveOneDSYM(t.Context(), server.URL, path)
			if err != nil || size != 4 || skipped != existing {
				t.Fatalf("saveOneDSYM() = %d, skipped=%v, %v", size, skipped, err)
			}
		})
	}
}

func TestSaveOneDSYMStalledBodyHonorsParentCancellation(t *testing.T) {
	t.Setenv("ASC_UPLOAD_TIMEOUT", "2s")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "partial")
		w.(http.Flusher).Flush()
		cancel()
		<-r.Context().Done()
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "test.dSYM.zip")
	_, _, _, err := saveOneDSYM(ctx, server.URL, path)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("saveOneDSYM() error = %v, want cancellation", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial artifact remains: %v", err)
	}
}

func TestSaveOneDSYMStalledBodyHonorsDownloadDeadline(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "2s")
	t.Setenv("ASC_UPLOAD_TIMEOUT", "40ms")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "partial")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "test.dSYM.zip")
	_, _, _, err := saveOneDSYM(t.Context(), server.URL, path)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("saveOneDSYM() error = %v, want deadline exceeded", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial artifact remains: %v", err)
	}
}
