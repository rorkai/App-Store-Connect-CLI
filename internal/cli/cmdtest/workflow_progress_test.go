package cmdtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

type workflowProgressEvent struct {
	Event, RunID, StepID, Status string
	DryRun                       bool
}

func readWorkflowProgress(t *testing.T, path string) (string, []workflowProgressEvent) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var events []workflowProgressEvent
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var event workflowProgressEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("invalid progress line %q: %v", line, err)
		}
		events = append(events, event)
	}
	return string(data), events
}

func TestWorkflowRun_ProgressFileRefusesInvalidDestinationBeforeExecution(t *testing.T) {
	for _, name := range []string{"existing", "symlink", "missing parent", "empty"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "executed")
			t.Setenv("PROGRESS_EXECUTED", marker)
			path := writeWorkflowJSON(t, dir, `{"workflows":{"test":{"steps":["printf ran > \"$PROGRESS_EXECUTED\""]}}}`)
			existing := filepath.Join(dir, "existing")
			if err := os.WriteFile(existing, []byte("preserved"), 0o600); err != nil {
				t.Fatal(err)
			}
			progressPath := existing
			switch name {
			case "symlink":
				progressPath = filepath.Join(dir, "link")
				if err := os.Symlink(existing, progressPath); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			case "missing parent":
				progressPath = filepath.Join(dir, "missing", "progress.jsonl")
			case "empty":
				progressPath = ""
			}
			stdout, stderr, code := runWithDeps(t, nil, "workflow", "run", "--progress-file", progressPath, "--file", path, "test")
			if code != rootcmd.ExitUsage || stdout != "" || !strings.Contains(stderr, "--progress-file") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("workflow started: marker stat=%v", err)
			}
			if data, err := os.ReadFile(existing); err != nil || string(data) != "preserved" {
				t.Fatalf("destination changed: %q, %v", data, err)
			}
		})
	}
}
