package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func mustDurationValue(t *testing.T, raw string) DurationValue {
	t.Helper()
	value, err := ParseDurationValue(raw)
	if err != nil {
		t.Fatalf("ParseDurationValue(%q) error: %v", raw, err)
	}
	return value
}

func TestConfigSaveLoadRemove(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)

	cfg := &Config{
		KeyID:                 "KEY123",
		IssuerID:              "ISSUER456",
		PrivateKeyPath:        "/tmp/AuthKey.p8",
		DefaultKeyName:        "default",
		AppID:                 "APP123",
		VendorNumber:          "VENDOR123",
		AnalyticsVendorNumber: "ANALYTICS456",
		Timeout:               mustDurationValue(t, "90s"),
		TimeoutSeconds:        mustDurationValue(t, "120"),
		UploadTimeout:         mustDurationValue(t, "60s"),
		UploadTimeoutSeconds:  mustDurationValue(t, "180"),
		MaxRetries:            "5",
		BaseDelay:             "2s",
		MaxDelay:              "45s",
		RetryLog:              "1",
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.KeyID != cfg.KeyID {
		t.Fatalf("KeyID mismatch: got %q want %q", loaded.KeyID, cfg.KeyID)
	}
	if loaded.IssuerID != cfg.IssuerID {
		t.Fatalf("IssuerID mismatch: got %q want %q", loaded.IssuerID, cfg.IssuerID)
	}
	if loaded.PrivateKeyPath != cfg.PrivateKeyPath {
		t.Fatalf("PrivateKeyPath mismatch: got %q want %q", loaded.PrivateKeyPath, cfg.PrivateKeyPath)
	}
	if loaded.DefaultKeyName != cfg.DefaultKeyName {
		t.Fatalf("DefaultKeyName mismatch: got %q want %q", loaded.DefaultKeyName, cfg.DefaultKeyName)
	}
	if loaded.AppID != cfg.AppID {
		t.Fatalf("AppID mismatch: got %q want %q", loaded.AppID, cfg.AppID)
	}
	if loaded.VendorNumber != cfg.VendorNumber {
		t.Fatalf("VendorNumber mismatch: got %q want %q", loaded.VendorNumber, cfg.VendorNumber)
	}
	if loaded.AnalyticsVendorNumber != cfg.AnalyticsVendorNumber {
		t.Fatalf("AnalyticsVendorNumber mismatch: got %q want %q", loaded.AnalyticsVendorNumber, cfg.AnalyticsVendorNumber)
	}
	if loaded.Timeout.String() != cfg.Timeout.String() {
		t.Fatalf("Timeout mismatch: got %q want %q", loaded.Timeout.String(), cfg.Timeout.String())
	}
	if loaded.TimeoutSeconds.String() != cfg.TimeoutSeconds.String() {
		t.Fatalf("TimeoutSeconds mismatch: got %q want %q", loaded.TimeoutSeconds.String(), cfg.TimeoutSeconds.String())
	}
	if loaded.UploadTimeout.String() != cfg.UploadTimeout.String() {
		t.Fatalf("UploadTimeout mismatch: got %q want %q", loaded.UploadTimeout.String(), cfg.UploadTimeout.String())
	}
	if loaded.UploadTimeoutSeconds.String() != cfg.UploadTimeoutSeconds.String() {
		t.Fatalf("UploadTimeoutSeconds mismatch: got %q want %q", loaded.UploadTimeoutSeconds.String(), cfg.UploadTimeoutSeconds.String())
	}
	if loaded.MaxRetries != cfg.MaxRetries {
		t.Fatalf("MaxRetries mismatch: got %q want %q", loaded.MaxRetries, cfg.MaxRetries)
	}
	if loaded.BaseDelay != cfg.BaseDelay {
		t.Fatalf("BaseDelay mismatch: got %q want %q", loaded.BaseDelay, cfg.BaseDelay)
	}
	if loaded.MaxDelay != cfg.MaxDelay {
		t.Fatalf("MaxDelay mismatch: got %q want %q", loaded.MaxDelay, cfg.MaxDelay)
	}
	if loaded.RetryLog != cfg.RetryLog {
		t.Fatalf("RetryLog mismatch: got %q want %q", loaded.RetryLog, cfg.RetryLog)
	}

	if err := Remove(); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := Load(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after Remove(), got %v", err)
	}
}

func TestSaveAtOverwritesExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.json")

	initial := &Config{KeyID: "OLD_KEY"}
	if err := SaveAt(path, initial); err != nil {
		t.Fatalf("SaveAt(initial) error: %v", err)
	}

	updated := &Config{KeyID: "NEW_KEY"}
	if err := SaveAt(path, updated); err != nil {
		t.Fatalf("SaveAt(updated) error: %v", err)
	}

	loaded, err := LoadAt(path)
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	if loaded.KeyID != "NEW_KEY" {
		t.Fatalf("expected updated key id, got %q", loaded.KeyID)
	}
}

func TestSaveAtRestoresSecurePermissionsOnOverwrite(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.json")

	if err := SaveAt(path, &Config{KeyID: "OLD_KEY"}); err != nil {
		t.Fatalf("SaveAt(initial) error: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}

	if err := SaveAt(path, &Config{KeyID: "NEW_KEY"}); err != nil {
		t.Fatalf("SaveAt(updated) error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected permissions 0600, got %#o", got)
	}
}

func TestSaveAtRejectsSymlinkPath(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "target.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error: %v", err)
	}

	link := filepath.Join(tempDir, "config.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	err := SaveAt(link, &Config{KeyID: "KEY123"})
	if err == nil {
		t.Fatal("expected symlink write to fail, got nil")
	}
}

func TestSaveAtRejectsSymlinkParentDirectory(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "actual")
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(targetDir) error: %v", err)
	}

	linkDir := filepath.Join(tempDir, "linked")
	if err := os.Symlink(targetDir, linkDir); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	path := filepath.Join(linkDir, "config.json")
	err := SaveAt(path, &Config{KeyID: "KEY123"})
	if err == nil {
		t.Fatal("expected symlink parent write to fail, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(targetDir, "config.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected target config file to remain absent, got %v", statErr)
	}
}

