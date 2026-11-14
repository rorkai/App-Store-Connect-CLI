package screenshots

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

// MatrixReviewRequest describes the local report to write after a matrix run.
type MatrixReviewRequest struct {
	Result      *MatrixResult
	OutputDir   string
	LockContext context.Context
}

// MatrixReviewManifest and MatrixReviewResult are aliases for the governed
// output contracts. The screenshots package keeps execution details private
// while the asc package owns public JSON field naming and renderers.
type (
	MatrixReviewManifest = asc.MatrixReviewManifest
	MatrixReviewResult   = asc.MatrixReviewResult
)

// matrixReviewManifestLoadedForTest is a narrow race-test seam. It runs after
// the manifest has been decoded but before its HTML pair is validated.
var matrixReviewManifestLoadedForTest func()

// These narrow seams make filesystem replacement and cancellation races
// deterministic in package tests. They are unset in production.
var (
	matrixReviewAssetBeforeHashForTest    func(path string)
	matrixReviewAssetRootBeforePinForTest func(kind, path string)
	matrixReviewAssetRootsCapturedForTest func(kind, path string)
	matrixReviewAssetValidatedForTest     func(path string)
)

// GenerateMatrixReview writes an offline HTML report and its JSON manifest.
// It includes every planned cell, including failed and canceled cells.
func GenerateMatrixReview(ctx context.Context, request MatrixReviewRequest) (*MatrixReviewResult, error) {
	return generateMatrixReviewWithWriter(ctx, request, func(root rootfs.Root, name string, data []byte, perm os.FileMode) error {
		return root.WriteFilePreservingMode(name, data, perm)
	})
}

func generateMatrixReviewWithRoot(ctx context.Context, request MatrixReviewRequest, reviewRoot rootfs.Root) (*MatrixReviewResult, error) {
	return generateMatrixReviewWithWriterAndRoot(ctx, request, func(root rootfs.Root, name string, data []byte, perm os.FileMode) error {
		return root.WriteFilePreservingMode(name, data, perm)
	}, &reviewRoot)
}

// matrixReviewGeneratedFiles lists every file generateMatrixReviewWithWriter
// publishes into the review directory. Plan validation refuses inputs that would
// be overwritten by these names, so this must stay in step with the writer below;
// TestGenerateMatrixReviewWritesOnlyTheDeclaredFiles pins that.
var errMatrixReviewPairMismatch = errors.New("matrix review HTML does not match manifest")

// matrixReviewLockReleaseErrForTest injects a release failure after the report
// pair has been committed, so tests can prove a cleanup-only unlock error does
// not rewrite a successful generation into a failed return.
var matrixReviewLockReleaseErrForTest error

var matrixReviewGeneratedFiles = []string{".asc-matrix-review.lock", "index.html", "manifest.json"}

const (
	matrixReviewLockAfterCancelTimeout = 250 * time.Millisecond
	// Browser snapshots are deliberately bounded independently from the
	// generated report limit. A matrix can reference many small images, so a
	// per-file limit alone is not enough to keep the opener's memory/disk work
	// predictable. 4096 covers the 256-cell maximum with eight screenshots
	// per cell and both raw and framed references.
	maxMatrixReviewBrowserAssets = 4096
	maxMatrixReviewBrowserBytes  = 256 << 20
)

func openMatrixReviewFile(root *os.Root, name string) (*os.File, error) {
	if root == nil {
		return nil, errors.New("matrix review root is unavailable")
	}
	file, err := secureopen.OpenExistingNoFollowInRoot(root, name)
	if err == nil {
		return file, nil
	}
	// Preserve the rootfs error contract for a final symlink while keeping the
	// actual read anchored to the already-open directory descriptor.
	if info, statErr := root.Lstat(name); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: refusing to follow symlink %q", rootfs.ErrSymlink, name)
	}
	return nil, err
}

func matrixReviewLockContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() == nil {
		return ctx, func() {}
	}
	// Report generation is intentionally detached from a canceled matrix run,
	// but lock acquisition must not wait forever behind another process.
	return context.WithTimeout(context.Background(), matrixReviewLockAfterCancelTimeout)
}

type matrixReviewWriter func(rootfs.Root, string, []byte, os.FileMode) error

func generateMatrixReviewWithWriter(ctx context.Context, request MatrixReviewRequest, write matrixReviewWriter) (result *MatrixReviewResult, retErr error) {
	return generateMatrixReviewWithWriterAndRoot(ctx, request, write, nil)
}

