package web

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const webWarningText = "EXPERIMENTAL / UNOFFICIAL / DISCOURAGED: This command family uses private, undocumented Apple web-session endpoints (not the public App Store Connect API). These endpoints are not sanctioned by Apple for third-party use. Using them may violate Apple's Developer Program License Agreement and may result in account restrictions, lockouts, or termination. You use these commands entirely at your own risk. The authors of this tool accept no responsibility for any action Apple takes against your account. As a precaution, these commands enforce an intentionally low request rate (default 1 request/second) to avoid appearing as bot traffic, require user-owned Apple Account sessions, and redact signed URLs/tokens by default. They may break without notice and should not be used for production-critical automation."

// WebCommand returns the detached experimental web command group.
func WebCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "web",
		ShortUsage: "asc web <subcommand> [flags]",
		ShortHelp:  "[experimental] Unofficial web-session workflows (discouraged).",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Use Apple web-session /iris flows that are not part of the official App Store Connect API.
These commands can break without notice and are intentionally detached from official asc workflows.
Use ` + "`asc web apps create`" + ` as the canonical app-creation command when you need this unofficial path.

` + webWarningText + `

Examples:
  asc web auth status
  asc web sandbox create --first-name "Jane" --last-name "Tester" --email "jane+sandbox@example.com" --password "Passwordtest1" --territory "USA"
  asc web auth login --apple-id "user@example.com"
  asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123"
  asc web privacy plan --app "123456789" --file "./privacy.json"
  asc web review list --app "123456789" --apple-id "user@example.com"
  asc web review show --app "123456789" --apple-id "user@example.com"
  asc web review subscriptions list --app "123456789" --apple-id "user@example.com"
  asc web analytics overview --app "123456789" --start 2025-12-24 --end 2026-03-23`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAuthCommand(),
			WebSandboxCommand(),
			WebAppsCommand(),
			WebPrivacyCommand(),
			WebReviewCommand(),
			WebAnalyticsCommand(),
			WebXcodeCloudCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return flag.ErrHelp
			}
			fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n", strings.TrimSpace(args[0]))
			return flag.ErrHelp
		},
	}
}
