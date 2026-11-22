package shared

import (
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// flagAlias is the value of a second spelling bound with BindFlagAlias or
// BindBuildFlagAlias. The first target whose shape matches the given value is
// the flag the alias stands for.
type flagAlias struct {
	targets []flagAliasTarget
	// name and implied are set for a value alias bound with
	// BindFlagValueAlias: a flag that takes no value and stands for its
	// target given implied.
	name    string
	implied string
}

type flagAliasTarget struct {
	flag *flag.Flag
	// matches reports whether a value has this target's shape; nil matches
	// every value.
	matches func(string) bool
}

func (a *flagAlias) target(value string) *flag.Flag {
	for _, target := range a.targets {
		if target.matches == nil || target.matches(value) {
			return target.flag
		}
	}
	return a.targets[len(a.targets)-1].flag
}

func (a *flagAlias) String() string {
	if a == nil || len(a.targets) == 0 {
		return ""
	}
	return a.targets[0].flag.Value.String()
}

func (a *flagAlias) Set(value string) error {
	if a.implied != "" {
		if enabled, err := strconv.ParseBool(value); err != nil || !enabled {
			return fmt.Errorf("--%s takes no value; it means --%s %s", a.name, a.targets[0].flag.Name, a.implied)
		}
		return a.targets[0].flag.Value.Set(a.implied)
	}
	return a.target(value).Value.Set(value)
}

func (a *flagAlias) IsBoolFlag() bool {
	if a.implied != "" {
		return true
	}
	boolValue, ok := a.targets[0].flag.Value.(interface{ IsBoolFlag() bool })
	return ok && boolValue.IsBoolFlag()
}

// BindFlagAlias accepts --alias as a second spelling of the flag canonical,
// which must already be bound on fs. The alias is hidden from help, and the
// root rewrites it to the canonical spelling before parsing, so it is
// validated, reported and sent exactly as the canonical flag is.
func BindFlagAlias(fs *flag.FlagSet, alias, canonical string) {
	bindFlagAlias(fs, alias, flagAliasTarget{flag: boundFlag(fs, alias, canonical)})
}

// BindFlagValueAlias accepts --alias, which takes no value, as a second
// spelling of --canonical given value: `--json` for `--output json`. The
// canonical flag must already be bound on fs. The alias is hidden from help,
// and the root rewrites it to `--canonical value` before parsing, so giving
// it next to the canonical flag with another value is a usage error.
func BindFlagValueAlias(fs *flag.FlagSet, alias, canonical, value string) {
	fs.Var(&flagAlias{
		targets: []flagAliasTarget{{flag: boundFlag(fs, alias, canonical)}},
		name:    alias,
		implied: value,
	}, alias, fmt.Sprintf("Alias of --%s %s", canonical, value))
}

var buildIDShape = regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`)

// BindBuildFlagAlias accepts --build on a command that selects a build by
// --build-id or --build-number, both already bound on fs. App Store Connect
// build IDs are UUIDs and a build number (CFBundleVersion) is a list of at
// most three period-separated integers, so the value's shape says which one
// it is: a UUID or a URL (a build's self-link) is the build ID, any other
// value the build number.
func BindBuildFlagAlias(fs *flag.FlagSet) {
	bindFlagAlias(fs, "build",
		flagAliasTarget{
			flag: boundFlag(fs, "build", "build-id"),
			matches: func(value string) bool {
				value = strings.TrimSpace(value)
				return buildIDShape.MatchString(value) || looksLikeHTTPURL(value)
			},
		},
		flagAliasTarget{flag: boundFlag(fs, "build", "build-number")},
	)
}

func boundFlag(fs *flag.FlagSet, alias, canonical string) *flag.Flag {
	target := fs.Lookup(canonical)
	if target == nil {
		panic(fmt.Sprintf("shared: flag alias --%s names undefined flag --%s", alias, canonical))
	}
	return target
}

func bindFlagAlias(fs *flag.FlagSet, alias string, targets ...flagAliasTarget) {
	names := make([]string, len(targets))
	for i, target := range targets {
		names[i] = "--" + target.flag.Name
	}
	fs.Var(&flagAlias{targets: targets}, alias, "Alias of "+strings.Join(names, " or "))
}

// IsFlagAlias reports whether f is a second spelling bound with
// BindFlagAlias or BindBuildFlagAlias.
func IsFlagAlias(f *flag.Flag) bool {
	if f == nil {
		return false
	}
	alias, ok := f.Value.(*flagAlias)
	return ok && len(alias.targets) > 0
}

// FlagAliasDependsOnValue reports whether the alias f stands for a different
// flag depending on the value it is given.
func FlagAliasDependsOnValue(f *flag.Flag) bool {
	return IsFlagAlias(f) && len(f.Value.(*flagAlias).targets) > 1
}

// FlagAliasTarget returns the name of the flag that the alias f stands for
// when it is given value.
func FlagAliasTarget(f *flag.Flag, value string) (string, bool) {
	if !IsFlagAlias(f) {
		return "", false
	}
	return f.Value.(*flagAlias).target(value).Name, true
}

// FlagAliasImpliedValue returns the canonical flag and value that the value
// alias f, bound with BindFlagValueAlias, stands for.
func FlagAliasImpliedValue(f *flag.Flag) (canonical, value string, ok bool) {
	if !IsFlagAlias(f) {
		return "", "", false
	}
	alias := f.Value.(*flagAlias)
	if alias.implied == "" {
		return "", "", false
	}
	return alias.targets[0].flag.Name, alias.implied, true
}
