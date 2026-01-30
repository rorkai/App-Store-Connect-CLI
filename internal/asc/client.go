package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// FeedbackAttributes describes beta feedback screenshot submissions.
type FeedbackAttributes struct {
	CreatedDate    string                    `json:"createdDate"`
	Comment        string                    `json:"comment"`
	Email          string                    `json:"email"`
	DeviceModel    string                    `json:"deviceModel,omitempty"`
	OSVersion      string                    `json:"osVersion,omitempty"`
	AppPlatform    string                    `json:"appPlatform,omitempty"`
	DevicePlatform string                    `json:"devicePlatform,omitempty"`
	Screenshots    []FeedbackScreenshotImage `json:"screenshots,omitempty"`
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
	CreatedDate    string `json:"createdDate"`
	Comment        string `json:"comment"`
	Email          string `json:"email"`
	DeviceModel    string `json:"deviceModel,omitempty"`
	OSVersion      string `json:"osVersion,omitempty"`
	AppPlatform    string `json:"appPlatform,omitempty"`
	DevicePlatform string `json:"devicePlatform,omitempty"`
	CrashLog       string `json:"crashLog,omitempty"`
}

// ReviewAttributes describes App Store customer reviews.
type ReviewAttributes struct {
	Rating           int    `json:"rating"`
	Title            string `json:"title"`
	Body             string `json:"body"`
	ReviewerNickname string `json:"reviewerNickname"`
	CreatedDate      string `json:"createdDate"`
	Territory        string `json:"territory"`
}

// FeedbackResponse is the response from beta feedback screenshots endpoint.
type FeedbackResponse = Response[FeedbackAttributes]

// CrashesResponse is the response from beta feedback crashes endpoint.
type CrashesResponse = Response[CrashAttributes]

// ReviewsResponse is the response from customer reviews endpoint.
type ReviewsResponse = Response[ReviewAttributes]

// AppStoreVersionLocalizationsResponse is the response from app store version localizations endpoints.
type AppStoreVersionLocalizationsResponse = Response[AppStoreVersionLocalizationAttributes]

// AppStoreVersionLocalizationResponse is the response from app store version localization detail/creates.
type AppStoreVersionLocalizationResponse = SingleResponse[AppStoreVersionLocalizationAttributes]

// BetaAppLocalizationsResponse is the response from beta app localization endpoints.
type BetaAppLocalizationsResponse = Response[BetaAppLocalizationAttributes]

// BetaAppLocalizationResponse is the response from beta app localization detail/creates.
type BetaAppLocalizationResponse = SingleResponse[BetaAppLocalizationAttributes]

// BetaBuildLocalizationsResponse is the response from beta build localization endpoints.
type BetaBuildLocalizationsResponse = Response[BetaBuildLocalizationAttributes]

// BetaBuildLocalizationResponse is the response from beta build localization detail/creates.
type BetaBuildLocalizationResponse = SingleResponse[BetaBuildLocalizationAttributes]

// AppInfoLocalizationsResponse is the response from app info localizations endpoints.
type AppInfoLocalizationsResponse = Response[AppInfoLocalizationAttributes]

// AppInfoLocalizationResponse is the response from app info localization detail/creates.
type AppInfoLocalizationResponse = SingleResponse[AppInfoLocalizationAttributes]

// AppInfosResponse is the response from app info endpoints.
type AppInfosResponse = Response[AppInfoAttributes]

// AppInfoResponse is the response from app info detail endpoints.
type AppInfoResponse = SingleResponse[AppInfoAttributes]

// BetaGroupsResponse is the response from beta groups endpoints.
type BetaGroupsResponse = Response[BetaGroupAttributes]

// BetaGroupResponse is the response from beta group detail/creates.
type BetaGroupResponse = SingleResponse[BetaGroupAttributes]

// BetaTestersResponse is the response from beta testers endpoints.
type BetaTestersResponse = Response[BetaTesterAttributes]

// BetaTesterResponse is the response from beta tester detail/creates.
type BetaTesterResponse = SingleResponse[BetaTesterAttributes]

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
}

