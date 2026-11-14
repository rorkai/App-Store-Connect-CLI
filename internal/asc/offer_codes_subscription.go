package asc

// SubscriptionOfferCodeCustomCodeUpdateData is the data portion of an update request.
type SubscriptionOfferCodeCustomCodeUpdateData struct {
	Type       ResourceType                                    `json:"type"`
	ID         string                                          `json:"id"`
	Attributes SubscriptionOfferCodeCustomCodeUpdateAttributes `json:"attributes"`
}

// SubscriptionOfferCodeOneTimeUseCodeUpdateData is the data portion of an update request.
type SubscriptionOfferCodeOneTimeUseCodeUpdateData struct {
	Type       ResourceType                                        `json:"type"`
	ID         string                                              `json:"id"`
	Attributes SubscriptionOfferCodeOneTimeUseCodeUpdateAttributes `json:"attributes"`
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
