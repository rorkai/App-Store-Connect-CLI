package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// FeedbackAttributes describes beta feedback screenshot submissions.
type FeedbackAttributes struct {
	CreatedDate             string                    `json:"createdDate"`
	Comment                 string                    `json:"comment"`
	Email                   string                    `json:"email"`
	DeviceModel             string                    `json:"deviceModel,omitempty"`
	OSVersion               string                    `json:"osVersion,omitempty"`
	Locale                  string                    `json:"locale,omitempty"`
	TimeZone                string                    `json:"timeZone,omitempty"`
	Architecture            string                    `json:"architecture,omitempty"`
	ConnectionType          DeviceConnectionType      `json:"connectionType,omitempty"`
	PairedAppleWatch        string                    `json:"pairedAppleWatch,omitempty"`
	AppUptimeInMilliseconds int64                     `json:"appUptimeInMilliseconds,omitempty"`
	DiskBytesAvailable      int64                     `json:"diskBytesAvailable,omitempty"`
	DiskBytesTotal          int64                     `json:"diskBytesTotal,omitempty"`
	BatteryPercentage       int                       `json:"batteryPercentage,omitempty"`
	ScreenWidthInPoints     int                       `json:"screenWidthInPoints,omitempty"`
	ScreenHeightInPoints    int                       `json:"screenHeightInPoints,omitempty"`
	AppPlatform             string                    `json:"appPlatform,omitempty"`
	DevicePlatform          string                    `json:"devicePlatform,omitempty"`
	DeviceFamily            DeviceFamily              `json:"deviceFamily,omitempty"`
	BuildBundleID           string                    `json:"buildBundleId,omitempty"`
	Screenshots             []FeedbackScreenshotImage `json:"screenshots,omitempty"`
}

