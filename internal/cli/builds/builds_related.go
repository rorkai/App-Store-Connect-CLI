package builds

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type buildBetaAppReviewSubmissionNotFoundError struct {
	buildID string
}

func (e buildBetaAppReviewSubmissionNotFoundError) Error() string {
	return fmt.Sprintf("builds beta-app-review-submission view: no beta app review submission found for build %q", e.buildID)
}

func (e buildBetaAppReviewSubmissionNotFoundError) Unwrap() error {
	return asc.ErrNotFound
}

// BuildsAppCommand returns the builds app command group.
func BuildsAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app", flag.ExitOnError)
	viewCmd := BuildsAppViewCommand()

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc builds app <subcommand> [flags]",
		ShortHelp:  "View the app related to a build.",
		LongHelp: `View the app related to a build.

Examples:
  asc builds app view --build-id "BUILD_ID"
  asc builds app view --app "123456789" --latest`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			viewCmd,
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsAppViewCommand returns the builds app view subcommand.
func BuildsAppViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app view", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(shared.DefaultDeps(), fs, buildSelectorFlagOptions{})
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc builds app view (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER --platform PLATFORM [--version VERSION])",
		ShortHelp:  "View the app for a build.",
		LongHelp: `View the app for a build.

Examples:
  asc builds app view --build-id "BUILD_ID"
  asc builds app view --app "123456789" --latest`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.validate(); err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds app view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			buildID, err := selectors.resolveBuildID(requestCtx, client)
			if err != nil {
				return fmt.Errorf("builds app view: %w", err)
			}

			resp, err := client.GetBuildApp(requestCtx, buildID)
			if err != nil {
				return fmt.Errorf("builds app view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsPreReleaseVersionCommand returns the pre-release-version command group.
func BuildsPreReleaseVersionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-release-version", flag.ExitOnError)
	viewCmd := BuildsPreReleaseVersionViewCommand()

	return &ffcli.Command{
		Name:       "pre-release-version",
		ShortUsage: "asc builds pre-release-version <subcommand> [flags]",
		ShortHelp:  "View the pre-release version related to a build.",
		LongHelp: `View the pre-release version related to a build.

Examples:
  asc builds pre-release-version view --build-id "BUILD_ID"
  asc builds pre-release-version view --app "123456789" --latest`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			viewCmd,
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsPreReleaseVersionViewCommand returns the pre-release-version view subcommand.
func BuildsPreReleaseVersionViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-release-version view", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(shared.DefaultDeps(), fs, buildSelectorFlagOptions{})
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc builds pre-release-version view (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER --platform PLATFORM [--version VERSION])",
		ShortHelp:  "View the pre-release version for a build.",
		LongHelp: `View the pre-release version for a build.

Examples:
  asc builds pre-release-version view --build-id "BUILD_ID"
  asc builds pre-release-version view --app "123456789" --latest`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.validate(); err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds pre-release-version view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			buildID, err := selectors.resolveBuildID(requestCtx, client)
			if err != nil {
				return fmt.Errorf("builds pre-release-version view: %w", err)
			}

			resp, err := client.GetBuildPreReleaseVersion(requestCtx, buildID)
			if err != nil {
				return fmt.Errorf("builds pre-release-version view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsIconsCommand returns the builds icons command group.
func BuildsIconsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("icons", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "icons",
		ShortUsage: "asc builds icons <subcommand> [flags]",
		ShortHelp:  "List build icons for a build.",
		LongHelp: `List build icons for a build.

Examples:
  asc builds icons list --build-id "BUILD_ID"
  asc builds icons list --app "123456789" --latest`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsIconsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsIconsListCommand returns the builds icons list subcommand.
func BuildsIconsListCommand() *ffcli.Command {
	var selectors buildSelectorFlags

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "icons list",
		Name:        "list",
		ShortUsage:  "asc builds icons list [flags]",
		ShortHelp:   "List build icons for a build.",
		LongHelp: `List build icons for a build.

Examples:
  asc builds icons list --build-id "BUILD_ID"
  asc builds icons list --app "123456789" --latest
  asc builds icons list --app "123456789" --latest --paginate`,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		Flags: func(fs *flag.FlagSet) func() error {
			selectors = bindBuildSelectorFlags(shared.DefaultDeps(), fs, buildSelectorFlagOptions{})
			next := fs.Lookup("next").Value
			return func() error {
				if strings.TrimSpace(next.String()) == "" {
					return selectors.validate()
				}
				if err := selectors.validateNextPageSelectorFlags(); err != nil {
					return fmt.Errorf("builds icons list: %w", err)
				}
				return nil
			}
		},
		LookupParent: func(ctx context.Context, client *asc.Client, _ string) (string, error) {
			buildID, err := selectors.resolveBuildID(ctx, client)
			if err != nil {
				return "", fmt.Errorf("builds icons list: %w", err)
			}
			return buildID, nil
		},
		FetchPage: func(ctx context.Context, client *asc.Client, buildID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetBuildIcons(ctx, buildID, asc.Limit[asc.BuildIconsOption](limit), asc.NextURL[asc.BuildIconsOption](next))
		},
	})
}

// BuildsBetaAppReviewSubmissionCommand returns the beta-app-review-submission command group.
func BuildsBetaAppReviewSubmissionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-app-review-submission", flag.ExitOnError)
	viewCmd := BuildsBetaAppReviewSubmissionViewCommand()

	return &ffcli.Command{
		Name:       "beta-app-review-submission",
		ShortUsage: "asc builds beta-app-review-submission <subcommand> [flags]",
		ShortHelp:  "View beta app review submission for a build.",
		LongHelp: `View beta app review submission for a build.

Examples:
  asc builds beta-app-review-submission view --build-id "BUILD_ID"
  asc builds beta-app-review-submission view --app "123456789" --latest`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			viewCmd,
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsBetaAppReviewSubmissionViewCommand returns the beta-app-review-submission view subcommand.
func BuildsBetaAppReviewSubmissionViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-app-review-submission view", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(shared.DefaultDeps(), fs, buildSelectorFlagOptions{})
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc builds beta-app-review-submission view (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER --platform PLATFORM [--version VERSION])",
		ShortHelp:  "View beta app review submission for a build.",
		LongHelp: `View beta app review submission for a build.

Examples:
  asc builds beta-app-review-submission view --build-id "BUILD_ID"
  asc builds beta-app-review-submission view --app "123456789" --latest`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.validate(); err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds beta-app-review-submission view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			buildID, err := selectors.resolveBuildID(requestCtx, client)
			if err != nil {
				return fmt.Errorf("builds beta-app-review-submission view: %w", err)
			}

			resp, err := client.GetBuildBetaAppReviewSubmission(requestCtx, buildID)
			if err != nil {
				var missingErr asc.MissingBuildBetaAppReviewSubmissionError
				if errors.As(err, &missingErr) {
					return buildBetaAppReviewSubmissionNotFoundError{buildID: buildID}
				}
				return fmt.Errorf("builds beta-app-review-submission view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsBuildBetaDetailCommand returns the build-beta-detail command group.
func BuildsBuildBetaDetailCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build-beta-detail", flag.ExitOnError)
	viewCmd := BuildsBuildBetaDetailViewCommand()

	return &ffcli.Command{
		Name:       "build-beta-detail",
		ShortUsage: "asc builds build-beta-detail <subcommand> [flags]",
		ShortHelp:  "View build beta detail for a build.",
		LongHelp: `View build beta detail for a build.

Examples:
  asc builds build-beta-detail view --build-id "BUILD_ID"
  asc builds build-beta-detail view --app "123456789" --latest`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			viewCmd,
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsBuildBetaDetailViewCommand returns the build-beta-detail view subcommand.
func BuildsBuildBetaDetailViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build-beta-detail view", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(shared.DefaultDeps(), fs, buildSelectorFlagOptions{})
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc builds build-beta-detail view (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER --platform PLATFORM [--version VERSION])",
		ShortHelp:  "View build beta detail for a build.",
		LongHelp: `View build beta detail for a build.

Examples:
  asc builds build-beta-detail view --build-id "BUILD_ID"
  asc builds build-beta-detail view --app "123456789" --latest`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.validate(); err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds build-beta-detail view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			buildID, err := selectors.resolveBuildID(requestCtx, client)
			if err != nil {
				return fmt.Errorf("builds build-beta-detail view: %w", err)
			}

			resp, err := client.GetBuildBuildBetaDetail(requestCtx, buildID)
			if err != nil {
				return fmt.Errorf("builds build-beta-detail view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