// BetaAppLocalizationAttributes describes TestFlight app localization metadata.
type BetaAppLocalizationAttributes struct {
	FeedbackEmail     string `json:"feedbackEmail,omitempty"`
	MarketingURL      string `json:"marketingUrl,omitempty"`
	PrivacyPolicyURL  string `json:"privacyPolicyUrl,omitempty"`
	TvOsPrivacyPolicy string `json:"tvOsPrivacyPolicy,omitempty"`
	Description       string `json:"description,omitempty"`
	Locale            string `json:"locale,omitempty"`
}

// BetaBuildLocalizationAttributes describes TestFlight build localization notes.
type BetaBuildLocalizationAttributes struct {
	Locale   string `json:"locale,omitempty"`
	WhatsNew string `json:"whatsNew,omitempty"`
}

// AppInfoLocalizationAttributes describes app info localization metadata.
type AppInfoLocalizationAttributes struct {
	Locale            string `json:"locale,omitempty"`
	Name              string `json:"name,omitempty"`
	Subtitle          string `json:"subtitle,omitempty"`
	PrivacyPolicyURL  string `json:"privacyPolicyUrl,omitempty"`
	PrivacyChoicesURL string `json:"privacyChoicesUrl,omitempty"`
	PrivacyPolicyText string `json:"privacyPolicyText,omitempty"`
}

// AppInfoAttributes describes app info resources.
type AppInfoAttributes map[string]any

// BetaGroupAttributes describes a beta group resource.
type BetaGroupAttributes struct {
	Name                   string `json:"name"`
	CreatedDate            string `json:"createdDate,omitempty"`
	IsInternalGroup        bool   `json:"isInternalGroup,omitempty"`
	HasAccessToAllBuilds   bool   `json:"hasAccessToAllBuilds,omitempty"`
	PublicLinkEnabled      bool   `json:"publicLinkEnabled,omitempty"`
	PublicLinkLimitEnabled bool   `json:"publicLinkLimitEnabled,omitempty"`
	PublicLinkLimit        int    `json:"publicLinkLimit,omitempty"`
	PublicLink             string `json:"publicLink,omitempty"`
	FeedbackEnabled        bool   `json:"feedbackEnabled,omitempty"`
}

