package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	crashescmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/crashes"
	feedbackcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/feedback"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestFlightFeedbackCommand() *ffcli.Command {
	fs := flag.NewFlagSet("feedback", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "feedback",
		ShortUsage: "asc testflight feedback <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight feedback.",
		LongHelp: `Manage TestFlight feedback.

Examples:
  asc testflight feedback list --app "APP_ID"
  asc testflight feedback view --submission-id "SUBMISSION_ID"
  asc testflight feedback delete --submission-id "SUBMISSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: testflightVisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightFeedbackListCommand(),
			TestFlightFeedbackViewCommand(),
			TestFlightFeedbackDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func TestFlightFeedbackListCommand() *ffcli.Command {
	return feedbackcmd.NewListCommand(shared.ListCommandConfig{
		Name:       "list",
		ShortUsage: "asc testflight feedback list [flags]",
		ShortHelp:  "List TestFlight feedback.",
		LongHelp: `List TestFlight feedback.

Examples:
  asc testflight feedback list --app "123456789"
  asc testflight feedback list --app "123456789" --include-screenshots
  asc testflight feedback list --app "123456789" --device-model "iPhone15,3" --os-version "17.2"
  asc testflight feedback list --next "<links.next>"
  asc testflight feedback list --app "123456789" --paginate`,
		ErrorPrefix: "testflight feedback list",
	})
}

func TestFlightFeedbackViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	submissionID := fs.String("submission-id", "", "Feedback submission ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight feedback view --submission-id \"SUBMISSION_ID\"",
		ShortHelp:  "View a feedback submission by ID.",
		LongHelp: `View a feedback submission by ID.

Examples:
  asc testflight feedback view --submission-id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			submissionIDValue := strings.TrimSpace(*submissionID)
			if submissionIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission-id is required")
				return flag.ErrHelp
			}
			return runFeedbackSubmissionView(ctx, submissionIDValue, output)
		},
	}
}

func TestFlightFeedbackDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	submissionID := fs.String("submission-id", "", "Feedback submission ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc testflight feedback delete --submission-id \"SUBMISSION_ID\" --confirm",
		ShortHelp:  "Delete a feedback submission by ID.",
		LongHelp: `Delete a feedback submission by ID.

Examples:
  asc testflight feedback delete --submission-id "SUBMISSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			submissionIDValue := strings.TrimSpace(*submissionID)
			if submissionIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}
			return runFeedbackSubmissionDelete(ctx, submissionIDValue, output)
		},
	}
}

func TestFlightCrashesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("crashes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "crashes",
		ShortUsage: "asc testflight crashes <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight crash submissions.",
		LongHelp: `Manage TestFlight crash submissions.

Examples:
  asc testflight crashes list --app "APP_ID"
  asc testflight crashes view --submission-id "SUBMISSION_ID"
  asc testflight crashes delete --submission-id "SUBMISSION_ID" --confirm
  asc testflight crashes log --submission-id "SUBMISSION_ID"
  asc testflight crashes log --crash-log-id "CRASH_LOG_ID"`,
		FlagSet:   fs,
		UsageFunc: testflightVisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			TestFlightCrashesListCommand(),
			TestFlightCrashesViewCommand(),
			TestFlightCrashesDeleteCommand(),
			TestFlightCrashesLogCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func TestFlightCrashesListCommand() *ffcli.Command {
	return crashescmd.NewListCommand(shared.ListCommandConfig{
		Name:       "list",
		ShortUsage: "asc testflight crashes list [flags]",
		ShortHelp:  "List TestFlight crash submissions.",
		LongHelp: `List TestFlight crash submissions.

Examples:
  asc testflight crashes list --app "123456789"
  asc testflight crashes list --app "123456789" --device-model "iPhone15,3" --os-version "17.2"
  asc testflight crashes list --next "<links.next>"
  asc testflight crashes list --app "123456789" --paginate`,
		ErrorPrefix: "testflight crashes list",
	})
}

func TestFlightCrashesViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	submissionID := fs.String("submission-id", "", "Crash submission ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight crashes view --submission-id \"SUBMISSION_ID\"",
		ShortHelp:  "View a crash submission by ID.",
		LongHelp: `View a crash submission by ID.

Examples:
  asc testflight crashes view --submission-id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			submissionIDValue := strings.TrimSpace(*submissionID)
			if submissionIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission-id is required")
				return flag.ErrHelp
			}
			return runCrashSubmissionView(ctx, submissionIDValue, output)
		},
	}
}

func TestFlightCrashesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	submissionID := fs.String("submission-id", "", "Crash submission ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc testflight crashes delete --submission-id \"SUBMISSION_ID\" --confirm",
		ShortHelp:  "Delete a crash submission by ID.",
		LongHelp: `Delete a crash submission by ID.

Examples:
  asc testflight crashes delete --submission-id "SUBMISSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			submissionIDValue := strings.TrimSpace(*submissionID)
			if submissionIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}
			return runCrashSubmissionDelete(ctx, submissionIDValue, output)
		},
	}
}

func TestFlightCrashesLogCommand() *ffcli.Command {
	fs := flag.NewFlagSet("log", flag.ExitOnError)

	submissionID := fs.String("submission-id", "", "Crash submission ID")
	crashLogID := fs.String("crash-log-id", "", "Crash log ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "log",
		ShortUsage: "asc testflight crashes log [--submission-id SUBMISSION_ID | --crash-log-id CRASH_LOG_ID]",
		ShortHelp:  "Fetch a crash log by submission ID or crash log ID.",
		LongHelp: `Fetch a crash log by submission ID or crash log ID.

Examples:
  asc testflight crashes log --submission-id "SUBMISSION_ID"
  asc testflight crashes log --crash-log-id "CRASH_LOG_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			submissionIDValue := strings.TrimSpace(*submissionID)
			crashLogIDValue := strings.TrimSpace(*crashLogID)
			if (submissionIDValue == "" && crashLogIDValue == "") || (submissionIDValue != "" && crashLogIDValue != "") {
				fmt.Fprintln(os.Stderr, "Error: exactly one of --submission-id or --crash-log-id is required")
				return flag.ErrHelp
			}
			if submissionIDValue != "" {
				return runCrashLogBySubmissionID(ctx, submissionIDValue, output)
			}
			return runCrashLogByCrashLogID(ctx, crashLogIDValue, output)
		},
	}
}

func DeprecatedBetaFeedbackAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-feedback", flag.ExitOnError)

	cmd := &ffcli.Command{
		Name:       "beta-feedback",
		ShortUsage: "asc testflight feedback <subcommand> | asc testflight crashes <subcommand>",
		ShortHelp:  "Compatibility aliases for older feedback paths.",
		LongHelp: `Compatibility aliases for older feedback and crash paths.

Prefer:
  asc testflight feedback ...
  asc testflight crashes ...`,
		FlagSet:   fs,
		UsageFunc: shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			deprecatedBetaFeedbackCrashSubmissionsAliasCommand(),
			deprecatedBetaFeedbackScreenshotSubmissionsAliasCommand(),
			deprecatedBetaFeedbackCrashLogAliasCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}

	return hideTestFlightCommand(cmd)
}

func deprecatedBetaFeedbackCrashSubmissionsAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("crash-submissions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "crash-submissions",
		ShortUsage: "asc testflight crashes <view|delete> [flags]",
		ShortHelp:  "Compatibility alias: use `asc testflight crashes ...`.",
		LongHelp:   `Compatibility alias: use ` + "`asc testflight crashes view`" + ` and ` + "`asc testflight crashes delete`" + `.`,
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			deprecatedBetaFeedbackCrashSubmissionsGetAliasCommand(),
			deprecatedBetaFeedbackCrashSubmissionsDeleteAliasCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func deprecatedBetaFeedbackCrashSubmissionsGetAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Beta feedback crash submission ID")
	submissionID := fs.String("submission-id", "", "Crash submission ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight crashes view --submission-id \"SUBMISSION_ID\"",
		ShortHelp:  "Compatibility alias: use `asc testflight crashes view`.",
		LongHelp:   "Compatibility alias: use `asc testflight crashes view --submission-id SUBMISSION_ID`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue, err := resolveLegacyAliasID(strings.TrimSpace(*id), strings.TrimSpace(*submissionID), "--submission-id")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return flag.ErrHelp
			}
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission-id is required")
				return flag.ErrHelp
			}
			return runCrashSubmissionView(ctx, idValue, output)
		},
	}
}

func deprecatedBetaFeedbackCrashSubmissionsDeleteAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Beta feedback crash submission ID")
	submissionID := fs.String("submission-id", "", "Crash submission ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc testflight crashes delete --submission-id \"SUBMISSION_ID\" --confirm",
		ShortHelp:  "Compatibility alias: use `asc testflight crashes delete`.",
		LongHelp:   "Compatibility alias: use `asc testflight crashes delete --submission-id SUBMISSION_ID --confirm`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue, err := resolveLegacyAliasID(strings.TrimSpace(*id), strings.TrimSpace(*submissionID), "--submission-id")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return flag.ErrHelp
			}
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}
			return runCrashSubmissionDelete(ctx, idValue, output)
		},
	}
}

func deprecatedBetaFeedbackScreenshotSubmissionsAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshot-submissions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshot-submissions",
		ShortUsage: "asc testflight feedback <view|delete> [flags]",
		ShortHelp:  "Compatibility alias: use `asc testflight feedback ...`.",
		LongHelp:   "Compatibility alias: use `asc testflight feedback view` and `asc testflight feedback delete`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			deprecatedBetaFeedbackScreenshotSubmissionsGetAliasCommand(),
			deprecatedBetaFeedbackScreenshotSubmissionsDeleteAliasCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func deprecatedBetaFeedbackScreenshotSubmissionsGetAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Beta feedback screenshot submission ID")
	submissionID := fs.String("submission-id", "", "Feedback submission ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight feedback view --submission-id \"SUBMISSION_ID\"",
		ShortHelp:  "Compatibility alias: use `asc testflight feedback view`.",
		LongHelp:   "Compatibility alias: use `asc testflight feedback view --submission-id SUBMISSION_ID`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue, err := resolveLegacyAliasID(strings.TrimSpace(*id), strings.TrimSpace(*submissionID), "--submission-id")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return flag.ErrHelp
			}
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission-id is required")
				return flag.ErrHelp
			}
			return runFeedbackSubmissionView(ctx, idValue, output)
		},
	}
}

