package iap

import (
	"context"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const iapPricingCanonicalRoot = "asc iap pricing"

// IAPPricingCommand returns the canonical pricing command tree for IAPs.
func IAPPricingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pricing",
		ShortUsage: "asc iap pricing <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase pricing workflows.",
		LongHelp: `Manage in-app purchase pricing workflows.

Examples:
  asc iap pricing summary --app "APP_ID"
  asc iap pricing summary --iap-id "IAP_ID"
  asc iap pricing price-points list --iap-id "IAP_ID"
  asc iap pricing schedules get --iap-id "IAP_ID"
  asc iap pricing availability get --iap-id "IAP_ID"
  asc iap pricing availabilities get --id "AVAILABILITY_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPPricesCommand(),
			IAPPricePointsCommand(),
			IAPPriceSchedulesCommand(),
			IAPAvailabilityCommand(),
			IAPAvailabilitiesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func deprecatedIAPPricingGroupAlias(name, shortUsage, canonicalCommand string, subcommands []*ffcli.Command) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	return &ffcli.Command{
		Name:        name,
		ShortUsage:  shortUsage,
		ShortHelp:   fmt.Sprintf("DEPRECATED: use `%s ...`.", canonicalCommand),
		LongHelp:    fmt.Sprintf("Compatibility alias: use `%s ...`.", canonicalCommand),
		FlagSet:     fs,
		UsageFunc:   shared.DeprecatedUsageFunc,
		Subcommands: subcommands,
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func DeprecatedIAPPricesAliasCommand() *ffcli.Command {
	return shared.DeprecatedAliasLeafCommand(
		IAPPricesCommand(),
		"prices",
		iapPricingCanonicalRoot+" summary [flags]",
		iapPricingCanonicalRoot+" summary",
		"Warning: `asc iap prices` is deprecated. Use `asc iap pricing summary`.",
	)
}

func DeprecatedIAPPricePointsAliasCommand() *ffcli.Command {
	return deprecatedIAPPricingGroupAlias(
		"price-points",
		iapPricingCanonicalRoot+" price-points <subcommand> [flags]",
		iapPricingCanonicalRoot+" price-points",
		[]*ffcli.Command{
			shared.DeprecatedAliasLeafCommand(
				IAPPricePointsListCommand(),
				"list",
				iapPricingCanonicalRoot+" price-points list --iap-id \"IAP_ID\"",
				iapPricingCanonicalRoot+" price-points list",
				"Warning: `asc iap price-points list` is deprecated. Use `asc iap pricing price-points list`.",
			),
			shared.DeprecatedAliasLeafCommand(
				IAPPricePointsEqualizationsCommand(),
				"equalizations",
				iapPricingCanonicalRoot+" price-points equalizations --id \"PRICE_POINT_ID\"",
				iapPricingCanonicalRoot+" price-points equalizations",
				"Warning: `asc iap price-points equalizations` is deprecated. Use `asc iap pricing price-points equalizations`.",
			),
		},
	)
}

func DeprecatedIAPPriceSchedulesAliasCommand() *ffcli.Command {
	return deprecatedIAPPricingGroupAlias(
		"price-schedules",
		iapPricingCanonicalRoot+" schedules <subcommand> [flags]",
		iapPricingCanonicalRoot+" schedules",
		[]*ffcli.Command{
			shared.DeprecatedAliasLeafCommand(
				IAPPriceSchedulesGetCommand(),
				"get",
				iapPricingCanonicalRoot+" schedules get --iap-id \"IAP_ID\"",
				iapPricingCanonicalRoot+" schedules get",
				"Warning: `asc iap price-schedules get` is deprecated. Use `asc iap pricing schedules get`.",
			),
			shared.DeprecatedAliasLeafCommand(
				IAPPriceSchedulesBaseTerritoryCommand(),
				"base-territory",
				iapPricingCanonicalRoot+" schedules base-territory --schedule-id \"SCHEDULE_ID\"",
				iapPricingCanonicalRoot+" schedules base-territory",
				"Warning: `asc iap price-schedules base-territory` is deprecated. Use `asc iap pricing schedules base-territory`.",
			),
			shared.DeprecatedAliasLeafCommand(
				IAPPriceSchedulesCreateCommand(),
				"create",
				iapPricingCanonicalRoot+" schedules create --iap-id \"IAP_ID\" --base-territory \"USA\" --prices \"PRICE_POINT_ID:2024-03-01\"",
				iapPricingCanonicalRoot+" schedules create",
				"Warning: `asc iap price-schedules create` is deprecated. Use `asc iap pricing schedules create`.",
			),
			shared.DeprecatedAliasLeafCommand(
				IAPPriceSchedulesManualPricesCommand(),
				"manual-prices",
				iapPricingCanonicalRoot+" schedules manual-prices --schedule-id \"SCHEDULE_ID\"",
				iapPricingCanonicalRoot+" schedules manual-prices",
				"Warning: `asc iap price-schedules manual-prices` is deprecated. Use `asc iap pricing schedules manual-prices`.",
			),
			shared.DeprecatedAliasLeafCommand(
				IAPPriceSchedulesAutomaticPricesCommand(),
				"automatic-prices",
				iapPricingCanonicalRoot+" schedules automatic-prices --schedule-id \"SCHEDULE_ID\"",
				iapPricingCanonicalRoot+" schedules automatic-prices",
				"Warning: `asc iap price-schedules automatic-prices` is deprecated. Use `asc iap pricing schedules automatic-prices`.",
			),
		},
	)
}

func DeprecatedIAPAvailabilityAliasCommand() *ffcli.Command {
	return deprecatedIAPPricingGroupAlias(
		"availability",
		iapPricingCanonicalRoot+" availability <subcommand> [flags]",
		iapPricingCanonicalRoot+" availability",
		[]*ffcli.Command{
			shared.DeprecatedAliasLeafCommand(
				IAPAvailabilityGetCommand(),
				"get",
				iapPricingCanonicalRoot+" availability get --iap-id \"IAP_ID\"",
				iapPricingCanonicalRoot+" availability get",
				"Warning: `asc iap availability get` is deprecated. Use `asc iap pricing availability get`.",
			),
			shared.DeprecatedAliasLeafCommand(
				IAPAvailabilitySetCommand(),
				"set",
				iapPricingCanonicalRoot+" availability set --iap-id \"IAP_ID\" --territories \"USA,CAN\"",
				iapPricingCanonicalRoot+" availability set",
				"Warning: `asc iap availability set` is deprecated. Use `asc iap pricing availability set`.",
			),
		},
	)
}

func DeprecatedIAPAvailabilitiesAliasCommand() *ffcli.Command {
	return deprecatedIAPPricingGroupAlias(
		"availabilities",
		iapPricingCanonicalRoot+" availabilities <subcommand> [flags]",
		iapPricingCanonicalRoot+" availabilities",
		[]*ffcli.Command{
			shared.DeprecatedAliasLeafCommand(
				IAPAvailabilitiesGetCommand(),
				"get",
				iapPricingCanonicalRoot+" availabilities get --id \"AVAILABILITY_ID\"",
				iapPricingCanonicalRoot+" availabilities get",
				"Warning: `asc iap availabilities get` is deprecated. Use `asc iap pricing availabilities get`.",
			),
			shared.DeprecatedAliasLeafCommand(
				IAPAvailabilitiesAvailableTerritoriesCommand(),
				"available-territories",
				iapPricingCanonicalRoot+" availabilities available-territories --id \"AVAILABILITY_ID\"",
				iapPricingCanonicalRoot+" availabilities available-territories",
				"Warning: `asc iap availabilities available-territories` is deprecated. Use `asc iap pricing availabilities available-territories`.",
			),
		},
	)
}
