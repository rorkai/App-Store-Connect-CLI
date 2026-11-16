package cmdtest

import (
	"regexp"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// resumeExamplePattern matches a documented `asc workflow run <name> --resume <run-id>`
// invocation so the example can be checked against the run-ID format.
var resumeExamplePattern = regexp.MustCompile(`asc workflow run (?:--file \S+ )?([A-Za-z0-9_-]+) --resume (\S+)`)

// TestWorkflowHelp_ResumeExamplesUseTheirOwnRunID guards a copy-paste trap: run
// IDs are generated as <workflow-name>-<timestamp>-<hex> and the runner rejects
// a resume whose stored workflow name differs from the requested one, so
// `asc workflow run release --resume beta-...` can never succeed.
func TestWorkflowHelp_ResumeExamplesUseTheirOwnRunID(t *testing.T) {
	for _, args := range [][]string{
		{"workflow", "--help"},
		{"workflow", "run", "--help"},
	} {
		stdout, _, code := runWithDeps(t, nil, args...)
		if code != rootcmd.ExitSuccess {
			t.Fatalf("%v: exit code = %d, want %d", args, code, rootcmd.ExitSuccess)
		}

		matches := resumeExamplePattern.FindAllStringSubmatch(stdout, -1)
		if len(matches) == 0 {
			t.Fatalf("%v: expected help to document a --resume example, got %q", args, stdout)
		}
		for _, match := range matches {
			workflowName, runID := match[1], match[2]
			if !strings.HasPrefix(runID, workflowName+"-") {
				t.Errorf("%v: example %q resumes run %q, which belongs to workflow %q and is always rejected",
					args, match[0], runID, strings.SplitN(runID, "-", 2)[0])
			}
		}
	}
}
