package apps

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestAppSetupInfoSetCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	cmd := AppSetupInfoSetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestAppSetupInfoSetCommand_MissingUpdates(t *testing.T) {
	cmd := AppSetupInfoSetCommand()

	if err := cmd.FlagSet.Parse([]string{"--app", "APP"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when no update flags provided, got %v", err)
	}
}

func TestAppSetupInfoSetCommand_MissingLocale(t *testing.T) {
	cmd := AppSetupInfoSetCommand()

	if err := cmd.FlagSet.Parse([]string{"--app", "APP", "--name", "My App"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when locale is missing, got %v", err)
	}
}

func TestAppSetupCategoriesSetCommand_MissingFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing app", args: []string{"--primary", "GAMES"}},
		{name: "missing primary", args: []string{"--app", "APP"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := AppSetupCategoriesSetCommand()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), []string{}); err == nil {
				t.Fatal("expected error for missing flags")
			}
		})
	}
}

func TestAppSetupCategoriesSetCommand_HelpMentionsSubcategoryFlags(t *testing.T) {
	cmd := AppSetupCategoriesSetCommand()

	if !strings.Contains(cmd.ShortUsage, "[flags]") {
		t.Fatalf("expected [flags] in short usage, got %q", cmd.ShortUsage)
	}

	if !strings.Contains(cmd.LongHelp, "--primary-subcategory-one") {
		t.Fatalf("expected subcategory example in long help, got %q", cmd.LongHelp)
	}
}

func TestAppSetupAvailabilitySetCommand_MissingFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing app", args: []string{"--territory", "USA", "--available", "true", "--available-in-new-territories", "true"}},
		{name: "missing territory", args: []string{"--app", "APP", "--available", "true", "--available-in-new-territories", "true"}},
		{name: "invalid territory csv", args: []string{"--app", "APP", "--territory", ",,,", "--available", "true", "--available-in-new-territories", "true"}},
		{name: "missing available", args: []string{"--app", "APP", "--territory", "USA", "--available-in-new-territories", "true"}},
		{name: "missing available in new territories", args: []string{"--app", "APP", "--territory", "USA", "--available", "true"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := AppSetupAvailabilitySetCommand()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
	}
}

func TestAppSetupAvailabilitySetCommand_HelpMentionsAllTerritories(t *testing.T) {
	cmd := AppSetupAvailabilitySetCommand()

	if !strings.Contains(cmd.LongHelp, "--all-territories") {
		t.Fatalf("expected --all-territories example in long help, got %q", cmd.LongHelp)
	}
}

func TestAppSetupPricingSetCommand_MissingFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing app", args: []string{"--price-point", "PP"}},
		{name: "missing price point", args: []string{"--app", "APP"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := AppSetupPricingSetCommand()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
	}
}

func TestAppSetupPricingSetCommand_HelpMentionsFreeExample(t *testing.T) {
	cmd := AppSetupPricingSetCommand()

	if !strings.Contains(cmd.LongHelp, "--free") {
		t.Fatalf("expected --free example in long help, got %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.FlagSet.Lookup("tier").Usage, "--free") {
		t.Fatalf("expected --tier help to mention --free, got %q", cmd.FlagSet.Lookup("tier").Usage)
	}
	if !strings.Contains(cmd.FlagSet.Lookup("price").Usage, "--free") {
		t.Fatalf("expected --price help to mention --free, got %q", cmd.FlagSet.Lookup("price").Usage)
	}
}

func TestAppSetupLocalizationsUploadCommand_MissingPath(t *testing.T) {
	cmd := AppSetupLocalizationsUploadCommand()

	if err := cmd.FlagSet.Parse([]string{"--version", "VERSION_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --path is missing, got %v", err)
	}
}

func TestAppSetupCommands_DefaultOutputJSON(t *testing.T) {
	commands := []*struct {
		name string
		cmd  func() *ffcli.Command
	}{
		{"info set", AppSetupInfoSetCommand},
		{"categories set", AppSetupCategoriesSetCommand},
		{"availability set", AppSetupAvailabilitySetCommand},
		{"pricing set", AppSetupPricingSetCommand},
		{"localizations upload", AppSetupLocalizationsUploadCommand},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd()
			f := cmd.FlagSet.Lookup("output")
			if f == nil {
				t.Fatalf("expected --output flag to be defined")
			}
			if f.DefValue != "json" {
				t.Fatalf("expected --output default to be 'json', got %q", f.DefValue)
			}
		})
	}
}
