package shared

import (
	"flag"
	"fmt"
)

// flagAlias is the value of a second spelling bound with BindFlagAlias.
type flagAlias struct {
	canonical *flag.Flag
}

func (a *flagAlias) String() string {
	if a == nil || a.canonical == nil {
		return ""
	}
	return a.canonical.Value.String()
}

func (a *flagAlias) Set(value string) error {
	return a.canonical.Value.Set(value)
}

func (a *flagAlias) IsBoolFlag() bool {
	boolValue, ok := a.canonical.Value.(interface{ IsBoolFlag() bool })
	return ok && boolValue.IsBoolFlag()
}

// BindFlagAlias accepts --alias as a second spelling of the flag canonical,
// which must already be bound on fs. The alias is hidden from help, and the
// root rewrites it to the canonical spelling before parsing, so it is
// validated, reported and sent exactly as the canonical flag is.
func BindFlagAlias(fs *flag.FlagSet, alias, canonical string) {
	target := fs.Lookup(canonical)
	if target == nil {
		panic(fmt.Sprintf("shared: flag alias --%s names undefined flag --%s", alias, canonical))
	}
	fs.Var(&flagAlias{canonical: target}, alias, "Alias of --"+canonical)
	HideFlagFromHelp(fs.Lookup(alias))
}

// FlagAliasTarget returns the name of the flag that f is an alias of.
func FlagAliasTarget(f *flag.Flag) (string, bool) {
	if f == nil {
		return "", false
	}
	alias, ok := f.Value.(*flagAlias)
	if !ok || alias.canonical == nil {
		return "", false
	}
	return alias.canonical.Name, true
}
