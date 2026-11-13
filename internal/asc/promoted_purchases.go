package asc

// PromotedPurchaseAttributes describes a promoted purchase resource.
type PromotedPurchaseAttributes struct {
	VisibleForAllUsers *bool  `json:"visibleForAllUsers,omitempty"`
	Enabled            *bool  `json:"enabled,omitempty"`
	State              string `json:"state,omitempty"`
}

// PromotedPurchasesResponse is the response from promoted purchases list endpoints.
type PromotedPurchasesResponse = Response[PromotedPurchaseAttributes]

// PromotedPurchaseResponse is the response from promoted purchase detail endpoints.
type PromotedPurchaseResponse = SingleResponse[PromotedPurchaseAttributes]

// PromotedPurchaseCreateAttributes describes attributes for creating a promoted purchase.
type PromotedPurchaseCreateAttributes struct {
	VisibleForAllUsers bool  `json:"visibleForAllUsers"`
	Enabled            *bool `json:"enabled,omitempty"`
}

// PromotedPurchaseCreateRelationships describes relationships for creating a promoted purchase.
type PromotedPurchaseCreateRelationships struct {
	App             Relationship  `json:"app"`
	InAppPurchaseV2 *Relationship `json:"inAppPurchaseV2,omitempty"`
	Subscription    *Relationship `json:"subscription,omitempty"`
}

// PromotedPurchaseCreateData is the data portion of a promoted purchase create request.
type PromotedPurchaseCreateData struct {
	Type          ResourceType                        `json:"type"`
	Attributes    PromotedPurchaseCreateAttributes    `json:"attributes"`
	Relationships PromotedPurchaseCreateRelationships `json:"relationships"`
}

// PromotedPurchaseCreateRequest is a request to create a promoted purchase.
type PromotedPurchaseCreateRequest struct {
	Data PromotedPurchaseCreateData `json:"data"`
}

// PromotedPurchaseUpdateAttributes describes attributes for updating a promoted purchase.
type PromotedPurchaseUpdateAttributes struct {
	VisibleForAllUsers *bool `json:"visibleForAllUsers,omitempty"`
	Enabled            *bool `json:"enabled,omitempty"`
}

// PromotedPurchaseUpdateData is the data portion of a promoted purchase update request.
type PromotedPurchaseUpdateData struct {
	Type       ResourceType                      `json:"type"`
	ID         string                            `json:"id"`
	Attributes *PromotedPurchaseUpdateAttributes `json:"attributes,omitempty"`
}

// PromotedPurchaseUpdateRequest is a request to update a promoted purchase.
type PromotedPurchaseUpdateRequest struct {
	Data PromotedPurchaseUpdateData `json:"data"`
}
