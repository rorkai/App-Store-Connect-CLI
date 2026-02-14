package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BetaNotificationsCommand returns the beta notifications command group.
func BetaNotificationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-notifications", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-notifications",
		ShortUsage: "asc testflight beta-notifications <subcommand> [flags]",
		ShortHelp:  "Send TestFlight beta build notifications.",
		LongHelp: `Send TestFlight beta build notifications.

Examples:
  asc testflight beta-notifications create --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaNotificationsCreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaNotificationsCreateCommand sends a beta notification for a build.
func BetaNotificationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc testflight beta-notifications create --build \"BUILD_ID\"",
		ShortHelp:  "Send a beta notification for a build.",
		LongHelp: `Send a beta notification for a build.

Examples:
  asc testflight beta-notifications create --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-notifications create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateBuildBetaNotification(requestCtx, trimmedBuildID)
			if err != nil {
				return fmt.Errorf("beta-notifications create: failed to send: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
