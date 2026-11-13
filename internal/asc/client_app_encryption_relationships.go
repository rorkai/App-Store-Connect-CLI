package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetAppEncryptionDeclarationApp retrieves the app for an encryption declaration.
func (c *Client) GetAppEncryptionDeclarationApp(ctx context.Context, declarationID string) (*AppResponse, error) {
	declarationID = strings.TrimSpace(declarationID)
	if declarationID == "" {
		return nil, fmt.Errorf("declarationID is required")
	}

	return getJSON[AppResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appEncryptionDeclarations/%s/app", declarationID)})
}

// GetAppEncryptionDeclarationDocumentForDeclaration retrieves the document for an encryption declaration.
func (c *Client) GetAppEncryptionDeclarationDocumentForDeclaration(ctx context.Context, declarationID string) (*AppEncryptionDeclarationDocumentResponse, error) {
	declarationID = strings.TrimSpace(declarationID)
	if declarationID == "" {
		return nil, fmt.Errorf("declarationID is required")
	}

	return getJSON[AppEncryptionDeclarationDocumentResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appEncryptionDeclarations/%s/appEncryptionDeclarationDocument", declarationID)})
}

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
