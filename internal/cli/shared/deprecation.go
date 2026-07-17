package shared

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// DeprecatedCommand marks a command as deprecated while preserving its
// flags and execution behavior during the migration window.
func DeprecatedCommand(cmd *ffcli.Command, oldCommand, replacement string) *ffcli.Command {
	guidance := fmt.Sprintf("Use `%s`.", replacement)
	return DeprecatedCommandWithGuidance(cmd, oldCommand, guidance)
}

// DeprecatedCommandWithGuidance marks a command as deprecated when the
// migration needs more detail than a single replacement invocation.
func DeprecatedCommandWithGuidance(cmd *ffcli.Command, oldCommand, guidance string) *ffcli.Command {
	if cmd == nil {
		return nil
	}

	message := fmt.Sprintf("App Store Connect API 4.4.1 replaced this resource. %s", strings.TrimSpace(guidance))
	helpMessage := message
	if strings.HasPrefix(strings.TrimSpace(cmd.ShortHelp), "[experimental]") {
		helpMessage = "[experimental] " + message
	}
	cmd.ShortHelp = "DEPRECATED: " + helpMessage
	if longHelp := strings.TrimSpace(cmd.LongHelp); longHelp != "" {
		cmd.LongHelp = "DEPRECATED: " + helpMessage + "\n\n" + longHelp
	} else {
		cmd.LongHelp = "DEPRECATED: " + helpMessage
	}

	originalExec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		fmt.Fprintf(os.Stderr, "Warning: `%s` is deprecated by App Store Connect API 4.4.1. %s\n", oldCommand, strings.TrimSpace(guidance))
		if originalExec == nil {
			return nil
		}
		return originalExec(ctx, args)
	}
	return cmd
}
