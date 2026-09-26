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

func TestValidateDirAcceptsArabicKeywordsWithinCharacterLimit(t *testing.T) {
	dir := t.TempDir()
	version := "1.2.3"

	if err := os.MkdirAll(filepath.Join(dir, versionDirName, version), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}

	body := `{"description":"وصف تطبيق عربي كامل","keywords":"تغريدات,ردود,اعجابات,فلترة,بحث,ارشفة,ازالة,سجل,ريتويت,لايكات,منشن,خصوصية,منشورات,قديمة,حساب"}`
	if err := os.WriteFile(filepath.Join(dir, versionDirName, version, "ar-SA.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write Arabic localization: %v", err)
	}

	result, err := validateDir(dir)
	if err != nil {
		t.Fatalf("validateDir() error: %v", err)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %+v", result.Issues)
	}
	if !result.Valid {
		t.Fatalf("expected valid metadata result, got %+v", result)
	}
}

func TestValidateDirAcceptsExplicitClearOnlyLocalizations(t *testing.T) {
	dir := t.TempDir()
	version := "1.2.3"

	if err := os.MkdirAll(filepath.Join(dir, appInfoDirName), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, versionDirName, version), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, appInfoDirName, "en-US.json"), []byte(`{"subtitle":null}`), 0o644); err != nil {
		t.Fatalf("write app-info clear-only file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, versionDirName, version, "en-US.json"), []byte(`{"promotionalText":null}`), 0o644); err != nil {
		t.Fatalf("write version clear-only file: %v", err)
	}

	result, err := validateDir(dir)
	if err != nil {
		t.Fatalf("validateDir() error: %v", err)
	}
	if !result.Valid || len(result.Issues) != 0 {
		t.Fatalf("expected explicit clear-only files to validate offline, got %+v", result)
	}
}

func TestValidateDirAcceptsExplicitMixedClearAndSetLocalizations(t *testing.T) {
	dir := t.TempDir()
	version := "1.2.3"

	if err := os.MkdirAll(filepath.Join(dir, appInfoDirName), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, versionDirName, version), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, appInfoDirName, "en-US.json"), []byte(`{"privacyPolicyUrl":"https://example.com/privacy","subtitle":null}`), 0o644); err != nil {
		t.Fatalf("write app-info mixed patch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, versionDirName, version, "en-US.json"), []byte(`{"description":"Updated description","promotionalText":null}`), 0o644); err != nil {
		t.Fatalf("write version mixed patch: %v", err)
	}

	result, err := validateDir(dir)
	if err != nil {
		t.Fatalf("validateDir() error: %v", err)
	}
	if !result.Valid || len(result.Issues) != 0 {
		t.Fatalf("expected mixed clear-and-set patches to validate offline, got %+v", result)
	}
}

func TestValidateDirRejectsClearFieldsInDefaultFiles(t *testing.T) {
	dir := t.TempDir()
	version := "1.2.3"

	if err := os.MkdirAll(filepath.Join(dir, appInfoDirName), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, versionDirName, version), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, appInfoDirName, "default.json"), []byte(`{"name":"Default Name","subtitle":null}`), 0o644); err != nil {
		t.Fatalf("write app-info default file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, versionDirName, version, "default.json"), []byte(`{"description":"Default description","promotionalText":null}`), 0o644); err != nil {
		t.Fatalf("write version default file: %v", err)
	}

	result, err := validateDir(dir)
	if err != nil {
		t.Fatalf("validateDir() error: %v", err)
	}
	if result.Valid || result.ErrorCount != 2 {
		t.Fatalf("expected two default clear errors, got %+v", result)
	}
	for _, issue := range result.Issues {
		if issue.Severity == issueSeverityError && !strings.Contains(issue.Message, "cannot be cleared in default.json") {
			t.Fatalf("unexpected default clear issue: %+v", issue)
		}
	}
}

func longHTTPSURL(length int) string {
	const prefix = "https://example.com/"
	return prefix + strings.Repeat("a", length-len(prefix))
}

func TestURLLengthIssuesBoundaries(t *testing.T) {
	noVersionIssues := versionLengthIssues("file", "1.2.3", "en-US", VersionLocalization{
		MarketingURL: longHTTPSURL(validation.LimitMarketingURL),
		SupportURL:   longHTTPSURL(validation.LimitSupportURL),
	})
	if len(noVersionIssues) != 0 {
		t.Fatalf("expected no issues at URL limits, got %+v", noVersionIssues)
	}

	versionIssues := versionLengthIssues("file", "1.2.3", "en-US", VersionLocalization{
		MarketingURL: longHTTPSURL(validation.LimitMarketingURL + 1),
		SupportURL:   longHTTPSURL(validation.LimitSupportURL + 1),
	})
	if len(versionIssues) != 2 {
		t.Fatalf("expected 2 version URL issues, got %+v", versionIssues)
	}
	for _, issue := range versionIssues {
		if issue.Severity != issueSeverityWarning {
			t.Fatalf("expected warning severity for URL length, got %+v", issue)
		}
		if issue.Limit != 255 {
			t.Fatalf("expected limit 255, got %+v", issue)
		}
	}

	noAppInfoIssues := appInfoLengthIssues("file", "en-US", AppInfoLocalization{
		PrivacyPolicyURL:  longHTTPSURL(validation.LimitPrivacyPolicyURL),
		PrivacyChoicesURL: longHTTPSURL(validation.LimitPrivacyChoicesURL),
	})
	if len(noAppInfoIssues) != 0 {
		t.Fatalf("expected no issues at app-info URL limits, got %+v", noAppInfoIssues)
	}

	appInfoIssues := appInfoLengthIssues("file", "en-US", AppInfoLocalization{
		PrivacyPolicyURL:  longHTTPSURL(validation.LimitPrivacyPolicyURL + 1),
		PrivacyChoicesURL: longHTTPSURL(validation.LimitPrivacyChoicesURL + 1),
	})
	if len(appInfoIssues) != 2 {
		t.Fatalf("expected 2 app-info URL issues, got %+v", appInfoIssues)
	}
	for _, issue := range appInfoIssues {
		if issue.Severity != issueSeverityWarning {
			t.Fatalf("expected warning severity for URL length, got %+v", issue)
		}
	}
}

