package asc

// SigningFetchResult represents CLI output for signing fetch.
type SigningFetchResult struct {
	BundleID                 string                     `json:"bundleId"`
	BundleIDResource         string                     `json:"bundleIdResourceId"`
	ProfileType              string                     `json:"profileType"`
	ProfileID                string                     `json:"profileId"`
	ProfileFile              string                     `json:"profileFile"`
	CertificateIDs           []string                   `json:"certificateIds"`
	CertificateFiles         []string                   `json:"certificateFiles"`
	OutputPath               string                     `json:"outputPath"`
	Created                  bool                       `json:"created,omitempty"`
	CertificateCreated       *bool                      `json:"certificateCreated,omitempty"`
	CertificateSHA256        string                     `json:"certificateSha256,omitempty"`
	PrivateKeyPath           string                     `json:"privateKeyPath,omitempty"`
	CSRPath                  string                     `json:"csrPath,omitempty"`
	P12Path                  string                     `json:"p12Path,omitempty"`
	ProfilesMetadataPath     string                     `json:"profilesMetadataPath,omitempty"`
	CertificateCreationState string                     `json:"certificateCreationState,omitempty"`
	ProfileCreationState     string                     `json:"profileCreationState,omitempty"`
	Partial                  bool                       `json:"partial,omitempty"`
	StaleProfiles            *SigningFetchStaleProfiles `json:"staleProfiles,omitempty"`
}

// SigningFetchStaleProfiles reports the --delete-stale-profiles plan and its
// outcome. Planned lists every expired or INVALID profile found; Deleted lists
// only profiles Apple confirmed deleted; Failed lists profiles whose deletion
// returned an error. DryRun is true when nothing was deleted by design.
type SigningFetchStaleProfiles struct {
	DryRun  bool                         `json:"dryRun,omitempty"`
	Planned []SigningStaleProfile        `json:"planned"`
	Deleted []SigningStaleProfile        `json:"deleted"`
	Failed  []SigningStaleProfileFailure `json:"failed,omitempty"`
}

// SigningStaleProfile is one expired or invalid profile selected for deletion.
type SigningStaleProfile struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ExpirationDate string `json:"expirationDate,omitempty"`
	State          string `json:"state,omitempty"`
}

// SigningStaleProfileFailure is one stale profile whose deletion failed.
type SigningStaleProfileFailure struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Error string `json:"error"`
}
