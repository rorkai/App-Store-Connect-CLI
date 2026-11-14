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
	Locale       string                              `json:"locale,omitempty"`
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

// BackgroundAssetVersionsResponse represents a background asset versions list response.
type BackgroundAssetVersionsResponse = Response[BackgroundAssetVersionAttributes]

// BackgroundAssetVersionAppStoreReleaseAttributes describes an App Store release.
type BackgroundAssetVersionAppStoreReleaseAttributes struct {
	State string `json:"state,omitempty"`
}

// BackgroundAssetVersionExternalBetaReleaseAttributes describes an external beta release.
type BackgroundAssetVersionExternalBetaReleaseAttributes struct {
	State string `json:"state,omitempty"`
}

// BackgroundAssetUploadFilesResponse represents a background asset upload files list response.
type BackgroundAssetUploadFilesResponse = Response[BackgroundAssetUploadFileAttributes]

// BackgroundAssetUploadFileCreateAttributes describes attributes for creating upload files.
type BackgroundAssetUploadFileCreateAttributes struct {
	AssetType BackgroundAssetUploadFileAssetType `json:"assetType"`
	FileName  string                             `json:"fileName"`
	FileSize  int64                              `json:"fileSize"`
}

// BackgroundAssetUploadFileUpdateAttributes describes fields for updating upload files.
type BackgroundAssetUploadFileUpdateAttributes struct {
	SourceFileChecksum  *string    `json:"sourceFileChecksum,omitempty"`
	SourceFileChecksums *Checksums `json:"sourceFileChecksums,omitempty"`
	Uploaded            *bool      `json:"uploaded,omitempty"`
}
