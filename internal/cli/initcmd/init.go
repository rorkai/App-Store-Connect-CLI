package initcmd

import (
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/docs"
)

// InitCommand returns the root init command.
func InitCommand() *ffcli.Command {
	return docs.NewInitReferenceCommand(
		"init",
		"init",
		"asc init [flags]",
		"Initialize ASC helper docs in the current repo.",
		`Initialize ASC helper docs in the current repo.

Examples:
  asc init
  asc init --path ./ASC.md
  asc init --force --link=false`,
		"init",
	)
}
