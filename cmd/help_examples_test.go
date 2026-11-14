package cmd

import (
	"strings"
	"testing"

	"github.com/kballard/go-shellquote"
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// Help examples are copied by agents verbatim, so every concrete example must
// at least resolve to a real command and parse with the CLI's own argv rules.
func TestHelpExamplesParseAgainstCurrentCLI(t *testing.T) {
	var examples []string
	var collect func(*ffcli.Command)
	collect = func(command *ffcli.Command) {
		lines := []string{command.ShortUsage}
		pending := ""
		for _, line := range strings.Split(command.LongHelp, "\n") {
			// Examples are indented; unindented `asc ...` lines are wrapped prose.
			if pending == "" && !strings.HasPrefix(line, "  asc ") {
				continue
			}
			pending += strings.TrimSpace(line)
			if strings.HasSuffix(pending, "\\") {
				pending = strings.TrimSuffix(pending, "\\")
				continue
			}
			lines = append(lines, pending)
			pending = ""
		}
		for _, line := range lines {
			// Skip placeholder syntax and shell composition.
			if strings.HasPrefix(line, "asc ") && !strings.ContainsAny(line, "<[|$`") && !strings.Contains(line, "...") {
				examples = append(examples, line)
			}
		}
		for _, subcommand := range command.Subcommands {
			collect(subcommand)
		}
	}
	collect(RootCommand("dev"))
	if len(examples) < 1000 {
		t.Fatalf("collected %d help examples, want at least 1000", len(examples))
	}

	for _, example := range examples {
		fields, err := shellquote.Split(example)
		if err != nil {
			t.Errorf("split %q: %v", example, err)
			continue
		}
		args := fields[1:]
		root := rootCommandForArgs("dev", shared.DefaultDeps(), args)
		if _, helpRequested := requestedHelpArgs(root, args); helpRequested {
			continue
		}
		args = normalizeSpacedBooleanFlags(root, args)
		if analysis := analyzeInvocation(root, args); analysis.unknownToken != "" {
			t.Errorf("%q: unknown token %q", example, analysis.unknownToken)
			continue
		}
		restore := prepareFlagParsing(root, args, &parseOutputBuffer{})
		err = root.Parse(args)
		restore()
		if err != nil {
			t.Errorf("%q: %v", example, err)
		}
	}
}
