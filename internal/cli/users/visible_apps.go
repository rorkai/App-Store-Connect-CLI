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

// newUsersVisibleAppsCommand returns the visible apps command group.
func newUsersVisibleAppsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("visible-apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "visible-apps",
		ShortUsage: "asc users visible-apps <subcommand> [flags]",
		ShortHelp:  "View visible apps for a user.",
		LongHelp: `View visible apps for a user.

Examples:
  asc users visible-apps list --id "USER_ID"
  asc users visible-apps view --id "USER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newUsersVisibleAppsListCommand(d),
			newUsersVisibleAppsGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newUsersVisibleAppsListCommand returns the visible apps list subcommand.
func newUsersVisibleAppsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "visible-apps list",
		Name:        "list",
		ShortUsage:  "asc users visible-apps list --id \"USER_ID\" [flags]",
		ShortHelp:   "List visible apps for a user.",
		LongHelp: `List visible apps for a user.

Examples:
  asc users visible-apps list --id "USER_ID"
  asc users visible-apps list --id "USER_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "User ID",
		ParentType:           "users",
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
				if _, err := extractUserIDFromNextURL(next.String()); err != nil {
					return d.UsageErrorf("users visible-apps list: %v", err)
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetUserVisibleApps(ctx, id,
				asc.Limit[asc.UserVisibleAppsOption](limit),
				asc.NextURL[asc.UserVisibleAppsOption](next),
			)
		},
	})
}

// newUsersVisibleAppsGetCommand returns the visible apps relationship get subcommand.
func newUsersVisibleAppsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "visible-apps view",
		Name:        "view",
		ShortUsage:  "asc users visible-apps view --id \"USER_ID\" [flags]",
		ShortHelp:   "View visible app relationships for a user.",
		LongHelp: `View visible app relationships for a user.

Examples:
  asc users visible-apps view --id "USER_ID"
  asc users visible-apps view --id "USER_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "User ID",
		ParentType:           "users",
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
				if _, err := extractUserIDFromNextURL(next.String()); err != nil {
					return d.UsageErrorf("users visible-apps view: %v", err)
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetUserVisibleAppsRelationships(ctx, id,
				asc.Limit[asc.LinkagesOption](limit),
				asc.NextURL[asc.LinkagesOption](next),
			)
		},
	})
}

func extractUserIDFromNextURL(nextURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(nextURL))
	if err != nil {
		return "", fmt.Errorf("invalid --next URL")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "v1" || parts[1] != "users" {
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
