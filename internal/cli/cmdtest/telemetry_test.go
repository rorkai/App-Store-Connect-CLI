package cmdtest

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestTelemetryCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{"telemetry", "disable"}, "1.2.3")
		if code != 0 {
			t.Fatalf("disable exit code = %d", code)
		}
	})
	if stderr != "" {
		t.Fatalf("disable stderr = %q", stderr)
	}
	if !strings.Contains(stdout, "Telemetry disabled") {
		t.Fatalf("disable stdout = %q", stdout)
	}

	stdout, stderr = captureOutput(t, func() {
		code := rootcmd.Run([]string{"telemetry", "status", "--output", "json"}, "1.2.3")
		if code != 0 {
			t.Fatalf("status exit code = %d", code)
		}
	})
	if stderr != "" {
		t.Fatalf("status stderr = %q", stderr)
	}
	var status struct {
		Path    string `json:"path"`
		Enabled bool   `json:"enabled"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("status json error: %v\n%s", err, stdout)
	}
	if status.Enabled || status.Reason != "state" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if !strings.HasPrefix(filepath.Clean(status.Path), filepath.Clean(home)) {
		t.Fatalf("expected status path under home %q, got %q", home, status.Path)
	}

	stdout, stderr = captureOutput(t, func() {
		code := rootcmd.Run([]string{"telemetry", "enable"}, "1.2.3")
		if code != 0 {
			t.Fatalf("enable exit code = %d", code)
		}
	})
	if stderr != "" {
		t.Fatalf("enable stderr = %q", stderr)
	}
	if !strings.Contains(stdout, "Telemetry enabled") {
		t.Fatalf("enable stdout = %q", stdout)
	}

	stdout, stderr = captureOutput(t, func() {
		code := rootcmd.Run([]string{"telemetry", "reset-id"}, "1.2.3")
		if code != 0 {
			t.Fatalf("reset-id exit code = %d", code)
		}
	})
	if stderr != "" {
		t.Fatalf("reset-id stderr = %q", stderr)
	}
	if !strings.Contains(stdout, "Telemetry install ID reset") {
		t.Fatalf("reset-id stdout = %q", stdout)
	}
}
