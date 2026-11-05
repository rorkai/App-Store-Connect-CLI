package screenshots

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestOpenReview_DryRun(t *testing.T) {
	outputDir := t.TempDir()
	htmlPath := filepath.Join(outputDir, defaultReviewHTMLName)
	if err := os.WriteFile(htmlPath, []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	result, err := OpenReview(context.Background(), ReviewOpenRequest{
		OutputDir: outputDir,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("OpenReview() error: %v", err)
	}
	if result.Opened {
		t.Fatal("expected dry-run open result to be false")
	}
	if result.HTMLPath != htmlPath {
		t.Fatalf("html path = %q, want %q", result.HTMLPath, htmlPath)
	}
}

func TestCreateMatrixReviewBrowserSnapshotRejectsReplacementBeforeRootPin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	previous := matrixReviewSnapshotBeforeRootForTest
	var originalPath string
	matrixReviewSnapshotBeforeRootForTest = func(path string) {
		originalPath = path
		movedPath := path + "-original"
		if err := os.Rename(path, movedPath); err != nil {
			t.Errorf("rename newly-created snapshot directory: %v", err)
			return
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Errorf("create replacement snapshot directory: %v", err)
			return
		}
		if err := os.WriteFile(filepath.Join(path, "replacement-sentinel"), []byte("keep"), 0o600); err != nil {
			t.Errorf("write replacement sentinel: %v", err)
		}
	}
	t.Cleanup(func() {
		matrixReviewSnapshotBeforeRootForTest = previous
		if originalPath == "" {
			return
		}
		_ = os.RemoveAll(originalPath)
		_ = os.RemoveAll(originalPath + "-original")
	})

	path, err := createMatrixReviewBrowserSnapshotWithContext(context.Background(), []byte("<html></html>"), nil)
	if !errors.Is(err, errMatrixReviewSnapshotUnavailable) || path != "" {
		t.Fatalf("createMatrixReviewBrowserSnapshotWithContext() = %q, %v, want replacement rejection", path, err)
	}
	if originalPath == "" {
		t.Fatal("replacement hook did not observe the created snapshot path")
	}
	if _, err := os.Stat(filepath.Join(originalPath, "replacement-sentinel")); err != nil {
		t.Fatalf("replacement snapshot was removed after pin failure: %v", err)
	}
}

func TestOpenReviewPreservesSymlinkedLegacyHTML(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("legacy review HTML symlink fallback is a Unix path contract")
	}
	outputDir := t.TempDir()
	realHTML := filepath.Join(outputDir, "legacy-real.html")
	if err := os.WriteFile(realHTML, []byte("<html><body>legacy</body></html>"), 0o644); err != nil {
		t.Fatalf("write real legacy HTML: %v", err)
	}
	htmlPath := filepath.Join(outputDir, defaultReviewHTMLName)
	if err := os.Symlink(realHTML, htmlPath); err != nil {
		t.Fatalf("symlink legacy HTML: %v", err)
	}
	previous := matrixReviewOpenPathForTest
	var openedPath string
	matrixReviewOpenPathForTest = func(path string) error {
		openedPath = path
		return nil
	}
	t.Cleanup(func() { matrixReviewOpenPathForTest = previous })
	result, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir})
	if err != nil {
		t.Fatalf("OpenReview() error = %v, want historical open of unbound symlink HTML", err)
	}
	if result == nil || result.HTMLPath != htmlPath || !result.Opened || openedPath != htmlPath {
		t.Fatalf("OpenReview() result = %+v, opened path = %q, want legacy symlink %q", result, openedPath, htmlPath)
	}
	dryRun, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir, DryRun: true})
	if err != nil {
		t.Fatalf("OpenReview(dry-run) error = %v, want historical dry-run of unbound symlink HTML", err)
	}
	if dryRun == nil || dryRun.HTMLPath != htmlPath || dryRun.Opened {
		t.Fatalf("OpenReview(dry-run) result = %+v, want unbound symlink path without opening", dryRun)
	}
}

