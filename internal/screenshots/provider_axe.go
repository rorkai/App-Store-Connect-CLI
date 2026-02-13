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

// Capture runs `axe screenshot --output <path> --udid <udid>` and returns the PNG path.
func (p *AXeProvider) Capture(ctx context.Context, req CaptureRequest) (string, error) {
	udid := strings.TrimSpace(req.UDID)
	if udid == "" {
		udid = "booted"
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
