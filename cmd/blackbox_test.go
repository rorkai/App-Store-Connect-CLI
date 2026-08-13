package cmd

import (
	"bytes"
	"errors"
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

func TestUnknownInputRecoveryWithBuiltBinary(t *testing.T) {
	binaryPath := buildASCBlackboxBinary(t)
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "unknown command",
			args: []string{"builds", "lsit"},
			wantStderr: "Error: unknown command `asc builds lsit`\n" +
				"Try:\n" +
				"  asc builds list\n" +
				"For help:\n" +
				"  asc builds --help\n",
		},
		{
			name: "unknown flag",
			args: []string{"builds", "list", "--ap", "PRIVATE_VALUE"},
			wantStderr: "Error: unknown flag `--ap` for `asc builds list`\n" +
				"Try:\n" +
				"  --app\n" +
				"For help:\n" +
				"  asc builds list --help\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command(binaryPath, test.args...)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != ExitUsage {
				t.Fatalf("exit error = %v, want exit %d", err, ExitUsage)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if stderr.String() != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.wantStderr)
			}
			if bytes.Contains(stderr.Bytes(), []byte("PRIVATE_VALUE")) {
				t.Fatalf("stderr leaked a following argument: %q", stderr.String())
			}
		})
	}
}
