package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

var testTempDir string

// testTempDirVariable names the folder of the test binary that started this
// one. Helper processes re-run the binary and exit from inside a test, which
// skips the cleanup at the end of TestMain, so their folder goes inside it.
const testTempDirVariable = "ASC_CMD_TEST_TEMP_DIR"

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp(os.Getenv(testTempDirVariable), "asc-cmd-*")
	if err != nil {
		panic(err)
	}
	testTempDir = tempDir
	_ = os.Setenv(testTempDirVariable, tempDir)

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
