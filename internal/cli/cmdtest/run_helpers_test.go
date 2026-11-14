package cmdtest

import (
	"bytes"
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func assertUsageExit(t *testing.T, args []string, wantErr string) {
	t.Helper()

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", rootcmd.ExitUsage, code)
		}
	})
	assertUsageExitOutput(t, stdout, stderr, wantErr)
}

// assertUsageExitWithDeps is assertUsageExit for a command built on
// shared.Deps, run through rootcmd.RunWithDeps with client.
func assertUsageExitWithDeps(t *testing.T, client func() (*asc.Client, error), args []string, wantErr string) {
	t.Helper()

	stdout, stderr, code := runWithDeps(t, client, args...)
	if code != rootcmd.ExitUsage {
		t.Fatalf("expected exit code %d, got %d", rootcmd.ExitUsage, code)
	}
	assertUsageExitOutput(t, stdout, stderr, wantErr)
}

func assertUsageExitOutput(t *testing.T, stdout, stderr, wantErr string) {
	t.Helper()

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, wantErr) {
		t.Fatalf("expected stderr to contain %q, got %q", wantErr, stderr)
	}
}

// runRootWithDeps parses and runs args on a root command built on test deps
// and returns what the commands wrote to the deps' stdout and stderr. Only
// commands built on shared.Deps use client and write to these streams.
func runRootWithDeps(t *testing.T, client func() (*asc.Client, error), args ...string) (string, string, error) {
	t.Helper()

	d, stdout, stderr := newTestDeps(t, client)
	root := rootcmd.RootCommandWithDeps("1.2.3", d)
	if err := root.Parse(args); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	return stdout.String(), stderr.String(), err
}

// runWithDeps runs args through rootcmd.RunWithDeps on test deps and returns
// what was written to the deps' stdout and stderr, and the exit code. Only
// commands built on shared.Deps use client and write to these streams.
func runWithDeps(t *testing.T, client func() (*asc.Client, error), args ...string) (string, string, int) {
	t.Helper()

	d, stdout, stderr := newTestDeps(t, client)
	code := rootcmd.RunWithDeps(args, "1.2.3", d)
	return stdout.String(), stderr.String(), code
}

func newTestDeps(t *testing.T, client func() (*asc.Client, error)) (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	resetCmdtestState()
	t.Cleanup(resetCmdtestState)

	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		Client:  client,
		IO:      &shared.IO{In: strings.NewReader(""), Out: &stdout, ErrOut: &stderr},
		Now:     time.Now,
		Timeout: shared.ContextWithTimeout,
	}, &stdout, &stderr
}

// stubClient returns a client factory whose clients send every request to rt.
func stubClient(t *testing.T, rt http.RoundTripper) func() (*asc.Client, error) {
	t.Helper()

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	return func() (*asc.Client, error) {
		return asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, &http.Client{Transport: rt})
	}
}
