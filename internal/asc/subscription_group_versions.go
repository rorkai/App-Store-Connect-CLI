package asc

// SubscriptionGroupVersionAttributes describes a discrete group review version.
type SubscriptionGroupVersionAttributes struct {
	Version int    `json:"version,omitempty"`
	State   string `json:"state,omitempty"`
}

// SubscriptionGroupLocalizationV2Attributes describes version-scoped localization attributes.
// Unlike the legacy v1 resource, the v2 schema does not expose state.
type SubscriptionGroupLocalizationV2Attributes struct {
	Name          string `json:"name,omitempty"`
	CustomAppName string `json:"customAppName,omitempty"`
	Locale        string `json:"locale,omitempty"`
}

type (
	SubscriptionGroupVersionResponse  = SingleResponse[SubscriptionGroupVersionAttributes]
	SubscriptionGroupVersionsResponse = Response[SubscriptionGroupVersionAttributes]
)

// SubscriptionGroupVersionCreateRelationships contains the required group owner.
type SubscriptionGroupVersionCreateRelationships struct {
	SubscriptionGroup Relationship `json:"subscriptionGroup"`
}

// SubscriptionGroupVersionCreateData is the data portion of a create request.
type SubscriptionGroupVersionCreateData struct {
	Type          ResourceType                                `json:"type"`
	Relationships SubscriptionGroupVersionCreateRelationships `json:"relationships"`
}

// SubscriptionGroupVersionCreateRequest creates a version for a group.
type SubscriptionGroupVersionCreateRequest struct {
	Data SubscriptionGroupVersionCreateData `json:"data"`
}

// SubscriptionGroupLocalizationV2CreateAttributes describes version-scoped localization attributes.
type SubscriptionGroupLocalizationV2CreateAttributes struct {
	Name          string          `json:"name"`
	CustomAppName *NullableString `json:"customAppName,omitempty"`
	Locale        string          `json:"locale"`
}

// SubscriptionGroupLocalizationV2UpdateAttributes describes nullable v2 updates.
type SubscriptionGroupLocalizationV2UpdateAttributes struct {
	Name          *NullableString `json:"name,omitempty"`
	CustomAppName *NullableString `json:"customAppName,omitempty"`
}

type SubscriptionGroupLocalizationV2CreateRelationships struct {
	Version Relationship `json:"version"`
}

type SubscriptionGroupLocalizationV2CreateData struct {
	Type          ResourceType                                       `json:"type"`
	Attributes    SubscriptionGroupLocalizationV2CreateAttributes    `json:"attributes"`
	Relationships SubscriptionGroupLocalizationV2CreateRelationships `json:"relationships"`
}

type SubscriptionGroupLocalizationV2CreateRequest struct {
	Data SubscriptionGroupLocalizationV2CreateData `json:"data"`
}

type SubscriptionGroupLocalizationV2UpdateData struct {
	Type       ResourceType                                     `json:"type"`
	ID         string                                           `json:"id"`
	Attributes *SubscriptionGroupLocalizationV2UpdateAttributes `json:"attributes,omitempty"`
}

type SubscriptionGroupLocalizationV2UpdateRequest struct {
	Data SubscriptionGroupLocalizationV2UpdateData `json:"data"`
}

type (
	SubscriptionGroupLocalizationV2Response  = SingleResponse[SubscriptionGroupLocalizationV2Attributes]
	SubscriptionGroupLocalizationsV2Response = Response[SubscriptionGroupLocalizationV2Attributes]
)
