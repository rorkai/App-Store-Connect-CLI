package shared

import (
	"flag"
	"io"
	"os"
	"strconv"
	"strings"
)

// RecoverBoolFlagTailArgs reparses leftover args when the standard flag parser
// stops after a space-separated boolean value for a bool flag. This preserves
// support for `--flag`, `--flag=true`, and `--flag=false` while also
// recovering `--flag true` / `--flag false` for commands that don't accept
// positional args.
func RecoverBoolFlagTailArgs(original *flag.FlagSet, args []string, currentValue *bool) error {
	return recoverBoolFlagTailArgsTo(os.Stderr, original, args, currentValue)
}

// RecoverBoolFlagTailArgs is RecoverBoolFlagTailArgs with the "Error:" line
// written to d's stderr.
func (d *Deps) RecoverBoolFlagTailArgs(original *flag.FlagSet, args []string, currentValue *bool) error {
	return recoverBoolFlagTailArgsTo(d.IO.ErrOut, original, args, currentValue)
}

func recoverBoolFlagTailArgsTo(stderr io.Writer, original *flag.FlagSet, args []string, currentValue *bool) error {
	if original == nil || len(args) == 0 {
		return nil
	}

	remaining := args
	if currentValue != nil && *currentValue {
		if parsed, err := strconv.ParseBool(strings.TrimSpace(args[0])); err == nil {
			*currentValue = parsed
			remaining = args[1:]
		}
	}

	if len(remaining) == 0 {
		return nil
	}

	reparsed := flag.NewFlagSet(original.Name(), flag.ContinueOnError)
	reparsed.SetOutput(io.Discard)
	original.VisitAll(func(f *flag.Flag) {
		reparsed.Var(f.Value, f.Name, f.Usage)
	})

	if err := reparsed.Parse(remaining); err != nil {
		return usageErrorTo(stderr, err.Error())
	}
	if extras := reparsed.Args(); len(extras) > 0 {
		return usageErrorfTo(stderr, "unexpected argument(s): %s", strings.Join(extras, " "))
	}
	return nil
}
