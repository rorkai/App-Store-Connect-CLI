package appclips

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppClipDefaultExperienceRelationshipsCommand returns the default experience links command group.
func newAppClipDefaultExperienceRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("links", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "links",
		ShortUsage: "asc app-clips default-experiences links <subcommand> [flags]",
		ShortHelp:  "Manage default experience relationships.",
		LongHelp: `Manage default experience relationships.

Examples:
  asc app-clips default-experiences links app-store-review-detail --experience-id "EXP_ID"
  asc app-clips default-experiences links release-with-app-store-version --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppClipDefaultExperienceReviewDetailRelationshipCommand(d),
			newAppClipDefaultExperienceReleaseWithAppStoreVersionRelationshipCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppClipDefaultExperienceReviewDetailRelationshipCommand fetches review detail relationship.
func newAppClipDefaultExperienceReviewDetailRelationshipCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-store-review-detail", flag.ExitOnError)

	experienceID := shared.BindResourceIDFlag(fs, "experience-id", "appClipDefaultExperiences", "Default experience ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "app-store-review-detail",
		ShortUsage: "asc app-clips default-experiences links app-store-review-detail --experience-id \"EXP_ID\"",
		ShortHelp:  "Get review detail relationship for a default experience.",
		LongHelp: `Get review detail relationship for a default experience.

Examples:
  asc app-clips default-experiences links app-store-review-detail --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				return d.RequiredFlag("--experience-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences links app-store-review-detail: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceReviewDetailRelationship(requestCtx, experienceValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences links app-store-review-detail: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipDefaultExperienceReleaseWithAppStoreVersionRelationshipCommand fetches releaseWithAppStoreVersion relationship.
func newAppClipDefaultExperienceReleaseWithAppStoreVersionRelationshipCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("release-with-app-store-version", flag.ExitOnError)

	experienceID := shared.BindResourceIDFlag(fs, "experience-id", "appClipDefaultExperiences", "Default experience ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "release-with-app-store-version",
		ShortUsage: "asc app-clips default-experiences links release-with-app-store-version --experience-id \"EXP_ID\"",
		ShortHelp:  "Get release with App Store version relationship for a default experience.",
		LongHelp: `Get release with App Store version relationship for a default experience.

Examples:
  asc app-clips default-experiences links release-with-app-store-version --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				return d.RequiredFlag("--experience-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences links release-with-app-store-version: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceReleaseWithAppStoreVersionRelationship(requestCtx, experienceValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences links release-with-app-store-version: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
