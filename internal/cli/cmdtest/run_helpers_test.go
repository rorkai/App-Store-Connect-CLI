package cmdtest

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
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

// runRootWithHTTPTransport is runRootWithDeps whose clients for services other
// than App Store Connect send every request to rt.
func runRootWithHTTPTransport(t *testing.T, client func() (*asc.Client, error), rt http.RoundTripper, args ...string) (string, string, error) {
	t.Helper()

	d, stdout, stderr := newTestDeps(t, client)
	d.HTTPTransport = func() http.RoundTripper { return rt }
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

var registerSignalsOutsideBubbles sync.Once

// runWithDepsOnFakeClock runs rootcmd.RunWithDeps in a synctest bubble, so the
// command's deadlines and poll intervals use the bubble's fake clock.
//
// RunWithDeps registers for os.Interrupt. The runtime sets up its signal
// handling on the first registration in the process and makes the channels it
// keeps for good in the caller's bubble, which crashes the process ("select on
// synctest channel from outside bubble"). The first registration is therefore
// made here, outside any bubble.
func runWithDepsOnFakeClock(t *testing.T, d *shared.Deps, version string, args ...string) int {
	t.Helper()

	registerSignalsOutsideBubbles.Do(func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt)
		signal.Stop(signals)
	})
	var code int
	synctest.Test(t, func(*testing.T) { code = rootcmd.RunWithDeps(args, version, d) })
	return code
}

// runWithHTTPTransport is runWithDeps whose clients for services other than
// App Store Connect send every request to rt.
func runWithHTTPTransport(t *testing.T, client func() (*asc.Client, error), rt http.RoundTripper, args ...string) (string, string, int) {
	t.Helper()

	d, stdout, stderr := newTestDeps(t, client)
	d.HTTPTransport = func() http.RoundTripper { return rt }
	code := rootcmd.RunWithDeps(args, "1.2.3", d)
	return stdout.String(), stderr.String(), code
}

func newTestDeps(t *testing.T, client func() (*asc.Client, error)) (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	resetCmdtestState()
	t.Cleanup(resetCmdtestState)

	var clientWithTimeout func(time.Duration) (*asc.Client, error)
	if client != nil {
		clientWithTimeout = func(time.Duration) (*asc.Client, error) { return client() }
	}
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		Client:         client,
		IO:             &shared.IO{In: strings.NewReader(""), Out: &stdout, ErrOut: &stderr},
		Now:            time.Now,
		Timeout:        shared.ContextWithTimeout,
		UploadTimeout:  shared.ContextWithUploadTimeout,
		ResolveTimeout: shared.ResolveTimeoutSetting,
		After:          time.After,
		Getwd:          os.Getwd,
		LookupEnv:      os.LookupEnv,
		HTTPTransport:  func() http.RoundTripper { return http.DefaultTransport },

		ClientWithTimeout: clientWithTimeout,
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

// stubServerClient returns a client factory whose clients send every request
// to server, keeping the request path and query.
func stubServerClient(t *testing.T, server *httptest.Server) func() (*asc.Client, error) {
	t.Helper()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return stubClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	}))
}
