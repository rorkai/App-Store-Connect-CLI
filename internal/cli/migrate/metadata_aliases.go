package migrate

import (
	"context"
	"flag"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// MigrateMetadataCommand reports that the asc migrate metadata aliases were
// removed and names the asc metadata command to use instead.
func MigrateMetadataCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "metadata",
		ShortUsage: "asc migrate metadata <pull|push|validate>",
		ShortHelp:  "REMOVED: Use `asc metadata pull|push|validate`.",
		FlagSet:    flag.NewFlagSet("migrate metadata", flag.ExitOnError),
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			command := "metadata"
			if len(args) > 0 {
				switch sub := strings.TrimSpace(args[0]); sub {
				case "pull", "push", "validate":
					command += " " + sub
				}
			}
			return shared.WithDiagnostic(shared.UsageErrorf(
				"`asc migrate %s` was removed in 6.0.0; use `asc %s` instead",
				command, command,
			), shared.DiagnosticInvalidInput, "")
		},
	}
}
