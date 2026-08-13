package cmd

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	authcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRun_VersionFlag(t *testing.T) {
	resetReportFlags(t)
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var telemetryCalls int
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, _ telemetry.EventContext) {
		telemetryCalls++
	}

	stdout, _ := captureCommandOutput(t, func() {
		code := Run([]string{"--version"}, "9.9.9")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if !strings.Contains(stdout, "9.9.9") {
		t.Fatalf("expected version in stdout, got %q", stdout)
	}
	if telemetryCalls != 0 {
		t.Fatalf("telemetry calls = %d, want 0 for version", telemetryCalls)
	}
}

func TestRun_ReportFlagValidationError(t *testing.T) {
	resetReportFlags(t)

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"--report-file", filepath.Join(t.TempDir(), "junit.xml"), "completion", "--shell", "bash"}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "--report is required") {
		t.Fatalf("expected report validation error, got %q", stderr)
	}
}

func TestRun_ReportFlagValidationErrorEmitsTelemetry(t *testing.T) {
	resetReportFlags(t)

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var calls int
	var commandName string
	var duration time.Duration
	var exitCode int
	emitTelemetry = func(command, _ string, elapsed time.Duration, code int, _ telemetry.EventContext) {
		calls++
		commandName = command
		duration = elapsed
		exitCode = code
	}

	captureCommandOutput(t, func() {
		Run([]string{"--report-file", filepath.Join(t.TempDir(), "junit.xml"), "builds", "list"}, "1.0.0")
	})

	if calls != 1 {
		t.Fatalf("telemetry calls = %d, want 1", calls)
	}
	if commandName != "asc builds list" {
		t.Fatalf("telemetry command = %q, want %q", commandName, "asc builds list")
	}
	if duration != 0 {
		t.Fatalf("telemetry duration = %s, want 0", duration)
	}
	if exitCode != ExitUsage {
		t.Fatalf("telemetry exit code = %d, want %d", exitCode, ExitUsage)
	}
}

func TestRun_InvalidReportFormatWinsOverUnknownFlag(t *testing.T) {
	resetReportFlags(t)
	reportPath := filepath.Join(t.TempDir(), "result.xml")

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var calls int
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		calls++
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"--report", "xml",
			"--report-file", reportPath,
			"builds", "list", "--ap", "PRIVATE_VALUE",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	want := "Error: --report must be \"junit\" if specified, got \"xml\"\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if strings.Contains(stderr, "PRIVATE_VALUE") {
		t.Fatalf("stderr leaked a following argument: %q", stderr)
	}
	if _, err := os.Stat(reportPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid report format must not write a report, stat error = %v", err)
	}
	if calls != 1 || gotContext.FailureStage != telemetry.FailureStageValidation ||
		gotContext.OutcomeKind != telemetry.OutcomeUsageError {
		t.Fatalf("unexpected telemetry: calls=%d context=%+v", calls, gotContext)
	}
}

func TestRun_UnknownFlagBetweenCompleteReportOptionsWritesJUnit(t *testing.T) {
	resetReportFlags(t)
	reportPath := filepath.Join(t.TempDir(), "result.xml")

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"--report", "junit",
			"--bogus",
			"--report-file", reportPath,
			"builds", "list",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	want := "Error: unknown flag `--bogus` for `asc`\nFor help:\n  asc --help\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if !strings.Contains(string(data), "unknown flag `--bogus` for `asc`") {
		t.Fatalf("report does not identify the unknown flag: %s", data)
	}
}

func TestRun_UnknownFlagBeforeSpacedBooleanAndReportOptionsWritesJUnit(t *testing.T) {
	resetReportFlags(t)
	reportPath := filepath.Join(t.TempDir(), "result.xml")

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"--bogus",
			"--strict-auth", "false",
			"--report", "junit",
			"--report-file", reportPath,
			"builds", "list",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	want := "Error: unknown flag `--bogus` for `asc`\nFor help:\n  asc --help\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if !strings.Contains(string(data), "unknown flag `--bogus` for `asc`") {
		t.Fatalf("report does not identify the unknown flag: %s", data)
	}
}

func TestRun_UnknownFlagWithValueBeforeReportOptionsWritesJUnit(t *testing.T) {
	resetReportFlags(t)
	reportPath := filepath.Join(t.TempDir(), "result.xml")

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"--profiel", "ci",
			"--report", "junit",
			"--report-file", reportPath,
			"builds", "list",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	want := "Error: unknown flag `--profiel` for `asc`\nTry:\n  --profile\nFor help:\n  asc --help\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if !strings.Contains(string(data), "unknown flag `--profiel` for `asc`") {
		t.Fatalf("report does not identify the unknown flag: %s", data)
	}
}

func TestRun_MalformedTripleDashReportPreservesParseError(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"---report", "xml"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	want := "Error: bad flag syntax: ---report\nFor help:\n  asc --help\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestRun_InvalidReportFormatAfterUnknownFlagStillWins(t *testing.T) {
	resetReportFlags(t)
	reportPath := filepath.Join(t.TempDir(), "result.xml")

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"--bogus",
			"--report", "xml",
			"--report-file", reportPath,
			"builds", "list",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	want := "Error: --report must be \"junit\" if specified, got \"xml\"\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if _, err := os.Stat(reportPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid report format must not write a report, stat error = %v", err)
	}
}

func TestRun_ParseErrorEmitsTelemetry(t *testing.T) {
	resetReportFlags(t)

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var calls int
	var commandName string
	var duration time.Duration
	var exitCode int
	emitTelemetry = func(command, _ string, elapsed time.Duration, code int, _ telemetry.EventContext) {
		calls++
		commandName = command
		duration = elapsed
		exitCode = code
	}

	captureCommandOutput(t, func() {
		Run([]string{"builds", "--definitely-invalid"}, "1.0.0")
	})

	if calls != 1 {
		t.Fatalf("telemetry calls = %d, want 1", calls)
	}
	if commandName != "asc builds" {
		t.Fatalf("telemetry command = %q, want %q", commandName, "asc builds")
	}
	if duration != 0 {
		t.Fatalf("telemetry duration = %s, want 0", duration)
	}
	if exitCode != ExitUsage {
		t.Fatalf("telemetry exit code = %d, want %d", exitCode, ExitUsage)
	}
}

func TestRun_ParseErrorWithoutFlagOutputReturnsUsage(t *testing.T) {
	resetReportFlags(t)

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotExitCode int
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExitCode = exitCode
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"ads", "geo"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "doesn't define an Exec function") {
		t.Fatalf("expected parse failure, got %q", stderr)
	}
	if gotExitCode != ExitUsage || gotContext.FailureStage != telemetry.FailureStageParse ||
		gotContext.OutcomeKind != telemetry.OutcomeUsageError {
		t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
	}
}

func TestRun_ReportWriteFailureReturnsExitError(t *testing.T) {
	resetReportFlags(t)

	reportPath := filepath.Join(t.TempDir(), "junit.xml")
	if err := os.WriteFile(reportPath, []byte("existing"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"--report", "junit",
			"--report-file", reportPath,
			"completion", "--shell", "bash",
		}, "1.0.0")
		if code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	if !strings.Contains(stderr, "failed to write JUnit report") {
		t.Fatalf("expected JUnit write failure in stderr, got %q", stderr)
	}
}

func TestRun_UnknownCommandReturnsUsage(t *testing.T) {
	resetReportFlags(t)

	code := Run([]string{"unknown-command"}, "1.0.0")
	if code != ExitUsage {
		t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
	}
}

func TestRun_BareGroupPrintsHelpToStdoutAndExitsSuccessfully(t *testing.T) {
	resetReportFlags(t)
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var telemetryCalls int
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, _ telemetry.EventContext) {
		telemetryCalls++
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"builds"}, "1.0.0"); code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if !strings.Contains(stdout, "asc builds list") {
		t.Fatalf("expected builds help on stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if telemetryCalls != 0 {
		t.Fatalf("telemetry calls = %d, want 0 for group help", telemetryCalls)
	}
}

