package shared

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestDeprecatedCommandPreservesExecutionAndFlags(t *testing.T) {
	sentinel := errors.New("sentinel")
	fs := flag.NewFlagSet("legacy", flag.ContinueOnError)
	fs.String("id", "", "Resource ID")
	cmd := &ffcli.Command{
		Name:       "legacy",
		ShortUsage: "asc legacy --id ID",
		ShortHelp:  "Run the legacy command.",
		LongHelp:   "Legacy details.",
		FlagSet:    fs,
		UsageFunc:  DefaultUsageFunc,
		Exec: func(context.Context, []string) error {
			_, _ = io.WriteString(os.Stdout, "payload")
			return sentinel
		},
	}

	wrapped := DeprecatedCommand(cmd, "asc legacy", "asc current")
	if wrapped != cmd {
		t.Fatal("DeprecatedCommand must preserve the command instance")
	}
	if wrapped.FlagSet.Lookup("id") == nil {
		t.Fatal("DeprecatedCommand removed an existing flag")
	}
	if !strings.HasPrefix(wrapped.ShortHelp, "DEPRECATED:") || !strings.Contains(wrapped.LongHelp, "asc current") {
		t.Fatalf("deprecation help missing: short=%q long=%q", wrapped.ShortHelp, wrapped.LongHelp)
	}

	stdout, stderr := captureDeprecationStreams(t, func() {
		if err := wrapped.Exec(context.Background(), nil); !errors.Is(err, sentinel) {
			t.Fatalf("Exec error = %v, want sentinel", err)
		}
	})
	if stdout != "payload" {
		t.Fatalf("stdout = %q, want payload", stdout)
	}
	if strings.Count(stderr, "Warning:") != 1 || !strings.Contains(stderr, "`asc legacy`") || !strings.Contains(stderr, "`asc current`") {
		t.Fatalf("stderr = %q, want one precise warning", stderr)
	}
}

func TestDeprecatedCommandPreservesExperimentalLifecycleLabel(t *testing.T) {
	cmd := &ffcli.Command{
		Name:      "sync",
		ShortHelp: "[experimental] Sync legacy resources.",
		LongHelp:  "This command is experimental.",
	}

	wrapped := DeprecatedCommand(cmd, "asc legacy sync", "asc current")
	const wantPrefix = "DEPRECATED: [experimental] "
	if !strings.HasPrefix(wrapped.ShortHelp, wantPrefix) {
		t.Fatalf("ShortHelp = %q, want prefix %q", wrapped.ShortHelp, wantPrefix)
	}
	if !strings.HasPrefix(wrapped.LongHelp, wantPrefix) {
		t.Fatalf("LongHelp = %q, want prefix %q", wrapped.LongHelp, wantPrefix)
	}
}

func captureDeprecationStreams(t *testing.T, run func()) (string, string) {
	t.Helper()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	t.Cleanup(func() {
		os.Stdout, os.Stderr = oldStdout, oldStderr
	})

	run()
	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	os.Stdout, os.Stderr = oldStdout, oldStderr
	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(stdout), string(stderr)
}
