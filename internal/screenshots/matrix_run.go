package screenshots

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// matrixOutputLocksAcquiredForTest is a test-only seam placed after all
// execution/review roots have been opened and their filesystem identities have
// been locked. It models a path replacement between lock acquisition and the
// first rooted operation without changing production behavior.
var matrixOutputLocksAcquiredForTest func()

// matrixOutputLockRootOpenedForTest observes compatibility-wrapper roots so
// tests can prove that a later open failure closes earlier descriptors.
var matrixOutputLockRootOpenedForTest func(*os.Root)

// matrixOutputLockReleaseErrForTest injects a release failure after the real
// output locks have been dropped, so tests can prove the printed result and
// the published review both stop reporting success when the command still
// returns an error.
var matrixOutputLockReleaseErrForTest error

// matrixBeforeReviewPublishForTest runs after cells are counted and before
// the review transaction, while output locks are still held.
var matrixBeforeReviewPublishForTest func()

// RunMatrix loads the base plan, validates the complete matrix, executes local
// cells, and writes a report even when execution is partially unsuccessful.
func RunMatrix(ctx context.Context, matrixPath string, matrixPlan *MatrixPlan, options MatrixOptions) (*MatrixResult, error) {
	return RunMatrixWithDependencies(ctx, matrixPath, matrixPlan, options, MatrixDependencies{})
}

