package screenshots

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type matrixSimulatorGuard struct {
	mu      sync.Mutex
	blocked bool
}

func executeMatrixCellWithSimulatorLock(ctx context.Context, cell MatrixCell, base *Plan, matrixPlan *MatrixPlan, maxAttempts int, backoff time.Duration, deps MatrixDependencies, outputRoots matrixOutputRoots, guard *matrixSimulatorGuard) MatrixCellResult {
	started := time.Now()
	result := newMatrixCellResult(cell)
	result.Status = MatrixCellFailed
	if guard == nil {
		guard = &matrixSimulatorGuard{}
	}
	guard.mu.Lock()
	defer guard.mu.Unlock()
	if guard.blocked {
		result.FailureStage = "appearance"
		result.FailureCode = "simulator_blocked_after_cleanup_failure"
		result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "simulator blocked after appearance cleanup failure")
		setMatrixScreenshotStatuses(&result)
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}

	if err := ctx.Err(); err != nil {
		return finishMatrixCellFailure(result, started, "execution", "canceled", "cell canceled")
	}
	state, err := deps.Appearance.Snapshot(ctx, cell.UDID)
	if err != nil {
		if ctx.Err() != nil {
			return finishMatrixCellFailure(result, started, "execution", "canceled", "cell canceled")
		}
		return finishMatrixCellFailure(result, started, "appearance", "snapshot_failed", "appearance state could not be read")
	}
	if err := deps.Appearance.Set(ctx, cell.UDID, cell.Appearance); err != nil {
		if ctx.Err() != nil {
			result = finishMatrixCellFailure(result, started, "execution", "canceled", "cell canceled")
		} else {
			result = finishMatrixCellFailure(result, started, "appearance", "set_failed", "requested appearance could not be applied")
		}
		if restoreErr := restoreMatrixAppearance(deps.Appearance, cell.UDID, state); restoreErr != nil {
			guard.blocked = true
			result.Status = MatrixCellCleanupFailed
			result.FailureStage = "cleanup"
			result.FailureCode = "appearance_restore_failed"
			result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "simulator appearance could not be restored")
		}
		setMatrixScreenshotStatuses(&result)
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result.Attempts = attempt
		if err := ctx.Err(); err != nil {
			result.Status = MatrixCellCanceled
			result.FailureStage = "execution"
			result.FailureCode = "canceled"
			result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "cell canceled")
			break
		}
		attemptResult, attemptErr := executeMatrixCellAttempt(ctx, cell, base, matrixPlan, deps, outputRoots)
		result.Steps = attemptResult.Steps
		mergeMatrixAttemptResult(&result, cell, attemptResult)
		if attemptErr == nil {
			result.Status = MatrixCellSuccess
			result.FailureStage = ""
			result.FailureCode = ""
			result.Error = nil
			break
		}
		if ctx.Err() != nil && !attemptResult.CleanupFailed {
			result.Status = MatrixCellCanceled
			if attemptResult.FailureStage == "framing" {
				result.FailureStage = "framing"
			} else {
				result.FailureStage = "execution"
			}
			result.FailureCode = "canceled"
			result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "cell canceled")
			break
		}
		if attemptResult.CleanupFailed {
			result.Status = MatrixCellCleanupFailed
		}
		if attemptResult.SimulatorDirty {
			guard.blocked = true
		}
		result.FailureStage = attemptResult.FailureStage
		result.FailureCode = attemptResult.FailureCode
		result.Error = newMatrixCellError(attemptResult.FailureStage, attemptResult.FailureCode, attemptResult.Error)
		if attempt == maxAttempts || attemptResult.CleanupFailed || (attemptResult.FailureStage != "execution" && attemptResult.FailureStage != "framing") {
			break
		}
		if err := waitContext(ctx, backoff); err != nil {
			result.Status = MatrixCellCanceled
			if result.FailureStage != "framing" {
				result.FailureStage = "execution"
			}
			result.FailureCode = "canceled"
			result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "cell canceled")
			break
		}
	}

	restoreErr := restoreMatrixAppearance(deps.Appearance, cell.UDID, state)
	if restoreErr != nil {
		guard.blocked = true
		result.Status = MatrixCellCleanupFailed
		result.FailureStage = "cleanup"
		result.FailureCode = "appearance_restore_failed"
		result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "simulator appearance could not be restored")
	} else if result.Status == MatrixCellFailed && result.FailureCode == "" {
		result.FailureStage = "execution"
		result.FailureCode = "execution_failed"
		result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "cell execution failed")
	}
	setMatrixScreenshotStatuses(&result)
	result.DurationMS = time.Since(started).Milliseconds()
	return result
}

