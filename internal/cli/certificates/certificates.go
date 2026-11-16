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

// NewCertificatesCommand returns the certificates command with subcommands.
func NewCertificatesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("certificates", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "certificates",
		ShortUsage: "asc certificates <subcommand> [flags]",
		ShortHelp:  "Manage signing certificates.",
		LongHelp: `Manage signing certificates.

Examples:
  asc certificates list
  asc certificates list --certificate-type IOS_DISTRIBUTION
  asc certificates view --id "CERT_ID" --include passTypeId
  asc certificates developer-id-check --output table
  asc certificates export --certificate "./push/push.cer" --private-key "./push/push.key" --password-file "./secrets/push.p12.password" --p12-out "./push/push.p12"
  asc certificates create --certificate-type IOS_DISTRIBUTION --csr "./cert.csr"
  asc certificates create --certificate-type PASS_TYPE_ID --pass-type-id "PASS_TYPE_ID" --csr "./pass.csr"
  asc certificates update --id "CERT_ID" --activated true
  asc certificates update --id "CERT_ID" --activated false
  asc certificates revoke --id "CERT_ID" --confirm
  asc certificates links pass-type-id --id "CERT_ID"`,
		FlagSet:   fs,
		UsageFunc: d.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newCertificatesListCommand(d),
			newCertificatesGetCommand(d),
			newCertificatesDeveloperIDCheckCommand(d),
			newCertificatesCSRCommand(d),
			newCertificatesExportCommand(d),
			newCertificatesCreateCommand(d),
			newCertificatesUpdateCommand(d),
			newCertificatesRevokeCommand(d),
			newCertificatesRelationshipsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newCertificatesListCommand returns the certificates list subcommand.
func newCertificatesListCommand(d *shared.Deps) *ffcli.Command {
	var (
		certificateType       *string
		displayName           *string
		serialNumber          *string
		ids                   *string
		sortValue             string
		fieldsValue           []string
		passTypeIDFieldsValue []string
		includeValues         []string
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
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
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		Deps:                 d,
		NextConflicts:        []string{"display-name", "certificate-type", "serial-number", "id", "sort", "fields", "pass-type-id-fields", "include", "limit"},
		Flags: func(fs *flag.FlagSet) func() error {
			certificateType = fs.String("certificate-type", "", "Filter by certificate type(s), comma-separated")
			displayName = fs.String("display-name", "", "Filter by display name(s), comma-separated")
			serialNumber = fs.String("serial-number", "", "Filter by serial number(s), comma-separated")
			ids = fs.String("id", "", "Filter by certificate ID(s), comma-separated")
			sort := fs.String("sort", "", "Sort by key(s), comma-separated: "+strings.Join(certificateSortList(), ", "))
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(certificateFieldsList(), ", "))
			passTypeIDFields := fs.String("pass-type-id-fields", "", "Fields to include for pass type IDs: "+strings.Join(certificatePassTypeIDFieldsList(), ", "))
			include := fs.String("include", "", "Include relationships: "+strings.Join(certificateIncludeList(), ", "))
			return func() error {
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
						return d.UsageErrorf("certificates list: --%s must not be empty", selector.name)
					}
				}
				var err error
				sortValue, err = normalizeCertificateSort(*sort)
				if err != nil {
					return d.UsageErrorf("certificates list: %v", err)
				}
				fieldsValue, err = normalizeCertificateFields(*fields, "--fields")
				if err != nil {
					return d.UsageErrorf("certificates list: %v", err)
				}
				passTypeIDFieldsValue, err = normalizeCertificatePassTypeIDFields(*passTypeIDFields, "--pass-type-id-fields")
				if err != nil {
					return d.UsageErrorf("certificates list: %v", err)
				}
				includeValues, err = normalizeCertificatesInclude(*include)
				if err != nil {
					return d.UsageErrorf("certificates list: %v", err)
				}
				if len(passTypeIDFieldsValue) > 0 && !shared.HasInclude(includeValues, "passTypeId") {
					const message = "--pass-type-id-fields requires --include passTypeId"
					fmt.Fprintln(d.IO.ErrOut, "Error: "+message)
					return shared.WithDiagnostic(
						shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message),
						shared.DiagnosticInvalidInput,
						"--pass-type-id-fields",
					)
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.CertificatesOption{
				asc.Limit[asc.CertificatesOption](limit),
				asc.NextURL[asc.CertificatesOption](next),
			}
			if displayNames := shared.SplitCSV(*displayName); len(displayNames) > 0 {
				opts = append(opts, asc.WithCertificatesFilterDisplayNames(displayNames))
			}
			if certificateTypes := shared.SplitCSVUpper(*certificateType); len(certificateTypes) > 0 {
				opts = append(opts, asc.WithCertificatesTypes(certificateTypes))
			}
			if serialNumbers := shared.SplitCSV(*serialNumber); len(serialNumbers) > 0 {
				opts = append(opts, asc.WithCertificatesFilterSerialNumbers(serialNumbers))
			}
			if idsValue := shared.SplitCSV(*ids); len(idsValue) > 0 {
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
			return client.GetCertificates(ctx, opts...)
		},
	})
}