// RunMatrixWithDependencies is the testable implementation of RunMatrix.
func RunMatrixWithDependencies(ctx context.Context, matrixPath string, matrixPlan *MatrixPlan, options MatrixOptions, dependencies MatrixDependencies) (*MatrixResult, error) {
	if dependencies.RunPlan != nil && dependencies.RunPlanRooted != nil {
		return nil, newMatrixValidationError(errors.New("RunPlan and RunPlanRooted are mutually exclusive"))
	}
	if dependencies.Frame != nil && dependencies.FrameRooted != nil {
		return nil, newMatrixValidationError(errors.New("Frame and FrameRooted are mutually exclusive"))
	}
	if matrixPlan == nil {
		return nil, newMatrixValidationError(errors.New("matrix plan is required"))
	}
	if matrixPlan.Version != 1 {
		return nil, newMatrixValidationError(fmt.Errorf("unsupported matrix plan version %d (expected 1)", matrixPlan.Version))
	}
	if strings.TrimSpace(matrixPlan.BasePlan) == "" {
		return nil, newMatrixValidationError(errors.New("base_plan is required"))
	}
	baseDir := matrixPlanSourceDir(matrixPath, matrixPlan.sourcePath)
	base, err := loadMatrixBasePlan(matrixPath, matrixPlan)
	if err != nil {
		return nil, newMatrixValidationError(fmt.Errorf("load base plan: %w", err))
	}
	if err := validateMatrixPlan(matrixPlan, base, baseDir); err != nil {
		return nil, newMatrixValidationError(err)
	}
	concurrency, attempts, backoff, err := resolveMatrixExecution(matrixPlan.Execution, options)
	if err != nil {
		return nil, newMatrixValidationError(err)
	}
	cells, err := expandMatrix(matrixPlan, base, baseDir)
	if err != nil {
		return nil, newMatrixValidationError(err)
	}
	if err := validateMatrixArtifactPathsDoNotOverwritePlans(matrixPlan, matrixPath, baseDir, cells); err != nil {
		return nil, newMatrixValidationError(err)
	}
	for i := range cells {
		cells[i].RawDir = resolveMatrixArtifactPath(baseDir, cells[i].RawDir)
		cells[i].FramedDir = resolveMatrixArtifactPath(baseDir, cells[i].FramedDir)
		for j := range cells[i].RawPaths {
			cells[i].RawPaths[j] = resolveMatrixArtifactPath(baseDir, cells[i].RawPaths[j])
			cells[i].FramedPaths[j] = resolveMatrixArtifactPath(baseDir, cells[i].FramedPaths[j])
		}
	}
	rawDir := resolveMatrixArtifactPath(baseDir, matrixPlan.Output.RawDir)
	if strings.TrimSpace(matrixPlan.Output.RawDir) == "" {
		rawDir = resolveMatrixArtifactPath(baseDir, defaultMatrixRawDir)
	}
	framedDir := resolveMatrixArtifactPath(baseDir, matrixPlan.Output.FramedDir)
	if strings.TrimSpace(matrixPlan.Output.FramedDir) == "" {
		framedDir = resolveMatrixArtifactPath(baseDir, defaultMatrixFramedDir)
	}
	reviewDir := resolveMatrixArtifactPath(baseDir, matrixPlan.Output.ReviewDir)
	if strings.TrimSpace(matrixPlan.Output.ReviewDir) == "" {
		reviewDir = resolveMatrixArtifactPath(baseDir, defaultMatrixReviewDir)
	}
	planPath := matrixPath
	if strings.TrimSpace(planPath) == "" {
		planPath = matrixPlan.sourcePath
	}
	if absolutePlanPath, pathErr := filepath.Abs(planPath); pathErr == nil {
		planPath = absolutePlanPath
	}
	result := &MatrixResult{
		PlanPath:   planPath,
		BundleID:   base.App.BundleID,
		RawDir:     rawDir,
		FramedDir:  framedDir,
		ReviewDir:  reviewDir,
		Total:      len(cells),
		TotalCells: len(cells),
		Cells:      make([]MatrixCellResult, len(cells)),
	}
	for i, cell := range cells {
		result.Cells[i] = newMatrixCellResult(cell)
	}

	deps := dependencies
	useDefaultDeviceCheck := deps.RunPlan == nil && deps.RunPlanRooted == nil && deps.Frame == nil && deps.FrameRooted == nil && deps.Appearance == nil && deps.CheckDevice == nil
	if deps.RunPlan == nil && deps.RunPlanRooted == nil {
		deps.RunPlanRooted = runPlanWithRoot
	}
	if deps.Frame == nil && deps.FrameRooted == nil {
		deps.FrameRooted = frameIntoRoot
	}
	if deps.Appearance == nil {
		deps.Appearance = &simctlMatrixAppearance{}
	}
	deviceFailures := make(map[string]matrixDeviceFailure)
	var preflightErr error
	if deps.CheckDevice == nil && useDefaultDeviceCheck {
		deviceFailures, preflightErr = checkMatrixDevices(ctx, matrixPlan)
	} else if deps.CheckDevice != nil {
		for _, device := range matrixPlan.Devices {
			if err := deps.CheckDevice(ctx, device); err != nil {
				if isMatrixContextTermination(err) {
					preflightErr = err
					break
				}
				deviceFailures[device.ID] = matrixDeviceFailure{
					Code:    matrixPreflightSimulatorNotReady,
					Message: "target simulator is not ready",
				}
			}
		}
	}
	if preflightErr != nil {
		markMatrixCellsCanceled(result)
	} else {
		for index, cell := range cells {
			failure, failed := deviceFailures[cell.Device]
			if !failed {
				continue
			}
			result.Cells[index].Status = MatrixCellFailed
			result.Cells[index].FailureStage = "preflight"
			if failure.Code == "" {
				failure.Code = matrixPreflightSimulatorNotReady
			}
			if failure.Message == "" {
				failure.Message = "target simulator is not ready"
			}
			result.Cells[index].FailureCode = failure.Code
			result.Cells[index].Error = newMatrixCellError("preflight", failure.Code, failure.Message)
			setMatrixScreenshotStatuses(&result.Cells[index])
		}
	}
	runErr := preflightErr
	var outputRoots matrixOutputRoots
	var reviewRoot rootfs.Root
	var reviewRootReady bool
	var outputRootErr error
	var releaseOutputLocks func() error
	var outputLockErr error
	rawRoot, rawRootErr := openMatrixOutputRoot(rawDir)
	if rawRootErr != nil {
		outputRootErr = fmt.Errorf("create raw output directory: %w", rawRootErr)
	} else {
		rawOpened, rootOpenErr := rawRoot.OpenRoot()
		if rootOpenErr != nil {
			_ = rawRoot.Close()
			outputRootErr = fmt.Errorf("open raw output directory: %w", rootOpenErr)
		} else {
			defer func() { _ = rawRoot.Close() }()
			defer func() { _ = rawOpened.Close() }()
			outputRoots = matrixOutputRoots{raw: rawRoot, rawPath: rawDir}
			var framedRoot rootfs.Root
			var framedOpened *os.Root
			if matrixPlan.Output.Frame.Enabled {
				framedRoot, rootOpenErr = openMatrixOutputRoot(framedDir)
				if rootOpenErr != nil {
					outputRootErr = fmt.Errorf("create framed output directory: %w", rootOpenErr)
				} else {
					framedOpened, rootOpenErr = framedRoot.OpenRoot()
					if rootOpenErr != nil {
						_ = framedRoot.Close()
						outputRootErr = fmt.Errorf("open framed output directory: %w", rootOpenErr)
					} else {
						defer func() { _ = framedRoot.Close() }()
						defer func() { _ = framedOpened.Close() }()
						outputRoots.framed = framedRoot
						outputRoots.framedPath = framedDir
						outputRoots.hasFramed = true
					}
				}
			}
			var reviewOpened *os.Root
			if outputRootErr == nil {
				reviewRoot, rootOpenErr = openMatrixOutputRoot(reviewDir)
				if rootOpenErr != nil {
					outputRootErr = fmt.Errorf("create matrix review output directory: %w", rootOpenErr)
				} else {
					reviewOpened, rootOpenErr = reviewRoot.OpenRoot()
					if rootOpenErr != nil {
						_ = reviewRoot.Close()
						outputRootErr = fmt.Errorf("open matrix review output directory: %w", rootOpenErr)
					} else {
						reviewRootReady = true
						defer func() { _ = reviewRoot.Close() }()
						defer func() { _ = reviewOpened.Close() }()
					}
				}
			}
			if outputRootErr == nil {
				targets := []matrixOutputLockTarget{{root: rawRoot, opened: rawOpened}, {root: reviewRoot, opened: reviewOpened}}
				if matrixPlan.Output.Frame.Enabled {
					targets = append(targets, matrixOutputLockTarget{root: outputRoots.framed, opened: framedOpened})
				}
				releaseOutputLocks, outputLockErr = acquireMatrixOutputLocksForRoots(ctx, targets)
				if outputLockErr != nil {
					releaseOutputLocks = nil
				} else if matrixOutputLocksAcquiredForTest != nil {
					matrixOutputLocksAcquiredForTest()
				}
			}
		}
	}
	if outputLockErr != nil {
		outputRootErr = matrixLockError("output", outputLockErr)
	}
	if outputRootErr != nil {
		if isMatrixContextTermination(outputRootErr) {
			markMatrixCellsCanceled(result)
		} else {
			markMatrixOutputFailure(result)
		}
		if runErr == nil {
			runErr = outputRootErr
		} else {
			runErr = errors.Join(runErr, outputRootErr)
		}
	}
	if runErr == nil {
		runErr = executeMatrixCells(ctx, cells, deviceFailures, base, matrixPlan, concurrency, attempts, backoff, deps, outputRoots, result)
	}
	rawVerificationCtx := ctx
	if ctx.Err() != nil {
		rawVerificationCtx = context.WithoutCancel(ctx)
	}
	var retainedRawRoot, retainedFramedRoot *rootfs.Root
	if outputRoots.raw.Path() != "" {
		retainedRawRoot = &outputRoots.raw
	}
	if outputRoots.hasFramed && outputRoots.framed.Path() != "" {
		retainedFramedRoot = &outputRoots.framed
	}
	if rawErr := revalidateMatrixRawPathsWithRoot(rawVerificationCtx, result, matrixSubprocessTimeout, retainedRawRoot); rawErr != nil {
		if runErr == nil {
			runErr = rawErr
		} else {
			runErr = errors.Join(runErr, rawErr)
		}
	}
	framedVerificationCtx := ctx
	if ctx.Err() != nil {
		// A run can finish its last cell successfully and be canceled during
		// appearance restoration immediately afterward. Preserve those already
		// completed framed artifacts by verifying them with bounded per-artifact
		// contexts detached from the canceled caller. Cancellation that arrives
		// after verification starts still propagates through the live ctx above.
		framedVerificationCtx = context.WithoutCancel(ctx)
	}
	if framedErr := revalidateMatrixFramedPathsWithRoot(framedVerificationCtx, result, matrixSubprocessTimeout, retainedFramedRoot); framedErr != nil {
		if runErr == nil {
			runErr = framedErr
		} else {
			runErr = errors.Join(runErr, framedErr)
		}
	}
	countMatrixResultStatuses(result)
	if matrixBeforeReviewPublishForTest != nil {
		matrixBeforeReviewPublishForTest()
	}
	reviewCtx := context.WithoutCancel(ctx)
	if outputLockErr == nil {
		review, reviewErr := publishMatrixReview(reviewCtx, result, reviewDir, reviewRoot, reviewRootReady, ctx)
		if reviewErr == nil {
			result.Review = review
		} else if runErr == nil {
			result.Status = MatrixCellFailed
			runErr = fmt.Errorf("write matrix review: %w", reviewErr)
		} else {
			result.Status = MatrixCellFailed
			runErr = errors.Join(runErr, fmt.Errorf("write matrix review: %w", reviewErr))
		}
	}
	if releaseOutputLocks != nil {
		releaseErr := releaseOutputLocks()
		if matrixOutputLockReleaseErrForTest != nil {
			releaseErr = errors.Join(releaseErr, matrixOutputLockReleaseErrForTest)
		}
		if releaseErr != nil {
			lockErr := matrixLockError("output release", releaseErr)
			if result.Status == MatrixCellSuccess {
				result.Status = MatrixCellFailed
			}
			if runErr == nil {
				runErr = lockErr
			} else {
				runErr = errors.Join(runErr, lockErr)
			}
			if republished, republishErr := publishMatrixReview(reviewCtx, result, reviewDir, reviewRoot, reviewRootReady, ctx); republishErr == nil {
				result.Review = republished
			} else {
				runErr = errors.Join(runErr, fmt.Errorf("write matrix review: %w", republishErr))
			}
		}
	}
	return result, runErr
}