func restoreMatrixAppearance(appearance MatrixAppearance, udid, state string) error {
	restoreCtx, cancel := context.WithTimeout(context.Background(), matrixSubprocessTimeout)
	defer cancel()
	return appearance.Restore(restoreCtx, udid, state)
}

type matrixAttemptResult struct {
	RawPaths        []string
	RawArtifacts    map[string]matrixArtifactInfo
	FramedPaths     []string
	FramedArtifacts map[string]matrixArtifactInfo
	CleanupFailed   bool
	SimulatorDirty  bool
	Screenshots     []MatrixScreenshotResult
	Steps           []RunStepResult
	FailureStage    string
	FailureCode     string
	Error           string
}

func matrixPrivateAttemptConstructionResult(stage, message string, err error) (matrixAttemptResult, error) {
	result := matrixAttemptResult{FailureStage: stage, FailureCode: "temporary_output_failed", Error: message}
	if errors.Is(err, errMatrixPrivateAttemptCleanupUncertain) {
		result.CleanupFailed = true
		result.FailureStage = "cleanup"
		result.FailureCode = "temporary_output_cleanup_failed"
		result.Error = "temporary screenshot output cleanup is uncertain"
	}
	return result, err
}

func executeMatrixCellAttempt(ctx context.Context, cell MatrixCell, base *Plan, matrixPlan *MatrixPlan, deps MatrixDependencies, outputRoots matrixOutputRoots) (attempt matrixAttemptResult, returnErr error) {
	rawRelative, err := relativeMatrixOutputPath(outputRoots.rawPath, cell.RawDir)
	if err != nil {
		return matrixAttemptResult{FailureStage: "execution", FailureCode: "temporary_output_failed", Error: "temporary output directory could not be created"}, err
	}
	if err := outputRoots.raw.MkdirAll(rawRelative, 0o755); err != nil {
		return matrixAttemptResult{FailureStage: "execution", FailureCode: "temporary_output_failed", Error: "temporary output directory could not be created"}, err
	}
	attemptRoot, err := createMatrixPrivateAttemptRoot()
	if err != nil {
		return matrixPrivateAttemptConstructionResult("execution", "temporary output directory could not be created", err)
	}
	defer func() {
		cleanupErr := cleanupMatrixPrivateAttemptForExecution(&attemptRoot)
		closeErr := closeMatrixPrivateAttemptForExecution(&attemptRoot)
		joinMatrixAttemptResourceErrors(&attempt, &returnErr, errors.Join(cleanupErr, closeErr))
	}()
	attemptDir := attemptRoot.path
	var planOutputRoot rootfs.Root
	planUsesRootedOutput := deps.RunPlan == nil
	if planUsesRootedOutput {
		planOutputRoot, err = openMatrixPrivateAttemptOutputRoot(&attemptRoot)
		if err != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "temporary output directory could not be created"
			return attempt, err
		}
		defer func() {
			if closeErr := planOutputRoot.Close(); closeErr != nil {
				joinMatrixAttemptResourceErrors(&attempt, &returnErr, closeErr)
			}
		}()
		if err := lockMatrixPrivateAttemptChild(&attemptRoot); err != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "temporary output directory could not be protected"
			return attempt, err
		}
	}
	plan, err := cloneScreenshotPlan(base)
	if err != nil {
		return matrixAttemptResult{FailureStage: "execution", FailureCode: "plan_clone_failed", Error: "base plan could not be prepared"}, err
	}
	plan.App.UDID = cell.UDID
	if planUsesRootedOutput {
		plan.App.OutputDir = planOutputRoot.Path()
	} else {
		plan.App.OutputDir = attemptDir
	}
	plan.App.LaunchArguments = append(append([]string(nil), base.App.LaunchArguments...), cell.LaunchArguments...)
	plan.App.terminateRunningProcess = true
	ensureMatrixLaunchStep(plan)
	var runResult *RunResult
	if deps.RunPlan != nil {
		runResult, err = deps.RunPlan(ctx, plan)
	} else {
		runResult, err = deps.RunPlanRooted(ctx, plan, planOutputRoot)
	}
	attempt = matrixAttemptResult{}
	if runResult != nil {
		attempt.Steps = sanitizeMatrixSteps(runResult.Steps)
	}
	stabilityErr := matrixPrivateAttemptRootIsStable(attemptRoot)
	if err != nil {
		if errors.Is(err, errStatusBarClearFailed) {
			attempt.CleanupFailed = true
			attempt.SimulatorDirty = true
			attempt.FailureStage = "cleanup"
			attempt.FailureCode = "status_bar_clear_failed"
			attempt.Error = "simulator status bar could not be cleared"
			return attempt, errors.Join(err, stabilityErr)
		}
		if stabilityErr != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "screenshot capture output changed during execution"
			return attempt, errors.Join(err, stabilityErr)
		}
		attempt.FailureStage = "execution"
		attempt.FailureCode = "plan_failed"
		attempt.Error = "screenshot plan execution failed"
		return attempt, err
	}
	if stabilityErr != nil {
		attempt.FailureStage = "execution"
		attempt.FailureCode = "temporary_output_failed"
		attempt.Error = "screenshot capture output changed during execution"
		return attempt, stabilityErr
	}
	if runResult == nil {
		attempt.FailureStage = "execution"
		attempt.FailureCode = "empty_result"
		attempt.Error = "screenshot plan returned no result"
		return attempt, errors.New("empty screenshot result")
	}
	sourceRoot := attemptRoot.root
	sourceRootPath := attemptDir
	if planUsesRootedOutput {
		sourceRoot = planOutputRoot
		sourceRootPath = planOutputRoot.Path()
	}
	sources := make([]string, 0, len(cell.RawPaths))
	names := make([]string, 0, len(cell.RawPaths))
	dimensionsList := make([]asc.ImageDimensions, 0, len(cell.RawPaths))
	for _, rawPath := range cell.RawPaths {
		name := filepath.Base(rawPath)
		source := filepath.Join(sourceRootPath, name)
		sourceRelative := name
		sourceFile, openErr := sourceRoot.OpenFile(sourceRelative)
		if openErr != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "missing_screenshot"
			attempt.Error = "screenshot plan did not produce every requested image"
			return attempt, openErr
		}
		dimensions, imageErr := readMatrixImageDimensions(sourceFile, source)
		closeErr := sourceFile.Close()
		if imageErr != nil || closeErr != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "invalid_screenshot"
			attempt.Error = "screenshot plan produced an invalid image"
			if imageErr != nil {
				return attempt, imageErr
			}
			return attempt, closeErr
		}
		sources = append(sources, source)
		names = append(names, strings.TrimSuffix(name, filepath.Ext(name)))
		dimensionsList = append(dimensionsList, dimensions)
	}
	for index, rawPath := range cell.RawPaths {
		source := sources[index]
		destination := rawPath
		artifact, err := promoteMatrixArtifactWithInfoFromRoots(ctx, sourceRoot, sourceRootPath, source, outputRoots.raw, outputRoots.rawPath, destination)
		if err != nil {
			attempt.FailureStage = "execution"
			attempt.FailureCode = "raw_output_failed"
			attempt.Error = "raw screenshot could not be promoted"
			return attempt, err
		}
		if attempt.RawArtifacts == nil {
			attempt.RawArtifacts = make(map[string]matrixArtifactInfo)
		}
		attempt.RawPaths = append(attempt.RawPaths, destination)
		attempt.RawArtifacts[destination] = artifact
		attempt.Screenshots = append(attempt.Screenshots, MatrixScreenshotResult{
			Name: names[index], Status: MatrixCellSuccess,
			RawPath: destination, Width: dimensionsList[index].Width, Height: dimensionsList[index].Height,
		})
	}

	if !matrixPlan.Output.Frame.Enabled {
		return attempt, nil
	}
	framedRelative, err := relativeMatrixOutputPath(outputRoots.framedPath, cell.FramedDir)
	if err != nil {
		attempt.FailureStage = "framing"
		attempt.FailureCode = "framed_output_failed"
		attempt.Error = "framed output directory could not be created"
		return attempt, err
	}
	if err := outputRoots.framed.MkdirAll(framedRelative, 0o755); err != nil {
		attempt.FailureStage = "framing"
		attempt.FailureCode = "framed_output_failed"
		attempt.Error = "framed output directory could not be created"
		return attempt, err
	}
	frameAttemptRoot, err := createMatrixPrivateAttemptRoot()
	if err != nil {
		construction, constructionErr := matrixPrivateAttemptConstructionResult("framing", "temporary frame output directory could not be created", err)
		attempt.CleanupFailed = construction.CleanupFailed
		attempt.FailureStage = construction.FailureStage
		attempt.FailureCode = construction.FailureCode
		attempt.Error = construction.Error
		return attempt, constructionErr
	}
	defer func() {
		cleanupErr := cleanupMatrixPrivateAttemptForExecution(&frameAttemptRoot)
		closeErr := closeMatrixPrivateAttemptForExecution(&frameAttemptRoot)
		joinMatrixAttemptResourceErrors(&attempt, &returnErr, errors.Join(cleanupErr, closeErr))
	}()
	frameAttemptDir := frameAttemptRoot.path
	var frameOutputRoot rootfs.Root
	frameUsesRootedOutput := deps.Frame == nil
	if frameUsesRootedOutput {
		frameOutputRoot, err = openMatrixPrivateAttemptOutputRoot(&frameAttemptRoot)
		if err != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "temporary frame output directory could not be created"
			return attempt, err
		}
		defer func() {
			if closeErr := frameOutputRoot.Close(); closeErr != nil {
				joinMatrixAttemptResourceErrors(&attempt, &returnErr, closeErr)
			}
		}()
		if err := lockMatrixPrivateAttemptChild(&frameAttemptRoot); err != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "temporary frame output directory could not be protected"
			return attempt, err
		}
	}
	frameDevice, frameMappingFound := matrixFrameMappingForDevice(matrixPlan.Output.Frame.DeviceByMatrixDevice, cell.Device)
	if !frameMappingFound || strings.TrimSpace(frameDevice) == "" {
		attempt.FailureStage = "framing"
		attempt.FailureCode = "framing_mapping_missing"
		attempt.Error = "framing device mapping could not be resolved"
		return attempt, errors.New("framing device mapping is missing")
	}
	frameDevice = strings.TrimSpace(frameDevice)
	frameSourceRoot := frameAttemptRoot.root
	frameSourceRootPath := frameAttemptDir
	if frameUsesRootedOutput {
		frameSourceRoot = frameOutputRoot
		frameSourceRootPath = frameOutputRoot.Path()
	}
	if err := verifyMatrixRetainedOutputRoot(outputRoots.raw); err != nil {
		attempt.FailureStage = "framing"
		attempt.FailureCode = "raw_output_failed"
		attempt.Error = "raw screenshot root changed before framing"
		return attempt, err
	}
	frameSources := make([]string, 0, len(attempt.RawPaths))
	for index, inputPath := range attempt.RawPaths {
		contained, containsErr := outputRoots.raw.ContainsPath(inputPath)
		if containsErr != nil || !contained {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "raw_output_failed"
			attempt.Error = "raw screenshot is no longer inside the retained raw root"
			if containsErr != nil {
				return attempt, containsErr
			}
			return attempt, errors.New("raw screenshot left the retained raw root")
		}
		tempFrame := filepath.Join(frameSourceRootPath, filepath.Base(cell.FramedPaths[index]))
		frameInputPath := inputPath
		if deps.Frame == nil {
			pinnedInput, pinErr := pinMatrixFrameInputFromRetainedRoot(ctx, outputRoots.raw, inputPath, frameSourceRoot, frameSourceRootPath, index)
			if pinErr != nil {
				attempt.FailureStage = "framing"
				attempt.FailureCode = "raw_output_failed"
				attempt.Error = "raw screenshot could not be pinned for framing"
				return attempt, pinErr
			}
			frameInputPath = pinnedInput
		}
		frameRequest := FrameRequest{InputPath: frameInputPath, OutputPath: tempFrame, Device: frameDevice}
		var frameResult *FrameResult
		var frameErr error
		if deps.Frame != nil {
			frameResult, frameErr = deps.Frame(ctx, frameRequest)
		} else {
			frameResult, frameErr = deps.FrameRooted(ctx, frameRequest, frameOutputRoot)
		}
		stabilityErr := matrixPrivateAttemptRootIsStable(frameAttemptRoot)
		if frameErr != nil || frameResult == nil {
			attempt.FailureStage = "framing"
			if stabilityErr != nil {
				attempt.FailureCode = "temporary_output_failed"
				attempt.Error = "framing output changed during execution"
				return attempt, errors.Join(frameErr, stabilityErr)
			}
			attempt.FailureCode = "frame_failed"
			attempt.Error = "screenshot framing failed"
			if frameErr != nil {
				return attempt, frameErr
			}
			return attempt, errors.New("frame failed")
		}
		if stabilityErr != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "temporary_output_failed"
			attempt.Error = "framing output changed during execution"
			return attempt, stabilityErr
		}
		tempFrameFile, openErr := frameSourceRoot.OpenFile(filepath.Base(tempFrame))
		if openErr != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "invalid_frame"
			attempt.Error = "screenshot framing produced an invalid image"
			return attempt, openErr
		}
		_, imageErr := readMatrixImageDimensions(tempFrameFile, tempFrame)
		closeErr := tempFrameFile.Close()
		if imageErr != nil || closeErr != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "invalid_frame"
			attempt.Error = "screenshot framing produced an invalid image"
			if imageErr != nil {
				return attempt, imageErr
			}
			return attempt, closeErr
		}
		frameSources = append(frameSources, tempFrame)
	}
	for index, tempFrame := range frameSources {
		artifact, err := promoteMatrixArtifactWithInfoFromRoots(ctx, frameSourceRoot, frameSourceRootPath, tempFrame, outputRoots.framed, outputRoots.framedPath, cell.FramedPaths[index])
		if err != nil {
			attempt.FailureStage = "framing"
			attempt.FailureCode = "framed_output_failed"
			attempt.Error = "framed screenshot could not be promoted"
			return attempt, err
		}
		if attempt.FramedArtifacts == nil {
			attempt.FramedArtifacts = make(map[string]matrixArtifactInfo)
		}
		attempt.FramedPaths = append(attempt.FramedPaths, cell.FramedPaths[index])
		attempt.FramedArtifacts[cell.FramedPaths[index]] = artifact
		attempt.Screenshots[index].FramedPath = cell.FramedPaths[index]
	}
	return attempt, nil
}