func TestSaveAtRejectsSymlinkAncestorDirectory(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "actual")
	nestedDir := filepath.Join(targetDir, "nested")
	if err := os.MkdirAll(nestedDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(nestedDir) error: %v", err)
	}

	linkDir := filepath.Join(tempDir, "linked")
	if err := os.Symlink(targetDir, linkDir); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	path := filepath.Join(linkDir, "nested", "config.json")
	err := SaveAt(path, &Config{KeyID: "KEY123"})
	if err == nil {
		t.Fatal("expected symlink ancestor write to fail, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(nestedDir, "config.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected nested target config file to remain absent, got %v", statErr)
	}
}

func TestLoadMissingConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))

	if _, err := Load(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing config, got %v", err)
	}
}

func TestGlobalPath(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	path, err := GlobalPath()
	if err != nil {
		t.Fatalf("GlobalPath() error: %v", err)
	}

	expected := filepath.Join(tempDir, ".asc", "config.json")
	if path != expected {
		t.Fatalf("GlobalPath() mismatch: got %q want %q", path, expected)
	}
}

func TestPathEnvOverride(t *testing.T) {
	tempDir := t.TempDir()
	override := filepath.Join(tempDir, "nested", "..", "config.json")
	t.Setenv("ASC_CONFIG_PATH", override)

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}

	expected := filepath.Clean(override)
	if path != expected {
		t.Fatalf("Path() mismatch: got %q want %q", path, expected)
	}
}

func TestPathEnvOverrideRequiresAbsolutePath(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", "config.json")

	_, err := Path()
	if err == nil {
		t.Fatal("expected error for relative ASC_CONFIG_PATH, got nil")
	}
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected ErrInvalidPath, got %v", err)
	}
}

func TestPathUsesLocalConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("HOME", t.TempDir())
	resolvedTempDir, err := filepath.EvalSymlinks(tempDir)
	if err != nil {
		resolvedTempDir = tempDir
	}

	localDir := filepath.Join(tempDir, ".asc")
	if err := os.MkdirAll(localDir, 0o700); err != nil {
		t.Fatalf("mkdir .asc: %v", err)
	}
	localPath := filepath.Join(localDir, "config.json")
	if err := os.WriteFile(localPath, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write local config: %v", err)
	}

	subdir := filepath.Join(tempDir, "nested")
	if err := os.MkdirAll(subdir, 0o700); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	expected := filepath.Join(resolvedTempDir, ".asc", "config.json")
	if path != expected {
		t.Fatalf("Path() mismatch: got %q want %q", path, expected)
	}
}

func TestPathFallsBackToGlobal(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("HOME", t.TempDir())

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	expected, err := GlobalPath()
	if err != nil {
		t.Fatalf("GlobalPath() error: %v", err)
	}
	if path != expected {
		t.Fatalf("Path() mismatch: got %q want %q", path, expected)
	}
}

func TestLocalPathUsesRepoRoot(t *testing.T) {
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.MkdirAll(gitDir, 0o700); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	resolvedTempDir, err := filepath.EvalSymlinks(tempDir)
	if err != nil {
		resolvedTempDir = tempDir
	}

	subdir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subdir, 0o700); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	path, err := LocalPath()
	if err != nil {
		t.Fatalf("LocalPath() error: %v", err)
	}

	expected := filepath.Join(resolvedTempDir, ".asc", "config.json")
	if path != expected {
		t.Fatalf("LocalPath() mismatch: got %q want %q", path, expected)
	}
}

func TestLocalPathFallsBackToCwd(t *testing.T) {
	tempDir := t.TempDir()
	resolvedTempDir, err := filepath.EvalSymlinks(tempDir)
	if err != nil {
		resolvedTempDir = tempDir
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	path, err := LocalPath()
	if err != nil {
		t.Fatalf("LocalPath() error: %v", err)
	}

	expected := filepath.Join(resolvedTempDir, ".asc", "config.json")
	if path != expected {
		t.Fatalf("LocalPath() mismatch: got %q want %q", path, expected)
	}
}

func TestLoadAtRejectsInvalidTimeout(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.json")
	cfg := &Config{
		Timeout: DurationValue{Raw: "not-a-duration"},
	}
	if err := SaveAt(path, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	_, err := LoadAt(path)
	if err == nil {
		t.Fatal("expected error for invalid timeout, got nil")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestLoadAtRejectsMaxRetriesOutOfRange(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.json")
	cfg := &Config{
		MaxRetries: "31",
	}
	if err := SaveAt(path, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	_, err := LoadAt(path)
	if err == nil {
		t.Fatal("expected error for max retries out of range, got nil")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestLoadAtRejectsMaxDelayBelowBaseDelay(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.json")
	cfg := &Config{
		BaseDelay: "10s",
		MaxDelay:  "1s",
	}
	if err := SaveAt(path, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	_, err := LoadAt(path)
	if err == nil {
		t.Fatal("expected error for max delay below base delay, got nil")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}
