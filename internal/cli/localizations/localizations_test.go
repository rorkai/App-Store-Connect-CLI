package localizations

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestLocalizationsCommandConstructors(t *testing.T) {
	top := LocalizationsCommand()
	if top == nil {
		t.Fatal("expected localizations command")
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := LocalizationsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}

	if got := LocalizationsPreviewSetsCommand(); got == nil {
		t.Fatal("expected preview sets command")
	}
	if got := LocalizationsSearchKeywordsCommand(); got == nil {
		t.Fatal("expected search keywords command")
	}
	if got := LocalizationsCreateCommand(); got == nil {
		t.Fatal("expected create command")
	}
}

func TestLocalizationsCreateCommand_MissingFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		execArgs []string
	}{
		{name: "unexpected args", args: []string{"--version", "VERSION_ID", "--locale", "ja", "unexpected"}, execArgs: []string{"unexpected"}},
		{name: "missing version", args: []string{"--locale", "ja"}},
		{name: "missing locale", args: []string{"--version", "VERSION_ID"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := LocalizationsCreateCommand()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), test.execArgs); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
	}
}

func TestLocalizationsCreateCommand_InvalidLocale(t *testing.T) {
	cmd := LocalizationsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--version", "VERSION_ID", "--locale", "not_a_locale"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected invalid locale error, got nil")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}
