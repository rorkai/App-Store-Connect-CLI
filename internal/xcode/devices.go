package xcode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// DevicesOptions controls read-only physical iOS discovery through CoreDevice.
type DevicesOptions struct {
	AvailableOnly bool
	Timeout       time.Duration
	Environment   []string
}

// ValidateDevicesOptions validates discovery flags before invoking any tool.
func ValidateDevicesOptions(options DevicesOptions) error {
	if options.Timeout < installMinimumTimeout || options.Timeout > installMaximumTimeout {
		return fmt.Errorf("--timeout must be between 5s and 10m")
	}
	return nil
}

// ListDevices exposes the exact CoreDevice selectors used by Install. It never
// installs, pairs or otherwise changes a device.
func ListDevices(ctx context.Context, options DevicesOptions) (*asc.XcodeDevicesResult, error) {
	if err := ValidateDevicesOptions(options); err != nil {
		return nil, err
	}
	if runtimeGOOS != "darwin" {
		return nil, fmt.Errorf("connected iOS device discovery is supported on macOS only")
	}
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	runner := installRunner
	if runner == nil {
		return nil, fmt.Errorf("CoreDevice discovery runner is required")
	}
	environment := installEnvironment(options.Environment)
	tool, err := runner.ResolveDevicectl(ctx, environment)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		return nil, fmt.Errorf("resolve devicectl through the active Xcode toolchain")
	}
	if !filepath.IsAbs(tool) {
		return nil, fmt.Errorf("resolved devicectl path is not absolute")
	}
	directory, err := os.MkdirTemp("", ".asc-xcode-devices-json-")
	if err != nil {
		return nil, fmt.Errorf("create private device discovery directory")
	}
	defer os.RemoveAll(directory)
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("protect private device discovery directory")
	}
	entries, err := listInstallDevices(ctx, runner, tool, environment, directory, options.Timeout)
	if err != nil {
		return nil, err
	}
	result := &asc.XcodeDevicesResult{Devices: []asc.XcodeDevice{}}
	seen := map[string]bool{}
	for _, entry := range entries {
		if err := validateInstallDeviceEntry(entry); err != nil {
			return nil, err
		}
		device, err := normalizeInstallDevice(entry)
		var missingMembership *installDeviceMembershipUnavailableError
		if err != nil && !errors.As(err, &missingMembership) {
			return nil, err
		}
		if !strings.EqualFold(device.Platform, "iOS") || !strings.EqualFold(device.Reality, "physical") || strings.Contains(strings.ToLower(device.VisibilityClass), "simulator") {
			continue
		}
		if seen[device.Identifier] {
			return nil, fmt.Errorf("device discovery returned an ambiguous CoreDevice identifier")
		}
		seen[device.Identifier] = true
		available := err == nil && isUsableInstallDevice(device)
		if options.AvailableOnly && !available {
			continue
		}
		properties, _ := decodeInstallObject(entry.Properties)
		state, _ := objectFromObject(properties, "state")
		name := firstInstallObjectString(state, "name")
		if name == "" {
			legacy, _ := decodeInstallObject(entry.DeviceProperties)
			name = firstInstallObjectString(legacy, "name")
		}
		result.Devices = append(result.Devices, asc.XcodeDevice{DeviceID: device.Identifier, Name: name, Platform: device.Platform, PairingState: device.PairingState, ConnectionState: device.ConnectionState, Available: available})
	}
	slices.SortFunc(result.Devices, func(a, b asc.XcodeDevice) int { return strings.Compare(a.DeviceID, b.DeviceID) })
	return result, nil
}
