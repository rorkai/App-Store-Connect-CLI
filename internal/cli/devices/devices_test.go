package devices

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
	"time"
)

func TestDevicesRegisterCommand_UDIDConflict(t *testing.T) {
	cmd := DevicesRegisterCommand()

	if err := cmd.FlagSet.Parse([]string{"--name", "Device", "--udid", "UDID", "--udid-from-system", "--platform", "MAC_OS"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when udid flags conflict, got %v", err)
	}
}

func TestDevicePlatformContract(t *testing.T) {
	if got := strings.Join(devicePlatformList(), ", "); got != "IOS, MAC_OS, UNIVERSAL" {
		t.Fatalf("devicePlatformList() = %q, want %q", got, "IOS, MAC_OS, UNIVERSAL")
	}

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr string
	}{
		{name: "iOS", value: " ios ", want: "IOS"},
		{name: "macOS", value: "mac_os", want: "MAC_OS"},
		{name: "universal", value: "universal", want: "UNIVERSAL"},
		{name: "reject tvOS", value: "TV_OS", wantErr: "--platform must be one of: IOS, MAC_OS, UNIVERSAL"},
		{name: "reject visionOS", value: "VISION_OS", wantErr: "--platform must be one of: IOS, MAC_OS, UNIVERSAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeDevicePlatform(tt.value)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("normalizeDevicePlatform(%q) error = %v, want %q", tt.value, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeDevicePlatform(%q) error = %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeDevicePlatform(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestLocalMacUDID_RejectsNonDarwin(t *testing.T) {
	prevGOOS := localUDIDGOOS
	localUDIDGOOS = "linux"
	t.Cleanup(func() {
		localUDIDGOOS = prevGOOS
	})

	_, err := localMacUDID()
	if err == nil {
		t.Fatal("expected error on non-macOS")
	}
	if !strings.Contains(err.Error(), "only supported on macOS") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLocalMacUDID_ParsesUUID(t *testing.T) {
	prevGOOS := localUDIDGOOS
	prevRunner := runLocalUDIDCommand
	localUDIDGOOS = "darwin"
	runLocalUDIDCommand = func(context.Context) ([]byte, error) {
		return []byte(`
		"IOPlatformSerialNumber" = "SERIAL"
		"IOPlatformUUID" = "A1B2C3D4-E5F6-7890-ABCD-1234567890EF"
		`), nil
	}
	t.Cleanup(func() {
		localUDIDGOOS = prevGOOS
		runLocalUDIDCommand = prevRunner
	})

	got, err := localMacUDID()
	if err != nil {
		t.Fatalf("localMacUDID() error = %v", err)
	}
	want := "A1B2C3D4-E5F6-7890-ABCD-1234567890EF"
	if got != want {
		t.Fatalf("localMacUDID() = %q, want %q", got, want)
	}
}

func TestLocalMacUDID_UsesContextTimeout(t *testing.T) {
	prevGOOS := localUDIDGOOS
	prevRunner := runLocalUDIDCommand
	prevTimeout := localUDIDCommandTimeout
	localUDIDGOOS = "darwin"
	localUDIDCommandTimeout = 3 * time.Second
	runLocalUDIDCommand = func(ctx context.Context) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected deadline in context")
		}
		if time.Until(deadline) <= 0 || time.Until(deadline) > 4*time.Second {
			t.Fatalf("unexpected deadline: %v", deadline)
		}
		return nil, context.DeadlineExceeded
	}
	t.Cleanup(func() {
		localUDIDGOOS = prevGOOS
		runLocalUDIDCommand = prevRunner
		localUDIDCommandTimeout = prevTimeout
	})

	_, err := localMacUDID()
	if err == nil {
		t.Fatal("expected error when command fails")
	}
	if !strings.Contains(err.Error(), "failed to read local hardware UUID") {
		t.Fatalf("unexpected error: %v", err)
	}
}
