package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadMetadataKeywordsPushEntriesSupportsStringAndObjectValues(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "keywords.json")
	body := `{
		"ja": {"keywords": "nihon,go"},
		"EN-us": "alpha,beta"
	}`
	if err := os.WriteFile(inputPath, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	entries, err := readMetadataKeywordsPushEntries(metadataTestDeps(), inputPath)
	if err != nil {
		t.Fatalf("readMetadataKeywordsPushEntries() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Locale != "en-US" || entries[0].Keywords != "alpha,beta" {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].Locale != "ja" || entries[1].Keywords != "nihon,go" {
		t.Fatalf("unexpected second entry: %+v", entries[1])
	}
}

func TestReadMetadataKeywordsPushEntriesRejectsInvalidEntryShape(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "keywords.json")
	if err := os.WriteFile(inputPath, []byte(`{"en-US":{"value":"alpha,beta"}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readMetadataKeywordsPushEntries(metadataTestDeps(), inputPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "keywords field") {
		t.Fatalf("expected keywords field error, got %v", err)
	}
}

func TestReadMetadataKeywordsPushEntriesRejectsJSONNullKeywords(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "keywords.json")
	if err := os.WriteFile(inputPath, []byte(`{"en-US":null}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readMetadataKeywordsPushEntries(metadataTestDeps(), inputPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must not be null") {
		t.Fatalf("expected null rejection error, got %v", err)
	}
}

func TestReadMetadataKeywordsPushEntriesRejectsUnsupportedLocale(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "keywords.json")
	if err := os.WriteFile(inputPath, []byte(`{"en-ZZ":"alpha,beta"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readMetadataKeywordsPushEntries(metadataTestDeps(), inputPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unsupported locale "en-ZZ"`) {
		t.Fatalf("expected unsupported locale error, got %v", err)
	}
}

func TestReadMetadataKeywordsPushEntriesRejectsEmptyKeywords(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "keywords.json")
	if err := os.WriteFile(inputPath, []byte(`{"en-US":"   "}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readMetadataKeywordsPushEntries(metadataTestDeps(), inputPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "keywords must not be empty") {
		t.Fatalf("expected empty keywords error, got %v", err)
	}
}
