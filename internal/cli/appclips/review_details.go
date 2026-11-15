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

// newAppClipReviewDetailsCommand returns the review-details command group.
func newAppClipReviewDetailsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("review-details", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review-details",
		ShortUsage: "asc app-clips review-details <subcommand> [flags]",
		ShortHelp:  "Manage App Clip App Store review details.",
		LongHelp: `Manage App Clip App Store review details (invocation URLs).

Examples:
  asc app-clips review-details view --id "DETAIL_ID"
  asc app-clips review-details create --experience-id "EXP_ID" --url "https://example.com/clip"
  asc app-clips review-details update --id "DETAIL_ID" --url "https://example.com/clip"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppClipReviewDetailsGetCommand(d),
			newAppClipReviewDetailsCreateCommand(d),
			newAppClipReviewDetailsUpdateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppClipReviewDetailsGetCommand gets review details by ID.
func newAppClipReviewDetailsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc app-clips review-details view --id \"DETAIL_ID\"",
		ShortHelp:  "View App Clip review details by ID.",
		LongHelp: `View App Clip review details by ID.

Examples:
  asc app-clips review-details view --id "DETAIL_ID"`,
		IDUsage:            "Review detail ID",
		IDType:             "appClipAppStoreReviewDetails",
		RequiredDiagnostic: true,
		Deps:               d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppClipAppStoreReviewDetail(ctx, id)
		},
	})
}

// newAppClipReviewDetailsCreateCommand creates review details.
func newAppClipReviewDetailsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	experienceID := shared.BindResourceIDFlag(fs, "experience-id", "appClipDefaultExperiences", "Default experience ID")
	urls := shared.BindOnceCSVFlag(fs, "url", "Invocation URL(s), comma-separated")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips review-details create --experience-id \"EXP_ID\" --url \"https://example.com/clip\" [flags]",
		ShortHelp:  "Create App Clip review details.",
		LongHelp: `Create App Clip review details.

Examples:
  asc app-clips review-details create --experience-id "EXP_ID" --url "https://example.com/clip"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				return d.RequiredFlag("--experience-id")
			}

			urlValues := shared.SplitCSV(urls.String())
			if len(urlValues) == 0 {
				return d.RequiredFlag("--url")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips review-details create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := &asc.AppClipAppStoreReviewDetailCreateAttributes{InvocationURLs: urlValues}
			resp, err := client.CreateAppClipAppStoreReviewDetail(requestCtx, experienceValue, attrs)
			if err != nil {
				return fmt.Errorf("app-clips review-details create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipReviewDetailsUpdateCommand updates review details.
func newAppClipReviewDetailsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	detailID := shared.BindResourceIDFlag(fs, "id", "appClipAppStoreReviewDetails", "Review detail ID")
	urls := shared.BindOnceCSVFlag(fs, "url", "Invocation URL(s), comma-separated")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc app-clips review-details update --id \"DETAIL_ID\" --url \"https://example.com/clip\"",
		ShortHelp:  "Update App Clip review details.",
		LongHelp: `Update App Clip review details.

Examples:
  asc app-clips review-details update --id "DETAIL_ID" --url "https://example.com/clip"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			detailValue := strings.TrimSpace(*detailID)
			if detailValue == "" {
				return d.RequiredFlag("--id")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})
			if !visited["url"] {
				return d.UpdateFlagRequired(fs, urls)
			}

			urlValues := shared.SplitCSV(urls.String())
			attrs := &asc.AppClipAppStoreReviewDetailUpdateAttributes{InvocationURLs: urlValues}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips review-details update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppClipAppStoreReviewDetail(requestCtx, detailValue, attrs)
			if err != nil {
				return fmt.Errorf("app-clips review-details update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
