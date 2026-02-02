package gamecenter

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// GameCenterEnabledVersionsCommand returns the enabled versions command group.
func GameCenterEnabledVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("enabled-versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "enabled-versions",
		ShortUsage: "asc game-center enabled-versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center enabled versions.",
		LongHelp: `Manage Game Center enabled versions.

Examples:
  asc game-center enabled-versions list --app "APP_ID"
  asc game-center enabled-versions compatible-versions --id "ENABLED_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterEnabledVersionsListCommand(),
			GameCenterEnabledVersionsCompatibleVersionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterEnabledVersionsListCommand returns the enabled versions list subcommand.
func GameCenterEnabledVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center enabled-versions list [flags]",
		ShortHelp:  "List Game Center enabled versions for an app.",
		LongHelp: `List Game Center enabled versions for an app.

Examples:
  asc game-center enabled-versions list --app "APP_ID"
  asc game-center enabled-versions list --app "APP_ID" --limit 50
  asc game-center enabled-versions list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center enabled-versions list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("game-center enabled-versions list: %w", err)
			}
			format, err := normalizeEnabledVersionsOutput(*output, *pretty)
			if err != nil {
				return fmt.Errorf("game-center enabled-versions list: %w", err)
			}

			resolvedAppID := resolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("game-center enabled-versions list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCEnabledVersionsOption{
				asc.WithGCEnabledVersionsLimit(*limit),
				asc.WithGCEnabledVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := []asc.GCEnabledVersionsOption{asc.WithGCEnabledVersionsNextURL(*next)}
				if nextURL == "" {
					paginateOpts = []asc.GCEnabledVersionsOption{asc.WithGCEnabledVersionsLimit(200)}
				}
				firstPage, err := client.GetAppGameCenterEnabledVersions(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center enabled-versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppGameCenterEnabledVersions(ctx, resolvedAppID, asc.WithGCEnabledVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center enabled-versions list: %w", err)
				}

				return printOutput(resp, format, *pretty)
			}

			resp, err := client.GetAppGameCenterEnabledVersions(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("game-center enabled-versions list: failed to fetch: %w", err)
			}

			return printOutput(resp, format, *pretty)
		},
	}
}

// GameCenterEnabledVersionsCompatibleVersionsCommand returns the compatible versions subcommand.
func GameCenterEnabledVersionsCompatibleVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("compatible-versions", flag.ExitOnError)

	enabledVersionID := fs.String("id", "", "Game Center enabled version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "compatible-versions",
		ShortUsage: "asc game-center enabled-versions compatible-versions --id \"ENABLED_VERSION_ID\"",
		ShortHelp:  "List compatible Game Center enabled versions.",
		LongHelp: `List compatible Game Center enabled versions.

Examples:
  asc game-center enabled-versions compatible-versions --id "ENABLED_VERSION_ID"
  asc game-center enabled-versions compatible-versions --id "ENABLED_VERSION_ID" --limit 50
  asc game-center enabled-versions compatible-versions --id "ENABLED_VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center enabled-versions compatible-versions: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("game-center enabled-versions compatible-versions: %w", err)
			}
			format, err := normalizeEnabledVersionsOutput(*output, *pretty)
			if err != nil {
				return fmt.Errorf("game-center enabled-versions compatible-versions: %w", err)
			}

			id := strings.TrimSpace(*enabledVersionID)
			nextURL := strings.TrimSpace(*next)
			if id == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("game-center enabled-versions compatible-versions: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCEnabledVersionsOption{
				asc.WithGCEnabledVersionsLimit(*limit),
				asc.WithGCEnabledVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := []asc.GCEnabledVersionsOption{asc.WithGCEnabledVersionsNextURL(*next)}
				if nextURL == "" {
					paginateOpts = []asc.GCEnabledVersionsOption{asc.WithGCEnabledVersionsLimit(200)}
				}
				firstPage, err := client.GetGameCenterEnabledVersionCompatibleVersions(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center enabled-versions compatible-versions: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterEnabledVersionCompatibleVersions(ctx, id, asc.WithGCEnabledVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center enabled-versions compatible-versions: %w", err)
				}

				return printOutput(resp, format, *pretty)
			}

			resp, err := client.GetGameCenterEnabledVersionCompatibleVersions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center enabled-versions compatible-versions: failed to fetch: %w", err)
			}

			return printOutput(resp, format, *pretty)
		},
	}
}

func normalizeEnabledVersionsOutput(output string, pretty bool) (string, error) {
	format := strings.ToLower(strings.TrimSpace(output))
	if format == "" {
		format = "json"
	}
	if format == "md" {
		format = "markdown"
	}

	switch format {
	case "json":
		return format, nil
	case "table", "markdown":
		if pretty {
			return "", fmt.Errorf("--pretty is only valid with JSON output")
		}
		return format, nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}