func deprecatedBetaFeedbackScreenshotSubmissionsDeleteAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Beta feedback screenshot submission ID")
	submissionID := fs.String("submission-id", "", "Feedback submission ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc testflight feedback delete --submission-id \"SUBMISSION_ID\" --confirm",
		ShortHelp:  "Compatibility alias: use `asc testflight feedback delete`.",
		LongHelp:   "Compatibility alias: use `asc testflight feedback delete --submission-id SUBMISSION_ID --confirm`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue, err := resolveLegacyAliasID(strings.TrimSpace(*id), strings.TrimSpace(*submissionID), "--submission-id")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return flag.ErrHelp
			}
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}
			return runFeedbackSubmissionDelete(ctx, idValue, output)
		},
	}
}

func deprecatedBetaFeedbackCrashLogAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("crash-log", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "crash-log",
		ShortUsage: "asc testflight crashes log [flags]",
		ShortHelp:  "Compatibility alias: use `asc testflight crashes log`.",
		LongHelp:   "Compatibility alias: use `asc testflight crashes log --submission-id SUBMISSION_ID`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			deprecatedBetaFeedbackCrashLogGetAliasCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func deprecatedBetaFeedbackCrashLogGetAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Beta feedback crash submission ID")
	submissionID := fs.String("submission-id", "", "Crash submission ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight crashes log --submission-id \"SUBMISSION_ID\"",
		ShortHelp:  "Compatibility alias: use `asc testflight crashes log`.",
		LongHelp:   "Compatibility alias: use `asc testflight crashes log --submission-id SUBMISSION_ID`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue, err := resolveLegacyAliasID(strings.TrimSpace(*id), strings.TrimSpace(*submissionID), "--submission-id")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return flag.ErrHelp
			}
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission-id is required")
				return flag.ErrHelp
			}
			return runCrashLogBySubmissionID(ctx, idValue, output)
		},
	}
}

func resolveLegacyAliasID(legacyID, canonicalID, canonicalFlag string) (string, error) {
	switch {
	case legacyID != "" && canonicalID != "" && legacyID != canonicalID:
		return "", fmt.Errorf("%s and --id must match", canonicalFlag)
	case canonicalID != "":
		return canonicalID, nil
	default:
		return legacyID, nil
	}
}

func runFeedbackSubmissionView(ctx context.Context, submissionID string, output shared.OutputFlags) error {
	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("testflight feedback view: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resp, err := client.GetBetaFeedbackScreenshotSubmission(requestCtx, submissionID)
	if err != nil {
		return fmt.Errorf("testflight feedback view: failed to fetch: %w", err)
	}

	return shared.PrintOutput(resp, *output.Output, *output.Pretty)
}

func runFeedbackSubmissionDelete(ctx context.Context, submissionID string, output shared.OutputFlags) error {
	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("testflight feedback delete: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	if err := client.DeleteBetaFeedbackScreenshotSubmission(requestCtx, submissionID); err != nil {
		return fmt.Errorf("testflight feedback delete: failed to delete: %w", err)
	}

	result := &asc.BetaFeedbackSubmissionDeleteResult{
		ID:      submissionID,
		Deleted: true,
	}

	return shared.PrintOutput(result, *output.Output, *output.Pretty)
}

func runCrashSubmissionView(ctx context.Context, submissionID string, output shared.OutputFlags) error {
	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("testflight crashes view: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resp, err := client.GetBetaFeedbackCrashSubmission(requestCtx, submissionID)
	if err != nil {
		return fmt.Errorf("testflight crashes view: failed to fetch: %w", err)
	}

	return shared.PrintOutput(resp, *output.Output, *output.Pretty)
}

func runCrashSubmissionDelete(ctx context.Context, submissionID string, output shared.OutputFlags) error {
	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("testflight crashes delete: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	if err := client.DeleteBetaFeedbackCrashSubmission(requestCtx, submissionID); err != nil {
		return fmt.Errorf("testflight crashes delete: failed to delete: %w", err)
	}

	result := &asc.BetaFeedbackSubmissionDeleteResult{
		ID:      submissionID,
		Deleted: true,
	}

	return shared.PrintOutput(result, *output.Output, *output.Pretty)
}

func runCrashLogBySubmissionID(ctx context.Context, submissionID string, output shared.OutputFlags) error {
	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("testflight crashes log: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resp, err := client.GetBetaFeedbackCrashSubmissionCrashLog(requestCtx, submissionID)
	if err != nil {
		return fmt.Errorf("testflight crashes log: failed to fetch: %w", err)
	}

	return shared.PrintOutput(resp, *output.Output, *output.Pretty)
}
