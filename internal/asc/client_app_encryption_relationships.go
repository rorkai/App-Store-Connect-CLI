package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AppEncryptionDeclarationAppLinkageResponse is the response for encryption declaration app relationship.
type AppEncryptionDeclarationAppLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppEncryptionDeclarationDocumentLinkageResponse is the response for encryption declaration document relationship.
type AppEncryptionDeclarationDocumentLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetAppEncryptionDeclarationApp retrieves the app for an encryption declaration.
func (c *Client) GetAppEncryptionDeclarationApp(ctx context.Context, declarationID string) (*AppResponse, error) {
	declarationID = strings.TrimSpace(declarationID)
	if declarationID == "" {
		return nil, fmt.Errorf("declarationID is required")
	}

	path := fmt.Sprintf("/v1/appEncryptionDeclarations/%s/app", declarationID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEncryptionDeclarationAppRelationship retrieves the app linkage for an encryption declaration.
func (c *Client) GetAppEncryptionDeclarationAppRelationship(ctx context.Context, declarationID string) (*AppEncryptionDeclarationAppLinkageResponse, error) {
	declarationID = strings.TrimSpace(declarationID)
	if declarationID == "" {
		return nil, fmt.Errorf("declarationID is required")
	}

	path := fmt.Sprintf("/v1/appEncryptionDeclarations/%s/relationships/app", declarationID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEncryptionDeclarationAppLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEncryptionDeclarationDocumentForDeclaration retrieves the document for an encryption declaration.
func (c *Client) GetAppEncryptionDeclarationDocumentForDeclaration(ctx context.Context, declarationID string) (*AppEncryptionDeclarationDocumentResponse, error) {
	declarationID = strings.TrimSpace(declarationID)
	if declarationID == "" {
		return nil, fmt.Errorf("declarationID is required")
	}

	path := fmt.Sprintf("/v1/appEncryptionDeclarations/%s/appEncryptionDeclarationDocument", declarationID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEncryptionDeclarationDocumentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEncryptionDeclarationDocumentRelationship retrieves the document linkage for an encryption declaration.
func (c *Client) GetAppEncryptionDeclarationDocumentRelationship(ctx context.Context, declarationID string) (*AppEncryptionDeclarationDocumentLinkageResponse, error) {
	declarationID = strings.TrimSpace(declarationID)
	if declarationID == "" {
		return nil, fmt.Errorf("declarationID is required")
	}

	path := fmt.Sprintf("/v1/appEncryptionDeclarations/%s/relationships/appEncryptionDeclarationDocument", declarationID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEncryptionDeclarationDocumentLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEncryptionDeclarationsForApp retrieves encryption declarations for an app (v1/apps).
func (c *Client) GetAppEncryptionDeclarationsForApp(ctx context.Context, appID string, opts ...AppEncryptionDeclarationsOption) (*AppEncryptionDeclarationsResponse, error) {
	query := &appEncryptionDeclarationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/appEncryptionDeclarations", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appEncryptionDeclarations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppEncryptionDeclarationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEncryptionDeclarationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
