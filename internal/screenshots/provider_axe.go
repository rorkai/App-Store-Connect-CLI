package screenshots

import (
	"context"
	"errors"
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
	udid, err := resolveSimulatorUDID(ctx, req.UDID)
	if err != nil {
		return "", err
	}
	req.UDID = udid
	return captureAXe(ctx, req)
}

func captureAXe(ctx context.Context, req CaptureRequest) (string, error) {
	bundleID := strings.TrimSpace(req.BundleID)
	if bundleID != "" {
		launchCmd := exec.CommandContext(ctx, "xcrun", "simctl", "launch", req.UDID, bundleID)
		launchOut, launchErr := launchCmd.CombinedOutput()
		if launchErr != nil {
			return "", fmt.Errorf("xcrun simctl launch %q: %w (output: %s)", bundleID, launchErr, strings.TrimSpace(string(launchOut)))
		}
	}

	pngPath := filepath.Join(req.OutputDir, req.Name+".png")
	cmd := exec.CommandContext(ctx, "axe", "screenshot", "--output", pngPath, "--udid", req.UDID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("axe: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	if _, statErr := os.Stat(pngPath); statErr != nil {
		return "", fmt.Errorf("axe: screenshot not found at %q: %w", pngPath, statErr)
	}
	return pngPath, nil
}

// resolveSimulatorUDID validates an explicit UDID or selects the one booted
// iPhone or iPad simulator, because the AXe CLI needs a concrete, ready target.
func resolveSimulatorUDID(ctx context.Context, requested string) (string, error) {
	udid := strings.TrimSpace(requested)
	explicit := udid != "" && !strings.EqualFold(udid, "booted")
	devices, err := readMatrixSimulatorInventory(ctx)
	if err != nil {
		if explicit {
			return "", fmt.Errorf("find simulator %q: %w", udid, err)
		}
		return "", fmt.Errorf("find booted simulator: %w", err)
	}
	if explicit {
		device, found := findMatrixSimulatorDevice(devices, udid)
		if !found {
			return "", fmt.Errorf("simulator %q was not found; choose a UDID from xcrun simctl list devices", udid)
		}
		if err := validateMatrixSimulatorDevice(device); err != nil {
			return "", fmt.Errorf("simulator %q: %w; select an available, booted simulator from xcrun simctl list devices", udid, err)
		}
		return strings.TrimSpace(device.UDID), nil
	}
	var booted []matrixSimulatorDevice
	for _, device := range devices {
		family := matrixSimulatorFamily(device)
		if validateMatrixSimulatorDevice(device) == nil && (family == "iphone" || family == "ipad") {
			booted = append(booted, device)
		}
	}
	switch len(booted) {
	case 1:
		return strings.TrimSpace(booted[0].UDID), nil
	case 0:
		return "", errors.New("no iPhone or iPad simulator is booted; boot one or pass --udid")
	default:
		names := make([]string, 0, len(booted))
		for _, device := range booted {
			names = append(names, fmt.Sprintf("%s (%s)", device.Name, device.UDID))
		}
		return "", fmt.Errorf("%d simulators are booted (%s); pass --udid to choose one", len(booted), strings.Join(names, ", "))
	}
}