// FeedbackScreenshotImage describes a screenshot attached to feedback.
type FeedbackScreenshotImage struct {
	URL            string `json:"url"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	ExpirationDate string `json:"expirationDate,omitempty"`
}

// CrashAttributes describes beta feedback crash submissions.
type CrashAttributes struct {
	CreatedDate             string               `json:"createdDate"`
	Comment                 string               `json:"comment"`
	Email                   string               `json:"email"`
	DeviceModel             string               `json:"deviceModel,omitempty"`
	OSVersion               string               `json:"osVersion,omitempty"`
	Locale                  string               `json:"locale,omitempty"`
	TimeZone                string               `json:"timeZone,omitempty"`
	Architecture            string               `json:"architecture,omitempty"`
	ConnectionType          DeviceConnectionType `json:"connectionType,omitempty"`
	PairedAppleWatch        string               `json:"pairedAppleWatch,omitempty"`
	AppUptimeInMilliseconds int64                `json:"appUptimeInMilliseconds,omitempty"`
	DiskBytesAvailable      int64                `json:"diskBytesAvailable,omitempty"`
	DiskBytesTotal          int64                `json:"diskBytesTotal,omitempty"`
	BatteryPercentage       int                  `json:"batteryPercentage,omitempty"`
	ScreenWidthInPoints     int                  `json:"screenWidthInPoints,omitempty"`
	ScreenHeightInPoints    int                  `json:"screenHeightInPoints,omitempty"`
	AppPlatform             string               `json:"appPlatform,omitempty"`
	DevicePlatform          string               `json:"devicePlatform,omitempty"`
	DeviceFamily            DeviceFamily         `json:"deviceFamily,omitempty"`
	BuildBundleID           string               `json:"buildBundleId,omitempty"`
	CrashLog                string               `json:"crashLog,omitempty"`
}

// ReviewAttributes describes App Store customer reviews.
type ReviewAttributes struct {
	Rating           int    `json:"rating"`
	Title            string `json:"title"`
	Body             string `json:"body"`
	ReviewerNickname string `json:"reviewerNickname"`
	CreatedDate      string `json:"createdDate"`
	Territory        string `json:"territory"`

	api apiAttributes
}

// FeedbackResponse is the response from beta feedback screenshots endpoint.
type FeedbackResponse = Response[FeedbackAttributes]

// CrashesResponse is the response from beta feedback crashes endpoint.
type CrashesResponse = Response[CrashAttributes]

// ReviewsResponse is the response from customer reviews endpoint.
type ReviewsResponse = Response[ReviewAttributes]

// CustomerReviewResponse is the response from customer review detail endpoint.
type CustomerReviewResponse = SingleResponse[ReviewAttributes]

// AppStoreVersionLocalizationsResponse is the response from app store version localizations endpoints.
type AppStoreVersionLocalizationsResponse = Response[AppStoreVersionLocalizationAttributes]

// AppInfoLocalizationsResponse is the response from app info localizations endpoints.
type AppInfoLocalizationsResponse = Response[AppInfoLocalizationAttributes]

// AppInfosResponse is the response from app info endpoints.
type AppInfosResponse = Response[AppInfoAttributes]

// BetaTesterInvitationResponse is the response from beta tester invitations.
type BetaTesterInvitationResponse = SingleResponse[struct{}]

// AppStoreVersionLocalizationAttributes describes app store version localization metadata.
type AppStoreVersionLocalizationAttributes struct {
	Locale          string `json:"locale,omitempty"`
	Description     string `json:"description,omitempty"`
	Keywords        string `json:"keywords,omitempty"`
	MarketingURL    string `json:"marketingUrl,omitempty"`
	PromotionalText string `json:"promotionalText,omitempty"`
	SupportURL      string `json:"supportUrl,omitempty"`
	WhatsNew        string `json:"whatsNew,omitempty"`

	api apiAttributes
}

// BetaAppLocalizationAttributes describes TestFlight app localization metadata.
type BetaAppLocalizationAttributes struct {
	FeedbackEmail     string `json:"feedbackEmail,omitempty"`
	MarketingURL      string `json:"marketingUrl,omitempty"`
	PrivacyPolicyURL  string `json:"privacyPolicyUrl,omitempty"`
	TvOsPrivacyPolicy string `json:"tvOsPrivacyPolicy,omitempty"`
	Description       string `json:"description,omitempty"`
	Locale            string `json:"locale,omitempty"`

	api apiAttributes
}

// BetaBuildLocalizationAttributes describes TestFlight build localization notes.
type BetaBuildLocalizationAttributes struct {
	Locale   string `json:"locale,omitempty"`
	WhatsNew string `json:"whatsNew,omitempty"`

	api apiAttributes
}

// AppInfoLocalizationAttributes describes app info localization metadata.
type AppInfoLocalizationAttributes struct {
	Locale            string `json:"locale,omitempty"`
	Name              string `json:"name,omitempty"`
	Subtitle          string `json:"subtitle,omitempty"`
	PrivacyPolicyURL  string `json:"privacyPolicyUrl,omitempty"`
	PrivacyChoicesURL string `json:"privacyChoicesUrl,omitempty"`
	PrivacyPolicyText string `json:"privacyPolicyText,omitempty"`

	api apiAttributes
}

type localizationFieldsUpdateData struct {
	Type       ResourceType              `json:"type"`
	ID         string                    `json:"id"`
	Attributes map[string]NullableString `json:"attributes"`
}

type localizationFieldsUpdateRequest struct {
	Data localizationFieldsUpdateData `json:"data"`
}

// nullableLocalizationFields converts set-only field values into nullable
// attributes so string callers keep their existing wire format.
func nullableLocalizationFields(fields map[string]string) map[string]NullableString {
	if fields == nil {
		return nil
	}
	attributes := make(map[string]NullableString, len(fields))
	for field, value := range fields {
		attributes[field] = NullableString{Value: &value}
	}
	return attributes
}

// AppInfoAttributes describes app info resources.
type AppInfoAttributes map[string]any

// BetaGroupAttributes describes a beta group resource.
type BetaGroupAttributes struct {
	Name                                 string `json:"name"`
	CreatedDate                          string `json:"createdDate,omitempty"`
	IsInternalGroup                      bool   `json:"isInternalGroup,omitempty"`
	HasAccessToAllBuilds                 bool   `json:"hasAccessToAllBuilds,omitempty"`
	PublicLinkEnabled                    bool   `json:"publicLinkEnabled,omitempty"`
	PublicLinkID                         string `json:"publicLinkId,omitempty"`
	PublicLinkLimitEnabled               bool   `json:"publicLinkLimitEnabled,omitempty"`
	PublicLinkLimit                      int    `json:"publicLinkLimit,omitempty"`
	PublicLink                           string `json:"publicLink,omitempty"`
	FeedbackEnabled                      bool   `json:"feedbackEnabled,omitempty"`
	IOSBuildsAvailableForAppleSiliconMac *bool  `json:"iosBuildsAvailableForAppleSiliconMac,omitempty"`
	IOSBuildsAvailableForAppleVision     *bool  `json:"iosBuildsAvailableForAppleVision,omitempty"`

	attributesDecoded                         bool
	attributesNull                            bool
	nameJSON                                  json.RawMessage
	createdDateJSON                           json.RawMessage
	isInternalGroupJSON                       json.RawMessage
	hasAccessToAllBuildsJSON                  json.RawMessage
	publicLinkEnabledJSON                     json.RawMessage
	publicLinkIDJSON                          json.RawMessage
	publicLinkLimitEnabledJSON                json.RawMessage
	publicLinkLimitJSON                       json.RawMessage
	publicLinkJSON                            json.RawMessage
	feedbackEnabledJSON                       json.RawMessage
	iosBuildsAvailableForAppleSiliconMacJSON  json.RawMessage
	iosBuildsAvailableForAppleVisionJSON      json.RawMessage
	nameValue                                 string
	createdDateValue                          string
	isInternalGroupValue                      bool
	hasAccessToAllBuildsValue                 bool
	publicLinkEnabledValue                    bool
	publicLinkIDValue                         string
	publicLinkLimitEnabledValue               bool
	publicLinkLimitValue                      int
	publicLinkValue                           string
	feedbackEnabledValue                      bool
	iosBuildsAvailableForAppleSiliconMacValue *bool
	iosBuildsAvailableForAppleVisionValue     *bool
}

// UnmarshalJSON retains sparse field presence without changing the public,
// value-shaped BetaGroupAttributes API.
func (a *BetaGroupAttributes) UnmarshalJSON(data []byte) error {
	type alias BetaGroupAttributes
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*a = BetaGroupAttributes(decoded)
	a.attributesDecoded = true
	a.attributesNull = bytes.Equal(bytes.TrimSpace(data), []byte("null"))
	a.nameJSON = fields["name"]
	a.createdDateJSON = fields["createdDate"]
	a.isInternalGroupJSON = fields["isInternalGroup"]
	a.hasAccessToAllBuildsJSON = fields["hasAccessToAllBuilds"]
	a.publicLinkEnabledJSON = fields["publicLinkEnabled"]
	a.publicLinkIDJSON = fields["publicLinkId"]
	a.publicLinkLimitEnabledJSON = fields["publicLinkLimitEnabled"]
	a.publicLinkLimitJSON = fields["publicLinkLimit"]
	a.publicLinkJSON = fields["publicLink"]
	a.feedbackEnabledJSON = fields["feedbackEnabled"]
	a.iosBuildsAvailableForAppleSiliconMacJSON = fields["iosBuildsAvailableForAppleSiliconMac"]
	a.iosBuildsAvailableForAppleVisionJSON = fields["iosBuildsAvailableForAppleVision"]
	a.nameValue = a.Name
	a.createdDateValue = a.CreatedDate
	a.isInternalGroupValue = a.IsInternalGroup
	a.hasAccessToAllBuildsValue = a.HasAccessToAllBuilds
	a.publicLinkEnabledValue = a.PublicLinkEnabled
	a.publicLinkIDValue = a.PublicLinkID
	a.publicLinkLimitEnabledValue = a.PublicLinkLimitEnabled
	a.publicLinkLimitValue = a.PublicLinkLimit
	a.publicLinkValue = a.PublicLink
	a.feedbackEnabledValue = a.FeedbackEnabled
	a.iosBuildsAvailableForAppleSiliconMacValue = cloneBool(a.IOSBuildsAvailableForAppleSiliconMac)
	a.iosBuildsAvailableForAppleVisionValue = cloneBool(a.IOSBuildsAvailableForAppleVision)
	return nil
}

// MarshalJSON preserves fields Apple supplied in sparse responses while still
// serializing values changed by callers after decoding.
func (a BetaGroupAttributes) MarshalJSON() ([]byte, error) {
	if !a.attributesDecoded {
		type alias BetaGroupAttributes
		return json.Marshal(alias(a))
	}
	if a.attributesNull {
		return []byte("null"), nil
	}

	type attributesJSON struct {
		Name                                 json.RawMessage `json:"name,omitempty"`
		CreatedDate                          json.RawMessage `json:"createdDate,omitempty"`
		IsInternalGroup                      json.RawMessage `json:"isInternalGroup,omitempty"`
		HasAccessToAllBuilds                 json.RawMessage `json:"hasAccessToAllBuilds,omitempty"`
		PublicLinkEnabled                    json.RawMessage `json:"publicLinkEnabled,omitempty"`
		PublicLinkID                         json.RawMessage `json:"publicLinkId,omitempty"`
		PublicLinkLimitEnabled               json.RawMessage `json:"publicLinkLimitEnabled,omitempty"`
		PublicLinkLimit                      json.RawMessage `json:"publicLinkLimit,omitempty"`
		PublicLink                           json.RawMessage `json:"publicLink,omitempty"`
		FeedbackEnabled                      json.RawMessage `json:"feedbackEnabled,omitempty"`
		IOSBuildsAvailableForAppleSiliconMac json.RawMessage `json:"iosBuildsAvailableForAppleSiliconMac,omitempty"`
		IOSBuildsAvailableForAppleVision     json.RawMessage `json:"iosBuildsAvailableForAppleVision,omitempty"`
	}
	fields := attributesJSON{}
	var err error
	if len(a.nameJSON) > 0 || a.Name != "" {
		fields.Name, err = sparseAttributeJSON(a.nameJSON, a.nameValue, a.Name)
		if err != nil {
			return nil, err
		}
	}
	if len(a.createdDateJSON) > 0 || a.CreatedDate != "" {
		fields.CreatedDate, err = sparseAttributeJSON(a.createdDateJSON, a.createdDateValue, a.CreatedDate)
		if err != nil {
			return nil, err
		}
	}
	if len(a.isInternalGroupJSON) > 0 || a.IsInternalGroup {
		fields.IsInternalGroup, err = sparseAttributeJSON(a.isInternalGroupJSON, a.isInternalGroupValue, a.IsInternalGroup)
		if err != nil {
			return nil, err
		}
	}
	if len(a.hasAccessToAllBuildsJSON) > 0 || a.HasAccessToAllBuilds {
		fields.HasAccessToAllBuilds, err = sparseAttributeJSON(a.hasAccessToAllBuildsJSON, a.hasAccessToAllBuildsValue, a.HasAccessToAllBuilds)
		if err != nil {
			return nil, err
		}
	}
	if len(a.publicLinkEnabledJSON) > 0 || a.PublicLinkEnabled {
		fields.PublicLinkEnabled, err = sparseAttributeJSON(a.publicLinkEnabledJSON, a.publicLinkEnabledValue, a.PublicLinkEnabled)
		if err != nil {
			return nil, err
		}
	}
	if len(a.publicLinkIDJSON) > 0 || a.PublicLinkID != "" {
		fields.PublicLinkID, err = sparseAttributeJSON(a.publicLinkIDJSON, a.publicLinkIDValue, a.PublicLinkID)
		if err != nil {
			return nil, err
		}
	}
	if len(a.publicLinkLimitEnabledJSON) > 0 || a.PublicLinkLimitEnabled {
		fields.PublicLinkLimitEnabled, err = sparseAttributeJSON(a.publicLinkLimitEnabledJSON, a.publicLinkLimitEnabledValue, a.PublicLinkLimitEnabled)
		if err != nil {
			return nil, err
		}
	}
	if len(a.publicLinkLimitJSON) > 0 || a.PublicLinkLimit != 0 {
		fields.PublicLinkLimit, err = sparseAttributeJSON(a.publicLinkLimitJSON, a.publicLinkLimitValue, a.PublicLinkLimit)
		if err != nil {
			return nil, err
		}
	}
	if len(a.publicLinkJSON) > 0 || a.PublicLink != "" {
		fields.PublicLink, err = sparseAttributeJSON(a.publicLinkJSON, a.publicLinkValue, a.PublicLink)
		if err != nil {
			return nil, err
		}
	}
	if len(a.feedbackEnabledJSON) > 0 || a.FeedbackEnabled {
		fields.FeedbackEnabled, err = sparseAttributeJSON(a.feedbackEnabledJSON, a.feedbackEnabledValue, a.FeedbackEnabled)
		if err != nil {
			return nil, err
		}
	}
	if len(a.iosBuildsAvailableForAppleSiliconMacJSON) > 0 || a.IOSBuildsAvailableForAppleSiliconMac != nil {
		fields.IOSBuildsAvailableForAppleSiliconMac, err = sparseAttributeJSON(a.iosBuildsAvailableForAppleSiliconMacJSON, a.iosBuildsAvailableForAppleSiliconMacValue, a.IOSBuildsAvailableForAppleSiliconMac)
		if err != nil {
			return nil, err
		}
	}
	if len(a.iosBuildsAvailableForAppleVisionJSON) > 0 || a.IOSBuildsAvailableForAppleVision != nil {
		fields.IOSBuildsAvailableForAppleVision, err = sparseAttributeJSON(a.iosBuildsAvailableForAppleVisionJSON, a.iosBuildsAvailableForAppleVisionValue, a.IOSBuildsAvailableForAppleVision)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(fields)
}

// BetaTesterAttributes describes a beta tester resource.
type BetaTesterAttributes struct {
	FirstName  string                `json:"firstName,omitempty"`
	LastName   string                `json:"lastName,omitempty"`
	Email      string                `json:"email,omitempty"`
	InviteType BetaInviteType        `json:"inviteType,omitempty"`
	State      BetaTesterState       `json:"state,omitempty"`
	AppDevices []BetaTesterAppDevice `json:"-"`

	api apiAttributes
}

// UnmarshalJSON keeps absent appDevices nil while retaining explicit arrays.
func (a *BetaTesterAttributes) UnmarshalJSON(data []byte) error {
	type betaTesterAttributesAlias BetaTesterAttributes
	*a = BetaTesterAttributes{}
	aux := struct {
		*betaTesterAttributesAlias
		AppDevices []BetaTesterAppDevice `json:"appDevices"`
	}{
		betaTesterAttributesAlias: (*betaTesterAttributesAlias)(a),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	a.AppDevices = aux.AppDevices
	baseline, err := a.modeledJSON()
	if err != nil {
		return err
	}
	a.api, err = captureAPIAttributes(data, baseline, reflect.TypeFor[BetaTesterAttributes](), "appDevices")
	return err
}

// MarshalJSON preserves absent and explicit empty appDevices values.
func (a BetaTesterAttributes) MarshalJSON() ([]byte, error) {
	encoded, err := a.modeledJSON()
	if err != nil {
		return nil, err
	}
	return a.api.apply(encoded)
}

func (a BetaTesterAttributes) modeledJSON() ([]byte, error) {
	type betaTesterAttributesAlias BetaTesterAttributes
	var appDevices *[]BetaTesterAppDevice
	if a.AppDevices != nil {
		appDevices = &a.AppDevices
	}
	return json.Marshal(struct {
		betaTesterAttributesAlias
		AppDevices *[]BetaTesterAppDevice `json:"appDevices,omitempty"`
	}{
		betaTesterAttributesAlias: betaTesterAttributesAlias(a),
		AppDevices:                appDevices,
	})
}

// BetaTesterAppDevice describes a device on which a tester installed the app.
type BetaTesterAppDevice struct {
	Model           string `json:"model,omitempty"`
	Platform        string `json:"platform,omitempty"`
	OSVersion       string `json:"osVersion,omitempty"`
	AppBuildVersion string `json:"appBuildVersion,omitempty"`
}

// BetaInviteType represents the invitation type for a beta tester.
type BetaInviteType string

const (
	BetaInviteTypeEmail      BetaInviteType = "EMAIL"
	BetaInviteTypePublicLink BetaInviteType = "PUBLIC_LINK"
)

// BetaTesterState represents the invitation state for a beta tester.
type BetaTesterState string

const (
	BetaTesterStateNotInvited BetaTesterState = "NOT_INVITED"
	BetaTesterStateInvited    BetaTesterState = "INVITED"
	BetaTesterStateAccepted   BetaTesterState = "ACCEPTED"
	BetaTesterStateInstalled  BetaTesterState = "INSTALLED"
	BetaTesterStateRevoked    BetaTesterState = "REVOKED"
)

// AppStoreVersionLocalizationCreateData is the data portion of a version localization create request.
type AppStoreVersionLocalizationCreateData struct {
	Type          ResourceType                              `json:"type"`
	Attributes    AppStoreVersionLocalizationAttributes     `json:"attributes"`
	Relationships *AppStoreVersionLocalizationRelationships `json:"relationships"`
}

// AppStoreVersionLocalizationUpdateData is the data portion of a version localization update request.
type AppStoreVersionLocalizationUpdateData struct {
	Type       ResourceType                          `json:"type"`
	ID         string                                `json:"id"`
	Attributes AppStoreVersionLocalizationAttributes `json:"attributes"`
}

// AppStoreVersionLocalizationRelationships describes relationships for version localizations.
type AppStoreVersionLocalizationRelationships struct {
	AppStoreVersion *Relationship `json:"appStoreVersion"`
}

// BetaAppLocalizationCreateData is the data portion of a beta app localization create request.
type BetaAppLocalizationCreateData struct {
	Type          ResourceType                      `json:"type"`
	Attributes    BetaAppLocalizationAttributes     `json:"attributes"`
	Relationships *BetaAppLocalizationRelationships `json:"relationships"`
}

// BetaAppLocalizationUpdateAttributes describes attributes for updating beta app localizations.
type BetaAppLocalizationUpdateAttributes struct {
	FeedbackEmail     *string `json:"feedbackEmail,omitempty"`
	MarketingURL      *string `json:"marketingUrl,omitempty"`
	PrivacyPolicyURL  *string `json:"privacyPolicyUrl,omitempty"`
	TvOsPrivacyPolicy *string `json:"tvOsPrivacyPolicy,omitempty"`
	Description       *string `json:"description,omitempty"`
}

// BetaBuildLocalizationCreateData is the data portion of a beta build localization create request.
type BetaBuildLocalizationCreateData struct {
	Type          ResourceType                        `json:"type"`
	Attributes    BetaBuildLocalizationAttributes     `json:"attributes"`
	Relationships *BetaBuildLocalizationRelationships `json:"relationships"`
}

// BetaBuildLocalizationUpdateData is the data portion of a beta build localization update request.
type BetaBuildLocalizationUpdateData struct {
	Type       ResourceType                    `json:"type"`
	ID         string                          `json:"id"`
	Attributes BetaBuildLocalizationAttributes `json:"attributes"`
}

// AppInfoLocalizationCreateData is the data portion of an app info localization create request.
type AppInfoLocalizationCreateData struct {
	Type          ResourceType                      `json:"type"`
	Attributes    AppInfoLocalizationAttributes     `json:"attributes"`
	Relationships *AppInfoLocalizationRelationships `json:"relationships"`
}

// AppInfoLocalizationUpdateData is the data portion of an app info localization update request.
type AppInfoLocalizationUpdateData struct {
	Type       ResourceType                  `json:"type"`
	ID         string                        `json:"id"`
	Attributes AppInfoLocalizationAttributes `json:"attributes"`
}

// BetaGroupUpdateAttributes describes attributes for updating a beta group.
type BetaGroupUpdateAttributes struct {
	Name                                 string `json:"name,omitempty"`
	PublicLinkEnabled                    *bool  `json:"publicLinkEnabled,omitempty"`
	PublicLinkLimitEnabled               *bool  `json:"publicLinkLimitEnabled,omitempty"`
	PublicLinkLimit                      int    `json:"publicLinkLimit,omitempty"`
	FeedbackEnabled                      *bool  `json:"feedbackEnabled,omitempty"`
	IOSBuildsAvailableForAppleSiliconMac *bool  `json:"iosBuildsAvailableForAppleSiliconMac,omitempty"`
	IOSBuildsAvailableForAppleVision     *bool  `json:"iosBuildsAvailableForAppleVision,omitempty"`
}

// BetaTesterCreateAttributes describes attributes for creating a beta tester.
type BetaTesterCreateAttributes struct {
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Email     string `json:"email"`
}

// BetaTesterInvitationCreateRelationships describes relationships for invitations.
type BetaTesterInvitationCreateRelationships struct {
	App        *Relationship `json:"app"`
	BetaTester *Relationship `json:"betaTester,omitempty"`
}

// GetReviews retrieves App Store reviews
func (c *Client) GetReviews(ctx context.Context, appID string, opts ...ReviewOption) (*ReviewsResponse, error) {
	query := applyOptions(opts)
	return getJSON[ReviewsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/apps/%s/customerReviews", appID),
		Query:       buildReviewQuery(opts),
		Next:        query.nextURL,
		NextContext: "reviews",
	})
}

// GetCustomerReview retrieves a customer review by ID. Only include and fields
// options apply; Apple accepts no filters on this endpoint.
func (c *Client) GetCustomerReview(ctx context.Context, reviewID string, opts ...ReviewOption) (*CustomerReviewResponse, error) {
	reviewID = strings.TrimSpace(reviewID)
	if reviewID == "" {
		return nil, fmt.Errorf("reviewID is required")
	}
	return getJSON[CustomerReviewResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/customerReviews/%s", reviewID),
		Query: buildCustomerReviewQuery(opts),
	})
}

// CreateBetaGroupWithAttributes creates a beta group for an app with explicit
// attributes, attaching any builds and beta testers in the same request.
// Note: some attributes (e.g., isInternalGroup) are create-only in the ASC API.
func (c *Client) CreateBetaGroupWithAttributes(ctx context.Context, appID string, attrs BetaGroupCreateAttributes, buildIDs, testerIDs []string) (*BetaGroupResponse, error) {
	payload := BetaGroupCreateRequest{
		Data: BetaGroupCreateData{
			Type:       ResourceTypeBetaGroups,
			Attributes: attrs,
			Relationships: &BetaGroupRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
				Builds:      buildRelationshipList(ResourceTypeBuilds, buildIDs),
				BetaTesters: buildRelationshipList(ResourceTypeBetaTesters, testerIDs),
			},
		},
	}
	return sendJSON[BetaGroupResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/betaGroups", Payload: payload})
}

// UpdateBetaGroup updates a beta group by ID.
func (c *Client) UpdateBetaGroup(ctx context.Context, groupID string, req BetaGroupUpdateRequest) (*BetaGroupResponse, error) {
	return sendJSON[BetaGroupResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/betaGroups/%s", groupID), Payload: req})
}

// GetBetaTesters retrieves beta testers for an app.
func (c *Client) GetBetaTesters(ctx context.Context, appID string, opts ...BetaTestersOption) (*BetaTestersResponse, error) {
	query := applyOptions(opts)
	path := "/v1/betaTesters"
	if query.nextURL == "" {
		queryString, err := buildBetaTestersQuery(appID, query)
		if err != nil {
			return nil, fmt.Errorf("betaTesters: %w", err)
		}
		if queryString != "" {
			path += "?" + queryString
		}
	}
	return getJSON[BetaTestersResponse](ctx, c, readRequest{Path: path, Next: query.nextURL, NextContext: "betaTesters"})
}

// CreateBetaTester creates a beta tester, attaching any beta groups and builds
// in the same request.
func (c *Client) CreateBetaTester(ctx context.Context, email, firstName, lastName string, groupIDs, buildIDs []string) (*BetaTesterResponse, error) {
	groups := buildRelationshipList(ResourceTypeBetaGroups, normalizeUniqueList(groupIDs))
	builds := buildRelationshipList(ResourceTypeBuilds, buildIDs)
	var relationships *BetaTesterCreateRelationships
	if groups != nil || builds != nil {
		relationships = &BetaTesterCreateRelationships{BetaGroups: groups, Builds: builds}
	}

	payload := BetaTesterCreateRequest{
		Data: BetaTesterCreateData{
			Type: ResourceTypeBetaTesters,
			Attributes: BetaTesterCreateAttributes{
				FirstName: strings.TrimSpace(firstName),
				LastName:  strings.TrimSpace(lastName),
				Email:     strings.TrimSpace(email),
			},
			Relationships: relationships,
		},
	}
	return sendJSON[BetaTesterResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/betaTesters", Payload: payload})
}

// CreateAppStoreVersionLocalization creates a localization for an app store version.
func (c *Client) CreateAppStoreVersionLocalization(ctx context.Context, versionID string, attributes AppStoreVersionLocalizationAttributes) (*AppStoreVersionLocalizationResponse, error) {
	attributes.api = apiAttributes{}
	payload := AppStoreVersionLocalizationCreateRequest{
		Data: AppStoreVersionLocalizationCreateData{
			Type:       ResourceTypeAppStoreVersionLocalizations,
			Attributes: attributes,
			Relationships: &AppStoreVersionLocalizationRelationships{
				AppStoreVersion: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersions,
						ID:   versionID,
					},
				},
			},
		},
	}
	return sendJSON[AppStoreVersionLocalizationResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appStoreVersionLocalizations", Payload: payload})
}

// UpdateAppStoreVersionLocalization updates a localization for an app store version.
func (c *Client) UpdateAppStoreVersionLocalization(ctx context.Context, localizationID string, attributes AppStoreVersionLocalizationAttributes) (*AppStoreVersionLocalizationResponse, error) {
	attributes.api = apiAttributes{}
	// ASC rejects locale in PATCH updates; it's immutable and should only be set on create.
	attributes.Locale = ""
	payload := AppStoreVersionLocalizationUpdateRequest{
		Data: AppStoreVersionLocalizationUpdateData{
			Type:       ResourceTypeAppStoreVersionLocalizations,
			ID:         localizationID,
			Attributes: attributes,
		},
	}
	return sendJSON[AppStoreVersionLocalizationResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/appStoreVersionLocalizations/%s", localizationID), Payload: payload})
}

// UpdateAppStoreVersionLocalizationFields updates exactly the supplied fields,
// preserving explicit empty strings while leaving omitted fields unchanged.
func (c *Client) UpdateAppStoreVersionLocalizationFields(ctx context.Context, localizationID string, fields map[string]string) (*AppStoreVersionLocalizationResponse, error) {
	return c.UpdateAppStoreVersionLocalizationNullableFields(ctx, localizationID, nullableLocalizationFields(fields))
}

// UpdateAppStoreVersionLocalizationNullableFields updates exactly the supplied
// fields, sending JSON null for fields whose value is nil so callers can clear
// nullable attributes while leaving omitted fields unchanged.
func (c *Client) UpdateAppStoreVersionLocalizationNullableFields(ctx context.Context, localizationID string, fields map[string]NullableString) (*AppStoreVersionLocalizationResponse, error) {
	payload := localizationFieldsUpdateRequest{Data: localizationFieldsUpdateData{
		Type:       ResourceTypeAppStoreVersionLocalizations,
		ID:         localizationID,
		Attributes: fields,
	}}
	return sendJSON[AppStoreVersionLocalizationResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/appStoreVersionLocalizations/%s", localizationID), Payload: payload})
}

// CreateBetaBuildLocalization creates a beta build localization for a build.
func (c *Client) CreateBetaBuildLocalization(ctx context.Context, buildID string, attributes BetaBuildLocalizationAttributes) (*BetaBuildLocalizationResponse, error) {
	attributes.api = apiAttributes{}
	payload := BetaBuildLocalizationCreateRequest{
		Data: BetaBuildLocalizationCreateData{
			Type:       ResourceTypeBetaBuildLocalizations,
			Attributes: attributes,
			Relationships: &BetaBuildLocalizationRelationships{
				Build: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeBuilds,
						ID:   buildID,
					},
				},
			},
		},
	}
	return sendJSON[BetaBuildLocalizationResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/betaBuildLocalizations", Payload: payload})
}

// UpdateBetaBuildLocalization updates a beta build localization by ID.
func (c *Client) UpdateBetaBuildLocalization(ctx context.Context, localizationID string, attributes BetaBuildLocalizationAttributes) (*BetaBuildLocalizationResponse, error) {
	attributes.api = apiAttributes{}
	payload := BetaBuildLocalizationUpdateRequest{
		Data: BetaBuildLocalizationUpdateData{
			Type:       ResourceTypeBetaBuildLocalizations,
			ID:         localizationID,
			Attributes: attributes,
		},
	}
	return sendJSON[BetaBuildLocalizationResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/betaBuildLocalizations/%s", localizationID), Payload: payload})
}

// CreateAppInfoLocalization creates a localization for an app info resource.
func (c *Client) CreateAppInfoLocalization(ctx context.Context, appInfoID string, attributes AppInfoLocalizationAttributes) (*AppInfoLocalizationResponse, error) {
	attributes.api = apiAttributes{}
	payload := AppInfoLocalizationCreateRequest{
		Data: AppInfoLocalizationCreateData{
			Type:       ResourceTypeAppInfoLocalizations,
			Attributes: attributes,
			Relationships: &AppInfoLocalizationRelationships{
				AppInfo: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppInfos,
						ID:   appInfoID,
					},
				},
			},
		},
	}
	return sendJSON[AppInfoLocalizationResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appInfoLocalizations", Payload: payload})
}

// UpdateAppInfoLocalization updates a localization for an app info resource.
func (c *Client) UpdateAppInfoLocalization(ctx context.Context, localizationID string, attributes AppInfoLocalizationAttributes) (*AppInfoLocalizationResponse, error) {
	attributes.api = apiAttributes{}
	// ASC rejects locale in PATCH updates; it's immutable and should only be set on create.
	attributes.Locale = ""
	payload := AppInfoLocalizationUpdateRequest{
		Data: AppInfoLocalizationUpdateData{
			Type:       ResourceTypeAppInfoLocalizations,
			ID:         localizationID,
			Attributes: attributes,
		},
	}
	return sendJSON[AppInfoLocalizationResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/appInfoLocalizations/%s", localizationID), Payload: payload})
}

// UpdateAppInfoLocalizationFields updates exactly the supplied fields,
// preserving explicit empty strings while leaving omitted fields unchanged.
func (c *Client) UpdateAppInfoLocalizationFields(ctx context.Context, localizationID string, fields map[string]string) (*AppInfoLocalizationResponse, error) {
	return c.UpdateAppInfoLocalizationNullableFields(ctx, localizationID, nullableLocalizationFields(fields))
}

// UpdateAppInfoLocalizationNullableFields updates exactly the supplied fields,
// sending JSON null for fields whose value is nil so callers can clear nullable
// attributes while leaving omitted fields unchanged.
func (c *Client) UpdateAppInfoLocalizationNullableFields(ctx context.Context, localizationID string, fields map[string]NullableString) (*AppInfoLocalizationResponse, error) {
	payload := localizationFieldsUpdateRequest{Data: localizationFieldsUpdateData{
		Type:       ResourceTypeAppInfoLocalizations,
		ID:         localizationID,
		Attributes: fields,
	}}
	return sendJSON[AppInfoLocalizationResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/appInfoLocalizations/%s", localizationID), Payload: payload})
}
