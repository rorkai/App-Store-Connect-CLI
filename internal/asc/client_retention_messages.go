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

// RetentionMessageLocalizationUpdateAttributes describes a localization update
// request; a NullableString with a nil Value clears the text.
type RetentionMessageLocalizationUpdateAttributes struct {
	Header *NullableString `json:"header,omitempty"`
	Body   *NullableString `json:"body,omitempty"`
}

// RetentionMessageVersionCreateRelationships is the data.relationships object
// of the RetentionMessageVersionCreateRequest schema, with the placement
// declared first so the body lists its keys in alphabetical order.
type RetentionMessageVersionCreateRelationships struct {
	Placement        *Relationship `json:"placement,omitempty"`
	RetentionMessage *Relationship `json:"retentionMessage"`
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

func getRetentionMessageRead[T any](c *Client, ctx context.Context, pathFormat, id string, opts []RetentionMessagesOption) (*T, error) {
	q := &retentionMessagesQuery{values: url.Values{}}
	for _, opt := range opts {
		opt(q)
	}
	id = strings.TrimSpace(id)
	if q.nextURL == "" && id == "" {
		return nil, fmt.Errorf("resource ID is required")
	}
	return getJSON[T](ctx, c, readRequest{
		Path:        fmt.Sprintf(pathFormat, id),
		Query:       q.values.Encode(),
		Next:        q.nextURL,
		NextContext: "retention messages",
	})
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

// GetRetentionMessage retrieves a retention message.
func (c *Client) GetRetentionMessage(ctx context.Context, messageID string, opts ...RetentionMessagesOption) (*RetentionMessageResponse, error) {
	return getRetentionMessageRead[RetentionMessageResponse](c, ctx, "/v1/retentionMessages/%s", messageID, opts)
}

// GetRetentionMessageVersions lists versions of a retention message.
func (c *Client) GetRetentionMessageVersions(ctx context.Context, messageID string, opts ...RetentionMessagesOption) (*RetentionMessageVersionsResponse, error) {
	return getRetentionMessageRead[RetentionMessageVersionsResponse](c, ctx, "/v1/retentionMessages/%s/versions", messageID, opts)
}

// GetRetentionMessageSubscriptions lists subscriptions assigned to a retention message.
func (c *Client) GetRetentionMessageSubscriptions(ctx context.Context, messageID string, opts ...RetentionMessagesOption) (*SubscriptionsResponse, error) {
	return getRetentionMessageRead[SubscriptionsResponse](c, ctx, "/v1/retentionMessages/%s/subscriptions", messageID, opts)
}

// GetRetentionMessageVersion retrieves a retention message version.
func (c *Client) GetRetentionMessageVersion(ctx context.Context, versionID string, opts ...RetentionMessagesOption) (*RetentionMessageVersionResponse, error) {
	return getRetentionMessageRead[RetentionMessageVersionResponse](c, ctx, "/v1/retentionMessageVersions/%s", versionID, opts)
}

// GetRetentionMessageVersionLocalizations lists localizations of a retention message version.
func (c *Client) GetRetentionMessageVersionLocalizations(ctx context.Context, versionID string, opts ...RetentionMessagesOption) (*RetentionMessageLocalizationsResponse, error) {
	return getRetentionMessageRead[RetentionMessageLocalizationsResponse](c, ctx, "/v1/retentionMessageVersions/%s/localizations", versionID, opts)
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

// SetSubscriptionRetentionMessage assigns a retention message to a subscription,
// replacing any message already assigned.
func (c *Client) SetSubscriptionRetentionMessage(ctx context.Context, subscriptionID, messageID string) error {
	if err := requireRetentionMessageID("subscriptionID", subscriptionID); err != nil {
		return err
	}
	if err := requireRetentionMessageID("messageID", messageID); err != nil {
		return err
	}
	return sendNoContent(ctx, c, http.MethodPatch, "/v1/subscriptions/"+strings.TrimSpace(subscriptionID)+"/relationships/retentionMessage", retentionMessageRelationship(ResourceTypeRetentionMessages, messageID))
}
