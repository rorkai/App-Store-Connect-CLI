package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestMetadataValidateRequiresDir(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if errors.Is(runErr, flag.ErrHelp) || !shared.IsReportedUsageError(runErr) {
		t.Fatalf("expected reported usage error without ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "Error: --dir is required\n" {
		t.Fatalf("stderr = %q, want concise missing-dir error", stderr)
	}
	diagnostic, ok := shared.DiagnosticFromError(runErr)
	if !ok || diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--dir" {
		t.Fatalf("diagnostic = %+v, found=%t", diagnostic, ok)
	}
}

func TestMetadataValidateRejectsUnknownSchemaKeys(t *testing.T) {
	dir := t.TempDir()
	appInfoDir := filepath.Join(dir, "app-info")
	if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appInfoDir, "en-US.json"), []byte(`{"name":"App","unknown":"x"}`), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "unknown field") {
		t.Fatalf("expected unknown-field schema error, got %q", stderr)
	}
}

func TestMetadataValidateReportsMissingRequiredFields(t *testing.T) {
	dir := t.TempDir()
	appInfoDir := filepath.Join(dir, "app-info")
	if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appInfoDir, "en-US.json"), []byte(`{"subtitle":"Only subtitle"}`), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected validation error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Valid      bool `json:"valid"`
		ErrorCount int  `json:"errorCount"`
		Issues     []struct {
			Field string `json:"field"`
		} `json:"issues"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.Valid {
		t.Fatalf("expected invalid report, got valid=%v", payload.Valid)
	}
	if payload.ErrorCount == 0 {
		t.Fatalf("expected errors in report, got %+v", payload)
	}
	foundNameIssue := false
	for _, issue := range payload.Issues {
		if issue.Field == "name" {
			foundNameIssue = true
			break
		}
	}
	if !foundNameIssue {
		t.Fatalf("expected name issue, got %+v", payload.Issues)
	}
}

func TestMetadataValidateRejectsEmptyTemplateKeys(t *testing.T) {
	dir := t.TempDir()
	appInfoDir := filepath.Join(dir, "app-info")
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appInfoDir, "en-US.json"), []byte(`{"name":"App","subtitle":""}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"Description","keywords":""}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected validation error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Valid      bool `json:"valid"`
		ErrorCount int  `json:"errorCount"`
		Issues     []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"issues"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.Valid || payload.ErrorCount == 0 {
		t.Fatalf("expected invalid report, got %+v", payload)
	}
	foundSubtitleIssue := false
	foundKeywordsIssue := false
	for _, issue := range payload.Issues {
		if issue.Field == "subtitle" && strings.Contains(issue.Message, `field "subtitle" cannot be empty`) {
			foundSubtitleIssue = true
		}
		if issue.Field == "keywords" && strings.Contains(issue.Message, `field "keywords" cannot be empty`) {
			foundKeywordsIssue = true
		}
	}
	if !foundSubtitleIssue {
		t.Fatalf("expected empty subtitle issue, got %+v", payload.Issues)
	}
	if !foundKeywordsIssue {
		t.Fatalf("expected empty keywords issue, got %+v", payload.Issues)
	}
}

