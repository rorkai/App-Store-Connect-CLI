package shared

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

// ParseInterspersedFlags applies flags that appear after positional arguments
// to fs and returns the remaining positional arguments in order. The standard
// flag package stops at the first positional token, so commands with a
// positional payload call this from Exec to accept `asc cmd VALUE --flag`.
//
// A leading `--` was already consumed by flag parsing, so when the first
// remaining argument still starts with a dash it was escaped and the whole
// list is returned as positional arguments. A later `--` ends flag parsing.
func ParseInterspersedFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	if fs == nil || len(args) == 0 {
		return args, nil
	}
	if strings.HasPrefix(args[0], "-") {
		return args, nil
	}

	setter := NewInterspersedFlagSetter(fs)

	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}

		name, value, hasValue, isFlag := splitInterspersedFlagArg(arg)
		if !isFlag {
			positional = append(positional, arg)
			continue
		}

		f := fs.Lookup(name)
		if f == nil {
			if name == RootProfileFlagName {
				return nil, fmt.Errorf("`--profile` must appear before positional arguments")
			}
			return nil, fmt.Errorf("flag provided but not defined: --%s", name)
		}

		if isBoolFlagValue(f) && !hasValue {
			value, hasValue = "true", true
		}
		if !hasValue {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag needs an argument: --%s", name)
			}
			i++
			value = args[i]
		}

		if err := setter.Set(f, name, value); err != nil {
			return nil, err
		}
	}

	return positional, nil
}

func splitInterspersedFlagArg(arg string) (name, value string, hasValue, isFlag bool) {
	if arg == "" || arg == "-" || !strings.HasPrefix(arg, "-") {
		return "", "", false, false
	}

	trimmed := strings.TrimPrefix(arg, "--")
	if trimmed == arg {
		trimmed = strings.TrimPrefix(arg, "-")
	}
	if trimmed == "" {
		return "", "", false, false
	}

	name, value, hasValue = strings.Cut(trimmed, "=")
	if name == "" {
		return "", "", false, false
	}
	return name, value, hasValue, true
}

func isBoolFlagValue(f *flag.Flag) bool {
	if f == nil {
		return false
	}
	if boolFlag, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && boolFlag.IsBoolFlag() {
		return true
	}
	getter, ok := f.Value.(flag.Getter)
	if !ok {
		return false
	}
	_, ok = getter.Get().(bool)
	return ok
}

// InterspersedFlagSetter sets flags that appear after positional arguments,
// keeping an alias the same flag as its canonical spelling, as
// resolveFlagAliases makes it before parsing. A second spelling with another
// value is an error instead of a silent last-one-wins, and so is a value that
// differs from the one given before the positional arguments. The root
// rewrites an alias there to its canonical spelling, so `--data A POST PATH
// --body B` reaches the setter as --body twice. A repeatable flag adds every
// value under any spelling, so it has no conflict.
type InterspersedFlagSetter struct {
	fs    *flag.FlagSet
	given map[string]givenFlagValue
}

type givenFlagValue struct {
	spelling, value  string
	beforePositional bool
}

// NewInterspersedFlagSetter records the flags fs parsed before the first
// positional argument.
func NewInterspersedFlagSetter(fs *flag.FlagSet) *InterspersedFlagSetter {
	setter := &InterspersedFlagSetter{fs: fs, given: map[string]givenFlagValue{}}
	fs.Visit(func(f *flag.Flag) {
		setter.given[f.Name] = givenFlagValue{spelling: f.Name, value: f.Value.String(), beforePositional: true}
	})
	return setter
}

// Set applies value for f, given as --spelling after a positional argument.
func (s *InterspersedFlagSetter) Set(f *flag.Flag, spelling, value string) error {
	canonical := spelling
	if target, implied, ok := FlagAliasImpliedValue(f); ok {
		if enabled, err := strconv.ParseBool(value); err == nil && enabled {
			canonical, value = target, implied
		}
	} else if target, ok := FlagAliasTarget(f, value); ok {
		canonical = target
	}
	repeatable := IsRepeatableFlag(s.fs.Lookup(canonical))
	if previous, ok := s.given[canonical]; ok && !repeatable && previous.value != value {
		if previous.spelling != spelling {
			return fmt.Errorf("`--%s` and `--%s` are the same flag; pass one value", previous.spelling, spelling)
		}
		if previous.beforePositional {
			return fmt.Errorf("`--%s` is given before and after the positional arguments with different values%s; pass one value", canonical, s.aliasNote(canonical))
		}
	}
	s.given[canonical] = givenFlagValue{spelling: spelling, value: value}

	if err := s.fs.Set(canonical, value); err != nil {
		return fmt.Errorf("invalid value %q for --%s: %w", value, spelling, err)
	}
	return nil
}

// aliasNote names the aliases of canonical, which the root has already
// rewritten to canonical when they came before the positional arguments.
func (s *InterspersedFlagSetter) aliasNote(canonical string) string {
	var notes []string
	s.fs.VisitAll(func(f *flag.Flag) {
		if !IsFlagAlias(f) {
			return
		}
		if target, implied, ok := FlagAliasImpliedValue(f); ok {
			if target == canonical {
				notes = append(notes, fmt.Sprintf("`--%s` means `--%s %s`", f.Name, target, implied))
			}
			return
		}
		for _, target := range f.Value.(*flagAlias).targets {
			if target.flag.Name == canonical {
				notes = append(notes, fmt.Sprintf("`--%s` is the same flag", f.Name))
				return
			}
		}
	})
	if len(notes) == 0 {
		return ""
	}
	return " (" + strings.Join(notes, "; ") + ")"
}

// IsRepeatableFlag reports whether each value given for f adds to the
// earlier ones instead of replacing them.
func IsRepeatableFlag(f *flag.Flag) bool {
	if f == nil {
		return false
	}
	_, ok := f.Value.(*MultiStringFlag)
	return ok
}
