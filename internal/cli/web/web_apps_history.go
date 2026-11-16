package web

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var getWebAppStatusHistoryFn = func(ctx context.Context, client *webcore.Client, appID, versionID string) (*webcore.AppStatusHistory, error) {
	return client.GetAppStatusHistory(ctx, appID, versionID)
}

// newWebAppsHistoryCommand returns the app status history command.
func newWebAppsHistoryCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps history", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	versionID := fs.String("version-id", "", "Read status history for a single app store version ID")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "history",
		ShortUsage: "asc web apps history --app APP_ID [--version-id VERSION_ID] [flags]",
		ShortHelp:  "View App Store version status history.",
		LongHelp: `WEB SESSION WORKFLOWS

View the app status history App Store Connect shows under an app's History
view: each recorded App Store version status change with its date and the actor
that initiated it.

The public App Store Connect API has no status-history endpoint, so this reads
Apple's internal per-version state changes. App Store Connect records history
per app store version and exposes no app-level history resource, so this lists
the app's versions and then reads each version's state changes. Use
--version-id to read a single version and skip that fan-out.

Both reads follow Apple's pagination links internally, so there is no
--paginate flag.

The fan-out issues one request per version under a single request timeout, so
an app with a long release history can exceed the 30s default. Scope the read
with --version-id, or raise ASC_TIMEOUT.

Viewing status history is gated by the App Status History role capability, so
accounts without it can get an authorization error even when the app is
readable.

Examples:
  asc web apps history --app 6759231657
  asc web apps history --app 6759231657 --version-id 123456789
  asc web apps history --app 6759231657 --output json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(d.ResolveAppID(*appID))
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			var result *webcore.AppStatusHistory
			err = withWebSpinnerWith(d, "Fetching app status history", func() error {
				var err error
				result, err = getWebAppStatusHistoryFn(requestCtx, newWebClientFn(session), resolvedAppID, strings.TrimSpace(*versionID))
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps history")
			}

			return printWebAppStatusHistory(d, result, *output.Output, *output.Pretty)
		},
	}
}

var webAppStatusHistoryHeaders = []string{"version", "platform", "status", "date", "initiator"}

func printWebAppStatusHistory(d *shared.Deps, result *webcore.AppStatusHistory, output string, pretty bool) error {
	return d.PrintOutputWithRenderers(
		result,
		output,
		pretty,
		func() error {
			asc.RenderTableTo(d.IO.Out, webAppStatusHistoryHeaders, webAppStatusHistoryRows(result))
			return nil
		},
		func() error {
			asc.RenderMarkdownTo(d.IO.Out, webAppStatusHistoryHeaders, webAppStatusHistoryRows(result))
			return nil
		},
	)
}

func webAppStatusHistoryRows(result *webcore.AppStatusHistory) [][]string {
	if result == nil {
		return nil
	}
	rows := make([][]string, 0)
	for _, version := range result.Versions {
		versionLabel := version.VersionString
		if strings.TrimSpace(versionLabel) == "" {
			versionLabel = version.VersionID
		}
		for _, change := range version.Changes {
			status := change.AppStoreState
			if strings.TrimSpace(status) == "" {
				status = change.AppVersionState
			}
			rows = append(rows, []string{
				webAppValueOrUnknown(versionLabel),
				webAppValueOrUnknown(version.Platform),
				webAppValueOrUnknown(status),
				webAppValueOrUnknown(change.Date),
				webAppValueOrUnknown(change.Initiator),
			})
		}
	}
	return rows
}
