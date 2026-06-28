package cmd

import (
	"os"
	"testing"
)

var testTempDir string

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "asc-cmd-*")
	if err != nil {
		panic(err)
	}
	testTempDir = tempDir

	_ = os.Setenv("ASC_TELEMETRY_DISABLED", "1")
	code := m.Run()

	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}
