package betabuildlocalizations

import (
	"context"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func deprecatedBetaBuildLocalizationsLeafCommand(cmd *ffcli.Command, replacement, warning string) *ffcli.Command {
	if cmd == nil {
		return nil
	}

	clone := *cmd
	if replacement != "" {
		clone.ShortHelp = fmt.Sprintf("DEPRECATED: use `%s`.", replacement)
		clone.LongHelp = fmt.Sprintf("Deprecated compatibility alias for `%s`.\n\n%s", replacement, cmd.LongHelp)
	} else {
		clone.ShortHelp = "DEPRECATED: legacy-only compatibility path."
		clone.LongHelp = "Deprecated compatibility path retained during migration.\n\n" + cmd.LongHelp
	}

	origExec := cmd.Exec
	clone.Exec = func(ctx context.Context, args []string) error {
		fmt.Fprintln(os.Stderr, warning)
		return origExec(ctx, args)
	}

	return &clone
}
