package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// RetentionMessageAttributes describes a retention message.
type RetentionMessageAttributes struct {
	ReferenceName string `json:"referenceName,omitempty"`

	api apiAttributes
}

// RetentionMessageVersionAttributes describes a retention message version.
type RetentionMessageVersionAttributes struct {
	State string `json:"state,omitempty"`

	api apiAttributes
}

// RetentionMessageLocalizationAttributes describes a retention message localization.
type RetentionMessageLocalizationAttributes struct {
	Locale string `json:"locale,omitempty"`
	Header string `json:"header,omitempty"`
	Body   string `json:"body,omitempty"`

	api apiAttributes
}

func (a *RetentionMessageAttributes) UnmarshalJSON(data []byte) error {
	type alias RetentionMessageAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a RetentionMessageAttributes) MarshalJSON() ([]byte, error) {
	type alias RetentionMessageAttributes
	return a.api.marshal(alias(a))
}

func (a *RetentionMessageVersionAttributes) UnmarshalJSON(data []byte) error {
	type alias RetentionMessageVersionAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a RetentionMessageVersionAttributes) MarshalJSON() ([]byte, error) {
	type alias RetentionMessageVersionAttributes
	return a.api.marshal(alias(a))
}

func (a *RetentionMessageLocalizationAttributes) UnmarshalJSON(data []byte) error {
	type alias RetentionMessageLocalizationAttributes
	return unmarshalAPIAttributes(data, (*alias)(a), &a.api)
}

func (a RetentionMessageLocalizationAttributes) MarshalJSON() ([]byte, error) {
	type alias RetentionMessageLocalizationAttributes
	return a.api.marshal(alias(a))
}

type (
	RetentionMessageResponse              = SingleResponse[RetentionMessageAttributes]
	RetentionMessagesResponse             = Response[RetentionMessageAttributes]
	RetentionMessageVersionResponse       = SingleResponse[RetentionMessageVersionAttributes]
	RetentionMessageVersionsResponse      = Response[RetentionMessageVersionAttributes]
	RetentionMessageLocalizationResponse  = SingleResponse[RetentionMessageLocalizationAttributes]
	RetentionMessageLocalizationsResponse = Response[RetentionMessageLocalizationAttributes]
)

// RetentionMessageLinkageResponse is a to-one retention messaging linkage; data is null when unset.
type RetentionMessageLinkageResponse struct {
	Data  *ResourceData `json:"data"`
	Links Links         `json:"links"`
}

// RetentionMessageLocalizationCreateAttributes describes a localization create request.
type RetentionMessageLocalizationCreateAttributes struct {
	Locale string  `json:"locale"`
	Header *string `json:"header,omitempty"`
	Body   *string `json:"body,omitempty"`
}

// RetentionMessageLocalizationUpdateAttributes describes a localization update
// request; a NullableString with a nil Value clears the text.
type RetentionMessageLocalizationUpdateAttributes struct {
	Header *NullableString `json:"header,omitempty"`
	Body   *NullableString `json:"body,omitempty"`
}

// RetentionMessagesOption configures a retention messaging read.
type RetentionMessagesOption func(*retentionMessagesQuery)

type retentionMessagesQuery struct {
	nextURL string
	values  url.Values
}

// WithRetentionMessagesNextURL fetches a links.next URL instead of building a query.
func WithRetentionMessagesNextURL(next string) RetentionMessagesOption {
	return func(q *retentionMessagesQuery) { q.nextURL = strings.TrimSpace(next) }
}

// WithRetentionMessagesQuery sets one query parameter, such as include,
// fields[type], filter[name], sort, limit, or limit[relationship], to the
// comma-joined values. Empty values leave the parameter unset.
func WithRetentionMessagesQuery(name string, values ...string) RetentionMessagesOption {
	return func(q *retentionMessagesQuery) { addCSV(q.values, name, values) }
}

