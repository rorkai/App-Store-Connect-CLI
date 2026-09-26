package screenshots

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestMain(m *testing.M) {
	lockBase, err := os.MkdirTemp("", "asc-matrix-lock-tests-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create matrix lock test root: %v\n", err)
		os.Exit(1)
	}
	matrixGlobalLockBaseDirForTest = lockBase
	status := m.Run()
	_ = os.RemoveAll(lockBase)
	os.Exit(status)
}

func TestLoadMatrixPlanAndExpand_UsesStableAxisOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.jsonc")
	writeMatrixTestFile(t, basePath, `{
  "version": 1,
  "app": {"bundle_id": "com.example.app"},
  "steps": [{"action": "launch"}, {"action": "screenshot", "name": "home"}]
}`)
	writeMatrixTestFile(t, matrixPath, `{
  // Matrix plans accept JSONC comments.
  "version": 1,
  "base_plan": "base.json",
  "devices": [
    {"id": "phone", "udid": "PHONE-UDID"},
    {"id": "tablet", "udid": "TABLET-UDID"}
  ],
  "locales": ["en-US", "ja-JP"],
  "appearances": ["light", "dark"],
  "content_variants": [{"id": "default"}, {"id": "empty", "launch_arguments": ["--fixture", "empty"]}],
  "execution": {"max_concurrency": 2, "max_attempts": 2, "retry_backoff_ms": 1},
  "output": {"raw_dir": "raw", "framed_dir": "framed", "review_dir": "review", "frame": {"enabled": false}}
}`)

	matrix, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	base, err := LoadPlan(filepath.Join(dir, matrix.BasePlan))
	if err != nil {
		t.Fatalf("LoadPlan() error = %v", err)
	}
	cells, err := ExpandMatrix(matrix, base)
	if err != nil {
		t.Fatalf("ExpandMatrix() error = %v", err)
	}
	if len(cells) != 16 {
		t.Fatalf("got %d cells, want 16", len(cells))
	}
	got := make([]string, 0, len(cells))
	for _, cell := range cells {
		got = append(got, cell.ID)
	}
	want := []string{
		"phone|en-US|light|default", "phone|en-US|light|empty",
		"phone|en-US|dark|default", "phone|en-US|dark|empty",
		"phone|ja-JP|light|default", "phone|ja-JP|light|empty",
		"phone|ja-JP|dark|default", "phone|ja-JP|dark|empty",
		"tablet|en-US|light|default", "tablet|en-US|light|empty",
		"tablet|en-US|dark|default", "tablet|en-US|dark|empty",
		"tablet|ja-JP|light|default", "tablet|ja-JP|light|empty",
		"tablet|ja-JP|dark|default", "tablet|ja-JP|dark|empty",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cell order = %v, want %v", got, want)
	}
	if len(cells[1].RawPaths) != 1 || cells[1].RawPaths[0] != filepath.Join("raw", "en-US", "phone", "light", "empty", "home.png") {
		t.Fatalf("raw paths = %q", cells[1].RawPaths)
	}
	if got := cells[1].LaunchArguments; !reflect.DeepEqual(got, []string{"-AppleLanguages", "(en)", "-AppleLocale", "en_US", "--fixture", "empty"}) {
		t.Fatalf("launch arguments = %v", got)
	}
}

func TestLoadMatrixPlan_RejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matrix.json")
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", maxMatrixPlanBytes+1)), 0o644); err != nil {
		t.Fatalf("write oversized matrix plan: %v", err)
	}
	_, err := LoadMatrixPlan(path)
	if err == nil || !errors.Is(err, ErrMatrixPlanRead) || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("LoadMatrixPlan() error = %v, want bounded-size read error", err)
	}
}

func TestLoadMatrixPlan_RejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	link := filepath.Join(dir, "matrix.json")
	if err := os.WriteFile(target, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatalf("write target matrix plan: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create matrix plan symlink: %v", err)
	}
	_, err := LoadMatrixPlan(link)
	if err == nil || !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("LoadMatrixPlan() error = %v, want symlink rejection", err)
	}
}

func TestLoadMatrixPlanDoesNotDefaultMissingVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matrix.json")
	writeMatrixTestFile(t, path, `{"devices":[]}`)
	plan, err := LoadMatrixPlan(path)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	if plan.Version != 0 {
		t.Fatalf("matrix plan version = %d, want missing version to remain invalid", plan.Version)
	}
}

func TestLoadMatrixPlanRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matrix.json")
	writeMatrixTestFile(t, path, `{"version":1,"unknown_axis":true}`)
	_, err := LoadMatrixPlan(path)
	if err == nil || !errors.Is(err, ErrMatrixPlanParseJSON) || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("LoadMatrixPlan() error = %v, want unknown-field parse error", err)
	}
}

func TestLoadMatrixPlanRejectsDuplicateAndMisCasedFields(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "duplicate", data: `{"version":1,"version":1}`, want: "duplicate fields"},
		{name: "mis-cased", data: `{"Version":1}`, want: "exact spelling"},
		{name: "nested mis-cased", data: `{"version":1,"execution":{"Max_Attempts":2}}`, want: "exact spelling"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "matrix.json")
			writeMatrixTestFile(t, path, tc.data)
			_, err := LoadMatrixPlan(path)
			if err == nil || !errors.Is(err, ErrMatrixPlanParseJSON) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadMatrixPlan() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadMatrixPlanRejectsNullDeviceByMatrixDeviceEntryWhenFramingDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matrix.json")
	writeMatrixTestFile(t, path, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review","frame":{"enabled":false,"device_by_matrix_device":{"phone":null}}}}`)
	_, err := LoadMatrixPlan(path)
	if err == nil || !errors.Is(err, ErrMatrixPlanParseJSON) || !strings.Contains(err.Error(), "must not be null") {
		t.Fatalf("LoadMatrixPlan() error = %v, want explicit null mapping rejection", err)
	}
}

func TestRunMatrixRejectsMissingVersionBeforeLoadingBasePlan(t *testing.T) {
	plan := &MatrixPlan{BasePlan: "missing.json", sourcePath: filepath.Join(t.TempDir(), "matrix.json")}
	_, err := RunMatrixWithDependencies(context.Background(), plan.sourcePath, plan, MatrixOptions{}, MatrixDependencies{})
	var validationErr *MatrixValidationError
	if err == nil || !errors.As(err, &validationErr) || !strings.Contains(err.Error(), "expected 1") {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want matrix version validation", err)
	}
}

func TestLoadMatrixBasePlanIsRootedBoundedAndNoFollow(t *testing.T) {
	t.Parallel()

	testPlan := `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`
	tests := []struct {
		name      string
		basePlan  string
		setup     func(t *testing.T, dir string) string
		wantError string
	}{
		{
			name:      "absolute reference",
			basePlan:  "",
			setup:     func(t *testing.T, dir string) string { return filepath.Join(dir, "outside.json") },
			wantError: "must be relative",
		},
		{
			name:      "parent traversal",
			basePlan:  "../outside.json",
			setup:     func(*testing.T, string) string { return "" },
			wantError: "must stay below",
		},
		{
			name:     "symlink",
			basePlan: "base.json",
			setup: func(t *testing.T, dir string) string {
				target := filepath.Join(dir, "target.json")
				writeMatrixTestFile(t, target, testPlan)
				if err := os.Symlink(target, filepath.Join(dir, "base.json")); err != nil {
					t.Fatalf("create base plan symlink: %v", err)
				}
				return target
			},
			wantError: "symlink",
		},
		{
			name:     "oversized",
			basePlan: "base.json",
			setup: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "base.json")
				writeMatrixTestFile(t, path, strings.Repeat(" ", maxMatrixPlanBytes+1))
				return path
			},
			wantError: "size limit",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			outside := tc.setup(t, dir)
			basePlan := tc.basePlan
			if basePlan == "" {
				basePlan = outside
			}
			matrixPath := filepath.Join(dir, "matrix.json")
			plan := &MatrixPlan{BasePlan: basePlan, sourcePath: matrixPath}
			_, err := loadMatrixBasePlan(matrixPath, plan)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wantError)) {
				t.Fatalf("loadMatrixBasePlan() error = %v, want %q", err, tc.wantError)
			}
		})
	}
}

func TestLoadMatrixBasePlanPreservesLiteralFilename(t *testing.T) {
	dir := t.TempDir()
	baseName := "base plan .json "
	basePath := filepath.Join(dir, baseName)
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	matrixPath := filepath.Join(dir, "matrix.json")
	plan := &MatrixPlan{BasePlan: baseName, sourcePath: matrixPath}
	loaded, err := loadMatrixBasePlan(matrixPath, plan)
	if err != nil {
		t.Fatalf("loadMatrixBasePlan() error = %v", err)
	}
	if loaded.App.BundleID != "com.example.app" {
		t.Fatalf("loaded base plan = %+v", loaded)
	}
}

