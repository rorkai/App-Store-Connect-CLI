package testflight

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	crashescmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/crashes"
	feedbackcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/feedback"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newTestFlightFeedbackCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("feedback", flag.ExitOnError)

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newTestFlightFeedbackListCommand(d),
			newTestFlightFeedbackViewCommand(d),
			newTestFlightFeedbackDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newTestFlightFeedbackListCommand(d *shared.Deps) *ffcli.Command {
	return feedbackcmd.NewListCommand(d, shared.ListCommandConfig{
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

func newTestFlightFeedbackViewCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc testflight feedback view --submission-id \"SUBMISSION_ID\"",
		ShortHelp:  "View a feedback submission by ID.",
		LongHelp: `View a feedback submission by ID.

Examples:
  asc testflight feedback view --submission-id "SUBMISSION_ID"`,
		IDFlag:             "submission-id",
		IDUsage:            "Feedback submission ID",
		IDType:             "betaFeedbackScreenshotSubmissions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBetaFeedbackScreenshotSubmission(ctx, id)
		},
	})
}

func newTestFlightFeedbackDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:       d,
		Name:       "delete",
		ShortUsage: "asc testflight feedback delete --submission-id \"SUBMISSION_ID\" --confirm",
		ShortHelp:  "Delete a feedback submission by ID.",
		LongHelp: `Delete a feedback submission by ID.

Examples:
  asc testflight feedback delete --submission-id "SUBMISSION_ID" --confirm`,
		IDFlag:             "submission-id",
		IDUsage:            "Feedback submission ID",
		IDType:             "betaFeedbackScreenshotSubmissions",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteBetaFeedbackScreenshotSubmission(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

func newTestFlightCrashesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("crashes", flag.ExitOnError)

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newTestFlightCrashesListCommand(d),
			newTestFlightCrashesViewCommand(d),
			newTestFlightCrashesDeleteCommand(d),
			newTestFlightCrashesLogCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newTestFlightCrashesListCommand(d *shared.Deps) *ffcli.Command {
	return crashescmd.NewListCommand(d, shared.ListCommandConfig{
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

func newTestFlightCrashesViewCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc testflight crashes view --submission-id \"SUBMISSION_ID\"",
		ShortHelp:  "View a crash submission by ID.",
		LongHelp: `View a crash submission by ID.

Examples:
  asc testflight crashes view --submission-id "SUBMISSION_ID"`,
		IDFlag:             "submission-id",
		IDUsage:            "Crash submission ID",
		IDType:             "betaFeedbackCrashSubmissions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBetaFeedbackCrashSubmission(ctx, id)
		},
	})
}

func newTestFlightCrashesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:       d,
		Name:       "delete",
		ShortUsage: "asc testflight crashes delete --submission-id \"SUBMISSION_ID\" --confirm",
		ShortHelp:  "Delete a crash submission by ID.",
		LongHelp: `Delete a crash submission by ID.

Examples:
  asc testflight crashes delete --submission-id "SUBMISSION_ID" --confirm`,
		IDFlag:             "submission-id",
		IDUsage:            "Crash submission ID",
		IDType:             "betaFeedbackCrashSubmissions",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteBetaFeedbackCrashSubmission(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

func newTestFlightCrashesLogCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("log", flag.ExitOnError)

	submissionID := shared.BindResourceIDFlag(fs, "submission-id", "betaFeedbackCrashSubmissions", "Crash submission ID")
	crashLogID := shared.BindResourceIDFlag(fs, "crash-log-id", "betaCrashLogs", "Crash log ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "log",
		ShortUsage: "asc testflight crashes log [--submission-id SUBMISSION_ID | --crash-log-id CRASH_LOG_ID]",
		ShortHelp:  "Fetch a crash log by submission ID or crash log ID.",
		LongHelp: `Fetch a crash log by submission ID or crash log ID.

Examples:
  asc testflight crashes log --submission-id "SUBMISSION_ID"
  asc testflight crashes log --crash-log-id "CRASH_LOG_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			submissionIDValue := strings.TrimSpace(*submissionID)
			crashLogIDValue := strings.TrimSpace(*crashLogID)
			if submissionIDValue == "" && crashLogIDValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: exactly one of --submission-id or --crash-log-id is required")
				return shared.MissingRequiredUsageError("")
			}
			if submissionIDValue != "" && crashLogIDValue != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: exactly one of --submission-id or --crash-log-id is required")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}
			if submissionIDValue != "" {
				return runCrashLogBySubmissionID(ctx, d, submissionIDValue, output)
			}
			return runCrashLogByCrashLogID(ctx, d, crashLogIDValue, output)
		},
	}
}

func runCrashLogBySubmissionID(ctx context.Context, d *shared.Deps, submissionID string, output shared.OutputFlags) error {
	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("testflight crashes log: %w", err)
	}

	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	resp, err := client.GetBetaFeedbackCrashSubmissionCrashLog(requestCtx, submissionID)
	if err != nil {
		return crashLogBySubmissionFetchError(err, submissionID)
	}

	return d.PrintOutput(resp, *output.Output, *output.Pretty)
}
