package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc/types"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func TestReadFileIfExists_FileExists(t *testing.T) {
	// Create a temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	got := readFileIfExists(path)
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

func TestReadFileIfExists_FileDoesNotExist(t *testing.T) {
	got := readFileIfExists("/nonexistent/path/file.txt")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestReadFileIfExists_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("  trimmed  \n\n"), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	got := readFileIfExists(path)
	if got != "trimmed" {
		t.Errorf("expected 'trimmed', got %q", got)
	}
}

func TestWriteAndCount_EmptyContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	count := writeAndCount(path, "")
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	// File should not exist
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file to not exist")
	}
}

func TestWriteAndCount_WritesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	count := writeAndCount(path, "content")
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != "content\n" {
		t.Errorf("expected 'content\\n', got %q", string(data))
	}
}

func TestCountNonEmptyFields_AllEmpty(t *testing.T) {
	loc := FastlaneLocalization{Locale: "en-US"}
	count := countNonEmptyFields(loc)
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestCountNonEmptyFields_AllFilled(t *testing.T) {
	loc := FastlaneLocalization{
		Locale:          "en-US",
		Description:     "desc",
		Keywords:        "key1, key2",
		WhatsNew:        "new stuff",
		PromotionalText: "promo",
		SupportURL:      "https://support.example.com",
		MarketingURL:    "https://marketing.example.com",
	}
	count := countNonEmptyFields(loc)
	if count != 6 {
		t.Errorf("expected 6, got %d", count)
	}
}

func TestCountNonEmptyFields_Partial(t *testing.T) {
	loc := FastlaneLocalization{
		Locale:      "en-US",
		Description: "desc",
		Keywords:    "key1, key2",
		WhatsNew:    "new stuff",
	}
	count := countNonEmptyFields(loc)
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestReadFastlaneMetadata_ValidStructure(t *testing.T) {
	// Create a temp fastlane structure
	dir := t.TempDir()

	// Create en-US locale
	enDir := filepath.Join(dir, "en-US")
	if err := os.MkdirAll(enDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(enDir, "description.txt"), []byte("English description"), 0o644); err != nil {
		t.Fatalf("failed to write description: %v", err)
	}
	if err := os.WriteFile(filepath.Join(enDir, "keywords.txt"), []byte("app, mobile, utility"), 0o644); err != nil {
		t.Fatalf("failed to write keywords: %v", err)
	}
	if err := os.WriteFile(filepath.Join(enDir, "release_notes.txt"), []byte("Bug fixes"), 0o644); err != nil {
		t.Fatalf("failed to write release notes: %v", err)
	}

	// Create de-DE locale
	deDir := filepath.Join(dir, "de-DE")
	if err := os.MkdirAll(deDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deDir, "description.txt"), []byte("German description"), 0o644); err != nil {
		t.Fatalf("failed to write localized description: %v", err)
	}

	locs, err := readFastlaneMetadata(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(locs) != 2 {
		t.Errorf("expected 2 localizations, got %d", len(locs))
	}

	// Check content (order may vary)
	for _, loc := range locs {
		switch loc.Locale {
		case "en-US":
			if loc.Description != "English description" {
				t.Errorf("expected 'English description', got %q", loc.Description)
			}
			if loc.Keywords != "app, mobile, utility" {
				t.Errorf("expected 'app, mobile, utility', got %q", loc.Keywords)
			}
			if loc.WhatsNew != "Bug fixes" {
				t.Errorf("expected 'Bug fixes', got %q", loc.WhatsNew)
			}
		case "de-DE":
			if loc.Description != "German description" {
				t.Errorf("expected 'German description', got %q", loc.Description)
			}
		default:
			t.Errorf("unexpected locale: %s", loc.Locale)
		}
	}
}

func TestReadFastlaneMetadata_SkipsSpecialDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create review_information directory (should be skipped)
	reviewDir := filepath.Join(dir, "review_information")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "description.txt"), []byte("Should be skipped"), 0o644); err != nil {
		t.Fatalf("failed to write review description: %v", err)
	}

	// Create default directory (should be skipped)
	defaultDir := filepath.Join(dir, "default")
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Create en-US locale (should be included)
	enDir := filepath.Join(dir, "en-US")
	if err := os.MkdirAll(enDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(enDir, "description.txt"), []byte("English description"), 0o644); err != nil {
		t.Fatalf("failed to write locale description: %v", err)
	}

	locs, err := readFastlaneMetadata(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(locs) != 1 {
		t.Errorf("expected 1 localization (special dirs skipped), got %d", len(locs))
	}

	if locs[0].Locale != "en-US" {
		t.Errorf("expected locale 'en-US', got %q", locs[0].Locale)
	}
}

