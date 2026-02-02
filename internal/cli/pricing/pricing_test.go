package pricing

import (
	"context"
	"flag"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestPricingPricePointsCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	cmd := PricingPricePointsCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestPricingPricePointsGetCommand_MissingPricePoint(t *testing.T) {
	cmd := PricingPricePointsGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --price-point is missing, got %v", err)
	}
}

func TestPricingPricePointsEqualizationsCommand_MissingPricePoint(t *testing.T) {
	cmd := PricingPricePointsEqualizationsCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --price-point is missing, got %v", err)
	}
}

func TestPricingScheduleGetCommand_MissingAppAndID(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	cmd := PricingScheduleGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestPricingScheduleGetCommand_MutuallyExclusive(t *testing.T) {
	cmd := PricingScheduleGetCommand()

	if err := cmd.FlagSet.Parse([]string{"--app", "APP_ID", "--id", "SCHEDULE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --app and --id are both set, got %v", err)
	}
}

func TestPricingScheduleManualPricesCommand_MissingSchedule(t *testing.T) {
	cmd := PricingScheduleManualPricesCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --schedule is missing, got %v", err)
	}
}

func TestPricingScheduleAutomaticPricesCommand_MissingSchedule(t *testing.T) {
	cmd := PricingScheduleAutomaticPricesCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --schedule is missing, got %v", err)
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
		{name: "missing start date", args: []string{"--app", "APP", "--price-point", "PP", "--base-territory", "USA"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := PricingScheduleCreateCommand()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
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
	if err == flag.ErrHelp {
		t.Fatal("expected non-ErrHelp error for invalid start date")
	}
}

func TestPricingAvailabilityGetCommand_MissingAppAndID(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	cmd := PricingAvailabilityGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestPricingAvailabilityGetCommand_MutuallyExclusive(t *testing.T) {
	cmd := PricingAvailabilityGetCommand()

	if err := cmd.FlagSet.Parse([]string{"--app", "APP_ID", "--id", "AVAILABILITY_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --app and --id are both set, got %v", err)
	}
}

func TestPricingAvailabilityTerritoryAvailabilitiesCommand_MissingAvailability(t *testing.T) {
	cmd := PricingAvailabilityTerritoryAvailabilitiesCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --availability is missing, got %v", err)
	}
}

func TestPricingAvailabilitySetCommand_MissingFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing app", args: []string{"--territory", "USA", "--available", "true"}},
		{name: "missing territory", args: []string{"--app", "APP", "--available", "true"}},
		{name: "missing available", args: []string{"--app", "APP", "--territory", "USA"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := PricingAvailabilitySetCommand()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
	}
}

func TestPricingCommands_DefaultOutputJSON(t *testing.T) {
	commands := []*struct {
		name string
		cmd  func() *ffcli.Command
	}{
		{"territories list", PricingTerritoriesListCommand},
		{"price-points", PricingPricePointsCommand},
		{"price-points get", PricingPricePointsGetCommand},
		{"price-points equalizations", PricingPricePointsEqualizationsCommand},
		{"schedule get", PricingScheduleGetCommand},
		{"schedule create", PricingScheduleCreateCommand},
		{"schedule manual-prices", PricingScheduleManualPricesCommand},
		{"schedule automatic-prices", PricingScheduleAutomaticPricesCommand},
		{"availability get", PricingAvailabilityGetCommand},
		{"availability territory-availabilities", PricingAvailabilityTerritoryAvailabilitiesCommand},
		{"availability set", PricingAvailabilitySetCommand},
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
