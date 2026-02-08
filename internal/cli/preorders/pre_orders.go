package preorders

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// PreOrdersCommand returns the pre-orders command group.
func PreOrdersCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "pre-orders",
		ShortUsage: "asc pre-orders <subcommand> [flags]",
		ShortHelp:  "Manage app pre-orders.",
		LongHelp: `Manage app pre-orders.

Examples:
  asc pre-orders get --app "123456789"
  asc pre-orders list --availability "AVAILABILITY_ID"
  asc pre-orders enable --app "123456789" --territory "USA,GBR" --release-date "2026-02-01"
  asc pre-orders update --territory-availability "TERRITORY_AVAILABILITY_ID" --release-date "2026-03-01"
  asc pre-orders disable --territory-availability "TERRITORY_AVAILABILITY_ID"
  asc pre-orders end --territory-availability "TA_1,TA_2"`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PreOrdersGetCommand(),
			PreOrdersListCommand(),
			PreOrdersEnableCommand(),
			PreOrdersUpdateCommand(),
			PreOrdersDisableCommand(),
			PreOrdersEndCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PreOrdersGetCommand returns the get subcommand.
func PreOrdersGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-orders get", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc pre-orders get [flags]",
		ShortHelp:  "Get app pre-order availability.",
		LongHelp: `Get app pre-order availability.

Examples:
  asc pre-orders get --app "123456789"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pre-orders get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppAvailabilityV2(requestCtx, resolvedAppID)
			if err != nil {
				if shared.IsAppAvailabilityMissing(err) {
					return fmt.Errorf("pre-orders get: app availability not found for app %q", resolvedAppID)
				}
				return fmt.Errorf("pre-orders get: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// PreOrdersListCommand returns the list subcommand.
func PreOrdersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-orders list", flag.ExitOnError)

	availabilityID := fs.String("availability", "", "App availability ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc pre-orders list --availability AVAILABILITY_ID",
		ShortHelp:  "List territory availabilities for pre-orders.",
		LongHelp: `List territory availabilities for pre-orders.