func publishMatrixReview(ctx context.Context, result *MatrixResult, reviewDir string, reviewRoot rootfs.Root, reviewRootReady bool, lockCtx context.Context) (*MatrixReviewResult, error) {
	reviewRequest := MatrixReviewRequest{Result: result, OutputDir: reviewDir, LockContext: lockCtx}
	if reviewRootReady {
		if err := verifyMatrixRetainedOutputRoot(reviewRoot); err != nil {
			return nil, err
		}
		return generateMatrixReviewWithRoot(ctx, reviewRequest, reviewRoot)
	}
	return GenerateMatrixReview(ctx, reviewRequest)
}

func resolveMatrixExecution(execution MatrixExecution, options MatrixOptions) (int, int, time.Duration, error) {
	concurrency := execution.MaxConcurrency
	if options.MaxConcurrencySet || options.MaxConcurrency != 0 {
		concurrency = options.MaxConcurrency
	}
	if concurrency == 0 && !options.MaxConcurrencySet {
		concurrency = defaultMatrixConcurrency
	}
	if concurrency < 1 || concurrency > maxMatrixConcurrency {
		return 0, 0, 0, fmt.Errorf("max concurrency must be between 1 and %d", maxMatrixConcurrency)
	}
	attempts := execution.MaxAttempts
	if options.MaxAttemptsSet || options.MaxAttempts != 0 {
		attempts = options.MaxAttempts
	}
	if attempts == 0 && !options.MaxAttemptsSet {
		attempts = defaultMatrixAttempts
	}
	if attempts < 1 || attempts > maxMatrixAttempts {
		return 0, 0, 0, fmt.Errorf("max attempts must be between 1 and %d", maxMatrixAttempts)
	}
	if int64(execution.RetryBackoffMS) > maxMatrixRetryBackoffMS {
		return 0, 0, 0, errors.New("retry backoff milliseconds exceeds maximum duration")
	}
	backoff := time.Duration(execution.RetryBackoffMS) * time.Millisecond
	if strings.TrimSpace(execution.RetryBackoff) != "" {
		parsed, err := time.ParseDuration(strings.TrimSpace(execution.RetryBackoff))
		if err != nil {
			return 0, 0, 0, errors.New("retry backoff must be a valid duration")
		}
		backoff = parsed
	}
	if options.RetryBackoffSet || options.RetryBackoff != 0 {
		backoff = options.RetryBackoff
	}
	if backoff < 0 {
		return 0, 0, 0, errors.New("retry backoff must be >= 0")
	}
	return concurrency, attempts, backoff, nil
}

