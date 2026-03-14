package apps

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppsInfoTerritoryAgeRatingsCommand returns the apps info territory-age-ratings command group.
func AppsInfoTerritoryAgeRatingsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps info territory-age-ratings", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "territory-age-ratings",
		ShortUsage: "asc apps info territory-age-ratings <subcommand> [flags]",
		ShortHelp:  "List territory age ratings for an app info.",
		LongHelp: `List territory age ratings for an app info.

Examples:
  asc apps info territory-age-ratings list --app "APP_ID"
  asc apps info territory-age-ratings list --info-id "APP_INFO_ID" --include territory --territory-fields currency`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppsInfoTerritoryAgeRatingsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppsInfoTerritoryAgeRatingsListCommand returns the list subcommand for territory age ratings.
func AppsInfoTerritoryAgeRatingsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps info territory-age-ratings list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	infoID := fs.String("info-id", "", "App Info ID (optional override)")
	legacyID := fs.String("id", "", "Deprecated alias for --info-id")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(territoryAgeRatingFieldsList(), ", "))
	territoryFields := fs.String("territory-fields", "", "Territory fields to include: "+strings.Join(territoryFieldsList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(territoryAgeRatingIncludeList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc apps info territory-age-ratings list [flags]",
		ShortHelp:  "List territory age ratings for an app info.",
		LongHelp: `List territory age ratings for an app info.

Examples:
  asc apps info territory-age-ratings list --app "APP_ID"
  asc apps info territory-age-ratings list --info-id "APP_INFO_ID" --include territory --territory-fields currency
  asc apps info territory-age-ratings list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			infoIDValue, err := resolveInfoIDFlags(*infoID, *legacyID, "--id")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("apps info territory-age-ratings list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("apps info territory-age-ratings list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && infoIDValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --info-id is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			fieldsValue, err := normalizeTerritoryAgeRatingFields(*fields)
			if err != nil {
				return fmt.Errorf("apps info territory-age-ratings list: %w", err)
			}
			territoryFieldsValue, err := normalizeTerritoryFields(*territoryFields)
			if err != nil {
				return fmt.Errorf("apps info territory-age-ratings list: %w", err)
			}
			includeValue, err := normalizeTerritoryAgeRatingInclude(*include)
			if err != nil {
				return fmt.Errorf("apps info territory-age-ratings list: %w", err)
			}
			if len(territoryFieldsValue) > 0 && !contains(includeValue, "territory") {
				fmt.Fprintln(os.Stderr, "Error: --territory-fields requires --include territory")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps info territory-age-ratings list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedInfoID := infoIDValue
			if resolvedInfoID == "" && strings.TrimSpace(*next) == "" {
				resolvedInfoID, err = shared.ResolveAppInfoID(requestCtx, client, resolvedAppID, infoIDValue)
				if err != nil {
					return fmt.Errorf("apps info territory-age-ratings list: %w", err)
				}
			}

			opts := []asc.TerritoryAgeRatingsOption{
				asc.WithTerritoryAgeRatingsFields(fieldsValue),
				asc.WithTerritoryAgeRatingsTerritoryFields(territoryFieldsValue),
				asc.WithTerritoryAgeRatingsInclude(includeValue),
				asc.WithTerritoryAgeRatingsLimit(*limit),
				asc.WithTerritoryAgeRatingsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithTerritoryAgeRatingsLimit(200))
				firstPage, err := client.GetAppInfoTerritoryAgeRatings(requestCtx, resolvedInfoID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("apps info territory-age-ratings list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppInfoTerritoryAgeRatings(ctx, resolvedInfoID, asc.WithTerritoryAgeRatingsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("apps info territory-age-ratings list: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppInfoTerritoryAgeRatings(requestCtx, resolvedInfoID, opts...)
			if err != nil {
				return fmt.Errorf("apps info territory-age-ratings list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func normalizeTerritoryAgeRatingFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, field := range territoryAgeRatingFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(territoryAgeRatingFieldsList(), ", "))
		}
	}

	return fields, nil
}

func normalizeTerritoryAgeRatingInclude(value string) ([]string, error) {
	include := shared.SplitCSV(value)
	if len(include) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, item := range territoryAgeRatingIncludeList() {
		allowed[item] = struct{}{}
	}
	for _, item := range include {
		if _, ok := allowed[item]; !ok {
			return nil, fmt.Errorf("--include must be one of: %s", strings.Join(territoryAgeRatingIncludeList(), ", "))
		}
	}

	return include, nil
}

func territoryAgeRatingFieldsList() []string {
	return []string{"appStoreAgeRating", "territory"}
}

func territoryAgeRatingIncludeList() []string {
	return []string{"territory"}
}

func contains(values []string, value string) bool {
	return slices.Contains(values, value)
}