func generateMatrixReviewWithWriterAndRoot(ctx context.Context, request MatrixReviewRequest, write matrixReviewWriter, selectedRoot *rootfs.Root) (result *MatrixReviewResult, retErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.Result == nil {
		return nil, errors.New("matrix result is required")
	}
	outputDir := request.OutputDir
	if strings.TrimSpace(outputDir) == "" {
		return nil, errors.New("matrix review output directory is required")
	}
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve matrix review output directory: %w", err)
	}
	var reviewRoot rootfs.Root
	if selectedRoot != nil {
		reviewRoot = *selectedRoot
	} else {
		var err error
		reviewRoot, err = openMatrixOutputRoot(absOutputDir)
		if err != nil {
			return nil, fmt.Errorf("create matrix review output directory: %w", err)
		}
		defer func() { _ = reviewRoot.Close() }()
	}
	lockContextSource := request.LockContext
	if lockContextSource == nil {
		lockContextSource = ctx
	}
	lockCtx, cancelLock := matrixReviewLockContext(lockContextSource)
	defer cancelLock()
	releaseReviewLock, err := acquireMatrixReviewLock(lockCtx, reviewRoot)
	if err != nil {
		return nil, fmt.Errorf("lock matrix review output directory: %w", err)
	}
	defer func() {
		releaseErr := releaseReviewLock()
		if matrixReviewLockReleaseErrForTest != nil {
			releaseErr = errors.Join(releaseErr, matrixReviewLockReleaseErrForTest)
		}
		if releaseErr != nil && retErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("release matrix review output lock: %w", releaseErr))
		}
	}()
	if write == nil {
		return nil, errors.New("matrix review writer is required")
	}
	if err := reviewRoot.CheckWriteFilePreservingMode("index.html"); err != nil {
		return nil, fmt.Errorf("prepare matrix review HTML: %w", err)
	}
	if err := reviewRoot.CheckWriteFilePreservingMode("manifest.json"); err != nil {
		return nil, fmt.Errorf("prepare matrix review manifest: %w", err)
	}
	previousHTML, hadHTML, err := readMatrixReviewFile(reviewRoot, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read previous matrix review HTML: %w", err)
	}
	previousManifest, hadManifest, err := readMatrixReviewFile(reviewRoot, "manifest.json")
	if err != nil {
		return nil, fmt.Errorf("read previous matrix review manifest: %w", err)
	}

	total, succeeded, failed, canceled, cleanupFailed := matrixReviewCounts(request.Result)
	status := request.Result.Status
	if status == "" {
		status = MatrixCellSuccess
		if failed > 0 || canceled > 0 {
			status = MatrixCellFailed
		}
	}
	manifest := MatrixReviewManifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		PlanPath:    request.Result.PlanPath,
		BundleID:    request.Result.BundleID,
		RawDir:      relativeOrCleanPath(absOutputDir, request.Result.RawDir),
		FramedDir:   relativeOrCleanPath(absOutputDir, request.Result.FramedDir),
		OutputDir:   absOutputDir,
		Status:      status,
		TotalCells:  total,
		Succeeded:   succeeded,
		Failed:      failed,
		Canceled:    canceled,
		Retried:     request.Result.Retried,

		CleanupFailed: cleanupFailed,
		Cells:         make([]asc.MatrixCellResult, len(request.Result.Cells)),
	}
	for i, cell := range request.Result.Cells {
		manifest.Cells[i] = matrixReviewCellOutput(cell)
		// Error values are produced by the matrix executor from a fixed set of
		// messages. Keep this defensive check in case a future caller supplies a
		// result directly to the report writer.
		if cell.Status != MatrixCellSuccess {
			manifest.Cells[i].Error = matrixReviewErrorOutput(sanitizeMatrixReviewError(cell.Error))
		}
	}
	htmlContent := renderMatrixReviewHTML(manifest)
	htmlDigest := sha256.Sum256([]byte(htmlContent))
	manifest.HTMLSHA256 = fmt.Sprintf("%x", htmlDigest[:])
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal matrix review manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := validateMatrixReviewSize("HTML", []byte(htmlContent)); err != nil {
		return nil, err
	}
	if err := validateMatrixReviewSize("manifest", manifestData); err != nil {
		return nil, err
	}
	// Publish HTML before the manifest. The manifest is the report's commit
	// marker. Both writes are rooted and atomic. If the manifest publication
	// fails, restore both files so the old marker and HTML remain a pair.
	if err := write(reviewRoot, "index.html", []byte(htmlContent), 0o644); err != nil {
		rollbackErr := restoreMatrixReviewFile(reviewRoot, "index.html", previousHTML, hadHTML)
		return nil, joinMatrixReviewWriteErrors(fmt.Errorf("write matrix review HTML: %w", err), rollbackErr)
	}
	if err := write(reviewRoot, "manifest.json", manifestData, 0o644); err != nil {
		manifestRollbackErr := restoreMatrixReviewFile(reviewRoot, "manifest.json", previousManifest, hadManifest)
		htmlRollbackErr := restoreMatrixReviewFile(reviewRoot, "index.html", previousHTML, hadHTML)
		return nil, joinMatrixReviewWriteErrors(
			fmt.Errorf("write matrix review manifest: %w", err),
			errors.Join(manifestRollbackErr, htmlRollbackErr),
		)
	}
	manifestPath := filepath.Join(absOutputDir, "manifest.json")
	htmlPath := filepath.Join(absOutputDir, "index.html")
	return &MatrixReviewResult{
		ManifestPath: manifestPath,
		HTMLPath:     htmlPath,
		Total:        manifest.TotalCells,
		Succeeded:    manifest.Succeeded,
		Failed:       manifest.Failed,
		Canceled:     manifest.Canceled,
	}, nil
}

