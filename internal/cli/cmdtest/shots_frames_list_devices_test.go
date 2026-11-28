package cmdtest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestShotsFramesListDevices_JSON(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	root, rootStdout, rootStderr := shotsRootWithDeps(t)
	if err := root.Parse([]string{
		"screenshots", "list-frame-devices",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
	stdout, stderr := rootStdout.String(), rootStderr.String()
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result struct {
		Default string `json:"default"`
		Devices []struct {
			ID                string   `json:"id"`
			Default           bool     `json:"default"`
			Family            string   `json:"family"`
			DefaultFrameColor string   `json:"defaultFrameColor"`
			FrameColors       []string `json:"frameColors"`
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
		"mac",
		"ipad-pro-13",
		"ipad-pro-11",
		"ipad-air-13",
		"ipad-air-11",
		"ipad-mini",
		"watch-series-11",
		"watch-ultra-3",
		"apple-tv",
		"iphone-duo-outer",
		"iphone-duo-outer-landscape",
		"iphone-duo-inner",
		"iphone-duo-inner-landscape",
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
		if strings.HasPrefix(want, "iphone-duo-") && (got.Family != "iphone" || got.DefaultFrameColor != "night-sky" || len(got.FrameColors) != 2 || got.FrameColors[0] != "night-sky" || got.FrameColors[1] != "star-white") {
			t.Fatalf("%s colors = %+v", want, got)
		}
	}
	if defaultCount != 1 || !result.Devices[0].Default {
		t.Fatalf("expected only iphone-air to be marked default: %+v", result.Devices)
	}
	air := result.Devices[0]
	if air.Family != "iphone" || air.DefaultFrameColor != "light-gold" || len(air.FrameColors) != 4 || air.FrameColors[0] != "light-gold" {
		t.Fatalf("iphone-air colors = %+v", air)
	}
	mac := result.Devices[5]
	if mac.Family != "mac" || mac.DefaultFrameColor != "" || len(mac.FrameColors) != 0 {
		t.Fatalf("mac must not advertise frame colors: %+v", mac)
	}
}
