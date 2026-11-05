package screenshots

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// ReviewOpenRequest configures opening a generated review HTML file.
type ReviewOpenRequest struct {
	OutputDir string
	HTMLPath  string
	DryRun    bool
}

// ReviewOpenResult describes the resolved review HTML path and open state.
type ReviewOpenResult struct {
	HTMLPath string `json:"html_path"`
	Opened   bool   `json:"opened"`
}

var (
	// matrixReviewSnapshotValidatedForTest replaces the original pathname after
	// validation so tests can prove browser consumption uses retained bytes.
	matrixReviewSnapshotValidatedForTest func(string)
	// matrixReviewSnapshotBeforeRootForTest swaps the newly-created directory
	// before its rooted handle is pinned, making construction races deterministic.
	matrixReviewSnapshotBeforeRootForTest func(string)
	matrixReviewOpenPathForTest           func(string) error
	errMatrixReviewSnapshotUnavailable    = errors.New("matrix review snapshot unavailable")
)

const (
	matrixReviewSnapshotDirPrefix    = ".asc-matrix-review-open-"
	matrixReviewSnapshotMaxAge       = 24 * time.Hour
	matrixReviewSnapshotCleanupLimit = 16
)

// cleanupStaleMatrixReviewSnapshots bounds the lifetime of successful browser
// snapshots without touching arbitrary temporary directories. Snapshots live
// in owner-only directories under the per-user temporary directory and are
// retained after a successful launch because the browser may consume them
// asynchronously. Only old, owner-only directories with our exact prefix are
// eligible for opportunistic cleanup.
func cleanupStaleMatrixReviewSnapshots() {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-matrixReviewSnapshotMaxAge)
	removed := 0
	for _, entry := range entries {
		if removed >= matrixReviewSnapshotCleanupLimit {
			return
		}
		if !strings.HasPrefix(entry.Name(), matrixReviewSnapshotDirPrefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		path := filepath.Join(os.TempDir(), entry.Name())
		if !matrixReviewSnapshotDirIsProtected(info, path) {
			continue
		}
		current, err := os.Lstat(path)
		if err != nil || current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(info, current) || !matrixReviewSnapshotDirIsProtected(current, path) {
			continue
		}
		if err := os.RemoveAll(path); err == nil {
			removed++
		}
	}
}

func removeMatrixReviewBrowserSnapshot(path string) {
	dir := filepath.Dir(path)
	if !strings.HasPrefix(filepath.Base(dir), matrixReviewSnapshotDirPrefix) {
		return
	}
	info, err := os.Lstat(dir)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || !matrixReviewSnapshotDirIsProtected(info, dir) {
		return
	}
	_ = os.RemoveAll(dir)
}

