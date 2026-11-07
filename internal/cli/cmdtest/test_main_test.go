package cmdtest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/uploadlock"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var testConfigPath string

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "asc-cmdtest-*")
	if err != nil {
		panic(err)
	}
	testConfigPath = filepath.Join(tempDir, "config.json")
	testStdin, err := os.Open(os.DevNull)
	if err != nil {
		panic(err)
	}
	originalStdin := os.Stdin
	os.Stdin = testStdin
	restoreControllingTTY := webcli.DisableControllingTTYForTesting()
	// The core web package tests the default and rate-limit behavior.
	restorePacing := webcore.DisableRequestPacingForTesting()
	restoreAssetLibraryPoll := asc.SetMediaPollIntervalsForTest(time.Millisecond, time.Millisecond)
	// The upload lock lives under the OS account's home, not HOME.
	restoreUploadLockDir := uploadlock.SetDirForTest(filepath.Join(tempDir, "upload-lock"))

	_ = os.Setenv("ASC_CONFIG_PATH", testConfigPath)
	_ = os.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	_ = os.Setenv("ASC_MAX_RETRIES", "0")
	_ = os.Setenv("ASC_TELEMETRY_DISABLED", "1")
	_ = os.Setenv("HOME", tempDir)
	// The Apple ID environment fallback for "asc web" commands is a
	// process-wide input: a developer or CI host that exports it would
	// otherwise select an account for every session-resolving command under
	// test. Tests that want it set it themselves.
	_ = os.Unsetenv(webAppleIDEnvNameForTest())

	code := m.Run()

	restoreUploadLockDir()
	restoreAssetLibraryPoll()
	restorePacing()
	restoreControllingTTY()
	os.Stdin = originalStdin
	_ = testStdin.Close()
	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}
