package testflight

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

// BetaFeedbackCommand returns the beta-feedback command group.
func BetaFeedbackCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-feedback", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-feedback",
		ShortUsage: "asc testflight beta-feedback <subcommand> [flags]",
		ShortHelp:  "View TestFlight beta feedback submissions.",
		LongHelp: `View TestFlight beta feedback submissions.

Examples:
  asc testflight beta-feedback crash-submissions get --id "SUBMISSION_ID"
  asc testflight beta-feedback screenshot-submissions get --id "SUBMISSION_ID"
  asc testflight beta-feedback crash-log get --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaFeedbackCrashSubmissionsCommand(),
			BetaFeedbackScreenshotSubmissionsCommand(),
			BetaFeedbackCrashLogCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaFeedbackCrashSubmissionsCommand returns the crash-submissions command group.
func BetaFeedbackCrashSubmissionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("crash-submissions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "crash-submissions",
		ShortUsage: "asc testflight beta-feedback crash-submissions <subcommand> [flags]",
		ShortHelp:  "Fetch beta feedback crash submission details.",
		LongHelp: `Fetch beta feedback crash submission details.

Examples:
  asc testflight beta-feedback crash-submissions get --id "SUBMISSION_ID"
  asc testflight beta-feedback crash-submissions delete --id "SUBMISSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaFeedbackCrashSubmissionsGetCommand(),
			BetaFeedbackCrashSubmissionsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaFeedbackCrashSubmissionsGetCommand returns the crash-submissions get subcommand.
func BetaFeedbackCrashSubmissionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("crash-submissions get", flag.ExitOnError)

	id := fs.String("id", "", "Beta feedback crash submission ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-feedback crash-submissions get --id \"SUBMISSION_ID\"",
		ShortHelp:  "Get a beta feedback crash submission by ID.",
		LongHelp: `Get a beta feedback crash submission by ID.

Examples:
  asc testflight beta-feedback crash-submissions get --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-feedback crash-submissions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaFeedbackCrashSubmission(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("testflight beta-feedback crash-submissions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BetaFeedbackScreenshotSubmissionsCommand returns the screenshot-submissions command group.
func BetaFeedbackScreenshotSubmissionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshot-submissions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshot-submissions",
		ShortUsage: "asc testflight beta-feedback screenshot-submissions <subcommand> [flags]",
		ShortHelp:  "Fetch beta feedback screenshot submission details.",
		LongHelp: `Fetch beta feedback screenshot submission details.

Examples:
  asc testflight beta-feedback screenshot-submissions get --id "SUBMISSION_ID"
  asc testflight beta-feedback screenshot-submissions delete --id "SUBMISSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaFeedbackScreenshotSubmissionsGetCommand(),
			BetaFeedbackScreenshotSubmissionsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaFeedbackCrashSubmissionsDeleteCommand deletes a beta feedback crash submission by ID.
func BetaFeedbackCrashSubmissionsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("crash-submissions delete", flag.ExitOnError)

	id := fs.String("id", "", "Beta feedback crash submission ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc testflight beta-feedback crash-submissions delete --id \"SUBMISSION_ID\" --confirm",
		ShortHelp:  "Delete a beta feedback crash submission by ID.",
		LongHelp: `Delete a beta feedback crash submission by ID.

Examples:
  asc testflight beta-feedback crash-submissions delete --id "SUBMISSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-feedback crash-submissions delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaFeedbackCrashSubmission(requestCtx, idValue); err != nil {
				return fmt.Errorf("testflight beta-feedback crash-submissions delete: failed to delete: %w", err)
			}

			result := &asc.BetaFeedbackSubmissionDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// BetaFeedbackScreenshotSubmissionsGetCommand returns the screenshot-submissions get subcommand.
func BetaFeedbackScreenshotSubmissionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshot-submissions get", flag.ExitOnError)

	id := fs.String("id", "", "Beta feedback screenshot submission ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-feedback screenshot-submissions get --id \"SUBMISSION_ID\"",
		ShortHelp:  "Get a beta feedback screenshot submission by ID.",
		LongHelp: `Get a beta feedback screenshot submission by ID.

Examples:
  asc testflight beta-feedback screenshot-submissions get --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-feedback screenshot-submissions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaFeedbackScreenshotSubmission(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("testflight beta-feedback screenshot-submissions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BetaFeedbackScreenshotSubmissionsDeleteCommand deletes a beta feedback screenshot submission by ID.
func BetaFeedbackScreenshotSubmissionsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshot-submissions delete", flag.ExitOnError)

	id := fs.String("id", "", "Beta feedback screenshot submission ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc testflight beta-feedback screenshot-submissions delete --id \"SUBMISSION_ID\" --confirm",
		ShortHelp:  "Delete a beta feedback screenshot submission by ID.",
		LongHelp: `Delete a beta feedback screenshot submission by ID.

Examples:
  asc testflight beta-feedback screenshot-submissions delete --id "SUBMISSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-feedback screenshot-submissions delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaFeedbackScreenshotSubmission(requestCtx, idValue); err != nil {
				return fmt.Errorf("testflight beta-feedback screenshot-submissions delete: failed to delete: %w", err)
			}

			result := &asc.BetaFeedbackSubmissionDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// BetaFeedbackCrashLogCommand returns the crash-log command group.
func BetaFeedbackCrashLogCommand() *ffcli.Command {
	fs := flag.NewFlagSet("crash-log", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "crash-log",
		ShortUsage: "asc testflight beta-feedback crash-log <subcommand> [flags]",
		ShortHelp:  "Fetch crash logs for beta feedback crash submissions.",
		LongHelp: `Fetch crash logs for beta feedback crash submissions.

Examples:
  asc testflight beta-feedback crash-log get --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaFeedbackCrashLogGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaFeedbackCrashLogGetCommand returns the crash-log get subcommand.
func BetaFeedbackCrashLogGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("crash-log get", flag.ExitOnError)

	id := fs.String("id", "", "Beta feedback crash submission ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-feedback crash-log get --id \"SUBMISSION_ID\"",
		ShortHelp:  "Get the crash log for a beta feedback crash submission.",
		LongHelp: `Get the crash log for a beta feedback crash submission.

Examples:
  asc testflight beta-feedback crash-log get --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-feedback crash-log get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaFeedbackCrashSubmissionCrashLog(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("testflight beta-feedback crash-log get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
