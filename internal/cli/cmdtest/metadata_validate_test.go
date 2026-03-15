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

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --dir is required") {
		t.Fatalf("expected missing dir error, got %q", stderr)
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