func TestRun_BareGroupWritesJUnitReport(t *testing.T) {
	resetReportFlags(t)
	reportPath := filepath.Join(t.TempDir(), "junit.xml")

	captureCommandOutput(t, func() {
		if code := Run([]string{
			"--report", "junit",
			"--report-file", reportPath,
			"builds",
		}, "1.0.0"); code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var suite struct {
		Failures  int `xml:"failures,attr"`
		TestCases []struct {
			Name string `xml:"name,attr"`
		} `xml:"testcase"`
	}
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}
	if suite.Failures != 0 {
		t.Fatalf("failures = %d, want 0", suite.Failures)
	}
	if len(suite.TestCases) != 1 || suite.TestCases[0].Name != "asc builds" {
		t.Fatalf("unexpected testcase payload: %+v", suite.TestCases)
	}
}

func TestRun_UnknownChildWritesJUnitReport(t *testing.T) {
	resetReportFlags(t)
	reportPath := filepath.Join(t.TempDir(), "junit.xml")

	captureCommandOutput(t, func() {
		if code := Run([]string{
			"--report", "junit",
			"--report-file", reportPath,
			"builds", "lsit",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var suite struct {
		Failures  int `xml:"failures,attr"`
		TestCases []struct {
			Name    string `xml:"name,attr"`
			Failure struct {
				Message string `xml:"message,attr"`
			} `xml:"failure"`
		} `xml:"testcase"`
	}
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}
	if suite.Failures != 1 {
		t.Fatalf("failures = %d, want 1", suite.Failures)
	}
	if len(suite.TestCases) != 1 || suite.TestCases[0].Name != "asc builds" {
		t.Fatalf("unexpected testcase payload: %+v", suite.TestCases)
	}
	if suite.TestCases[0].Failure.Message != "unknown command `asc builds lsit`" {
		t.Fatalf("failure message = %q", suite.TestCases[0].Failure.Message)
	}
}

func TestRun_UnknownFlagWritesJUnitReport(t *testing.T) {
	resetReportFlags(t)
	reportPath := filepath.Join(t.TempDir(), "junit.xml")

	captureCommandOutput(t, func() {
		if code := Run([]string{
			"--report", "junit",
			"--report-file", reportPath,
			"builds", "list", "--ap=PRIVATE_VALUE",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var suite struct {
		Failures  int `xml:"failures,attr"`
		TestCases []struct {
			Name    string `xml:"name,attr"`
			Failure struct {
				Message string `xml:"message,attr"`
			} `xml:"failure"`
		} `xml:"testcase"`
	}
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}
	if suite.Failures != 1 {
		t.Fatalf("failures = %d, want 1", suite.Failures)
	}
	if len(suite.TestCases) != 1 || suite.TestCases[0].Name != "asc builds list" {
		t.Fatalf("unexpected testcase payload: %+v", suite.TestCases)
	}
	if suite.TestCases[0].Failure.Message != "unknown flag `--ap` for `asc builds list`" {
		t.Fatalf("failure message = %q", suite.TestCases[0].Failure.Message)
	}
	if strings.Contains(string(data), "PRIVATE_VALUE") {
		t.Fatalf("report leaked an inline flag value: %s", data)
	}
}

func TestRun_UnknownInputReportWriteFailureReturnsExitError(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPrefix string
	}{
		{
			name:       "unknown command",
			args:       []string{"builds", "lsit"},
			wantPrefix: "Error: unknown command `asc builds lsit`\n",
		},
		{
			name:       "unknown flag",
			args:       []string{"builds", "list", "--ap", "PRIVATE_VALUE"},
			wantPrefix: "Error: unknown flag `--ap` for `asc builds list`\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			reportPath := filepath.Join(t.TempDir(), "junit.xml")
			if err := os.WriteFile(reportPath, []byte("existing"), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var calls int
			var gotCommand string
			var gotExit int
			var gotContext telemetry.EventContext
			emitTelemetry = func(command, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				calls++
				gotCommand = command
				gotExit = exitCode
				gotContext = eventContext
			}

			args := append([]string{
				"--report", "junit",
				"--report-file", reportPath,
			}, test.args...)
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(args, "1.0.0"); code != ExitError {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.HasPrefix(stderr, test.wantPrefix) || !strings.Contains(stderr, "Error: failed to write JUnit report:") {
				t.Fatalf("unexpected stderr: %q", stderr)
			}
			if strings.Contains(stderr, "PRIVATE_VALUE") {
				t.Fatalf("stderr leaked a following argument: %q", stderr)
			}
			if calls != 1 || gotExit != ExitError || gotContext.FailureStage != telemetry.FailureStageExecution ||
				gotContext.ErrorKind != telemetry.ErrorKindOther || gotContext.OutcomeKind != telemetry.OutcomeInternalError {
				t.Fatalf("unexpected telemetry: calls=%d command=%q exit=%d context=%+v", calls, gotCommand, gotExit, gotContext)
			}
			if !strings.HasPrefix(gotCommand, "asc builds") {
				t.Fatalf("telemetry command = %q, want canonical builds path", gotCommand)
			}
		})
	}
}

func TestUnknownInputJUnitErrorsAreTerminalSafe(t *testing.T) {
	commandErr := unknownCommandError(
		invocationAnalysis{unknownToken: "bad\x1b[31m\r\n"},
		"asc builds",
	)
	if got, want := commandErr.Error(), "unknown command `asc builds bad[31m  `"; got != want {
		t.Fatalf("unknown command error = %q, want %q", got, want)
	}
	if strings.ContainsAny(commandErr.Error(), "\x1b\r\n") {
		t.Fatalf("unknown command report error contains terminal controls: %q", commandErr)
	}

	flagErr := unknownFlagError(
		invocationAnalysis{unknownToken: "--ap=PRIVATE_VALUE\x1b[31m"},
		"asc builds list",
	)
	if got, want := flagErr.Error(), "unknown flag `--ap` for `asc builds list`"; got != want {
		t.Fatalf("unknown flag error = %q, want %q", got, want)
	}
}

func TestRun_ValidateMissingRequiredFlagsReturnsUsage(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"validate"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--version or --version-id is required") {
		t.Fatalf("expected missing version error, got %q", stderr)
	}
	if gotContext.ErrorKind != telemetry.ErrorKindMissingRequired || gotContext.FailureStage != telemetry.FailureStageValidation {
		t.Fatalf("unexpected telemetry context: %+v", gotContext)
	}
	if gotContext.FailureParameter != "--version" || gotContext.OutcomeKind != telemetry.OutcomeUsageError {
		t.Fatalf("unexpected missing-parameter telemetry context: %+v", gotContext)
	}
}

func TestRun_IntroductoryOfferSelectorReportedUsageEmitsUsageTelemetry(t *testing.T) {
	resetReportFlags(t)
	for _, key := range []string{
		"ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH",
		"ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_STRICT_AUTH",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	for _, test := range []struct {
		name     string
		selector []string
		wantKind telemetry.ErrorKind
	}{
		{name: "missing", wantKind: telemetry.ErrorKindMissingRequired},
		{name: "conflicting", selector: []string{"--territory", "USA", "--all-territories"}, wantKind: telemetry.ErrorKindInvalidValue},
	} {
		t.Run(test.name, func(t *testing.T) {
			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				gotExitCode = exitCode
				gotContext = eventContext
			}

			args := []string{
				"subscriptions", "offers", "introductory", "create",
				"--subscription-id", "8000000001",
				"--offer-duration", "ONE_MONTH",
				"--offer-mode", "FREE_TRIAL",
				"--number-of-periods", "1",
			}
			args = append(args, test.selector...)
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})
			if stdout != "" || !strings.Contains(stderr, "For help:\n  asc subscriptions offers introductory create --help") {
				t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
			}
			if gotExitCode != ExitUsage || gotContext.ErrorKind != test.wantKind ||
				gotContext.FailureStage != telemetry.FailureStageValidation ||
				gotContext.OutcomeKind != telemetry.OutcomeUsageError {
				t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
			}
		})
	}
}

func TestRun_AuthLoginInvalidKeyTypeEmitsFailureParameter(t *testing.T) {
	resetReportFlags(t)
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_KEY_TYPE", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
	resetSelectedProfile(t)

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"auth", "login",
			"--name", "Test Key",
			"--key-id", "KEY123",
			"--key-type", "personal",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--key-type must be one of: team, individual") {
		t.Fatalf("expected invalid key type error, got %q", stderr)
	}
	if gotContext.ErrorKind != telemetry.ErrorKindInvalidValue ||
		gotContext.FailureStage != telemetry.FailureStageValidation ||
		gotContext.OutcomeKind != telemetry.OutcomeUsageError ||
		gotContext.FailureParameter != "--key-type" {
		t.Fatalf("unexpected invalid-value context: %+v", gotContext)
	}
}

func TestRun_AuthStatusValidationFailuresEmitExpectedNegative(t *testing.T) {
	resetReportFlags(t)
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)
	configPath := filepath.Join(tempDir, "config.json")
	if err := config.SaveAt(configPath, &config.Config{
		DefaultKeyName: "default",
		Keys: []config.Credential{{
			Name:           "default",
			KeyID:          "KEY123",
			IssuerID:       "ISS456",
			PrivateKeyPath: keyPath,
		}},
	}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	restoreValidator := authcli.SetStatusValidateCredential(func(context.Context, auth.Credential) error {
		return errors.New("validation failed")
	})
	t.Cleanup(restoreValidator)

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotExitCode int
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExitCode = exitCode
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"auth", "status", "--validate", "--output", "table"}, "1.0.0"); code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "default (Key ID: KEY123): failed") {
		t.Fatalf("expected validation failure output, got %q", stdout)
	}
	if gotExitCode != ExitError || gotContext.FailureStage != telemetry.FailureStageValidation ||
		gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative {
		t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
	}
}

