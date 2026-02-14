package shared

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// AvailabilitySetCommandConfig configures the availability set command.
type AvailabilitySetCommandConfig struct {
	FlagSetName                      string
	CommandName                      string
	ShortUsage                       string
	ShortHelp                        string
	LongHelp                         string
	ErrorPrefix                      string
	IncludeAvailableInNewTerritories bool
}

// NewAvailabilitySetCommand builds a shared availability set command.
func NewAvailabilitySetCommand(config AvailabilitySetCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	territory := fs.String("territory", "", "Territory IDs (comma-separated, e.g., USA,GBR)")
	var available OptionalBool
	fs.Var(&available, "available", "Set availability: true or false")
	var availableInNewTerritories OptionalBool
	if config.IncludeAvailableInNewTerritories {
		fs.Var(&availableInNewTerritories, "available-in-new-territories", "Set availability for new territories: true or false")
	}
	output := BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       config.CommandName,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*territory) == "" {
				fmt.Fprintln(os.Stderr, "Error: --territory is required")
				return flag.ErrHelp
			}
			if !available.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --available is required (true or false)")
				return flag.ErrHelp
			}
			if config.IncludeAvailableInNewTerritories && !availableInNewTerritories.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --available-in-new-territories is required (true or false)")
				return flag.ErrHelp
			}

			territories := splitCSVUpper(*territory)
			if len(territories) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --territory must include at least one value")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			availabilities := make([]asc.TerritoryAvailabilityCreate, 0, len(territories))
			for _, territoryID := range territories {
				availabilities = append(availabilities, asc.TerritoryAvailabilityCreate{
					TerritoryID: territoryID,
					Available:   available.Value(),
				})
			}

			attributes := asc.AppAvailabilityV2CreateAttributes{
				TerritoryAvailabilities: availabilities,
			}
			if config.IncludeAvailableInNewTerritories {
				availableInNewTerritoriesValue := availableInNewTerritories.Value()
				attributes.AvailableInNewTerritories = &availableInNewTerritoriesValue
			}

			resp, err := client.CreateAppAvailabilityV2(requestCtx, resolvedAppID, attributes)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			return printOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
