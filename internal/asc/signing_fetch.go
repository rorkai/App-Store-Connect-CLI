package asc

// SigningFetchResult represents CLI output for signing fetch.
type SigningFetchResult struct {
	BundleID         string                `json:"bundleId"`
	BundleIDResource string                `json:"bundleIdResourceId"`
	ProfileType      string                `json:"profileType"`
	ProfileID        string                `json:"profileId"`
	ProfileFile      string                `json:"profileFile"`
	CertificateIDs   []string              `json:"certificateIds"`
	CertificateFiles []string              `json:"certificateFiles"`
	OutputPath       string                `json:"outputPath"`
	Created          bool                  `json:"created,omitempty"`
	DeletedProfiles  []DeletedStaleProfile `json:"deletedProfiles,omitempty"`
}

// DeletedStaleProfile is one expired or invalid profile removed before fetch.
type DeletedStaleProfile struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ExpirationDate string `json:"expirationDate,omitempty"`
	State          string `json:"state,omitempty"`
}
