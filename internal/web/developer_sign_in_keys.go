package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	signInKeyService = "APPLE_ID_AUTH_KEY_CONFIGURATION"
	pushKeyService   = "U27F4V844T"
)

var developerKeyIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{1,32}$`)

type DeveloperSignInKey struct {
	Raw         json.RawMessage `json:"-"`
	KeyID       string          `json:"keyId"`
	KeyName     string          `json:"keyName"`
	CanDownload bool            `json:"canDownload"`
	CanRevoke   bool            `json:"canRevoke"`
	Services    []struct {
		ID             string            `json:"id"`
		Configurations []json.RawMessage `json:"configurations"`
	} `json:"services,omitempty"`
}

type DeveloperSignInKeysResult struct {
	Keys []DeveloperSignInKey `json:"keys"`
	Raw  json.RawMessage      `json:"-"`
}

func (r *DeveloperSignInKeysResult) MarshalJSON() ([]byte, error) { return r.Raw, nil }

// developerSignInKeyDownloadRecoveryError retains a successful response that
// failed P8 validation. Apple may consume a one-time download even when it
// returns a malformed body, so callers need a private recovery path without
// exposing the response through the error text.
type developerSignInKeyDownloadRecoveryError struct {
	cause error
	body  []byte
}

func (e *developerSignInKeyDownloadRecoveryError) Error() string { return e.cause.Error() }

func (e *developerSignInKeyDownloadRecoveryError) Unwrap() error { return e.cause }

func (e *developerSignInKeyDownloadRecoveryError) RecoveryBody() []byte {
	return append([]byte(nil), e.body...)
}

func parseDeveloperSignInKeys(body []byte) (*DeveloperSignInKeysResult, error) {
	var envelope struct {
		ResultCode *int                 `json:"resultCode"`
		Keys       []DeveloperSignInKey `json:"keys"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("invalid Developer Portal key response")
	}
	if envelope.ResultCode == nil {
		return nil, fmt.Errorf("developer portal key response missing result code")
	}
	if *envelope.ResultCode != 0 {
		return nil, fmt.Errorf("developer portal key request failed (result code %d)", *envelope.ResultCode)
	}
	if envelope.Keys == nil {
		return nil, fmt.Errorf("developer portal key response missing keys")
	}
	for _, key := range envelope.Keys {
		if !developerKeyIDPattern.MatchString(key.KeyID) {
			return nil, fmt.Errorf("developer portal returned invalid key ID")
		}
	}
	return &DeveloperSignInKeysResult{Keys: envelope.Keys, Raw: body}, nil
}

// ListDeveloperSignInKeys preserves the legacy portal envelope, including metadata
// for other authentication-key services. List is also Apple's key-family CSRF bootstrap.
func (c *Client) ListDeveloperSignInKeys(ctx context.Context) (*DeveloperSignInKeysResult, error) {
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	body, err := c.doDeveloperPortalLegacyFormRequest(ctx, "/account/auth/key/list", url.Values{
		"teamId": {c.developerPortalTeamID()}, "pageNumber": {"1"}, "pageSize": {"1000"}, "sort": {"name=asc"},
	}, false)
	if err != nil {
		return nil, err
	}
	return parseDeveloperSignInKeys(body)
}

func (c *Client) GetDeveloperSignInKey(ctx context.Context, keyID string) (*DeveloperSignInKey, error) {
	if !developerKeyIDPattern.MatchString(keyID) {
		return nil, fmt.Errorf("invalid key ID")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	body, err := c.doDeveloperPortalLegacyFormRequest(ctx, "/account/auth/key/get", url.Values{"teamId": {c.developerPortalTeamID()}, "keyId": {keyID}}, false)
	if err != nil {
		return nil, err
	}
	result, err := parseDeveloperSignInKeys(body)
	if err != nil {
		return nil, err
	}
	if len(result.Keys) != 1 || result.Keys[0].KeyID != keyID {
		return nil, fmt.Errorf("developer portal returned mismatched key")
	}
	result.Keys[0].Raw = result.Raw
	return &result.Keys[0], nil
}

func (c *Client) CreateDeveloperSignInKey(ctx context.Context, name, bundleID string) (*DeveloperSignInKey, error) {
	if !developerKeyIDPattern.MatchString(bundleID) {
		return nil, fmt.Errorf("name and opaque bundle resource ID are required")
	}
	request := map[string]any{"isNew": true, "serviceId": signInKeyService, "identifiers": map[string]any{"bundle": []string{bundleID}}}
	detail, err := c.createDeveloperKey(ctx, name, request)
	if err != nil {
		return nil, err
	}
	for _, service := range detail.Services {
		if service.ID != signInKeyService {
			continue
		}
		for _, raw := range service.Configurations {
			var config struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			}
			if json.Unmarshal(raw, &config) == nil && config.ID == bundleID && config.Type == "bundle" {
				return detail, nil
			}
		}
	}
	return nil, fmt.Errorf("key %s created, but requested Sign in with Apple bundle association was not verified; inspect before retrying", detail.KeyID)
}

// CreateDeveloperPushKey creates a team-scoped APNs key for sandbox and production.
func (c *Client) CreateDeveloperPushKey(ctx context.Context, name string) (*DeveloperSignInKey, error) {
	request := map[string]any{"isNew": true, "serviceId": pushKeyService, "identifiers": map[string]any{}, "environment": "all", "scope": "team"}
	detail, err := c.createDeveloperKey(ctx, name, request)
	if err != nil {
		return nil, err
	}
	if !detail.hasService(pushKeyService) {
		return nil, fmt.Errorf("key %s created, but the APNs service was not verified; inspect before retrying", detail.KeyID)
	}
	return detail, nil
}

