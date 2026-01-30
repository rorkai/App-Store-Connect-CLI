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
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaAppLocalizationsListCommand(),
			BetaAppLocalizationsGetCommand(),
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
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("beta-app-localizations list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("beta-app-localizations list: %w", err)
			}

			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			locales := splitCSV(*locale)
			if err := shared.ValidateBuildLocalizationLocales(locales); err != nil {
				return fmt.Errorf("beta-app-localizations list: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-app-localizations list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaAppLocalizationsOption{
				asc.WithBetaAppLocalizationsLimit(*limit),
				asc.WithBetaAppLocalizationsNextURL(*next),
			}
			if resolvedAppID != "" {
				opts = append(opts, asc.WithBetaAppLocalizationAppIDs([]string{resolvedAppID}))
			}
			if len(locales) > 0 {
				opts = append(opts, asc.WithBetaAppLocalizationLocales(locales))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBetaAppLocalizationsLimit(200))
				firstPage, err := client.GetBetaAppLocalizations(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("beta-app-localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBetaAppLocalizations(ctx, asc.WithBetaAppLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("beta-app-localizations list: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBetaAppLocalizations(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("beta-app-localizations list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// BetaAppLocalizationsGetCommand returns the get subcommand.
func BetaAppLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Beta app localization ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc beta-app-localizations get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get a beta app localization by ID.",
		LongHelp: `Get a beta app localization by ID.

Examples:
  asc beta-app-localizations get --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-app-localizations get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaAppLocalization(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("beta-app-localizations get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
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
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc beta-app-localizations create [flags]",
		ShortHelp:  "Create a beta app localization.",
		LongHelp: `Create a beta app localization.

Examples:
  asc beta-app-localizations create --app "APP_ID" --locale "en-US"
  asc beta-app-localizations create --app "APP_ID" --locale "en-US" --description "Welcome testers"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return fmt.Errorf("beta-app-localizations create: %w", err)
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

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-app-localizations create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateBetaAppLocalization(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("beta-app-localizations create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// BetaAppLocalizationsUpdateCommand returns the update subcommand.
func BetaAppLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Beta app localization ID")
	description := fs.String("description", "", "Beta app description")
	feedbackEmail := fs.String("feedback-email", "", "Feedback email")
	marketingURL := fs.String("marketing-url", "", "Marketing URL")
	privacyPolicyURL := fs.String("privacy-policy-url", "", "Privacy policy URL")
	tvOsPrivacyPolicy := fs.String("tv-os-privacy-policy", "", "tvOS privacy policy")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc beta-app-localizations update [flags]",
		ShortHelp:  "Update a beta app localization.",
		LongHelp: `Update a beta app localization.

Examples:
  asc beta-app-localizations update --id "LOCALIZATION_ID" --description "Updated copy"
  asc beta-app-localizations update --id "LOCALIZATION_ID" --feedback-email "qa@example.com"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
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
				return flag.ErrHelp
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

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-app-localizations update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateBetaAppLocalization(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("beta-app-localizations update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// BetaAppLocalizationsDeleteCommand returns the delete subcommand.
func BetaAppLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Beta app localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc beta-app-localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete a beta app localization.",
		LongHelp: `Delete a beta app localization.

Examples:
  asc beta-app-localizations delete --id "LOCALIZATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-app-localizations delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaAppLocalization(requestCtx, idValue); err != nil {
				return fmt.Errorf("beta-app-localizations delete: failed to delete: %w", err)
			}

			result := &asc.BetaAppLocalizationDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}
