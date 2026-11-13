package screenshots

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

type matrixSimulatorDevice struct {
	UDID                 string `json:"udid"`
	State                string `json:"state"`
	IsAvailable          bool   `json:"isAvailable"`
	Name                 string `json:"name"`
	DeviceTypeIdentifier string `json:"deviceTypeIdentifier"`
}

func readMatrixSimulatorInventory(ctx context.Context) ([]matrixSimulatorDevice, error) {
	return readMatrixSimulatorInventoryWithTimeout(ctx, matrixSubprocessTimeout)
}

func readMatrixSimulatorInventoryWithTimeout(ctx context.Context, timeout time.Duration) ([]matrixSimulatorDevice, error) {
	inventoryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(inventoryCtx, "xcrun", "simctl", "list", "devices", "--json")
	var output cappedMatrixBuffer
	output.limit = maxMatrixInventoryBytes
	command.Stdout = &output
	command.Stderr = io.Discard
	err := command.Run()
	if parentErr := ctx.Err(); parentErr != nil {
		return nil, parentErr
	}
	if errors.Is(inventoryCtx.Err(), context.DeadlineExceeded) {
		return nil, ErrMatrixInventoryTimeout
	}
	if output.exceeded {
		return nil, errors.New("simulator inventory exceeded the output size limit")
	}
	if err != nil {
		return nil, errors.New("simulator inventory could not be read")
	}
	out := output.Bytes()
	var inventory struct {
		Devices map[string][]matrixSimulatorDevice `json:"devices"`
	}
	if err := json.Unmarshal(out, &inventory); err != nil {
		return nil, errors.New("simulator inventory was invalid")
	}
	devices := make([]matrixSimulatorDevice, 0)
	for _, runtimeDevices := range inventory.Devices {
		devices = append(devices, runtimeDevices...)
	}
	return devices, nil
}

const (
	matrixPreflightSimulatorNotReady = "simulator_not_ready"
	matrixPreflightFrameMismatch     = "frame_family_mismatch"
	matrixPreflightFamilyUnknown     = "simulator_family_unknown"
	matrixPreflightMappingInvalid    = "frame_mapping_invalid"
)

type matrixDeviceFailure struct {
	Code    string
	Message string
}

type matrixFrameMappingError struct {
	device  string
	code    string
	message string
	cause   error
}

func (e *matrixFrameMappingError) Error() string {
	if e == nil {
		return "matrix frame mapping failed"
	}
	return fmt.Sprintf("device %q %s", e.device, e.message)
}

