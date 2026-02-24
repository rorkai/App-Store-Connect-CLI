package completion

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// CompletionCommand prints shell completion scripts to stdout.
// It is intentionally simple and does not require auth or network access.
func CompletionCommand(rootSubcommands []*ffcli.Command) *ffcli.Command {
	fs := flag.NewFlagSet("completion", flag.ExitOnError)
	shell := fs.String("shell", "", "Shell: bash, zsh, or fish")

	cmd := &ffcli.Command{
		Name:       "completion",
		ShortUsage: "asc completion --shell <bash|zsh|fish>",
		ShortHelp:  "Print shell completion scripts.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
	}

	cmd.Exec = func(ctx context.Context, args []string) error {
		_ = ctx
		_ = args

		s := strings.ToLower(strings.TrimSpace(*shell))
		if s == "" {
			fmt.Fprintln(os.Stderr, "Error: --shell is required")
			return flag.ErrHelp
		}

		names := rootCommandNames(rootSubcommands)
		switch s {
		case "bash":
			_, _ = fmt.Fprint(os.Stdout, bashScript(names))
			return nil
		case "zsh":
			_, _ = fmt.Fprint(os.Stdout, zshScript(names))
			return nil
		case "fish":
			_, _ = fmt.Fprint(os.Stdout, fishScript(names))
			return nil
		default:
			_, _ = fmt.Fprintf(os.Stderr, "Error: unsupported shell: %s\n", shared.SanitizeTerminal(s))
			return flag.ErrHelp
		}
	}

	return cmd
}

func rootCommandNames(rootSubcommands []*ffcli.Command) []string {
	set := make(map[string]struct{}, len(rootSubcommands)+1)
	for _, c := range rootSubcommands {
		if c == nil {
			continue
		}
		name := strings.TrimSpace(c.Name)
		if name == "" {
			continue
		}
		set[name] = struct{}{}
	}

	// Ensure the completion command can complete itself even if the slice passed
	// doesn't include it (by design).
	set["completion"] = struct{}{}

	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func bashScript(subcommands []string) string {
	words := strings.Join(subcommands, " ")
	return fmt.Sprintf(`# bash completion for asc
_asc_completions() {
  local cur
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"

  if [[ $COMP_CWORD -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "%s" -- "$cur") )
    return 0
  fi
}

complete -F _asc_completions asc
`, words)
}

func zshScript(subcommands []string) string {
	// zsh _arguments wants a space-separated list inside ((...))
	words := strings.Join(subcommands, " ")
	return fmt.Sprintf(`#compdef asc

_arguments \
  '1:command:(%s)' \
  '*::arg:->args'
`, words)
}

func fishScript(subcommands []string) string {
	words := strings.Join(subcommands, " ")
	return fmt.Sprintf(`# fish completion for asc
complete -c asc -f -a '%s'
`, words)
}