// newCertificatesGetCommand returns the certificates view subcommand.
func newCertificatesGetCommand(d *shared.Deps) *ffcli.Command {
	var (
		includeValues []string
		included      asc.IncludedFields
	)

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc certificates view --id \"CERT_ID\" [flags]",
		ShortHelp:  "View a signing certificate by ID.",
		LongHelp: `View a signing certificate by ID.

Examples:
  asc certificates view --id "CERT_ID"
  asc certificates view --id "CERT_ID" --include passTypeId`,
		IDUsage:            "Certificate ID",
		IDType:             "certificates",
		RequiredDiagnostic: true,
		Deps:               d,
		Flags: func(fs *flag.FlagSet) func() error {
			include := fs.String("include", "", "Include related resources: passTypeId")
			includedFields := shared.BindIncludedFieldFlags(fs, map[string]string{"passTypeId": "passTypeIds"})
			return func() (err error) {
				if includeValues, err = normalizeCertificatesInclude(*include); err != nil {
					return d.UsageErrorf("certificates view: %v", err)
				}
				included, err = includedFields.ResolveWith(d, "certificates view", "", includeValues)
				return err
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			opts := []asc.CertificatesOption{asc.WithIncludedFields[asc.CertificatesOption](included)}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithCertificatesInclude(includeValues))
			}
			return client.GetCertificate(ctx, id, opts...)
		},
	})
}

// certificateCreateSpec describes an entry point for POST /v1/certificates.
// asc certificates create exposes the full surface, while the merchant ID
// variant scopes every request to one merchant.
type certificateCreateSpec struct {
	commandPath      string
	shortUsage       string
	shortHelp        string
	longHelp         string
	certificateTypes []string
	supportsPassType bool
	requireMerchant  bool
	merchantIDUsage  string
}

// newCertificatesCreateCommand returns the certificates create subcommand.
func newCertificatesCreateCommand(d *shared.Deps) *ffcli.Command {
	return newCertificateCreateCommand(d, certificateCreateSpec{
		commandPath:      "certificates create",
		shortUsage:       "asc certificates create --certificate-type TYPE [--pass-type-id ID] [--merchant-id ID] (--csr ./cert.csr | --generate-csr --key-out ./cert.key --csr-out ./cert.csr)",
		shortHelp:        "Create a signing certificate.",
		certificateTypes: shared.CertificateCreateTypeList(),
		supportsPassType: true,
		merchantIDUsage:  "Merchant ID resource ID (required for the APPLE_PAY certificate types)",
		longHelp: `Create a signing certificate.

Examples:
  asc certificates create --certificate-type IOS_DISTRIBUTION --csr "./cert.csr"
  asc certificates create --certificate-type PASS_TYPE_ID --pass-type-id "PASS_TYPE_ID" --csr "./pass.csr"
  asc certificates create --certificate-type APPLE_PAY_MERCHANT_IDENTITY --merchant-id "MERCHANT_ID" --csr "./merchant.csr"
  asc certificates create --certificate-type IOS_DISTRIBUTION --generate-csr --key-out "./signing/dist.key" --csr-out "./signing/dist.csr"`,
	})
}