func TestOpenReviewDoesNotTreatNonRegularSiblingManifestAsLegacy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO fixtures are a Unix path contract")
	}
	outputDir := t.TempDir()
	realHTML := filepath.Join(outputDir, "legacy-real.html")
	if err := os.WriteFile(realHTML, []byte("<html><body>legacy</body></html>"), 0o644); err != nil {
		t.Fatalf("write real legacy HTML: %v", err)
	}
	htmlPath := filepath.Join(outputDir, defaultReviewHTMLName)
	if err := os.Symlink(realHTML, htmlPath); err != nil {
		t.Fatalf("symlink legacy HTML: %v", err)
	}
	if err := os.Mkdir(filepath.Join(outputDir, defaultReviewManifestName), 0o700); err != nil {
		t.Fatalf("create sibling directory manifest: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := OpenReview(ctx, ReviewOpenRequest{OutputDir: outputDir, DryRun: true})
	if err == nil {
		t.Fatal("OpenReview() error = nil, want non-regular sibling manifest to fail closed instead of hanging as legacy")
	}
}

func TestOpenReviewKeepsLegacyHTMLPath(t *testing.T) {
	outputDir := t.TempDir()
	htmlPath := filepath.Join(outputDir, defaultReviewHTMLName)
	if err := os.WriteFile(htmlPath, []byte("<html><body>legacy</body></html>"), 0o644); err != nil {
		t.Fatalf("write legacy HTML: %v", err)
	}
	previous := matrixReviewOpenPathForTest
	var openedPath string
	matrixReviewOpenPathForTest = func(path string) error {
		openedPath = path
		return nil
	}
	t.Cleanup(func() { matrixReviewOpenPathForTest = previous })
	result, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir})
	if err != nil {
		t.Fatalf("OpenReview() error = %v", err)
	}
	if result == nil || result.HTMLPath != htmlPath || !result.Opened || openedPath != htmlPath {
		t.Fatalf("OpenReview() result = %+v, opened path = %q, want legacy path %q", result, openedPath, htmlPath)
	}
}

func TestOpenReviewPreservesExplicitWhitespaceHTMLPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows trims trailing spaces from path components")
	}
	outputDir := t.TempDir()
	htmlPath := filepath.Join(outputDir, "custom.html ")
	if err := os.WriteFile(htmlPath, []byte("<html><body>legacy</body></html>"), 0o644); err != nil {
		t.Fatalf("write whitespace HTML: %v", err)
	}
	result, err := OpenReview(context.Background(), ReviewOpenRequest{
		OutputDir: outputDir,
		HTMLPath:  htmlPath,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("OpenReview() error = %v", err)
	}
	if result == nil || result.HTMLPath != htmlPath {
		t.Fatalf("OpenReview() result = %+v, want explicit HTML path %q", result, htmlPath)
	}
}

func TestOpenReviewPreservesWhitespaceInOutputAndAssetPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows trims trailing spaces from path components")
	}
	baseDir := t.TempDir()
	outputDir := filepath.Join(baseDir, "review ")
	rawDir := filepath.Join(outputDir, "raw ")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create whitespace output/raw directories: %v", err)
	}
	assetPath := filepath.Join(rawDir, "home.png")
	if err := os.WriteFile(assetPath, []byte("asset"), 0o600); err != nil {
		t.Fatalf("write whitespace-path asset: %v", err)
	}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
		Result: &MatrixResult{
			RawDir: rawDir,
			Cells:  []MatrixCellResult{{ID: "cell", Status: MatrixCellSuccess, RawPaths: []string{assetPath}}},
		},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("GenerateMatrixReview() error: %v", err)
	}
	previous := matrixReviewOpenPathForTest
	var snapshotPath string
	matrixReviewOpenPathForTest = func(path string) error {
		snapshotPath = path
		assetCopy := filepath.Join(filepath.Dir(path), "assets", "000000.png")
		got, err := os.ReadFile(assetCopy)
		if err != nil {
			return err
		}
		if string(got) != "asset" {
			return fmt.Errorf("snapshot asset = %q, want asset", got)
		}
		return nil
	}
	t.Cleanup(func() {
		matrixReviewOpenPathForTest = previous
		if snapshotPath != "" {
			removeMatrixReviewBrowserSnapshot(snapshotPath)
		}
	})
	opened, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir})
	if err != nil {
		t.Fatalf("OpenReview() error: %v", err)
	}
	wantHTMLPath := filepath.Join(outputDir, defaultReviewHTMLName)
	if opened == nil || !opened.Opened || opened.HTMLPath != wantHTMLPath {
		t.Fatalf("OpenReview() result = %+v, want opened HTML path %q", opened, wantHTMLPath)
	}
}

