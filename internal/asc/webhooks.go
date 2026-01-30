package asc

// WebhookEventType represents a webhook event type.
type WebhookEventType string

const (
	WebhookEventAlternativeDistributionPackageAvailableUpdated        WebhookEventType = "ALTERNATIVE_DISTRIBUTION_PACKAGE_AVAILABLE_UPDATED"
	WebhookEventAlternativeDistributionPackageVersionCreated          WebhookEventType = "ALTERNATIVE_DISTRIBUTION_PACKAGE_VERSION_CREATED"
	WebhookEventAlternativeDistributionTerritoryAvailabilityUpdated   WebhookEventType = "ALTERNATIVE_DISTRIBUTION_TERRITORY_AVAILABILITY_UPDATED"
	WebhookEventAppStoreVersionStateUpdated                           WebhookEventType = "APP_STORE_VERSION_APP_VERSION_STATE_UPDATED"
	WebhookEventBackgroundAssetVersionAppStoreReleaseStateUpdated     WebhookEventType = "BACKGROUND_ASSET_VERSION_APP_STORE_RELEASE_STATE_UPDATED"
	WebhookEventBackgroundAssetVersionExternalBetaReleaseStateUpdated WebhookEventType = "BACKGROUND_ASSET_VERSION_EXTERNAL_BETA_RELEASE_STATE_UPDATED"
	WebhookEventBackgroundAssetVersionInternalBetaReleaseCreated      WebhookEventType = "BACKGROUND_ASSET_VERSION_INTERNAL_BETA_RELEASE_CREATED"
	WebhookEventBackgroundAssetVersionStateUpdated                    WebhookEventType = "BACKGROUND_ASSET_VERSION_STATE_UPDATED"
	WebhookEventBetaFeedbackCrashSubmissionCreated                    WebhookEventType = "BETA_FEEDBACK_CRASH_SUBMISSION_CREATED"
	WebhookEventBetaFeedbackScreenshotSubmissionCreated               WebhookEventType = "BETA_FEEDBACK_SCREENSHOT_SUBMISSION_CREATED"
	WebhookEventBuildBetaDetailExternalBuildStateUpdated              WebhookEventType = "BUILD_BETA_DETAIL_EXTERNAL_BUILD_STATE_UPDATED"
	WebhookEventBuildUploadStateUpdated                               WebhookEventType = "BUILD_UPLOAD_STATE_UPDATED"
)

// WebhookAttributes describes a webhook resource.
type WebhookAttributes struct {
	Enabled    bool               `json:"enabled,omitempty"`
	EventTypes []WebhookEventType `json:"eventTypes,omitempty"`
	Name       string             `json:"name,omitempty"`
	URL        string             `json:"url,omitempty"`
}

// WebhooksResponse is the response from webhook list endpoints.
type WebhooksResponse = Response[WebhookAttributes]

// WebhookResponse is the response from webhook detail endpoints.
type WebhookResponse = SingleResponse[WebhookAttributes]

// WebhookCreateAttributes describes attributes for creating a webhook.
type WebhookCreateAttributes struct {
	Enabled    bool               `json:"enabled"`
	EventTypes []WebhookEventType `json:"eventTypes"`
	Name       string             `json:"name"`
	Secret     string             `json:"secret"`
	URL        string             `json:"url"`
}

// WebhookCreateRelationships describes relationships for creating a webhook.
type WebhookCreateRelationships struct {
	App Relationship `json:"app"`
}

// WebhookCreateData is the data portion of a webhook create request.
type WebhookCreateData struct {
	Type          ResourceType               `json:"type"`
	Attributes    WebhookCreateAttributes    `json:"attributes"`
	Relationships WebhookCreateRelationships `json:"relationships"`
}

// WebhookCreateRequest is a request to create a webhook.
type WebhookCreateRequest struct {
	Data WebhookCreateData `json:"data"`
}

// WebhookUpdateAttributes describes attributes for updating a webhook.
type WebhookUpdateAttributes struct {
	Enabled    *bool              `json:"enabled,omitempty"`
	EventTypes []WebhookEventType `json:"eventTypes,omitempty"`
	Name       *string            `json:"name,omitempty"`
	Secret     *string            `json:"secret,omitempty"`
	URL        *string            `json:"url,omitempty"`
}

// WebhookUpdateData is the data portion of a webhook update request.
type WebhookUpdateData struct {
	Type       ResourceType             `json:"type"`
	ID         string                   `json:"id"`
	Attributes *WebhookUpdateAttributes `json:"attributes,omitempty"`
}

// WebhookUpdateRequest is a request to update a webhook.
type WebhookUpdateRequest struct {
	Data WebhookUpdateData `json:"data"`
}

// WebhookDeliveryRequest describes the delivery request payload.
type WebhookDeliveryRequest struct {
	URL string `json:"url,omitempty"`
}

// WebhookDeliveryResponsePayload describes the delivery response payload.
type WebhookDeliveryResponsePayload struct {
	HTTPStatusCode int    `json:"httpStatusCode,omitempty"`
	Body           string `json:"body,omitempty"`
}

// WebhookDeliveryAttributes describes webhook delivery attributes.
type WebhookDeliveryAttributes struct {
	CreatedDate   string                          `json:"createdDate,omitempty"`
	DeliveryState string                          `json:"deliveryState,omitempty"`
	ErrorMessage  string                          `json:"errorMessage,omitempty"`
	Redelivery    *bool                           `json:"redelivery,omitempty"`
	SentDate      string                          `json:"sentDate,omitempty"`
	Request       *WebhookDeliveryRequest         `json:"request,omitempty"`
	Response      *WebhookDeliveryResponsePayload `json:"response,omitempty"`
}

// WebhookDeliveriesResponse is the response from webhook deliveries list endpoints.
type WebhookDeliveriesResponse = Response[WebhookDeliveryAttributes]

// WebhookDeliveriesLinkagesResponse is the response from webhook delivery relationship endpoints.
type WebhookDeliveriesLinkagesResponse = LinkagesResponse

// WebhookDeliveryResponse is the response from webhook delivery endpoints.
type WebhookDeliveryResponse = SingleResponse[WebhookDeliveryAttributes]

// WebhookDeliveryCreateRelationships describes relationships for delivery create requests.
type WebhookDeliveryCreateRelationships struct {
	Template Relationship `json:"template"`
}

// WebhookDeliveryCreateData is the data portion of a webhook delivery create request.
type WebhookDeliveryCreateData struct {
	Type          ResourceType                       `json:"type"`
	Relationships WebhookDeliveryCreateRelationships `json:"relationships"`
}

// WebhookDeliveryCreateRequest is a request to create a webhook delivery.
type WebhookDeliveryCreateRequest struct {
	Data WebhookDeliveryCreateData `json:"data"`
}

// WebhookPingResponse is the response from webhook ping endpoints.
type WebhookPingResponse = SingleResponse[struct{}]

// WebhookPingCreateRelationships describes relationships for ping requests.
type WebhookPingCreateRelationships struct {
	Webhook Relationship `json:"webhook"`
}

// WebhookPingCreateData is the data portion of a webhook ping request.
type WebhookPingCreateData struct {
	Type          ResourceType                   `json:"type"`
	Relationships WebhookPingCreateRelationships `json:"relationships"`
}

// WebhookPingCreateRequest is a request to create a webhook ping.
type WebhookPingCreateRequest struct {
	Data WebhookPingCreateData `json:"data"`
}
