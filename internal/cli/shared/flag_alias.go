package shared

import (
	"flag"
	"fmt"
	"regexp"
	"strings"
)

// flagAlias is the value of a second spelling bound with BindFlagAlias or
// BindBuildFlagAlias. The first target whose shape matches the given value is
// the flag the alias stands for.
type flagAlias struct {
	targets []flagAliasTarget
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
	return a.target(value).Value.Set(value)
}

func (a *flagAlias) IsBoolFlag() bool {
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

// FlagAliasTarget returns the name of the flag that the alias f stands for
// when it is given value.
func FlagAliasTarget(f *flag.Flag, value string) (string, bool) {
	if !IsFlagAlias(f) {
		return "", false
	}
	return f.Value.(*flagAlias).target(value).Name, true
}
