package apps

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// AppsRenameCommand returns the apps rename subcommand.
func AppsRenameCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps rename", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override)")
	locale := fs.String("locale", "", "App name locale (e.g., en-US) (required)")
	name := fs.String("name", "", "New localized app name (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "rename",
		ShortUsage: "asc apps rename --app APP_ID --locale LOCALE --name NAME [flags]",
		ShortHelp:  "Rename an app for one App Store locale.",
		LongHelp: `Create or update the localized App Store name for an app.

If the locale does not exist, a new app info localization is created.
Use --app-info to select a specific App Info record when the app has multiple records.

Examples:
  asc apps rename --app "APP_ID" --locale "en-US" --name "New Name"
  asc apps rename --app "APP_ID" --app-info "APP_INFO_ID" --locale "fr-FR" --name "Nouveau nom"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: apps rename does not accept positional arguments")
				return flag.ErrHelp
			}

			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}
			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return shared.MissingRequiredUsageError()
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError()
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return shared.UsageError(err.Error())
			}
			for _, issue := range validation.AppInfoLocalizationLengthIssues(validation.AppInfoLocalization{Name: nameValue}) {
				return shared.UsageErrorf("--%s exceeds %d %s", issue.Field, issue.Limit, issue.Unit)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps rename: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			plan, err := shared.PlanAppInfoLocalizationUpsert(
				requestCtx,
				client,
				resolvedAppID,
				strings.TrimSpace(*appInfoID),
				localeValue,
				map[string]string{"name": nameValue},
			)
			if err != nil {
				return fmt.Errorf("apps rename: %w", err)
			}
			resp, action, err := shared.ApplyAppInfoLocalizationUpsert(requestCtx, client, plan)
			if err != nil {
				return fmt.Errorf("apps rename: %w", err)
			}
			if resp == nil {
				return fmt.Errorf("apps rename: empty app info localization response")
			}

			result := &asc.AppRenameResult{
				AppID:          resolvedAppID,
				AppInfoID:      plan.AppInfoID,
				Locale:         localeValue,
				Name:           nameValue,
				Action:         action,
				LocalizationID: strings.TrimSpace(resp.Data.ID),
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