func TestMetadataValidateBlankOnlyTemplateKeysFallBackToMissingMetadata(t *testing.T) {
	dir := t.TempDir()
	appInfoDir := filepath.Join(dir, "app-info")
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appInfoDir, "en-US.json"), []byte(`{"name":"","subtitle":""}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"","keywords":""}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected validation error")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Issues []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"issues"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}

	foundAppInfoMissing := false
	foundVersionMissing := false
	for _, issue := range payload.Issues {
		if strings.Contains(issue.Message, "cannot be empty") {
			t.Fatalf("did not expect field-level empty issue for blank-only template: %+v", payload.Issues)
		}
		if issue.Field == "metadata" && issue.Message == "at least one app-info field is required" {
			foundAppInfoMissing = true
		}
		if issue.Field == "metadata" && issue.Message == "at least one version metadata field is required" {
			foundVersionMissing = true
		}
	}
	if !foundAppInfoMissing || !foundVersionMissing {
		t.Fatalf("expected missing metadata issues, got %+v", payload.Issues)
	}
}

func TestMetadataValidatePassesForValidFiles(t *testing.T) {
	dir := t.TempDir()
	appInfoDir := filepath.Join(dir, "app-info")
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(appInfoDir, "en-US.json"), []byte(`{"name":"App Name","subtitle":"Great app"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"English description","keywords":"one,two"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Valid      bool `json:"valid"`
		ErrorCount int  `json:"errorCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Valid {
		t.Fatalf("expected valid report, got %+v", payload)
	}
	if payload.ErrorCount != 0 {
		t.Fatalf("expected zero errors, got %+v", payload)
	}
}

func TestMetadataValidateWarningsKeepReportValid(t *testing.T) {
	tests := []struct {
		name         string
		appInfo      string
		version      string
		wantWarnings map[string]string
	}{
		{
			name:         "invalid URL syntax",
			appInfo:      `{"name":"App Name","privacyPolicyUrl":"example.com/privacy"}`,
			version:      `{"description":"English description","supportUrl":"example.com"}`,
			wantWarnings: map[string]string{"supportUrl": "not a valid HTTP/HTTPS URL", "privacyPolicyUrl": "not a valid HTTP/HTTPS URL"},
		},
		{
			name:         "implausibly short metadata",
			appInfo:      `{"name":"X"}`,
			version:      `{"description":"TBD"}`,
			wantWarnings: map[string]string{"name": "shorter than", "description": "shorter than"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			appInfoDir := filepath.Join(dir, "app-info")
			versionDir := filepath.Join(dir, "version", "1.2.3")
			for path, body := range map[string]string{
				filepath.Join(appInfoDir, "en-US.json"): test.appInfo,
				filepath.Join(versionDir, "en-US.json"): test.version,
			} {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatalf("write fixture: %v", err)
				}
			}

			stdout, stderr, runErr := runRootCommand(t, []string{"metadata", "validate", "--dir", dir})
			if runErr != nil {
				t.Fatalf("expected warning-only validation, got %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var payload struct {
				Valid        bool `json:"valid"`
				ErrorCount   int  `json:"errorCount"`
				WarningCount int  `json:"warningCount"`
				Issues       []struct {
					Field    string `json:"field"`
					Severity string `json:"severity"`
					Message  string `json:"message"`
				} `json:"issues"`
			}
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
			}
			if !payload.Valid || payload.ErrorCount != 0 || payload.WarningCount != len(test.wantWarnings) {
				t.Fatalf("expected a valid report with %d warnings, got %+v", len(test.wantWarnings), payload)
			}
			found := map[string]bool{}
			for _, issue := range payload.Issues {
				want, ok := test.wantWarnings[issue.Field]
				if !ok || issue.Severity != "warning" || !strings.Contains(issue.Message, want) {
					t.Fatalf("unexpected issue %+v, want warnings %v", issue, test.wantWarnings)
				}
				found[issue.Field] = true
			}
			if len(found) != len(test.wantWarnings) {
				t.Fatalf("warnings = %+v, want one per field in %v", payload.Issues, test.wantWarnings)
			}
		})
	}
}

func TestMetadataValidateWarnsForOverlongURLs(t *testing.T) {
	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	supportURL := "https://example.com/" + strings.Repeat("a", 236)
	if len(supportURL) != 256 {
		t.Fatalf("expected 256-character support URL, got %d", len(supportURL))
	}
	body := `{"description":"English description","supportUrl":"` + supportURL + `"}`
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected warning-only validation, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Valid        bool `json:"valid"`
		ErrorCount   int  `json:"errorCount"`
		WarningCount int  `json:"warningCount"`
		Issues       []struct {
			Field    string `json:"field"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
			Length   int    `json:"length"`
			Limit    int    `json:"limit"`
		} `json:"issues"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Valid || payload.ErrorCount != 0 {
		t.Fatalf("expected valid warning-only report, got %+v", payload)
	}
	if payload.WarningCount != 1 {
		t.Fatalf("expected exactly one URL length warning, got %+v", payload)
	}

	issue := payload.Issues[0]
	if issue.Field != "supportUrl" || issue.Severity != "warning" {
		t.Fatalf("expected supportUrl warning, got %+v", issue)
	}
	if issue.Length != 256 || issue.Limit != 255 {
		t.Fatalf("expected length 256 and limit 255, got %+v", issue)
	}
	if !strings.Contains(issue.Message, "support URL exceeds 255 characters") {
		t.Fatalf("expected URL length message, got %+v", issue)
	}
}

func TestMetadataValidateAcceptsDefaultLocaleFiles(t *testing.T) {
	dir := t.TempDir()
	appInfoDir := filepath.Join(dir, "app-info")
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(appInfoDir, "default.json"), []byte(`{"name":"Default App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "default.json"), []byte(`{"description":"Default description"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Valid      bool `json:"valid"`
		ErrorCount int  `json:"errorCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Valid || payload.ErrorCount != 0 {
		t.Fatalf("expected valid report, got %+v", payload)
	}
}

func TestMetadataValidateRejectsDuplicateCanonicalLocaleFiles(t *testing.T) {
	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"One"}`), 0o644); err != nil {
		t.Fatalf("write en-US file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en_US.json"), []byte(`{"description":"Two"}`), 0o644); err != nil {
		t.Fatalf("write en_US file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{`duplicate canonical locale "en-US"`, `"en-US.json"`, `"en_US.json"`} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}

func TestMetadataValidateReportsErrorForEmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir, "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected validation error for empty directory")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "no metadata .json files found") {
		t.Fatalf("expected empty-directory issue in output, got %q", stdout)
	}
}

