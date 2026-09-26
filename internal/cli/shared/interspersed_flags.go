package shared

import (
	"flag"
	"fmt"
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
			return nil, fmt.Errorf("flag provided but not defined: -%s", name)
		}

		if isBoolFlagValue(f) && !hasValue {
			if err := fs.Set(name, "true"); err != nil {
				return nil, fmt.Errorf("invalid value %q for --%s: %w", "true", name, err)
			}
			continue
		}

		if !hasValue {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag needs an argument: --%s", name)
			}
			i++
			value = args[i]
		}

		if err := fs.Set(name, value); err != nil {
			return nil, fmt.Errorf("invalid value %q for --%s: %w", value, name, err)
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