func TestMatrixReviewBrowserAssetsRejectsAggregateCountCap(t *testing.T) {
	outputDir := t.TempDir()
	paths := make([]string, maxMatrixReviewBrowserAssets+1)
	for i := range paths {
		paths[i] = filepath.Join(outputDir, "raw", "image", strings.Repeat("x", 1), string(rune('a'+i%26))+".png")
	}
	manifest := &MatrixReviewManifest{
		OutputDir: outputDir,
		RawDir:    filepath.Join(outputDir, "raw"),
		Cells:     []asc.MatrixCellResult{{RawPaths: paths}},
	}
	if _, err := matrixReviewBrowserAssets(filepath.Join(outputDir, defaultReviewHTMLName), nil, manifest); !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("matrixReviewBrowserAssets() error = %v, want aggregate count rejection", err)
	}
}

func TestMatrixReviewBrowserAssetsRejectsAggregateByteCap(t *testing.T) {
	outputDir := t.TempDir()
	rawDir := filepath.Join(outputDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	assetPath := filepath.Join(rawDir, "oversized.png")
	file, err := os.OpenFile(assetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatalf("create oversized asset: %v", err)
	}
	if err := file.Truncate(int64(maxMatrixReviewBrowserBytes) + 1); err != nil {
		_ = file.Close()
		t.Fatalf("sparsely grow oversized asset: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close oversized asset: %v", err)
	}
	manifest := &MatrixReviewManifest{
		OutputDir: outputDir,
		RawDir:    rawDir,
		Cells:     []asc.MatrixCellResult{{RawPaths: []string{assetPath}}},
	}
	if _, err := matrixReviewBrowserAssets(filepath.Join(outputDir, defaultReviewHTMLName), nil, manifest); !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("matrixReviewBrowserAssets() error = %v, want aggregate byte rejection", err)
	}
}

func TestMatrixReviewBrowserAssetsBoundsGrowingAssetByAggregateByteCap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sparse growth fixture is not portable to all Windows filesystems")
	}
	outputDir := t.TempDir()
	rawDir := filepath.Join(outputDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	assetPath := filepath.Join(rawDir, "growing.png")
	if err := os.WriteFile(assetPath, []byte("small"), 0o600); err != nil {
		t.Fatalf("write initial asset: %v", err)
	}
	previous := matrixReviewAssetBeforeHashForTest
	matrixReviewAssetBeforeHashForTest = func(path string) {
		if path != assetPath {
			return
		}
		file, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			t.Errorf("open growing asset: %v", err)
			return
		}
		if err := file.Truncate(int64(maxMatrixReviewBrowserBytes) + 1); err != nil {
			t.Errorf("grow asset: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Errorf("close growing asset: %v", err)
		}
	}
	t.Cleanup(func() { matrixReviewAssetBeforeHashForTest = previous })
	manifest := &MatrixReviewManifest{
		OutputDir: outputDir,
		RawDir:    rawDir,
		Cells:     []asc.MatrixCellResult{{RawPaths: []string{assetPath}}},
	}
	htmlData := []byte(`href="raw/growing.png" src="raw/growing.png"`)
	if _, err := matrixReviewBrowserAssets(filepath.Join(outputDir, defaultReviewHTMLName), htmlData, manifest); !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("matrixReviewBrowserAssets() error = %v, want bounded growth rejection", err)
	}
}

func TestCreateMatrixReviewBrowserSnapshotRejectsCanceledContextWithoutLeak(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if path, err := createMatrixReviewBrowserSnapshotWithContext(ctx, []byte("<html></html>"), nil); !errors.Is(err, errMatrixReviewSnapshotUnavailable) || path != "" {
		t.Fatalf("createMatrixReviewBrowserSnapshotWithContext() = %q, %v, want no snapshot", path, err)
	}
}

