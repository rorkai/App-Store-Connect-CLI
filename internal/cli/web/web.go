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

const webWarningText = "EXPERIMENTAL / UNOFFICIAL / DISCOURAGED: This command family uses Apple web-session behavior (not the public App Store Connect API). It may break anytime and should not be used for production-critical automation."

// WebCommand returns the detached experimental web command group.
func WebCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "web",
		ShortUsage: "asc web <subcommand> [flags]",
		ShortHelp:  "EXPERIMENTAL: Unofficial web-session workflows (discouraged).",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Use Apple web-session flows that are not part of the official App Store Connect API.
These commands can break without notice and are intentionally detached from official asc workflows.

` + webWarningText + `

Examples:
  asc web auth status
  asc web auth login --apple-id "user@example.com" --password-stdin
  asc web apps create --name "My App" --bundle-id "com.example.app" --sku "MYAPP123" --apple-id "user@example.com" --password-stdin`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAuthCommand(),
			WebAppsCommand(),
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
