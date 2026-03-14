package shared

import (
	"context"
	"errors"
	"flag"
	"strings"
	"sync"

	"github.com/peterbourgon/ff/v3/ffcli"
)

var hiddenCommandHelpRegistry struct {
	sync.RWMutex
	flags map[*flag.Flag]struct{}
}

func init() {
	hiddenCommandHelpRegistry.flags = make(map[*flag.Flag]struct{})
}

// RewriteCommandTreePath rewrites usage/help path prefixes for an existing command tree.
func RewriteCommandTreePath(cmd *ffcli.Command, currentPrefix, replacementPrefix string) *ffcli.Command {
	if cmd == nil || currentPrefix == "" || replacementPrefix == "" {
		return cmd
	}

	rewriteCommandTree(cmd, func(node *ffcli.Command) {
		originalUsage := strings.TrimSpace(node.ShortUsage)
		rewrittenUsage := originalUsage
		if strings.TrimSpace(node.ShortUsage) != "" {
			rewrittenUsage = strings.ReplaceAll(node.ShortUsage, currentPrefix, replacementPrefix)
			node.ShortUsage = rewrittenUsage
		}
		if strings.TrimSpace(node.LongHelp) != "" {
			node.LongHelp = strings.ReplaceAll(node.LongHelp, currentPrefix, replacementPrefix)
		}
		if node.Exec != nil {
			currentErrorPrefix := commandErrorPrefixFromUsage(originalUsage)
			replacementErrorPrefix := commandErrorPrefixFromUsage(rewrittenUsage)
			if currentErrorPrefix != "" && replacementErrorPrefix != "" && currentErrorPrefix != replacementErrorPrefix {
				originalExec := node.Exec
				node.Exec = func(ctx context.Context, args []string) error {
					err := originalExec(ctx, args)
					if err == nil || errors.Is(err, flag.ErrHelp) {
						return err
					}
					return rewriteCommandErrorPrefix(err, currentErrorPrefix, replacementErrorPrefix)
				}
			}
		}
	})

	return cmd
}

// HideFlagFromHelp hides a flag from command help output while keeping it accepted at runtime.
func HideFlagFromHelp(f *flag.Flag) {
	if f == nil {
		return
	}
	hiddenCommandHelpRegistry.Lock()
	hiddenCommandHelpRegistry.flags[f] = struct{}{}
	hiddenCommandHelpRegistry.Unlock()
}

// VisibleHelpFlags returns the flags that should appear in help output.
func VisibleHelpFlags(fs *flag.FlagSet) []*flag.Flag {
	if fs == nil {
		return nil
	}

	hiddenCommandHelpRegistry.RLock()
	defer hiddenCommandHelpRegistry.RUnlock()

	visible := []*flag.Flag{}
	fs.VisitAll(func(f *flag.Flag) {
		if _, hidden := hiddenCommandHelpRegistry.flags[f]; hidden {
			return
		}
		visible = append(visible, f)
	})
	return visible
}

func rewriteCommandTree(cmd *ffcli.Command, visit func(node *ffcli.Command)) {
	if cmd == nil {
		return
	}

	visit(cmd)
	for _, sub := range cmd.Subcommands {
		rewriteCommandTree(sub, visit)
	}
}

func commandPathFromUsage(usage string) string {
	usage = strings.TrimSpace(usage)
	if usage == "" {
		return ""
	}

	tokens := strings.Fields(usage)
	if len(tokens) == 0 {
		return ""
	}

	path := make([]string, 0, len(tokens))
	for _, token := range tokens {
		switch {
		case strings.HasPrefix(token, "--"):
			return strings.Join(path, " ")
		case strings.HasPrefix(token, "["):
			return strings.Join(path, " ")
		case strings.HasPrefix(token, "<"):
			return strings.Join(path, " ")
		default:
			path = append(path, token)
		}
	}

	return strings.Join(path, " ")
}

func commandErrorPrefixFromUsage(usage string) string {
	path := commandPathFromUsage(usage)
	path = strings.TrimSpace(strings.TrimPrefix(path, "asc "))
	return path
}

type rewrittenCommandError struct {
	err               error
	currentPrefix     string
	replacementPrefix string
}

func (e *rewrittenCommandError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return strings.Replace(e.err.Error(), e.currentPrefix, e.replacementPrefix, 1)
}

func (e *rewrittenCommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func rewriteCommandErrorPrefix(err error, currentPrefix, replacementPrefix string) error {
	if err == nil || currentPrefix == "" || replacementPrefix == "" {
		return err
	}
	if !strings.HasPrefix(err.Error(), currentPrefix) {
		return err
	}
	return &rewrittenCommandError{
		err:               err,
		currentPrefix:     currentPrefix,
		replacementPrefix: replacementPrefix,
	}
}