func openCreatedMatrixReviewSnapshotRoot(path string) (*os.Root, error) {
	createdInfo, err := os.Lstat(path)
	if err != nil || !createdInfo.IsDir() || createdInfo.Mode()&os.ModeSymlink != 0 {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("snapshot directory is not a real directory")
	}
	if matrixReviewSnapshotBeforeRootForTest != nil {
		matrixReviewSnapshotBeforeRootForTest(path)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	openedInfo, err := root.Stat(".")
	if err != nil || !os.SameFile(createdInfo, openedInfo) {
		_ = root.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("snapshot directory changed while opening")
	}
	return root, nil
}

func validMatrixReviewSnapshotLink(link string) bool {
	if link == "" || filepath.IsAbs(filepath.FromSlash(link)) || strings.Contains(link, `\`) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(link)))
	if clean != link || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	for _, component := range strings.Split(clean, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return true
}

// rewriteMatrixReviewAssetLinks replaces complete HTML attributes rather than
// substrings. This prevents a path such as home.png from corrupting a
// neighboring home.pngx reference and proves every generated link is present
// in both the href and img src attributes emitted by the renderer.
func rewriteMatrixReviewAssetLinks(data []byte, assets []matrixReviewBrowserAsset) ([]byte, error) {
	rewritten := append([]byte(nil), data...)
	seenDestinations := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		if !validMatrixReviewSnapshotLink(asset.linkPath) || asset.originalLink == "" {
			return nil, errMatrixReviewSnapshotUnavailable
		}
		if _, exists := seenDestinations[asset.linkPath]; exists {
			return nil, errMatrixReviewSnapshotUnavailable
		}
		seenDestinations[asset.linkPath] = struct{}{}
		hrefNeedle := []byte(`href="` + asset.originalLink + `"`)
		srcNeedle := []byte(`src="` + asset.originalLink + `"`)
		hrefCount, srcCount := bytes.Count(rewritten, hrefNeedle), bytes.Count(rewritten, srcNeedle)
		if hrefCount == 0 || hrefCount != srcCount {
			return nil, errMatrixReviewSnapshotUnavailable
		}
		rewritten = bytes.ReplaceAll(rewritten, hrefNeedle, []byte(`href="`+asset.linkPath+`"`))
		rewritten = bytes.ReplaceAll(rewritten, srcNeedle, []byte(`src="`+asset.linkPath+`"`))
	}
	return rewritten, nil
}

type matrixReviewSnapshotContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r matrixReviewSnapshotContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	if contextErr := r.ctx.Err(); contextErr != nil {
		return n, contextErr
	}
	return n, err
}

func closeMatrixReviewBrowserAssets(assets []matrixReviewBrowserAsset) error {
	seen := make(map[*rootfs.Root]struct{}, len(assets))
	var closeErr error
	for _, asset := range assets {
		if asset.sourceRoot == nil {
			continue
		}
		if _, ok := seen[asset.sourceRoot]; ok {
			continue
		}
		seen[asset.sourceRoot] = struct{}{}
		closeErr = errors.Join(closeErr, asset.sourceRoot.Close())
	}
	return closeErr
}

func createMatrixReviewBrowserSnapshotWithContext(ctx context.Context, data []byte, assets []matrixReviewBrowserAsset) (snapshotPath string, retErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", errMatrixReviewSnapshotUnavailable
	}
	cleanupStaleMatrixReviewSnapshots()
	snapshotDir, err := createMatrixReviewSnapshotDir()
	if err != nil {
		return "", errMatrixReviewSnapshotUnavailable
	}
	createdInfo, err := os.Lstat(snapshotDir)
	if err != nil || !createdInfo.IsDir() || createdInfo.Mode()&os.ModeSymlink != 0 {
		_ = os.RemoveAll(snapshotDir)
		return "", errMatrixReviewSnapshotUnavailable
	}
	removeOnError := true
	defer func() {
		if !removeOnError {
			return
		}
		current, statErr := os.Lstat(snapshotDir)
		if statErr != nil || !os.SameFile(createdInfo, current) {
			return
		}
		_ = os.RemoveAll(snapshotDir)
	}()
	snapshotRoot, err := openCreatedMatrixReviewSnapshotRoot(snapshotDir)
	if err != nil {
		return "", errMatrixReviewSnapshotUnavailable
	}
	defer func() { _ = snapshotRoot.Close() }()
	if err := snapshotRoot.Chmod(".", 0o700); err != nil {
		return "", errMatrixReviewSnapshotUnavailable
	}
	data, err = rewriteMatrixReviewAssetLinks(data, assets)
	if err != nil {
		return "", errMatrixReviewSnapshotUnavailable
	}
	if err := ctx.Err(); err != nil {
		return "", errMatrixReviewSnapshotUnavailable
	}
	path := filepath.Join(snapshotDir, "index.html")
	file, err := createMatrixReviewSnapshotFileInRoot(snapshotRoot, "index.html", path)
	if err != nil {
		return "", errMatrixReviewSnapshotUnavailable
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return "", errMatrixReviewSnapshotUnavailable
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", errMatrixReviewSnapshotUnavailable
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", errMatrixReviewSnapshotUnavailable
	}
	if err := file.Close(); err != nil {
		return "", errMatrixReviewSnapshotUnavailable
	}
	var copiedAssetBytes int64
	for _, asset := range assets {
		if err := ctx.Err(); err != nil {
			return "", errMatrixReviewSnapshotUnavailable
		}
		if !validMatrixReviewSnapshotLink(asset.linkPath) || asset.sourceRoot == nil {
			return "", errMatrixReviewSnapshotUnavailable
		}
		assetPath := filepath.Join(snapshotDir, filepath.FromSlash(asset.linkPath))
		assetRelativePath := filepath.FromSlash(asset.linkPath)
		assetRoot, err := openMatrixReviewSnapshotDirInRoot(snapshotRoot, filepath.Dir(assetRelativePath))
		if err != nil {
			return "", errMatrixReviewSnapshotUnavailable
		}
		sourceFile, err := asset.sourceRoot.OpenFile(asset.relativePath)
		if err != nil {
			_ = assetRoot.Close()
			return "", errMatrixReviewSnapshotUnavailable
		}
		sourceInfo, statErr := sourceFile.Stat()
		if statErr != nil || !sourceInfo.Mode().IsRegular() || sourceInfo.Size() > maxMatrixArtifactBytes {
			_ = sourceFile.Close()
			_ = assetRoot.Close()
			return "", errMatrixReviewSnapshotUnavailable
		}
		if asset.identity != nil && !os.SameFile(asset.identity, sourceInfo) {
			_ = sourceFile.Close()
			_ = assetRoot.Close()
			return "", errMatrixReviewSnapshotUnavailable
		}
		if sourceInfo.Size() != asset.size {
			_ = sourceFile.Close()
			_ = assetRoot.Close()
			return "", errMatrixReviewSnapshotUnavailable
		}
		remainingAssetBytes := int64(maxMatrixReviewBrowserBytes) - copiedAssetBytes
		if remainingAssetBytes <= 0 {
			_ = sourceFile.Close()
			_ = assetRoot.Close()
			return "", errMatrixReviewSnapshotUnavailable
		}
		assetFile, err := createMatrixReviewSnapshotFileInRoot(assetRoot, filepath.Base(assetRelativePath), assetPath)
		if err != nil {
			_ = sourceFile.Close()
			_ = assetRoot.Close()
			return "", errMatrixReviewSnapshotUnavailable
		}
		if err := assetFile.Chmod(0o600); err != nil {
			_ = sourceFile.Close()
			_ = assetFile.Close()
			_ = assetRoot.Close()
			return "", errMatrixReviewSnapshotUnavailable
		}
		hasher := sha256.New()
		written, copyErr := io.Copy(io.MultiWriter(assetFile, hasher), io.LimitReader(matrixReviewSnapshotContextReader{ctx: ctx, r: sourceFile}, remainingAssetBytes+1))
		sourceCloseErr := sourceFile.Close()
		var digest [sha256.Size]byte
		copy(digest[:], hasher.Sum(nil))
		if copyErr != nil || sourceCloseErr != nil || written > maxMatrixArtifactBytes || written > remainingAssetBytes || written != sourceInfo.Size() || digest != asset.digest {
			_ = assetFile.Close()
			_ = assetRoot.Close()
			return "", errMatrixReviewSnapshotUnavailable
		}
		if err := ctx.Err(); err != nil {
			_ = assetFile.Close()
			_ = assetRoot.Close()
			return "", errMatrixReviewSnapshotUnavailable
		}
		if err := assetFile.Sync(); err != nil {
			_ = assetFile.Close()
			_ = assetRoot.Close()
			return "", errMatrixReviewSnapshotUnavailable
		}
		assetCloseErr := assetFile.Close()
		assetRootCloseErr := assetRoot.Close()
		if assetCloseErr != nil || assetRootCloseErr != nil {
			return "", errMatrixReviewSnapshotUnavailable
		}
		copiedAssetBytes += written
	}
	// The browser may open the file after this function returns. Keep the
	// owner-only snapshot after a successful launch and let the bounded stale
	// cleanup above reclaim it on a later invocation.
	removeOnError = false
	return path, nil
}

// ReviewApproveRequest configures updates to approved.json.
type ReviewApproveRequest struct {
	OutputDir    string
	ManifestPath string
	ApprovalPath string
	AllReady     bool
	Keys         []string
	ScreenshotID string
	Locale       string
	Device       string
}

// ReviewApproveResult summarizes approval updates.
type ReviewApproveResult struct {
	ManifestPath  string   `json:"manifest_path"`
	ApprovalPath  string   `json:"approval_path"`
	Matched       int      `json:"matched"`
	Added         int      `json:"added"`
	TotalApproved int      `json:"total_approved"`
	Keys          []string `json:"keys,omitempty"`
}

// OpenReview opens the generated review HTML in the default browser.
func OpenReview(ctx context.Context, req ReviewOpenRequest) (*ReviewOpenResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	outputDir, err := ResolveReviewOutputDir(req.OutputDir)
	if err != nil {
		return nil, err
	}
	htmlPath, err := resolveReviewArtifactPath(outputDir, req.HTMLPath, defaultReviewHTMLName)
	if err != nil {
		return nil, err
	}
	pair, err := readMatrixReviewPairSnapshotWithRoots(htmlPath)
	if err != nil {
		if !isUnboundLegacyReviewHTML(ctx, htmlPath) {
			return nil, err
		}
		if req.DryRun {
			return &ReviewOpenResult{HTMLPath: htmlPath, Opened: false}, nil
		}
		open := openPathInBrowser
		if matrixReviewOpenPathForTest != nil {
			open = matrixReviewOpenPathForTest
		}
		if openErr := open(htmlPath); openErr != nil {
			return nil, openErr
		}
		return &ReviewOpenResult{HTMLPath: htmlPath, Opened: true}, nil
	}
	htmlData, manifest := pair.htmlData, pair.manifest
	if req.DryRun {
		_ = closeMatrixReviewAssetRoots(pair.assetRootRoots)
		return &ReviewOpenResult{
			HTMLPath: htmlPath,
			Opened:   false,
		}, nil
	}
	// Reports without a valid matrix binding retain the historical opener
	// behavior. Only generated, digest-bound matrix HTML is copied into a
	// private snapshot with its referenced assets.
	if manifest == nil {
		open := openPathInBrowser
		if matrixReviewOpenPathForTest != nil {
			open = matrixReviewOpenPathForTest
		}
		if err := open(htmlPath); err != nil {
			return nil, err
		}
		return &ReviewOpenResult{HTMLPath: htmlPath, Opened: true}, nil
	}
	assets, err := matrixReviewBrowserAssetsWithExpectedRoots(ctx, htmlPath, htmlData, manifest, pair.assetRootRoots)
	if err != nil {
		_ = closeMatrixReviewAssetRoots(pair.assetRootRoots)
		return nil, err
	}
	snapshotPath, snapshotErr := createMatrixReviewBrowserSnapshotWithContext(ctx, htmlData, assets)
	closeErr := closeMatrixReviewBrowserAssets(assets)
	if snapshotErr != nil || closeErr != nil {
		if snapshotPath != "" {
			removeMatrixReviewBrowserSnapshot(snapshotPath)
		}
		return nil, errMatrixReviewSnapshotUnavailable
	}
	if matrixReviewSnapshotValidatedForTest != nil {
		matrixReviewSnapshotValidatedForTest(htmlPath)
	}
	open := openPathInBrowser
	if matrixReviewOpenPathForTest != nil {
		open = matrixReviewOpenPathForTest
	}
	if err := open(snapshotPath); err != nil {
		removeMatrixReviewBrowserSnapshot(snapshotPath)
		return nil, err
	}
	return &ReviewOpenResult{
		HTMLPath: htmlPath,
		Opened:   true,
	}, nil
}

// ApproveReview writes/updates approval keys for review entries.
func ApproveReview(ctx context.Context, req ReviewApproveRequest) (*ReviewApproveResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	outputDir, err := ResolveReviewOutputDir(req.OutputDir)
	if err != nil {
		return nil, err
	}
	manifestPath, err := resolveReviewArtifactPath(outputDir, req.ManifestPath, defaultReviewManifestName)
	if err != nil {
		return nil, err
	}
	approvalPath, err := resolveReviewArtifactPath(outputDir, req.ApprovalPath, defaultReviewApprovalsName)
	if err != nil {
		return nil, err
	}

	manifest, err := LoadReviewManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	selectedKeys, err := selectApprovalKeys(manifest, req)
	if err != nil {
		return nil, err
	}
	approvals, err := loadApprovals(approvalPath)
	if err != nil {
		return nil, err
	}

	added := 0
	for _, key := range selectedKeys {
		if approvals[key] {
			continue
		}
		approvals[key] = true
		added++
	}

	if err := SaveApprovals(approvalPath, approvals); err != nil {
		return nil, err
	}
	return &ReviewApproveResult{
		ManifestPath:  manifestPath,
		ApprovalPath:  approvalPath,
		Matched:       len(selectedKeys),
		Added:         added,
		TotalApproved: countApproved(approvals),
		Keys:          selectedKeys,
	}, nil
}

func resolveReviewArtifactPath(outputDir, override, defaultName string) (string, error) {
	path := override
	if strings.TrimSpace(path) == "" {
		path = filepath.Join(outputDir, defaultName)
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(outputDir, path)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve artifact path: %w", err)
	}
	return absPath, nil
}

func selectApprovalKeys(manifest *ReviewManifest, req ReviewApproveRequest) ([]string, error) {
	if manifest == nil {
		return nil, fmt.Errorf("review manifest is required")
	}

	keySet := make(map[string]struct{})
	for _, key := range req.Keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		keySet[trimmed] = struct{}{}
	}
	localeFilter := strings.TrimSpace(req.Locale)
	deviceFilter := strings.TrimSpace(req.Device)
	idFilter := strings.TrimSpace(req.ScreenshotID)
	hasSelector := req.AllReady || len(keySet) > 0 || idFilter != "" || localeFilter != "" || deviceFilter != ""
	if !hasSelector {
		return nil, fmt.Errorf("provide at least one selector: --all-ready, --key, --id, --locale, or --device")
	}

	filterOnlySelection := !req.AllReady && len(keySet) == 0 && idFilter == "" && (localeFilter != "" || deviceFilter != "")
	entryByKey := make(map[string]ReviewEntry, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		entryByKey[entry.Key] = entry
	}

	selected := make([]string, 0)
	for key := range keySet {
		entry, ok := entryByKey[key]
		if !ok {
			return nil, fmt.Errorf("review key not found in manifest: %s", key)
		}
		if !matchesLocaleDeviceFilters(entry, localeFilter, deviceFilter) {
			continue
		}
		selected = append(selected, key)
	}

	for _, entry := range manifest.Entries {
		if filterOnlySelection && matchesLocaleDeviceFilters(entry, localeFilter, deviceFilter) {
			selected = append(selected, entry.Key)
		}
		if req.AllReady && entry.Status == reviewStatusReady && matchesLocaleDeviceFilters(entry, localeFilter, deviceFilter) {
			selected = append(selected, entry.Key)
		}
		if idFilter != "" && entry.ScreenshotID == idFilter && matchesLocaleDeviceFilters(entry, localeFilter, deviceFilter) {
			selected = append(selected, entry.Key)
		}
	}

	selected = uniqueSorted(selected)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no review entries matched approval selectors")
	}
	return selected, nil
}

func uniqueSorted(keys []string) []string {
	unique := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, key)
	}
	slices.Sort(unique)
	return unique
}

func matchesLocaleDeviceFilters(entry ReviewEntry, locale, device string) bool {
	if locale != "" && entry.Locale != locale {
		return false
	}
	if device != "" && entry.Device != device {
		return false
	}
	return true
}

func countApproved(approvals map[string]bool) int {
	total := 0
	for _, approved := range approvals {
		if approved {
			total++
		}
	}
	return total
}

func openPathInBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("open review HTML: %w", err)
	}
	return nil
}
