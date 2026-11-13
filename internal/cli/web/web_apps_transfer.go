package web

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

// WebAppsTransferCommand exposes read-only app-transfer information.
func WebAppsTransferCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps transfer", flag.ExitOnError)
	return &ffcli.Command{
		Name: "transfer", ShortUsage: "asc web apps transfer <subcommand> [flags]",
		ShortHelp: "Read app-transfer status via a web session.",
		LongHelp: `Read the transfer request attached to an app in App Store Connect.
Initiate, accept, cancel, and decline remain manual Apple workflows.`,
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{WebAppsTransferStatusCommand()},
		Exec:        func(context.Context, []string) error { return flag.ErrHelp },
	}
}

// WebAppsTransferStatusCommand reads Apple's appTransferRequest relationship.
func WebAppsTransferStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps transfer status", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "status", ShortUsage: "asc web apps transfer status --app APP_ID [flags]",
		ShortHelp: "Read the app-attached transfer request and state.",
		LongHelp: `Read the app through Apple's web-session API with appTransferRequest included.
JSON preserves Apple's full response envelope. Table and Markdown report request
presence as none for explicit null, present for a resource reference, or unknown
when Apple omits linkage. Missing state stays unknown; returned state values are
not normalized. A null relationship does not prove the app is eligible to transfer.

This reads one app, not the recipient's transfer list or the legacy prerequisite
page. It does not initiate, accept, cancel, or decline a transfer.

Examples:
  asc web apps transfer status --app 6759231657
  asc web apps transfer status --app 6759231657 --output json`,
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolved := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolved == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web apps transfer status")
			}
			result, err := newWebClientFn(session).GetAppTransferStatus(requestCtx, resolved)
			if err != nil {
				return withWebAuthHint(err, "web apps transfer status")
			}
			state := result.State
			if strings.TrimSpace(state) == "" {
				state = "unknown"
			}
			headers := []string{"App ID", "Request", "Transfer ID", "State"}
			rows := [][]string{{result.AppID, result.Presence, shared.OrNA(result.RequestID), state}}
			return shared.PrintOutputWithRenderers(
				result, *output.Output, *output.Pretty,
				func() error { asc.RenderTable(headers, rows); return nil },
				func() error { asc.RenderMarkdown(headers, rows); return nil },
			)
		},
	}
}
