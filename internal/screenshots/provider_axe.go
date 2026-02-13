package screenshots

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AXeProvider captures a screenshot via the AXe CLI.
type AXeProvider struct{}

// Capture launches the requested app and captures a screenshot via AXe.
func (p *AXeProvider) Capture(ctx context.Context, req CaptureRequest) (string, error) {
	udid := strings.TrimSpace(req.UDID)
	if udid == "" {
		udid = "booted"
	}
	bundleID := strings.TrimSpace(req.BundleID)
	if bundleID != "" {
		launchCmd := exec.CommandContext(ctx, "xcrun", "simctl", "launch", udid, bundleID)
		launchOut, launchErr := launchCmd.CombinedOutput()
		if launchErr != nil {
			return "", fmt.Errorf("xcrun simctl launch %q: %w (output: %s)", bundleID, launchErr, strings.TrimSpace(string(launchOut)))
		}
	}

	pngPath := filepath.Join(req.OutputDir, req.Name+".png")
	cmd := exec.CommandContext(ctx, "axe", "screenshot", "--output", pngPath, "--udid", udid)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("axe: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	if _, statErr := os.Stat(pngPath); statErr != nil {
		return "", fmt.Errorf("axe: screenshot not found at %q: %w", pngPath, statErr)
	}
	return pngPath, nil
}