func TestLoadMatrixBasePlanRetainsVersionZeroCompatibility(t *testing.T) {
	dir := t.TempDir()
	baseName := "base.json"
	writeMatrixTestFile(t, filepath.Join(dir, baseName), `{"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	plan := &MatrixPlan{BasePlan: baseName, sourcePath: filepath.Join(dir, "matrix.json")}
	loaded, err := loadMatrixBasePlan(plan.sourcePath, plan)
	if err != nil {
		t.Fatalf("loadMatrixBasePlan() error = %v", err)
	}
	if loaded.Version != 1 {
		t.Fatalf("loaded base plan version = %d, want compatibility default 1", loaded.Version)
	}
}

func TestExpandMatrixPreservesLiteralOutputDirectorySpelling(t *testing.T) {
	base := &Plan{
		Version: 1,
		App:     PlanApp{BundleID: "com.example.app"},
		Steps:   []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}},
	}
	plan := &MatrixPlan{
		Version:         1,
		Devices:         []MatrixDevice{{ID: "phone", UDID: "SIM-UDID"}},
		Locales:         []string{"en-US"},
		Appearances:     []string{"light"},
		ContentVariants: []MatrixContentVariant{{ID: "default"}},
		Output:          MatrixOutput{RawDir: "raw screenshots ", FramedDir: "framed screenshots ", ReviewDir: "review screenshots "},
	}
	cells, err := ExpandMatrix(plan, base)
	if err != nil {
		t.Fatalf("ExpandMatrix() error = %v", err)
	}
	if got, want := cells[0].RawDir, filepath.Join("raw screenshots ", "en-US", "phone", "light", "default"); got != want {
		t.Fatalf("raw directory = %q, want %q", got, want)
	}
}

func TestRunMatrixRejectsOutputAliasesResolvedFromMatrixPlanDirectory(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	plan := &MatrixPlan{
		Version:         1,
		BasePlan:        "base.json",
		Devices:         []MatrixDevice{{ID: "phone", UDID: "SIM-UDID"}},
		Locales:         []string{"en-US"},
		Appearances:     []string{"light"},
		ContentVariants: []MatrixContentVariant{{ID: "default"}},
		Output: MatrixOutput{
			RawDir:    "raw",
			FramedDir: filepath.Join(dir, "raw"),
			ReviewDir: "review",
		},
		sourcePath: matrixPath,
	}
	_, err := RunMatrixWithDependencies(context.Background(), matrixPath, plan, MatrixOptions{}, MatrixDependencies{})
	var validationErr *MatrixValidationError
	if err == nil || !errors.As(err, &validationErr) || !strings.Contains(err.Error(), "must be different directories") {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want plan-directory output collision", err)
	}
}

func TestRunMatrixWritesReviewAfterOutputRootSetupFailure(t *testing.T) {
	tests := []struct {
		name      string
		rawDir    string
		framedDir string
		frame     MatrixFrame
		badPath   string
	}{
		{
			name:    "raw root",
			rawDir:  "raw-file",
			badPath: "raw-file",
		},
		{
			name:      "framed root",
			rawDir:    "raw",
			framedDir: "framed-file",
			frame: MatrixFrame{
				Enabled:              true,
				DeviceByMatrixDevice: map[string]string{"phone": "iphone-17-pro"},
			},
			badPath: "framed-file",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			basePath := filepath.Join(dir, "base.json")
			matrixPath := filepath.Join(dir, "matrix.json")
			writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
			if err := os.WriteFile(filepath.Join(dir, tc.badPath), []byte("not a directory"), 0o600); err != nil {
				t.Fatalf("write unusable output root: %v", err)
			}
			matrixPlan := &MatrixPlan{
				Version:         1,
				BasePlan:        "base.json",
				Devices:         []MatrixDevice{{ID: "phone", UDID: "SIM-UDID"}},
				Locales:         []string{"en-US"},
				Appearances:     []string{"light"},
				ContentVariants: []MatrixContentVariant{{ID: "default"}},
				Output: MatrixOutput{
					RawDir:    tc.rawDir,
					FramedDir: tc.framedDir,
					ReviewDir: "review",
					Frame:     tc.frame,
				},
				sourcePath: matrixPath,
			}
			result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
				CheckDevice: func(context.Context, MatrixDevice) error { return nil },
			})
			if runErr == nil {
				t.Fatal("RunMatrixWithDependencies() error = nil, want output-root failure")
			}
			if result == nil {
				t.Fatal("RunMatrixWithDependencies() result = nil, want partial result")
			}
			if result.Review == nil {
				t.Fatalf("RunMatrixWithDependencies() review = nil, want review after %s failure", tc.name)
			}
			if result.Status != MatrixCellFailed || result.Failed != len(result.Cells) {
				t.Fatalf("unexpected output-root result: %+v", result)
			}
			if result.Cells[0].FailureCode != "output_root_failed" {
				t.Fatalf("cell failure code = %q, want output_root_failed", result.Cells[0].FailureCode)
			}
			for _, name := range []string{"manifest.json", "index.html"} {
				if _, err := os.Stat(filepath.Join(dir, "review", name)); err != nil {
					t.Fatalf("review %s missing after output-root failure: %v", name, err)
				}
			}
		})
	}
}

func TestRunMatrixDropsReplacedFramedPathBeforeReview(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"review","frame":{"enabled":true,"device_by_matrix_device":{"phone":"iphone-17-pro"}}}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	framedPath := filepath.Join(dir, "framed", "en-US", "phone", "light", "default", "home.png")
	appearance := &matrixTestAppearance{restoreFunc: func() {
		if err := os.Remove(framedPath); err != nil {
			t.Errorf("remove framed path for replacement: %v", err)
			return
		}
		if err := os.WriteFile(framedPath, []byte("replacement"), 0o644); err != nil {
			t.Errorf("write replacement framed path: %v", err)
		}
	}}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Frame: func(_ context.Context, request FrameRequest) (*FrameResult, error) {
			writeMatrixPNG(t, request.OutputPath)
			return &FrameResult{}, nil
		},
		Appearance: appearance,
	})
	if runErr == nil {
		t.Fatal("RunMatrixWithDependencies() error = nil, want replaced-frame uncertainty")
	}
	if result == nil || len(result.Cells) != 1 {
		t.Fatalf("result = %+v, want one cell", result)
	}
	cell := result.Cells[0]
	if len(cell.FramedPaths) != 0 {
		t.Fatalf("framed paths = %v, want replaced path dropped", cell.FramedPaths)
	}
	if len(cell.Screenshots) != 1 || cell.Screenshots[0].FramedPath != "" {
		t.Fatalf("screenshot metadata = %+v, want no stale framed path", cell.Screenshots)
	}
	if cell.Screenshots[0].Status != MatrixCellFailed {
		t.Fatalf("screenshot status = %q, want failed after framed path invalidation", cell.Screenshots[0].Status)
	}
	if cell.FailureStage != "framing" || cell.FailureCode != "framed_output_unavailable" {
		t.Fatalf("cell failure = %s/%s, want framing/framed_output_unavailable", cell.FailureStage, cell.FailureCode)
	}
	manifest, err := LoadMatrixReviewManifest(filepath.Join(dir, "review", "manifest.json"))
	if err != nil {
		t.Fatalf("LoadMatrixReviewManifest() error = %v", err)
	}
	if len(manifest.Cells) != 1 || len(manifest.Cells[0].FramedPaths) != 0 {
		t.Fatalf("review manifest retained stale framed path: %+v", manifest.Cells)
	}
}

func TestRunMatrixMarksScreenshotsFailedWhenFramingFailsBeforePromotion(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"review","frame":{"enabled":true,"device_by_matrix_device":{"phone":"iphone-17-pro"}}}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Frame: func(context.Context, FrameRequest) (*FrameResult, error) {
			return nil, errors.New("injected koubou failure")
		},
		Appearance: &matrixTestAppearance{},
	})
	if runErr == nil {
		t.Fatal("RunMatrixWithDependencies() error = nil, want framing cell failure")
	}
	if result == nil || len(result.Cells) != 1 {
		t.Fatalf("result = %+v, want one failed framing cell", result)
	}
	cell := result.Cells[0]
	if cell.FailureStage != "framing" || len(cell.FramedPaths) != 0 {
		t.Fatalf("cell = %+v, want framing failure with no promoted frames", cell)
	}
	if len(cell.Screenshots) != 1 || cell.Screenshots[0].RawPath == "" || cell.Screenshots[0].FramedPath != "" {
		t.Fatalf("screenshot metadata = %+v, want raw path without framed path", cell.Screenshots)
	}
	if cell.Screenshots[0].Status != MatrixCellFailed {
		t.Fatalf("screenshot status = %q, want failed when requested framing fails", cell.Screenshots[0].Status)
	}
}

func TestRunMatrixMarksCanceledUnframedScreenshotsIncomplete(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"review","frame":{"enabled":true,"device_by_matrix_device":{"phone":"iphone-17-pro"}}}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result, runErr := RunMatrixWithDependencies(ctx, matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Frame: func(context.Context, FrameRequest) (*FrameResult, error) {
			cancel()
			return nil, context.Canceled
		},
		Appearance: &matrixTestAppearance{},
	})
	if runErr == nil {
		t.Fatal("RunMatrixWithDependencies() error = nil, want canceled framing")
	}
	if result == nil || len(result.Cells) != 1 {
		t.Fatalf("result = %+v, want one canceled framing cell", result)
	}
	cell := result.Cells[0]
	if cell.Status != MatrixCellCanceled || cell.FailureStage != "framing" {
		t.Fatalf("cell = %+v, want canceled framing stage", cell)
	}
	if len(cell.Screenshots) != 1 || cell.Screenshots[0].RawPath == "" || cell.Screenshots[0].FramedPath != "" {
		t.Fatalf("screenshot metadata = %+v, want raw path without framed path", cell.Screenshots)
	}
	if cell.Screenshots[0].Status != MatrixCellCanceled {
		t.Fatalf("screenshot status = %q, want canceled when requested framing never completed", cell.Screenshots[0].Status)
	}
}

func TestRunMatrixAcceptsCaseInsensitiveFrameMapping(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"review","frame":{"enabled":true,"device_by_matrix_device":{"PHONE":"iphone-17-pro"}}}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	var gotFrameDevice string
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Frame: func(_ context.Context, request FrameRequest) (*FrameResult, error) {
			gotFrameDevice = request.Device
			writeMatrixPNG(t, request.OutputPath)
			return &FrameResult{}, nil
		},
		Appearance: &matrixTestAppearance{},
	})
	if runErr != nil {
		t.Fatalf("RunMatrixWithDependencies() error = %v", runErr)
	}
	if gotFrameDevice != "iphone-17-pro" {
		t.Fatalf("frame device = %q, want case-insensitive mapping value", gotFrameDevice)
	}
	if result == nil || result.Succeeded != 1 || len(result.Cells[0].FramedPaths) != 1 {
		t.Fatalf("result = %+v, want successful framed cell", result)
	}
}

func TestRunMatrixDoesNotComparePlanRelativeOutputsAgainstWorkingDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	plan := &MatrixPlan{
		Version:         1,
		BasePlan:        "base.json",
		Devices:         []MatrixDevice{{ID: "phone", UDID: "SIM-UDID"}},
		Locales:         []string{"en-US"},
		Appearances:     []string{"light"},
		ContentVariants: []MatrixContentVariant{{ID: "default"}},
		Output: MatrixOutput{
			RawDir:    "raw",
			FramedDir: filepath.Join(cwd, "raw"),
			ReviewDir: "review",
		},
		sourcePath: matrixPath,
	}
	runPlan := func(_ context.Context, screenshotPlan *Plan) (*RunResult, error) {
		writeMatrixPNG(t, filepath.Join(screenshotPlan.App.OutputDir, "home.png"))
		return &RunResult{Steps: []RunStepResult{{Index: 1, Action: "screenshot", Status: "ok"}}}, nil
	}
	_, err = RunMatrixWithDependencies(context.Background(), matrixPath, plan, MatrixOptions{}, MatrixDependencies{
		RunPlan: runPlan, Appearance: &matrixTestAppearance{},
	})
	if err != nil {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want distinct plan-relative outputs", err)
	}
}

func TestValidateMatrixPlan_RejectsUnsafeAndConflictingValues(t *testing.T) {
	t.Parallel()

	base := &Plan{
		Version: 1,
		App:     PlanApp{BundleID: "com.example.app"},
		Steps:   []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}},
	}
	cases := []struct {
		name string
		plan MatrixPlan
		want string
	}{
		{
			name: "path device id",
			plan: MatrixPlan{Version: 1, Devices: []MatrixDevice{{ID: "../phone", UDID: "u"}}, Locales: []string{"en-US"}, Appearances: []string{"light"}, ContentVariants: []MatrixContentVariant{{ID: "default"}}},
			want: "device id",
		},
		{
			name: "duplicate udid",
			plan: MatrixPlan{Version: 1, Devices: []MatrixDevice{{ID: "one", UDID: "same"}, {ID: "two", UDID: " SAME "}}, Locales: []string{"en-US"}, Appearances: []string{"light"}, ContentVariants: []MatrixContentVariant{{ID: "default"}}},
			want: "unique",
		},
		{
			name: "case-insensitive device id",
			plan: MatrixPlan{Version: 1, Devices: []MatrixDevice{{ID: "Phone", UDID: "one"}, {ID: "phone", UDID: "two"}}, Locales: []string{"en-US"}, Appearances: []string{"light"}, ContentVariants: []MatrixContentVariant{{ID: "default"}}},
			want: "unique",
		},
		{
			name: "content locale override",
			plan: MatrixPlan{Version: 1, Devices: []MatrixDevice{{ID: "phone", UDID: "u"}}, Locales: []string{"en-US"}, Appearances: []string{"light"}, ContentVariants: []MatrixContentVariant{{ID: "default", LaunchArguments: []string{"-AppleLocale", "fr_FR"}}}},
			want: "AppleLocale",
		},
		{
			name: "too many cells",
			plan: MatrixPlan{Version: 1, Devices: makeMatrixDevices(17), Locales: makeMatrixLocales(16), Appearances: []string{"light"}, ContentVariants: []MatrixContentVariant{{ID: "default"}}},
			want: "256",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMatrixPlan(&tc.plan, base)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateMatrixPlan() error = %v, want substring %q", err, tc.want)
			}
		})
	}
	caseInsensitiveScreenshots := *base
	caseInsensitiveScreenshots.Steps = []PlanStep{
		{Action: ActionScreenshot, Name: stringPtr("Home")},
		{Action: ActionScreenshot, Name: stringPtr("home")},
	}
	if err := ValidateMatrixPlan(&MatrixPlan{
		Version:         1,
		Devices:         []MatrixDevice{{ID: "phone", UDID: "u"}},
		Locales:         []string{"en-US"},
		Appearances:     []string{"light"},
		ContentVariants: []MatrixContentVariant{{ID: "default"}},
	}, &caseInsensitiveScreenshots); err == nil || !strings.Contains(err.Error(), "unique") {
		t.Fatalf("ValidateMatrixPlan() error = %v, want case-insensitive screenshot-name collision", err)
	}
}

func TestBuildLocaleLaunchArguments_NormalizesLocale(t *testing.T) {
	args, err := BuildLocaleLaunchArguments("pt-BR")
	if err != nil {
		t.Fatalf("BuildLocaleLaunchArguments() error = %v", err)
	}
	want := []string{"-AppleLanguages", "(pt)", "-AppleLocale", "pt_BR"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestBuildLocaleLaunchArgumentsPreservesScriptSubtags(t *testing.T) {
	tests := []struct {
		locale string
		want   []string
	}{
		{locale: "zh-hans", want: []string{"-AppleLanguages", "(zh-Hans)", "-AppleLocale", "zh_Hans"}},
		{locale: "zh_Hant", want: []string{"-AppleLanguages", "(zh-Hant)", "-AppleLocale", "zh_Hant"}},
		{locale: "sr-latn", want: []string{"-AppleLanguages", "(sr-Latn)", "-AppleLocale", "sr_Latn"}},
		{locale: "zh-Hans-CN", want: []string{"-AppleLanguages", "(zh-Hans)", "-AppleLocale", "zh_Hans_CN"}},
		{locale: "pt-BR", want: []string{"-AppleLanguages", "(pt)", "-AppleLocale", "pt_BR"}},
	}
	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			args, err := BuildLocaleLaunchArguments(tt.locale)
			if err != nil {
				t.Fatalf("BuildLocaleLaunchArguments() error = %v", err)
			}
			if !reflect.DeepEqual(args, tt.want) {
				t.Fatalf("BuildLocaleLaunchArguments(%q) = %v, want %v", tt.locale, args, tt.want)
			}
		})
	}
}

func TestBuildLocaleLaunchArgumentsRejectsMalformedSubtagStructure(t *testing.T) {
	for _, locale := range []string{"en-12", "en-USA", "en-US-Latn", "en-Latn-US-extra", "en--US"} {
		t.Run(locale, func(t *testing.T) {
			if _, err := BuildLocaleLaunchArguments(locale); err == nil {
				t.Fatalf("BuildLocaleLaunchArguments(%q) error = nil, want malformed locale rejection", locale)
			}
		})
	}

	for _, locale := range []string{"en", "en-US", "es-419", "zh-Hans", "zh-Hans-CN"} {
		t.Run("valid_"+locale, func(t *testing.T) {
			if _, err := BuildLocaleLaunchArguments(locale); err != nil {
				t.Fatalf("BuildLocaleLaunchArguments(%q) error = %v, want valid locale", locale, err)
			}
		})
	}
}

func TestSanitizeMatrixReviewErrorPreservesFramePreflightReasons(t *testing.T) {
	tests := []struct {
		code    string
		message string
	}{
		{code: matrixPreflightFrameMismatch, message: "configured frame does not match simulator family"},
		{code: matrixPreflightFamilyUnknown, message: "simulator family could not be identified"},
		{code: matrixPreflightMappingInvalid, message: "configured frame mapping is invalid"},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			got := sanitizeMatrixReviewError(newMatrixCellError("preflight", test.code, test.message))
			if got == nil || got.Stage != "preflight" || got.Code != test.code || got.Message != test.message {
				t.Fatalf("sanitizeMatrixReviewError() = %+v, want stable frame preflight reason", got)
			}
		})
	}
}

func TestValidateMatrixPlanDefersFrameFamilyToSimulator(t *testing.T) {
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	plan := &MatrixPlan{
		Version:         1,
		Devices:         []MatrixDevice{{ID: "ipad-pro-13", UDID: "IPAD-UDID"}},
		Locales:         []string{"en-US"},
		Appearances:     []string{"light"},
		ContentVariants: []MatrixContentVariant{{ID: "default"}},
		Output:          MatrixOutput{Frame: MatrixFrame{Enabled: true, DeviceByMatrixDevice: map[string]string{"ipad-pro-13": "iphone-17-pro"}}},
	}
	if err := ValidateMatrixPlan(plan, base); err != nil {
		t.Fatalf("ValidateMatrixPlan() error = %v, want syntax-only frame validation", err)
	}
}

func TestValidateMatrixPlanRejectsMappingsForUndeclaredDevices(t *testing.T) {
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	plan := &MatrixPlan{
		Version:         1,
		Devices:         []MatrixDevice{{ID: "phone", UDID: "SIM-UDID"}},
		Locales:         []string{"en-US"},
		Appearances:     []string{"light"},
		ContentVariants: []MatrixContentVariant{{ID: "default"}},
		Output: MatrixOutput{Frame: MatrixFrame{Enabled: true, DeviceByMatrixDevice: map[string]string{
			"phone": "iphone-17-pro", "stale-device": "ipad-pro-13",
		}}},
	}
	if err := ValidateMatrixPlan(plan, base); err == nil || !strings.Contains(err.Error(), "undeclared") {
		t.Fatalf("ValidateMatrixPlan() error = %v, want undeclared frame mapping failure", err)
	}
}

func TestValidateMatrixPlanRejectsCaseInsensitiveDuplicateFrameMappings(t *testing.T) {
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	plan := &MatrixPlan{
		Version:         1,
		Devices:         []MatrixDevice{{ID: "phone", UDID: "SIM-UDID"}},
		Locales:         []string{"en-US"},
		Appearances:     []string{"light"},
		ContentVariants: []MatrixContentVariant{{ID: "default"}},
		Output: MatrixOutput{Frame: MatrixFrame{Enabled: true, DeviceByMatrixDevice: map[string]string{
			"phone": "iphone-17-pro", " PHONE ": "iphone-air",
		}}},
	}
	if err := ValidateMatrixPlan(plan, base); err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Fatalf("ValidateMatrixPlan() error = %v, want case-insensitive duplicate mapping failure", err)
	}
}

func TestValidateMatrixPlanRejectsInvalidFrameMappingWhenDisabled(t *testing.T) {
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	plan := &MatrixPlan{
		Version:         1,
		Devices:         []MatrixDevice{{ID: "phone", UDID: "SIM-UDID"}},
		Locales:         []string{"en-US"},
		Appearances:     []string{"light"},
		ContentVariants: []MatrixContentVariant{{ID: "default"}},
		Output: MatrixOutput{Frame: MatrixFrame{Enabled: false, DeviceByMatrixDevice: map[string]string{
			"phone": "not-a-device",
		}}},
	}
	if err := ValidateMatrixPlan(plan, base); err == nil || !strings.Contains(err.Error(), "not-a-device") {
		t.Fatalf("ValidateMatrixPlan() error = %v, want stated mapping validation while framing is disabled", err)
	}
	plan.Output.Frame.DeviceByMatrixDevice = map[string]string{"tablet": "iphone-17-pro"}
	if err := ValidateMatrixPlan(plan, base); err == nil || !strings.Contains(err.Error(), "undeclared") {
		t.Fatalf("ValidateMatrixPlan() error = %v, want undeclared mapping rejection while framing is disabled", err)
	}
}

func TestValidateMatrixFrameMappingForSimulatorUsesActualFamily(t *testing.T) {
	tests := []struct {
		name      string
		matrixID  string
		simulator matrixSimulatorDevice
		wantError bool
	}{
		{
			name:     "iPhone actual and logical phone",
			matrixID: "sim-a",
			simulator: matrixSimulatorDevice{
				Name:                 "iPhone 16 Pro",
				DeviceTypeIdentifier: "com.apple.CoreSimulator.SimDeviceType.iPhone-16-Pro",
			},
		},
		{
			name:     "iPhone actual despite iPad logical label",
			matrixID: "ipad-demo",
			simulator: matrixSimulatorDevice{
				Name:                 "iPhone 16 Pro",
				DeviceTypeIdentifier: "com.apple.CoreSimulator.SimDeviceType.iPhone-16-Pro",
			},
		},
		{
			name:     "iPad actual",
			matrixID: "sim-a",
			simulator: matrixSimulatorDevice{
				Name:                 "iPad Pro (13-inch)",
				DeviceTypeIdentifier: "com.apple.CoreSimulator.SimDeviceType.iPad-Pro-13-inch-M4",
			},
			wantError: true,
		},
		{
			name:     "unknown actual type",
			matrixID: "sim-a",
			simulator: matrixSimulatorDevice{
				Name:                 "Test Device",
				DeviceTypeIdentifier: "com.apple.CoreSimulator.SimDeviceType.Unknown",
			},
			wantError: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMatrixFrameMappingForSimulator(tc.matrixID, "iphone-17-pro", tc.simulator)
			if (err != nil) != tc.wantError {
				t.Fatalf("validateMatrixFrameMappingForSimulator() error = %v, wantError=%t", err, tc.wantError)
			}
		})
	}
}

func TestCheckMatrixDeviceRejectsOversizedInventory(t *testing.T) {
	skipWindowsUnixExecutableFixtures(t)
	binDir := t.TempDir()
	xcrunPath := filepath.Join(binDir, "xcrun")
	script := "#!/bin/sh\nexec /usr/bin/head -c 4194305 /dev/zero\n"
	if err := os.WriteFile(xcrunPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write xcrun fixture: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := checkMatrixDevice(context.Background(), MatrixDevice{ID: "phone", UDID: "SIM"})
	if err == nil || !strings.Contains(err.Error(), "exceeded the output size limit") {
		t.Fatalf("checkMatrixDevice() error = %v, want bounded-output error", err)
	}
}

func TestCheckMatrixDevicesUsesSimulatorModelForFrameFamily(t *testing.T) {
	skipWindowsUnixExecutableFixtures(t)
	binDir := t.TempDir()
	xcrunPath := filepath.Join(binDir, "xcrun")
	script := `#!/bin/sh
set -eu
printf '%s\n' '{"devices":{"runtime":[{"udid":"SIM-UDID","state":"Booted","isAvailable":true,"name":"iPad Pro (13-inch)","deviceTypeIdentifier":"com.apple.CoreSimulator.SimDeviceType.iPad-Pro"}]}}'
`
	if err := os.WriteFile(xcrunPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write xcrun fixture: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	plan := &MatrixPlan{
		Devices: []MatrixDevice{{ID: "iphone-demo", UDID: "SIM-UDID"}},
		Output: MatrixOutput{
			Frame: MatrixFrame{Enabled: true, DeviceByMatrixDevice: map[string]string{"iphone-demo": "iphone-17-pro"}},
		},
	}
	failures, err := checkMatrixDevices(context.Background(), plan)
	if err != nil {
		t.Fatalf("checkMatrixDevices() error = %v", err)
	}
	if _, failed := failures["iphone-demo"]; !failed {
		t.Fatalf("checkMatrixDevices() failures = %v, want model/frame mismatch", failures)
	}
}

func TestReadMatrixSimulatorInventoryUsesBoundedTimeout(t *testing.T) {
	skipWindowsUnixExecutableFixtures(t)
	binDir := t.TempDir()
	xcrunPath := filepath.Join(binDir, "xcrun")
	script := "#!/bin/sh\nwhile :; do :; done\n"
	if err := os.WriteFile(xcrunPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write xcrun fixture: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	parentCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	started := time.Now()
	_, err := readMatrixSimulatorInventoryWithTimeout(parentCtx, 50*time.Millisecond)
	if !errors.Is(err, ErrMatrixInventoryTimeout) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("readMatrixSimulatorInventoryWithTimeout() error = %v, want non-context inventory timeout", err)
	}
	if elapsed := time.Since(started); elapsed >= 500*time.Millisecond {
		t.Fatalf("inventory command took %s, want derived timeout before caller deadline", elapsed)
	}
}

func TestReadMatrixSimulatorInventoryWithTimeoutPreservesParentContext(t *testing.T) {
	skipWindowsUnixExecutableFixtures(t)
	tests := []struct {
		name       string
		newContext func() (context.Context, context.CancelFunc)
		wantError  error
	}{
		{
			name: "caller canceled",
			newContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			wantError: context.Canceled,
		},
		{
			name: "caller deadline exceeded",
			newContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 50*time.Millisecond)
			},
			wantError: context.DeadlineExceeded,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binDir := t.TempDir()
			xcrunPath := filepath.Join(binDir, "xcrun")
			script := "#!/bin/sh\nwhile :; do :; done\n"
			if err := os.WriteFile(xcrunPath, []byte(script), 0o755); err != nil {
				t.Fatalf("write xcrun fixture: %v", err)
			}
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			ctx, cancel := tc.newContext()
			defer cancel()
			_, err := readMatrixSimulatorInventoryWithTimeout(ctx, time.Second)
			if !errors.Is(err, tc.wantError) {
				t.Fatalf("readMatrixSimulatorInventoryWithTimeout() error = %v, want %v", err, tc.wantError)
			}
			if errors.Is(err, ErrMatrixInventoryTimeout) {
				t.Fatalf("readMatrixSimulatorInventoryWithTimeout() error = %v, want parent context error", err)
			}
		})
	}
}

func TestSimctlMatrixAppearanceUsesSupportedUIContractAndRestores(t *testing.T) {
	skipWindowsUnixExecutableFixtures(t)
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "xcrun.log")
	xcrunPath := filepath.Join(binDir, "xcrun")
	script := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$XCRUN_LOG"
if [ "$#" -eq 4 ] && [ "$1" = "simctl" ] && [ "$2" = "ui" ] && [ "$4" = "appearance" ]; then
  printf '%s\n' dark
fi
`
	if err := os.WriteFile(xcrunPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write xcrun fixture: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XCRUN_LOG", logPath)

	appearance := simctlMatrixAppearance{}
	state, err := appearance.Snapshot(context.Background(), "SIM-UDID")
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if state != "dark" {
		t.Fatalf("Snapshot() state = %q, want dark", state)
	}
	if err := appearance.Set(context.Background(), "SIM-UDID", "light"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := appearance.Restore(context.Background(), "SIM-UDID", "dark"); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read xcrun log: %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{
		"simctl ui SIM-UDID appearance",
		"simctl ui SIM-UDID appearance light",
		"simctl ui SIM-UDID appearance dark",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("xcrun argv = %v, want %v", got, want)
	}
	if strings.Contains(string(data), "spawn") || strings.Contains(string(data), "defaults") || strings.Contains(string(data), "AppleInterfaceStyle") {
		t.Fatalf("appearance used unsupported command: %s", data)
	}
}

func TestSimctlMatrixAppearanceBoundsCapturedOutput(t *testing.T) {
	skipWindowsUnixExecutableFixtures(t)
	binDir := t.TempDir()
	xcrunPath := filepath.Join(binDir, "xcrun")
	script := "#!/bin/sh\nexec /usr/bin/head -c " + fmt.Sprint(maxMatrixAppearanceBytes+1) + " /dev/zero\n"
	if err := os.WriteFile(xcrunPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write xcrun fixture: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := (simctlMatrixAppearance{}).Snapshot(context.Background(), "SIM-UDID")
	if err == nil || !strings.Contains(err.Error(), "output size limit") {
		t.Fatalf("Snapshot() error = %v, want bounded-output error", err)
	}
}

func TestMatrixReviewSanitizerClearsSuccessfulCellFailureMetadata(t *testing.T) {
	dir := t.TempDir()
	result := &MatrixResult{Cells: []MatrixCellResult{{
		ID: "phone|en-US|light|default", Status: MatrixCellSuccess,
		FailureStage: "execution", FailureCode: "stale_failure",
		Error: &MatrixCellError{Stage: "execution", Code: "stale_failure", Message: "capture failed"},
	}}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: dir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	manifest, err := LoadMatrixReviewManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("LoadMatrixReviewManifest() error = %v", err)
	}
	cell := manifest.Cells[0]
	if cell.FailureStage != "" || cell.FailureCode != "" || cell.Error != nil {
		t.Fatalf("successful cell retained failure metadata: %+v", cell)
	}
}

func TestRenderMatrixReviewURLEncodesArtifactPaths(t *testing.T) {
	dir := t.TempDir()
	rawPath := filepath.Join(dir, "raw screenshots", "home image#1?.png")
	result := &MatrixResult{RawDir: filepath.Join(dir, "raw screenshots"), Cells: []MatrixCellResult{{
		ID: "phone|en-US|light|default", Status: MatrixCellSuccess,
		RawPaths: []string{rawPath},
	}}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: dir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	html := string(data)
	for _, want := range []string{`href="raw%20screenshots/home%20image%231%3F.png"`, `src="raw%20screenshots/home%20image%231%3F.png"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("HTML missing URL-encoded artifact path %q: %s", want, html)
		}
	}
	if strings.Contains(html, `href="raw screenshots/home image#1?.png"`) {
		t.Fatalf("HTML contains unescaped artifact path: %s", html)
	}
}

func TestRenderMatrixReview_ContainsFailedCellsAndEscapesLabels(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := &MatrixResult{
		Cells: []MatrixCellResult{{
			ID:         "phone|en-US|light|default",
			Device:     "phone",
			Locale:     "en-US",
			Appearance: "light",
			Content:    "<default>",
			Status:     MatrixCellSuccess,
		}, {
			ID:           "phone|ja-JP|dark|empty",
			Device:       "phone",
			Locale:       "ja-JP",
			Appearance:   "dark",
			Content:      "empty",
			Status:       MatrixCellFailed,
			FailureStage: "raw command output /private/keychain",
			FailureCode:  "raw command output /private/keychain",
			Error:        &MatrixCellError{Stage: "execution", Code: "plan_failed", Message: "capture failed"},
		}},
	}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: dir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	html := string(data)
	if !strings.Contains(html, "phone|ja-JP|dark|empty") || !strings.Contains(html, "matrix execution failed") || !strings.Contains(html, "missing image") {
		t.Fatalf("HTML omitted cell status: %s", html)
	}
	if strings.Contains(html, "<default>") || !strings.Contains(html, "&lt;default&gt;") {
		t.Fatalf("HTML did not escape content label: %s", html)
	}
	manifest, err := LoadMatrixReviewManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("LoadMatrixReviewManifest() error = %v", err)
	}
	if len(manifest.Cells) != 2 || manifest.Cells[1].Status != MatrixCellFailed {
		t.Fatalf("manifest cells = %+v", manifest.Cells)
	}
	manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	for _, field := range []string{`"generatedAt"`, `"planPath"`, `"totalCells"`, `"contentVariant"`} {
		if !strings.Contains(string(manifestData), field) {
			t.Fatalf("manifest missing governed camelCase field %s: %s", field, manifestData)
		}
	}
	for _, field := range []string{`"generated_at"`, `"plan_path"`, `"total_cells"`, `"content_variant"`} {
		if strings.Contains(string(manifestData), field) {
			t.Fatalf("manifest contains legacy snake_case field %s: %s", field, manifestData)
		}
	}
	if strings.Contains(string(manifestData), "/private/keychain") {
		t.Fatalf("manifest leaked unsanitized failure fields: %s", manifestData)
	}
}

func TestGenerateMatrixReview_DoesNotReplaceManifestWhenHTMLPublishFails(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	oldManifest := []byte(`{"status":"previous"}
`)
	if err := os.WriteFile(manifestPath, oldManifest, 0o644); err != nil {
		t.Fatalf("write previous manifest: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "index.html"), 0o755); err != nil {
		t.Fatalf("create blocked HTML destination: %v", err)
	}

	_, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
		Result:    &MatrixResult{Cells: []MatrixCellResult{{ID: "phone|en-US|light|default", Status: MatrixCellSuccess}}},
		OutputDir: dir,
	})
	if err == nil || !strings.Contains(err.Error(), "matrix review HTML") {
		t.Fatalf("GenerateMatrixReview() error = %v, want HTML write failure", err)
	}
	got, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read previous manifest: %v", err)
	}
	if string(got) != string(oldManifest) {
		t.Fatalf("manifest changed after HTML publish failure: %q", got)
	}
}

func TestGenerateMatrixReviewPreservesExistingFileModes(t *testing.T) {
	skipWindowsUnixFileModes(t)
	dir := t.TempDir()
	for _, name := range []string{"index.html", "manifest.json"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("previous"), 0o600); err != nil {
			t.Fatalf("write previous %s: %v", name, err)
		}
	}

	_, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
		Result:    &MatrixResult{Cells: []MatrixCellResult{{ID: "phone|en-US|light|default", Status: MatrixCellSuccess}}},
		OutputDir: dir,
	})
	if err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	for _, name := range []string{"index.html", "manifest.json"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("stat generated %s: %v", name, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("generated %s mode = %o, want preserved 600", name, got)
		}
	}
}

func TestGenerateMatrixReview_RollsBackPairWhenManifestPublishFails(t *testing.T) {
	skipWindowsUnixFileModes(t)
	dir := t.TempDir()
	oldHTML := []byte("<html>previous</html>\n")
	oldManifest := []byte("{\"status\":\"previous\"}\n")
	if err := os.WriteFile(filepath.Join(dir, "index.html"), oldHTML, 0o600); err != nil {
		t.Fatalf("write previous HTML: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), oldManifest, 0o600); err != nil {
		t.Fatalf("write previous manifest: %v", err)
	}

	write := func(root rootfs.Root, name string, data []byte, perm os.FileMode) error {
		if name == "manifest.json" {
			return errors.New("injected manifest publication failure")
		}
		return root.WriteFilePreservingMode(name, data, perm)
	}
	_, err := generateMatrixReviewWithWriter(context.Background(), MatrixReviewRequest{
		Result:    &MatrixResult{Cells: []MatrixCellResult{{ID: "phone|en-US|light|default", Status: MatrixCellSuccess}}},
		OutputDir: dir,
	}, write)
	if err == nil || !strings.Contains(err.Error(), "injected manifest publication failure") {
		t.Fatalf("generateMatrixReviewWithWriter() error = %v, want injected manifest failure", err)
	}
	gotHTML, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("read HTML after rollback: %v", err)
	}
	gotManifest, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest after rollback: %v", err)
	}
	if string(gotHTML) != string(oldHTML) || string(gotManifest) != string(oldManifest) {
		t.Fatalf("review pair changed after rollback: HTML=%q manifest=%q", gotHTML, gotManifest)
	}
	for _, name := range []string{"index.html", "manifest.json"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("stat rolled-back %s: %v", name, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("rolled-back %s mode = %o, want preserved 600", name, got)
		}
	}
}

func TestGenerateMatrixReview_LockReleaseFailureKeepsSuccessfulResult(t *testing.T) {
	dir := t.TempDir()
	previous := matrixReviewLockReleaseErrForTest
	matrixReviewLockReleaseErrForTest = errors.New("injected review lock release failure")
	t.Cleanup(func() { matrixReviewLockReleaseErrForTest = previous })
	result, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
		Result:    &MatrixResult{Cells: []MatrixCellResult{{ID: "phone|en-US|light|default", Status: MatrixCellSuccess}}},
		OutputDir: dir,
	})
	if err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v, want successful generation after cleanup-only unlock failure", err)
	}
	if result == nil {
		t.Fatal("GenerateMatrixReview() result = nil")
	}
	manifest, err := LoadMatrixReviewManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("LoadMatrixReviewManifest() error = %v", err)
	}
	if manifest.Status != MatrixCellSuccess {
		t.Fatalf("published review status = %q, want success to match the returned generation", manifest.Status)
	}
}

func TestGenerateMatrixReviewSerializesPublicationOfEachPair(t *testing.T) {
	dir := t.TempDir()
	firstHTMLPublished := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondWriterEntered := make(chan struct{}, 1)
	firstResult := &MatrixResult{PlanPath: "first.json", Cells: []MatrixCellResult{{ID: "generation-first", Status: MatrixCellSuccess}}}
	secondResult := &MatrixResult{PlanPath: "second.json", Cells: []MatrixCellResult{{ID: "generation-second", Status: MatrixCellSuccess}}}

	firstDone := make(chan error, 1)
	go func() {
		_, err := generateMatrixReviewWithWriter(context.Background(), MatrixReviewRequest{Result: firstResult, OutputDir: dir}, func(root rootfs.Root, name string, data []byte, perm os.FileMode) error {
			if err := root.WriteFilePreservingMode(name, data, perm); err != nil {
				return err
			}
			if name == "index.html" {
				close(firstHTMLPublished)
				<-releaseFirst
			}
			return nil
		})
		firstDone <- err
	}()
	<-firstHTMLPublished

	secondDone := make(chan error, 1)
	go func() {
		_, err := generateMatrixReviewWithWriter(context.Background(), MatrixReviewRequest{Result: secondResult, OutputDir: dir}, func(root rootfs.Root, name string, data []byte, perm os.FileMode) error {
			select {
			case secondWriterEntered <- struct{}{}:
			default:
			}
			return root.WriteFilePreservingMode(name, data, perm)
		})
		secondDone <- err
	}()

	select {
	case <-secondWriterEntered:
		t.Fatal("second review writer entered while the first pair was only partially published")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first GenerateMatrixReview() error = %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second GenerateMatrixReview() error = %v", err)
	}

	manifest, err := LoadMatrixReviewManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("LoadMatrixReviewManifest() error = %v", err)
	}
	htmlData, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("ReadFile(index.html) error = %v", err)
	}
	if len(manifest.Cells) != 1 || manifest.Cells[0].ID != "generation-second" || !bytes.Contains(htmlData, []byte("generation-second")) {
		t.Fatalf("published pair does not describe the same final generation: manifest=%+v html=%q", manifest.Cells, htmlData)
	}
}

func TestMatrixReviewConsumersRejectMixedHTMLAndManifest(t *testing.T) {
	dir := t.TempDir()
	result := &MatrixResult{PlanPath: "plan.json", Cells: []MatrixCellResult{{ID: "generation", Status: MatrixCellSuccess}}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: dir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	manifest, err := LoadMatrixReviewManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadMatrixReviewManifest() error = %v", err)
	}
	if len(manifest.HTMLSHA256) != sha256.Size*2 {
		t.Fatalf("HTMLSHA256 = %q, want a SHA-256 hex digest", manifest.HTMLSHA256)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("different generation"), 0o644); err != nil {
		t.Fatalf("WriteFile(index.html) error = %v", err)
	}
	if _, err := LoadMatrixReviewManifest(manifestPath); err == nil || strings.Contains(err.Error(), dir) {
		t.Fatalf("LoadMatrixReviewManifest() error = %v, want stable mixed-generation rejection", err)
	}
	if _, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: dir, DryRun: true}); err == nil || strings.Contains(err.Error(), dir) {
		t.Fatalf("OpenReview() error = %v, want stable mixed-generation rejection", err)
	}
}

func TestLoadMatrixReviewManifestRejectsSameBytesFromReplacedReviewRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	dir := t.TempDir()
	reviewDir := filepath.Join(dir, "review")
	if err := os.Mkdir(reviewDir, 0o755); err != nil {
		t.Fatalf("create review directory: %v", err)
	}
	result := &MatrixResult{PlanPath: "plan.json", Cells: []MatrixCellResult{{ID: "generation", Status: MatrixCellSuccess}}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: reviewDir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	replacementDir := filepath.Join(dir, "replacement")
	if err := os.Mkdir(replacementDir, 0o755); err != nil {
		t.Fatalf("create replacement review directory: %v", err)
	}
	for _, name := range []string{"manifest.json", "index.html"} {
		data, err := os.ReadFile(filepath.Join(reviewDir, name))
		if err != nil {
			t.Fatalf("read initial %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(replacementDir, name), data, 0o644); err != nil {
			t.Fatalf("write replacement %s: %v", name, err)
		}
	}
	originalDir := filepath.Join(dir, "review-original")
	previous := matrixReviewManifestLoadedForTest
	matrixReviewManifestLoadedForTest = func() {
		if err := os.Rename(reviewDir, originalDir); err != nil {
			t.Errorf("rename original review directory: %v", err)
			return
		}
		if err := os.Rename(replacementDir, reviewDir); err != nil {
			t.Errorf("install replacement review directory: %v", err)
		}
	}
	t.Cleanup(func() {
		matrixReviewManifestLoadedForTest = previous
		if info, err := os.Lstat(reviewDir); err == nil && info.IsDir() {
			_ = os.RemoveAll(reviewDir)
		}
		if info, err := os.Lstat(originalDir); err == nil && info.IsDir() {
			_ = os.RemoveAll(originalDir)
		}
	})
	manifest, err := LoadMatrixReviewManifest(filepath.Join(reviewDir, "manifest.json"))
	if err != nil {
		t.Fatalf("LoadMatrixReviewManifest() error = %v, want the pinned original pair", err)
	}
	if manifest == nil || manifest.HTMLSHA256 == "" {
		t.Fatalf("LoadMatrixReviewManifest() result = %+v, want validated pinned manifest", manifest)
	}
}

func TestOpenReviewConsumesValidatedHTMLSnapshotAfterPathReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	dir := t.TempDir()
	reviewDir := filepath.Join(dir, "review")
	if err := os.Mkdir(reviewDir, 0o755); err != nil {
		t.Fatalf("create review directory: %v", err)
	}
	result := &MatrixResult{PlanPath: "plan.json", Cells: []MatrixCellResult{{ID: "generation", Status: MatrixCellSuccess}}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: reviewDir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	htmlPath := filepath.Join(reviewDir, "index.html")
	expectedHTML, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("read generated HTML: %v", err)
	}
	replacementDir := filepath.Join(dir, "replacement")
	if err := os.Mkdir(replacementDir, 0o755); err != nil {
		t.Fatalf("create replacement review directory: %v", err)
	}
	for _, name := range []string{"manifest.json", "index.html"} {
		data, readErr := os.ReadFile(filepath.Join(reviewDir, name))
		if readErr != nil {
			t.Fatalf("read initial %s: %v", name, readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(replacementDir, name), data, 0o644); writeErr != nil {
			t.Fatalf("write replacement %s: %v", name, writeErr)
		}
	}
	originalDir := filepath.Join(dir, "review-original")
	previousValidated := matrixReviewSnapshotValidatedForTest
	previousOpen := matrixReviewOpenPathForTest
	var snapshotPath string
	matrixReviewSnapshotValidatedForTest = func(path string) {
		if err := os.Rename(filepath.Dir(path), originalDir); err != nil {
			t.Errorf("rename original review directory: %v", err)
			return
		}
		if err := os.Rename(replacementDir, filepath.Dir(path)); err != nil {
			t.Errorf("install replacement review directory: %v", err)
		}
	}
	matrixReviewOpenPathForTest = func(path string) error {
		snapshotPath = path
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(got, expectedHTML) {
			return fmt.Errorf("browser snapshot differs from validated HTML")
		}
		return nil
	}
	t.Cleanup(func() {
		matrixReviewSnapshotValidatedForTest = previousValidated
		matrixReviewOpenPathForTest = previousOpen
		if snapshotPath != "" {
			removeMatrixReviewBrowserSnapshot(snapshotPath)
		}
		_ = os.RemoveAll(reviewDir)
		_ = os.RemoveAll(originalDir)
	})
	opened, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: reviewDir})
	if err != nil {
		t.Fatalf("OpenReview() error = %v", err)
	}
	if opened == nil || opened.HTMLPath != htmlPath || !opened.Opened {
		t.Fatalf("OpenReview() result = %+v, want original path with browser open", opened)
	}
	if snapshotPath == "" || snapshotPath == htmlPath {
		t.Fatalf("browser path = %q, want retained snapshot path", snapshotPath)
	}
	if info, statErr := os.Stat(filepath.Dir(snapshotPath)); statErr != nil {
		t.Fatalf("browser snapshot directory stat error = %v", statErr)
	} else if info.Mode().Perm() != 0o700 {
		t.Fatalf("browser snapshot directory mode = %v, want owner-only 0700", info.Mode())
	}
}

func TestOpenReviewConsumesValidatedHTMLAndAssetsSnapshotAfterPathReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	dir := t.TempDir()
	reviewDir := filepath.Join(dir, "review")
	rawDir := filepath.Join(dir, "raw")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("create review directory: %v", err)
	}
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	rawPath := filepath.Join(rawDir, "home.png")
	writeMinimalPNG(t, rawPath, 200, 300)
	expectedAsset, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read expected raw asset: %v", err)
	}
	result := &MatrixResult{
		RawDir: rawDir,
		Cells: []MatrixCellResult{{
			ID:       "cell",
			Status:   MatrixCellSuccess,
			RawPaths: []string{rawPath},
		}},
	}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: reviewDir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	htmlPath := filepath.Join(reviewDir, "index.html")
	originalReviewDir := filepath.Join(dir, "review-original")
	originalRawDir := filepath.Join(dir, "raw-original")
	previousValidated := matrixReviewSnapshotValidatedForTest
	previousOpen := matrixReviewOpenPathForTest
	var snapshotPath string
	matrixReviewSnapshotValidatedForTest = func(path string) {
		if err := os.Rename(filepath.Dir(path), originalReviewDir); err != nil {
			t.Errorf("rename original review directory: %v", err)
			return
		}
		if err := os.Mkdir(reviewDir, 0o755); err != nil {
			t.Errorf("replace review directory: %v", err)
		}
		if err := os.Rename(rawDir, originalRawDir); err != nil {
			t.Errorf("rename original raw directory: %v", err)
			return
		}
		if err := os.Mkdir(rawDir, 0o755); err != nil {
			t.Errorf("replace raw directory: %v", err)
			return
		}
		if err := os.WriteFile(rawPath, []byte("replacement asset"), 0o600); err != nil {
			t.Errorf("write replacement raw asset: %v", err)
		}
	}
	matrixReviewOpenPathForTest = func(path string) error {
		snapshotPath = path
		htmlData, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(htmlData, []byte("assets/000000.png")) {
			return errors.New("browser snapshot omitted rewritten asset link")
		}
		asset, err := os.ReadFile(filepath.Join(filepath.Dir(path), "assets", "000000.png"))
		if err != nil {
			return err
		}
		if !bytes.Equal(asset, expectedAsset) {
			return errors.New("browser snapshot asset differs from validated bytes")
		}
		return nil
	}
	t.Cleanup(func() {
		matrixReviewSnapshotValidatedForTest = previousValidated
		matrixReviewOpenPathForTest = previousOpen
		if snapshotPath != "" {
			removeMatrixReviewBrowserSnapshot(snapshotPath)
		}
		_ = os.RemoveAll(reviewDir)
		_ = os.RemoveAll(originalReviewDir)
		_ = os.RemoveAll(rawDir)
		_ = os.RemoveAll(originalRawDir)
	})
	opened, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: reviewDir})
	if err != nil {
		t.Fatalf("OpenReview() error = %v", err)
	}
	if opened == nil || opened.HTMLPath != htmlPath || !opened.Opened {
		t.Fatalf("OpenReview() result = %+v, want original path with browser open", opened)
	}
	if snapshotPath == "" {
		t.Fatal("browser hook did not receive a snapshot path")
	}
}

func TestOpenMatrixReviewRejectsPublishedHTMLWithoutBoundManifest(t *testing.T) {
	for _, test := range []struct {
		name       string
		manifest   []byte
		removeFile bool
	}{
		{name: "manifest missing after HTML publication", removeFile: true},
		{name: "legacy manifest left beside new HTML", manifest: []byte(`{"planPath":"old.json"}`)},
		{name: "malformed manifest left beside new HTML", manifest: []byte(`{"htmlSha256":`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			result := &MatrixResult{PlanPath: "new.json", Cells: []MatrixCellResult{{ID: "new-generation", Status: MatrixCellSuccess}}}
			if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: dir}); err != nil {
				t.Fatalf("GenerateMatrixReview() error = %v", err)
			}
			manifestPath := filepath.Join(dir, "manifest.json")
			if test.removeFile {
				if err := os.Remove(manifestPath); err != nil {
					t.Fatalf("Remove(manifest.json) error = %v", err)
				}
			} else if err := os.WriteFile(manifestPath, test.manifest, 0o644); err != nil {
				t.Fatalf("WriteFile(manifest.json) error = %v", err)
			}
			if _, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: dir, DryRun: true}); err == nil || strings.Contains(err.Error(), dir) {
				t.Fatalf("OpenReview() error = %v, want stable unbound-generation rejection", err)
			}
			if !test.removeFile {
				if _, err := LoadMatrixReviewManifest(manifestPath); err == nil || strings.Contains(err.Error(), dir) {
					t.Fatalf("LoadMatrixReviewManifest() error = %v, want stable unbound-generation rejection", err)
				}
			}
		})
	}
}

func TestAcquireMatrixReviewLockHonorsCancellationWhileWaiting(t *testing.T) {
	root, err := openMatrixOutputRoot(t.TempDir())
	if err != nil {
		t.Fatalf("openMatrixOutputRoot() error = %v", err)
	}
	defer root.Close()
	release, err := acquireMatrixReviewLock(context.Background(), root)
	if err != nil {
		t.Fatalf("first acquireMatrixReviewLock() error = %v", err)
	}
	defer func() {
		if err := release(); err != nil {
			t.Errorf("release matrix review lock: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = acquireMatrixReviewLock(ctx, root)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second acquireMatrixReviewLock() error = %v, want context deadline", err)
	}
}

func TestLoadMatrixReviewManifestRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", maxMatrixReviewBytes+1)), 0o644); err != nil {
		t.Fatalf("write oversized matrix review manifest: %v", err)
	}
	_, err := LoadMatrixReviewManifest(path)
	if err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("LoadMatrixReviewManifest() error = %v, want bounded-size read error", err)
	}
}

func TestValidateMatrixReviewSizeUsesInclusiveByteBoundary(t *testing.T) {
	within := bytes.Repeat([]byte{'x'}, maxMatrixReviewBytes)
	if err := validateMatrixReviewSize("manifest", within); err != nil {
		t.Fatalf("validateMatrixReviewSize() at limit = %v, want nil", err)
	}
	withTerminator := append(append([]byte(nil), within...), '\n')
	if err := validateMatrixReviewSize("manifest", withTerminator); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("validateMatrixReviewSize() with newline = %v, want size-limit error", err)
	}
}

func TestGenerateMatrixReviewRejectsOversizedHTMLBeforePublish(t *testing.T) {
	dir := t.TempDir()
	oldHTML := []byte("<html>previous</html>\n")
	oldManifest := []byte(`{"status":"previous"}` + "\n")
	if err := os.WriteFile(filepath.Join(dir, "index.html"), oldHTML, 0o600); err != nil {
		t.Fatalf("write previous HTML: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), oldManifest, 0o600); err != nil {
		t.Fatalf("write previous manifest: %v", err)
	}

	_, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
		Result: &MatrixResult{Cells: []MatrixCellResult{{
			ID:         strings.Repeat("x", maxMatrixReviewBytes),
			Device:     "phone",
			Locale:     "en-US",
			Appearance: "light",
			Content:    "default",
			Status:     MatrixCellSuccess,
		}}},
		OutputDir: dir,
	})
	if err == nil || !strings.Contains(err.Error(), "matrix review HTML") || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("GenerateMatrixReview() error = %v, want bounded HTML-size failure", err)
	}
	gotHTML, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("read previous HTML: %v", err)
	}
	gotManifest, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read previous manifest: %v", err)
	}
	if string(gotHTML) != string(oldHTML) || string(gotManifest) != string(oldManifest) {
		t.Fatalf("review pair changed after oversized HTML: HTML=%q manifest=%q", gotHTML, gotManifest)
	}
}

func TestGenerateMatrixReviewRejectsOversizedManifestBeforePublish(t *testing.T) {
	dir := t.TempDir()
	oldHTML := []byte("<html>previous</html>\n")
	oldManifest := []byte(`{"status":"previous"}` + "\n")
	if err := os.WriteFile(filepath.Join(dir, "index.html"), oldHTML, 0o600); err != nil {
		t.Fatalf("write previous HTML: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), oldManifest, 0o600); err != nil {
		t.Fatalf("write previous manifest: %v", err)
	}

	_, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
		Result: &MatrixResult{
			PlanPath: strings.Repeat("p", maxMatrixReviewBytes),
			Cells:    []MatrixCellResult{{ID: "phone|en-US|light|default", Status: MatrixCellSuccess}},
		},
		OutputDir: dir,
	})
	if err == nil || !strings.Contains(err.Error(), "matrix review manifest") || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("GenerateMatrixReview() error = %v, want bounded manifest-size failure", err)
	}
	gotHTML, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("read previous HTML: %v", err)
	}
	gotManifest, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read previous manifest: %v", err)
	}
	if string(gotHTML) != string(oldHTML) || string(gotManifest) != string(oldManifest) {
		t.Fatalf("review pair changed after oversized manifest: HTML=%q manifest=%q", gotHTML, gotManifest)
	}
}

func TestLoadMatrixReviewManifestRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	link := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(target, []byte(`{"status":"success"}`), 0o644); err != nil {
		t.Fatalf("write target matrix review manifest: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create matrix review manifest symlink: %v", err)
	}
	_, err := LoadMatrixReviewManifest(link)
	if err == nil || !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("LoadMatrixReviewManifest() error = %v, want symlink rejection", err)
	}
}

func TestPromoteMatrixArtifactRejectsParentSymlink(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "raw")
	outsideDir := filepath.Join(dir, "outside")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	source := filepath.Join(outputDir, "source.png")
	writeMatrixPNG(t, source)
	parentLink := filepath.Join(outputDir, "nested")
	if err := os.Symlink(outsideDir, parentLink); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}
	root, err := rootfs.New(outputDir)
	if err != nil {
		t.Fatalf("open output root: %v", err)
	}
	defer root.Close()

	err = promoteMatrixArtifact(root, outputDir, source, filepath.Join(parentLink, "result.png"))
	if err == nil || !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("promoteMatrixArtifact() error = %v, want parent symlink rejection", err)
	}
	if _, err := os.Stat(filepath.Join(outsideDir, "result.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside destination was touched: %v", err)
	}
}

func TestPromoteMatrixArtifactRejectsFinalSymlink(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "raw")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	source := filepath.Join(outputDir, "source.png")
	writeMatrixPNG(t, source)
	target := filepath.Join(dir, "outside.png")
	if err := os.WriteFile(target, []byte("outside"), 0o644); err != nil {
		t.Fatalf("write outside target: %v", err)
	}
	destination := filepath.Join(outputDir, "result.png")
	if err := os.Symlink(target, destination); err != nil {
		t.Fatalf("create final symlink: %v", err)
	}
	root, err := rootfs.New(outputDir)
	if err != nil {
		t.Fatalf("open output root: %v", err)
	}
	defer root.Close()

	err = promoteMatrixArtifact(root, outputDir, source, destination)
	if err == nil || !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("promoteMatrixArtifact() error = %v, want final symlink rejection", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read outside target: %v", err)
	}
	if string(got) != "outside" {
		t.Fatalf("outside target changed: %q", got)
	}
}

func TestPromoteMatrixArtifactPreservesExistingMode(t *testing.T) {
	skipWindowsUnixFileModes(t)
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "raw")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	source := filepath.Join(outputDir, "source.png")
	destination := filepath.Join(outputDir, "result.png")
	writeMatrixPNG(t, source)
	if err := os.WriteFile(destination, []byte("previous"), 0o600); err != nil {
		t.Fatalf("write existing destination: %v", err)
	}
	root, err := rootfs.New(outputDir)
	if err != nil {
		t.Fatalf("open output root: %v", err)
	}
	defer root.Close()

	if err := promoteMatrixArtifact(root, outputDir, source, destination); err != nil {
		t.Fatalf("promoteMatrixArtifact() error = %v", err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatalf("stat promoted destination: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("promoted destination mode = %o, want preserved 600", got)
	}
}

func TestExecuteMatrixCellPreservesPartiallyPromotedFramedPaths(t *testing.T) {
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	framedDir := filepath.Join(dir, "framed")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	if err := os.MkdirAll(framedDir, 0o755); err != nil {
		t.Fatalf("create framed directory: %v", err)
	}
	cell := MatrixCell{
		ID:         "phone|en-US|light|default",
		Device:     "phone",
		UDID:       "SIM-UDID",
		Locale:     "en-US",
		Appearance: "light",
		Content:    "default",
		RawDir:     filepath.Join(rawDir, "en-US", "phone", "light", "default"),
		FramedDir:  filepath.Join(framedDir, "en-US", "phone", "light", "default"),
		RawPaths: []string{
			filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png"),
			filepath.Join(rawDir, "en-US", "phone", "light", "default", "details.png"),
		},
		FramedPaths: []string{
			filepath.Join(framedDir, "en-US", "phone", "light", "default", "home.png"),
			filepath.Join(framedDir, "en-US", "phone", "light", "default", "details.png"),
		},
	}
	if err := os.MkdirAll(filepath.Dir(cell.FramedPaths[1]), 0o755); err != nil {
		t.Fatalf("create framed cell directory: %v", err)
	}
	if err := os.Mkdir(cell.FramedPaths[1], 0o755); err != nil {
		t.Fatalf("create blocked second framed destination: %v", err)
	}
	rawRoot, err := rootfs.New(rawDir)
	if err != nil {
		t.Fatalf("open raw root: %v", err)
	}
	defer rawRoot.Close()
	framedRoot, err := rootfs.New(framedDir)
	if err != nil {
		t.Fatalf("open framed root: %v", err)
	}
	defer framedRoot.Close()
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{
		{Action: ActionScreenshot, Name: stringPtr("home")},
		{Action: ActionScreenshot, Name: stringPtr("details")},
	}}
	matrixPlan := &MatrixPlan{Output: MatrixOutput{Frame: MatrixFrame{Enabled: true, DeviceByMatrixDevice: map[string]string{"phone": "iphone-17-pro"}}}}
	deps := MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "details.png"))
			return &RunResult{}, nil
		},
		Frame: func(_ context.Context, request FrameRequest) (*FrameResult, error) {
			writeMatrixPNG(t, request.OutputPath)
			return &FrameResult{}, nil
		},
		Appearance: &matrixTestAppearance{},
	}
	result := executeMatrixCell(context.Background(), cell, base, matrixPlan, 1, 0, deps, matrixOutputRoots{
		raw: rawRoot, rawPath: rawDir, framed: framedRoot, framedPath: framedDir, hasFramed: true,
	}, &matrixSimulatorGuard{})
	if result.Status != MatrixCellFailed || result.FailureCode != "framed_output_failed" {
		t.Fatalf("result = %+v, want framed promotion failure", result)
	}
	if len(result.FramedPaths) != 1 || result.FramedPaths[0] != cell.FramedPaths[0] {
		t.Fatalf("framed paths = %v, want first promoted path %q", result.FramedPaths, cell.FramedPaths[0])
	}
}

func TestExecuteMatrixCellPreservesFullScreenshotRecordsAfterPartialRawPromotion(t *testing.T) {
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	cell := MatrixCell{
		ID:         "phone|en-US|light|default",
		Device:     "phone",
		UDID:       "SIM-UDID",
		Locale:     "en-US",
		Appearance: "light",
		Content:    "default",
		RawDir:     filepath.Join(rawDir, "en-US", "phone", "light", "default"),
		RawPaths: []string{
			filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png"),
			filepath.Join(rawDir, "en-US", "phone", "light", "default", "details.png"),
		},
	}
	if err := os.MkdirAll(filepath.Dir(cell.RawPaths[1]), 0o755); err != nil {
		t.Fatalf("create raw cell directory: %v", err)
	}
	if err := os.Mkdir(cell.RawPaths[1], 0o755); err != nil {
		t.Fatalf("create blocked second raw destination: %v", err)
	}
	rawRoot, err := rootfs.New(rawDir)
	if err != nil {
		t.Fatalf("open raw root: %v", err)
	}
	defer rawRoot.Close()
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{
		{Action: ActionScreenshot, Name: stringPtr("home")},
		{Action: ActionScreenshot, Name: stringPtr("details")},
	}}
	matrixPlan := &MatrixPlan{}
	deps := MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "details.png"))
			return &RunResult{}, nil
		},
		Appearance: &matrixTestAppearance{},
	}
	result := executeMatrixCell(context.Background(), cell, base, matrixPlan, 1, 0, deps, matrixOutputRoots{raw: rawRoot, rawPath: rawDir}, &matrixSimulatorGuard{})
	if result.Status != MatrixCellFailed || result.FailureCode != "raw_output_failed" {
		t.Fatalf("result = %+v, want raw promotion failure", result)
	}
	if len(result.Screenshots) != len(cell.RawPaths) {
		t.Fatalf("screenshots = %d, want one record per requested path: %+v", len(result.Screenshots), result.Screenshots)
	}
	if got := result.Screenshots[0].RawPath; got != cell.RawPaths[0] {
		t.Fatalf("first screenshot raw path = %q, want %q", got, cell.RawPaths[0])
	}
	if got := result.Screenshots[1].Name; got != "details" {
		t.Fatalf("missing screenshot record name = %q, want details", got)
	}
	if got := result.Screenshots[1].RawPath; got != "" {
		t.Fatalf("missing screenshot raw path = %q, want empty", got)
	}
}

func TestExecuteMatrixCellUnionsFramedPathsAcrossShorterRetry(t *testing.T) {
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	framedDir := filepath.Join(dir, "framed")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	if err := os.MkdirAll(framedDir, 0o755); err != nil {
		t.Fatalf("create framed directory: %v", err)
	}
	cell := MatrixCell{
		ID:         "phone|en-US|light|default",
		Device:     "phone",
		UDID:       "SIM-UDID",
		Locale:     "en-US",
		Appearance: "light",
		Content:    "default",
		RawDir:     filepath.Join(rawDir, "en-US", "phone", "light", "default"),
		FramedDir:  filepath.Join(framedDir, "en-US", "phone", "light", "default"),
		RawPaths: []string{
			filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png"),
			filepath.Join(rawDir, "en-US", "phone", "light", "default", "details.png"),
			filepath.Join(rawDir, "en-US", "phone", "light", "default", "settings.png"),
		},
		FramedPaths: []string{
			filepath.Join(framedDir, "en-US", "phone", "light", "default", "home.png"),
			filepath.Join(framedDir, "en-US", "phone", "light", "default", "details.png"),
			filepath.Join(framedDir, "en-US", "phone", "light", "default", "settings.png"),
		},
	}
	if err := os.MkdirAll(filepath.Dir(cell.FramedPaths[2]), 0o755); err != nil {
		t.Fatalf("create framed cell directory: %v", err)
	}
	if err := os.Mkdir(cell.FramedPaths[2], 0o755); err != nil {
		t.Fatalf("create blocked third framed destination: %v", err)
	}
	rawRoot, err := rootfs.New(rawDir)
	if err != nil {
		t.Fatalf("open raw root: %v", err)
	}
	defer rawRoot.Close()
	framedRoot, err := rootfs.New(framedDir)
	if err != nil {
		t.Fatalf("open framed root: %v", err)
	}
	defer framedRoot.Close()
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{
		{Action: ActionScreenshot, Name: stringPtr("home")},
		{Action: ActionScreenshot, Name: stringPtr("details")},
		{Action: ActionScreenshot, Name: stringPtr("settings")},
	}}
	matrixPlan := &MatrixPlan{Output: MatrixOutput{Frame: MatrixFrame{Enabled: true, DeviceByMatrixDevice: map[string]string{"phone": "iphone-17-pro"}}}}
	var mu sync.Mutex
	currentAttempt, frameCalls := 0, 0
	deps := MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			mu.Lock()
			currentAttempt++
			frameCalls = 0
			mu.Unlock()
			for _, name := range []string{"home", "details", "settings"} {
				writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, name+".png"))
			}
			return &RunResult{}, nil
		},
		Frame: func(_ context.Context, request FrameRequest) (*FrameResult, error) {
			writeMatrixPNG(t, request.OutputPath)
			mu.Lock()
			frameCalls++
			attempt, call := currentAttempt, frameCalls
			mu.Unlock()
			if attempt == 2 && call == 3 {
				if err := os.Remove(cell.FramedPaths[1]); err != nil {
					return nil, err
				}
				if err := os.Mkdir(cell.FramedPaths[1], 0o755); err != nil {
					return nil, err
				}
			}
			return &FrameResult{}, nil
		},
		Appearance: &matrixTestAppearance{},
	}
	result := executeMatrixCell(context.Background(), cell, base, matrixPlan, 2, 0, deps, matrixOutputRoots{
		raw: rawRoot, rawPath: rawDir, framed: framedRoot, framedPath: framedDir, hasFramed: true,
	}, &matrixSimulatorGuard{})
	if result.Status != MatrixCellFailed || result.FailureCode != "framed_output_failed" {
		t.Fatalf("result = %+v, want framed promotion failure", result)
	}
	if got, want := result.FramedPaths, cell.FramedPaths[:2]; !reflect.DeepEqual(got, want) {
		t.Fatalf("framed paths = %v, want union in declaration order %v", got, want)
	}
	if len(result.Screenshots) != len(cell.FramedPaths) || result.Screenshots[1].FramedPath != cell.FramedPaths[1] {
		t.Fatalf("screenshot metadata lost prior framed path: %+v", result.Screenshots)
	}

	// The retry merge intentionally retains the earlier successful paths so a
	// later partial attempt cannot make the review look complete. Before that
	// metadata is published, however, the retained paths must still be bound to
	// the files that produced them. The second attempt replaced details.png with
	// a directory, so it must be dropped while the valid home.png remains.
	reviewResult := &MatrixResult{
		FramedDir: framedDir,
		ReviewDir: filepath.Join(dir, "review"),
		Cells:     []MatrixCellResult{result},
	}
	if err := revalidateMatrixFramedPaths(reviewResult); err == nil {
		t.Fatal("revalidateMatrixFramedPaths() error = nil, want replaced retry artifact uncertainty")
	}
	if got, want := reviewResult.Cells[0].FramedPaths, cell.FramedPaths[:1]; !reflect.DeepEqual(got, want) {
		t.Fatalf("revalidated framed paths = %v, want valid prior path %v", got, want)
	}
	if got := reviewResult.Cells[0].Screenshots[0].FramedPath; got != cell.FramedPaths[0] {
		t.Fatalf("valid prior screenshot path = %q, want %q", got, cell.FramedPaths[0])
	}
	if got := reviewResult.Cells[0].Screenshots[1].FramedPath; got != "" {
		t.Fatalf("replaced retry screenshot path = %q, want dropped", got)
	}
	if got := reviewResult.Cells[0].Screenshots[1].Status; got != MatrixCellFailed {
		t.Fatalf("replaced retry screenshot status = %q, want failed", got)
	}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: reviewResult, OutputDir: reviewResult.ReviewDir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	manifest, err := LoadMatrixReviewManifest(filepath.Join(reviewResult.ReviewDir, "manifest.json"))
	if err != nil {
		t.Fatalf("LoadMatrixReviewManifest() error = %v", err)
	}
	if len(manifest.Cells) != 1 || !reflect.DeepEqual(manifest.Cells[0].FramedPaths, cell.FramedPaths[:1]) {
		t.Fatalf("review manifest retained replaced retry path: %+v", manifest.Cells)
	}
}

func TestMatrixResultJSONDoesNotPersistSimulatorSecrets(t *testing.T) {
	data, err := json.Marshal(MatrixResult{
		BundleID: "com.example.app",
		Cells:    []MatrixCellResult{{ID: "phone|en-US|light|default", Device: "phone", Status: MatrixCellSuccess}},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	encoded := string(data)
	for _, forbidden := range []string{"UDID", "udid", "launch_arguments", "PRIVATE_KEY"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("result JSON contains %q: %s", forbidden, encoded)
		}
	}
}

func TestRunMatrix_BoundsConcurrencyAndWritesPartialSafeResult(t *testing.T) {
	dir := t.TempDir()
	previousLockBase := matrixGlobalLockBaseDirForTest
	matrixGlobalLockBaseDirForTest = t.TempDir()
	t.Cleanup(func() { matrixGlobalLockBaseDirForTest = previousLockBase })
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	udidPrefix := "UDID-" + filepath.Base(dir)
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"launch"},{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, fmt.Sprintf(`{
  "version":1,"base_plan":"base.json",
  "devices":[
    {"id":"phone-a","udid":%q},
    {"id":"phone-b","udid":%q},
    {"id":"phone-c","udid":%q},
    {"id":"phone-d","udid":%q}
  ],
  "locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],
  "output":{"raw_dir":"raw","review_dir":"review"}
}`, udidPrefix+"-a", udidPrefix+"-b", udidPrefix+"-c", udidPrefix+"-d"))
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	appearance := &matrixTestAppearance{}
	var mu sync.Mutex
	active, maxActive := 0, 0
	runPlan := func(ctx context.Context, plan *Plan) (*RunResult, error) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		defer func() {
			mu.Lock()
			active--
			mu.Unlock()
		}()
		if err := waitContext(ctx, 5*time.Millisecond); err != nil {
			return nil, err
		}
		if err := writeMatrixPNGFile(filepath.Join(plan.App.OutputDir, "home.png")); err != nil {
			return nil, err
		}
		return &RunResult{BundleID: plan.App.BundleID, UDID: plan.App.UDID, OutputDir: plan.App.OutputDir, Steps: []RunStepResult{{Index: 1, Action: "screenshot", Status: "ok"}}}, nil
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{MaxConcurrency: 2}, MatrixDependencies{
		RunPlan:     runPlan,
		Appearance:  appearance,
		CheckDevice: func(context.Context, MatrixDevice) error { return nil },
	})
	if runErr != nil {
		if result != nil {
			for _, cell := range result.Cells {
				t.Logf("cell %s status=%s stage=%s code=%s err=%v", cell.ID, cell.Status, cell.FailureStage, cell.FailureCode, cell.Error)
			}
		}
		t.Fatalf("RunMatrixWithDependencies() error = %v", runErr)
	}
	if maxActive > 2 {
		t.Fatalf("max concurrent runs = %d, want <= 2", maxActive)
	}
	if result.Succeeded != 4 || result.Failed != 0 || len(result.Cells) != 4 {
		t.Fatalf("unexpected result summary: %+v", result)
	}
	if appearance.setCount != 4 || appearance.restoreCount != 4 {
		t.Fatalf("appearance calls = set %d restore %d", appearance.setCount, appearance.restoreCount)
	}
	if _, err := os.Stat(filepath.Join(dir, "review", "manifest.json")); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
}

func TestRunMatrix_OutputLockReleaseFailureMarksResultFailed(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{
  "version":1,"base_plan":"base.json",
  "devices":[{"id":"phone","udid":"UDID"}],
  "locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],
  "output":{"raw_dir":"raw","review_dir":"review"}
}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	releaseErr := errors.New("injected output lock release failure")
	previous := matrixOutputLockReleaseErrForTest
	matrixOutputLockReleaseErrForTest = releaseErr
	t.Cleanup(func() { matrixOutputLockReleaseErrForTest = previous })
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Appearance:  &matrixTestAppearance{},
		CheckDevice: func(context.Context, MatrixDevice) error { return nil },
	})
	if runErr == nil || !errors.Is(runErr, releaseErr) {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want injected release failure", runErr)
	}
	if result == nil || result.Status != MatrixCellFailed {
		t.Fatalf("result status = %+v, want failed after output-lock release error", result)
	}
	if result.Succeeded != 1 || result.Failed != 0 {
		t.Fatalf("cell totals = succeeded %d failed %d, want successful cells with failed run status", result.Succeeded, result.Failed)
	}
	manifest, err := LoadMatrixReviewManifest(filepath.Join(dir, "review", "manifest.json"))
	if err != nil {
		t.Fatalf("LoadMatrixReviewManifest() error = %v", err)
	}
	if manifest.Status != MatrixCellFailed {
		t.Fatalf("published review status = %q, want failed to match the command result", manifest.Status)
	}
}

func TestRunMatrix_HoldsOutputLocksThroughReviewPublish(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	rawDir := filepath.Join(dir, "raw")
	reviewDir := filepath.Join(dir, "review")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{
  "version":1,"base_plan":"base.json",
  "devices":[{"id":"phone","udid":"UDID-LOCK-HOLD"}],
  "locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],
  "output":{"raw_dir":"raw","review_dir":"review"}
}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	previous := matrixBeforeReviewPublishForTest
	matrixBeforeReviewPublishForTest = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
		defer cancel()
		release, lockErr := acquireMatrixOutputLocks(ctx, []string{rawDir, reviewDir})
		if release != nil {
			if releaseErr := release(); releaseErr != nil {
				t.Errorf("release competing output lock: %v", releaseErr)
			}
			t.Errorf("acquired output locks during review publish")
			return
		}
		if !errors.Is(lockErr, context.DeadlineExceeded) {
			t.Errorf("competing output lock error = %v, want deadline while the run still holds them", lockErr)
		}
	}
	t.Cleanup(func() { matrixBeforeReviewPublishForTest = previous })
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Appearance:  &matrixTestAppearance{},
		CheckDevice: func(context.Context, MatrixDevice) error { return nil },
	})
	if runErr != nil {
		t.Fatalf("RunMatrixWithDependencies() error = %v", runErr)
	}
	if result == nil || result.Status != MatrixCellSuccess || result.Review == nil {
		t.Fatalf("result = %+v, want successful review publication while output locks were held", result)
	}
}

func TestRunMatrix_ReviewRootReplacementFailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows temporary volumes do not reliably distinguish a replaced review directory identity")
	}
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	reviewDir := filepath.Join(dir, "review")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{
  "version":1,"base_plan":"base.json",
  "devices":[{"id":"phone","udid":"UDID"}],
  "locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],
  "output":{"raw_dir":"raw","review_dir":"review"}
}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	replacementSentinel := filepath.Join(reviewDir, "replacement-sentinel")
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			if err := writeMatrixPNGFile(filepath.Join(plan.App.OutputDir, "home.png")); err != nil {
				return nil, err
			}
			original := reviewDir + "-original"
			if err := os.Rename(reviewDir, original); err != nil {
				return nil, err
			}
			if err := os.Mkdir(reviewDir, 0o700); err != nil {
				return nil, err
			}
			if err := os.WriteFile(replacementSentinel, []byte("replacement must survive"), 0o600); err != nil {
				return nil, err
			}
			return &RunResult{}, nil
		},
		Appearance:  &matrixTestAppearance{},
		CheckDevice: func(context.Context, MatrixDevice) error { return nil },
	})
	if runErr == nil {
		t.Fatal("RunMatrixWithDependencies() error = nil, want replaced review root failure")
	}
	if result != nil && result.Review != nil {
		t.Fatalf("review result = %+v, want no success report for the replacement path", result.Review)
	}
	if _, err := os.Stat(replacementSentinel); err != nil {
		t.Fatalf("replacement sentinel stat error = %v, want replacement preserved", err)
	}
	if _, err := os.Stat(filepath.Join(reviewDir, "manifest.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replacement manifest stat error = %v, want no write through the replaced review path", err)
	}
}

func TestRunMatrix_DoesNotRetryAfterAttemptCleanupFailure(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{
  "version":1,"base_plan":"base.json",
  "devices":[{"id":"phone","udid":"UDID"}],
  "locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],
  "execution":{"max_attempts":2},"output":{"raw_dir":"raw","review_dir":"review"}
}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	var attempts int
	previous := matrixPrivateAttemptCleanupForTest
	matrixPrivateAttemptCleanupForTest = func(string) error { return errors.New("injected attempt cleanup failure") }
	t.Cleanup(func() { matrixPrivateAttemptCleanupForTest = previous })
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			attempts++
			return nil, errors.New("injected execution failure")
		},
		Appearance:  &matrixTestAppearance{},
		CheckDevice: func(context.Context, MatrixDevice) error { return nil },
	})
	if runErr == nil {
		t.Fatal("RunMatrixWithDependencies() error = nil, want cleanup-failed cell")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want no retry after cleanup failure", attempts)
	}
	if result == nil || len(result.Cells) != 1 {
		t.Fatalf("result = %+v, want one cell", result)
	}
	if result.Cells[0].Status != MatrixCellCleanupFailed || result.Cells[0].Attempts != 1 {
		t.Fatalf("cell = %+v, want cleanupFailed after a single attempt", result.Cells[0])
	}
}

func TestRunMatrix_CanceledOutputLockMarksCellsCanceled(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{
  "version":1,"base_plan":"base.json",
  "devices":[{"id":"phone","udid":"UDID"}],
  "locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],
  "output":{"raw_dir":"raw","review_dir":"review"}
}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, runErr := RunMatrixWithDependencies(ctx, matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(context.Context, *Plan) (*RunResult, error) {
			t.Fatal("RunPlan should not run after caller cancellation")
			return nil, nil
		},
		Appearance:  &matrixTestAppearance{},
		CheckDevice: func(context.Context, MatrixDevice) error { return nil },
	})
	if runErr == nil {
		t.Fatal("RunMatrixWithDependencies() error = nil, want caller cancellation")
	}
	if result == nil || len(result.Cells) != 1 {
		t.Fatalf("result = %+v, want one canceled cell", result)
	}
	if result.Cells[0].Status != MatrixCellCanceled || result.Cells[0].FailureCode != "canceled" {
		t.Fatalf("cell = %+v, want canceled rather than output_root_failed", result.Cells[0])
	}
}

func TestRunMatrix_RetriesExecutionButNotValidation(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default","launch_arguments":["--fixture","value with spaces;$(touch should-not-run)"]}],"execution":{"max_attempts":2},"output":{"raw_dir":"raw","review_dir":"review"}}`)
	matrixPlan, _ := LoadMatrixPlan(matrixPath)
	appearance := &matrixTestAppearance{}
	var mu sync.Mutex
	attempts := 0
	runPlan := func(_ context.Context, plan *Plan) (*RunResult, error) {
		mu.Lock()
		attempts++
		current := attempts
		mu.Unlock()
		if got, want := plan.App.LaunchArguments, []string{"-AppleLanguages", "(en)", "-AppleLocale", "en_US", "--fixture", "value with spaces;$(touch should-not-run)"}; !reflect.DeepEqual(got, want) {
			t.Errorf("launch arguments = %v, want %v", got, want)
		}
		if len(plan.Steps) == 0 || plan.Steps[0].Action != ActionLaunch {
			t.Errorf("matrix plan did not ensure an explicit launch step: %+v", plan.Steps)
		}
		if current == 1 {
			return &RunResult{Steps: []RunStepResult{{Index: 1, Action: "screenshot", Status: "error"}}}, errors.New("injected failure")
		}
		writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
		return &RunResult{Steps: []RunStepResult{{Index: 1, Action: "screenshot", Status: "ok"}}}, nil
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{RunPlan: runPlan, Appearance: appearance})
	if runErr != nil {
		t.Fatalf("RunMatrixWithDependencies() error = %v", runErr)
	}
	if attempts != 2 || result.Cells[0].Attempts != 2 || result.Cells[0].Status != MatrixCellSuccess {
		t.Fatalf("attempts=%d cell=%+v", attempts, result.Cells[0])
	}
	if result.Cells[0].FailureStage != "" || result.Cells[0].FailureCode != "" || result.Cells[0].Error != nil {
		t.Fatalf("successful retry retained failure metadata: %+v", result.Cells[0])
	}
}

func TestRunMatrixRestartsAppBeforeEveryCellAttempt(t *testing.T) {
	dir := t.TempDir()
	binDir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	xcrunLog := filepath.Join(dir, "xcrun.log")
	captureCount := filepath.Join(dir, "capture-count")
	axeTemplate := filepath.Join(dir, "template.png")
	writeMinimalPNG(t, axeTemplate, 10, 10)
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"launch"},{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"first","launch_arguments":["--fixture","first"]},{"id":"second","launch_arguments":["--fixture","second"]}],"execution":{"max_attempts":2,"retry_backoff_ms":0},"output":{"raw_dir":"raw","review_dir":"review"}}`)

	writeExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$XCRUN_LOG"
`)
	writeExecutable(t, filepath.Join(binDir, "axe"), `#!/bin/sh
set -eu
count=0
if [ -f "$CAPTURE_COUNT" ]; then count=$(cat "$CAPTURE_COUNT"); fi
count=$((count + 1))
printf '%s' "$count" > "$CAPTURE_COUNT"
if [ "$count" -eq 1 ]; then exit 1; fi
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output" ]; then out="$2"; break; fi
  shift
done
cp "$AXE_TEMPLATE_PNG" "$out"
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XCRUN_LOG", xcrunLog)
	t.Setenv("CAPTURE_COUNT", captureCount)
	t.Setenv("AXE_TEMPLATE_PNG", axeTemplate)

	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{Appearance: &matrixTestAppearance{}})
	if runErr != nil {
		t.Fatalf("RunMatrixWithDependencies() error = %v", runErr)
	}
	if result.Succeeded != 2 {
		t.Fatalf("result = %+v, want two successful cells", result)
	}
	data, err := os.ReadFile(xcrunLog)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", xcrunLog, err)
	}
	launches := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(launches) != 3 {
		t.Fatalf("launches = %q, want one failed attempt, its retry, and the second cell", launches)
	}
	wantLaunches := []string{
		"simctl launch --terminate-running-process SIM-UDID com.example.app -AppleLanguages (en) -AppleLocale en_US --fixture first",
		"simctl launch --terminate-running-process SIM-UDID com.example.app -AppleLanguages (en) -AppleLocale en_US --fixture first",
		"simctl launch --terminate-running-process SIM-UDID com.example.app -AppleLanguages (en) -AppleLocale en_US --fixture second",
	}
	if !reflect.DeepEqual(launches, wantLaunches) {
		t.Fatalf("launches = %q, want %q", launches, wantLaunches)
	}
}

func TestRunMatrixLateCancellationAfterCompletedCellsStillSucceeds(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review"}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	appearance := &matrixTestAppearance{restoreFunc: cancel}
	runPlan := func(_ context.Context, plan *Plan) (*RunResult, error) {
		writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
		return &RunResult{Steps: []RunStepResult{{Index: 1, Action: "screenshot", Status: "ok"}}}, nil
	}
	result, runErr := RunMatrixWithDependencies(ctx, matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan:     runPlan,
		Appearance:  appearance,
		CheckDevice: func(context.Context, MatrixDevice) error { return nil },
	})
	if runErr != nil {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want completed matrix to remain successful", runErr)
	}
	if result == nil || result.Succeeded != 1 || result.Failed != 0 || result.Canceled != 0 || result.Status != MatrixCellSuccess {
		t.Fatalf("late-cancel result = %+v, want one successful cell", result)
	}
	if result.Cells[0].Status != MatrixCellSuccess {
		t.Fatalf("cell status = %q, want success", result.Cells[0].Status)
	}
}

func TestRunMatrixPreservesCompletedCellsWhenLaterCancellationOccurs(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"first"},{"id":"second"}],"execution":{"max_concurrency":1},"output":{"raw_dir":"raw","review_dir":"review"}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	appearance := &matrixTestAppearance{restoreFunc: cancel}
	var calls int
	result, runErr := RunMatrixWithDependencies(ctx, matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			calls++
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{Steps: []RunStepResult{{Index: 1, Action: "screenshot", Status: "ok"}}}, nil
		},
		Appearance:  appearance,
		CheckDevice: func(context.Context, MatrixDevice) error { return nil },
	})
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want caller cancellation", runErr)
	}
	if calls != 1 {
		t.Fatalf("RunPlan calls = %d, want only the first cell before cancellation", calls)
	}
	if result == nil || result.Succeeded != 1 || result.Canceled != 1 || result.Failed != 0 {
		t.Fatalf("result summary = %+v, want one success and one cancellation", result)
	}
	if result.Cells[0].Status != MatrixCellSuccess || len(result.Cells[0].RawPaths) != 1 {
		t.Fatalf("completed cell = %+v, want success with retained raw artifact", result.Cells[0])
	}
	if result.Cells[1].Status != MatrixCellCanceled || result.Cells[1].FailureCode != "canceled" {
		t.Fatalf("canceled cell = %+v, want canceled status", result.Cells[1])
	}
}

func TestRunMatrixPreservesCompletedFramedCellsWhenLaterCancellationOccurs(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"first"},{"id":"second"}],"execution":{"max_concurrency":1},"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"review","frame":{"enabled":true,"device_by_matrix_device":{"phone":"iphone-17-pro"}}}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	appearance := &matrixTestAppearance{restoreFunc: cancel}
	var calls int
	result, runErr := RunMatrixWithDependencies(ctx, matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			calls++
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{Steps: []RunStepResult{{Index: 1, Action: "screenshot", Status: "ok"}}}, nil
		},
		Frame: func(_ context.Context, request FrameRequest) (*FrameResult, error) {
			writeMatrixPNG(t, request.OutputPath)
			return &FrameResult{}, nil
		},
		Appearance:  appearance,
		CheckDevice: func(context.Context, MatrixDevice) error { return nil },
	})
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want caller cancellation", runErr)
	}
	if calls != 1 {
		t.Fatalf("RunPlan calls = %d, want only the first cell before cancellation", calls)
	}
	if result == nil || result.Succeeded != 1 || result.Canceled != 1 || result.Failed != 0 {
		t.Fatalf("result summary = %+v, want one framed success and one cancellation", result)
	}
	if result.Cells[0].Status != MatrixCellSuccess || len(result.Cells[0].FramedPaths) != 1 || result.Cells[0].Screenshots[0].FramedPath == "" {
		t.Fatalf("completed framed cell = %+v, want retained framed artifact", result.Cells[0])
	}
	if result.Cells[1].Status != MatrixCellCanceled || result.Cells[1].FailureCode != "canceled" {
		t.Fatalf("canceled cell = %+v, want canceled status", result.Cells[1])
	}
}

func TestRunMatrix_PreflightFailureDoesNotSuppressReadyDevices(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone-a","udid":"READY"},{"id":"phone-b","udid":"NOT-READY"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review"}}`)
	matrixPlan, _ := LoadMatrixPlan(matrixPath)
	appearance := &matrixTestAppearance{}
	var ran sync.Mutex
	readyRuns := 0
	runPlan := func(_ context.Context, plan *Plan) (*RunResult, error) {
		ran.Lock()
		readyRuns++
		ran.Unlock()
		writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
		return &RunResult{Steps: []RunStepResult{{Index: 1, Action: "screenshot", Status: "ok"}}}, nil
	}
	checkDevice := func(_ context.Context, device MatrixDevice) error {
		if device.ID == "phone-b" {
			return errors.New("not ready")
		}
		return nil
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{RunPlan: runPlan, Appearance: appearance, CheckDevice: checkDevice})
	if runErr == nil {
		t.Fatal("expected preflight failure")
	}
	if readyRuns != 1 || result.Succeeded != 1 || result.Failed != 1 || result.Status != MatrixCellFailed {
		t.Fatalf("unexpected preflight result: runs=%d result=%+v", readyRuns, result)
	}
	if result.Cells[1].FailureCode != "simulator_not_ready" || result.Cells[1].Error == nil {
		t.Fatalf("missing sanitized preflight error: %+v", result.Cells[1])
	}
	if _, err := os.Stat(filepath.Join(dir, "review", "manifest.json")); err != nil {
		t.Fatalf("manifest missing after preflight failure: %v", err)
	}
}

func TestRunMatrix_InventoryCancellationMarksAllCellsCanceled(t *testing.T) {
	tests := []struct {
		name       string
		newContext func() (context.Context, context.CancelFunc)
		wantError  error
	}{
		{
			name: "caller canceled",
			newContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			wantError: context.Canceled,
		},
		{
			name: "caller deadline exceeded",
			newContext: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			wantError: context.DeadlineExceeded,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			basePath := filepath.Join(dir, "base.json")
			matrixPath := filepath.Join(dir, "matrix.json")
			writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
			writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"sim-a","udid":"SIM-A"},{"id":"sim-b","udid":"SIM-B"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review"}}`)
			matrixPlan, err := LoadMatrixPlan(matrixPath)
			if err != nil {
				t.Fatalf("LoadMatrixPlan() error = %v", err)
			}
			ctx, cancel := tc.newContext()
			defer cancel()

			result, runErr := RunMatrixWithDependencies(ctx, matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{})
			if !errors.Is(runErr, tc.wantError) {
				t.Fatalf("RunMatrixWithDependencies() error = %v, want %v", runErr, tc.wantError)
			}
			if result == nil {
				t.Fatal("RunMatrixWithDependencies() result = nil, want partial canceled result")
			}
			if result.Succeeded != 0 || result.Failed != 0 || result.Canceled != len(result.Cells) {
				t.Fatalf("unexpected cancellation summary: %+v", result)
			}
			if result.Review != nil {
				t.Fatalf("canceled-before-lock run published a review without output locks: %+v", result.Review)
			}
			for _, cell := range result.Cells {
				if cell.Status != MatrixCellCanceled {
					t.Errorf("cell %q status = %q, want canceled", cell.ID, cell.Status)
				}
				if cell.FailureCode == "simulator_not_ready" {
					t.Errorf("cell %q reported simulator_not_ready for inventory cancellation", cell.ID)
				}
			}
		})
	}
}

func TestRunMatrixReportsFrameFamilyMismatchPreflight(t *testing.T) {
	skipWindowsUnixExecutableFixtures(t)
	binDir := t.TempDir()
	xcrunPath := filepath.Join(binDir, "xcrun")
	script := `#!/bin/sh
set -eu
printf '%s\n' '{"devices":{"runtime":[{"udid":"SIM-UDID","state":"Booted","isAvailable":true,"name":"iPad Pro (13-inch)","deviceTypeIdentifier":"com.apple.CoreSimulator.SimDeviceType.iPad-Pro"}]}}'
`
	if err := os.WriteFile(xcrunPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write xcrun fixture: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"iphone-demo","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review","frame":{"enabled":true,"device_by_matrix_device":{"iphone-demo":"iphone-17-pro"}}}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{})
	if runErr == nil {
		t.Fatal("RunMatrixWithDependencies() error = nil, want frame-family preflight failure")
	}
	if result == nil || len(result.Cells) != 1 {
		t.Fatalf("result = %+v, want one-cell preflight result", result)
	}
	cell := result.Cells[0]
	if cell.Status != MatrixCellFailed || cell.FailureStage != "preflight" || cell.FailureCode != "frame_family_mismatch" {
		t.Fatalf("cell = %+v, want specific frame-family preflight failure", cell)
	}
	if cell.Error == nil || cell.Error.Message != "configured frame does not match simulator family" {
		t.Fatalf("cell error = %+v, want stable frame-family reason", cell.Error)
	}
}

