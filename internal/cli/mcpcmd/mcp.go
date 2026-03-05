package mcpcmd

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/mcp"
)

// MCPCommand returns the mcp command.
func MCPCommand(rootProvider func(version string) *ffcli.Command, version string) *ffcli.Command {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "mcp",
		ShortUsage: "asc mcp",
		ShortHelp:  "Start a Model Context Protocol (MCP) server over stdio.",
		LongHelp: `Start a Model Context Protocol (MCP) server over stdio.

The MCP server exposes every asc CLI command as a typed tool that AI agents
can discover and invoke via JSON-RPC 2.0 over stdin/stdout. This eliminates
shell escaping, argument construction, and output parsing errors.

Agents call tools/list to discover available commands (with typed parameter
schemas derived from CLI flags) and tools/call to invoke them, receiving
structured JSON output.

Examples:
  asc mcp
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize"}' | asc mcp`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			root := rootProvider(version)

			mcp.CloneRoot = func(v string) *ffcli.Command {
				return rootProvider(v)
			}

			server := mcp.NewServer(root, version)
			return server.Run(ctx)
		},
	}
}
