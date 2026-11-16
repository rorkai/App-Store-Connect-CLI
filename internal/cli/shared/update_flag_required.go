package shared

import (
	"flag"
	"fmt"
	"io"
	"reflect"
	"strings"
)

func updateFlagRequiredTo(stderr io.Writer, fs *flag.FlagSet, updateFlags ...any) error {
	names := make([]string, 0, len(updateFlags))
	for _, value := range updateFlags {
		names = append(names, "--"+boundFlagName(fs, value))
	}
	fmt.Fprintf(stderr, "Error: at least one update flag is required (%s)\n", strings.Join(names, ", "))
	parameter := ""
	if len(names) == 1 {
		parameter = names[0]
	}
	return MissingRequiredUsageError(parameter)
}

// wrappingValue is a flag.Value that stores the parsed value in a separate
// variable, the one its Bind function returns.
type wrappingValue interface {
	storage() any
}

func boundFlagName(fs *flag.FlagSet, value any) string {
	target := reflect.ValueOf(value)
	if target.Kind() != reflect.Pointer || target.IsNil() {
		panic(fmt.Sprintf("shared: update flag variable %T is not a pointer", value))
	}
	var names []string
	fs.VisitAll(func(f *flag.Flag) {
		var storage any = f.Value
		if wrapper, ok := f.Value.(wrappingValue); ok {
			storage = wrapper.storage()
		}
		bound := reflect.ValueOf(storage)
		if bound.Kind() == reflect.Pointer && bound.Pointer() == target.Pointer() {
			names = append(names, f.Name)
		}
	})
	if len(names) != 1 {
		panic(fmt.Sprintf("shared: %d flags in %q are bound to update flag variable %T, want 1", len(names), fs.Name(), value))
	}
	return names[0]
}