func TestRunMatrix_InventoryTimeoutMarksCellsFailed(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"sim-a","udid":"SIM-A"},{"id":"sim-b","udid":"SIM-B"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review"}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	checkDevice := func(context.Context, MatrixDevice) error {
		return ErrMatrixInventoryTimeout
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{CheckDevice: checkDevice})
	if runErr == nil {
		t.Fatal("RunMatrixWithDependencies() error = nil, want preflight failure")
	}
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want non-context preflight failure", runErr)
	}
	if result.Succeeded != 0 || result.Failed != len(result.Cells) || result.Canceled != 0 || result.Status != MatrixCellFailed {
		t.Fatalf("unexpected inventory-timeout result: %+v", result)
	}
	if result.Review == nil || result.Review.Failed != len(result.Cells) || result.Review.Canceled != 0 {
		t.Fatalf("unexpected inventory-timeout review: %+v", result.Review)
	}
	for _, cell := range result.Cells {
		if cell.Status != MatrixCellFailed || cell.FailureStage != "preflight" || cell.FailureCode != "simulator_not_ready" {
			t.Errorf("cell %q = %+v, want preflight simulator_not_ready failure", cell.ID, cell)
		}
	}
}

func TestRunMatrix_InventoryTimeoutPreservesReadyCellInPartialReview(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"sim-ready","udid":"SIM-READY"},{"id":"sim-timeout","udid":"SIM-TIMEOUT"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review"}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	checkDevice := func(_ context.Context, device MatrixDevice) error {
		if device.ID == "sim-timeout" {
			return ErrMatrixInventoryTimeout
		}
		return nil
	}
	runPlan := func(_ context.Context, plan *Plan) (*RunResult, error) {
		writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
		return &RunResult{Steps: []RunStepResult{{Index: 1, Action: "screenshot", Status: "ok"}}}, nil
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan:     runPlan,
		Appearance:  &matrixTestAppearance{},
		CheckDevice: checkDevice,
	})
	if runErr == nil || errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want child-only timeout without caller cancellation", runErr)
	}
	if result.Succeeded != 1 || result.Failed != 1 || result.Canceled != 0 {
		t.Fatalf("result summary = %+v, want one ready success and one timeout failure", result)
	}
	if result.Cells[0].Status != MatrixCellSuccess || result.Cells[1].Status != MatrixCellFailed {
		t.Fatalf("cell statuses = %q, %q, want success/failed", result.Cells[0].Status, result.Cells[1].Status)
	}
	if result.Cells[1].FailureCode != "simulator_not_ready" {
		t.Fatalf("timeout cell failure code = %q, want simulator_not_ready", result.Cells[1].FailureCode)
	}
	if result.Review == nil || result.Review.Succeeded != 1 || result.Review.Failed != 1 || result.Review.Canceled != 0 {
		t.Fatalf("review summary = %+v, want one success and one failure", result.Review)
	}
}

