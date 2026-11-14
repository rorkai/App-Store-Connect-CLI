package asc

import (
	"context"
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

// AppEncryptionDeclarationBuildsUpdateResult represents CLI output for build assignments.
type AppEncryptionDeclarationBuildsUpdateResult struct {
	DeclarationID string   `json:"declarationId"`
	BuildIDs      []string `json:"buildIds"`
	Action        string   `json:"action"`
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

// AppEncryptionDeclarationDocumentCreateAttributes describes create attributes.
type AppEncryptionDeclarationDocumentCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// GetAppEncryptionDeclarations retrieves encryption declarations for an app.
func (c *Client) GetAppEncryptionDeclarations(ctx context.Context, appID string, opts ...AppEncryptionDeclarationsOption) (*AppEncryptionDeclarationsResponse, error) {
	query := applyOptions(opts)
	query.appID = strings.TrimSpace(appID)
	return getJSON[AppEncryptionDeclarationsResponse](ctx, c, readRequest{
		Path:        "/v1/appEncryptionDeclarations",
		Query:       buildAppEncryptionDeclarationsQuery(query),
		Next:        query.nextURL,
		NextContext: "app-encryption-declarations",
	})
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

	return sendJSON[AppEncryptionDeclarationDocumentResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/appEncryptionDeclarationDocuments", Payload: payload})
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

	return sendJSON[AppEncryptionDeclarationDocumentResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/appEncryptionDeclarationDocuments/%s", documentID), Payload: payload})
}
