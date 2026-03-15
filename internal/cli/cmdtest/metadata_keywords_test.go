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
)

func TestMetadataHelpShowsKeywordsWorkflow(t *testing.T) {
	root := RootCommand("1.2.3")

	metadataCmd := findSubcommand(root, "metadata")
	if metadataCmd == nil {
		t.Fatal("expected metadata command")
	}

	metadataUsage := metadataCmd.UsageFunc(metadataCmd)
	if !usageListsSubcommand(metadataUsage, "keywords") {
		t.Fatalf("expected metadata help to list keywords, got %q", metadataUsage)
	}
	if !strings.Contains(metadataUsage, "searchKeywords") {
		t.Fatalf("expected metadata help to explain searchKeywords distinction, got %q", metadataUsage)
	}

	keywordsCmd := findSubcommand(root, "metadata", "keywords")
	if keywordsCmd == nil {
		t.Fatal("expected metadata keywords command")
	}
	keywordsUsage := keywordsCmd.UsageFunc(keywordsCmd)
	for _, subcommand := range []string{"import", "plan", "diff", "localize", "apply", "sync"} {
		if !usageListsSubcommand(keywordsUsage, subcommand) {
			t.Fatalf("expected metadata keywords help to list %s, got %q", subcommand, keywordsUsage)
		}
	}
	if !strings.Contains(keywordsUsage, "asc apps search-keywords") {
		t.Fatalf("expected metadata keywords help to point to raw relationship commands, got %q", keywordsUsage)
	}
}

func TestRawSearchKeywordsHelpPointsToMetadataKeywords(t *testing.T) {
	root := RootCommand("1.2.3")

	appsCmd := findSubcommand(root, "apps", "search-keywords")
	if appsCmd == nil {
		t.Fatal("expected apps search-keywords command")
	}
	appsUsage := appsCmd.UsageFunc(appsCmd)
	if !strings.Contains(appsUsage, "asc metadata keywords") {
		t.Fatalf("expected apps search-keywords help to point to metadata keywords, got %q", appsUsage)
	}

	localizationsCmd := findSubcommand(root, "localizations", "search-keywords")
	if localizationsCmd == nil {
		t.Fatal("expected localizations search-keywords command")
	}
	localizationsUsage := localizationsCmd.UsageFunc(localizationsCmd)
	if !strings.Contains(localizationsUsage, "asc metadata keywords") {
		t.Fatalf("expected localizations search-keywords help to point to metadata keywords, got %q", localizationsUsage)
	}
}

func TestRootHelpShowsMetadataInAppManagement(t *testing.T) {
	root := RootCommand("1.2.3")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{}); err != nil {
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

	appManagement := metadataKeywordHelpSection(stderr, "APP MANAGEMENT COMMANDS", "TESTFLIGHT & BUILD COMMANDS")
	if !strings.Contains(appManagement, "metadata:") {
		t.Fatalf("expected metadata in app management help section, got %q", appManagement)
	}
	if strings.Contains(stderr, "ADDITIONAL COMMANDS\n  metadata:") {
		t.Fatalf("expected metadata to be removed from additional commands, got %q", stderr)
	}
}

