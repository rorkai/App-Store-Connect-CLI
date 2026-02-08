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

// AppClipAdvancedExperiencesCommand returns the advanced experiences command group.
func AppClipAdvancedExperiencesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("advanced-experiences", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "advanced-experiences",
		ShortUsage: "asc app-clips advanced-experiences <subcommand> [flags]",
		ShortHelp:  "Manage App Clip advanced experiences.",
		LongHelp: `Manage App Clip advanced experiences.

Examples:
  asc app-clips advanced-experiences list --app-clip-id "CLIP_ID"
  asc app-clips advanced-experiences create --app-clip-id "CLIP_ID" --link "https://example.com" --default-language EN --is-powered-by`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipAdvancedExperiencesListCommand(),
			AppClipAdvancedExperiencesGetCommand(),
			AppClipAdvancedExperiencesCreateCommand(),
			AppClipAdvancedExperiencesUpdateCommand(),
			AppClipAdvancedExperiencesDeleteCommand(),
			AppClipAdvancedExperienceImagesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipAdvancedExperiencesListCommand lists advanced experiences.
func AppClipAdvancedExperiencesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appClipID := fs.String("app-clip-id", "", "App Clip ID")
	action := fs.String("action", "", "Filter by action(s): OPEN, VIEW, PLAY (comma-separated)")
	status := fs.String("status", "", "Filter by status(es), comma-separated")
	placeStatus := fs.String("place-status", "", "Filter by place status(es), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-clips advanced-experiences list --app-clip-id \"CLIP_ID\" [flags]",
		ShortHelp:  "List advanced experiences for an App Clip.",
		LongHelp: `List advanced experiences for an App Clip.

Examples:
  asc app-clips advanced-experiences list --app-clip-id "CLIP_ID"
  asc app-clips advanced-experiences list --app-clip-id "CLIP_ID" --action OPEN`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-clips advanced-experiences list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-clips advanced-experiences list: %w", err)
			}

			actionValues, err := normalizeAppClipActionList(*action)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences list: %w", err)
			}

			appClipValue := strings.TrimSpace(*appClipID)
			if appClipValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app-clip-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppClipAdvancedExperiencesOption{
				asc.WithAppClipAdvancedExperiencesLimit(*limit),
				asc.WithAppClipAdvancedExperiencesNextURL(*next),
				asc.WithAppClipAdvancedExperiencesActions(actionValues),
				asc.WithAppClipAdvancedExperiencesStatuses(shared.SplitCSVUpper(*status)),
				asc.WithAppClipAdvancedExperiencesPlaceStatuses(shared.SplitCSVUpper(*placeStatus)),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppClipAdvancedExperiencesLimit(200))
				firstPage, err := client.GetAppClipAdvancedExperiences(requestCtx, appClipValue, paginateOpts...)
				if err != nil {
					if asc.IsNotFound(err) {
						empty := &asc.AppClipAdvancedExperiencesResponse{Data: []asc.Resource[asc.AppClipAdvancedExperienceAttributes]{}}
						return shared.PrintOutput(empty, *output, *pretty)
					}
					return fmt.Errorf("app-clips advanced-experiences list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClipAdvancedExperiences(ctx, appClipValue, asc.WithAppClipAdvancedExperiencesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips advanced-experiences list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppClipAdvancedExperiences(requestCtx, appClipValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.AppClipAdvancedExperiencesResponse{Data: []asc.Resource[asc.AppClipAdvancedExperienceAttributes]{}}
					return shared.PrintOutput(empty, *output, *pretty)
				}
				return fmt.Errorf("app-clips advanced-experiences list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppClipAdvancedExperiencesGetCommand gets an advanced experience by ID.
func AppClipAdvancedExperiencesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Advanced experience ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-clips advanced-experiences get --experience-id \"EXP_ID\"",
		ShortHelp:  "Get an advanced experience by ID.",
		LongHelp: `Get an advanced experience by ID.

Examples:
  asc app-clips advanced-experiences get --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*experienceID)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipAdvancedExperience(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppClipAdvancedExperiencesCreateCommand creates an advanced experience.
func AppClipAdvancedExperiencesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appClipID := fs.String("app-clip-id", "", "App Clip ID")
	bundleID := fs.String("bundle-id", "", "App Clip bundle ID (requires --app)")
	link := fs.String("link", "", "Invocation URL (required)")
	defaultLanguage := fs.String("default-language", "", "Default language (e.g., EN)")
	isPoweredBy := fs.Bool("is-powered-by", false, "Powered by your app")
	action := fs.String("action", "", "Action (OPEN, VIEW, PLAY)")
	category := fs.String("category", "", "Business category")
	headerImageID := fs.String("header-image-id", "", "Header image ID")
	localizationIDs := fs.String("localization-id", "", "Localization ID(s), comma-separated")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips advanced-experiences create --app-clip-id \"CLIP_ID\" --link \"https://example.com\" --default-language EN --is-powered-by [flags]",
		ShortHelp:  "Create an advanced experience.",
		LongHelp: `Create an advanced experience.

Examples:
  asc app-clips advanced-experiences create --app-clip-id "CLIP_ID" --link "https://example.com" --default-language EN --is-powered-by
  asc app-clips advanced-experiences create --app "APP_ID" --bundle-id "com.example.clip" --link "https://example.com" --default-language EN --is-powered-by`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			linkValue := strings.TrimSpace(*link)
			if linkValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --link is required")
				return flag.ErrHelp
			}

			if strings.TrimSpace(*defaultLanguage) == "" {
				fmt.Fprintln(os.Stderr, "Error: --default-language is required")
				return flag.ErrHelp
			}

			langValue, err := normalizeAppClipLanguage(*defaultLanguage)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences create: %w", err)
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})
			if !visited["is-powered-by"] {
				fmt.Fprintln(os.Stderr, "Error: --is-powered-by is required")
				return flag.ErrHelp
			}

			var actionValue *asc.AppClipAction
			if strings.TrimSpace(*action) != "" {
				parsed, err := normalizeAppClipAction(*action)
				if err != nil {
					return fmt.Errorf("app-clips advanced-experiences create: %w", err)
				}
				actionValue = &parsed
			}

			var categoryValue *asc.AppClipAdvancedExperienceBusinessCategory
			if strings.TrimSpace(*category) != "" {
				parsed, err := normalizeAppClipBusinessCategory(*category)
				if err != nil {
					return fmt.Errorf("app-clips advanced-experiences create: %w", err)
				}
				categoryValue = &parsed
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			appClipValue := strings.TrimSpace(*appClipID)
			bundleValue := strings.TrimSpace(*bundleID)
			appValue := strings.TrimSpace(shared.ResolveAppID(*appID))
			if appClipValue == "" && bundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app-clip-id or --bundle-id is required")
				return flag.ErrHelp
			}
			if appClipValue == "" && appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required with --bundle-id")
				return flag.ErrHelp
			}

			appClipValue, err = resolveAppClipID(requestCtx, client, appValue, appClipValue, bundleValue)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences create: %w", err)
			}

			attrs := asc.AppClipAdvancedExperienceCreateAttributes{
				Link:             linkValue,
				DefaultLanguage:  langValue,
				IsPoweredBy:      *isPoweredBy,
				Action:           actionValue,
				BusinessCategory: categoryValue,
			}

			resp, err := client.CreateAppClipAdvancedExperience(requestCtx, appClipValue, attrs, *headerImageID, shared.SplitCSV(*localizationIDs))
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppClipAdvancedExperiencesUpdateCommand updates an advanced experience.
func AppClipAdvancedExperiencesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Advanced experience ID")
	appClipID := fs.String("app-clip-id", "", "App Clip ID")
	action := fs.String("action", "", "Action (OPEN, VIEW, PLAY)")
	category := fs.String("category", "", "Business category")
	defaultLanguage := fs.String("default-language", "", "Default language (e.g., EN)")
	isPoweredBy := fs.Bool("is-powered-by", false, "Powered by your app")
	removed := fs.Bool("removed", false, "Mark the experience as removed")
	headerImageID := fs.String("header-image-id", "", "Header image ID")
	localizationIDs := fs.String("localization-id", "", "Localization ID(s), comma-separated")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc app-clips advanced-experiences update --experience-id \"EXP_ID\" [flags]",
		ShortHelp:  "Update an advanced experience.",
		LongHelp: `Update an advanced experience.

Examples:
  asc app-clips advanced-experiences update --experience-id "EXP_ID" --action VIEW
  asc app-clips advanced-experiences update --experience-id "EXP_ID" --category FOOD_AND_DRINK`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			hasUpdate := visited["action"] || visited["category"] || visited["default-language"] || visited["is-powered-by"] || visited["removed"] || visited["header-image-id"] || visited["localization-id"] || visited["app-clip-id"]
			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			var attrs *asc.AppClipAdvancedExperienceUpdateAttributes
			if visited["action"] || visited["category"] || visited["default-language"] || visited["is-powered-by"] || visited["removed"] {
				update := asc.AppClipAdvancedExperienceUpdateAttributes{}
				if visited["action"] {
					parsed, err := normalizeAppClipAction(*action)
					if err != nil {
						return fmt.Errorf("app-clips advanced-experiences update: %w", err)
					}
					update.Action = &parsed
				}
				if visited["category"] {
					parsed, err := normalizeAppClipBusinessCategory(*category)
					if err != nil {
						return fmt.Errorf("app-clips advanced-experiences update: %w", err)
					}
					update.BusinessCategory = &parsed
				}
				if visited["default-language"] {
					parsed, err := normalizeAppClipLanguage(*defaultLanguage)
					if err != nil {
						return fmt.Errorf("app-clips advanced-experiences update: %w", err)
					}
					update.DefaultLanguage = &parsed
				}
				if visited["is-powered-by"] {
					value := *isPoweredBy
					update.IsPoweredBy = &value
				}
				if visited["removed"] {
					value := *removed
					update.Removed = &value
				}
				attrs = &update
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppClipAdvancedExperience(requestCtx, experienceValue, attrs, *appClipID, *headerImageID, shared.SplitCSV(*localizationIDs))
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppClipAdvancedExperiencesDeleteCommand deletes an advanced experience.
func AppClipAdvancedExperiencesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Advanced experience ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-clips advanced-experiences delete --experience-id \"EXP_ID\" --confirm",
		ShortHelp:  "Delete an advanced experience.",
		LongHelp: `Delete an advanced experience.

Examples:
  asc app-clips advanced-experiences delete --experience-id "EXP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppClipAdvancedExperience(requestCtx, experienceValue); err != nil {
				return fmt.Errorf("app-clips advanced-experiences delete: failed to delete: %w", err)
			}

			result := &asc.AppClipAdvancedExperienceDeleteResult{
				ID:      experienceValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
