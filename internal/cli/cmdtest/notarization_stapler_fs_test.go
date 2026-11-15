package cmdtest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	notarizationcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/notarization"
)

func TestNotarizationTargetFilesystemFailuresAreRuntime(t *testing.T) {
	artifactPath := filepath.Join(t.TempDir(), "private-op-path-marker.dmg")
	rawFilesystemError := errors.New("permission denied: " + artifactPath)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "staple",
			args: []string{"notarization", "staple", "--file", artifactPath, "--confirm"},
		},
		{
			name: "validate",
			args: []string{"notarization", "validate", "--file", artifactPath},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restore := notarizationcli.SetValidateStaplerTargetForTesting(func(string) (string, error) {
				return "", rawFilesystemError
			})
			t.Cleanup(restore)
			stdout, stderr, code := runWithDeps(t, nil, test.args...)
			if code != rootcmd.ExitError {
				t.Fatalf("Run() exit code = %d, want runtime exit %d", code, rootcmd.ExitError)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if strings.Contains(stderr, artifactPath) {
				t.Fatalf("stderr = %q, must not expose artifact path", stderr)
			}
			if strings.Contains(stderr, rawFilesystemError.Error()) {
				t.Fatalf("stderr = %q, must not expose raw filesystem error", stderr)
			}
			if !strings.Contains(stderr, "could not inspect artifact filesystem") {
				t.Fatalf("stderr = %q, want sanitized filesystem diagnostic", stderr)
			}
		})
	}
}

func TestNotarizationUnsearchableLexicalParentIsRuntimeAndSkipsChild(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatalf("create blocked parent: %v", err)
	}
	target := filepath.Join(root, "App.dmg")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	pathValue := blocked + string(filepath.Separator) + ".." + string(filepath.Separator) + "App.dmg"

	calls := 0
	restore := notarizationcli.SetOpenStaplerLexicalDirectoryForTesting(func(path string) (*os.File, error) {
		calls++
		if filepath.Clean(path) != blocked {
			t.Fatalf("lexical directory path = %q, want %q", path, blocked)
		}
		return nil, syscall.EACCES
	})
	t.Cleanup(restore)
	stdout, stderr, code := runWithDeps(t, nil, "notarization", "validate", "--file", pathValue)
	if code != rootcmd.ExitError {
		t.Fatalf("Run() exit code = %d, want runtime exit %d", code, rootcmd.ExitError)
	}
	if calls != 1 {
		t.Fatalf("lexical directory opens = %d, want one pre-clean searchability check", calls)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "could not inspect artifact filesystem") {
		t.Fatalf("stderr = %q, want sanitized filesystem diagnostic", stderr)
	}
	if strings.Contains(stderr, pathValue) || strings.Contains(stderr, target) {
		t.Fatalf("stderr = %q, must not expose artifact path", stderr)
	}
}

func TestNotarizationTargetShapeFailuresRemainUsage(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "artifact.zip")
	if err := os.WriteFile(zipPath, []byte("fixture"), 0o600); err != nil {
		t.Fatalf("write zip fixture: %v", err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "staple zip",
			args: []string{"notarization", "staple", "--file", zipPath, "--confirm"},
		},
		{
			name: "validate zip",
			args: []string{"notarization", "validate", "--file", zipPath},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, code := runWithDeps(t, nil, test.args...)
			if code != rootcmd.ExitUsage {
				t.Fatalf("Run() exit code = %d, want usage exit %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "cannot be stapled or validated directly") {
				t.Fatalf("stderr = %q, want semantic target diagnostic", stderr)
			}
		})
	}
}
