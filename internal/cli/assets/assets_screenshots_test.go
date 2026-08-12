package assets

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAssetsScreenshotsSizesCommandDefaultFocused(t *testing.T) {
	cmd := AssetsScreenshotsSizesCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), cmd.FlagSet.Args()); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.ScreenshotSizesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(result.Sizes) != 2 {
		t.Fatalf("expected 2 default focused entries, got %d", len(result.Sizes))
	}

	if result.Sizes[0].DisplayType != "APP_IPHONE_65" {
		t.Fatalf("expected first focused type APP_IPHONE_65, got %q", result.Sizes[0].DisplayType)
	}
	if result.Sizes[1].DisplayType != "APP_IPAD_PRO_3GEN_129" {
		t.Fatalf("expected second focused type APP_IPAD_PRO_3GEN_129, got %q", result.Sizes[1].DisplayType)
	}
}

func TestOrderScreenshotsForDownloadUsesRelationshipOrder(t *testing.T) {
	shots := []asc.Resource[asc.AppScreenshotAttributes]{
		{ID: "shot-b", Attributes: asc.AppScreenshotAttributes{FileName: "01-home.png"}},
		{ID: "shot-c", Attributes: asc.AppScreenshotAttributes{FileName: "02-settings.png"}},
		{ID: "shot-a", Attributes: asc.AppScreenshotAttributes{FileName: "03-paywall.png"}},
	}

	ordered := orderScreenshotsForDownload(shots, []string{"shot-a", "shot-b"})

	gotIDs := make([]string, 0, len(ordered))
	for _, shot := range ordered {
		gotIDs = append(gotIDs, shot.ID)
	}
	wantIDs := []string{"shot-a", "shot-b", "shot-c"}
	if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("ordered IDs = %v, want %v", gotIDs, wantIDs)
	}
}

func TestAssetsScreenshotsDownloadCommandRequiredFlags(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{
			name:   "missing source",
			stderr: "Error: --id or --version-localization is required\n",
		},
		{
			name:   "missing output file",
			args:   []string{"--id", "shot-1"},
			stderr: "Error: --output is required with --id\n",
		},
		{
			name:   "missing output directory",
			args:   []string{"--version-localization", "loc-1"},
			stderr: "Error: --output-dir is required with --version-localization\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := AssetsScreenshotsDownloadCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != tt.stderr {
				t.Fatalf("stderr = %q, want %q", stderr, tt.stderr)
			}
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("error = %v, want flag.ErrHelp", runErr)
			}
		})
	}
}

func TestResolveScreenshotDownloadURLPreservesMetadataFetchError(t *testing.T) {
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appScreenshots/shot-1" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		writeAssetsTestJSON(w, http.StatusServiceUnavailable, `{"errors":[{"status":"503","detail":"metadata unavailable"}]}`)
	}))

	_, err := resolveScreenshotDownloadURL(
		context.Background(),
		client,
		asc.Resource[asc.AppScreenshotAttributes]{
			ID:         "shot-1",
			Attributes: asc.AppScreenshotAttributes{FileName: "home.png"},
		},
	)
	if err == nil {
		t.Fatal("expected screenshot metadata error")
	}
	if !strings.Contains(err.Error(), "metadata unavailable") {
		t.Fatalf("error = %v, want original metadata failure", err)
	}
}

func TestAssetsScreenshotsSizesCommandFilter(t *testing.T) {
	cmd := AssetsScreenshotsSizesCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--display-type", "APP_IPHONE_65"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), cmd.FlagSet.Args()); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.ScreenshotSizesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(result.Sizes) != 1 {
		t.Fatalf("expected 1 size entry, got %d", len(result.Sizes))
	}
	if result.Sizes[0].DisplayType != "APP_IPHONE_65" {
		t.Fatalf("expected APP_IPHONE_65, got %q", result.Sizes[0].DisplayType)
	}
}

func TestAssetsScreenshotsSizesCommandSupportsIPhone69Alias(t *testing.T) {
	cmd := AssetsScreenshotsSizesCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--display-type", "IPHONE_69"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), cmd.FlagSet.Args()); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.ScreenshotSizesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(result.Sizes) != 1 {
		t.Fatalf("expected 1 size entry, got %d", len(result.Sizes))
	}
	if result.Sizes[0].DisplayType != "APP_IPHONE_69" {
		t.Fatalf("expected APP_IPHONE_69, got %q", result.Sizes[0].DisplayType)
	}
}

