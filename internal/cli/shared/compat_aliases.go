package shared

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

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
	clone.Name = name
	clone.ShortUsage = shortUsage
	clone.ShortHelp = fmt.Sprintf("DEPRECATED: use `%s`.", newCommand)
	clone.LongHelp = fmt.Sprintf("Deprecated compatibility alias for `%s`.", newCommand)
	clone.UsageFunc = DeprecatedUsageFunc

	origExec := cmd.Exec
	clone.Exec = func(ctx context.Context, args []string) error {
		fmt.Fprintln(os.Stderr, warning)
		return origExec(ctx, args)
	}

	return &clone
}
