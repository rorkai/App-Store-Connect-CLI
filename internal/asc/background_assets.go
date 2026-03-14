package asc

// BackgroundAssetAttributes describes a background asset.
type BackgroundAssetAttributes struct {
	Archived            bool   `json:"archived,omitempty"`
	AssetPackIdentifier string `json:"assetPackIdentifier,omitempty"`
	CreatedDate         string `json:"createdDate,omitempty"`
	UsedBytes           *int64 `json:"usedBytes,omitempty"`
}

// BackgroundAssetVersionStateDetails describes status details for a version.
type BackgroundAssetVersionStateDetails struct {
	Errors   []StateDetail `json:"errors,omitempty"`
	Warnings []StateDetail `json:"warnings,omitempty"`
	Infos    []StateDetail `json:"infos,omitempty"`
}

// BackgroundAssetVersionAttributes describes a background asset version.
type BackgroundAssetVersionAttributes struct {
	CreatedDate  string                              `json:"createdDate,omitempty"`
	Platforms    []Platform                          `json:"platforms,omitempty"`
	State        string                              `json:"state,omitempty"`
	StateDetails *BackgroundAssetVersionStateDetails `json:"stateDetails,omitempty"`
	Version      string                              `json:"version,omitempty"`
}

// BackgroundAssetUploadFileAssetType describes the upload file type.
type BackgroundAssetUploadFileAssetType string

const (
	BackgroundAssetUploadFileAssetTypeAsset    BackgroundAssetUploadFileAssetType = "ASSET"
	BackgroundAssetUploadFileAssetTypeManifest BackgroundAssetUploadFileAssetType = "MANIFEST"
)

// BackgroundAssetUploadFileAttributes describes a background asset upload file.
type BackgroundAssetUploadFileAttributes struct {
	AssetDeliveryState  *AppMediaAssetState                `json:"assetDeliveryState,omitempty"`
	AssetToken          string                             `json:"assetToken,omitempty"`
	AssetType           BackgroundAssetUploadFileAssetType `json:"assetType,omitempty"`
	FileName            string                             `json:"fileName,omitempty"`
	FileSize            int64                              `json:"fileSize,omitempty"`
	SourceFileChecksum  string                             `json:"sourceFileChecksum,omitempty"`
	SourceFileChecksums *Checksums                         `json:"sourceFileChecksums,omitempty"`
	UploadOperations    []UploadOperation                  `json:"uploadOperations,omitempty"`
}

// BackgroundAssetsResponse represents a background assets list response.
type BackgroundAssetsResponse = Response[BackgroundAssetAttributes]

// BackgroundAssetResponse represents a background asset response.
type BackgroundAssetResponse = SingleResponse[BackgroundAssetAttributes]

// BackgroundAssetVersionsResponse represents a background asset versions list response.
type BackgroundAssetVersionsResponse = Response[BackgroundAssetVersionAttributes]

// BackgroundAssetVersionResponse represents a background asset version response.
type BackgroundAssetVersionResponse = SingleResponse[BackgroundAssetVersionAttributes]

// BackgroundAssetVersionAppStoreReleaseAttributes describes an App Store release.
type BackgroundAssetVersionAppStoreReleaseAttributes struct {
	State string `json:"state,omitempty"`
}

// BackgroundAssetVersionExternalBetaReleaseAttributes describes an external beta release.
type BackgroundAssetVersionExternalBetaReleaseAttributes struct {
	State string `json:"state,omitempty"`
}

// BackgroundAssetVersionInternalBetaReleaseAttributes describes an internal beta release.
type BackgroundAssetVersionInternalBetaReleaseAttributes struct {
	State string `json:"state,omitempty"`
}

// BackgroundAssetVersionAppStoreReleaseResponse represents an App Store release response.
type BackgroundAssetVersionAppStoreReleaseResponse = SingleResponse[BackgroundAssetVersionAppStoreReleaseAttributes]

// BackgroundAssetVersionExternalBetaReleaseResponse represents an external beta release response.
type BackgroundAssetVersionExternalBetaReleaseResponse = SingleResponse[BackgroundAssetVersionExternalBetaReleaseAttributes]

// BackgroundAssetVersionInternalBetaReleaseResponse represents an internal beta release response.
type BackgroundAssetVersionInternalBetaReleaseResponse = SingleResponse[BackgroundAssetVersionInternalBetaReleaseAttributes]

// BackgroundAssetUploadFilesResponse represents a background asset upload files list response.
type BackgroundAssetUploadFilesResponse = Response[BackgroundAssetUploadFileAttributes]