func validateMatrixReviewSize(kind string, data []byte) error {
	if len(data) > maxMatrixReviewBytes {
		return fmt.Errorf("matrix review %s exceeds the %d-byte size limit", kind, maxMatrixReviewBytes)
	}
	return nil
}

func readMatrixReviewFile(root rootfs.Root, name string) ([]byte, bool, error) {
	data, err := root.ReadFileLimited(name, maxMatrixReviewBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func restoreMatrixReviewFile(root rootfs.Root, name string, data []byte, existed bool) error {
	if existed {
		return root.WriteFilePreservingMode(name, data, 0o644)
	}
	rooted, err := root.OpenRoot()
	if err != nil {
		return err
	}
	defer rooted.Close()
	if err := rooted.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func joinMatrixReviewWriteErrors(primary, rollback error) error {
	if rollback == nil {
		return primary
	}
	return errors.Join(primary, fmt.Errorf("rollback matrix review: %w", rollback))
}

// matrixReviewCounts mirrors countMatrixResultStatuses so the persisted review
// agrees with the command result: a cleanup failure counts in both failed and
// cleanupFailed, the latter being a labelled subset rather than a sibling.
func matrixReviewCounts(result *MatrixResult) (total, succeeded, failed, canceled, cleanupFailed int) {
	total = len(result.Cells)
	for _, cell := range result.Cells {
		switch cell.Status {
		case MatrixCellSuccess:
			succeeded++
		case MatrixCellCanceled:
			canceled++
		case MatrixCellCleanupFailed:
			cleanupFailed++
			failed++
		default:
			failed++
		}
	}
	if result.TotalCells > total {
		total = result.TotalCells
	}
	if result.Total > total {
		total = result.Total
	}
	return total, succeeded, failed, canceled, cleanupFailed
}

// LoadMatrixReviewManifest parses a generated matrix review manifest.
func LoadMatrixReviewManifest(path string) (*MatrixReviewManifest, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("read matrix review manifest: %w", err)
	}
	reviewRoot, err := rootfs.New(filepath.Dir(absPath))
	if err != nil {
		return nil, fmt.Errorf("read matrix review manifest: %w", err)
	}
	defer reviewRoot.Close()
	openedRoot, err := reviewRoot.OpenRoot()
	if err != nil {
		return nil, fmt.Errorf("read matrix review manifest: %w", err)
	}
	defer openedRoot.Close()
	file, err := openMatrixReviewFile(openedRoot, filepath.Base(absPath))
	if err != nil {
		return nil, fmt.Errorf("read matrix review manifest: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxMatrixReviewBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read matrix review manifest: %w", err)
	}
	if len(data) > maxMatrixReviewBytes {
		return nil, fmt.Errorf("read matrix review manifest: file exceeds the %d-byte size limit", maxMatrixReviewBytes)
	}
	var manifest MatrixReviewManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse matrix review manifest: %w", err)
	}
	if matrixReviewManifestLoadedForTest != nil {
		matrixReviewManifestLoadedForTest()
	}
	htmlFile, err := openMatrixReviewFile(openedRoot, "index.html")
	if err != nil {
		return nil, errMatrixReviewPairMismatch
	}
	htmlData, readErr := io.ReadAll(io.LimitReader(htmlFile, maxMatrixReviewBytes+1))
	closeErr := htmlFile.Close()
	if readErr != nil || closeErr != nil {
		return nil, errMatrixReviewPairMismatch
	}
	if err := validateMatrixReviewHTMLData(htmlData, manifest.HTMLSHA256); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// matrixReviewBrowserAsset is a bounded copy of one artifact referenced by a
// generated review. Browser snapshots rewrite links to these copies so the
// browser never follows the mutable published output tree after validation.
type matrixReviewBrowserAsset struct {
	sourceRoot   *rootfs.Root
	relativePath string
	sourcePath   string
	originalLink string
	linkPath     string
	size         int64
	identity     os.FileInfo
	digest       [sha256.Size]byte
}

type matrixReviewPairSnapshot struct {
	htmlData       []byte
	manifest       *MatrixReviewManifest
	assetRootRoots map[string]*rootfs.Root
}

// isUnboundLegacyReviewHTML reports whether path is a historical review HTML
// file that is not digest-bound to a matrix manifest. OpenReview keeps the
// direct-open fallback for those reports, including when the HTML path itself
// is a symlink that the no-follow pair reader refuses.
func isUnboundLegacyReviewHTML(ctx context.Context, path string) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil || strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	reviewRoot, err := rootfs.New(filepath.Dir(path))
	if err != nil {
		return true
	}
	defer func() { _ = reviewRoot.Close() }()
	openedRoot, err := reviewRoot.OpenRoot()
	if err != nil {
		return true
	}
	defer func() { _ = openedRoot.Close() }()
	manifestInfo, err := openedRoot.Lstat("manifest.json")
	if err != nil {
		return true
	}
	if !manifestInfo.Mode().IsRegular() || manifestInfo.Mode()&os.ModeSymlink != 0 {
		return false
	}
	if err := ctx.Err(); err != nil {
		return false
	}
	file, err := openMatrixReviewFile(openedRoot, "manifest.json")
	if err != nil {
		return true
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxMatrixReviewBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(data) > maxMatrixReviewBytes {
		return false
	}
	var binding struct {
		HTMLSHA256 string `json:"htmlSha256"`
	}
	if err := json.Unmarshal(data, &binding); err != nil || strings.TrimSpace(binding.HTMLSHA256) == "" {
		return true
	}
	return false
}

func isMatrixReviewHTMLPath(path string) bool {
	name := filepath.Base(path)
	if name == defaultReviewHTMLName {
		return true
	}
	return strings.EqualFold(name, defaultReviewHTMLName) && matrixFilesystemCaseInsensitive(path)
}

func readMatrixReviewPairSnapshotWithRoots(path string) (matrixReviewPairSnapshot, error) {
	matrixPair := isMatrixReviewHTMLPath(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return matrixReviewPairSnapshot{}, errMatrixReviewPairMismatch
	}
	reviewRoot, err := rootfs.New(filepath.Dir(absPath))
	if err != nil {
		return matrixReviewPairSnapshot{}, errMatrixReviewPairMismatch
	}
	defer func() { _ = reviewRoot.Close() }()
	openedRoot, err := reviewRoot.OpenRoot()
	if err != nil {
		return matrixReviewPairSnapshot{}, errMatrixReviewPairMismatch
	}
	defer func() { _ = openedRoot.Close() }()
	htmlFile, err := openMatrixReviewFile(openedRoot, filepath.Base(absPath))
	if err != nil {
		return matrixReviewPairSnapshot{}, errMatrixReviewPairMismatch
	}
	htmlData, readErr := io.ReadAll(io.LimitReader(htmlFile, maxMatrixReviewBytes+1))
	closeErr := htmlFile.Close()
	if readErr != nil || closeErr != nil {
		return matrixReviewPairSnapshot{}, errMatrixReviewPairMismatch
	}
	if !matrixPair {
		return matrixReviewPairSnapshot{htmlData: htmlData}, nil
	}
	matrixMarked := bytes.Contains(htmlData, []byte(`<meta name="asc-matrix-review" content="1">`))
	file, err := openMatrixReviewFile(openedRoot, "manifest.json")
	if err != nil {
		if matrixMarked {
			return matrixReviewPairSnapshot{}, errMatrixReviewPairMismatch
		}
		return matrixReviewPairSnapshot{htmlData: htmlData}, nil
	}
	data, err := io.ReadAll(io.LimitReader(file, maxMatrixReviewBytes+1))
	closeErr = file.Close()
	if err != nil || closeErr != nil || len(data) > maxMatrixReviewBytes {
		if matrixMarked {
			return matrixReviewPairSnapshot{}, errMatrixReviewPairMismatch
		}
		return matrixReviewPairSnapshot{htmlData: htmlData}, nil
	}
	var manifest MatrixReviewManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		if matrixMarked {
			return matrixReviewPairSnapshot{}, errMatrixReviewPairMismatch
		}
		return matrixReviewPairSnapshot{htmlData: htmlData}, nil
	}
	var binding struct {
		HTMLSHA256 string `json:"htmlSha256"`
	}
	if err := json.Unmarshal(data, &binding); err != nil || strings.TrimSpace(binding.HTMLSHA256) == "" {
		if matrixMarked {
			return matrixReviewPairSnapshot{}, errMatrixReviewPairMismatch
		}
		// Non-matrix and pre-binding review pairs remain backward compatible.
		return matrixReviewPairSnapshot{htmlData: htmlData}, nil
	}
	assetRootRoots, rootsErr := captureMatrixReviewAssetRootIdentities(filepath.Dir(absPath), &manifest)
	if rootsErr != nil {
		return matrixReviewPairSnapshot{}, rootsErr
	}
	if err := validateMatrixReviewHTMLData(htmlData, binding.HTMLSHA256); err != nil {
		_ = closeMatrixReviewAssetRoots(assetRootRoots)
		return matrixReviewPairSnapshot{}, err
	}
	return matrixReviewPairSnapshot{htmlData: htmlData, manifest: &manifest, assetRootRoots: assetRootRoots}, nil
}

func matrixReviewAssetRootPaths(outputDir string, manifest *MatrixReviewManifest) (map[string]string, error) {
	if manifest == nil {
		return nil, nil
	}
	manifestOutputDir, err := filepath.Abs(manifest.OutputDir)
	if err != nil || strings.TrimSpace(manifest.OutputDir) == "" || !sameMatrixDirectory(manifestOutputDir, outputDir) {
		return nil, errMatrixReviewPairMismatch
	}
	assetRoots := make(map[string]string, 2)
	for kind, value := range map[string]string{"raw": manifest.RawDir, "framed": manifest.FramedDir} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		rootPath := filepath.FromSlash(value)
		if !filepath.IsAbs(rootPath) {
			rootPath = filepath.Join(outputDir, rootPath)
		}
		rootPath, err = filepath.Abs(rootPath)
		if err != nil {
			return nil, errMatrixReviewPairMismatch
		}
		assetRoots[kind] = filepath.Clean(rootPath)
	}
	return assetRoots, nil
}

