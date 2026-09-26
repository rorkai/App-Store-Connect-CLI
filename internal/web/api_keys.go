package web

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const apiKeyTypePublic = "PUBLIC_API"

const (
	APIKeyKindTeam       = "team"
	APIKeyKindIndividual = "individual"
)

// ErrAPIKeyResponseInvalid reports a malformed or incomplete one-time P8 response.
var ErrAPIKeyResponseInvalid = errors.New("invalid api key download response")

// APIKeyCreateAttributes describes a team API key created through a web session.
type APIKeyCreateAttributes struct {
	Nickname string
	Role     string
}

// APIKey contains non-secret metadata for an App Store Connect API key.
type APIKey struct {
	KeyID          string   `json:"keyId"`
	Name           string   `json:"name,omitempty"`
	IssuerID       string   `json:"issuerId,omitempty"`
	Roles          []string `json:"roles,omitempty"`
	Active         bool     `json:"active"`
	AllAppsVisible bool     `json:"allAppsVisible"`
	CanDownload    bool     `json:"canDownload"`
	KeyType        string   `json:"keyType,omitempty"`
	LastUsed       string   `json:"lastUsed,omitempty"`
	RevokingDate   string   `json:"revokingDate,omitempty"`
}

// APIKeyListItem is non-secret metadata for one listed App Store Connect API key.
type APIKeyListItem struct {
	KeyID       string    `json:"keyId"`
	Name        string    `json:"name,omitempty"`
	Kind        string    `json:"kind"`
	Roles       []string  `json:"roles,omitempty"`
	Active      bool      `json:"active"`
	KeyType     string    `json:"keyType,omitempty"`
	LastUsed    string    `json:"lastUsed,omitempty"`
	GeneratedBy *KeyActor `json:"generatedBy,omitempty"`
	RevokedBy   *KeyActor `json:"revokedBy,omitempty"`
}

