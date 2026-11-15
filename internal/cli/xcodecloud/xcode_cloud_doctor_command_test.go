package xcodecloud

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestXcodeCloudDoctorValidatesOutputBeforeSaveSideEffects(t *testing.T) {
	saveDirectory := filepath.Join(t.TempDir(), "logs")
	command := newXcodeCloudDoctorCommand(&shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}})
	command.FlagSet.SetOutput(io.Discard)
	if err := command.Parse([]string{
		"--run-id", "run-1",
		"--save-logs", saveDirectory,
		"--output", "table",
		"--pretty",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := command.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--pretty") {
		t.Fatalf("run error = %v, want invalid output combination", err)
	}
	if _, statErr := os.Stat(saveDirectory); !os.IsNotExist(statErr) {
		t.Fatalf("save directory stat error = %v, want no filesystem side effect", statErr)
	}
}

func TestXcodeCloudRunValidatesOutputBeforeTriggerSideEffects(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	command := newXcodeCloudRunCommand(&shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}})
	command.FlagSet.SetOutput(io.Discard)
	if err := command.Parse([]string{
		"--workflow-id", "workflow-1",
		"--git-reference-id", "reference-1",
		"--output", "table",
		"--pretty",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := command.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--pretty") {
		t.Fatalf("run error = %v, want invalid output combination before authentication or trigger", err)
	}
}
