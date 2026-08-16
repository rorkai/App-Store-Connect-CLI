package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// BuildBetaNotificationResource represents a build beta notification resource.
type BuildBetaNotificationResource struct {
	Type ResourceType `json:"type"`
	ID   string       `json:"id"`
}

// BuildBetaNotificationResponse is the response from build beta notification endpoints.
type BuildBetaNotificationResponse struct {
	Data               BuildBetaNotificationResource     `json:"data"`
	Links              Links                             `json:"links"`
	NotificationAction BuildBetaGroupsNotificationAction `json:"notificationAction,omitempty"`
}

// MarshalJSON keeps idempotent action results focused on the action instead of
// emitting zero-valued API resource fields when no notification was created.
func (r BuildBetaNotificationResponse) MarshalJSON() ([]byte, error) {
	if r.NotificationAction != BuildBetaGroupsNotificationActionNone {
		return json.Marshal(struct {
			NotificationAction BuildBetaGroupsNotificationAction `json:"notificationAction"`
		}{NotificationAction: r.NotificationAction})
	}

	type responseAlias BuildBetaNotificationResponse
	return json.Marshal(responseAlias(r))
}

// BuildBetaNotificationRelationships describes relationships for a build beta notification.
type BuildBetaNotificationRelationships struct {
	Build Relationship `json:"build"`
}

// BuildBetaNotificationCreateData is the data portion of a build beta notification create request.
type BuildBetaNotificationCreateData struct {
	Type          ResourceType                       `json:"type"`
	Relationships BuildBetaNotificationRelationships `json:"relationships"`
}

// BuildBetaNotificationCreateRequest is a request to create a build beta notification.
type BuildBetaNotificationCreateRequest struct {
	Data BuildBetaNotificationCreateData `json:"data"`
}

// CreateBuildBetaNotification creates a build beta notification for a build.
func (c *Client) CreateBuildBetaNotification(ctx context.Context, buildID string) (*BuildBetaNotificationResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	payload := BuildBetaNotificationCreateRequest{
		Data: BuildBetaNotificationCreateData{
			Type: ResourceTypeBuildBetaNotifications,
			Relationships: BuildBetaNotificationRelationships{
				Build: Relationship{
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

	data, err := c.do(ctx, "POST", "/v1/buildBetaNotifications", body)
	if err != nil {
		// The preflight detail can become stale before the create reaches Apple.
		// Treat only Apple's known redundant auto-notify conflict as the same
		// successful idempotent outcome.
		if isAutoNotifyAlreadyEnabledNotificationError(err) {
			return &BuildBetaNotificationResponse{
				NotificationAction: BuildBetaGroupsNotificationActionAutoNotifyEnabled,
			}, nil
		}
		return nil, err
	}

	var response BuildBetaNotificationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
