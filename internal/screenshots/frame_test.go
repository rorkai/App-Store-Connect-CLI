package screenshots

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFrameDevice_DefaultIsIPhoneAir(t *testing.T) {
	device, err := ParseFrameDevice("")
	if err != nil {
		t.Fatalf("ParseFrameDevice() error = %v", err)
	}
	if device != DefaultFrameDevice() {
		t.Fatalf("expected default device %q, got %q", DefaultFrameDevice(), device)
	}
}

func TestFrameDeviceOptions_DefaultMarked(t *testing.T) {
	options := FrameDeviceOptions()
	if len(options) != len(FrameDeviceValues()) {
		t.Fatalf("expected %d options, got %d", len(FrameDeviceValues()), len(options))
	}

	defaultCount := 0
	for _, option := range options {
		if !option.Default {
			continue
		}
		defaultCount++
		if option.ID != string(DefaultFrameDevice()) {
			t.Fatalf("unexpected default option %q", option.ID)
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly 1 default option, got %d", defaultCount)
	}
}

func TestParseFrameDevice_NormalizesInput(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want FrameDevice
	}{
		{name: "underscores", raw: "iphone_17_pro", want: FrameDeviceIPhone17Pro},
		{name: "spaces mixed case", raw: " iPhone 17 Pro Max ", want: FrameDeviceIPhone17PM},
		{name: "hyphenated", raw: "iphone-16e", want: FrameDeviceIPhone16e},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseFrameDevice(test.raw)
			if err != nil {
				t.Fatalf("ParseFrameDevice(%q) error = %v", test.raw, err)
			}
			if got != test.want {
				t.Fatalf("ParseFrameDevice(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

func TestParseFrameDevice_InvalidValue(t *testing.T) {
	_, err := ParseFrameDevice("iphone-se")
	if err == nil {
		t.Fatal("expected invalid device error")
	}
	if !strings.Contains(err.Error(), "allowed:") {
		t.Fatalf("expected allowed values in error, got %v", err)
	}
}

func TestResolveKoubouOutputSize(t *testing.T) {
	tests := []struct {
		name       string
		value      any
		wantWidth  int
		wantHeight int
		wantOK     bool
	}{
		{name: "named size", value: "iPhone6_9", wantWidth: 1320, wantHeight: 2868, wantOK: true},
		{name: "custom list", value: []any{1200, 2500}, wantWidth: 1200, wantHeight: 2500, wantOK: true},
		{name: "unknown name", value: "iphone7_2", wantOK: false},
		{name: "invalid list", value: []any{"bad", 2}, wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			width, height, ok := resolveKoubouOutputSize(test.value)
			if ok != test.wantOK {
				t.Fatalf("ok = %v, want %v", ok, test.wantOK)
			}
			if !ok {
				return
			}
			if width != test.wantWidth || height != test.wantHeight {
				t.Fatalf("dimensions = %dx%d, want %dx%d", width, height, test.wantWidth, test.wantHeight)
			}
		})
	}
}

func TestParseKoubouConfigMetadata(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "frame.yaml")
	config := `project:
  name: "Demo"
  output_dir: "./out"
  device: "iPhone 17 Pro - Silver - Portrait"
  output_size: "iPhone6_7"
screenshots:
  framed:
    content:
      - type: "image"
        asset: "screenshots/raw.png"
        frame: true
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	metadata := parseKoubouConfigMetadata(configPath)
	if metadata == nil {
		t.Fatal("expected parsed metadata")
	}
	if metadata.FrameRef != "iPhone 17 Pro - Silver - Portrait" {
		t.Fatalf("unexpected frame ref %q", metadata.FrameRef)
	}
	if metadata.DisplayType != "APP_IPHONE_67" {
		t.Fatalf("unexpected display type %q", metadata.DisplayType)
	}
	if metadata.UploadWidth != 1290 || metadata.UploadHeight != 2796 {
		t.Fatalf("unexpected upload dimensions %dx%d", metadata.UploadWidth, metadata.UploadHeight)
	}
}

func TestSelectGeneratedScreenshot_RelativePath(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "frame.yaml")
	if err := os.WriteFile(configPath, []byte("project: {}"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := selectGeneratedScreenshot(configPath, []koubouGenerateResult{
		{Name: "framed", Path: "output/framed.png", Success: true},
	})
	if err != nil {
		t.Fatalf("selectGeneratedScreenshot() error = %v", err)
	}
	want := filepath.Join(configDir, "output", "framed.png")
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}
