package prerelease

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// RemovedPreReleaseVersionsCommand keeps the old root path available only to
// print migration guidance for the canonical TestFlight surface.
func RemovedPreReleaseVersionsCommand() *ffcli.Command {
	cmd := PreReleaseVersionsCommand()
	configureRemovedPreReleaseTree(
		cmd,
		"asc pre-release-versions",
		"asc testflight pre-release",
		map[string]string{
			"pre-release-versions": "pre-release",
			"relationships":        "links",
			"get":                  "view",
		},
	)
	return cmd
}

func configureRemovedPreReleaseTree(cmd *ffcli.Command, oldPath, newPath string, nameRenames map[string]string) {
	if cmd == nil {
		return
	}

	if len(cmd.Subcommands) > 0 {
		cmd.ShortUsage = newPath + " <subcommand> [flags]"
	} else {
		cmd.ShortUsage = newPath + " [flags]"
	}

	if oldPath == "asc pre-release-versions" {
		cmd.ShortHelp = "DEPRECATED: use `asc testflight pre-release`."
	} else {
		cmd.ShortHelp = fmt.Sprintf("REMOVED: use `%s`.", newPath)
	}
	cmd.LongHelp = fmt.Sprintf("Removed legacy command. Use `%s` instead.", newPath)
	cmd.UsageFunc = shared.DeprecatedUsageFunc
	cmd.Exec = func(ctx context.Context, args []string) error {
		fmt.Fprintf(os.Stderr, "Error: `%s` was removed. Use `%s` instead.\n", oldPath, newPath)
		return flag.ErrHelp
	}

	for _, sub := range cmd.Subcommands {
		if sub == nil {
			continue
		}

		newChildName := sub.Name
		if renamed, ok := nameRenames[sub.Name]; ok {
			newChildName = renamed
		}

		configureRemovedPreReleaseTree(
			sub,
			oldPath+" "+sub.Name,
			newPath+" "+newChildName,
			nameRenames,
		)
	}
}
