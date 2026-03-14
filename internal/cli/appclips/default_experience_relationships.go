package appclips

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppClipDefaultExperienceRelationshipsCommand returns the default experience links command group.
func AppClipDefaultExperienceRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("links", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "links",
		ShortUsage: "asc app-clips default-experiences links <subcommand> [flags]",
		ShortHelp:  "Manage default experience relationships.",
		LongHelp: `Manage default experience relationships.

Examples:
  asc app-clips default-experiences links app-store-review-detail --experience-id "EXP_ID"
  asc app-clips default-experiences links release-with-app-store-version --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipDefaultExperienceReviewDetailRelationshipCommand(),
			AppClipDefaultExperienceReleaseWithAppStoreVersionRelationshipCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipDefaultExperienceReviewDetailRelationshipCommand fetches review detail relationship.
func AppClipDefaultExperienceReviewDetailRelationshipCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-store-review-detail", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Default experience ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "app-store-review-detail",
		ShortUsage: "asc app-clips default-experiences links app-store-review-detail --experience-id \"EXP_ID\"",
		ShortHelp:  "Get review detail relationship for a default experience.",
		LongHelp: `Get review detail relationship for a default experience.

Examples:
  asc app-clips default-experiences links app-store-review-detail --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences links app-store-review-detail: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceReviewDetailRelationship(requestCtx, experienceValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences links app-store-review-detail: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDefaultExperienceReleaseWithAppStoreVersionRelationshipCommand fetches releaseWithAppStoreVersion relationship.
func AppClipDefaultExperienceReleaseWithAppStoreVersionRelationshipCommand() *ffcli.Command {
	fs := flag.NewFlagSet("release-with-app-store-version", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Default experience ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "release-with-app-store-version",
		ShortUsage: "asc app-clips default-experiences links release-with-app-store-version --experience-id \"EXP_ID\"",
		ShortHelp:  "Get release with App Store version relationship for a default experience.",
		LongHelp: `Get release with App Store version relationship for a default experience.

Examples:
  asc app-clips default-experiences links release-with-app-store-version --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences links release-with-app-store-version: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceReleaseWithAppStoreVersionRelationship(requestCtx, experienceValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences links release-with-app-store-version: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func DeprecatedAppClipDefaultExperienceRelationshipsAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc app-clips default-experiences links <subcommand> [flags]",
		ShortHelp:  "DEPRECATED: use `asc app-clips default-experiences links ...`.",
		LongHelp:   "Deprecated compatibility alias for `asc app-clips default-experiences links ...`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			shared.DeprecatedAliasLeafCommand(
				AppClipDefaultExperienceReviewDetailRelationshipCommand(),
				"app-store-review-detail",
				"asc app-clips default-experiences links app-store-review-detail --experience-id \"EXP_ID\"",
				"asc app-clips default-experiences links app-store-review-detail",
				"Warning: `asc app-clips default-experiences relationships app-store-review-detail` is deprecated. Use `asc app-clips default-experiences links app-store-review-detail`.",
			),
			shared.DeprecatedAliasLeafCommand(
				AppClipDefaultExperienceReleaseWithAppStoreVersionRelationshipCommand(),
				"release-with-app-store-version",
				"asc app-clips default-experiences links release-with-app-store-version --experience-id \"EXP_ID\"",
				"asc app-clips default-experiences links release-with-app-store-version",
				"Warning: `asc app-clips default-experiences relationships release-with-app-store-version` is deprecated. Use `asc app-clips default-experiences links release-with-app-store-version`.",
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
