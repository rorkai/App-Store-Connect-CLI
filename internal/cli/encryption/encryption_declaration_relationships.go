package encryption

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newEncryptionDeclarationsAppCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("encryption declarations app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc encryption declarations app <subcommand> [flags]",
		ShortHelp:  "Access the app for an encryption declaration.",
		LongHelp: `Access the app for an encryption declaration.

Examples:
  asc encryption declarations app view --id "DECL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newEncryptionDeclarationsAppGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newEncryptionDeclarationsAppGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
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
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppEncryptionDeclarationApp(ctx, id)
		},
	})
}

func newEncryptionDeclarationsDeclarationDocumentCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("encryption declarations app-encryption-declaration-document", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-encryption-declaration-document",
		ShortUsage: "asc encryption declarations app-encryption-declaration-document <subcommand> [flags]",
		ShortHelp:  "Access the document for an encryption declaration.",
		LongHelp: `Access the document for an encryption declaration.

Examples:
  asc encryption declarations app-encryption-declaration-document view --id "DECL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newEncryptionDeclarationsDeclarationDocumentGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newEncryptionDeclarationsDeclarationDocumentGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
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
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppEncryptionDeclarationDocumentForDeclaration(ctx, id)
		},
	})
}