func TestAssetsScreenshotsSizesCommandSupportsIMessageIPhone69Alias(t *testing.T) {
	cmd := AssetsScreenshotsSizesCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--display-type", "IMESSAGE_APP_IPHONE_69"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), cmd.FlagSet.Args()); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.ScreenshotSizesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(result.Sizes) != 1 {
		t.Fatalf("expected 1 size entry, got %d", len(result.Sizes))
	}
	if result.Sizes[0].DisplayType != "IMESSAGE_APP_IPHONE_69" {
		t.Fatalf("expected IMESSAGE_APP_IPHONE_69, got %q", result.Sizes[0].DisplayType)
	}
}

func TestAssetsScreenshotsSizesCommandAllIncludesNonFocusedTypes(t *testing.T) {
	cmd := AssetsScreenshotsSizesCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--all"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), cmd.FlagSet.Args()); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.ScreenshotSizesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(result.Sizes) <= 2 {
		t.Fatalf("expected --all to return more than focused entries, got %d", len(result.Sizes))
	}

	foundDesktop := false
	for _, entry := range result.Sizes {
		if entry.DisplayType == "APP_DESKTOP" {
			foundDesktop = true
			break
		}
	}
	if !foundDesktop {
		t.Fatal("expected APP_DESKTOP in --all sizes output")
	}
}

func TestAssetsScreenshotsSizesCommandRejectsAllWithDisplayType(t *testing.T) {
	cmd := AssetsScreenshotsSizesCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--all", "--display-type", "APP_IPHONE_65"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--display-type and --all are mutually exclusive") {
		t.Fatalf("expected mutually exclusive error in stderr, got %q", stderr)
	}
}

func TestAssetsScreenshotsUploadCommandRejectsSkipExistingWithReplace(t *testing.T) {
	cmd := AssetsScreenshotsUploadCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--version-localization", "LOC_ID",
		"--path", "./screenshots",
		"--device-type", "IPHONE_65",
		"--skip-existing",
		"--replace",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--skip-existing and --replace are mutually exclusive") {
		t.Fatalf("expected mutually exclusive error in stderr, got %q", stderr)
	}
}

func TestAssetsScreenshotsUploadCommandRequiresUploadMode(t *testing.T) {
	cmd := AssetsScreenshotsUploadCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse(nil); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	const want = "Error: choose an upload mode: --version-localization VERSION_LOCALIZATION_ID; --app APP_ID with --version VERSION or --version-id VERSION_ID; or --resume ARTIFACT_PATH\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestAssetsScreenshotsUploadCommandRejectsMaxScreenshotsWithResume(t *testing.T) {
	cmd := AssetsScreenshotsUploadCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--resume", ".asc/reports/screenshots-upload/failures.json",
		"--max-screenshots", "10",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--resume cannot be combined with --skip-existing, --replace, --confirm, --dry-run, or --max-screenshots") {
		t.Fatalf("expected max-screenshots resume error in stderr, got %q", stderr)
	}
}

func TestExecuteScreenshotUploadCommandRejectsMoreThanTenScreenshotsBeforeAuth(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 11; i++ {
		writeAssetsTestPNGWithSize(t, dir, fmt.Sprintf("%02d-home.png", i), 1242, 2688)
	}

	clientCalled := false
	_, err := executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
		VersionLocalizationID: "LOC_ID",
		Path:                  dir,
		DeviceType:            "IPHONE_65",
	}, screenshotUploadDependencies{
		GetClient: func() (*asc.Client, error) {
			clientCalled = true
			return &asc.Client{}, nil
		},
	})

	if err == nil {
		t.Fatal("expected screenshot-count error")
	}
	if !shared.IsValidationError(err) {
		t.Fatalf("expected shared validation error, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "allow at most 10 images") {
		t.Fatalf("expected max screenshot guidance, got %v", err)
	}
	if !strings.Contains(err.Error(), "--max-screenshots 10") {
		t.Fatalf("expected --max-screenshots guidance, got %v", err)
	}
	if clientCalled {
		t.Fatal("expected local screenshot-count validation before auth/client creation")
	}
}