// NewMerchantIDCertificatesCreateCommand returns the merchant ID scoped
// certificate create subcommand built on d. It lives here so the CSR generation
// and relationship validation have a single implementation.
func NewMerchantIDCertificatesCreateCommand(d *shared.Deps) *ffcli.Command {
	return newCertificateCreateCommand(d, certificateCreateSpec{
		commandPath: "merchant-ids certificates create",
		shortUsage:  "asc merchant-ids certificates create --merchant-id \"MERCHANT_ID\" --certificate-type TYPE (--csr ./merchant.csr | --generate-csr --key-out ./merchant.key --csr-out ./merchant.csr)",
		shortHelp:   "Create a certificate for a merchant ID.",
		// Every certificate created against a merchant ID is an Apple Pay type.
		certificateTypes: shared.ApplePayCertificateTypeList(),
		requireMerchant:  true,
		merchantIDUsage:  "Merchant ID",
		longHelp: `Create a certificate for a merchant ID.

Examples:
  asc merchant-ids certificates create --merchant-id "MERCHANT_ID" --certificate-type APPLE_PAY_MERCHANT_IDENTITY --csr "./merchant.csr"
  asc merchant-ids certificates create --merchant-id "MERCHANT_ID" --certificate-type APPLE_PAY_RSA --generate-csr --key-out "./signing/merchant.key" --csr-out "./signing/merchant.csr"`,
	})
}

