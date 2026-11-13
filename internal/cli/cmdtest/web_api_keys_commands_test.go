package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestWebAPIKeysListCommandRegistration(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "web", "api-keys", "list")
	if cmd == nil {
		t.Fatal("expected web api-keys list command")
	}
	for _, flagName := range []string{
		"apple-id",
		"two-factor-code-command",
		"provider-id",
		"public-provider-id",
		"output",
		"pretty",
	} {
		if cmd.FlagSet.Lookup(flagName) == nil {
			t.Fatalf("expected --%s flag", flagName)
		}
	}
	if cmd.FlagSet.Lookup("paginate") != nil {
		t.Fatal("did not expect --paginate on web api-keys list; the team and individual key readers already follow every page")
	}
}

func TestWebAPIKeysViewMissingKeyIDFailsBeforeHTTP(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "api-keys", "view"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --key-id is required") {
		t.Fatalf("expected missing --key-id error, got %q", stderr)
	}
}

func TestWebAPIKeysCreateCommandRegistration(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "web", "api-keys", "create")
	if cmd == nil {
		t.Fatal("expected web api-keys create command")
	}
	for _, flagName := range []string{
		"name",
		"role",
		"output-dir",
		"apple-id",
		"two-factor-code-command",
		"provider-id",
		"public-provider-id",
		"output",
		"pretty",
	} {
		if cmd.FlagSet.Lookup(flagName) == nil {
			t.Fatalf("expected --%s flag", flagName)
		}
	}
	if got := cmd.FlagSet.Lookup("role").DefValue; got != "ADMIN" {
		t.Fatalf("expected --role default ADMIN, got %q", got)
	}
}
