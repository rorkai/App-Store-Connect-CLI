package betaapplocalizations

import "testing"

func TestBetaAppLocalizationsCommandConstructors(t *testing.T) {
	top := BetaAppLocalizationsCommand()
	if top == nil {
		t.Fatal("expected beta-app-localizations command")
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := BetaAppLocalizationsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
	if got := DeprecatedBetaAppLocalizationsCommand(); got == nil {
		t.Fatal("expected deprecated root alias command")
	}
	if got := BetaAppLocalizationsAppCommand(); got == nil {
		t.Fatal("expected app relationship command")
	}
}