func (e *matrixFrameMappingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func checkMatrixDevices(ctx context.Context, plan *MatrixPlan) (map[string]matrixDeviceFailure, error) {
	failures := make(map[string]matrixDeviceFailure)
	devices, inventoryErr := readMatrixSimulatorInventory(ctx)
	if inventoryErr != nil && isMatrixContextTermination(inventoryErr) {
		return nil, inventoryErr
	}
	for _, device := range plan.Devices {
		if inventoryErr != nil {
			failures[device.ID] = matrixDeviceFailure{
				Code:    matrixPreflightSimulatorNotReady,
				Message: "target simulator is not ready",
			}
			continue
		}
		candidate, found := findMatrixSimulatorDevice(devices, device.UDID)
		if !found || validateMatrixSimulatorDevice(candidate) != nil {
			failures[device.ID] = matrixDeviceFailure{
				Code:    matrixPreflightSimulatorNotReady,
				Message: "target simulator is not ready",
			}
			continue
		}
		if plan.Output.Frame.Enabled {
			frame, _ := matrixFrameMappingForDevice(plan.Output.Frame.DeviceByMatrixDevice, device.ID)
			if err := validateMatrixFrameMappingForSimulator(device.ID, frame, candidate); err != nil {
				failure := matrixDeviceFailure{
					Code:    matrixPreflightFrameMismatch,
					Message: "configured frame does not match simulator family",
				}
				var mappingErr *matrixFrameMappingError
				if errors.As(err, &mappingErr) {
					failure.Code = mappingErr.code
					failure.Message = mappingErr.message
				}
				failures[device.ID] = failure
			}
		}
	}
	return failures, nil
}

func findMatrixSimulatorDevice(devices []matrixSimulatorDevice, udid string) (matrixSimulatorDevice, bool) {
	wanted := normalizeMatrixUDID(udid)
	for _, device := range devices {
		if normalizeMatrixUDID(device.UDID) == wanted {
			return device, true
		}
	}
	return matrixSimulatorDevice{}, false
}

func validateMatrixSimulatorDevice(device matrixSimulatorDevice) error {
	if !device.IsAvailable {
		return errors.New("simulator is unavailable")
	}
	if !strings.EqualFold(strings.TrimSpace(device.State), "booted") {
		return errors.New("simulator is not booted")
	}
	return nil
}

func validateMatrixFrameMappingForSimulator(matrixDevice, frame string, simulator matrixSimulatorDevice) error {
	parsed, err := ParseFrameDevice(frame)
	if err != nil {
		return &matrixFrameMappingError{
			device:  matrixDevice,
			code:    matrixPreflightMappingInvalid,
			message: "configured frame mapping is invalid",
			cause:   err,
		}
	}
	actualFamily := matrixSimulatorFamily(simulator)
	if actualFamily == "unknown" {
		return &matrixFrameMappingError{
			device:  matrixDevice,
			code:    matrixPreflightFamilyUnknown,
			message: "simulator family could not be identified",
		}
	}
	// iPad simulators pass only with an iPad frame; the family check below
	// rejects any other pairing.
	if actualFamily != frameDeviceFamily(parsed) {
		return &matrixFrameMappingError{
			device:  matrixDevice,
			code:    matrixPreflightFrameMismatch,
			message: "configured frame does not match simulator family",
		}
	}
	return nil
}

func matrixSimulatorFamily(device matrixSimulatorDevice) string {
	return matrixDeviceFamily(device.Name + " " + device.DeviceTypeIdentifier)
}

func normalizeMatrixUDID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeMatrixDeviceID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func matrixFrameMappingForDevice(mapping map[string]string, deviceID string) (string, bool) {
	wanted := normalizeMatrixDeviceID(deviceID)
	if wanted == "" {
		return "", false
	}
	var value string
	found := false
	for candidate, frame := range mapping {
		if normalizeMatrixDeviceID(candidate) != wanted {
			continue
		}
		if found {
			return "", false
		}
		found = true
		value = frame
	}
	return value, found
}

type cappedMatrixBuffer struct {
	data     []byte
	limit    int
	exceeded bool
}

func (b *cappedMatrixBuffer) Write(data []byte) (int, error) {
	if b.limit < 0 {
		return 0, errors.New("output limit must not be negative")
	}
	if b.exceeded {
		return len(data), nil
	}
	remaining := b.limit - len(b.data)
	if remaining <= 0 {
		b.exceeded = len(data) > 0
		return len(data), nil
	}
	if len(data) > remaining {
		b.data = append(b.data, data[:remaining]...)
		b.exceeded = true
		return len(data), nil
	}
	b.data = append(b.data, data...)
	return len(data), nil
}

func (b *cappedMatrixBuffer) Bytes() []byte { return b.data }

type simctlMatrixAppearance struct{}

func (simctlMatrixAppearance) Snapshot(ctx context.Context, udid string) (string, error) {
	out, err := runMatrixAppearanceOutput(ctx, "xcrun", "simctl", "ui", udid, "appearance")
	if err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(out)) {
	case "dark":
		return "dark", nil
	case "light":
		return "light", nil
	default:
		return "", errors.New("simulator appearance state was invalid")
	}
}

func (simctlMatrixAppearance) Set(ctx context.Context, udid, appearance string) error {
	appearance = strings.ToLower(strings.TrimSpace(appearance))
	if appearance != "light" && appearance != "dark" {
		return errors.New("simulator appearance must be light or dark")
	}
	return runMatrixAppearance(ctx, "xcrun", "simctl", "ui", udid, "appearance", appearance)
}

func (simctlMatrixAppearance) Restore(ctx context.Context, udid, state string) error {
	state = strings.ToLower(strings.TrimSpace(state))
	if state != "light" && state != "dark" {
		return errors.New("simulator appearance state must be light or dark")
	}
	return runMatrixAppearance(ctx, "xcrun", "simctl", "ui", udid, "appearance", state)
}

func runMatrixAppearance(ctx context.Context, name string, args ...string) error {
	_, err := runMatrixAppearanceOutput(ctx, name, args...)
	return err
}

func runMatrixAppearanceOutput(ctx context.Context, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	stdout := &cappedMatrixBuffer{limit: maxMatrixAppearanceBytes}
	stderr := &cappedMatrixBuffer{limit: maxMatrixAppearanceBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	if stdout.exceeded || stderr.exceeded {
		return "", errors.New("simulator appearance command exceeded the output size limit")
	}
	if err != nil {
		return "", fmt.Errorf("%s failed", name)
	}
	return string(stdout.Bytes()), nil
}
