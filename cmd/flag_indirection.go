package cmd

import (
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// indirectionExcludedFlags lists the flag names that never resolve
// @env:/@file: values at any level of the command tree. `--output` selects
// output rendering on nearly every command and the usage renderers read it
// back, so it is excluded under both of its meanings: the format enum and the
// output path on `signing resign`, `profiles`, and the asset commands.
//
// Command-local flags are not excluded merely because they select a format:
// `--format` on `signing fetch`, `signing resign`, and the asset commands is
// read only from its own parsed FlagSet, so a resolved value has exactly one
// reader and the enum validation runs on the resolved text.
var indirectionExcludedFlags = map[string]struct{}{
	"output": {},
}

// indirectionExcludedRootFlags lists the root selectors that never resolve
// @env:/@file: values. They choose the credential profile and the CI report
// plumbing for the whole process, and the parse-failure paths re-read them
// from the raw argv (recoverCIReportFlags, recoverySuggestedRootArgs), so
// resolving them would create two sources of truth.
//
// The exclusion is scoped to the root FlagSet because these names are not
// reserved: `signing run --profile` is a provisioning-profile path owned by
// that command, has a single reader, and stays resolvable.
var indirectionExcludedRootFlags = map[string]struct{}{
	"profile":     {},
	"report":      {},
	"report-file": {},
}

// isIndirectionExcluded reports whether the named flag on the command
// currently being walked keeps its raw value.
func isIndirectionExcluded(name string, isRootFlag bool) bool {
	if _, excluded := indirectionExcludedFlags[name]; excluded {
		return true
	}
	if !isRootFlag {
		return false
	}
	_, excluded := indirectionExcludedRootFlags[name]
	return excluded
}

// resolveFlagValueIndirection rewrites argv so that every value-taking flag on
// the resolved command chain receives its @env:NAME or @file:PATH value before
// flag parsing. The result has the same token count and structure as args:
// only value tokens change, so structural analyses of the original args stay
// valid and resolved values never flow into diagnostics.
//
// Boolean flags, excluded flags, positional arguments, and everything after
// `--` are untouched. Rewriting stops at the first unknown flag so the
// unknown-flag diagnostic stays authoritative for that invocation.
func resolveFlagValueIndirection(root *ffcli.Command, args []string) ([]string, error) {
	return rewriteFlagValueIndirection(root, args, false)
}

// dropIndirectFlagValues removes every flag token that carries an indirect
// value, together with its value token, using the same walk as
// resolveFlagValueIndirection.
//
// Help rendering depends only on which command the args select, never on a
// flag value, so a help request drops these tokens instead of resolving them.
// That keeps `--help` free of environment and filesystem reads, guarantees
// that help prints whatever the indirect value would have been, and keeps a
// resolved value out of help output and out of any parse diagnostic.
func dropIndirectFlagValues(root *ffcli.Command, args []string) []string {
	dropped, err := rewriteFlagValueIndirection(root, args, true)
	if err != nil {
		// Dropping never resolves a value, so it cannot fail.
		return args
	}
	return dropped
}

func rewriteFlagValueIndirection(root *ffcli.Command, args []string, drop bool) ([]string, error) {
	command := root
	sawPositional := false
	resolved := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		token := args[index]
		if token == "--" {
			resolved = append(resolved, args[index:]...)
			break
		}
		if token == "" || token == "-" || !strings.HasPrefix(token, "-") {
			if token != "" && !sawPositional {
				if subcommand := findDirectSubcommand(command, token); subcommand != nil {
					command = subcommand
					resolved = append(resolved, token)
					continue
				}
				sawPositional = true
			}
			resolved = append(resolved, token)
			continue
		}

		if !hasValidFlagPrefix(token) {
			// A malformed spelling such as `---flag` is a bad-flag-syntax
			// usage error, not a flag this walk may rewrite or remove. Leave
			// it and everything after it verbatim so that diagnostic stays
			// authoritative for the invocation.
			resolved = append(resolved, args[index:]...)
			break
		}
		name, inlineValue, hasInlineValue := strings.Cut(strings.TrimLeft(token, "-"), "=")
		if command == nil || command.FlagSet == nil || name == "" {
			resolved = append(resolved, args[index:]...)
			break
		}
		item := command.FlagSet.Lookup(name)
		if item == nil {
			resolved = append(resolved, args[index:]...)
			break
		}
		if isBoolFlag(item) {
			resolved = append(resolved, token)
			continue
		}
		if isIndirectionExcluded(name, command == root) {
			// Consume the value token untouched so it is never mistaken for
			// a positional argument that ends subcommand descent.
			resolved = append(resolved, token)
			if !hasInlineValue && index+1 < len(args) {
				index++
				resolved = append(resolved, args[index])
			}
			continue
		}
		if hasInlineValue {
			if drop {
				if shared.HasFlagValueIndirection(inlineValue) {
					continue
				}
				resolved = append(resolved, token)
				continue
			}
			value, err := shared.ResolveFlagValueIndirection(name, inlineValue)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, token[:len(token)-len(inlineValue)]+value)
			continue
		}
		if index+1 >= len(args) {
			resolved = append(resolved, token)
			continue
		}
		if drop {
			index++
			if shared.HasFlagValueIndirection(args[index]) {
				continue
			}
			resolved = append(resolved, token, args[index])
			continue
		}
		resolved = append(resolved, token)
		index++
		value, err := shared.ResolveFlagValueIndirection(name, args[index])
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, value)
	}
	return resolved, nil
}
