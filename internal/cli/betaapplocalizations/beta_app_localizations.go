package betaapplocalizations

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

// BetaAppLocalizationsCommand returns the beta-app-localizations command group.
func BetaAppLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-app-localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-app-localizations",
		ShortUsage: "asc beta-app-localizations <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight beta app localizations.",
		LongHelp: `Manage TestFlight beta app localizations.

Examples:
  asc beta-app-localizations list --app "APP_ID"
  asc beta-app-localizations create --app "APP_ID" --locale "en-US" --description "Welcome testers"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaAppLocalizationsListCommand(),
			BetaAppLocalizationsGetCommand(),
			BetaAppLocalizationsAppCommand(),
			BetaAppLocalizationsCreateCommand(),
			BetaAppLocalizationsUpdateCommand(),
			BetaAppLocalizationsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaAppLocalizationsListCommand returns the list subcommand.
func BetaAppLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc beta-app-localizations list [flags]",
		ShortHelp:  "List beta app localizations for an app.",
		LongHelp: `List beta app localizations for an app.

Examples:
  asc beta-app-localizations list --app "APP_ID"
  asc beta-app-localizations list --app "APP_ID" --locale "en-US,ja"
  asc beta-app-localizations list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageErrorCtx(ctx, "beta-app-localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorfCtx(ctx, "beta-app-localizations list: %v", err)
			}
			if err := shared.RejectNextFlagConflicts(fs, *next, "testflight app-localizations list", "locale"); err != nil {
				return err
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			locales := shared.SplitCSV(*locale)
			if err := shared.ValidateBuildLocalizationLocales(locales); err != nil {
				return shared.UsageErrorfCtx(ctx, "beta-app-localizations list: %v", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-app-localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaAppLocalizationsOption{
				asc.Limit[asc.BetaAppLocalizationsOption](*limit),
				asc.NextURL[asc.BetaAppLocalizationsOption](*next),
			}
			if resolvedAppID != "" {
				opts = append(opts, asc.WithBetaAppLocalizationAppIDs([]string{resolvedAppID}))
			}
			if len(locales) > 0 {
				opts = append(opts, asc.WithBetaAppLocalizationLocales(locales))
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.BetaAppLocalizationsOption](200))
				firstPage, err := client.GetBetaAppLocalizations(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("beta-app-localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBetaAppLocalizations(ctx, asc.NextURL[asc.BetaAppLocalizationsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("beta-app-localizations list: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBetaAppLocalizations(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("beta-app-localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BetaAppLocalizationsGetCommand returns the get subcommand.
func BetaAppLocalizationsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc beta-app-localizations view --id \"LOCALIZATION_ID\"",
		ShortHelp:   "View a beta app localization by ID.",
		LongHelp: `View a beta app localization by ID.

Examples:
  asc beta-app-localizations view --id "LOCALIZATION_ID"`,
		IDUsage:            "Beta app localization ID",
		IDType:             "betaAppLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "beta-app-localizations view",
		FetchErrorPrefix:   "beta-app-localizations view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBetaAppLocalization(ctx, id)
		},
	})
}

// BetaAppLocalizationsCreateCommand returns the create subcommand.
func BetaAppLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	description := fs.String("description", "", "Beta app description")
	feedbackEmail := fs.String("feedback-email", "", "Feedback email")
	marketingURL := fs.String("marketing-url", "", "Marketing URL")
	privacyPolicyURL := fs.String("privacy-policy-url", "", "Privacy policy URL")
	tvOsPrivacyPolicy := fs.String("tv-os-privacy-policy", "", "tvOS privacy policy")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc beta-app-localizations create [flags]",
		ShortHelp:  "Create a beta app localization.",
		LongHelp: `Create a beta app localization.

Examples:
  asc beta-app-localizations create --app "APP_ID" --locale "en-US"
  asc beta-app-localizations create --app "APP_ID" --locale "en-US" --description "Welcome testers"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				return shared.RequiredFlag("--locale")
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return shared.UsageErrorfCtx(ctx, "beta-app-localizations create: %v", err)
			}

			attrs := asc.BetaAppLocalizationAttributes{
				Locale: localeValue,
			}

			if value := strings.TrimSpace(*description); value != "" {
				attrs.Description = value
			}
			if value := strings.TrimSpace(*feedbackEmail); value != "" {
				attrs.FeedbackEmail = value
			}
			if value := strings.TrimSpace(*marketingURL); value != "" {
				attrs.MarketingURL = value
			}
			if value := strings.TrimSpace(*privacyPolicyURL); value != "" {
				attrs.PrivacyPolicyURL = value
			}
			if value := strings.TrimSpace(*tvOsPrivacyPolicy); value != "" {
				attrs.TvOsPrivacyPolicy = value
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-app-localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateBetaAppLocalization(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("beta-app-localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BetaAppLocalizationsUpdateCommand returns the update subcommand.
func BetaAppLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaAppLocalizations", "Beta app localization ID")
	description := fs.String("description", "", "Beta app description")
	feedbackEmail := fs.String("feedback-email", "", "Feedback email")
	marketingURL := fs.String("marketing-url", "", "Marketing URL")
	privacyPolicyURL := fs.String("privacy-policy-url", "", "Privacy policy URL")
	tvOsPrivacyPolicy := fs.String("tv-os-privacy-policy", "", "tvOS privacy policy")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc beta-app-localizations update [flags]",
		ShortHelp:  "Update a beta app localization.",
		LongHelp: `Update a beta app localization.

Examples:
  asc beta-app-localizations update --id "LOCALIZATION_ID" --description "Updated copy"
  asc beta-app-localizations update --id "LOCALIZATION_ID" --feedback-email "qa@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return shared.RequiredFlag("--id")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			hasUpdates := visited["description"] ||
				visited["feedback-email"] ||
				visited["marketing-url"] ||
				visited["privacy-policy-url"] ||
				visited["tv-os-privacy-policy"]
			if !hasUpdates {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("")
			}

			attrs := asc.BetaAppLocalizationUpdateAttributes{}
			if visited["description"] {
				value := strings.TrimSpace(*description)
				attrs.Description = &value
			}
			if visited["feedback-email"] {
				value := strings.TrimSpace(*feedbackEmail)
				attrs.FeedbackEmail = &value
			}
			if visited["marketing-url"] {
				value := strings.TrimSpace(*marketingURL)
				attrs.MarketingURL = &value
			}
			if visited["privacy-policy-url"] {
				value := strings.TrimSpace(*privacyPolicyURL)
				attrs.PrivacyPolicyURL = &value
			}
			if visited["tv-os-privacy-policy"] {
				value := strings.TrimSpace(*tvOsPrivacyPolicy)
				attrs.TvOsPrivacyPolicy = &value
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-app-localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateBetaAppLocalization(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("beta-app-localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BetaAppLocalizationsDeleteCommand returns the delete subcommand.
func BetaAppLocalizationsDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc beta-app-localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:   "Delete a beta app localization.",
		LongHelp: `Delete a beta app localization.

Examples:
  asc beta-app-localizations delete --id "LOCALIZATION_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Beta app localization ID",
		IDType:             "betaAppLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "beta-app-localizations delete",
		DeleteErrorPrefix:  "beta-app-localizations delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteBetaAppLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.BetaAppLocalizationDeleteResult{ID: id, Deleted: true}
		},
	})
}
