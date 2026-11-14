package asc

// AlternativeDistributionDomainDeleteResult represents CLI output for domain deletions.
type AlternativeDistributionDomainDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AlternativeDistributionKeyAttributes describes an alternative distribution key.
type AlternativeDistributionKeyAttributes struct {
	PublicKey string `json:"publicKey,omitempty"`
}

// AlternativeDistributionKeysResponse is the response from key list endpoints.
type AlternativeDistributionKeysResponse = Response[AlternativeDistributionKeyAttributes]

// AlternativeDistributionKeyResponse is the response from key detail endpoints.
type AlternativeDistributionKeyResponse = SingleResponse[AlternativeDistributionKeyAttributes]

// AlternativeDistributionKeyCreateAttributes describes attributes for creating a key.
type AlternativeDistributionKeyCreateAttributes struct {
	PublicKey string `json:"publicKey"`
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

// AlternativeDistributionKeyCreateRequest is a request to create a key.
type AlternativeDistributionKeyCreateRequest struct {
	Data AlternativeDistributionKeyCreateData `json:"data"`
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

// AlternativeDistributionPackageResponse is the response from package detail endpoints.
type AlternativeDistributionPackageResponse = SingleResponse[AlternativeDistributionPackageAttributes]

// AlternativeDistributionPackageCreateRelationships describes relationships for package create requests.
type AlternativeDistributionPackageCreateRelationships struct {
	AppStoreVersion Relationship `json:"appStoreVersion"`
}

// AlternativeDistributionPackageCreateData is the data payload for package create requests.
type AlternativeDistributionPackageCreateData struct {
	Type          ResourceType                                      `json:"type"`
	Relationships AlternativeDistributionPackageCreateRelationships `json:"relationships"`
}

// AlternativeDistributionPackageCreateRequest is a request to create a package.
type AlternativeDistributionPackageCreateRequest struct {
	Data AlternativeDistributionPackageCreateData `json:"data"`
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

// AlternativeDistributionPackageVersionResponse is the response from package version detail endpoints.
type AlternativeDistributionPackageVersionResponse = SingleResponse[AlternativeDistributionPackageVersionAttributes]

// AlternativeDistributionPackageVariantAttributes describes a package variant.
type AlternativeDistributionPackageVariantAttributes struct {
	URL                            string `json:"url,omitempty"`
	URLExpirationDate              string `json:"urlExpirationDate,omitempty"`
	AlternativeDistributionKeyBlob string `json:"alternativeDistributionKeyBlob,omitempty"`
	FileChecksum                   string `json:"fileChecksum,omitempty"`
}

// AlternativeDistributionPackageVariantsResponse is the response from package variant list endpoints.
type AlternativeDistributionPackageVariantsResponse = Response[AlternativeDistributionPackageVariantAttributes]

// AlternativeDistributionPackageVariantResponse is the response from package variant detail endpoints.
type AlternativeDistributionPackageVariantResponse = SingleResponse[AlternativeDistributionPackageVariantAttributes]

// AlternativeDistributionPackageDeltaAttributes describes a package delta.
type AlternativeDistributionPackageDeltaAttributes struct {
	URL                            string `json:"url,omitempty"`
	URLExpirationDate              string `json:"urlExpirationDate,omitempty"`
	AlternativeDistributionKeyBlob string `json:"alternativeDistributionKeyBlob,omitempty"`
	FileChecksum                   string `json:"fileChecksum,omitempty"`
}

// AlternativeDistributionPackageDeltasResponse is the response from package delta list endpoints.
type AlternativeDistributionPackageDeltasResponse = Response[AlternativeDistributionPackageDeltaAttributes]

// AlternativeDistributionPackageDeltaResponse is the response from package delta detail endpoints.
type AlternativeDistributionPackageDeltaResponse = SingleResponse[AlternativeDistributionPackageDeltaAttributes]

// AppStoreVersionAlternativeDistributionPackageLinkageResponse is the response for app store version package relationships.
type AppStoreVersionAlternativeDistributionPackageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}
