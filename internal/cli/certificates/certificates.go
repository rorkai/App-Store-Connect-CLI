package certificates

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// CertificatesCommand returns the certificates command with subcommands.
func CertificatesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("certificates", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "certificates",
		ShortUsage: "asc certificates <subcommand> [flags]",
		ShortHelp:  "Manage signing certificates.",
		LongHelp: `Manage signing certificates.

Examples:
  asc certificates list
  asc certificates list --certificate-type IOS_DISTRIBUTION
  asc certificates get --id "CERT_ID" --include passTypeId
  asc certificates create --certificate-type IOS_DISTRIBUTION --csr "./cert.csr"
  asc certificates update --id "CERT_ID" --activated true
  asc certificates update --id "CERT_ID" --activated false
  asc certificates revoke --id "CERT_ID" --confirm
  asc certificates relationships pass-type-id --id "CERT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CertificatesListCommand(),
			CertificatesGetCommand(),
			CertificatesCreateCommand(),
			CertificatesUpdateCommand(),
			CertificatesRevokeCommand(),
			CertificatesRelationshipsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CertificatesListCommand returns the certificates list subcommand.
func CertificatesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	certificateType := fs.String("certificate-type", "", "Filter by certificate type(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc certificates list [flags]",
		ShortHelp:  "List signing certificates.",
		LongHelp: `List signing certificates.

Examples:
  asc certificates list
  asc certificates list --certificate-type IOS_DISTRIBUTION
  asc certificates list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("certificates list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("certificates list: %w", err)
			}

			certificateTypes := shared.SplitCSVUpper(*certificateType)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("certificates list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.CertificatesOption{
				asc.WithCertificatesLimit(*limit),
				asc.WithCertificatesNextURL(*next),
			}
			if len(certificateTypes) > 0 {
				opts = append(opts, asc.WithCertificatesTypes(certificateTypes))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCertificatesLimit(200))
				firstPage, err := client.GetCertificates(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("certificates list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCertificates(ctx, asc.WithCertificatesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("certificates list: %w", err)
				}

				return shared.PrintOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetCertificates(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("certificates list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// CertificatesGetCommand returns the certificates get subcommand.
func CertificatesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Certificate ID")
	include := fs.String("include", "", "Include related resources: passTypeId")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc certificates get --id \"CERT_ID\" [flags]",
		ShortHelp:  "Get a signing certificate by ID.",
		LongHelp: `Get a signing certificate by ID.

Examples:
  asc certificates get --id "CERT_ID"
  asc certificates get --id "CERT_ID" --include passTypeId`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			includeValues, err := normalizeCertificatesInclude(*include)
			if err != nil {
				return fmt.Errorf("certificates get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("certificates get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.CertificatesOption{}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithCertificatesInclude(includeValues))
			}

			resp, err := client.GetCertificate(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("certificates get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// CertificatesCreateCommand returns the certificates create subcommand.
func CertificatesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	certificateType := fs.String("certificate-type", "", "Certificate type (e.g., IOS_DISTRIBUTION)")
	csrPath := fs.String("csr", "", "CSR file path")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc certificates create --certificate-type TYPE --csr ./cert.csr",
		ShortHelp:  "Create a signing certificate.",
		LongHelp: `Create a signing certificate.

Examples:
  asc certificates create --certificate-type IOS_DISTRIBUTION --csr "./cert.csr"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			certificateValue := strings.ToUpper(strings.TrimSpace(*certificateType))
			if certificateValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --certificate-type is required")
				return flag.ErrHelp
			}
			csrValue := strings.TrimSpace(*csrPath)
			if csrValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --csr is required")
				return flag.ErrHelp
			}

			csrContent, err := readCSRContent(csrValue)
			if err != nil {
				return fmt.Errorf("certificates create: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("certificates create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateCertificate(requestCtx, csrContent, certificateValue)
			if err != nil {
				return fmt.Errorf("certificates create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// CertificatesUpdateCommand returns the certificates update subcommand.
func CertificatesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Certificate ID")
	activated := fs.String("activated", "", "Set activated (true/false)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc certificates update --id \"CERT_ID\" --activated true",
		ShortHelp:  "Update a signing certificate.",
		LongHelp: `Update a signing certificate.

Examples:
  asc certificates update --id "CERT_ID" --activated true
  asc certificates update --id "CERT_ID" --activated false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			activatedValue, err := shared.ParseOptionalBoolFlag("--activated", *activated)
			if err != nil {
				return fmt.Errorf("certificates update: %w", err)
			}
			if activatedValue == nil {
				fmt.Fprintln(os.Stderr, "Error: --activated is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("certificates update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateCertificate(requestCtx, idValue, asc.CertificateUpdateAttributes{
				Activated: activatedValue,
			})
			if err != nil {
				return fmt.Errorf("certificates update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// CertificatesRevokeCommand returns the certificates revoke subcommand.
func CertificatesRevokeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("revoke", flag.ExitOnError)

	id := fs.String("id", "", "Certificate ID")
	confirm := fs.Bool("confirm", false, "Confirm revocation")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "revoke",
		ShortUsage: "asc certificates revoke --id \"CERT_ID\" --confirm",
		ShortHelp:  "Revoke a signing certificate.",
		LongHelp: `Revoke a signing certificate.

Examples:
  asc certificates revoke --id "CERT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("certificates revoke: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.RevokeCertificate(requestCtx, idValue); err != nil {
				return fmt.Errorf("certificates revoke: failed to revoke: %w", err)
			}

			result := &asc.CertificateRevokeResult{
				ID:      idValue,
				Revoked: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func readCSRContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return "", fmt.Errorf("CSR file is empty")
	}
	if block, _ := pem.Decode(data); block != nil {
		return base64.StdEncoding.EncodeToString(block.Bytes), nil
	}
	normalized := strings.Join(strings.Fields(string(data)), "")
	if normalized == "" {
		return "", fmt.Errorf("CSR file is empty")
	}
	return normalized, nil
}

func normalizeCertificatesInclude(value string) ([]string, error) {
	include := shared.SplitCSV(value)
	if len(include) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, item := range certificateIncludeList() {
		allowed[item] = struct{}{}
	}
	for _, item := range include {
		if _, ok := allowed[item]; !ok {
			return nil, fmt.Errorf("--include must be one of: %s", strings.Join(certificateIncludeList(), ", "))
		}
	}
	return include, nil
}

func certificateIncludeList() []string {
	return []string{"passTypeId"}
}
