package appclips

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppClipDefaultExperienceLocalizationsCommand returns the localizations command group.
func AppClipDefaultExperienceLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc app-clips default-experiences localizations <subcommand> [flags]",
		ShortHelp:  "Manage App Clip default experience localizations.",
		LongHelp: `Manage App Clip default experience localizations.

Examples:
  asc app-clips default-experiences localizations list --experience-id "EXP_ID"
  asc app-clips default-experiences localizations create --experience-id "EXP_ID" --locale "en-US" --subtitle "Try it"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipDefaultExperienceLocalizationsListCommand(),
			AppClipDefaultExperienceLocalizationsGetCommand(),
			AppClipDefaultExperienceLocalizationsCreateCommand(),
			AppClipDefaultExperienceLocalizationsUpdateCommand(),
			AppClipDefaultExperienceLocalizationsDeleteCommand(),
			AppClipDefaultExperienceLocalizationHeaderImageRelationshipCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipDefaultExperienceLocalizationsListCommand lists localizations.
func AppClipDefaultExperienceLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Default experience ID")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-clips default-experiences localizations list --experience-id \"EXP_ID\" [flags]",
		ShortHelp:  "List localizations for a default experience.",
		LongHelp: `List localizations for a default experience.

Examples:
  asc app-clips default-experiences localizations list --experience-id "EXP_ID"
  asc app-clips default-experiences localizations list --experience-id "EXP_ID" --locale "en-US"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-clips default-experiences localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-clips default-experiences localizations list: %w", err)
			}

			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppClipDefaultExperienceLocalizationsOption{
				asc.WithAppClipDefaultExperienceLocalizationsLimit(*limit),
				asc.WithAppClipDefaultExperienceLocalizationsNextURL(*next),
				asc.WithAppClipDefaultExperienceLocalizationsLocales(shared.SplitCSV(*locale)),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppClipDefaultExperienceLocalizationsLimit(200))
				firstPage, err := client.GetAppClipDefaultExperienceLocalizations(requestCtx, experienceValue, paginateOpts...)
				if err != nil {
					if asc.IsNotFound(err) {
						empty := &asc.AppClipDefaultExperienceLocalizationsResponse{Data: []asc.Resource[asc.AppClipDefaultExperienceLocalizationAttributes]{}}
						return shared.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("app-clips default-experiences localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClipDefaultExperienceLocalizations(ctx, experienceValue, asc.WithAppClipDefaultExperienceLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips default-experiences localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppClipDefaultExperienceLocalizations(requestCtx, experienceValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.AppClipDefaultExperienceLocalizationsResponse{Data: []asc.Resource[asc.AppClipDefaultExperienceLocalizationAttributes]{}}
					return shared.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips default-experiences localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperienceLocalizationsGetCommand gets a localization by ID.
func AppClipDefaultExperienceLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-clips default-experiences localizations get --localization-id \"LOC_ID\"",
		ShortHelp:  "Get a localization by ID.",
		LongHelp: `Get a localization by ID.

Examples:
  asc app-clips default-experiences localizations get --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceLocalization(requestCtx, locValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperienceLocalizationsCreateCommand creates a localization.
func AppClipDefaultExperienceLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Default experience ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	subtitle := fs.String("subtitle", "", "Subtitle")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips default-experiences localizations create --experience-id \"EXP_ID\" --locale \"en-US\" [flags]",
		ShortHelp:  "Create a localization for a default experience.",
		LongHelp: `Create a localization for a default experience.

Examples:
  asc app-clips default-experiences localizations create --experience-id "EXP_ID" --locale "en-US" --subtitle "Try it"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}

			var subtitleValue *string
			if strings.TrimSpace(*subtitle) != "" {
				value := strings.TrimSpace(*subtitle)
				subtitleValue = &value
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.AppClipDefaultExperienceLocalizationCreateAttributes{
				Locale:   localeValue,
				Subtitle: subtitleValue,
			}

			resp, err := client.CreateAppClipDefaultExperienceLocalization(requestCtx, experienceValue, attrs)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperienceLocalizationsUpdateCommand updates a localization.
func AppClipDefaultExperienceLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Localization ID")
	subtitle := fs.String("subtitle", "", "Subtitle")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc app-clips default-experiences localizations update --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "Update a localization.",
		LongHelp: `Update a localization.

Examples:
  asc app-clips default-experiences localizations update --localization-id "LOC_ID" --subtitle "Try it"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if !visited["subtitle"] {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			var attrs *asc.AppClipDefaultExperienceLocalizationUpdateAttributes
			if visited["subtitle"] {
				value := strings.TrimSpace(*subtitle)
				attrs = &asc.AppClipDefaultExperienceLocalizationUpdateAttributes{
					Subtitle: &value,
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppClipDefaultExperienceLocalization(requestCtx, locValue, attrs)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperienceLocalizationsDeleteCommand deletes a localization.
func AppClipDefaultExperienceLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-clips default-experiences localizations delete --localization-id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a localization.",
		LongHelp: `Delete a localization.

Examples:
  asc app-clips default-experiences localizations delete --localization-id "LOC_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppClipDefaultExperienceLocalization(requestCtx, locValue); err != nil {
				return fmt.Errorf("app-clips default-experiences localizations delete: failed to delete: %w", err)
			}

			result := &asc.AppClipDefaultExperienceLocalizationDeleteResult{
				ID:      locValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperienceLocalizationHeaderImageRelationshipCommand returns the header image relationship command.
func AppClipDefaultExperienceLocalizationHeaderImageRelationshipCommand() *ffcli.Command {
	fs := flag.NewFlagSet("header-image-relationship", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "header-image-relationship",
		ShortUsage: "asc app-clips default-experiences localizations header-image-relationship --localization-id \"LOC_ID\"",
		ShortHelp:  "Get header image relationship for a localization.",
		LongHelp: `Get header image relationship for a localization.

Examples:
  asc app-clips default-experiences localizations header-image-relationship --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations header-image-relationship: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceLocalizationHeaderImageRelationship(requestCtx, locValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations header-image-relationship: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
