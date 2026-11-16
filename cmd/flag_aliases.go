package cmd

import (
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type givenFlagValue struct {
	spelling string
	value    string
}

// resolveFlagAliases rewrites each flag alias bound with shared.BindFlagAlias
// to the canonical flag it stands for, so parsing, validation, diagnostics and
// telemetry only ever see the canonical spelling. It walks the arguments the
// way the standard flag package parses them and stops where parsing would
// stop: at `--`, at the first positional argument, and at an unknown flag.
//
// A flag given under two spellings with the same value is kept once. With
// different values the invocation is ambiguous, so it is a usage error.
func resolveFlagAliases(root *ffcli.Command, args []string) ([]string, error) {
	command := root
	commandPath := make([]string, 0, 4)
	if root != nil && root.Name != "" {
		commandPath = append(commandPath, root.Name)
	}
	given := map[string]givenFlagValue{}
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
				given = map[string]givenFlagValue{}
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
		if target, ok := shared.FlagAliasTarget(item); ok {
			canonical = target
			tokens[0] = dashes + canonical
			if hasInlineValue {
				tokens[0] += "=" + inlineValue
			}
		}

		previous, seen := given[canonical]
		if !seen {
			given[canonical] = givenFlagValue{spelling: name, value: value}
			resolved = append(resolved, tokens...)
			continue
		}
		if previous.spelling == name {
			resolved = append(resolved, tokens...)
			continue
		}
		if strings.TrimSpace(previous.value) == strings.TrimSpace(value) {
			continue
		}
		return args, shared.WithDiagnostic(
			fmt.Errorf("`--%s` and `--%s` are the same flag; pass one value", previous.spelling, name),
			shared.DiagnosticConflictingInput,
			"--"+canonical,
		)
	}
	return resolved, nil
}
