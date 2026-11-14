package encryption

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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
  asc encryption declarations app view --id "DECL_ID"`,
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
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "encryption declarations app view",
		Name:        "view",
		ShortUsage:  "asc encryption declarations app view --id \"DECL_ID\"",
		ShortHelp:   "View the app for an encryption declaration.",
		LongHelp: `View the app for an encryption declaration.

Examples:
  asc encryption declarations app view --id "DECL_ID"`,
		IDUsage:            "Encryption declaration ID (required)",
		IDType:             "appEncryptionDeclarations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "encryption declarations app view",
		FetchErrorPrefix:   "encryption declarations app view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppEncryptionDeclarationApp(ctx, id)
		},
	})
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
  asc encryption declarations app-encryption-declaration-document view --id "DECL_ID"`,
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
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "encryption declarations app-encryption-declaration-document view",
		Name:        "view",
		ShortUsage:  "asc encryption declarations app-encryption-declaration-document view --id \"DECL_ID\"",
		ShortHelp:   "View the document for an encryption declaration.",
		LongHelp: `View the document for an encryption declaration.

Examples:
  asc encryption declarations app-encryption-declaration-document view --id "DECL_ID"`,
		IDUsage:            "Encryption declaration ID (required)",
		IDType:             "appEncryptionDeclarations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "encryption declarations app-encryption-declaration-document view",
		FetchErrorPrefix:   "encryption declarations app-encryption-declaration-document view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppEncryptionDeclarationDocumentForDeclaration(ctx, id)
		},
	})
}
