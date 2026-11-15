package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDeliverfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Deliverfile")
	content := `
		# Deliverfile example
		metadata_path "./metadata"
		screenshots_path('./screenshots')
		app_identifier "com.example.app"
		app_version "1.2.3"
		platform "ios"
		skip_metadata true
		skip_screenshots false
	`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write deliverfile: %v", err)
	}

	got, err := parseDeliverfile(path)
	if err != nil {
		t.Fatalf("parseDeliverfile() error: %v", err)
	}

	if got.MetadataPath != "./metadata" {
		t.Fatalf("expected metadata_path ./metadata, got %q", got.MetadataPath)
	}
	if got.ScreenshotsPath != "./screenshots" {
		t.Fatalf("expected screenshots_path ./screenshots, got %q", got.ScreenshotsPath)
	}
	if got.AppIdentifier != "com.example.app" {
		t.Fatalf("expected app_identifier com.example.app, got %q", got.AppIdentifier)
	}
	if got.AppVersion != "1.2.3" {
		t.Fatalf("expected app_version 1.2.3, got %q", got.AppVersion)
	}
	if got.Platform != "ios" {
		t.Fatalf("expected platform ios, got %q", got.Platform)
	}
	if !got.SkipMetadata {
		t.Fatalf("expected skip_metadata true, got false")
	}
	if got.SkipScreenshots {
		t.Fatalf("expected skip_screenshots false, got true")
	}
	if len(got.IgnoredKeys) != 0 {
		t.Fatalf("expected no ignored keys, got %+v", got.IgnoredKeys)
	}
}

func TestParseDeliverfile_InvalidBool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Deliverfile")
	content := "skip_screenshots maybe"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write deliverfile: %v", err)
	}

	_, err := parseDeliverfile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "skip_screenshots") {
		t.Fatalf("expected error to mention skip_screenshots, got %v", err)
	}
}
