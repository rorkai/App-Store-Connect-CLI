package testflight

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBetaTestersRemoveCommand_RequiresConfirm(t *testing.T) {
	isolateTestFlightAuthEnvForAddTests(t)

	cmd := newBetaTestersRemoveCommand(&shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}, LookupEnv: os.LookupEnv})
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--email", "tester@example.com",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("remove without --confirm should fail validation, got %v", err)
	}
	if !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("usage error should name --confirm, got %q", err.Error())
	}
}

func TestBetaTestersRemoveCommand_ConfirmPassesValidation(t *testing.T) {
	isolateTestFlightAuthEnvForAddTests(t)

	cmd := newBetaTestersRemoveCommand(passedValidationDeps())
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--email", "tester@example.com",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("remove with --confirm should pass validation, got %v", err)
	}
	if !errors.Is(err, errClientRequested) {
		t.Fatalf("remove with --confirm should request a client after validation, got %v", err)
	}
}