// BackgroundAssetUploadFileResponse represents a background asset upload file response.
type BackgroundAssetUploadFileResponse = SingleResponse[BackgroundAssetUploadFileAttributes]

// BackgroundAssetCreateAttributes describes attributes for creating a background asset.
type BackgroundAssetCreateAttributes struct {
	AssetPackIdentifier string `json:"assetPackIdentifier"`
}

// BackgroundAssetCreateRelationships describes relationships for creating a background asset.
type BackgroundAssetCreateRelationships struct {
	App Relationship `json:"app"`
}

// BackgroundAssetCreateData is the data portion of a create request.
type BackgroundAssetCreateData struct {
	Type          ResourceType                       `json:"type"`
	Attributes    BackgroundAssetCreateAttributes    `json:"attributes"`
	Relationships BackgroundAssetCreateRelationships `json:"relationships"`
}

// BackgroundAssetCreateRequest is a request to create a background asset.
type BackgroundAssetCreateRequest struct {
	Data BackgroundAssetCreateData `json:"data"`
}

// BackgroundAssetUpdateAttributes describes attributes for updating a background asset.
type BackgroundAssetUpdateAttributes struct {
	Archived *bool `json:"archived,omitempty"`
}

// BackgroundAssetUpdateData is the data portion of an update request.
type BackgroundAssetUpdateData struct {
	Type       ResourceType                     `json:"type"`
	ID         string                           `json:"id"`
	Attributes *BackgroundAssetUpdateAttributes `json:"attributes,omitempty"`
}

// BackgroundAssetUpdateRequest is a request to update a background asset.
type BackgroundAssetUpdateRequest struct {
	Data BackgroundAssetUpdateData `json:"data"`
}

// BackgroundAssetVersionCreateRelationships describes relationships for creating a version.
type BackgroundAssetVersionCreateRelationships struct {
	BackgroundAsset Relationship `json:"backgroundAsset"`
}

// BackgroundAssetVersionCreateData is the data portion of a version create request.
type BackgroundAssetVersionCreateData struct {
	Type          ResourceType                              `json:"type"`
	Relationships BackgroundAssetVersionCreateRelationships `json:"relationships"`
}

// BackgroundAssetVersionCreateRequest is a request to create a background asset version.
type BackgroundAssetVersionCreateRequest struct {
	Data BackgroundAssetVersionCreateData `json:"data"`
}

// BackgroundAssetUploadFileCreateAttributes describes attributes for creating upload files.
type BackgroundAssetUploadFileCreateAttributes struct {
	AssetType BackgroundAssetUploadFileAssetType `json:"assetType"`
	FileName  string                             `json:"fileName"`
	FileSize  int64                              `json:"fileSize"`
}

// BackgroundAssetUploadFileCreateRelationships describes relationships for creating upload files.
type BackgroundAssetUploadFileCreateRelationships struct {
	BackgroundAssetVersion Relationship `json:"backgroundAssetVersion"`
}

// BackgroundAssetUploadFileCreateData is the data portion of an upload file create request.
type BackgroundAssetUploadFileCreateData struct {
	Type          ResourceType                                 `json:"type"`
	Attributes    BackgroundAssetUploadFileCreateAttributes    `json:"attributes"`
	Relationships BackgroundAssetUploadFileCreateRelationships `json:"relationships"`
}

// BackgroundAssetUploadFileCreateRequest is a request to create a background asset upload file.
type BackgroundAssetUploadFileCreateRequest struct {
	Data BackgroundAssetUploadFileCreateData `json:"data"`
}

// BackgroundAssetUploadFileUpdateAttributes describes fields for updating upload files.
type BackgroundAssetUploadFileUpdateAttributes struct {
	SourceFileChecksum  *string    `json:"sourceFileChecksum,omitempty"`
	SourceFileChecksums *Checksums `json:"sourceFileChecksums,omitempty"`
	Uploaded            *bool      `json:"uploaded,omitempty"`
}

// BackgroundAssetUploadFileUpdateData is the data portion of an upload file update request.
type BackgroundAssetUploadFileUpdateData struct {
	Type       ResourceType                               `json:"type"`
	ID         string                                     `json:"id"`
	Attributes *BackgroundAssetUploadFileUpdateAttributes `json:"attributes,omitempty"`
}

// BackgroundAssetUploadFileUpdateRequest is a request to update a background asset upload file.
type BackgroundAssetUploadFileUpdateRequest struct {
	Data BackgroundAssetUploadFileUpdateData `json:"data"`
}
