package cmdtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

type screenshotValidateOutput struct {
	Path         string                          `json:"path"`
	DisplayType  string                          `json:"displayType"`
	TotalFiles   int                             `json:"totalFiles"`
	ReadyFiles   int                             `json:"readyFiles"`
	ErrorCount   int                             `json:"errorCount"`
	WarningCount int                             `json:"warningCount"`
	Files        []screenshotValidateFileOutput  `json:"files"`
	Issues       []screenshotValidateIssueOutput `json:"issues"`
}

type screenshotValidateFileOutput struct {
	Order    int    `json:"order"`
	FileName string `json:"fileName"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Status   string `json:"status,omitempty"`
}

type screenshotValidateIssueOutput struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	FileName    string `json:"fileName,omitempty"`
	DuplicateOf string `json:"duplicateOf,omitempty"`
	Match       string `json:"match,omitempty"`
	Message     string `json:"message"`
}

func TestAssetsScreenshotsValidateOutputReportsOrderingHiddenFilesAndDimensionErrors(t *testing.T) {
	dir := t.TempDir()
	writeScreenshotValidatePNG(t, filepath.Join(dir, "02-details.png"), 1242, 2688, color.RGBA{R: 20, A: 255}, png.DefaultCompression)
	writeScreenshotValidatePNG(t, filepath.Join(dir, "01-home.png"), 1242, 2688, color.RGBA{R: 10, A: 255}, png.DefaultCompression)
	writeScreenshotValidatePNG(t, filepath.Join(dir, ".hidden.png"), 1242, 2688, color.RGBA{R: 30, A: 255}, png.DefaultCompression)
	writePNG(t, filepath.Join(dir, "03-bad.png"), 100, 100)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots", "validate",
			"--path", dir,
			"--device-type", "IPHONE_65",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(t.Context())
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runErr == nil {
		t.Fatal("expected reported validation error, got nil")
	}
	if !strings.Contains(runErr.Error(), "screenshots validate: found 1 error(s)") {
		t.Fatalf("expected reported error count, got %v", runErr)
	}

	var result screenshotValidateOutput
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}

	if result.Path != dir {
		t.Fatalf("expected path %q, got %q", dir, result.Path)
	}
	if result.DisplayType != "APP_IPHONE_65" {
		t.Fatalf("expected normalized display type APP_IPHONE_65, got %q", result.DisplayType)
	}
	if result.TotalFiles != 4 {
		t.Fatalf("expected 4 discovered files, got %d", result.TotalFiles)
	}
	if result.ErrorCount != 1 {
		t.Fatalf("expected 1 error, got %d", result.ErrorCount)
	}
	if result.WarningCount != 1 {
		t.Fatalf("expected 1 warning, got %d", result.WarningCount)
	}
	if len(result.Files) != 4 {
		t.Fatalf("expected 4 file rows, got %d", len(result.Files))
	}

	wantOrder := []string{".hidden.png", "01-home.png", "02-details.png", "03-bad.png"}
	for i, want := range wantOrder {
		if result.Files[i].Order != i+1 {
			t.Fatalf("expected order %d for row %d, got %d", i+1, i, result.Files[i].Order)
		}
		if result.Files[i].FileName != want {
			t.Fatalf("expected file %q at row %d, got %q", want, i, result.Files[i].FileName)
		}
	}

	if !hasScreenshotValidateIssue(result.Issues, "hidden_file", "warning", ".hidden.png") {
		t.Fatalf("expected hidden-file warning in issues, got %+v", result.Issues)
	}
	if !hasScreenshotValidateIssue(result.Issues, "dimension_mismatch", "error", "03-bad.png") {
		t.Fatalf("expected dimension mismatch error in issues, got %+v", result.Issues)
	}
}

func TestAssetsScreenshotsValidateReportsDecodedPixelDuplicates(t *testing.T) {
	dir := t.TempDir()
	originalPath := filepath.Join(dir, "01-original.png")
	duplicatePath := filepath.Join(dir, "02-duplicate.png")
	distinctPath := filepath.Join(dir, "03-distinct.png")
	marker := color.RGBA{R: 10, G: 20, B: 30, A: 255}

	writeScreenshotValidatePNG(t, originalPath, 312, 390, marker, png.BestSpeed)
	writeScreenshotValidatePNG(t, duplicatePath, 312, 390, marker, png.BestCompression)
	writeScreenshotValidatePNG(t, distinctPath, 312, 390, color.RGBA{R: 11, G: 20, B: 30, A: 255}, png.DefaultCompression)

	originalBytes, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	duplicateBytes, err := os.ReadFile(duplicatePath)
	if err != nil {
		t.Fatalf("read duplicate: %v", err)
	}
	if bytes.Equal(originalBytes, duplicateBytes) {
		t.Fatal("expected differently encoded PNG files")
	}

	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "validate",
		"--path", dir,
		"--device-type", "APP_WATCH_SERIES_3",
		"--output", "json",
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runErr == nil {
		t.Fatal("expected reported validation error, got nil")
	}
	if !strings.Contains(runErr.Error(), "screenshots validate: found 1 error(s)") {
		t.Fatalf("expected one duplicate-content error, got %v", runErr)
	}

	var result screenshotValidateOutput
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if result.ErrorCount != 1 {
		t.Fatalf("expected 1 error, got %d", result.ErrorCount)
	}
	if result.ReadyFiles != 2 {
		t.Fatalf("expected 2 ready files, got %d", result.ReadyFiles)
	}
	if len(result.Files) != 3 || result.Files[1].Status != "error" {
		t.Fatalf("expected sorted duplicate file to have error status, got %+v", result.Files)
	}

	var duplicateIssue *screenshotValidateIssueOutput
	for i := range result.Issues {
		issue := &result.Issues[i]
		if issue.Code == "duplicate_content" && issue.FileName == "02-duplicate.png" {
			duplicateIssue = issue
			break
		}
	}
	if duplicateIssue == nil {
		t.Fatalf("expected duplicate-content issue, got %+v", result.Issues)
	}
	if duplicateIssue.Severity != "error" {
		t.Fatalf("expected duplicate error severity, got %q", duplicateIssue.Severity)
	}
	if duplicateIssue.DuplicateOf != "01-original.png" {
		t.Fatalf("expected duplicateOf 01-original.png, got %q", duplicateIssue.DuplicateOf)
	}
	if duplicateIssue.Match != "pixels" {
		t.Fatalf("expected pixel match, got %q", duplicateIssue.Match)
	}
}

func TestAssetsScreenshotsValidateReportsFormatExtensionMismatch(t *testing.T) {
	dir := t.TempDir()
	writeScreenshotValidatePNG(t, filepath.Join(dir, "01-home.png"), 1242, 2688, color.RGBA{R: 10, A: 255}, png.DefaultCompression)
	writeCmdtestScreenshotJPEG(t, dir, "02-details.png")

	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "validate",
		"--path", dir,
		"--device-type", "IPHONE_65",
		"--output", "json",
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runErr == nil {
		t.Fatal("expected reported validation error, got nil")
	}
	if !strings.Contains(runErr.Error(), "screenshots validate: found 1 error(s)") {
		t.Fatalf("expected one reported error, got %v", runErr)
	}
	if code := cmd.ExitCodeFromError(runErr); code == cmd.ExitSuccess {
		t.Fatalf("expected a non-zero exit code for a reported validation error, got %d", code)
	}

	var result screenshotValidateOutput
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if result.ErrorCount != 1 {
		t.Fatalf("expected 1 error, got %d", result.ErrorCount)
	}
	if result.ReadyFiles != 1 {
		t.Fatalf("expected 1 ready file, got %d", result.ReadyFiles)
	}
	if !hasScreenshotValidateIssue(result.Issues, "format_mismatch", "error", "02-details.png") {
		t.Fatalf("expected format mismatch issue, got %+v", result.Issues)
	}
	for _, issue := range result.Issues {
		if issue.Code != "format_mismatch" {
			continue
		}
		for _, want := range []string{"JPEG", "02-details.jpg", "PNG"} {
			if !strings.Contains(issue.Message, want) {
				t.Fatalf("expected format mismatch message to mention %q, got %q", want, issue.Message)
			}
		}
	}
}

func hasScreenshotValidateIssue(issues []screenshotValidateIssueOutput, code, severity, fileName string) bool {
	for _, issue := range issues {
		if issue.Code == code && issue.Severity == severity && issue.FileName == fileName {
			return true
		}
	}
	return false
}

func writeScreenshotValidatePNG(t *testing.T, path string, width, height int, marker color.RGBA, compression png.CompressionLevel) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.SetRGBA(0, 0, marker)

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	defer file.Close()

	encoder := png.Encoder{CompressionLevel: compression}
	if err := encoder.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

func TestAssetsScreenshotsValidateReportsUnreadableHiddenDotfiles(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "01-home.png"), 1242, 2688)
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("not an image"), 0o644); err != nil {
		t.Fatalf("write dotfile: %v", err)
	}

	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "validate",
		"--path", dir,
		"--device-type", "IPHONE_65",
		"--output", "json",
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runErr == nil {
		t.Fatal("expected reported validation error, got nil")
	}

	var result screenshotValidateOutput
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}

	if !hasScreenshotValidateIssue(result.Issues, "hidden_file", "warning", ".DS_Store") {
		t.Fatalf("expected hidden dotfile warning, got %+v", result.Issues)
	}
	if !hasScreenshotValidateIssue(result.Issues, "read_failure", "error", ".DS_Store") {
		t.Fatalf("expected unreadable-dotfile error, got %+v", result.Issues)
	}
}

func TestAssetsScreenshotsValidateRejectsInvalidDeviceTypeAsUsageError(t *testing.T) {
	dir := t.TempDir()

	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "validate",
		"--path", dir,
		"--device-type", "not-a-device",
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "unsupported screenshot display type") {
		t.Fatalf("expected invalid device-type error, got %q", stderr)
	}
}

func TestAssetsScreenshotsValidatePreservesRawPathSemanticsForFileWithTrailingSeparator(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")
	writePNG(t, path, 1242, 2688)

	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "validate",
		"--path", path + string(filepath.Separator),
		"--device-type", "IPHONE_65",
		"--output", "json",
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if runErr == nil {
		t.Fatal("expected path error, got nil")
	}
	if !strings.Contains(runErr.Error(), "screenshots validate:") {
		t.Fatalf("expected validate command context, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "not a directory") {
		t.Fatalf("expected not-a-directory error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