type retentionMessageWriteData struct {
	Type          ResourceType            `json:"type"`
	ID            string                  `json:"id,omitempty"`
	Attributes    any                     `json:"attributes,omitempty"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
}

type retentionMessageWriteRequest struct {
	Data retentionMessageWriteData `json:"data"`
}

func getRetentionMessageRead[T any](c *Client, ctx context.Context, pathFormat, id string, opts []RetentionMessagesOption) (*T, error) {
	q := &retentionMessagesQuery{values: url.Values{}}
	for _, opt := range opts {
		opt(q)
	}
	id = strings.TrimSpace(id)
	path := fmt.Sprintf(pathFormat, id)
	if q.nextURL != "" {
		if err := validateNextURL(q.nextURL); err != nil {
			return nil, fmt.Errorf("retention messages: %w", err)
		}
		path = q.nextURL
	} else if id == "" {
		return nil, fmt.Errorf("resource ID is required")
	} else if query := q.values.Encode(); query != "" {
		path += "?" + query
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response T
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

func writeRetentionMessageResource[T any](c *Client, ctx context.Context, method, path string, data retentionMessageWriteData) (*T, error) {
	body, err := BuildRequestBody(retentionMessageWriteRequest{Data: data})
	if err != nil {
		return nil, err
	}
	response, err := c.do(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	var decoded T
	if err := json.Unmarshal(response, &decoded); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &decoded, nil
}

func retentionMessageRelationship(resourceType ResourceType, id string) Relationship {
	return Relationship{Data: ResourceData{Type: resourceType, ID: strings.TrimSpace(id)}}
}

func requireRetentionMessageID(name, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

// GetAppRetentionMessages lists retention messages for an app.
func (c *Client) GetAppRetentionMessages(ctx context.Context, appID string, opts ...RetentionMessagesOption) (*RetentionMessagesResponse, error) {
	return getRetentionMessageRead[RetentionMessagesResponse](c, ctx, "/v1/apps/%s/retentionMessages", appID, opts)
}

// GetAppRetentionMessagesRelationships lists retention message linkages for an app.
func (c *Client) GetAppRetentionMessagesRelationships(ctx context.Context, appID string, opts ...RetentionMessagesOption) (*LinkagesResponse, error) {
	return getRetentionMessageRead[LinkagesResponse](c, ctx, "/v1/apps/%s/relationships/retentionMessages", appID, opts)
}

// GetRetentionMessage retrieves a retention message.
func (c *Client) GetRetentionMessage(ctx context.Context, messageID string, opts ...RetentionMessagesOption) (*RetentionMessageResponse, error) {
	return getRetentionMessageRead[RetentionMessageResponse](c, ctx, "/v1/retentionMessages/%s", messageID, opts)
}

// GetRetentionMessageVersions lists versions of a retention message.
func (c *Client) GetRetentionMessageVersions(ctx context.Context, messageID string, opts ...RetentionMessagesOption) (*RetentionMessageVersionsResponse, error) {
	return getRetentionMessageRead[RetentionMessageVersionsResponse](c, ctx, "/v1/retentionMessages/%s/versions", messageID, opts)
}

// GetRetentionMessageVersionsRelationships lists version linkages of a retention message.
func (c *Client) GetRetentionMessageVersionsRelationships(ctx context.Context, messageID string, opts ...RetentionMessagesOption) (*LinkagesResponse, error) {
	return getRetentionMessageRead[LinkagesResponse](c, ctx, "/v1/retentionMessages/%s/relationships/versions", messageID, opts)
}

// GetRetentionMessageSubscriptions lists subscriptions assigned to a retention message.
func (c *Client) GetRetentionMessageSubscriptions(ctx context.Context, messageID string, opts ...RetentionMessagesOption) (*SubscriptionsResponse, error) {
	return getRetentionMessageRead[SubscriptionsResponse](c, ctx, "/v1/retentionMessages/%s/subscriptions", messageID, opts)
}

// GetRetentionMessageSubscriptionsRelationships lists subscription linkages of a retention message.
func (c *Client) GetRetentionMessageSubscriptionsRelationships(ctx context.Context, messageID string, opts ...RetentionMessagesOption) (*LinkagesResponse, error) {
	return getRetentionMessageRead[LinkagesResponse](c, ctx, "/v1/retentionMessages/%s/relationships/subscriptions", messageID, opts)
}

// GetRetentionMessageVersion retrieves a retention message version.
func (c *Client) GetRetentionMessageVersion(ctx context.Context, versionID string, opts ...RetentionMessagesOption) (*RetentionMessageVersionResponse, error) {
	return getRetentionMessageRead[RetentionMessageVersionResponse](c, ctx, "/v1/retentionMessageVersions/%s", versionID, opts)
}

// GetRetentionMessageVersionLocalizations lists localizations of a retention message version.
func (c *Client) GetRetentionMessageVersionLocalizations(ctx context.Context, versionID string, opts ...RetentionMessagesOption) (*RetentionMessageLocalizationsResponse, error) {
	return getRetentionMessageRead[RetentionMessageLocalizationsResponse](c, ctx, "/v1/retentionMessageVersions/%s/localizations", versionID, opts)
}

// GetRetentionMessageVersionLocalizationsRelationships lists localization linkages of a version.
func (c *Client) GetRetentionMessageVersionLocalizationsRelationships(ctx context.Context, versionID string, opts ...RetentionMessagesOption) (*LinkagesResponse, error) {
	return getRetentionMessageRead[LinkagesResponse](c, ctx, "/v1/retentionMessageVersions/%s/relationships/localizations", versionID, opts)
}

// GetRetentionMessageVersionPlacement retrieves the raw asset library placement of a version.
func (c *Client) GetRetentionMessageVersionPlacement(ctx context.Context, versionID string, opts ...RetentionMessagesOption) (*json.RawMessage, error) {
	return getRetentionMessageRead[json.RawMessage](c, ctx, "/v1/retentionMessageVersions/%s/placement", versionID, opts)
}

// GetRetentionMessageVersionPlacementRelationship retrieves the placement linkage of a version.
func (c *Client) GetRetentionMessageVersionPlacementRelationship(ctx context.Context, versionID string, opts ...RetentionMessagesOption) (*RetentionMessageLinkageResponse, error) {
	return getRetentionMessageRead[RetentionMessageLinkageResponse](c, ctx, "/v1/retentionMessageVersions/%s/relationships/placement", versionID, opts)
}

// GetRetentionMessageLocalization retrieves a retention message localization.
func (c *Client) GetRetentionMessageLocalization(ctx context.Context, localizationID string, opts ...RetentionMessagesOption) (*RetentionMessageLocalizationResponse, error) {
	return getRetentionMessageRead[RetentionMessageLocalizationResponse](c, ctx, "/v1/retentionMessageLocalizations/%s", localizationID, opts)
}

// GetSubscriptionRetentionMessage retrieves the raw retention message assigned to a
// subscription; data is null when none is assigned.
func (c *Client) GetSubscriptionRetentionMessage(ctx context.Context, subscriptionID string, opts ...RetentionMessagesOption) (*json.RawMessage, error) {
	return getRetentionMessageRead[json.RawMessage](c, ctx, "/v1/subscriptions/%s/retentionMessage", subscriptionID, opts)
}

// GetSubscriptionRetentionMessageRelationship retrieves the retention message linkage of a subscription.
func (c *Client) GetSubscriptionRetentionMessageRelationship(ctx context.Context, subscriptionID string, opts ...RetentionMessagesOption) (*RetentionMessageLinkageResponse, error) {
	return getRetentionMessageRead[RetentionMessageLinkageResponse](c, ctx, "/v1/subscriptions/%s/relationships/retentionMessage", subscriptionID, opts)
}

// CreateRetentionMessage creates a retention message for an app.
func (c *Client) CreateRetentionMessage(ctx context.Context, appID, referenceName string) (*RetentionMessageResponse, error) {
	if err := requireRetentionMessageID("appID", appID); err != nil {
		return nil, err
	}
	return writeRetentionMessageResource[RetentionMessageResponse](c, ctx, http.MethodPost, "/v1/retentionMessages", retentionMessageWriteData{
		Type:          ResourceTypeRetentionMessages,
		Attributes:    map[string]string{"referenceName": referenceName},
		Relationships: map[string]Relationship{"app": retentionMessageRelationship(ResourceTypeApps, appID)},
	})
}

// DeleteRetentionMessage deletes a retention message.
func (c *Client) DeleteRetentionMessage(ctx context.Context, messageID string) error {
	if err := requireRetentionMessageID("messageID", messageID); err != nil {
		return err
	}
	_, err := c.do(ctx, http.MethodDelete, "/v1/retentionMessages/"+strings.TrimSpace(messageID), nil)
	return err
}

// CreateRetentionMessageVersion creates a version of a retention message,
// optionally linking an existing asset library placement.
func (c *Client) CreateRetentionMessageVersion(ctx context.Context, messageID, placementID string) (*RetentionMessageVersionResponse, error) {
	if err := requireRetentionMessageID("messageID", messageID); err != nil {
		return nil, err
	}
	relationships := map[string]Relationship{"retentionMessage": retentionMessageRelationship(ResourceTypeRetentionMessages, messageID)}
	if strings.TrimSpace(placementID) != "" {
		relationships["placement"] = retentionMessageRelationship(ResourceTypeAppAssetLibraryPlacements, placementID)
	}
	return writeRetentionMessageResource[RetentionMessageVersionResponse](c, ctx, http.MethodPost, "/v1/retentionMessageVersions", retentionMessageWriteData{
		Type:          ResourceTypeRetentionMessageVersions,
		Relationships: relationships,
	})
}

// CreateRetentionMessageLocalization creates a localization for a retention message version.
func (c *Client) CreateRetentionMessageLocalization(ctx context.Context, versionID string, attrs RetentionMessageLocalizationCreateAttributes) (*RetentionMessageLocalizationResponse, error) {
	if err := requireRetentionMessageID("versionID", versionID); err != nil {
		return nil, err
	}
	return writeRetentionMessageResource[RetentionMessageLocalizationResponse](c, ctx, http.MethodPost, "/v1/retentionMessageLocalizations", retentionMessageWriteData{
		Type:          ResourceTypeRetentionMessageLocalizations,
		Attributes:    attrs,
		Relationships: map[string]Relationship{"retentionMessageVersion": retentionMessageRelationship(ResourceTypeRetentionMessageVersions, versionID)},
	})
}

// UpdateRetentionMessageLocalization updates the header or body of a localization.
func (c *Client) UpdateRetentionMessageLocalization(ctx context.Context, localizationID string, attrs RetentionMessageLocalizationUpdateAttributes) (*RetentionMessageLocalizationResponse, error) {
	if err := requireRetentionMessageID("localizationID", localizationID); err != nil {
		return nil, err
	}
	localizationID = strings.TrimSpace(localizationID)
	return writeRetentionMessageResource[RetentionMessageLocalizationResponse](c, ctx, http.MethodPatch, "/v1/retentionMessageLocalizations/"+localizationID, retentionMessageWriteData{
		Type:       ResourceTypeRetentionMessageLocalizations,
		ID:         localizationID,
		Attributes: attrs,
	})
}

// DeleteRetentionMessageLocalization deletes a retention message localization.
func (c *Client) DeleteRetentionMessageLocalization(ctx context.Context, localizationID string) error {
	if err := requireRetentionMessageID("localizationID", localizationID); err != nil {
		return err
	}
	_, err := c.do(ctx, http.MethodDelete, "/v1/retentionMessageLocalizations/"+strings.TrimSpace(localizationID), nil)
	return err
}

// SetSubscriptionRetentionMessage assigns a retention message to a subscription,
// replacing any message already assigned.
func (c *Client) SetSubscriptionRetentionMessage(ctx context.Context, subscriptionID, messageID string) error {
	if err := requireRetentionMessageID("subscriptionID", subscriptionID); err != nil {
		return err
	}
	if err := requireRetentionMessageID("messageID", messageID); err != nil {
		return err
	}
	body, err := BuildRequestBody(retentionMessageRelationship(ResourceTypeRetentionMessages, messageID))
	if err != nil {
		return err
	}
	_, err = c.do(ctx, http.MethodPatch, "/v1/subscriptions/"+strings.TrimSpace(subscriptionID)+"/relationships/retentionMessage", body)
	return err
}
