package shared

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestOnceCSVValue_RejectsRepeatedUse(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	value := BindOnceCSVFlag(fs, "ids", "Comma-separated IDs")

	if err := value.Set("a,b"); err != nil {
		t.Fatalf("first Set() error: %v", err)
	}
	if got := value.String(); got != "a,b" {
		t.Fatalf("String() = %q, want %q", got, "a,b")
	}

	err := value.Set("c")
	if err == nil {
		t.Fatal("second Set() should fail")
	}
	wantMessage := `--ids specified multiple times; pass one comma-separated list, for example --ids "a,b,c"`
	if err.Error() != wantMessage {
		t.Fatalf("second Set() error = %q, want %q", err.Error(), wantMessage)
	}
	if got := value.String(); got != "a,b" {
		t.Fatalf("rejected Set() must not overwrite value, got %q", got)
	}
	if !value.Provided() {
		t.Fatal("Provided() should remain true after a rejected second Set()")
	}
}

func TestOnceCSVValue_ProvidedAfterRecoveryParse(t *testing.T) {
	original := flag.NewFlagSet("test", flag.ContinueOnError)
	original.SetOutput(nopWriter{})
	value := BindOnceCSVFlag(original, "territories", "territories")
	confirm := original.Bool("confirm", false, "confirm")
	availableInNew := original.Bool("available-in-new-territories", false, "available in new")

	if err := original.Parse([]string{"--available-in-new-territories", "false", "--territories", "USA", "--confirm"}); err != nil {
		t.Fatalf("original parse error: %v", err)
	}
	if value.Provided() {
		t.Fatal("Go flag parse should stop at the spaced boolean and leave --territories unvisited")
	}

	if err := DefaultDeps().RecoverBoolFlagTailArgs(original, original.Args(), availableInNew); err != nil {
		t.Fatalf("RecoverBoolFlagTailArgs() error: %v", err)
	}
	if !value.Provided() || value.String() != "USA" {
		t.Fatalf("recovered --territories, Provided()=%t String()=%q", value.Provided(), value.String())
	}
	if !*confirm {
		t.Fatal("expected --confirm to be recovered onto the shared value")
	}
}

func TestOnceCSVValue_ParseRejectsRepeatedFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(nopWriter{})
	BindOnceCSVFlag(fs, "ids", "Comma-separated IDs")

	err := fs.Parse([]string{"--ids", "a", "--ids", "b"})
	if err == nil {
		t.Fatal("parse with repeated --ids should fail")
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("repeated flag should be a parse error, not help: %v", err)
	}
	wantFragment := `--ids specified multiple times; pass one comma-separated list, for example --ids "a,b"`
	if !strings.Contains(err.Error(), wantFragment) {
		t.Fatalf("parse error = %q, want it to contain %q", err.Error(), wantFragment)
	}
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
