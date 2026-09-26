package cmd

import (
	"flag"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/suggest"
)

// maxUnknownFlagSuggestions caps the `Try:` block. Three lines stay scannable
// and keep a wrong guess from burying the flag the caller wanted.
const maxUnknownFlagSuggestions = 3

// flagSynonyms maps a spelling operators actually type, in telemetry, to the
// canonical flag names that spelling means, in preference order. A target is
// only ever printed when the command being invoked really defines it, so an
// entry can list every canonical form the CLI uses for that concept.
var flagSynonyms = map[string][]string{
	"app":        {"id", "bundle"},
	"app-id":     {"app", "id"},
	"build":      {"build-id", "build-number"},
	"build-id":   {"build", "build-number"},
	"bundle":     {"bundle-id", "identifier"},
	"bundle-id":  {"bundle", "identifier"},
	"dir":        {"path", "output-dir"},
	"file":       {"ipa", "pkg", "path", "dir"},
	"group":      {"group-id"},
	"group-id":   {"group"},
	"id":         {"app"},
	"path":       {"file", "dir", "ipa"},
	"version":    {"version-id", "app-store-version-id"},
	"version-id": {"version", "app-store-version-id", "id"},
}

// conditionalSynonym is a synonym target that is only right when the target's
// own help text agrees. `--output` is the format selector on most commands but
// a filesystem destination on a few, and only the latter can be what a caller
// meant by `--path`.
type conditionalSynonym struct {
	target string
	when   func(usage string) bool
}

// conditionalSynonyms holds the synonyms that depend on the target's help text.
var conditionalSynonyms = map[string][]conditionalSynonym{
	"path": {{target: "output", when: isPathValuedUsage}},
	"dir":  {{target: "output", when: isPathValuedUsage}},
	"file": {{target: "output", when: isPathValuedUsage}},
}

// pathValuedUsagePattern matches help text that describes a filesystem
// destination, such as "Output CSV file path", "Path for the newly re-signed
// IPA", or "Output directory for signing files", and not a format selector such
// as "Output format: json, table, markdown".
var pathValuedUsagePattern = regexp.MustCompile(`(?i)\b(paths?|directory|directories|folder|destination)\b`)

// isPathValuedUsage reports whether a flag's help text describes a filesystem
// destination rather than a value such as an output format.
func isPathValuedUsage(usage string) bool {
	return pathValuedUsagePattern.MatchString(usage)
}

// identifierFlagNouns names the resource words whose flag spellings operators
// most often invent (`--app` on a build-scoped command, for example). Only
// these open the selector fallback, so an ordinary typo still relies on the
// nearest-name ranking instead of a list of unrelated identifier flags.
var identifierFlagNouns = map[string]struct{}{
	"app": {}, "application": {}, "build": {}, "bundle": {}, "certificate": {},
	"device": {}, "experience": {}, "group": {}, "item": {}, "key": {},
	"localization": {}, "product": {}, "profile": {}, "run": {}, "screenshot": {},
	"submission": {}, "subscription": {}, "tester": {}, "territory": {},
	"version": {}, "workflow": {},
}

// identifierUsagePattern matches a flag whose help text names an identifier as
// a word of its own, such as "Build ID" or "Filter by profile ID(s)". Those
// flags are the selectors a caller was reaching for when they invented an
// identifier flag. Underscored and angle-bracketed spellings are excluded so a
// filename template such as "AuthKey_<KEY_ID>.p8" does not read as a selector.
var identifierUsagePattern = regexp.MustCompile(`(^|[^A-Za-z_<])IDs?([^A-Za-z_>]|$)`)

// guidanceFlagPattern extracts the long-form flags a removed-alias message
// already names, so the `Try:` block never repeats them.
var guidanceFlagPattern = regexp.MustCompile(`--([a-z][a-z0-9-]*)`)

