package shared

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// RejectNextFlagConflicts rejects explicitly provided query flags when --next
// supplies the complete continuation request, including its query string.
func RejectNextFlagConflicts(fs *flag.FlagSet, next, command string, names ...string) error {
	return rejectNextFlagConflicts(os.Stderr, fs, next, command, names...)
}

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
