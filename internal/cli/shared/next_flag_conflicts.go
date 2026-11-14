package shared

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
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

// WrapNextLimitConflicts makes every command in the tree that binds both --next
// and --limit reject the pair, since a links.next URL is requested verbatim and
// would silently drop --limit. The diagnostic goes to d's stderr.
func WrapNextLimitConflicts(cmd *ffcli.Command, d *Deps) {
	wrapNextLimitConflicts(cmd, d, "")
}

func wrapNextLimitConflicts(cmd *ffcli.Command, d *Deps, parent string) {
	path := strings.TrimSpace(parent + " " + cmd.Name)
	for _, sub := range cmd.Subcommands {
		wrapNextLimitConflicts(sub, d, path)
	}

	fs := cmd.FlagSet
	if cmd.Exec == nil || fs == nil || fs.Lookup("next") == nil || fs.Lookup("limit") == nil {
		return
	}
	exec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		// An invalid --next keeps the command's own, more specific diagnostic.
		if next := fs.Lookup("next").Value.String(); ValidateNextURL(next) == nil {
			if err := d.RejectNextFlagConflicts(fs, next, path, "limit"); err != nil {
				return err
			}
		}
		return exec(ctx, args)
	}
}
