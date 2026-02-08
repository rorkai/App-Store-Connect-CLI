package encryption

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// EncryptionDeclarationsAppCommand returns the declarations app subcommand group.
func EncryptionDeclarationsAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("encryption declarations app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc encryption declarations app <subcommand> [flags]",
		ShortHelp:  "Access the app for an encryption declaration.",
		LongHelp: `Access the app for an encryption declaration.

Examples:
  asc encryption declarations app get --id "DECL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			EncryptionDeclarationsAppGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// EncryptionDeclarationsAppGetCommand returns the get subcommand for declaration apps.
func EncryptionDeclarationsAppGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("encryption declarations app get", flag.ExitOnError)

	declarationID := fs.String("id", "", "Encryption declaration ID (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc encryption declarations app get --id \"DECL_ID\"",
		ShortHelp:  "Get the app for an encryption declaration.",
		LongHelp: `Get the app for an encryption declaration.

Examples:
  asc encryption declarations app get --id "DECL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*declarationID)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("encryption declarations app get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppEncryptionDeclarationApp(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("encryption declarations app get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// EncryptionDeclarationsDeclarationDocumentCommand returns the declaration document subcommand group.
func EncryptionDeclarationsDeclarationDocumentCommand() *ffcli.Command {
	fs := flag.NewFlagSet("encryption declarations app-encryption-declaration-document", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-encryption-declaration-document",
		ShortUsage: "asc encryption declarations app-encryption-declaration-document <subcommand> [flags]",
		ShortHelp:  "Access the document for an encryption declaration.",
		LongHelp: `Access the document for an encryption declaration.

Examples:
  asc encryption declarations app-encryption-declaration-document get --id "DECL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			EncryptionDeclarationsDeclarationDocumentGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// EncryptionDeclarationsDeclarationDocumentGetCommand returns the get subcommand for declaration documents.
func EncryptionDeclarationsDeclarationDocumentGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("encryption declarations app-encryption-declaration-document get", flag.ExitOnError)

	declarationID := fs.String("id", "", "Encryption declaration ID (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc encryption declarations app-encryption-declaration-document get --id \"DECL_ID\"",
		ShortHelp:  "Get the document for an encryption declaration.",
		LongHelp: `Get the document for an encryption declaration.

Examples:
  asc encryption declarations app-encryption-declaration-document get --id "DECL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*declarationID)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("encryption declarations app-encryption-declaration-document get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppEncryptionDeclarationDocumentForDeclaration(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("encryption declarations app-encryption-declaration-document get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
