package buildlocalizations

import "testing"

func TestBuildLocalizationsCommandConstructors(t *testing.T) {
	top := BuildLocalizationsCommand()
	if top == nil {
		t.Fatal("expected build-localizations command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := BuildLocalizationsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}