func sanitizeMatrixSteps(steps []RunStepResult) []RunStepResult {
	if len(steps) == 0 {
		return nil
	}
	sanitized := make([]RunStepResult, len(steps))
	for index, step := range steps {
		sanitized[index] = RunStepResult{
			Index: step.Index, Action: step.Action, Status: step.Status, DurationMS: step.DurationMS,
		}
		if strings.TrimSpace(step.Error) != "" {
			sanitized[index].Error = "step execution failed"
		}
	}
	return sanitized
}

// ensureMatrixLaunchStep guarantees the cell's app session is established with
// this cell's locale and content-variant launch arguments before any step that
// observes or drives the app runs.
//
// A plain "does the plan launch anywhere" check is not sufficient: a valid base
// plan may place a screenshot or interaction before a later launch, and those
// early steps would then run against whatever session the simulator already had,
// producing artifacts mislabeled for the requested axes. Only leading
// app-independent steps may precede the launch.
func ensureMatrixLaunchStep(plan *Plan) {
	for _, step := range plan.Steps {
		if matrixStepIsAppIndependent(step.Action) {
			continue
		}
		if step.Action == ActionLaunch {
			return
		}
		break
	}
	plan.Steps = append([]PlanStep{{Action: ActionLaunch}}, plan.Steps...)
}

