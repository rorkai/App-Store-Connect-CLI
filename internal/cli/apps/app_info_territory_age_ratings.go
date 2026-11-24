package apps

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppsInfoTerritoryAgeRatingsCommand returns the apps info territory-age-ratings command group.
func newAppsInfoTerritoryAgeRatingsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apps info territory-age-ratings", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "territory-age-ratings",
		ShortUsage: "asc apps info territory-age-ratings <subcommand> [flags]",
		ShortHelp:  "List territory age ratings for an app info.",
		LongHelp: `List territory age ratings for an app info.

Examples:
  asc apps info territory-age-ratings list --app "APP_ID"
  asc apps info territory-age-ratings list --info-id "APP_INFO_ID" --include territory --territory-fields currency`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppsInfoTerritoryAgeRatingsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppsInfoTerritoryAgeRatingsListCommand returns the list subcommand for territory age ratings.
func newAppsInfoTerritoryAgeRatingsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apps info territory-age-ratings list", flag.ExitOnError)

	appID := shared.BindResourceIDFlag(fs, "app", "apps", "App Store Connect app ID (or ASC_APP_ID env)")
	infoID := shared.BindResourceIDFlag(fs, "info-id", "appInfos", "App Info ID (optional override)")
	shared.BindFlagAlias(fs, "id", "info-id")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(territoryAgeRatingFieldsList(), ", "))
	territoryFields := fs.String("territory-fields", "", "Territory fields to include: "+strings.Join(territoryFieldsList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(territoryAgeRatingIncludeList(), ", "))
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "apps info territory-age-ratings list", 200, "fields", "territory-fields", "include"); err != nil {
				return err
			}
			infoIDValue := strings.TrimSpace(*infoID)

			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" && infoIDValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app or --info-id is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("")
			}

			fieldsValue, err := normalizeTerritoryAgeRatingFields(*fields)
			if err != nil {
				return d.UsageErrorf("apps info territory-age-ratings list: %v", err)
			}
			territoryFieldsValue, err := normalizeTerritoryFields(*territoryFields)
			if err != nil {
				return d.UsageErrorf("apps info territory-age-ratings list: %v", err)
			}
			includeValue, err := normalizeTerritoryAgeRatingInclude(*include)
			if err != nil {
				return d.UsageErrorf("apps info territory-age-ratings list: %v", err)
			}
			if len(territoryFieldsValue) > 0 && !contains(includeValue, "territory") {
				fmt.Fprintln(d.IO.ErrOut, "Error: --territory-fields requires --include territory")
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("apps info territory-age-ratings list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resolvedInfoID := infoIDValue
			nextValue := strings.TrimSpace(*next)
			if resolvedAppID != "" && nextValue != "" {
				nextInfoID, nextErr := territoryAgeRatingsAppInfoID(nextValue)
				if nextErr != nil {
					return d.UsageErrorf("apps info territory-age-ratings list: %v", nextErr)
				}
				if infoIDValue != "" && infoIDValue != nextInfoID {
					return d.UsageErrorf("apps info territory-age-ratings list: --info-id %q does not match the --next target %q", infoIDValue, nextInfoID)
				}
				resolvedInfoID, err = d.ResolveAppInfoIDWithFlag(requestCtx, client, resolvedAppID, nextInfoID, "--info-id")
				if err != nil {
					return fmt.Errorf("apps info territory-age-ratings list: %w", err)
				}
			} else if nextValue == "" {
				resolvedInfoID, err = d.ResolveAppInfoIDWithFlag(requestCtx, client, resolvedAppID, infoIDValue, "--info-id")
				if err != nil {
					return fmt.Errorf("apps info territory-age-ratings list: %w", err)
				}
			}

			opts := []asc.TerritoryAgeRatingsOption{
				asc.WithTerritoryAgeRatingsFields(fieldsValue),
				asc.WithTerritoryAgeRatingsTerritoryFields(territoryFieldsValue),
				asc.WithTerritoryAgeRatingsInclude(includeValue),
				asc.Limit[asc.TerritoryAgeRatingsOption](*limit),
				asc.NextURL[asc.TerritoryAgeRatingsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.TerritoryAgeRatingsOption](200))
				firstPage, err := client.GetAppInfoTerritoryAgeRatings(requestCtx, resolvedInfoID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("apps info territory-age-ratings list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					if resolvedAppID != "" {
						nextInfoID, nextErr := territoryAgeRatingsAppInfoID(nextURL)
						if nextErr != nil {
							return nil, nextErr
						}
						if nextInfoID != resolvedInfoID {
							return nil, fmt.Errorf("--next target %q does not match selected app info %q", nextInfoID, resolvedInfoID)
						}
					}
					return client.GetAppInfoTerritoryAgeRatings(ctx, resolvedInfoID, asc.NextURL[asc.TerritoryAgeRatingsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("apps info territory-age-ratings list: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppInfoTerritoryAgeRatings(requestCtx, resolvedInfoID, opts...)
			if err != nil {
				return fmt.Errorf("apps info territory-age-ratings list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func territoryAgeRatingsAppInfoID(nextValue string) (string, error) {
	parsed, err := url.Parse(nextValue)
	if err != nil {
		return "", fmt.Errorf("--next is not a valid territory age-ratings URL: %w", err)
	}
	parts := strings.Split(parsed.Path, "/")
	if len(parts) != 5 || parts[0] != "" || parts[1] != "v1" || parts[2] != "appInfos" || parts[4] != "territoryAgeRatings" {
		return "", fmt.Errorf("--next must target /v1/appInfos/{id}/territoryAgeRatings when --app is selected")
	}
	appInfoID := parts[3]
	if appInfoID == "" || appInfoID == "." || appInfoID == ".." || strings.Contains(appInfoID, "%") {
		return "", fmt.Errorf("--next must contain an app info ID")
	}
	return appInfoID, nil
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
