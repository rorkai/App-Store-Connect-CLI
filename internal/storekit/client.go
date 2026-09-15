package storekit

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

const (
	tokenLifetime = 20 * time.Minute
	// jwtIssuedAtSkew backdates the issued-at claim so a client clock running
	// ahead of Apple's does not produce a token rejected as issued in the future.
	jwtIssuedAtSkew = 60 * time.Second
)

type ClientOption func(*Client)

type Client struct {
	httpClient  *http.Client
	baseURL     string
	environment Environment
	credentials Credentials
	now         func() time.Time

	privateKeyMu sync.Mutex
	privateKey   *ecdsa.PrivateKey
}

func NewClient(credentials Credentials, environment Environment, opts ...ClientOption) (*Client, error) {
	credentials = normalizeCredentials(credentials)
	if err := validateCredentials(credentials); err != nil {
		return nil, err
	}
	if environment != Production && environment != Sandbox {
		return nil, fmt.Errorf("environment must be one of: production, sandbox")
	}
	client := &Client{
		httpClient:  &http.Client{Timeout: asc.ResolveTimeout()},
		baseURL:     environment.baseURL(),
		environment: environment,
		credentials: credentials,
		now:         time.Now,
	}
	for _, option := range opts {
		option(client)
	}
	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: asc.ResolveTimeout()}
	}
	client.baseURL = strings.TrimRight(strings.TrimSpace(client.baseURL), "/")
	if client.baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	if client.now == nil {
		client.now = time.Now
	}
	return client, nil
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) { client.httpClient = httpClient }
}

// WithBaseURL overrides the complete Retention Messaging API base URL.
// It exists for tests and Apple-compatible proxies.
func WithBaseURL(baseURL string) ClientOption {
	return func(client *Client) { client.baseURL = baseURL }
}

func WithNow(now func() time.Time) ClientOption {
	return func(client *Client) { client.now = now }
}

// Validate signs a token and parses the configured private key without making
// a network request.
func (c *Client) Validate() error {
	_, err := c.signedToken()
	return err
}

func normalizeCredentials(credentials Credentials) Credentials {
	credentials.KeyID = strings.TrimSpace(credentials.KeyID)
	credentials.IssuerID = strings.TrimSpace(credentials.IssuerID)
	credentials.PrivateKeyPath = strings.TrimSpace(credentials.PrivateKeyPath)
	credentials.PrivateKeyPEM = strings.TrimSpace(credentials.PrivateKeyPEM)
	credentials.BundleID = strings.TrimSpace(credentials.BundleID)
	credentials.Profile = strings.TrimSpace(credentials.Profile)
	return credentials
}

func validateCredentials(credentials Credentials) error {
	if credentials.KeyID == "" {
		return fmt.Errorf("key ID is required")
	}
	if credentials.IssuerID == "" {
		return fmt.Errorf("issuer ID is required")
	}
	if credentials.PrivateKeyPath == "" && credentials.PrivateKeyPEM == "" {
		return fmt.Errorf("private key is required")
	}
	if credentials.BundleID == "" {
		return fmt.Errorf("bundle ID is required")
	}
	return nil
}

func (c *Client) signedToken() (string, error) {
	key, err := c.loadPrivateKey()
	if err != nil {
		return "", err
	}
	now := c.now().UTC()
	claims := jwt.MapClaims{
		"iss": c.credentials.IssuerID,
		"iat": now.Add(-jwtIssuedAtSkew).Unix(),
		"exp": now.Add(tokenLifetime).Unix(),
		"aud": "appstoreconnect-v1",
		"bid": c.credentials.BundleID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = c.credentials.KeyID
	token.Header["typ"] = "JWT"
	signed, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("sign StoreKit token: %w", err)
	}
	return signed, nil
}

func (c *Client) loadPrivateKey() (*ecdsa.PrivateKey, error) {
	c.privateKeyMu.Lock()
	defer c.privateKeyMu.Unlock()
	if c.privateKey != nil {
		return c.privateKey, nil
	}
	var (
		key *ecdsa.PrivateKey
		err error
	)
	if c.credentials.PrivateKeyPEM != "" {
		key, err = auth.LoadPrivateKeyFromPEM([]byte(c.credentials.PrivateKeyPEM))
	} else {
		key, err = auth.LoadPrivateKey(c.credentials.PrivateKeyPath)
	}
	if err != nil {
		return nil, fmt.Errorf("load StoreKit private key: %w", err)
	}
	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("StoreKit private key must use the P-256 curve")
	}
	c.privateKey = key
	return key, nil
}

func (c *Client) request(ctx context.Context, method, path, contentType string, body []byte, response any) error {
	requestURL, err := url.Parse(c.baseURL + "/" + strings.TrimLeft(path, "/"))
	if err != nil {
		return fmt.Errorf("build StoreKit request URL: %w", err)
	}
	if err := readonly.Check(ctx, method, readonly.Target(requestURL.String())); err != nil {
		return err
	}
	token, err := c.signedToken()
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), reader)
	if err != nil {
		return fmt.Errorf("create StoreKit request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("StoreKit request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read StoreKit response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(responseBody, resp.StatusCode, resp.Header.Get("Retry-After"))
	}
	if response == nil || len(bytes.TrimSpace(responseBody)) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, response); err != nil {
		return fmt.Errorf("decode StoreKit response: %w", err)
	}
	return nil
}

func jsonBody(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode StoreKit request: %w", err)
	}
	return body, nil
}
