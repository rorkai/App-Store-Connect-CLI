package performance

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// PerformanceDiagnosticsCommand returns the diagnostics subcommand group.
func PerformanceDiagnosticsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("diagnostics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "diagnostics",
		ShortUsage: "asc performance diagnostics <subcommand> [flags]",
		ShortHelp:  "Work with diagnostic signatures and logs.",
		LongHelp: `Work with diagnostic signatures and logs.

Examples:
  asc performance diagnostics list --build "BUILD_ID"
  asc performance diagnostics get --id "SIGNATURE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PerformanceDiagnosticsListCommand(),
			PerformanceDiagnosticsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PerformanceDiagnosticsListCommand returns the diagnostics list subcommand.
func PerformanceDiagnosticsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("diagnostics list", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID to list diagnostics for")
	diagnosticType := fs.String("diagnostic-type", "", "Diagnostic type filter (comma-separated: "+strings.Join(diagnosticSignatureTypeList(), ", ")+")")
	fields := fs.String("fields", "", "Fields to return (comma-separated: "+strings.Join(diagnosticSignatureFieldList(), ", ")+")")
	limit := fs.Int("limit", 0, "Limit number of signatures (max 200)")
	next := fs.String("next", "", "Next page URL")
	paginate := fs.Bool("paginate", false, "Fetch all pages")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc performance diagnostics list --build \"BUILD_ID\"",
		ShortHelp:  "List diagnostic signatures for a build.",
		LongHelp: `List diagnostic signatures for a build.

Examples:
  asc performance diagnostics list --build "BUILD_ID"
  asc performance diagnostics list --build "BUILD_ID" --diagnostic-type "HANGS" --limit 50`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("performance diagnostics list: --limit must be between 1 and 200")
			}

			diagnosticTypes, err := normalizeDiagnosticSignatureTypes(shared.SplitCSVUpper(*diagnosticType))
			if err != nil {
				return fmt.Errorf("performance diagnostics list: %w", err)
			}
			fieldValues, err := normalizeDiagnosticSignatureFields(*fields)
			if err != nil {
				return fmt.Errorf("performance diagnostics list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("performance diagnostics list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.DiagnosticSignaturesOption{
				asc.WithDiagnosticSignaturesDiagnosticTypes(diagnosticTypes),
				asc.WithDiagnosticSignaturesFields(fieldValues),
				asc.WithDiagnosticSignaturesLimit(*limit),
				asc.WithDiagnosticSignaturesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithDiagnosticSignaturesLimit(200))
				firstPage, err := client.GetDiagnosticSignaturesForBuild(requestCtx, trimmedBuildID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("performance diagnostics list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetDiagnosticSignaturesForBuild(ctx, trimmedBuildID, asc.WithDiagnosticSignaturesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("performance diagnostics list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetDiagnosticSignaturesForBuild(requestCtx, trimmedBuildID, opts...)
			if err != nil {
				return fmt.Errorf("performance diagnostics list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PerformanceDiagnosticsGetCommand returns the diagnostics get subcommand.
func PerformanceDiagnosticsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("diagnostics get", flag.ExitOnError)

	signatureID := fs.String("id", "", "Diagnostic signature ID")
	limit := fs.Int("limit", 0, "Limit number of logs (max 200)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc performance diagnostics get --id \"SIGNATURE_ID\"",
		ShortHelp:  "Get diagnostic logs for a signature.",
		LongHelp: `Get diagnostic logs for a signature.

Examples:
  asc performance diagnostics get --id "SIGNATURE_ID"
  asc performance diagnostics get --id "SIGNATURE_ID" --limit 50`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*signatureID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("performance diagnostics get: --limit must be between 1 and 200")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("performance diagnostics get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetDiagnosticSignatureLogs(requestCtx, trimmedID, asc.WithDiagnosticLogsLimit(*limit))
			if err != nil {
				return fmt.Errorf("performance diagnostics get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

var diagnosticSignatureTypes = map[string]struct{}{
	string(asc.DiagnosticSignatureTypeDiskWrites): {},
	string(asc.DiagnosticSignatureTypeHangs):      {},
	string(asc.DiagnosticSignatureTypeLaunches):   {},
}

func diagnosticSignatureTypeList() []string {
	return []string{
		string(asc.DiagnosticSignatureTypeDiskWrites),
		string(asc.DiagnosticSignatureTypeHangs),
		string(asc.DiagnosticSignatureTypeLaunches),
	}
}

func normalizeDiagnosticSignatureTypes(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := diagnosticSignatureTypes[value]; !ok {
			return nil, fmt.Errorf("--diagnostic-type must be one of: %s", strings.Join(diagnosticSignatureTypeList(), ", "))
		}
	}
	return values, nil
}

func diagnosticSignatureFieldList() []string {
	return []string{
		"diagnosticType",
		"signature",
		"weight",
		"insight",
		"logs",
	}
}

func normalizeDiagnosticSignatureFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, field := range diagnosticSignatureFieldList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(diagnosticSignatureFieldList(), ", "))
		}
	}

	return fields, nil
}