func TestRun_MetadataValidateFindingsEmitExpectedNegative(t *testing.T) {
	resetReportFlags(t)
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var gotExitCode int
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExitCode = exitCode
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"metadata", "validate", "--dir", t.TempDir()}, "1.0.0"); code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result struct {
		Valid      bool `json:"valid"`
		ErrorCount int  `json:"errorCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if result.Valid || result.ErrorCount != 1 {
		t.Fatalf("unexpected validation result: %+v", result)
	}
	if gotExitCode != ExitError || gotContext.FailureStage != telemetry.FailureStageValidation ||
		gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative {
		t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
	}
}

func TestRun_MetadataApplyMixedFailuresUseFirstCauseConsistently(t *testing.T) {
	resetReportFlags(t)
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)
	metadataDir := filepath.Join(tempDir, "metadata")
	versionDir := filepath.Join(metadataDir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error: %v", err)
	}
	for locale, description := range map[string]string{
		"fr-FR": "French update",
		"ja":    "Japanese update",
	} {
		path := filepath.Join(versionDir, locale+".json")
		if err := os.WriteFile(path, []byte(fmt.Sprintf(`{"description":%q}`, description)), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error: %v", locale, err)
		}
	}

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_MAX_RETRIES", "0")
	resetSelectedProfile(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = metadataApplyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		responses := map[string]struct {
			status int
			body   string
		}{
			"/v1/apps/app-1/appInfos": {
				status: http.StatusOK,
				body:   `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`,
			},
			"/v1/apps/app-1/appStoreVersions": {
				status: http.StatusOK,
				body:   `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`,
			},
			"/v1/appInfos/appinfo-1/appInfoLocalizations": {
				status: http.StatusOK,
				body:   `{"data":[],"links":{"next":""}}`,
			},
			"/v1/appStoreVersions/version-1/appStoreVersionLocalizations": {
				status: http.StatusOK,
				body:   `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Old French"}},{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Old Japanese"}}],"links":{"next":""}}`,
			},
			"/v1/appStoreVersionLocalizations/loc-fr": {
				status: http.StatusInternalServerError,
				body:   `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"first failure"}]}`,
			},
			"/v1/appStoreVersionLocalizations/loc-ja": {
				status: http.StatusNotFound,
				body:   `{"errors":[{"status":"404","code":"NOT_FOUND","detail":"later failure"}]}`,
			},
		}
		response, ok := responses[req.URL.Path]
		if !ok {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return &http.Response{
			StatusCode: response.status,
			Body:       io.NopCloser(strings.NewReader(response.body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotExitCode int
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExitCode = exitCode
		gotContext = eventContext
	}

	var exitCode int
	stdout, stderr := captureCommandOutput(t, func() {
		exitCode = Run([]string{
			"metadata", "apply",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", metadataDir,
			"--output", "json",
		}, "1.0.0")
	})
	if exitCode != ExitHTTPInternalServer {
		t.Fatalf("Run() exit code = %d, want %d; stdout=%s", exitCode, ExitHTTPInternalServer, stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result struct {
		Failed  int `json:"failed"`
		Actions []struct {
			Status string `json:"status"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if result.Failed != 2 || len(result.Actions) != 2 || result.Actions[0].Status != "failed" || result.Actions[1].Status != "failed" {
		t.Fatalf("unexpected partial result: %+v", result)
	}
	if gotExitCode != ExitHTTPInternalServer || gotContext.HTTPStatus != http.StatusInternalServerError ||
		gotContext.OutcomeKind != telemetry.OutcomeAPIServerError {
		t.Fatalf("inconsistent event classification: exit=%d context=%+v", gotExitCode, gotContext)
	}
}

type metadataApplyRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn metadataApplyRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRun_MissingRequiredFlagsEmitContext(t *testing.T) {
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	tests := []struct {
		args       []string
		wantStderr string
	}{
		{args: []string{"reviews"}, wantStderr: "--app is required"},
		{args: []string{"reviews", "list"}, wantStderr: "--app is required"},
		{args: []string{"screenshots", "plan", "--version", "1.0"}, wantStderr: "--app is required"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			resetReportFlags(t)
			t.Setenv("ASC_APP_ID", "")
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
				gotContext = eventContext
			}

			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("expected %q, got %q", test.wantStderr, stderr)
			}
			if gotContext.ErrorKind != telemetry.ErrorKindMissingRequired || gotContext.FailureStage != telemetry.FailureStageValidation {
				t.Fatalf("unexpected telemetry context: %+v", gotContext)
			}
			if gotContext.FailureParameter != "--app" || gotContext.OutcomeKind != telemetry.OutcomeUsageError {
				t.Fatalf("unexpected missing-parameter telemetry context: %+v", gotContext)
			}
		})
	}
}

func TestRun_XcodeExportManagedPassthroughEmitsUsageContext(t *testing.T) {
	resetReportFlags(t)
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var gotExitCode int
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExitCode = exitCode
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"xcode", "export",
			"--archive-path", "Demo.xcarchive",
			"--ipa-path", "Demo.ipa",
			"--xcodebuild-flag=-exportPath=/tmp/elsewhere",
		}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	wantError := `--xcodebuild-flag cannot override asc-managed argument "-exportPath"`
	firstLine, _, _ := strings.Cut(stderr, "\n")
	if firstLine != "Error: "+wantError {
		t.Fatalf("stderr first line = %q, want %q", firstLine, "Error: "+wantError)
	}
	if gotExitCode != ExitUsage || gotContext.FailureStage != telemetry.FailureStageValidation ||
		gotContext.OutcomeKind != telemetry.OutcomeUsageError || gotContext.FailureParameter != "--xcodebuild-flag" {
		t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
	}
}

func TestRun_PublishManagedExportPassthroughEmitsUsageContext(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{
			name: "testflight",
			args: []string{
				"publish", "testflight",
				"--app", "app-1",
				"--workspace", "Demo.xcworkspace",
				"--scheme", "Demo",
				"--version", "1.2.3",
				"--group", "External",
				"--export-xcodebuild-flag=-exportPath=/tmp/elsewhere",
			},
		},
		{
			name: "appstore",
			args: []string{
				"publish", "appstore",
				"--app", "app-1",
				"--workspace", "Demo.xcworkspace",
				"--scheme", "Demo",
				"--version", "1.2.3",
				"--export-xcodebuild-flag=-exportPath=/tmp/elsewhere",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				gotExitCode = exitCode
				gotContext = eventContext
			}

			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			wantError := `--export-xcodebuild-flag cannot override asc-managed argument "-exportPath"`
			firstLine, _, _ := strings.Cut(stderr, "\n")
			if firstLine != "Error: "+wantError {
				t.Fatalf("stderr first line = %q, want %q", firstLine, "Error: "+wantError)
			}
			if gotExitCode != ExitUsage || gotContext.FailureStage != telemetry.FailureStageValidation ||
				gotContext.OutcomeKind != telemetry.OutcomeUsageError || gotContext.FailureParameter != "--export-xcodebuild-flag" {
				t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
			}
		})
	}
}

func TestRun_UnknownCommandsReturnConciseRecovery(t *testing.T) {
	resetReportFlags(t)

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "root typo",
			args: []string{"builts"},
			wantStderr: "Error: unknown command `asc builts`\n" +
				"Try:\n" +
				"  asc builds\n" +
				"For help:\n" +
				"  asc --help\n",
		},
		{
			name: "nested typo",
			args: []string{"builds", "lsit"},
			wantStderr: "Error: unknown command `asc builds lsit`\n" +
				"Try:\n" +
				"  asc builds list\n" +
				"For help:\n" +
				"  asc builds --help\n",
		},
		{
			name: "deep nested typo",
			args: []string{"xcode-cloud", "workflows", "lsit", "--app", "APP_ID"},
			wantStderr: "Error: unknown command `asc xcode-cloud workflows lsit`\n" +
				"Try:\n" +
				"  asc xcode-cloud workflows list\n" +
				"For help:\n" +
				"  asc xcode-cloud workflows --help\n",
		},
		{
			name: "no close match",
			args: []string{"builds", "qqqqq"},
			wantStderr: "Error: unknown command `asc builds qqqqq`\n" +
				"For help:\n" +
				"  asc builds --help\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			for _, fullHelpSection := range []string{"DESCRIPTION", "USAGE", "SUBCOMMANDS"} {
				if strings.Contains(stderr, fullHelpSection) {
					t.Fatalf("stderr contains full help section %q: %q", fullHelpSection, stderr)
				}
			}
		})
	}
}

