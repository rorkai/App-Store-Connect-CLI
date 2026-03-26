package release

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/metadata"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	validatecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

const (
	stepEnsureVersion     = "ensure_version"
	stepApplyMetadata     = "apply_metadata"
	stepAttachBuild       = "attach_build"
	stepValidateReadiness = "validate_readiness"
	stepSubmitReview      = "submit_review"
	releaseModeRun        = "run"
	releaseModeStage      = "stage"
	releaseRunTimeout     = 30 * time.Minute
)

var (
	releaseClientFactory   = shared.GetASCClient
	metadataPushExecutor   = metadata.ExecutePush
	metadataCopyExecutor   = shared.CopyVersionMetadataFromSource
	readinessReportBuilder = validatecli.BuildReadinessReport
)

type metadataCopyOptions = shared.VersionMetadataCopyOptions

type runOptions struct {
	AppID              string
	Version            string
	BuildID            string
	MetadataDir        string
	CopyMetadataFrom   string
	SelectedCopyFields []string
	Platform           string
	Timeout            time.Duration
	DryRun             bool
	Confirm            bool
	StrictValidate     bool
	CheckpointFile     string
	Mode               string
	SubmitForReview    bool
}

type stepResult struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
	Remediation string `json:"remediation,omitempty"`
	DurationMS  int64  `json:"durationMs"`
	Details     any    `json:"details,omitempty"`
}

type runResult struct {
	AppID            string       `json:"appId"`
	Version          string       `json:"version"`
	VersionID        string       `json:"versionId,omitempty"`
	BuildID          string       `json:"buildId"`
	SubmissionID     string       `json:"submissionId,omitempty"`
	MetadataDir      string       `json:"metadataDir,omitempty"`
	CopyMetadataFrom string       `json:"copyMetadataFrom,omitempty"`
	Platform         string       `json:"platform"`
	DryRun           bool         `json:"dryRun"`
	StrictValidate   bool         `json:"strictValidate,omitempty"`
	CheckpointFile   string       `json:"checkpointFile,omitempty"`
	Resumed          bool         `json:"resumed,omitempty"`
	Status           string       `json:"status"`
	FailedStep       string       `json:"failedStep,omitempty"`
	Error            string       `json:"error,omitempty"`
	Steps            []stepResult `json:"steps"`
}

type runCheckpoint struct {
	AppID              string          `json:"appId"`
	Version            string          `json:"version"`
	BuildID            string          `json:"buildId"`
	MetadataDir        string          `json:"metadataDir,omitempty"`
	CopyMetadataFrom   string          `json:"copyMetadataFrom,omitempty"`
	SelectedCopyFields []string        `json:"selectedCopyFields,omitempty"`
	Platform           string          `json:"platform"`
	VersionID          string          `json:"versionId,omitempty"`
	SubmissionID       string          `json:"submissionId,omitempty"`
	Mode               string          `json:"mode,omitempty"`
	Completed          map[string]bool `json:"completed"`
	UpdatedAt          string          `json:"updatedAt,omitempty"`
}

type stepOutcome struct {
	Status       string
	Message      string
	Details      any
	Persist      bool
	ResolvedID   string
	SubmissionID string
}

