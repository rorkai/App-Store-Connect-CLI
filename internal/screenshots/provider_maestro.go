package screenshots

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const maestroFlowTemplate = `appId: %s
---
- launchApp
- takeScreenshot: %s
`

// MaestroProvider captures a screenshot via Maestro (launchApp + takeScreenshot).
type MaestroProvider struct{}

// Capture runs a minimal Maestro flow and returns the path to the PNG.
func (p *MaestroProvider) Capture(ctx context.Context, req CaptureRequest) (string, error) {
	dir, err := os.MkdirTemp("", "asc-shots-maestro-*")
	if err != nil {
		return "", fmt.Errorf("maestro: create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	flowPath := filepath.Join(dir, "flow.yaml")
	flowContent := fmt.Sprintf(maestroFlowTemplate, req.BundleID, req.Name)
	if err := os.WriteFile(flowPath, []byte(flowContent), 0o600); err != nil {
		return "", fmt.Errorf("maestro: write flow: %w", err)
	}

	var args []string
	if req.UDID != "" && strings.TrimSpace(strings.ToLower(req.UDID)) != "booted" {
		args = append(args, "--device", strings.TrimSpace(req.UDID))
	}
	args = append(args, "test", flowPath, "--test-output-dir", req.OutputDir)
	cmd := exec.CommandContext(ctx, "maestro", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("maestro: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	pngPath := filepath.Join(req.OutputDir, req.Name+".png")
	if _, statErr := os.Stat(pngPath); statErr != nil {
		return "", fmt.Errorf("maestro: screenshot not found at %q: %w", pngPath, statErr)
	}
	return pngPath, nil
}
