package cmd

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/update"
)

func TestRun_VersionFlag(t *testing.T) {
	t.Setenv("ASC_NO_UPDATE", "1")
	resetReportFlags(t)

	stdout, _ := captureCommandOutput(t, func() {
		code := Run([]string{"--version"}, "9.9.9")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if !strings.Contains(stdout, "9.9.9") {
		t.Fatalf("expected version in stdout, got %q", stdout)
	}
}

func TestRun_ReportFlagValidationError(t *testing.T) {
	t.Setenv("ASC_NO_UPDATE", "1")
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

func TestRun_ReportWriteFailureReturnsExitError(t *testing.T) {
	t.Setenv("ASC_NO_UPDATE", "1")
	resetReportFlags(t)

	reportPath := filepath.Join(t.TempDir(), "junit.xml")
	if err := os.WriteFile(reportPath, []byte("existing"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"--no-update",
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
	t.Setenv("ASC_NO_UPDATE", "1")
	resetReportFlags(t)

	code := Run([]string{"unknown-command"}, "1.0.0")
	if code != ExitUsage {
		t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
	}
}

func TestRun_RemovedTopLevelCommandsReturnUnknown(t *testing.T) {
	t.Setenv("ASC_NO_UPDATE", "1")
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
			if !strings.Contains(stderr, "Unknown command: "+test.arg) {
				t.Fatalf("expected unknown command in stderr, got %q", stderr)
			}
		})
	}
}

func TestRun_NoArgsShowsHelpReturnsSuccess(t *testing.T) {
	t.Setenv("ASC_NO_UPDATE", "1")
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

func TestRun_WaitsForAsyncUpdateCheckBeforeExit(t *testing.T) {
	t.Setenv("ASC_NO_UPDATE", "0")
	resetReportFlags(t)
	resetUpdateHooks(t)

	cachedUpdateAvailableFn = func(update.Options) (bool, error) {
		return false, nil
	}

	started := make(chan update.Options, 1)
	release := make(chan struct{})
	checkAndUpdateFn = func(_ context.Context, opts update.Options) (update.Result, error) {
		started <- opts
		<-release
		return update.Result{}, nil
	}

	runDone := make(chan int, 1)
	go func() {
		runDone <- Run([]string{"completion", "--shell", "bash"}, "1.0.0")
	}()

	select {
	case opts := <-started:
		if opts.AutoUpdate {
			t.Fatal("expected async update check to disable auto-update")
		}
		if opts.CheckInterval != updateCheckInterval {
			t.Fatalf("CheckInterval = %s, want %s", opts.CheckInterval, updateCheckInterval)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected async update check goroutine to start")
	}

	select {
	case code := <-runDone:
		t.Fatalf("Run() returned before async update check finished, code=%d", code)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case code := <-runDone:
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run() did not return after async update check finished")
	}
}

func TestRun_CachedUpdateChecksSynchronously(t *testing.T) {
	t.Setenv("ASC_NO_UPDATE", "0")
	resetReportFlags(t)
	resetUpdateHooks(t)

	cachedUpdateAvailableFn = func(update.Options) (bool, error) {
		return true, nil
	}
	checkAndUpdateFn = func(_ context.Context, opts update.Options) (update.Result, error) {
		if !opts.AutoUpdate {
			t.Fatal("expected sync update path to enable auto-update")
		}
		time.Sleep(150 * time.Millisecond)
		return update.Result{}, nil
	}

	start := time.Now()
	code := Run([]string{"completion", "--shell", "bash"}, "1.0.0")
	elapsed := time.Since(start)
	if code != ExitSuccess {
		t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
	}
	if elapsed < 140*time.Millisecond {
		t.Fatalf("Run() should block on synchronous update check, elapsed=%s", elapsed)
	}
}

func TestRun_CachedUpdateCanRestartProcess(t *testing.T) {
	t.Setenv("ASC_NO_UPDATE", "0")
	resetReportFlags(t)
	resetUpdateHooks(t)

	cachedUpdateAvailableFn = func(update.Options) (bool, error) {
		return true, nil
	}
	checkAndUpdateFn = func(_ context.Context, _ update.Options) (update.Result, error) {
		return update.Result{
			Updated:        true,
			ExecutablePath: "/tmp/asc",
		}, nil
	}
	restartFn = func(_ string, _ []string, _ []string) (int, error) {
		return 7, nil
	}

	code := Run([]string{"completion", "--shell", "bash"}, "1.0.0")
	if code != 7 {
		t.Fatalf("Run() exit code = %d, want 7", code)
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

	if !strings.Contains(usage, "  auth:") || !strings.Contains(usage, "  install:") || !strings.Contains(usage, "  init:") {
		t.Fatalf("expected grouped getting started commands with gh-style spacing, got %q", usage)
	}

	if !strings.Contains(usage, "ANALYTICS & FINANCE COMMANDS") {
		t.Fatalf("expected analytics group header, got %q", usage)
	}

	if !strings.Contains(usage, "  analytics:") || !strings.Contains(usage, "  finance:") {
		t.Fatalf("expected grouped analytics/finance commands, got %q", usage)
	}

	if !strings.Contains(usage, "  screenshots:") || !strings.Contains(usage, "  video-previews:") {
		t.Fatalf("expected screenshots and video-previews commands in root usage, got %q", usage)
	}

	if strings.Contains(usage, "  assets:") || strings.Contains(usage, "  shots:") {
		t.Fatalf("expected old assets/shots commands to be removed from root usage, got %q", usage)
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

func TestCmdSharedWrappersAndReportedError(t *testing.T) {
	CleanupTempPrivateKey()
	CleanupTempPrivateKeys()

	if Bold("hi") == "" {
		t.Fatal("Bold() should not return empty string")
	}

	usage := DefaultUsageFunc(&ffcli.Command{
		Name:      "asc",
		ShortHelp: "test help",
		FlagSet:   flag.NewFlagSet("asc", flag.ContinueOnError),
	})
	if usage == "" {
		t.Fatal("DefaultUsageFunc() should return non-empty usage string")
	}

	inner := errors.New("already printed")
	wrapped := NewReportedError(inner)
	if !errors.Is(wrapped, inner) {
		t.Fatalf("NewReportedError() should wrap original error")
	}
}

func resetReportFlags(t *testing.T) {
	t.Helper()
	shared.SetReportFormat("")
	shared.SetReportFile("")
}

func resetUpdateHooks(t *testing.T) {
	t.Helper()

	originalCachedUpdateAvailableFn := cachedUpdateAvailableFn
	originalCheckAndUpdateFn := checkAndUpdateFn
	originalRestartFn := restartFn

	t.Cleanup(func() {
		cachedUpdateAvailableFn = originalCachedUpdateAvailableFn
		checkAndUpdateFn = originalCheckAndUpdateFn
		restartFn = originalRestartFn
	})
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
