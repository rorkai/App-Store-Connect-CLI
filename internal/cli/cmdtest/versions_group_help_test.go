package cmdtest

import (
	"io"
	"strings"
	"testing"

	"github.com/kballard/go-shellquote"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func versionsGroupHelpText(t *testing.T) string {
	t.Helper()

	var help string
	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"versions", "--help"}, "1.2.3"); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})
	help = stdout + stderr
	if !strings.Contains(help, "SUBCOMMANDS") {
		t.Fatalf("expected group help, got %q", help)
	}
	return help
}

func TestVersionsGroupHelpDocumentsCoreWorkflows(t *testing.T) {
	help := versionsGroupHelpText(t)

	if !strings.Contains(help, "Examples:") {
		t.Fatalf("expected an Examples block in `asc versions --help`, got %q", help)
	}

	wantSubcommands := []string{
		"asc versions list ",
		"asc versions view ",
		"asc versions create ",
		"asc versions update ",
		"asc versions attach-build ",
		"asc versions release ",
		"asc versions phased-release ",
		"asc versions rating-reset ",
	}
	for _, want := range wantSubcommands {
		if !strings.Contains(help, want) {
			t.Fatalf("expected example for %q in group help, got %q", strings.TrimSpace(want), help)
		}
	}
}

func TestVersionsGroupHelpExamplesParseAgainstCurrentCLI(t *testing.T) {
	help := versionsGroupHelpText(t)

	examples := make([]string, 0, 8)
	for _, line := range strings.Split(help, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "asc versions ") {
			continue
		}
		// Skip the USAGE placeholder line rather than the concrete examples.
		if strings.Contains(trimmed, "<") {
			continue
		}
		examples = append(examples, trimmed)
	}
	if len(examples) < 5 {
		t.Fatalf("expected example invocations in group help, got %d in %q", len(examples), help)
	}

	for _, example := range examples {
		t.Run(example, func(t *testing.T) {
			args, err := shellquote.Split(example)
			if err != nil {
				t.Fatalf("split %q: %v", example, err)
			}
			for _, arg := range args {
				if strings.HasPrefix(arg, "-") && arg != "-" && !strings.HasPrefix(arg, "--") {
					t.Fatalf("example %q uses short-form flag %q", example, arg)
				}
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			if err := root.Parse(args[1:]); err != nil {
				t.Fatalf("parse %q: %v", example, err)
			}
		})
	}
}

func TestVersionsGroupHelpListsEverySubcommand(t *testing.T) {
	help := versionsGroupHelpText(t)

	wantSubcommands := []string{
		"list",
		"view",
		"links",
		"experiments-v2",
		"customer-reviews",
		"app-clip-default-experience",
		"create",
		"update",
		"delete",
		"attach-build",
		"release",
		"phased-release",
		"rating-reset",
		"promotions",
	}

	subcommandsBlock := help
	if index := strings.Index(help, "SUBCOMMANDS"); index >= 0 {
		subcommandsBlock = help[index:]
	}
	for _, name := range wantSubcommands {
		if !strings.Contains(subcommandsBlock, "\n  "+name+" ") {
			t.Fatalf("expected subcommand %q listed in group help, got %q", name, subcommandsBlock)
		}
	}
}
