package screenshots

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAxeMatchesTarget_ParsesJSONWhenStderrHasWarnings(t *testing.T) {
	binDir := t.TempDir()
	axePath := filepath.Join(binDir, "axe")
	script := `#!/bin/sh
if [ "$1" = "describe-ui" ]; then
  echo "warning: test noise" 1>&2
  echo '{"AXLabel":"Ready"}'
  exit 0
fi
exit 1
`
	if err := os.WriteFile(axePath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", axePath, err)
	}
	t.Setenv("PATH", binDir)

	label := "Ready"
	matched, err := axeMatchesTarget(context.Background(), "booted", PlanStep{Label: &label})
	if err != nil {
		t.Fatalf("axeMatchesTarget() error = %v", err)
	}
	if !matched {
		t.Fatal("expected matcher to find AXLabel in describe-ui output")
	}
}

func TestRunExternalOutput_IncludesStderrOnFailure(t *testing.T) {
	binDir := t.TempDir()
	cmdPath := filepath.Join(binDir, "tool")
	script := `#!/bin/sh
echo "stdout text"
echo "stderr text" 1>&2
exit 1
`
	if err := os.WriteFile(cmdPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", cmdPath, err)
	}
	t.Setenv("PATH", binDir)

	_, err := runExternalOutput(context.Background(), "tool")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "stderr text") {
		t.Fatalf("expected stderr output in error, got %v", err)
	}
}
