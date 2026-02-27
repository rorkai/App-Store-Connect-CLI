//go:build darwin

package screenshots

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const xcodeCLTHint = "Install Xcode Command Line Tools with: xcode-select --install"

// MacOSProvider captures a screenshot of a running macOS app window via screencapture.
type MacOSProvider struct{}

// Capture finds the app window by bundle ID and captures it with screencapture -l.
func (p *MacOSProvider) Capture(ctx context.Context, req CaptureRequest) (string, error) {
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	pngPath := filepath.Join(req.OutputDir, req.Name+".png")

	bundleID := strings.TrimSpace(req.BundleID)
	wid, err := macOSWindowID(ctx, bundleID)
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "screencapture", "-x", "-l", strconv.Itoa(wid), pngPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("screencapture: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	if _, err := os.Stat(pngPath); err != nil {
		return "", fmt.Errorf("screenshot not written to %q: %w", pngPath, err)
	}
	return pngPath, nil
}

// macOSWindowID returns the CGWindowID for the frontmost visible window of the
// app identified by bundleID. Uses a Swift one-liner so no external tools are needed.
func macOSWindowID(ctx context.Context, bundleID string) (int, error) {
	// Embed the bundle ID directly so no argument passing is needed.
	src := fmt.Sprintf(`import Cocoa
let bid = %q
let apps = NSRunningApplication.runningApplications(withBundleIdentifier: bid)
guard let app = apps.first else {
    fputs("app not running: \(bid)\n", stderr); exit(1)
}
let pid = app.processIdentifier
let opts = CGWindowListOption([.optionOnScreenOnly, .excludeDesktopElements])
guard let list = CGWindowListCopyWindowInfo(opts, kCGNullWindowID) as? [[String:Any]] else { exit(1) }
for w in list {
    guard let p = w[kCGWindowOwnerPID as String] as? Int32, p == pid else { continue }
    guard let layer = w[kCGWindowLayer as String] as? Int, layer >= 0 else { continue }
    guard let wid = w[kCGWindowNumber as String] as? Int else { continue }
    guard let b = w[kCGWindowBounds as String] as? [String:CGFloat],
          (b["Width"] ?? 0) > 10, (b["Height"] ?? 0) > 10 else { continue }
    print(wid); exit(0)
}
fputs("no visible window for: \(bid)\n", stderr); exit(1)
`, bundleID)

	cmd := exec.CommandContext(ctx, "swift", "-")
	cmd.Stdin = strings.NewReader(src)
	out, err := cmd.Output()
	if err != nil {
		return 0, formatMacOSWindowLookupError(bundleID, err, out)
	}
	wid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse window ID %q: %w", strings.TrimSpace(string(out)), err)
	}
	return wid, nil
}

func formatMacOSWindowLookupError(bundleID string, err error, out []byte) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("find window for %q: swift not found in PATH. %s", bundleID, xcodeCLTHint)
	}

	msg := ""
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		msg = strings.TrimSpace(string(ee.Stderr))
	}
	if msg == "" {
		msg = strings.TrimSpace(string(out))
	}
	if msg == "" {
		msg = strings.TrimSpace(err.Error())
	}
	if msg == "" {
		msg = "unknown swift error"
	}

	if looksLikeMissingCLT(msg) {
		return fmt.Errorf("find window for %q: %s. %s", bundleID, msg, xcodeCLTHint)
	}

	return fmt.Errorf("find window for %q: %s", bundleID, msg)
}

func looksLikeMissingCLT(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return false
	}
	clues := []string{
		"xcode command line tools",
		"xcrun: error: invalid active developer path",
		"unable to find utility \"swift\"",
		"unable to find utility 'swift'",
		"tool \"swift\" requires xcode",
		"tool 'swift' requires xcode",
	}
	for _, clue := range clues {
		if strings.Contains(normalized, clue) {
			return true
		}
	}
	return false
}
