package builds

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

// BuildsAppCommand returns the builds app command group.
func BuildsAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc builds app <subcommand> [flags]",
		ShortHelp:  "View the app related to a build.",
		LongHelp: `View the app related to a build.

Examples:
  asc builds app get --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsAppGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsAppGetCommand returns the builds app get subcommand.
func BuildsAppGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app get", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	aliasID := fs.String("id", "", "Build ID (alias of --build)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc builds app get --build \"BUILD_ID\"",
		ShortHelp:  "Get the app for a build.",
		LongHelp: `Get the app for a build.

Examples:
  asc builds app get --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			aliasValue := strings.TrimSpace(*aliasID)
			if buildValue == "" {
				buildValue = aliasValue
			} else if aliasValue != "" && aliasValue != buildValue {
				return fmt.Errorf("builds app get: --build and --id must match")
			}
			if buildValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds app get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildApp(requestCtx, buildValue)
			if err != nil {
				return fmt.Errorf("builds app get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BuildsPreReleaseVersionCommand returns the pre-release-version command group.
func BuildsPreReleaseVersionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-release-version", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pre-release-version",
		ShortUsage: "asc builds pre-release-version <subcommand> [flags]",
		ShortHelp:  "View the pre-release version related to a build.",
		LongHelp: `View the pre-release version related to a build.

Examples:
  asc builds pre-release-version get --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsPreReleaseVersionGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsPreReleaseVersionGetCommand returns the pre-release-version get subcommand.
func BuildsPreReleaseVersionGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-release-version get", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	aliasID := fs.String("id", "", "Build ID (alias of --build)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc builds pre-release-version get --build \"BUILD_ID\"",
		ShortHelp:  "Get the pre-release version for a build.",
		LongHelp: `Get the pre-release version for a build.

Examples:
  asc builds pre-release-version get --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			aliasValue := strings.TrimSpace(*aliasID)
			if buildValue == "" {
				buildValue = aliasValue
			} else if aliasValue != "" && aliasValue != buildValue {
				return fmt.Errorf("builds pre-release-version get: --build and --id must match")
			}
			if buildValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds pre-release-version get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildPreReleaseVersion(requestCtx, buildValue)
			if err != nil {
				return fmt.Errorf("builds pre-release-version get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
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
  asc builds icons list --build "BUILD_ID"`,
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
	fs := flag.NewFlagSet("icons list", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	aliasID := fs.String("id", "", "Build ID (alias of --build)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc builds icons list [flags]",
		ShortHelp:  "List build icons for a build.",
		LongHelp: `List build icons for a build.

Examples:
  asc builds icons list --build "BUILD_ID"
  asc builds icons list --build "BUILD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("builds icons list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("builds icons list: %w", err)
			}

			buildValue := strings.TrimSpace(*buildID)
			aliasValue := strings.TrimSpace(*aliasID)
			if buildValue == "" {
				buildValue = aliasValue
			} else if aliasValue != "" && aliasValue != buildValue {
				return fmt.Errorf("builds icons list: --build and --id must match")
			}
			if buildValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds icons list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BuildIconsOption{
				asc.WithBuildIconsLimit(*limit),
				asc.WithBuildIconsNextURL(*next),
			}

			if *paginate {
				if buildValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --build is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithBuildIconsLimit(200))
				firstPage, err := client.GetBuildIcons(requestCtx, buildValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("builds icons list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuildIcons(ctx, buildValue, asc.WithBuildIconsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("builds icons list: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBuildIcons(requestCtx, buildValue, opts...)
			if err != nil {
				return fmt.Errorf("builds icons list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BuildsBetaAppReviewSubmissionCommand returns the beta-app-review-submission command group.
func BuildsBetaAppReviewSubmissionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-app-review-submission", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-app-review-submission",
		ShortUsage: "asc builds beta-app-review-submission <subcommand> [flags]",
		ShortHelp:  "View beta app review submission for a build.",
		LongHelp: `View beta app review submission for a build.

Examples:
  asc builds beta-app-review-submission get --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsBetaAppReviewSubmissionGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsBetaAppReviewSubmissionGetCommand returns the beta-app-review-submission get subcommand.
func BuildsBetaAppReviewSubmissionGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-app-review-submission get", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	aliasID := fs.String("id", "", "Build ID (alias of --build)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc builds beta-app-review-submission get --build \"BUILD_ID\"",
		ShortHelp:  "Get beta app review submission for a build.",
		LongHelp: `Get beta app review submission for a build.

Examples:
  asc builds beta-app-review-submission get --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			aliasValue := strings.TrimSpace(*aliasID)
			if buildValue == "" {
				buildValue = aliasValue
			} else if aliasValue != "" && aliasValue != buildValue {
				return fmt.Errorf("builds beta-app-review-submission get: --build and --id must match")
			}
			if buildValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds beta-app-review-submission get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildBetaAppReviewSubmission(requestCtx, buildValue)
			if err != nil {
				return fmt.Errorf("builds beta-app-review-submission get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BuildsBuildBetaDetailCommand returns the build-beta-detail command group.
func BuildsBuildBetaDetailCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build-beta-detail", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "build-beta-detail",
		ShortUsage: "asc builds build-beta-detail <subcommand> [flags]",
		ShortHelp:  "View build beta detail for a build.",
		LongHelp: `View build beta detail for a build.

Examples:
  asc builds build-beta-detail get --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsBuildBetaDetailGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsBuildBetaDetailGetCommand returns the build-beta-detail get subcommand.
func BuildsBuildBetaDetailGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build-beta-detail get", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	aliasID := fs.String("id", "", "Build ID (alias of --build)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc builds build-beta-detail get --build \"BUILD_ID\"",
		ShortHelp:  "Get build beta detail for a build.",
		LongHelp: `Get build beta detail for a build.

Examples:
  asc builds build-beta-detail get --build "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			aliasValue := strings.TrimSpace(*aliasID)
			if buildValue == "" {
				buildValue = aliasValue
			} else if aliasValue != "" && aliasValue != buildValue {
				return fmt.Errorf("builds build-beta-detail get: --build and --id must match")
			}
			if buildValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds build-beta-detail get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildBuildBetaDetail(requestCtx, buildValue)
			if err != nil {
				return fmt.Errorf("builds build-beta-detail get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
