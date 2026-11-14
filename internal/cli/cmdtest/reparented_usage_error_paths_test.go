package cmdtest

import (
	"flag"
	"regexp"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// Usage errors print while the command runs, before the re-parenting of
// these trees can rename the command path, so each leaf here must build
// them with the path that was typed.
func TestReparentedCommandUsageErrorsNameTheTypedPath(t *testing.T) {
	setupUsageExitCodeEnv(t)
	t.Chdir(t.TempDir())
	commandPrefix := regexp.MustCompile(`^Error: ([a-z0-9][a-z0-9-]*(?: [a-z0-9][a-z0-9-]*)+): `)

	var walk func(cmd *ffcli.Command, path []string)
	walk = func(cmd *ffcli.Command, path []string) {
		if len(cmd.Subcommands) > 0 {
			for _, sub := range cmd.Subcommands {
				walk(sub, append(append([]string{}, path...), sub.Name))
			}
			return
		}
		var inputs [][]string
		cmd.FlagSet.VisitAll(func(f *flag.Flag) {
			getter, ok := f.Value.(flag.Getter)
			if !ok {
				return
			}
			if _, isString := getter.Get().(string); !isString {
				return
			}
			inputs = append(inputs, []string{"--" + f.Name, ""})
			if f.Name == "next" {
				inputs = append(inputs, []string{"--next", "bad"})
			}
		})
		typed := strings.Join(path, " ")
		for _, input := range inputs {
			var code int
			_, stderr := captureOutput(t, func() {
				code = rootcmd.Run(append(append([]string{}, path...), input...), "1.2.3")
			})
			if code != rootcmd.ExitUsage {
				continue
			}
			firstLine, _, _ := strings.Cut(stderr, "\n")
			match := commandPrefix.FindStringSubmatch(firstLine)
			if match == nil {
				continue
			}
			if !strings.HasSuffix(typed, match[1]) {
				t.Errorf("asc %s %s: stderr first line = %q, want it to name %q", typed, strings.Join(input, " "), firstLine, typed)
			}
			return
		}
	}

	root := RootCommand("1.2.3")
	for _, tree := range [][]string{{"review", "submissions"}, {"subscriptions"}, {"testflight"}} {
		walk(findCommand(root, tree...), tree)
	}
}
