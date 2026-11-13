package screenshots

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"sync"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// matrixRawVerificationContextForTest observes each fresh per-artifact raw
// verification context without adding timing dependencies to tests.
var matrixRawVerificationContextForTest func(context.Context)

// matrixFramedVerificationBeforeArtifactForTest is the framed counterpart of
// the raw verification seam. It is intentionally nil in production.
var matrixFramedVerificationBeforeArtifactForTest func()

// matrixFramedVerificationContextForTest observes each fresh per-artifact
// verification context without adding timing dependencies to tests.
var matrixFramedVerificationContextForTest func(context.Context)

// matrixArtifactBeforeOpenForTest observes the last point before a
// revalidation read. It is intentionally nil in production and lets tests
// prove a replaced output root is rejected before any artifact is opened.
var matrixArtifactBeforeOpenForTest func(rootfs.Root, string)

func promoteMatrixArtifactFromRoots(ctx context.Context, sourceRoot rootfs.Root, sourceRootPath, source string, destinationRoot rootfs.Root, destinationRootPath, destination string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	sourceRelative, err := relativeMatrixOutputPath(sourceRootPath, source)
	if err != nil {
		return err
	}
	sourceFile, err := sourceRoot.OpenFile(sourceRelative)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	destinationRelative, err := relativeMatrixOutputPath(destinationRootPath, destination)
	if err != nil {
		return err
	}
	written, err := destinationRoot.WriteFromPreservingMode(destinationRelative, &matrixContextReader{ctx: ctx, reader: io.LimitReader(sourceFile, maxMatrixArtifactBytes+1)}, 0o644)
	if err != nil {
		return err
	}
	if written > maxMatrixArtifactBytes {
		return errors.New("screenshot exceeds the artifact size limit")
	}
	return nil
}

type matrixArtifactInfo struct {
	identity os.FileInfo
	size     int64
	digest   [sha256.Size]byte
}

func promoteMatrixArtifactWithInfoFromRoots(ctx context.Context, sourceRoot rootfs.Root, sourceRootPath, source string, destinationRoot rootfs.Root, destinationRootPath, destination string) (matrixArtifactInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return matrixArtifactInfo{}, err
	}
	if err := promoteMatrixArtifactFromRoots(ctx, sourceRoot, sourceRootPath, source, destinationRoot, destinationRootPath, destination); err != nil {
		return matrixArtifactInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return matrixArtifactInfo{}, err
	}
	return inspectMatrixArtifactWithContext(ctx, destinationRoot, destinationRootPath, destination)
}

func inspectMatrixArtifactWithContext(ctx context.Context, outputRoot rootfs.Root, outputRootPath, path string) (matrixArtifactInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return matrixArtifactInfo{}, err
	}
	relative, err := relativeMatrixOutputPath(outputRootPath, path)
	if err != nil {
		return matrixArtifactInfo{}, err
	}
	if matrixArtifactBeforeOpenForTest != nil {
		matrixArtifactBeforeOpenForTest(outputRoot, path)
	}
	file, err := outputRoot.OpenFile(relative)
	if err != nil {
		return matrixArtifactInfo{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return matrixArtifactInfo{}, err
	}
	if info.Size() > maxMatrixArtifactBytes {
		return matrixArtifactInfo{}, errors.New("matrix artifact exceeds the size limit")
	}
	hasher := sha256.New()
	size, err := io.Copy(hasher, io.LimitReader(&matrixContextReader{ctx: ctx, reader: file}, maxMatrixArtifactBytes+1))
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return matrixArtifactInfo{}, contextErr
		}
		return matrixArtifactInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return matrixArtifactInfo{}, err
	}
	if size > maxMatrixArtifactBytes {
		return matrixArtifactInfo{}, errors.New("matrix artifact exceeds the size limit")
	}
	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	return matrixArtifactInfo{identity: info, size: size, digest: digest}, nil
}

type matrixContextReader struct {
	ctx    context.Context
	reader io.Reader
}

// matrixArtifactBudgetContext combines a per-artifact timer with the caller's
// context while forwarding Err directly to the caller. The latter matters for
// context implementations whose Done channel is nil but whose Err can still
// report cancellation (and keeps cancellation observable during hashing).
type matrixArtifactBudgetContext struct {
	parent context.Context
	timer  context.Context
	done   chan struct{}

	closeOnce  sync.Once
	stopParent func() bool
	stopTimer  func() bool
}

