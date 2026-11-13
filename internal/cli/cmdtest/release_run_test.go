package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestReleaseRunIsNotRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"release", "run",
			"--app", "APP_123",
			"--version", "1.2.3",
			"--build", "BUILD_123",
			"--metadata-dir", "./metadata/version/1.2.3",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "Run high-level App Store release workflows.") {
		t.Fatalf("expected release help for unknown run subcommand, got %q", stderr)
	}
	if !strings.Contains(stderr, "asc release stage") {
		t.Fatalf("expected replacement guidance to mention asc release stage, got %q", stderr)
	}
	if strings.Contains(stderr, "\n  run") {
		t.Fatalf("expected release run to be omitted from help, got %q", stderr)
	}
}