func (c *Client) createDeveloperKey(ctx context.Context, name string, serviceRequest map[string]any) (*DeveloperSignInKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("key name is required")
	}
	existing, err := c.ListDeveloperSignInKeys(ctx)
	if err != nil {
		return nil, err
	}
	for _, key := range existing.Keys {
		if key.KeyName == name {
			return nil, fmt.Errorf("key %s already has this name; inspect it instead of creating a duplicate", key.KeyID)
		}
	}
	payload := map[string]any{"teamId": c.developerPortalTeamID(), "name": name, "serviceConfigurationsRequests": []any{serviceRequest}}
	headers := developerPortalHeaders("")
	headers.Set("Content-Type", "application/json")
	if err := c.applyDeveloperPortalCSRF(headers, true); err != nil {
		return nil, err
	}
	body, err := c.doDeveloperPortalHTTPAndCapture(ctx, http.MethodPost, c.developerPortalOrigin()+developerPortalLegacyPath+"/account/auth/key/v2/create", payload, headers)
	if err != nil {
		return nil, fmt.Errorf("key creation outcome is unknown; inspect sign-in-keys list before retrying: %w", err)
	}
	result, err := parseDeveloperSignInKeys(body)
	if err != nil {
		return nil, fmt.Errorf("inspect sign-in-keys list before retrying: %w", err)
	}
	if len(result.Keys) != 1 {
		return nil, fmt.Errorf("key creation returned unexpected key count; inspect list before retrying")
	}
	keyID := result.Keys[0].KeyID
	detail, err := c.GetDeveloperSignInKey(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("key %s created, but configuration verification failed; inspect before retrying: %w", keyID, err)
	}
	return detail, nil
}

func (k *DeveloperSignInKey) hasService(serviceID string) bool {
	for _, service := range k.Services {
		if service.ID == serviceID {
			return true
		}
	}
	return false
}

func (c *Client) DownloadDeveloperSignInKey(ctx context.Context, keyID string) ([]byte, error) {
	return c.downloadDeveloperKey(ctx, keyID, signInKeyService, "Sign in with Apple")
}

func (c *Client) DownloadDeveloperPushKey(ctx context.Context, keyID string) ([]byte, error) {
	return c.downloadDeveloperKey(ctx, keyID, pushKeyService, "APNs")
}

func (c *Client) downloadDeveloperKey(ctx context.Context, keyID, serviceID, label string) ([]byte, error) {
	if _, err := c.ListDeveloperSignInKeys(ctx); err != nil {
		return nil, err
	}
	key, err := c.GetDeveloperSignInKey(ctx, keyID)
	if err != nil {
		return nil, err
	}
	if !key.hasService(serviceID) {
		return nil, fmt.Errorf("key %s is not a %s key", keyID, label)
	}
	if !key.CanDownload {
		return nil, fmt.Errorf("key %s is no longer downloadable", keyID)
	}
	query := url.Values{"teamId": {c.developerPortalTeamID()}, "keyId": {keyID}}
	headers := developerPortalHeaders("")
	headers.Del("Content-Type")
	headers.Set("Accept", "application/json, text/plain, */*")
	if err := c.applyDeveloperPortalCSRF(headers, true); err != nil {
		return nil, err
	}
	body, err := c.doDeveloperPortalHTTPAndCapture(ctx, http.MethodGet, c.developerPortalOrigin()+developerPortalLegacyPath+"/account/auth/key/download?"+query.Encode(), nil, headers)
	if err != nil {
		return nil, err
	}
	if err := validateAPIKeyP8(body); err != nil {
		return nil, &developerSignInKeyDownloadRecoveryError{cause: err, body: append([]byte(nil), body...)}
	}
	return body, nil
}

func (c *Client) RevokeDeveloperSignInKey(ctx context.Context, keyID string) error {
	return c.revokeDeveloperKey(ctx, keyID, signInKeyService, "Sign in with Apple")
}

func (c *Client) RevokeDeveloperPushKey(ctx context.Context, keyID string) error {
	return c.revokeDeveloperKey(ctx, keyID, pushKeyService, "APNs")
}

func (c *Client) revokeDeveloperKey(ctx context.Context, keyID, serviceID, label string) error {
	key, err := c.GetDeveloperSignInKey(ctx, keyID)
	if err != nil {
		return err
	}
	if !key.hasService(serviceID) {
		return fmt.Errorf("key %s is not a %s key", keyID, label)
	}
	if !key.CanRevoke {
		return fmt.Errorf("key %s cannot be revoked by this account", keyID)
	}
	if _, err := c.ListDeveloperSignInKeys(ctx); err != nil {
		return err
	}
	body, err := c.doDeveloperPortalLegacyFormRequest(ctx, "/account/auth/key/revoke", url.Values{"teamId": {c.developerPortalTeamID()}, "keyId": {keyID}}, true)
	if err != nil {
		return fmt.Errorf("key %s revoke outcome is unknown; inspect sign-in-keys list before retrying: %w", keyID, err)
	}
	var envelope struct {
		ResultCode *int `json:"resultCode"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.ResultCode == nil {
		return fmt.Errorf("key %s revoke returned an invalid response; inspect sign-in-keys list before retrying", keyID)
	}
	if *envelope.ResultCode != 0 {
		return fmt.Errorf("key %s revoke failed (result code %d)", keyID, *envelope.ResultCode)
	}
	remaining, err := c.ListDeveloperSignInKeys(ctx)
	if err != nil {
		return fmt.Errorf("key %s revoke was accepted, but verification failed; inspect sign-in-keys list: %w", keyID, err)
	}
	for _, key := range remaining.Keys {
		if key.KeyID == keyID {
			return fmt.Errorf("key %s revoke was accepted, but the key is still listed; inspect sign-in-keys list", keyID)
		}
	}
	return nil
}
