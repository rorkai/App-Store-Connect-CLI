package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// CreateAccessibilityDeclaration creates a new accessibility declaration.
func (c *Client) CreateAccessibilityDeclaration(ctx context.Context, appID string, attrs AccessibilityDeclarationCreateAttributes) (*AccessibilityDeclarationResponse, error) {
	payload := AccessibilityDeclarationCreateRequest{
		Data: AccessibilityDeclarationCreateData{
			Type:       ResourceTypeAccessibilityDeclarations,
			Attributes: attrs,
			Relationships: &AccessibilityDeclarationRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   strings.TrimSpace(appID),
					},
				},
			},
		},
	}

	return sendJSON[AccessibilityDeclarationResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/accessibilityDeclarations", Payload: payload})
}

// UpdateAccessibilityDeclaration updates an accessibility declaration by ID.
func (c *Client) UpdateAccessibilityDeclaration(ctx context.Context, declarationID string, attrs AccessibilityDeclarationUpdateAttributes) (*AccessibilityDeclarationResponse, error) {
	declarationID = strings.TrimSpace(declarationID)
	payload := AccessibilityDeclarationUpdateRequest{
		Data: AccessibilityDeclarationUpdateData{
			Type: ResourceTypeAccessibilityDeclarations,
			ID:   declarationID,
		},
	}
	if HasAccessibilityDeclarationUpdates(attrs) {
		payload.Data.Attributes = &attrs
	}

	return sendJSON[AccessibilityDeclarationResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/accessibilityDeclarations/%s", declarationID), Payload: payload})
}

// HasAccessibilityDeclarationUpdates reports whether any update attributes are set.
func HasAccessibilityDeclarationUpdates(attrs AccessibilityDeclarationUpdateAttributes) bool {
	return attrs.Publish != nil ||
		attrs.SupportsAudioDescriptions != nil ||
		attrs.SupportsCaptions != nil ||
		attrs.SupportsDarkInterface != nil ||
		attrs.SupportsDifferentiateWithoutColorAlone != nil ||
		attrs.SupportsLargerText != nil ||
		attrs.SupportsReducedMotion != nil ||
		attrs.SupportsSufficientContrast != nil ||
		attrs.SupportsVoiceControl != nil ||
		attrs.SupportsVoiceover != nil
}
