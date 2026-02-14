package apps

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppTagsCommand returns the app-tags command with subcommands.
func AppTagsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-tags", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-tags",
		ShortUsage: "asc app-tags <subcommand> [flags]",
		ShortHelp:  "Manage app tags for App Store visibility.",
		LongHelp: `Manage app tags for App Store visibility.

Examples:
  asc app-tags list --app "APP_ID"
  asc app-tags get --app "APP_ID" --id "TAG_ID"
  asc app-tags update --id "TAG_ID" --visible-in-app-store=false --confirm
  asc app-tags territories --id "TAG_ID"
  asc app-tags relationships --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppTagsListCommand(),
			AppTagsGetCommand(),
			AppTagsUpdateCommand(),
			AppTagsTerritoriesCommand(),
			AppTagsTerritoriesRelationshipsCommand(),
			AppTagsRelationshipsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppTagsListCommand returns the list subcommand.
func AppTagsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-tags list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	visible := fs.String("visible-in-app-store", "", "Filter by visibility (true/false), comma-separated")
	sort := fs.String("sort", "", "Sort by name or -name")
	fields := fs.String("fields", "", "Fields to include: name, visibleInAppStore, territories")
	include := fs.String("include", "", "Include related resources: territories")
	territoryFields := fs.String("territory-fields", "", "Territory fields to include: currency")
	territoryLimit := fs.Int("territory-limit", 0, "Maximum territories per tag when including territories (1-50)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-tags list [flags]",
		ShortHelp:  "List app tags for an app.",
		LongHelp: `List app tags for an app.