Examples:
  asc pre-orders list --availability "AVAILABILITY_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAvailabilityID := strings.TrimSpace(*availabilityID)
			if trimmedAvailabilityID == "" {
				fmt.Fprintln(os.Stderr, "Error: --availability is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pre-orders list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetTerritoryAvailabilities(requestCtx, trimmedAvailabilityID)
			if err != nil {
				return fmt.Errorf("pre-orders list: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// PreOrdersEnableCommand returns the enable subcommand.
func PreOrdersEnableCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-orders enable", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	territory := fs.String("territory", "", "Territory IDs (comma-separated, e.g., USA,GBR)")
	releaseDate := fs.String("release-date", "", "Release date (YYYY-MM-DD)")
	var availableInNewTerritories shared.OptionalBool
	fs.Var(&availableInNewTerritories, "available-in-new-territories", "Set available-in-new-territories: true or false")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "enable",
		ShortUsage: "asc pre-orders enable [flags]",
		ShortHelp:  "Enable pre-orders for territories.",
		LongHelp: `Enable pre-orders for territories.

Examples:
  asc pre-orders enable --app "123456789" --territory "USA,GBR" --release-date "2026-02-01"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*territory) == "" {
				fmt.Fprintln(os.Stderr, "Error: --territory is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*releaseDate) == "" {
				fmt.Fprintln(os.Stderr, "Error: --release-date is required")
				return flag.ErrHelp
			}
			if !availableInNewTerritories.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --available-in-new-territories is required")
				return flag.ErrHelp
			}

			normalizedReleaseDate, err := normalizePreOrderReleaseDate(*releaseDate)
			if err != nil {
				return fmt.Errorf("pre-orders enable: %w", err)
			}

			territories := shared.SplitCSVUpper(*territory)
			if len(territories) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --territory must include at least one value")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pre-orders enable: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			availabilityResp, err := client.GetAppAvailabilityV2(requestCtx, resolvedAppID)
			if err != nil {
				if shared.IsAppAvailabilityMissing(err) {
					return fmt.Errorf("pre-orders enable: app availability not found for app %q", resolvedAppID)
				}
				return fmt.Errorf("pre-orders enable: %w", err)
			}
			availabilityID := strings.TrimSpace(availabilityResp.Data.ID)
			if availabilityID == "" {
				return fmt.Errorf("pre-orders enable: app availability ID missing from response")
			}
			availableInNew := availableInNewTerritories.Value()
			createResp, err := client.CreateAppAvailabilityV2(requestCtx, resolvedAppID, asc.AppAvailabilityV2CreateAttributes{
				AvailableInNewTerritories: &availableInNew,
			})
			if err != nil {
				return fmt.Errorf("pre-orders enable: %w", err)
			}
			availabilityID = strings.TrimSpace(createResp.Data.ID)
			if availabilityID == "" {
				return fmt.Errorf("pre-orders enable: app availability ID missing from response")
			}

			firstPage, err := client.GetTerritoryAvailabilities(requestCtx, availabilityID, asc.WithTerritoryAvailabilitiesLimit(200))
			if err != nil {
				return fmt.Errorf("pre-orders enable: %w", err)
			}
			paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetTerritoryAvailabilities(ctx, availabilityID, asc.WithTerritoryAvailabilitiesNextURL(nextURL))
			})
			if err != nil {
				return fmt.Errorf("pre-orders enable: %w", err)
			}
			territoryResp, ok := paginated.(*asc.TerritoryAvailabilitiesResponse)
			if !ok {
				return fmt.Errorf("pre-orders enable: unexpected territory availabilities response")
			}

			territoryMap, err := mapTerritoryAvailabilityIDs(territoryResp)
			if err != nil {
				return fmt.Errorf("pre-orders enable: %w", err)
			}

			missingTerritories := make([]string, 0)
			territoryAvailabilityIDs := make([]string, 0, len(territories))
			for _, territoryID := range territories {
				territoryAvailabilityID := territoryMap[territoryID]
				if territoryAvailabilityID == "" {
					missingTerritories = append(missingTerritories, territoryID)
					continue
				}
				territoryAvailabilityIDs = append(territoryAvailabilityIDs, territoryAvailabilityID)
			}
			if len(missingTerritories) > 0 {
				return fmt.Errorf("pre-orders enable: territory availability not found for territories: %s", strings.Join(missingTerritories, ", "))
			}

			preOrderEnabled := true
			available := true
			updated := make([]asc.Resource[asc.TerritoryAvailabilityAttributes], 0, len(territoryAvailabilityIDs))
			for _, territoryAvailabilityID := range territoryAvailabilityIDs {
				updateResp, err := client.UpdateTerritoryAvailability(requestCtx, territoryAvailabilityID, asc.TerritoryAvailabilityUpdateAttributes{
					Available:       &available,
					ReleaseDate:     &normalizedReleaseDate,
					PreOrderEnabled: &preOrderEnabled,
				})
				if err != nil {
					return fmt.Errorf("pre-orders enable: %w", err)
				}
				updated = append(updated, updateResp.Data)
			}

			return shared.PrintOutput(&asc.TerritoryAvailabilitiesResponse{Data: updated}, *output, *pretty)
		},
	}
}

// PreOrdersUpdateCommand returns the update subcommand.
func PreOrdersUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-orders update", flag.ExitOnError)

	territoryAvailabilityID := fs.String("territory-availability", "", "Territory availability ID")
	releaseDate := fs.String("release-date", "", "Release date (YYYY-MM-DD)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc pre-orders update --territory-availability TERRITORY_AVAILABILITY_ID [flags]",
		ShortHelp:  "Update pre-order release date for a territory availability.",
		LongHelp: `Update pre-order release date for a territory availability.

Examples:
  asc pre-orders update --territory-availability "TERRITORY_AVAILABILITY_ID" --release-date "2026-03-01"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*territoryAvailabilityID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --territory-availability is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*releaseDate) == "" {
				fmt.Fprintln(os.Stderr, "Error: --release-date is required")
				return flag.ErrHelp
			}

			normalizedReleaseDate, err := normalizePreOrderReleaseDate(*releaseDate)
			if err != nil {
				return fmt.Errorf("pre-orders update: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pre-orders update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateTerritoryAvailability(requestCtx, trimmedID, asc.TerritoryAvailabilityUpdateAttributes{
				ReleaseDate: &normalizedReleaseDate,
			})
			if err != nil {
				return fmt.Errorf("pre-orders update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// PreOrdersDisableCommand returns the disable subcommand.
func PreOrdersDisableCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-orders disable", flag.ExitOnError)

	territoryAvailabilityID := fs.String("territory-availability", "", "Territory availability ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "disable",
		ShortUsage: "asc pre-orders disable --territory-availability TERRITORY_AVAILABILITY_ID",
		ShortHelp:  "Disable pre-orders for a territory availability.",
		LongHelp: `Disable pre-orders for a territory availability.

Examples:
  asc pre-orders disable --territory-availability "TERRITORY_AVAILABILITY_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*territoryAvailabilityID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --territory-availability is required")
				return flag.ErrHelp
			}

			preOrderEnabled := false
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pre-orders disable: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateTerritoryAvailability(requestCtx, trimmedID, asc.TerritoryAvailabilityUpdateAttributes{
				PreOrderEnabled: &preOrderEnabled,
			})
			if err != nil {
				return fmt.Errorf("pre-orders disable: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// PreOrdersEndCommand returns the end subcommand.
func PreOrdersEndCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-orders end", flag.ExitOnError)

	territoryAvailabilityIDs := fs.String("territory-availability", "", "Territory availability IDs (comma-separated)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "end",
		ShortUsage: "asc pre-orders end --territory-availability TERRITORY_AVAILABILITY_ID[,ID...]",
		ShortHelp:  "End pre-orders for territory availabilities.",
		LongHelp: `End pre-orders for territory availabilities.

Examples:
  asc pre-orders end --territory-availability "TA_1,TA_2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ids := shared.SplitCSV(*territoryAvailabilityIDs)
			if len(ids) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --territory-availability is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pre-orders end: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.EndAppAvailabilityPreOrders(requestCtx, ids)
			if err != nil {
				return fmt.Errorf("pre-orders end: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

func normalizePreOrderReleaseDate(value string) (string, error) {
	return shared.NormalizeDate(value, "--release-date")
}

type territoryAvailabilityIDPayload struct {
	Territory string `json:"t"`
}

func mapTerritoryAvailabilityIDs(resp *asc.TerritoryAvailabilitiesResponse) (map[string]string, error) {
	if resp == nil {
		return nil, fmt.Errorf("territory availabilities response is nil")
	}
	ids := make(map[string]string, len(resp.Data))
	for _, item := range resp.Data {
		territoryID := ""
		if len(item.Relationships) > 0 {
			var relationships asc.TerritoryAvailabilityRelationships
			if err := json.Unmarshal(item.Relationships, &relationships); err != nil {
				return nil, fmt.Errorf("decode territory availability relationships for %q: %w", item.ID, err)
			}
			territoryID = strings.ToUpper(strings.TrimSpace(relationships.Territory.Data.ID))
		}
		if territoryID == "" {
			var ok bool
			territoryID, ok = territoryIDFromAvailabilityID(item.ID)
			if !ok {
				return nil, fmt.Errorf("territory availability %q missing territory id", item.ID)
			}
		}
		ids[territoryID] = item.ID
	}
	return ids, nil
}

func territoryIDFromAvailabilityID(availabilityID string) (string, bool) {
	trimmed := strings.TrimSpace(availabilityID)
	if trimmed == "" {
		return "", false
	}
	decoded, err := base64.RawStdEncoding.DecodeString(trimmed)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(trimmed)
		if err != nil {
			decoded, err = base64.RawURLEncoding.DecodeString(trimmed)
			if err != nil {
				decoded, err = base64.URLEncoding.DecodeString(trimmed)
				if err != nil {
					return "", false
				}
			}
		}
	}
	var payload territoryAvailabilityIDPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return "", false
	}
	territoryID := strings.TrimSpace(payload.Territory)
	if territoryID == "" {
		return "", false
	}
	return strings.ToUpper(territoryID), true
}
