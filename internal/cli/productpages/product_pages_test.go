package productpages

import "testing"

func TestProductPagesCommandConstructors(t *testing.T) {
	top := ProductPagesCommand()
	if top == nil {
		t.Fatal("expected product-pages command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := ProductPagesCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}

	constructors := []func() any{
		func() any { return CustomPagesCommand() },
		func() any { return CustomPageLocalizationsCommand() },
		func() any { return CustomPageVersionsCommand() },
		func() any { return ExperimentsCommand() },
		func() any { return ExperimentTreatmentsCommand() },
	}
	for _, ctor := range constructors {
		if got := ctor(); got == nil {
			t.Fatal("expected constructor to return command")
		}
	}
}
