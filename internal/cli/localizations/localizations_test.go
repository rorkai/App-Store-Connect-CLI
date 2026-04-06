package localizations

import (
	"context"
	"errors"
	"flag"
	"reflect"
	"strings"
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
	if got := LocalizationsSupportedLocalesCommand(); got == nil {
		t.Fatal("expected supported locales command")
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

func TestLocalizationsCreateCommand_HelpMentionsCanonicalLocaleForms(t *testing.T) {
	cmd := LocalizationsCreateCommand()

	localeFlag := cmd.FlagSet.Lookup("locale")
	if localeFlag == nil {
		t.Fatal("expected --locale flag")
	}
	for _, want := range []string{"canonical ASC values", "ar-SA", "zh-Hans"} {
		if !strings.Contains(localeFlag.Usage, want) {
			t.Fatalf("expected --locale usage to contain %q, got %q", want, localeFlag.Usage)
		}
	}
	for _, want := range []string{
		`asc localizations supported-locales --version "VERSION_ID"`,
		`"ar" is usually rejected; use "ar-SA"`,
		`"de" should usually be "de-DE"`,
		`"zh-Hans-CN"`,
		`"zh-Hant-TW"`,
	} {
		if !strings.Contains(cmd.LongHelp, want) {
			t.Fatalf("expected long help to contain %q, got %q", want, cmd.LongHelp)
		}
	}
}

func TestLocalizationsUpdateCommand_HelpMentionsCanonicalLocaleForms(t *testing.T) {
	cmd := LocalizationsUpdateCommand()

	localeFlag := cmd.FlagSet.Lookup("locale")
	if localeFlag == nil {
		t.Fatal("expected --locale flag")
	}
	for _, want := range []string{"reuse exact ASC locale", "ar-SA", "zh-Hans"} {
		if !strings.Contains(localeFlag.Usage, want) {
			t.Fatalf("expected --locale usage to contain %q, got %q", want, localeFlag.Usage)
		}
	}
	for _, want := range []string{
		`asc localizations supported-locales --version "VERSION_ID"`,
		`asc localizations list --version "VERSION_ID"`,
		`asc localizations list --app "APP_ID" --type app-info`,
		`"ar" is usually stored as "ar-SA"`,
		`"de" is usually stored as "de-DE"`,
		`"zh-Hans-CN" and "zh-Hant-TW"`,
	} {
		if !strings.Contains(cmd.LongHelp, want) {
			t.Fatalf("expected long help to contain %q, got %q", want, cmd.LongHelp)
		}
	}
}

func TestLocalizationsSupportedLocalesCommand_MissingVersion(t *testing.T) {
	cmd := LocalizationsSupportedLocalesCommand()
	if err := cmd.FlagSet.Parse(nil); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestAppInfoAttemptedFieldsKeepsWhitespaceOnlyValues(t *testing.T) {
	got := appInfoAttemptedFields(updateAppInfoParams{
		name:              " ",
		privacyChoicesURL: "\t",
	})

	want := []string{"name", "privacyChoicesUrl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appInfoAttemptedFields() = %v, want %v", got, want)
	}
}

func TestVersionAttemptedFieldsKeepsWhitespaceOnlyValues(t *testing.T) {
	got := versionAttemptedFields(updateVersionParams{
		description: " ",
		supportURL:  "\n",
	})

	want := []string{"description", "supportUrl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("versionAttemptedFields() = %v, want %v", got, want)
	}
}
