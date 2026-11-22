package cmd

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// resolveFlagAliases rewrites each flag alias bound with shared.BindFlagAlias
// to the canonical flag it stands for, so parsing, validation, diagnostics and
// telemetry only ever see the canonical spelling. It walks the arguments the
// way the standard flag package parses them and stops where parsing would
// stop: at `--`, at the first positional argument, and at an unknown flag.
//
// An alias whose target depends on its value reads an @env:/@file: value
// through resolveIndirect to choose the target; the token itself is left for
// the target flag's own indirection rules. resolveIndirect may be nil.
//
// A flag given under two spellings with the same value is kept once. When a
// spelling's value differs from the latest value given under another
// spelling, the invocation is ambiguous, so it is a usage error. A repeatable
// flag keeps every value under any spelling.
func resolveFlagAliases(root *ffcli.Command, args []string, resolveIndirect func(name, value string) (string, error)) ([]string, error) {
	command := root
	commandPath := make([]string, 0, 4)
	if root != nil && root.Name != "" {
		commandPath = append(commandPath, root.Name)
	}
	given := map[string]map[string]string{}
	resolved := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		token := args[index]
		if token == "--" {
			resolved = append(resolved, args[index:]...)
			break
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			if subcommand := findDirectSubcommand(command, token); token != "" && subcommand != nil {
				command = subcommand
				commandPath = append(commandPath, subcommand.Name)
				given = map[string]map[string]string{}
				resolved = append(resolved, token)
				continue
			}
			resolved = append(resolved, args[index:]...)
			break
		}
		if !hasValidFlagPrefix(token) {
			resolved = append(resolved, args[index:]...)
			break
		}

		dashes := token[:len(token)-len(strings.TrimLeft(token, "-"))]
		name, inlineValue, hasInlineValue := strings.Cut(token[len(dashes):], "=")
		item := commandFlag(command, name)
		if item == nil {
			resolved = append(resolved, args[index:]...)
			break
		}

		tokens := []string{token}
		value := inlineValue
		switch {
		case hasInlineValue:
		case isBoolFlag(item):
			value = "true"
			if consumesSpacedBooleanValue(command, commandPath, args, index) {
				index++
				value = args[index]
				tokens = append(tokens, value)
			}
		case index+1 < len(args):
			index++
			value = args[index]
			tokens = append(tokens, value)
		}

		canonical := name
		shape := value
		if target, implied, ok := shared.FlagAliasImpliedValue(item); ok {
			// A value alias that is not given as true stays as written, so
			// parsing reports it with the alias's own message.
			if enabled, err := strconv.ParseBool(value); err == nil && enabled {
				canonical = target
				value = implied
				tokens = []string{dashes + target, implied}
			}
		} else {
			if resolveIndirect != nil && shared.FlagAliasDependsOnValue(item) && shared.HasFlagValueIndirection(value) {
				if resolved, err := resolveIndirect(name, value); err == nil {
					shape = resolved
				}
			}
			if target, ok := shared.FlagAliasTarget(item, shape); ok {
				canonical = target
				tokens[0] = dashes + canonical
				if hasInlineValue {
					tokens[0] += "=" + inlineValue
				}
			}
		}

		spellings := given[canonical]
		if spellings == nil {
			spellings = map[string]string{}
			given[canonical] = spellings
		}
		if shared.IsRepeatableFlag(commandFlag(command, canonical)) {
			resolved = append(resolved, tokens...)
			continue
		}
		drop := false
		for _, other := range slices.Sorted(maps.Keys(spellings)) {
			otherValue := spellings[other]
			if other == name {
				continue
			}
			if otherValue != value {
				return args, shared.WithDiagnostic(
					fmt.Errorf("`--%s` and `--%s` are the same flag; pass one value", other, name),
					shared.DiagnosticConflictingInput,
					"--"+canonical,
				)
			}
			drop = true
		}
		spellings[name] = value
		if !drop {
			resolved = append(resolved, tokens...)
		}
	}
	return resolved, nil
}