func newMatrixArtifactBudgetContext(parent context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	timer, cancelTimer := context.WithTimeout(context.Background(), budget)
	combined := &matrixArtifactBudgetContext{parent: parent, timer: timer, done: make(chan struct{})}
	combined.stopTimer = context.AfterFunc(timer, combined.closeDone)
	if parent.Done() != nil {
		combined.stopParent = context.AfterFunc(parent, combined.closeDone)
	}
	return combined, func() {
		cancelTimer()
		if combined.stopParent != nil {
			combined.stopParent()
		}
		if combined.stopTimer != nil {
			combined.stopTimer()
		}
		combined.closeDone()
	}
}

func (c *matrixArtifactBudgetContext) closeDone() {
	c.closeOnce.Do(func() { close(c.done) })
}

func (c *matrixArtifactBudgetContext) Deadline() (time.Time, bool) {
	parentDeadline, parentOK := c.parent.Deadline()
	timerDeadline, timerOK := c.timer.Deadline()
	switch {
	case !parentOK:
		return timerDeadline, timerOK
	case !timerOK:
		return parentDeadline, true
	case parentDeadline.Before(timerDeadline):
		return parentDeadline, true
	default:
		return timerDeadline, true
	}
}

func (c *matrixArtifactBudgetContext) Done() <-chan struct{} { return c.done }

func (c *matrixArtifactBudgetContext) Err() error {
	if err := c.parent.Err(); err != nil {
		return err
	}
	return c.timer.Err()
}

func (c *matrixArtifactBudgetContext) Value(key any) any { return c.parent.Value(key) }

func (r *matrixContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if contextErr := r.ctx.Err(); contextErr != nil {
		return n, contextErr
	}
	return n, err
}

func mergeMatrixRawArtifacts(result *MatrixCellResult, attempt matrixAttemptResult) {
	if len(attempt.RawArtifacts) == 0 {
		return
	}
	if result.rawArtifacts == nil {
		result.rawArtifacts = make(map[string]matrixArtifactInfo, len(attempt.RawArtifacts))
	}
	for path, artifact := range attempt.RawArtifacts {
		result.rawArtifacts[path] = artifact
	}
}

func revalidateMatrixRawPathsWithRoot(ctx context.Context, result *MatrixResult, budget time.Duration, retainedRoot *rootfs.Root) error {
	if result == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	hasRawPaths := false
	for _, cell := range result.Cells {
		if len(cell.RawPaths) > 0 {
			hasRawPaths = true
			break
		}
	}
	if !hasRawPaths {
		return nil
	}
	rawRootPath := result.RawDir
	var rawRoot rootfs.Root
	var rootErr error
	if retainedRoot != nil {
		rawRoot = *retainedRoot
		rawRootPath = rawRoot.Path()
		rootErr = verifyMatrixRetainedOutputRoot(rawRoot)
	} else {
		rawRoot, rootErr = rootfs.New(result.RawDir)
		if rootErr == nil {
			defer rawRoot.Close()
		}
	}
	invalid := false
	var contextErr error
	for cellIndex := range result.Cells {
		cell := &result.Cells[cellIndex]
		validPaths := make([]string, 0, len(cell.RawPaths))
		validSet := make(map[string]struct{}, len(cell.RawPaths))
		for _, path := range cell.RawPaths {
			if contextErr == nil {
				contextErr = ctx.Err()
			}
			if contextErr != nil {
				invalid = true
				break
			}
			expected, known := cell.rawArtifacts[path]
			if rootErr != nil || !known {
				invalid = true
				continue
			}
			artifactCtx := ctx
			var cancelArtifact context.CancelFunc
			if budget > 0 {
				artifactCtx, cancelArtifact = newMatrixArtifactBudgetContext(ctx, budget)
			}
			if matrixRawVerificationContextForTest != nil {
				matrixRawVerificationContextForTest(artifactCtx)
			}
			current, err := inspectMatrixArtifactWithContext(artifactCtx, rawRoot, rawRootPath, path)
			if cancelArtifact != nil {
				cancelArtifact()
			}
			if contextErr == nil {
				contextErr = ctx.Err()
			}
			if err != nil || contextErr != nil || !matrixArtifactMatches(expected, current) {
				invalid = true
				if contextErr != nil {
					break
				}
				continue
			}
			validPaths = append(validPaths, path)
			validSet[path] = struct{}{}
		}
		if len(validPaths) != len(cell.RawPaths) {
			cell.RawPaths = validPaths
			if contextErr != nil && cell.Status == MatrixCellSuccess {
				cell.Status = MatrixCellCanceled
				cell.FailureStage = "execution"
				cell.FailureCode = "canceled"
				cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "cell canceled")
			} else if cell.Status == MatrixCellSuccess {
				cell.Status = MatrixCellFailed
				cell.FailureStage = "execution"
				cell.FailureCode = "raw_output_unavailable"
				cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "raw screenshot became unavailable")
			}
			for screenshotIndex := range cell.Screenshots {
				path := cell.Screenshots[screenshotIndex].RawPath
				if path == "" {
					continue
				}
				if _, ok := validSet[path]; !ok {
					cell.Screenshots[screenshotIndex].RawPath = ""
				}
			}
			setMatrixScreenshotStatuses(cell)
		}
	}
	if retainedRoot != nil && rootErr == nil {
		if err := verifyMatrixRetainedOutputRoot(rawRoot); err != nil {
			invalid = true
			for cellIndex := range result.Cells {
				cell := &result.Cells[cellIndex]
				if len(cell.RawPaths) == 0 {
					continue
				}
				cell.RawPaths = nil
				for screenshotIndex := range cell.Screenshots {
					cell.Screenshots[screenshotIndex].RawPath = ""
				}
				if cell.Status == MatrixCellSuccess {
					cell.Status = MatrixCellFailed
					cell.FailureStage = "execution"
					cell.FailureCode = "raw_output_unavailable"
					cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "raw screenshot became unavailable")
				}
				setMatrixScreenshotStatuses(cell)
			}
		}
	}
	if contextErr != nil {
		return contextErr
	}
	if invalid {
		return errors.New("one or more raw screenshots became unavailable")
	}
	return nil
}

