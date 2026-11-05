package pricing

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestPricingPricePointsCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	cmd := PricingPricePointsCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestPricingPricePointsGetCommand_MissingPricePoint(t *testing.T) {
	cmd := PricingPricePointsGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --price-point is missing, got %v", err)
	}
}

func TestPricingScheduleManualPricesCommand_MissingSchedule(t *testing.T) {
	cmd := PricingScheduleManualPricesCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --schedule is missing, got %v", err)
	}
}

func TestPricingScheduleAutomaticPricesCommand_MissingSchedule(t *testing.T) {
	cmd := PricingScheduleAutomaticPricesCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --schedule is missing, got %v", err)
	}
}

func TestPricingSchedulePriceCommands_HelpMentionsResolved(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  func() *ffcli.Command
	}{
		{name: "manual", cmd: PricingScheduleManualPricesCommand},
		{name: "automatic", cmd: PricingScheduleAutomaticPricesCommand},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd()
			if cmd.FlagSet.Lookup("resolved") == nil {
				t.Fatalf("expected --resolved flag")
			}
			if !strings.Contains(cmd.LongHelp, "--resolved") {
				t.Fatalf("expected long help to mention --resolved, got %q", cmd.LongHelp)
			}
		})
	}
}

func TestPricingScheduleCreateCommand_MissingFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing app", args: []string{"--price-point", "PP", "--base-territory", "USA", "--start-date", "2024-03-01"}},
		{name: "missing price point", args: []string{"--app", "APP", "--base-territory", "USA", "--start-date", "2024-03-01"}},
		{name: "missing base territory", args: []string{"--app", "APP", "--price-point", "PP", "--start-date", "2024-03-01"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := PricingScheduleCreateCommand()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
	}
}

func TestPricingScheduleCreateCommand_MutuallyExclusivePriceInputs(t *testing.T) {
	cmd := PricingScheduleCreateCommand()

	if err := cmd.FlagSet.Parse([]string{
		"--app", "APP",
		"--price-point", "PP",
		"--price", "0.99",
		"--base-territory", "USA",
		"--start-date", "2024-03-01",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --price-point and --price are both set, got %v", err)
	}
}

func TestPricingScheduleCreateCommand_InvalidPriceValue(t *testing.T) {
	tests := []string{"abc", "NaN"}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			cmd := PricingScheduleCreateCommand()

			if err := cmd.FlagSet.Parse([]string{
				"--app", "APP",
				"--price", value,
				"--base-territory", "USA",
				"--start-date", "2024-03-01",
			}); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp for invalid --price value %q, got %v", value, err)
			}
		})
	}
}

func TestPricingScheduleCreateCommand_InvalidDate(t *testing.T) {
	cmd := PricingScheduleCreateCommand()

	if err := cmd.FlagSet.Parse([]string{"--app", "APP", "--price-point", "PP", "--base-territory", "USA", "--start-date", "invalid"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for invalid start date")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error (flag.ErrHelp) for invalid start date, got %v", err)
	}
}

func TestPricingScheduleCreateCommand_HelpMentionsFreeExample(t *testing.T) {
	cmd := PricingScheduleCreateCommand()

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

func TestPricingAvailabilityTerritoryAvailabilitiesCommand_MissingAvailability(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	cmd := PricingAvailabilityTerritoryAvailabilitiesCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --availability is missing, got %v", err)
	}
}

func TestPricingAvailabilitySetCommand_MissingFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing app", args: []string{"--territory", "USA", "--available", "true", "--available-in-new-territories", "true"}},
		{name: "missing territory", args: []string{"--app", "APP", "--available", "true", "--available-in-new-territories", "true"}},
		{name: "invalid territory csv", args: []string{"--app", "APP", "--territory", ",,,", "--available", "true", "--available-in-new-territories", "true"}},
		{name: "missing available", args: []string{"--app", "APP", "--territory", "USA", "--available-in-new-territories", "true"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := PricingAvailabilitySetCommand()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
	}
}

func capturePricingStderr(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	originalStderr := os.Stderr
	os.Stderr = writer
	defer func() {
		os.Stderr = originalStderr
	}()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return string(output)
}

