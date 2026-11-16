package cmdtest

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowRun_RetryKeepsStdoutStructured(t *testing.T) {
	requireWorkflowShell(t)
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "attempts")
	path := writeWorkflowJSON(t, dir, fmt.Sprintf(`{
		"env": {"COUNTER_PATH": %q},
		"workflows": {
			"beta": {
				"steps": [{
					"name": "eventual_build_group",
					"run": "count=$(cat \"$COUNTER_PATH\" 2>/dev/null || echo 0); count=$((count+1)); printf '%%s' \"$count\" > \"$COUNTER_PATH\"; if [ \"$count\" -lt 2 ]; then printf 'transient-404'; exit 44; fi; printf 'linked'",
					"retry": {"max_attempts": 2, "delay": "1ms"},
					"timeout": "2s"
				}]
			}
		}
	}`, counterPath))

	stdout, stderr, err := runRootWithDeps(t, nil, "workflow", "run", "--file", path, "beta")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var result struct {
		Status string `json:"status"`
		Steps  []struct {
			Status   string `json:"status"`
			Attempts []struct {
				Attempt int    `json:"attempt"`
				Status  string `json:"status"`
			} `json:"attempts"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout must be one JSON result, got %q: %v", stdout, err)
	}
	if result.Status != "ok" || len(result.Steps) != 1 || result.Steps[0].Status != "ok" {
		t.Fatalf("unexpected workflow result: %+v", result)
	}
	if len(result.Steps[0].Attempts) != 2 {
		t.Fatalf("attempts = %+v, want two attempts", result.Steps[0].Attempts)
	}
	if result.Steps[0].Attempts[0].Status != "error" || result.Steps[0].Attempts[1].Status != "ok" {
		t.Fatalf("attempt statuses = %+v, want error then ok", result.Steps[0].Attempts)
	}
	if !strings.Contains(stderr, "attempt 1/2") || !strings.Contains(stderr, "retrying in 1ms") {
		t.Fatalf("stderr must expose retry attempt and delay, got %q", stderr)
	}
	if !strings.Contains(stderr, "transient-404") || !strings.Contains(stderr, "linked") {
		t.Fatalf("step output must stream to stderr, got %q", stderr)
	}
}

func requireWorkflowShell(t *testing.T) {
	t.Helper()
	for _, shell := range []string{"bash", "sh"} {
		if _, err := exec.LookPath(shell); err == nil {
			return
		}
	}
	t.Skip("asc workflow run requires bash or sh in PATH")
}
