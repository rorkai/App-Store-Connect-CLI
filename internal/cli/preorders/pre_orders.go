package preorders

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewPreOrdersCommand returns the pre-orders command group built on d.
func NewPreOrdersCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "pre-orders",
		ShortUsage: "asc pre-orders <subcommand> [flags]",
		ShortHelp:  "Manage app pre-orders.",
		LongHelp: `Manage app pre-orders.

Examples:
  asc pre-orders view --app "123456789"
  asc pre-orders list --availability "AVAILABILITY_ID"
  asc pre-orders enable --app "123456789" --territory "US,France" --release-date "2026-06-01"
  asc pre-orders update --territory-availability "TERRITORY_AVAILABILITY_ID" --pre-order-enabled true --release-date "2026-03-01"
  asc pre-orders disable --territory-availability "TERRITORY_AVAILABILITY_ID"
  asc pre-orders end --territory-availability "TA_1,TA_2" --confirm`,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPreOrdersGetCommand(d),
			newPreOrdersListCommand(d),
			newPreOrdersEnableCommand(d),
			newPreOrdersUpdateCommand(d),
			newPreOrdersDisableCommand(d),
			newPreOrdersEndCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newPreOrdersGetCommand returns the get subcommand.
func newPreOrdersGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "pre-orders view",
		Name:        "view",
		ShortUsage:  "asc pre-orders view [flags]",
		ShortHelp:   "View app pre-order availability.",
		LongHelp: `View app pre-order availability.

Examples:
  asc pre-orders view --app "123456789"`,
		IDFlag:             "app",
		IDUsage:            "App Store Connect app ID (or ASC_APP_ID)",
		ResolveApp:         true,
		RequiredDiagnostic: true,
		FetchErrorPrefix:   "pre-orders view",
		Deps:               d,
		Fetch: func(ctx context.Context, client *asc.Client, appID string) (any, error) {
			resp, err := client.GetAppAvailabilityV2(ctx, appID)
			if shared.IsAppAvailabilityMissing(err) {
				return nil, fmt.Errorf("app availability not found for app %q", appID)
			}
			return resp, err
		},
	})
}

// newPreOrdersListCommand returns the list subcommand.
func newPreOrdersListCommand(d *shared.Deps) *ffcli.Command {
	cmd := shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "pre-orders list",
		Name:        "list",
		ShortUsage:  "asc pre-orders list --availability AVAILABILITY_ID [--limit N] [--next URL] [--paginate]",
		ShortHelp:   "List territory availabilities for pre-orders.",
		LongHelp: `List territory availabilities for pre-orders.

Examples:
  asc pre-orders list --availability "AVAILABILITY_ID"
  asc pre-orders list --availability "AVAILABILITY_ID" --limit 175
  asc pre-orders list --availability "AVAILABILITY_ID" --paginate
  asc pre-orders list --next "NEXT_URL"`,
		ParentFlag:       "availability",
		ParentUsage:      "App availability ID",
		ParentType:       "appAvailabilities",
		LimitMax:         200,
		FetchErrorPrefix: "pre-orders list",
		Deps:             d,
		FetchPage: func(ctx context.Context, client *asc.Client, availabilityID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := make([]asc.TerritoryAvailabilitiesOption, 0, 2)
			if limit > 0 {
				opts = append(opts, asc.Limit[asc.TerritoryAvailabilitiesOption](limit))
			}
			if strings.TrimSpace(next) != "" {
				opts = append(opts, asc.NextURL[asc.TerritoryAvailabilitiesOption](next))
			}
			return client.GetTerritoryAvailabilities(ctx, availabilityID, opts...)
		},
	})

	originalExec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		err := originalExec(ctx, args)
		if err == nil || errors.Is(err, flag.ErrHelp) {
			return err
		}
		if isPreOrdersListUsageError(err) {
			return d.UsageError(err.Error())
		}
		return err
	}

	return cmd
}

func isPreOrdersListUsageError(err error) bool {
	message := err.Error()
	return strings.HasPrefix(message, "pre-orders list: --limit must be between 1 and ") ||
		strings.HasPrefix(message, "pre-orders list: --next ")
}