func TestRun_ConciseUnknownCommandPreservesTelemetryClassification(t *testing.T) {
	resetReportFlags(t)
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var calls int
	var commandName string
	var exitCode int
	var eventContext telemetry.EventContext
	emitTelemetry = func(command, _ string, _ time.Duration, code int, context telemetry.EventContext) {
		calls++
		commandName = command
		exitCode = code
		eventContext = context
	}

	_, _ = captureCommandOutput(t, func() {
		if code := Run([]string{"builds", "lsit"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if calls != 1 || commandName != "asc builds" || exitCode != ExitUsage {
		t.Fatalf("telemetry calls=%d command=%q exit=%d", calls, commandName, exitCode)
	}
	if eventContext.InvocationShape != telemetry.InvocationShapeUnknownChild ||
		eventContext.ErrorKind != telemetry.ErrorKindOther ||
		eventContext.FailureStage != telemetry.FailureStageValidation ||
		eventContext.OutcomeKind != telemetry.OutcomeUsageError ||
		eventContext.FailureParameter != "" {
		t.Fatalf("unexpected telemetry context: %+v", eventContext)
	}
}

func TestRun_UnknownCommandSuggestionsAreBoundedAndTerminalSafe(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"app"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	tryBlock, _, found := strings.Cut(strings.TrimPrefix(stderr, "Error: unknown command `asc app`\nTry:\n"), "For help:\n")
	if !found || strings.Count(strings.TrimSpace(tryBlock), "\n") != 1 {
		t.Fatalf("suggestion count is not 2; stderr=%q", stderr)
	}

	_, hostileStderr := captureCommandOutput(t, func() {
		if code := Run([]string{"bad\x1b[31m\r\ncommand"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if strings.ContainsAny(hostileStderr, "\x1b\r") {
		t.Fatalf("stderr contains terminal control characters: %q", hostileStderr)
	}
	firstLine, _, _ := strings.Cut(hostileStderr, "\n")
	if firstLine != "Error: unknown command `asc bad[31m  command`" {
		t.Fatalf("unknown command line = %q", firstLine)
	}
}

func TestRun_UnknownCommandRanksClosestPrefixBeforeSuggestionLimit(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"buil"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Try:\n  asc builds\n") {
		t.Fatalf("closest prefix was truncated from suggestions: %q", stderr)
	}
}

func TestRun_UnknownFlagReturnsConciseRecovery(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"builds", "list", "--ap", "SECRET_VALUE"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := "Error: unknown flag `--ap` for `asc builds list`\n" +
		"Try:\n" +
		"  --app\n" +
		"For help:\n" +
		"  asc builds list --help\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if strings.Contains(stderr, "SECRET_VALUE") {
		t.Fatalf("stderr leaked a following argument: %q", stderr)
	}
}

func TestRun_CommonWrongCommandPathsRecoverInOneStep(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")

	tests := []struct {
		name        string
		args        []string
		wantStderr  string
		wantCommand string
	}{
		{
			name:        "version info",
			args:        []string{"--profile", "Team Profile", "versions", "info", "--version-id", "VERSION_ID", "--include-build"},
			wantStderr:  "Error: unknown command `asc versions info`\nTry:\n  asc --profile 'Team Profile' versions view --version-id VERSION_ID --include-build\nFor help:\n  asc versions --help\n",
			wantCommand: "asc versions",
		},
		{
			name:        "joined review submissions",
			args:        []string{"reviewsubmissions", "list", "--app", "APP_ID"},
			wantStderr:  "Error: unknown command `asc reviewsubmissions list`\nTry:\n  asc review submissions list --app APP_ID\nFor help:\n  asc --help\n",
			wantCommand: "asc",
		},
		{
			name:        "groups builds list",
			args:        []string{"testflight", "groups", "builds", "list", "--build-id", "BUILD_ID"},
			wantStderr:  "Error: unknown command `asc testflight groups builds list`\nTry:\n  asc testflight groups list --build-id BUILD_ID\nFor help:\n  asc testflight groups --help\n",
			wantCommand: "asc testflight groups",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

			var telemetryCalls int
			var gotCommand string
			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(command, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				telemetryCalls++
				gotCommand = command
				gotExitCode = exitCode
				gotContext = eventContext
			}

			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if telemetryCalls != 1 || gotCommand != test.wantCommand || gotExitCode != ExitUsage {
				t.Fatalf("unexpected telemetry call: calls=%d command=%q exit=%d", telemetryCalls, gotCommand, gotExitCode)
			}
			if gotContext.InvocationShape != telemetry.InvocationShapeUnknownChild ||
				gotContext.ErrorKind != telemetry.ErrorKindOther ||
				gotContext.FailureStage != telemetry.FailureStageValidation ||
				gotContext.OutcomeKind != telemetry.OutcomeUsageError ||
				gotContext.FailureParameter != "" {
				t.Fatalf("unexpected telemetry context: %+v", gotContext)
			}
		})
	}
}

func TestRun_CommonWrongCommandPathDoesNotCopyUnsupportedSuffix(t *testing.T) {
	tests := [][]string{
		{"versions", "info", "--version-id", "VERSION_ID", "localizations"},
		{"versions", "info", "--version-id", "--include-build"},
		{"versions", "info", "--version-id="},
	}
	want := "Error: unknown command `asc versions info`\n" +
		"For help:\n" +
		"  asc versions --help\n"

	for _, args := range tests {
		resetReportFlags(t)
		stdout, stderr := captureCommandOutput(t, func() {
			if code := Run(args, "1.0.0"); code != ExitUsage {
				t.Fatalf("Run(%q) exit code = %d, want %d", args, code, ExitUsage)
			}
		})

		if stdout != "" {
			t.Fatalf("Run(%q) stdout = %q, want empty", args, stdout)
		}
		if stderr != want {
			t.Fatalf("Run(%q) stderr = %q, want %q", args, stderr, want)
		}
	}
}

func TestRun_CommonWrongCommandPathRecoveryDoesNotInterceptCanonicalHelp(t *testing.T) {
	resetReportFlags(t)

	tests := [][]string{
		{"versions", "view", "--help"},
		{"review", "submissions", "list", "--help"},
		{"review", "submissions-list", "--help"},
		{"testflight", "groups", "list", "--help"},
	}
	for _, args := range tests {
		stdout, stderr := captureCommandOutput(t, func() {
			if code := Run(args, "1.0.0"); code != ExitSuccess {
				t.Fatalf("Run(%q) exit code = %d, want %d", args, code, ExitSuccess)
			}
		})
		if stdout != "" {
			t.Fatalf("Run(%q) stdout = %q, want empty", args, stdout)
		}
		if strings.Contains(stderr, "Try:") {
			t.Fatalf("Run(%q) was intercepted by recovery: %q", args, stderr)
		}
		if !strings.Contains(stderr, "USAGE") {
			t.Fatalf("Run(%q) stderr = %q, want command help", args, stderr)
		}
	}
}

func TestRun_CommonWrongCommandPathWritesJUnitReport(t *testing.T) {
	resetReportFlags(t)
	reportPath := filepath.Join(t.TempDir(), "junit.xml")

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"--report", "junit",
			"--report-file", reportPath,
			"versions", "info", "--version-id", "VERSION_ID",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Try:\n") || !strings.Contains(stderr, "For help:\n  asc versions --help\n") {
		t.Fatalf("stderr = %q, want semantic recovery and help footer", stderr)
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	var suite struct {
		Failures  int `xml:"failures,attr"`
		TestCases []struct {
			Name    string `xml:"name,attr"`
			Failure struct {
				Message string `xml:"message,attr"`
			} `xml:"failure"`
		} `xml:"testcase"`
	}
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}
	if suite.Failures != 1 || len(suite.TestCases) != 1 || suite.TestCases[0].Name != "asc versions" {
		t.Fatalf("unexpected JUnit payload: %+v", suite)
	}
	if got, want := suite.TestCases[0].Failure.Message, "unknown command `asc versions info`"; got != want {
		t.Fatalf("failure message = %q, want %q", got, want)
	}
}

func TestRun_CommonWrongCommandPathReportWriteFailureReturnsExitError(t *testing.T) {
	resetReportFlags(t)
	reportPath := filepath.Join(t.TempDir(), "junit.xml")
	if err := os.WriteFile(reportPath, []byte("existing"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var calls int
	var gotCommand string
	var gotExit int
	var gotContext telemetry.EventContext
	emitTelemetry = func(command, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		calls++
		gotCommand = command
		gotExit = exitCode
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"--report", "junit",
			"--report-file", reportPath,
			"reviewsubmissions", "list", "--app", "APP_ID",
		}, "1.0.0"); code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.HasPrefix(stderr, "Error: unknown command `asc reviewsubmissions list`\nTry:\n") ||
		!strings.Contains(stderr, "For help:\n  asc --help\n") ||
		!strings.Contains(stderr, "Error: failed to write JUnit report:") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if calls != 1 || gotCommand != "asc" || gotExit != ExitError ||
		gotContext.FailureStage != telemetry.FailureStageExecution ||
		gotContext.ErrorKind != telemetry.ErrorKindOther ||
		gotContext.OutcomeKind != telemetry.OutcomeInternalError {
		t.Fatalf("unexpected telemetry: calls=%d command=%q exit=%d context=%+v", calls, gotCommand, gotExit, gotContext)
	}
}

func TestRun_UnknownHybridSubcommandReturnsUsageBeforeAuth(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")

	tests := []struct {
		name string
		args []string
	}{
		{name: "reviews", args: []string{"reviews", "lsti", "--app", "APP_ID"}},
		{name: "xcode cloud workflows", args: []string{"xcode-cloud", "workflows", "lsti", "--app", "APP_ID"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "For help:\n") {
				t.Fatalf("expected concise help pointer, got %q", stderr)
			}
			if strings.Contains(stderr, "SUBCOMMANDS") {
				t.Fatalf("did not expect full command help, got %q", stderr)
			}
			if strings.Contains(stderr, "missing authentication") {
				t.Fatalf("expected unknown child validation before auth resolution, got %q", stderr)
			}
		})
	}
}

func TestRun_RemovedCommandsPreserveMigrationGuidance(t *testing.T) {
	resetReportFlags(t)

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "apps create", args: []string{"apps", "create"}, wantStderr: "Use `asc web apps create` instead."},
		{name: "submit create", args: []string{"submit", "create"}, wantStderr: "Use `asc review submit`"},
		{name: "submit preflight", args: []string{"submit", "preflight"}, wantStderr: "Use `asc validate` instead."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("expected migration guidance %q, got %q", test.wantStderr, stderr)
			}
		})
	}
}

