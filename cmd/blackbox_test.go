package cmd

import (
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var (
	ascBlackboxBinaryOnce   sync.Once
	ascBlackboxBinaryPath   string
	ascBlackboxBinaryOutput []byte
	ascBlackboxBinaryErr    error
)

func buildASCBlackboxBinary(t *testing.T) string {
	t.Helper()

	ascBlackboxBinaryOnce.Do(func() {
		ascBlackboxBinaryPath = filepath.Join(testTempDir, "asc-test")

		build := exec.Command("go", "build", "-o", ascBlackboxBinaryPath, ".")
		build.Dir = ".."
		ascBlackboxBinaryOutput, ascBlackboxBinaryErr = build.CombinedOutput()
	})
	if ascBlackboxBinaryErr != nil {
		t.Fatalf("failed to build binary: %v\n%s", ascBlackboxBinaryErr, ascBlackboxBinaryOutput)
	}

	return ascBlackboxBinaryPath
}
