package shared

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// NewAvailabilityViewCommand builds the app availability view command under
// group, the command path without "asc " (`pricing availability`).
func NewAvailabilityViewCommand(d *Deps, group string) *ffcli.Command {
	name := group + " view"
	fs := d.NewFlagSet(name, flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	id := BindResourceIDFlag(fs, "id", "appAvailabilities", "App availability ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: fmt.Sprintf("asc %s --app \"APP_ID\" | asc %s --id \"AVAILABILITY_ID\"", name, name),
		ShortHelp:  "View app availability.",
		LongHelp: fmt.Sprintf(`View app availability.

Examples:
  asc %s --app "123456789"
  asc %s --id "AVAILABILITY_ID"`, name, name),
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			appValue := ""
			if idValue == "" {
				appValue = d.ResolveAppID(*appID)
			}
			if idValue == "" && appValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app or --id is required (or set ASC_APP_ID)")
				return MissingRequiredUsageError("")
			}
			if idValue != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --id and --app are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			var resp *asc.AppAvailabilityV2Response
			if idValue != "" {
				resp, err = client.GetAppAvailabilityV2ByID(requestCtx, idValue)
			} else {
				resp, err = client.GetAppAvailabilityV2(requestCtx, appValue)
			}
			if err != nil {
				if idValue == "" && asc.IsMissingResourceOfType(err, "appAvailabilities") {
					safeAppID := asc.SanitizeTerminalText(appValue)
					fmt.Fprintf(d.IO.ErrOut, "App %s has no availability configured yet; create it with: asc pricing availability create --app %s --territory \"USA\" --available true --available-in-new-territories true\n", safeAppID, safeAppID)
					return NewNotConfiguredReportedError(fmt.Errorf("%s: app %q has no availability configured", name, appValue))
				}
				if idValue == "" && IsAppAvailabilityMissing(err) {
					return NewErrorWithCause(
						fmt.Errorf("%s: app availability not found for app %q: %w", name, appValue, asc.ErrNotFound),
						err,
					)
				}
				return fmt.Errorf("%s: %w", name, err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
