package xcode

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
)

func TestReadArchiveBundleInfoRejectsOversizedRootInfoPlist(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "Demo.xcarchive")
	if err := os.MkdirAll(archivePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "Info.plist"), bytes.Repeat([]byte("A"), infoplist.MaxBytes+1), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readArchiveBundleInfo(archivePath)
	if err == nil {
		t.Fatal("expected oversized archive Info.plist rejection, got nil")
	}
	if !strings.Contains(err.Error(), "Info.plist limit") {
		t.Fatalf("expected Info.plist limit error, got %v", err)
	}
}

func TestReadArchiveExportOptionsTeamIDRejectsOversizedEmbeddedAppInfoPlist(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "Demo.xcarchive")
	if err := writeArchiveInfoPlist(archivePath); err != nil {
		t.Fatalf("writeArchiveInfoPlist() error: %v", err)
	}
	appInfoPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app", "Info.plist")
	if err := os.MkdirAll(filepath.Dir(appInfoPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(appInfoPath, bytes.Repeat([]byte("A"), infoplist.MaxBytes+1), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readArchiveExportOptionsTeamID(archivePath)
	if err == nil {
		t.Fatal("expected oversized embedded app Info.plist rejection, got nil")
	}
	if !strings.Contains(err.Error(), "Info.plist limit") {
		t.Fatalf("expected Info.plist limit error, got %v", err)
	}
}
