package encryption

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestEncryptionCommandConstructors(t *testing.T) {
	top := EncryptionCommand()
	if top == nil {
		t.Fatal("expected encryption command")
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := EncryptionCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}

	constructors := []func() any{
		func() any { return EncryptionDeclarationsCommand() },
		func() any { return EncryptionDocumentsCommand() },
		func() any { return EncryptionDeclarationsAppCommand() },
	}
	for _, ctor := range constructors {
		if got := ctor(); got == nil {
			t.Fatal("expected constructor to return command")
		}
	}
}

func TestEncryptionDeclarationsExemptDeclareCommand_RejectsPositionalArgs(t *testing.T) {
	cmd := EncryptionDeclarationsExemptDeclareCommand()

	if err := cmd.FlagSet.Parse([]string{"unexpected"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{"unexpected"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestEncryptionDeclarationsExemptDeclareCommand_RejectsEmptyPlistValue(t *testing.T) {
	cmd := EncryptionDeclarationsExemptDeclareCommand()

	if err := cmd.FlagSet.Parse([]string{"--plist", ""}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}