// ReleaseRunCommand runs the end-to-end release orchestration flow.
func ReleaseRunCommand() *ffcli.Command {
	fs := flag.NewFlagSet("release run", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string (required)")
	buildID := fs.String("build", "", "Build ID to attach (required)")
	metadataDir := fs.String("metadata-dir", "", "Metadata directory to apply (required)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	timeout := fs.Duration("timeout", releaseRunTimeout, "Maximum time to run the release pipeline")
	dryRun := fs.Bool("dry-run", false, "Preview deterministic plan without mutations")
	confirm := fs.Bool("confirm", false, "Confirm release mutations (required unless --dry-run)")
	strictValidate := fs.Bool("strict-validate", false, "Treat readiness warnings as blocking")
	checkpointFile := fs.String("checkpoint-file", "", "Checkpoint path for resumable runs")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "run",
		ShortUsage: "asc release run --app \"APP_ID\" --version \"2.4.0\" --build \"BUILD_ID\" --metadata-dir \"./metadata/version/2.4.0\" [flags]",
		ShortHelp:  "Run version + metadata + attach + validate + submit.",
		LongHelp: `Run a deterministic App Store release pipeline:
1. Ensure/create version
2. Apply metadata/localizations
3. Attach selected build
4. Run readiness checks
5. Submit for review

Supports dry-run planning, step-level structured output, and checkpointed resume.

Examples:
  asc release run --app "APP_ID" --version "2.4.0" --build "BUILD_ID" --metadata-dir "./metadata/version/2.4.0" --dry-run
  asc release run --app "APP_ID" --version "2.4.0" --build "BUILD_ID" --metadata-dir "./metadata/version/2.4.0" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("release run does not accept positional arguments")
			}
			if !*dryRun && !*confirm {
				return shared.UsageError("--confirm is required unless --dry-run is set")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if strings.TrimSpace(resolvedAppID) == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			trimmedVersion := strings.TrimSpace(*version)
			if trimmedVersion == "" {
				return shared.UsageError("--version is required")
			}
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				return shared.UsageError("--build is required")
			}
			trimmedMetadataDir := strings.TrimSpace(*metadataDir)
			if trimmedMetadataDir == "" {
				return shared.UsageError("--metadata-dir is required")
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if *timeout <= 0 {
				return shared.UsageError("--timeout must be greater than 0")
			}

			checkpointPath := strings.TrimSpace(*checkpointFile)
			if checkpointPath == "" {
				checkpointPath = defaultCheckpointPath(resolvedAppID, trimmedVersion, trimmedBuildID, normalizedPlatform)
			}
			absCheckpointPath, err := filepath.Abs(checkpointPath)
			if err != nil {
				return fmt.Errorf("release run: resolve checkpoint path: %w", err)
			}

			result, runErr := executeRun(ctx, runOptions{
				AppID:          resolvedAppID,
				Version:        trimmedVersion,
				BuildID:        trimmedBuildID,
				MetadataDir:    trimmedMetadataDir,
				Platform:       normalizedPlatform,
				Timeout:        *timeout,
				DryRun:         *dryRun,
				Confirm:        *confirm,
				StrictValidate: *strictValidate,
				CheckpointFile: absCheckpointPath,
			})

			if printErr := shared.PrintOutput(result, *output.Output, *output.Pretty); printErr != nil {
				return printErr
			}
			if runErr != nil {
				return shared.NewReportedError(runErr)
			}
			return nil
		},
	}
}

func executeRun(ctx context.Context, opts runOptions) (runResult, error) {
	opts.Mode = releaseModeRun
	opts.SubmitForReview = true
	return executePipeline(ctx, opts)
}

func executeStage(ctx context.Context, opts runOptions) (runResult, error) {
	opts.Mode = releaseModeStage
	opts.SubmitForReview = false
	return executePipeline(ctx, opts)
}

func executePipeline(ctx context.Context, opts runOptions) (runResult, error) {
	stepCapacity := 4
	if opts.SubmitForReview {
		stepCapacity = 5
	}
	result := runResult{
		AppID:            opts.AppID,
		Version:          opts.Version,
		BuildID:          opts.BuildID,
		MetadataDir:      opts.MetadataDir,
		CopyMetadataFrom: opts.CopyMetadataFrom,
		Platform:         opts.Platform,
		DryRun:           opts.DryRun,
		StrictValidate:   opts.StrictValidate,
		CheckpointFile:   opts.CheckpointFile,
		Status:           "ok",
		Steps:            make([]stepResult, 0, stepCapacity),
	}
	if opts.DryRun {
		result.Status = "dry-run"
	}

	checkpoint := runCheckpoint{
		AppID:              opts.AppID,
		Version:            opts.Version,
		BuildID:            opts.BuildID,
		MetadataDir:        opts.MetadataDir,
		CopyMetadataFrom:   opts.CopyMetadataFrom,
		SelectedCopyFields: append([]string(nil), opts.SelectedCopyFields...),
		Platform:           opts.Platform,
		Mode:               opts.Mode,
		Completed:          map[string]bool{},
	}

	if !opts.DryRun {
		existing, err := loadCheckpoint(opts.CheckpointFile)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result, err
		}
		if existing != nil {
			if existing.AppID != opts.AppID ||
				existing.Version != opts.Version ||
				existing.BuildID != opts.BuildID ||
				existing.Platform != opts.Platform ||
				existing.MetadataDir != opts.MetadataDir ||
				existing.CopyMetadataFrom != opts.CopyMetadataFrom ||
				!equalStringSlices(existing.SelectedCopyFields, opts.SelectedCopyFields) ||
				!checkpointModeMatches(existing.Mode, opts.Mode) {
				err := fmt.Errorf("checkpoint does not match current run arguments")
				result.Status = "error"
				result.Error = err.Error()
				return result, err
			}
			checkpoint = *existing
			if checkpoint.Completed == nil {
				checkpoint.Completed = map[string]bool{}
			}
			result.Resumed = len(checkpoint.Completed) > 0
			result.VersionID = checkpoint.VersionID
			result.SubmissionID = checkpoint.SubmissionID
		}
	}

	client, err := releaseClientFactory()
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result, err
	}

	requestCtx, cancel := shared.ContextWithTimeoutDuration(ctx, opts.Timeout)
	defer cancel()

	versionID := strings.TrimSpace(checkpoint.VersionID)
	submissionID := strings.TrimSpace(checkpoint.SubmissionID)
	versionPlannedCreate := false

	runStep := func(name, remediation string, fn func() (stepOutcome, error)) error {
		start := time.Now()
		step := stepResult{Name: name}

		if !opts.DryRun && checkpoint.Completed[name] {
			step.Status = "skipped"
			step.Message = "skipped (already completed in checkpoint)"
			step.DurationMS = time.Since(start).Milliseconds()
			result.Steps = append(result.Steps, step)
			return nil
		}

		outcome, stepErr := fn()
		step.DurationMS = time.Since(start).Milliseconds()
		if stepErr != nil {
			step.Status = "error"
			if strings.TrimSpace(outcome.Message) != "" {
				step.Message = outcome.Message
			} else {
				step.Message = stepErr.Error()
			}
			step.Remediation = remediation
			step.Details = outcome.Details
			result.Steps = append(result.Steps, step)
			result.Status = "error"
			result.FailedStep = name
			result.Error = stepErr.Error()
			return stepErr
		}

		if strings.TrimSpace(outcome.Status) == "" {
			outcome.Status = "ok"
		}
		step.Status = outcome.Status
		step.Message = outcome.Message
		step.Details = outcome.Details
		result.Steps = append(result.Steps, step)

		if strings.TrimSpace(outcome.ResolvedID) != "" {
			versionID = strings.TrimSpace(outcome.ResolvedID)
			result.VersionID = versionID
			checkpoint.VersionID = versionID
		}
		if strings.TrimSpace(outcome.SubmissionID) != "" {
			submissionID = strings.TrimSpace(outcome.SubmissionID)
			result.SubmissionID = submissionID
			checkpoint.SubmissionID = submissionID
		}

		if !opts.DryRun && outcome.Persist {
			checkpoint.Completed[name] = true
			if saveErr := saveCheckpoint(opts.CheckpointFile, checkpoint); saveErr != nil {
				result.Status = "error"
				result.FailedStep = name
				result.Error = saveErr.Error()
				return saveErr
			}
		}

		return nil
	}

	if err := runStep(stepEnsureVersion, "Verify app/version/platform and ensure only one matching version exists.", func() (stepOutcome, error) {
		versionResp, getErr := client.GetAppStoreVersions(
			requestCtx,
			opts.AppID,
			asc.WithAppStoreVersionsVersionStrings([]string{opts.Version}),
			asc.WithAppStoreVersionsPlatforms([]string{opts.Platform}),
			asc.WithAppStoreVersionsLimit(10),
		)
		if getErr != nil {
			return stepOutcome{}, fmt.Errorf("ensure version: %w", getErr)
		}

		switch len(versionResp.Data) {
		case 0:
			if opts.DryRun {
				versionPlannedCreate = true
				return stepOutcome{
					Status:  "dry-run",
					Message: "would create app store version",
					Details: map[string]any{"action": "create", "platform": opts.Platform, "version": opts.Version},
					Persist: false,
				}, nil
			}
			created, createErr := client.CreateAppStoreVersion(requestCtx, opts.AppID, asc.AppStoreVersionCreateAttributes{
				Platform:      asc.Platform(opts.Platform),
				VersionString: opts.Version,
			})
			if createErr != nil {
				return stepOutcome{}, fmt.Errorf("ensure version: create app store version: %w", createErr)
			}
			return stepOutcome{
				Status:     "ok",
				Message:    "created app store version",
				Details:    map[string]any{"action": "created", "versionId": created.Data.ID},
				Persist:    true,
				ResolvedID: created.Data.ID,
			}, nil
		case 1:
			foundID := strings.TrimSpace(versionResp.Data[0].ID)
			status := "ok"
			message := "reused existing app store version"
			if opts.DryRun {
				status = "dry-run"
				message = "would reuse existing app store version"
			}
			return stepOutcome{
				Status:     status,
				Message:    message,
				Details:    map[string]any{"action": "reuse", "versionId": foundID},
				Persist:    !opts.DryRun,
				ResolvedID: foundID,
			}, nil
		default:
			return stepOutcome{}, fmt.Errorf("ensure version: multiple app store versions found for version %q and platform %q", opts.Version, opts.Platform)
		}
	}); err != nil {
		return result, err
	}

	if err := runStep(stepApplyMetadata, "Fix metadata files (try `asc metadata validate --dir <path>`) and rerun.", func() (stepOutcome, error) {
		if opts.DryRun && versionPlannedCreate && strings.TrimSpace(versionID) == "" {
			message := "metadata plan deferred until version exists"
			if strings.TrimSpace(opts.CopyMetadataFrom) != "" {
				message = "metadata copy plan deferred until version exists"
			}
			return stepOutcome{
				Status:  "dry-run",
				Message: message,
				Details: map[string]any{"deferred": true, "reason": "version would be created during real run"},
				Persist: false,
			}, nil
		}

		if strings.TrimSpace(opts.MetadataDir) != "" {
			pushResult, pushErr := metadataPushExecutor(requestCtx, metadata.PushExecutionOptions{
				AppID:        opts.AppID,
				Version:      opts.Version,
				Platform:     opts.Platform,
				Dir:          opts.MetadataDir,
				Include:      "localizations",
				DryRun:       opts.DryRun,
				AllowDeletes: false,
				Confirm:      false,
			})
			if pushErr != nil {
				return stepOutcome{}, fmt.Errorf("apply metadata: %w", pushErr)
			}

			changeCount := len(pushResult.Adds) + len(pushResult.Updates) + len(pushResult.Deletes)
			status := "ok"
			message := "applied metadata changes"
			if opts.DryRun {
				status = "dry-run"
				message = "computed metadata dry-run plan"
			}
			if changeCount == 0 {
				if opts.DryRun {
					message = "metadata already in sync (no planned changes)"
				} else {
					message = "metadata already in sync (no changes applied)"
				}
			}

			return stepOutcome{
				Status:  status,
				Message: message,
				Details: map[string]any{
					"adds":     len(pushResult.Adds),
					"updates":  len(pushResult.Updates),
					"deletes":  len(pushResult.Deletes),
					"apiCalls": pushResult.APICalls,
				},
				Persist: !opts.DryRun,
			}, nil
		}

		copySummary, copyErr := metadataCopyExecutor(requestCtx, client, metadataCopyOptions{
			AppID:                opts.AppID,
			Platform:             opts.Platform,
			SourceVersion:        opts.CopyMetadataFrom,
			DestinationVersionID: versionID,
			SelectedFields:       append([]string(nil), opts.SelectedCopyFields...),
			DryRun:               opts.DryRun,
		})
		if copyErr != nil {
			return stepOutcome{}, fmt.Errorf("apply metadata: %w", copyErr)
		}

		status := "ok"
		message := "copied metadata from source version"
		if opts.DryRun {
			status = "dry-run"
			message = "computed metadata copy dry-run plan"
		}
		if copySummary.CopiedLocales == 0 && copySummary.CopiedFieldUpdates == 0 {
			if opts.DryRun {
				message = "metadata copy already in sync (no planned changes)"
			} else {
				message = "metadata copy already in sync (no changes applied)"
			}
		}

		return stepOutcome{
			Status:  status,
			Message: message,
			Details: map[string]any{
				"summary": copySummary,
			},
			Persist: !opts.DryRun,
		}, nil
	}); err != nil {
		return result, err
	}

	if err := runStep(stepAttachBuild, "Ensure --build points to a valid processed build for this app.", func() (stepOutcome, error) {
		if strings.TrimSpace(versionID) == "" {
			if opts.DryRun {
				return stepOutcome{
					Status:  "dry-run",
					Message: "build attach deferred until version exists",
					Details: map[string]any{"deferred": true},
					Persist: false,
				}, nil
			}
			return stepOutcome{}, fmt.Errorf("attach build: resolved version ID is empty")
		}

		existingBuildID := ""
		buildResp, buildErr := client.GetAppStoreVersionBuild(requestCtx, versionID)
		if buildErr != nil {
			if !asc.IsNotFound(buildErr) {
				return stepOutcome{}, fmt.Errorf("attach build: failed to fetch current build: %w", buildErr)
			}
		} else {
			existingBuildID = strings.TrimSpace(buildResp.Data.ID)
		}

		if existingBuildID == opts.BuildID {
			status := "skipped"
			message := "build already attached"
			if opts.DryRun {
				status = "dry-run"
				message = "build already attached (no action needed)"
			}
			return stepOutcome{
				Status:  status,
				Message: message,
				Details: map[string]any{"buildId": opts.BuildID, "alreadyAttached": true},
				Persist: !opts.DryRun,
			}, nil
		}

		if opts.DryRun {
			return stepOutcome{
				Status:  "dry-run",
				Message: "would attach build to version",
				Details: map[string]any{"versionId": versionID, "buildId": opts.BuildID, "currentBuildId": existingBuildID},
				Persist: false,
			}, nil
		}

		if attachErr := client.AttachBuildToVersion(requestCtx, versionID, opts.BuildID); attachErr != nil {
			return stepOutcome{}, fmt.Errorf("attach build: %w", attachErr)
		}
		return stepOutcome{
			Status:  "ok",
			Message: "attached build to version",
			Details: map[string]any{"versionId": versionID, "buildId": opts.BuildID},
			Persist: true,
		}, nil
	}); err != nil {
		return result, err
	}

	if err := runStep(stepValidateReadiness, "Resolve readiness issues (`asc validate ...`) before submitting.", func() (stepOutcome, error) {
		if strings.TrimSpace(versionID) == "" {
			if opts.DryRun {
				return stepOutcome{
					Status:  "dry-run",
					Message: "readiness checks deferred until version exists",
					Details: map[string]any{"deferred": true},
					Persist: false,
				}, nil
			}
			return stepOutcome{}, fmt.Errorf("validate readiness: resolved version ID is empty")
		}

		report, reportErr := readinessReportBuilder(requestCtx, validatecli.ReadinessOptions{
			AppID:     opts.AppID,
			VersionID: versionID,
			Platform:  opts.Platform,
			Strict:    opts.StrictValidate,
		})
		if reportErr != nil {
			return stepOutcome{}, fmt.Errorf("validate readiness: %w", reportErr)
		}
		if report.Summary.Blocking > 0 {
			return stepOutcome{
				Message: "readiness checks reported blocking issues",
				Details: map[string]any{"report": report},
			}, fmt.Errorf("validate readiness: found %d blocking issue(s)", report.Summary.Blocking)
		}

		status := "ok"
		if opts.DryRun {
			status = "dry-run"
		}
		message := releaseReadinessSuccessMessage(report, opts.DryRun)
		return stepOutcome{
			Status:  status,
			Message: message,
			Details: map[string]any{"report": report},
			Persist: !opts.DryRun,
		}, nil
	}); err != nil {
		return result, err
	}

	if opts.SubmitForReview {
		if err := runStep(stepSubmitReview, "Check review submission prerequisites and rerun with --confirm.", func() (stepOutcome, error) {
			if strings.TrimSpace(versionID) == "" {
				if opts.DryRun {
					return stepOutcome{
						Status:  "dry-run",
						Message: "submission deferred until version exists",
						Details: map[string]any{"deferred": true},
						Persist: false,
					}, nil
				}
				return stepOutcome{}, fmt.Errorf("submit review: resolved version ID is empty")
			}

			legacySubmission, subErr := client.GetAppStoreVersionSubmissionForVersion(requestCtx, versionID)
			if subErr != nil && !asc.IsNotFound(subErr) {
				return stepOutcome{}, fmt.Errorf("submit review: failed to lookup existing submission: %w", subErr)
			}
			if subErr == nil && strings.TrimSpace(legacySubmission.Data.ID) != "" {
				existingID := strings.TrimSpace(legacySubmission.Data.ID)
				status := "skipped"
				message := "submission already exists for version"
				if opts.DryRun {
					status = "dry-run"
					message = "submission already exists for version (no action needed)"
				}
				return stepOutcome{
					Status:       status,
					Message:      message,
					Details:      map[string]any{"submissionId": existingID, "alreadySubmitted": true},
					Persist:      !opts.DryRun,
					SubmissionID: existingID,
				}, nil
			}

			if opts.DryRun {
				return stepOutcome{
					Status:  "dry-run",
					Message: "would create and submit review submission",
					Details: map[string]any{"versionId": versionID, "buildId": opts.BuildID},
					Persist: false,
				}, nil
			}

			warnings := cancelStaleReviewSubmissions(requestCtx, client, opts.AppID, opts.Platform)

			reviewSubmission, createErr := client.CreateReviewSubmission(requestCtx, opts.AppID, asc.Platform(opts.Platform))
			if createErr != nil {
				return stepOutcome{}, fmt.Errorf("submit review: create review submission: %w", createErr)
			}
			if _, addErr := client.AddReviewSubmissionItem(requestCtx, reviewSubmission.Data.ID, versionID); addErr != nil {
				return stepOutcome{}, fmt.Errorf("submit review: add version to submission: %w", addErr)
			}
			submitResp, submitErr := client.SubmitReviewSubmission(requestCtx, reviewSubmission.Data.ID)
			if submitErr != nil {
				return stepOutcome{}, fmt.Errorf("submit review: submit for review: %w", submitErr)
			}

			return stepOutcome{
				Status:       "ok",
				Message:      "submitted version for review",
				Details:      map[string]any{"submissionId": submitResp.Data.ID, "warnings": warnings},
				Persist:      true,
				SubmissionID: submitResp.Data.ID,
			}, nil
		}); err != nil {
			return result, err
		}
	}

	if strings.TrimSpace(result.SubmissionID) == "" {
		result.SubmissionID = strings.TrimSpace(submissionID)
	}
	if strings.TrimSpace(result.VersionID) == "" {
		result.VersionID = strings.TrimSpace(versionID)
	}

	return result, nil
}

func cancelStaleReviewSubmissions(ctx context.Context, client *asc.Client, appID, platform string) []string {
	warnings := make([]string, 0)
	existing, err := client.GetReviewSubmissions(
		ctx,
		appID,
		asc.WithReviewSubmissionsStates([]string{string(asc.ReviewSubmissionStateReadyForReview)}),
		asc.WithReviewSubmissionsPlatforms([]string{platform}),
	)
	if err != nil {
		return append(warnings, fmt.Sprintf("failed to query stale review submissions: %v", err))
	}
	normalizedPlatform := strings.ToUpper(strings.TrimSpace(platform))
	for _, sub := range existing.Data {
		if sub.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
			continue
		}
		if normalizedPlatform != "" && !strings.EqualFold(string(sub.Attributes.Platform), normalizedPlatform) {
			continue
		}
		if _, cancelErr := client.CancelReviewSubmission(ctx, sub.ID); cancelErr != nil {
			warnings = append(warnings, fmt.Sprintf("failed to cancel stale submission %s: %v", sub.ID, cancelErr))
		}
	}
	return warnings
}

func releaseReadinessSuccessMessage(report validation.Report, dryRun bool) string {
	message := "readiness checks passed"
	if summary := releaseReadinessNonBlockingSummary(report.Summary); summary != "" {
		message += " with " + summary
	}
	if hasReleaseReadinessCheckID(report.Checks, "privacy.publish_state.unverified") {
		message += "; App Privacy may still block submission"
	}
	if dryRun {
		message += " (dry-run)"
	}
	return message
}

func releaseReadinessNonBlockingSummary(summary validation.Summary) string {
	parts := make([]string, 0, 2)
	if summary.Warnings > 0 {
		parts = append(parts, releaseReadinessCountLabel(summary.Warnings, "warning", "warnings"))
	}
	if summary.Infos > 0 {
		parts = append(parts, releaseReadinessCountLabel(summary.Infos, "advisory", "advisories"))
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return parts[0] + " and " + parts[1]
	}
}

func releaseReadinessCountLabel(count int, singular, plural string) string {
	label := plural
	if count == 1 {
		label = singular
	}
	return fmt.Sprintf("%d %s", count, label)
}

func hasReleaseReadinessCheckID(checks []validation.CheckResult, wantID string) bool {
	for _, check := range checks {
		if strings.TrimSpace(check.ID) == wantID {
			return true
		}
	}
	return false
}

func defaultCheckpointPath(appID, version, buildID, platform string) string {
	fileName := fmt.Sprintf(
		"%s_%s_%s_%s.json",
		sanitizeCheckpointToken(appID),
		sanitizeCheckpointToken(version),
		sanitizeCheckpointToken(buildID),
		sanitizeCheckpointToken(platform),
	)
	return filepath.Join(".asc", "release", "checkpoints", fileName)
}

func defaultStageCheckpointPath(appID, version, buildID, platform string) string {
	fileName := fmt.Sprintf(
		"stage_%s_%s_%s_%s.json",
		sanitizeCheckpointToken(appID),
		sanitizeCheckpointToken(version),
		sanitizeCheckpointToken(buildID),
		sanitizeCheckpointToken(platform),
	)
	return filepath.Join(".asc", "release", "checkpoints", fileName)
}

func checkpointModeMatches(existingMode, desiredMode string) bool {
	normalizedExistingMode := strings.TrimSpace(existingMode)
	switch normalizedExistingMode {
	case "":
		return desiredMode == releaseModeRun
	default:
		return normalizedExistingMode == desiredMode
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sanitizeCheckpointToken(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	result := strings.Trim(b.String(), "._")
	if result == "" {
		return "unknown"
	}
	return result
}

func loadCheckpoint(path string) (*runCheckpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}
	var checkpoint runCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("parse checkpoint: %w", err)
	}
	if checkpoint.Completed == nil {
		checkpoint.Completed = map[string]bool{}
	}
	return &checkpoint, nil
}

func saveCheckpoint(path string, checkpoint runCheckpoint) error {
	checkpoint.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create checkpoint directory: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("persist checkpoint: %w", err)
	}
	return nil
}
