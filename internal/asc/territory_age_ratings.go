package asc

// TerritoryAgeRatingAttributes describes territory age rating attributes.
type TerritoryAgeRatingAttributes struct {
	AppStoreAgeRating AppStoreAgeRating `json:"appStoreAgeRating,omitempty"`
}

// TerritoryAgeRatingRelationships describes territory age rating relationships.
type TerritoryAgeRatingRelationships struct {
	Territory Relationship `json:"territory"`
}

// TerritoryAgeRatingsResponse is the response from territory age ratings endpoints.
type TerritoryAgeRatingsResponse = Response[TerritoryAgeRatingAttributes]
