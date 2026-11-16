package shared

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

func rejectNextFlagConflicts(stderr io.Writer, fs *flag.FlagSet, next, command string, names ...string) error {
	if strings.TrimSpace(next) == "" {
		return nil
	}

	provided := make(map[string]struct{}, len(names))
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = struct{}{}
	})
	for _, name := range names {
		if _, ok := provided[name]; ok {
			return WithDiagnostic(
				usageErrorTo(stderr, fmt.Sprintf("%s: --next cannot be combined with --%s", command, name)),
				DiagnosticConflictingInput,
				"--"+name,
			)
		}
	}

	return nil
}
