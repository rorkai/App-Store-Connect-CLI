package shared

import (
	"flag"
	"strings"
)

// primaryIDFlagNames is the small allowlist of flag names that carry a
// command's primary resource identifier. A caller who typed the identifier as a
// bare operand almost always meant one of these, so the stray-operand
// diagnostic can name the flag instead of only rejecting the token.
//
// Keep the list short and only add names that identify one resource. Filter
// flags that happen to take an identifier-shaped value (`--bundle-id` takes a
// reverse-DNS string, `--territory` a country code) would turn the hint into a
// guess, and a wrong hint costs more than no hint at all.
var primaryIDFlagNames = []string{
	"id",
	"app",
	"build-id",
	"version-id",
	"group-id",
	"iap-id",
	"subscription-id",
	"localization-id",
}

// PrimaryIDFlagName returns the single primary identifier flag that flagSet
// defines and the caller has not already passed. It reports false when the
// command defines none of them, when it defines several (with more than one
// candidate the intended flag would be a guess), and when the one candidate is
// already set, because a caller who wrote `--app APP_ID` did not mean the
// stray token as a second app.
func PrimaryIDFlagName(flagSet *flag.FlagSet) (string, bool) {
	if flagSet == nil {
		return "", false
	}

	defined := make(map[string]struct{})
	flagSet.VisitAll(func(item *flag.Flag) {
		defined[item.Name] = struct{}{}
	})
	provided := make(map[string]struct{})
	flagSet.Visit(func(item *flag.Flag) {
		provided[item.Name] = struct{}{}
	})

	match := ""
	for _, name := range primaryIDFlagNames {
		if _, ok := defined[name]; !ok {
			continue
		}
		if match != "" {
			return "", false
		}
		match = name
	}
	if match == "" {
		return "", false
	}
	if _, alreadyProvided := provided[match]; alreadyProvided {
		return "", false
	}
	return match, true
}

// UsageDeclaresOperands reports whether a command's own ShortUsage declares
// positional operands, as `asc search [flags] <query>` does and
// `asc apps view --id APP_ID` does not.
//
// Only the tokens before the first flag spelling are operands. Everything from
// the first flag onward is either a flag or a flag value, and a value
// placeholder (`<bash|zsh|fish>` in `asc completion --shell <bash|zsh|fish>`)
// is not an operand. A standalone `--` terminator always introduces operands,
// because the tokens after it are handed to a child process verbatim.
func UsageDeclaresOperands(usage string) bool {
	tokens := strings.Fields(usage)
	for _, token := range tokens {
		if token == "--" {
			return true
		}
	}

	for _, token := range tokens {
		bare := strings.TrimLeft(token, "[(")
		if strings.HasPrefix(bare, "-") {
			return false
		}
		if !strings.HasPrefix(token, "<") && !strings.HasPrefix(token, "[") {
			continue
		}
		// `[flags]` and `<subcommand>` are structural placeholders every group
		// and leaf usage string shares, not operands of their own.
		switch strings.Trim(bare, "<>[]()") {
		case "flags", "subcommand":
			continue
		}
		return true
	}
	return false
}
