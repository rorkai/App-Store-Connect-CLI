package screenshots

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestRunPlanFailureScreenshotPreservesCancellation(t *testing.T) {
	t.Setenv("PATH", "")
	var plan Plan
	if err := json.Unmarshal([]byte(`{"version":1,"app":{"bundle_id":"com.example.app"},"defaults":{"failure_screenshot":true},"steps":[{"action":"wait","duration_ms":60000}]}`), &plan); err != nil {
		t.Fatal(err)
	}
	plan.App.OutputDir = t.TempDir()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var result *RunResult
		var err error
		done := make(chan struct{})
		go func() { result, err = RunPlan(ctx, &plan); close(done) }()
		synctest.Wait()
		cancel()
		<-done
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("original cancellation lost: %v", err)
		}
		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		var receipt struct {
			FailureEvidence *struct{ Status, Path, StepError string }
		}
		if err := json.Unmarshal(data, &receipt); err != nil || receipt.FailureEvidence == nil || receipt.FailureEvidence.Status != "skipped" || receipt.FailureEvidence.Path != "" || receipt.FailureEvidence.StepError != "context canceled" {
			t.Fatalf("canceled run attempted collection: %s; %v", data, err)
		}
	})
}

func TestCaptureWithRootPrivateOutputDoesNotReplaceExistingFile(t *testing.T) {
	directory := t.TempDir()
	root, err := rootfs.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	var originalMode os.FileMode
	previous := matrixCaptureRootBeforePublishForTest
	matrixCaptureRootBeforePublishForTest = func(path string) {
		if err := os.WriteFile(path, []byte("existing public file"), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		originalMode = info.Mode()
	}
	t.Cleanup(func() { matrixCaptureRootBeforePublishForTest = previous })
	provider := ProviderFunc(func(_ context.Context, request CaptureRequest) (string, error) {
		path := filepath.Join(request.OutputDir, request.Name+".png")
		writeMinimalPNG(t, path, 100, 200)
		return path, nil
	})
	result, err := captureWithRootProvider(context.Background(), CaptureRequest{
		Name: "failure", OutputDir: directory, privateOutput: true,
	}, root, provider)
	if result != nil || !errors.Is(err, os.ErrExist) {
		t.Fatalf("private capture result=%+v error=%v; want existing-file rejection", result, err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "failure.png"))
	if err != nil || string(data) != "existing public file" {
		t.Fatalf("existing file changed: %q; %v", data, err)
	}
	info, err := os.Stat(filepath.Join(directory, "failure.png"))
	if err != nil || info.Mode() != originalMode {
		t.Fatalf("existing file mode changed: %v; %v", info, err)
	}
}