func newCertificateCreateCommand(d *shared.Deps, spec certificateCreateSpec) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	certificateType := fs.String("certificate-type", "", "Certificate type: "+strings.Join(spec.certificateTypes, ", "))
	var passTypeID *string
	if spec.supportsPassType {
		passTypeID = shared.BindResourceIDFlag(fs, "pass-type-id", "passTypeIds", "Pass Type ID resource ID (required for PASS_TYPE_ID and PASS_TYPE_ID_WITH_NFC)")
	}
	merchantID := shared.BindResourceIDFlag(fs, "merchant-id", "merchantIds", spec.merchantIDUsage)
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
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: spec.shortUsage,
		ShortHelp:  spec.shortHelp,
		LongHelp:   spec.longHelp,
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			merchantIDValue := strings.TrimSpace(*merchantID)
			if spec.requireMerchant && merchantIDValue == "" {
				return d.RequiredFlag("--merchant-id")
			}
			certificateValue := strings.ToUpper(strings.TrimSpace(*certificateType))
			if certificateValue == "" {
				return d.RequiredFlag("--certificate-type")
			}
			// Every relationship check runs before --generate-csr writes a
			// private key and CSR for a request App Store Connect would refuse.
			canonicalCertificateType, err := d.ValidateCertificateCreateType("--certificate-type", certificateValue)
			if err != nil {
				return err
			}
			certificateValue = canonicalCertificateType
			passTypeIDValue := ""
			if passTypeID != nil {
				passTypeIDValue = strings.TrimSpace(*passTypeID)
			}
			if passTypeIDValue != "" && merchantIDValue != "" {
				return d.UsageError("--merchant-id cannot be used with --pass-type-id")
			}
			if err := d.ValidateCertificateCreateMerchantID(certificateValue, merchantIDValue); err != nil {
				return err
			}
			isPassTypeCertificate := certificateValue == "PASS_TYPE_ID" || certificateValue == "PASS_TYPE_ID_WITH_NFC"
			if spec.supportsPassType {
				if isPassTypeCertificate && passTypeIDValue == "" {
					fmt.Fprintf(d.IO.ErrOut, "Error: --pass-type-id is required with --certificate-type %s\n", certificateValue)
					return shared.MissingRequiredUsageError("--pass-type-id")
				}
				if !isPassTypeCertificate && passTypeIDValue != "" {
					return d.UsageError("--pass-type-id can only be used with --certificate-type PASS_TYPE_ID or PASS_TYPE_ID_WITH_NFC")
				}
			}
			csrValue := strings.TrimSpace(*csrPath)

			var csrContent string
			if *generateCSR {
				if csrValue != "" {
					return d.UsageError("--csr cannot be used with --generate-csr")
				}
				keyOutValue := strings.TrimSpace(*keyOut)
				if keyOutValue == "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --key-out is required with --generate-csr")
					return shared.MissingRequiredUsageError("--key-out")
				}
				csrOutValue := strings.TrimSpace(*csrOut)
				if csrOutValue == "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --csr-out is required with --generate-csr")
					return shared.MissingRequiredUsageError("--csr-out")
				}

				_, csrPEM, err := generateCSRFiles(d, csrGenerateOptions{
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
					return fmt.Errorf("%s: generate csr: %w", spec.commandPath, err)
				}
				csrContent, err = encodeCSRContent(csrPEM)
				if err != nil {
					return fmt.Errorf("%s: generate csr: %w", spec.commandPath, err)
				}
			} else {
				if csrCreateOnlyFlagsSet(fs) {
					return d.UsageError("--key-out, --csr-out, CSR subject flags, --key-type, --key-size, and --force require --generate-csr")
				}
				if csrValue == "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --csr is required (or use --generate-csr with --key-out and --csr-out)")
					return shared.MissingRequiredUsageError("--csr")
				}

				var err error
				csrContent, err = readCSRContent(csrValue)
				if err != nil {
					return fmt.Errorf("%s: %w", spec.commandPath, err)
				}
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("%s: %w", spec.commandPath, err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			createOpts := []asc.CertificateCreateOption{}
			if passTypeIDValue != "" {
				createOpts = append(createOpts, asc.WithCertificatePassTypeID(passTypeIDValue))
			}
			if merchantIDValue != "" {
				createOpts = append(createOpts, asc.WithCertificateMerchantID(merchantIDValue))
			}

			resp, err := client.CreateCertificate(requestCtx, csrContent, certificateValue, createOpts...)
			if err != nil {
				return fmt.Errorf("%s: failed to create: %w", spec.commandPath, err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
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

// newCertificatesUpdateCommand returns the certificates update subcommand.
func newCertificatesUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "certificates", "Certificate ID")
	activated := fs.String("activated", "", "Set activated (true/false)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc certificates update --id \"CERT_ID\" --activated true",
		ShortHelp:  "Update a signing certificate.",
		LongHelp: `Update a signing certificate.

Examples:
  asc certificates update --id "CERT_ID" --activated true
  asc certificates update --id "CERT_ID" --activated false`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}

			activatedValue, err := shared.ParseOptionalBoolFlag("--activated", *activated)
			if err != nil {
				return d.UsageErrorf("certificates update: %v", err)
			}
			if activatedValue == nil {
				return d.RequiredFlag("--activated")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("certificates update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateCertificate(requestCtx, idValue, asc.CertificateUpdateAttributes{
				Activated: activatedValue,
			})
			if err != nil {
				return fmt.Errorf("certificates update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCertificatesRevokeCommand returns the certificates revoke subcommand.
func newCertificatesRevokeCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("revoke", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "certificates", "Certificate ID")
	confirm := fs.Bool("confirm", false, "Confirm revocation")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "revoke",
		ShortUsage: "asc certificates revoke --id \"CERT_ID\" --confirm",
		ShortHelp:  "Revoke a signing certificate.",
		LongHelp: `Revoke a signing certificate.

Examples:
  asc certificates revoke --id "CERT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}
			if err := shared.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("certificates revoke: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.RevokeCertificate(requestCtx, idValue); err != nil {
				return fmt.Errorf("certificates revoke: failed to revoke: %w", err)
			}

			result := &asc.CertificateRevokeResult{
				ID:      idValue,
				Revoked: true,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
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
