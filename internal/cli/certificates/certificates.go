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

var getCertificatesASCClient = shared.GetASCClient

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
  asc certificates view --id "CERT_ID" --include passTypeId
  asc certificates export --certificate "./push/push.cer" --private-key "./push/push.key" --password-file "./secrets/push.p12.password" --p12-out "./push/push.p12"
  asc certificates create --certificate-type IOS_DISTRIBUTION --csr "./cert.csr"
  asc certificates create --certificate-type PASS_TYPE_ID --pass-type-id "PASS_TYPE_ID" --csr "./pass.csr"
  asc certificates update --id "CERT_ID" --activated true
  asc certificates update --id "CERT_ID" --activated false
  asc certificates revoke --id "CERT_ID" --confirm
  asc certificates links pass-type-id --id "CERT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			CertificatesListCommand(),
			CertificatesGetCommand(),
			CertificatesCSRCommand(),
			CertificatesExportCommand(),
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
	displayName := fs.String("display-name", "", "Filter by display name(s), comma-separated")
	serialNumber := fs.String("serial-number", "", "Filter by serial number(s), comma-separated")
	ids := fs.String("id", "", "Filter by certificate ID(s), comma-separated")
	sort := fs.String("sort", "", "Sort by key(s), comma-separated: "+strings.Join(certificateSortList(), ", "))
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(certificateFieldsList(), ", "))
	passTypeIDFields := fs.String("pass-type-id-fields", "", "Fields to include for pass type IDs: "+strings.Join(certificatePassTypeIDFieldsList(), ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(certificateIncludeList(), ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc certificates list [flags]",
		ShortHelp:  "List signing certificates.",
		LongHelp: `List signing certificates.

Examples:
  asc certificates list
  asc certificates list --certificate-type IOS_DISTRIBUTION
  asc certificates list --display-name "Example Certificate"
  asc certificates list --sort "-displayName" --fields "displayName,serialNumber"
  asc certificates list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("certificates list: %w", err)
			}
			if err := shared.RejectNextFlagConflicts(
				fs,
				*next,
				"certificates list",
				"display-name",
				"certificate-type",
				"serial-number",
				"id",
				"sort",
				"fields",
				"pass-type-id-fields",
				"include",
				"limit",
			); err != nil {
				return err
			}
			provided := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				provided[f.Name] = true
			})
			for _, selector := range []struct {
				name  string
				value string
			}{
				{name: "certificate-type", value: *certificateType},
				{name: "display-name", value: *displayName},
				{name: "serial-number", value: *serialNumber},
				{name: "id", value: *ids},
				{name: "sort", value: *sort},
				{name: "fields", value: *fields},
				{name: "pass-type-id-fields", value: *passTypeIDFields},
				{name: "include", value: *include},
			} {
				if provided[selector.name] && len(shared.SplitCSV(selector.value)) == 0 {
					return shared.UsageErrorf("certificates list: --%s must not be empty", selector.name)
				}
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("certificates list: --limit must be between 1 and 200")
			}
			sortValue, err := normalizeCertificateSort(*sort)
			if err != nil {
				return shared.UsageErrorf("certificates list: %v", err)
			}
			fieldsValue, err := normalizeCertificateFields(*fields, "--fields")
			if err != nil {
				return shared.UsageErrorf("certificates list: %v", err)
			}
			passTypeIDFieldsValue, err := normalizeCertificatePassTypeIDFields(*passTypeIDFields, "--pass-type-id-fields")
			if err != nil {
				return shared.UsageErrorf("certificates list: %v", err)
			}
			includeValues, err := normalizeCertificatesInclude(*include)
			if err != nil {
				return shared.UsageErrorf("certificates list: %v", err)
			}
			if len(passTypeIDFieldsValue) > 0 && !shared.HasInclude(includeValues, "passTypeId") {
				const message = "--pass-type-id-fields requires --include passTypeId"
				fmt.Fprintln(os.Stderr, "Error: "+message)
				return shared.WithDiagnostic(
					shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message),
					shared.DiagnosticInvalidInput,
					"--pass-type-id-fields",
				)
			}

			certificateTypes := shared.SplitCSVUpper(*certificateType)
			displayNames := shared.SplitCSV(*displayName)
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
			if len(displayNames) > 0 {
				opts = append(opts, asc.WithCertificatesFilterDisplayNames(displayNames))
			}
			if len(certificateTypes) > 0 {
				opts = append(opts, asc.WithCertificatesTypes(certificateTypes))
			}
			serialNumbers := shared.SplitCSV(*serialNumber)
			if len(serialNumbers) > 0 {
				opts = append(opts, asc.WithCertificatesFilterSerialNumbers(serialNumbers))
			}
			idsValue := shared.SplitCSV(*ids)
			if len(idsValue) > 0 {
				opts = append(opts, asc.WithCertificatesFilterIDs(idsValue))
			}
			if sortValue != "" {
				opts = append(opts, asc.WithCertificatesSort(sortValue))
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithCertificatesFields(fieldsValue))
			}
			if len(passTypeIDFieldsValue) > 0 {
				opts = append(opts, asc.WithCertificatesPassTypeIDFields(passTypeIDFieldsValue))
			}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithCertificatesInclude(includeValues))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCertificatesLimit(200))
				paginated, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetCertificates(ctx, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetCertificates(ctx, asc.WithCertificatesNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("certificates list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetCertificates(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("certificates list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CertificatesGetCommand returns the certificates view subcommand.
func CertificatesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	id := fs.String("id", "", "Certificate ID")
	include := fs.String("include", "", "Include related resources: passTypeId")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc certificates view --id \"CERT_ID\" [flags]",
		ShortHelp:  "View a signing certificate by ID.",
		LongHelp: `View a signing certificate by ID.

Examples:
  asc certificates view --id "CERT_ID"
  asc certificates view --id "CERT_ID" --include passTypeId`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			includeValues, err := normalizeCertificatesInclude(*include)
			if err != nil {
				return fmt.Errorf("certificates view: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("certificates view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.CertificatesOption{}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithCertificatesInclude(includeValues))
			}

			resp, err := client.GetCertificate(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("certificates view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CertificatesCreateCommand returns the certificates create subcommand.
func CertificatesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	certificateType := fs.String("certificate-type", "", "Certificate type: "+strings.Join(shared.CertificateCreateTypeList(), ", "))
	passTypeID := shared.BindResourceIDFlag(fs, "pass-type-id", "passTypeIds", "Pass Type ID resource ID (required for PASS_TYPE_ID and PASS_TYPE_ID_WITH_NFC)")
	csrPath := fs.String("csr", "", "CSR file path")
	generateCSR := fs.Bool("generate-csr", false, "Generate a private key and CSR before creating the certificate")
	keyOut := fs.String("key-out", "", "Private key output path for --generate-csr (PEM)")
	csrOut := fs.String("csr-out", "", "CSR output path for --generate-csr (PEM)")
	commonName := fs.String("common-name", "asc", "CSR subject Common Name (CN) for --generate-csr")
	email := fs.String("email", "", "CSR subject email address for --generate-csr")
	organization := fs.String("organization", "", "CSR subject organization (O) for --generate-csr")
	orgUnit := fs.String("organizational-unit", "", "CSR subject organizational unit (OU) for --generate-csr")
	country := fs.String("country", "", "CSR subject country (C) for --generate-csr")
	keyType := fs.String("key-type", "rsa", "CSR key type for --generate-csr: rsa")
	keySize := fs.Int("key-size", 2048, "CSR RSA key size in bits for --generate-csr")
	force := fs.Bool("force", false, "Overwrite generated CSR/key output files")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc certificates create --certificate-type TYPE [--pass-type-id ID] (--csr ./cert.csr | --generate-csr --key-out ./cert.key --csr-out ./cert.csr)",
		ShortHelp:  "Create a signing certificate.",
		LongHelp: `Create a signing certificate.

Examples:
  asc certificates create --certificate-type IOS_DISTRIBUTION --csr "./cert.csr"
  asc certificates create --certificate-type PASS_TYPE_ID --pass-type-id "PASS_TYPE_ID" --csr "./pass.csr"
  asc certificates create --certificate-type IOS_DISTRIBUTION --generate-csr --key-out "./signing/dist.key" --csr-out "./signing/dist.csr"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			certificateValue := strings.ToUpper(strings.TrimSpace(*certificateType))
			if certificateValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --certificate-type is required")
				return shared.MissingRequiredUsageError("--certificate-type")
			}
			// Rejects unknown types and the Apple Pay types this command cannot
			// create, before --generate-csr writes a private key and CSR for a
			// request App Store Connect will refuse.
			canonicalCertificateType, err := shared.ValidateCertificateCreateType("--certificate-type", certificateValue)
			if err != nil {
				return err
			}
			certificateValue = canonicalCertificateType
			passTypeIDValue := strings.TrimSpace(*passTypeID)
			isPassTypeCertificate := certificateValue == "PASS_TYPE_ID" || certificateValue == "PASS_TYPE_ID_WITH_NFC"
			if isPassTypeCertificate && passTypeIDValue == "" {
				fmt.Fprintf(os.Stderr, "Error: --pass-type-id is required with --certificate-type %s\n", certificateValue)
				return shared.MissingRequiredUsageError("--pass-type-id")
			}
			if !isPassTypeCertificate && passTypeIDValue != "" {
				return shared.UsageError("--pass-type-id can only be used with --certificate-type PASS_TYPE_ID or PASS_TYPE_ID_WITH_NFC")
			}
			csrValue := strings.TrimSpace(*csrPath)

			var csrContent string
			if *generateCSR {
				if csrValue != "" {
					return shared.UsageError("--csr cannot be used with --generate-csr")
				}
				keyOutValue := strings.TrimSpace(*keyOut)
				if keyOutValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --key-out is required with --generate-csr")
					return shared.MissingRequiredUsageError("--key-out")
				}
				csrOutValue := strings.TrimSpace(*csrOut)
				if csrOutValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --csr-out is required with --generate-csr")
					return shared.MissingRequiredUsageError("--csr-out")
				}

				_, csrPEM, err := generateCSRFiles(csrGenerateOptions{
					KeyOut:             keyOutValue,
					CSROut:             csrOutValue,
					CommonName:         *commonName,
					Email:              *email,
					Organization:       *organization,
					OrganizationalUnit: *orgUnit,
					Country:            *country,
					KeyType:            *keyType,
					KeySize:            *keySize,
					Force:              *force,
				})
				if err != nil {
					return fmt.Errorf("certificates create: generate csr: %w", err)
				}
				csrContent, err = encodeCSRContent(csrPEM)
				if err != nil {
					return fmt.Errorf("certificates create: generate csr: %w", err)
				}
			} else {
				if csrCreateOnlyFlagsSet(fs) {
					return shared.UsageError("--key-out, --csr-out, CSR subject flags, --key-type, --key-size, and --force require --generate-csr")
				}
				if csrValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --csr is required (or use --generate-csr with --key-out and --csr-out)")
					return shared.MissingRequiredUsageError("--csr")
				}

				var err error
				csrContent, err = readCSRContent(csrValue)
				if err != nil {
					return fmt.Errorf("certificates create: %w", err)
				}
			}

			client, err := getCertificatesASCClient()
			if err != nil {
				return fmt.Errorf("certificates create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			createOpts := []asc.CertificateCreateOption{}
			if passTypeIDValue != "" {
				createOpts = append(createOpts, asc.WithCertificatePassTypeID(passTypeIDValue))
			}

			resp, err := client.CreateCertificate(requestCtx, csrContent, certificateValue, createOpts...)
			if err != nil {
				return fmt.Errorf("certificates create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func csrCreateOnlyFlagsSet(fs *flag.FlagSet) bool {
	seen := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "key-out", "csr-out", "common-name", "email", "organization", "organizational-unit", "country", "key-type", "key-size", "force":
			seen = true
		}
	})
	return seen
}

