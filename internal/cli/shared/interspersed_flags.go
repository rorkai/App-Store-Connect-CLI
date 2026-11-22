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

	// given records, per canonical flag, the spelling and value it was set
	// with, so an alias after a positional argument is the same flag as its
	// canonical spelling, as resolveFlagAliases makes it before parsing.
	type spelledValue struct{ spelling, value string }
	given := map[string]spelledValue{}
	fs.Visit(func(f *flag.Flag) {
		given[f.Name] = spelledValue{spelling: f.Name, value: f.Value.String()}
	})

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

		spelling, canonical := name, name
		if target, implied, ok := FlagAliasImpliedValue(f); ok {
			if enabled, err := strconv.ParseBool(value); err == nil && enabled {
				canonical, value = target, implied
			}
		} else if target, ok := FlagAliasTarget(f, value); ok {
			canonical = target
		}
		if previous, ok := given[canonical]; ok && previous.spelling != spelling && previous.value != value {
			return nil, fmt.Errorf("`--%s` and `--%s` are the same flag; pass one value", previous.spelling, spelling)
		}
		given[canonical] = spelledValue{spelling: spelling, value: value}

		if err := fs.Set(canonical, value); err != nil {
			return nil, fmt.Errorf("invalid value %q for --%s: %w", value, spelling, err)
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
