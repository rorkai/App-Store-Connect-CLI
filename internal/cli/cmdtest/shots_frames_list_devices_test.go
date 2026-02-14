package cmdtest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestShotsFramesListDevices_JSON(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "list-frame-devices",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result struct {
		Default string `json:"default"`
		Devices []struct {
			ID      string `json:"id"`
			Default bool   `json:"default"`
		} `json:"devices"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal devices output: %v\nstdout=%q", err, stdout)
	}

	if result.Default != "iphone-air" {
		t.Fatalf("expected default iphone-air, got %q", result.Default)
	}

	expected := []string{
		"iphone-air",
		"iphone-17-pro",
		"iphone-17-pro-max",
		"iphone-16e",
		"iphone-17",
	}
	if len(result.Devices) != len(expected) {
		t.Fatalf("expected %d devices, got %d", len(expected), len(result.Devices))
	}
	defaultCount := 0
	for idx, want := range expected {
		got := result.Devices[idx]
		if got.ID != want {
			t.Fatalf("device[%d]=%q, want %q", idx, got.ID, want)
		}
		if got.Default {
			defaultCount++
		}
	}
	if defaultCount != 1 || !result.Devices[0].Default {
		t.Fatalf("expected only iphone-air to be marked default: %+v", result.Devices)
	}
}
