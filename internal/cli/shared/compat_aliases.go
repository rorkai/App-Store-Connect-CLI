package shared

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/peterbourgon/ff/v3/ffcli"
)

type deprecatedAliasLeafMetadata struct {
	warning   string
	canonical *ffcli.Command
}

var deprecatedAliasLeafRegistry sync.Map

// VisibleUsageFunc renders command help while omitting deprecated aliases from
// nested subcommand listings. Root-level deprecated commands are already hidden
// elsewhere; this keeps nested canonical help focused on current surfaces.
func VisibleUsageFunc(c *ffcli.Command) string {
	clone := *c
	if len(c.Subcommands) > 0 {
		visible := make([]*ffcli.Command, 0, len(c.Subcommands))
		for _, sub := range c.Subcommands {
			if sub == nil {
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(sub.ShortHelp), "DEPRECATED:") {
				continue
			}
			visible = append(visible, sub)
		}
		clone.Subcommands = visible
	}
	return DefaultUsageFunc(&clone)
}

// DeprecatedAliasLeafCommand clones a canonical leaf command into a deprecated
// compatibility alias that warns and then delegates to the canonical Exec.
func DeprecatedAliasLeafCommand(cmd *ffcli.Command, name, shortUsage, newCommand, warning string) *ffcli.Command {
	if cmd == nil {
		return nil
	}

	clone := *cmd
	clone.FlagSet = cloneFlagSet(cmd.FlagSet)
	if clone.FlagSet != nil {
		renameFlagSetLastToken(clone.FlagSet, strings.TrimSpace(cmd.Name), name)
	}
	clone.Name = name
	clone.ShortUsage = shortUsage
	clone.ShortHelp = fmt.Sprintf("DEPRECATED: use `%s`.", newCommand)
	clone.LongHelp = fmt.Sprintf("Deprecated compatibility alias for `%s`.", newCommand)
	clone.UsageFunc = DeprecatedUsageFunc

	meta := &deprecatedAliasLeafMetadata{
		warning:   warning,
		canonical: cmd,
	}
	deprecatedAliasLeafRegistry.Store(&clone, meta)

	clone.Exec = func(ctx context.Context, args []string) error {
		fmt.Fprintln(os.Stderr, meta.warning)
		if meta.canonical == nil || meta.canonical.Exec == nil {
			return nil
		}
		return meta.canonical.Exec(ctx, args)
	}

	return &clone
}

func cloneFlagSet(fs *flag.FlagSet) *flag.FlagSet {
	if fs == nil {
		return nil
	}

	clone := flag.NewFlagSet(fs.Name(), fs.ErrorHandling())
	if output := fs.Output(); output != nil {
		clone.SetOutput(output)
	}
	clone.Usage = fs.Usage

	fs.VisitAll(func(f *flag.Flag) {
		clone.Var(f.Value, f.Name, f.Usage)
		if copied := clone.Lookup(f.Name); copied != nil {
			copied.DefValue = f.DefValue
			if flagHiddenFromHelp(f) {
				HideFlagFromHelp(copied)
			}
		}
	})

	return clone
}

func flagHiddenFromHelp(f *flag.Flag) bool {
	if f == nil {
		return false
	}

	hiddenCommandHelpRegistry.RLock()
	defer hiddenCommandHelpRegistry.RUnlock()

	_, hidden := hiddenCommandHelpRegistry.flags[f]
	return hidden
}

func rewriteDeprecatedAliasLeafWarnings(cmd *ffcli.Command, rewrite func(string) string) {
	if cmd == nil || rewrite == nil {
		return
	}

	if raw, ok := deprecatedAliasLeafRegistry.Load(cmd); ok {
		if meta, ok := raw.(*deprecatedAliasLeafMetadata); ok && meta != nil {
			meta.warning = rewrite(meta.warning)
		}
	}

	for _, sub := range cmd.Subcommands {
		rewriteDeprecatedAliasLeafWarnings(sub, rewrite)
	}
}
