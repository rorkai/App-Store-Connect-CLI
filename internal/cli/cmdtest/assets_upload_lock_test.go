package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/uploadlock"
)

func TestAssetUploadsWaitForAnotherAscUploadBeforeSendingRequests(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv(uploadlock.EnvVar, "")

	screenshotDir := t.TempDir()
	writeCmdtestScreenshotPNG(t, screenshotDir, "01-home.png")
	previewDir := t.TempDir()
	writePreviewFile(t, filepath.Join(previewDir, "01-first.mov"))

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "screenshots upload",
			args: []string{"screenshots", "upload", "--version-localization", "LOC_123", "--path", screenshotDir, "--device-type", "IPHONE_65"},
		},
		{
			name: "video-previews upload",
			args: []string{"video-previews", "upload", "--version-localization", "LOC_123", "--path", previewDir, "--device-type", "IPHONE_65"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// This test holds the lock as the other asc process would.
			release, err := uploadlock.Acquire(context.Background(), io.Discard)
			if err != nil {
				t.Fatalf("hold upload lock: %v", err)
			}
			defer release()
			t.Cleanup(uploadlock.SetWaitLimitForTest(0))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			requests := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				return statusJSONResponse(`{"data":[],"links":{}}`), nil
			})

			stdout, stderr, runErr := runRootCommand(t, test.args)
			if runErr == nil || !strings.Contains(runErr.Error(), "another asc upload still held") {
				t.Fatalf("run error = %v, want the held upload lock error", runErr)
			}
			if !strings.HasPrefix(stderr, "Waiting for another asc upload to finish") {
				t.Fatalf("stderr = %q, want the waiting notice", stderr)
			}
			if requests != 0 {
				t.Fatalf("sent %d requests while another upload held the lock", requests)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
		})
	}
}