// newPreOrdersEnableCommand returns the enable subcommand.
func newPreOrdersEnableCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pre-orders enable", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	territory := fs.String("territory", "", "Territory inputs (comma-separated; accepts alpha-2, alpha-3, or exact English country names)")
	releaseDate := fs.String("release-date", "", "Release date (YYYY-MM-DD)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "enable",
		ShortUsage: "asc pre-orders enable --app \"APP_ID\" --territory \"US,France\" --release-date \"2026-06-01\"",
		ShortHelp:  "Enable pre-orders for territories.",
		LongHelp: `Enable pre-orders for territories.

Enables pre-orders on the specified territories by setting preOrderEnabled=true,
available=true, and the given release date on each territory availability.

Examples:
  asc pre-orders enable --app "123456789" --territory "United States" --release-date "2026-06-01"
  asc pre-orders enable --app "123456789" --territory "US,France,DEU" --release-date "2026-06-01"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			if strings.TrimSpace(*territory) == "" {
				return d.RequiredFlag("--territory")
			}
			if strings.TrimSpace(*releaseDate) == "" {
				return d.RequiredFlag("--release-date")
			}

			normalizedReleaseDate, err := normalizePreOrderReleaseDate(*releaseDate)
			if err != nil {
				return d.UsageErrorf("pre-orders enable: %v", err)
			}

			territories, err := shared.NormalizeASCTerritoryCSV(*territory)
			if err != nil {
				return d.UsageError(err.Error())
			}
			if len(territories) == 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: --territory must include at least one value")
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pre-orders enable: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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
			firstPage, err := client.GetTerritoryAvailabilities(requestCtx, availabilityID, asc.Limit[asc.TerritoryAvailabilitiesOption](200))
			if err != nil {
				return fmt.Errorf("pre-orders enable: %w", err)
			}
			paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetTerritoryAvailabilities(ctx, availabilityID, asc.NextURL[asc.TerritoryAvailabilitiesOption](nextURL))
			})
			if err != nil {
				return fmt.Errorf("pre-orders enable: %w", err)
			}
			territoryResp, ok := paginated.(*asc.TerritoryAvailabilitiesResponse)
			if !ok {
				return fmt.Errorf("pre-orders enable: unexpected territory availabilities response")
			}
			territoryMap, err := shared.MapTerritoryAvailabilityIDs(territoryResp)
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

			return d.PrintOutput(&asc.TerritoryAvailabilitiesResponse{Data: updated}, *output.Output, *output.Pretty)
		},
	}
}

// newPreOrdersUpdateCommand returns the update subcommand.
func newPreOrdersUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pre-orders update", flag.ExitOnError)

	territoryAvailabilityID := shared.BindResourceIDFlag(fs, "territory-availability", "territoryAvailabilities", "Territory availability ID")
	releaseDate := fs.String("release-date", "", "Release date (YYYY-MM-DD)")
	var preOrderEnabled shared.OptionalBool
	fs.Var(&preOrderEnabled, "pre-order-enabled", "Set pre-order enabled: true or false")
	var available shared.OptionalBool
	fs.Var(&available, "available", "Set territory available: true or false")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc pre-orders update --territory-availability TERRITORY_AVAILABILITY_ID [flags]",
		ShortHelp:  "Update pre-order settings for a territory availability.",
		LongHelp: `Update pre-order settings for a territory availability.

At least one of --release-date, --pre-order-enabled, or --available is required.

Examples:
  asc pre-orders update --territory-availability "TERRITORY_AVAILABILITY_ID" --pre-order-enabled true --release-date "2026-03-01"
  asc pre-orders update --territory-availability "TERRITORY_AVAILABILITY_ID" --pre-order-enabled false
  asc pre-orders update --territory-availability "TERRITORY_AVAILABILITY_ID" --available true`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*territoryAvailabilityID)
			if trimmedID == "" {
				return d.RequiredFlag("--territory-availability")
			}

			attrs := asc.TerritoryAvailabilityUpdateAttributes{}
			hasAttr := false

			if strings.TrimSpace(*releaseDate) != "" {
				normalizedReleaseDate, err := normalizePreOrderReleaseDate(*releaseDate)
				if err != nil {
					return d.UsageErrorf("pre-orders update: %v", err)
				}
				attrs.ReleaseDate = &normalizedReleaseDate
				hasAttr = true
			}
			if preOrderEnabled.IsSet() {
				v := preOrderEnabled.Value()
				attrs.PreOrderEnabled = &v
				hasAttr = true
				if !v {
					if attrs.ReleaseDate != nil {
						return d.UsageError("--release-date cannot be set when disabling pre-orders (releaseDate must be null)")
					}
					attrs.ClearReleaseDate = true
				}
			}
			if available.IsSet() {
				v := available.Value()
				attrs.Available = &v
				hasAttr = true
			}
			if !hasAttr {
				return d.UpdateFlagRequired(fs, releaseDate, &preOrderEnabled, &available)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pre-orders update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateTerritoryAvailability(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("pre-orders update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPreOrdersDisableCommand returns the disable subcommand.
func newPreOrdersDisableCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pre-orders disable", flag.ExitOnError)

	territoryAvailabilityID := shared.BindResourceIDFlag(fs, "territory-availability", "territoryAvailabilities", "Territory availability ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "disable",
		ShortUsage: "asc pre-orders disable --territory-availability TERRITORY_AVAILABILITY_ID",
		ShortHelp:  "Disable pre-orders for a territory availability.",
		LongHelp: `Disable pre-orders for a territory availability.

Examples:
  asc pre-orders disable --territory-availability "TERRITORY_AVAILABILITY_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*territoryAvailabilityID)
			if trimmedID == "" {
				return d.RequiredFlag("--territory-availability")
			}

			preOrderEnabled := false
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pre-orders disable: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateTerritoryAvailability(requestCtx, trimmedID, asc.TerritoryAvailabilityUpdateAttributes{
				PreOrderEnabled:  &preOrderEnabled,
				ClearReleaseDate: true,
			})
			if err != nil {
				return fmt.Errorf("pre-orders disable: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPreOrdersEndCommand returns the end subcommand.
func newPreOrdersEndCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pre-orders end", flag.ExitOnError)

	territoryAvailabilityIDs := fs.String("territory-availability", "", "Territory availability IDs (comma-separated)")
	confirm := fs.Bool("confirm", false, "Confirm ending pre-orders (required)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "end",
		ShortUsage: "asc pre-orders end --territory-availability TERRITORY_AVAILABILITY_ID[,ID...] --confirm",
		ShortHelp:  "End pre-orders for territory availabilities.",
		LongHelp: `End pre-orders for territory availabilities.

Examples:
  asc pre-orders end --territory-availability "TA_1,TA_2" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ids := shared.SplitCSV(*territoryAvailabilityIDs)
			if len(ids) == 0 {
				return d.RequiredFlag("--territory-availability")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pre-orders end: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.EndAppAvailabilityPreOrders(requestCtx, ids)
			if err != nil {
				return fmt.Errorf("pre-orders end: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func normalizePreOrderReleaseDate(value string) (string, error) {
	return shared.NormalizeDate(value, "--release-date")
}
