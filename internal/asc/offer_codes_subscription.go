package asc

// SubscriptionOfferCodeCustomCodeResponse is the response from detail endpoints.
type SubscriptionOfferCodeCustomCodeResponse = SingleResponse[SubscriptionOfferCodeCustomCodeAttributes]

// SubscriptionOfferCodeCustomCodeCreateAttributes describes attributes for creating custom codes.
type SubscriptionOfferCodeCustomCodeCreateAttributes struct {
	CustomCode     string  `json:"customCode"`
	NumberOfCodes  int     `json:"numberOfCodes"`
	ExpirationDate *string `json:"expirationDate,omitempty"`
}

// SubscriptionOfferCodeCustomCodeCreateRelationships describes relationships for creating custom codes.
type SubscriptionOfferCodeCustomCodeCreateRelationships struct {
	OfferCode Relationship `json:"offerCode"`
}

// SubscriptionOfferCodeCustomCodeCreateData is the data portion of a create request.
type SubscriptionOfferCodeCustomCodeCreateData struct {
	Type          ResourceType                                       `json:"type"`
	Attributes    SubscriptionOfferCodeCustomCodeCreateAttributes    `json:"attributes"`
	Relationships SubscriptionOfferCodeCustomCodeCreateRelationships `json:"relationships"`
}

// SubscriptionOfferCodeCustomCodeCreateRequest is a request to create custom codes.
type SubscriptionOfferCodeCustomCodeCreateRequest struct {
	Data SubscriptionOfferCodeCustomCodeCreateData `json:"data"`
}

// SubscriptionOfferCodeCustomCodeUpdateAttributes describes attributes for updating custom codes.
type SubscriptionOfferCodeCustomCodeUpdateAttributes struct {
	Active *bool `json:"active,omitempty"`
}

// SubscriptionOfferCodeCustomCodeUpdateData is the data portion of an update request.
type SubscriptionOfferCodeCustomCodeUpdateData struct {
	Type       ResourceType                                    `json:"type"`
	ID         string                                          `json:"id"`
	Attributes SubscriptionOfferCodeCustomCodeUpdateAttributes `json:"attributes"`
}

// SubscriptionOfferCodeCustomCodeUpdateRequest is a request to update custom codes.
type SubscriptionOfferCodeCustomCodeUpdateRequest struct {
	Data SubscriptionOfferCodeCustomCodeUpdateData `json:"data"`
}

// SubscriptionOfferCodeOneTimeUseCodeUpdateAttributes describes attributes for updating one-time use codes.
type SubscriptionOfferCodeOneTimeUseCodeUpdateAttributes struct {
	Active *bool `json:"active,omitempty"`
}

// SubscriptionOfferCodeOneTimeUseCodeUpdateData is the data portion of an update request.
type SubscriptionOfferCodeOneTimeUseCodeUpdateData struct {
	Type       ResourceType                                        `json:"type"`
	ID         string                                              `json:"id"`
	Attributes SubscriptionOfferCodeOneTimeUseCodeUpdateAttributes `json:"attributes"`
}

// SubscriptionOfferCodeOneTimeUseCodeUpdateRequest is a request to update one-time use codes.
type SubscriptionOfferCodeOneTimeUseCodeUpdateRequest struct {
	Data SubscriptionOfferCodeOneTimeUseCodeUpdateData `json:"data"`
}

// SubscriptionOfferCodePriceRelationships describes price relationships.
type SubscriptionOfferCodePriceRelationships struct {
	Territory              Relationship  `json:"territory"`
	SubscriptionPricePoint *Relationship `json:"subscriptionPricePoint,omitempty"`
}

// SubscriptionOfferCodePriceInlineCreate describes inline creation data for prices.
type SubscriptionOfferCodePriceInlineCreate struct {
	Type          ResourceType                            `json:"type"`
	ID            string                                  `json:"id,omitempty"`
	Relationships SubscriptionOfferCodePriceRelationships `json:"relationships"`
}
