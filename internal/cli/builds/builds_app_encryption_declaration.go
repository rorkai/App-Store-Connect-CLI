package builds

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BuildsAppEncryptionDeclarationCommand returns the builds app-encryption-declaration command group.
func BuildsAppEncryptionDeclarationCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-encryption-declaration", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-encryption-declaration",
		ShortUsage: "asc builds app-encryption-declaration <subcommand> [flags]",
		ShortHelp:  "Get the app encryption declaration for a build.",
		LongHelp: `Get the app encryption declaration for a build.

Examples:
  asc builds app-encryption-declaration get --id "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsAppEncryptionDeclarationGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsAppEncryptionDeclarationGetCommand returns the get subcommand.
func BuildsAppEncryptionDeclarationGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-encryption-declaration get", flag.ExitOnError)

	buildID := fs.String("id", "", "Build ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc builds app-encryption-declaration get --id \"BUILD_ID\"",
		ShortHelp:  "Get the encryption declaration for a build.",
		LongHelp: `Get the encryption declaration for a build.

Examples:
  asc builds app-encryption-declaration get --id "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds app-encryption-declaration get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildAppEncryptionDeclaration(requestCtx, buildValue)
			if err != nil {
				return fmt.Errorf("builds app-encryption-declaration get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