func TestMetadataKeywordsImportJSONDryRun(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.json")
	input := `{"localizations":[{"locale":"en-US","keywords":[" habit tracker ","mood journal","habit tracker"]},{"locale":"fr-FR","keywords":"journal d'humeur,habitudes"}]}`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "json",
			"--dry-run",
		}); err != nil {
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
		DryRun          bool     `json:"dryRun"`
		DetectedLocales []string `json:"detectedLocales"`
		Results         []struct {
			Locale            string   `json:"locale"`
			Action            string   `json:"action"`
			KeywordField      string   `json:"keywordField"`
			KeywordCount      int      `json:"keywordCount"`
			DuplicateCount    int      `json:"duplicateCount"`
			SkippedDuplicates []string `json:"skippedDuplicates"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.DryRun {
		t.Fatal("expected dryRun true")
	}
	if len(payload.DetectedLocales) != 2 || payload.DetectedLocales[0] != "en-US" || payload.DetectedLocales[1] != "fr-FR" {
		t.Fatalf("expected detected locales [en-US fr-FR], got %+v", payload.DetectedLocales)
	}
	if len(payload.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(payload.Results))
	}
	if payload.Results[0].Locale != "en-US" || payload.Results[0].Action != "create" || payload.Results[0].KeywordField != "habit tracker,mood journal" || payload.Results[0].KeywordCount != 2 || payload.Results[0].DuplicateCount != 1 || len(payload.Results[0].SkippedDuplicates) != 1 || payload.Results[0].SkippedDuplicates[0] != "habit tracker" {
		t.Fatalf("unexpected en-US result: %+v", payload.Results[0])
	}
	if payload.Results[1].Locale != "fr-FR" || payload.Results[1].KeywordField != "journal d'humeur,habitudes" {
		t.Fatalf("unexpected fr-FR result: %+v", payload.Results[1])
	}

	path, err := filepath.Abs(filepath.Join(dir, "version", "1.2.3", "en-US.json"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dry-run to avoid writing %s, got err=%v", path, err)
	}
}

func TestMetadataKeywordsImportDryRunReportsOverLimitIssue(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.txt")
	// 101 runes, above the App Store keyword limit.
	if err := os.WriteFile(inputPath, []byte(strings.Repeat("k", 101)), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "text",
			"--locale", "en-US",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected non-nil run error for invalid preview")
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Valid  bool `json:"valid"`
		Issues []struct {
			Locale       string `json:"locale"`
			Message      string `json:"message"`
			KeywordField string `json:"keywordField"`
			Length       int    `json:"length"`
			Limit        int    `json:"limit"`
		} `json:"issues"`
		Results []struct {
			Locale string `json:"locale"`
			Action string `json:"action"`
			Reason string `json:"reason"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.Valid {
		t.Fatalf("expected invalid preview payload, got %+v", payload)
	}
	if len(payload.Issues) != 1 || payload.Issues[0].Locale != "en-US" || payload.Issues[0].Message != "keywords exceed 100 characters" || payload.Issues[0].Length != 101 || payload.Issues[0].Limit != 100 {
		t.Fatalf("unexpected issues payload: %+v", payload.Issues)
	}
	if len(payload.Results) != 1 || payload.Results[0].Action != "invalid" || payload.Results[0].Reason != "keywords exceed 100 characters" {
		t.Fatalf("unexpected result payload: %+v", payload.Results)
	}
}

func TestMetadataKeywordsImportTextCanonicalizesLocaleAlias(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.txt")
	if err := os.WriteFile(inputPath, []byte("habit tracker,\nmood journal"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "text",
			"--locale", "en_US",
			"--dry-run",
		}); err != nil {
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
		Results []struct {
			Locale string `json:"locale"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 1 || payload.Results[0].Locale != "en-US" {
		t.Fatalf("expected canonical en-US locale, got %+v", payload.Results)
	}
}

func TestMetadataKeywordsImportReusesExistingAliasLocaleFile(t *testing.T) {
	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	aliasPath := filepath.Join(versionDir, "en_US.json")
	if err := os.WriteFile(aliasPath, []byte(`{"description":"Existing description","keywords":"old,keywords"}`), 0o644); err != nil {
		t.Fatalf("write alias file: %v", err)
	}
	inputPath := filepath.Join(t.TempDir(), "keywords.txt")
	if err := os.WriteFile(inputPath, []byte("habit tracker,mood journal"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "text",
			"--locale", "en-US",
		}); err != nil {
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
		Results []struct {
			File         string `json:"file"`
			Locale       string `json:"locale"`
			KeywordField string `json:"keywordField"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 1 || payload.Results[0].File != aliasPath || payload.Results[0].Locale != "en-US" {
		t.Fatalf("expected alias path reused in output, got %+v", payload.Results)
	}

	data, err := os.ReadFile(aliasPath)
	if err != nil {
		t.Fatalf("read alias file: %v", err)
	}
	var filePayload map[string]string
	if err := json.Unmarshal(data, &filePayload); err != nil {
		t.Fatalf("unmarshal alias file: %v", err)
	}
	if filePayload["description"] != "Existing description" || filePayload["keywords"] != "habit tracker,mood journal" {
		t.Fatalf("unexpected alias file payload: %+v", filePayload)
	}
	if _, err := os.Stat(filepath.Join(versionDir, "en-US.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected canonical duplicate file not to be created, got err=%v", err)
	}
}

func TestMetadataKeywordsImportTextNormalizesMixedSeparatorsAndDuplicates(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.txt")
	input := "habit tracker，mood journal\nHabit Tracker； sleep log"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "text",
			"--locale", "en-US",
			"--dry-run",
		}); err != nil {
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
		Results []struct {
			KeywordField string `json:"keywordField"`
			KeywordCount int    `json:"keywordCount"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("expected 1 result, got %+v", payload.Results)
	}
	if payload.Results[0].KeywordField != "habit tracker,mood journal,sleep log" {
		t.Fatalf("expected normalized keyword field, got %+v", payload.Results[0])
	}
	if payload.Results[0].KeywordCount != 3 {
		t.Fatalf("expected keyword count 3, got %+v", payload.Results[0])
	}
}

func TestMetadataKeywordsImportRejectsAmbiguousLocaleAlias(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.txt")
	if err := os.WriteFile(inputPath, []byte("habit tracker"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "text",
			"--locale", "english",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{`ambiguous locale "english"`, "en-AU", "en-CA", "en-GB", "en-US"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}

func TestMetadataKeywordsImportCSVWritesCanonicalFiles(t *testing.T) {
	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"Existing description","keywords":"old,keywords"}`), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	inputPath := filepath.Join(t.TempDir(), "keywords.csv")
	input := "locale,keyword\nen-US,habit tracker\nen-US,mood journal\nfr-FR,journal humeur\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "csv",
		}); err != nil {
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
		Results []struct {
			Locale string `json:"locale"`
			Action string `json:"action"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(payload.Results))
	}
	if payload.Results[0].Action != "update" || payload.Results[1].Action != "create" {
		t.Fatalf("unexpected import actions: %+v", payload.Results)
	}

	enData, err := os.ReadFile(filepath.Join(versionDir, "en-US.json"))
	if err != nil {
		t.Fatalf("read en-US file: %v", err)
	}
	var enPayload map[string]string
	if err := json.Unmarshal(enData, &enPayload); err != nil {
		t.Fatalf("unmarshal en-US file: %v", err)
	}
	if enPayload["description"] != "Existing description" {
		t.Fatalf("expected description preserved, got %+v", enPayload)
	}
	if enPayload["keywords"] != "habit tracker,mood journal" {
		t.Fatalf("expected keywords replaced, got %+v", enPayload)
	}

	frData, err := os.ReadFile(filepath.Join(versionDir, "fr-FR.json"))
	if err != nil {
		t.Fatalf("read fr-FR file: %v", err)
	}
	var frPayload map[string]string
	if err := json.Unmarshal(frData, &frPayload); err != nil {
		t.Fatalf("unmarshal fr-FR file: %v", err)
	}
	if frPayload["keywords"] != "journal humeur" {
		t.Fatalf("expected fr-FR keywords file, got %+v", frPayload)
	}
}

func TestMetadataKeywordsImportCSVNormalizesRowDuplicatesAndChineseCommas(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.csv")
	input := "locale,keyword\nen-US,\" habit tracker， mood journal \"\nen-US,\"Habit Tracker\"\nen-US,\"sleep log\"\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "csv",
			"--dry-run",
		}); err != nil {
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
		Results []struct {
			Locale       string `json:"locale"`
			KeywordField string `json:"keywordField"`
			KeywordCount int    `json:"keywordCount"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("expected 1 result, got %+v", payload.Results)
	}
	if payload.Results[0].Locale != "en-US" {
		t.Fatalf("expected en-US locale, got %+v", payload.Results[0])
	}
	if payload.Results[0].KeywordField != "habit tracker,mood journal,sleep log" {
		t.Fatalf("expected normalized keyword field, got %+v", payload.Results[0])
	}
	if payload.Results[0].KeywordCount != 3 {
		t.Fatalf("expected keyword count 3, got %+v", payload.Results[0])
	}
}

func TestMetadataKeywordsImportCSVAcceptsUTF8BOMHeader(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords-bom.csv")
	input := "\ufefflocale,keyword\nen-US,habit tracker\nen-US,mood journal\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "csv",
			"--dry-run",
		}); err != nil {
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
		DetectedLocales []string `json:"detectedLocales"`
		Results         []struct {
			Locale       string `json:"locale"`
			KeywordField string `json:"keywordField"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.DetectedLocales) != 1 || payload.DetectedLocales[0] != "en-US" {
		t.Fatalf("expected detected en-US locale, got %+v", payload.DetectedLocales)
	}
	if len(payload.Results) != 1 || payload.Results[0].Locale != "en-US" || payload.Results[0].KeywordField != "habit tracker,mood journal" {
		t.Fatalf("unexpected BOM csv payload: %+v", payload.Results)
	}
}

func TestMetadataKeywordsImportJSONIgnoresResearchSideFields(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.json")
	reportPath := filepath.Join(t.TempDir(), "side-data.json")
	input := `{
		"en-US": {
			"keywords": ["habit tracker", "mood journal"],
			"popularity": 42,
			"difficulty": 31,
			"notes": "high intent",
			"tags": ["opportunity"],
			"history": [{"rank": 7}]
		}
	}`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "json",
			"--side-data-report-file", reportPath,
		}); err != nil {
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
		Results []struct {
			Locale       string `json:"locale"`
			KeywordField string `json:"keywordField"`
			KeywordCount int    `json:"keywordCount"`
		} `json:"results"`
		SideDataRecordCount int    `json:"sideDataRecordCount"`
		SideDataReportPath  string `json:"sideDataReportPath"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 1 || payload.Results[0].Locale != "en-US" || payload.Results[0].KeywordField != "habit tracker,mood journal" || payload.Results[0].KeywordCount != 2 {
		t.Fatalf("unexpected output payload: %+v", payload.Results)
	}
	if payload.SideDataRecordCount != 1 || payload.SideDataReportPath != reportPath {
		t.Fatalf("expected side data artifact metadata, got %+v", payload)
	}

	data, err := os.ReadFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"))
	if err != nil {
		t.Fatalf("read canonical file: %v", err)
	}
	var filePayload map[string]any
	if err := json.Unmarshal(data, &filePayload); err != nil {
		t.Fatalf("unmarshal canonical file: %v", err)
	}
	if len(filePayload) != 1 {
		t.Fatalf("expected only publishable metadata fields in canonical file, got %+v", filePayload)
	}
	if filePayload["keywords"] != "habit tracker,mood journal" {
		t.Fatalf("expected canonical keywords only, got %+v", filePayload)
	}

	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read side data report: %v", err)
	}
	var reportPayload struct {
		Records []struct {
			Locale string         `json:"locale"`
			Fields map[string]any `json:"fields"`
		} `json:"records"`
	}
	if err := json.Unmarshal(reportData, &reportPayload); err != nil {
		t.Fatalf("unmarshal side data report: %v", err)
	}
	if len(reportPayload.Records) != 1 || reportPayload.Records[0].Locale != "en-US" {
		t.Fatalf("unexpected side data report payload: %+v", reportPayload.Records)
	}
	for _, key := range []string{"popularity", "difficulty", "notes", "tags", "history"} {
		if _, ok := reportPayload.Records[0].Fields[key]; !ok {
			t.Fatalf("expected side data field %q in report, got %+v", key, reportPayload.Records[0].Fields)
		}
	}
}

