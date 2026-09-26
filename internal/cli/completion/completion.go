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

type completionNode struct {
	path        string
	subcommands []string
	flags       []string
	valueFlags  []string
}

// CompletionCommand prints shell completion scripts to stdout.
// The full command tree is resolved only after the requested shell is valid,
// so normal command startup remains lazy and completion needs no auth or network
// access.
func CompletionCommand(resolveCommands func() []*ffcli.Command, resolveRootFlags func() *flag.FlagSet) *ffcli.Command {
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
			return shared.MissingRequiredUsageError("--shell")
		}
		if s != "bash" && s != "zsh" && s != "fish" {
			fmt.Fprintf(os.Stderr, "Error: unsupported shell: %s\n", shared.SanitizeTerminal(s))
			return flag.ErrHelp
		}

		var commands []*ffcli.Command
		if resolveCommands != nil {
			commands = resolveCommands()
		}
		var rootFlags *flag.FlagSet
		if resolveRootFlags != nil {
			rootFlags = resolveRootFlags()
		}
		nodes := completionNodes(commands, rootFlags)
		switch s {
		case "bash":
			fmt.Fprint(os.Stdout, bashScript(nodes))
		case "zsh":
			fmt.Fprint(os.Stdout, zshScript(nodes))
		case "fish":
			fmt.Fprint(os.Stdout, fishScript(nodes))
		}
		return nil
	}

	return cmd
}

func completionNodes(rootSubcommands []*ffcli.Command, rootFlags *flag.FlagSet) []completionNode {
	byPath := map[string]completionNode{"": {path: ""}}
	root := byPath[""]
	root.subcommands = visibleSubcommandNames(rootSubcommands)
	root.flags, root.valueFlags = completionFlags(rootFlags)
	byPath[""] = root

	var visit func(parentPath string, commands []*ffcli.Command)
	visit = func(parentPath string, commands []*ffcli.Command) {
		for _, command := range commands {
			if hiddenCommand(command) {
				continue
			}
			name := strings.TrimSpace(command.Name)
			if name == "" {
				continue
			}
			path := name
			if parentPath != "" {
				path = parentPath + " " + name
			}
			node := completionNode{
				path:        path,
				subcommands: visibleSubcommandNames(command.Subcommands),
			}
			node.flags, node.valueFlags = completionFlags(command.FlagSet)
			node.flags, node.valueFlags = withRootProfileFlag(command.FlagSet, node.flags, node.valueFlags)
			byPath[path] = node
			visit(path, command.Subcommands)
		}
	}
	visit("", rootSubcommands)

	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	nodes := make([]completionNode, 0, len(paths))
	for _, path := range paths {
		nodes = append(nodes, byPath[path])
	}
	return nodes
}

func completionFlags(fs *flag.FlagSet) ([]string, []string) {
	var flags []string
	var valueFlags []string
	for _, commandFlag := range shared.VisibleHelpFlags(fs) {
		flagName := "--" + commandFlag.Name
		flags = append(flags, flagName)
		if flagRequiresValue(commandFlag) {
			valueFlags = append(valueFlags, flagName)
		}
	}
	sort.Strings(flags)
	sort.Strings(valueFlags)
	return flags, valueFlags
}

// withRootProfileFlag offers the root-owned credential selector on every
// command, because it is accepted after the command name as well. A command
// that binds its own `profile` flag already lists it.
func withRootProfileFlag(fs *flag.FlagSet, flags, valueFlags []string) ([]string, []string) {
	if fs != nil && fs.Lookup(shared.RootProfileFlagName) != nil {
		return flags, valueFlags
	}

	flagName := "--" + shared.RootProfileFlagName
	flags = append(flags, flagName)
	valueFlags = append(valueFlags, flagName)
	sort.Strings(flags)
	sort.Strings(valueFlags)
	return flags, valueFlags
}