// captureMatrixReviewAssetRootIdentities retains the declared source roots while
// the review pair is still held open. OpenReview uses these descriptors for the
// later rooted source reads, so an inode cannot be recycled between pair
// validation and snapshot publication. The references, rather than only
// FileInfo values, are carried across that handoff.
func captureMatrixReviewAssetRootIdentities(outputDir string, manifest *MatrixReviewManifest) (map[string]*rootfs.Root, error) {
	if manifest == nil {
		return nil, nil
	}
	assetRoots, err := matrixReviewAssetRootPaths(outputDir, manifest)
	if err != nil {
		return nil, err
	}
	needed := make(map[string]struct{}, len(assetRoots))
	for _, cell := range manifest.Cells {
		if len(cell.RawPaths) > 0 {
			needed["raw"] = struct{}{}
		}
		if len(cell.FramedPaths) > 0 {
			needed["framed"] = struct{}{}
		}
	}
	roots := make(map[string]*rootfs.Root, len(needed))
	for kind := range needed {
		rootPath, ok := assetRoots[kind]
		if !ok {
			_ = closeMatrixReviewAssetRoots(roots)
			return nil, errMatrixReviewPairMismatch
		}
		info, statErr := os.Lstat(rootPath)
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			_ = closeMatrixReviewAssetRoots(roots)
			return nil, errMatrixReviewPairMismatch
		}
		if matrixReviewAssetRootBeforePinForTest != nil {
			matrixReviewAssetRootBeforePinForTest(kind, rootPath)
		}
		root, rootErr := rootfs.New(rootPath)
		if rootErr != nil {
			_ = closeMatrixReviewAssetRoots(roots)
			return nil, errMatrixReviewPairMismatch
		}
		opened, openErr := root.OpenRoot()
		if openErr != nil {
			_ = root.Close()
			_ = closeMatrixReviewAssetRoots(roots)
			return nil, errMatrixReviewPairMismatch
		}
		openedInfo, openedErr := opened.Stat(".")
		currentInfo, currentErr := os.Lstat(rootPath)
		_ = opened.Close()
		if openedErr != nil || currentErr != nil || currentInfo.Mode()&os.ModeSymlink != 0 || !currentInfo.IsDir() || !os.SameFile(info, openedInfo) || !os.SameFile(openedInfo, currentInfo) {
			_ = root.Close()
			_ = closeMatrixReviewAssetRoots(roots)
			return nil, errMatrixReviewPairMismatch
		}
		rootCopy := root
		roots[kind] = &rootCopy
	}
	if matrixReviewAssetRootsCapturedForTest != nil {
		for kind, rootPath := range assetRoots {
			if _, needed := roots[kind]; needed {
				matrixReviewAssetRootsCapturedForTest(kind, rootPath)
			}
		}
	}
	return roots, nil
}

