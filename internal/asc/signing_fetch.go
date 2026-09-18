package asc

// SigningFetchResult represents CLI output for signing fetch.
type SigningFetchResult struct {
	BundleID             string   `json:"bundleId"`
	BundleIDResource     string   `json:"bundleIdResourceId"`
	ProfileType          string   `json:"profileType"`
	ProfileID            string   `json:"profileId"`
	ProfileFile          string   `json:"profileFile"`
	CertificateIDs       []string `json:"certificateIds"`
	CertificateFiles     []string `json:"certificateFiles"`
	OutputPath           string   `json:"outputPath"`
	Created              bool     `json:"created,omitempty"`
	CertificateCreated   *bool    `json:"certificateCreated,omitempty"`
	CertificateSHA256    string   `json:"certificateSha256,omitempty"`
	PrivateKeyPath       string   `json:"privateKeyPath,omitempty"`
	CSRPath              string   `json:"csrPath,omitempty"`
	P12Path              string   `json:"p12Path,omitempty"`
	ProfilesMetadataPath string   `json:"profilesMetadataPath,omitempty"`
}
