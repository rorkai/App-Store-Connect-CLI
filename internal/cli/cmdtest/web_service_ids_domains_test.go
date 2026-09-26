package cmdtest

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebServiceIDsDomainsSetReportsRefusalOnce(t *testing.T) {
	home := setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(home, "config.json"))
	sessionCalls := 0
	restore := webcmd.SetResolveWebSession(func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		sessionCalls++
		return nil, "", nil
	})
	t.Cleanup(restore)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{
			"web", "service-ids", "domains", "set",
			"--service-id", "service-1",
			"--domain", "example.com",
			"--return-url", "https://example.com/callback",
			"--confirm",
		}, "test")
	})
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitError, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if count := strings.Count(stderr, "no accepted write request has been captured"); count != 1 {
		t.Fatalf("refusal count = %d, want 1; stderr=%q", count, stderr)
	}
	if sessionCalls != 0 {
		t.Fatalf("session resolutions = %d, want 0", sessionCalls)
	}
}

func TestWebServiceIDsDomainsSetRejectsUnusedFlags(t *testing.T) {
	home := setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(home, "config.json"))
	for _, flags := range [][]string{
		{"--apple-id", "unused@example.com"},
		{"--two-factor-code-command", "unused"},
		{"--provider-id", "123"},
		{"--public-provider-id", "unused"},
		{"--developer-team", "unused"},
		{"--output", "json"},
		{"--pretty"},
	} {
		t.Run(flags[0], func(t *testing.T) {
			args := []string{
				"web", "service-ids", "domains", "set",
				"--service-id", "service-1", "--domain", "example.com",
				"--return-url", "https://example.com/callback", "--confirm",
			}
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(append(args, flags...), "test")
			})
			if code != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "unknown flag `"+flags[0]+"`") {
				t.Fatalf("expected unsupported flag diagnostic, got %q", stderr)
			}
		})
	}
}