func TestReadFastlaneMetadata_SkipsFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a file (should be skipped)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("This is a file"), 0o644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	// Create en-US locale
	enDir := filepath.Join(dir, "en-US")
	if err := os.MkdirAll(enDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(enDir, "description.txt"), []byte("English description"), 0o644); err != nil {
		t.Fatalf("failed to write locale description: %v", err)
	}

	locs, err := readFastlaneMetadata(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(locs) != 1 {
		t.Errorf("expected 1 localization (file skipped), got %d", len(locs))
	}
}

func TestValidateVersionLocalization_UsesSharedLimits(t *testing.T) {
	loc := FastlaneLocalization{
		Locale:      "en-US",
		Description: strings.Repeat("a", validation.LimitDescription+1),
	}

	issues := validateVersionLocalization(loc)
	if len(issues) == 0 {
		t.Fatalf("expected issues, got none")
	}

	found := false
	for _, issue := range issues {
		if issue.Field == "description" {
			found = true
			if issue.Limit != validation.LimitDescription {
				t.Fatalf("expected limit %d, got %d", validation.LimitDescription, issue.Limit)
			}
		}
	}
	if !found {
		t.Fatalf("expected description issue")
	}
}

func TestPrintMigrateOutput_RejectsPrettyForTable(t *testing.T) {
	err := printMigrateOutput(&MigrateImportResult{}, "table", true)
	if err == nil || !strings.Contains(err.Error(), "--pretty is only valid with JSON output") {
		t.Fatalf("expected pretty validation error, got %v", err)
	}
}

