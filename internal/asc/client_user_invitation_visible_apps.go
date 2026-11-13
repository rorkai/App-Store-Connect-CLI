package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetUserInvitationVisibleApps retrieves visible apps for a user invitation.
func (c *Client) GetUserInvitationVisibleApps(ctx context.Context, invitationID string, opts ...UserInvitationVisibleAppsOption) (*AppsResponse, error) {
	query := applyOptions(opts)
	return getJSON[AppsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/userInvitations/%s/visibleApps", strings.TrimSpace(invitationID)),
		Query:       buildUserInvitationVisibleAppsQuery(query),
		Next:        query.nextURL,
		NextContext: "userInvitationVisibleApps",
	})
}