type unknownFlagSuggestionOptions struct {
	// allowSelectorFallback permits the identifier-selector tier. It stays off
	// for removed aliases, whose message already names the replacement.
	allowSelectorFallback bool
	// exclude lists flag names, without leading dashes, that the surrounding
	// diagnostic already printed.
	exclude []string
}

// unknownFlagSuggestions ranks up to maxUnknownFlagSuggestions defined flags of
// one command for an unknown flag spelling. Tiers, highest first:
//
//  1. curated synonyms for that spelling, including the ones that depend on the
//     target's help text;
//  2. nearest defined flag name by prefix and edit distance (suggest.Flags);
//  3. when the spelling is an invented identifier and nothing above matched,
//     the command's own identifier selectors.
//
// Every returned name is defined on the provided flag set, so a suggestion can
// never name a flag the command does not accept.
func unknownFlagSuggestions(flags *flag.FlagSet, flagName string, options unknownFlagSuggestionOptions) []string {
	name := strings.ToLower(strings.TrimLeft(strings.TrimSpace(flagName), "-"))
	if flags == nil || name == "" {
		return nil
	}

	candidates := suggestibleFlags(flags)
	if len(candidates) == 0 {
		return nil
	}

	defined := make(map[string]string, len(candidates))
	names := make([]string, 0, len(candidates))
	identifierInput := isInventedIdentifierFlag(name)
	for _, candidate := range candidates {
		defined[candidate.Name] = candidate.Usage
		if !identifierInput || !isSparseFieldFlag(candidate.Name) {
			names = append(names, candidate.Name)
		}
	}

	skip := map[string]struct{}{name: {}}
	for _, excluded := range options.exclude {
		skip[strings.ToLower(strings.TrimLeft(strings.TrimSpace(excluded), "-"))] = struct{}{}
	}

	suggestions := make([]string, 0, maxUnknownFlagSuggestions)
	add := func(candidate string) {
		if len(suggestions) >= maxUnknownFlagSuggestions {
			return
		}
		if _, skipped := skip[candidate]; skipped {
			return
		}
		if _, ok := defined[candidate]; !ok {
			return
		}
		skip[candidate] = struct{}{}
		suggestions = append(suggestions, candidate)
	}

	for _, synonym := range flagSynonyms[name] {
		add(synonym)
	}
	for _, conditional := range conditionalSynonyms[name] {
		if usage, ok := defined[conditional.target]; ok && conditional.when(usage) {
			add(conditional.target)
		}
	}
	for _, nearest := range suggest.Flags(name, names) {
		add(nearest)
	}
	if len(suggestions) == 0 && options.allowSelectorFallback {
		if selector := identifierSelectorFlag(name, candidates); selector != "" {
			add(selector)
		}
	}
	return suggestions
}

func isSparseFieldFlag(name string) bool {
	return name == "fields" || strings.HasSuffix(name, "-fields")
}

// suggestibleFlags lists the flags a suggestion may name: visible in help and
// not deprecated.
func suggestibleFlags(flags *flag.FlagSet) []*flag.Flag {
	visible := shared.VisibleHelpFlags(flags)
	suggestible := make([]*flag.Flag, 0, len(visible))
	for _, item := range visible {
		if item == nil || isDeprecatedFlagHelp(item.Usage) {
			continue
		}
		suggestible = append(suggestible, item)
	}
	return suggestible
}

// identifierSelectorFlag answers an invented identifier spelling with the one
// flag that names the resource the command acts on, and only when that flag is
// the command's only required input. `asc builds groups list --app` has no near
// name, but the command selects its resource one way only, with `--build-id`.
// A command that takes several required inputs is left alone: guessing which
// one the caller meant would be worse than the plain unknown-flag error.
func identifierSelectorFlag(name string, candidates []*flag.Flag) string {
	if !isInventedIdentifierFlag(name) {
		return ""
	}

	selector := ""
	selectors := 0
	otherRequired := 0
	for _, candidate := range candidates {
		switch {
		case isIdentifierSelectorFlag(candidate.Name, candidate.Usage):
			selector = candidate.Name
			selectors++
		case isRequiredInputFlag(candidate.Usage):
			otherRequired++
		}
	}
	if selectors != 1 || otherRequired > 0 {
		return ""
	}
	return selector
}