func TestOpenReviewRewritesPrefixCollidingAssetLinks(t *testing.T) {
	outputDir := t.TempDir()
	rawDir := filepath.Join(outputDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	for _, name := range []string{"home.png", "home.pngx"} {
		if err := os.WriteFile(filepath.Join(rawDir, name), []byte(name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	result := &MatrixResult{RawDir: rawDir, Cells: []MatrixCellResult{{
		ID: "cell", Status: MatrixCellSuccess,
		RawPaths: []string{filepath.Join(rawDir, "home.png"), filepath.Join(rawDir, "home.pngx")},
	}}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: outputDir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error: %v", err)
	}
	previous := matrixReviewOpenPathForTest
	var snapshotPath string
	matrixReviewOpenPathForTest = func(path string) error {
		snapshotPath = path
		htmlData, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, link := range []string{"assets/000000.png", "assets/000001.pngx"} {
			if !bytes.Contains(htmlData, []byte(`href="`+link+`"`)) || !bytes.Contains(htmlData, []byte(`src="`+link+`"`)) {
				return fmt.Errorf("snapshot is missing exact link %q", link)
			}
		}
		return nil
	}
	t.Cleanup(func() {
		matrixReviewOpenPathForTest = previous
		if snapshotPath != "" {
			removeMatrixReviewBrowserSnapshot(snapshotPath)
		}
	})
	if _, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir}); err != nil {
		t.Fatalf("OpenReview() error: %v", err)
	}
}

func TestOpenReviewRejectsSymlinkedMatrixAsset(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix permissions")
	}
	outputDir := t.TempDir()
	rawDir := filepath.Join(outputDir, "raw")
	outsideDir := t.TempDir()
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	outsidePath := filepath.Join(outsideDir, "outside.png")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write outside asset: %v", err)
	}
	symlinkPath := filepath.Join(rawDir, "asset.png")
	if err := os.Symlink(outsidePath, symlinkPath); err != nil {
		t.Fatalf("create asset symlink: %v", err)
	}
	result := &MatrixResult{RawDir: rawDir, Cells: []MatrixCellResult{{
		ID: "cell", Status: MatrixCellSuccess, RawPaths: []string{symlinkPath},
	}}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: outputDir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error: %v", err)
	}
	if _, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir}); !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("OpenReview() error = %v, want stable pair mismatch", err)
	}
	if got, err := os.ReadFile(outsidePath); err != nil || string(got) != "outside" {
		t.Fatalf("outside asset = %q, %v, want unchanged sentinel", got, err)
	}
}

func TestOpenReviewRejectsAssetRootReplacementAfterPairRootCapture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable with open Windows handles")
	}
	outputDir := t.TempDir()
	rawDir := filepath.Join(outputDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	rawPath := filepath.Join(rawDir, "home.png")
	if err := os.WriteFile(rawPath, []byte("original"), 0o600); err != nil {
		t.Fatalf("write raw asset: %v", err)
	}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
		Result:    &MatrixResult{RawDir: rawDir, Cells: []MatrixCellResult{{ID: "cell", Status: MatrixCellSuccess, RawPaths: []string{rawPath}}}},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("GenerateMatrixReview() error: %v", err)
	}
	originalDir := rawDir + "-original"
	var replacementSentinel string
	previous := matrixReviewAssetRootsCapturedForTest
	matrixReviewAssetRootsCapturedForTest = func(kind, path string) {
		if kind != "raw" {
			return
		}
		if err := os.Rename(path, originalDir); err != nil {
			t.Errorf("rename source root: %v", err)
			return
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Errorf("replace source root: %v", err)
			return
		}
		replacementSentinel = filepath.Join(path, "replacement-sentinel")
		if err := os.WriteFile(replacementSentinel, []byte("replacement"), 0o600); err != nil {
			t.Errorf("write replacement sentinel: %v", err)
		}
	}
	t.Cleanup(func() {
		matrixReviewAssetRootsCapturedForTest = previous
		_ = os.RemoveAll(rawDir)
		_ = os.RemoveAll(originalDir)
	})
	if _, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir}); !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("OpenReview() error = %v, want source-root identity rejection", err)
	}
	if replacementSentinel == "" {
		t.Fatal("replacement callback did not create sentinel")
	}
	if _, err := os.Stat(replacementSentinel); err != nil {
		t.Fatalf("replacement sentinel stat error = %v, want replacement preserved", err)
	}
}

func TestCaptureMatrixReviewAssetRootIdentitiesClosesPinnedRootsOnLaterFailure(t *testing.T) {
	outputDir := t.TempDir()
	rawDir := filepath.Join(outputDir, "raw")
	framedDir := filepath.Join(outputDir, "framed")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	if err := os.MkdirAll(framedDir, 0o755); err != nil {
		t.Fatalf("create framed directory: %v", err)
	}
	seen := 0
	previous := matrixReviewAssetRootBeforePinForTest
	matrixReviewAssetRootBeforePinForTest = func(_, path string) {
		seen++
		if seen == 2 {
			if err := os.RemoveAll(path); err != nil {
				t.Errorf("remove later asset root: %v", err)
			}
		}
	}
	t.Cleanup(func() { matrixReviewAssetRootBeforePinForTest = previous })
	roots, err := captureMatrixReviewAssetRootIdentities(outputDir, &MatrixReviewManifest{
		OutputDir: outputDir,
		RawDir:    rawDir,
		FramedDir: framedDir,
		Cells: []asc.MatrixCellResult{{
			RawPaths:    []string{filepath.Join(rawDir, "home.png")},
			FramedPaths: []string{filepath.Join(framedDir, "home.png")},
		}},
	})
	if !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("captureMatrixReviewAssetRootIdentities() error = %v, want later pin failure", err)
	}
	if roots != nil {
		t.Fatalf("roots = %v, want nil after later pin failure", roots)
	}
	if seen != 2 {
		t.Fatalf("pin attempts = %d, want both roots visited", seen)
	}
}

