package builds

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func deprecatedBuildsRelationshipsAliasCommand() *ffcli.Command {
	fs := BuildsRelationshipsCommand().FlagSet

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc builds links <subcommand> [flags]",
		ShortHelp:  "DEPRECATED: use `asc builds links ...`.",
		LongHelp:   "Deprecated compatibility alias for `asc builds links ...`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			shared.DeprecatedAliasLeafCommand(
				BuildsRelationshipsGetCommand(),
				"get",
				"asc builds links view --build \"BUILD_ID\" --type \"RELATIONSHIP\" [flags]",
				"asc builds links view",
				"Warning: `asc builds relationships get` is deprecated. Use `asc builds links view`.",
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
