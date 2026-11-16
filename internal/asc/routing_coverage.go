package asc

// RoutingAppCoverageAttributes describes routing app coverage attributes.
type RoutingAppCoverageAttributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	SourceFileChecksum string              `json:"sourceFileChecksum,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
}

// RoutingAppCoverageCreateAttributes describes create attributes.
type RoutingAppCoverageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}