func TestPricingAvailabilityCreateCommand_MissingFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "missing app", args: []string{"--territory", "USA", "--available", "true", "--available-in-new-territories", "true"}, wantStderr: "--app is required"},
		{name: "missing territory", args: []string{"--app", "APP", "--available", "true", "--available-in-new-territories", "true"}, wantStderr: "--territory or --all-territories is required"},
		{name: "territory and all territories", args: []string{"--app", "APP", "--territory", "USA", "--all-territories", "--available", "true", "--available-in-new-territories", "true"}, wantStderr: "--territory and --all-territories are mutually exclusive"},
		{name: "invalid territory csv", args: []string{"--app", "APP", "--territory", ",,,", "--available", "true", "--available-in-new-territories", "true"}, wantStderr: "--territory must include at least one value"},
		{name: "missing available", args: []string{"--app", "APP", "--territory", "USA", "--available-in-new-territories", "true"}, wantStderr: "--available is required"},
		{name: "missing available in new territories", args: []string{"--app", "APP", "--territory", "USA", "--available", "true"}, wantStderr: "--available-in-new-territories is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := PricingAvailabilityCreateCommand()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			stderr := capturePricingStderr(t, func() {
				if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantStderr, stderr)
			}
		})
	}
}

func TestPricingAvailabilitySetCommand_HasAvailableInNewTerritoriesFlag(t *testing.T) {
	cmd := PricingAvailabilitySetCommand()

	if f := cmd.FlagSet.Lookup("available-in-new-territories"); f == nil {
		t.Fatal("expected --available-in-new-territories flag to be defined")
	}
}

func TestPricingAvailabilityCommandRegistersSubcommands(t *testing.T) {
	registered := map[string]bool{}
	for _, subcommand := range PricingAvailabilityCommand().Subcommands {
		registered[subcommand.Name] = true
	}
	for _, name := range []string{"create", "remove-from-sale", "platforms"} {
		if !registered[name] {
			t.Errorf("expected pricing availability %s to be registered", name)
		}
	}
}

func TestPricingAvailabilityRemoveFromSaleCommand_MissingConfirmBeforeAuth(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	called := false
	originalFactory := pricingAvailabilityClientFactory
	pricingAvailabilityClientFactory = func() (*asc.Client, error) {
		called = true
		return nil, errors.New("unexpected auth")
	}
	t.Cleanup(func() { pricingAvailabilityClientFactory = originalFactory })

	cmd := PricingAvailabilityRemoveFromSaleCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "APP"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stderr := capturePricingStderr(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if called {
		t.Fatal("client factory called before --confirm validation")
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected confirmation diagnostic, got %q", stderr)
	}
}

func TestPricingAvailabilitySetCommand_HelpMentionsAllTerritories(t *testing.T) {
	cmd := PricingAvailabilitySetCommand()

	if !strings.Contains(cmd.LongHelp, "--all-territories") {
		t.Fatalf("expected --all-territories example in long help, got %q", cmd.LongHelp)
	}
}

func TestPricingCommands_DefaultOutputJSON(t *testing.T) {
	commands := []*struct {
		name string
		cmd  func() *ffcli.Command
	}{
		{"current", PricingCurrentCommand},
		{"territories list", PricingTerritoriesListCommand},
		{"tiers", PricingTiersCommand},
		{"price-points", PricingPricePointsCommand},
		{"price-points get", PricingPricePointsGetCommand},
		{"price-points equalizations", PricingPricePointsEqualizationsCommand},
		{"schedule get", PricingScheduleGetCommand},
		{"schedule create", PricingScheduleCreateCommand},
		{"schedule manual-prices", PricingScheduleManualPricesCommand},
		{"schedule automatic-prices", PricingScheduleAutomaticPricesCommand},
		{"availability get", PricingAvailabilityGetCommand},
		{"availability create", PricingAvailabilityCreateCommand},
		{"availability territory-availabilities", PricingAvailabilityTerritoryAvailabilitiesCommand},
		{"availability set", PricingAvailabilitySetCommand},
		{"availability remove-from-sale", PricingAvailabilityRemoveFromSaleCommand},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd()
			f := cmd.FlagSet.Lookup("output")
			if f == nil {
				t.Fatalf("expected --output flag to be defined")
				return
			}
			if f.DefValue != "json" {
				t.Fatalf("expected --output default to be 'json', got %q", f.DefValue)
			}
		})
	}
}
