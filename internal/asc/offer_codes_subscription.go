package asc

// SubscriptionCustomerEligibility represents customer eligibility for offer codes.
type SubscriptionCustomerEligibility string

const (
	SubscriptionCustomerEligibilityNew      SubscriptionCustomerEligibility = "NEW"
	SubscriptionCustomerEligibilityExisting SubscriptionCustomerEligibility = "EXISTING"
	SubscriptionCustomerEligibilityExpired  SubscriptionCustomerEligibility = "EXPIRED"
)

// SubscriptionOfferEligibility represents offer eligibility behavior.
type SubscriptionOfferEligibility string

const (
	SubscriptionOfferEligibilityStackWithIntroOffers SubscriptionOfferEligibility = "STACK_WITH_INTRO_OFFERS"
	SubscriptionOfferEligibilityReplaceIntroOffers   SubscriptionOfferEligibility = "REPLACE_INTRO_OFFERS"
)

// SubscriptionOfferCodeAttributes describes a subscription offer code.
type SubscriptionOfferCodeAttributes struct {
	Name                  string                            `json:"name,omitempty"`
	CustomerEligibilities []SubscriptionCustomerEligibility `json:"customerEligibilities,omitempty"`
	OfferEligibility      SubscriptionOfferEligibility      `json:"offerEligibility,omitempty"`
	Duration              SubscriptionOfferDuration         `json:"duration,omitempty"`
	OfferMode             SubscriptionOfferMode             `json:"offerMode,omitempty"`
	NumberOfPeriods       int                               `json:"numberOfPeriods,omitempty"`
	TotalNumberOfCodes    int                               `json:"totalNumberOfCodes,omitempty"`
	ProductionCodeCount   int                               `json:"productionCodeCount,omitempty"`
	SandboxCodeCount      int                               `json:"sandboxCodeCount,omitempty"`
	Active                bool                              `json:"active,omitempty"`
	AutoRenewEnabled      bool                              `json:"autoRenewEnabled,omitempty"`
}

// SubscriptionOfferCodeCreateAttributes describes attributes for creating an offer code.
type SubscriptionOfferCodeCreateAttributes struct {
	Name                  string                            `json:"name"`
	CustomerEligibilities []SubscriptionCustomerEligibility `json:"customerEligibilities"`
	OfferEligibility      SubscriptionOfferEligibility      `json:"offerEligibility"`
	Duration              SubscriptionOfferDuration         `json:"duration"`
	OfferMode             SubscriptionOfferMode             `json:"offerMode"`
	NumberOfPeriods       int                               `json:"numberOfPeriods"`
	AutoRenewEnabled      *bool                             `json:"autoRenewEnabled,omitempty"`
}

// SubscriptionOfferCodeUpdateAttributes describes attributes for updating an offer code.
type SubscriptionOfferCodeUpdateAttributes struct {
	Active *bool `json:"active,omitempty"`
}

// SubscriptionOfferCodeCreateRelationships describes relationships for creating an offer code.
type SubscriptionOfferCodeCreateRelationships struct {
	Subscription Relationship     `json:"subscription"`
	Prices       RelationshipList `json:"prices"`
}

// SubscriptionOfferCodeCreateData is the data portion of a create request.
type SubscriptionOfferCodeCreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    SubscriptionOfferCodeCreateAttributes    `json:"attributes"`
	Relationships SubscriptionOfferCodeCreateRelationships `json:"relationships"`
}

// SubscriptionOfferCodeCreateRequest is a request to create an offer code.
type SubscriptionOfferCodeCreateRequest struct {
	Data     SubscriptionOfferCodeCreateData          `json:"data"`
	Included []SubscriptionOfferCodePriceInlineCreate `json:"included,omitempty"`
}

// SubscriptionOfferCodeUpdateData is the data portion of an update request.
type SubscriptionOfferCodeUpdateData struct {
	Type       ResourceType                          `json:"type"`
	ID         string                                `json:"id"`
	Attributes SubscriptionOfferCodeUpdateAttributes `json:"attributes"`
}

// SubscriptionOfferCodeUpdateRequest is a request to update an offer code.
type SubscriptionOfferCodeUpdateRequest struct {
	Data SubscriptionOfferCodeUpdateData `json:"data"`
}

// SubscriptionOfferCodeResponse is the response from detail endpoints.
type SubscriptionOfferCodeResponse = SingleResponse[SubscriptionOfferCodeAttributes]

// SubscriptionOfferCodeCustomCodeAttributes describes a custom code resource.
type SubscriptionOfferCodeCustomCodeAttributes struct {
	CustomCode     string `json:"customCode,omitempty"`
	NumberOfCodes  int    `json:"numberOfCodes,omitempty"`
	CreatedDate    string `json:"createdDate,omitempty"`
	ExpirationDate string `json:"expirationDate,omitempty"`
	Active         bool   `json:"active,omitempty"`
}

// SubscriptionOfferCodeCustomCodesResponse is the response from list endpoints.
type SubscriptionOfferCodeCustomCodesResponse = Response[SubscriptionOfferCodeCustomCodeAttributes]

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

// SubscriptionOfferCodePriceAttributes describes a subscription offer code price (attributes are empty).
type SubscriptionOfferCodePriceAttributes struct{}

// SubscriptionOfferCodePriceRelationships describes price relationships.
type SubscriptionOfferCodePriceRelationships struct {
	Territory              Relationship `json:"territory"`
	SubscriptionPricePoint Relationship `json:"subscriptionPricePoint"`
}

// SubscriptionOfferCodePriceInlineCreate describes inline creation data for prices.
type SubscriptionOfferCodePriceInlineCreate struct {
	Type          ResourceType                            `json:"type"`
	ID            string                                  `json:"id,omitempty"`
	Relationships SubscriptionOfferCodePriceRelationships `json:"relationships,omitempty"`
}

// SubscriptionOfferCodePricesResponse is the response from prices endpoints.
type SubscriptionOfferCodePricesResponse = Response[SubscriptionOfferCodePriceAttributes]
