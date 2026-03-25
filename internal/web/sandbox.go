package web

import (
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
)

// SandboxAccountCreateAttributes defines inputs for creating a sandbox tester
// via App Store Connect's private web session endpoints.
type SandboxAccountCreateAttributes struct {
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	AccountName     string `json:"acAccountName"`
	AccountPassword string `json:"acAccountPassword"`
	StoreFront      string `json:"storeFront"`
}

func normalizeSandboxAccountCreateAttributes(attrs SandboxAccountCreateAttributes) (SandboxAccountCreateAttributes, error) {
	attrs.FirstName = strings.TrimSpace(attrs.FirstName)
	attrs.LastName = strings.TrimSpace(attrs.LastName)
	attrs.AccountName = strings.TrimSpace(attrs.AccountName)
	attrs.AccountPassword = strings.TrimSpace(attrs.AccountPassword)
	attrs.StoreFront = strings.ToUpper(strings.TrimSpace(attrs.StoreFront))

	if attrs.FirstName == "" {
		return attrs, fmt.Errorf("first name is required")
	}
	if attrs.LastName == "" {
		return attrs, fmt.Errorf("last name is required")
	}
	if attrs.AccountName == "" {
		return attrs, fmt.Errorf("account name is required")
	}
	parsedAddress, err := mail.ParseAddress(attrs.AccountName)
	if err != nil || parsedAddress == nil || strings.TrimSpace(parsedAddress.Address) != attrs.AccountName {
		return attrs, fmt.Errorf("account name must be a valid email address")
	}
	if attrs.AccountPassword == "" {
		return attrs, fmt.Errorf("account password is required")
	}
	if attrs.StoreFront == "" {
		return attrs, fmt.Errorf("store front is required")
	}
	return attrs, nil
}

func sandboxOriginBaseURL(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return appStoreBaseURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return appStoreBaseURL
	}
	return parsed.Scheme + "://" + parsed.Host
}

func (c *Client) doSandboxRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	origin := sandboxOriginBaseURL(c.baseURL)

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "*/*")
	headers.Set("Origin", origin)
	headers.Set("Referer", origin+"/access/users/sandbox")

	return c.doRequestBase(ctx, origin, method, path, body, headers)
}

func (c *Client) validateSandboxAccountFields(ctx context.Context, body map[string]string) error {
	if _, err := c.doSandboxRequest(ctx, http.MethodPost, "/sandbox/v2/account/validateFields", body); err != nil {
		return err
	}
	return nil
}

// CreateSandboxAccount creates a sandbox tester by mirroring the current App
// Store Connect web flow: validate fields twice, then submit the create request.
func (c *Client) CreateSandboxAccount(ctx context.Context, attrs SandboxAccountCreateAttributes) error {
	normalized, err := normalizeSandboxAccountCreateAttributes(attrs)
	if err != nil {
		return err
	}

	validateBody := map[string]string{
		"firstName":     normalized.FirstName,
		"lastName":      normalized.LastName,
		"acAccountName": normalized.AccountName,
	}
	if err := c.validateSandboxAccountFields(ctx, validateBody); err != nil {
		return err
	}

	validateWithPasswordBody := map[string]string{
		"firstName":         normalized.FirstName,
		"lastName":          normalized.LastName,
		"acAccountName":     normalized.AccountName,
		"acAccountPassword": normalized.AccountPassword,
	}
	if err := c.validateSandboxAccountFields(ctx, validateWithPasswordBody); err != nil {
		return err
	}

	createBody := map[string]string{
		"firstName":         normalized.FirstName,
		"lastName":          normalized.LastName,
		"acAccountName":     normalized.AccountName,
		"acAccountPassword": normalized.AccountPassword,
		"storeFront":        normalized.StoreFront,
	}
	if _, err := c.doSandboxRequest(ctx, http.MethodPost, "/sandbox/v2/account/create", createBody); err != nil {
		return err
	}
	return nil
}
