package preorders

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestPreOrdersGetCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	d, stdout, stderr := newTestDeps(nil)
	cmd := newPreOrdersGetCommand(d)
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
	assertOutput(t, stdout, stderr, "Error: --app is required (or set ASC_APP_ID)\n")
}

func TestPreOrdersListCommand_MissingAvailability(t *testing.T) {
	d, stdout, stderr := newTestDeps(nil)
	cmd := newPreOrdersListCommand(d)

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --availability is missing, got %v", err)
	}
	assertOutput(t, stdout, stderr, "Error: --availability is required\n")
}

func TestPreOrdersEnableCommand_MissingFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "missing app", args: []string{"--territory", "USA", "--release-date", "2026-01-20"}, wantStderr: "Error: --app is required (or set ASC_APP_ID)\n"},
		{name: "missing territory", args: []string{"--app", "APP", "--release-date", "2026-01-20"}, wantStderr: "Error: --territory is required\n"},
		{name: "missing release date", args: []string{"--app", "APP", "--territory", "USA"}, wantStderr: "Error: --release-date is required\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, stdout, stderr := newTestDeps(nil)
			cmd := newPreOrdersEnableCommand(d)
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
			assertOutput(t, stdout, stderr, test.wantStderr)
		})
	}
}

func TestPreOrdersEnableCommand_InvalidDate(t *testing.T) {
	d, stdout, stderr := newTestDeps(nil)
	cmd := newPreOrdersEnableCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--app", "APP", "--territory", "USA", "--release-date", "invalid"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for invalid release date")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for invalid release date, got %v", err)
	}
	assertOutput(t, stdout, stderr, "Error: pre-orders enable: --release-date must be in YYYY-MM-DD format\n")
}

func TestPreOrdersUpdateCommand_MissingID(t *testing.T) {
	d, stdout, stderr := newTestDeps(nil)
	cmd := newPreOrdersUpdateCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--release-date", "2026-02-01"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --territory-availability is missing, got %v", err)
	}
	assertOutput(t, stdout, stderr, "Error: --territory-availability is required\n")
}

func TestPreOrdersUpdateCommand_MissingAllAttributes(t *testing.T) {
	d, stdout, stderr := newTestDeps(nil)
	cmd := newPreOrdersUpdateCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--territory-availability", "ta-1"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when no attributes are provided, got %v", err)
	}
	assertOutput(t, stdout, stderr, "Error: at least one update flag is required (--release-date, --pre-order-enabled, --available)\n")
}

func TestPreOrdersUpdateCommand_InvalidDate(t *testing.T) {
	d, stdout, stderr := newTestDeps(nil)
	cmd := newPreOrdersUpdateCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--territory-availability", "ta-1", "--release-date", "invalid"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for invalid release date")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for invalid release date, got %v", err)
	}
	assertOutput(t, stdout, stderr, "Error: pre-orders update: --release-date must be in YYYY-MM-DD format\n")
}

func TestPreOrdersDisableCommand_MissingID(t *testing.T) {
	d, stdout, stderr := newTestDeps(nil)
	cmd := newPreOrdersDisableCommand(d)

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --territory-availability is missing, got %v", err)
	}
	assertOutput(t, stdout, stderr, "Error: --territory-availability is required\n")
}

func TestPreOrdersEndCommand_MissingIDs(t *testing.T) {
	d, stdout, stderr := newTestDeps(nil)
	cmd := newPreOrdersEndCommand(d)

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --territory-availability is missing, got %v", err)
	}
	assertOutput(t, stdout, stderr, "Error: --territory-availability is required\n")
}

func TestPreOrdersEndCommand_RequiresConfirmationBeforeClient(t *testing.T) {
	clientRequested := false
	d, stdout, stderr := newTestDeps(func() (*asc.Client, error) {
		clientRequested = true
		return nil, errors.New("client must not be created")
	})

	cmd := newPreOrdersEndCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--territory-availability", "ta-1"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
	if clientRequested {
		t.Fatal("expected missing confirmation to fail before client creation")
	}
	assertOutput(t, stdout, stderr, "Error: --confirm is required\n")
}

