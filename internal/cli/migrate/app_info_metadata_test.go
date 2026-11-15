package migrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFastlaneAppInfoMetadata_IncludesPrivacyURL(t *testing.T) {
	dir := t.TempDir()
	localeDir := filepath.Join(dir, "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir locale dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localeDir, "privacy_url.txt"), []byte("https://example.com/privacy"), 0o644); err != nil {
		t.Fatalf("write privacy_url: %v", err)
	}

	locs, err := readFastlaneAppInfoMetadata(migrateTestDeps(), dir)
	if err != nil {
		t.Fatalf("readFastlaneAppInfoMetadata() error: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("expected 1 localization, got %d", len(locs))
	}
	if locs[0].PrivacyURL != "https://example.com/privacy" {
		t.Fatalf("expected privacy url, got %q", locs[0].PrivacyURL)
	}
}
