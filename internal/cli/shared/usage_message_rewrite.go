package shared

import (
	"context"
	"fmt"
	"strings"
)

// UsageMessageRewrite renames a command path inside a usage diagnostic.
//
// Several command trees are re-parented after construction, by
// RewriteCommandTreePath here and by the TestFlight command wrappers. Both
// rewrite help text and, for a command that returns an ordinary error, the
// message that error renders, so an operator sees the command path they typed
// rather than the one the command was originally written under.
//
// A usage error cannot travel that path. UsageError writes "Error: <message>"
// to stderr while the command is still running and returns an error wrapping
// flag.ErrHelp, which both rewriters skip and neither can un-print. A command
// under a re-parented tree therefore installs its rewrites on the context, and
// UsageErrorCtx applies them before the diagnostic reaches stderr. Call sites
// keep the literal command path they were written with, which the raw
// per-command tests assert directly.
type UsageMessageRewrite struct {
	// Old is the text to replace. A rewrite with an empty Old or New, or with
	// Old equal to New, is ignored.
	Old string
	// New is the replacement text.
	New string
	// AnchoredPrefix replaces only a single leading occurrence of Old, matching
	// rewriteCommandErrorPrefix. Without it every occurrence is replaced,
	// matching the TestFlight wrappers' text replacements.
	AnchoredPrefix bool
}

func (r UsageMessageRewrite) apply(message string) string {
	if r.Old == "" || r.New == "" || r.Old == r.New {
		return message
	}
	if r.AnchoredPrefix {
		if !strings.HasPrefix(message, r.Old) {
			return message
		}
		return strings.Replace(message, r.Old, r.New, 1)
	}
	return strings.ReplaceAll(message, r.Old, r.New)
}

type usageMessageRewriteKey struct{}

// ContextWithUsageMessageRewrites returns a context carrying rewrites in
// addition to any already present.
//
// Wrappers install their rewrites while a command is being entered, outermost
// first, whereas an error is rewritten on the way out, innermost first. The
// most recently installed rewrites are therefore applied first, which keeps a
// nested tree rendering exactly what the post-hoc error rewriters render.
func ContextWithUsageMessageRewrites(ctx context.Context, rewrites ...UsageMessageRewrite) context.Context {
	if ctx == nil || len(rewrites) == 0 {
		return ctx
	}

	existing := usageMessageRewritesFromContext(ctx)
	combined := make([]UsageMessageRewrite, 0, len(rewrites)+len(existing))
	combined = append(combined, rewrites...)
	combined = append(combined, existing...)
	return context.WithValue(ctx, usageMessageRewriteKey{}, combined)
}

func usageMessageRewritesFromContext(ctx context.Context) []UsageMessageRewrite {
	if ctx == nil {
		return nil
	}
	rewrites, _ := ctx.Value(usageMessageRewriteKey{}).([]UsageMessageRewrite)
	return rewrites
}

// RewriteUsageMessage applies the command-path rewrites carried by ctx to
// message. A context without rewrites returns message unchanged.
func RewriteUsageMessage(ctx context.Context, message string) string {
	for _, rewrite := range usageMessageRewritesFromContext(ctx) {
		message = rewrite.apply(message)
	}
	return message
}

// UsageErrorCtx is UsageError with ctx's command-path rewrites applied to the
// message. Commands under a re-parented tree should prefer it so the printed
// diagnostic names the command path the operator invoked.
func UsageErrorCtx(ctx context.Context, message string) error {
	return UsageError(RewriteUsageMessage(ctx, message))
}

// UsageErrorfCtx formats and returns a usage-class validation error with ctx's
// command-path rewrites applied.
func UsageErrorfCtx(ctx context.Context, format string, args ...any) error {
	return UsageErrorCtx(ctx, fmt.Sprintf(format, args...))
}

// UsageErrorCtx is UsageErrorCtx with the "Error:" line written to d's
// stderr.
func (d *Deps) UsageErrorCtx(ctx context.Context, message string) error {
	return d.UsageError(RewriteUsageMessage(ctx, message))
}

// UsageErrorfCtx is UsageErrorfCtx with the "Error:" line written to d's
// stderr.
func (d *Deps) UsageErrorfCtx(ctx context.Context, format string, args ...any) error {
	return d.UsageErrorCtx(ctx, fmt.Sprintf(format, args...))
}
