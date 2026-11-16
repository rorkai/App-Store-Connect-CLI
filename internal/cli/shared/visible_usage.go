package shared

import (
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func visibleUsage(stderrTTY bool, c *ffcli.Command) string {
	clone := *c
	if len(c.Subcommands) > 0 {
		visible := make([]*ffcli.Command, 0, len(c.Subcommands))
		for _, sub := range c.Subcommands {
			if sub == nil {
				continue
			}
			shortHelp := strings.TrimSpace(sub.ShortHelp)
			if strings.HasPrefix(shortHelp, "DEPRECATED:") || strings.HasPrefix(shortHelp, "REMOVED:") {
				continue
			}
			visible = append(visible, sub)
		}
		clone.Subcommands = visible
	}
	return defaultUsage(stderrTTY, &clone)
}
