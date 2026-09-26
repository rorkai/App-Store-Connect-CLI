package xcodecloud

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type xcodeCloudDoctorOptions struct {
	Wait         bool
	PollInterval time.Duration
	SkipLogs     bool
	SaveLogs     string
}

// XcodeCloudDoctorCommand returns the xcode-cloud doctor subcommand.
func XcodeCloudDoctorCommand() *ffcli.Command {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)

	runID := shared.BindResourceIDFlag(fs, "run-id", "ciBuildRuns", "Build run ID to diagnose")
	wait := fs.Bool("wait", false, "Wait for the build run to complete before diagnosing it")
	pollInterval := fs.Duration("poll-interval", 10*time.Second, "Poll interval when waiting")
	timeout := fs.Duration("timeout", 0, "Timeout for Xcode Cloud requests (0 = use ASC_TIMEOUT or 30m default)")
	skipLogs := fs.Bool("skip-logs", false, "Skip automatic inspection of failed-action log bundles")
	saveLogs := fs.String("save-logs", "", "Directory in which to retain inspected log bundles")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "doctor",
		ShortUsage: "asc xcode-cloud doctor --run-id \"BUILD_RUN_ID\" [flags]",
		ShortHelp:  "Diagnose an Xcode Cloud build run and inspect failure logs.",
		LongHelp: `Diagnose an Xcode Cloud build run.

The command combines run status, actions, issues, and artifacts into one report.
For failed runs, it inspects failed-action LOG_BUNDLE artifacts in memory and
reports App Store import diagnostics when those details are present. Use
--save-logs to retain the downloaded bundles. A failed build is report data and
does not make doctor exit non-zero.

Examples:
  asc xcode-cloud doctor --run-id "BUILD_RUN_ID"
  asc xcode-cloud doctor --run-id "BUILD_RUN_ID" --wait
  asc xcode-cloud doctor --run-id "BUILD_RUN_ID" --wait --save-logs ./xcode-cloud-logs
  asc xcode-cloud doctor --run-id "BUILD_RUN_ID" --skip-logs --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.WithDiagnostic(shared.UsageError("xcode-cloud doctor does not accept positional arguments"), shared.DiagnosticInvalidInput, "")
			}
			runIDValue := strings.TrimSpace(*runID)
			if runIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --run-id is required")
				return shared.MissingRequiredUsageError("--run-id")
			}
			if *timeout < 0 {
				return shared.UsageError("--timeout must be greater than or equal to 0")
			}
			if *wait && *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}
			if !*wait && flagWasSet(fs, "poll-interval") {
				return shared.UsageError("--poll-interval requires --wait")
			}
			if *skipLogs && strings.TrimSpace(*saveLogs) != "" {
				return shared.UsageError("--save-logs and --skip-logs are mutually exclusive")
			}
			if flagWasSet(fs, "save-logs") && strings.TrimSpace(*saveLogs) == "" {
				return shared.UsageError("--save-logs must not be empty")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud doctor: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, *timeout)
			defer cancel()

			result, err := diagnoseXcodeCloudRun(requestCtx, client, runIDValue, xcodeCloudDoctorOptions{
				Wait:         *wait,
				PollInterval: *pollInterval,
				SkipLogs:     *skipLogs,
				SaveLogs:     strings.TrimSpace(*saveLogs),
			})
			if err != nil {
				return fmt.Errorf("xcode-cloud doctor: %w", err)
			}

			return printXcodeCloudDoctorResult(result, *output.Output, *output.Pretty)
		},
	}
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(current *flag.Flag) {
		if current.Name == name {
			set = true
		}
	})
	return set
}

func diagnoseXcodeCloudRun(ctx context.Context, client *asc.Client, runID string, options xcodeCloudDoctorOptions) (*asc.XcodeCloudDoctorResult, error) {
	var (
		run *asc.CiBuildRunResponse
		err error
	)
	if options.Wait {
		run, err = waitForBuildRunForDoctor(ctx, client, runID, options.PollInterval)
	} else {
		run, err = getCiBuildRun(ctx, client, runID)
	}
	if err != nil {
		return nil, err
	}

	actions, err := listBuildActionsForRunAllowEmpty(ctx, client, runID)
	if err != nil {
		return nil, err
	}

	result := &asc.XcodeCloudDoctorResult{
		Run:              buildStatusResult(run),
		Actions:          make([]asc.XcodeCloudDoctorAction, 0, len(actions)),
		LogBundles:       make([]asc.XcodeCloudDoctorLogBundle, 0),
		CoverageWarnings: make([]asc.XcodeCloudDoctorCoverageWarning, 0),
	}
	for _, action := range actions {
		actionResult, err := diagnoseXcodeCloudAction(ctx, client, action)
		if err != nil {
			return nil, err
		}
		result.Actions = append(result.Actions, actionResult)
	}

	summarizeXcodeCloudDoctorResult(result)
	if options.SkipLogs && doctorFailedActionLogBundleCount(result) > 0 && doctorRunFailed(result) {
		result.CoverageWarnings = append(result.CoverageWarnings, asc.XcodeCloudDoctorCoverageWarning{
			ID:          "log_bundle_inspection_skipped",
			Message:     "Log bundle inspection was disabled with --skip-logs.",
			Remediation: "Re-run without --skip-logs to inspect failed-action log bundles.",
		})
	}
	if shouldInspectDoctorLogs(result, options) {
		if err := inspectXcodeCloudDoctorLogs(ctx, client, result, options); err != nil {
			return nil, err
		}
	}
	finishXcodeCloudDoctorResult(result)
	return result, nil
}

func waitForBuildRunForDoctor(ctx context.Context, client *asc.Client, runID string, pollInterval time.Duration) (*asc.CiBuildRunResponse, error) {
	lastStatus := "unknown"
	run, err := asc.PollUntil(ctx, pollInterval, func(ctx context.Context) (*asc.CiBuildRunResponse, bool, error) {
		resp, err := getCiBuildRun(ctx, client, runID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to check status: %w", err)
		}
		lastStatus = string(resp.Data.Attributes.ExecutionProgress)
		return resp, asc.IsBuildRunComplete(resp.Data.Attributes.ExecutionProgress), nil
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("canceled waiting for build run %s (last status: %s)", runID, lastStatus)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("timed out waiting for build run %s (last status: %s)", runID, lastStatus)
		}
		return nil, err
	}
	return run, nil
}

func diagnoseXcodeCloudAction(ctx context.Context, client *asc.Client, action asc.CiBuildActionResource) (asc.XcodeCloudDoctorAction, error) {
	actionID := strings.TrimSpace(action.ID)
	result := asc.XcodeCloudDoctorAction{
		ID:                actionID,
		Name:              action.Attributes.Name,
		ActionType:        action.Attributes.ActionType,
		ExecutionProgress: string(action.Attributes.ExecutionProgress),
		CompletionStatus:  string(action.Attributes.CompletionStatus),
		IsRequiredToPass:  action.Attributes.IsRequiredToPass,
		Issues:            make([]asc.XcodeCloudDoctorIssue, 0),
		Artifacts:         make([]asc.XcodeCloudDoctorArtifact, 0),
	}
	if actionID == "" {
		return result, nil
	}

	issues, err := listAllXcodeCloudActionIssues(ctx, client, actionID)
	if err != nil {
		return result, fmt.Errorf("list issues for action %q: %w", actionID, err)
	}
	for _, issue := range issues {
		result.Issues = append(result.Issues, asc.XcodeCloudDoctorIssue{
			ID:         issue.ID,
			IssueType:  issue.Attributes.IssueType,
			Category:   issue.Attributes.Category,
			Message:    issue.Attributes.Message,
			FileSource: issue.Attributes.FileSource,
		})
	}

	artifacts, err := listAllXcodeCloudActionArtifacts(ctx, client, actionID)
	if err != nil {
		return result, fmt.Errorf("list artifacts for action %q: %w", actionID, err)
	}
	for _, artifact := range artifacts {
		result.Artifacts = append(result.Artifacts, asc.XcodeCloudDoctorArtifact{
			ID:       artifact.ID,
			FileType: artifact.Attributes.FileType,
			FileName: artifact.Attributes.FileName,
			FileSize: artifact.Attributes.FileSize,
		})
	}
	return result, nil
}

func listAllXcodeCloudActionIssues(ctx context.Context, client *asc.Client, actionID string) ([]asc.CiIssueResource, error) {
	resp, err := client.GetCiBuildActionIssues(ctx, actionID, asc.WithCiIssuesLimit(200))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.Links.Next) == "" {
		return resp.Data, nil
	}
	allPages, err := asc.PaginateAll(ctx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetCiBuildActionIssues(ctx, actionID, asc.WithCiIssuesNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	allIssues, ok := allPages.(*asc.CiIssuesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected issues response type %T", allPages)
	}
	return allIssues.Data, nil
}

func listAllXcodeCloudActionArtifacts(ctx context.Context, client *asc.Client, actionID string) ([]asc.CiArtifactResource, error) {
	resp, err := client.GetCiBuildActionArtifacts(ctx, actionID, asc.WithCiArtifactsLimit(200))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.Links.Next) == "" {
		return resp.Data, nil
	}
	allPages, err := asc.PaginateAll(ctx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetCiBuildActionArtifacts(ctx, actionID, asc.WithCiArtifactsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	allArtifacts, ok := allPages.(*asc.CiArtifactsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected artifacts response type %T", allPages)
	}
	return allArtifacts.Data, nil
}

func summarizeXcodeCloudDoctorResult(result *asc.XcodeCloudDoctorResult) {
	result.Summary.TotalActions = len(result.Actions)
	for _, action := range result.Actions {
		switch strings.ToUpper(strings.TrimSpace(action.CompletionStatus)) {
		case "FAILED", "ERRORED":
			result.Summary.FailedActions++
		case "CANCELED":
			result.Summary.CanceledActions++
		case "SKIPPED":
			result.Summary.SkippedActions++
		}
		for _, issue := range action.Issues {
			switch strings.ToUpper(strings.TrimSpace(issue.IssueType)) {
			case "ERROR", "TEST_FAILURE":
				result.Summary.Errors++
			case "WARNING", "ANALYZER_WARNING":
				result.Summary.Warnings++
			}
		}
		result.Summary.Artifacts += len(action.Artifacts)
		for _, artifact := range action.Artifacts {
			if strings.EqualFold(strings.TrimSpace(artifact.FileType), "LOG_BUNDLE") {
				result.Summary.LogBundles++
			}
		}
	}
}

func shouldInspectDoctorLogs(result *asc.XcodeCloudDoctorResult, options xcodeCloudDoctorOptions) bool {
	if options.SkipLogs || result == nil {
		return false
	}
	if strings.TrimSpace(options.SaveLogs) != "" {
		return result.Summary.LogBundles > 0
	}
	return doctorRunFailed(result) && doctorFailedActionLogBundleCount(result) > 0
}

func finishXcodeCloudDoctorResult(result *asc.XcodeCloudDoctorResult) {
	status := strings.ToUpper(strings.TrimSpace(result.Run.CompletionStatus))
	if !strings.EqualFold(result.Run.ExecutionProgress, string(asc.CiBuildRunExecutionProgressComplete)) {
		result.Conclusion = "The Xcode Cloud build run is not complete."
		result.NextAction = "Re-run this command with --wait to diagnose the terminal result."
		return
	}
	if status == string(asc.CiBuildRunCompletionStatusSucceeded) {
		result.Conclusion = "The Xcode Cloud build run completed successfully."
		result.NextAction = "No corrective action is required."
		return
	}
	if status == string(asc.CiBuildRunCompletionStatusCanceled) {
		result.Conclusion = "The Xcode Cloud build run was canceled."
		result.NextAction = "Review the cancellation reason and start a new build run when ready."
		return
	}
	if status == string(asc.CiBuildRunCompletionStatusSkipped) {
		result.Conclusion = "The Xcode Cloud build run was skipped."
		result.NextAction = "Review the workflow conditions and action results to determine why the run was skipped."
		return
	}

	hasImportDiagnostic := false
	hasSuccessfulExport := false
	failedActionLogBundles := doctorFailedActionLogBundleCount(result)
	failedActionLogBundlesInspected := doctorFailedActionLogBundlesInspected(result)
	failedActionLogBundlesPartiallyInspected := failedActionLogBundlesInspected < failedActionLogBundles
	failedActionLogBundlesSaved := doctorFailedActionLogBundlesSaved(result)
	failedActionLogBundleInspectionFailed := doctorHasCoverageWarning(result, "log_bundle_inspection_failed")
	failedActionIDs := doctorFailedActionIDs(result)
	for _, bundle := range result.LogBundles {
		if _, failed := failedActionIDs[bundle.ActionID]; !failed {
			continue
		}
		if bundle.ExportStatus == "SUCCEEDED" {
			hasSuccessfulExport = true
		}
		if len(bundle.Diagnostics) > 0 {
			hasImportDiagnostic = true
		}
	}
	if hasImportDiagnostic {
		result.Conclusion = "The Xcode Cloud log bundles contain App Store import diagnostics."
		result.NextAction = "Resolve the reported ITMS diagnostics, then start a new build run."
		return
	}
	hasAppStorePreparationIssue := doctorHasAppStorePreparationIssue(result)
	if hasSuccessfulExport || hasAppStorePreparationIssue {
		if failedActionLogBundlesPartiallyInspected {
			if hasAppStorePreparationIssue {
				result.Conclusion = "Xcode Cloud reported an App Store Connect preparation failure, but one or more failed-action log bundles were not inspected."
			} else {
				result.Conclusion = "The archive export succeeded, but one or more failed-action log bundles were not inspected."
			}
			if failedActionLogBundleInspectionFailed {
				result.NextAction = "Follow the log bundle inspection remediation in this report, then check App Store Connect if the available logs contain no import detail."
			} else if failedActionLogBundlesSaved > 0 {
				result.NextAction = "Inspect the saved failed-action log bundles, then check App Store Connect if they contain no import detail."
			} else {
				result.NextAction = "Re-run without --skip-logs, then check App Store Connect if the logs still contain no import detail."
			}
		} else if hasSuccessfulExport {
			result.Conclusion = "The archive export succeeded, but Xcode Cloud reported a later failure without an ITMS-level import diagnostic."
			result.NextAction = "Check the App Store Connect delivery notification or build processing state for the server-side import rejection."
		} else {
			result.Conclusion = "Xcode Cloud reported an App Store Connect preparation failure without an ITMS-level import diagnostic."
			result.NextAction = "Check the App Store Connect delivery notification or build processing state for the server-side import rejection."
		}
		coverageMessage := "The Xcode Cloud API did not expose a detailed App Store import rejection."
		coverageRemediation := "Check the App Store Connect delivery notification or build processing state; do not infer an ITMS root cause from the generic Xcode Cloud issue."
		if failedActionLogBundlesPartiallyInspected {
			coverageMessage = "One or more failed-action log bundles were not inspected, so a detailed App Store import rejection may be missing."
			coverageRemediation = "Complete the log bundle inspection remediation in this report before checking the App Store Connect delivery notification or build processing state."
		} else if failedActionLogBundlesInspected > 0 {
			coverageMessage = "The Xcode Cloud API and inspected log bundles did not expose a detailed App Store import rejection."
		}
		result.CoverageWarnings = append(result.CoverageWarnings, asc.XcodeCloudDoctorCoverageWarning{
			ID:          "app_store_import_detail_unavailable",
			Message:     coverageMessage,
			Remediation: coverageRemediation,
		})
		return
	}

	if failedActionLogBundlesPartiallyInspected {
		result.Conclusion = "The Xcode Cloud build run failed, but one or more failed-action log bundles were not inspected."
		if failedActionLogBundleInspectionFailed {
			result.NextAction = "Follow the log bundle inspection remediation in this report to inspect the failed-action logs locally."
		} else if failedActionLogBundlesSaved > 0 {
			result.NextAction = "Inspect the saved failed-action log bundles for the underlying failure."
		} else {
			result.NextAction = "Re-run without --skip-logs or download the listed log bundle artifacts for inspection."
		}
	} else if result.Summary.Errors > 0 {
		result.Conclusion = "The Xcode Cloud build run failed and reported actionable issues."
		result.NextAction = "Resolve the reported issues, then start a new build run."
	} else {
		result.Conclusion = "The Xcode Cloud build run failed without a more specific diagnostic."
		result.NextAction = "Review the available issues and artifacts in the report."
	}
}

func doctorFailedActionLogBundleCount(result *asc.XcodeCloudDoctorResult) int {
	if result == nil {
		return 0
	}
	count := 0
	for _, action := range result.Actions {
		if !isFailedDoctorAction(action.CompletionStatus) {
			continue
		}
		for _, artifact := range action.Artifacts {
			if strings.EqualFold(strings.TrimSpace(artifact.FileType), "LOG_BUNDLE") {
				count++
			}
		}
	}
	return count
}

func doctorFailedActionIDs(result *asc.XcodeCloudDoctorResult) map[string]struct{} {
	failed := make(map[string]struct{})
	if result == nil {
		return failed
	}
	for _, action := range result.Actions {
		if isFailedDoctorAction(action.CompletionStatus) {
			failed[action.ID] = struct{}{}
		}
	}
	return failed
}

func doctorFailedActionLogBundlesInspected(result *asc.XcodeCloudDoctorResult) int {
	failedActionIDs := doctorFailedActionIDs(result)
	count := 0
	for _, bundle := range result.LogBundles {
		if _, failed := failedActionIDs[bundle.ActionID]; failed && bundle.Inspected {
			count++
		}
	}
	return count
}

func doctorFailedActionLogBundlesSaved(result *asc.XcodeCloudDoctorResult) int {
	failedActionIDs := doctorFailedActionIDs(result)
	count := 0
	for _, bundle := range result.LogBundles {
		if _, failed := failedActionIDs[bundle.ActionID]; failed && strings.TrimSpace(bundle.SavedPath) != "" {
			count++
		}
	}
	return count
}

func doctorHasCoverageWarning(result *asc.XcodeCloudDoctorResult, id string) bool {
	if result == nil {
		return false
	}
	for _, warning := range result.CoverageWarnings {
		if warning.ID == id {
			return true
		}
	}
	return false
}

func doctorRunFailed(result *asc.XcodeCloudDoctorResult) bool {
	if result == nil || result.Run == nil {
		return false
	}
	if !asc.IsBuildRunComplete(asc.CiBuildRunExecutionProgress(result.Run.ExecutionProgress)) {
		return false
	}
	status := strings.ToUpper(strings.TrimSpace(result.Run.CompletionStatus))
	return status == string(asc.CiBuildRunCompletionStatusFailed) ||
		status == string(asc.CiBuildRunCompletionStatusErrored)
}

func doctorHasAppStorePreparationIssue(result *asc.XcodeCloudDoctorResult) bool {
	for _, action := range result.Actions {
		if !isFailedDoctorAction(action.CompletionStatus) {
			continue
		}
		for _, issue := range action.Issues {
			switch strings.ToUpper(strings.TrimSpace(issue.IssueType)) {
			case "ERROR", "TEST_FAILURE":
			default:
				continue
			}
			text := strings.ToLower(issue.Category + " " + issue.Message)
			if strings.Contains(text, "app store connect") || strings.Contains(text, "prepare build for app store") {
				return true
			}
		}
	}
	return false
}
