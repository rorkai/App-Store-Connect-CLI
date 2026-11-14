package asc

import (
	"context"
	"fmt"
	"strings"
)

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
