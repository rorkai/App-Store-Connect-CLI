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

func TestListCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	d, stdout, stderr := newTestDeps()
	cmd := NewListCommand(d, shared.ListCommandConfig{Name: "list"})

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if got, want := stderr.String(), "Error: --app is required (or set ASC_APP_ID)\n\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestListCommand_InvalidLimit(t *testing.T) {
	t.Parallel()

	d, stdout, stderr := newTestDeps()
	cmd := NewListCommand(d, shared.ListCommandConfig{Name: "list"})

	if err := cmd.FlagSet.Parse([]string{"--limit", "201", "--app", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error for --limit, got %v", err)
	}
	if got, want := stderr.String(), "Error: feedback: --limit must be between 1 and 200\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestListCommand_InvalidSort(t *testing.T) {
	t.Parallel()

	d, stdout, stderr := newTestDeps()
	cmd := NewListCommand(d, shared.ListCommandConfig{Name: "list"})

	if err := cmd.FlagSet.Parse([]string{"--sort", "invalid", "--app", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected sort validation error, got %v", err)
	}
	if got, want := stderr.String(), "Error: feedback: --sort must be one of: createdDate, -createdDate\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestListCommand_DefaultsNameAndUsageFunc(t *testing.T) {
	t.Parallel()

	d, _, _ := newTestDeps()
	cmd := NewListCommand(d, shared.ListCommandConfig{})
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
