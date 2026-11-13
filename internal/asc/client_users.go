package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetUsers retrieves the list of users.
func (c *Client) GetUsers(ctx context.Context, opts ...UsersOption) (*UsersResponse, error) {
	query := applyOptions(opts)
	return getJSON[UsersResponse](ctx, c, readRequest{
		Path:        "/v1/users",
		Query:       buildUsersQuery(query),
		Next:        query.nextURL,
		NextContext: "users",
	})
}

// GetUser retrieves a single user by ID.
func (c *Client) GetUser(ctx context.Context, userID string, opts ...UsersOption) (*UserResponse, error) {
	userID = strings.TrimSpace(userID)
	query := applyOptions(opts)
	return getJSON[UserResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/users/%s", userID),
		Query: buildUsersQuery(query),
	})
}

// UpdateUser updates a user by ID.
//
// UserUpdateRequest treats attributes.allAppsVisible and
// relationships.visibleApps as independent optional fields, and this builder
// sends only what the caller supplies so an attributes-only or a
// relationships-only PATCH stays expressible. Pairing them is therefore the
// caller's responsibility: a visible-apps list has no effect while the account
// still has allAppsVisible=true, so `asc users update` sets AllAppsVisible to
// false whenever --visible-app is present, which
// TestUsersUpdateSendsRolesAndVisibleAppsInOneRequest asserts. This differs from
// CreateUserInvitation, which creates the account and therefore defaults the
// attribute itself.
func (c *Client) UpdateUser(ctx context.Context, userID string, attrs UserUpdateAttributes, visibleAppIDs []string) (*UserResponse, error) {
	userID = strings.TrimSpace(userID)
	visibleAppIDs = normalizeList(visibleAppIDs)
	payload := UserUpdateRequest{
		Data: UserUpdateData{
			Type: ResourceTypeUsers,
			ID:   userID,
		},
	}
	if len(attrs.Roles) > 0 || attrs.AllAppsVisible != nil || attrs.ProvisioningAllowed != nil {
		payload.Data.Attributes = &attrs
	}
	if len(visibleAppIDs) > 0 {
		relationships := &UserUpdateRelationships{
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
	return sendJSON[UserResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/users/%s", userID), Payload: payload})
}

// DeleteUser deletes a user by ID.
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/users/%s", userID), nil)
}

// GetUserInvitations retrieves the list of user invitations.
func (c *Client) GetUserInvitations(ctx context.Context, opts ...UserInvitationsOption) (*UserInvitationsResponse, error) {
	query := applyOptions(opts)
	return getJSON[UserInvitationsResponse](ctx, c, readRequest{
		Path:        "/v1/userInvitations",
		Query:       buildUserInvitationsQuery(query),
		Next:        query.nextURL,
		NextContext: "userInvitations",
	})
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
	return sendJSON[UserInvitationResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/userInvitations", Payload: payload})
}

// GetUserInvitation retrieves a user invitation by ID.
func (c *Client) GetUserInvitation(ctx context.Context, inviteID string) (*UserInvitationResponse, error) {
	inviteID = strings.TrimSpace(inviteID)
	return getJSON[UserInvitationResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/userInvitations/%s", inviteID)})
}

// DeleteUserInvitation deletes a user invitation by ID.
func (c *Client) DeleteUserInvitation(ctx context.Context, inviteID string) error {
	inviteID = strings.TrimSpace(inviteID)
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/userInvitations/%s", inviteID), nil)
}

// GetUserVisibleApps retrieves the visible apps for a user.
func (c *Client) GetUserVisibleApps(ctx context.Context, userID string, opts ...UserVisibleAppsOption) (*AppsResponse, error) {
	userID = strings.TrimSpace(userID)
	query := applyOptions(opts)
	return getJSON[AppsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/users/%s/visibleApps", userID),
		Query:       buildUserVisibleAppsQuery(query),
		Next:        query.nextURL,
		NextContext: "userVisibleApps",
	})
}

// GetUserVisibleAppsRelationships retrieves visible app linkages for a user.
func (c *Client) GetUserVisibleAppsRelationships(ctx context.Context, userID string, opts ...LinkagesOption) (*UserVisibleAppsLinkagesResponse, error) {
	userID = strings.TrimSpace(userID)
	query := applyOptions(opts)
	if query.nextURL == "" && userID == "" {
		return nil, fmt.Errorf("userID is required")
	}
	return getJSON[UserVisibleAppsLinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/users/%s/relationships/visibleApps", userID),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: "userVisibleAppsRelationships",
	})
}

// AddUserVisibleApps adds visible apps to a user.
func (c *Client) AddUserVisibleApps(ctx context.Context, userID string, appIDs []string) error {
	userID = strings.TrimSpace(userID)
	appIDs = normalizeList(appIDs)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}
	if len(appIDs) == 0 {
		return fmt.Errorf("appIDs are required")
	}
	payload := RelationshipRequest{
		Data: make([]RelationshipData, 0, len(appIDs)),
	}
	for _, id := range appIDs {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeApps,
			ID:   id,
		})
	}
	return sendNoContent(ctx, c, "POST", fmt.Sprintf("/v1/users/%s/relationships/visibleApps", userID), payload)
}

// RemoveUserVisibleApps removes visible apps from a user.
func (c *Client) RemoveUserVisibleApps(ctx context.Context, userID string, appIDs []string) error {
	userID = strings.TrimSpace(userID)
	appIDs = normalizeList(appIDs)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}
	if len(appIDs) == 0 {
		return fmt.Errorf("appIDs are required")
	}
	payload := RelationshipRequest{
		Data: make([]RelationshipData, 0, len(appIDs)),
	}
	for _, id := range appIDs {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeApps,
			ID:   id,
		})
	}
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/users/%s/relationships/visibleApps", userID), payload)
}
