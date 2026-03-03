package screenshots

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateReview_WritesManifestAndHTML(t *testing.T) {
	baseDir := t.TempDir()
	rawDir := filepath.Join(baseDir, "raw")
	framedDir := filepath.Join(baseDir, "framed")
	outputDir := filepath.Join(baseDir, "review")

	writeReviewImage(t, filepath.Join(rawDir, "home.png"), 1320, 2868)
	writeReviewImage(t, filepath.Join(framedDir, "en", "iPhone_Air", "home.png"), 1320, 2868)
	writeReviewImage(t, filepath.Join(framedDir, "en", "iPhone_Air", "details.png"), 1000, 1000)

	approvalPath := filepath.Join(outputDir, defaultReviewApprovalsName)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(approvalPath, []byte(`["en|iPhone_Air|home"]`), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	result, err := GenerateReview(context.Background(), ReviewRequest{
		RawDir:    rawDir,
		FramedDir: framedDir,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("GenerateReview() error: %v", err)
	}

	if _, err := os.Stat(result.ManifestPath); err != nil {
		t.Fatalf("expected manifest at %q: %v", result.ManifestPath, err)
	}
	if _, err := os.Stat(result.HTMLPath); err != nil {
		t.Fatalf("expected HTML report at %q: %v", result.HTMLPath, err)
	}

	manifestData, err := os.ReadFile(result.ManifestPath)
	if err != nil {
		t.Fatalf("ReadFile(manifest) error: %v", err)
	}
	var manifest ReviewManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Unmarshal(manifest) error: %v", err)
	}

	if manifest.Summary.Total != 2 {
		t.Fatalf("expected total=2, got %d", manifest.Summary.Total)
	}
	if manifest.Summary.Ready != 1 {
		t.Fatalf("expected ready=1, got %d", manifest.Summary.Ready)
	}
	if manifest.Summary.MissingRaw != 1 {
		t.Fatalf("expected missing_raw=1, got %d", manifest.Summary.MissingRaw)
	}
	if manifest.Summary.InvalidSize != 1 {
		t.Fatalf("expected invalid_size=1, got %d", manifest.Summary.InvalidSize)
	}
	if manifest.Summary.Approved != 1 {
		t.Fatalf("expected approved=1, got %d", manifest.Summary.Approved)
	}
	if manifest.Summary.PendingApproval != 1 {
		t.Fatalf("expected pending=1, got %d", manifest.Summary.PendingApproval)
	}

	home := findReviewEntryByID(t, manifest.Entries, "home")
	if home.Status != reviewStatusReady {
		t.Fatalf("home status=%q, want %q", home.Status, reviewStatusReady)
	}
	if !home.Approved {
		t.Fatal("expected home to be approved")
	}
	if !home.ValidAppStoreSize {
		t.Fatal("expected home to have valid App Store size")
	}
	if len(home.DisplayTypes) == 0 {
		t.Fatal("expected home display types to be populated")
	}

	details := findReviewEntryByID(t, manifest.Entries, "details")
	if details.Status != reviewStatusMissingAndInvalid {
		t.Fatalf("details status=%q, want %q", details.Status, reviewStatusMissingAndInvalid)
	}
	if details.Approved {
		t.Fatal("expected details approval to be pending")
	}
	if details.ValidAppStoreSize {
		t.Fatal("expected details to be invalid App Store size")
	}

	htmlData, err := os.ReadFile(result.HTMLPath)
	if err != nil {
		t.Fatalf("ReadFile(html) error: %v", err)
	}
	html := string(htmlData)
	if !strings.Contains(html, "ASC Shots Review") {
		t.Fatalf("expected report title in HTML, got: %q", html)
	}
	if !strings.Contains(html, "home") {
		t.Fatalf("expected screenshot ID in HTML, got: %q", html)
	}
}

func TestGenerateReview_RequiresFramedDirectory(t *testing.T) {
	_, err := GenerateReview(context.Background(), ReviewRequest{
		FramedDir: filepath.Join(t.TempDir(), "missing"),
	})
	if err == nil {
		t.Fatal("expected error for missing framed directory")
	}
	if !strings.Contains(err.Error(), "read framed directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateReview_MatchesRawByLocaleAndDevicePath(t *testing.T) {
	baseDir := t.TempDir()
	rawDir := filepath.Join(baseDir, "raw")
	framedDir := filepath.Join(baseDir, "framed")
	outputDir := filepath.Join(baseDir, "review")

	writeReviewImage(t, filepath.Join(rawDir, "en", "iPhone_Air", "home.png"), 1320, 2868)
	writeReviewImage(t, filepath.Join(rawDir, "fr", "iPhone_Air", "home.png"), 1320, 2868)
	writeReviewImage(t, filepath.Join(framedDir, "en", "iPhone_Air", "home.png"), 1320, 2868)
	writeReviewImage(t, filepath.Join(framedDir, "fr", "iPhone_Air", "home.png"), 1320, 2868)

	result, err := GenerateReview(context.Background(), ReviewRequest{
		RawDir:    rawDir,
		FramedDir: framedDir,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("GenerateReview() error: %v", err)
	}

	manifestData, err := os.ReadFile(result.ManifestPath)
	if err != nil {
		t.Fatalf("ReadFile(manifest) error: %v", err)
	}
	var manifest ReviewManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Unmarshal(manifest) error: %v", err)
	}

	en := findReviewEntryByIDAndLocale(t, manifest.Entries, "home", "en")
	if en.RawRelative != filepath.ToSlash(filepath.Join("en", "iPhone_Air", "home.png")) {
		t.Fatalf("en raw path = %q", en.RawRelative)
	}
	if en.Status != reviewStatusReady {
		t.Fatalf("en status = %q, want %q", en.Status, reviewStatusReady)
	}

	fr := findReviewEntryByIDAndLocale(t, manifest.Entries, "home", "fr")
	if fr.RawRelative != filepath.ToSlash(filepath.Join("fr", "iPhone_Air", "home.png")) {
		t.Fatalf("fr raw path = %q", fr.RawRelative)
	}
	if fr.Status != reviewStatusReady {
		t.Fatalf("fr status = %q, want %q", fr.Status, reviewStatusReady)
	}
}

func TestGenerateReview_DoesNotFallbackRawAcrossDifferentDevice(t *testing.T) {
	baseDir := t.TempDir()
	rawDir := filepath.Join(baseDir, "raw")
	framedDir := filepath.Join(baseDir, "framed")
	outputDir := filepath.Join(baseDir, "review")

	writeReviewImage(t, filepath.Join(rawDir, "en", "iPhone_17_Pro", "home.png"), 1320, 2868)
	writeReviewImage(t, filepath.Join(framedDir, "en", "iPhone_Air", "home.png"), 1320, 2868)

	result, err := GenerateReview(context.Background(), ReviewRequest{
		RawDir:    rawDir,
		FramedDir: framedDir,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("GenerateReview() error: %v", err)
	}

	manifestData, err := os.ReadFile(result.ManifestPath)
	if err != nil {
		t.Fatalf("ReadFile(manifest) error: %v", err)
	}
	var manifest ReviewManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Unmarshal(manifest) error: %v", err)
	}

	entry := findReviewEntryByIDAndLocale(t, manifest.Entries, "home", "en")
	if entry.Device != "iPhone_Air" {
		t.Fatalf("entry device = %q, want %q", entry.Device, "iPhone_Air")
	}
	if entry.RawPath != "" || entry.RawRelative != "" {
		t.Fatalf("expected no raw match, got raw path %q (%q)", entry.RawPath, entry.RawRelative)
	}
	if entry.Status != reviewStatusMissingRaw {
		t.Fatalf("entry status = %q, want %q", entry.Status, reviewStatusMissingRaw)
	}
}

func TestGenerateReview_DoesNotFallbackRawWhenScreenshotIDIsAmbiguous(t *testing.T) {
	baseDir := t.TempDir()
	rawDir := filepath.Join(baseDir, "raw")
	framedDir := filepath.Join(baseDir, "framed")
	outputDir := filepath.Join(baseDir, "review")

	writeReviewImage(t, filepath.Join(rawDir, "devA", "home.png"), 1320, 2868)
	writeReviewImage(t, filepath.Join(rawDir, "devB", "home.png"), 1320, 2868)
	writeReviewImage(t, filepath.Join(rawDir, "devC", "home.png"), 1320, 2868)
	writeReviewImage(t, filepath.Join(framedDir, "home.png"), 1320, 2868)

	result, err := GenerateReview(context.Background(), ReviewRequest{
		RawDir:    rawDir,
		FramedDir: framedDir,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("GenerateReview() error: %v", err)
	}

	manifestData, err := os.ReadFile(result.ManifestPath)
	if err != nil {
		t.Fatalf("ReadFile(manifest) error: %v", err)
	}
	var manifest ReviewManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Unmarshal(manifest) error: %v", err)
	}

	entry := findReviewEntryByID(t, manifest.Entries, "home")
	if entry.RawPath != "" || entry.RawRelative != "" {
		t.Fatalf("expected no raw fallback for ambiguous screenshot id, got raw path %q (%q)", entry.RawPath, entry.RawRelative)
	}
	if entry.Status != reviewStatusMissingRaw {
		t.Fatalf("entry status = %q, want %q", entry.Status, reviewStatusMissingRaw)
	}
}

func TestPathOnlyURLPath_PrefixesWindowsDrivePath(t *testing.T) {
	got := pathOnlyURLPath("C:/Users/dev/screenshots/home.png")
	want := "/C:/Users/dev/screenshots/home.png"
	if got != want {
		t.Fatalf("pathOnlyURLPath() = %q, want %q", got, want)
	}
}

func TestPathOnlyURLPath_PreservesUnixAbsolutePath(t *testing.T) {
	got := pathOnlyURLPath("/tmp/screenshots/home.png")
	want := "/tmp/screenshots/home.png"
	if got != want {
		t.Fatalf("pathOnlyURLPath() = %q, want %q", got, want)
	}
}

func findReviewEntryByID(t *testing.T, entries []ReviewEntry, screenshotID string) ReviewEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.ScreenshotID == screenshotID {
			return entry
		}
	}
	t.Fatalf("entry not found for screenshot id %q", screenshotID)
	return ReviewEntry{}
}

func findReviewEntryByIDAndLocale(t *testing.T, entries []ReviewEntry, screenshotID, locale string) ReviewEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.ScreenshotID == screenshotID && entry.Locale == locale {
			return entry
		}
	}
	t.Fatalf("entry not found for screenshot id %q locale %q", screenshotID, locale)
	return ReviewEntry{}
}

func writeReviewImage(t *testing.T, path string, width, height int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", filepath.Dir(path), err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%q) error: %v", path, err)
	}
	defer func() { _ = file.Close() }()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 255) / max(width, 1)),
				G: uint8((y * 255) / max(height, 1)),
				B: 200,
				A: 255,
			})
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode(%q) error: %v", path, err)
	}
}
