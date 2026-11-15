package feedback

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{IO: &shared.IO{Out: &stdout, ErrOut: &stderr}}, &stdout, &stderr
}

func assertStreams(t *testing.T, stdout, stderr *bytes.Buffer, wantStderr string) {
	t.Helper()
	if got := stderr.String(); got != wantStderr {
		t.Fatalf("stderr = %q, want %q", got, wantStderr)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestListCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	d, stdout, stderr := newTestDeps()
	cmd := NewListCommandWith(d, shared.ListCommandConfig{Name: "list"})

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	assertStreams(t, stdout, stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
}

func TestListCommand_InvalidLimit(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := NewListCommandWith(d, shared.ListCommandConfig{Name: "list"})

	if err := cmd.FlagSet.Parse([]string{"--limit", "201", "--app", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error for --limit, got %v", err)
	}
	assertStreams(t, stdout, stderr, "Error: feedback: --limit must be between 1 and 200\n")
}

func TestListCommand_InvalidSort(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := NewListCommandWith(d, shared.ListCommandConfig{Name: "list"})

	if err := cmd.FlagSet.Parse([]string{"--sort", "invalid", "--app", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected sort validation error, got %v", err)
	}
	assertStreams(t, stdout, stderr, "Error: feedback: --sort must be one of: createdDate, -createdDate\n")
}

func TestListCommand_DefaultsNameAndUsageFunc(t *testing.T) {
	d, _, _ := newTestDeps()
	cmd := NewListCommandWith(d, shared.ListCommandConfig{})
	if cmd.Name != "feedback" {
		t.Fatalf("expected default name feedback, got %q", cmd.Name)
	}
	if cmd.UsageFunc == nil {
		t.Fatal("expected default usage func")
	}
	if cmd.FlagSet.Lookup("build") != nil {
		t.Fatal("expected removed --build alias to be unregistered")
	}
}
