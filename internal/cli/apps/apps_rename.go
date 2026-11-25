package apps

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// newAppsRenameCommand returns the apps rename subcommand.
func newAppsRenameCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apps rename", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := shared.BindResourceIDFlag(fs, "app-info", "appInfos", "App Info ID (optional override)")
	locale := fs.String("locale", "", "App name locale (e.g., en-US) (required)")
	name := fs.String("name", "", "New localized app name (required)")
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appSelector := d.ResolveAppSelector(*appID)
			resolvedAppID := strings.TrimSpace(appSelector.Value)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				return d.RequiredFlag("--locale")
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return d.RequiredFlag("--name")
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return d.UsageError(err.Error())
			}
			for _, issue := range validation.AppInfoLocalizationLengthIssues(validation.AppInfoLocalization{Name: nameValue}) {
				return d.UsageErrorf("--%s exceeds %d %s", issue.Field, issue.Limit, issue.Unit)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("apps rename: %w", err)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resolvedAppID, err = shared.ResolveScopedAppID(requestCtx, client, appSelector)
			if err != nil {
				return fmt.Errorf("apps rename: resolve app: %w", err)
			}

			plan, err := d.PlanAppInfoLocalizationUpsert(
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
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
