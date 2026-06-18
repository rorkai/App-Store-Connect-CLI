package cmdtest

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestTelemetryStatusIsEnabledByDefault(t *testing.T) {
	setCmdtestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"telemetry", "status", "--output", "json"}, "1.2.3"); code != 0 {
			t.Fatalf("status exit code = %d", code)
		}
	})
	if stderr != "" {
		t.Fatalf("status stderr = %q", stderr)
	}

	var status struct {
		Enabled bool   `json:"enabled"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("status json error: %v\n%s", err, stdout)
	}
	if !status.Enabled || status.Reason != "" {
		t.Fatalf("expected telemetry enabled by default, got %+v", status)
	}
}

func TestTelemetryCommands(t *testing.T) {
	home := setCmdtestHome(t)
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

func TestTelemetryStatusUsesDefaultOutput(t *testing.T) {
	tests := []struct {
		name          string
		defaultOutput string
		wantJSON      bool
	}{
		{name: "non-interactive defaults to JSON", wantJSON: true},
		{name: "table environment default", defaultOutput: "table"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setCmdtestHome(t)
			t.Setenv("ASC_DEFAULT_OUTPUT", test.defaultOutput)

			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run([]string{"telemetry", "status"}, "1.2.3"); code != 0 {
					t.Fatalf("status exit code = %d", code)
				}
			})
			if stderr != "" {
				t.Fatalf("status stderr = %q", stderr)
			}
			if test.wantJSON {
				var status map[string]any
				if err := json.Unmarshal([]byte(stdout), &status); err != nil {
					t.Fatalf("default status output is not JSON: %v\n%s", err, stdout)
				}
				return
			}
			if !strings.Contains(stdout, "Telemetry") || !strings.Contains(stdout, "Enabled") {
				t.Fatalf("status table output = %q", stdout)
			}
		})
	}
}

func TestTelemetryStatusRejectsInvalidOutput(t *testing.T) {
	setCmdtestHome(t)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"telemetry", "status", "--output", "yaml"}, "1.2.3")
	})
	if code != 2 {
		t.Fatalf("status exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Fatalf("status stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "unsupported format: yaml") {
		t.Fatalf("status stderr = %q, want invalid output error", stderr)
	}
}
