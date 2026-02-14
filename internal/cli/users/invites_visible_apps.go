package users

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// UsersInvitesVisibleAppsCommand returns the invites visible apps command group.
func UsersInvitesVisibleAppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("visible-apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "visible-apps",
		ShortUsage: "asc users invites visible-apps <subcommand> [flags]",
		ShortHelp:  "List visible apps for a user invitation.",
		LongHelp: `List visible apps for a user invitation.

Examples:
  asc users invites visible-apps list --id "INVITE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			UsersInvitesVisibleAppsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// UsersInvitesVisibleAppsListCommand returns the invites visible apps list subcommand.
func UsersInvitesVisibleAppsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("visible-apps list", flag.ExitOnError)

	id := fs.String("id", "", "Invitation ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc users invites visible-apps list --id \"INVITE_ID\" [flags]",
		ShortHelp:  "List visible apps for a user invitation.",
		LongHelp: `List visible apps for a user invitation.

Examples:
  asc users invites visible-apps list --id "INVITE_ID"
  asc users invites visible-apps list --id "INVITE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("users invites visible-apps list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("users invites visible-apps list: %w", err)
			}
			if idValue == "" && strings.TrimSpace(*next) != "" {
				derivedID, err := extractUserInvitationIDFromNextURL(*next)
				if err != nil {
					return fmt.Errorf("users invites visible-apps list: %w", err)
				}
				idValue = derivedID
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users invites visible-apps list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.UserInvitationVisibleAppsOption{
				asc.WithUserInvitationVisibleAppsLimit(*limit),
				asc.WithUserInvitationVisibleAppsNextURL(*next),
			}

			if *paginate {
				if idValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithUserInvitationVisibleAppsLimit(200))
				firstPage, err := client.GetUserInvitationVisibleApps(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("users invites visible-apps list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetUserInvitationVisibleApps(ctx, idValue, asc.WithUserInvitationVisibleAppsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("users invites visible-apps list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetUserInvitationVisibleApps(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("users invites visible-apps list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func extractUserInvitationIDFromNextURL(nextURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(nextURL))
	if err != nil {
		return "", fmt.Errorf("invalid --next URL")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "v1" || parts[1] != "userInvitations" {
		return "", fmt.Errorf("invalid --next URL")
	}
	if strings.TrimSpace(parts[2]) == "" {
		return "", fmt.Errorf("invalid --next URL")
	}
	if parts[3] == "visibleApps" {
		return parts[2], nil
	}
	if len(parts) >= 5 && parts[3] == "relationships" && parts[4] == "visibleApps" {
		return parts[2], nil
	}
	return "", fmt.Errorf("invalid --next URL")
}
