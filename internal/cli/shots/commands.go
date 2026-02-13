package shots

import "github.com/peterbourgon/ff/v3/ffcli"

// Command returns the shots command group.
func Command() *ffcli.Command {
	return ShotsCommand()
}