func executeMatrixCells(ctx context.Context, cells []MatrixCell, deviceFailures map[string]matrixDeviceFailure, base *Plan, matrixPlan *MatrixPlan, concurrency, attempts int, backoff time.Duration, deps MatrixDependencies, outputRoots matrixOutputRoots, result *MatrixResult) error {
	jobs := make(chan int)
	var workers sync.WaitGroup
	guards := make(map[string]*matrixSimulatorGuard)
	for _, cell := range cells {
		guardKey := normalizeMatrixUDID(cell.UDID)
		if _, ok := guards[guardKey]; !ok {
			guards[guardKey] = &matrixSimulatorGuard{}
		}
	}
	workerCount := concurrency
	if workerCount > len(cells) {
		workerCount = len(cells)
	}
	if workerCount == 0 {
		return nil
	}
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				if _, failed := deviceFailures[cells[index].Device]; failed {
					continue
				}
				cellResult := executeMatrixCell(ctx, cells[index], base, matrixPlan, attempts, backoff, deps, outputRoots, guards[normalizeMatrixUDID(cells[index].UDID)])
				result.Cells[index] = cellResult
			}
		}()
	}
	for index := range cells {
		select {
		case jobs <- index:
		case <-ctx.Done():
			break
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	workers.Wait()
	if ctx.Err() != nil {
		canceledCells := false
		for i := range result.Cells {
			if result.Cells[i].Status != MatrixCellCanceled {
				continue
			}
			canceledCells = true
			if result.Cells[i].Attempts == 0 {
				result.Cells[i].FailureStage = "execution"
				result.Cells[i].FailureCode = "canceled"
				result.Cells[i].Error = newMatrixCellError("execution", "canceled", "cell canceled")
			}
		}
		if canceledCells {
			return ctx.Err()
		}
	}
	for _, cell := range result.Cells {
		if cell.Status == MatrixCellFailed || cell.Status == MatrixCellCleanupFailed {
			return errors.New("one or more matrix cells failed")
		}
	}
	return nil
}

func isMatrixContextTermination(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