func TestRunMatrix_CleanupFailureBlocksLaterCellsOnSameSimulator(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM"}],"locales":["en-US","ja-JP"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review"}}`)
	matrixPlan, _ := LoadMatrixPlan(matrixPath)
	appearance := &matrixTestAppearance{restoreErr: true}
	runPlan := func(_ context.Context, plan *Plan) (*RunResult, error) {
		writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
		return &RunResult{Steps: []RunStepResult{{Index: 1, Action: "screenshot", Status: "ok"}}}, nil
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{RunPlan: runPlan, Appearance: appearance})
	if runErr == nil {
		t.Fatal("expected cleanup failure")
	}
	if result.Cells[0].Status != MatrixCellCleanupFailed || result.Cells[1].FailureCode != "simulator_blocked_after_cleanup_failure" {
		t.Fatalf("unexpected cleanup result: %+v", result.Cells)
	}
	if appearance.restoreCount != 1 {
		t.Fatalf("restore count = %d, want one before simulator was blocked", appearance.restoreCount)
	}
}

type matrixTestAppearance struct {
	mu           sync.Mutex
	setCount     int
	restoreCount int
	setErr       bool
	restoreErr   bool
	restoreFunc  func()
}

func (*matrixTestAppearance) Snapshot(context.Context, string) (string, error) { return "light", nil }

func (a *matrixTestAppearance) Set(context.Context, string, string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.setCount++
	if a.setErr {
		return errors.New("set failed")
	}
	return nil
}

func (a *matrixTestAppearance) Restore(context.Context, string, string) error {
	a.mu.Lock()
	a.restoreCount++
	restoreErr := a.restoreErr
	restoreFunc := a.restoreFunc
	a.mu.Unlock()
	if restoreFunc != nil {
		restoreFunc()
	}
	if restoreErr {
		return errors.New("restore failed")
	}
	return nil
}

func writeMatrixTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func stringPtr(value string) *string { return &value }

func makeMatrixDevices(count int) []MatrixDevice {
	devices := make([]MatrixDevice, count)
	for i := range devices {
		devices[i] = MatrixDevice{ID: "device-" + string(rune('a'+i)), UDID: "udid-" + string(rune('a'+i))}
	}
	return devices
}

func makeMatrixLocales(count int) []string {
	values := make([]string, count)
	for i := range values {
		values[i] = fmt.Sprintf("en-%03d", i+1)
	}
	return values
}

func writeMatrixPNGFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, image.NewRGBA(image.Rect(0, 0, 2, 2)))
}

func writeMatrixPNG(t *testing.T, path string) {
	t.Helper()
	if err := writeMatrixPNGFile(path); err != nil {
		t.Fatalf("write fake screenshot: %v", err)
	}
}

func TestEnsureMatrixLaunchStepPrependsLaunchBeforePreLaunchSteps(t *testing.T) {
	tests := []struct {
		name  string
		steps []StepAction
		want  []StepAction
	}{
		{
			name:  "no launch at all",
			steps: []StepAction{ActionScreenshot},
			want:  []StepAction{ActionLaunch, ActionScreenshot},
		},
		{
			name:  "already launches first",
			steps: []StepAction{ActionLaunch, ActionScreenshot},
			want:  []StepAction{ActionLaunch, ActionScreenshot},
		},
		{
			name:  "wait before launch stays untouched",
			steps: []StepAction{ActionWait, ActionLaunch, ActionScreenshot},
			want:  []StepAction{ActionWait, ActionLaunch, ActionScreenshot},
		},
		{
			name:  "screenshot before a later launch",
			steps: []StepAction{ActionScreenshot, ActionLaunch, ActionScreenshot},
			want:  []StepAction{ActionLaunch, ActionScreenshot, ActionLaunch, ActionScreenshot},
		},
		{
			name:  "interaction before a later launch",
			steps: []StepAction{ActionTap, ActionLaunch, ActionScreenshot},
			want:  []StepAction{ActionLaunch, ActionTap, ActionLaunch, ActionScreenshot},
		},
		{
			name:  "wait_for before a later launch",
			steps: []StepAction{ActionWaitFor, ActionLaunch},
			want:  []StepAction{ActionLaunch, ActionWaitFor, ActionLaunch},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}}
			for _, action := range tt.steps {
				plan.Steps = append(plan.Steps, PlanStep{Action: action})
			}
			ensureMatrixLaunchStep(plan)
			got := make([]StepAction, 0, len(plan.Steps))
			for _, step := range plan.Steps {
				got = append(got, step.Action)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("steps = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunMatrixLaunchesBeforeBaseStepsThatPrecedeALaterLaunch(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"},{"action":"launch"},{"action":"screenshot","name":"details"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"promo","launch_arguments":["-promo","1"]}],"output":{"raw_dir":"raw","review_dir":"review"}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	var gotSteps []StepAction
	var gotArgs []string
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			for _, step := range plan.Steps {
				gotSteps = append(gotSteps, step.Action)
			}
			gotArgs = append([]string(nil), plan.App.LaunchArguments...)
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "details.png"))
			return &RunResult{}, nil
		},
		Appearance: &matrixTestAppearance{},
	})
	if runErr != nil {
		t.Fatalf("RunMatrixWithDependencies() error = %v", runErr)
	}
	if result == nil || result.Succeeded != 1 {
		t.Fatalf("result = %+v, want one successful cell", result)
	}
	if len(gotSteps) == 0 || gotSteps[0] != ActionLaunch {
		t.Fatalf("executed steps = %v, want a launch before the first pre-launch base step", gotSteps)
	}
	if !slices.Contains(gotArgs, "-promo") {
		t.Fatalf("launch arguments = %v, want the cell's content-variant arguments applied", gotArgs)
	}
}

func TestMatrixReviewManifestPersistsCleanupFailedTotal(t *testing.T) {
	dir := t.TempDir()
	result := &MatrixResult{Cells: []MatrixCellResult{
		{ID: "phone|en-US|light|default", Status: MatrixCellSuccess},
		{ID: "phone|en-US|dark|default", Status: MatrixCellCleanupFailed, FailureStage: "appearance", FailureCode: "restore_failed"},
		{ID: "phone|fr-FR|light|default", Status: MatrixCellFailed, FailureStage: "execution", FailureCode: "run_failed"},
	}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: dir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	manifest, err := LoadMatrixReviewManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("LoadMatrixReviewManifest() error = %v", err)
	}
	if manifest.CleanupFailed != 1 {
		t.Fatalf("manifest cleanupFailed = %d, want 1", manifest.CleanupFailed)
	}
	// countMatrixResultStatuses counts a cleanup failure in both Failed and
	// CleanupFailed; the persisted manifest must mirror that contract exactly.
	if manifest.Failed != 2 {
		t.Fatalf("manifest failed = %d, want cleanup failures counted in failed too", manifest.Failed)
	}
	if manifest.Succeeded != 1 || manifest.TotalCells != 3 {
		t.Fatalf("manifest totals = succeeded %d total %d, want 1 and 3", manifest.Succeeded, manifest.TotalCells)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if !strings.Contains(string(raw), `"cleanupFailed": 1`) {
		t.Fatalf("manifest JSON missing cleanupFailed aggregate: %s", raw)
	}
	html, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("read HTML: %v", err)
	}
	if !strings.Contains(string(html), "cleanup failed: 1") {
		t.Fatalf("HTML summary missing cleanup-failure total: %s", html)
	}
}

func TestMatrixReviewManifestOmitsCleanupFailedWhenZero(t *testing.T) {
	dir := t.TempDir()
	result := &MatrixResult{Cells: []MatrixCellResult{{ID: "phone|en-US|light|default", Status: MatrixCellSuccess}}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: dir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if strings.Contains(string(raw), "cleanupFailed") {
		t.Fatalf("manifest JSON emitted cleanupFailed for a clean run: %s", raw)
	}
	html, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("read HTML: %v", err)
	}
	if strings.Contains(string(html), "cleanup failed") {
		t.Fatalf("HTML summary emitted cleanup-failure total for a clean run: %s", html)
	}
}

func TestLoadMatrixPlanRejectsExplicitZeroExecutionLimits(t *testing.T) {
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	tests := []struct {
		name      string
		execution string
		wantErr   string
	}{
		{
			name:      "explicit zero concurrency",
			execution: `{"max_concurrency":0}`,
			wantErr:   "execution.max_concurrency must be between 1 and 8 when set",
		},
		{
			name:      "explicit zero attempts",
			execution: `{"max_attempts":0}`,
			wantErr:   "execution.max_attempts must be between 1 and 3 when set",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			matrixPath := filepath.Join(dir, "matrix.json")
			writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"execution":`+tt.execution+`,"output":{"raw_dir":"raw","review_dir":"review"}}`)
			plan, err := LoadMatrixPlan(matrixPath)
			if err != nil {
				t.Fatalf("LoadMatrixPlan() error = %v", err)
			}
			if err := ValidateMatrixPlan(plan, base); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateMatrixPlan() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadMatrixPlanAcceptsOmittedExecutionLimits(t *testing.T) {
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	dir := t.TempDir()
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"execution":{"retry_backoff_ms":10},"output":{"raw_dir":"raw","review_dir":"review"}}`)
	plan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	if err := ValidateMatrixPlan(plan, base); err != nil {
		t.Fatalf("ValidateMatrixPlan() error = %v, want omitted execution limits to default", err)
	}
	// A programmatically constructed plan carries no presence information, so a
	// zero value must still mean "use the default" rather than "explicit zero".
	if err := ValidateMatrixPlan(&MatrixPlan{
		Version:         1,
		Devices:         []MatrixDevice{{ID: "phone", UDID: "SIM-UDID"}},
		Locales:         []string{"en-US"},
		Appearances:     []string{"light"},
		ContentVariants: []MatrixContentVariant{{ID: "default"}},
	}, base); err != nil {
		t.Fatalf("ValidateMatrixPlan() on constructed plan error = %v, want zero to mean default", err)
	}
}

func TestValidateMatrixPlanRejectsReviewOutputAliasingPlanInputs(t *testing.T) {
	tests := []struct {
		name          string
		matrixName    string
		baseName      string
		wantSubstring string
	}{
		{
			name:          "review manifest overwrites the matrix plan",
			matrixName:    "manifest.json",
			baseName:      "base.json",
			wantSubstring: "matrix plan",
		},
		{
			name:          "review HTML overwrites the matrix plan",
			matrixName:    "index.html",
			baseName:      "base.json",
			wantSubstring: "matrix plan",
		},
		{
			name:          "review manifest overwrites the base plan",
			matrixName:    "matrix.json",
			baseName:      "manifest.json",
			wantSubstring: "base plan",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configDir := filepath.Join(dir, "config")
			if err := os.MkdirAll(configDir, 0o755); err != nil {
				t.Fatalf("create config dir: %v", err)
			}
			basePath := filepath.Join(configDir, tt.baseName)
			matrixPath := filepath.Join(configDir, tt.matrixName)
			writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
			writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"`+tt.baseName+`","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"."}}`)
			matrixPlan, err := LoadMatrixPlan(matrixPath)
			if err != nil {
				t.Fatalf("LoadMatrixPlan() error = %v", err)
			}
			_, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
				RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
					writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
					return &RunResult{}, nil
				},
				Appearance: &matrixTestAppearance{},
			})
			if runErr == nil || !strings.Contains(runErr.Error(), tt.wantSubstring) {
				t.Fatalf("RunMatrixWithDependencies() error = %v, want refusal naming the %s", runErr, tt.wantSubstring)
			}
			// The plan input must survive untouched.
			data, err := os.ReadFile(matrixPath)
			if err != nil {
				t.Fatalf("read matrix plan: %v", err)
			}
			if !strings.Contains(string(data), `"base_plan"`) {
				t.Fatalf("matrix plan was overwritten by generated output: %s", data)
			}
			baseData, err := os.ReadFile(basePath)
			if err != nil {
				t.Fatalf("read base plan: %v", err)
			}
			if !strings.Contains(string(baseData), `"bundle_id"`) {
				t.Fatalf("base plan was overwritten by generated output: %s", baseData)
			}
		})
	}
}

func TestValidateMatrixReviewDoesNotOverwritePlansPreservesLiteralSourceFilename(t *testing.T) {
	dir := t.TempDir()
	err := validateMatrixReviewDoesNotOverwritePlans(&MatrixPlan{
		sourcePath: filepath.Join(dir, "index.html "),
		Output:     MatrixOutput{ReviewDir: "."},
	}, dir)
	if err != nil {
		t.Fatalf("validateMatrixReviewDoesNotOverwritePlans() error = %v, want trailing-space plan name to stay distinct from index.html", err)
	}
}

