package testflight

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
	"testing"
)

func isolateTestFlightAuthEnvForAddTests(t *testing.T) {
	t.Helper()

	// Keep tests hermetic: avoid host keychain/config/env credentials.
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
	t.Setenv("ASC_APP_ID", "")
}

func TestBetaTestersAddCommand_EmailOnlyPassesValidation(t *testing.T) {
	isolateTestFlightAuthEnvForAddTests(t)

	cmd := BetaTestersAddCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--email", "tester@example.com",
		"--group", "Beta",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("email-only add should pass validation, got %v", err)
	}
}

func TestBetaTestersAddCommand_FirstNameOnlyPassesValidation(t *testing.T) {
	isolateTestFlightAuthEnvForAddTests(t)

	cmd := BetaTestersAddCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--email", "tester@example.com",
		"--first-name", "Solo",
		"--group", "Beta",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("first-name + email add should pass validation, got %v", err)
	}
}

func TestBetaTestersAddCommand_NameWithoutEmailFailsValidation(t *testing.T) {
	isolateTestFlightAuthEnvForAddTests(t)

	cmd := BetaTestersAddCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--first-name", "Only",
		"--last-name", "Name",
		"--group", "Beta",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("name without email should fail validation, got %v", err)
	}
}

func TestBetaGroupsAddTestersCommand_EmailFlagPassesValidation(t *testing.T) {
	isolateTestFlightAuthEnvForAddTests(t)

	cmd := BetaGroupsAddTestersCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--group", "group-1",
		"--email", "tester@example.com",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("email-based add-testers should pass validation, got %v", err)
	}
}
