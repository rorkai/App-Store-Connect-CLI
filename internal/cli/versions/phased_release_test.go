package versions

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestPhasedReleaseGetCommand_MissingVersion(t *testing.T) {
	cmd := PhasedReleaseGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp when --version is missing, got %v", err)
	}
}

func TestPhasedReleaseCreateCommand_MissingVersion(t *testing.T) {
	cmd := PhasedReleaseCreateCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp when --version is missing, got %v", err)
	}
}

func TestPhasedReleaseCreateCommand_InvalidState(t *testing.T) {
	cmd := PhasedReleaseCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--version-id", "123", "--state", "INVALID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp for invalid state, got %v", err)
	}
}

func TestPhasedReleaseCreateCommand_ValidStates(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	validStates := []string{"INACTIVE", "ACTIVE", "inactive", "active"}

	for _, state := range validStates {
		t.Run(state, func(t *testing.T) {
			cmd := PhasedReleaseCreateCommand()

			if err := cmd.FlagSet.Parse([]string{"--version-id", "123", "--state", state}); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			err := cmd.Exec(context.Background(), []string{})
			// Should not be flag.ErrHelp for valid states (will fail later due to no auth)
			if errors.Is(err, flag.ErrHelp) {
				t.Errorf("state %s should be valid but got flag.ErrHelp", state)
			}
		})
	}
}

func TestPhasedReleaseUpdateCommand_MissingID(t *testing.T) {
	cmd := PhasedReleaseUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--state", "PAUSED"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestPhasedReleaseUpdateCommand_MissingState(t *testing.T) {
	cmd := PhasedReleaseUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp when --state is missing, got %v", err)
	}
}

func TestPhasedReleaseUpdateCommand_InvalidState(t *testing.T) {
	cmd := PhasedReleaseUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "123", "--state", "INVALID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp for invalid state, got %v", err)
	}
}

func TestPhasedReleaseUpdateCommand_ValidStates(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	validStates := []string{"ACTIVE", "PAUSED", "COMPLETE", "active", "paused", "complete"}

	for _, state := range validStates {
		t.Run(state, func(t *testing.T) {
			cmd := PhasedReleaseUpdateCommand()

			if err := cmd.FlagSet.Parse([]string{"--id", "123", "--state", state}); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			err := cmd.Exec(context.Background(), []string{})
			// Should not be flag.ErrHelp for valid states (will fail later due to no auth)
			if errors.Is(err, flag.ErrHelp) {
				t.Errorf("state %s should be valid but got flag.ErrHelp", state)
			}
		})
	}
}

func TestPhasedReleaseDeleteCommand_MissingID(t *testing.T) {
	cmd := PhasedReleaseDeleteCommand()

	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestPhasedReleaseDeleteCommand_MissingConfirm(t *testing.T) {
	cmd := PhasedReleaseDeleteCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}

func TestPhasedReleaseCommand_FlagDefinitions(t *testing.T) {
	// Test get command flags
	getCmd := PhasedReleaseGetCommand()
	expectedGetFlags := []string{"version-id", "output", "pretty"}
	for _, name := range expectedGetFlags {
		if getCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("get: expected flag --%s to be defined", name)
		}
	}

	// Test create command flags
	createCmd := PhasedReleaseCreateCommand()
	expectedCreateFlags := []string{"version-id", "state", "output", "pretty"}
	for _, name := range expectedCreateFlags {
		if createCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("create: expected flag --%s to be defined", name)
		}
	}

	// Test update command flags
	updateCmd := PhasedReleaseUpdateCommand()
	expectedUpdateFlags := []string{"id", "state", "output", "pretty"}
	for _, name := range expectedUpdateFlags {
		if updateCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("update: expected flag --%s to be defined", name)
		}
	}

	// Test delete command flags
	deleteCmd := PhasedReleaseDeleteCommand()
	expectedDeleteFlags := []string{"id", "confirm", "output", "pretty"}
	for _, name := range expectedDeleteFlags {
		if deleteCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("delete: expected flag --%s to be defined", name)
		}
	}
}

func TestPhasedReleaseCommand_DefaultOutputJSON(t *testing.T) {
	commands := []*struct {
		name string
		cmd  func() *ffcli.Command
	}{
		{"get", PhasedReleaseGetCommand},
		{"create", PhasedReleaseCreateCommand},
		{"update", PhasedReleaseUpdateCommand},
		{"delete", PhasedReleaseDeleteCommand},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd()
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