func TestValidateMatrixReviewDoesNotOverwritePlansHonorsFilesystemCase(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "INDEX.HTML")
	writeMatrixTestFile(t, planPath, `{"version":1}`)
	err := validateMatrixReviewDoesNotOverwritePlans(&MatrixPlan{
		sourcePath: planPath,
		Output:     MatrixOutput{ReviewDir: dir},
	}, dir)
	if matrixFilesystemCaseInsensitive(dir) {
		if err == nil || !strings.Contains(err.Error(), "matrix plan") {
			t.Fatalf("validateMatrixReviewDoesNotOverwritePlans() error = %v, want case-insensitive collision", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("validateMatrixReviewDoesNotOverwritePlans() error = %v, want INDEX.HTML to stay distinct from index.html", err)
	}
}

func TestRunMatrixRejectsArtifactOutputAliasingBasePlan(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "raw", "en-US", "phone", "light", "default", "home.png")
	if err := os.MkdirAll(filepath.Dir(basePath), 0o755); err != nil {
		t.Fatalf("create base-plan directory: %v", err)
	}
	baseContents := []byte(`{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	if err := os.WriteFile(basePath, baseContents, 0o644); err != nil {
		t.Fatalf("write base plan: %v", err)
	}
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"raw/en-US/phone/light/default/home.png","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"review"}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	runCalled := false
	_, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			runCalled = true
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Appearance: &matrixTestAppearance{},
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "base plan") {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want base-plan artifact collision", runErr)
	}
	if runCalled {
		t.Fatal("RunMatrixWithDependencies() invoked the screenshot plan before rejecting the artifact collision")
	}
	got, err := os.ReadFile(basePath)
	if err != nil {
		t.Fatalf("read base plan after rejected run: %v", err)
	}
	if !bytes.Equal(got, baseContents) {
		t.Fatalf("base plan changed after rejected run: got %q, want %q", got, baseContents)
	}
}

func TestRunMatrixIgnoresFramedArtifactCollisionsWhenFramingDisabled(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "framed", "en-US", "phone", "light", "default", "home.png")
	if err := os.MkdirAll(filepath.Dir(basePath), 0o755); err != nil {
		t.Fatalf("create base-plan directory: %v", err)
	}
	if err := os.WriteFile(basePath, []byte(`{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`), 0o644); err != nil {
		t.Fatalf("write base plan: %v", err)
	}
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"framed/en-US/phone/light/default/home.png","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"review","frame":{"enabled":false}}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Appearance: &matrixTestAppearance{},
	})
	if runErr != nil {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want raw-only run to ignore unused framed paths", runErr)
	}
	if result == nil || result.Succeeded != 1 {
		t.Fatalf("result = %+v, want one successful raw-only cell", result)
	}
}

func TestRunMatrixRejectsPhysicallyAliasedOutputDirectories(t *testing.T) {
	dir := t.TempDir()
	realRoot := filepath.Join(dir, "real")
	linkRoot := filepath.Join(dir, "link")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatalf("create real output parent: %v", err)
	}
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	basePath := filepath.Join(dir, "base.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"real/out","framed_dir":"link/out","review_dir":"review","frame":{"enabled":true,"device_by_matrix_device":{"phone":"iphone-17-pro"}}}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	if filepath.Clean(filepath.Join(realRoot, "out")) == filepath.Clean(filepath.Join(linkRoot, "out")) {
		t.Fatal("test setup produced lexically equal output paths; identity validation was not exercised")
	}
	runCalled := false
	_, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			runCalled = true
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Frame: func(_ context.Context, request FrameRequest) (*FrameResult, error) {
			writeMatrixPNG(t, request.OutputPath)
			return &FrameResult{}, nil
		},
		Appearance: &matrixTestAppearance{},
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "raw_dir and output.framed_dir") {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want physically aliased output rejection", runErr)
	}
	if runCalled {
		t.Fatal("RunMatrixWithDependencies() invoked the screenshot plan before rejecting aliased output roots")
	}
}

func TestValidateMatrixOutputPathsHonorsFilesystemCase(t *testing.T) {
	dir := t.TempDir()
	output := MatrixOutput{
		RawDir:    filepath.Join(dir, "raw"),
		FramedDir: filepath.Join(dir, "Raw"),
		ReviewDir: filepath.Join(dir, "review"),
	}
	err := validateMatrixOutputPaths(output, "")
	if matrixFilesystemCaseInsensitive(dir) {
		if err == nil || !strings.Contains(err.Error(), "must be different directories") {
			t.Fatalf("validateMatrixOutputPaths() error = %v, want case-insensitive collision", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("validateMatrixOutputPaths() error = %v, want distinct case-sensitive output roots", err)
	}
}

func TestMatrixReviewAssetRootPathsAcceptsFilesystemAliases(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "Review")
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		t.Fatalf("create review dir: %v", err)
	}
	alias := outputDir
	if matrixFilesystemCaseInsensitive(dir) {
		alias = filepath.Join(dir, "review")
	}
	_, err := matrixReviewAssetRootPaths(alias, &MatrixReviewManifest{OutputDir: outputDir})
	if err != nil {
		t.Fatalf("matrixReviewAssetRootPaths() error = %v, want filesystem identity match", err)
	}
}

func TestSameMatrixPathResolvesMissingSymlinkedSuffix(t *testing.T) {
	dir := t.TempDir()
	realRoot := filepath.Join(dir, "real")
	linkRoot := filepath.Join(dir, "link")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatalf("create real path: %v", err)
	}
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	left := filepath.Join(realRoot, "future", "plan.json")
	right := filepath.Join(linkRoot, "future", "plan.json")
	if _, err := os.Lstat(left); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("left test path unexpectedly exists: %v", err)
	}
	if _, err := os.Lstat(right); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("right test path unexpectedly exists: %v", err)
	}
	leftPhysical, leftOK := resolveMatrixPhysicalPath(left)
	rightPhysical, rightOK := resolveMatrixPhysicalPath(right)
	if !leftOK || !rightOK || !strings.EqualFold(leftPhysical, rightPhysical) {
		t.Fatalf("missing aliased paths resolved to %q and %q (ok=%t/%t)", leftPhysical, rightPhysical, leftOK, rightOK)
	}
	if !sameMatrixPath(left, right) {
		t.Fatalf("sameMatrixPath(%q, %q) = false, want physical identity", left, right)
	}

	distinct := filepath.Join(linkRoot, "other", "plan.json")
	if sameMatrixPath(left, distinct) {
		t.Fatalf("sameMatrixPath(%q, %q) = true, want distinct missing suffixes to remain distinct", left, distinct)
	}
}

func TestValidateMatrixPlanAllowsReviewDirBesidePlanWithDifferentNames(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	basePath := filepath.Join(configDir, "base.json")
	matrixPath := filepath.Join(configDir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"."}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Appearance: &matrixTestAppearance{},
	})
	if runErr != nil {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want a review dir beside differently named plans to be allowed", runErr)
	}
	if result == nil || result.Succeeded != 1 {
		t.Fatalf("result = %+v, want one successful cell", result)
	}
	if _, err := os.Stat(filepath.Join(configDir, "manifest.json")); err != nil {
		t.Fatalf("review manifest not written beside the plans: %v", err)
	}
}

func TestGenerateMatrixReviewWritesOnlyTheDeclaredFiles(t *testing.T) {
	dir := t.TempDir()
	result := &MatrixResult{Cells: []MatrixCellResult{{ID: "phone|en-US|light|default", Status: MatrixCellSuccess}}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: dir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read review dir: %v", err)
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	sort.Strings(got)
	want := append([]string(nil), matrixReviewGeneratedFiles...)
	sort.Strings(want)
	// Plan validation refuses inputs aliasing matrixReviewGeneratedFiles, so an
	// undeclared published file would be an unguarded overwrite hazard.
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("review dir contents = %v, want exactly matrixReviewGeneratedFiles %v", got, want)
	}
}

func TestValidateMatrixPlanRejectsReviewOutputAliasingPlanThroughSymlinkedDirectory(t *testing.T) {
	dir := t.TempDir()
	// openMatrixOutputRoot resolves the review directory's parent physically, so
	// a symlinked ancestor still lands the generated files in the real
	// directory. Only components below the opened root are refused as symlinks.
	realParent := filepath.Join(dir, "real")
	realDir := filepath.Join(realParent, "project")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("create real dir: %v", err)
	}
	if err := os.Symlink(realParent, filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	linkDir := filepath.Join(dir, "link", "project")
	basePath := filepath.Join(realDir, "base.json")
	matrixPath := filepath.Join(realDir, "manifest.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	// review_dir reaches the plan's own directory through a different path.
	// Cleaned-string comparison cannot see that these are the same directory.
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":`+strconv.Quote(linkDir)+`}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	if filepath.Clean(filepath.Join(linkDir, "manifest.json")) == filepath.Clean(matrixPath) {
		t.Fatal("test setup produced lexically equal paths; the identity check would not be exercised")
	}
	_, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Appearance: &matrixTestAppearance{},
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "matrix plan") {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want refusal naming the matrix plan", runErr)
	}
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("read matrix plan: %v", err)
	}
	if !strings.Contains(string(data), `"base_plan"`) {
		t.Fatalf("matrix plan was overwritten through the symlinked review dir: %s", data)
	}
}

func TestValidateMatrixPlanRejectsBothRetryBackoffFieldsWhenMillisecondsIsZero(t *testing.T) {
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	tests := []struct {
		name      string
		execution string
		wantErr   string
	}{
		{
			name:      "explicit zero milliseconds beside a duration string",
			execution: `{"retry_backoff_ms":0,"retry_backoff":"1s"}`,
			wantErr:   "set only one of execution.retry_backoff or execution.retry_backoff_ms",
		},
		{
			name:      "nonzero milliseconds beside a duration string",
			execution: `{"retry_backoff_ms":5,"retry_backoff":"1s"}`,
			wantErr:   "set only one of execution.retry_backoff or execution.retry_backoff_ms",
		},
		{
			name:      "explicit zero milliseconds beside an empty duration string",
			execution: `{"retry_backoff_ms":0,"retry_backoff":""}`,
			wantErr:   "set only one of execution.retry_backoff or execution.retry_backoff_ms",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			matrixPath := filepath.Join(dir, "matrix.json")
			writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"execution":`+tt.execution+`,"output":{"raw_dir":"raw","review_dir":"review"}}`)
			plan, err := LoadMatrixPlan(matrixPath)
			if err != nil {
				t.Fatalf("LoadMatrixPlan() error = %v", err)
			}
			if err := ValidateMatrixPlan(plan, base); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateMatrixPlan() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMatrixPlanRejectsExplicitEmptyRetryBackoff(t *testing.T) {
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	for _, value := range []string{"", "   "} {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			dir := t.TempDir()
			matrixPath := filepath.Join(dir, "matrix.json")
			writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"execution":{"retry_backoff":"`+value+`"},"output":{"raw_dir":"raw","review_dir":"review"}}`)
			plan, err := LoadMatrixPlan(matrixPath)
			if err != nil {
				t.Fatalf("LoadMatrixPlan() error = %v", err)
			}
			if err := ValidateMatrixPlan(plan, base); err == nil || !strings.Contains(err.Error(), "execution.retry_backoff must not be empty") {
				t.Fatalf("ValidateMatrixPlan() error = %v, want explicit-empty retry rejection", err)
			}
		})
	}
}

func TestValidateMatrixPlanAcceptsASingleRetryBackoffEncoding(t *testing.T) {
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	for _, execution := range []string{
		`{"retry_backoff_ms":0}`,
		`{"retry_backoff_ms":250}`,
		`{"retry_backoff":"1s"}`,
		`{}`,
	} {
		t.Run(execution, func(t *testing.T) {
			dir := t.TempDir()
			matrixPath := filepath.Join(dir, "matrix.json")
			writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"execution":`+execution+`,"output":{"raw_dir":"raw","review_dir":"review"}}`)
			plan, err := LoadMatrixPlan(matrixPath)
			if err != nil {
				t.Fatalf("LoadMatrixPlan() error = %v", err)
			}
			if err := ValidateMatrixPlan(plan, base); err != nil {
				t.Fatalf("ValidateMatrixPlan() error = %v, want a single retry-backoff encoding to be accepted", err)
			}
		})
	}
}

func TestValidateMatrixPlanBoundsRetryBackoffMillisecondsToDuration(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("retry_backoff_ms cannot represent the duration boundary on 32-bit platforms")
	}

	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	const maxRetryBackoffMS = (1<<63 - 1) / int64(time.Millisecond)
	for _, test := range []struct {
		name    string
		value   int64
		wantErr bool
	}{
		{name: "maximum representable duration", value: maxRetryBackoffMS},
		{name: "one millisecond beyond duration", value: maxRetryBackoffMS + 1, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := &MatrixPlan{
				Version:         1,
				Devices:         []MatrixDevice{{ID: "phone", UDID: "SIM-UDID"}},
				Locales:         []string{"en-US"},
				Appearances:     []string{"light"},
				ContentVariants: []MatrixContentVariant{{ID: "default"}},
				Execution:       MatrixExecution{RetryBackoffMS: int(test.value)},
			}
			err := ValidateMatrixPlan(plan, base)
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "execution.retry_backoff_ms exceeds maximum duration") {
					t.Fatalf("ValidateMatrixPlan() error = %v, want retry-backoff overflow rejection", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateMatrixPlan() error = %v, want maximum duration to be accepted", err)
			}
			_, _, backoff, err := resolveMatrixExecution(plan.Execution, MatrixOptions{})
			if err != nil {
				t.Fatalf("resolveMatrixExecution() error = %v, want maximum duration to be accepted", err)
			}
			want := time.Duration(maxRetryBackoffMS) * time.Millisecond
			if backoff != want {
				t.Fatalf("resolved backoff = %s, want %s", backoff, want)
			}
		})
	}
}

func TestRunMatrixRejectsRetryBackoffOverflowBeforeExecution(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("retry_backoff_ms cannot represent the duration boundary on 32-bit platforms")
	}

	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	const maxRetryBackoffMS = (1<<63 - 1) / int64(time.Millisecond)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"execution":{"retry_backoff_ms":`+strconv.FormatInt(maxRetryBackoffMS+1, 10)+`},"output":{"raw_dir":"raw","review_dir":"review"}}`)
	plan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	runCalls := 0
	_, err = RunMatrixWithDependencies(context.Background(), matrixPath, plan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(context.Context, *Plan) (*RunResult, error) {
			runCalls++
			return &RunResult{}, nil
		},
	})
	var validationErr *MatrixValidationError
	if err == nil || !errors.As(err, &validationErr) || !strings.Contains(err.Error(), "execution.retry_backoff_ms exceeds maximum duration") {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want retry-backoff overflow validation error", err)
	}
	if runCalls != 0 {
		t.Fatalf("RunPlan calls = %d, want validation to reject before execution", runCalls)
	}
}

func TestLoadMatrixPlanRejectsExplicitNullObjectSections(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "execution", data: `{"version":1,"base_plan":"base.json","devices":[],"locales":[],"appearances":[],"content_variants":[],"execution":null}`},
		{name: "output", data: `{"version":1,"base_plan":"base.json","devices":[],"locales":[],"appearances":[],"content_variants":[],"output":null}`},
		{name: "nested frame", data: `{"version":1,"base_plan":"base.json","devices":[],"locales":[],"appearances":[],"content_variants":[],"output":{"frame":null}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "matrix.json")
			writeMatrixTestFile(t, path, tc.data)
			_, err := LoadMatrixPlan(path)
			if err == nil || !errors.Is(err, ErrMatrixPlanParseJSON) || !strings.Contains(err.Error(), "must be an object") {
				t.Fatalf("LoadMatrixPlan() error = %v, want explicit-null object rejection", err)
			}
		})
	}
}

func TestLoadMatrixPlanRejectsExplicitNullScalarValues(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "raw directory", data: `{"version":1,"base_plan":"base.json","devices":[],"locales":[],"appearances":[],"content_variants":[],"output":{"raw_dir":null}}`},
		{name: "frame enabled", data: `{"version":1,"base_plan":"base.json","devices":[],"locales":[],"appearances":[],"content_variants":[],"output":{"frame":{"enabled":null}}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "matrix.json")
			writeMatrixTestFile(t, path, tc.data)
			_, err := LoadMatrixPlan(path)
			if err == nil || !errors.Is(err, ErrMatrixPlanParseJSON) || !strings.Contains(err.Error(), "must not be null") {
				t.Fatalf("LoadMatrixPlan() error = %v, want explicit-null scalar rejection", err)
			}
		})
	}
}

func TestOpenMatrixOutputRootRejectsChildSwapAfterAnchoring(t *testing.T) {
	dir := t.TempDir()
	selected := filepath.Join(dir, "selected")
	original := filepath.Join(dir, "original")
	replacement := filepath.Join(dir, "replacement")
	if err := os.Mkdir(selected, 0o755); err != nil {
		t.Fatalf("create selected directory: %v", err)
	}
	if err := os.Mkdir(replacement, 0o755); err != nil {
		t.Fatalf("create replacement directory: %v", err)
	}
	var hookErr error
	matrixOutputRootBeforeChildRootForTest = func() {
		if err := os.Rename(selected, original); err != nil {
			hookErr = err
			return
		}
		if err := os.Rename(replacement, selected); err != nil {
			hookErr = err
		}
	}
	t.Cleanup(func() { matrixOutputRootBeforeChildRootForTest = nil })
	_, err := openMatrixOutputRoot(selected)
	if hookErr != nil {
		t.Fatalf("swap hook error: %v", hookErr)
	}
	if err == nil || !strings.Contains(err.Error(), "matrix output root changed") {
		t.Fatalf("openMatrixOutputRoot() error = %v, want anchored identity rejection", err)
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatalf("original directory after rejection: %v", err)
	}
	if _, err := os.Stat(selected); err != nil {
		t.Fatalf("replacement directory after rejection: %v", err)
	}
}

func TestAcquireMatrixSimulatorLockSerializesSameUDID(t *testing.T) {
	first, err := acquireMatrixSimulatorLock(context.Background(), "simulator-a")
	if err != nil {
		t.Fatalf("first simulator lock: %v", err)
	}
	defer func() {
		if err := first(); err != nil {
			t.Errorf("release first simulator lock: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := acquireMatrixSimulatorLock(ctx, "simulator-a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second same-UDID lock error = %v, want deadline", err)
	}
	other, err := acquireMatrixSimulatorLock(context.Background(), "simulator-b")
	if err != nil {
		t.Fatalf("unrelated simulator lock: %v", err)
	}
	if err := other(); err != nil {
		t.Fatalf("release unrelated simulator lock: %v", err)
	}
}

func TestAcquireMatrixSimulatorLockUsesStableNamespaceAcrossTempDirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TMPDIR does not control os.TempDir on Windows")
	}
	firstTemp := t.TempDir()
	secondTemp := t.TempDir()
	key := fmt.Sprintf("stable-lock-%d-%s", os.Getpid(), filepath.Base(firstTemp))
	t.Setenv("TMPDIR", firstTemp)
	first, err := acquireMatrixSimulatorLock(context.Background(), key)
	if err != nil {
		t.Fatalf("first simulator lock: %v", err)
	}
	t.Cleanup(func() {
		if err := first(); err != nil {
			t.Errorf("release first simulator lock: %v", err)
		}
	})
	t.Setenv("TMPDIR", secondTemp)
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	second, err := acquireMatrixSimulatorLock(ctx, key)
	if second != nil {
		if releaseErr := second(); releaseErr != nil {
			t.Errorf("release unexpectedly acquired second simulator lock: %v", releaseErr)
		}
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second simulator lock error = %v, want deadline across temp roots", err)
	}
}

func TestAcquireMatrixSimulatorLockUsesStableNamespaceAcrossHomeDirs(t *testing.T) {
	previousBase := matrixGlobalLockBaseDirForTest
	previousSystemBase := matrixGlobalLockSystemBaseDirForTest
	matrixGlobalLockBaseDirForTest = ""
	matrixGlobalLockSystemBaseDirForTest = t.TempDir()
	t.Cleanup(func() {
		matrixGlobalLockBaseDirForTest = previousBase
		matrixGlobalLockSystemBaseDirForTest = previousSystemBase
	})

	firstHome := t.TempDir()
	secondHome := t.TempDir()
	key := fmt.Sprintf("stable-home-lock-%d", os.Getpid())
	t.Setenv("HOME", firstHome)
	first, err := acquireMatrixSimulatorLock(context.Background(), key)
	if err != nil {
		t.Fatalf("first simulator lock: %v", err)
	}
	t.Cleanup(func() {
		if err := first(); err != nil {
			t.Errorf("release first simulator lock: %v", err)
		}
	})

	t.Setenv("HOME", secondHome)
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	second, err := acquireMatrixSimulatorLock(ctx, key)
	if second != nil {
		if releaseErr := second(); releaseErr != nil {
			t.Errorf("release unexpectedly acquired second simulator lock: %v", releaseErr)
		}
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second simulator lock error = %v, want deadline across home roots", err)
	}
}

func TestNormalizeMatrixLockPathUsesWindowsCaseFoldForMissingRoots(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		want string
	}{
		{name: "drive root", path: `C:\ASC-MATRIX-MISSING\Output`, want: `c:\asc-matrix-missing\output`},
		{name: "UNC root", path: `\\SERVER\Share\ASC-MATRIX-MISSING\Output`, want: `\\server\share\asc-matrix-missing\output`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeMatrixLockPathWithCase(test.path, true); got != test.want {
				t.Fatalf("normalizeMatrixLockPathWithCase(%q, true) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestRunMatrixSerializesSharedArtifactAndReviewPublication(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"review","frame":{"enabled":true,"device_by_matrix_device":{"phone":"iphone-17-pro"}}}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	matrixPlan.Devices[0].UDID = fmt.Sprintf("matrix-lock-%d-%s", os.Getpid(), filepath.Base(dir))
	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseFirst) }) }
	var mu sync.Mutex
	calls := 0
	runPlan := func(ctx context.Context, plan *Plan) (*RunResult, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		if call == 1 {
			close(firstEntered)
			<-releaseFirst
		} else {
			close(secondEntered)
		}
		writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
		return &RunResult{Steps: []RunStepResult{{Index: 1, Action: "screenshot", Status: "ok"}}}, nil
	}
	appearance := &matrixTestAppearance{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	frame := func(ctx context.Context, request FrameRequest) (*FrameResult, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		writeMatrixPNG(t, request.OutputPath)
		return &FrameResult{}, nil
	}
	firstDone := make(chan error, 1)
	go func() {
		_, err := RunMatrixWithDependencies(ctx, matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{RunPlan: runPlan, Frame: frame, Appearance: appearance})
		firstDone <- err
	}()
	select {
	case <-firstEntered:
	case firstErr := <-firstDone:
		t.Fatalf("first RunMatrixWithDependencies() returned before execution gate: %v", firstErr)
	case <-time.After(5 * time.Second):
		cancel()
		release()
		select {
		case <-firstDone:
		case <-time.After(time.Second):
		}
		t.Fatal("first RunMatrixWithDependencies() did not reach execution gate")
	}
	secondDone := make(chan error, 1)
	go func() {
		_, err := RunMatrixWithDependencies(ctx, matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{RunPlan: runPlan, Frame: frame, Appearance: appearance})
		secondDone <- err
	}()
	select {
	case <-secondEntered:
		release()
		t.Fatal("second matrix run entered execution before first publication completed")
	case <-time.After(125 * time.Millisecond):
	}
	release()
	if err := <-firstDone; err != nil {
		t.Fatalf("first RunMatrixWithDependencies() error = %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second RunMatrixWithDependencies() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("run plan calls = %d, want two serialized runs", calls)
	}
}

func TestMatrixReviewConsumersRejectHTMLSymlink(t *testing.T) {
	dir := t.TempDir()
	result := &MatrixResult{PlanPath: "plan.json", Cells: []MatrixCellResult{{ID: "generation", Status: MatrixCellSuccess}}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: dir}); err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	original := filepath.Join(dir, "original.html")
	if err := os.Rename(filepath.Join(dir, "index.html"), original); err != nil {
		t.Fatalf("rename HTML: %v", err)
	}
	if err := os.Symlink(original, filepath.Join(dir, "index.html")); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if _, err := LoadMatrixReviewManifest(manifestPath); !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("LoadMatrixReviewManifest() error = %v, want stable pair mismatch", err)
	}
	if _, err := OpenReview(context.Background(), ReviewOpenRequest{OutputDir: dir, DryRun: true}); !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("OpenReview() error = %v, want stable pair mismatch", err)
	}
}

func TestRunMatrixCanceledContextBoundsReviewLockWait(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	reviewDir := filepath.Join(dir, "review")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review"}}`)
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	reviewRoot, err := openMatrixOutputRoot(reviewDir)
	if err != nil {
		t.Fatalf("open review root: %v", err)
	}
	defer func() {
		if err := reviewRoot.Close(); err != nil {
			t.Errorf("close review root: %v", err)
		}
	}()
	release, err := acquireMatrixReviewLock(context.Background(), reviewRoot)
	if err != nil {
		t.Fatalf("acquire held review lock: %v", err)
	}
	defer func() {
		if err := release(); err != nil {
			t.Errorf("release held review lock: %v", err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runPlanCalled := false
	done := make(chan error, 1)
	started := time.Now()
	go func() {
		_, runErr := RunMatrixWithDependencies(ctx, matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
			RunPlan: func(context.Context, *Plan) (*RunResult, error) {
				runPlanCalled = true
				return &RunResult{}, nil
			},
			Appearance: &matrixTestAppearance{},
		})
		done <- runErr
	}()
	select {
	case <-done:
		if time.Since(started) > time.Second {
			t.Fatalf("canceled RunMatrixWithDependencies() waited %s for review lock", time.Since(started))
		}
	case <-time.After(time.Second):
		t.Fatal("canceled RunMatrixWithDependencies() waited indefinitely for review lock")
	}
	if runPlanCalled {
		t.Fatal("canceled RunMatrixWithDependencies() invoked the screenshot adapter")
	}
}

type matrixLockSpanAppearance struct {
	mu                  sync.Mutex
	snapshots           int
	restores            int
	firstRestoreEntered chan struct{}
	secondSnapshot      chan struct{}
	releaseFirstRestore chan struct{}
	restoreEnteredOnce  sync.Once
	secondSnapshotOnce  sync.Once
}

func (a *matrixLockSpanAppearance) Snapshot(context.Context, string) (string, error) {
	a.mu.Lock()
	a.snapshots++
	snapshotNumber := a.snapshots
	a.mu.Unlock()
	if snapshotNumber == 2 {
		a.secondSnapshotOnce.Do(func() { close(a.secondSnapshot) })
	}
	return "light", nil
}

func (*matrixLockSpanAppearance) Set(context.Context, string, string) error {
	return errors.New("set failed")
}

func (a *matrixLockSpanAppearance) Restore(context.Context, string, string) error {
	a.mu.Lock()
	a.restores++
	restoreNumber := a.restores
	a.mu.Unlock()
	if restoreNumber == 1 {
		a.restoreEnteredOnce.Do(func() { close(a.firstRestoreEntered) })
		<-a.releaseFirstRestore
	}
	return nil
}

func TestExecuteMatrixCellSimulatorLockSpansAppearanceRestore(t *testing.T) {
	appearance := &matrixLockSpanAppearance{
		firstRestoreEntered: make(chan struct{}),
		secondSnapshot:      make(chan struct{}),
		releaseFirstRestore: make(chan struct{}),
	}
	cell := MatrixCell{ID: "cell", UDID: "matrix-lock-span", Appearance: "dark"}
	firstDone := make(chan MatrixCellResult, 1)
	go func() {
		firstDone <- executeMatrixCell(context.Background(), cell, nil, nil, 1, 0, MatrixDependencies{Appearance: appearance}, matrixOutputRoots{}, &matrixSimulatorGuard{})
	}()
	select {
	case <-appearance.firstRestoreEntered:
	case <-time.After(time.Second):
		t.Fatal("first cell did not reach appearance restore")
	}

	secondDone := make(chan MatrixCellResult, 1)
	go func() {
		secondDone <- executeMatrixCell(context.Background(), cell, nil, nil, 1, 0, MatrixDependencies{Appearance: appearance}, matrixOutputRoots{}, &matrixSimulatorGuard{})
	}()
	secondEntered := false
	select {
	case <-appearance.secondSnapshot:
		secondEntered = true
	case <-time.After(125 * time.Millisecond):
	}
	close(appearance.releaseFirstRestore)
	first := <-firstDone
	second := <-secondDone
	if secondEntered {
		t.Fatal("second cell entered appearance before first restore completed")
	}
	if first.FailureCode != "set_failed" || second.FailureCode != "set_failed" {
		t.Fatalf("unexpected cell results: first=%+v second=%+v", first, second)
	}
	appearance.mu.Lock()
	snapshots, restores := appearance.snapshots, appearance.restores
	appearance.mu.Unlock()
	if snapshots != 2 || restores != 2 {
		t.Fatalf("appearance calls = snapshots %d, restores %d; want two of each", snapshots, restores)
	}
}

func TestGenerateMatrixReviewUsesLiveLockContext(t *testing.T) {
	dir := t.TempDir()
	heldRoot, err := openMatrixOutputRoot(dir)
	if err != nil {
		t.Fatalf("open held review root: %v", err)
	}
	defer func() {
		if err := heldRoot.Close(); err != nil {
			t.Errorf("close held review root: %v", err)
		}
	}()
	releaseHeld, err := acquireMatrixReviewLock(context.Background(), heldRoot)
	if err != nil {
		t.Fatalf("acquire held review lock: %v", err)
	}
	defer func() {
		if err := releaseHeld(); err != nil {
			t.Errorf("release held review lock: %v", err)
		}
	}()

	lockCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, reviewErr := GenerateMatrixReview(context.Background(), MatrixReviewRequest{
			Result:      &MatrixResult{PlanPath: "plan.json", Cells: []MatrixCellResult{{ID: "cell", Status: MatrixCellSuccess}}},
			OutputDir:   dir,
			LockContext: lockCtx,
		})
		done <- reviewErr
	}()
	time.Sleep(3 * matrixReviewLockPollInterval)
	cancel()
	select {
	case reviewErr := <-done:
		if !errors.Is(reviewErr, context.Canceled) {
			t.Fatalf("GenerateMatrixReview() error = %v, want live lock cancellation", reviewErr)
		}
	case <-time.After(time.Second):
		if err := releaseHeld(); err != nil {
			t.Errorf("release held review lock after timeout: %v", err)
		}
		<-done
		t.Fatal("GenerateMatrixReview() did not observe live lock cancellation")
	}
}

func TestMatrixOutputLockIdentityUsesFilesystemCaseSemantics(t *testing.T) {
	dir := t.TempDir()
	if !matrixFilesystemCaseInsensitive(dir) {
		t.Skip("selected filesystem is case-sensitive")
	}
	upper := filepath.Join(dir, "Output", "raw")
	lower := filepath.Join(dir, "output", "raw")
	upperIdentity := matrixOutputLockIdentity(upper)
	lowerIdentity := matrixOutputLockIdentity(lower)
	if upperIdentity != lowerIdentity {
		t.Fatalf("case-variant output lock identities differ: %q != %q", upperIdentity, lowerIdentity)
	}
}