func TestOpenReviewRejectsAssetRootReplacementBeforePairRootPin(t *testing.T) {
	outputDir := t.TempDir()
	rawDir := filepath.Join(outputDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	rawPath := filepath.Join(rawDir, "home.png")
	writeMinimalPNG(t, rawPath, 10, 10)
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
		Result:    &MatrixResult{RawDir: rawDir, Cells: []MatrixCellResult{{ID: "cell", Status: MatrixCellSuccess, RawPaths: []string{rawPath}}}},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("GenerateMatrixReview() error: %v", err)
	}
	originalDir := rawDir + "-original"
	var replacementSentinel string
	previous := matrixReviewAssetRootBeforePinForTest
	matrixReviewAssetRootBeforePinForTest = func(kind, path string) {
		if kind != "raw" {
			return
		}
		if err := os.Rename(path, originalDir); err != nil {
			t.Errorf("rename source root: %v", err)
			return
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Errorf("replace source root: %v", err)
			return
		}
		replacementSentinel = filepath.Join(path, "replacement-sentinel")
		if err := os.WriteFile(replacementSentinel, []byte("replacement"), 0o600); err != nil {
			t.Errorf("write replacement sentinel: %v", err)
		}
	}
	t.Cleanup(func() {
		matrixReviewAssetRootBeforePinForTest = previous
		_ = os.Remove(rawDir)
		_ = os.RemoveAll(originalDir)
	})
	if _, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir}); !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("OpenReview() error = %v, want pre-pin source-root rejection", err)
	}
	if replacementSentinel == "" {
		t.Fatal("replacement callback did not create sentinel")
	}
	if got, err := os.ReadFile(replacementSentinel); err != nil || string(got) != "replacement" {
		t.Fatalf("replacement sentinel = %q, %v, want preserved replacement", got, err)
	}
}

func TestOpenReviewRejectsSameNameAssetReplacementAfterValidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rename fixture is not reliable with open Windows handles")
	}
	outputDir := t.TempDir()
	rawDir := filepath.Join(outputDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	rawPath := filepath.Join(rawDir, "home.png")
	if err := os.WriteFile(rawPath, []byte("original"), 0o600); err != nil {
		t.Fatalf("write raw asset: %v", err)
	}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
		Result:    &MatrixResult{RawDir: rawDir, Cells: []MatrixCellResult{{ID: "cell", Status: MatrixCellSuccess, RawPaths: []string{rawPath}}}},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("GenerateMatrixReview() error: %v", err)
	}
	replacedPath := rawPath + "-original"
	previous := matrixReviewAssetValidatedForTest
	matrixReviewAssetValidatedForTest = func(path string) {
		if err := os.Rename(path, replacedPath); err != nil {
			t.Errorf("rename validated asset: %v", err)
			return
		}
		if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
			t.Errorf("write replacement asset: %v", err)
		}
	}
	t.Cleanup(func() {
		matrixReviewAssetValidatedForTest = previous
		_ = os.RemoveAll(rawDir)
	})
	if _, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir}); !errors.Is(err, errMatrixReviewSnapshotUnavailable) {
		t.Fatalf("OpenReview() error = %v, want same-name identity rejection", err)
	}
	if got, err := os.ReadFile(rawPath); err != nil || string(got) != "replacement" {
		t.Fatalf("replacement asset = %q, %v, want preserved replacement", got, err)
	}
}

