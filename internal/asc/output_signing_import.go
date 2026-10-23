package asc

// SigningSyncImportResult is the receipt for signing sync import. It lists
// every certificate and profile decrypted and verified from the source
// repository and the destination artifacts that were written or, with DryRun,
// would be written.
type SigningSyncImportResult struct {
	Operation        string                         `json:"operation"`
	Source           string                         `json:"source"`
	RepoURL          string                         `json:"repoUrl,omitempty"`
	Storage          *SigningSyncStorage            `json:"storage,omitempty"`
	DryRun           bool                           `json:"dryRun"`
	Certificates     []SigningSyncImportCertificate `json:"certificates"`
	Profiles         []SigningSyncImportProfile     `json:"profiles"`
	Files            []string                       `json:"files"`
	PublicationState string                         `json:"publicationState"`
}

// SigningSyncImportCertificate is one certificate paired with its private key.
type SigningSyncImportCertificate struct {
	SourcePath     string `json:"sourcePath"`
	Type           string `json:"type"`
	Name           string `json:"name"`
	TeamID         string `json:"teamId"`
	SerialNumber   string `json:"serialNumber"`
	SHA256         string `json:"sha256"`
	ExpirationDate string `json:"expirationDate"`
}

// SigningSyncImportProfile is one verified provisioning profile.
type SigningSyncImportProfile struct {
	SourcePath     string `json:"sourcePath"`
	Name           string `json:"name"`
	UUID           string `json:"uuid"`
	BundleID       string `json:"bundleId"`
	ProfileType    string `json:"profileType"`
	TeamID         string `json:"teamId"`
	ExpirationDate string `json:"expirationDate"`
}

func signingSyncImportRows(result *SigningSyncImportResult) ([]string, [][]string) {
	headers := []string{"Kind", "Name", "Type", "Team", "Bundle ID", "Expires", "ID"}
	if result == nil {
		return headers, nil
	}
	rows := make([][]string, 0, len(result.Certificates)+len(result.Profiles))
	for _, certificate := range result.Certificates {
		rows = append(rows, []string{"certificate", certificate.Name, certificate.Type, certificate.TeamID, "", certificate.ExpirationDate, certificate.SHA256})
	}
	for _, profile := range result.Profiles {
		rows = append(rows, []string{"profile", profile.Name, profile.ProfileType, profile.TeamID, profile.BundleID, profile.ExpirationDate, profile.UUID})
	}
	return headers, rows
}