func TestRun_SnitchPreservesPositionalDescription(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"snitch", "--dry-run", "status command needs bundle ID support"}, "1.0.0"); code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if !strings.Contains(stderr, "status command needs bundle ID support") {
		t.Fatalf("expected snitch preview to preserve description, got %q", stderr)
	}
}

func TestRun_UnknownFlagSuggestsRealFlagAndEmitsContext(t *testing.T) {
	resetReportFlags(t)
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"versions", "attach-build", "--version-id", "VERSION_ID", "--buid-id", "BUILD_ID"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: unknown flag `--buid-id` for `asc versions attach-build`") {
		t.Fatalf("expected normalized unknown flag diagnostic, got %q", stderr)
	}
	if strings.Contains(stderr, "flag provided but not defined") {
		t.Fatalf("did not expect raw Go flag diagnostic, got %q", stderr)
	}
	if !strings.Contains(stderr, "Try:\n  --build-id\n") {
		t.Fatalf("expected --build-id suggestion, got %q", stderr)
	}
	if gotContext.InvocationShape != telemetry.InvocationShapeLeaf ||
		gotContext.ErrorKind != telemetry.ErrorKindUnknownFlag ||
		gotContext.FailureStage != telemetry.FailureStageParse ||
		gotContext.OutcomeKind != telemetry.OutcomeUsageError {
		t.Fatalf("unexpected telemetry context: %+v", gotContext)
	}
	if gotContext.FailureParameter != "--buid-id" {
		t.Fatalf("FailureParameter = %q, want --buid-id", gotContext.FailureParameter)
	}
}

func TestRun_UnknownIdentifierFlagsSuggestCommandSpecificFlags(t *testing.T) {
	resetReportFlags(t)

	tests := []struct {
		name           string
		args           []string
		wantSuggestion string
	}{
		{
			name:           "generic version ID",
			args:           []string{"versions", "app-clip-default-experience", "view", "--id", "VERSION_ID"},
			wantSuggestion: "Try:\n  --version-id\n",
		},
		{
			name:           "qualified subscription ID",
			args:           []string{"subscriptions", "groups", "view", "--subscription-id", "SUBSCRIPTION_ID"},
			wantSuggestion: "Try:\n  --id\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantSuggestion) {
				t.Fatalf("expected %q, got %q", test.wantSuggestion, stderr)
			}
		})
	}
}

func TestRun_UnknownFlagDoesNotSuggestHiddenCompatibilityFlag(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"iap", "setup", "--nam", "value"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: unknown flag `--nam` for `asc iap setup`") {
		t.Fatalf("expected normalized unknown flag diagnostic, got %q", stderr)
	}
	for _, line := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(line, "  --") && strings.Contains(line, "--name") {
			t.Fatalf("hidden compatibility flag must not be suggested, got %q", stderr)
		}
	}
}

func TestRun_UnknownFlagDoesNotSuggestDeprecatedFlag(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"testflight", "beta-details", "update", "--external-testin", "true",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if strings.Contains(stderr, "Try:\n  --external-testing\n") {
		t.Fatalf("deprecated flag must not be suggested, got %q", stderr)
	}
}

func TestRun_UnknownFlagDoesNotSuggestMixedCaseDeprecatedFlag(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"web", "apps", "create", "--two-factor-cod", "PRIVATE_VALUE",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	want := "Error: unknown flag `--two-factor-cod` for `asc web apps create`\n" +
		"Try:\n" +
		"  --two-factor-code-command\n" +
		"For help:\n" +
		"  asc web apps create --help\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if strings.Contains(stderr, "PRIVATE_VALUE") {
		t.Fatalf("stderr leaked a following argument: %q", stderr)
	}
}

func TestRun_UnknownFlagDoesNotSuggestDeprecatedAlias(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"apps", "public", "view", "--idd", "APP_ID"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if strings.Contains(stderr, "Try:\n  --id\n") {
		t.Fatalf("deprecated alias must not be suggested, got %q", stderr)
	}
}

func TestRun_UnknownFlagDoesNotSuggestSuffixDeprecatedAlias(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"iap", "localizations", "list", "--idd", "IAP_ID"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Try:\n  --iap-id\n") {
		t.Fatalf("expected canonical --iap-id suggestion, got %q", stderr)
	}
	if strings.Contains(stderr, "Try:\n  --id\n") {
		t.Fatalf("deprecated --id alias must not be suggested, got %q", stderr)
	}
}

func TestRun_UnknownCommandDoesNotSuggestDeprecatedSurface(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"iap", "imagez"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if strings.Contains(stderr, "asc iap images") {
		t.Fatalf("deprecated command must not be suggested, got %q", stderr)
	}
}

func TestDeprecatedHelpDetectionUsesLifecycleMarkers(t *testing.T) {
	for _, help := range []string{
		"DEPRECATED: use --app",
		"Deprecated alias for --app",
		"[deprecated, ignored] Previously used this flag",
		"In-app purchase ID, product ID, or exact current name (deprecated)",
	} {
		if !isDeprecatedFlagHelp(help) {
			t.Fatalf("isDeprecatedFlagHelp(%q) = false, want true", help)
		}
	}
	for _, help := range []string{
		"Sparse fields: kidsAgeBand (deprecated by Apple; prefer age-rating data)",
		"not-deprecated compatibility surface",
	} {
		if isDeprecatedFlagHelp(help) {
			t.Fatalf("isDeprecatedFlagHelp(%q) = true, want false", help)
		}
	}

	for _, help := range []string{
		"DEPRECATED: use `asc iap versions images`.",
		"Manage deprecated product-scoped images.",
		"Manage subscription availability (deprecated by Apple).",
	} {
		if !isDeprecatedCommandHelp(help) {
			t.Fatalf("isDeprecatedCommandHelp(%q) = false, want true", help)
		}
	}
	for _, help := range []string{
		"Submit a version that replaces a deprecated resource.",
		"Manage current images.",
		"not-deprecated compatibility surface",
	} {
		if isDeprecatedCommandHelp(help) {
			t.Fatalf("isDeprecatedCommandHelp(%q) = true, want false", help)
		}
	}
}

