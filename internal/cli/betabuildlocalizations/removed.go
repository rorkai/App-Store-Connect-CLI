package betabuildlocalizations

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BetaBuildLocalizationsCommand keeps the legacy root path available only to
// print removal guidance toward the canonical builds test-notes surface.
func BetaBuildLocalizationsCommand() *ffcli.Command {
	cmd := legacyBetaBuildLocalizationsCommand()
	configureRemovedBetaBuildLocalizationsTree(
		cmd,
		"asc beta-build-localizations",
		"asc builds test-notes",
	)
	return cmd
}

func configureRemovedBetaBuildLocalizationsTree(cmd *ffcli.Command, oldPath, newPath string) {
	if cmd == nil {
		return
	}

	if len(cmd.Subcommands) > 0 {
		if newPath != "" {
			cmd.ShortUsage = newPath + " <subcommand> [flags]"
		} else {
			cmd.ShortUsage = oldPath + " <subcommand> [flags]"
		}
	} else if newPath != "" {
		cmd.ShortUsage = newPath + " [flags]"
	}

	if newPath != "" {
		cmd.ShortHelp = fmt.Sprintf("DEPRECATED: removed; use `%s`.", newPath)
		cmd.LongHelp = fmt.Sprintf("Removed legacy command. Use `%s` instead.", newPath)
	} else {
		cmd.ShortHelp = "DEPRECATED: removed; no canonical replacement."
		cmd.LongHelp = "Removed legacy command. No canonical replacement exists yet."
	}
	cmd.UsageFunc = shared.DeprecatedUsageFunc
	cmd.Exec = func(ctx context.Context, args []string) error {
		if newPath != "" {
			fmt.Fprintf(os.Stderr, "Error: `%s` was removed. Use `%s` instead.\n", oldPath, newPath)
		} else {
			fmt.Fprintf(os.Stderr, "Error: `%s` was removed. No canonical replacement exists yet.\n", oldPath)
		}
		return flag.ErrHelp
	}

	for _, sub := range cmd.Subcommands {
		if sub == nil {
			continue
		}

		nextPath, nextReplacement := removedBetaBuildLocalizationsChildPath(oldPath, newPath, sub.Name)
		configureRemovedBetaBuildLocalizationsTree(sub, nextPath, nextReplacement)
	}
}

func removedBetaBuildLocalizationsChildPath(oldPath, newPath, childName string) (string, string) {
	nextOldPath := oldPath + " " + childName

	if oldPath == "asc beta-build-localizations build" {
		return nextOldPath, ""
	}

	if childName == "build" {
		return nextOldPath, ""
	}

	if newPath == "" {
		return nextOldPath, ""
	}

	nextName := childName
	if childName == "get" {
		nextName = "view"
	}

	return nextOldPath, newPath + " " + nextName
}
