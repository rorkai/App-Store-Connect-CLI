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

// ValidWebhookEventTypes lists the event types accepted by webhook create and update requests.
var ValidWebhookEventTypes = []string{
	string(WebhookEventAlternativeDistributionPackageAvailableUpdated),
	string(WebhookEventAlternativeDistributionPackageVersionCreated),
	string(WebhookEventAlternativeDistributionTerritoryAvailabilityUpdated),
	string(WebhookEventAppStoreVersionStateUpdated),
	string(WebhookEventBackgroundAssetVersionAppStoreReleaseStateUpdated),
	string(WebhookEventBackgroundAssetVersionExternalBetaReleaseStateUpdated),
	string(WebhookEventBackgroundAssetVersionInternalBetaReleaseCreated),
	string(WebhookEventBackgroundAssetVersionStateUpdated),
	string(WebhookEventBetaFeedbackCrashSubmissionCreated),
	string(WebhookEventBetaFeedbackScreenshotSubmissionCreated),
	string(WebhookEventBuildBetaDetailExternalBuildStateUpdated),
	string(WebhookEventBuildUploadStateUpdated),
}

// WebhookAttributes describes a webhook resource.
type WebhookAttributes struct {
	Enabled    bool               `json:"enabled,omitempty"`
	EventTypes []WebhookEventType `json:"eventTypes,omitempty"`
	Name       string             `json:"name,omitempty"`
	URL        string             `json:"url,omitempty"`
}

// WebhooksResponse is the response from webhook list endpoints.
type WebhooksResponse = Response[WebhookAttributes]

// WebhookCreateAttributes describes attributes for creating a webhook.
type WebhookCreateAttributes struct {
	Enabled    bool               `json:"enabled"`
	EventTypes []WebhookEventType `json:"eventTypes"`
	Name       string             `json:"name"`
	Secret     string             `json:"secret"`
	URL        string             `json:"url"`
}

// WebhookUpdateAttributes describes attributes for updating a webhook.
type WebhookUpdateAttributes struct {
	Enabled    *bool              `json:"enabled,omitempty"`
	EventTypes []WebhookEventType `json:"eventTypes,omitempty"`
	Name       *string            `json:"name,omitempty"`
	Secret     *string            `json:"secret,omitempty"`
	URL        *string            `json:"url,omitempty"`
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

// WebhookPingResponse is the response from webhook ping endpoints.
type WebhookPingResponse = SingleResponse[struct{}]