func mergeMatrixFramedArtifacts(result *MatrixCellResult, attempt matrixAttemptResult) {
	if len(attempt.FramedArtifacts) == 0 {
		return
	}
	if result.framedArtifacts == nil {
		result.framedArtifacts = make(map[string]matrixArtifactInfo, len(attempt.FramedArtifacts))
	}
	for path, artifact := range attempt.FramedArtifacts {
		result.framedArtifacts[path] = artifact
	}
}

func revalidateMatrixFramedPathsWithRoot(ctx context.Context, result *MatrixResult, budget time.Duration, retainedRoot *rootfs.Root) error {
	if result == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	hasFramedPaths := false
	for _, cell := range result.Cells {
		if len(cell.FramedPaths) > 0 {
			hasFramedPaths = true
			break
		}
	}
	if !hasFramedPaths {
		return nil
	}
	framedRootPath := result.FramedDir
	var framedRoot rootfs.Root
	var rootErr error
	if retainedRoot != nil {
		framedRoot = *retainedRoot
		framedRootPath = framedRoot.Path()
		rootErr = verifyMatrixRetainedOutputRoot(framedRoot)
	} else {
		framedRoot, rootErr = rootfs.New(result.FramedDir)
		if rootErr == nil {
			defer framedRoot.Close()
		}
	}
	invalid := false
	var contextErr error
	for cellIndex := range result.Cells {
		cell := &result.Cells[cellIndex]
		validPaths := make([]string, 0, len(cell.FramedPaths))
		validSet := make(map[string]struct{}, len(cell.FramedPaths))
		for _, path := range cell.FramedPaths {
			if contextErr == nil {
				contextErr = ctx.Err()
			}
			if contextErr != nil {
				invalid = true
				break
			}
			expected, known := cell.framedArtifacts[path]
			if rootErr != nil || !known {
				invalid = true
				continue
			}
			artifactCtx := ctx
			var cancelArtifact context.CancelFunc
			if budget > 0 {
				artifactCtx, cancelArtifact = newMatrixArtifactBudgetContext(ctx, budget)
			}
			if matrixFramedVerificationContextForTest != nil {
				matrixFramedVerificationContextForTest(artifactCtx)
			}
			if matrixFramedVerificationBeforeArtifactForTest != nil {
				matrixFramedVerificationBeforeArtifactForTest()
			}
			current, err := inspectMatrixArtifactWithContext(artifactCtx, framedRoot, framedRootPath, path)
			if cancelArtifact != nil {
				cancelArtifact()
			}
			if contextErr == nil {
				contextErr = ctx.Err()
			}
			if err != nil || contextErr != nil || !matrixArtifactMatches(expected, current) {
				invalid = true
				if contextErr != nil {
					break
				}
				continue
			}
			validPaths = append(validPaths, path)
			validSet[path] = struct{}{}
		}
		if len(validPaths) != len(cell.FramedPaths) {
			cell.FramedPaths = validPaths
			if contextErr != nil && cell.Status == MatrixCellSuccess {
				cell.Status = MatrixCellCanceled
				cell.FailureStage = "execution"
				cell.FailureCode = "canceled"
				cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "cell canceled")
			} else if cell.Status == MatrixCellSuccess {
				cell.Status = MatrixCellFailed
				cell.FailureStage = "framing"
				cell.FailureCode = "framed_output_unavailable"
				cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "framed screenshot became unavailable")
			}
		}
		for screenshotIndex := range cell.Screenshots {
			path := cell.Screenshots[screenshotIndex].FramedPath
			if path == "" {
				continue
			}
			if _, ok := validSet[path]; !ok {
				cell.Screenshots[screenshotIndex].FramedPath = ""
			}
		}
		setMatrixScreenshotStatuses(cell)
		for screenshotIndex := range cell.Screenshots {
			if cell.Screenshots[screenshotIndex].FramedPath != "" {
				continue
			}
			// A requested framed artifact that was just invalidated is not a
			// successful screenshot merely because its raw counterpart remains
			// available. Keep the public per-screenshot projection aligned with
			// the cell's framing/cancellation failure.
			if contextErr != nil && cell.FailureStage == "execution" && cell.FailureCode == "canceled" {
				cell.Screenshots[screenshotIndex].Status = MatrixCellCanceled
			} else if cell.FailureStage == "framing" {
				cell.Screenshots[screenshotIndex].Status = MatrixCellFailed
			}
		}
	}
	if retainedRoot != nil && rootErr == nil {
		if err := verifyMatrixRetainedOutputRoot(framedRoot); err != nil {
			invalid = true
			for cellIndex := range result.Cells {
				cell := &result.Cells[cellIndex]
				if len(cell.FramedPaths) == 0 {
					continue
				}
				cell.FramedPaths = nil
				for screenshotIndex := range cell.Screenshots {
					cell.Screenshots[screenshotIndex].FramedPath = ""
				}
				if cell.Status == MatrixCellSuccess {
					cell.Status = MatrixCellFailed
					cell.FailureStage = "framing"
					cell.FailureCode = "framed_output_unavailable"
					cell.Error = newMatrixCellError(cell.FailureStage, cell.FailureCode, "framed screenshot became unavailable")
				}
				setMatrixScreenshotStatuses(cell)
				for screenshotIndex := range cell.Screenshots {
					if cell.FailureStage == "framing" {
						cell.Screenshots[screenshotIndex].Status = MatrixCellFailed
					}
				}
			}
		}
	}
	if contextErr != nil {
		return contextErr
	}
	if invalid {
		return errors.New("one or more framed screenshots became unavailable")
	}
	return nil
}