func TestMetadataKeywordsImportCSVIgnoresResearchColumns(t *testing.T) {
	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	reportPath := filepath.Join(t.TempDir(), "side-data.csv.json")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"Existing description"}`), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	inputPath := filepath.Join(t.TempDir(), "keywords.csv")
	input := "locale,keyword,popularity,difficulty,notes,tags,rank\nen-US,habit tracker,42,31,high intent,opportunity,7\nen-US,mood journal,35,28,secondary,core,9\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "csv",
			"--side-data-report-file", reportPath,
		}); err != nil {
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
		Results []struct {
			Locale       string `json:"locale"`
			KeywordField string `json:"keywordField"`
		} `json:"results"`
		SideDataRecordCount int    `json:"sideDataRecordCount"`
		SideDataReportPath  string `json:"sideDataReportPath"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 1 || payload.Results[0].Locale != "en-US" || payload.Results[0].KeywordField != "habit tracker,mood journal" {
		t.Fatalf("unexpected output payload: %+v", payload.Results)
	}
	if payload.SideDataRecordCount != 2 || payload.SideDataReportPath != reportPath {
		t.Fatalf("expected side data artifact metadata, got %+v", payload)
	}

	data, err := os.ReadFile(filepath.Join(versionDir, "en-US.json"))
	if err != nil {
		t.Fatalf("read canonical file: %v", err)
	}
	var filePayload map[string]any
	if err := json.Unmarshal(data, &filePayload); err != nil {
		t.Fatalf("unmarshal canonical file: %v", err)
	}
	if len(filePayload) != 2 {
		t.Fatalf("expected only description and keywords in canonical file, got %+v", filePayload)
	}
	if filePayload["description"] != "Existing description" || filePayload["keywords"] != "habit tracker,mood journal" {
		t.Fatalf("unexpected canonical metadata contents: %+v", filePayload)
	}

	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read side data report: %v", err)
	}
	var reportPayload struct {
		Records []struct {
			Locale string         `json:"locale"`
			Fields map[string]any `json:"fields"`
		} `json:"records"`
	}
	if err := json.Unmarshal(reportData, &reportPayload); err != nil {
		t.Fatalf("unmarshal side data report: %v", err)
	}
	if len(reportPayload.Records) != 2 {
		t.Fatalf("expected 2 side data records, got %+v", reportPayload.Records)
	}
	for _, key := range []string{"popularity", "difficulty", "notes", "tags", "rank"} {
		if _, ok := reportPayload.Records[0].Fields[key]; !ok {
			t.Fatalf("expected side data field %q in report, got %+v", key, reportPayload.Records[0].Fields)
		}
	}
}

