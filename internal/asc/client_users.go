package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetUsers retrieves the list of users.
func (c *Client) GetUsers(ctx context.Context, opts ...UsersOption) (*UsersResponse, error) {
	query := &usersQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/users"
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("users: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildUsersQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response UsersResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetUser retrieves a single user by ID.
func (c *Client) GetUser(ctx context.Context, userID string, opts ...UsersOption) (*UserResponse, error) {
	userID = strings.TrimSpace(userID)
	query := &usersQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/users/%s", userID)
	if queryString := buildUsersQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response UserResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateUser updates a user by ID.
func (c *Client) UpdateUser(ctx context.Context, userID string, attrs UserUpdateAttributes) (*UserResponse, error) {
	userID = strings.TrimSpace(userID)
	payload := UserUpdateRequest{
		Data: UserUpdateData{
			Type: ResourceTypeUsers,
			ID:   userID,
		},
	}
	if len(attrs.Roles) > 0 || attrs.AllAppsVisible != nil || attrs.ProvisioningAllowed != nil {
		payload.Data.Attributes = &attrs
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/users/%s", userID), body)
	if err != nil {
		return nil, err
	}

	var response UserResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteUser deletes a user by ID.
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	path := fmt.Sprintf("/v1/users/%s", userID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetUserInvitations retrieves the list of user invitations.
func (c *Client) GetUserInvitations(ctx context.Context, opts ...UserInvitationsOption) (*UserInvitationsResponse, error) {
	query := &userInvitationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/userInvitations"
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("userInvitations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildUserInvitationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response UserInvitationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateUserInvitation creates a new user invitation.
func (c *Client) CreateUserInvitation(ctx context.Context, attrs UserInvitationCreateAttributes, visibleAppIDs []string) (*UserInvitationResponse, error) {
	visibleAppIDs = normalizeList(visibleAppIDs)
	if len(visibleAppIDs) > 0 && attrs.AllAppsVisible == nil {
		allAppsVisible := false
		attrs.AllAppsVisible = &allAppsVisible
	}

	payload := UserInvitationCreateRequest{
		Data: UserInvitationCreateData{
			Type:       ResourceTypeUserInvitations,
			Attributes: attrs,
		},
	}

	if len(visibleAppIDs) > 0 {
		relationships := &UserInvitationCreateRelationships{
			VisibleApps: &RelationshipList{
				Data: make([]ResourceData, 0, len(visibleAppIDs)),
			},
		}
		for _, appID := range visibleAppIDs {
			relationships.VisibleApps.Data = append(relationships.VisibleApps.Data, ResourceData{
				Type: ResourceTypeApps,
				ID:   appID,
			})
		}
		payload.Data.Relationships = relationships
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/userInvitations", body)
	if err != nil {
		return nil, err
	}

	var response UserInvitationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetUserInvitation retrieves a user invitation by ID.
func (c *Client) GetUserInvitation(ctx context.Context, inviteID string) (*UserInvitationResponse, error) {
	inviteID = strings.TrimSpace(inviteID)
	path := fmt.Sprintf("/v1/userInvitations/%s", inviteID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response UserInvitationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteUserInvitation deletes a user invitation by ID.
func (c *Client) DeleteUserInvitation(ctx context.Context, inviteID string) error {
	inviteID = strings.TrimSpace(inviteID)
	path := fmt.Sprintf("/v1/userInvitations/%s", inviteID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetUserVisibleApps retrieves the visible apps for a user.
func (c *Client) GetUserVisibleApps(ctx context.Context, userID string, opts ...UserVisibleAppsOption) (*AppsResponse, error) {
	userID = strings.TrimSpace(userID)
	query := &userVisibleAppsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/users/%s/visibleApps", userID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("userVisibleApps: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildUserVisibleAppsQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetUserVisibleAppsRelationships retrieves visible app linkages for a user.
func (c *Client) GetUserVisibleAppsRelationships(ctx context.Context, userID string, opts ...LinkagesOption) (*UserVisibleAppsLinkagesResponse, error) {
	userID = strings.TrimSpace(userID)
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/users/%s/relationships/visibleApps", userID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("userVisibleAppsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response UserVisibleAppsLinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// AddUserVisibleApps adds visible apps to a user.
func (c *Client) AddUserVisibleApps(ctx context.Context, userID string, appIDs []string) error {
	userID = strings.TrimSpace(userID)
	appIDs = normalizeList(appIDs)
	payload := RelationshipRequest{
		Data: make([]RelationshipData, 0, len(appIDs)),
	}
	for _, id := range appIDs {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeApps,
			ID:   id,
		})
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/users/%s/relationships/visibleApps", userID)
	_, err = c.do(ctx, "POST", path, body)
	return err
}

// RemoveUserVisibleApps removes visible apps from a user.
func (c *Client) RemoveUserVisibleApps(ctx context.Context, userID string, appIDs []string) error {
	userID = strings.TrimSpace(userID)
	appIDs = normalizeList(appIDs)
	payload := RelationshipRequest{
		Data: make([]RelationshipData, 0, len(appIDs)),
	}
	for _, id := range appIDs {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeApps,
			ID:   id,
		})
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/users/%s/relationships/visibleApps", userID)
	_, err = c.do(ctx, "DELETE", path, body)
	return err
}

// SetUserVisibleApps replaces the visible apps list for a user.
func (c *Client) SetUserVisibleApps(ctx context.Context, userID string, appIDs []string) error {
	userID = strings.TrimSpace(userID)
	appIDs = normalizeList(appIDs)
	payload := RelationshipRequest{
		Data: make([]RelationshipData, 0, len(appIDs)),
	}
	for _, id := range appIDs {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeApps,
			ID:   id,
		})
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/users/%s/relationships/visibleApps", userID)
	_, err = c.do(ctx, "PATCH", path, body)
	return err
}
