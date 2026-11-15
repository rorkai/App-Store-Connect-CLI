package metadata

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBuildPlanRowsNoChangesPlaceholder(t *testing.T) {
	rows := buildPlanRows(PushPlanResult{})
	if len(rows) != 1 {
		t.Fatalf("expected one placeholder row, got %d", len(rows))
	}
	if rows[0][0] != "none" || rows[0][6] != "no changes" {
		t.Fatalf("unexpected placeholder row: %v", rows[0])
	}
}

func TestSanitizePlanCellNormalizesAndTruncates(t *testing.T) {
	input := "line1\n" + strings.Repeat("x", 120)
	got := sanitizePlanCell(input)

	if strings.Contains(got, "\n") {
		t.Fatalf("expected no literal newlines, got %q", got)
	}
	if !strings.Contains(got, "\\n") {
		t.Fatalf("expected escaped newline marker, got %q", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected truncated output to end with ellipsis, got %q", got)
	}
}

func TestBuildAPICallSummarySorted(t *testing.T) {
	summary := buildAPICallSummary(
		scopeCallCounts{create: 1, update: 2},
		scopeCallCounts{create: 1, delete: 1},
	)
	if len(summary) != 4 {
		t.Fatalf("expected 4 summary items, got %d", len(summary))
	}

	for i := 1; i < len(summary); i++ {
		prev := summary[i-1]
		curr := summary[i]
		if prev.Scope > curr.Scope {
			t.Fatalf("summary not sorted by scope: %v", summary)
		}
		if prev.Scope == curr.Scope && prev.Operation > curr.Operation {
			t.Fatalf("summary not sorted by operation within scope: %v", summary)
		}
	}
}

func TestRenderHelpersProduceOutput(t *testing.T) {
	pushResult := PushPlanResult{
		AppID:    "app-1",
		Version:  "1.2.3",
		Dir:      "/tmp/meta",
		DryRun:   true,
		Applied:  true,
		Adds:     []PlanItem{{Key: "app-info:en-US:name", Scope: appInfoDirName, Locale: "en-US", Field: "name", Reason: "add", To: "Local"}},
		Updates:  []PlanItem{{Key: "version:1.2.3:en-US:description", Scope: versionDirName, Locale: "en-US", Version: "1.2.3", Field: "description", Reason: "update", From: "Old", To: "New"}},
		Deletes:  []PlanItem{{Key: "version:1.2.3:fr:keywords", Scope: versionDirName, Locale: "fr", Version: "1.2.3", Field: "keywords", Reason: "delete", From: "remote"}},
		APICalls: []PlanAPICall{{Operation: "update_localization", Scope: appInfoDirName, Count: 1}},
		Actions:  []ApplyAction{{Scope: appInfoDirName, Locale: "en-US", Action: "update", LocalizationID: "loc-1"}},
	}
	validateResult := ValidateResult{
		Dir:          "/tmp/meta",
		FilesScanned: 2,
		Issues: []ValidateIssue{
			{Scope: appInfoDirName, File: "/tmp/meta/app-info/en-US.json", Locale: "en-US", Field: "name", Severity: issueSeverityError, Message: "name is required"},
		},
		ErrorCount: 1,
		Valid:      false,
	}
	pullResult := PullResult{
		AppID:     "app-1",
		Version:   "1.2.3",
		Dir:       "/tmp/meta",
		Includes:  []string{"localizations"},
		FileCount: 1,
		Files:     []string{"/tmp/meta/app-info/en-US.json"},
	}

	tableOut := captureStdout(t, func(d *shared.Deps) {
		if err := printPullResultTable(d, pullResult); err != nil {
			t.Fatalf("printPullResultTable() error: %v", err)
		}
		if err := printPushPlanTable(d, pushResult); err != nil {
			t.Fatalf("printPushPlanTable() error: %v", err)
		}
		if err := printValidateResultTable(d, validateResult); err != nil {
			t.Fatalf("printValidateResultTable() error: %v", err)
		}
	})
	if !strings.Contains(tableOut, "Dry Run: true") || !strings.Contains(tableOut, "Files Scanned: 2") {
		t.Fatalf("expected key table render text, got %q", tableOut)
	}

	markdownOut := captureStdout(t, func(d *shared.Deps) {
		if err := printPullResultMarkdown(d, pullResult); err != nil {
			t.Fatalf("printPullResultMarkdown() error: %v", err)
		}
		if err := printPushPlanMarkdown(d, pushResult); err != nil {
			t.Fatalf("printPushPlanMarkdown() error: %v", err)
		}
		if err := printValidateResultMarkdown(d, validateResult); err != nil {
			t.Fatalf("printValidateResultMarkdown() error: %v", err)
		}
	})
	if !strings.Contains(markdownOut, "**Dry Run:** true") || !strings.Contains(markdownOut, "**Files Scanned:** 2") {
		t.Fatalf("expected key markdown render text, got %q", markdownOut)
	}
}

func TestPushPlanPartialRenderersExposeRecoveryDetails(t *testing.T) {
	result := PushPlanResult{
		AppID:               "app-1",
		Version:             "1.2.3",
		Dir:                 "/tmp/meta",
		Total:               2,
		Succeeded:           1,
		Failed:              1,
		FailureArtifactPath: ".asc/reports/metadata-apply/failures-1.json",
		Actions: []ApplyAction{
			{Scope: appInfoDirName, Locale: "en-US", Action: "update", Status: "succeeded", LocalizationID: "loc-en"},
			{Scope: versionDirName, Locale: "fr-FR", Action: "update", Status: "failed", LocalizationID: "loc-fr", Error: "rejected"},
		},
	}
	tableOut := captureStdout(t, func(d *shared.Deps) {
		if err := printPushPlanTable(d, result); err != nil {
			t.Fatalf("printPushPlanTable() error: %v", err)
		}
	})
	for _, expected := range []string{"Total: 2", "Succeeded: 1", "Failed: 1", "Failure Artifact: .asc/reports/metadata-apply/", "rejected"} {
		if !strings.Contains(tableOut, expected) {
			t.Fatalf("table output missing %q: %s", expected, tableOut)
		}
	}

	result.FailureArtifactPath = ""
	result.FailureArtifactError = "mkdir .asc/reports: not a directory"
	markdownOut := captureStdout(t, func(d *shared.Deps) {
		if err := printPushPlanMarkdown(d, result); err != nil {
			t.Fatalf("printPushPlanMarkdown() error: %v", err)
		}
	})
	if !strings.Contains(markdownOut, "**Failure Artifact Error:**") || !strings.Contains(markdownOut, "not a directory") {
		t.Fatalf("markdown output missing artifact write failure: %s", markdownOut)
	}
}

func captureStdout(t *testing.T, fn func(d *shared.Deps)) string {
	t.Helper()

	d := metadataTestDeps()
	fn(d)
	return d.IO.Out.(*bytes.Buffer).String()
}

func metadataTestDeps() *shared.Deps {
	return &shared.Deps{
		IO:      &shared.IO{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}},
		Now:     time.Now,
		Timeout: shared.ContextWithTimeout,
		After:   time.After,
	}
}
