package shared

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DeprecatedStringFlagAlias holds a hidden compatibility spelling for a
// canonical string flag and records whether callers supplied it.
type DeprecatedStringFlagAlias struct {
	value         string
	set           bool
	flagSet       *flag.FlagSet
	aliasName     string
	canonicalName string
}

// String returns the compatibility flag value for flag.Value formatting.
func (f *DeprecatedStringFlagAlias) String() string {
	if f == nil {
		return ""
	}
	return f.value
}

// Set records that the compatibility spelling was explicitly supplied.
func (f *DeprecatedStringFlagAlias) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}

// BindDeprecatedStringFlagAlias accepts a compatibility spelling without
// advertising it as part of the command's canonical interface.
func BindDeprecatedStringFlagAlias(fs *flag.FlagSet, aliasName, canonicalName string) *DeprecatedStringFlagAlias {
	alias := &DeprecatedStringFlagAlias{
		flagSet:       fs,
		aliasName:     strings.TrimSpace(aliasName),
		canonicalName: strings.TrimSpace(canonicalName),
	}
	fs.Var(alias, alias.aliasName, fmt.Sprintf("DEPRECATED: use --%s", alias.canonicalName))
	HideFlagFromHelp(fs.Lookup(alias.aliasName))
	return alias
}

// Apply copies a supplied alias into the canonical value, warns about the
// migration path, and rejects conflicting values before command side effects.
func (f *DeprecatedStringFlagAlias) Apply(canonical *string) error {
	if f == nil || !f.set {
		return nil
	}

	aliasValue := strings.TrimSpace(f.value)
	canonicalValue := ""
	if canonical != nil {
		canonicalValue = strings.TrimSpace(*canonical)
	}
	fmt.Fprintf(os.Stderr, "Warning: `--%s` is deprecated. Use `--%s`.\n", f.aliasName, f.canonicalName)
	canonicalSet := f.canonicalWasSet()
	if canonical != nil && (canonicalSet || canonicalValue != "") && canonicalValue != aliasValue {
		return UsageErrorf("--%s conflicts with --%s; use only --%s", f.aliasName, f.canonicalName, f.canonicalName)
	}
	if canonical != nil && !canonicalSet && canonicalValue == "" {
		*canonical = aliasValue
	}

	return nil
}

func (f *DeprecatedStringFlagAlias) canonicalWasSet() bool {
	if f == nil || f.flagSet == nil {
		return false
	}

	set := false
	f.flagSet.Visit(func(flag *flag.Flag) {
		if flag.Name == f.canonicalName {
			set = true
		}
	})
	return set
}

// DeprecatedIntFlagAlias holds a hidden compatibility spelling for a
// canonical integer flag and records whether callers supplied it.
type DeprecatedIntFlagAlias struct {
	value         int
	set           bool
	flagSet       *flag.FlagSet
	aliasName     string
	canonicalName string
}

// String returns the compatibility flag value for flag.Value formatting.
func (f *DeprecatedIntFlagAlias) String() string {
	if f == nil {
		return "0"
	}
	return strconv.Itoa(f.value)
}

// Set records that the compatibility spelling was explicitly supplied.
func (f *DeprecatedIntFlagAlias) Set(value string) error {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	f.value = parsed
	f.set = true
	return nil
}

// BindDeprecatedIntFlagAlias accepts a compatibility spelling without
// advertising it as part of the command's canonical interface.
func BindDeprecatedIntFlagAlias(fs *flag.FlagSet, aliasName, canonicalName string) *DeprecatedIntFlagAlias {
	alias := &DeprecatedIntFlagAlias{
		flagSet:       fs,
		aliasName:     strings.TrimSpace(aliasName),
		canonicalName: strings.TrimSpace(canonicalName),
	}
	fs.Var(alias, alias.aliasName, fmt.Sprintf("DEPRECATED: use --%s", alias.canonicalName))
	HideFlagFromHelp(fs.Lookup(alias.aliasName))
	return alias
}

// Apply copies a supplied alias into the canonical value, warns about the
// migration path, and rejects dual spelling before command side effects.
func (f *DeprecatedIntFlagAlias) Apply(canonical *int) error {
	if f == nil || !f.set {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Warning: `--%s` is deprecated. Use `--%s`.\n", f.aliasName, f.canonicalName)
	if f.canonicalWasSet() {
		return UsageErrorf("--%s conflicts with --%s; use only --%s", f.aliasName, f.canonicalName, f.canonicalName)
	}
	if canonical != nil {
		*canonical = f.value
	}
	return nil
}

// WasProvided reports whether the compatibility spelling was explicitly set.
func (f *DeprecatedIntFlagAlias) WasProvided() bool {
	return f != nil && f.set
}

func (f *DeprecatedIntFlagAlias) canonicalWasSet() bool {
	if f == nil || f.flagSet == nil {
		return false
	}

	set := false
	f.flagSet.Visit(func(parsed *flag.Flag) {
		if parsed.Name == f.canonicalName {
			set = true
		}
	})
	return set
}
