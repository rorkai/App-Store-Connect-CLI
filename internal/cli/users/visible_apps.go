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

// UsersVisibleAppsCommand returns the visible apps command group.
func UsersVisibleAppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("visible-apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "visible-apps",
		ShortUsage: "asc users visible-apps <subcommand> [flags]",
		ShortHelp:  "View visible apps for a user.",
		LongHelp: `View visible apps for a user.

Examples:
  asc users visible-apps list --id "USER_ID"
  asc users visible-apps get --id "USER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			UsersVisibleAppsListCommand(),
			UsersVisibleAppsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// UsersVisibleAppsListCommand returns the visible apps list subcommand.
func UsersVisibleAppsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("visible-apps list", flag.ExitOnError)

	id := fs.String("id", "", "User ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc users visible-apps list --id \"USER_ID\" [flags]",
		ShortHelp:  "List visible apps for a user.",
		LongHelp: `List visible apps for a user.

Examples:
  asc users visible-apps list --id "USER_ID"
  asc users visible-apps list --id "USER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("users visible-apps list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("users visible-apps list: %w", err)
			}
			if idValue == "" && strings.TrimSpace(*next) != "" {
				derivedID, err := extractUserIDFromNextURL(*next)
				if err != nil {
					return fmt.Errorf("users visible-apps list: %w", err)
				}
				idValue = derivedID
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users visible-apps list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.UserVisibleAppsOption{
				asc.WithUserVisibleAppsLimit(*limit),
				asc.WithUserVisibleAppsNextURL(*next),
			}

			if *paginate {
				if idValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithUserVisibleAppsLimit(200))
				firstPage, err := client.GetUserVisibleApps(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("users visible-apps list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetUserVisibleApps(ctx, idValue, asc.WithUserVisibleAppsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("users visible-apps list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetUserVisibleApps(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("users visible-apps list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// UsersVisibleAppsGetCommand returns the visible apps relationship get subcommand.
func UsersVisibleAppsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("visible-apps get", flag.ExitOnError)

	id := fs.String("id", "", "User ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc users visible-apps get --id \"USER_ID\" [flags]",
		ShortHelp:  "Get visible app relationships for a user.",
		LongHelp: `Get visible app relationships for a user.

Examples:
  asc users visible-apps get --id "USER_ID"
  asc users visible-apps get --id "USER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("users visible-apps get: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("users visible-apps get: %w", err)
			}
			if idValue == "" && strings.TrimSpace(*next) != "" {
				derivedID, err := extractUserIDFromNextURL(*next)
				if err != nil {
					return fmt.Errorf("users visible-apps get: %w", err)
				}
				idValue = derivedID
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("users visible-apps get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				if idValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetUserVisibleAppsRelationships(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("users visible-apps get: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetUserVisibleAppsRelationships(ctx, idValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("users visible-apps get: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetUserVisibleAppsRelationships(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("users visible-apps get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
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
