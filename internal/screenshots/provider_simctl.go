package screenshots

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// SimctlProvider captures a screenshot via xcrun simctl io ... screenshot.
type SimctlProvider struct{}

// Capture runs simctl io <udid> screenshot <path> and returns the path to the PNG.
func (p *SimctlProvider) Capture(ctx context.Context, req CaptureRequest) (string, error) {
	udid := strings.TrimSpace(req.UDID)
	if udid == "" {
		udid = "booted"
	}

	pngPath := filepath.Join(req.OutputDir, req.Name+".png")
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "io", udid, "screenshot", pngPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("simctl: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return pngPath, nil
}
