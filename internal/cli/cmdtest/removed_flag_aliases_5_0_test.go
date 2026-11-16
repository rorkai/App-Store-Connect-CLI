package cmdtest

import (
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestRemovedFlagAliasesAreUnknownFlags locks the 5.0.0 removal of the
// compatibility spellings that 4.x accepted behind a deprecation warning. Each
// removed alias now fails with migration guidance before any HTTP request, and
// the canonical spelling stays registered. TestRemovedFlagAliasesNameTheirReplacement
// pins the full diagnostic for a representative set.
func TestRemovedFlagAliasesAreUnknownFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	tests := []struct {
		path      []string
		alias     string
		canonical string
		args      []string
		deps      bool // the command is built on shared.Deps: run it on test deps
	}{
		{path: []string{"versions", "attach-build"}, alias: "build", canonical: "build-id", args: []string{"--version-id", "version-1", "--build", "build-1"}, deps: true},
	}

	asctest.NewRegistry(t).Install()

	root := RootCommand("1.2.3")
	for _, test := range tests {
		commandPath := strings.Join(test.path, " ")
		t.Run(commandPath+" --"+test.alias, func(t *testing.T) {
			command := findSubcommand(root, test.path...)
			if command == nil {
				t.Fatalf("command %q not found", commandPath)
			}
			if command.FlagSet.Lookup(test.canonical) == nil {
				t.Fatalf("canonical flag --%s not found on %q", test.canonical, commandPath)
			}

			args := append(append([]string{}, test.path...), test.args...)
			wantErr := "Error: `--" + test.alias + "` was removed in 5.0.0; use `--" + test.canonical + "`"
			if test.deps {
				assertUsageExitWithDeps(t, nil, args, wantErr)
				return
			}
			assertUsageExit(t, args, wantErr)
		})
	}
}