func TestAcquireMatrixOutputLocksUsesOpenedFilesystemIdentity(t *testing.T) {
	parent := t.TempDir()
	realPath := filepath.Join(parent, "real")
	aliasPath := filepath.Join(parent, "alias")
	if err := os.Mkdir(realPath, 0o755); err != nil {
		t.Fatalf("create real output root: %v", err)
	}
	if err := os.Symlink(realPath, aliasPath); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	realRoot, err := rootfs.New(realPath)
	if err != nil {
		t.Fatalf("open real output root: %v", err)
	}
	defer realRoot.Close()
	aliasRoot, err := rootfs.New(aliasPath)
	if err != nil {
		t.Fatalf("open aliased output root: %v", err)
	}
	defer aliasRoot.Close()
	release, err := acquireMatrixOutputLocksForRoots(context.Background(), []matrixOutputLockTarget{
		{root: realRoot},
	})
	if err != nil {
		t.Fatalf("acquire real output lock: %v", err)
	}
	defer func() {
		if err := release(); err != nil {
			t.Errorf("release real output lock: %v", err)
		}
	}()
	if err := os.Chmod(realPath, 0o700); err != nil {
		t.Skipf("metadata mutation unavailable on this platform: %v", err)
	}
	if err := os.Mkdir(filepath.Join(realPath, "metadata-child"), 0o700); err != nil {
		t.Skipf("directory mutation unavailable on this platform: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	if _, err := acquireMatrixOutputLocksForRoots(ctx, []matrixOutputLockTarget{{root: aliasRoot}}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("acquire aliased output lock error = %v, want deadline across one filesystem identity", err)
	}
}

func TestAcquireMatrixOutputLocksClosesEarlierOpenedRootsOnLaterFailure(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first")
	second := filepath.Join(dir, "second")
	if err := os.Mkdir(first, 0o755); err != nil {
		t.Fatalf("create first output root: %v", err)
	}
	if err := os.WriteFile(second, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create invalid second output path: %v", err)
	}
	var firstOpened *os.Root
	previousHook := matrixOutputLockRootOpenedForTest
	matrixOutputLockRootOpenedForTest = func(opened *os.Root) {
		if firstOpened == nil {
			firstOpened = opened
		}
	}
	t.Cleanup(func() { matrixOutputLockRootOpenedForTest = previousHook })
	if _, err := acquireMatrixOutputLocks(context.Background(), []string{first, second}); err == nil {
		t.Fatal("acquireMatrixOutputLocks() error = nil, want later path failure")
	}
	if firstOpened == nil {
		t.Fatal("first output root was not observed")
	}
	if _, err := firstOpened.Stat("."); err == nil {
		t.Fatal("earlier opened root remains usable after later path failure")
	}
}

func TestRunMatrixKeepsExecutionRootsPinnedAfterLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable while the rooted handle is open on Windows")
	}
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	rawDir := filepath.Join(dir, "raw")
	rawOriginal := filepath.Join(dir, "raw-original")
	rawReplacement := filepath.Join(dir, "raw-replacement")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"PINNED-ROOT-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review"}}`)
	if err := os.Mkdir(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	if err := os.Mkdir(rawReplacement, 0o755); err != nil {
		t.Fatalf("create replacement raw directory: %v", err)
	}
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	previousHook := matrixOutputLocksAcquiredForTest
	var swapErr error
	matrixOutputLocksAcquiredForTest = func() {
		if err := os.Rename(rawDir, rawOriginal); err != nil {
			swapErr = err
			return
		}
		if err := os.Rename(rawReplacement, rawDir); err != nil {
			swapErr = err
		}
	}
	t.Cleanup(func() { matrixOutputLocksAcquiredForTest = previousHook })
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Appearance: &matrixTestAppearance{},
	})
	if swapErr != nil {
		t.Fatalf("swap output root: %v", swapErr)
	}
	if runErr == nil {
		t.Fatalf("RunMatrixWithDependencies() error = nil, want fail-closed root replacement")
	}
	if result == nil || len(result.Cells) != 1 {
		t.Fatalf("RunMatrixWithDependencies() result = %+v, want one partial cell", result)
	}
	replacementArtifact := filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png")
	if _, err := os.Stat(replacementArtifact); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replacement artifact stat error = %v, want no write through reopened root", err)
	}
	if result.Cells[0].Status == MatrixCellSuccess {
		t.Fatalf("cell status = %q, want fail-closed root replacement", result.Cells[0].Status)
	}
	if _, err := os.Stat(filepath.Join(dir, "review", "manifest.json")); err != nil {
		t.Fatalf("partial review manifest stat error = %v, want review publication to remain available", err)
	}
}

func TestExecuteMatrixCellAttemptRejectsReplacedRawRootBeforeFraming(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows temporary volumes do not reliably distinguish a replaced raw directory identity")
	}
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	framedDir := filepath.Join(dir, "framed")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw output root: %v", err)
	}
	if err := os.MkdirAll(framedDir, 0o755); err != nil {
		t.Fatalf("create framed output root: %v", err)
	}
	rawRoot, err := rootfs.New(rawDir)
	if err != nil {
		t.Fatalf("open raw output root: %v", err)
	}
	defer rawRoot.Close()
	framedRoot, err := rootfs.New(framedDir)
	if err != nil {
		t.Fatalf("open framed output root: %v", err)
	}
	defer framedRoot.Close()
	cell := MatrixCell{
		ID:          "phone|en-US|light|default",
		Device:      "phone",
		UDID:        "SIM-UDID",
		RawDir:      filepath.Join(rawDir, "en-US", "phone", "light", "default"),
		FramedDir:   filepath.Join(framedDir, "en-US", "phone", "light", "default"),
		RawPaths:    []string{filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png")},
		FramedPaths: []string{filepath.Join(framedDir, "en-US", "phone", "light", "default", "home.png")},
	}
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	matrixPlan := &MatrixPlan{Output: MatrixOutput{Frame: MatrixFrame{Enabled: true, DeviceByMatrixDevice: map[string]string{"phone": "iphone-17-pro"}}}}
	creates := 0
	previous := matrixPrivateAttemptParentCreatedForTest
	matrixPrivateAttemptParentCreatedForTest = func(string) {
		creates++
		if creates != 2 {
			return
		}
		original := rawDir + "-original"
		if err := os.Rename(rawDir, original); err != nil {
			t.Errorf("rename raw root: %v", err)
			return
		}
		if err := os.Mkdir(rawDir, 0o700); err != nil {
			t.Errorf("replace raw root: %v", err)
		}
	}
	t.Cleanup(func() { matrixPrivateAttemptParentCreatedForTest = previous })
	_, attemptErr := executeMatrixCellAttempt(context.Background(), cell, base, matrixPlan, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			if err := writeMatrixPNGFile(filepath.Join(plan.App.OutputDir, "home.png")); err != nil {
				return nil, err
			}
			return &RunResult{}, nil
		},
		Frame: func(context.Context, FrameRequest) (*FrameResult, error) {
			t.Fatal("Frame should not run after the retained raw root was replaced")
			return nil, nil
		},
	}, matrixOutputRoots{raw: rawRoot, rawPath: rawDir, framed: framedRoot, framedPath: framedDir, hasFramed: true})
	if attemptErr == nil {
		t.Fatal("executeMatrixCellAttempt() error = nil, want replaced raw root failure")
	}
}

func TestMatrixAttemptProvidersUsePrivatePinnedDestinations(t *testing.T) {
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	framedDir := filepath.Join(dir, "framed")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw output root: %v", err)
	}
	if err := os.MkdirAll(framedDir, 0o755); err != nil {
		t.Fatalf("create framed output root: %v", err)
	}
	rawRoot, err := rootfs.New(rawDir)
	if err != nil {
		t.Fatalf("open raw output root: %v", err)
	}
	defer rawRoot.Close()
	framedRoot, err := rootfs.New(framedDir)
	if err != nil {
		t.Fatalf("open framed output root: %v", err)
	}
	defer framedRoot.Close()

	cell := MatrixCell{
		ID:          "phone|en-US|light|default",
		Device:      "phone",
		UDID:        "SIM-UDID",
		Locale:      "en-US",
		Appearance:  "light",
		Content:     "default",
		RawDir:      filepath.Join(rawDir, "en-US", "phone", "light", "default"),
		FramedDir:   filepath.Join(framedDir, "en-US", "phone", "light", "default"),
		RawPaths:    []string{filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png")},
		FramedPaths: []string{filepath.Join(framedDir, "en-US", "phone", "light", "default", "home.png")},
	}
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	matrixPlan := &MatrixPlan{Output: MatrixOutput{Frame: MatrixFrame{Enabled: true, DeviceByMatrixDevice: map[string]string{"phone": "iphone-17-pro"}}}}
	inside := func(root, path string) bool {
		relative, err := filepath.Rel(root, path)
		return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
	}
	runChecked, frameChecked := false, false
	deps := MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			if inside(rawDir, plan.App.OutputDir) {
				t.Errorf("capture output directory %q remains under selected raw root %q", plan.App.OutputDir, rawDir)
			}
			info, statErr := os.Stat(plan.App.OutputDir)
			if statErr != nil {
				t.Errorf("stat private capture output directory: %v", statErr)
			} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
				t.Errorf("capture output directory mode = %o, want 700", info.Mode().Perm())
			}
			runChecked = true
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Frame: func(_ context.Context, request FrameRequest) (*FrameResult, error) {
			frameDir := filepath.Dir(request.OutputPath)
			if inside(framedDir, frameDir) {
				t.Errorf("framing output directory %q remains under selected framed root %q", frameDir, framedDir)
			}
			info, statErr := os.Stat(frameDir)
			if statErr != nil {
				t.Errorf("stat private framing output directory: %v", statErr)
			} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
				t.Errorf("framing output directory mode = %o, want 700", info.Mode().Perm())
			}
			frameChecked = true
			writeMatrixPNG(t, request.OutputPath)
			return &FrameResult{}, nil
		},
	}
	if _, err := executeMatrixCellAttempt(context.Background(), cell, base, matrixPlan, deps, matrixOutputRoots{
		raw: rawRoot, rawPath: rawDir, framed: framedRoot, framedPath: framedDir, hasFramed: true,
	}); err != nil {
		t.Fatalf("executeMatrixCellAttempt() error = %v", err)
	}
	if !runChecked || !frameChecked {
		t.Fatalf("provider checks = capture:%t frame:%t, want both providers invoked", runChecked, frameChecked)
	}
}

func TestCaptureWithRootPublishesThroughPinnedDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	dir := t.TempDir()
	destinationPath := filepath.Join(dir, "destination")
	outsidePath := filepath.Join(dir, "outside")
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := os.MkdirAll(outsidePath, 0o755); err != nil {
		t.Fatalf("create outside: %v", err)
	}
	destination, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination: %v", err)
	}
	defer destination.Close()
	originalPath := destinationPath + "-original"
	previousHook := matrixCaptureRootBeforePublishForTest
	matrixCaptureRootBeforePublishForTest = func(string) {
		if err := os.Rename(destinationPath, originalPath); err != nil {
			t.Fatalf("rename destination: %v", err)
		}
		if err := os.Symlink(outsidePath, destinationPath); err != nil {
			t.Fatalf("replace destination: %v", err)
		}
	}
	t.Cleanup(func() { matrixCaptureRootBeforePublishForTest = previousHook })
	provider := ProviderFunc(func(_ context.Context, request CaptureRequest) (string, error) {
		path := filepath.Join(request.OutputDir, request.Name+".png")
		writeMinimalPNG(t, path, 100, 200)
		return path, nil
	})
	result, err := captureWithRootProvider(context.Background(), CaptureRequest{Name: "home", OutputDir: destinationPath}, destination, provider)
	if err == nil {
		t.Fatalf("captureWithRootProvider() error = nil, want fail-closed destination replacement")
	}
	if result != nil {
		t.Fatalf("capture result = %+v, want no result after destination replacement", result)
	}
	if _, err := os.Stat(filepath.Join(originalPath, "home.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pinned destination artifact stat error = %v, want no publication after replacement", err)
	}
	if _, err := os.Stat(filepath.Join(outsidePath, "home.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("capture wrote through replaced destination: %v", err)
	}
}

func TestCaptureWithRootRejectsProviderScratchReplacementBeforeWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	dir := t.TempDir()
	destinationPath := filepath.Join(dir, "destination")
	outsidePath := filepath.Join(dir, "outside")
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := os.MkdirAll(outsidePath, 0o755); err != nil {
		t.Fatalf("create outside: %v", err)
	}
	sentinelPath := filepath.Join(outsidePath, "sentinel")
	if err := os.WriteFile(sentinelPath, []byte("untouched"), 0o600); err != nil {
		t.Fatalf("write outside sentinel: %v", err)
	}
	destination, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination: %v", err)
	}
	defer destination.Close()
	var replacedScratch string
	provider := ProviderFunc(func(_ context.Context, request CaptureRequest) (string, error) {
		replacedScratch = request.OutputDir
		original := request.OutputDir + "-original"
		if err := os.Rename(request.OutputDir, original); err != nil {
			return "", fmt.Errorf("rename capture scratch: %w", err)
		}
		if err := os.Symlink(outsidePath, request.OutputDir); err != nil {
			return "", fmt.Errorf("replace capture scratch: %w", err)
		}
		path := filepath.Join(request.OutputDir, request.Name+".png")
		writeMinimalPNG(t, path, 100, 200)
		return path, nil
	})
	t.Cleanup(func() {
		if replacedScratch != "" {
			if info, statErr := os.Lstat(replacedScratch); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
				_ = os.Remove(replacedScratch)
			}
		}
	})
	result, err := captureWithRootProvider(context.Background(), CaptureRequest{Name: "home", OutputDir: destinationPath}, destination, provider)
	if err == nil {
		t.Fatal("captureWithRootProvider() error = nil, want provider scratch replacement rejection")
	}
	if result != nil {
		t.Fatalf("capture result = %+v, want no result after provider scratch replacement", result)
	}
	if got, readErr := os.ReadFile(sentinelPath); readErr != nil || string(got) != "untouched" {
		t.Fatalf("outside sentinel = %q, read error = %v, want unchanged", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(outsidePath, "home.png")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("capture wrote through replaceable scratch path: %v", statErr)
	}
}

func TestCaptureWithRootRejectsFinalSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("final symlink fixtures require elevated privileges on Windows")
	}
	dir := t.TempDir()
	destinationPath := filepath.Join(dir, "destination")
	outsidePath := filepath.Join(dir, "outside")
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := os.MkdirAll(outsidePath, 0o755); err != nil {
		t.Fatalf("create outside: %v", err)
	}
	sentinelPath := filepath.Join(outsidePath, "sentinel")
	if err := os.WriteFile(sentinelPath, []byte("untouched"), 0o600); err != nil {
		t.Fatalf("write outside sentinel: %v", err)
	}
	destination, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination: %v", err)
	}
	defer destination.Close()
	previousHook := matrixCaptureRootBeforePublishForTest
	matrixCaptureRootBeforePublishForTest = func(outputPath string) {
		if err := os.Symlink(sentinelPath, outputPath); err != nil {
			t.Fatalf("replace final destination with symlink: %v", err)
		}
	}
	t.Cleanup(func() { matrixCaptureRootBeforePublishForTest = previousHook })
	provider := ProviderFunc(func(_ context.Context, request CaptureRequest) (string, error) {
		path := filepath.Join(request.OutputDir, request.Name+".png")
		writeMinimalPNG(t, path, 100, 200)
		return path, nil
	})
	result, err := captureWithRootProvider(context.Background(), CaptureRequest{Name: "home", OutputDir: destinationPath}, destination, provider)
	if err == nil {
		t.Fatal("captureWithRootProvider() error = nil, want final symlink rejection")
	}
	if result != nil {
		t.Fatalf("capture result = %+v, want no result after final symlink rejection", result)
	}
	if got, readErr := os.ReadFile(sentinelPath); readErr != nil || string(got) != "untouched" {
		t.Fatalf("outside sentinel = %q, read error = %v, want unchanged", got, readErr)
	}
	info, statErr := os.Lstat(filepath.Join(destinationPath, "home.png"))
	if statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("final destination = %+v, stat error = %v, want preserved symlink", info, statErr)
	}
}

func TestCaptureWithRootHonorsCancellationDuringPublish(t *testing.T) {
	dir := t.TempDir()
	destinationPath := filepath.Join(dir, "destination")
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	destination, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination: %v", err)
	}
	defer destination.Close()
	ctx, cancel := context.WithCancel(context.Background())
	previousHook := matrixCaptureRootBeforePublishForTest
	matrixCaptureRootBeforePublishForTest = func(string) {
		cancel()
	}
	t.Cleanup(func() { matrixCaptureRootBeforePublishForTest = previousHook })
	provider := ProviderFunc(func(_ context.Context, request CaptureRequest) (string, error) {
		path := filepath.Join(request.OutputDir, request.Name+".png")
		writeMinimalPNG(t, path, 100, 200)
		return path, nil
	})
	result, err := captureWithRootProvider(ctx, CaptureRequest{Name: "home", OutputDir: destinationPath}, destination, provider)
	if err == nil {
		t.Fatal("captureWithRootProvider() error = nil, want canceled publish")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("captureWithRootProvider() error = %v, want context.Canceled", err)
	}
	if result != nil {
		t.Fatalf("capture result = %+v, want no result after canceled publish", result)
	}
}

// ProviderFunc adapts a function to the screenshot Provider contract for
// rooted publication tests.
type ProviderFunc func(context.Context, CaptureRequest) (string, error)

func (f ProviderFunc) Capture(ctx context.Context, request CaptureRequest) (string, error) {
	return f(ctx, request)
}

func TestRunPlanWithRootUsesPinnedCaptureScratch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	dir := t.TempDir()
	destinationPath := filepath.Join(dir, "destination")
	outsidePath := filepath.Join(dir, "outside")
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := os.MkdirAll(outsidePath, 0o755); err != nil {
		t.Fatalf("create outside: %v", err)
	}
	sentinelPath := filepath.Join(outsidePath, "sentinel")
	if err := os.WriteFile(sentinelPath, []byte("untouched"), 0o600); err != nil {
		t.Fatalf("write outside sentinel: %v", err)
	}
	templatePath := filepath.Join(dir, "template.png")
	writeMinimalPNG(t, templatePath, 100, 200)

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
set -eu
`)
	writeExecutable(t, filepath.Join(binDir, "axe"), `#!/bin/sh
set -eu
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output" ]; then out="$2"; break; fi
  shift
done
cp "$AXE_TEMPLATE_PNG" "$out"
scratch=$(dirname "$out")
mv "$scratch" "$scratch-original"
ln -s "$MATRIX_OUTSIDE" "$scratch"
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AXE_TEMPLATE_PNG", templatePath)
	t.Setenv("MATRIX_OUTSIDE", outsidePath)

	root, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination root: %v", err)
	}
	defer root.Close()
	name := "home"
	plan := &Plan{
		Version: 1,
		App:     PlanApp{BundleID: "com.example.app", OutputDir: destinationPath},
		Steps:   []PlanStep{{Action: ActionScreenshot, Name: &name}},
	}
	result, err := runPlanWithRoot(context.Background(), plan, root)
	if err == nil {
		t.Fatalf("runPlanWithRoot() error = nil, want swapped capture scratch rejection")
	}
	if result == nil || len(result.Steps) != 1 || result.Steps[0].Status != "error" {
		t.Fatalf("runPlanWithRoot() result = %+v, want failed screenshot step", result)
	}
	if got, readErr := os.ReadFile(sentinelPath); readErr != nil || string(got) != "untouched" {
		t.Fatalf("outside sentinel = %q, read error = %v, want unchanged", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(destinationPath, "home.png")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination artifact stat error = %v, want no publication", statErr)
	}
}

func TestFrameIntoRootUsesPinnedKoubouScratch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	resetKoubouVersionCacheForTest()
	dir := t.TempDir()
	destinationPath := filepath.Join(dir, "destination")
	outsidePath := filepath.Join(dir, "outside")
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := os.MkdirAll(outsidePath, 0o755); err != nil {
		t.Fatalf("create outside: %v", err)
	}
	sentinelPath := filepath.Join(outsidePath, "sentinel")
	if err := os.WriteFile(sentinelPath, []byte("untouched"), 0o600); err != nil {
		t.Fatalf("write outside sentinel: %v", err)
	}
	rawPath := filepath.Join(dir, "raw.png")
	writeMinimalPNG(t, rawPath, 200, 300)
	templatePath := filepath.Join(dir, "template.png")
	writeMinimalPNG(t, templatePath, 2880, 1800)

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "kou"), `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then
  echo "kou 0.18.1"
  exit 0
fi
if [ "$1" != "generate" ]; then
  exit 1
fi
work=$(dirname "$2")
mkdir -p "$work/output"
cp "$KOU_TEMPLATE_PNG" "$work/output/framed.png"
mv "$work" "$work-original"
ln -s "$MATRIX_OUTSIDE" "$work"
printf '[{"name":"framed","path":"output/framed.png","success":true}]'
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KOU_TEMPLATE_PNG", templatePath)
	t.Setenv("MATRIX_OUTSIDE", outsidePath)

	root, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination root: %v", err)
	}
	defer root.Close()
	result, err := frameIntoRoot(context.Background(), FrameRequest{
		InputPath:  rawPath,
		OutputPath: filepath.Join(destinationPath, "home.png"),
		Device:     string(FrameDeviceMac),
	}, root)
	if err == nil {
		t.Fatalf("frameIntoRoot() error = nil, want swapped Koubou scratch rejection")
	}
	if result != nil {
		t.Fatalf("frameIntoRoot() result = %+v, want no result", result)
	}
	if got, readErr := os.ReadFile(sentinelPath); readErr != nil || string(got) != "untouched" {
		t.Fatalf("outside sentinel = %q, read error = %v, want unchanged", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(destinationPath, "home.png")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination artifact stat error = %v, want no publication", statErr)
	}
}

func TestFrameIntoRootRejectsKoubouScratchReplacementBeforeWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	resetKoubouVersionCacheForTest()
	dir := t.TempDir()
	destinationPath := filepath.Join(dir, "destination")
	outsidePath := filepath.Join(dir, "outside")
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := os.MkdirAll(outsidePath, 0o755); err != nil {
		t.Fatalf("create outside: %v", err)
	}
	sentinelPath := filepath.Join(outsidePath, "sentinel")
	if err := os.WriteFile(sentinelPath, []byte("untouched"), 0o600); err != nil {
		t.Fatalf("write outside sentinel: %v", err)
	}
	rawPath := filepath.Join(dir, "raw.png")
	writeMinimalPNG(t, rawPath, 200, 300)
	templatePath := filepath.Join(dir, "template.png")
	writeMinimalPNG(t, templatePath, 2880, 1800)

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "kou"), `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then
  echo "kou 0.18.1"
  exit 0
fi
if [ "$1" != "generate" ]; then
  exit 1
fi
work=$(dirname "$2")
mv "$work" "$work-original"
ln -s "$MATRIX_OUTSIDE" "$work"
mkdir -p "$MATRIX_OUTSIDE/output"
cp "$KOU_TEMPLATE_PNG" "$work/output/framed.png"
printf '[{"name":"framed","path":"output/framed.png","success":true}]'
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KOU_TEMPLATE_PNG", templatePath)
	t.Setenv("MATRIX_OUTSIDE", outsidePath)
	previous := matrixFrameWorkRootBeforeReadForTest
	var replacedWork string
	matrixFrameWorkRootBeforeReadForTest = func(path string) {
		replacedWork = path
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(path); err != nil {
				t.Errorf("remove replaced Koubou work root: %v", err)
			}
		}
	}
	t.Cleanup(func() {
		matrixFrameWorkRootBeforeReadForTest = previous
		if replacedWork != "" {
			if info, err := os.Lstat(replacedWork); err == nil && info.Mode()&os.ModeSymlink != 0 {
				_ = os.Remove(replacedWork)
			}
		}
	})

	root, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination root: %v", err)
	}
	defer root.Close()
	result, err := frameIntoRoot(context.Background(), FrameRequest{
		InputPath:  rawPath,
		OutputPath: filepath.Join(destinationPath, "home.png"),
		Device:     string(FrameDeviceMac),
	}, root)
	if err == nil {
		t.Fatal("frameIntoRoot() error = nil, want provider scratch replacement rejection")
	}
	if result != nil {
		t.Fatalf("frameIntoRoot() result = %+v, want no result after provider scratch replacement", result)
	}
	if got, readErr := os.ReadFile(sentinelPath); readErr != nil || string(got) != "untouched" {
		t.Fatalf("outside sentinel = %q, read error = %v, want unchanged", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(outsidePath, "output", "framed.png")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Koubou wrote through replaceable scratch path: %v", statErr)
	}
}

func TestFrameIntoRootRejectsKoubouNestedOutputReplacementBeforeWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	resetKoubouVersionCacheForTest()
	dir := t.TempDir()
	destinationPath := filepath.Join(dir, "destination")
	outsidePath := filepath.Join(dir, "outside")
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := os.MkdirAll(outsidePath, 0o755); err != nil {
		t.Fatalf("create outside: %v", err)
	}
	sentinelPath := filepath.Join(outsidePath, "sentinel")
	if err := os.WriteFile(sentinelPath, []byte("untouched"), 0o600); err != nil {
		t.Fatalf("write outside sentinel: %v", err)
	}
	rawPath := filepath.Join(dir, "raw.png")
	writeMinimalPNG(t, rawPath, 200, 300)
	templatePath := filepath.Join(dir, "template.png")
	writeMinimalPNG(t, templatePath, 2880, 1800)

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "kou"), `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then
  echo "kou 0.18.1"
  exit 0
fi
if [ "$1" != "generate" ]; then
  exit 1
fi
work=$(dirname "$2")
mv "$work/output" "$work/output-original"
ln -s "$MATRIX_OUTSIDE" "$work/output"
mkdir -p "$MATRIX_OUTSIDE"
cp "$KOU_TEMPLATE_PNG" "$work/output/framed.png"
printf '[{"name":"framed","path":"output/framed.png","success":true}]'
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KOU_TEMPLATE_PNG", templatePath)
	t.Setenv("MATRIX_OUTSIDE", outsidePath)

	root, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination root: %v", err)
	}
	defer root.Close()
	result, err := frameIntoRoot(context.Background(), FrameRequest{
		InputPath:  rawPath,
		OutputPath: filepath.Join(destinationPath, "home.png"),
		Device:     string(FrameDeviceMac),
	}, root)
	if err == nil {
		t.Fatal("frameIntoRoot() error = nil, want nested output replacement rejection")
	}
	if result != nil {
		t.Fatalf("frameIntoRoot() result = %+v, want no result", result)
	}
	if got, readErr := os.ReadFile(sentinelPath); readErr != nil || string(got) != "untouched" {
		t.Fatalf("outside sentinel = %q, read error = %v, want unchanged", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(outsidePath, "framed.png")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Koubou wrote through replaceable nested output: %v", statErr)
	}
}

func TestFrameIntoRootRejectsSymlinkedInputBeforeKoubou(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires elevated permissions on Windows")
	}
	dir := t.TempDir()
	destinationPath := filepath.Join(dir, "destination")
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	inputPath := filepath.Join(dir, "input.png")
	originalPath := inputPath + "-original"
	outsidePath := filepath.Join(dir, "outside.png")
	writeMinimalPNG(t, inputPath, 200, 300)
	writeMinimalPNG(t, outsidePath, 200, 300)
	outsideBefore, err := os.Stat(outsidePath)
	if err != nil {
		t.Fatalf("stat outside input before replacement: %v", err)
	}
	previous := matrixFrameInputBeforeCopyForTest
	called := false
	matrixFrameInputBeforeCopyForTest = func(path string) {
		called = true
		if err := os.Rename(path, originalPath); err != nil {
			t.Errorf("rename input: %v", err)
			return
		}
		if err := os.Symlink(outsidePath, path); err != nil {
			t.Errorf("replace input with symlink: %v", err)
		}
	}
	t.Cleanup(func() {
		matrixFrameInputBeforeCopyForTest = previous
		_ = os.Remove(inputPath)
		_ = os.Remove(originalPath)
	})
	root, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination root: %v", err)
	}
	defer root.Close()
	result, err := frameIntoRoot(context.Background(), FrameRequest{
		InputPath:  inputPath,
		OutputPath: filepath.Join(destinationPath, "home.png"),
		Device:     string(FrameDeviceMac),
	}, root)
	if !called {
		t.Fatal("input replacement seam was not called")
	}
	if err == nil {
		t.Fatal("frameIntoRoot() error = nil, want symlinked input rejection")
	}
	if result != nil {
		t.Fatalf("frameIntoRoot() result = %+v, want no result", result)
	}
	if _, statErr := os.Lstat(filepath.Join(destinationPath, "home.png")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination stat error = %v, want no publication", statErr)
	}
	outsideAfter, err := os.Stat(outsidePath)
	if err != nil {
		t.Fatalf("stat outside input after replacement: %v", err)
	}
	if outsideAfter.Mode().Perm() != outsideBefore.Mode().Perm() {
		t.Fatalf("outside input mode changed from %v to %v", outsideBefore.Mode().Perm(), outsideAfter.Mode().Perm())
	}
}

func TestFrameIntoRootRejectsReplacedInputBeforeKoubou(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires elevated permissions on Windows")
	}
	dir := t.TempDir()
	destinationPath := filepath.Join(dir, "destination")
	outsidePath := filepath.Join(dir, "outside.png")
	inputPath := filepath.Join(dir, "input.png")
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	writeMinimalPNG(t, inputPath, 200, 300)
	writeMinimalPNG(t, outsidePath, 200, 300)
	previous := matrixFrameInputBeforeGenerateForTest
	var replacedOriginalPath string
	called := false
	matrixFrameInputBeforeGenerateForTest = func(path string) {
		called = true
		if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
			t.Errorf("unlock input scratch for race fixture: %v", err)
			return
		}
		replacedOriginalPath = path + "-original"
		if err := os.Rename(path, replacedOriginalPath); err != nil {
			t.Errorf("rename prepared input: %v", err)
			return
		}
		if err := os.Symlink(outsidePath, path); err != nil {
			t.Errorf("replace prepared input with symlink: %v", err)
		}
	}
	t.Cleanup(func() {
		matrixFrameInputBeforeGenerateForTest = previous
		_ = os.Remove(inputPath)
		if replacedOriginalPath != "" {
			_ = os.Remove(replacedOriginalPath)
		}
	})
	root, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination root: %v", err)
	}
	defer root.Close()
	result, err := frameIntoRoot(context.Background(), FrameRequest{
		InputPath:  inputPath,
		OutputPath: filepath.Join(destinationPath, "home.png"),
		Device:     string(FrameDeviceMac),
	}, root)
	if !called {
		t.Fatal("prepared input replacement seam was not called")
	}
	if err == nil {
		t.Fatal("frameIntoRoot() error = nil, want replaced input rejection")
	}
	if result != nil {
		t.Fatalf("frameIntoRoot() result = %+v, want no result", result)
	}
	if _, statErr := os.Lstat(filepath.Join(destinationPath, "home.png")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination stat error = %v, want no publication", statErr)
	}
}

func TestFrameIntoRootPreservesReplacementWhenWorkRootAnchoringFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	dir := t.TempDir()
	destinationPath := filepath.Join(dir, "destination")
	outsidePath := filepath.Join(dir, "outside")
	inputPath := filepath.Join(dir, "input.png")
	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := os.MkdirAll(outsidePath, 0o755); err != nil {
		t.Fatalf("create outside: %v", err)
	}
	sentinelPath := filepath.Join(outsidePath, "sentinel")
	if err := os.WriteFile(sentinelPath, []byte("preserve"), 0o600); err != nil {
		t.Fatalf("write outside sentinel: %v", err)
	}
	writeMinimalPNG(t, inputPath, 200, 300)
	var originalWorkRoot, replacedWorkRoot string
	previous := matrixFrameWorkRootBeforeAnchorForTest
	matrixFrameWorkRootBeforeAnchorForTest = func(path string) {
		originalWorkRoot = path + "-original"
		if err := os.Rename(path, originalWorkRoot); err != nil {
			t.Errorf("rename Koubou work root: %v", err)
			return
		}
		if err := os.Symlink(outsidePath, path); err != nil {
			t.Errorf("replace Koubou work root: %v", err)
			return
		}
		replacedWorkRoot = path
	}
	t.Cleanup(func() {
		matrixFrameWorkRootBeforeAnchorForTest = previous
		if originalWorkRoot != "" {
			_ = os.RemoveAll(originalWorkRoot)
		}
		if replacedWorkRoot == "" {
			return
		}
		info, statErr := os.Lstat(replacedWorkRoot)
		if errors.Is(statErr, os.ErrNotExist) {
			return
		}
		if statErr != nil {
			t.Errorf("stat replaced Koubou work root: %v", statErr)
			return
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("replaced Koubou work root = %s, want test-created symlink", info.Mode())
			return
		}
		if removeErr := os.Remove(replacedWorkRoot); removeErr != nil {
			t.Errorf("remove replaced Koubou work root: %v", removeErr)
			return
		}
		if _, statErr := os.Lstat(replacedWorkRoot); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("replaced Koubou work root residue: %v", statErr)
		}
	})
	root, err := rootfs.New(destinationPath)
	if err != nil {
		t.Fatalf("open destination root: %v", err)
	}
	defer root.Close()
	result, err := frameIntoRoot(context.Background(), FrameRequest{
		InputPath:  inputPath,
		OutputPath: filepath.Join(destinationPath, "home.png"),
		Device:     string(FrameDeviceMac),
	}, root)
	if err == nil {
		t.Fatal("frameIntoRoot() error = nil, want work-root anchoring failure")
	}
	if result != nil {
		t.Fatalf("frameIntoRoot() result = %+v, want no result", result)
	}
	if got, readErr := os.ReadFile(sentinelPath); readErr != nil || string(got) != "preserve" {
		t.Fatalf("outside sentinel = %q, read error = %v, want replacement preserved", got, readErr)
	}
}

