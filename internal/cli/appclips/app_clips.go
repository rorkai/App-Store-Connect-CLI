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

// AppClipsCommand returns the app-clips command group.
func AppClipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-clips", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-clips",
		ShortUsage: "asc app-clips <subcommand> [flags]",
		ShortHelp:  "Manage App Clip experiences and invocations.",
		LongHelp: `Manage App Clip experiences and invocations.

Examples:
  asc app-clips list --app "APP_ID"
  asc app-clips get --id "CLIP_ID"
  asc app-clips default-experiences list --app-clip-id "CLIP_ID"
  asc app-clips advanced-experiences create --app "APP_ID" --bundle-id "com.example.clip" --link "https://example.com" --default-language EN --is-powered-by
  asc app-clips invocations list --build-bundle-id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipsListCommand(),
			AppClipsGetCommand(),
			AppClipDefaultExperiencesCommand(),
			AppClipAdvancedExperiencesCommand(),
			AppClipDefaultExperiencesRelationshipsCommand(),
			AppClipAdvancedExperiencesRelationshipsCommand(),
			AppClipHeaderImagesCommand(),
			AppClipInvocationsCommand(),
			AppClipDomainStatusCommand(),
			AppClipReviewDetailsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipsListCommand returns the app clips list subcommand.
func AppClipsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	bundleID := fs.String("bundle-id", "", "Filter by bundle ID(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-clips list --app \"APP_ID\" [flags]",
		ShortHelp:  "List App Clips for an app.",
		LongHelp: `List App Clips for an app.

Examples:
  asc app-clips list --app "APP_ID"
  asc app-clips list --app "APP_ID" --bundle-id "com.example.clip"
  asc app-clips list --app "APP_ID" --limit 50
  asc app-clips list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-clips list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-clips list: %w", err)
			}

			appValue := strings.TrimSpace(shared.ResolveAppID(*appID))
			if appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppClipsOption{
				asc.WithAppClipsLimit(*limit),
				asc.WithAppClipsNextURL(*next),
				asc.WithAppClipsBundleIDs(shared.SplitCSV(*bundleID)),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppClipsLimit(200))
				firstPage, err := client.GetAppClips(requestCtx, appValue, paginateOpts...)
				if err != nil {
					if asc.IsNotFound(err) {
						empty := &asc.AppClipsResponse{Data: []asc.Resource[asc.AppClipAttributes]{}}
						return shared.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("app-clips list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClips(ctx, appValue, asc.WithAppClipsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppClips(requestCtx, appValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.AppClipsResponse{Data: []asc.Resource[asc.AppClipAttributes]{}}
					return shared.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipsGetCommand returns the app clips get subcommand.
func AppClipsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	appClipID := fs.String("id", "", "App Clip ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-clips get --id \"CLIP_ID\"",
		ShortHelp:  "Get App Clip details by ID.",
		LongHelp: `Get App Clip details by ID.

Examples:
  asc app-clips get --id "CLIP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*appClipID)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClip(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("app-clips get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