func visibleSubcommandNames(commands []*ffcli.Command) []string {
	set := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		if hiddenCommand(command) {
			continue
		}
		name := strings.TrimSpace(command.Name)
		if name != "" {
			set[name] = struct{}{}
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func hiddenCommand(command *ffcli.Command) bool {
	if command == nil {
		return true
	}
	shortHelp := strings.TrimSpace(command.ShortHelp)
	return strings.HasPrefix(shortHelp, "DEPRECATED:") ||
		strings.HasPrefix(shortHelp, "REMOVED:") ||
		strings.HasPrefix(shortHelp, "Compatibility alias:") ||
		strings.HasPrefix(shortHelp, "Compatibility aliases ")
}

func flagRequiresValue(commandFlag *flag.Flag) bool {
	if commandFlag == nil || commandFlag.Value == nil {
		return false
	}
	boolValue, ok := commandFlag.Value.(interface{ IsBoolFlag() bool })
	return !ok || !boolValue.IsBoolFlag()
}

func bashScript(nodes []completionNode) string {
	var b strings.Builder
	b.WriteString("# bash completion for asc\n")
	writeShellCompletionData(&b, nodes)
	b.WriteString(`
_asc_completion_lookup() {
  _asc_subcommands=()
  _asc_flags=()
  _asc_value_flags=()
  local index
  for ((index = 0; index < ${#_ASC_COMPLETION_PATHS[@]}; index++)); do
    if [[ "${_ASC_COMPLETION_PATHS[index]}" == "$1" ]]; then
      read -r -a _asc_subcommands <<< "${_ASC_COMPLETION_SUBCOMMAND_GROUPS[index]}"
      read -r -a _asc_flags <<< "${_ASC_COMPLETION_FLAG_GROUPS[index]}"
      read -r -a _asc_value_flags <<< "${_ASC_COMPLETION_VALUE_FLAG_GROUPS[index]}"
      return 0
    fi
  done
}

_asc_completion_candidates() {
  local path="" token subcommand value_flag
  local expect_value=0
  local -a _asc_subcommands _asc_flags _asc_value_flags

  for token in "$@"; do
    if [[ $expect_value -eq 1 ]]; then
      expect_value=0
      continue
    fi

    _asc_completion_lookup "$path"
    if [[ "$token" == --*=* ]]; then
      continue
    fi
    if [[ "$token" == --* ]]; then
      for value_flag in "${_asc_value_flags[@]}"; do
        if [[ "$token" == "$value_flag" ]]; then
          expect_value=1
          break
        fi
      done
      continue
    fi
    for subcommand in "${_asc_subcommands[@]}"; do
      if [[ "$token" == "$subcommand" ]]; then
        if [[ -n "$path" ]]; then
          path="$path $token"
        else
          path="$token"
        fi
        break
      fi
    done
  done

  if [[ $expect_value -eq 1 ]]; then
    return 0
  fi
  _asc_completion_lookup "$path"
  if [[ ${#_asc_subcommands[@]} -gt 0 ]]; then
    printf '%s\n' "${_asc_subcommands[@]}"
  fi
  if [[ ${#_asc_flags[@]} -gt 0 ]]; then
    printf '%s\n' "${_asc_flags[@]}"
  fi
}

_asc_completions() {
  local cur candidates
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  candidates="$(_asc_completion_candidates "${COMP_WORDS[@]:1:COMP_CWORD-1}")"
  COMPREPLY=( $(compgen -W "$candidates" -- "$cur") )
}

complete -F _asc_completions asc
`)
	return b.String()
}

func zshScript(nodes []completionNode) string {
	var b strings.Builder
	b.WriteString("#compdef asc\n\n")
	writeShellCompletionData(&b, nodes)
	b.WriteString(`
_asc_completion_lookup() {
  _asc_subcommands=()
  _asc_flags=()
  _asc_value_flags=()
  local index
  for ((index = 1; index <= ${#_ASC_COMPLETION_PATHS[@]}; index++)); do
    if [[ "${_ASC_COMPLETION_PATHS[index]}" == "$1" ]]; then
      _asc_subcommands=(${=_ASC_COMPLETION_SUBCOMMAND_GROUPS[index]})
      _asc_flags=(${=_ASC_COMPLETION_FLAG_GROUPS[index]})
      _asc_value_flags=(${=_ASC_COMPLETION_VALUE_FLAG_GROUPS[index]})
      return 0
    fi
  done
}

_asc_completion_candidates() {
  local path="" token subcommand value_flag
  local -i expect_value=0
  local -a _asc_subcommands _asc_flags _asc_value_flags

  for token in "$@"; do
    if (( expect_value )); then
      expect_value=0
      continue
    fi

    _asc_completion_lookup "$path"
    if [[ "$token" == --*=* ]]; then
      continue
    fi
    if [[ "$token" == --* ]]; then
      for value_flag in "${_asc_value_flags[@]}"; do
        if [[ "$token" == "$value_flag" ]]; then
          expect_value=1
          break
        fi
      done
      continue
    fi
    for subcommand in "${_asc_subcommands[@]}"; do
      if [[ "$token" == "$subcommand" ]]; then
        if [[ -n "$path" ]]; then
          path="$path $token"
        else
          path="$token"
        fi
        break
      fi
    done
  done

  if (( expect_value )); then
    return 0
  fi
  _asc_completion_lookup "$path"
  if (( ${#_asc_subcommands[@]} )); then
    print -rl -- "${_asc_subcommands[@]}"
  fi
  if (( ${#_asc_flags[@]} )); then
    print -rl -- "${_asc_flags[@]}"
  fi
}

_asc() {
  local -a candidates prior_words
  if (( CURRENT > 2 )); then
    prior_words=("${words[2,$((CURRENT - 1))][@]}")
  fi
  candidates=("${(@f)$(_asc_completion_candidates "${prior_words[@]}")}")
  compadd -- "${candidates[@]}"
}

compdef _asc asc
`)
	return b.String()
}

func fishScript(nodes []completionNode) string {
	var b strings.Builder
	b.WriteString("# fish completion for asc\n")
	writeFishCompletionData(&b, nodes)
	b.WriteString(`
function __asc_completion_index
    contains -i -- "$argv[1]" $__asc_completion_paths
end

function __asc_completion_candidates
    set -l path ''
    set -l expect_value 0

    for token in $argv
        if test $expect_value -eq 1
            set expect_value 0
            continue
        end

        set -l index (__asc_completion_index "$path")
        if test -z "$index"
            return 0
        end
        set -l subcommands
        set -l subcommand_group "$__asc_completion_subcommand_groups[$index]"
        if test -n "$subcommand_group"
            set subcommands (string split ' ' -- "$subcommand_group")
        end
        set -l value_flags
        set -l value_flag_group "$__asc_completion_value_flag_groups[$index]"
        if test -n "$value_flag_group"
            set value_flags (string split ' ' -- "$value_flag_group")
        end
        if string match -q -- '--*=*' "$token"
            continue
        end
        if string match -q -- '--*' "$token"
            if contains -- "$token" $value_flags
                set expect_value 1
            end
            continue
        end
        if contains -- "$token" $subcommands
            if test -n "$path"
                set path "$path $token"
            else
                set path "$token"
            end
        end
    end

    if test $expect_value -eq 1
        return 0
    end
    set -l index (__asc_completion_index "$path")
    if test -z "$index"
        return 0
    end
    set -l subcommand_group "$__asc_completion_subcommand_groups[$index]"
    if test -n "$subcommand_group"
        string split ' ' -- "$subcommand_group"
    end
    set -l flag_group "$__asc_completion_flag_groups[$index]"
    if test -n "$flag_group"
        string split ' ' -- "$flag_group"
    end
end

function __asc_completion_current
    set -l tokens (commandline -opc)
    if test (count $tokens) -gt 0
        set -e tokens[1]
    end
    __asc_completion_candidates $tokens
end

complete -c asc -f -a '(__asc_completion_current)'
`)
	return b.String()
}

func writeShellCompletionData(b *strings.Builder, nodes []completionNode) {
	writeShellGroups(b, "_ASC_COMPLETION_PATHS", nodes, func(node completionNode) string { return node.path })
	writeShellGroups(b, "_ASC_COMPLETION_SUBCOMMAND_GROUPS", nodes, func(node completionNode) string {
		return strings.Join(node.subcommands, " ")
	})
	writeShellGroups(b, "_ASC_COMPLETION_FLAG_GROUPS", nodes, func(node completionNode) string {
		return strings.Join(node.flags, " ")
	})
	writeShellGroups(b, "_ASC_COMPLETION_VALUE_FLAG_GROUPS", nodes, func(node completionNode) string {
		return strings.Join(node.valueFlags, " ")
	})
}

func writeShellGroups(b *strings.Builder, variable string, nodes []completionNode, value func(completionNode) string) {
	b.WriteString(variable)
	b.WriteString("=(")
	for i, node := range nodes {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(shellSingleQuote(value(node)))
	}
	b.WriteString(")\n")
}

func writeFishCompletionData(b *strings.Builder, nodes []completionNode) {
	writeFishGroups(b, "__asc_completion_paths", nodes, func(node completionNode) string { return node.path })
	writeFishGroups(b, "__asc_completion_subcommand_groups", nodes, func(node completionNode) string {
		return strings.Join(node.subcommands, " ")
	})
	writeFishGroups(b, "__asc_completion_flag_groups", nodes, func(node completionNode) string {
		return strings.Join(node.flags, " ")
	})
	writeFishGroups(b, "__asc_completion_value_flag_groups", nodes, func(node completionNode) string {
		return strings.Join(node.valueFlags, " ")
	})
}

func writeFishGroups(b *strings.Builder, variable string, nodes []completionNode, value func(completionNode) string) {
	b.WriteString("set -g ")
	b.WriteString(variable)
	for _, node := range nodes {
		b.WriteByte(' ')
		b.WriteString(fishSingleQuote(value(node)))
	}
	b.WriteByte('\n')
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func fishSingleQuote(value string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(value) + "'"
}