func TestRunMatrixRejectsAmbiguousRootedCallbacksBeforeExecution(t *testing.T) {
	var runPlanCalled, rootedRunPlanCalled, frameCalled, rootedFrameCalled bool
	_, err := RunMatrixWithDependencies(context.Background(), "", nil, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(context.Context, *Plan) (*RunResult, error) {
			runPlanCalled = true
			return nil, nil
		},
		RunPlanRooted: func(context.Context, *Plan, rootfs.Root) (*RunResult, error) {
			rootedRunPlanCalled = true
			return nil, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "RunPlan and RunPlanRooted are mutually exclusive") {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want ambiguous plan callback validation", err)
	}
	if runPlanCalled || rootedRunPlanCalled || frameCalled || rootedFrameCalled {
		t.Fatal("ambiguous callback validation invoked a provider")
	}
	_, err = RunMatrixWithDependencies(context.Background(), "", nil, MatrixOptions{}, MatrixDependencies{
		Frame: func(context.Context, FrameRequest) (*FrameResult, error) {
			frameCalled = true
			return nil, nil
		},
		FrameRooted: func(context.Context, FrameRequest, rootfs.Root) (*FrameResult, error) {
			rootedFrameCalled = true
			return nil, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "Frame and FrameRooted are mutually exclusive") {
		t.Fatalf("RunMatrixWithDependencies() error = %v, want ambiguous frame callback validation", err)
	}
	if frameCalled || rootedFrameCalled {
		t.Fatal("ambiguous frame callback validation invoked a provider")
	}
}

func TestMatrixAttemptRejectsReplacedPrivateCaptureRoot(t *testing.T) {
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw output root: %v", err)
	}
	rawRoot, err := rootfs.New(rawDir)
	if err != nil {
		t.Fatalf("open raw output root: %v", err)
	}
	defer rawRoot.Close()
	cell := MatrixCell{
		ID:     "phone|en-US|light|default",
		Device: "phone",
		UDID:   "SIM-UDID",
		RawDir: filepath.Join(rawDir, "en-US", "phone", "light", "default"),
		RawPaths: []string{
			filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png"),
		},
	}
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	matrixPlan := &MatrixPlan{}
	var attemptParentPath string
	previousParentCreated := matrixPrivateAttemptParentCreatedForTest
	matrixPrivateAttemptParentCreatedForTest = func(path string) { attemptParentPath = path }
	t.Cleanup(func() { matrixPrivateAttemptParentCreatedForTest = previousParentCreated })
	t.Cleanup(func() {
		if attemptParentPath != "" {
			_ = os.RemoveAll(attemptParentPath)
		}
	})
	outside := filepath.Join(dir, "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	_, attemptErr := executeMatrixCellAttempt(context.Background(), cell, base, matrixPlan, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			original := plan.App.OutputDir + "-original"
			if err := os.Rename(plan.App.OutputDir, original); err != nil {
				return nil, err
			}
			if err := os.Symlink(outside, plan.App.OutputDir); err != nil {
				return nil, err
			}
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
	}, matrixOutputRoots{raw: rawRoot, rawPath: rawDir})
	if attemptErr == nil {
		t.Fatal("executeMatrixCellAttempt() error = nil, want replaced private root failure")
	}
	if _, err := os.Stat(cell.RawPaths[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("raw destination stat error = %v, want no promoted artifact", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "home.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("provider wrote outside private attempt root: %v", err)
	}
}

func TestMatrixAttemptRejectsReplacedPrivateFrameRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable on Windows")
	}
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	framedDir := filepath.Join(dir, "framed")
	outside := filepath.Join(dir, "outside")
	for _, path := range []string{rawDir, framedDir, outside} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
	}
	rawRoot, err := rootfs.New(rawDir)
	if err != nil {
		t.Fatalf("open raw output root: %v", err)
	}
	defer rawRoot.Close()
	framedRoot, err := rootfs.New(framedDir)
	if err != nil {
		t.Fatalf("open framed output root: %v", err)
	}
	defer framedRoot.Close()
	cell := MatrixCell{
		ID:          "phone|en-US|light|default",
		Device:      "phone",
		UDID:        "SIM-FRAME-SWAP",
		RawDir:      filepath.Join(rawDir, "en-US", "phone", "light", "default"),
		FramedDir:   filepath.Join(framedDir, "en-US", "phone", "light", "default"),
		RawPaths:    []string{filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png")},
		FramedPaths: []string{filepath.Join(framedDir, "en-US", "phone", "light", "default", "home.png")},
	}
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	matrixPlan := &MatrixPlan{Output: MatrixOutput{Frame: MatrixFrame{Enabled: true, DeviceByMatrixDevice: map[string]string{"phone": "iphone-17-pro"}}}}
	_, attemptErr := executeMatrixCellAttempt(context.Background(), cell, base, matrixPlan, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Frame: func(_ context.Context, request FrameRequest) (*FrameResult, error) {
			frameDir := filepath.Dir(request.OutputPath)
			original := frameDir + "-original"
			if err := os.Rename(frameDir, original); err != nil {
				return nil, err
			}
			if err := os.Symlink(outside, frameDir); err != nil {
				return nil, err
			}
			writeMatrixPNG(t, filepath.Join(frameDir, "home.png"))
			return &FrameResult{}, nil
		},
	}, matrixOutputRoots{raw: rawRoot, rawPath: rawDir, framed: framedRoot, framedPath: framedDir, hasFramed: true})
	if attemptErr == nil {
		t.Fatal("executeMatrixCellAttempt() error = nil, want replaced private frame root failure")
	}
	if _, err := os.Stat(filepath.Join(outside, "home.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("frame provider wrote outside private attempt root: %v", err)
	}
}

func TestCreateMatrixPrivateAttemptRootRejectsParentReplacementBeforeOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows temporary volumes do not reliably distinguish a replaced parent directory identity")
	}
	var parentPath, originalPath, replacementSentinel string
	var swapErr error
	previous := matrixPrivateAttemptParentCreatedForTest
	matrixPrivateAttemptParentCreatedForTest = func(path string) {
		parentPath = path
		originalPath = path + "-original"
		if err := os.Rename(path, originalPath); err != nil {
			swapErr = err
			return
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			swapErr = err
			return
		}
		replacementSentinel = filepath.Join(path, "replacement-sentinel")
		swapErr = os.WriteFile(replacementSentinel, []byte("replacement must survive"), 0o600)
	}
	t.Cleanup(func() {
		matrixPrivateAttemptParentCreatedForTest = previous
		if parentPath != "" {
			_ = os.RemoveAll(parentPath)
		}
		if originalPath != "" {
			_ = os.RemoveAll(originalPath)
		}
	})

	attempt, err := createMatrixPrivateAttemptRoot()
	if err == nil {
		_ = attempt.close()
		t.Fatal("createMatrixPrivateAttemptRoot() error = nil, want parent identity failure")
	}
	if swapErr != nil {
		t.Fatalf("replace private attempt parent: %v", swapErr)
	}
	if _, err := os.Stat(replacementSentinel); err != nil {
		t.Fatalf("replacement sentinel stat error = %v, want replacement preserved", err)
	}
}

func TestCreateMatrixPrivateAttemptRootRejectsChildReplacementBeforeOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows no-delete-share creator prevents child replacement before open")
	}
	var parentPath, originalPath, replacementSentinel string
	var swapErr error
	previousParent := matrixPrivateAttemptParentCreatedForTest
	previousOperation := matrixPrivateAttemptOperationForTest
	matrixPrivateAttemptParentCreatedForTest = func(path string) { parentPath = path }
	matrixPrivateAttemptOperationForTest = func(stage, path string) error {
		if stage != "child_open" {
			return nil
		}
		originalPath = path + "-original"
		if err := os.Rename(path, originalPath); err != nil {
			swapErr = err
			return nil
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			swapErr = err
			return nil
		}
		replacementSentinel = filepath.Join(path, "replacement-sentinel")
		swapErr = os.WriteFile(replacementSentinel, []byte("replacement must survive"), 0o600)
		return nil
	}
	t.Cleanup(func() {
		matrixPrivateAttemptParentCreatedForTest = previousParent
		matrixPrivateAttemptOperationForTest = previousOperation
		if parentPath != "" {
			_ = os.RemoveAll(parentPath)
		}
		if originalPath != "" {
			_ = os.RemoveAll(originalPath)
		}
	})

	attempt, err := createMatrixPrivateAttemptRoot()
	if err == nil {
		_ = attempt.close()
		t.Fatal("createMatrixPrivateAttemptRoot() error = nil, want child identity failure")
	}
	if swapErr != nil {
		t.Fatalf("replace private attempt child: %v", swapErr)
	}
	if _, err := os.Stat(replacementSentinel); err != nil {
		t.Fatalf("replacement sentinel stat error = %v, want replacement preserved", err)
	}
}

func TestCreateMatrixPrivateAttemptRootRejectsReplacementBetweenRootAndChildOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows no-delete-share creator prevents child replacement while opening roots")
	}
	var parentPath string
	var originalPath string
	var replacementSentinel string
	var swapErr error
	previousParentCreated := matrixPrivateAttemptParentCreatedForTest
	previousHook := matrixPrivateAttemptRootBeforeChildRootForTest
	matrixPrivateAttemptParentCreatedForTest = func(path string) { parentPath = path }
	matrixPrivateAttemptRootBeforeChildRootForTest = func(path string) {
		originalPath = path + "-original"
		if err := os.Rename(path, originalPath); err != nil {
			swapErr = err
			return
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			swapErr = err
			return
		}
		replacementSentinel = filepath.Join(path, "replacement-sentinel")
		swapErr = os.WriteFile(replacementSentinel, []byte("replacement must survive"), 0o600)
	}
	t.Cleanup(func() {
		matrixPrivateAttemptParentCreatedForTest = previousParentCreated
		matrixPrivateAttemptRootBeforeChildRootForTest = previousHook
	})

	attempt, err := createMatrixPrivateAttemptRoot()
	if swapErr != nil {
		t.Fatalf("replace private attempt root: %v", swapErr)
	}
	if err == nil {
		_ = attempt.close()
		t.Fatal("createMatrixPrivateAttemptRoot() error = nil, want construction identity failure")
	}
	if replacementSentinel == "" {
		t.Fatal("replacement callback did not create sentinel")
	}
	if _, statErr := os.Stat(replacementSentinel); statErr != nil {
		t.Fatalf("replacement sentinel stat error = %v, want replacement preserved", statErr)
	}
	t.Cleanup(func() {
		if originalPath != "" {
			_ = os.RemoveAll(originalPath)
		}
		if replacementSentinel != "" {
			_ = os.RemoveAll(filepath.Dir(replacementSentinel))
		}
		if parentPath != "" {
			_ = os.RemoveAll(parentPath)
		}
	})
}

func TestCreateMatrixPrivateAttemptRootRejectsReplacementBeforeInitialPin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows no-delete-share creator prevents child replacement before pinning")
	}
	var parentPath, originalPath, replacementPath, replacementSentinel string
	var swapErr error
	previousParentCreated := matrixPrivateAttemptParentCreatedForTest
	previousBeforePin := matrixPrivateAttemptBeforePinForTest
	matrixPrivateAttemptParentCreatedForTest = func(path string) { parentPath = path }
	matrixPrivateAttemptBeforePinForTest = func(path string) {
		originalPath = path + "-original"
		if err := os.Rename(path, originalPath); err != nil {
			swapErr = err
			return
		}
		replacementPath = path
		if err := os.Mkdir(path, 0o700); err != nil {
			swapErr = err
			return
		}
		replacementSentinel = filepath.Join(path, "replacement-sentinel")
		swapErr = os.WriteFile(replacementSentinel, []byte("replacement must survive"), 0o600)
	}
	t.Cleanup(func() {
		matrixPrivateAttemptParentCreatedForTest = previousParentCreated
		matrixPrivateAttemptBeforePinForTest = previousBeforePin
		if replacementPath != "" {
			_ = os.RemoveAll(replacementPath)
		}
		if originalPath != "" {
			_ = os.RemoveAll(originalPath)
		}
		if parentPath != "" {
			_ = os.RemoveAll(parentPath)
		}
	})

	attempt, err := createMatrixPrivateAttemptRoot()
	if err == nil {
		_ = attempt.close()
		t.Fatal("createMatrixPrivateAttemptRoot() error = nil, want initial pin identity failure")
	}
	if swapErr != nil {
		t.Fatalf("replace private attempt root before initial pin: %v", swapErr)
	}
	if replacementSentinel == "" {
		t.Fatal("replacement callback did not create sentinel")
	}
	if _, statErr := os.Stat(replacementSentinel); statErr != nil {
		t.Fatalf("replacement sentinel stat error = %v, want replacement preserved", statErr)
	}
}

func TestCreateMatrixPrivateAttemptRootRejectsReplacementBeforeParentLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable with open Windows handles")
	}
	var parentPath, originalPath, replacementSentinel string
	var swapErr error
	previousParentCreated := matrixPrivateAttemptParentCreatedForTest
	previousFinalHook := matrixPrivateAttemptBeforeParentLockForTest
	matrixPrivateAttemptParentCreatedForTest = func(path string) { parentPath = path }
	matrixPrivateAttemptBeforeParentLockForTest = func(path string) {
		originalPath = path + "-original"
		if err := os.Rename(path, originalPath); err != nil {
			swapErr = err
			return
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			swapErr = err
			return
		}
		replacementSentinel = filepath.Join(path, "replacement-sentinel")
		swapErr = os.WriteFile(replacementSentinel, []byte("replacement must survive"), 0o600)
	}
	t.Cleanup(func() {
		matrixPrivateAttemptParentCreatedForTest = previousParentCreated
		matrixPrivateAttemptBeforeParentLockForTest = previousFinalHook
		if parentPath != "" {
			_ = os.RemoveAll(parentPath)
		}
		if originalPath != "" {
			_ = os.RemoveAll(originalPath)
		}
	})

	attempt, err := createMatrixPrivateAttemptRoot()
	if err == nil {
		_ = attempt.close()
		t.Fatal("createMatrixPrivateAttemptRoot() error = nil, want final identity failure")
	}
	if swapErr != nil {
		t.Fatalf("replace private attempt root before parent lock: %v", swapErr)
	}
	if replacementSentinel == "" {
		t.Fatal("replacement callback did not create sentinel")
	}
	if _, statErr := os.Stat(replacementSentinel); statErr != nil {
		t.Fatalf("replacement sentinel stat error = %v, want replacement preserved", statErr)
	}
}

func TestLockMatrixPrivateAttemptChildRejectsReplacementAtLockBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows no-delete-share creator prevents child replacement at the lock boundary")
	}
	attempt, err := createMatrixPrivateAttemptRoot()
	if err != nil {
		t.Fatalf("createMatrixPrivateAttemptRoot() error: %v", err)
	}
	originalPath := attempt.path + "-original"
	replacementSentinel := filepath.Join(attempt.path, "replacement-sentinel")
	previous := matrixPrivateAttemptBeforeChildLockForTest
	var swapErr error
	matrixPrivateAttemptBeforeChildLockForTest = func(path string) {
		if swapErr = unlockMatrixPrivateAttemptParentRetained(attempt.parentDACL, attempt.parent); swapErr != nil {
			return
		}
		attempt.parentDACL = nil
		attempt.parentLocked = false
		if swapErr = os.Rename(path, originalPath); swapErr != nil {
			return
		}
		if swapErr = os.Mkdir(path, 0o700); swapErr != nil {
			return
		}
		if swapErr = os.WriteFile(replacementSentinel, []byte("replacement must survive"), 0o600); swapErr != nil {
			return
		}
		attempt.parentDACL, swapErr = lockMatrixPrivateAttemptParentRetained(attempt.parent)
		attempt.parentLocked = swapErr == nil
	}
	t.Cleanup(func() {
		matrixPrivateAttemptBeforeChildLockForTest = previous
		_ = attempt.cleanup()
		_ = attempt.close()
		_ = os.RemoveAll(attempt.path)
		_ = os.RemoveAll(originalPath)
	})

	err = lockMatrixPrivateAttemptChild(&attempt)
	if err == nil {
		t.Fatal("lockMatrixPrivateAttemptChild() error = nil, want replacement rejection")
	}
	if swapErr != nil {
		t.Fatalf("replace private attempt child at lock boundary: %v", swapErr)
	}
	if _, err := os.Stat(replacementSentinel); err != nil {
		t.Fatalf("replacement sentinel stat error = %v, want replacement preserved", err)
	}
}

func TestCleanupMatrixPrivateAttemptRemovesNamespace(t *testing.T) {
	attempt, err := createMatrixPrivateAttemptRoot()
	if err != nil {
		t.Fatalf("createMatrixPrivateAttemptRoot() error: %v", err)
	}
	namespace := filepath.Dir(filepath.Dir(attempt.path))
	if err := cleanupMatrixPrivateAttemptForExecution(&attempt); err != nil {
		t.Fatalf("cleanupMatrixPrivateAttemptForExecution() error = %v", err)
	}
	if err := closeMatrixPrivateAttemptForExecution(&attempt); err != nil {
		t.Fatalf("closeMatrixPrivateAttemptForExecution() error = %v", err)
	}
	if _, err := os.Lstat(namespace); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("private attempt namespace %q still exists after cleanup: %v", namespace, err)
	}
}

func TestLockMatrixPrivateAttemptChildRechecksOutputIdentityAtLockBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("output replacement is not reliable while Windows rooted handles are open")
	}
	attempt, err := createMatrixPrivateAttemptRoot()
	if err != nil {
		t.Fatalf("createMatrixPrivateAttemptRoot() error: %v", err)
	}
	outputRoot, err := openMatrixPrivateAttemptOutputRoot(&attempt)
	if err != nil {
		_ = attempt.cleanup()
		_ = attempt.close()
		t.Fatalf("openMatrixPrivateAttemptOutputRoot() error: %v", err)
	}
	outputPath := filepath.Join(attempt.path, "output")
	originalPath := outputPath + "-original"
	replacementSentinel := filepath.Join(outputPath, "replacement-sentinel")
	previous := matrixPrivateAttemptBeforeChildLockForTest
	var swapErr error
	matrixPrivateAttemptBeforeChildLockForTest = func(string) {
		if swapErr = os.Rename(outputPath, originalPath); swapErr != nil {
			return
		}
		if swapErr = os.Mkdir(outputPath, 0o700); swapErr != nil {
			return
		}
		swapErr = os.WriteFile(replacementSentinel, []byte("replacement must survive"), 0o600)
	}
	t.Cleanup(func() {
		matrixPrivateAttemptBeforeChildLockForTest = previous
		_ = outputRoot.Close()
		_ = attempt.cleanup()
		_ = attempt.close()
		_ = os.RemoveAll(outputPath)
		_ = os.RemoveAll(originalPath)
	})

	err = lockMatrixPrivateAttemptChild(&attempt)
	if err == nil {
		t.Fatal("lockMatrixPrivateAttemptChild() error = nil, want output identity rejection")
	}
	if swapErr != nil {
		t.Fatalf("replace output at child lock boundary: %v", swapErr)
	}
	if _, err := os.Stat(replacementSentinel); err != nil {
		t.Fatalf("replacement sentinel stat error = %v, want replacement preserved", err)
	}
}

func TestCreateMatrixPrivateAttemptRootEarlyFailuresPreserveReplacements(t *testing.T) {
	tests := []struct {
		name      string
		stage     string
		uncertain bool
	}{
		{name: "grandparent open", stage: "grandparent_open", uncertain: true},
		{name: "parent open", stage: "parent_open", uncertain: true},
		{name: "parent stat", stage: "parent_stat", uncertain: true},
		{name: "mkdir", stage: "mkdir"},
		{name: "child lstat", stage: "child_lstat", uncertain: true},
		{name: "child open", stage: "child_open"},
		{name: "child stat", stage: "child_stat"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, nonEmpty := range []bool{false, true} {
				t.Run(fmt.Sprintf("replacement-empty-%t", nonEmpty), func(t *testing.T) {
					var parentPath string
					var originalPath, replacementPath, replacementSentinel string
					var swapErr error
					var renameBlocked bool
					previousParentCreated := matrixPrivateAttemptParentCreatedForTest
					previousOperation := matrixPrivateAttemptOperationForTest
					matrixPrivateAttemptParentCreatedForTest = func(path string) { parentPath = path }
					matrixPrivateAttemptOperationForTest = func(stage, path string) error {
						if stage != tc.stage {
							return nil
						}
						target := path
						if stage == "grandparent_open" || stage == "mkdir" {
							target = parentPath
						}
						originalPath = target + "-original"
						if err := os.Rename(target, originalPath); err != nil {
							if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(32)) {
								// Creation handles deny delete sharing until cleanup.
								renameBlocked = true
								return errors.New("injected matrix private attempt construction failure")
							}
							swapErr = err
							return swapErr
						}
						if err := os.Mkdir(target, 0o700); err != nil {
							swapErr = err
							return swapErr
						}
						replacementPath = target
						if nonEmpty {
							replacementSentinel = filepath.Join(target, "replacement-sentinel")
							swapErr = os.WriteFile(replacementSentinel, []byte("replacement must survive"), 0o600)
						}
						return errors.New("injected matrix private attempt construction failure")
					}
					t.Cleanup(func() {
						matrixPrivateAttemptParentCreatedForTest = previousParentCreated
						matrixPrivateAttemptOperationForTest = previousOperation
						if replacementPath != "" {
							_ = os.RemoveAll(replacementPath)
						}
						if originalPath != "" {
							_ = os.RemoveAll(originalPath)
						}
						if parentPath != "" {
							_ = os.RemoveAll(parentPath)
						}
					})

					attempt, err := createMatrixPrivateAttemptRoot()
					if got := errors.Is(err, errMatrixPrivateAttemptCleanupUncertain); got != tc.uncertain {
						t.Fatalf("cleanup uncertainty = %t, want %t (err=%v)", got, tc.uncertain, err)
					}
					if err == nil {
						_ = attempt.close()
						t.Fatal("createMatrixPrivateAttemptRoot() error = nil, want injected construction failure")
					}
					if swapErr != nil {
						t.Fatalf("replace construction path: %v", swapErr)
					}
					if runtime.GOOS == "windows" {
						if !renameBlocked {
							t.Fatal("construction handle did not block directory rename")
						}
						if _, statErr := os.Stat(originalPath); !errors.Is(statErr, os.ErrNotExist) {
							t.Fatalf("rename destination exists after blocked rename: %v", statErr)
						}
						_, statErr := os.Stat(parentPath)
						if tc.uncertain && statErr != nil {
							t.Fatalf("unverified original directory was not preserved: %v", statErr)
						}
						if !tc.uncertain && !errors.Is(statErr, os.ErrNotExist) {
							t.Fatalf("verified original directory was not cleaned: %v", statErr)
						}
						return
					}
					if replacementPath == "" {
						t.Fatal("replacement path was not created")
					}
					if _, statErr := os.Stat(replacementPath); statErr != nil {
						t.Fatalf("replacement stat error = %v, want replacement preserved", statErr)
					}
					if nonEmpty {
						if _, statErr := os.Stat(replacementSentinel); statErr != nil {
							t.Fatalf("replacement sentinel stat error = %v, want replacement preserved", statErr)
						}
					}
				})
			}
		})
	}
}

func TestCreateMatrixPrivateAttemptRootClosesReopenedChildOnParentLockFailure(t *testing.T) {
	var parentPath string
	previousParentCreated := matrixPrivateAttemptParentCreatedForTest
	previousOperation := matrixPrivateAttemptOperationForTest
	matrixPrivateAttemptParentCreatedForTest = func(path string) { parentPath = path }
	matrixPrivateAttemptOperationForTest = func(stage, _ string) error {
		if stage == "parent_lock" {
			return errors.New("injected parent lock failure")
		}
		return nil
	}
	t.Cleanup(func() {
		matrixPrivateAttemptParentCreatedForTest = previousParentCreated
		matrixPrivateAttemptOperationForTest = previousOperation
		if parentPath != "" {
			_ = os.RemoveAll(parentPath)
		}
	})
	attempt, err := createMatrixPrivateAttemptRoot()
	if err == nil {
		_ = attempt.close()
		t.Fatal("createMatrixPrivateAttemptRoot() error = nil, want injected lock failure")
	}
	if !strings.Contains(err.Error(), "injected parent lock failure") {
		t.Fatalf("createMatrixPrivateAttemptRoot() error = %v, want injected lock failure", err)
	}
	if parentPath == "" {
		t.Fatal("private attempt parent callback was not called")
	}
}

func TestMatrixAttemptCleanupPreservesEmptyReplacement(t *testing.T) {
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw output root: %v", err)
	}
	rawRoot, err := rootfs.New(rawDir)
	if err != nil {
		t.Fatalf("open raw output root: %v", err)
	}
	defer rawRoot.Close()
	cell := MatrixCell{
		ID:     "phone|en-US|light|default",
		Device: "phone",
		UDID:   "SIM-UDID",
		RawDir: filepath.Join(rawDir, "en-US", "phone", "light", "default"),
		RawPaths: []string{
			filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png"),
		},
	}
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	var replacementPath string
	var originalPath string
	var swapErr error
	var swapped bool
	previousHook := matrixPrivateAttemptBeforeCleanupRemoveForTest
	matrixPrivateAttemptBeforeCleanupRemoveForTest = func(path string) {
		if swapped {
			return
		}
		swapped = true
		originalPath = path + "-original"
		if err := os.Rename(path, originalPath); err != nil {
			swapErr = err
			return
		}
		replacementPath = path
		swapErr = os.Mkdir(replacementPath, 0o700)
	}
	t.Cleanup(func() { matrixPrivateAttemptBeforeCleanupRemoveForTest = previousHook })
	_, attemptErr := executeMatrixCellAttempt(context.Background(), cell, base, &MatrixPlan{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
	}, matrixOutputRoots{raw: rawRoot, rawPath: rawDir})
	if swapErr != nil {
		t.Fatalf("replace private attempt root during cleanup: %v", swapErr)
	}
	if attemptErr != nil {
		t.Fatalf("executeMatrixCellAttempt() error = %v, want cleanup to preserve replacement", attemptErr)
	}
	if !swapped || replacementPath == "" {
		t.Fatal("cleanup replacement seam did not run")
	}
	if _, err := os.Stat(replacementPath); err != nil {
		t.Fatalf("empty replacement stat error = %v, want replacement preserved", err)
	}
	if _, err := os.Stat(originalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original attempt root stat error = %v, want original cleaned", err)
	}
	t.Cleanup(func() {
		if replacementPath != "" {
			_ = os.RemoveAll(filepath.Dir(replacementPath))
		}
	})
}

func TestMatrixAttemptCleanupPreservesReplacedPrivateRoot(t *testing.T) {
	for _, tc := range []struct {
		name          string
		frame         bool
		callbackError bool
	}{
		{name: "capture success", frame: false},
		{name: "capture callback error", frame: false, callbackError: true},
		{name: "frame success", frame: true},
		{name: "frame callback error", frame: true, callbackError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			rawDir := filepath.Join(dir, "raw")
			framedDir := filepath.Join(dir, "framed")
			if err := os.MkdirAll(rawDir, 0o755); err != nil {
				t.Fatalf("create raw output root: %v", err)
			}
			if tc.frame {
				if err := os.MkdirAll(framedDir, 0o755); err != nil {
					t.Fatalf("create framed output root: %v", err)
				}
			}
			rawRoot, err := rootfs.New(rawDir)
			if err != nil {
				t.Fatalf("open raw output root: %v", err)
			}
			defer rawRoot.Close()
			var framedRoot rootfs.Root
			if tc.frame {
				framedRoot, err = rootfs.New(framedDir)
				if err != nil {
					t.Fatalf("open framed output root: %v", err)
				}
				defer framedRoot.Close()
			}
			cell := MatrixCell{
				ID:          "phone|en-US|light|default",
				Device:      "phone",
				UDID:        "SIM-UDID",
				RawDir:      filepath.Join(rawDir, "en-US", "phone", "light", "default"),
				RawPaths:    []string{filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png")},
				FramedDir:   filepath.Join(framedDir, "en-US", "phone", "light", "default"),
				FramedPaths: []string{filepath.Join(framedDir, "en-US", "phone", "light", "default", "home.png")},
			}
			base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
			matrixPlan := &MatrixPlan{}
			if tc.frame {
				matrixPlan.Output.Frame = MatrixFrame{Enabled: true, DeviceByMatrixDevice: map[string]string{"phone": "iphone-17-pro"}}
			}
			var replacementSentinel string
			var originalAttemptPath string
			var blockedPath string
			replace := func(path string) error {
				// The production parent is mutation-locked while providers run.
				// Explicitly restore the test fixture's owner write bit here so
				// this test continues to exercise cleanup after an adversarial
				// replacement rather than the provider-destination guard.
				if runtime.GOOS != "windows" {
					if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
						return err
					}
				}
				originalAttemptPath = path + "-original"
				if err := os.Rename(path, originalAttemptPath); err != nil {
					if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(32)) {
						blockedPath = path
						writeMatrixPNG(t, filepath.Join(path, "home.png"))
						return nil
					}
					return err
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					return err
				}
				replacementSentinel = filepath.Join(path, "replacement-sentinel")
				return os.WriteFile(replacementSentinel, []byte("must survive cleanup"), 0o600)
			}
			deps := MatrixDependencies{
				RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
					if tc.frame {
						writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
						return &RunResult{}, nil
					}
					if err := replace(plan.App.OutputDir); err != nil {
						return nil, err
					}
					if tc.callbackError {
						return nil, errors.New("capture callback failed")
					}
					return &RunResult{}, nil
				},
			}
			if tc.frame {
				deps.Frame = func(_ context.Context, request FrameRequest) (*FrameResult, error) {
					if err := replace(filepath.Dir(request.OutputPath)); err != nil {
						return nil, err
					}
					if tc.callbackError {
						return nil, errors.New("frame callback failed")
					}
					return &FrameResult{}, nil
				}
			}
			_, _ = executeMatrixCellAttempt(context.Background(), cell, base, matrixPlan, deps, matrixOutputRoots{
				raw: rawRoot, rawPath: rawDir, framed: framedRoot, framedPath: framedDir, hasFramed: tc.frame,
			})
			if runtime.GOOS == "windows" {
				if blockedPath == "" {
					t.Fatal("provider directory rename was not blocked")
				}
				for _, path := range []string{blockedPath, originalAttemptPath} {
					if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("protected attempt was not cleaned: %s: %v", path, err)
					}
				}
				return
			}
			if replacementSentinel == "" {
				t.Fatal("replacement callback did not run")
			}
			if _, err := os.Stat(replacementSentinel); err != nil {
				t.Fatalf("replacement sentinel stat error = %v, want replacement preserved", err)
			}
			if _, err := os.Stat(originalAttemptPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("original attempt root stat error = %v, want original cleaned", err)
			}
			t.Cleanup(func() {
				if originalAttemptPath != "" {
					_ = os.RemoveAll(filepath.Dir(originalAttemptPath))
				}
			})
		})
	}
}

