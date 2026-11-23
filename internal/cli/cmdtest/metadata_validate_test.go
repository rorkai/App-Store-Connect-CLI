package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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
	tests := []struct {
		name     string
		file     string
		body     string
		wantLine string
	}{
		{
			name:     "version field in an app-info file",
			file:     "app-info/en-US.json",
			body:     `{"name":"App","description":"An app"}`,
			wantLine: `unknown field "description"; it is a version localization field: move it to version/<version>/<locale>.json`,
		},
		{
			name:     "app-info field in a version file",
			file:     "version/1.0/en-US.json",
			body:     `{"description":"An app","subtitle":"Short"}`,
			wantLine: `unknown field "subtitle"; it is an app-info localization field: move it to app-info/<locale>.json`,
		},
		{
			name:     "copyright in a version file",
			file:     "version/1.0/en-US.json",
			body:     `{"description":"An app","copyright":"2026 Example"}`,
			wantLine: "unknown field \"copyright\"; copyright is not a localization field: remove it and set it with `asc versions update --version-id VERSION_ID --copyright \"TEXT\"`",
		},
		{
			name:     "locale key",
			file:     "app-info/en-US.json",
			body:     `{"name":"App","locale":"en-US"}`,
			wantLine: `unknown field "locale"; the file name sets the locale: remove this key`,
		},
		{
			name:     "unrelated key",
			file:     "app-info/en-US.json",
			body:     `{"name":"App","unknown":"x"}`,
			wantLine: `unknown field "unknown"; app-info localization fields: name, subtitle, privacyPolicyUrl, privacyChoicesUrl, privacyPolicyText`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, filepath.FromSlash(test.file))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(path, []byte(test.body), 0o644); err != nil {
				t.Fatalf("write file: %v", err)
			}

			stdout, stderr, runErr := runRootWithDeps(t, nil, "metadata", "validate", "--dir", dir)

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			firstLine, _, _ := strings.Cut(stderr, "\n")
			if want := "Error: invalid metadata schema in " + path + ": json: " + test.wantLine; firstLine != want {
				t.Fatalf("stderr first line = %q, want %q", firstLine, want)
			}
		})
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

func TestMetadataValidateSubscriptionTermsLink(t *testing.T) {
	tests := []struct {
		name            string
		description     string
		subscriptionApp bool
		wantWarning     bool
	}{
		{name: "missing terms link warns", description: "Subscription description without a legal link", subscriptionApp: true, wantWarning: true},
		{name: "skipped without subscription app flag", description: "Subscription description without a legal link"},
		{name: "apple standard EULA URL", description: "Terms of Use: https://www.apple.com/legal/internet-services/itunes/dev/stdeula/", subscriptionApp: true},
		{name: "terms keyword with URL", description: "Read our Terms of Use: https://example.com/subscription-agreement", subscriptionApp: true},
		{name: "unrelated URL warns", description: "Learn more at https://example.com/about", subscriptionApp: true, wantWarning: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := writeMetadataValidateTermsFiles(t, test.description)
			args := []string{"metadata", "validate", "--dir", dir}
			if test.subscriptionApp {
				args = append(args, "--subscription-app")
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
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
				t.Fatalf("expected valid report without errors, got %+v", payload)
			}
			if !test.wantWarning {
				if payload.WarningCount != 0 {
					t.Fatalf("expected zero warnings, got %+v", payload)
				}
				return
			}
			found := false
			for _, issue := range payload.Issues {
				if issue.Field == "description" && issue.Severity == "warning" {
					found = true
					break
				}
			}
			if payload.WarningCount == 0 || !found {
				t.Fatalf("expected description warning, got %+v", payload)
			}
		})
	}
}

// TestMetadataValidateChecksTheVersionPushReads covers --version, which limits
// validation to the directory metadata push --version reads, and --app, which
// checks online that the app has that App Store version.
func TestMetadataValidateChecksTheVersionPushReads(t *testing.T) {
	type stub struct {
		matcher asctest.Matcher
		body    string
	}
	versionsQuery := []string{"filter[versionString]=1.1", "filter[platform]=IOS", "limit=10"}
	tests := []struct {
		name      string
		args      []string
		stubs     []stub
		wantErr   string
		wantFiles int
	}{
		{
			name:      "version skips the other version directories",
			args:      []string{"--version", "1.1"},
			wantFiles: 2,
		},
		{
			name:    "version without a directory",
			args:    []string{"--version", "2.0"},
			wantErr: `--version "2.0" has no directory`,
		},
		{
			name: "app has the version",
			args: []string{"--app", "100000001", "--version", "1.1", "--platform", "IOS"},
			stubs: []stub{{
				matcher: asctest.GET("/v1/apps/100000001/appStoreVersions", versionsQuery...),
				body:    `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.1","platform":"IOS"}}]}`,
			}},
			wantFiles: 2,
		},
		{
			name: "app lacks the version",
			args: []string{"--app", "100000001", "--version", "1.1", "--platform", "IOS"},
			stubs: []stub{
				{matcher: asctest.GET("/v1/apps/100000001/appStoreVersions", versionsQuery...), body: `{"data":[]}`},
				{matcher: asctest.GET("/v1/apps/100000001/appStoreVersions", "filter[platform]=IOS", "limit=200"), body: `{"data":[]}`},
				{matcher: asctest.GET("/v1/apps/100000001/appStoreVersions", "limit=200"), body: `{"data":[]}`},
			},
			wantErr: `metadata validate: app store version not found for version "1.1" and platform "IOS"`,
		},
		{
			name:    "platform without app",
			args:    []string{"--version", "1.1", "--platform", "IOS"},
			wantErr: "--platform selects the App Store version --app checks; pass --app with it",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range map[string]string{
				"app-info/en-US.json":    `{"name":"Example App"}`,
				"version/1.0/en-US.json": `{"description":"Example description","keywords":""}`,
				"version/1.1/en-US.json": `{"description":"Example description for this release"}`,
			} {
				path := filepath.Join(dir, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			reg := asctest.NewRegistry(t)
			for _, stub := range test.stubs {
				reg.Register(stub.matcher, asctest.JSON(http.StatusOK, stub.body))
			}

			args := append([]string{"metadata", "validate", "--dir", dir, "--output", "json"}, test.args...)
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)

			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("err = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("run error: %v (stderr %q)", err, stderr)
			}
			var result struct {
				FilesScanned int  `json:"filesScanned"`
				Valid        bool `json:"valid"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("unmarshal %q: %v", stdout, err)
			}
			if !result.Valid || result.FilesScanned != test.wantFiles {
				t.Fatalf("result = %+v, want valid with %d files", result, test.wantFiles)
			}
		})
	}
}
