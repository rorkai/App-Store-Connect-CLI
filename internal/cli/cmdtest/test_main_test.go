package cmdtest

import (
	"os"
	"path/filepath"
	"testing"
)

var testConfigPath string

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "asc-cmdtest-*")
	if err != nil {
		panic(err)
	}
	testConfigPath = filepath.Join(tempDir, "config.json")

	_ = os.Setenv("ASC_CONFIG_PATH", testConfigPath)
	_ = os.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	_ = os.Setenv("ASC_TELEMETRY_DISABLED", "1")
	_ = os.Setenv("HOME", tempDir)

	code := m.Run()

	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}
