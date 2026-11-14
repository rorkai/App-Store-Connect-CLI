package asc

type (
	InAppPurchaseVersionResponse  = SingleResponse[InAppPurchaseVersionAttributes]
	InAppPurchaseVersionsResponse = Response[InAppPurchaseVersionAttributes]
)

// InAppPurchaseVersionImageLinkageResponse describes the version's to-one image linkage.
type InAppPurchaseVersionImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// InAppPurchaseLocalizationV2CreateAttributes describes version-scoped localization create attributes.
type InAppPurchaseLocalizationV2CreateAttributes struct {
	Name        string          `json:"name"`
	Locale      string          `json:"locale"`
	Description *NullableString `json:"description,omitempty"`
}

type InAppPurchaseLocalizationV2UpdateData struct {
	Type       ResourceType                               `json:"type"`
	ID         string                                     `json:"id"`
	Attributes *InAppPurchaseLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

type InAppPurchaseImageV2Attributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	AssetToken         string              `json:"assetToken,omitempty"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
}

type (
	InAppPurchaseImageV2Response  = SingleResponse[InAppPurchaseImageV2Attributes]
	InAppPurchaseImagesV2Response = Response[InAppPurchaseImageV2Attributes]
)

type InAppPurchaseImageV2CreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

type InAppPurchaseImageV2UpdateAttributes struct {
	Uploaded *NullableBool `json:"uploaded,omitempty"`
}
