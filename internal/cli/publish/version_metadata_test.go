package publish

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPublishVersionMetadataValuesReadsOnlyVersionLocalizationFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("create app-info dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"Ignored App Name"}`), 0o600); err != nil {
		t.Fatalf("write app-info fixture: %v", err)
	}

	versionDir := filepath.Join(dir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("create version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"Updated description","whatsNew":"Bug fixes"}`), 0o600); err != nil {
		t.Fatalf("write en-US fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "fr-FR.json"), []byte(`{"keywords":"one,two","marketingUrl":"https://example.com/fr"}`), 0o600); err != nil {
		t.Fatalf("write fr-FR fixture: %v", err)
	}

	values, err := loadPublishVersionMetadataValues(dir, "1.2.3")
	if err != nil {
		t.Fatalf("loadPublishVersionMetadataValues() error: %v", err)
	}

	if len(values) != 2 {
		t.Fatalf("expected 2 locales, got %d: %+v", len(values), values)
	}
	if got := values["en-US"]["description"]; got != "Updated description" {
		t.Fatalf("expected en-US description, got %q", got)
	}
	if got := values["en-US"]["whatsNew"]; got != "Bug fixes" {
		t.Fatalf("expected en-US whatsNew, got %q", got)
	}
	if _, ok := values["en-US"]["keywords"]; ok {
		t.Fatalf("did not expect omitted en-US keywords to be populated: %+v", values["en-US"])
	}
	if got := values["fr-FR"]["keywords"]; got != "one,two" {
		t.Fatalf("expected fr-FR keywords, got %q", got)
	}
	if got := values["fr-FR"]["marketingUrl"]; got != "https://example.com/fr" {
		t.Fatalf("expected fr-FR marketingUrl, got %q", got)
	}
}

func TestLoadPublishVersionMetadataValuesRequiresVersionFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("create version dir: %v", err)
	}

	_, err := loadPublishVersionMetadataValues(dir, "1.2.3")
	if err == nil {
		t.Fatal("expected missing version metadata JSON files to fail")
	}
}
