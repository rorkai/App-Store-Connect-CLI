package subscriptions

import (
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func wrapSubscriptionsCommand(
	cmd *ffcli.Command,
	currentPrefix string,
	replacementPrefix string,
	newName string,
	newShortHelp string,
) *ffcli.Command {
	cmd = shared.RewriteCommandTreePath(cmd, currentPrefix, replacementPrefix)
	if cmd == nil {
		return nil
	}
	if strings.TrimSpace(newName) != "" {
		cmd.Name = newName
	}
	if strings.TrimSpace(newShortHelp) != "" {
		cmd.ShortHelp = newShortHelp
	}
	return cmd
}