func TestValidateDirWarnsForInvalidURLSyntax(t *testing.T) {
	dir := t.TempDir()
	version := "1.2.3"

	if err := os.MkdirAll(filepath.Join(dir, appInfoDirName), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, versionDirName, version), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	appInfoBody := `{"name":"App Name","privacyPolicyUrl":"example.com/privacy","privacyChoicesUrl":"example.com/choices"}`
	if err := os.WriteFile(filepath.Join(dir, appInfoDirName, "en-US.json"), []byte(appInfoBody), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	versionBody := `{"description":"English description","supportUrl":"example.com","marketingUrl":"www.example.com/app"}`
	if err := os.WriteFile(filepath.Join(dir, versionDirName, version, "en-US.json"), []byte(versionBody), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	result, err := validateDir(dir)
	if err != nil {
		t.Fatalf("validateDir() error: %v", err)
	}
	if result.ErrorCount != 0 {
		t.Fatalf("expected URL syntax issues to stay warnings, got %+v", result.Issues)
	}
	if !result.Valid {
		t.Fatalf("expected valid=true for warning-only report, got %+v", result)
	}

	wantFields := map[string]bool{
		"supportUrl":        false,
		"marketingUrl":      false,
		"privacyPolicyUrl":  false,
		"privacyChoicesUrl": false,
	}
	for _, issue := range result.Issues {
		if _, ok := wantFields[issue.Field]; !ok {
			continue
		}
		if issue.Severity != issueSeverityWarning {
			t.Fatalf("expected warning severity for %s, got %+v", issue.Field, issue)
		}
		if !strings.Contains(issue.Message, "not a valid HTTP/HTTPS URL") {
			t.Fatalf("expected URL syntax message for %s, got %+v", issue.Field, issue)
		}
		wantFields[issue.Field] = true
	}
	for field, found := range wantFields {
		if !found {
			t.Fatalf("expected URL syntax warning for %s, got %+v", field, result.Issues)
		}
	}
}

func TestValidateDirWarnsForImplausiblyShortMetadata(t *testing.T) {
	dir := t.TempDir()
	version := "1.2.3"

	if err := os.MkdirAll(filepath.Join(dir, appInfoDirName), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, versionDirName, version), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, appInfoDirName, "en-US.json"), []byte(`{"name":"X"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, versionDirName, version, "en-US.json"), []byte(`{"description":"TBD"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	result, err := validateDir(dir)
	if err != nil {
		t.Fatalf("validateDir() error: %v", err)
	}
	if result.ErrorCount != 0 || !result.Valid {
		t.Fatalf("expected short values to stay warnings, got %+v", result)
	}
	if len(result.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %+v", result.Issues)
	}

	wantFields := map[string]bool{"name": false, "description": false}
	for _, issue := range result.Issues {
		if _, ok := wantFields[issue.Field]; !ok {
			t.Fatalf("unexpected issue: %+v", issue)
		}
		if issue.Severity != issueSeverityWarning {
			t.Fatalf("expected warning severity, got %+v", issue)
		}
		if !strings.Contains(issue.Message, "shorter than") {
			t.Fatalf("expected minimum-length message, got %+v", issue)
		}
		wantFields[issue.Field] = true
	}
	for field, found := range wantFields {
		if !found {
			t.Fatalf("expected minimum-length warning for %s, got %+v", field, result.Issues)
		}
	}
}

func TestValidateDirAcceptsValidURLSyntax(t *testing.T) {
	dir := t.TempDir()
	version := "1.2.3"

	if err := os.MkdirAll(filepath.Join(dir, appInfoDirName), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, versionDirName, version), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	appInfoBody := `{"name":"App Name","privacyPolicyUrl":"https://example.com/privacy","privacyChoicesUrl":"http://example.com/choices"}`
	if err := os.WriteFile(filepath.Join(dir, appInfoDirName, "en-US.json"), []byte(appInfoBody), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	versionBody := `{"description":"English description","supportUrl":"https://example.com","marketingUrl":"https://example.com/app"}`
	if err := os.WriteFile(filepath.Join(dir, versionDirName, version, "en-US.json"), []byte(versionBody), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	result, err := validateDir(dir)
	if err != nil {
		t.Fatalf("validateDir() error: %v", err)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues for valid URLs, got %+v", result.Issues)
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

	result, err := validateDir(dir)
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

	result, err := validateDir(dir)
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

	result, err := validateDir(dir)
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
