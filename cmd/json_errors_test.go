package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		{"group flag before subcommand", []string{"--error-format", "json", "reviews", "--stars", "1", "list", "--app", "123"}, ExitUsage, "invalid_input"},
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

func TestRunReviewDetailsFileDiagnostics(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("DO_NOT_TRACK", "1")

	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid-review.json")
	if err := os.WriteFile(invalid, []byte(`{"private_fixture_field":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		path string
		kind shared.DiagnosticCode
	}{
		{"missing file", filepath.Join(dir, "missing-review.json"), shared.DiagnosticFileNotFound},
		{"invalid schema", invalid, shared.DiagnosticFileInvalidFormat},
	} {
		t.Run(test.name, func(t *testing.T) {
			deps := shared.DefaultDeps()
			deps.Client = func() (*asc.Client, error) {
				t.Fatal("review file must be validated before making an API client")
				return nil, nil
			}
			var stdout, stderr bytes.Buffer
			deps.IO.Out, deps.IO.ErrOut = &stdout, &stderr
			args := []string{"--error-format=json", "review", "details", "plan", "--version-id", "fixture-version", "--file", test.path}
			if got := RunWithDeps(args, "test-version", deps); got != ExitUsage || stdout.Len() != 0 {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", got, stdout.String(), stderr.String())
			}
			var envelope struct {
				Error jsonError `json:"error"`
			}
			if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
				t.Fatalf("decode stderr %q: %v", stderr.String(), err)
			}
			if envelope.Error.Type != string(test.kind) || envelope.Error.ExitCode != ExitUsage {
				t.Fatalf("error = %+v, want %s with exit %d", envelope.Error, test.kind, ExitUsage)
			}
			if strings.Contains(stderr.String(), filepath.Base(test.path)) || strings.Contains(stderr.String(), "private_fixture_field") {
				t.Fatalf("JSON stderr contains private file details: %q", stderr.String())
			}
		})
	}
}

func TestRunJSONErrorFormatPreservesTerminalStdout(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("DO_NOT_TRACK", "1")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	var outputs []string
	for _, format := range []string{"text", "json"} {
		d := shared.DefaultDeps()
		var stdout, stderr bytes.Buffer
		d.IO.Out, d.IO.ErrOut = &stdout, &stderr
		d.IO.StdoutTTY, d.IO.StderrTTY = true, true
		if code := RunWithDeps([]string{"--error-format", format, "telemetry", "status", "--output", "table"}, "test-version", d); code != ExitSuccess {
			t.Fatalf("%s exit = %d, stderr = %q", format, code, stderr.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("%s stderr = %q, want empty", format, stderr.String())
		}
		outputs = append(outputs, stdout.String())
	}
	if !strings.Contains(outputs[0], "\x1b[1mTELEMETRY\x1b[22m") {
		t.Fatalf("text mode omitted terminal section styling: %q", outputs[0])
	}
	if outputs[1] != outputs[0] {
		t.Fatalf("JSON error mode changed terminal stdout: got %q, want %q", outputs[1], outputs[0])
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
