package asc

// AppStoreVersionPromotionAttributes describes promotion metadata.
type AppStoreVersionPromotionAttributes struct {
	CreatedDate string `json:"createdDate,omitempty"`
}

// AppStoreVersionPromotionRelationships describes promotion relationships.
type AppStoreVersionPromotionRelationships struct {
	AppStoreVersion                    Relationship `json:"appStoreVersion"`
	AppStoreVersionExperimentTreatment Relationship `json:"appStoreVersionExperimentTreatment"`
}

// AppStoreVersionPromotionResponse is the response from promotion create endpoints.
type AppStoreVersionPromotionResponse struct {
	Data  Resource[AppStoreVersionPromotionAttributes] `json:"data"`
	Links Links                                        `json:"links"`
}
