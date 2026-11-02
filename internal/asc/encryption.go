package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// AppEncryptionDeclarationState represents the state of an encryption declaration.
type AppEncryptionDeclarationState string

const (
	AppEncryptionDeclarationStateCreated  AppEncryptionDeclarationState = "CREATED"
	AppEncryptionDeclarationStateInReview AppEncryptionDeclarationState = "IN_REVIEW"
	AppEncryptionDeclarationStateApproved AppEncryptionDeclarationState = "APPROVED"
	AppEncryptionDeclarationStateRejected AppEncryptionDeclarationState = "REJECTED"
	AppEncryptionDeclarationStateInvalid  AppEncryptionDeclarationState = "INVALID"
	AppEncryptionDeclarationStateExpired  AppEncryptionDeclarationState = "EXPIRED"
)

// AppEncryptionDeclarationAttributes describes encryption declaration attributes.
type AppEncryptionDeclarationAttributes struct {
	AppDescription                  string                        `json:"appDescription,omitempty"`
	CreatedDate                     string                        `json:"createdDate,omitempty"`
	UsesEncryption                  *bool                         `json:"usesEncryption,omitempty"`
	Exempt                          *bool                         `json:"exempt,omitempty"`
	ContainsProprietaryCryptography *bool                         `json:"containsProprietaryCryptography,omitempty"`
	ContainsThirdPartyCryptography  *bool                         `json:"containsThirdPartyCryptography,omitempty"`
	AvailableOnFrenchStore          *bool                         `json:"availableOnFrenchStore,omitempty"`
	Platform                        Platform                      `json:"platform,omitempty"`
	UploadedDate                    string                        `json:"uploadedDate,omitempty"`
	DocumentURL                     string                        `json:"documentUrl,omitempty"`
	DocumentName                    string                        `json:"documentName,omitempty"`
	DocumentType                    string                        `json:"documentType,omitempty"`
	AppEncryptionDeclarationState   AppEncryptionDeclarationState `json:"appEncryptionDeclarationState,omitempty"`
	CodeValue                       string                        `json:"codeValue,omitempty"`
}

// AppEncryptionDeclarationsResponse is the response for encryption declaration lists.
type AppEncryptionDeclarationsResponse = Response[AppEncryptionDeclarationAttributes]

// AppEncryptionDeclarationResponse is the response for encryption declaration endpoints.
type AppEncryptionDeclarationResponse = SingleResponse[AppEncryptionDeclarationAttributes]

// AppEncryptionDeclarationCreateAttributes describes create attributes.
type AppEncryptionDeclarationCreateAttributes struct {
	AppDescription                  string `json:"appDescription"`
	ContainsProprietaryCryptography bool   `json:"containsProprietaryCryptography"`
	ContainsThirdPartyCryptography  bool   `json:"containsThirdPartyCryptography"`
	AvailableOnFrenchStore          bool   `json:"availableOnFrenchStore"`
}

// AppEncryptionDeclarationRelationships describes declaration relationships.
type AppEncryptionDeclarationRelationships struct {
	App *Relationship `json:"app"`
}

// AppEncryptionDeclarationCreateData is the data portion of a create request.
type AppEncryptionDeclarationCreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    AppEncryptionDeclarationCreateAttributes `json:"attributes"`
	Relationships *AppEncryptionDeclarationRelationships   `json:"relationships"`
}

// AppEncryptionDeclarationCreateRequest is a request to create an encryption declaration.
type AppEncryptionDeclarationCreateRequest struct {
	Data AppEncryptionDeclarationCreateData `json:"data"`
}

// AppEncryptionDeclarationBuildsUpdateResult represents CLI output for build assignments.
type AppEncryptionDeclarationBuildsUpdateResult struct {
	DeclarationID string                                 `json:"declarationId"`
	BuildIDs      []string                               `json:"buildIds"`
	Action        string                                 `json:"action"`
	Failures      []AppEncryptionDeclarationBuildFailure `json:"failures,omitempty"`
}

// AppEncryptionDeclarationBuildFailure records a build that was not assigned.
type AppEncryptionDeclarationBuildFailure struct {
	BuildID string `json:"buildId"`
	Error   string `json:"error"`
}