func closeMatrixReviewAssetRoots(roots map[string]*rootfs.Root) error {
	seen := make(map[*rootfs.Root]struct{}, len(roots))
	var closeErr error
	for _, root := range roots {
		if root == nil {
			continue
		}
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		closeErr = errors.Join(closeErr, root.Close())
	}
	return closeErr
}

func matrixReviewBrowserAssetsWithExpectedRoots(ctx context.Context, htmlPath string, htmlData []byte, manifest *MatrixReviewManifest, expectedRoots map[string]*rootfs.Root) ([]matrixReviewBrowserAsset, error) {
	if manifest == nil {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, errMatrixReviewPairMismatch
	}
	outputDir, err := filepath.Abs(manifest.OutputDir)
	if err != nil || strings.TrimSpace(manifest.OutputDir) == "" {
		return nil, errMatrixReviewPairMismatch
	}
	htmlDir, err := filepath.Abs(filepath.Dir(htmlPath))
	if err != nil || !sameMatrixDirectory(htmlDir, outputDir) {
		return nil, errMatrixReviewPairMismatch
	}
	assetRoots, err := matrixReviewAssetRootPaths(outputDir, manifest)
	if err != nil {
		return nil, err
	}
	assetCount := 0
	for _, cell := range manifest.Cells {
		assetCount += len(cell.RawPaths) + len(cell.FramedPaths)
	}
	if assetCount > maxMatrixReviewBrowserAssets {
		return nil, errMatrixReviewPairMismatch
	}
	type openedAssetRoot struct{ root *rootfs.Root }
	openedRoots := make(map[string]openedAssetRoot)
	closeRoots := func() error {
		var closeErr error
		for _, opened := range openedRoots {
			if opened.root != nil {
				closeErr = errors.Join(closeErr, opened.root.Close())
			}
		}
		return closeErr
	}
	failed := true
	defer func() {
		if failed {
			_ = closeRoots()
		}
	}()
	assets := make([]matrixReviewBrowserAsset, 0)
	seen := make(map[string]struct{})
	var totalSize int64
	for _, cell := range manifest.Cells {
		for kind, paths := range map[string][]string{"raw": cell.RawPaths, "framed": cell.FramedPaths} {
			rootPath, ok := assetRoots[kind]
			if !ok {
				if len(paths) > 0 {
					return nil, errMatrixReviewPairMismatch
				}
				continue
			}
			for _, path := range paths {
				absolute, err := filepath.Abs(path)
				if err != nil {
					return nil, errMatrixReviewPairMismatch
				}
				relative, err := filepath.Rel(rootPath, absolute)
				if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
					return nil, errMatrixReviewPairMismatch
				}
				absolute = filepath.Clean(absolute)
				if _, ok := seen[absolute]; ok {
					continue
				}
				opened, ok := openedRoots[kind]
				if !ok {
					if expected, ok := expectedRoots[kind]; ok {
						opened = openedAssetRoot{root: expected}
					} else {
						// The declared source root is part of the validated matrix
						// manifest, but it is still a filesystem boundary. Reject a
						// final symlink and retain the rooted descriptor so later
						// reads cannot follow a replacement path.
						rootInfo, statErr := os.Lstat(rootPath)
						if statErr != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
							return nil, errMatrixReviewPairMismatch
						}
						root, err := rootfs.New(rootPath)
						if err != nil {
							return nil, errMatrixReviewPairMismatch
						}
						openedRoot, openErr := root.OpenRoot()
						if openErr != nil {
							_ = root.Close()
							return nil, errMatrixReviewPairMismatch
						}
						openedInfo, openedErr := openedRoot.Stat(".")
						currentInfo, currentErr := os.Lstat(rootPath)
						_ = openedRoot.Close()
						if openedErr != nil || currentErr != nil || currentInfo.Mode()&os.ModeSymlink != 0 || !currentInfo.IsDir() || !os.SameFile(rootInfo, openedInfo) || !os.SameFile(openedInfo, currentInfo) {
							_ = root.Close()
							return nil, errMatrixReviewPairMismatch
						}
						opened = openedAssetRoot{root: &root}
					}
					openedRoots[kind] = opened
				}
				file, err := opened.root.OpenFile(relative)
				if err != nil {
					return nil, errMatrixReviewPairMismatch
				}
				info, statErr := file.Stat()
				closeErr := file.Close()
				if statErr != nil || closeErr != nil || !info.Mode().IsRegular() || info.Size() > maxMatrixArtifactBytes {
					return nil, errMatrixReviewPairMismatch
				}
				if info.Size() > maxMatrixReviewBrowserBytes-totalSize {
					return nil, errMatrixReviewPairMismatch
				}
				if err := ctx.Err(); err != nil {
					return nil, errMatrixReviewPairMismatch
				}
				remainingBrowserBytes := maxMatrixReviewBrowserBytes - totalSize
				if remainingBrowserBytes <= 0 {
					return nil, errMatrixReviewPairMismatch
				}
				hasher := sha256.New()
				hashFile, hashOpenErr := opened.root.OpenFile(relative)
				if hashOpenErr != nil {
					return nil, errMatrixReviewPairMismatch
				}
				hashInfo, hashStatErr := hashFile.Stat()
				if hashStatErr != nil || !os.SameFile(info, hashInfo) || hashInfo.Size() != info.Size() {
					_ = hashFile.Close()
					return nil, errMatrixReviewPairMismatch
				}
				if matrixReviewAssetBeforeHashForTest != nil {
					matrixReviewAssetBeforeHashForTest(absolute)
				}
				hashedSize, hashErr := io.Copy(hasher, io.LimitReader(matrixReviewSnapshotContextReader{ctx: ctx, r: hashFile}, remainingBrowserBytes+1))
				hashCloseErr := hashFile.Close()
				if hashErr != nil || hashCloseErr != nil || hashedSize != info.Size() {
					return nil, errMatrixReviewPairMismatch
				}
				if err := ctx.Err(); err != nil {
					return nil, errMatrixReviewPairMismatch
				}
				var digest [sha256.Size]byte
				copy(digest[:], hasher.Sum(nil))
				originalLink := filepath.ToSlash(relativeOrCleanPath(outputDir, absolute))
				escapedLink := html.EscapeString(matrixArtifactURLPath(originalLink))
				if originalLink == "" || !bytes.Contains(htmlData, []byte(`href="`+escapedLink+`"`)) || !bytes.Contains(htmlData, []byte(`src="`+escapedLink+`"`)) {
					return nil, errMatrixReviewPairMismatch
				}
				assets = append(assets, matrixReviewBrowserAsset{
					sourceRoot:   opened.root,
					relativePath: relative,
					sourcePath:   absolute,
					originalLink: escapedLink,
					linkPath:     filepath.ToSlash(fmt.Sprintf("assets/%06d%s", len(assets), filepath.Ext(absolute))),
					size:         info.Size(),
					identity:     info,
					digest:       digest,
				})
				seen[absolute] = struct{}{}
				totalSize += info.Size()
				if matrixReviewAssetValidatedForTest != nil {
					matrixReviewAssetValidatedForTest(absolute)
				}
			}
		}
	}
	failed = false
	return assets, nil
}