type apiKeyResource struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Attributes struct {
		Nickname       string   `json:"nickname"`
		Roles          []string `json:"roles"`
		AllAppsVisible bool     `json:"allAppsVisible"`
		CanDownload    bool     `json:"canDownload"`
		IsActive       bool     `json:"isActive"`
		KeyType        string   `json:"keyType"`
		LastUsed       string   `json:"lastUsed"`
		RevokingDate   string   `json:"revokingDate"`
		PrivateKey     string   `json:"privateKey"`
		Provider       *struct {
			ID string `json:"id"`
		} `json:"provider"`
	} `json:"attributes"`
	Relationships struct {
		Provider struct {
			Data *struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"provider"`
	} `json:"relationships"`
}

type apiKeyPayload struct {
	Data     apiKeyResource   `json:"data"`
	Included []apiKeyResource `json:"included"`
}

// CreateAPIKey creates an all-apps team API key using the selected web session.
func (c *Client) CreateAPIKey(ctx context.Context, attrs APIKeyCreateAttributes) (*APIKey, error) {
	nickname := strings.TrimSpace(attrs.Nickname)
	if nickname == "" {
		return nil, fmt.Errorf("api key nickname is required")
	}
	role := strings.ToUpper(strings.TrimSpace(attrs.Role))
	if role == "" {
		return nil, fmt.Errorf("api key role is required")
	}

	request := map[string]any{
		"data": map[string]any{
			"type": "apiKeys",
			"attributes": map[string]any{
				"nickname":       nickname,
				"roles":          []string{role},
				"allAppsVisible": true,
				"keyType":        apiKeyTypePublic,
			},
		},
	}
	body, err := c.doIrisV1Request(ctx, http.MethodPost, "/apiKeys", request)
	if err != nil {
		return nil, err
	}
	return parseAPIKeyResponse(body, "create api key")
}

// GetAPIKey returns API key metadata, including its issuer/provider ID.
func (c *Client) GetAPIKey(ctx context.Context, keyID string) (*APIKey, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, fmt.Errorf("api key id is required")
	}
	path := "/apiKeys/" + url.PathEscape(keyID) + "?include=provider"
	body, err := c.doIrisV1Request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return parseAPIKeyResponse(body, "get api key")
}

// ListAPIKeys returns team and individual API keys visible to the web session.
// Team keys come from the iris v1 integrations list; individual keys come from
// iris v2. Both readers already follow pagination links, so this method returns
// the complete visible set. Creation date is not present on either payload.
func (c *Client) ListAPIKeys(ctx context.Context) ([]APIKeyListItem, error) {
	teamKeys, teamErr := c.listTeamKeys(ctx)
	if teamErr != nil && !shouldFallbackToIndividualKeys(teamErr) {
		return nil, teamErr
	}

	individualKeys, individualErr := c.listIndividualKeys(ctx)
	if individualErr != nil && !shouldFallbackToIndividualKeys(individualErr) {
		return nil, individualErr
	}
	if teamErr != nil && individualErr != nil {
		return nil, teamErr
	}

	nTeam, nIndividual := 0, 0
	if teamErr == nil {
		nTeam = len(teamKeys)
	}
	if individualErr == nil {
		nIndividual = len(individualKeys)
	}
	items := make([]APIKeyListItem, 0, nTeam+nIndividual)
	if teamErr == nil {
		items = append(items, teamAPIKeyListItems(teamKeys)...)
	}
	if individualErr == nil {
		items = append(items, individualAPIKeyListItems(individualKeys)...)
	}
	return items, nil
}

// ListAPIKeysByKind returns only the requested API-key family. Unlike
// ListAPIKeys, it deliberately does not query the other family; callers use
// this boundary before and after a destructive operation to verify the exact
// resource kind they selected.
func (c *Client) ListAPIKeysByKind(ctx context.Context, kind string) ([]APIKeyListItem, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case APIKeyKindTeam:
		keys, err := c.listTeamKeys(ctx)
		if err != nil {
			return nil, err
		}
		return teamAPIKeyListItems(keys), nil
	case APIKeyKindIndividual:
		keys, err := c.listIndividualKeys(ctx)
		if err != nil {
			return nil, err
		}
		return individualAPIKeyListItems(keys), nil
	default:
		return nil, fmt.Errorf("api key type must be team or individual")
	}
}

func teamAPIKeyListItems(keys []teamAPIKey) []APIKeyListItem {
	items := make([]APIKeyListItem, 0, len(keys))
	for _, key := range keys {
		items = append(items, APIKeyListItem{
			KeyID:       key.KeyID,
			Name:        key.Name,
			Kind:        APIKeyKindTeam,
			Roles:       append([]string(nil), key.Roles...),
			Active:      key.Active,
			KeyType:     key.KeyType,
			LastUsed:    key.LastUsed,
			GeneratedBy: cloneKeyActor(key.GeneratedBy),
			RevokedBy:   cloneKeyActor(key.RevokedBy),
		})
	}
	return items
}

func individualAPIKeyListItems(keys []individualAPIKey) []APIKeyListItem {
	items := make([]APIKeyListItem, 0, len(keys))
	for _, key := range keys {
		item := APIKeyListItem{
			KeyID:    key.KeyID,
			Name:     key.Name,
			Kind:     APIKeyKindIndividual,
			Roles:    append([]string(nil), key.Roles...),
			Active:   key.Active,
			KeyType:  key.KeyType,
			LastUsed: key.LastUsed,
		}
		if key.CreatedByActorID != "" {
			item.GeneratedBy = &KeyActor{ID: key.CreatedByActorID}
		}
		if key.RevokedByActorID != "" {
			item.RevokedBy = &KeyActor{ID: key.RevokedByActorID}
		}
		items = append(items, item)
	}
	return items
}

// RevokeAPIKey marks one team or individual API key inactive using the
// type-specific Iris web resource. The caller must preflight and verify the
// key through ListAPIKeysByKind.
func (c *Client) RevokeAPIKey(ctx context.Context, keyID, kind string) error {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return fmt.Errorf("api key id is required")
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != APIKeyKindTeam && kind != APIKeyKindIndividual {
		return fmt.Errorf("api key type must be team or individual")
	}

	request := map[string]any{
		"data": map[string]any{
			"id":   keyID,
			"type": "apiKeys",
			"attributes": map[string]any{
				"isActive": false,
			},
		},
	}
	path := "/apiKeys/" + url.PathEscape(keyID)
	if kind == APIKeyKindTeam {
		_, err := c.doIrisV1Request(ctx, http.MethodPatch, path, request)
		return err
	}
	_, err := c.doIrisV2Request(ctx, http.MethodPatch, path, request)
	return err
}

// DownloadAPIKey downloads and decodes the one-time P8 for an API key.
func (c *Client) DownloadAPIKey(ctx context.Context, keyID string) ([]byte, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, fmt.Errorf("api key id is required")
	}
	query := url.Values{}
	query.Set("fields[apiKeys]", "privateKey")
	path := "/apiKeys/" + url.PathEscape(keyID) + "?" + query.Encode()
	body, err := c.doIrisV1Request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var payload apiKeyPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: failed to parse JSON: %w", ErrAPIKeyResponseInvalid, err)
	}
	if strings.TrimSpace(payload.Data.ID) != keyID {
		return nil, fmt.Errorf("%w: response resource id did not match the created key", ErrAPIKeyResponseInvalid)
	}
	encoded := strings.TrimSpace(payload.Data.Attributes.PrivateKey)
	if encoded == "" {
		return nil, fmt.Errorf("%w: response did not include a P8", ErrAPIKeyResponseInvalid)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode P8: %w", ErrAPIKeyResponseInvalid, err)
	}
	if err := validateAPIKeyP8(decoded); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(decoded), nil
}

func validateAPIKeyP8(decoded []byte) error {
	trimmed := bytes.TrimSpace(decoded)
	if !bytes.HasPrefix(trimmed, []byte("-----BEGIN ")) {
		return fmt.Errorf("%w: response contained an invalid P8", ErrAPIKeyResponseInvalid)
	}
	block, rest := pem.Decode(trimmed)
	if block == nil {
		return fmt.Errorf("%w: response contained an invalid P8", ErrAPIKeyResponseInvalid)
	}
	if block.Type != "PRIVATE KEY" {
		return fmt.Errorf("%w: response P8 is not PKCS#8", ErrAPIKeyResponseInvalid)
	}
	if len(bytes.TrimSpace(rest)) > 0 {
		return fmt.Errorf("%w: response contained extra PEM data", ErrAPIKeyResponseInvalid)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("%w: response P8 is not a usable PKCS#8 private key", ErrAPIKeyResponseInvalid)
	}
	ecKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || ecKey.Curve != elliptic.P256() {
		return fmt.Errorf("%w: response P8 is not a P-256 EC private key", ErrAPIKeyResponseInvalid)
	}
	return nil
}

// IsAPIKeyDownloadRetryable reports whether a newly created key download may
// be retried because Apple's API-key resource is still propagating. Transport
// and body-read failures are not retried: those can occur after the one-time
// P8 has already been served.
func IsAPIKeyDownloadRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrAPIKeyResponseInvalid) {
		return false
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		// A dropped connection or a failed body read can happen after Apple
		// already served the one-time P8. Only a classified API error is proof
		// that the download did not complete.
		return false
	}
	switch apiErr.Status {
	case http.StatusNotFound, http.StatusConflict, http.StatusTooManyRequests:
		return true
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusBadRequest:
		return false
	default:
		return apiErr.Status >= http.StatusInternalServerError
	}
}

func parseAPIKeyResponse(body []byte, operation string) (*APIKey, error) {
	var payload apiKeyPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse %s response: %w", operation, err)
	}
	if strings.TrimSpace(payload.Data.ID) == "" {
		return nil, fmt.Errorf("%s response did not include a key id", operation)
	}

	issuerID := ""
	if payload.Data.Attributes.Provider != nil {
		issuerID = strings.TrimSpace(payload.Data.Attributes.Provider.ID)
	}
	if issuerID == "" && payload.Data.Relationships.Provider.Data != nil {
		issuerID = strings.TrimSpace(payload.Data.Relationships.Provider.Data.ID)
	}

	return &APIKey{
		KeyID:          strings.TrimSpace(payload.Data.ID),
		Name:           strings.TrimSpace(payload.Data.Attributes.Nickname),
		IssuerID:       issuerID,
		Roles:          append([]string(nil), payload.Data.Attributes.Roles...),
		Active:         payload.Data.Attributes.IsActive,
		AllAppsVisible: payload.Data.Attributes.AllAppsVisible,
		CanDownload:    payload.Data.Attributes.CanDownload,
		KeyType:        strings.TrimSpace(payload.Data.Attributes.KeyType),
		LastUsed:       strings.TrimSpace(payload.Data.Attributes.LastUsed),
		RevokingDate:   strings.TrimSpace(payload.Data.Attributes.RevokingDate),
	}, nil
}

func cloneKeyActor(actor *KeyActor) *KeyActor {
	if actor == nil {
		return nil
	}
	cloned := *actor
	return &cloned
}