func TestMatrixAttemptResourceErrorsSuppressSuccess(t *testing.T) {
	for _, tc := range []struct {
		name       string
		setCleanup bool
	}{
		{name: "cleanup", setCleanup: true},
		{name: "close"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			rawDir := filepath.Join(dir, "raw")
			if err := os.MkdirAll(rawDir, 0o755); err != nil {
				t.Fatalf("create raw output root: %v", err)
			}
			rawRoot, err := rootfs.New(rawDir)
			if err != nil {
				t.Fatalf("open raw output root: %v", err)
			}
			defer rawRoot.Close()
			cell := MatrixCell{
				ID:     "phone|en-US|light|default",
				Device: "phone",
				UDID:   "SIM-UDID",
				RawDir: filepath.Join(rawDir, "en-US", "phone", "light", "default"),
				RawPaths: []string{
					filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png"),
				},
			}
			base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
			resourceErr := errors.New("injected private attempt " + tc.name + " error")
			previousCleanup := matrixPrivateAttemptCleanupForTest
			previousClose := matrixPrivateAttemptCloseForTest
			if tc.setCleanup {
				matrixPrivateAttemptCleanupForTest = func(string) error { return resourceErr }
			} else {
				matrixPrivateAttemptCloseForTest = func(string) error { return resourceErr }
			}
			t.Cleanup(func() {
				matrixPrivateAttemptCleanupForTest = previousCleanup
				matrixPrivateAttemptCloseForTest = previousClose
			})
			attempt, attemptErr := executeMatrixCellAttempt(context.Background(), cell, base, &MatrixPlan{}, MatrixDependencies{
				RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
					writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
					return &RunResult{}, nil
				},
			}, matrixOutputRoots{raw: rawRoot, rawPath: rawDir})
			if attemptErr == nil || !errors.Is(attemptErr, resourceErr) {
				t.Fatalf("executeMatrixCellAttempt() error = %v, want injected resource error", attemptErr)
			}
			if attempt.FailureStage != "cleanup" || attempt.FailureCode != "temporary_output_cleanup_failed" {
				t.Fatalf("attempt failure = %s/%s, want cleanup/temporary_output_cleanup_failed", attempt.FailureStage, attempt.FailureCode)
			}
			if !attempt.CleanupFailed {
				t.Fatal("attempt cleanupFailed = false, want cleanup uncertainty marker")
			}
		})
	}
}

func TestMatrixAttemptPrimaryAndCleanupErrorsRemainJoined(t *testing.T) {
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("create raw output root: %v", err)
	}
	rawRoot, err := rootfs.New(rawDir)
	if err != nil {
		t.Fatalf("open raw output root: %v", err)
	}
	defer rawRoot.Close()
	cell := MatrixCell{
		ID:     "phone|en-US|light|default",
		Device: "phone",
		UDID:   "SIM-UDID",
		RawDir: filepath.Join(rawDir, "en-US", "phone", "light", "default"),
		RawPaths: []string{
			filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png"),
		},
	}
	base := &Plan{Version: 1, App: PlanApp{BundleID: "com.example.app"}, Steps: []PlanStep{{Action: ActionScreenshot, Name: stringPtr("home")}}}
	primaryErr := errors.New("primary attempt error")
	cleanupErr := errors.New("cleanup attempt error")
	previousHook := matrixPrivateAttemptCleanupForTest
	matrixPrivateAttemptCleanupForTest = func(string) error { return cleanupErr }
	t.Cleanup(func() { matrixPrivateAttemptCleanupForTest = previousHook })
	attempt, attemptErr := executeMatrixCellAttempt(context.Background(), cell, base, &MatrixPlan{}, MatrixDependencies{
		RunPlan: func(context.Context, *Plan) (*RunResult, error) { return nil, primaryErr },
	}, matrixOutputRoots{raw: rawRoot, rawPath: rawDir})
	if attemptErr == nil || !errors.Is(attemptErr, primaryErr) || !errors.Is(attemptErr, cleanupErr) {
		t.Fatalf("executeMatrixCellAttempt() error = %v, want primary and cleanup causes", attemptErr)
	}
	if attempt.FailureCode != "plan_failed" || !strings.Contains(attempt.Error, "cleanup") {
		t.Fatalf("attempt failure = %s/%s %q, want primary code plus cleanup uncertainty", attempt.FailureStage, attempt.FailureCode, attempt.Error)
	}
	if !attempt.CleanupFailed {
		t.Fatal("attempt cleanupFailed = false, want cleanup uncertainty marker")
	}
}

func TestRevalidateMatrixFramedPathsUsesPerArtifactBudget(t *testing.T) {
	dir := t.TempDir()
	framedDir := filepath.Join(dir, "framed")
	if err := os.Mkdir(framedDir, 0o755); err != nil {
		t.Fatalf("create framed directory: %v", err)
	}
	paths := []string{filepath.Join(framedDir, "first.png"), filepath.Join(framedDir, "second.png")}
	for _, path := range paths {
		writeMatrixPNG(t, path)
	}
	framedRoot, err := rootfs.New(framedDir)
	if err != nil {
		t.Fatalf("open framed root: %v", err)
	}
	expected := make(map[string]matrixArtifactInfo, len(paths))
	for _, path := range paths {
		artifact, inspectErr := inspectMatrixArtifact(framedRoot, framedDir, path)
		if inspectErr != nil {
			framedRoot.Close()
			t.Fatalf("inspect expected artifact %q: %v", path, inspectErr)
		}
		expected[path] = artifact
	}
	if err := framedRoot.Close(); err != nil {
		t.Fatalf("close framed root: %v", err)
	}
	result := &MatrixResult{FramedDir: framedDir, Cells: []MatrixCellResult{{
		Status:          MatrixCellSuccess,
		FramedPaths:     append([]string(nil), paths...),
		Screenshots:     []MatrixScreenshotResult{{FramedPath: paths[0], Status: MatrixCellSuccess}, {FramedPath: paths[1], Status: MatrixCellSuccess}},
		framedArtifacts: expected,
	}}}
	previousContextHook := matrixFramedVerificationContextForTest
	var deadlines []time.Time
	matrixFramedVerificationContextForTest = func(ctx context.Context) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Errorf("verification context has no deadline")
			return
		}
		deadlines = append(deadlines, deadline)
	}
	t.Cleanup(func() { matrixFramedVerificationContextForTest = previousContextHook })
	if err := revalidateMatrixFramedPathsWithBudget(context.Background(), result, 20*time.Millisecond); err != nil {
		t.Fatalf("revalidateMatrixFramedPathsWithBudget() error = %v, want each artifact to receive its own budget", err)
	}
	if got := result.Cells[0].FramedPaths; !reflect.DeepEqual(got, paths) {
		t.Fatalf("revalidated framed paths = %v, want %v", got, paths)
	}
	if len(deadlines) != len(paths) {
		t.Fatalf("verification context count = %d, want %d", len(deadlines), len(paths))
	}
}

func TestRevalidateMatrixFramedPathsPropagatesCallerCancellation(t *testing.T) {
	dir := t.TempDir()
	framedDir := filepath.Join(dir, "framed")
	if err := os.Mkdir(framedDir, 0o755); err != nil {
		t.Fatalf("create framed directory: %v", err)
	}
	path := filepath.Join(framedDir, "screen.png")
	writeMatrixPNG(t, path)
	framedRoot, err := rootfs.New(framedDir)
	if err != nil {
		t.Fatalf("open framed root: %v", err)
	}
	expected, err := inspectMatrixArtifact(framedRoot, framedDir, path)
	if err != nil {
		framedRoot.Close()
		t.Fatalf("inspect expected artifact: %v", err)
	}
	if err := framedRoot.Close(); err != nil {
		t.Fatalf("close framed root: %v", err)
	}
	result := &MatrixResult{FramedDir: framedDir, Cells: []MatrixCellResult{{
		Status:          MatrixCellSuccess,
		FramedPaths:     []string{path},
		Screenshots:     []MatrixScreenshotResult{{FramedPath: path, Status: MatrixCellSuccess}},
		framedArtifacts: map[string]matrixArtifactInfo{path: expected},
	}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousHook := matrixFramedVerificationBeforeArtifactForTest
	matrixFramedVerificationBeforeArtifactForTest = cancel
	t.Cleanup(func() { matrixFramedVerificationBeforeArtifactForTest = previousHook })
	if err := revalidateMatrixFramedPathsWithBudget(ctx, result, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("revalidateMatrixFramedPathsWithBudget() error = %v, want context cancellation", err)
	}
	if got := result.Cells[0].Status; got != MatrixCellCanceled {
		t.Fatalf("cell status = %q, want canceled", got)
	}
	if len(result.Cells[0].FramedPaths) != 0 {
		t.Fatalf("framed paths = %v, want canceled path removed", result.Cells[0].FramedPaths)
	}
	if got := result.Cells[0].Screenshots[0].Status; got != MatrixCellCanceled {
		t.Fatalf("screenshot status = %q, want canceled with framed path removed", got)
	}
}

func TestRevalidateMatrixFramedPathsAlreadyCanceledStopsImmediately(t *testing.T) {
	dir := t.TempDir()
	framedDir := filepath.Join(dir, "framed")
	if err := os.Mkdir(framedDir, 0o755); err != nil {
		t.Fatalf("create framed directory: %v", err)
	}
	path := filepath.Join(framedDir, "screen.png")
	writeMatrixPNG(t, path)
	framedRoot, err := rootfs.New(framedDir)
	if err != nil {
		t.Fatalf("open framed root: %v", err)
	}
	expected, err := inspectMatrixArtifact(framedRoot, framedDir, path)
	if err != nil {
		framedRoot.Close()
		t.Fatalf("inspect expected artifact: %v", err)
	}
	if err := framedRoot.Close(); err != nil {
		t.Fatalf("close framed root: %v", err)
	}
	result := &MatrixResult{FramedDir: framedDir, Cells: []MatrixCellResult{{
		Status:          MatrixCellSuccess,
		FramedPaths:     []string{path},
		Screenshots:     []MatrixScreenshotResult{{FramedPath: path, Status: MatrixCellSuccess}},
		framedArtifacts: map[string]matrixArtifactInfo{path: expected},
	}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var inspected bool
	previousHook := matrixFramedVerificationContextForTest
	matrixFramedVerificationContextForTest = func(context.Context) { inspected = true }
	t.Cleanup(func() { matrixFramedVerificationContextForTest = previousHook })
	if err := revalidateMatrixFramedPathsWithBudget(ctx, result, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("revalidateMatrixFramedPathsWithBudget() error = %v, want context cancellation", err)
	}
	if inspected {
		t.Fatal("framed verification inspected an artifact after caller cancellation")
	}
	if result.Cells[0].Status != MatrixCellCanceled || len(result.Cells[0].FramedPaths) != 0 {
		t.Fatalf("canceled framed result = %+v, want canceled cell without paths", result.Cells[0])
	}
	if got := result.Cells[0].Screenshots[0].Status; got != MatrixCellCanceled {
		t.Fatalf("canceled screenshot status = %q, want canceled", got)
	}
}

func TestRevalidateMatrixFramedPathsChildTimeoutKeepsPartialReviewTruth(t *testing.T) {
	dir := t.TempDir()
	framedDir := filepath.Join(dir, "framed")
	if err := os.Mkdir(framedDir, 0o755); err != nil {
		t.Fatalf("create framed directory: %v", err)
	}
	paths := []string{filepath.Join(framedDir, "timeout.png"), filepath.Join(framedDir, "ready.png")}
	for _, path := range paths {
		writeMatrixPNG(t, path)
	}
	framedRoot, err := rootfs.New(framedDir)
	if err != nil {
		t.Fatalf("open framed root: %v", err)
	}
	expected := make([]matrixArtifactInfo, len(paths))
	for index, path := range paths {
		expected[index], err = inspectMatrixArtifact(framedRoot, framedDir, path)
		if err != nil {
			framedRoot.Close()
			t.Fatalf("inspect expected artifact %q: %v", path, err)
		}
	}
	if err := framedRoot.Close(); err != nil {
		t.Fatalf("close framed root: %v", err)
	}
	result := &MatrixResult{FramedDir: framedDir, ReviewDir: filepath.Join(dir, "review"), Cells: []MatrixCellResult{
		{
			ID:              "timeout",
			Status:          MatrixCellSuccess,
			FramedPaths:     []string{paths[0]},
			Screenshots:     []MatrixScreenshotResult{{FramedPath: paths[0], Status: MatrixCellSuccess}},
			framedArtifacts: map[string]matrixArtifactInfo{paths[0]: expected[0]},
		},
		{
			ID:              "ready",
			Status:          MatrixCellSuccess,
			FramedPaths:     []string{paths[1]},
			Screenshots:     []MatrixScreenshotResult{{FramedPath: paths[1], Status: MatrixCellSuccess}},
			framedArtifacts: map[string]matrixArtifactInfo{paths[1]: expected[1]},
		},
	}}
	previousHook := matrixFramedVerificationContextForTest
	seen := 0
	matrixFramedVerificationContextForTest = func(ctx context.Context) {
		seen++
		if seen == 1 {
			<-ctx.Done()
		}
	}
	t.Cleanup(func() { matrixFramedVerificationContextForTest = previousHook })
	if err := revalidateMatrixFramedPathsWithBudget(context.Background(), result, 20*time.Millisecond); err == nil {
		t.Fatal("revalidateMatrixFramedPathsWithBudget() error = nil, want child timeout")
	} else if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("revalidateMatrixFramedPathsWithBudget() error = %v, want bounded non-context failure", err)
	}
	if result.Cells[0].Status != MatrixCellFailed || result.Cells[0].FailureCode != "framed_output_unavailable" {
		t.Fatalf("timed-out cell = %+v, want framed output failure", result.Cells[0])
	}
	if result.Cells[1].Status != MatrixCellSuccess || len(result.Cells[1].FramedPaths) != 1 {
		t.Fatalf("ready cell = %+v, want retained success", result.Cells[1])
	}
	if got := result.Cells[0].Screenshots[0].Status; got != MatrixCellFailed {
		t.Fatalf("timed-out screenshot status = %q, want failed", got)
	}
	if got := result.Cells[1].Screenshots[0].Status; got != MatrixCellSuccess {
		t.Fatalf("ready screenshot status = %q, want success", got)
	}
	countMatrixResultStatuses(result)
	review, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: result, OutputDir: result.ReviewDir})
	if err != nil {
		t.Fatalf("GenerateMatrixReview() error = %v", err)
	}
	if review.Succeeded != 1 || review.Failed != 1 || review.Canceled != 0 {
		t.Fatalf("review counts = %+v, want one success and one failure", review)
	}
}

func TestRunMatrixRejectsReplacedRawOutputRootBeforeReview(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable while the rooted handle is open on Windows")
	}
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	rawDir := filepath.Join(dir, "raw")
	rawOriginal := filepath.Join(dir, "raw-original")
	rawReplacement := filepath.Join(dir, "raw-replacement")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review"}}`)
	if err := os.Mkdir(rawReplacement, 0o755); err != nil {
		t.Fatalf("create replacement raw root: %v", err)
	}
	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	matrixPlan.Devices[0].UDID = "raw-root-swap-" + filepath.Base(dir)
	var swapErr error
	swapped := false
	appearance := &matrixTestAppearance{restoreFunc: func() {
		if swapped {
			return
		}
		swapped = true
		if err := os.Rename(rawDir, rawOriginal); err != nil {
			swapErr = err
		} else if err := os.Rename(rawReplacement, rawDir); err != nil {
			swapErr = err
		}
	}}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Appearance: appearance,
	})
	if swapErr != nil {
		t.Fatalf("replace raw output root: %v", swapErr)
	}
	if runErr == nil {
		t.Fatal("RunMatrixWithDependencies() error = nil, want raw-output identity uncertainty")
	}
	if result == nil || len(result.Cells) != 1 {
		t.Fatalf("result = %+v, want one cell", result)
	}
	cell := result.Cells[0]
	if result.Status != MatrixCellFailed || result.Succeeded != 0 || result.Failed != 1 {
		t.Fatalf("result status/counts = %s/%d/%d, want failed/0/1", result.Status, result.Succeeded, result.Failed)
	}
	if cell.FailureStage != "execution" || cell.FailureCode != "raw_output_unavailable" {
		t.Fatalf("cell failure = %s/%s, want execution/raw_output_unavailable", cell.FailureStage, cell.FailureCode)
	}
	if len(cell.RawPaths) != 0 || len(cell.Screenshots) != 1 || cell.Screenshots[0].RawPath != "" {
		t.Fatalf("cell retained stale raw output: %+v", cell)
	}
	if _, err := os.Stat(filepath.Join(rawOriginal, "en-US", "phone", "light", "default", "home.png")); err != nil {
		t.Fatalf("original raw output was not preserved after root swap: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replacement raw root contains a stale claimed artifact: %v", err)
	}
	manifest, err := LoadMatrixReviewManifest(filepath.Join(dir, "review", "manifest.json"))
	if err != nil {
		t.Fatalf("LoadMatrixReviewManifest() error = %v", err)
	}
	if len(manifest.Cells) != 1 || len(manifest.Cells[0].RawPaths) != 0 {
		t.Fatalf("review manifest retained stale raw path: %+v", manifest.Cells)
	}
}

func TestRunMatrixRejectsSymlinkedOutputRootsBeforeRevalidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement is not reliable while the rooted handle is open on Windows")
	}
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	rawDir := filepath.Join(dir, "raw")
	framedDir := filepath.Join(dir, "framed")
	rawOriginal := filepath.Join(dir, "raw-original")
	framedOriginal := filepath.Join(dir, "framed-original")
	rawOutside := filepath.Join(dir, "raw-outside")
	framedOutside := filepath.Join(dir, "framed-outside")
	writeMatrixTestFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeMatrixTestFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","framed_dir":"framed","review_dir":"review","frame":{"enabled":true,"device_by_matrix_device":{"phone":"iphone-17-pro"}}}}`)
	if err := os.Mkdir(rawOutside, 0o755); err != nil {
		t.Fatalf("create outside raw root: %v", err)
	}
	if err := os.Mkdir(framedOutside, 0o755); err != nil {
		t.Fatalf("create outside framed root: %v", err)
	}
	rawCanary := filepath.Join(rawOutside, "en-US", "phone", "light", "default", "home.png")
	framedCanary := filepath.Join(framedOutside, "en-US", "phone", "light", "default", "home.png")
	if err := os.MkdirAll(filepath.Dir(rawCanary), 0o755); err != nil {
		t.Fatalf("create outside raw canary parent: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(framedCanary), 0o755); err != nil {
		t.Fatalf("create outside framed canary parent: %v", err)
	}
	writeMatrixTestFile(t, rawCanary, "outside raw read canary")
	writeMatrixTestFile(t, framedCanary, "outside framed read canary")

	matrixPlan, err := LoadMatrixPlan(matrixPath)
	if err != nil {
		t.Fatalf("LoadMatrixPlan() error = %v", err)
	}
	matrixPlan.Devices[0].UDID = "symlink-output-root-" + filepath.Base(dir)
	var swapErr error
	var swapped bool
	appearance := &matrixTestAppearance{restoreFunc: func() {
		if swapped {
			return
		}
		swapped = true
		if err := os.Rename(rawDir, rawOriginal); err != nil {
			swapErr = err
			return
		}
		if err := os.Symlink(rawOutside, rawDir); err != nil {
			swapErr = err
			return
		}
		if err := os.Rename(framedDir, framedOriginal); err != nil {
			swapErr = err
			return
		}
		if err := os.Symlink(framedOutside, framedDir); err != nil {
			swapErr = err
		}
	}}
	previousOpenHook := matrixArtifactBeforeOpenForTest
	outsideReadAttempted := false
	matrixArtifactBeforeOpenForTest = func(root rootfs.Root, _ string) {
		info, statErr := os.Lstat(root.Path())
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			outsideReadAttempted = true
		}
	}
	t.Cleanup(func() {
		matrixArtifactBeforeOpenForTest = previousOpenHook
		for _, path := range []string{rawDir, framedDir} {
			if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
				if removeErr := os.Remove(path); removeErr != nil {
					t.Errorf("remove replaced output-root symlink %q: %v", path, removeErr)
				}
			}
		}
	})
	deps := MatrixDependencies{
		RunPlan: func(_ context.Context, plan *Plan) (*RunResult, error) {
			writeMatrixPNG(t, filepath.Join(plan.App.OutputDir, "home.png"))
			return &RunResult{}, nil
		},
		Frame: func(_ context.Context, request FrameRequest) (*FrameResult, error) {
			writeMatrixPNG(t, request.OutputPath)
			return &FrameResult{}, nil
		},
		Appearance: appearance,
	}
	result, runErr := RunMatrixWithDependencies(context.Background(), matrixPath, matrixPlan, MatrixOptions{}, deps)
	if swapErr != nil {
		t.Fatalf("replace output roots: %v", swapErr)
	}
	if runErr == nil {
		t.Fatal("RunMatrixWithDependencies() error = nil, want output-root identity uncertainty")
	}
	if outsideReadAttempted {
		t.Fatal("revalidation attempted to open an artifact through a replaced output-root symlink")
	}
	if result == nil || len(result.Cells) != 1 {
		t.Fatalf("result = %+v, want one cell", result)
	}
	cell := result.Cells[0]
	if len(cell.RawPaths) != 0 || len(cell.FramedPaths) != 0 {
		t.Fatalf("result retained artifacts from replaced output roots: %+v", cell)
	}
	if got, readErr := os.ReadFile(rawCanary); readErr != nil || string(got) != "outside raw read canary" {
		t.Fatalf("outside raw canary = %q, read error = %v, want unchanged", got, readErr)
	}
	if got, readErr := os.ReadFile(framedCanary); readErr != nil || string(got) != "outside framed read canary" {
		t.Fatalf("outside framed canary = %q, read error = %v, want unchanged", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(rawOriginal, "en-US", "phone", "light", "default", "home.png")); statErr != nil {
		t.Fatalf("original raw artifact was not preserved: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(framedOriginal, "en-US", "phone", "light", "default", "home.png")); statErr != nil {
		t.Fatalf("original framed artifact was not preserved: %v", statErr)
	}
}

type matrixCancelAfterChecksContext struct {
	checks      int
	cancelAfter int
}

func (c *matrixCancelAfterChecksContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (*matrixCancelAfterChecksContext) Done() <-chan struct{} {
	return nil
}

func (c *matrixCancelAfterChecksContext) Err() error {
	c.checks++
	if c.checks > c.cancelAfter {
		return context.Canceled
	}
	return nil
}

func (*matrixCancelAfterChecksContext) Value(any) any {
	return nil
}

func TestPromoteMatrixArtifactWithInfoStopsAfterPromotionWhenContextCancels(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "raw")
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	source := filepath.Join(outputDir, "source.png")
	destination := filepath.Join(outputDir, "published.png")
	writeMatrixPNG(t, source)
	root, err := rootfs.New(outputDir)
	if err != nil {
		t.Fatalf("open output root: %v", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("close output root: %v", err)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	previous := matrixArtifactBeforeOpenForTest
	matrixArtifactBeforeOpenForTest = func(rootfs.Root, string) {
		cancel()
	}
	t.Cleanup(func() { matrixArtifactBeforeOpenForTest = previous })
	_, err = promoteMatrixArtifactWithInfo(ctx, root, outputDir, source, destination)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("promoteMatrixArtifactWithInfo() error = %v, want context cancellation", err)
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatalf("destination was not published before metadata cancellation: %v", err)
	}
}

func TestPromoteMatrixArtifactWithInfoHonorsCancellationDuringCopy(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "raw")
	if err := os.Mkdir(outputDir, 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	source := filepath.Join(outputDir, "source.png")
	destination := filepath.Join(outputDir, "published.png")
	writeMatrixPNG(t, source)
	root, err := rootfs.New(outputDir)
	if err != nil {
		t.Fatalf("open output root: %v", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("close output root: %v", err)
		}
	})

	ctx := &matrixCancelAfterChecksContext{cancelAfter: 1}
	_, err = promoteMatrixArtifactWithInfo(ctx, root, outputDir, source, destination)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("promoteMatrixArtifactWithInfo() error = %v, want context cancellation", err)
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination stat error = %v, want no committed file after canceled copy", err)
	}
}

func TestRevalidateMatrixRawPathsStopsOnCancellationDuringHash(t *testing.T) {
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	if err := os.Mkdir(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	rawPath := filepath.Join(rawDir, "home.png")
	writeMatrixPNG(t, rawPath)
	rawRoot, err := rootfs.New(rawDir)
	if err != nil {
		t.Fatalf("open raw root: %v", err)
	}
	expected, err := inspectMatrixArtifact(rawRoot, rawDir, rawPath)
	if closeErr := rawRoot.Close(); closeErr != nil {
		t.Fatalf("close raw root: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("inspect expected raw artifact: %v", err)
	}
	result := &MatrixResult{
		RawDir: rawDir,
		Cells: []MatrixCellResult{{
			Status:       MatrixCellSuccess,
			RawPaths:     []string{rawPath},
			Screenshots:  []MatrixScreenshotResult{{RawPath: rawPath, Status: MatrixCellSuccess}},
			rawArtifacts: map[string]matrixArtifactInfo{rawPath: expected},
		}},
	}
	ctx := &matrixCancelAfterChecksContext{cancelAfter: 3}
	err = revalidateMatrixRawPaths(ctx, result)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("revalidateMatrixRawPaths() error = %v, want context cancellation", err)
	}
	if ctx.checks <= ctx.cancelAfter {
		t.Fatalf("context checks = %d, want cancellation during hashing", ctx.checks)
	}
	cell := result.Cells[0]
	if cell.Status != MatrixCellCanceled || cell.FailureCode != "canceled" {
		t.Fatalf("cell status/failure = %s/%s, want canceled/canceled", cell.Status, cell.FailureCode)
	}
	if len(cell.RawPaths) != 0 || cell.Screenshots[0].RawPath != "" {
		t.Fatalf("canceled revalidation retained unverified raw path: %+v", cell)
	}
}

func TestRevalidateMatrixRawPathsUsesPerArtifactBudget(t *testing.T) {
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	if err := os.Mkdir(rawDir, 0o755); err != nil {
		t.Fatalf("create raw directory: %v", err)
	}
	paths := []string{filepath.Join(rawDir, "first.png"), filepath.Join(rawDir, "second.png")}
	for _, path := range paths {
		writeMatrixPNG(t, path)
	}
	rawRoot, err := rootfs.New(rawDir)
	if err != nil {
		t.Fatalf("open raw root: %v", err)
	}
	expected := make(map[string]matrixArtifactInfo, len(paths))
	for _, path := range paths {
		artifact, inspectErr := inspectMatrixArtifact(rawRoot, rawDir, path)
		if inspectErr != nil {
			rawRoot.Close()
			t.Fatalf("inspect expected artifact %q: %v", path, inspectErr)
		}
		expected[path] = artifact
	}
	if err := rawRoot.Close(); err != nil {
		t.Fatalf("close raw root: %v", err)
	}

	result := &MatrixResult{RawDir: rawDir, Cells: []MatrixCellResult{{
		Status:       MatrixCellSuccess,
		RawPaths:     append([]string(nil), paths...),
		Screenshots:  []MatrixScreenshotResult{{RawPath: paths[0], Status: MatrixCellSuccess}, {RawPath: paths[1], Status: MatrixCellSuccess}},
		rawArtifacts: expected,
	}}}
	previousContextHook := matrixRawVerificationContextForTest
	var deadlines []time.Time
	matrixRawVerificationContextForTest = func(ctx context.Context) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Errorf("verification context has no deadline")
			return
		}
		deadlines = append(deadlines, deadline)
	}
	t.Cleanup(func() { matrixRawVerificationContextForTest = previousContextHook })

	if err := revalidateMatrixRawPathsWithBudget(context.Background(), result, 20*time.Millisecond); err != nil {
		t.Fatalf("revalidateMatrixRawPathsWithBudget() error = %v, want each artifact to receive its own budget", err)
	}
	if got := result.Cells[0].RawPaths; !reflect.DeepEqual(got, paths) {
		t.Fatalf("revalidated raw paths = %v, want %v", got, paths)
	}
	if len(deadlines) != len(paths) {
		t.Fatalf("verification context count = %d, want %d", len(deadlines), len(paths))
	}
}

func TestLoadMatrixReviewManifestBindsDecodedGeneration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("review replacement DACL preserve is covered by rootfs tests; this race needs two successful writes")
	}
	dir := t.TempDir()
	first := &MatrixResult{PlanPath: "plan.json", Cells: []MatrixCellResult{{ID: "first-generation", Status: MatrixCellSuccess}}}
	if _, err := GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: first, OutputDir: dir}); err != nil {
		t.Fatalf("GenerateMatrixReview(first) error = %v", err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	var hookErr error
	matrixReviewManifestLoadedForTest = func() {
		second := &MatrixResult{PlanPath: "plan.json", Cells: []MatrixCellResult{{ID: "second-generation", Status: MatrixCellSuccess}}}
		_, hookErr = GenerateMatrixReview(context.Background(), MatrixReviewRequest{Result: second, OutputDir: dir})
	}
	t.Cleanup(func() { matrixReviewManifestLoadedForTest = nil })
	_, err := LoadMatrixReviewManifest(manifestPath)
	if hookErr != nil {
		t.Fatalf("GenerateMatrixReview(second) error = %v", hookErr)
	}
	if !errors.Is(err, errMatrixReviewPairMismatch) {
		t.Fatalf("LoadMatrixReviewManifest() error = %v, want decoded-generation pair mismatch", err)
	}
}
