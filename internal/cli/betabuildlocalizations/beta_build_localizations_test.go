package betabuildlocalizations

import (
	"strings"
	"testing"
)

func TestBetaBuildLocalizationsCommandConstructors(t *testing.T) {
	top := BetaBuildLocalizationsCommand()
	if top == nil {
		t.Fatal("expected beta-build-localizations command")
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := BetaBuildLocalizationsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
	if got := BetaBuildLocalizationsBuildCommand(); got == nil {
		t.Fatal("expected build relationship command")
	}
}

func TestBetaBuildLocalizationsCreateCommandUpsertFlag(t *testing.T) {
	cmd := BetaBuildLocalizationsCreateCommand()
	if cmd == nil {
		t.Fatal("expected create command")
	}

	upsertFlag := cmd.FlagSet.Lookup("upsert")
	if upsertFlag == nil {
		t.Fatal("expected --upsert flag")
	}
	if !strings.Contains(upsertFlag.Usage, "Create-or-update") {
		t.Fatalf("expected --upsert usage text, got %q", upsertFlag.Usage)
	}
}
