package auth

import (
	"context"
	"path/filepath"
	"testing"

	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
)

func TestAuthExportToConfigPreservesExactPathBytes(t *testing.T) {
	d := newAuthTestDeps()
	base := t.TempDir()
	spacedConfig := filepath.Join(base, " config.json ")
	spacedKeyDir := filepath.Join(base, " keys ")
	var captured authsvc.MigrateKeychainToConfigOptions
	restore := SetMigrateKeychainToConfig(func(opts authsvc.MigrateKeychainToConfigOptions) (authsvc.MigrateKeychainToConfigResult, error) {
		captured = opts
		return authsvc.MigrateKeychainToConfigResult{ConfigPath: opts.ConfigPath}, nil
	})
	t.Cleanup(restore)

	cmd := newAuthExportToConfigCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--confirm",
		"--output", "json",
		"--config", spacedConfig,
		"--private-key-dir", spacedKeyDir,
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	if captured.ConfigPath != spacedConfig {
		t.Fatalf("ConfigPath = %q, want exact path %q", captured.ConfigPath, spacedConfig)
	}
	if captured.PrivateKeyDir != spacedKeyDir {
		t.Fatalf("PrivateKeyDir = %q, want exact path %q", captured.PrivateKeyDir, spacedKeyDir)
	}
}

func TestAuthExportToConfigTreatsAllSpaceNamesAsPaths(t *testing.T) {
	d := newAuthTestDeps()
	var captured authsvc.MigrateKeychainToConfigOptions
	restore := SetMigrateKeychainToConfig(func(opts authsvc.MigrateKeychainToConfigOptions) (authsvc.MigrateKeychainToConfigResult, error) {
		captured = opts
		return authsvc.MigrateKeychainToConfigResult{ConfigPath: opts.ConfigPath}, nil
	})
	t.Cleanup(restore)

	cmd := newAuthExportToConfigCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--confirm",
		"--output", "json",
		"--config", "   ",
		"--private-key-dir", "   ",
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	want, err := filepath.Abs("   ")
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	if captured.ConfigPath != want {
		t.Fatalf("ConfigPath = %q, want all-space path %q", captured.ConfigPath, want)
	}
	if captured.PrivateKeyDir != "   " {
		t.Fatalf("PrivateKeyDir = %q, want exact all-space value", captured.PrivateKeyDir)
	}
}
