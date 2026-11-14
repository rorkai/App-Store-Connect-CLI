package asc

// PromotedPurchaseAttributes describes a promoted purchase resource.
type PromotedPurchaseAttributes struct {
	VisibleForAllUsers *bool  `json:"visibleForAllUsers,omitempty"`
	Enabled            *bool  `json:"enabled,omitempty"`
	State              string `json:"state,omitempty"`
}

// PromotedPurchasesResponse is the response from promoted purchases list endpoints.
type PromotedPurchasesResponse = Response[PromotedPurchaseAttributes]