func validateMatrixReviewHTMLData(data []byte, expected string) error {
	matrixMarked := bytes.Contains(data, []byte(`<meta name="asc-matrix-review" content="1">`))
	expected = strings.TrimSpace(expected)
	if len(data) > maxMatrixReviewBytes {
		if matrixMarked || expected != "" {
			return errMatrixReviewPairMismatch
		}
		return nil
	}
	if expected == "" {
		if matrixMarked {
			return errMatrixReviewPairMismatch
		}
		// Manifests created before the digest contract remain readable.
		return nil
	}
	digest := sha256.Sum256(data)
	if !strings.EqualFold(expected, fmt.Sprintf("%x", digest[:])) {
		return errMatrixReviewPairMismatch
	}
	return nil
}

func renderMatrixReviewHTML(manifest MatrixReviewManifest) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"asc-matrix-review\" content=\"1\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n")
	b.WriteString("<title>Screenshot matrix review</title>\n<style>")
	b.WriteString("body{font:14px system-ui,sans-serif;margin:2rem;color:#202124;background:#fff}table{border-collapse:collapse;width:100%}th,td{border:1px solid #dadce0;padding:.5rem;text-align:left;vertical-align:top}th{background:#f8f9fa}.success{color:#137333}.failed,.cleanup_failed{color:#b31412}.canceled{color:#8a4b08}.missing{color:#b31412;font-weight:600}ul{margin:.25rem 0;padding-left:1.2rem}a{word-break:break-all}img{display:block;max-width:240px;max-height:420px;margin:.25rem 0;border:1px solid #dadce0}")
	b.WriteString("</style>\n</head>\n<body>\n")
	b.WriteString("<h1>Screenshot matrix review</h1>\n")
	b.WriteString("<p>Total: " + html.EscapeString(fmt.Sprintf("%d", manifest.TotalCells)) + "; succeeded: " + html.EscapeString(fmt.Sprintf("%d", manifest.Succeeded)) + "; failed: " + html.EscapeString(fmt.Sprintf("%d", manifest.Failed)))
	// Mirrors the manifest's omitempty aggregate: only surfaced when a cell
	// captured successfully but could not restore simulator state.
	if manifest.CleanupFailed > 0 {
		b.WriteString(" (cleanup failed: " + html.EscapeString(fmt.Sprintf("%d", manifest.CleanupFailed)) + ")")
	}
	b.WriteString("; canceled: " + html.EscapeString(fmt.Sprintf("%d", manifest.Canceled)) + ".</p>\n")
	b.WriteString("<table><thead><tr><th>Cell</th><th>Axes</th><th>Status</th><th>Attempts</th><th>Artifacts</th><th>Failure</th></tr></thead><tbody>\n")
	for _, cell := range manifest.Cells {
		status := html.EscapeString(cell.Status)
		b.WriteString("<tr><td><code>" + html.EscapeString(cell.ID) + "</code></td><td>")
		b.WriteString("device=" + html.EscapeString(cell.Device) + "<br>locale=" + html.EscapeString(cell.Locale) + "<br>appearance=" + html.EscapeString(cell.Appearance) + "<br>content=" + html.EscapeString(cell.Content))
		b.WriteString("</td><td class=\"" + status + "\">" + status + "</td><td>" + html.EscapeString(fmt.Sprintf("%d", cell.Attempts)) + "</td><td>")
		artifactCount := 0
		artifactCount += writeMatrixArtifactLinks(&b, "raw", cell.RawPaths, manifest.OutputDir)
		artifactCount += writeMatrixArtifactLinks(&b, "framed", cell.FramedPaths, manifest.OutputDir)
		if artifactCount == 0 {
			b.WriteString(`<span class="missing">missing image</span><br>`)
		}
		for _, screenshot := range cell.Screenshots {
			b.WriteString(`<span class="screenshot-status">` + html.EscapeString(screenshot.Name) + `: ` + html.EscapeString(screenshot.Status) + `</span><br>`)
		}
		b.WriteString("</td><td>")
		if cell.FailureStage != "" || cell.FailureCode != "" || cell.Error != nil {
			parts := make([]string, 0, 3)
			partsToRender := []string{cell.FailureStage, cell.FailureCode}
			if cell.Error != nil {
				partsToRender = append(partsToRender, cell.Error.Message)
			}
			for _, part := range partsToRender {
				if strings.TrimSpace(part) != "" {
					parts = append(parts, strings.TrimSpace(part))
				}
			}
			b.WriteString(html.EscapeString(strings.Join(parts, ": ")))
		} else {
			b.WriteString("—")
		}
		b.WriteString("</td></tr>\n")
	}
	b.WriteString("</tbody></table>\n</body>\n</html>\n")
	return b.String()
}

