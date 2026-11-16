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

// NewAppClipsCommand returns the app-clips command group.
func NewAppClipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-clips", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-clips",
		ShortUsage: "asc app-clips <subcommand> [flags]",
		ShortHelp:  "Manage App Clip experiences and invocations.",
		LongHelp: `Manage App Clip experiences and invocations.

Examples:
  asc app-clips list --app "APP_ID"
  asc app-clips view --id "CLIP_ID"
  asc app-clips default-experiences list --app-clip-id "CLIP_ID"
  asc app-clips advanced-experiences create --app "APP_ID" --bundle-id "com.example.clip" --link "https://example.com" --default-language EN --is-powered-by --header-image-id "IMAGE_ID" --localization-id "LOCALIZATION_ID"
  asc app-clips invocations list --build-bundle-id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: d.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppClipsListCommand(d),
			newAppClipsGetCommand(d),
			newAppClipDefaultExperiencesCommand(d),
			newAppClipAdvancedExperiencesCommand(d),
			newAppClipDefaultExperiencesRelationshipsCommand(d),
			newAppClipAdvancedExperiencesRelationshipsCommand(d),
			newAppClipHeaderImagesCommand(d),
			newAppClipInvocationsCommand(d),
			newAppClipDomainStatusCommand(d),
			newAppClipReviewDetailsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppClipsListCommand returns the app clips list subcommand.
func newAppClipsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	bundleID := fs.String("bundle-id", "", "Filter by bundle ID(s), comma-separated")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "app-clips list", 200, "bundle-id"); err != nil {
				return err
			}

			appValue := strings.TrimSpace(shared.ResolveAppID(*appID))
			if appValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppClipsOption{
				asc.Limit[asc.AppClipsOption](*limit),
				asc.NextURL[asc.AppClipsOption](*next),
				asc.WithAppClipsBundleIDs(shared.SplitCSV(*bundleID)),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppClipsOption](200))
				firstPage, err := client.GetAppClips(requestCtx, appValue, paginateOpts...)
				if err != nil {
					if asc.IsNotFound(err) {
						empty := &asc.AppClipsResponse{Data: []asc.Resource[asc.AppClipAttributes]{}}
						return d.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("app-clips list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClips(ctx, appValue, asc.NextURL[asc.AppClipsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppClips(requestCtx, appValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.AppClipsResponse{Data: []asc.Resource[asc.AppClipAttributes]{}}
					return d.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipsGetCommand returns the app clips get subcommand.
func newAppClipsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc app-clips view --id \"CLIP_ID\"",
		ShortHelp:  "View App Clip details by ID.",
		LongHelp: `View App Clip details by ID.

Examples:
  asc app-clips view --id "CLIP_ID"`,
		IDUsage:            "App Clip ID",
		IDType:             "appClips",
		RequiredDiagnostic: true,
		Deps:               d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppClip(ctx, id)
		},
	})
}