func TestExecuteScreenshotUploadCommandRejectsMaxScreenshotsAboveAppleLimit(t *testing.T) {
	clientCalled := false
	var err error
	_, stderr := captureOutput(t, func() {
		_, err = executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
			VersionLocalizationID: "LOC_ID",
			Path:                  "unused",
			DeviceType:            "IPHONE_65",
			MaxScreenshots:        11,
		}, screenshotUploadDependencies{
			GetClient: func() (*asc.Client, error) {
				clientCalled = true
				return &asc.Client{}, nil
			},
		})
	})

	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if !strings.Contains(stderr, "--max-screenshots cannot exceed 10") {
		t.Fatalf("expected max-screenshots limit error, got %q", stderr)
	}
	if clientCalled {
		t.Fatal("expected max-screenshots validation before auth/client creation")
	}
}

func TestLimitScreenshotUploadFilesRejectsLimitAboveAppleMaximum(t *testing.T) {
	_, err := limitScreenshotUploadFiles([]string{"one.png"}, appScreenshotSetMaxScreenshots+1, "screenshots")
	if err == nil {
		t.Fatal("expected max-screenshots validation error")
	}
	if !strings.Contains(err.Error(), "--max-screenshots") || !strings.Contains(err.Error(), "10") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLimitScreenshotUploadFilesForExistingSetValidatesReplaceBeforeDelete(t *testing.T) {
	files := make([]string, appScreenshotSetMaxScreenshots+1)
	for i := range files {
		files[i] = fmt.Sprintf("%02d.png", i+1)
	}

	_, err := limitScreenshotUploadFilesForExistingSet(files, 0, nil, true, "set-1")
	if err == nil {
		t.Fatal("expected replacement upload above Apple maximum to fail")
	}
	if !strings.Contains(err.Error(), "allow at most 10") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteScreenshotUploadCommandMaxScreenshotsCapsSortedFiles(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 11; i++ {
		writeAssetsTestPNGWithSize(t, dir, fmt.Sprintf("%02d-home.png", i), 1242, 2688)
	}

	var gotFiles []string
	result, err := executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
		VersionLocalizationID: "LOC_ID",
		Path:                  dir,
		DeviceType:            "IPHONE_65",
		MaxScreenshots:        10,
	}, screenshotUploadDependencies{
		GetClient: func() (*asc.Client, error) {
			return &asc.Client{}, nil
		},
		ExecuteUpload: func(_ context.Context, cfg screenshotUploadConfig[asc.AppScreenshotUploadResult], _ string) (asc.AppScreenshotUploadResult, error) {
			gotFiles = append([]string(nil), cfg.Files...)
			return asc.AppScreenshotUploadResult{
				VersionLocalizationID: cfg.LocalizationID,
				DisplayType:           cfg.DisplayType,
				Total:                 len(cfg.Files),
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("executeScreenshotUploadCommand() error: %v", err)
	}
	uploadResult, ok := result.(*asc.AppScreenshotUploadResult)
	if !ok {
		t.Fatalf("expected *asc.AppScreenshotUploadResult, got %T", result)
	}
	if uploadResult.Total != 10 {
		t.Fatalf("expected capped result total 10, got %d", uploadResult.Total)
	}
	if len(gotFiles) != 10 {
		t.Fatalf("expected 10 files passed to upload, got %d", len(gotFiles))
	}
	if !strings.HasSuffix(gotFiles[0], "01-home.png") || !strings.HasSuffix(gotFiles[9], "10-home.png") {
		t.Fatalf("expected first 10 sorted screenshots, got %#v", gotFiles)
	}
}

func TestExecuteScreenshotUploadCommandCanonicalizesDisplayTypeBeforeASCRequests(t *testing.T) {
	dir := t.TempDir()
	writeAssetsTestPNGWithSize(t, dir, "01-home.png", 1260, 2736)

	var gotDisplayType string
	_, err := executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
		VersionLocalizationID: "LOC_ID",
		Path:                  dir,
		DeviceType:            "IPHONE_69",
	}, screenshotUploadDependencies{
		GetClient: func() (*asc.Client, error) {
			return &asc.Client{}, nil
		},
		ExecuteUpload: func(_ context.Context, cfg screenshotUploadConfig[asc.AppScreenshotUploadResult], _ string) (asc.AppScreenshotUploadResult, error) {
			gotDisplayType = cfg.DisplayType
			return asc.AppScreenshotUploadResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("executeScreenshotUploadCommand() error: %v", err)
	}
	if gotDisplayType != "APP_IPHONE_67" {
		t.Fatalf("display type sent to ASC = %q, want APP_IPHONE_67", gotDisplayType)
	}
}

func TestExecuteScreenshotUploadCommandMaxScreenshotsCapsBeforeDimensionValidation(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 10; i++ {
		writeAssetsTestPNGWithSize(t, dir, fmt.Sprintf("%02d-home.png", i), 1242, 2688)
	}
	writeAssetsTestPNGWithSize(t, dir, "11-wrong-size.png", 100, 100)

	var gotFiles []string
	_, err := executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
		VersionLocalizationID: "LOC_ID",
		Path:                  dir,
		DeviceType:            "IPHONE_65",
		MaxScreenshots:        10,
	}, screenshotUploadDependencies{
		GetClient: func() (*asc.Client, error) {
			return &asc.Client{}, nil
		},
		ExecuteUpload: func(_ context.Context, cfg screenshotUploadConfig[asc.AppScreenshotUploadResult], _ string) (asc.AppScreenshotUploadResult, error) {
			gotFiles = append([]string(nil), cfg.Files...)
			return asc.AppScreenshotUploadResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("executeScreenshotUploadCommand() error: %v", err)
	}
	if len(gotFiles) != 10 {
		t.Fatalf("expected 10 capped files, got %d", len(gotFiles))
	}
	for _, file := range gotFiles {
		if strings.HasSuffix(file, "11-wrong-size.png") {
			t.Fatalf("expected capped files to exclude wrong-size screenshot, got %#v", gotFiles)
		}
	}
}

func TestExecuteScreenshotUploadCommandMaxScreenshotsCapsBeforeImageValidation(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 10; i++ {
		writeAssetsTestPNGWithSize(t, dir, fmt.Sprintf("%02d-home.png", i), 1242, 2688)
	}
	if err := os.WriteFile(filepath.Join(dir, "11-corrupt.png"), []byte("not an image"), 0o644); err != nil {
		t.Fatalf("write corrupt screenshot: %v", err)
	}

	var gotFiles []string
	_, err := executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
		VersionLocalizationID: "LOC_ID",
		Path:                  dir,
		DeviceType:            "IPHONE_65",
		MaxScreenshots:        10,
	}, screenshotUploadDependencies{
		GetClient: func() (*asc.Client, error) {
			return &asc.Client{}, nil
		},
		ExecuteUpload: func(_ context.Context, cfg screenshotUploadConfig[asc.AppScreenshotUploadResult], _ string) (asc.AppScreenshotUploadResult, error) {
			gotFiles = append([]string(nil), cfg.Files...)
			return asc.AppScreenshotUploadResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("executeScreenshotUploadCommand() error: %v", err)
	}
	if len(gotFiles) != 10 {
		t.Fatalf("expected 10 capped files, got %d", len(gotFiles))
	}
	for _, file := range gotFiles {
		if strings.HasSuffix(file, "11-corrupt.png") {
			t.Fatalf("expected capped files to exclude corrupt screenshot, got %#v", gotFiles)
		}
	}
}

func TestNormalizeScreenshotDisplayTypeAliasIPhone69Variants(t *testing.T) {
	testCases := []struct {
		input string
		want  string
	}{
		{input: "IPHONE_69", want: "APP_IPHONE_69"},
		{input: "APP_IPHONE_69", want: "APP_IPHONE_69"},
		{input: "imessage_app_iphone_69", want: "IMESSAGE_APP_IPHONE_69"},
	}

	for _, tc := range testCases {
		got, err := normalizeScreenshotDisplayType(tc.input)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("expected %q for %q, got %q", tc.want, tc.input, got)
		}
	}
}

func TestFocusedScreenshotDisplayTypesForPlatformUnknownPlatformReturnsEmpty(t *testing.T) {
	got := focusedScreenshotDisplayTypesForPlatform("CARPLAY_OS")
	if len(got) != 0 {
		t.Fatalf("expected no focused display types for unknown platform, got %#v", got)
	}
}

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	origStdout := os.Stdout
	origStderr := os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	fn()

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	outBytes, _ := io.ReadAll(rOut)
	errBytes, _ := io.ReadAll(rErr)
	_ = rOut.Close()
	_ = rErr.Close()

	return string(outBytes), string(errBytes)
}
