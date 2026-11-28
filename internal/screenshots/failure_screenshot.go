package screenshots

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type failureScreenshotTarget struct {
	root      rootfs.Root
	outputDir string
}

func collectFailureScreenshot(ctx context.Context, target failureScreenshotTarget, udid string, step RunStepResult) *asc.ScreenshotFailureEvidence {
	evidence := &asc.ScreenshotFailureEvidence{
		Attempt: 1, StepIndex: step.Index, Action: step.Action, Status: "failed",
		StepError: step.Error,
	}
	if ctx.Err() != nil {
		evidence.Status, evidence.Error = "skipped", ctx.Err().Error()
		return evidence
	}
	captureCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result, err := captureWithRootProvider(captureCtx, CaptureRequest{
		Provider: ProviderAXe, UDID: udid, OutputDir: target.outputDir,
		Name:          fmt.Sprintf("failure-step-%d-%s", step.Index, uuid.NewString()),
		privateOutput: true,
	}, target.root, failureScreenshotProvider{})
	if err != nil {
		evidence.Error = "failure display could not be captured"
		if ctx.Err() != nil {
			evidence.Status, evidence.Error = "skipped", ctx.Err().Error()
		} else if captureCtx.Err() != nil {
			evidence.Error = "failure display capture timed out"
		}
		return evidence
	}
	relative, err := relativeMatrixOutputPath(target.root.Path(), result.Path)
	if err != nil {
		evidence.Error = "failure display output could not be retained"
		return evidence
	}
	evidence.Status, evidence.Path = "captured", filepath.ToSlash(relative)
	return evidence
}

// failureScreenshotProvider captures without relaunching the app or buffering
// provider diagnostics, which could contain unrelated on-screen information.
type failureScreenshotProvider struct{}

func (failureScreenshotProvider) Capture(ctx context.Context, req CaptureRequest) (string, error) {
	path := filepath.Join(req.OutputDir, req.Name+".png")
	command := exec.CommandContext(ctx, "axe", "screenshot", "--udid", req.UDID, "--output", path)
	command.Stdout, command.Stderr = io.Discard, io.Discard
	command.WaitDelay = time.Second
	if err := command.Run(); err != nil {
		return "", err
	}
	return path, nil
}
