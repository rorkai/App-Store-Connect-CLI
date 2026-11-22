package cmd

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

// agentCallsPath holds the command shapes coding agents ran against asc in
// production, with every value replaced by X. Each line is "<status> <args>":
//
//   - "ok": the shape selects a command and every flag on it. It must keep
//     doing so; a rename or removal that breaks it fails here first.
//   - "unknown-command" or "unknown-flag": a shape asc does not accept yet.
//     When a change makes it run, this test fails until the line says "ok",
//     so the file always records what agents still get wrong.
//
// Regenerate the statuses with ASC_UPDATE_AGENT_CALLS=1 after a deliberate
// change, and review the diff.
const agentCallsPath = "testdata/agent_calls.txt"

func TestAgentCallShapesKeepResolving(t *testing.T) {
	file, err := os.Open(filepath.FromSlash(agentCallsPath))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	update := os.Getenv("ASC_UPDATE_AGENT_CALLS") == "1"
	root := RootCommand("1.0.0")
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			lines = append(lines, line)
			continue
		}
		want, shape, _ := strings.Cut(line, " ")
		got := agentCallStatus(root, strings.Fields(shape))
		if update {
			lines = append(lines, got+" "+shape)
			continue
		}
		if got != want {
			t.Errorf("asc %s: %s, recorded as %s", shape, got, want)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if update {
		if err := os.WriteFile(filepath.FromSlash(agentCallsPath), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// agentCallStatus reports whether args select a command and all of its flags
// after the same alias rewrites Run applies.
func agentCallStatus(root *ffcli.Command, args []string) string {
	args = resolveCommandAliases(root, args)
	if resolved, err := resolveFlagAliases(root, args, nil); err == nil {
		args = resolved
	}
	analysis := analyzeInvocation(root, args)
	switch {
	case analysis.unknownFlag:
		return "unknown-flag"
	case analysis.shape == telemetry.InvocationShapeUnknownChild:
		return "unknown-command"
	default:
		return "ok"
	}
}
