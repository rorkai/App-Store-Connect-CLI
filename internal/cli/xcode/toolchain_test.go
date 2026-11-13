package xcode

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestXcodeDoctorCommandPassesFlagsAndPrintsJSON(t *testing.T) {
	originalRun := runToolchainDoctor
	t.Cleanup(func() { runToolchainDoctor = originalRun })

	var gotOptions localxcode.ToolchainOptions
	runToolchainDoctor = func(_ context.Context, options localxcode.ToolchainOptions) (*localxcode.ToolchainReport, error) {
		gotOptions = options
		return &localxcode.ToolchainReport{
			Status:       localxcode.ToolchainStatusOK,
			Source:       localxcode.ToolchainSourceFlag,
			DeveloperDir: "/Applications/Xcode.app/Contents/Developer",
			XcodePath:    "/Applications/Xcode.app",
			XcodeVersion: "16.4",
			XcodeBuild:   "16F6",
			Checks: []localxcode.ToolchainCheck{
				{Name: "developer_dir", Status: localxcode.ToolchainCheckStatusOK, Message: "Developer directory is available"},
			},
		}, nil
	}

	cmd := XcodeDoctorCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--developer-dir", "/Applications/Xcode.app",
		"--sdk", "iphonesimulator",
		"--output", "json",
		"--pretty",
	}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}

	var runErr error
	stdout, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error = %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if gotOptions.DeveloperDir != "/Applications/Xcode.app" || gotOptions.SDK != "iphonesimulator" {
		t.Fatalf("unexpected toolchain options: %+v", gotOptions)
	}
	if gotOptions.LogWriter == nil {
		t.Fatal("LogWriter = nil, want stderr writer")
	}

	var payload asc.XcodeToolchainDoctorResult
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if payload.Status != string(localxcode.ToolchainStatusOK) || payload.XcodeVersion != "16.4" || payload.XcodeBuild != "16F6" {
		t.Fatalf("unexpected JSON payload: %+v", payload)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("json.Unmarshal(raw) error = %v", err)
	}
	if _, ok := raw["developerDir"]; !ok {
		t.Fatalf("JSON payload missing camelCase developerDir: %s", stdout)
	}
	if _, ok := raw["developer_dir"]; ok {
		t.Fatalf("JSON payload contains internal snake_case developer_dir: %s", stdout)
	}
}

func TestXcodeDoctorCommandPrintsFailureReportAndReturnsError(t *testing.T) {
	originalRun := runToolchainDoctor
	t.Cleanup(func() { runToolchainDoctor = originalRun })

	runToolchainDoctor = func(_ context.Context, _ localxcode.ToolchainOptions) (*localxcode.ToolchainReport, error) {
		return &localxcode.ToolchainReport{
			Status: localxcode.ToolchainStatusFail,
			Checks: []localxcode.ToolchainCheck{{
				Name:    "developer_dir",
				Status:  localxcode.ToolchainCheckStatusFail,
				Message: "Developer directory is unavailable",
			}},
		}, errors.New("developer directory is unavailable")
	}

	cmd := XcodeDoctorCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}

	var runErr error
	stdout, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "xcode doctor") {
		t.Fatalf("Exec() error = %v, want xcode doctor failure", runErr)
	}
	if !strings.Contains(stderr, "Error: xcode doctor: toolchain checks failed") {
		t.Fatalf("stderr = %q, want a concise failure diagnostic", stderr)
	}
	var payload asc.XcodeToolchainDoctorResult
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if payload.Status != string(localxcode.ToolchainStatusFail) {
		t.Fatalf("payload status = %q, want fail", payload.Status)
	}
}

