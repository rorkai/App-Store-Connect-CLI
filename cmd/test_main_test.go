package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

var testTempDir string

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "asc-cmd-*")
	if err != nil {
		panic(err)
	}
	testTempDir = tempDir

	// The asc binary that the built-binary tests run is not a test binary, so
	// internal/testguard does not hide the developer's credentials from it.
	_ = os.Setenv("HOME", tempDir)
	_ = os.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
	_ = os.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	_ = os.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	_ = os.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "1")
	_ = os.Setenv("ASC_TELEMETRY_DISABLED", "1")
	// Classification tests assert on terminal HTTP failures, so keep clients on
	// the single-attempt path instead of waiting out retry backoff.
	_ = os.Setenv("ASC_MAX_RETRIES", "0")
	code := m.Run()

	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}