// CertificatesUpdateCommand returns the certificates update subcommand.
func CertificatesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Certificate ID")
	activated := fs.String("activated", "", "Set activated (true/false)")
	output := shared.BindOutputFlags(fs)

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
				return shared.MissingRequiredUsageError("--id")
			}

			activatedValue, err := shared.ParseOptionalBoolFlag("--activated", *activated)
			if err != nil {
				return fmt.Errorf("certificates update: %w", err)
			}
			if activatedValue == nil {
				fmt.Fprintln(os.Stderr, "Error: --activated is required")
				return shared.MissingRequiredUsageError("--activated")
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

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CertificatesRevokeCommand returns the certificates revoke subcommand.
func CertificatesRevokeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("revoke", flag.ExitOnError)

	id := fs.String("id", "", "Certificate ID")
	confirm := fs.Bool("confirm", false, "Confirm revocation")
	output := shared.BindOutputFlags(fs)

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
				return shared.MissingRequiredUsageError("--id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
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

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func readCSRContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return encodeCSRContent(data)
}

func encodeCSRContent(data []byte) (string, error) {
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

func normalizeCertificateSort(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	allowed := certificateSortList()
	parts := strings.Split(value, ",")
	for index, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", fmt.Errorf("--sort must be one of: %s", strings.Join(allowed, ", "))
		}
		if err := shared.ValidateSort(part, allowed...); err != nil {
			return "", err
		}
		parts[index] = part
	}
	return strings.Join(parts, ","), nil
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

func normalizeCertificateFields(value, flagName string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, field := range certificateFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(certificateFieldsList(), ", "))
		}
	}
	return fields, nil
}

func normalizeCertificatePassTypeIDFields(value, flagName string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, field := range certificatePassTypeIDFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(certificatePassTypeIDFieldsList(), ", "))
		}
	}
	return fields, nil
}

func certificateIncludeList() []string {
	return []string{"passTypeId"}
}

func certificateFieldsList() []string {
	return []string{
		"name",
		"certificateType",
		"displayName",
		"serialNumber",
		"platform",
		"expirationDate",
		"certificateContent",
		"activated",
		"passTypeId",
	}
}

func certificatePassTypeIDFieldsList() []string {
	return []string{"name", "identifier", "certificates"}
}

func certificateSortList() []string {
	return []string{
		"displayName",
		"-displayName",
		"certificateType",
		"-certificateType",
		"serialNumber",
		"-serialNumber",
		"id",
		"-id",
	}
}
