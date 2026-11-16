package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

// frameGateTimeout only stops a broken gate from hanging the test; no
// assertion depends on how long a render takes.
const frameGateTimeout = 30 * time.Second

func TestShotsFrame_ParallelWorkersBoundsConcurrencyAndKeepsInputOrder(t *testing.T) {
	if runtime.NumCPU() < 3 {
		t.Skip("--parallel-workers 3 needs at least 3 CPUs")
	}
	const workers = 3
	inputDir := t.TempDir()
	names := []string{"01-home", "02-search", "03-paywall", "04-profile", "05-settings", "06-share"}
	for _, name := range names {
		writeFramePNG(t, filepath.Join(inputDir, name+".png"), makeRawImage(20, 40))
	}
	outputDir := filepath.Join(t.TempDir(), "framed")

	var mu sync.Mutex
	inFlight, peak, started := 0, 0, 0
	var completed []string
	firstWave := make(chan struct{})
	othersDone := make(chan struct{})
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		name := strings.TrimSuffix(filepath.Base(req.OutputPath), "-iphone-air.png")
		mu.Lock()
		inFlight++
		started++
		peak = max(peak, inFlight)
		if inFlight == workers && started == workers {
			close(firstWave)
		}
		mu.Unlock()
		defer func() {
			mu.Lock()
			inFlight--
			completed = append(completed, name)
			if len(completed) == len(names)-1 && !slices.Contains(completed, names[0]) {
				close(othersDone)
			}
			mu.Unlock()
		}()

		// The first renders wait until N are in flight at once, proving the
		// batch reaches the requested concurrency.
		if !waitFrameGate(firstWave) {
			return nil, errors.New("fewer than --parallel-workers renders ran at once")
		}
		// The first input finishes last, so completion order differs from
		// input order.
		if name == names[0] && !waitFrameGate(othersDone) {
			return nil, errors.New("other inputs did not finish while the first was held")
		}
		if name == "04-profile" {
			return nil, errors.New("koubou generation failed: font not found")
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{Device: req.Device}), nil
	})

	stdout, stderr, err := runRootWithDeps(t, nil, []string{
		"screenshots", "frame",
		"--input-dir", inputDir,
		"--output-dir", outputDir,
		"--parallel-workers", fmt.Sprint(workers),
		"--output", "json",
	}...)
	if code := rootcmd.ExitCodeFromError(err); code != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d (err %v, stderr %q)", code, rootcmd.ExitError, err, stderr)
	}
	if !strings.Contains(stderr, "1 of 6 screenshots failed to frame") {
		t.Fatalf("stderr = %q", stderr)
	}
	if peak != workers {
		t.Fatalf("peak in-flight renders = %d, want %d", peak, workers)
	}
	if completed[len(completed)-1] != names[0] {
		t.Fatalf("completion order %v should end with %s", completed, names[0])
	}
	var receipt asc.ScreenshotFrameBatchResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("stdout is not a batch receipt: %v (%q)", err, stdout)
	}
	if receipt.Total != 6 || receipt.Framed != 5 || receipt.Failed != 1 || receipt.Skipped != 0 {
		t.Fatalf("receipt = %+v", receipt)
	}
	for index, file := range receipt.Files {
		wantPath := filepath.Join(outputDir, names[index]+"-iphone-air.png")
		wantStatus := asc.ScreenshotFrameBatchStatusFramed
		if names[index] == "04-profile" {
			wantStatus = asc.ScreenshotFrameBatchStatusFailed
		}
		if file.Path != wantPath || file.Input != filepath.Join(inputDir, names[index]+".png") || file.Status != wantStatus {
			t.Fatalf("receipt file %d = %+v, want %s %s", index, file, wantPath, wantStatus)
		}
	}
	if !strings.Contains(receipt.Files[3].Error, "font not found") {
		t.Fatalf("failed row = %+v", receipt.Files[3])
	}
}

func waitFrameGate(gate <-chan struct{}) bool {
	select {
	case <-gate:
		return true
	case <-time.After(frameGateTimeout):
		return false
	}
}

func TestShotsFrame_ParallelWorkersRecordsResumeStateForEveryInput(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Skip("--parallel-workers 2 needs at least 2 CPUs")
	}
	workDir := t.TempDir()
	t.Chdir(workDir)
	inputDir := filepath.Join(workDir, "raw")
	if err := os.Mkdir(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for index := range 8 {
		writeFramePNG(t, filepath.Join(inputDir, fmt.Sprintf("%02d-screen.png", index)), makeRawImage(20, 40))
	}
	var mu sync.Mutex
	renders := 0
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		mu.Lock()
		renders++
		mu.Unlock()
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{Device: req.Device}), nil
	})
	args := []string{
		"screenshots", "frame",
		"--input-dir", inputDir,
		"--output-dir", filepath.Join(workDir, "framed"),
		"--parallel-workers", "2",
		"--resume",
		"--output", "json",
	}
	if _, stderr, err := runRootWithDeps(t, nil, args...); err != nil {
		t.Fatalf("first run error = %v (stderr %q)", err, stderr)
	}
	if renders != 8 {
		t.Fatalf("first run renders = %d, want 8", renders)
	}

	renders = 0
	stdout, stderr, err := runRootWithDeps(t, nil, args...)
	if err != nil {
		t.Fatalf("resume run error = %v (stderr %q)", err, stderr)
	}
	if renders != 0 {
		t.Fatalf("resume run rendered %d inputs, want 0", renders)
	}
	var receipt asc.ScreenshotFrameBatchResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("stdout is not a batch receipt: %v (%q)", err, stdout)
	}
	if receipt.Skipped != 8 || receipt.Framed != 0 {
		t.Fatalf("resume receipt = %+v", receipt)
	}
}

func TestShotsFrame_ParallelWorkersValidation(t *testing.T) {
	inputDir := t.TempDir()
	writeFramePNG(t, filepath.Join(inputDir, "home.png"), makeRawImage(20, 40))
	rawPath := filepath.Join(inputDir, "home.png")
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "requires input dir",
			args:    []string{"--input", rawPath, "--parallel-workers", "2"},
			wantErr: "--parallel-workers requires --input-dir",
		},
		{
			name:    "rejects zero",
			args:    []string{"--input-dir", inputDir, "--output-dir", t.TempDir(), "--parallel-workers", "0"},
			wantErr: fmt.Sprintf("--parallel-workers must be between 1 and %d", runtime.NumCPU()),
		},
		{
			name:    "rejects more than the CPU count",
			args:    []string{"--input-dir", inputDir, "--output-dir", t.TempDir(), "--parallel-workers", fmt.Sprint(runtime.NumCPU() + 1)},
			wantErr: fmt.Sprintf("--parallel-workers must be between 1 and %d", runtime.NumCPU()),
		},
		{
			name:    "rejected by watch mode",
			args:    []string{"--config", "/tmp/frame.yaml", "--watch", "--parallel-workers", "2"},
			wantErr: "--parallel-workers cannot be used with --watch",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installMockFrame(t, func(context.Context, screenshots.FrameRequest) (*screenshots.FrameResult, error) {
				t.Fatal("renderer must not run for a usage error")
				return nil, nil
			})
			assertUsageExit(t, append([]string{"screenshots", "frame"}, test.args...), test.wantErr)
		})
	}
}
