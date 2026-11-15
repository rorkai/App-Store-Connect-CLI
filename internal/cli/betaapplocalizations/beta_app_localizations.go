package betaapplocalizations

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BetaAppLocalizationsCommand returns the beta-app-localizations command group
// built on the process dependencies.
func BetaAppLocalizationsCommand() *ffcli.Command {
	return NewBetaAppLocalizationsCommand(shared.DefaultDeps())
}

// NewBetaAppLocalizationsCommand returns the beta-app-localizations command
// group built on d.
func NewBetaAppLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("beta-app-localizations", flag.ExitOnError)

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
			newBetaAppLocalizationsListCommand(d),
			newBetaAppLocalizationsGetCommand(d),
			newBetaAppLocalizationsAppCommand(d),
			newBetaAppLocalizationsCreateCommand(d),
			newBetaAppLocalizationsUpdateCommand(d),
			newBetaAppLocalizationsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBetaAppLocalizationsListCommand returns the list subcommand built on d.
func newBetaAppLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

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
			if err := d.ValidatePaginationFlags(ctx, fs, "beta-app-localizations list", 200, "locale"); err != nil {
				return err
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			locales := shared.SplitCSV(*locale)
			if err := shared.ValidateBuildLocalizationLocales(locales); err != nil {
				return d.UsageErrorfCtx(ctx, "beta-app-localizations list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-app-localizations list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBetaAppLocalizations(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("beta-app-localizations list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBetaAppLocalizationsGetCommand returns the get subcommand built on d.
func newBetaAppLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc beta-app-localizations view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "View a beta app localization by ID.",
		LongHelp: `View a beta app localization by ID.

Examples:
  asc beta-app-localizations view --id "LOCALIZATION_ID"`,
		IDUsage:            "Beta app localization ID",
		IDType:             "betaAppLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBetaAppLocalization(ctx, id)
		},
	})
}

// newBetaAppLocalizationsCreateCommand returns the create subcommand built on d.
func newBetaAppLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	description := fs.String("description", "", "Beta app description")
	feedbackEmail := fs.String("feedback-email", "", "Feedback email")
	marketingURL := fs.String("marketing-url", "", "Marketing URL")
	privacyPolicyURL := fs.String("privacy-policy-url", "", "Privacy policy URL")
	tvOsPrivacyPolicy := fs.String("tv-os-privacy-policy", "", "tvOS privacy policy")
	output := d.BindOutputFlags(fs)

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
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				return d.RequiredFlag("--locale")
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return d.UsageErrorfCtx(ctx, "beta-app-localizations create: %v", err)
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

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-app-localizations create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateBetaAppLocalization(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("beta-app-localizations create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBetaAppLocalizationsUpdateCommand returns the update subcommand built on d.
func newBetaAppLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaAppLocalizations", "Beta app localization ID")
	description := fs.String("description", "", "Beta app description")
	feedbackEmail := fs.String("feedback-email", "", "Feedback email")
	marketingURL := fs.String("marketing-url", "", "Marketing URL")
	privacyPolicyURL := fs.String("privacy-policy-url", "", "Privacy policy URL")
	tvOsPrivacyPolicy := fs.String("tv-os-privacy-policy", "", "tvOS privacy policy")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--id")
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
				return d.UpdateFlagRequired(fs, description, feedbackEmail, marketingURL, privacyPolicyURL, tvOsPrivacyPolicy)
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

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-app-localizations update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateBetaAppLocalization(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("beta-app-localizations update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBetaAppLocalizationsDeleteCommand returns the delete subcommand built on d.
func newBetaAppLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:       d,
		Name:       "delete",
		ShortUsage: "asc beta-app-localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete a beta app localization.",
		LongHelp: `Delete a beta app localization.

Examples:
  asc beta-app-localizations delete --id "LOCALIZATION_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Beta app localization ID",
		IDType:             "betaAppLocalizations",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteBetaAppLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}
