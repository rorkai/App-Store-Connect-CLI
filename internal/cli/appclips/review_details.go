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

// AppClipReviewDetailsCommand returns the review-details command group.
func AppClipReviewDetailsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-details", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review-details",
		ShortUsage: "asc app-clips review-details <subcommand> [flags]",
		ShortHelp:  "Manage App Clip App Store review details.",
		LongHelp: `Manage App Clip App Store review details (invocation URLs).

Examples:
  asc app-clips review-details get --id "DETAIL_ID"
  asc app-clips review-details create --experience-id "EXP_ID" --url "https://example.com/clip"
  asc app-clips review-details update --id "DETAIL_ID" --url "https://example.com/clip"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipReviewDetailsGetCommand(),
			AppClipReviewDetailsCreateCommand(),
			AppClipReviewDetailsUpdateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipReviewDetailsGetCommand gets review details by ID.
func AppClipReviewDetailsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	detailID := fs.String("id", "", "Review detail ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-clips review-details get --id \"DETAIL_ID\"",
		ShortHelp:  "Get App Clip review details by ID.",
		LongHelp: `Get App Clip review details by ID.

Examples:
  asc app-clips review-details get --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			detailValue := strings.TrimSpace(*detailID)
			if detailValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips review-details get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipAppStoreReviewDetail(requestCtx, detailValue)
			if err != nil {
				return fmt.Errorf("app-clips review-details get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipReviewDetailsCreateCommand creates review details.
func AppClipReviewDetailsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Default experience ID")
	urls := fs.String("url", "", "Invocation URL(s), comma-separated")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			urlValues := shared.SplitCSV(*urls)
			if len(urlValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --url is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips review-details create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := &asc.AppClipAppStoreReviewDetailCreateAttributes{InvocationURLs: urlValues}
			resp, err := client.CreateAppClipAppStoreReviewDetail(requestCtx, experienceValue, attrs)
			if err != nil {
				return fmt.Errorf("app-clips review-details create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipReviewDetailsUpdateCommand updates review details.
func AppClipReviewDetailsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	detailID := fs.String("id", "", "Review detail ID")
	urls := fs.String("url", "", "Invocation URL(s), comma-separated")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})
			if !visited["url"] {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			urlValues := shared.SplitCSV(*urls)
			attrs := &asc.AppClipAppStoreReviewDetailUpdateAttributes{InvocationURLs: urlValues}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips review-details update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppClipAppStoreReviewDetail(requestCtx, detailValue, attrs)
			if err != nil {
				return fmt.Errorf("app-clips review-details update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
