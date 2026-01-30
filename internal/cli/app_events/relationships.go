package app_events

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// AppEventsRelationshipsCommand returns the app event relationships command.
func AppEventsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc app-events relationships --event-id \"EVENT_ID\" [flags]",
		ShortHelp:  "List localization relationships for an in-app event.",
		LongHelp: `List localization relationships for an in-app event.

Examples:
  asc app-events relationships --event-id "EVENT_ID"
  asc app-events relationships --event-id "EVENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-events relationships: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("app-events relationships: %w", err)
			}

			id := strings.TrimSpace(*eventID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events relationships: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetAppEventLocalizationsRelationships(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events relationships: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventLocalizationsRelationships(ctx, id, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events relationships: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppEventLocalizationsRelationships(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("app-events relationships: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