func TestRun_DeprecationMentionsRemainSuggestionCandidates(t *testing.T) {
	resetReportFlags(t)

	_, commandStderr := captureCommandOutput(t, func() {
		if code := Run([]string{"iap", "versions", "subimt"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if !strings.Contains(commandStderr, "Try:\n  asc iap versions submit\n") {
		t.Fatalf("stable command with deprecation context must remain suggestible, got %q", commandStderr)
	}

	_, flagStderr := captureCommandOutput(t, func() {
		if code := Run([]string{"apps", "list", "--app-info-field", "kidsAgeBand"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if !strings.Contains(flagStderr, "Try:\n  --app-info-fields\n") {
		t.Fatalf("stable flag with deprecation context must remain suggestible, got %q", flagStderr)
	}
}

func TestRun_UnknownGroupFlagIsNotClassifiedAsBareGroup(t *testing.T) {
	resetReportFlags(t)
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		gotContext = eventContext
	}

	captureCommandOutput(t, func() {
		if code := Run([]string{"builds", "--definitely-invalid"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if gotContext.InvocationShape != telemetry.InvocationShapeGroupWithFlags ||
		gotContext.ErrorKind != telemetry.ErrorKindUnknownFlag ||
		gotContext.FailureStage != telemetry.FailureStageParse {
		t.Fatalf("unexpected telemetry context: %+v", gotContext)
	}
}

func TestRun_AlternativeDistributionHelpIncludesEUAddendumAgentGuidance(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr, exitCode := runHelpSubprocess(
		t,
		t.TempDir(),
		"alternative-distribution", "--help",
	)

	if exitCode != ExitSuccess {
		t.Fatalf("help exit code = %d, want %d; stdout=%q stderr=%q", exitCode, ExitSuccess, stdout, stderr)
	}
	combined := stdout + stderr
	for _, expected := range []string{
		"Agent guidance:",
		"Alternative Distribution Addendum for EU Apps",
		"https://appstoreconnect.apple.com/agreements/#/",
	} {
		if !strings.Contains(combined, expected) {
			t.Fatalf("expected help output to contain %q, got stdout=%q stderr=%q", expected, stdout, stderr)
		}
	}
	if strings.Contains(combined, "\n  agreements  ") {
		t.Fatalf("did not expect agreements subcommand in help, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestRun_RemovedTopLevelCommandsReturnUnknown(t *testing.T) {
	resetReportFlags(t)

	tests := []struct {
		name string
		arg  string
	}{
		{name: "assets removed", arg: "assets"},
		{name: "shots removed", arg: "shots"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, stderr := captureCommandOutput(t, func() {
				code := Run([]string{test.arg}, "1.0.0")
				if code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})
			if !strings.Contains(stderr, "Error: unknown command `asc "+test.arg+"`") {
				t.Fatalf("expected unknown command in stderr, got %q", stderr)
			}
		})
	}
}

func TestRun_WebBundleIDSyncAppClipInvalidSettingsJSONReturnsUsage(t *testing.T) {
	resetReportFlags(t)

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"web", "bundle-ids", "capabilities", "sync-app-clip",
			"--bundle-id", "clip-1",
			"--parent-bundle-id", "parent-1",
			"--capability", "PUSH_NOTIFICATIONS",
			"--settings-json", "null",
		}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "--settings-json must be a JSON array") {
		t.Fatalf("expected settings-json usage error, got %q", stderr)
	}
	if strings.Contains(stderr, "--apple-id is required") {
		t.Fatalf("expected settings-json validation before auth resolution, got %q", stderr)
	}
}

func TestRun_NoArgsShowsHelpReturnsSuccess(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		code := Run([]string{}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if !strings.Contains(stdout, "USAGE") || !strings.Contains(stdout, "GETTING STARTED COMMANDS") {
		t.Fatalf("expected root help in stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestRun_DisablesAutomaticSkillsUpdateCheckForSubcommand(t *testing.T) {
	resetReportFlags(t)

	origCheck := maybeScheduleSkillsUpdateCheck
	t.Cleanup(func() { maybeScheduleSkillsUpdateCheck = origCheck })

	called := make(chan struct{}, 1)
	maybeScheduleSkillsUpdateCheck = func() {
		select {
		case called <- struct{}{}:
		default:
		}
	}

	_, _ = captureCommandOutput(t, func() {
		code := Run([]string{"completion", "--shell", "bash"}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	select {
	case <-called:
		t.Fatal("automatic skills update check unexpectedly ran")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRun_SkipsSkillsUpdateCheckForRootInvocation(t *testing.T) {
	resetReportFlags(t)

	origCheck := maybeScheduleSkillsUpdateCheck
	t.Cleanup(func() { maybeScheduleSkillsUpdateCheck = origCheck })

	called := false
	maybeScheduleSkillsUpdateCheck = func() {
		called = true
	}

	_, _ = captureCommandOutput(t, func() {
		code := Run([]string{}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if called {
		t.Fatal("expected skills update check to be skipped for root invocation")
	}
}

func TestRun_SkipsSkillsUpdateCheckForHelpAndVersionInvocations(t *testing.T) {
	origCheck := maybeScheduleSkillsUpdateCheck
	t.Cleanup(func() { maybeScheduleSkillsUpdateCheck = origCheck })

	tests := []struct {
		name string
		args []string
	}{
		{name: "root help", args: []string{"--help"}},
		{name: "subcommand help", args: []string{"completion", "--help"}},
		{name: "version flag", args: []string{"--version"}},
		{name: "version command", args: []string{"version"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetReportFlags(t)
			called := false
			maybeScheduleSkillsUpdateCheck = func() {
				called = true
			}

			_, _ = captureCommandOutput(t, func() {
				if code := Run(tt.args, "1.0.0"); code != ExitSuccess {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
				}
			})
			if called {
				t.Fatalf("skills update check scheduled for %v", tt.args)
			}
		})
	}
}

func TestShouldCancelRunContextAfterError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error does not cancel",
			err:  nil,
			want: false,
		},
		{
			name: "generic error does not cancel",
			err:  errors.New("boom"),
			want: false,
		},
		{
			name: "context canceled cancels",
			err:  context.Canceled,
			want: true,
		},
		{
			name: "wrapped context canceled cancels",
			err:  fmt.Errorf("prompt interrupted: %w", context.Canceled),
			want: true,
		},
		{
			name: "deadline exceeded cancels",
			err:  context.DeadlineExceeded,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldCancelRunContextAfterError(tt.err); got != tt.want {
				t.Fatalf("shouldCancelRunContextAfterError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestShouldRunSkillsUpdateCheck(t *testing.T) {
	for _, commandName := range []string{"asc", "asc completion", "asc install-skills", "asc version"} {
		t.Run(commandName, func(t *testing.T) {
			if shouldRunSkillsUpdateCheck(commandName, context.Background(), nil) {
				t.Fatal("automatic external skills check must remain disabled")
			}
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if shouldRunSkillsUpdateCheck("asc web auth login", ctx, context.Canceled) {
		t.Fatal("automatic external skills check must remain disabled")
	}
	if shouldRunSkillsUpdateCheck("asc web auth login", context.Background(), fmt.Errorf("boom")) {
		t.Fatal("automatic external skills check must remain disabled")
	}
}

func TestRun_HelpSkipsAuthResolution(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()

	tests := []struct {
		name          string
		args          []string
		wantHelpText  string
		avoidMessages []string
	}{
		{
			name:          "apps list help",
			args:          []string{"apps", "list", "--help"},
			wantHelpText:  "List apps from App Store Connect.",
			avoidMessages: []string{"missing authentication", "keychain access denied"},
		},
		{
			name:          "auth token help",
			args:          []string{"auth", "token", "--help"},
			wantHelpText:  "Print a signed JWT for direct App Store Connect API calls.",
			avoidMessages: []string{"missing authentication", "--confirm is required", "keychain access denied"},
		},
		{
			name:          "auth issuer-id help",
			args:          []string{"auth", "issuer-id", "--help"},
			wantHelpText:  "Print the active App Store Connect issuer ID.",
			avoidMessages: []string{"missing authentication", "keychain access denied"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, exitCode := runHelpSubprocess(t, tempDir, test.args...)
			combined := stdout + stderr

			if exitCode != 0 {
				t.Fatalf("help exit code = %d, want 0; stdout=%q stderr=%q", exitCode, stdout, stderr)
			}
			if !strings.Contains(combined, test.wantHelpText) {
				t.Fatalf("expected help text %q in output, got stdout=%q stderr=%q", test.wantHelpText, stdout, stderr)
			}
			for _, avoid := range test.avoidMessages {
				if strings.Contains(combined, avoid) {
					t.Fatalf("expected help path to avoid %q, got stdout=%q stderr=%q", avoid, stdout, stderr)
				}
			}
		})
	}
}

func TestRun_HelpSkipsTelemetry(t *testing.T) {
	resetReportFlags(t)
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var telemetryCalls int
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, _ telemetry.EventContext) {
		telemetryCalls++
	}

	captureCommandOutput(t, func() {
		if code := Run([]string{"builds", "--help"}, "1.0.0"); code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})
	if telemetryCalls != 0 {
		t.Fatalf("telemetry calls = %d, want 0 for help", telemetryCalls)
	}
}

func TestMergeEnvOverridesReplacesExistingKeys(t *testing.T) {
	env := mergeEnvOverrides(
		[]string{
			"ASC_BYPASS_KEYCHAIN=1",
			"ASC_KEY_ID=PARENT",
			"UNCHANGED=keep",
		},
		map[string]string{
			"ASC_BYPASS_KEYCHAIN":  "",
			"ASC_KEY_ID":           "",
			"GO_WANT_HELP_PROCESS": "1",
		},
	)

	values := map[string]string{}
	counts := map[string]int{}
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		value := ""
		if len(parts) == 2 {
			value = parts[1]
		}
		values[key] = value
		counts[key]++
	}

	if counts["ASC_BYPASS_KEYCHAIN"] != 1 || values["ASC_BYPASS_KEYCHAIN"] != "" {
		t.Fatalf("expected ASC_BYPASS_KEYCHAIN override, got counts=%v values=%v", counts, values)
	}
	if counts["ASC_KEY_ID"] != 1 || values["ASC_KEY_ID"] != "" {
		t.Fatalf("expected ASC_KEY_ID override, got counts=%v values=%v", counts, values)
	}
	if counts["GO_WANT_HELP_PROCESS"] != 1 || values["GO_WANT_HELP_PROCESS"] != "1" {
		t.Fatalf("expected GO_WANT_HELP_PROCESS override, got counts=%v values=%v", counts, values)
	}
	if values["UNCHANGED"] != "keep" {
		t.Fatalf("expected unrelated env to be preserved, got values=%v", values)
	}
}

func TestHasPositionalArgs_EndOfFlagsSeparator(t *testing.T) {
	root := RootCommand("1.0.0")

	if got := hasPositionalArgs(root.FlagSet, []string{"--", "--version"}); !got {
		t.Fatalf("hasPositionalArgs() = %v, want true", got)
	}
}

func TestRootCommand_UnknownCommandPrintsHelpError(t *testing.T) {
	root := RootCommand("1.2.3")
	if err := root.Parse([]string{"unknown-subcommand"}); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	_, stderr := captureCommandOutput(t, func() {
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("Run() error = %v, want %v", err, flag.ErrHelp)
		}
	})

	if !strings.Contains(stderr, "Unknown command: unknown-subcommand") {
		t.Fatalf("expected unknown command output, got %q", stderr)
	}
}

func TestRootCommand_UsageGroupsSubcommands(t *testing.T) {
	root := RootCommand("1.2.3")
	usage := root.UsageFunc(root)

	if strings.Contains(usage, "SUBCOMMANDS") {
		t.Fatalf("usage should not use a single SUBCOMMANDS section, got %q", usage)
	}

	if !strings.Contains(usage, "GETTING STARTED COMMANDS") {
		t.Fatalf("expected GETTING STARTED group header, got %q", usage)
	}

	if !strings.Contains(usage, "  auth:") || !strings.Contains(usage, "  doctor:") || !strings.Contains(usage, "  install-skills:") || !strings.Contains(usage, "  init:") {
		t.Fatalf("expected grouped getting started commands with gh-style spacing, got %q", usage)
	}

	if !strings.Contains(usage, "ANALYTICS & FINANCE COMMANDS") {
		t.Fatalf("expected analytics group header, got %q", usage)
	}

	if !strings.Contains(usage, "  analytics:") || !strings.Contains(usage, "  finance:") {
		t.Fatalf("expected grouped analytics/finance commands, got %q", usage)
	}

	if strings.Contains(usage, "  offer-codes:") || strings.Contains(usage, "  win-back-offers:") || strings.Contains(usage, "  promoted-purchases:") {
		t.Fatalf("expected deprecated monetization shims to be hidden from primary root usage, got %q", usage)
	}
	if strings.Contains(usage, "  beta-build-localizations:") {
		t.Fatalf("expected beta-build-localizations to remain hidden from primary root usage, got %q", usage)
	}

	if !strings.Contains(usage, "  subscriptions:") {
		t.Fatalf("expected subscriptions command to remain visible in root usage, got %q", usage)
	}

	if !strings.Contains(usage, "  screenshots:") || !strings.Contains(usage, "  video-previews:") {
		t.Fatalf("expected screenshots and video-previews commands in root usage, got %q", usage)
	}

	if strings.Contains(usage, "  assets:") || strings.Contains(usage, "  shots:") {
		t.Fatalf("expected old assets/shots commands to be removed from root usage, got %q", usage)
	}

	releaseIdx := strings.Index(usage, "  release:")
	reviewIdx := strings.Index(usage, "  review:")
	submitIdx := strings.Index(usage, "  submit:")
	publishIdx := strings.Index(usage, "  publish:")
	if releaseIdx == -1 || reviewIdx == -1 || submitIdx == -1 || publishIdx == -1 {
		t.Fatalf("expected release, review, submit, and publish commands in root usage, got %q", usage)
	}
	if releaseIdx > reviewIdx || releaseIdx > submitIdx {
		t.Fatalf("expected release to lead the review and release group, got %q", usage)
	}
	if strings.Contains(usage, "End-to-end publish workflows for TestFlight and App Store.") {
		t.Fatalf("expected publish root help to stop advertising App Store publishing, got %q", usage)
	}
}

func TestRootCommand_ReleaseHelpMentionsCanonicalPathAndStatus(t *testing.T) {
	root := RootCommand("1.2.3")

	var releaseCmd *ffcli.Command
	for _, subcommand := range root.Subcommands {
		if subcommand.Name == "release" {
			releaseCmd = subcommand
			break
		}
	}
	if releaseCmd == nil {
		t.Fatal("expected release subcommand to be registered")
		return
	}

	usage := releaseCmd.UsageFunc(releaseCmd)
	if !strings.Contains(usage, `asc publish appstore --app "APP_ID" --ipa app.ipa --version "VERSION" --submit --confirm`) {
		t.Fatalf("expected release help to point to the canonical publish appstore path, got %q", usage)
	}
	if !strings.Contains(usage, "canonical App Store shipping command") {
		t.Fatalf("expected release help to describe the canonical publish command, got %q", usage)
	}
	if !strings.Contains(usage, `asc status --app "APP_ID"`) {
		t.Fatalf("expected release help to mention status monitoring, got %q", usage)
	}
	if !strings.Contains(usage, `asc validate --app "APP_ID" --version "VERSION"`) {
		t.Fatalf("expected release help to mention canonical readiness validation, got %q", usage)
	}
	if !strings.Contains(usage, `asc submit status --version-id "VERSION_ID"`) {
		t.Fatalf("expected release help to mention submission status lookup, got %q", usage)
	}
	if !strings.Contains(usage, `asc submit cancel --version-id "VERSION_ID" --confirm`) {
		t.Fatalf("expected release help to mention submission cancellation, got %q", usage)
	}
	if strings.Contains(usage, `asc submit create --app "APP_ID" --version "VERSION" --build "BUILD_ID" --confirm`) {
		t.Fatalf("expected release help to stop promoting deprecated submit create guidance, got %q", usage)
	}
	if strings.Contains(usage, `asc submit preflight --app "APP_ID" --version "VERSION" --build "BUILD_ID"`) {
		t.Fatalf("expected release help to stop advertising deprecated submit preflight syntax, got %q", usage)
	}
}

func TestRootCommand_WorkflowHelpMentionsReleaseAndStatusMonitoring(t *testing.T) {
	root := RootCommand("1.2.3")

	var workflowCmd *ffcli.Command
	for _, subcommand := range root.Subcommands {
		if subcommand.Name == "workflow" {
			workflowCmd = subcommand
			break
		}
	}
	if workflowCmd == nil {
		t.Fatal("expected workflow subcommand to be registered")
		return
	}

	usage := workflowCmd.UsageFunc(workflowCmd)
	if !strings.Contains(usage, `asc publish appstore --app $APP_ID --ipa ./build/MyApp.ipa --version $VERSION --submit --confirm`) {
		t.Fatalf("expected workflow help to show the high-level release step, got %q", usage)
	}
	if !strings.Contains(usage, `asc status --app "APP_ID"`) {
		t.Fatalf("expected workflow help to mention post-release status monitoring, got %q", usage)
	}
}

func TestRun_InvalidOutputReturnsUsageBeforeAuth(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"devices", "register",
			"--name", "My Device",
			"--udid", "UDID",
			"--platform", "IOS",
			"--output", "yaml",
		}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "unsupported format: yaml") {
		t.Fatalf("expected output validation error, got %q", stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected output validation before auth resolution, got %q", stderr)
	}
}

func TestRun_InvalidPrettyReturnsUsageBeforeAuth(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"devices", "update",
			"--id", "DEVICE_ID",
			"--status", "ENABLED",
			"--output", "table",
			"--pretty",
		}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "--pretty is only valid with JSON output") {
		t.Fatalf("expected pretty/output validation error, got %q", stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected output validation before auth resolution, got %q", stderr)
	}
}

func TestRun_InvalidParentOutputReturnsUsageBeforeLeafExec(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"reviews",
			"--output", "yaml",
			"respond",
			"--review-id", "REVIEW_ID",
			"--response", "Thanks!",
		}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "unsupported format: yaml") {
		t.Fatalf("expected output validation error, got %q", stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected parent output validation before leaf execution, got %q", stderr)
	}
}

func TestRun_InvalidParentPrettyReturnsUsageBeforeLeafExec(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"reviews",
			"--output", "table",
			"--pretty",
			"respond",
			"--review-id", "REVIEW_ID",
			"--response", "Thanks!",
		}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "--pretty is only valid with JSON output") {
		t.Fatalf("expected pretty/output validation error, got %q", stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected parent pretty validation before leaf execution, got %q", stderr)
	}
}

func TestRun_AuthTokenRequiresConfirm(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)

	cfg := &config.Config{
		DefaultKeyName: "default",
		Keys: []config.Credential{
			{
				Name:           "default",
				KeyID:          "KEY123",
				IssuerID:       "ISS456",
				PrivateKeyPath: keyPath,
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"auth", "token"}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected confirm required error, got %q", stderr)
	}
}

func TestRun_AuthTokenEnvOnlyCredentials(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	stdout, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"auth", "token", "--confirm", "--output", "json"}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Token   string `json:"token"`
		KeyID   string `json:"keyId"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.KeyID != "ENVKEY" {
		t.Fatalf("expected keyId ENVKEY, got %q", payload.KeyID)
	}
	if payload.Profile != "" {
		t.Fatalf("expected empty profile for env credentials, got %q", payload.Profile)
	}
	if parts := strings.Split(payload.Token, "."); len(parts) != 3 {
		t.Fatalf("expected JWT token, got %q", payload.Token)
	}
}

func TestRun_AuthTokenHonorsRootProfileFlag(t *testing.T) {
	resetReportFlags(t)

	configPath := writeAuthTokenProfilesConfig(t)
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	stdout, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"--profile", "second", "auth", "token", "--confirm", "--output", "json"}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		KeyID   string `json:"keyId"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.KeyID != "KEY_B" || payload.Profile != "second" {
		t.Fatalf("expected second profile payload, got %+v", payload)
	}
}

func TestRun_AuthTokenHonorsASCProfileEnv(t *testing.T) {
	resetReportFlags(t)

	configPath := writeAuthTokenProfilesConfig(t)
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "second")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	stdout, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"auth", "token", "--confirm", "--output", "json"}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		KeyID   string `json:"keyId"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.KeyID != "KEY_B" || payload.Profile != "second" {
		t.Fatalf("expected second profile payload, got %+v", payload)
	}
}

func TestRun_AuthTokenAmbiguousProfilesReturnAuthExit(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)

	cfg := &config.Config{
		DefaultKeyName: "",
		Keys: []config.Credential{
			{
				Name:           "first",
				KeyID:          "KEY_A",
				IssuerID:       "ISS_A",
				PrivateKeyPath: keyPath,
			},
			{
				Name:           "second",
				KeyID:          "KEY_B",
				IssuerID:       "ISS_B",
				PrivateKeyPath: keyPath,
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"auth", "token", "--confirm"}, "1.0.0")
		if code != ExitAuth {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitAuth)
		}
	})

	if !strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected missing authentication error, got %q", stderr)
	}
}

func TestRun_AuthTokenRejectsPermissiveKeyFile(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)
	if err := os.Chmod(keyPath, 0o644); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}

	cfg := &config.Config{
		DefaultKeyName: "default",
		Keys: []config.Credential{
			{
				Name:           "default",
				KeyID:          "KEY123",
				IssuerID:       "ISS456",
				PrivateKeyPath: keyPath,
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"auth", "token", "--confirm"}, "1.0.0")
		if code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	if !strings.Contains(stderr, "private key file is too permissive") {
		t.Fatalf("expected permissive key file error, got %q", stderr)
	}
}

func TestWriteJUnitReport(t *testing.T) {
	resetReportFlags(t)

	reportPath := filepath.Join(t.TempDir(), "junit.xml")
	shared.SetReportFile(reportPath)
	t.Cleanup(func() {
		shared.SetReportFile("")
	})

	runErr := errors.New("boom")
	if err := writeJUnitReport("asc builds list", runErr, 2*time.Second); err != nil {
		t.Fatalf("writeJUnitReport() error: %v", err)
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var suite struct {
		XMLName   xml.Name `xml:"testsuite"`
		Failures  int      `xml:"failures,attr"`
		TestCases []struct {
			Name    string `xml:"name,attr"`
			Failure *struct {
				Type string `xml:"type,attr"`
			} `xml:"failure"`
		} `xml:"testcase"`
	}
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}
	if suite.Failures != 1 {
		t.Fatalf("failures = %d, want 1", suite.Failures)
	}
	if len(suite.TestCases) != 1 || suite.TestCases[0].Name != "asc builds list" {
		t.Fatalf("unexpected testcase payload: %+v", suite.TestCases)
	}
	if suite.TestCases[0].Failure == nil || suite.TestCases[0].Failure.Type != "ERROR" {
		t.Fatalf("expected failure type ERROR, got %+v", suite.TestCases[0].Failure)
	}
}

func resetReportFlags(t *testing.T) {
	t.Helper()
	shared.SetReportFormat("")
	shared.SetReportFile("")
}

func resetSelectedProfile(t *testing.T) {
	t.Helper()
	previousProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(previousProfile)
	})
}

func writeAuthTokenProfilesConfig(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)

	cfg := &config.Config{
		DefaultKeyName: "first",
		Keys: []config.Credential{
			{
				Name:           "first",
				KeyID:          "KEY_A",
				IssuerID:       "ISS_A",
				PrivateKeyPath: keyPath,
			},
			{
				Name:           "second",
				KeyID:          "KEY_B",
				IssuerID:       "ISS_B",
				PrivateKeyPath: keyPath,
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}
	return configPath
}

func writeRunTestECDSAPEM(t *testing.T, path string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
}

func captureCommandOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() stdout error: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() stderr error: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutR)
		_ = stdoutR.Close()
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrR)
		_ = stderrR.Close()
		errC <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = stdoutW.Close()
		_ = stderrW.Close()
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	return <-outC, <-errC
}

func runHelpSubprocess(t *testing.T, tempDir string, args ...string) (string, string, int) {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error: %v", err)
	}

	cmdArgs := []string{"-test.run=TestRunHelpHelperProcess", "--"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(exe, cmdArgs...)
	cmd.Env = mergeEnvOverrides(os.Environ(), map[string]string{
		"GO_WANT_HELP_PROCESS": "1",
		"ASC_BYPASS_KEYCHAIN":  "",
		"ASC_CONFIG_PATH":      filepath.Join(tempDir, "missing.json"),
		"ASC_PROFILE":          "",
		"ASC_KEY_ID":           "",
		"ASC_ISSUER_ID":        "",
		"ASC_PRIVATE_KEY_PATH": "",
		"ASC_PRIVATE_KEY":      "",
		"ASC_PRIVATE_KEY_B64":  "",
		"ASC_STRICT_AUTH":      "",
	})

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("help subprocess error: %v", err)
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func mergeEnvOverrides(base []string, overrides map[string]string) []string {
	filtered := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		if _, ok := overrides[key]; ok {
			continue
		}
		filtered = append(filtered, entry)
	}

	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		filtered = append(filtered, key+"="+overrides[key])
	}
	return filtered
}

func TestRunHelpHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELP_PROCESS") != "1" {
		return
	}

	sep := -1
	for i, arg := range os.Args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep == -1 || sep+1 >= len(os.Args) {
		os.Exit(2)
	}

	code := Run(os.Args[sep+1:], "1.0.0")
	os.Exit(code)
}