func TestPreOrdersCommand_FlagDefinitions(t *testing.T) {
	d, _, _ := newTestDeps(nil)
	getCmd := newPreOrdersGetCommand(d)
	for _, name := range []string{"app", "output", "pretty"} {
		if getCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("get: expected flag --%s to be defined", name)
		}
	}

	listCmd := newPreOrdersListCommand(d)
	for _, name := range []string{"availability", "output", "pretty"} {
		if listCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("list: expected flag --%s to be defined", name)
		}
	}

	enableCmd := newPreOrdersEnableCommand(d)
	for _, name := range []string{"app", "territory", "release-date", "output", "pretty"} {
		if enableCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("enable: expected flag --%s to be defined", name)
		}
	}
	if enableCmd.FlagSet.Lookup("available-in-new-territories") != nil {
		t.Error("enable: removed --available-in-new-territories flag should not be defined")
	}

	updateCmd := newPreOrdersUpdateCommand(d)
	for _, name := range []string{"territory-availability", "release-date", "pre-order-enabled", "available", "output", "pretty"} {
		if updateCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("update: expected flag --%s to be defined", name)
		}
	}

	disableCmd := newPreOrdersDisableCommand(d)
	for _, name := range []string{"territory-availability", "output", "pretty"} {
		if disableCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("disable: expected flag --%s to be defined", name)
		}
	}

	endCmd := newPreOrdersEndCommand(d)
	for _, name := range []string{"territory-availability", "confirm", "output", "pretty"} {
		if endCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("end: expected flag --%s to be defined", name)
		}
	}
}

func TestPreOrdersCommand_DefaultOutputJSON(t *testing.T) {
	commands := []*struct {
		name string
		cmd  func(*shared.Deps) *ffcli.Command
	}{
		{"get", newPreOrdersGetCommand},
		{"list", newPreOrdersListCommand},
		{"enable", newPreOrdersEnableCommand},
		{"update", newPreOrdersUpdateCommand},
		{"disable", newPreOrdersDisableCommand},
		{"end", newPreOrdersEndCommand},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			d, _, _ := newTestDeps(nil)
			cmd := tc.cmd(d)
			f := cmd.FlagSet.Lookup("output")
			if f == nil {
				t.Fatal("expected --output flag to be defined")
				return
			}
			if f.DefValue != "json" {
				t.Errorf("expected --output default to be 'json', got %q", f.DefValue)
			}
		})
	}
}

func TestPreOrdersUpdateCommand_PreOrderEnabledAlone(t *testing.T) {
	d, stdout, stderr := newTestDeps(failingTransportClient(t))
	cmd := newPreOrdersUpdateCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--territory-availability", "ta-1", "--pre-order-enabled", "true"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatal("did not expect flag.ErrHelp when --pre-order-enabled is set")
	}
	if !errors.Is(err, errTestTransport) {
		t.Fatalf("expected the transport error, got %v", err)
	}
	assertOutput(t, stdout, stderr, "")
}

func TestPreOrdersUpdateCommand_AvailableAlone(t *testing.T) {
	d, stdout, stderr := newTestDeps(failingTransportClient(t))
	cmd := newPreOrdersUpdateCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--territory-availability", "ta-1", "--available", "false"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatal("did not expect flag.ErrHelp when --available is set")
	}
	if !errors.Is(err, errTestTransport) {
		t.Fatalf("expected the transport error, got %v", err)
	}
	assertOutput(t, stdout, stderr, "")
}

func TestPreOrdersUpdateCommand_DisablePreOrderRejectsReleaseDate(t *testing.T) {
	d, stdout, stderr := newTestDeps(nil)
	cmd := newPreOrdersUpdateCommand(d)

	if err := cmd.FlagSet.Parse([]string{
		"--territory-availability", "ta-1",
		"--pre-order-enabled", "false",
		"--release-date", "2026-06-01",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp for conflicting flags, got %v", err)
	}
	assertOutput(t, stdout, stderr, "Error: --release-date cannot be set when disabling pre-orders (releaseDate must be null)\n")
}

func TestPreOrdersUpdateCommand_DisablePreOrderAlone(t *testing.T) {
	d, stdout, stderr := newTestDeps(failingTransportClient(t))
	cmd := newPreOrdersUpdateCommand(d)

	if err := cmd.FlagSet.Parse([]string{
		"--territory-availability", "ta-1",
		"--pre-order-enabled", "false",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatal("did not expect flag.ErrHelp when --pre-order-enabled false is set")
	}
	if !errors.Is(err, errTestTransport) {
		t.Fatalf("expected the transport error, got %v", err)
	}
	assertOutput(t, stdout, stderr, "")
}

var errTestTransport = errors.New("test transport unavailable")

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errTestTransport
}

// failingTransportClient returns a client factory whose requests fail with
// errTestTransport without leaving the process.
func failingTransportClient(t *testing.T) func() (*asc.Client, error) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return func() (*asc.Client, error) {
		return asc.NewClientWithHTTPClient("KEY", "ISSUER", path, &http.Client{Transport: failingTransport{}})
	}
}

func newTestDeps(client func() (*asc.Client, error)) (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		Client:    client,
		IO:        &shared.IO{Out: &stdout, ErrOut: &stderr},
		Timeout:   shared.ContextWithTimeout,
		LookupEnv: os.LookupEnv,
	}, &stdout, &stderr
}

func assertOutput(t *testing.T, stdout, stderr *bytes.Buffer, wantStderr string) {
	t.Helper()
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); got != wantStderr {
		t.Fatalf("stderr = %q, want %q", got, wantStderr)
	}
}