func matrixReviewCellOutput(cell MatrixCellResult) asc.MatrixCellResult {
	output := asc.MatrixCellResult{
		ID:           cell.ID,
		Device:       cell.Device,
		Locale:       cell.Locale,
		Appearance:   cell.Appearance,
		Content:      cell.Content,
		Status:       cell.Status,
		Attempts:     cell.Attempts,
		DurationMS:   cell.DurationMS,
		RawPaths:     append([]string(nil), cell.RawPaths...),
		FramedPaths:  append([]string(nil), cell.FramedPaths...),
		FailureStage: "",
		FailureCode:  "",
	}
	output.Screenshots = make([]asc.MatrixScreenshotResult, len(cell.Screenshots))
	for i, screenshot := range cell.Screenshots {
		output.Screenshots[i] = asc.MatrixScreenshotResult{
			Name: screenshot.Name, Status: screenshot.Status, RawPath: screenshot.RawPath,
			FramedPath: screenshot.FramedPath, Width: screenshot.Width, Height: screenshot.Height,
		}
	}
	output.Steps = make([]asc.MatrixStepResult, len(cell.Steps))
	for i, step := range sanitizeMatrixSteps(cell.Steps) {
		output.Steps[i] = asc.MatrixStepResult{
			Index: step.Index, Action: step.Action, Status: step.Status,
			DurationMS: step.DurationMS, Error: step.Error,
		}
	}
	if cell.Status != MatrixCellSuccess {
		output.FailureStage, output.FailureCode = sanitizeMatrixReviewFailure(cell.FailureStage, cell.FailureCode)
	}
	return output
}