func TestMetadataValidateSupportsTableAndMarkdownOutput(t *testing.T) {
	tests := []struct {
		name       string
		outputFlag string
		wantText   string
	}{
		{name: "table", outputFlag: "table", wantText: "Files Scanned: 1"},
		{name: "markdown", outputFlag: "markdown", wantText: "**Files Scanned:** 1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			appInfoDir := filepath.Join(dir, "app-info")
			if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
				t.Fatalf("mkdir app-info: %v", err)
			}
			if err := os.WriteFile(filepath.Join(appInfoDir, "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
				t.Fatalf("write app-info file: %v", err)
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"metadata", "validate", "--dir", dir, "--output", test.outputFlag}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, test.wantText) {
				t.Fatalf("expected %q in output, got %q", test.wantText, stdout)
			}
		})
	}
}

func writeMetadataValidateTermsFiles(t *testing.T, description string) string {
	t.Helper()

	dir := t.TempDir()
	appInfoDir := filepath.Join(dir, "app-info")
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appInfoDir, "en-US.json"), []byte(`{"name":"App Name","subtitle":"Great app"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"`+description+`","keywords":"one,two"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	return dir
}

func TestMetadataValidateWarnsWhenSubscriptionAppDescriptionMissingTermsLink(t *testing.T) {
	dir := writeMetadataValidateTermsFiles(t, "Subscription description without a legal link")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir, "--subscription-app"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected warning-only validation, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Valid        bool `json:"valid"`
		ErrorCount   int  `json:"errorCount"`
		WarningCount int  `json:"warningCount"`
		Issues       []struct {
			Field    string `json:"field"`
			Severity string `json:"severity"`
		} `json:"issues"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Valid || payload.ErrorCount != 0 {
		t.Fatalf("expected valid warning-only report, got %+v", payload)
	}
	if payload.WarningCount == 0 {
		t.Fatalf("expected at least one warning, got %+v", payload)
	}

	found := false
	for _, issue := range payload.Issues {
		if issue.Field == "description" && issue.Severity == "warning" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected description warning, got %+v", payload.Issues)
	}
}

func TestMetadataValidateSkipsTermsWarningWithoutSubscriptionAppFlag(t *testing.T) {
	dir := writeMetadataValidateTermsFiles(t, "Subscription description without a legal link")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected validation to succeed without warning, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		WarningCount int `json:"warningCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.WarningCount != 0 {
		t.Fatalf("expected zero warnings without --subscription-app, got %+v", payload)
	}
}

func TestMetadataValidateAcceptsAppleStandardEULAURLWithSubscriptionApp(t *testing.T) {
	dir := writeMetadataValidateTermsFiles(t, "Terms of Use: https://www.apple.com/legal/internet-services/itunes/dev/stdeula/")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir, "--subscription-app"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected validation to succeed, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		WarningCount int `json:"warningCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.WarningCount != 0 {
		t.Fatalf("expected zero warnings, got %+v", payload)
	}
}

func TestMetadataValidateAcceptsKeywordAndURLWithSubscriptionApp(t *testing.T) {
	dir := writeMetadataValidateTermsFiles(t, "Read our Terms of Use: https://example.com/subscription-agreement")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir, "--subscription-app"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected validation to succeed, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		WarningCount int `json:"warningCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.WarningCount != 0 {
		t.Fatalf("expected zero warnings, got %+v", payload)
	}
}

func TestMetadataValidateRejectsUnrelatedURLWithSubscriptionApp(t *testing.T) {
	dir := writeMetadataValidateTermsFiles(t, "Learn more at https://example.com/about")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "validate", "--dir", dir, "--subscription-app"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected warning-only validation, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		WarningCount int `json:"warningCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.WarningCount == 0 {
		t.Fatalf("expected unrelated URL to keep failing the terms heuristic, got %+v", payload)
	}
}
