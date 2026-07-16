package notarization

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestNotarizationSubmitValidation(t *testing.T) {
	cmd := submitCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}
