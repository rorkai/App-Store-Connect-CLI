package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

func TestRunReportedErrorPrintsOneStderrErrorLine(t *testing.T) {
	t.Run("reported on stdout only", func(t *testing.T) {
		path := writeWorkflowJSON(t, t.TempDir(), `{"workflows": {"beta": {"steps": []}}}`)

		stdout, stderr, code := runWithDeps(t, nil, "workflow", "validate", "--file", path, "--output", "json")

		if code != rootcmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitError)
		}
		var receipt struct {
			Valid bool `json:"valid"`
		}
		if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
			t.Fatalf("decode validation receipt: %v", err)
		}
		if receipt.Valid {
			t.Fatalf("receipt = %+v, want failed validation", receipt)
		}
		if want := "Error: workflow validate: found 1 error(s)\n"; stderr != want {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	})

	t.Run("already printed to stderr", func(t *testing.T) {
		workDir := t.TempDir()
		inputDir := filepath.Join(workDir, "raw")
		if err := os.Mkdir(inputDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFramePNG(t, filepath.Join(inputDir, "01-home.png"), makeRawImage(20, 40))
		installMockFrame(t, func(context.Context, screenshots.FrameRequest) (*screenshots.FrameResult, error) {
			return nil, errors.New("render failed")
		})

		var code int
		_, stderr := captureOutput(t, func() {
			code = rootcmd.Run([]string{
				"screenshots", "frame",
				"--input-dir", inputDir,
				"--output-dir", filepath.Join(workDir, "framed"),
				"--output", "json",
			}, "1.2.3")
		})

		if code != rootcmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitError)
		}
		if strings.Count(stderr, "Error:") != 1 || !strings.HasSuffix(stderr, "Error: screenshots frame: 1 of 1 screenshots failed to frame\n") {
			t.Fatalf("stderr = %q, want exactly one Error line from the command", stderr)
		}
	})
}