func matrixReviewErrorOutput(value *MatrixCellError) *asc.MatrixCellError {
	if value == nil {
		return nil
	}
	return &asc.MatrixCellError{Stage: value.Stage, Code: value.Code, Message: value.Message}
}

func writeMatrixArtifactLinks(b *strings.Builder, label string, paths []string, root string) int {
	count := 0
	for _, path := range paths {
		count++
		path = filepath.ToSlash(relativeOrCleanPath(root, path))
		escapedPath := html.EscapeString(matrixArtifactURLPath(path))
		escapedLabel := html.EscapeString(label)
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		escapedName := html.EscapeString(name)
		b.WriteString("<a href=\"" + escapedPath + "\"><img loading=\"lazy\" src=\"" + escapedPath + "\" alt=\"" + escapedLabel + " " + escapedName + " screenshot\"></a><span>" + escapedLabel + " " + escapedName + "</span><br>")
	}
	return count
}

func matrixArtifactURLPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func relativeOrCleanPath(root, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if relative, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func sanitizeMatrixReviewError(value *MatrixCellError) *MatrixCellError {
	if value == nil {
		return nil
	}
	message := strings.TrimSpace(value.Message)
	switch message {
	case "cell canceled", "screenshot plan execution failed", "screenshot framing failed", "raw screenshot could not be promoted", "framed screenshot could not be promoted", "framed screenshot became unavailable", "simulator appearance could not be restored", "simulator status bar could not be cleared", "simulator blocked after appearance cleanup failure", "appearance state could not be read", "requested appearance could not be applied", "cell execution failed", "target simulator is not ready", "configured frame does not match simulator family", "simulator family could not be identified", "configured frame mapping is invalid", "screenshot plan did not produce every requested image", "screenshot plan produced an invalid image", "screenshot framing produced an invalid image":
	default:
		message = "matrix execution failed"
	}
	stage, code := sanitizeMatrixReviewFailure(value.Stage, value.Code)
	return &MatrixCellError{Stage: stage, Code: code, Message: message}
}

func sanitizeMatrixReviewFailure(stage, code string) (string, string) {
	stage = strings.TrimSpace(stage)
	switch stage {
	case "execution", "framing", "appearance", "cleanup", "preflight":
	default:
		stage = "execution"
	}
	code = strings.TrimSpace(code)
	if !isSafeMatrixPathComponent(code) {
		code = "matrix_failure"
	}
	return stage, code
}
