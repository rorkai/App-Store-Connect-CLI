package mcpcmd

import (
	"context"
	"flag"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/mcp"
)

// MCPCommand returns the mcp command.
func MCPCommand(rootProvider func(version string) *ffcli.Command, version string) *ffcli.Command {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)

	commands := fs.String("commands", "", "Command groups to expose as tools (comma-separated, default: curated set, use 'all' for everything)")

	return &ffcli.Command{
		Name:       "mcp",
		ShortUsage: "asc mcp [flags]",
		ShortHelp:  "Start a Model Context Protocol (MCP) server over stdio.",
		LongHelp: `Start a Model Context Protocol (MCP) server over stdio.

The MCP server exposes asc CLI commands as typed tools that AI agents can
discover and invoke via JSON-RPC 2.0 over stdin/stdout. This eliminates
shell escaping, argument construction, and output parsing errors.

By default, a curated set of ~30 high-value command groups (~200 tools) is
exposed. Commands not in the curated set can still be reached via the
asc_run catch-all tool. Use --commands to customize which groups are exposed.

Examples:
  asc mcp                                          # Curated default tools
  asc mcp --commands all                            # All 1000+ tools
  asc mcp --commands apps,builds,testflight,submit  # Custom selection
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize"}' | asc mcp`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			root := rootProvider(version)

			mcp.CloneRoot = func(v string) *ffcli.Command {
				return rootProvider(v)
			}

			var groups []string
			if cmdVal := strings.TrimSpace(*commands); cmdVal != "" {
				for _, g := range strings.Split(cmdVal, ",") {
					if trimmed := strings.TrimSpace(g); trimmed != "" {
						groups = append(groups, trimmed)
					}
				}
			}

			server := mcp.NewServer(root, version, groups)
			return server.Run(ctx)
		},
	}
}