func TestXcodeDoctorCommandDoesNotInventBetaWhenSelectionFails(t *testing.T) {
	originalRun := runToolchainDoctor
	t.Cleanup(func() { runToolchainDoctor = originalRun })

	runToolchainDoctor = func(_ context.Context, _ localxcode.ToolchainOptions) (*localxcode.ToolchainReport, error) {
		return &localxcode.ToolchainReport{
			Status: localxcode.ToolchainStatusFail,
			Source: localxcode.ToolchainSourceXcodeSelect,
			Checks: []localxcode.ToolchainCheck{{
				Name:    "developer_dir",
				Status:  localxcode.ToolchainCheckStatusFail,
				Message: "xcode-select returned an empty developer directory",
			}},
		}, errors.New("resolve developer directory: xcode-select returned an empty developer directory")
	}

	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			cmd := XcodeDoctorCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{"--output", format}); err != nil {
				t.Fatalf("FlagSet.Parse() error = %v", err)
			}
			stdout, _ := captureCommandOutput(t, func() error {
				return cmd.Exec(context.Background(), nil)
			})
			if format == "json" {
				var raw map[string]json.RawMessage
				if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
					t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
				}
				if _, ok := raw["beta"]; ok {
					t.Fatalf("failed selection JSON must omit unknown beta, got %s", stdout)
				}
				return
			}
			if !strings.Contains(stdout, "beta") || !strings.Contains(stdout, "unknown") {
				t.Fatalf("%s output must make unknown beta explicit: %s", format, stdout)
			}
		})
	}
}

func TestXcodeDoctorCommandRendersHumanOutput(t *testing.T) {
	originalRun := runToolchainDoctor
	t.Cleanup(func() { runToolchainDoctor = originalRun })

	runToolchainDoctor = func(_ context.Context, _ localxcode.ToolchainOptions) (*localxcode.ToolchainReport, error) {
		beta := true
		return &localxcode.ToolchainReport{
			Status:       localxcode.ToolchainStatusWarn,
			Source:       localxcode.ToolchainSourceEnvironment,
			DeveloperDir: "/Applications/Xcode-beta.app/Contents/Developer",
			XcodePath:    "/Applications/Xcode-beta.app",
			XcodeVersion: "16.4 beta 2",
			XcodeBuild:   "16F6",
			Beta:         &beta,
			Checks: []localxcode.ToolchainCheck{
				{Name: "developer_dir", Status: localxcode.ToolchainCheckStatusOK, Message: "Developer directory is available"},
				{Name: "beta", Status: localxcode.ToolchainCheckStatusWarn, Message: "selected developer directory appears to be a beta Xcode build"},
			},
		}, nil
	}

	for _, format := range []string{"table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			cmd := XcodeDoctorCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{"--output", format}); err != nil {
				t.Fatalf("FlagSet.Parse() error = %v", err)
			}
			stdout, stderr := captureCommandOutput(t, func() error {
				return cmd.Exec(context.Background(), nil)
			})
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			for _, header := range []string{"check", "status", "path", "message"} {
				if !strings.Contains(stdout, header) {
					t.Fatalf("%s output missing %q: %s", format, header, stdout)
				}
			}
			if !strings.Contains(stdout, "summary") || !strings.Contains(stdout, "warn") {
				t.Fatalf("%s output missing final warning summary: %s", format, stdout)
			}
			if got := strings.Count(stdout, "selected developer directory appears to be a beta Xcode build"); got != 1 {
				t.Fatalf("%s output rendered the beta warning %d times, want exactly once: %s", format, got, stdout)
			}
		})
	}
}

func TestXcodeDoctorCommandRejectsInvalidInputBeforeRunning(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty developer dir", args: []string{"--developer-dir", ""}, want: "--developer-dir must not be empty"},
		{name: "empty sdk", args: []string{"--sdk", "  "}, want: "--sdk must not be empty"},
		{name: "invalid output", args: []string{"--output", "yaml"}, want: "must be one of"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalRun := runToolchainDoctor
			t.Cleanup(func() { runToolchainDoctor = originalRun })
			runToolchainDoctor = func(context.Context, localxcode.ToolchainOptions) (*localxcode.ToolchainReport, error) {
				t.Fatal("runToolchainDoctor must not run for invalid input")
				return nil, nil
			}

			cmd := XcodeDoctorCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("FlagSet.Parse() error = %v", err)
			}
			var runErr error
			stdout, stderr := captureCommandOutput(t, func() error {
				runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
				return runErr
			})
			if !shared.IsReportedUsageError(runErr) || errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("Exec() error = %v, want concise reported usage error", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
		})
	}
}
