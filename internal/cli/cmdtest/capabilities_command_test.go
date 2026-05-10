package cmdtest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

type capabilitiesTestResponse struct {
	Summary struct {
		Total               int            `json:"total"`
		SchemaEndpointCount int            `json:"schemaEndpointCount"`
		Statuses            map[string]int `json:"statuses"`
	} `json:"summary"`
	Capabilities []struct {
		Area       string   `json:"area"`
		Capability string   `json:"capability"`
		Status     string   `json:"status"`
		Commands   []string `json:"commands"`
		Notes      []string `json:"notes"`
	} `json:"capabilities"`
}

func TestRun_CapabilitiesJSONReportsKnownGaps(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"capabilities", "--output", "json"}, "1.0.0")
		if code != cmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
		}
	})

	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr, got: %s", stderr)
	}

	var resp capabilitiesTestResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("expected JSON output, got error %v and stdout %s", err, stdout)
	}
	if resp.Summary.Total == 0 {
		t.Fatalf("expected capability rows, got empty response")
	}
	if resp.Summary.SchemaEndpointCount == 0 {
		t.Fatalf("expected embedded schema endpoint count to be populated")
	}
	for _, status := range []string{"cli-supported", "experimental-web", "not-public-api"} {
		if resp.Summary.Statuses[status] == 0 {
			t.Fatalf("expected status %q to be represented in summary: %+v", status, resp.Summary.Statuses)
		}
	}

	assertCapability(t, resp, "App Store release submission", "cli-supported", "asc publish appstore --submit")
	assertCapability(t, resp, "App creation", "experimental-web", "asc web apps create")
	assertCapability(t, resp, "Metadata and localization sync", "cli-supported", "asc metadata init")
	assertCapability(t, resp, "Metadata and localization sync", "cli-supported", "asc metadata validate")
	assertCapability(t, resp, "App privacy data-use declarations", "experimental-web", "asc web privacy")
	assertCapability(t, resp, "Transaction tax reports", "not-public-api", "")
}

func TestRun_CapabilitiesFiltersByStatus(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"capabilities", "--status", "not-public-api", "--output", "json"}, "1.0.0")
		if code != cmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
		}
	})

	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr, got: %s", stderr)
	}

	var resp capabilitiesTestResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("expected JSON output, got error %v and stdout %s", err, stdout)
	}
	if len(resp.Capabilities) == 0 {
		t.Fatal("expected filtered capabilities")
	}
	for _, entry := range resp.Capabilities {
		if entry.Status != "not-public-api" {
			t.Fatalf("expected only not-public-api rows, got %+v", entry)
		}
	}
	assertCapability(t, resp, "Direct REST build upload", "not-public-api", "")
}

func TestRun_CapabilitiesFiltersByAreaInMarkdown(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"capabilities", "--area", "release", "--output", "markdown"}, "1.0.0")
		if code != cmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
		}
	})

	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr, got: %s", stderr)
	}
	if !strings.Contains(stdout, "| Area | Capability | Status | Commands | Notes |") {
		t.Fatalf("expected markdown table header, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Release readiness validation") {
		t.Fatalf("expected release capability in markdown output, got: %s", stdout)
	}
	if strings.Contains(stdout, "Transaction tax reports") {
		t.Fatalf("expected area filter to omit finance capability, got: %s", stdout)
	}
}

func TestRun_CapabilitiesInvalidStatusReturnsUsage(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"capabilities", "--status", "maybe"}, "1.0.0")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if !strings.Contains(stderr, "invalid --status") {
		t.Fatalf("expected invalid --status error, got stderr: %s", stderr)
	}
}

func TestRun_CapabilitiesInvalidAreaReturnsUsage(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"capabilities", "--area", "nope"}, "1.0.0")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if !strings.Contains(stderr, "invalid --area") {
		t.Fatalf("expected invalid --area error, got stderr: %s", stderr)
	}
}

func TestRun_CapabilitiesUnexpectedArgsReturnsUsage(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"capabilities", "typo"}, "1.0.0")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if !strings.Contains(stderr, "unexpected arguments: typo") {
		t.Fatalf("expected unexpected argument error, got stderr: %s", stderr)
	}
}

func TestRun_CapabilitiesCommandReferencesResolve(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"capabilities", "--output", "json"}, "1.0.0")
		if code != cmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
		}
	})
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr, got: %s", stderr)
	}

	var resp capabilitiesTestResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("expected JSON output, got error %v and stdout %s", err, stdout)
	}

	root := RootCommand("1.0.0")
	for _, entry := range resp.Capabilities {
		for _, command := range entry.Commands {
			args := capabilityCommandPath(command)
			if len(args) == 0 {
				t.Fatalf("invalid command reference %q", command)
			}
			if !commandPathExists(root, args) {
				t.Fatalf("command reference %q does not resolve in the CLI registry", command)
			}
		}
	}
}

func assertCapability(t *testing.T, resp capabilitiesTestResponse, capability, status, command string) {
	t.Helper()

	for _, entry := range resp.Capabilities {
		if entry.Capability != capability {
			continue
		}
		if entry.Status != status {
			t.Fatalf("expected %q status %q, got %q", capability, status, entry.Status)
		}
		if command == "" {
			return
		}
		for _, gotCommand := range entry.Commands {
			if gotCommand == command {
				return
			}
		}
		t.Fatalf("expected %q commands to include %q, got %v", capability, command, entry.Commands)
	}
	t.Fatalf("capability %q not found in response: %+v", capability, resp.Capabilities)
}

func capabilityCommandPath(command string) []string {
	parts := strings.Fields(command)
	if len(parts) < 2 || parts[0] != "asc" {
		return nil
	}

	args := make([]string, 0, len(parts))
	for _, part := range parts[1:] {
		if strings.HasPrefix(part, "-") {
			break
		}
		args = append(args, part)
	}
	return args
}

func commandPathExists(root *ffcli.Command, args []string) bool {
	current := root
	for _, arg := range args {
		var next *ffcli.Command
		for _, subcommand := range current.Subcommands {
			if subcommand.Name == arg {
				next = subcommand
				break
			}
		}
		if next == nil {
			return false
		}
		current = next
	}
	return true
}
