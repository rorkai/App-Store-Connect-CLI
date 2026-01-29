package server

import "github.com/peterbourgon/ff/v3/ffcli"

// Command returns the server API command group.
func Command() *ffcli.Command {
	return ServerCommand()
}