func TestOpenReviewRemovesBrowserSnapshotAfterLaunchFailure(t *testing.T) {
	outputDir := t.TempDir()
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
		Result:    &MatrixResult{PlanPath: "plan.json", Cells: []MatrixCellResult{{ID: "cell", Status: MatrixCellSuccess}}},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("GenerateMatrixReview() error: %v", err)
	}
	previous := matrixReviewOpenPathForTest
	var snapshotPath string
	matrixReviewOpenPathForTest = func(path string) error {
		snapshotPath = path
		return errors.New("browser launch failed")
	}
	t.Cleanup(func() { matrixReviewOpenPathForTest = previous })

	if _, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir}); err == nil {
		t.Fatal("OpenReview() error = nil, want browser launch failure")
	}
	if snapshotPath == "" {
		t.Fatal("browser hook did not receive a snapshot path")
	}
	if _, err := os.Lstat(snapshotPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("browser snapshot stat error = %v, want launch-failure cleanup", err)
	}
	if _, err := os.Lstat(filepath.Dir(snapshotPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("browser snapshot directory stat error = %v, want launch-failure cleanup", err)
	}
}

func TestCreateMatrixReviewBrowserSnapshotUsesOwnerOnlyBoundedDirectory(t *testing.T) {
	path, err := createMatrixReviewBrowserSnapshot([]byte("<html>snapshot</html>"))
	if err != nil {
		t.Fatalf("createMatrixReviewBrowserSnapshot() error: %v", err)
	}
	t.Cleanup(func() { removeMatrixReviewBrowserSnapshot(path) })

	dir := filepath.Dir(path)
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat browser snapshot directory: %v", err)
	}
	if !matrixReviewSnapshotDirIsProtected(dirInfo, dir) {
		t.Fatalf("browser snapshot directory is not owner-only: %v", dirInfo.Mode())
	}
	if runtime.GOOS == "windows" {
		return
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat browser snapshot: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("browser snapshot mode = %v, want owner-only 0600", fileInfo.Mode())
	}
}

func TestCleanupStaleMatrixReviewSnapshotsIsBoundedAndScoped(t *testing.T) {
	stalePath, err := createMatrixReviewSnapshotDir()
	if err != nil {
		t.Fatalf("create stale snapshot directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stalePath, "index.html"), []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale snapshot: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(stalePath, 0o700); err != nil {
			t.Fatalf("chmod stale snapshot directory: %v", err)
		}
	}
	old := time.Now().Add(-matrixReviewSnapshotMaxAge - time.Hour)
	if err := os.Chtimes(stalePath, old, old); err != nil {
		t.Fatalf("age stale snapshot directory: %v", err)
	}
	protectedPath, err := os.MkdirTemp("", matrixReviewSnapshotDirPrefix+"protected-*")
	if err != nil {
		t.Fatalf("create protected snapshot directory: %v", err)
	}
	if err := os.Chmod(protectedPath, 0o755); err != nil {
		t.Fatalf("chmod protected snapshot directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(protectedPath, "index.html"), []byte("protected"), 0o600); err != nil {
		t.Fatalf("write protected snapshot: %v", err)
	}
	if err := os.Chtimes(protectedPath, old, old); err != nil {
		t.Fatalf("age protected snapshot directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(stalePath)
		_ = os.RemoveAll(protectedPath)
	})

	cleanupStaleMatrixReviewSnapshots()
	if _, err := os.Lstat(stalePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale snapshot stat error = %v, want scoped cleanup", err)
	}
	if _, err := os.Lstat(protectedPath); err != nil {
		t.Fatalf("non-owner-mode snapshot cleanup error = %v, want protected directory preserved", err)
	}
}

func TestOpenReviewAllowsOversizedLegacyHTML(t *testing.T) {
	outputDir := t.TempDir()
	html := append([]byte("<!doctype html><html><body>legacy"), bytes.Repeat([]byte{'x'}, maxMatrixReviewBytes)...)
	html = append(html, []byte("</body></html>\n")...)
	if len(html) <= maxMatrixReviewBytes {
		t.Fatalf("legacy HTML length = %d, want over %d", len(html), maxMatrixReviewBytes)
	}
	if err := os.WriteFile(filepath.Join(outputDir, defaultReviewHTMLName), html, 0o644); err != nil {
		t.Fatalf("write oversized legacy HTML: %v", err)
	}

	result, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir, DryRun: true})
	if err != nil {
		t.Fatalf("OpenReview() error = %v, want oversized legacy HTML to remain readable", err)
	}
	if result.Opened {
		t.Fatal("expected dry-run result not to open browser")
	}
}