func TestMetadataKeywordsImportAstroCSVPresetUsesKeywordColumn(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "astro.csv")
	input := "Keyword,Notes,Popularity,Difficulty,Position,Apps in Ranking\nhabit tracker,high intent,42,31,7,App A\nmood journal,secondary,35,28,9,App B\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write astro csv: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "astro-csv",
			"--locale", "en-US",
			"--dry-run",
		}); err != nil {
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
		Results []struct {
			Locale       string `json:"locale"`
			KeywordField string `json:"keywordField"`
			KeywordCount int    `json:"keywordCount"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 1 || payload.Results[0].Locale != "en-US" || payload.Results[0].KeywordField != "habit tracker,mood journal" || payload.Results[0].KeywordCount != 2 {
		t.Fatalf("unexpected astro preset payload: %+v", payload.Results)
	}
}

func TestMetadataKeywordsImportAstroCSVPresetRequiresLocaleWithoutLocaleColumn(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "astro.csv")
	input := "Keyword,Notes,Popularity,Difficulty,Position,Apps in Ranking\nhabit tracker,high intent,42,31,7,App A\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write astro csv: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "astro-csv",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "missing a locale") {
		t.Fatalf("expected locale requirement error, got %q", stderr)
	}
}

func TestMetadataKeywordsLocalizeSkipsExistingWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"keywords":"habit tracker,mood journal"}`), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "fr-FR.json"), []byte(`{"keywords":"existing,keywords"}`), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "localize",
			"--dir", dir,
			"--version", "1.2.3",
			"--from-locale", "en-US",
			"--to-locales", "fr-FR,de-DE",
		}); err != nil {
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
		Results []struct {
			Locale string `json:"locale"`
			Action string `json:"action"`
			Reason string `json:"reason"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(payload.Results))
	}
	if payload.Results[1].Locale != "fr-FR" && payload.Results[0].Locale != "fr-FR" {
		t.Fatalf("expected fr-FR result, got %+v", payload.Results)
	}

	frData, err := os.ReadFile(filepath.Join(versionDir, "fr-FR.json"))
	if err != nil {
		t.Fatalf("read fr-FR file: %v", err)
	}
	if string(frData) != `{"keywords":"existing,keywords"}` {
		t.Fatalf("expected fr-FR unchanged, got %q", frData)
	}

	deData, err := os.ReadFile(filepath.Join(versionDir, "de-DE.json"))
	if err != nil {
		t.Fatalf("read de-DE file: %v", err)
	}
	var dePayload map[string]string
	if err := json.Unmarshal(deData, &dePayload); err != nil {
		t.Fatalf("unmarshal de-DE file: %v", err)
	}
	if dePayload["keywords"] != "habit tracker,mood journal" {
		t.Fatalf("expected de-DE keywords copy, got %+v", dePayload)
	}
}

func TestMetadataKeywordsLocalizeReusesExistingAliasTargetFile(t *testing.T) {
	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"keywords":"habit tracker,mood journal"}`), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	aliasPath := filepath.Join(versionDir, "de_DE.json")
	if err := os.WriteFile(aliasPath, []byte(`{"keywords":"old,keywords"}`), 0o644); err != nil {
		t.Fatalf("write alias target file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "localize",
			"--dir", dir,
			"--version", "1.2.3",
			"--from-locale", "en-US",
			"--to-locales", "de-DE",
			"--overwrite",
		}); err != nil {
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
		Results []struct {
			File   string `json:"file"`
			Locale string `json:"locale"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 1 || payload.Results[0].File != aliasPath || payload.Results[0].Locale != "de-DE" {
		t.Fatalf("expected alias target reused in output, got %+v", payload.Results)
	}

	data, err := os.ReadFile(aliasPath)
	if err != nil {
		t.Fatalf("read alias file: %v", err)
	}
	var filePayload map[string]string
	if err := json.Unmarshal(data, &filePayload); err != nil {
		t.Fatalf("unmarshal alias file: %v", err)
	}
	if filePayload["keywords"] != "habit tracker,mood journal" {
		t.Fatalf("unexpected alias target payload: %+v", filePayload)
	}
	if _, err := os.Stat(filepath.Join(versionDir, "de-DE.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected canonical duplicate file not to be created, got err=%v", err)
	}
}

func TestMetadataKeywordsLocalizeReadsExistingAliasSourceFile(t *testing.T) {
	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	sourceAliasPath := filepath.Join(versionDir, "en_US.json")
	if err := os.WriteFile(sourceAliasPath, []byte(`{"keywords":"habit tracker,mood journal"}`), 0o644); err != nil {
		t.Fatalf("write source alias file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "localize",
			"--dir", dir,
			"--version", "1.2.3",
			"--from-locale", "en-US",
			"--to-locales", "fr-FR",
		}); err != nil {
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
		Results []struct {
			Locale string `json:"locale"`
			Action string `json:"action"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 1 || payload.Results[0].Locale != "fr-FR" || payload.Results[0].Action != "create" {
		t.Fatalf("unexpected localize output: %+v", payload.Results)
	}

	frData, err := os.ReadFile(filepath.Join(versionDir, "fr-FR.json"))
	if err != nil {
		t.Fatalf("read fr-FR file: %v", err)
	}
	var frPayload map[string]string
	if err := json.Unmarshal(frData, &frPayload); err != nil {
		t.Fatalf("unmarshal fr-FR file: %v", err)
	}
	if frPayload["keywords"] != "habit tracker,mood journal" {
		t.Fatalf("expected alias source keywords copied, got %+v", frPayload)
	}
}

func TestMetadataKeywordsPlanBuildsKeywordOnlyRemotePlan(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"Local description","keywords":"one,two"}`), 0o644); err != nil {
		t.Fatalf("write en-US file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "ja.json"), []byte(`{"keywords":"nihongo"}`), 0o644); err != nil {
		t.Fatalf("write ja file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET only, got %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			return metadataKeywordsJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`)
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return metadataKeywordsJSONResponse(`{
				"data":[
					{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Remote description","keywords":"one,remote"}},
					{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","keywords":"remote-only"}}
				],
				"links":{"next":""}
			}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "plan",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
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
		Adds []struct {
			Locale string `json:"locale"`
			Field  string `json:"field"`
		} `json:"adds"`
		Updates []struct {
			Locale string `json:"locale"`
			Field  string `json:"field"`
			From   string `json:"from"`
			To     string `json:"to"`
		} `json:"updates"`
		Warnings []struct {
			Action        string   `json:"action"`
			Locale        string   `json:"locale"`
			Message       string   `json:"message"`
			MissingFields []string `json:"missingFields"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Adds) != 1 || payload.Adds[0].Locale != "ja" || payload.Adds[0].Field != "keywords" {
		t.Fatalf("expected one ja add, got %+v", payload.Adds)
	}
	if len(payload.Updates) != 1 || payload.Updates[0].Locale != "en-US" || payload.Updates[0].From != "one,remote" || payload.Updates[0].To != "one,two" {
		t.Fatalf("expected one en-US update, got %+v", payload.Updates)
	}
	if len(payload.Warnings) != 1 || payload.Warnings[0].Action != "create" || payload.Warnings[0].Locale != "ja" {
		t.Fatalf("expected one ja warning, got %+v", payload.Warnings)
	}
	if !strings.Contains(payload.Warnings[0].Message, "create would leave locale") || !strings.Contains(payload.Warnings[0].Message, "description, supportUrl") {
		t.Fatalf("expected create warning message with missing fields, got %+v", payload.Warnings[0])
	}
	if len(payload.Warnings[0].MissingFields) != 2 || payload.Warnings[0].MissingFields[0] != "description" || payload.Warnings[0].MissingFields[1] != "supportUrl" {
		t.Fatalf("expected missing description/supportUrl warning, got %+v", payload.Warnings[0])
	}
}