Examples:
  asc app-tags list --app "APP_ID"
  asc app-tags list --app "APP_ID" --visible-in-app-store true
  asc app-tags list --app "APP_ID" --include territories --territory-fields currency
  asc app-tags list --app "APP_ID" --sort -name --limit 10
  asc app-tags list --next "<links.next>"
  asc app-tags list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-tags list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-tags list: %w", err)
			}
			if err := shared.ValidateSort(*sort, "name", "-name"); err != nil {
				return fmt.Errorf("app-tags list: %w", err)
			}
			if *territoryLimit != 0 && (*territoryLimit < 1 || *territoryLimit > 50) {
				return fmt.Errorf("app-tags list: --territory-limit must be between 1 and 50")
			}

			visibleValues, err := normalizeAppTagVisibilityFilter(*visible)
			if err != nil {
				return fmt.Errorf("app-tags list: %w", err)
			}

			fieldsValue, err := normalizeAppTagFields(*fields)
			if err != nil {
				return fmt.Errorf("app-tags list: %w", err)
			}

			includeValues, err := normalizeAppTagInclude(*include)
			if err != nil {
				return fmt.Errorf("app-tags list: %w", err)
			}

			territoryFieldsValue, err := normalizeTerritoryFields(*territoryFields)
			if err != nil {
				return fmt.Errorf("app-tags list: %w", err)
			}

			if len(territoryFieldsValue) > 0 && !shared.HasInclude(includeValues, "territories") {
				fmt.Fprintf(os.Stderr, "Error: --territory-fields requires --include territories\n\n")
				return flag.ErrHelp
			}
			if *territoryLimit != 0 && !shared.HasInclude(includeValues, "territories") {
				fmt.Fprintf(os.Stderr, "Error: --territory-limit requires --include territories\n\n")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-tags list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppTagsOption{
				asc.WithAppTagsVisibleInAppStore(visibleValues),
				asc.WithAppTagsLimit(*limit),
				asc.WithAppTagsNextURL(*next),
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithAppTagsSort(*sort))
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithAppTagsFields(fieldsValue))
			}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithAppTagsInclude(includeValues))
			}
			if len(territoryFieldsValue) > 0 {
				opts = append(opts, asc.WithAppTagsTerritoryFields(territoryFieldsValue))
			}
			if *territoryLimit > 0 {
				opts = append(opts, asc.WithAppTagsTerritoryLimit(*territoryLimit))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppTagsLimit(200))
				firstPage, err := client.GetAppTags(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-tags list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppTags(ctx, resolvedAppID, asc.WithAppTagsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-tags list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppTags(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("app-tags list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppTagsGetCommand returns the get subcommand.
func AppTagsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-tags get", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	tagID := fs.String("id", "", "App tag ID")
	fields := fs.String("fields", "", "Fields to include: name, visibleInAppStore, territories")
	include := fs.String("include", "", "Include related resources: territories")
	territoryFields := fs.String("territory-fields", "", "Territory fields to include: currency")
	territoryLimit := fs.Int("territory-limit", 0, "Maximum territories per tag when including territories (1-50)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-tags get [flags]",
		ShortHelp:  "Get an app tag by ID.",
		LongHelp: `Get an app tag by ID.

This command searches the app's tags for the specified ID.

Examples:
  asc app-tags get --app "APP_ID" --id "TAG_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*tagID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			if *territoryLimit != 0 && (*territoryLimit < 1 || *territoryLimit > 50) {
				return fmt.Errorf("app-tags get: --territory-limit must be between 1 and 50")
			}

			fieldsValue, err := normalizeAppTagFields(*fields)
			if err != nil {
				return fmt.Errorf("app-tags get: %w", err)
			}

			includeValues, err := normalizeAppTagInclude(*include)
			if err != nil {
				return fmt.Errorf("app-tags get: %w", err)
			}

			territoryFieldsValue, err := normalizeTerritoryFields(*territoryFields)
			if err != nil {
				return fmt.Errorf("app-tags get: %w", err)
			}

			includeTerritories := shared.HasInclude(includeValues, "territories")
			if len(territoryFieldsValue) > 0 && !includeTerritories {
				fmt.Fprintf(os.Stderr, "Error: --territory-fields requires --include territories\n\n")
				return flag.ErrHelp
			}
			if *territoryLimit != 0 && !includeTerritories {
				fmt.Fprintf(os.Stderr, "Error: --territory-limit requires --include territories\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-tags get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppTagsOption{}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithAppTagsFields(fieldsValue))
			}
			if !includeTerritories {
				if len(includeValues) > 0 {
					opts = append(opts, asc.WithAppTagsInclude(includeValues))
				}
				if len(territoryFieldsValue) > 0 {
					opts = append(opts, asc.WithAppTagsTerritoryFields(territoryFieldsValue))
				}
				if *territoryLimit > 0 {
					opts = append(opts, asc.WithAppTagsTerritoryLimit(*territoryLimit))
				}
			}

			resp, err := findAppTagByID(requestCtx, client, resolvedAppID, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("app-tags get: %w", err)
			}

			if includeTerritories {
				territoryOpts := []asc.TerritoriesOption{}
				if len(territoryFieldsValue) > 0 {
					territoryOpts = append(territoryOpts, asc.WithTerritoriesFields(territoryFieldsValue))
				}
				if *territoryLimit > 0 {
					territoryOpts = append(territoryOpts, asc.WithTerritoriesLimit(*territoryLimit))
				}

				territories, err := client.GetAppTagTerritories(requestCtx, trimmedID, territoryOpts...)
				if err != nil {
					return fmt.Errorf("app-tags get: failed to fetch territories: %w", err)
				}
				if len(territories.Data) > 0 {
					included, err := json.Marshal(territories.Data)
					if err != nil {
						return fmt.Errorf("app-tags get: %w", err)
					}
					resp.Included = included
				}
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppTagsUpdateCommand returns the update subcommand.
func AppTagsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-tags update", flag.ExitOnError)

	tagID := fs.String("id", "", "App tag ID")
	visibleInAppStore := fs.Bool("visible-in-app-store", false, "Set visibility in the App Store")
	confirm := fs.Bool("confirm", false, "Confirm update")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc app-tags update --id TAG_ID --visible-in-app-store [true|false] --confirm",
		ShortHelp:  "Update an app tag.",
		LongHelp: `Update an app tag.

Examples:
  asc app-tags update --id "TAG_ID" --visible-in-app-store --confirm
  asc app-tags update --id "TAG_ID" --visible-in-app-store=false --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*tagID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})
			if !visited["visible-in-app-store"] {
				fmt.Fprintln(os.Stderr, "Error: --visible-in-app-store is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-tags update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.AppTagUpdateAttributes{}
			if visited["visible-in-app-store"] {
				attrs.VisibleInAppStore = visibleInAppStore
			}

			resp, err := client.UpdateAppTag(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("app-tags update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppTagsTerritoriesCommand returns the app tag territories subcommand.
func AppTagsTerritoriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-tags territories", flag.ExitOnError)

	tagID := fs.String("id", "", "App tag ID")
	fields := fs.String("fields", "", "Fields to include: currency")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "territories",
		ShortUsage: "asc app-tags territories --id TAG_ID [flags]",
		ShortHelp:  "List territories for an app tag.",
		LongHelp: `List territories for an app tag.

Examples:
  asc app-tags territories --id "TAG_ID"
  asc app-tags territories --id "TAG_ID" --fields currency
  asc app-tags territories --id "TAG_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*tagID)
			if trimmedID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-tags territories: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-tags territories: %w", err)
			}

			fieldsValue, err := normalizeTerritoryFields(*fields)
			if err != nil {
				return fmt.Errorf("app-tags territories: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-tags territories: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.TerritoriesOption{
				asc.WithTerritoriesLimit(*limit),
				asc.WithTerritoriesNextURL(*next),
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithTerritoriesFields(fieldsValue))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithTerritoriesLimit(200))
				firstPage, err := client.GetAppTagTerritories(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-tags territories: failed to fetch: %w", err)
				}

				territories, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppTagTerritories(ctx, trimmedID, asc.WithTerritoriesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-tags territories: %w", err)
				}

				return shared.PrintOutput(territories, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppTagTerritories(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("app-tags territories: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppTagsTerritoriesRelationshipsCommand returns the app tag territory relationships subcommand.
func AppTagsTerritoriesRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-tags territories-relationships", flag.ExitOnError)

	tagID := fs.String("id", "", "App tag ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "territories-relationships",
		ShortUsage: "asc app-tags territories-relationships --id TAG_ID [flags]",
		ShortHelp:  "List territory relationships for an app tag.",
		LongHelp: `List territory relationships for an app tag.

Examples:
  asc app-tags territories-relationships --id "TAG_ID"
  asc app-tags territories-relationships --id "TAG_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*tagID)
			if trimmedID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-tags territories-relationships: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-tags territories-relationships: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-tags territories-relationships: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetAppTagTerritoriesRelationships(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-tags territories-relationships: failed to fetch: %w", err)
				}

				linkages, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppTagTerritoriesRelationships(ctx, trimmedID, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-tags territories-relationships: %w", err)
				}

				return shared.PrintOutput(linkages, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppTagTerritoriesRelationships(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("app-tags territories-relationships: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppTagsRelationshipsCommand returns the app tag relationships subcommand.
func AppTagsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-tags relationships", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc app-tags relationships --app APP_ID [flags]",
		ShortHelp:  "List app tag relationships for an app.",
		LongHelp: `List app tag relationships for an app.

Examples:
  asc app-tags relationships --app "APP_ID"
  asc app-tags relationships --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-tags relationships: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-tags relationships: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-tags relationships: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetAppTagsRelationshipsForApp(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-tags relationships: failed to fetch: %w", err)
				}

				linkages, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppTagsRelationshipsForApp(ctx, resolvedAppID, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-tags relationships: %w", err)
				}

				return shared.PrintOutput(linkages, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppTagsRelationshipsForApp(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("app-tags relationships: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func normalizeAppTagVisibilityFilter(value string) ([]string, error) {
	values := shared.SplitCSV(value)
	if len(values) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(values))
	for _, item := range values {
		lower := strings.ToLower(strings.TrimSpace(item))
		switch lower {
		case "true", "false":
			normalized = append(normalized, lower)
		default:
			return nil, fmt.Errorf("--visible-in-app-store must be true or false")
		}
	}

	return normalized, nil
}

func normalizeAppTagFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, field := range appTagFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(appTagFieldsList(), ", "))
		}
	}

	return fields, nil
}

func normalizeAppTagInclude(value string) ([]string, error) {
	values := shared.SplitCSV(value)
	if len(values) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, include := range appTagIncludeList() {
		allowed[include] = struct{}{}
	}
	for _, include := range values {
		if _, ok := allowed[include]; !ok {
			return nil, fmt.Errorf("--include must be one of: %s", strings.Join(appTagIncludeList(), ", "))
		}
	}

	return values, nil
}

func normalizeTerritoryFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, field := range territoryFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--territory-fields must be one of: %s", strings.Join(territoryFieldsList(), ", "))
		}
	}

	return fields, nil
}

func appTagFieldsList() []string {
	return []string{"name", "visibleInAppStore", "territories"}
}

func appTagIncludeList() []string {
	return []string{"territories"}
}

func territoryFieldsList() []string {
	return []string{"currency"}
}

func findAppTagByID(ctx context.Context, client *asc.Client, appID, tagID string, opts ...asc.AppTagsOption) (*asc.AppTagResponse, error) {
	baseOpts := append([]asc.AppTagsOption{asc.WithAppTagsLimit(200)}, opts...)
	resp, err := client.GetAppTags(ctx, appID, baseOpts...)
	if err != nil {
		return nil, err
	}

	for {
		for _, item := range resp.Data {
			if item.ID == tagID {
				return &asc.AppTagResponse{Data: item}, nil
			}
		}

		if resp.Links.Next == "" {
			break
		}

		resp, err = client.GetAppTags(ctx, appID, asc.WithAppTagsNextURL(resp.Links.Next))
		if err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("tag %q not found", tagID)
}