func TestOpenReviewRejectsOversizedHTMLWithBoundManifest(t *testing.T) {
	outputDir := t.TempDir()
	html := append([]byte("<!doctype html><html><body>legacy"), bytes.Repeat([]byte{'x'}, maxMatrixReviewBytes)...)
	html = append(html, []byte("</body></html>\n")...)
	if len(html) <= maxMatrixReviewBytes {
		t.Fatalf("HTML length = %d, want over %d", len(html), maxMatrixReviewBytes)
	}
	if err := os.WriteFile(filepath.Join(outputDir, defaultReviewHTMLName), html, 0o644); err != nil {
		t.Fatalf("write oversized HTML: %v", err)
	}
	// A readable, digest-bound matrix manifest makes this a matrix pair even
	// when the marker is removed from the HTML. It must not be reclassified as
	// an unbounded legacy report.
	manifest := []byte(`{"htmlSha256":"bound-generation"}`)
	if err := os.WriteFile(filepath.Join(outputDir, defaultReviewManifestName), manifest, 0o644); err != nil {
		t.Fatalf("write bound manifest: %v", err)
	}
	manifestPath := filepath.Join(outputDir, defaultReviewManifestName)
	if _, err := LoadMatrixReviewManifest(manifestPath); !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("LoadMatrixReviewManifest() error = %v, want stable pair mismatch", err)
	}
	if _, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir, DryRun: true}); !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("OpenReview() error = %v, want stable pair mismatch", err)
	}
}

func TestOpenReviewRecognizesCaseVariantMatrixHTML(t *testing.T) {
	outputDir := t.TempDir()
	if !matrixFilesystemCaseInsensitive(outputDir) {
		t.Skip("filesystem is case-sensitive")
	}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
		Result:    &MatrixResult{Cells: []MatrixCellResult{{ID: "phone|en-US|light|default", Status: MatrixCellSuccess}}},
		OutputDir: outputDir,
	}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	htmlPath := filepath.Join(outputDir, "INDEX.HTML")
	result, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: outputDir, HTMLPath: htmlPath, DryRun: true})
	if err != nil {
		t.Fatalf("OpenReview() error = %v, want case-insensitive matrix HTML to use the bound pair", err)
	}
	if result == nil || result.HTMLPath != htmlPath {
		t.Fatalf("OpenReview() result = %+v, want HTML path %q", result, htmlPath)
	}
}

func TestApproveReview_AllReady(t *testing.T) {
	outputDir := t.TempDir()
	manifestPath := filepath.Join(outputDir, defaultReviewManifestName)
	approvalPath := filepath.Join(outputDir, defaultReviewApprovalsName)

	manifest := ReviewManifest{
		GeneratedAt: "2026-01-01T00:00:00Z",
		FramedDir:   filepath.Join(outputDir, "framed"),
		OutputDir:   outputDir,
		Entries: []ReviewEntry{
			{
				Key:          "en|iPhone_Air|home",
				ScreenshotID: "home",
				Locale:       "en",
				Device:       "iPhone_Air",
				Status:       reviewStatusReady,
			},
			{
				Key:          "en|iPhone_Air|details",
				ScreenshotID: "details",
				Locale:       "en",
				Device:       "iPhone_Air",
				Status:       reviewStatusInvalidSize,
			},
		},
	}
	writeReviewManifest(t, manifestPath, manifest)

	result, err := ApproveReview(context.Background(), ReviewApproveRequest{
		OutputDir: outputDir,
		AllReady:  true,
	})
	if err != nil {
		t.Fatalf("ApproveReview() error: %v", err)
	}

	if result.Matched != 1 {
		t.Fatalf("matched=%d, want 1", result.Matched)
	}
	if result.Added != 1 {
		t.Fatalf("added=%d, want 1", result.Added)
	}
	if result.TotalApproved != 1 {
		t.Fatalf("total_approved=%d, want 1", result.TotalApproved)
	}
	if len(result.Keys) != 1 || result.Keys[0] != "en|iPhone_Air|home" {
		t.Fatalf("unexpected approved keys: %+v", result.Keys)
	}

	approvals, err := loadApprovals(approvalPath)
	if err != nil {
		t.Fatalf("loadApprovals() error: %v", err)
	}
	if !approvals["en|iPhone_Air|home"] {
		t.Fatal("expected home key to be approved")
	}
}

func TestApproveReviewPreservesExplicitWhitespacePaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows trims trailing spaces from path components")
	}
	t.Run("manifest", func(t *testing.T) { runApproveReviewWhitespacePathTest(t, true, false) })
	t.Run("approval", func(t *testing.T) { runApproveReviewWhitespacePathTest(t, false, true) })
}

