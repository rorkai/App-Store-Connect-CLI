package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func runBuildIDFlagCommand(t *testing.T, args []string) (string, string, error) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	return stdout, stderr, runErr
}

func TestBuildIDRequiredErrorsNameCanonicalFlag(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name string
		args []string
		deps bool // the command is built on shared.Deps: run it on test deps
	}{
		{name: "validate testflight", args: []string{"validate", "testflight", "--app", "app-1"}},
		{name: "build-bundles list", args: []string{"build-bundles", "list"}, deps: true},
		{name: "build-localizations create", args: []string{"build-localizations", "create", "--locale", "en-US"}, deps: true},
		{name: "performance diagnostics list", args: []string{"performance", "diagnostics", "list"}},
		{name: "performance metrics view", args: []string{"performance", "metrics", "view"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr string
			var runErr error
			if test.deps {
				stdout, stderr, runErr = runRootWithDeps(t, nil, test.args...)
			} else {
				stdout, stderr, runErr = runBuildIDFlagCommand(t, test.args)
			}
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "Error: --build-id is required") {
				t.Fatalf("expected --build-id required error, got %q", stderr)
			}
		})
	}
}
