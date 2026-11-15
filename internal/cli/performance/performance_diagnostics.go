package performance

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newPerformanceDiagnosticsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("diagnostics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "diagnostics",
		ShortUsage: "asc performance diagnostics <subcommand> [flags]",
		ShortHelp:  "Work with diagnostic signatures and logs.",
		LongHelp: `Work with diagnostic signatures and logs.

Examples:
  asc performance diagnostics list --build-id "BUILD_ID"
  asc performance diagnostics view --id "SIGNATURE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPerformanceDiagnosticsListCommand(d),
			newPerformanceDiagnosticsGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newPerformanceDiagnosticsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("diagnostics list", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID to list diagnostics for")
	diagnosticType := fs.String("diagnostic-type", "", "Diagnostic type filter (comma-separated: "+strings.Join(diagnosticSignatureTypeList(), ", ")+")")
	fields := fs.String("fields", "", "Fields to return (comma-separated: "+strings.Join(diagnosticSignatureFieldList(), ", ")+")")
	limit := fs.Int("limit", 0, "Limit number of signatures (max 200)")
	next := fs.String("next", "", "Next page URL")
	paginate := fs.Bool("paginate", false, "Fetch all pages")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc performance diagnostics list --build-id \"BUILD_ID\"",
		ShortHelp:  "List diagnostic signatures for a build.",
		LongHelp: `List diagnostic signatures for a build.

Examples:
  asc performance diagnostics list --build-id "BUILD_ID"
  asc performance diagnostics list --build-id "BUILD_ID" --diagnostic-type "HANGS" --limit 50`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "performance diagnostics list", 200, "diagnostic-type", "fields"); err != nil {
				return err
			}
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				return d.RequiredFlag("--build-id")
			}
			if err := validateDiagnosticSignaturesCursor(*next, trimmedBuildID); err != nil {
				return d.UsageErrorf("performance diagnostics list: %v", err)
			}

			diagnosticTypes, err := normalizeDiagnosticSignatureTypes(shared.SplitCSVUpper(*diagnosticType))
			if err != nil {
				return d.UsageErrorf("performance diagnostics list: %v", err)
			}
			fieldValues, err := normalizeDiagnosticSignatureFields(*fields)
			if err != nil {
				return d.UsageErrorf("performance diagnostics list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("performance diagnostics list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.DiagnosticSignaturesOption{
				asc.WithDiagnosticSignaturesDiagnosticTypes(diagnosticTypes),
				asc.WithDiagnosticSignaturesFields(fieldValues),
				asc.Limit[asc.DiagnosticSignaturesOption](*limit),
				asc.NextURL[asc.DiagnosticSignaturesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.DiagnosticSignaturesOption](200))
				firstPage, err := client.GetDiagnosticSignaturesForBuild(requestCtx, trimmedBuildID, paginateOpts...)
				if err != nil {
					return diagnosticsListFetchError(trimmedBuildID, err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetDiagnosticSignaturesForBuild(ctx, trimmedBuildID, asc.NextURL[asc.DiagnosticSignaturesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("performance diagnostics list: %w", err)
				}

				return d.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetDiagnosticSignaturesForBuild(requestCtx, trimmedBuildID, opts...)
			if err != nil {
				return diagnosticsListFetchError(trimmedBuildID, err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func diagnosticsListFetchError(buildID string, err error) error {
	err = fmt.Errorf("performance diagnostics list: failed to fetch: %w", err)
	if !asc.IsNotFound(err) {
		return err
	}
	return &shared.HintedError{
		Err:  err,
		Hint: fmt.Sprintf("The build may exist without diagnostic data: Apple reports diagnostics for App Store versions only after a few days of significant usage. Check that the build exists with `asc builds info --build-id %q`.", buildID),
	}
}

func validateDiagnosticSignaturesCursor(next, buildID string) error {
	next = strings.TrimSpace(next)
	if next == "" {
		return nil
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return err
	}
	parsed, err := url.Parse(next)
	if err != nil {
		return err
	}
	path, hasPrefix := strings.CutPrefix(parsed.Path, "/v1/builds/")
	cursorBuildID, hasSuffix := strings.CutSuffix(path, "/diagnosticSignatures")
	if _, err := asc.ValidateResourcePathSegment(cursorBuildID); !hasPrefix || !hasSuffix || err != nil {
		return errors.New("--next must point to a build's diagnostic signatures")
	}
	if buildID != cursorBuildID {
		return errors.New("--build-id must match the build in --next")
	}
	return nil
}

func newPerformanceDiagnosticsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("diagnostics view", flag.ExitOnError)

	signatureID := fs.String("id", "", "Diagnostic signature ID")
	limit := fs.Int("limit", 0, "Limit number of logs (max 200)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc performance diagnostics view --id \"SIGNATURE_ID\"",
		ShortHelp:  "View diagnostic logs for a signature.",
		LongHelp: `View diagnostic logs for a signature.

Examples:
  asc performance diagnostics view --id "SIGNATURE_ID"
  asc performance diagnostics view --id "SIGNATURE_ID" --limit 50`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "performance diagnostics view", 200); err != nil {
				return err
			}
			trimmedID := strings.TrimSpace(*signatureID)
			if trimmedID == "" {
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("performance diagnostics view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetDiagnosticSignatureLogs(requestCtx, trimmedID, asc.Limit[asc.DiagnosticLogsOption](*limit))
			if err != nil {
				return fmt.Errorf("performance diagnostics view: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
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
