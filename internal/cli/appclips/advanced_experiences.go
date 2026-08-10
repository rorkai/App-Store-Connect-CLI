package appclips

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type inlineLocalizationFlag []string

func (f *inlineLocalizationFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *inlineLocalizationFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func parseInlineLocalizations(values []string) ([]asc.AppClipAdvancedExperienceLocalizationCreateAttributes, error) {
	localizations := make([]asc.AppClipAdvancedExperienceLocalizationCreateAttributes, 0, len(values))
	for i, value := range values {
		var localization asc.AppClipAdvancedExperienceLocalizationCreateAttributes
		decoder := json.NewDecoder(strings.NewReader(value))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&localization); err != nil {
			return nil, fmt.Errorf("--inline-localization %d must be a JSON object with language, title, and optional subtitle: %w", i+1, err)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return nil, fmt.Errorf("--inline-localization %d must contain exactly one JSON object", i+1)
		}

		language, err := normalizeAppClipLanguage(string(localization.Language))
		if err != nil {
			return nil, fmt.Errorf("--inline-localization %d: %w", i+1, err)
		}
		localization.Language = language
		localization.Title = strings.TrimSpace(localization.Title)
		localization.Subtitle = strings.TrimSpace(localization.Subtitle)
		if localization.Title == "" {
			return nil, fmt.Errorf("--inline-localization %d: title is required", i+1)
		}
		localizations = append(localizations, localization)
	}
	return localizations, nil
}

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
  asc app-clips advanced-experiences create --app-clip-id "CLIP_ID" --link "https://example.com" --default-language EN --is-powered-by --header-image-id "IMAGE_ID" --localization-id "LOCALIZATION_ID"`,
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
	output := shared.BindOutputFlags(fs)

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
				return shared.MissingRequiredUsageError()
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
						return shared.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("app-clips advanced-experiences list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClipAdvancedExperiences(ctx, appClipValue, asc.WithAppClipAdvancedExperiencesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips advanced-experiences list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppClipAdvancedExperiences(requestCtx, appClipValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.AppClipAdvancedExperiencesResponse{Data: []asc.Resource[asc.AppClipAdvancedExperienceAttributes]{}}
					return shared.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips advanced-experiences list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipAdvancedExperiencesGetCommand gets an advanced experience by ID.
func AppClipAdvancedExperiencesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Advanced experience ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc app-clips advanced-experiences view --experience-id \"EXP_ID\"",
		ShortHelp:  "View an advanced experience by ID.",
		LongHelp: `View an advanced experience by ID.