func TestPrintMigrateOutput_UnsupportedFormat(t *testing.T) {
	err := printMigrateOutput(&MigrateImportResult{}, "yaml", false)
	if err == nil || !strings.Contains(err.Error(), "unsupported format: yaml") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func TestPrintMigrateOutput_UnknownTypeTableUnsupported(t *testing.T) {
	err := printMigrateOutput(struct{}{}, "table", false)
	if err == nil || !strings.Contains(err.Error(), "unsupported format: table") {
		t.Fatalf("expected unsupported format for table, got %v", err)
	}
}

func TestPrintMigrateOutput_UnknownTypeJSONFallsBack(t *testing.T) {
	if err := printMigrateOutput(struct {
		OK bool `json:"ok"`
	}{OK: true}, "json", false); err != nil {
		t.Fatalf("expected JSON fallback to succeed, got %v", err)
	}
}

func TestValidateAppInfoLocalization_UsesSharedLimits(t *testing.T) {
	loc := AppInfoFastlaneLocalization{
		Locale: "en-US",
		Name:   strings.Repeat("n", validation.LimitName+1),
	}

	issues := validateAppInfoLocalization(loc)
	if len(issues) == 0 {
		t.Fatalf("expected issues, got none")
	}

	found := false
	for _, issue := range issues {
		if issue.Field == "name" {
			found = true
			if issue.Limit != validation.LimitName {
				t.Fatalf("expected limit %d, got %d", validation.LimitName, issue.Limit)
			}
		}
	}
	if !found {
		t.Fatalf("expected name issue")
	}
}

func TestSelectBestAppInfoID_PrefersPrepareForSubmission(t *testing.T) {
	appInfos := &asc.AppInfosResponse{
		Data: []types.Resource[asc.AppInfoAttributes]{
			{
				ID: "ready",
				Attributes: asc.AppInfoAttributes{
					"state":         "READY_FOR_DISTRIBUTION",
					"appStoreState": "READY_FOR_SALE",
				},
			},
			{
				ID: "prep",
				Attributes: asc.AppInfoAttributes{
					"state":         "PREPARE_FOR_SUBMISSION",
					"appStoreState": "PREPARE_FOR_SUBMISSION",
				},
			},
		},
	}

	if got := shared.SelectBestAppInfoID(appInfos); got != "prep" {
		t.Fatalf("expected appInfoID %q, got %q", "prep", got)
	}
}

func TestSelectBestAppInfoID_FallsBackToNonReadyForSale(t *testing.T) {
	appInfos := &asc.AppInfosResponse{
		Data: []types.Resource[asc.AppInfoAttributes]{
			{
				ID: "ready",
				Attributes: asc.AppInfoAttributes{
					"appStoreState": "READY_FOR_SALE",
				},
			},
			{
				ID: "not-ready",
				Attributes: asc.AppInfoAttributes{
					"appStoreState": "DEVELOPER_REMOVED_FROM_SALE",
				},
			},
		},
	}

	if got := shared.SelectBestAppInfoID(appInfos); got != "not-ready" {
		t.Fatalf("expected appInfoID %q, got %q", "not-ready", got)
	}
}

func TestSelectBestAppInfoID_EmptyInput(t *testing.T) {
	if got := shared.SelectBestAppInfoID(nil); got != "" {
		t.Fatalf("expected empty appInfoID for nil input, got %q", got)
	}

	if got := shared.SelectBestAppInfoID(&asc.AppInfosResponse{}); got != "" {
		t.Fatalf("expected empty appInfoID for empty input, got %q", got)
	}
}

func TestSelectBestAppInfoID_UsesStateWhenAppStoreStateMissing(t *testing.T) {
	appInfos := &asc.AppInfosResponse{
		Data: []types.Resource[asc.AppInfoAttributes]{
			{
				ID: "live",
				Attributes: asc.AppInfoAttributes{
					"state": "READY_FOR_DISTRIBUTION",
				},
			},
			{
				ID: "editable",
				Attributes: asc.AppInfoAttributes{
					"state": "IN_REVIEW",
				},
			},
		},
	}

	if got := shared.SelectBestAppInfoID(appInfos); got != "editable" {
		t.Fatalf("expected appInfoID %q, got %q", "editable", got)
	}
}

func TestReadFastlaneMetadata_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	locs, err := readFastlaneMetadata(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(locs) != 0 {
		t.Errorf("expected 0 localizations, got %d", len(locs))
	}
}

func TestValidateVersionLocalization_NoIssues(t *testing.T) {
	loc := FastlaneLocalization{
		Locale:          "en-US",
		Description:     "A valid description",
		Keywords:        "app, utility",
		WhatsNew:        "Bug fixes",
		PromotionalText: "Download now!",
	}

	issues := validateVersionLocalization(loc)
	// Should only have no errors (might have empty field warnings filtered)
	for _, issue := range issues {
		if issue.Severity == "error" {
			t.Errorf("unexpected error: %s - %s", issue.Field, issue.Message)
		}
	}
}

func TestValidateVersionLocalization_DescriptionTooLong(t *testing.T) {
	// Create a description that exceeds 4000 characters
	longDesc := make([]byte, 4001)
	for i := range longDesc {
		longDesc[i] = 'a'
	}

	loc := FastlaneLocalization{
		Locale:      "en-US",
		Description: string(longDesc),
	}

	issues := validateVersionLocalization(loc)
	foundError := false
	for _, issue := range issues {
		if issue.Field == "description" && issue.Severity == "error" {
			foundError = true
			if issue.Length != 4001 {
				t.Errorf("expected length 4001, got %d", issue.Length)
			}
			if issue.Limit != 4000 {
				t.Errorf("expected limit 4000, got %d", issue.Limit)
			}
		}
	}
	if !foundError {
		t.Error("expected error for description exceeding limit")
	}
}

func TestValidateVersionLocalization_KeywordsTooLong(t *testing.T) {
	// Create keywords that exceed 100 characters
	longKeywords := make([]byte, 101)
	for i := range longKeywords {
		longKeywords[i] = 'k'
	}

	loc := FastlaneLocalization{
		Locale:      "en-US",
		Description: "Valid description",
		Keywords:    string(longKeywords),
	}

	issues := validateVersionLocalization(loc)
	foundError := false
	for _, issue := range issues {
		if issue.Field == "keywords" && issue.Severity == "error" {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected error for keywords exceeding limit")
	}
}

func TestValidateVersionLocalization_PromotionalTextTooLong(t *testing.T) {
	// Create promotional text that exceeds 170 characters
	longPromo := make([]byte, 171)
	for i := range longPromo {
		longPromo[i] = 'p'
	}

	loc := FastlaneLocalization{
		Locale:          "en-US",
		Description:     "Valid description",
		PromotionalText: string(longPromo),
	}

	issues := validateVersionLocalization(loc)
	foundError := false
	for _, issue := range issues {
		if issue.Field == "promotionalText" && issue.Severity == "error" {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected error for promotional text exceeding limit")
	}
}

func TestValidateVersionLocalization_EmptyDescriptionWarning(t *testing.T) {
	loc := FastlaneLocalization{
		Locale:   "en-US",
		Keywords: "app, utility",
	}

	issues := validateVersionLocalization(loc)
	foundWarning := false
	for _, issue := range issues {
		if issue.Field == "description" && issue.Severity == "warning" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("expected warning for empty description")
	}
}

func TestValidateAppInfoLocalization_NoIssues(t *testing.T) {
	loc := AppInfoFastlaneLocalization{
		Locale:   "en-US",
		Name:     "My App",
		Subtitle: "A great app",
	}

	issues := validateAppInfoLocalization(loc)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

func TestValidateAppInfoLocalization_NameTooLong(t *testing.T) {
	loc := AppInfoFastlaneLocalization{
		Locale: "en-US",
		Name:   "This name is way too long for the App Store limit of 30 characters",
	}

	issues := validateAppInfoLocalization(loc)
	foundError := false
	for _, issue := range issues {
		if issue.Field == "name" && issue.Severity == "error" {
			foundError = true
			if issue.Limit != 30 {
				t.Errorf("expected limit 30, got %d", issue.Limit)
			}
		}
	}
	if !foundError {
		t.Error("expected error for name exceeding limit")
	}
}

func TestValidateAppInfoLocalization_SubtitleTooLong(t *testing.T) {
	loc := AppInfoFastlaneLocalization{
		Locale:   "en-US",
		Name:     "My App",
		Subtitle: "This subtitle is way too long for the App Store limit",
	}

	issues := validateAppInfoLocalization(loc)
	foundError := false
	for _, issue := range issues {
		if issue.Field == "subtitle" && issue.Severity == "error" {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected error for subtitle exceeding limit")
	}
}

func TestValidateVersionLocalization_UsesRuneCount(t *testing.T) {
	loc := FastlaneLocalization{
		Locale:      "en-US",
		Description: strings.Repeat("😀", 2000), // 2000 runes, exceeds byte limit but not rune limit
	}

	issues := validateVersionLocalization(loc)
	for _, issue := range issues {
		if issue.Field == "description" && issue.Severity == "error" {
			t.Fatalf("expected no description length error for 2000 runes, got %+v", issue)
		}
	}
}

func TestValidateAppInfoLocalization_UsesRuneCount(t *testing.T) {
	loc := AppInfoFastlaneLocalization{
		Locale:   "en-US",
		Name:     strings.Repeat("😀", 30),
		Subtitle: strings.Repeat("😀", 30),
	}

	issues := validateAppInfoLocalization(loc)
	for _, issue := range issues {
		if issue.Severity == "error" {
			t.Fatalf("expected no rune-count errors at exact limits, got %+v", issue)
		}
	}
}
