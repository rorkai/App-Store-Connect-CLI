package asc

// AlternativeDistributionDomainDeleteResult represents CLI output for domain deletions.
type AlternativeDistributionDomainDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AlternativeDistributionKeyDeleteResult represents CLI output for key deletions.
type AlternativeDistributionKeyDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AlternativeDistributionPackageAttributes describes an alternative distribution package.
type AlternativeDistributionPackageAttributes struct {
	SourceFileChecksum *Checksums `json:"sourceFileChecksum,omitempty"`
}

// AlternativeDistributionPackageVersionState represents a package version state.
type AlternativeDistributionPackageVersionState string

const (
	AlternativeDistributionPackageVersionStateCompleted AlternativeDistributionPackageVersionState = "COMPLETED"
	AlternativeDistributionPackageVersionStateReplaced  AlternativeDistributionPackageVersionState = "REPLACED"
)

// AlternativeDistributionPackageVersionAttributes describes a package version.
type AlternativeDistributionPackageVersionAttributes struct {
	URL               string                                     `json:"url,omitempty"`
	URLExpirationDate string                                     `json:"urlExpirationDate,omitempty"`
	Version           string                                     `json:"version,omitempty"`
	FileChecksum      string                                     `json:"fileChecksum,omitempty"`
	State             AlternativeDistributionPackageVersionState `json:"state,omitempty"`
}

// AlternativeDistributionPackageVersionsResponse is the response from package versions list endpoints.
type AlternativeDistributionPackageVersionsResponse = Response[AlternativeDistributionPackageVersionAttributes]

// AlternativeDistributionPackageVariantsResponse is the response from package variant list endpoints.
type AlternativeDistributionPackageVariantsResponse = Response[AlternativeDistributionPackageVariantAttributes]

// AlternativeDistributionPackageDeltasResponse is the response from package delta list endpoints.
type AlternativeDistributionPackageDeltasResponse = Response[AlternativeDistributionPackageDeltaAttributes]

// AppStoreVersionAlternativeDistributionPackageLinkageResponse is the response for app store version package relationships.
type AppStoreVersionAlternativeDistributionPackageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}
