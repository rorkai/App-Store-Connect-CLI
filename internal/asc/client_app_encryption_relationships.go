package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetAppEncryptionDeclarationsForApp retrieves encryption declarations for an app (v1/apps).
func (c *Client) GetAppEncryptionDeclarationsForApp(ctx context.Context, appID string, opts ...AppEncryptionDeclarationsOption) (*AppEncryptionDeclarationsResponse, error) {
	query := applyOptions(opts)

	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	return getJSON[AppEncryptionDeclarationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/apps/%s/appEncryptionDeclarations", appID),
		Query:       buildAppEncryptionDeclarationsQuery(query),
		Next:        query.nextURL,
		NextContext: "appEncryptionDeclarations",
	})
}