Examples:
  asc app-clips advanced-experiences view --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*experienceID)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipAdvancedExperience(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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
	localizationIDs := fs.String("localization-id", "", "Existing localization ID(s), comma-separated")
	var inlineLocalizationJSON inlineLocalizationFlag
	fs.Var(&inlineLocalizationJSON, "inline-localization", "Inline localization as JSON with language, title, and optional subtitle (repeatable)")
	language := fs.String("language", "", "Inline localization language (use with --title)")
	title := fs.String("title", "", "Inline localization title (use with --language)")
	subtitle := fs.String("subtitle", "", "Inline localization subtitle (optional; requires --language and --title)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips advanced-experiences create --app-clip-id \"CLIP_ID\" --link \"https://example.com\" --default-language EN --is-powered-by --header-image-id \"IMAGE_ID\" (--localization-id \"LOCALIZATION_ID\" | --language EN --title \"TITLE\" | --inline-localization JSON) [flags]",
		ShortHelp:  "Create an advanced experience.",
		LongHelp: `Create an advanced experience.

Upload the header image first with ` + "`asc app-clips advanced-experiences images create --file path/to/image.png`" + `.
Provide existing localization IDs with ` + "`--localization-id`" + `, one inline localization with ` + "`--language`" + ` and ` + "`--title`" + `, or repeat ` + "`--inline-localization`" + ` for multiple JSON localizations. These inputs may be combined.

Examples:
  asc app-clips advanced-experiences create --app-clip-id "CLIP_ID" --link "https://example.com" --default-language EN --is-powered-by --header-image-id "IMAGE_ID" --localization-id "LOCALIZATION_ID"
  asc app-clips advanced-experiences create --app "APP_ID" --bundle-id "com.example.clip" --link "https://example.com" --default-language EN --is-powered-by --header-image-id "IMAGE_ID" --language EN --title "Order ahead" --subtitle "Ready when you arrive"
  asc app-clips advanced-experiences create --app-clip-id "CLIP_ID" --link "https://example.com" --default-language EN --is-powered-by --header-image-id "IMAGE_ID" --inline-localization '{"language":"EN","title":"Order ahead"}' --inline-localization '{"language":"FR","title":"Commander"}'`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			linkValue := strings.TrimSpace(*link)
			if linkValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --link is required")
				return shared.MissingRequiredUsageError()
			}

			if strings.TrimSpace(*defaultLanguage) == "" {
				fmt.Fprintln(os.Stderr, "Error: --default-language is required")
				return shared.MissingRequiredUsageError()
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
				return shared.MissingRequiredUsageError()
			}

			headerImageValue := strings.TrimSpace(*headerImageID)
			if headerImageValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --header-image-id is required")
				return shared.MissingRequiredUsageError()
			}

			localizationValues := shared.SplitCSV(*localizationIDs)
			languageValue := strings.TrimSpace(*language)
			titleValue := strings.TrimSpace(*title)
			subtitleValue := strings.TrimSpace(*subtitle)
			if titleValue != "" && languageValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --language is required when --title is set")
				return shared.MissingRequiredUsageError()
			}
			if languageValue != "" && titleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --title is required when --language is set")
				return shared.MissingRequiredUsageError()
			}
			if subtitleValue != "" && (languageValue == "" || titleValue == "") {
				fmt.Fprintln(os.Stderr, "Error: --language and --title are required when --subtitle is set")
				return shared.MissingRequiredUsageError()
			}

			inlineLocalizations, err := parseInlineLocalizations(inlineLocalizationJSON)
			if err != nil {
				return shared.UsageErrorf("app-clips advanced-experiences create: %v", err)
			}
			if languageValue != "" && titleValue != "" {
				parsedLanguage, err := normalizeAppClipLanguage(languageValue)
				if err != nil {
					return shared.UsageErrorf("app-clips advanced-experiences create: %v", err)
				}
				inlineLocalizations = append(inlineLocalizations, asc.AppClipAdvancedExperienceLocalizationCreateAttributes{
					Language: parsedLanguage,
					Title:    titleValue,
					Subtitle: subtitleValue,
				})
			}
			if len(localizationValues) == 0 && len(inlineLocalizations) == 0 {
				fmt.Fprintln(os.Stderr, "Error: provide --localization-id, --inline-localization, or both --language and --title")
				return shared.MissingRequiredUsageError()
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

			appClipValue := strings.TrimSpace(*appClipID)
			bundleValue := strings.TrimSpace(*bundleID)
			if appClipValue == "" && bundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app-clip-id or --bundle-id is required")
				return shared.MissingRequiredUsageError()
			}
			appValue := strings.TrimSpace(shared.ResolveAppID(*appID))
			if appClipValue == "" && appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required with --bundle-id")
				return shared.MissingRequiredUsageError()
			}

			client, err := appClipsClientFactory()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

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

			resp, err := client.CreateAppClipAdvancedExperience(requestCtx, appClipValue, attrs, headerImageValue, localizationValues, inlineLocalizations)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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
	output := shared.BindOutputFlags(fs)

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
				return shared.MissingRequiredUsageError()
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			hasUpdate := visited["action"] || visited["category"] || visited["default-language"] || visited["is-powered-by"] || visited["removed"] || visited["header-image-id"] || visited["localization-id"] || visited["app-clip-id"]
			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError()
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

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipAdvancedExperiencesDeleteCommand deletes an advanced experience.
func AppClipAdvancedExperiencesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Advanced experience ID")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-clips advanced-experiences delete --experience-id \"EXP_ID\" --confirm",
		ShortHelp:  "Remove an advanced experience.",
		LongHelp: `Remove an advanced experience by setting its removed attribute to true.

Examples:
  asc app-clips advanced-experiences delete --experience-id "EXP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to remove")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppClipAdvancedExperience(requestCtx, experienceValue); err != nil {
				return fmt.Errorf("app-clips advanced-experiences delete: failed to remove: %w", err)
			}

			result := &asc.AppClipAdvancedExperienceRemoveResult{
				ID:      experienceValue,
				Removed: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
