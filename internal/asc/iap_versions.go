package asc

// InAppPurchaseVersionAttributes describes an IAP version lifecycle resource.
type InAppPurchaseVersionAttributes struct {
	Version int    `json:"version,omitempty"`
	State   string `json:"state,omitempty"`
}

type (
	InAppPurchaseVersionResponse  = SingleResponse[InAppPurchaseVersionAttributes]
	InAppPurchaseVersionsResponse = Response[InAppPurchaseVersionAttributes]
)

type InAppPurchaseVersionCreateRelationships struct {
	InAppPurchase Relationship `json:"inAppPurchase"`
}

type InAppPurchaseVersionCreateData struct {
	Type          ResourceType                            `json:"type"`
	Relationships InAppPurchaseVersionCreateRelationships `json:"relationships"`
}

type InAppPurchaseVersionCreateRequest struct {
	Data InAppPurchaseVersionCreateData `json:"data"`
}

// InAppPurchaseVersionImageLinkageResponse describes the version's to-one image linkage.
type InAppPurchaseVersionImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

type InAppPurchaseLocalizationV2CreateRelationships struct {
	Version Relationship `json:"version"`
}

// InAppPurchaseLocalizationV2CreateAttributes describes version-scoped localization create attributes.
type InAppPurchaseLocalizationV2CreateAttributes struct {
	Name        string          `json:"name"`
	Locale      string          `json:"locale"`
	Description *NullableString `json:"description,omitempty"`
}

type InAppPurchaseLocalizationV2CreateData struct {
	Type          ResourceType                                   `json:"type"`
	Attributes    InAppPurchaseLocalizationV2CreateAttributes    `json:"attributes"`
	Relationships InAppPurchaseLocalizationV2CreateRelationships `json:"relationships"`
}

type InAppPurchaseLocalizationV2CreateRequest struct {
	Data InAppPurchaseLocalizationV2CreateData `json:"data"`
}

type InAppPurchaseLocalizationV2UpdateData struct {
	Type       ResourceType                               `json:"type"`
	ID         string                                     `json:"id"`
	Attributes *InAppPurchaseLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

type InAppPurchaseLocalizationV2UpdateRequest struct {
	Data InAppPurchaseLocalizationV2UpdateData `json:"data"`
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

type InAppPurchaseImageV2CreateRelationships struct {
	Version Relationship `json:"version"`
}

type InAppPurchaseImageV2CreateData struct {
	Type          ResourceType                            `json:"type"`
	Attributes    InAppPurchaseImageV2CreateAttributes    `json:"attributes"`
	Relationships InAppPurchaseImageV2CreateRelationships `json:"relationships"`
}

type InAppPurchaseImageV2CreateRequest struct {
	Data InAppPurchaseImageV2CreateData `json:"data"`
}

type InAppPurchaseImageV2UpdateAttributes struct {
	Uploaded *NullableBool `json:"uploaded,omitempty"`
}

type InAppPurchaseImageV2UpdateData struct {
	Type       ResourceType                          `json:"type"`
	ID         string                                `json:"id"`
	Attributes *InAppPurchaseImageV2UpdateAttributes `json:"attributes,omitempty"`
}

type InAppPurchaseImageV2UpdateRequest struct {
	Data InAppPurchaseImageV2UpdateData `json:"data"`
}
