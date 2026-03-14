package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func TestVersionLengthIssuesBoundaries(t *testing.T) {
	noIssues := versionLengthIssues("file", "1.2.3", "en-US", VersionLocalization{
		Description:     strings.Repeat("a", validation.LimitDescription),
		Keywords:        strings.Repeat("b", validation.LimitKeywords),
		WhatsNew:        strings.Repeat("c", validation.LimitWhatsNew),
		PromotionalText: strings.Repeat("d", validation.LimitPromotionalText),
	})
	if len(noIssues) != 0 {
		t.Fatalf("expected no issues at limits, got %+v", noIssues)
	}

	withIssues := versionLengthIssues("file", "1.2.3", "en-US", VersionLocalization{
		Description:     strings.Repeat("a", validation.LimitDescription+1),
		Keywords:        strings.Repeat("b", validation.LimitKeywords+1),
		WhatsNew:        strings.Repeat("c", validation.LimitWhatsNew+1),
		PromotionalText: strings.Repeat("d", validation.LimitPromotionalText+1),
	})
	if len(withIssues) != 4 {
		t.Fatalf("expected 4 issues above limits, got %d", len(withIssues))
	}
}

func TestAppInfoLengthIssuesBoundaries(t *testing.T) {
	noIssues := appInfoLengthIssues("file", "en-US", AppInfoLocalization{
		Name:     strings.Repeat("n", validation.LimitName),
		Subtitle: strings.Repeat("s", validation.LimitSubtitle),
	})
	if len(noIssues) != 0 {
		t.Fatalf("expected no issues at limits, got %+v", noIssues)
	}

	withIssues := appInfoLengthIssues("file", "en-US", AppInfoLocalization{
		Name:     strings.Repeat("n", validation.LimitName+1),
		Subtitle: strings.Repeat("s", validation.LimitSubtitle+1),
	})
	if len(withIssues) != 2 {
		t.Fatalf("expected 2 issues above limits, got %d", len(withIssues))
	}
}

func TestLengthValidationCountsMultibyteRunes(t *testing.T) {
	noIssues := versionLengthIssues("file", "1.2.3", "ja", VersionLocalization{
		Description: strings.Repeat("あ", validation.LimitDescription),
	})
	if len(noIssues) != 0 {
		t.Fatalf("expected no issue at multibyte rune limit, got %+v", noIssues)
	}

	withIssue := versionLengthIssues("file", "1.2.3", "ja", VersionLocalization{
		Description: strings.Repeat("あ", validation.LimitDescription+1),
	})
	if len(withIssue) != 1 {
		t.Fatalf("expected one issue above multibyte rune limit, got %+v", withIssue)
	}
}

func TestValidateDirTreatsDefaultLocaleCaseInsensitively(t *testing.T) {
	dir := t.TempDir()
	version := "1.2.3"

	if err := os.MkdirAll(filepath.Join(dir, appInfoDirName), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, versionDirName, version), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, appInfoDirName, "Default.json"), []byte(`{"name":"Default App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info default file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, versionDirName, version, "DeFaUlT.json"), []byte(`{"description":"Default description"}`), 0o644); err != nil {
		t.Fatalf("write version default file: %v", err)
	}

	result, err := validateDir(dir, false)
	if err != nil {
		t.Fatalf("validateDir() error: %v", err)
	}
	if result.FilesScanned != 2 {
		t.Fatalf("expected 2 files scanned, got %d", result.FilesScanned)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %+v", result.Issues)
	}
}

func TestValidateDirAllowsDefaultAppInfoWithoutName(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, appInfoDirName), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, appInfoDirName, "default.json"), []byte(`{"subtitle":"My Subtitle"}`), 0o644); err != nil {
		t.Fatalf("write app-info default file: %v", err)
	}

	result, err := validateDir(dir, false)
	if err != nil {
		t.Fatalf("validateDir() error: %v", err)
	}
	if result.FilesScanned != 1 {
		t.Fatalf("expected 1 file scanned, got %d", result.FilesScanned)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %+v", result.Issues)
	}
	if !result.Valid {
		t.Fatalf("expected valid=true, got %+v", result)
	}
}

func TestValidateDirNormalizesVersionDefaultLocaleInIssues(t *testing.T) {
	dir := t.TempDir()
	version := "1.2.3"
	versionPath := filepath.Join(dir, versionDirName, version)
	if err := os.MkdirAll(versionPath, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionPath, "DeFaUlT.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write version default file: %v", err)
	}

	result, err := validateDir(dir, false)
	if err != nil {
		t.Fatalf("validateDir() error: %v", err)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %+v", result.Issues)
	}
	if result.Issues[0].Locale != DefaultLocale {
		t.Fatalf("expected locale %q, got %q", DefaultLocale, result.Issues[0].Locale)
	}
}
