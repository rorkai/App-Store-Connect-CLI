package testflight

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBetaNotificationsCommand returns the beta notifications command group.
func newBetaNotificationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("beta-notifications", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-notifications",
		ShortUsage: "asc testflight beta-notifications <subcommand> [flags]",
		ShortHelp:  "Send TestFlight beta build notifications.",
		LongHelp: `Send TestFlight beta build notifications.

Examples:
  asc testflight beta-notifications create --build-id "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBetaNotificationsCreateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBetaNotificationsCreateCommand sends a beta notification for a build.
func newBetaNotificationsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc testflight beta-notifications create --build-id \"BUILD_ID\"",
		ShortHelp:  "Send a beta notification for a build.",
		LongHelp: `Send a beta notification for a build.

Examples:
  asc testflight beta-notifications create --build-id "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				return d.RequiredFlag("--build-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-notifications create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			detail, err := client.GetBuildBuildBetaDetail(requestCtx, trimmedBuildID)
			if err != nil {
				return fmt.Errorf("beta-notifications create: failed to inspect notification state: %w", err)
			}
			if detail.Data.Attributes.AutoNotifyEnabled {
				return d.PrintOutput(&asc.BuildBetaNotificationResponse{
					NotificationAction: asc.BuildBetaGroupsNotificationActionAutoNotifyEnabled,
				}, *output.Output, *output.Pretty)
			}

			resp, err := client.CreateBuildBetaNotification(requestCtx, trimmedBuildID)
			if err != nil {
				return fmt.Errorf("beta-notifications create: failed to send: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