// BetaTesterAttributes describes a beta tester resource.
type BetaTesterAttributes struct {
	FirstName  string          `json:"firstName,omitempty"`
	LastName   string          `json:"lastName,omitempty"`
	Email      string          `json:"email,omitempty"`
	InviteType BetaInviteType  `json:"inviteType,omitempty"`
	State      BetaTesterState `json:"state,omitempty"`
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

// AppStoreVersionLocalizationCreateRequest is a request to create a version localization.
type AppStoreVersionLocalizationCreateRequest struct {
	Data AppStoreVersionLocalizationCreateData `json:"data"`
}

// AppStoreVersionLocalizationUpdateData is the data portion of a version localization update request.
type AppStoreVersionLocalizationUpdateData struct {
	Type       ResourceType                          `json:"type"`
	ID         string                                `json:"id"`
	Attributes AppStoreVersionLocalizationAttributes `json:"attributes"`
}

// AppStoreVersionLocalizationUpdateRequest is a request to update a version localization.
type AppStoreVersionLocalizationUpdateRequest struct {
	Data AppStoreVersionLocalizationUpdateData `json:"data"`
}

// AppStoreVersionLocalizationRelationships describes relationships for version localizations.
type AppStoreVersionLocalizationRelationships struct {
	AppStoreVersion *Relationship `json:"appStoreVersion"`
}

// BetaAppLocalizationCreateData is the data portion of a beta app localization create request.
type BetaAppLocalizationCreateData struct {
	Type          ResourceType                   `json:"type"`
	Attributes    BetaAppLocalizationAttributes  `json:"attributes"`
	Relationships *BetaAppLocalizationRelationships `json:"relationships"`
}

// BetaAppLocalizationCreateRequest is a request to create a beta app localization.
type BetaAppLocalizationCreateRequest struct {
	Data BetaAppLocalizationCreateData `json:"data"`
}

// BetaAppLocalizationUpdateAttributes describes attributes for updating beta app localizations.
type BetaAppLocalizationUpdateAttributes struct {
	FeedbackEmail     *string `json:"feedbackEmail,omitempty"`
	MarketingURL      *string `json:"marketingUrl,omitempty"`
	PrivacyPolicyURL  *string `json:"privacyPolicyUrl,omitempty"`
	TvOsPrivacyPolicy *string `json:"tvOsPrivacyPolicy,omitempty"`
	Description       *string `json:"description,omitempty"`
}

// BetaAppLocalizationUpdateData is the data portion of a beta app localization update request.
type BetaAppLocalizationUpdateData struct {
	Type       ResourceType                   `json:"type"`
	ID         string                         `json:"id"`
	Attributes *BetaAppLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

// BetaAppLocalizationUpdateRequest is a request to update a beta app localization.
type BetaAppLocalizationUpdateRequest struct {
	Data BetaAppLocalizationUpdateData `json:"data"`
}

// BetaAppLocalizationRelationships describes relationships for beta app localizations.
type BetaAppLocalizationRelationships struct {
	App *Relationship `json:"app"`
}

// BetaBuildLocalizationCreateData is the data portion of a beta build localization create request.
type BetaBuildLocalizationCreateData struct {
	Type          ResourceType                        `json:"type"`
	Attributes    BetaBuildLocalizationAttributes     `json:"attributes"`
	Relationships *BetaBuildLocalizationRelationships `json:"relationships"`
}

// BetaBuildLocalizationCreateRequest is a request to create a beta build localization.
type BetaBuildLocalizationCreateRequest struct {
	Data BetaBuildLocalizationCreateData `json:"data"`
}

// BetaBuildLocalizationUpdateData is the data portion of a beta build localization update request.
type BetaBuildLocalizationUpdateData struct {
	Type       ResourceType                    `json:"type"`
	ID         string                          `json:"id"`
	Attributes BetaBuildLocalizationAttributes `json:"attributes"`
}

// BetaBuildLocalizationUpdateRequest is a request to update a beta build localization.
type BetaBuildLocalizationUpdateRequest struct {
	Data BetaBuildLocalizationUpdateData `json:"data"`
}

// BetaBuildLocalizationRelationships describes relationships for beta build localizations.
type BetaBuildLocalizationRelationships struct {
	Build *Relationship `json:"build"`
}

// AppInfoLocalizationCreateData is the data portion of an app info localization create request.
type AppInfoLocalizationCreateData struct {
	Type          ResourceType                      `json:"type"`
	Attributes    AppInfoLocalizationAttributes     `json:"attributes"`
	Relationships *AppInfoLocalizationRelationships `json:"relationships"`
}

// AppInfoLocalizationCreateRequest is a request to create an app info localization.
type AppInfoLocalizationCreateRequest struct {
	Data AppInfoLocalizationCreateData `json:"data"`
}

// AppInfoLocalizationUpdateData is the data portion of an app info localization update request.
type AppInfoLocalizationUpdateData struct {
	Type       ResourceType                  `json:"type"`
	ID         string                        `json:"id"`
	Attributes AppInfoLocalizationAttributes `json:"attributes"`
}

// AppInfoLocalizationUpdateRequest is a request to update an app info localization.
type AppInfoLocalizationUpdateRequest struct {
	Data AppInfoLocalizationUpdateData `json:"data"`
}

// AppInfoLocalizationRelationships describes relationships for app info localizations.
type AppInfoLocalizationRelationships struct {
	AppInfo *Relationship `json:"appInfo"`
}

// BetaGroupCreateData is the data portion of a beta group create request.
type BetaGroupCreateData struct {
	Type          ResourceType            `json:"type"`
	Attributes    BetaGroupAttributes     `json:"attributes"`
	Relationships *BetaGroupRelationships `json:"relationships"`
}

// BetaGroupCreateRequest is a request to create a beta group.
type BetaGroupCreateRequest struct {
	Data BetaGroupCreateData `json:"data"`
}

// BetaGroupUpdateAttributes describes attributes for updating a beta group.
type BetaGroupUpdateAttributes struct {
	Name                   string `json:"name,omitempty"`
	PublicLinkEnabled      *bool  `json:"publicLinkEnabled,omitempty"`
	PublicLinkLimitEnabled *bool  `json:"publicLinkLimitEnabled,omitempty"`
	PublicLinkLimit        int    `json:"publicLinkLimit,omitempty"`
	FeedbackEnabled        *bool  `json:"feedbackEnabled,omitempty"`
	IsInternalGroup        *bool  `json:"isInternalGroup,omitempty"`
	HasAccessToAllBuilds   *bool  `json:"hasAccessToAllBuilds,omitempty"`
}

// BetaGroupUpdateData is the data portion of a beta group update request.
type BetaGroupUpdateData struct {
	Type       ResourceType               `json:"type"`
	ID         string                     `json:"id"`
	Attributes *BetaGroupUpdateAttributes `json:"attributes,omitempty"`
}

// BetaGroupUpdateRequest is a request to update a beta group.
type BetaGroupUpdateRequest struct {
	Data BetaGroupUpdateData `json:"data"`
}

// BetaGroupRelationships describes relationships for beta groups.
type BetaGroupRelationships struct {
	App *Relationship `json:"app"`
}

// BetaTesterCreateAttributes describes attributes for creating a beta tester.
type BetaTesterCreateAttributes struct {
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Email     string `json:"email"`
}

// BetaTesterCreateRelationships describes relationships for beta tester creation.
type BetaTesterCreateRelationships struct {
	BetaGroups *RelationshipList `json:"betaGroups,omitempty"`
}

// BetaTesterCreateData is the data portion of a beta tester create request.
type BetaTesterCreateData struct {
	Type          ResourceType                   `json:"type"`
	Attributes    BetaTesterCreateAttributes     `json:"attributes"`
	Relationships *BetaTesterCreateRelationships `json:"relationships,omitempty"`
}

// BetaTesterCreateRequest is a request to create a beta tester.
type BetaTesterCreateRequest struct {
	Data BetaTesterCreateData `json:"data"`
}

// BetaTesterInvitationCreateRelationships describes relationships for invitations.
type BetaTesterInvitationCreateRelationships struct {
	App        *Relationship `json:"app"`
	BetaTester *Relationship `json:"betaTester,omitempty"`
}

// BetaTesterInvitationCreateData is the data portion of an invitation create request.
type BetaTesterInvitationCreateData struct {
	Type          ResourceType                             `json:"type"`
	Relationships *BetaTesterInvitationCreateRelationships `json:"relationships"`
}

// BetaTesterInvitationCreateRequest is a request to create a beta tester invitation.
type BetaTesterInvitationCreateRequest struct {
	Data BetaTesterInvitationCreateData `json:"data"`
}

// GetFeedback retrieves TestFlight feedback
func (c *Client) GetFeedback(ctx context.Context, appID string, opts ...FeedbackOption) (*FeedbackResponse, error) {
	query := &feedbackQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/apps/%s/betaFeedbackScreenshotSubmissions", appID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("feedback: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildFeedbackQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response FeedbackResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCrashes retrieves TestFlight crash reports
func (c *Client) GetCrashes(ctx context.Context, appID string, opts ...CrashOption) (*CrashesResponse, error) {
	query := &crashQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/apps/%s/betaFeedbackCrashSubmissions", appID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("crashes: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCrashQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CrashesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetReviews retrieves App Store reviews
func (c *Client) GetReviews(ctx context.Context, appID string, opts ...ReviewOption) (*ReviewsResponse, error) {
	query := &reviewQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/apps/%s/customerReviews", appID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("reviews: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildReviewQuery(opts); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ReviewsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaGroups retrieves the list of beta groups for an app.
func (c *Client) GetBetaGroups(ctx context.Context, appID string, opts ...BetaGroupsOption) (*BetaGroupsResponse, error) {
	query := &betaGroupsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/apps/%s/betaGroups", appID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaGroups: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaGroupsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaGroupsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaGroupBuilds retrieves builds assigned to a beta group.
func (c *Client) GetBetaGroupBuilds(ctx context.Context, groupID string, opts ...BetaGroupBuildsOption) (*BuildsResponse, error) {
	query := &betaGroupBuildsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/betaGroups/%s/builds", groupID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaGroupBuilds: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaGroupBuildsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaGroupTesters retrieves beta testers assigned to a beta group.
func (c *Client) GetBetaGroupTesters(ctx context.Context, groupID string, opts ...BetaGroupTestersOption) (*BetaTestersResponse, error) {
	query := &betaGroupTestersQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/betaGroups/%s/betaTesters", groupID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaGroupTesters: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaGroupTestersQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaTestersResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateBetaGroup creates a beta group for an app.
func (c *Client) CreateBetaGroup(ctx context.Context, appID, name string) (*BetaGroupResponse, error) {
	payload := BetaGroupCreateRequest{
		Data: BetaGroupCreateData{
			Type:       ResourceTypeBetaGroups,
			Attributes: BetaGroupAttributes{Name: name},
			Relationships: &BetaGroupRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/betaGroups", body)
	if err != nil {
		return nil, err
	}

	var response BetaGroupResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaGroup retrieves a beta group by ID.
func (c *Client) GetBetaGroup(ctx context.Context, groupID string) (*BetaGroupResponse, error) {
	path := fmt.Sprintf("/v1/betaGroups/%s", groupID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaGroupResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateBetaGroup updates a beta group by ID.
func (c *Client) UpdateBetaGroup(ctx context.Context, groupID string, req BetaGroupUpdateRequest) (*BetaGroupResponse, error) {
	body, err := BuildRequestBody(req)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/betaGroups/%s", groupID), body)
	if err != nil {
		return nil, err
	}

	var response BetaGroupResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteBetaGroup deletes a beta group by ID.
func (c *Client) DeleteBetaGroup(ctx context.Context, groupID string) error {
	path := fmt.Sprintf("/v1/betaGroups/%s", groupID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// AddBetaTestersToGroup adds testers to a beta group.
func (c *Client) AddBetaTestersToGroup(ctx context.Context, groupID string, testerIDs []string) error {
	testerIDs = normalizeList(testerIDs)
	payload := RelationshipRequest{
		Data: make([]RelationshipData, 0, len(testerIDs)),
	}
	for _, testerID := range testerIDs {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeBetaTesters,
			ID:   testerID,
		})
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/betaGroups/%s/relationships/betaTesters", groupID)
	_, err = c.do(ctx, "POST", path, body)
	return err
}

// RemoveBetaTestersFromGroup removes testers from a beta group.
func (c *Client) RemoveBetaTestersFromGroup(ctx context.Context, groupID string, testerIDs []string) error {
	testerIDs = normalizeList(testerIDs)
	payload := RelationshipRequest{
		Data: make([]RelationshipData, 0, len(testerIDs)),
	}
	for _, testerID := range testerIDs {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeBetaTesters,
			ID:   testerID,
		})
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/betaGroups/%s/relationships/betaTesters", groupID)
	_, err = c.do(ctx, "DELETE", path, body)
	return err
}

// GetBetaTesters retrieves beta testers for an app.
func (c *Client) GetBetaTesters(ctx context.Context, appID string, opts ...BetaTestersOption) (*BetaTestersResponse, error) {
	query := &betaTestersQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/betaTesters"
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaTesters: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaTestersQuery(appID, query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaTestersResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaTester retrieves a beta tester by ID.
func (c *Client) GetBetaTester(ctx context.Context, testerID string) (*BetaTesterResponse, error) {
	path := fmt.Sprintf("/v1/betaTesters/%s", testerID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaTesterResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateBetaTester creates a beta tester.
func (c *Client) CreateBetaTester(ctx context.Context, email, firstName, lastName string, groupIDs []string) (*BetaTesterResponse, error) {
	groupIDs = normalizeList(groupIDs)
	var relationships *BetaTesterCreateRelationships
	if len(groupIDs) > 0 {
		relData := make([]ResourceData, 0, len(groupIDs))
		for _, groupID := range groupIDs {
			relData = append(relData, ResourceData{
				Type: ResourceTypeBetaGroups,
				ID:   groupID,
			})
		}
		relationships = &BetaTesterCreateRelationships{
			BetaGroups: &RelationshipList{Data: relData},
		}
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/betaTesters", body)
	if err != nil {
		return nil, err
	}

	var response BetaTesterResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// AddBetaTesterToGroups adds a tester to multiple beta groups.
func (c *Client) AddBetaTesterToGroups(ctx context.Context, testerID string, groupIDs []string) error {
	testerID = strings.TrimSpace(testerID)
	groupIDs = normalizeList(groupIDs)
	if testerID == "" {
		return fmt.Errorf("tester ID is required")
	}
	if len(groupIDs) == 0 {
		return fmt.Errorf("group IDs are required")
	}

	relData := make([]ResourceData, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		relData = append(relData, ResourceData{
			Type: ResourceTypeBetaGroups,
			ID:   groupID,
		})
	}

	payload := RelationshipList{Data: relData}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/betaTesters/%s/relationships/betaGroups", testerID)
	if _, err := c.do(ctx, "POST", path, body); err != nil {
		return err
	}
	return nil
}

// RemoveBetaTesterFromGroups removes a tester from multiple beta groups.
func (c *Client) RemoveBetaTesterFromGroups(ctx context.Context, testerID string, groupIDs []string) error {
	testerID = strings.TrimSpace(testerID)
	groupIDs = normalizeList(groupIDs)
	if testerID == "" {
		return fmt.Errorf("tester ID is required")
	}
	if len(groupIDs) == 0 {
		return fmt.Errorf("group IDs are required")
	}

	relData := make([]ResourceData, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		relData = append(relData, ResourceData{
			Type: ResourceTypeBetaGroups,
			ID:   groupID,
		})
	}

	payload := RelationshipList{Data: relData}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/betaTesters/%s/relationships/betaGroups", testerID)
	if _, err := c.do(ctx, "DELETE", path, body); err != nil {
		return err
	}
	return nil
}

// DeleteBetaTester deletes a beta tester by ID.
func (c *Client) DeleteBetaTester(ctx context.Context, testerID string) error {
	path := fmt.Sprintf("/v1/betaTesters/%s", testerID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// CreateBetaTesterInvitation creates a beta tester invitation.
func (c *Client) CreateBetaTesterInvitation(ctx context.Context, appID, testerID string) (*BetaTesterInvitationResponse, error) {
	payload := BetaTesterInvitationCreateRequest{
		Data: BetaTesterInvitationCreateData{
			Type: ResourceTypeBetaTesterInvitations,
			Relationships: &BetaTesterInvitationCreateRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
				BetaTester: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeBetaTesters,
						ID:   testerID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/betaTesterInvitations", body)
	if err != nil {
		return nil, err
	}

	var response BetaTesterInvitationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionLocalizations retrieves localizations for an app store version.
func (c *Client) GetAppStoreVersionLocalizations(ctx context.Context, versionID string, opts ...AppStoreVersionLocalizationsOption) (*AppStoreVersionLocalizationsResponse, error) {
	query := &appStoreVersionLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/appStoreVersionLocalizations", versionID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appStoreVersionLocalizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppStoreVersionLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionLocalization retrieves a single app store version localization by ID.
func (c *Client) GetAppStoreVersionLocalization(ctx context.Context, localizationID string) (*AppStoreVersionLocalizationResponse, error) {
	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s", localizationID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppStoreVersionLocalization creates a localization for an app store version.
func (c *Client) CreateAppStoreVersionLocalization(ctx context.Context, versionID string, attributes AppStoreVersionLocalizationAttributes) (*AppStoreVersionLocalizationResponse, error) {
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appStoreVersionLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppStoreVersionLocalization updates a localization for an app store version.
func (c *Client) UpdateAppStoreVersionLocalization(ctx context.Context, localizationID string, attributes AppStoreVersionLocalizationAttributes) (*AppStoreVersionLocalizationResponse, error) {
	payload := AppStoreVersionLocalizationUpdateRequest{
		Data: AppStoreVersionLocalizationUpdateData{
			Type:       ResourceTypeAppStoreVersionLocalizations,
			ID:         localizationID,
			Attributes: attributes,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s", localizationID)
	data, err := c.do(ctx, "PATCH", path, body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppStoreVersionLocalization deletes a localization by ID.
func (c *Client) DeleteAppStoreVersionLocalization(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s", localizationID)
	if _, err := c.do(ctx, "DELETE", path, nil); err != nil {
		return err
	}
	return nil
}

// GetBetaBuildLocalizations retrieves beta build localizations for a build.
func (c *Client) GetBetaBuildLocalizations(ctx context.Context, buildID string, opts ...BetaBuildLocalizationsOption) (*BetaBuildLocalizationsResponse, error) {
	query := &betaBuildLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/builds/%s/betaBuildLocalizations", buildID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaBuildLocalizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaBuildLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaBuildLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaBuildLocalization retrieves a single beta build localization by ID.
func (c *Client) GetBetaBuildLocalization(ctx context.Context, localizationID string) (*BetaBuildLocalizationResponse, error) {
	path := fmt.Sprintf("/v1/betaBuildLocalizations/%s", localizationID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaBuildLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateBetaBuildLocalization creates a beta build localization for a build.
func (c *Client) CreateBetaBuildLocalization(ctx context.Context, buildID string, attributes BetaBuildLocalizationAttributes) (*BetaBuildLocalizationResponse, error) {
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/betaBuildLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response BetaBuildLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateBetaBuildLocalization updates a beta build localization by ID.
func (c *Client) UpdateBetaBuildLocalization(ctx context.Context, localizationID string, attributes BetaBuildLocalizationAttributes) (*BetaBuildLocalizationResponse, error) {
	payload := BetaBuildLocalizationUpdateRequest{
		Data: BetaBuildLocalizationUpdateData{
			Type:       ResourceTypeBetaBuildLocalizations,
			ID:         localizationID,
			Attributes: attributes,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/betaBuildLocalizations/%s", localizationID)
	data, err := c.do(ctx, "PATCH", path, body)
	if err != nil {
		return nil, err
	}

	var response BetaBuildLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteBetaBuildLocalization deletes a beta build localization by ID.
func (c *Client) DeleteBetaBuildLocalization(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v1/betaBuildLocalizations/%s", localizationID)
	if _, err := c.do(ctx, "DELETE", path, nil); err != nil {
		return err
	}
	return nil
}

// GetAppInfoLocalizations retrieves localizations for an app info resource.
func (c *Client) GetAppInfoLocalizations(ctx context.Context, appInfoID string, opts ...AppInfoLocalizationsOption) (*AppInfoLocalizationsResponse, error) {
	query := &appInfoLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/appInfos/%s/appInfoLocalizations", appInfoID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appInfoLocalizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppInfoLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppInfoLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppInfoLocalization creates a localization for an app info resource.
func (c *Client) CreateAppInfoLocalization(ctx context.Context, appInfoID string, attributes AppInfoLocalizationAttributes) (*AppInfoLocalizationResponse, error) {
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appInfoLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response AppInfoLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppInfoLocalization updates a localization for an app info resource.
func (c *Client) UpdateAppInfoLocalization(ctx context.Context, localizationID string, attributes AppInfoLocalizationAttributes) (*AppInfoLocalizationResponse, error) {
	payload := AppInfoLocalizationUpdateRequest{
		Data: AppInfoLocalizationUpdateData{
			Type:       ResourceTypeAppInfoLocalizations,
			ID:         localizationID,
			Attributes: attributes,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appInfoLocalizations/%s", localizationID)
	data, err := c.do(ctx, "PATCH", path, body)
	if err != nil {
		return nil, err
	}

	var response AppInfoLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppInfos retrieves app info records for an app.
func (c *Client) GetAppInfos(ctx context.Context, appID string) (*AppInfosResponse, error) {
	path := fmt.Sprintf("/v1/apps/%s/appInfos", appID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppInfosResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppInfo retrieves an app info record by ID.
func (c *Client) GetAppInfo(ctx context.Context, appInfoID string, opts ...AppInfoOption) (*AppInfoResponse, error) {
	query := &appInfoQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	path := fmt.Sprintf("/v1/appInfos/%s", appInfoID)
	if queryString := buildAppInfoQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppInfoResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
