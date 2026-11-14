package asc

// PromotedPurchaseAttributes describes a promoted purchase resource.
type PromotedPurchaseAttributes struct {
	VisibleForAllUsers *bool  `json:"visibleForAllUsers,omitempty"`
	Enabled            *bool  `json:"enabled,omitempty"`
	State              string `json:"state,omitempty"`
}

// PromotedPurchasesResponse is the response from promoted purchases list endpoints.
type PromotedPurchasesResponse = Response[PromotedPurchaseAttributes]

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
