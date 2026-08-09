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

// WebCommand returns the web command group.
func WebCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "web",
		ShortUsage: "asc web <subcommand> [flags]",
		ShortHelp:  "Apple web-session workflows.",
		LongHelp: `WEB SESSION WORKFLOWS

Use Apple web sessions for App Store Connect and Developer Portal workflows.
Use ` + "`asc web apps create`" + ` as the canonical app-creation command in this family.

Examples:
  asc web auth status
  asc web api-keys create --name "Release automation"
  asc web sandbox create --first-name "Jane" --last-name "Tester" --email "jane+sandbox@example.com" --password "Passwordtest1" --territory "USA"
  asc web auth login --apple-id "user@example.com"
  asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123"
  asc web removed-apps list --paginate
  asc web privacy plan --app "123456789" --file "./privacy.json"
  asc web review list --app "123456789" --apple-id "user@example.com"
  asc web review show --app "123456789" --apple-id "user@example.com"
  asc web review subscriptions list --app "123456789" --apple-id "user@example.com"
  asc web subscriptions availability remove-from-sale --subscription-id "SUB_ID" --confirm
  asc web analytics overview --app "123456789" --start 2025-12-24 --end 2026-03-23`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAuthCommand(),
			WebAPIKeysCommand(),
			WebSandboxCommand(),
			WebAppsCommand(),
			WebRemovedAppsCommand(),
			WebBundleIDsCommand(),
			WebPrivacyCommand(),
			WebReviewCommand(),
			WebSubscriptionsCommand(),
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
