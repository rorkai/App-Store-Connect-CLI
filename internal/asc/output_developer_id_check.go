package asc

import "strconv"

// DeveloperIDCheckResult reports API-visible Developer ID certificates against
// the expiring Developer ID Sub-CA.
type DeveloperIDCheckResult struct {
	AuthorityExpiresOn   string                        `json:"authorityExpiresOn"`
	AuthorityExpired     bool                          `json:"authorityExpired"`
	DaysRemaining        int                           `json:"daysRemaining"`
	ReplacementAuthority string                        `json:"replacementAuthority"`
	ActionRequired       bool                          `json:"actionRequired"`
	Summary              DeveloperIDCheckSummary       `json:"summary"`
	CheckedTypes         []string                      `json:"checkedTypes"`
	NotChecked           []DeveloperIDCheckGap         `json:"notChecked"`
	Certificates         []DeveloperIDCheckCertificate `json:"certificates"`
}

// DeveloperIDCheckSummary counts certificates by status.
type DeveloperIDCheckSummary struct {
	Total          int `json:"total"`
	Affected       int `json:"affected"`
	LikelyAffected int `json:"likelyAffected"`
	NotAffected    int `json:"notAffected"`
	Unknown        int `json:"unknown"`
}

// DeveloperIDCheckGap names certificates the check cannot see.
type DeveloperIDCheckGap struct {
	Certificate string `json:"certificate"`
	Reason      string `json:"reason"`
}

// DeveloperIDCheckCertificate is the verdict for one certificate. Evidence
// names what the status rests on: issuer, certificateType, expirationDate, or
// none.
type DeveloperIDCheckCertificate struct {
	ID                       string `json:"id"`
	Name                     string `json:"name,omitempty"`
	DisplayName              string `json:"displayName,omitempty"`
	CertificateType          string `json:"certificateType,omitempty"`
	SerialNumber             string `json:"serialNumber,omitempty"`
	ExpirationDate           string `json:"expirationDate,omitempty"`
	Activated                *bool  `json:"activated,omitempty"`
	Expired                  bool   `json:"expired"`
	Status                   string `json:"status"`
	Evidence                 string `json:"evidence"`
	IssuerCommonName         string `json:"issuerCommonName,omitempty"`
	IssuerOrganizationalUnit string `json:"issuerOrganizationalUnit,omitempty"`
}

func developerIDCheckTables(result *DeveloperIDCheckResult, render func([]string, [][]string)) error {
	render(
		[]string{"Authority Expires", "Days Remaining", "Action Required", "Total", "Affected", "Likely Affected", "Not Affected", "Unknown"},
		[][]string{{
			result.AuthorityExpiresOn,
			strconv.Itoa(result.DaysRemaining),
			formatBool(result.ActionRequired),
			strconv.Itoa(result.Summary.Total),
			strconv.Itoa(result.Summary.Affected),
			strconv.Itoa(result.Summary.LikelyAffected),
			strconv.Itoa(result.Summary.NotAffected),
			strconv.Itoa(result.Summary.Unknown),
		}},
	)
	if len(result.Certificates) > 0 {
		rows := make([][]string, 0, len(result.Certificates))
		for _, certificate := range result.Certificates {
			rows = append(rows, []string{
				certificate.ID,
				certificate.CertificateType,
				certificate.Name,
				certificate.ExpirationDate,
				formatBool(certificate.Expired),
				certificate.Status,
				certificate.Evidence,
				certificate.IssuerOrganizationalUnit,
			})
		}
		render([]string{"ID", "Type", "Name", "Expires", "Expired", "Status", "Evidence", "Issuer OU"}, rows)
	}
	rows := make([][]string, 0, len(result.NotChecked))
	for _, gap := range result.NotChecked {
		rows = append(rows, []string{gap.Certificate, gap.Reason})
	}
	render([]string{"Not Checked", "Reason"}, rows)
	return nil
}
