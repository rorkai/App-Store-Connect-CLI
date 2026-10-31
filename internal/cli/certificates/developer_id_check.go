package certificates

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	developerIDStatusAffected       = artifacts.DeveloperIDAffected
	developerIDStatusLikelyAffected = "likely-affected"
	developerIDStatusNotAffected    = artifacts.DeveloperIDNotAffected
	developerIDStatusUnknown        = "unknown"

	developerIDEvidenceIssuer          = "issuer"
	developerIDEvidenceCertificateType = "certificateType"
	developerIDEvidenceExpirationDate  = "expirationDate"
	developerIDEvidenceNone            = "none"
)

var developerIDCheckNow = time.Now

// developerIDCheckTypes lists every Developer ID type the certificates endpoint
// accepts. It has no Developer ID Installer type.
var developerIDCheckTypes = []string{
	"DEVELOPER_ID_APPLICATION",
	"DEVELOPER_ID_APPLICATION_G2",
	"DEVELOPER_ID_KEXT",
	"DEVELOPER_ID_KEXT_G2",
}

// CertificatesDeveloperIDCheckCommand returns the certificates developer-id-check subcommand.
func CertificatesDeveloperIDCheckCommand() *ffcli.Command {
	fs := flag.NewFlagSet("developer-id-check", flag.ExitOnError)

	strict := fs.Bool("strict", false, "Exit non-zero when an unexpired certificate is not proven unaffected")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "developer-id-check",
		ShortUsage: "asc certificates developer-id-check [--strict] [flags]",
		ShortHelp:  "Find Developer ID certificates from the Sub-CA that expires on February 1, 2027.",
		LongHelp: `Find Developer ID certificates issued by the original Developer ID Certification Authority, which expires on February 1, 2027.
Certificates from that authority stop working on that date, and .pkg installers signed with them no longer install. Replace them with certificates from Developer ID Certification Authority (G2).

The check reads every DEVELOPER_ID_APPLICATION and DEVELOPER_ID_KEXT certificate, including their _G2 types. status is:
  affected         the issuer in certificateContent is the original authority (organizational unit Apple Certification Authority)
  not-affected     the issuer is the G2 authority, the type is a _G2 type, or the certificate expires after February 1, 2027
  likely-affected  the issuer could not be read and the certificate expires on or before February 1, 2027
  unknown          neither the issuer nor the expiration date could be read
evidence names what status rests on: issuer, certificateType, expirationDate, or none.

The App Store Connect API does not list Developer ID Installer certificates, so this check does not cover them, and notChecked says so. Review them in Certificates, Identifiers & Profiles, and use asc pkg-info --path PATH to inspect a signed package.

actionRequired is true when an unexpired certificate is anything but not-affected. Expired certificates are still reported, and do not set it.
A finding prints a warning to stderr and exits 0. With --strict, actionRequired exits 1 after writing the report.

Examples:
  asc certificates developer-id-check
  asc certificates developer-id-check --output table
  asc certificates developer-id-check --strict --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("certificates developer-id-check does not accept positional arguments")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := getCertificatesASCClient()
			if err != nil {
				return fmt.Errorf("certificates developer-id-check: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			paginated, err := shared.PaginateWithSpinner(
				requestCtx,
				func(ctx context.Context) (asc.PaginatedResponse, error) {
					return client.GetCertificates(ctx, asc.WithCertificatesTypes(developerIDCheckTypes), asc.WithCertificatesLimit(200))
				},
				func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCertificates(ctx, asc.WithCertificatesNextURL(nextURL))
				},
			)
			if err != nil {
				return fmt.Errorf("certificates developer-id-check: %w", err)
			}
			response, ok := paginated.(*asc.CertificatesResponse)
			if !ok {
				return fmt.Errorf("certificates developer-id-check: unexpected response type %T", paginated)
			}

			result := buildDeveloperIDCheck(response.Data, developerIDCheckNow())
			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if !result.ActionRequired {
				return nil
			}
			pending := developerIDCheckPending(result)
			fmt.Fprintf(os.Stderr, "Warning: certificates developer-id-check: %d certificate(s) may stop working on February 1, 2027; replace them with certificates from %s\n", pending, artifacts.DeveloperIDReplacementAuthority)
			if *strict {
				return shared.NewReportedError(fmt.Errorf("certificates developer-id-check: %d certificate(s) need replacement or review", pending))
			}
			return nil
		},
	}
}

func buildDeveloperIDCheck(certificates []asc.Resource[asc.CertificateAttributes], now time.Time) *asc.DeveloperIDCheckResult {
	expiry := artifacts.DeveloperIDPreviousAuthorityExpiry
	result := &asc.DeveloperIDCheckResult{
		AuthorityExpiresOn:   expiry.Format(time.DateOnly),
		AuthorityExpired:     !now.Before(expiry),
		ReplacementAuthority: artifacts.DeveloperIDReplacementAuthority,
		CheckedTypes:         developerIDCheckTypes,
		NotChecked: []asc.DeveloperIDCheckGap{{
			Certificate: "Developer ID Installer",
			Reason:      "The App Store Connect API does not list these certificates. Review them in Certificates, Identifiers & Profiles, and inspect signed packages with asc pkg-info --path PATH.",
		}},
		Certificates: make([]asc.DeveloperIDCheckCertificate, 0, len(certificates)),
	}
	if !result.AuthorityExpired {
		result.DaysRemaining = int(expiry.Sub(now).Hours() / 24)
	}
	for _, certificate := range certificates {
		item := classifyDeveloperIDCertificate(certificate, now)
		result.Certificates = append(result.Certificates, item)
		result.Summary.Total++
		switch item.Status {
		case developerIDStatusAffected:
			result.Summary.Affected++
		case developerIDStatusLikelyAffected:
			result.Summary.LikelyAffected++
		case developerIDStatusNotAffected:
			result.Summary.NotAffected++
		default:
			result.Summary.Unknown++
		}
	}
	result.ActionRequired = developerIDCheckPending(result) > 0
	return result
}

// developerIDCheckPending counts unexpired certificates not proven unaffected.
func developerIDCheckPending(result *asc.DeveloperIDCheckResult) int {
	pending := 0
	for _, certificate := range result.Certificates {
		if !certificate.Expired && certificate.Status != developerIDStatusNotAffected {
			pending++
		}
	}
	return pending
}

// classifyDeveloperIDCertificate prefers the issuer inside certificateContent,
// because Apple's expiration check alone does not identify the authority.
func classifyDeveloperIDCertificate(certificate asc.Resource[asc.CertificateAttributes], now time.Time) asc.DeveloperIDCheckCertificate {
	attributes := certificate.Attributes
	item := asc.DeveloperIDCheckCertificate{
		ID:              certificate.ID,
		Name:            attributes.Name,
		DisplayName:     attributes.DisplayName,
		CertificateType: attributes.CertificateType,
		SerialNumber:    attributes.SerialNumber,
		ExpirationDate:  attributes.ExpirationDate,
		Activated:       attributes.Activated,
	}
	expiration, hasExpiration := parseDeveloperIDExpiration(attributes.ExpirationDate)
	if parsed := parseDeveloperIDCertificate(attributes.CertificateContent); parsed != nil {
		expiration, hasExpiration = parsed.NotAfter, true
		if item.ExpirationDate == "" {
			item.ExpirationDate = parsed.NotAfter.UTC().Format(time.RFC3339)
		}
		item.IssuerCommonName = parsed.Issuer.CommonName
		if len(parsed.Issuer.OrganizationalUnit) > 0 {
			item.IssuerOrganizationalUnit = parsed.Issuer.OrganizationalUnit[0]
		}
		if status := artifacts.DeveloperIDAuthorityStatus(parsed.Issuer); status != "" {
			item.Status, item.Evidence = status, developerIDEvidenceIssuer
		}
	}
	item.Expired = hasExpiration && !now.Before(expiration)
	if item.Status != "" {
		return item
	}
	// Apple's check covers certificates expiring any time on February 1.
	dayAfterExpiry := artifacts.DeveloperIDPreviousAuthorityExpiry.AddDate(0, 0, 1)
	switch {
	case strings.HasSuffix(attributes.CertificateType, "_G2"):
		item.Status, item.Evidence = developerIDStatusNotAffected, developerIDEvidenceCertificateType
	case !hasExpiration:
		item.Status, item.Evidence = developerIDStatusUnknown, developerIDEvidenceNone
	case expiration.Before(dayAfterExpiry):
		item.Status, item.Evidence = developerIDStatusLikelyAffected, developerIDEvidenceExpirationDate
	default:
		item.Status, item.Evidence = developerIDStatusNotAffected, developerIDEvidenceExpirationDate
	}
	return item
}

func parseDeveloperIDExpiration(value string) (time.Time, bool) {
	expiration, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	return expiration, err == nil
}

// parseDeveloperIDCertificate returns nil when certificateContent is missing or
// unreadable, which leaves the expiration date to judge by.
func parseDeveloperIDCertificate(content string) *x509.Certificate {
	der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content))
	if err != nil || len(der) == 0 {
		return nil
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil
	}
	return certificate
}
