package cmdtest

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/uploadlock"
)

func TestAssetUploadsWaitForAnotherAscUploadAfterValidatingInput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv(uploadlock.EnvVar, "")

	screenshotDir := t.TempDir()
	writeCmdtestScreenshotPNG(t, screenshotDir, "01-home.png")
	previewDir := t.TempDir()
	writePreviewFile(t, filepath.Join(previewDir, "01-first.mov"))
	artifactDir := t.TempDir()
	artifactPath := filepath.Join(artifactDir, "failures.json")
	artifact := `{"versionLocalizationId":"LOC_123","setId":"SET_123","cleanupFailures":[{"assetId":"SHOT_BAD"}]}`
	if err := os.WriteFile(artifactPath, []byte(artifact), 0o600); err != nil {
		t.Fatal(err)
	}

	missingSourcePath := filepath.Join(artifactDir, "missing-source.json")
	missingSource := `{"versionLocalizationId":"LOC_123","setId":"SET_123","rootPath":"` + artifactDir + `","pendingFiles":["` + filepath.Join(artifactDir, "gone.png") + `"]}`
	if err := os.WriteFile(missingSourcePath, []byte(missingSource), 0o600); err != nil {
		t.Fatal(err)
	}

	const heldLockError = "another asc upload still held"
	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantWait bool
	}{
		{
			name:     "screenshots upload",
			args:     []string{"screenshots", "upload", "--version-localization", "LOC_123", "--path", screenshotDir, "--device-type", "IPHONE_65"},
			wantErr:  heldLockError,
			wantWait: true,
		},
		{
			name:     "screenshots upload --resume",
			args:     []string{"screenshots", "upload", "--resume", artifactPath},
			wantErr:  heldLockError,
			wantWait: true,
		},
		{
			name:    "screenshots upload --resume reports a missing artifact without waiting",
			args:    []string{"screenshots", "upload", "--resume", filepath.Join(artifactDir, "missing.json")},
			wantErr: "load resume artifact",
		},
		{
			name:    "screenshots upload --resume reports a missing pending file without waiting",
			args:    []string{"screenshots", "upload", "--resume", missingSourcePath},
			wantErr: "gone.png: no such file or directory",
		},
		{
			name:     "video-previews upload",
			args:     []string{"video-previews", "upload", "--version-localization", "LOC_123", "--path", previewDir, "--device-type", "IPHONE_65"},
			wantErr:  heldLockError,
			wantWait: true,
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

			reg := asctest.NewRegistry(t)
			reg.Install()

			stdout, stderr, runErr := runRootCommand(t, test.args)
			if runErr == nil || !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("run error = %v, want %q", runErr, test.wantErr)
			}
			if waited := strings.HasPrefix(stderr, "Waiting for another asc upload to finish"); waited != test.wantWait {
				t.Fatalf("stderr = %q, want waiting notice %t", stderr, test.wantWait)
			}
			if requests := len(reg.Requests()); requests != 0 {
				t.Fatalf("sent %d requests while another upload held the lock", requests)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
		})
	}
}
