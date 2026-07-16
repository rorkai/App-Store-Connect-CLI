package encryption

import (
	"context"
	"errors"
	"flag"
	"testing"
)

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
