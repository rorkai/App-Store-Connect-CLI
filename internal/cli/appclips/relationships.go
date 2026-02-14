package appclips

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppClipDefaultExperiencesRelationshipsCommand returns the default experiences relationships subcommand.
func AppClipDefaultExperiencesRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("default-experiences-relationships", flag.ExitOnError)

	appClipID := fs.String("app-clip-id", "", "App Clip ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "default-experiences-relationships",
		ShortUsage: "asc app-clips default-experiences-relationships --app-clip-id \"CLIP_ID\" [flags]",
		ShortHelp:  "List default experience relationships for an App Clip.",
		LongHelp: `List default experience relationships for an App Clip.

Examples:
  asc app-clips default-experiences-relationships --app-clip-id "CLIP_ID"
  asc app-clips default-experiences-relationships --app-clip-id "CLIP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-clips default-experiences-relationships: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-clips default-experiences-relationships: %w", err)
			}

			appClipValue := strings.TrimSpace(*appClipID)
			if appClipValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app-clip-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences-relationships: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetAppClipDefaultExperiencesRelationships(requestCtx, appClipValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-clips default-experiences-relationships: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClipDefaultExperiencesRelationships(ctx, appClipValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips default-experiences-relationships: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppClipDefaultExperiencesRelationships(requestCtx, appClipValue, opts...)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences-relationships: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipAdvancedExperiencesRelationshipsCommand returns the advanced experiences relationships subcommand.
func AppClipAdvancedExperiencesRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("advanced-experiences-relationships", flag.ExitOnError)

	appClipID := fs.String("app-clip-id", "", "App Clip ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "advanced-experiences-relationships",
		ShortUsage: "asc app-clips advanced-experiences-relationships --app-clip-id \"CLIP_ID\" [flags]",
		ShortHelp:  "List advanced experience relationships for an App Clip.",
		LongHelp: `List advanced experience relationships for an App Clip.

Examples:
  asc app-clips advanced-experiences-relationships --app-clip-id "CLIP_ID"
  asc app-clips advanced-experiences-relationships --app-clip-id "CLIP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-clips advanced-experiences-relationships: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-clips advanced-experiences-relationships: %w", err)
			}

			appClipValue := strings.TrimSpace(*appClipID)
			if appClipValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app-clip-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences-relationships: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetAppClipAdvancedExperiencesRelationships(requestCtx, appClipValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-clips advanced-experiences-relationships: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClipAdvancedExperiencesRelationships(ctx, appClipValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips advanced-experiences-relationships: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppClipAdvancedExperiencesRelationships(requestCtx, appClipValue, opts...)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences-relationships: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