func runApproveReviewWhitespacePathTest(t *testing.T, whitespaceManifest, whitespaceApproval bool) {
	t.Helper()
	outputDir := t.TempDir()
	manifestName := "manifest.json"
	if whitespaceManifest {
		manifestName += " "
	}
	approvalName := "approved.json"
	if whitespaceApproval {
		approvalName += " "
	}
	manifestPath := filepath.Join(outputDir, manifestName)
	approvalPath := filepath.Join(outputDir, approvalName)
	writeReviewManifest(t, manifestPath, ReviewManifest{
		GeneratedAt: "2026-01-01T00:00:00Z",
		FramedDir:   filepath.Join(outputDir, "framed"),
		OutputDir:   outputDir,
		Entries: []ReviewEntry{{
			Key:          "en|iPhone_Air|home",
			ScreenshotID: "home",
			Locale:       "en",
			Device:       "iPhone_Air",
			Status:       reviewStatusReady,
		}},
	})
	if err := os.WriteFile(approvalPath, []byte("[]\n"), 0o600); err != nil {
		t.Fatalf("write whitespace approval file: %v", err)
	}

	result, err := ApproveReview(context.Background(), ReviewApproveRequest{
		OutputDir:    outputDir,
		ManifestPath: manifestPath,
		ApprovalPath: approvalPath,
		Keys:         []string{"en|iPhone_Air|home"},
	})
	if err != nil {
		t.Fatalf("ApproveReview() error = %v", err)
	}
	if result == nil || result.ManifestPath != manifestPath || result.ApprovalPath != approvalPath {
		t.Fatalf("ApproveReview() result = %+v, want literal paths %q and %q", result, manifestPath, approvalPath)
	}
	approvals, err := loadApprovals(approvalPath)
	if err != nil {
		t.Fatalf("load approvals from literal path: %v", err)
	}
	if !approvals["en|iPhone_Air|home"] {
		t.Fatal("expected approval written to explicit whitespace path")
	}
}

func TestApproveReview_ApprovesByLocaleDeviceSelectors(t *testing.T) {
	outputDir := t.TempDir()
	manifestPath := filepath.Join(outputDir, defaultReviewManifestName)

	manifest := ReviewManifest{
		GeneratedAt: "2026-01-01T00:00:00Z",
		FramedDir:   filepath.Join(outputDir, "framed"),
		OutputDir:   outputDir,
		Entries: []ReviewEntry{
			{
				Key:          "en|iPhone_Air|home",
				ScreenshotID: "home",
				Locale:       "en",
				Device:       "iPhone_Air",
				Status:       reviewStatusInvalidSize,
			},
			{
				Key:          "fr|iPhone_Air|home",
				ScreenshotID: "home",
				Locale:       "fr",
				Device:       "iPhone_Air",
				Status:       reviewStatusReady,
			},
		},
	}
	writeReviewManifest(t, manifestPath, manifest)

	result, err := ApproveReview(context.Background(), ReviewApproveRequest{
		OutputDir: outputDir,
		Locale:    "en",
		Device:    "iPhone_Air",
	})
	if err != nil {
		t.Fatalf("ApproveReview() error: %v", err)
	}
	if result.Matched != 1 || result.Added != 1 || result.TotalApproved != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Keys) != 1 || result.Keys[0] != "en|iPhone_Air|home" {
		t.Fatalf("unexpected approved keys: %+v", result.Keys)
	}
}

func TestApproveReview_RequiresSelector(t *testing.T) {
	outputDir := t.TempDir()
	manifestPath := filepath.Join(outputDir, defaultReviewManifestName)
	writeReviewManifest(t, manifestPath, ReviewManifest{GeneratedAt: "2026-01-01T00:00:00Z"})

	_, err := ApproveReview(context.Background(), ReviewApproveRequest{
		OutputDir: outputDir,
	})
	if err == nil {
		t.Fatal("expected selector error")
	}
	if !strings.Contains(err.Error(), "provide at least one selector") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeReviewManifest(t *testing.T, path string, manifest ReviewManifest) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
}

func matrixReviewBrowserAssets(htmlPath string, htmlData []byte, manifest *MatrixReviewManifest) ([]matrixReviewBrowserAsset, error) {
	return matrixReviewBrowserAssetsWithExpectedRoots(context.Background(), htmlPath, htmlData, manifest, nil)
}

func createMatrixReviewBrowserSnapshot(data []byte) (string, error) {
	return createMatrixReviewBrowserSnapshotWithContext(context.Background(), data, nil)
}
