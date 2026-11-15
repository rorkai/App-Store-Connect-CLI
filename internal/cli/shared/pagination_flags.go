package shared

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
)

// NextFlagUsage and PaginateFlagUsage are the --next and --paginate help lines
// of list commands; LimitFlagUsage gives their --limit help line.
const (
	NextFlagUsage     = "Fetch next page using a links.next URL"
	PaginateFlagUsage = "Automatically fetch all pages (aggregate results)"
)

// LimitFlagUsage returns the --limit help line of a list command whose pages
// hold at most limitMax results.
func LimitFlagUsage(limitMax int) string {
	return fmt.Sprintf("Maximum results per page (1-%d)", limitMax)
}

// ValidatePaginationFlags reports the first --limit or --next mistake of a
// list command, in the order every list command checks them: a --limit
// outside 1..limitMax (0 leaves it unset), a --next value that is not an App
// Store Connect URL, then --limit or a flag in nextConflicts given together
// with --next, which is requested verbatim. Each message starts with command,
// the command path without "asc ", after the command-path rewrites carried by
// ctx. Run it before the command's required-flag and filter checks.
func ValidatePaginationFlags(ctx context.Context, fs *flag.FlagSet, command string, limitMax int, nextConflicts ...string) error {
	return validatePaginationFlags(ctx, os.Stderr, fs, command, limitMax, nextConflicts...)
}

// ValidatePaginationFlags is ValidatePaginationFlags with the "Error:" line
// written to d's stderr.
func (d *Deps) ValidatePaginationFlags(ctx context.Context, fs *flag.FlagSet, command string, limitMax int, nextConflicts ...string) error {
	return validatePaginationFlags(ctx, d.IO.ErrOut, fs, command, limitMax, nextConflicts...)
}

func validatePaginationFlags(ctx context.Context, stderr io.Writer, fs *flag.FlagSet, command string, limitMax int, nextConflicts ...string) error {
	command = RewriteUsageMessage(ctx, command)
	if limit, _ := flagValue(fs, "limit").(int); limit != 0 && (limit < 1 || limit > limitMax) {
		return WithDiagnostic(usageErrorfTo(stderr, "%s: --limit must be between 1 and %d", command, limitMax), DiagnosticInvalidInput, "--limit")
	}
	next, _ := flagValue(fs, "next").(string)
	if err := validateNextURL(next); err != nil {
		return WithDiagnostic(usageErrorfTo(stderr, "%s: %v", command, err), DiagnosticInvalidInput, "--next")
	}
	return rejectNextFlagConflicts(stderr, fs, next, command, append([]string{"limit"}, nextConflicts...)...)
}

func flagValue(fs *flag.FlagSet, name string) any {
	f := fs.Lookup(name)
	if f == nil {
		return nil
	}
	getter, ok := f.Value.(flag.Getter)
	if !ok {
		return nil
	}
	return getter.Get()
}
