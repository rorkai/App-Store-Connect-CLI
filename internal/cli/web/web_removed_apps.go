package web

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var listRemovedWebAppsFn = func(ctx context.Context, client *webcore.Client, opts webcore.RemovedAppsListOptions) (*webcore.RemovedAppsListResponse, error) {
	return client.ListRemovedApps(ctx, opts)
}

var restoreRemovedWebAppFn = func(ctx context.Context, client *webcore.Client, appID string) error {
	_, err := client.RestoreApp(ctx, appID)
	return err
}

var setRemovedWebAppPermissionFn = func(ctx context.Context, client *webcore.Client, appID, access string) error {
	return client.SetUserAppPermission(ctx, appID, access)
}

// WebRemovedAppsCommand returns the removed apps command group.
func WebRemovedAppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web removed-apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "removed-apps",
		ShortUsage: "asc web removed-apps <subcommand> [flags]",
		ShortHelp:  "Inspect removed apps via web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Inspect apps shown in App Store Connect's Removed Apps status view.

Examples:
  asc web removed-apps list
  asc web removed-apps list --paginate --output table
  asc web removed-apps list --apple-id "user@example.com" --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebRemovedAppsListCommand(),
			WebRemovedAppsRestoreCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebRemovedAppsListCommand lists removed apps using the internal web API.
func WebRemovedAppsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web removed-apps list", flag.ExitOnError)

	authFlags := bindWebSessionFlagsWithSessionFromEnv(fs)
	limit := fs.Int("limit", webcore.DefaultRemovedAppsLimit, fmt.Sprintf("Maximum results per page (1-%d)", webcore.MaxRemovedAppsLimit))
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web removed-apps list [flags]",
		ShortHelp:  "List removed apps via Apple web sessions.",
		LongHelp: `List apps from App Store Connect's Removed Apps status view using the
Apple web-session API.

By default, this command uses the cached web session. Pass --session-from-env
to use ASC_WEB_SESSION in memory without persistence. This mode does not read
or write the session cache or keychain.

Examples:
  asc web removed-apps list
  asc web removed-apps list --session-from-env --output json
  asc web removed-apps list --limit 25 --output table
  asc web removed-apps list --paginate --output json
  asc web removed-apps list --next "<links.next>"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit < 1 || *limit > webcore.MaxRemovedAppsLimit {
				return shared.UsageError(fmt.Sprintf("web removed-apps list: --limit must be between 1 and %d", webcore.MaxRemovedAppsLimit))
			}
			nextValue := strings.TrimSpace(*next)
			if nextValue != "" && *paginate {
				return shared.UsageError("web removed-apps list: --next cannot be combined with --paginate")
			}
			if err := validateRemovedAppsNextURL(nextValue); err != nil {
				return shared.UsageError("web removed-apps list: " + err.Error())
			}
			if err := shared.RejectNextFlagConflicts(fs, nextValue, "web removed-apps list", "limit"); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web removed-apps list")
			}

			result, err := listRemovedWebAppsFn(requestCtx, newWebClientFn(session), webcore.RemovedAppsListOptions{
				Limit:    *limit,
				Next:     nextValue,
				Paginate: *paginate,
			})
			if err != nil {
				return withWebAuthHint(err, "web removed-apps list")
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderRemovedAppsTable(result) },
				func() error { return renderRemovedAppsMarkdown(result) },
			)
		},
	}
}

// WebRemovedAppsRestoreCommand restores a removed app and configures access.
func WebRemovedAppsRestoreCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web removed-apps restore", flag.ExitOnError)
	app := fs.String("app", "", "App Store Connect app ID")
	access := fs.String("access", "", "Access mode: limited or full")
	confirm := fs.Bool("confirm", false, "Confirm restoring this app (required)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "restore", ShortUsage: "asc web removed-apps restore --app APP_ID --access limited|full --confirm [flags]", ShortHelp: "Restore a removed app via Apple web API.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id, mode := strings.TrimSpace(*app), strings.ToLower(strings.TrimSpace(*access))
			if id == "" {
				return shared.UsageError("--app is required")
			}
			if mode != "limited" && mode != "full" {
				return shared.UsageError("--access must be limited or full")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web removed-apps restore")
			}
			client := newWebClientFn(session)
			if err = restoreRemovedWebAppFn(requestCtx, client, id); err != nil {
				return withWebAuthHint(fmt.Errorf("web removed-apps restore failed: app PATCH: %w", err), "web removed-apps restore")
			}
			verified, err := getWebAppRemovalStateFn(requestCtx, client, id)
			if err != nil {
				return fmt.Errorf("web removed-apps restore failed: could not verify removed state: %w", err)
			}
			if verified == nil || !verified.RemovedKnown || verified.Removed {
				return fmt.Errorf("web removed-apps restore failed: Apple did not confirm app %q is restored", id)
			}
			if err = setRemovedWebAppPermissionFn(requestCtx, client, id, mode); err != nil {
				return withWebAuthHint(fmt.Errorf("web removed-apps restore failed: app was restored but access update failed: %w", err), "web removed-apps restore")
			}
			return shared.PrintOutput(&asc.WebRemovedAppRestoreResult{AppID: id, Access: mode, Removed: false, PermissionWritten: true}, *output.Output, *output.Pretty)
		},
	}
}

func validateRemovedAppsNextURL(next string) error {
	next = strings.TrimSpace(next)
	if next == "" {
		return nil
	}
	parsed, err := url.Parse(next)
	if err != nil {
		return fmt.Errorf("--next must be a valid URL: %w", err)
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "appstoreconnect.apple.com") {
			return fmt.Errorf("--next must be an App Store Connect web URL")
		}
	} else if parsed.Host != "" {
		return fmt.Errorf("--next must be an App Store Connect web URL")
	}
	if path := parsed.EscapedPath(); path != "/iris/v1/apps" && path != "/apps" {
		return fmt.Errorf("--next must be an App Store Connect web URL")
	}
	if parsed.Query().Get("filter[removed]") != "true" {
		return fmt.Errorf("--next must include filter[removed]=true")
	}
	return nil
}

func removedAppsHeaders() []string {
	return []string{"ID", "Name", "Bundle ID", "Version", "Status", "SKU"}
}

func removedAppsRows(result *webcore.RemovedAppsListResponse) [][]string {
	if result == nil {
		return nil
	}
	rows := make([][]string, 0, len(result.Data))
	for _, app := range result.Data {
		rows = append(rows, []string{
			shared.OrNA(app.ID),
			shared.OrNA(app.Name),
			shared.OrNA(app.BundleID),
			shared.OrNA(app.VersionSummary),
			shared.OrNA(app.Status),
			shared.OrNA(app.SKU),
		})
	}
	return rows
}

func renderRemovedAppsTable(result *webcore.RemovedAppsListResponse) error {
	asc.RenderTable(removedAppsHeaders(), removedAppsRows(result))
	return nil
}

func renderRemovedAppsMarkdown(result *webcore.RemovedAppsListResponse) error {
	asc.RenderMarkdown(removedAppsHeaders(), removedAppsRows(result))
	return nil
}