func TestMetadataKeywordsPlanDoesNotWarnForExistingLocaleUpdate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"keywords":"one,two"}`), 0o644); err != nil {
		t.Fatalf("write en-US file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET only, got %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			return metadataKeywordsJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`)
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return metadataKeywordsJSONResponse(`{
				"data":[
					{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Remote description","supportUrl":"https://example.com/support","keywords":"old,keywords"}}
				],
				"links":{"next":""}
			}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "plan",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
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
		Updates []struct {
			Locale string `json:"locale"`
			From   string `json:"from"`
			To     string `json:"to"`
		} `json:"updates"`
		Warnings []struct{} `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Updates) != 1 || payload.Updates[0].Locale != "en-US" || payload.Updates[0].From != "old,keywords" || payload.Updates[0].To != "one,two" {
		t.Fatalf("expected single en-US update, got %+v", payload.Updates)
	}
	if len(payload.Warnings) != 0 {
		t.Fatalf("expected no warnings for existing-locale update, got %+v", payload.Warnings)
	}
}

func TestMetadataKeywordsPlanIgnoresDefaultLocaleFile(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "default.json"), []byte(`{"keywords":"default,keywords"}`), 0o644); err != nil {
		t.Fatalf("write default file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"keywords":"one,two"}`), 0o644); err != nil {
		t.Fatalf("write en-US file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET only, got %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			return metadataKeywordsJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`)
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return metadataKeywordsJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","keywords":"remote,keywords"}}],"links":{"next":""}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "plan",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
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
		Adds    []struct{} `json:"adds"`
		Updates []struct {
			Locale string `json:"locale"`
			From   string `json:"from"`
			To     string `json:"to"`
		} `json:"updates"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Adds) != 0 {
		t.Fatalf("expected no adds from default locale fallback file, got %+v", payload.Adds)
	}
	if len(payload.Updates) != 1 || payload.Updates[0].Locale != "en-US" || payload.Updates[0].From != "remote,keywords" || payload.Updates[0].To != "one,two" {
		t.Fatalf("expected single en-US update, got %+v", payload.Updates)
	}
}

func TestMetadataKeywordsPlanRejectsDuplicateCanonicalLocaleFiles(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"keywords":"one,two"}`), 0o644); err != nil {
		t.Fatalf("write en-US file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en_US.json"), []byte(`{"keywords":"three,four"}`), 0o644); err != nil {
		t.Fatalf("write en_US file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "plan",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
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

func TestMetadataKeywordsApplyRequiresConfirm(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"keywords":"one,two"}`), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		t.Fatalf("expected apply without --confirm to stop before network/auth, got %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "apply",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if callCount != 0 {
		t.Fatalf("expected no network calls before confirm error, got %d", callCount)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --confirm is required") {
		t.Fatalf("expected confirm error, got %q", stderr)
	}
}

func TestMetadataKeywordsImportJSONLocaleMapUsesStableCanonicalOrder(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords-locale-map.json")
	reportPath := filepath.Join(t.TempDir(), "side-data-locale-map.json")
	input := `{
		"en_US": {
			"keyword": "second",
			"notes": "second-note"
		},
		"en-US": {
			"keyword": "first",
			"notes": "first-note"
		}
	}`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "import",
			"--dir", dir,
			"--version", "1.2.3",
			"--input", inputPath,
			"--format", "json",
			"--side-data-report-file", reportPath,
		}); err != nil {
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
		Results []struct {
			Locale       string `json:"locale"`
			KeywordField string `json:"keywordField"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if len(payload.Results) != 1 || payload.Results[0].Locale != "en-US" || payload.Results[0].KeywordField != "first,second" {
		t.Fatalf("expected deterministic canonical keyword order, got %+v", payload.Results)
	}

	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read side data report: %v", err)
	}
	var reportPayload struct {
		Records []struct {
			Locale   string         `json:"locale"`
			Keywords []string       `json:"keywords"`
			Fields   map[string]any `json:"fields"`
		} `json:"records"`
	}
	if err := json.Unmarshal(reportData, &reportPayload); err != nil {
		t.Fatalf("unmarshal side data report: %v", err)
	}
	if len(reportPayload.Records) != 2 {
		t.Fatalf("expected 2 side data records, got %+v", reportPayload.Records)
	}
	if reportPayload.Records[0].Locale != "en-US" || len(reportPayload.Records[0].Keywords) != 1 || reportPayload.Records[0].Keywords[0] != "first" {
		t.Fatalf("expected first record from canonical en-US key, got %+v", reportPayload.Records[0])
	}
	if reportPayload.Records[1].Locale != "en-US" || len(reportPayload.Records[1].Keywords) != 1 || reportPayload.Records[1].Keywords[0] != "second" {
		t.Fatalf("expected second record from en_US alias key, got %+v", reportPayload.Records[1])
	}
}

func TestMetadataKeywordsApplyCreatesLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "ja.json"), []byte(`{"description":"Japanese description","supportUrl":"https://example.com/support","keywords":"nihongo"}`), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var postBody string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			return metadataKeywordsJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`)
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET for localizations, got %s", req.Method)
			}
			return metadataKeywordsJSONResponse(`{"data":[],"links":{"next":""}}`)
		case "/v1/appStoreVersionLocalizations":
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST create, got %s", req.Method)
			}
			body, _ := io.ReadAll(req.Body)
			postBody = string(body)
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","keywords":"nihongo"}}}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "apply",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(postBody, `"locale":"ja"`) ||
		!strings.Contains(postBody, `"keywords":"nihongo"`) ||
		!strings.Contains(postBody, `"description":"Japanese description"`) ||
		!strings.Contains(postBody, `"supportUrl":"https://example.com/support"`) {
		t.Fatalf("expected create body to include locale and keywords, got %s", postBody)
	}

	var payload struct {
		Applied bool `json:"applied"`
		Actions []struct {
			Action string `json:"action"`
			Locale string `json:"locale"`
		} `json:"actions"`
		Warnings []struct {
			Action string `json:"action"`
			Locale string `json:"locale"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Applied {
		t.Fatal("expected applied result")
	}
	if len(payload.Actions) != 1 || payload.Actions[0].Action != "create" || payload.Actions[0].Locale != "ja" {
		t.Fatalf("expected one create action, got %+v", payload.Actions)
	}
	if len(payload.Warnings) != 0 {
		t.Fatalf("expected no warnings when create body includes required metadata, got %+v", payload.Warnings)
	}
}

