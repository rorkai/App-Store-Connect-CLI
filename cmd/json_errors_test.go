package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestRunOptInJSONError(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("DO_NOT_TRACK", "1")
	t.Setenv("ASC_DEFAULT_OUTPUT", "invalid")
	for _, tt := range []struct {
		name string
		args []string
		exit int
		kind string
	}{
		{"unknown command", []string{"--error-format=json", "not-a-command"}, ExitUsage, "unknown_command"},
		{"unknown group child", []string{"--error-format=json", "apps", "not-a-command"}, ExitUsage, "unknown_command"},
		{"removed group child", []string{"--error-format=json", "review", "items-get"}, ExitUsage, "unknown_command"},
		{"unknown flag", []string{"--error-format", "json", "apps", "list", "--not-a-flag"}, ExitUsage, "invalid_flag"},
		{"unknown root flag before format", []string{"--not-a-flag", "--error-format=json", "apps", "list"}, ExitUsage, "invalid_flag"},
		{"stray leaf operand", []string{"--error-format=json", "apps", "view", "123"}, ExitUsage, "usage_error"},
		{"literal flag query with invalid limit", []string{"--error-format=json", "search", "--output", "json", "--limit", "0", "--", "--app"}, ExitUsage, "usage_error"},
		{"missing input", []string{"--error-format=json", "metadata", "push", "--output", "json"}, ExitUsage, "required_input_missing"},
		{"legacy usage writer", []string{"--error-format=json", "profiles", "download", "--all", "--id", "X"}, ExitUsage, "usage_error"},
		{"auth", []string{"--error-format=json", "apps", "list"}, ExitAuth, "authentication_required"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := shared.DefaultDeps()
			d.Client = func() (*asc.Client, error) { return nil, shared.ErrMissingAuth }
			var stdout, stderr bytes.Buffer
			d.IO.Out, d.IO.ErrOut = &stdout, &stderr
			if got := RunWithDeps(tt.args, "test-version", d); got != tt.exit {
				t.Fatalf("exit = %d, want %d, stderr = %q", got, tt.exit, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("unexpected stdout: %q", stdout.String())
			}
			var envelope struct {
				SchemaVersion int `json:"schema_version"`
				Error         struct {
					Type     string `json:"type"`
					ExitCode int    `json:"exit_code"`
					Message  string `json:"message"`
					Hint     string `json:"hint"`
				} `json:"error"`
			}
			if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
				t.Fatalf("stderr must be one JSON error: %v, got %q", err, stderr.String())
			}
			if envelope.SchemaVersion != 1 || envelope.Error.Type != tt.kind || envelope.Error.ExitCode != tt.exit || envelope.Error.Message == "" || envelope.Error.Hint == "" {
				t.Fatalf("unexpected error envelope: %s", stderr.String())
			}
		})
	}
}

func TestRunRejectsInvalidErrorFormatBeforeExecution(t *testing.T) {
	t.Setenv("DO_NOT_TRACK", "1")
	d := shared.DefaultDeps()
	d.Client = func() (*asc.Client, error) {
		t.Fatal("invalid error format must be rejected before creating a client")
		return nil, nil
	}
	var stderr bytes.Buffer
	d.IO.ErrOut = &stderr
	if got := RunWithDeps([]string{"--error-format", "yaml", "apps", "list"}, "test-version", d); got != ExitUsage {
		t.Fatalf("exit = %d, want %d", got, ExitUsage)
	}
	if got := stderr.String(); got != "Error: --error-format requires text or json. Place it before the subcommand.\n" {
		t.Fatalf("unexpected stderr: %q", got)
	}
}
