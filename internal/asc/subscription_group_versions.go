package asc

type (
	SubscriptionGroupVersionResponse  = SingleResponse[SubscriptionGroupVersionAttributes]
	SubscriptionGroupVersionsResponse = Response[SubscriptionGroupVersionAttributes]
)

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

type (
	SubscriptionGroupLocalizationV2Response  = SingleResponse[SubscriptionGroupLocalizationV2Attributes]
	SubscriptionGroupLocalizationsV2Response = Response[SubscriptionGroupLocalizationV2Attributes]
)
