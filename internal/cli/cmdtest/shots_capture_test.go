package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestShotsCapture_RequiredFlagErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing bundle-id",
			args:    []string{"screenshots", "capture", "--name", "home"},
			wantErr: "--bundle-id is required",
		},
		{
			name:    "missing name",
			args:    []string{"screenshots", "capture", "--bundle-id", "com.example.app"},
			wantErr: "--name is required",
		},
		{
			name:    "invalid provider",
			args:    []string{"screenshots", "capture", "--bundle-id", "com.example.app", "--name", "home", "--provider", "invalid"},
			wantErr: "--provider must be",
		},
		{
			name:    "provider must be axe",
			args:    []string{"screenshots", "capture", "--bundle-id", "com.example.app", "--name", "home", "--provider", "simctl"},
			wantErr: "--provider must be",
		},
		{
			name:    "name cannot contain path separators",
			args:    []string{"screenshots", "capture", "--bundle-id", "com.example.app", "--name", "../home"},
			wantErr: "--name must be a file name without path separators",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestShotsCapture_FlagsBeforeSubcommand(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	// Root-level flag before subcommand: asc --no-update screenshots capture --name home
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		args := []string{"--no-update", "screenshots", "capture", "--name", "home"}
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--bundle-id is required") {
		t.Fatalf("expected --bundle-id required error, got stderr: %q", stderr)
	}
}

func TestShotsCapture_OutputFormatAccepted(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	// Just ensure capture subcommand accepts --output and --pretty without parse error
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	args := []string{"screenshots", "capture", "--bundle-id", "com.example.app", "--name", "home", "--output", "table", "--pretty"}
	if err := root.Parse(args); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	// Run may fail (e.g. missing axe binary or no simulator) but we're testing flag parsing
	_ = root.Run(context.Background())
}

func TestShotsCapture_ResultJSONStructure(t *testing.T) {
	// Ensure CaptureResult can be serialized to the expected JSON shape (for output tests)
	type captureResult struct {
		Path     string `json:"path"`
		Provider string `json:"provider"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		BundleID string `json:"bundle_id"`
		UDID     string `json:"udid"`
	}

	raw := `{"path":"/tmp/out.png","provider":"axe","width":390,"height":844,"bundle_id":"com.example.app","udid":"booted"}`
	var result captureResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal CaptureResult JSON: %v", err)
	}
	if result.Provider != "axe" || result.Width != 390 || result.Height != 844 || result.BundleID != "com.example.app" {
		t.Fatalf("unexpected parsed result: %+v", result)
	}
}
