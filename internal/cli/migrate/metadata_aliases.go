package migrate

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	metadatacmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/metadata"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// MigrateMetadataCommand provides deprecated aliases for asc metadata workflows.
func MigrateMetadataCommand() *ffcli.Command {
	fs := flag.NewFlagSet("migrate metadata", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metadata",
		ShortUsage: "asc migrate metadata <pull|push|validate> [flags]",
		ShortHelp:  "DEPRECATED: Use `asc metadata pull|push|validate`. Removed in 6.0.0.",
		LongHelp: `DEPRECATED: Use ` + "`asc metadata pull|push|validate`" + `. ` + "`asc migrate metadata`" + ` will be removed in 6.0.0.

Replace each alias with the matching asc metadata command; flags are identical:
  asc migrate metadata pull ...     -> asc metadata pull ...
  asc migrate metadata push ...     -> asc metadata push ...
  asc migrate metadata validate ... -> asc metadata validate ...`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			deprecatedMetadataAlias(metadatacmd.MetadataPullCommand()),
			deprecatedMetadataAlias(metadatacmd.MetadataPushCommand()),
			deprecatedMetadataAlias(metadatacmd.MetadataValidateCommand()),
		},
		Exec: func(ctx context.Context, args []string) error {
			fmt.Fprintln(os.Stderr, "Warning: `asc migrate metadata` is deprecated and will be removed in 6.0.0. Use `asc metadata`.")
			return flag.ErrHelp
		},
	}
}

func deprecatedMetadataAlias(cmd *ffcli.Command) *ffcli.Command {
	replacement := "asc metadata " + cmd.Name
	notice := fmt.Sprintf("DEPRECATED: Use `%s`. Removed in 6.0.0.", replacement)
	cmd.ShortHelp = notice
	cmd.LongHelp = notice + "\n\n" + cmd.LongHelp

	exec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		fmt.Fprintf(os.Stderr, "Warning: `asc migrate metadata %s` is deprecated and will be removed in 6.0.0. Use `%s`.\n", cmd.Name, replacement)
		return exec(ctx, args)
	}
	return cmd
}
