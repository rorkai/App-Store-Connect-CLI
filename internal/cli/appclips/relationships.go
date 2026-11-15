package appclips

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppClipDefaultExperiencesRelationshipsCommand returns the default experiences links subcommand.
func newAppClipDefaultExperiencesRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("default-experiences-links", flag.ExitOnError)

	appClipID := shared.BindResourceIDFlag(fs, "app-clip-id", "appClips", "App Clip ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "default-experiences-links",
		ShortUsage: "asc app-clips default-experiences-links --app-clip-id \"CLIP_ID\" [flags]",
		ShortHelp:  "List default experience relationships for an App Clip.",
		LongHelp: `List default experience relationships for an App Clip.

Examples:
  asc app-clips default-experiences-links --app-clip-id "CLIP_ID"
  asc app-clips default-experiences-links --app-clip-id "CLIP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "app-clips default-experiences-links", 200); err != nil {
				return err
			}

			appClipValue := strings.TrimSpace(*appClipID)
			if appClipValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--app-clip-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences-links: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](*limit),
				asc.NextURL[asc.LinkagesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.LinkagesOption](200))
				firstPage, err := client.GetAppClipDefaultExperiencesRelationships(requestCtx, appClipValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-clips default-experiences-links: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClipDefaultExperiencesRelationships(ctx, appClipValue, asc.NextURL[asc.LinkagesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips default-experiences-links: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppClipDefaultExperiencesRelationships(requestCtx, appClipValue, opts...)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences-links: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipAdvancedExperiencesRelationshipsCommand returns the advanced experiences links subcommand.
func newAppClipAdvancedExperiencesRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("advanced-experiences-links", flag.ExitOnError)

	appClipID := shared.BindResourceIDFlag(fs, "app-clip-id", "appClips", "App Clip ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "advanced-experiences-links",
		ShortUsage: "asc app-clips advanced-experiences-links --app-clip-id \"CLIP_ID\" [flags]",
		ShortHelp:  "List advanced experience relationships for an App Clip.",
		LongHelp: `List advanced experience relationships for an App Clip.

Examples:
  asc app-clips advanced-experiences-links --app-clip-id "CLIP_ID"
  asc app-clips advanced-experiences-links --app-clip-id "CLIP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "app-clips advanced-experiences-links", 200); err != nil {
				return err
			}

			appClipValue := strings.TrimSpace(*appClipID)
			if appClipValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--app-clip-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences-links: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](*limit),
				asc.NextURL[asc.LinkagesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.LinkagesOption](200))
				firstPage, err := client.GetAppClipAdvancedExperiencesRelationships(requestCtx, appClipValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-clips advanced-experiences-links: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClipAdvancedExperiencesRelationships(ctx, appClipValue, asc.NextURL[asc.LinkagesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips advanced-experiences-links: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppClipAdvancedExperiencesRelationships(requestCtx, appClipValue, opts...)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences-links: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