// matrixStepIsAppIndependent reports whether a step neither observes nor drives
// the app under test, and may therefore precede the matrix launch. Only an
// unconditional delay qualifies; every other action reads or manipulates app
// state. ActionWaitFor is excluded because it polls for on-screen content.
func matrixStepIsAppIndependent(action StepAction) bool {
	return action == ActionWait
}

func cloneScreenshotPlan(base *Plan) (*Plan, error) {
	data, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	var clone Plan
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}

func executeMatrixCell(ctx context.Context, cell MatrixCell, base *Plan, matrixPlan *MatrixPlan, maxAttempts int, backoff time.Duration, deps MatrixDependencies, outputRoots matrixOutputRoots, guard *matrixSimulatorGuard) MatrixCellResult {
	started := time.Now()
	result := newMatrixCellResult(cell)
	result.Status = MatrixCellFailed
	release, lockErr := acquireMatrixSimulatorLock(ctx, cell.UDID)
	if lockErr != nil {
		if isMatrixContextTermination(lockErr) {
			return finishMatrixCellFailure(result, started, "execution", "canceled", "cell canceled")
		}
		result.FailureStage = "appearance"
		result.FailureCode = "simulator_lock_failed"
		result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "simulator lock could not be acquired")
		setMatrixScreenshotStatuses(&result)
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}
	result = executeMatrixCellWithSimulatorLock(ctx, cell, base, matrixPlan, maxAttempts, backoff, deps, outputRoots, guard)
	if releaseErr := release(); releaseErr != nil {
		result.Status = MatrixCellCleanupFailed
		result.FailureStage = "cleanup"
		result.FailureCode = "simulator_lock_release_failed"
		result.Error = newMatrixCellError(result.FailureStage, result.FailureCode, "simulator lock could not be released")
		setMatrixScreenshotStatuses(&result)
	}
	return result
}