func TestMetadataKeywordsSyncDryRunUsesImportedStateWithoutWriting(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.json")
	if err := os.WriteFile(inputPath, []byte(`{"locale":"en-US","keywords":["habit tracker","mood journal"]}`), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET only for dry-run sync, got %s %s", req.Method, req.URL.Path)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			return metadataKeywordsJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`)
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return metadataKeywordsJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","keywords":"old,keywords"}}],"links":{"next":""}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "sync",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--input", inputPath,
			"--format", "json",
			"--dry-run",
		}); err != nil {
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
		Import struct {
			DryRun  bool `json:"dryRun"`
			Results []struct {
				Locale string `json:"locale"`
				Action string `json:"action"`
			} `json:"results"`
		} `json:"import"`
		Plan struct {
			DryRun  bool `json:"dryRun"`
			Applied bool `json:"applied"`
			Updates []struct {
				Locale string `json:"locale"`
				To     string `json:"to"`
			} `json:"updates"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !payload.Import.DryRun || !payload.Plan.DryRun {
		t.Fatalf("expected dry-run import and plan, got %+v", payload)
	}
	if payload.Plan.Applied {
		t.Fatalf("expected sync dry-run not to apply, got %+v", payload.Plan)
	}
	if len(payload.Import.Results) != 1 || payload.Import.Results[0].Action != "create" {
		t.Fatalf("expected one import create result, got %+v", payload.Import.Results)
	}
	if len(payload.Plan.Updates) != 1 || payload.Plan.Updates[0].Locale != "en-US" || payload.Plan.Updates[0].To != "habit tracker,mood journal" {
		t.Fatalf("expected one remote update plan, got %+v", payload.Plan.Updates)
	}

	if _, err := os.Stat(filepath.Join(dir, "version", "1.2.3", "en-US.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dry-run sync to avoid writing canonical file, got err=%v", err)
	}
}

func TestMetadataKeywordsSyncStopsWhenImportPreviewHasIssues(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.txt")
	if err := os.WriteFile(inputPath, []byte(strings.Repeat("k", 101)), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		t.Fatalf("expected sync to stop before remote planning, got %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "sync",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--input", inputPath,
			"--format", "text",
			"--locale", "en-US",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected non-nil run error for invalid sync preview")
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if callCount != 0 {
		t.Fatalf("expected no remote planning requests, got %d", callCount)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if _, ok := payload["plan"]; ok {
		t.Fatalf("expected sync output to omit plan when import preview fails, got %+v", payload)
	}
	importPayload, ok := payload["import"].(map[string]any)
	if !ok {
		t.Fatalf("expected import object, got %+v", payload["import"])
	}
	if valid, _ := importPayload["valid"].(bool); valid {
		t.Fatalf("expected invalid import payload, got %+v", importPayload)
	}
}

func TestMetadataKeywordsSyncMissingAppDoesNotWriteImportOutput(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.txt")
	if err := os.WriteFile(inputPath, []byte("habit tracker,mood journal"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "sync",
			"--version", "1.2.3",
			"--dir", dir,
			"--input", inputPath,
			"--format", "text",
			"--locale", "en-US",
		}); err != nil {
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
	if !strings.Contains(stderr, "Error: --app is required") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "version", "1.2.3", "en-US.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected sync preflight failure to avoid writing metadata file, got err=%v", err)
	}
}

func TestMetadataKeywordsSyncInvalidPlatformDoesNotWriteImportOutput(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.txt")
	if err := os.WriteFile(inputPath, []byte("habit tracker,mood journal"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "keywords", "sync",
			"--app", "app-1",
			"--version", "1.2.3",
			"--platform", "watchos",
			"--dir", dir,
			"--input", inputPath,
			"--format", "text",
			"--locale", "en-US",
		}); err != nil {
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
	if !strings.Contains(stderr, "--platform must be one of") {
		t.Fatalf("expected platform validation error, got %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "version", "1.2.3", "en-US.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected invalid platform to avoid writing metadata file, got err=%v", err)
	}
}

func metadataKeywordsJSONResponse(body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func metadataKeywordHelpSection(help string, startHeading string, endHeading string) string {
	start := strings.Index(help, startHeading)
	if start == -1 {
		return ""
	}
	section := help[start:]
	if endHeading == "" {
		return section
	}
	end := strings.Index(section, endHeading)
	if end == -1 {
		return section
	}
	return section[:end]
}
