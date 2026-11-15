package initcmd

import (
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/docs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewInitCommand returns the root init command.
func NewInitCommand(d *shared.Deps) *ffcli.Command {
	return docs.NewInitReferenceCommand(
		d,
		"init",
		"init",
		"asc init [flags]",
		"Initialize asc helper docs in the current repo.",
		`Initialize asc helper docs in the current repo.

Examples:
  asc init
  asc init --path ./ASC.md
  asc init --force --link=false`,
		"init",
	)
}
