package users

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newUsersInvitesVisibleAppsCommand returns the invites visible apps command group.
func newUsersInvitesVisibleAppsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("visible-apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "visible-apps",
		ShortUsage: "asc users invites visible-apps <subcommand> [flags]",
		ShortHelp:  "List visible apps for a user invitation.",
		LongHelp: `List visible apps for a user invitation.

Examples:
  asc users invites visible-apps list --id "INVITE_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newUsersInvitesVisibleAppsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newUsersInvitesVisibleAppsListCommand returns the invites visible apps list subcommand.
func newUsersInvitesVisibleAppsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "visible-apps list",
		Name:        "list",
		ShortUsage:  "asc users invites visible-apps list --id \"INVITE_ID\" [flags]",
		ShortHelp:   "List visible apps for a user invitation.",
		LongHelp: `List visible apps for a user invitation.

Examples:
  asc users invites visible-apps list --id "INVITE_ID"
  asc users invites visible-apps list --id "INVITE_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Invitation ID",
		ParentType:           "userInvitations",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		Deps:                 d,
		Flags: func(fs *flag.FlagSet) func() error {
			id, next := fs.Lookup("id").Value, fs.Lookup("next").Value
			return func() error {
				if strings.TrimSpace(id.String()) != "" || strings.TrimSpace(next.String()) == "" {
					return nil
				}
				if _, err := extractUserInvitationIDFromNextURL(next.String()); err != nil {
					return d.UsageErrorf("users invites visible-apps list: %v", err)
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetUserInvitationVisibleApps(ctx, id,
				asc.Limit[asc.UserInvitationVisibleAppsOption](limit),
				asc.NextURL[asc.UserInvitationVisibleAppsOption](next),
			)
		},
	})
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
