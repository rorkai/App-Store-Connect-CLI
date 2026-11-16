package cmdtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shots"
)

// installOutputSwappingKou installs a fake Koubou whose first generate call
// moves swapPath aside and replaces it with a symlink to outsidePath, so the
// selected output directory (or one of its parents) is redirected while the
// render is in flight. An empty swapPath renders without swapping.
func installOutputSwappingKou(t *testing.T, inputPNG, swapPath, outsidePath string) {
	t.Helper()
	binDir := t.TempDir()
	script := `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then
  echo "kou 0.20.0"
  exit 0
fi
if [ "$1" = "setup-frames" ]; then
  exit 0
fi
if [ "$1" != "generate" ]; then
  exit 1
fi
if [ -n "$KOU_SWAP_PATH" ] && [ ! -L "$KOU_SWAP_PATH" ]; then
  mv "$KOU_SWAP_PATH" "$KOU_SWAP_PATH.moved"
  ln -s "$KOU_OUTSIDE_PATH" "$KOU_SWAP_PATH"
fi
output_dir=$(dirname "$2")/output
mkdir -p "$output_dir"
cp "$KOU_INPUT" "$output_dir/framed.png"
echo '[{"name":"framed","path":"output/framed.png","success":true,"error":""}]'
`
	if err := os.WriteFile(filepath.Join(binDir, "kou"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KOU_INPUT", inputPNG)
	t.Setenv("KOU_SWAP_PATH", swapPath)
	t.Setenv("KOU_OUTSIDE_PATH", outsidePath)
	restore := shots.SetFrameFunc(nil)
	t.Cleanup(restore)
}

// assertNoFilesUnder fails if any regular file exists beneath dir.
func assertNoFilesUnder(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			t.Errorf("framed output escaped the selected --output-dir: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestShotsFrameOutputDirSwappedForSymlinkMidRunStaysAnchored replaces
// --output-dir, or its parent, with a symlink to another directory while
// Koubou renders. Publishing must stay anchored to the directory selected when
// the command started, for a single --input and for an --input-dir batch.
func TestShotsFrameOutputDirSwappedForSymlinkMidRunStaysAnchored(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The fake Koubou is a POSIX shell script, and replacing a directory
		// with a symlink needs mv/ln semantics and symlink privileges that
		// Windows runners do not provide.
		t.Skip("the fake Koubou script and directory symlink swap require POSIX")
	}
	for _, mode := range []string{"single", "batch"} {
		for _, swap := range []string{"output-dir", "parent"} {
			t.Run(mode+"/"+swap, func(t *testing.T) {
				dir := t.TempDir()
				t.Chdir(dir)
				t.Setenv("ASC_APP_ID", "")
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(dir, "config.json"))

				inputDir := filepath.Join(dir, "raw")
				if err := os.Mkdir(inputDir, 0o755); err != nil {
					t.Fatal(err)
				}
				inputPath := filepath.Join(inputDir, "01-home.png")
				writeFramePNG(t, inputPath, makeRawImage(20, 40))
				writeFramePNG(t, filepath.Join(inputDir, "02-settings.png"), makeRawImage(20, 40))

				parentDir := filepath.Join(dir, "shots")
				outputDir := filepath.Join(parentDir, "framed")
				if err := os.MkdirAll(outputDir, 0o755); err != nil {
					t.Fatal(err)
				}
				outside := t.TempDir()
				swapPath, outsidePath := outputDir, outside
				if swap == "parent" {
					// The redirected parent still offers a framed/ directory.
					if err := os.Mkdir(filepath.Join(outside, "framed"), 0o755); err != nil {
						t.Fatal(err)
					}
					swapPath = parentDir
				}
				installOutputSwappingKou(t, inputPath, swapPath, outsidePath)

				args := []string{"screenshots", "frame", "--output-dir", outputDir, "--output", "json"}
				if mode == "single" {
					args = append(args, "--input", inputPath)
				} else {
					args = append(args, "--input-dir", inputDir)
				}
				stdout, stderr, runErr := runRootWithDeps(t, nil, args...)

				if info, err := os.Lstat(swapPath); err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("fake Koubou did not swap %s for a symlink (err %v)", swapPath, err)
				}
				assertNoFilesUnder(t, outside)
				// The retained root refuses the swapped-in symlink, so the
				// publish fails closed instead of writing anywhere.
				assertNoFilesUnder(t, swapPath+".moved")
				if runErr == nil {
					t.Fatalf("expected the redirected publish to fail (stdout %q)", stdout)
				}
				if code := rootcmd.ExitCodeFromError(runErr); code != rootcmd.ExitError {
					t.Fatalf("exit code = %d, want %d (err %v, stderr %q)", code, rootcmd.ExitError, runErr, stderr)
				}
				if mode == "single" {
					if !strings.Contains(runErr.Error(), "refusing to follow symlink") {
						t.Fatalf("error = %v, want symlink refusal", runErr)
					}
					return
				}
				var receipt asc.ScreenshotFrameBatchResult
				if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
					t.Fatalf("stdout is not a batch receipt: %v (%q)", err, stdout)
				}
				if receipt.Total != 2 || receipt.Failed != 2 || receipt.Framed != 0 {
					t.Fatalf("receipt = %+v", receipt)
				}
				for _, file := range receipt.Files {
					if file.Status != asc.ScreenshotFrameBatchStatusFailed || !strings.Contains(file.Error, "refusing to follow symlink") {
						t.Fatalf("file = %+v, want symlink refusal", file)
					}
				}
			})
		}
	}
}

// TestShotsFrameCreatesMissingOutputDirOnFirstPublish checks that anchoring
// --output-dir before rendering still creates a missing nested directory when
// the first framed image is published.
func TestShotsFrameCreatesMissingOutputDirOnFirstPublish(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The fake Koubou is a POSIX shell script.
		t.Skip("the fake Koubou script requires POSIX")
	}
	for _, mode := range []string{"single", "batch"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(dir, "config.json"))

			inputDir := filepath.Join(dir, "raw")
			if err := os.Mkdir(inputDir, 0o755); err != nil {
				t.Fatal(err)
			}
			inputPath := filepath.Join(inputDir, "01-home.png")
			writeFramePNG(t, inputPath, makeRawImage(20, 40))
			outputDir := filepath.Join(dir, "shots", "framed")
			installOutputSwappingKou(t, inputPath, "", "")

			args := []string{"screenshots", "frame", "--output-dir", outputDir, "--output", "json"}
			if mode == "single" {
				args = append(args, "--input", inputPath)
			} else {
				args = append(args, "--input-dir", inputDir)
			}
			stdout, stderr, err := runRootWithDeps(t, nil, args...)
			if err != nil {
				t.Fatalf("run error = %v (stdout %q, stderr %q)", err, stdout, stderr)
			}
			want := filepath.Join(outputDir, "01-home-iphone-air.png")
			if _, err := os.Stat(want); err != nil {
				t.Fatalf("framed output missing: %v", err)
			}
			if !strings.Contains(stdout, want) {
				t.Fatalf("stdout %q does not report %s", stdout, want)
			}
		})
	}
}