func verifyMatrixRetainedOutputRoot(root rootfs.Root) error {
	if root.Path() == "" {
		return errors.New("matrix output root is unavailable")
	}
	ok, err := root.ContainsPath(root.Path())
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("matrix output root changed during execution")
	}
	return nil
}

func matrixArtifactMatches(expected, current matrixArtifactInfo) bool {
	if expected.identity == nil || current.identity == nil {
		return false
	}
	return os.SameFile(expected.identity, current.identity) && expected.size == current.size && expected.digest == current.digest
}

func readMatrixImageDimensions(file *os.File, path string) (asc.ImageDimensions, error) {
	if file == nil {
		return asc.ImageDimensions{}, errors.New("image file is required")
	}
	info, err := file.Stat()
	if err != nil {
		return asc.ImageDimensions{}, err
	}
	if !info.Mode().IsRegular() {
		return asc.ImageDimensions{}, fmt.Errorf("expected regular image file %q", path)
	}
	if info.Size() <= 0 {
		return asc.ImageDimensions{}, fmt.Errorf("image file %q is empty", path)
	}
	if info.Size() > maxMatrixArtifactBytes {
		return asc.ImageDimensions{}, fmt.Errorf("image file %q exceeds the size limit", path)
	}
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return asc.ImageDimensions{}, fmt.Errorf("decode image dimensions: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return asc.ImageDimensions{}, fmt.Errorf("image %q has invalid dimensions", path)
	}
	return asc.ImageDimensions{Width: config.Width, Height: config.Height}, nil
}
