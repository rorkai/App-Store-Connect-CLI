package cmdtest

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestTelemetryStatusIsEnabledByDefault(t *testing.T) {
	setCmdtestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	stdout, stderr, code := runWithDeps(t, nil, "telemetry", "status", "--output", "json")
	if code != 0 {
		t.Fatalf("status exit code = %d", code)
	}
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

	stdout, stderr, code := runWithDeps(t, nil, "telemetry", "disable")
	if code != 0 {
		t.Fatalf("disable exit code = %d", code)
	}
	if stderr != "" {
		t.Fatalf("disable stderr = %q", stderr)
	}
	if !strings.Contains(stdout, "Telemetry disabled") {
		t.Fatalf("disable stdout = %q", stdout)
	}

	stdout, stderr, code = runWithDeps(t, nil, "telemetry", "status", "--output", "json")
	if code != 0 {
		t.Fatalf("status exit code = %d", code)
	}
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

	stdout, stderr, code = runWithDeps(t, nil, "telemetry", "enable")
	if code != 0 {
		t.Fatalf("enable exit code = %d", code)
	}
	if stderr != "" {
		t.Fatalf("enable stderr = %q", stderr)
	}
	if !strings.Contains(stdout, "Telemetry enabled") {
		t.Fatalf("enable stdout = %q", stdout)
	}

	stdout, stderr, code = runWithDeps(t, nil, "telemetry", "reset-id")
	if code != 0 {
		t.Fatalf("reset-id exit code = %d", code)
	}
	if stderr != "" {
		t.Fatalf("reset-id stderr = %q", stderr)
	}
	if !strings.Contains(stdout, "No install ID is stored") {
		t.Fatalf("reset-id stdout = %q", stdout)
	}
}

type telemetryStatusJSON struct {
	Enabled          bool   `json:"enabled"`
	Reason           string `json:"reason"`
	InstallIDEnabled bool   `json:"install_id_enabled"`
	InstallID        string `json:"install_id"`
}

func readTelemetryStatus(t *testing.T) telemetryStatusJSON {
	t.Helper()
	stdout, stderr, code := runWithDeps(t, nil, "telemetry", "status", "--output", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("status exit code = %d, stderr = %q", code, stderr)
	}
	var status telemetryStatusJSON
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("status json error: %v\n%s", err, stdout)
	}
	return status
}

func TestTelemetryInstallIDIsOptIn(t *testing.T) {
	setCmdtestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	if status := readTelemetryStatus(t); status.InstallIDEnabled || status.InstallID != "" {
		t.Fatalf("default status = %+v, want no install ID", status)
	}

	stdout, stderr, code := runWithDeps(t, nil, "telemetry", "enable", "--install-id")
	if code != 0 || stderr != "" {
		t.Fatalf("enable --install-id exit code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "Telemetry enabled with install ID") {
		t.Fatalf("enable --install-id stdout = %q", stdout)
	}
	status := readTelemetryStatus(t)
	if !status.Enabled || !status.InstallIDEnabled || status.InstallID == "" {
		t.Fatalf("status after opt-in = %+v", status)
	}

	stdout, stderr, code = runWithDeps(t, nil, "telemetry", "reset-id")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Telemetry install ID reset") {
		t.Fatalf("reset-id exit code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if reset := readTelemetryStatus(t); reset.InstallID == "" || reset.InstallID == status.InstallID {
		t.Fatalf("install ID after reset = %q, was %q", reset.InstallID, status.InstallID)
	}

	stdout, stderr, code = runWithDeps(t, nil, "telemetry", "enable", "--install-id=false")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Telemetry enabled without install ID") {
		t.Fatalf("enable --install-id=false exit code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if status := readTelemetryStatus(t); !status.Enabled || status.InstallIDEnabled || status.InstallID != "" {
		t.Fatalf("status after install ID opt-out = %+v", status)
	}
}

func TestTelemetryOnAndOffAreAliases(t *testing.T) {
	setCmdtestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	stdout, stderr, code := runWithDeps(t, nil, "telemetry", "on", "--install-id")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Telemetry enabled with install ID") {
		t.Fatalf("on exit code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}

	stdout, stderr, code = runWithDeps(t, nil, "telemetry", "off")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Telemetry disabled") {
		t.Fatalf("off exit code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if status := readTelemetryStatus(t); status.Enabled || status.Reason != "state" || status.InstallID != "" {
		t.Fatalf("status after off = %+v, want disabled with no install ID", status)
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

			stdout, stderr, code := runWithDeps(t, nil, "telemetry", "status")
			if code != 0 {
				t.Fatalf("status exit code = %d", code)
			}
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

	stdout, stderr, code := runWithDeps(t, nil, "telemetry", "status", "--output", "yaml")
	if code != 2 {
		t.Fatalf("status exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Fatalf("status stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `(got "yaml")`) {
		t.Fatalf("status stderr = %q, want invalid output error", stderr)
	}
}