// isRequiredInputFlag reports whether a flag's help text marks it required.
// Such a flag is an input the caller must name, so a command that has one
// besides its selector is ambiguous enough to leave without a suggestion.
func isRequiredInputFlag(usage string) bool {
	return strings.Contains(strings.ToLower(usage), "(required")
}

// isInventedIdentifierFlag reports whether a spelling reads as a resource
// selector: an `id` form, or a bare resource noun such as `app` or `group`.
func isInventedIdentifierFlag(name string) bool {
	if name == "id" || strings.HasSuffix(name, "-id") || strings.HasSuffix(name, "-ids") {
		return true
	}
	_, ok := identifierFlagNouns[name]
	return ok
}

// authIdentityFlags name the account, team, and credential identifiers most
// commands inherit. They identify who is calling, never which resource the
// command acts on, so they can never answer an invented resource flag.
var authIdentityFlags = map[string]struct{}{
	"apple-id": {}, "developer-team": {}, "issuer-id": {}, "key-id": {},
	"private-key-id": {}, "provider-id": {}, "public-provider-id": {},
	"team-id": {},
}

// isIdentifierSelectorFlag reports whether a defined flag selects the resource a
// command acts on. Sparse-field, include, and paging flags mention identifiers
// in passing without ever selecting one, and credential identifiers name the
// caller rather than the resource, so both are excluded by name.
func isIdentifierSelectorFlag(name, usage string) bool {
	if _, ok := authIdentityFlags[name]; ok {
		return false
	}
	switch {
	case name == "fields", strings.HasSuffix(name, "-fields"),
		name == "include", strings.HasSuffix(name, "-include"),
		name == "limit", strings.HasSuffix(name, "-limit"), strings.HasPrefix(name, "limit-"),
		name == "sort", name == "next", name == "output", name == "filter":
		return false
	// A filesystem destination names where output goes, never which resource
	// the command acts on.
	case name == "dir", strings.HasSuffix(name, "-dir"),
		name == "path", strings.HasSuffix(name, "-path"),
		name == "file", strings.HasSuffix(name, "-file"):
		return false
	}
	lowered := strings.ToLower(usage)
	if strings.HasPrefix(lowered, "sparse fields") || strings.Contains(lowered, "fields to include") {
		return false
	}
	return identifierUsagePattern.MatchString(usage)
}

// printFlagSuggestions writes the `Try:` block for the ranked suggestions, and
// writes nothing when there are none.
func printFlagSuggestions(w io.Writer, suggestions []string) {
	if len(suggestions) == 0 {
		return
	}
	fmt.Fprintln(w, "Try:")
	for _, suggestion := range suggestions {
		fmt.Fprintf(w, "  --%s\n", shared.SanitizeTerminal(suggestion))
	}
}

// removedFlagGuidanceSuggestions ranks the flags to print after a removed-alias
// message. A rule that names a replacement has already answered the caller, and
// a generic guess after it could only contradict that answer, so only a
// note-only rule - one with no flag to type instead - gets suggestions.
func removedFlagGuidanceSuggestions(rule removedFlagRule, flags *flag.FlagSet, flagName string) []string {
	if rule.replacement != "" {
		return nil
	}
	return unknownFlagSuggestions(flags, flagName, unknownFlagSuggestionOptions{
		exclude: guidanceFlagNames(rule),
	})
}

// guidanceFlagNames lists the long-form flags a removed-alias message already
// names in its guidance clause.
func guidanceFlagNames(rule removedFlagRule) []string {
	guidance := rule.replacement + " " + rule.note
	matches := guidanceFlagPattern.FindAllStringSubmatch(guidance, -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match[1])
	}
	return names
}
