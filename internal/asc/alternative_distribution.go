package asc

// AlternativeDistributionDomainDeleteResult represents CLI output for domain deletions.
type AlternativeDistributionDomainDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AlternativeDistributionKeyCreateRelationships describes relationships for key create requests.
type AlternativeDistributionKeyCreateRelationships struct {
	App *Relationship `json:"app,omitempty"`
}

// AlternativeDistributionKeyCreateData is the data payload for key create requests.
type AlternativeDistributionKeyCreateData struct {
	Type          ResourceType                                   `json:"type"`
	Attributes    AlternativeDistributionKeyCreateAttributes     `json:"attributes"`
	Relationships *AlternativeDistributionKeyCreateRelationships `json:"relationships,omitempty"`
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

// AlternativeDistributionPackageCreateRelationships describes relationships for package create requests.
type AlternativeDistributionPackageCreateRelationships struct {
	AppStoreVersion Relationship `json:"appStoreVersion"`
}

// AlternativeDistributionPackageCreateData is the data payload for package create requests.
type AlternativeDistributionPackageCreateData struct {
	Type          ResourceType                                      `json:"type"`
	Relationships AlternativeDistributionPackageCreateRelationships `json:"relationships"`
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
