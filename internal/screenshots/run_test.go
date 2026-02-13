package screenshots

import (
	"context"
	"os"
	"path/filepath"
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