// AppEncryptionDeclarationDocumentAttributes describes document attributes.
type AppEncryptionDeclarationDocumentAttributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	AssetToken         string              `json:"assetToken,omitempty"`
	DownloadURL        string              `json:"downloadUrl,omitempty"`
	SourceFileChecksum string              `json:"sourceFileChecksum,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
}

// AppEncryptionDeclarationDocumentResponse is the response for document endpoints.
type AppEncryptionDeclarationDocumentResponse = SingleResponse[AppEncryptionDeclarationDocumentAttributes]

// AppEncryptionDeclarationDocumentCreateAttributes describes create attributes.
type AppEncryptionDeclarationDocumentCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// AppEncryptionDeclarationDocumentRelationships describes document relationships.
type AppEncryptionDeclarationDocumentRelationships struct {
	AppEncryptionDeclaration *Relationship `json:"appEncryptionDeclaration"`
}

// AppEncryptionDeclarationDocumentCreateData is the data portion of a create request.
type AppEncryptionDeclarationDocumentCreateData struct {
	Type          ResourceType                                     `json:"type"`
	Attributes    AppEncryptionDeclarationDocumentCreateAttributes `json:"attributes"`
	Relationships *AppEncryptionDeclarationDocumentRelationships   `json:"relationships"`
}

// AppEncryptionDeclarationDocumentCreateRequest is a request to create a document.
type AppEncryptionDeclarationDocumentCreateRequest struct {
	Data AppEncryptionDeclarationDocumentCreateData `json:"data"`
}

// AppEncryptionDeclarationDocumentUpdateAttributes describes update attributes.
type AppEncryptionDeclarationDocumentUpdateAttributes struct {
	SourceFileChecksum *string `json:"sourceFileChecksum,omitempty"`
	Uploaded           *bool   `json:"uploaded,omitempty"`
}

// AppEncryptionDeclarationDocumentUpdateData is the data portion of an update request.
type AppEncryptionDeclarationDocumentUpdateData struct {
	Type       ResourceType                                      `json:"type"`
	ID         string                                            `json:"id"`
	Attributes *AppEncryptionDeclarationDocumentUpdateAttributes `json:"attributes,omitempty"`
}

// AppEncryptionDeclarationDocumentUpdateRequest is a request to update a document.
type AppEncryptionDeclarationDocumentUpdateRequest struct {
	Data AppEncryptionDeclarationDocumentUpdateData `json:"data"`
}

// GetAppEncryptionDeclarations retrieves encryption declarations for an app.
func (c *Client) GetAppEncryptionDeclarations(ctx context.Context, appID string, opts ...AppEncryptionDeclarationsOption) (*AppEncryptionDeclarationsResponse, error) {
	query := &appEncryptionDeclarationsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	query.appID = strings.TrimSpace(appID)

	path := "/v1/appEncryptionDeclarations"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("app-encryption-declarations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppEncryptionDeclarationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEncryptionDeclarationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEncryptionDeclaration retrieves an encryption declaration by ID.
func (c *Client) GetAppEncryptionDeclaration(ctx context.Context, declarationID string, opts ...AppEncryptionDeclarationsOption) (*AppEncryptionDeclarationResponse, error) {
	declarationID = strings.TrimSpace(declarationID)
	if declarationID == "" {
		return nil, fmt.Errorf("declarationID is required")
	}

	query := &appEncryptionDeclarationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/appEncryptionDeclarations/%s", declarationID)
	if queryString := buildAppEncryptionDeclarationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEncryptionDeclarationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppEncryptionDeclaration creates a new encryption declaration.
func (c *Client) CreateAppEncryptionDeclaration(ctx context.Context, appID string, attrs AppEncryptionDeclarationCreateAttributes) (*AppEncryptionDeclarationResponse, error) {
	payload := AppEncryptionDeclarationCreateRequest{
		Data: AppEncryptionDeclarationCreateData{
			Type:       ResourceTypeAppEncryptionDeclarations,
			Attributes: attrs,
			Relationships: &AppEncryptionDeclarationRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   strings.TrimSpace(appID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appEncryptionDeclarations", body)
	if err != nil {
		return nil, err
	}

	var response AppEncryptionDeclarationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEncryptionDeclarationDocument retrieves a document by ID.
func (c *Client) GetAppEncryptionDeclarationDocument(ctx context.Context, documentID string, fields []string) (*AppEncryptionDeclarationDocumentResponse, error) {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil, fmt.Errorf("documentID is required")
	}

	path := fmt.Sprintf("/v1/appEncryptionDeclarationDocuments/%s", documentID)
	if queryString := buildAppEncryptionDeclarationDocumentFieldsQuery(fields); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEncryptionDeclarationDocumentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppEncryptionDeclarationDocument creates a new document upload reservation.
func (c *Client) CreateAppEncryptionDeclarationDocument(ctx context.Context, declarationID, fileName string, fileSize int64) (*AppEncryptionDeclarationDocumentResponse, error) {
	declarationID = strings.TrimSpace(declarationID)
	fileName = strings.TrimSpace(fileName)
	if declarationID == "" {
		return nil, fmt.Errorf("declarationID is required")
	}
	if fileName == "" {
		return nil, fmt.Errorf("fileName is required")
	}
	if fileSize <= 0 {
		return nil, fmt.Errorf("fileSize is required")
	}

	payload := AppEncryptionDeclarationDocumentCreateRequest{
		Data: AppEncryptionDeclarationDocumentCreateData{
			Type: ResourceTypeAppEncryptionDeclarationDocuments,
			Attributes: AppEncryptionDeclarationDocumentCreateAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: &AppEncryptionDeclarationDocumentRelationships{
				AppEncryptionDeclaration: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppEncryptionDeclarations,
						ID:   declarationID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appEncryptionDeclarationDocuments", body)
	if err != nil {
		return nil, err
	}

	var response AppEncryptionDeclarationDocumentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppEncryptionDeclarationDocument updates a document by ID.
func (c *Client) UpdateAppEncryptionDeclarationDocument(ctx context.Context, documentID string, attrs AppEncryptionDeclarationDocumentUpdateAttributes) (*AppEncryptionDeclarationDocumentResponse, error) {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil, fmt.Errorf("documentID is required")
	}

	payload := AppEncryptionDeclarationDocumentUpdateRequest{
		Data: AppEncryptionDeclarationDocumentUpdateData{
			Type: ResourceTypeAppEncryptionDeclarationDocuments,
			ID:   documentID,
		},
	}
	if attrs.SourceFileChecksum != nil || attrs.Uploaded != nil {
		payload.Data.Attributes = &attrs
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v1/appEncryptionDeclarationDocuments/%s", documentID), body)
	if err != nil {
		return nil, err
	}

	var response AppEncryptionDeclarationDocumentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
