package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// CiTestDestinationKind represents the kind of test destination.
type CiTestDestinationKind string

const (
	CiTestDestinationKindSimulator CiTestDestinationKind = "SIMULATOR"
	CiTestDestinationKindMac       CiTestDestinationKind = "MAC"
)

// CiMacOsVersionAttributes describes a CI macOS version resource.
type CiMacOsVersionAttributes struct {
	Version string `json:"version,omitempty"`
	Name    string `json:"name,omitempty"`
}

// CiMacOsVersionRelationships describes relationships for a CI macOS version.
type CiMacOsVersionRelationships struct {
	XcodeVersions *RelationshipList `json:"xcodeVersions,omitempty"`
}

// CiMacOsVersionResource represents a CI macOS version resource.
type CiMacOsVersionResource struct {
	Type          ResourceType                 `json:"type"`
	ID            string                       `json:"id"`
	Attributes    CiMacOsVersionAttributes     `json:"attributes"`
	Relationships *CiMacOsVersionRelationships `json:"relationships,omitempty"`
}

// CiMacOsVersionsResponse is the response from CI macOS versions endpoints.
type CiMacOsVersionsResponse struct {
	Data     []CiMacOsVersionResource `json:"data"`
	Included []CiXcodeVersionResource `json:"included,omitempty"`
	Links    Links                    `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *CiMacOsVersionsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *CiMacOsVersionsResponse) GetData() any {
	return r.Data
}

// CiMacOsVersionResponse is the response from CI macOS version detail endpoints.
type CiMacOsVersionResponse struct {
	Data     CiMacOsVersionResource   `json:"data"`
	Included []CiXcodeVersionResource `json:"included,omitempty"`
	Links    Links                    `json:"links"`
}

// CiTestDestinationRuntime describes an available runtime for a test destination.
type CiTestDestinationRuntime struct {
	RuntimeName       string `json:"runtimeName,omitempty"`
	RuntimeIdentifier string `json:"runtimeIdentifier,omitempty"`
}

// CiTestDestination describes a test destination for an Xcode version.
type CiTestDestination struct {
	DeviceTypeName       string                     `json:"deviceTypeName,omitempty"`
	DeviceTypeIdentifier string                     `json:"deviceTypeIdentifier,omitempty"`
	AvailableRuntimes    []CiTestDestinationRuntime `json:"availableRuntimes,omitempty"`
	Kind                 CiTestDestinationKind      `json:"kind,omitempty"`
}

// CiXcodeVersionAttributes describes a CI Xcode version resource.
type CiXcodeVersionAttributes struct {
	Version          string              `json:"version,omitempty"`
	Name             string              `json:"name,omitempty"`
	TestDestinations []CiTestDestination `json:"testDestinations,omitempty"`
}

// CiXcodeVersionRelationships describes relationships for a CI Xcode version.
type CiXcodeVersionRelationships struct {
	MacOsVersions *RelationshipList `json:"macOsVersions,omitempty"`
}

// CiXcodeVersionResource represents a CI Xcode version resource.
type CiXcodeVersionResource struct {
	Type          ResourceType                 `json:"type"`
	ID            string                       `json:"id"`
	Attributes    CiXcodeVersionAttributes     `json:"attributes"`
	Relationships *CiXcodeVersionRelationships `json:"relationships,omitempty"`
}

// CiXcodeVersionsResponse is the response from CI Xcode versions endpoints.
type CiXcodeVersionsResponse struct {
	Data     []CiXcodeVersionResource `json:"data"`
	Included []CiMacOsVersionResource `json:"included,omitempty"`
	Links    Links                    `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *CiXcodeVersionsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *CiXcodeVersionsResponse) GetData() any {
	return r.Data
}

// CiXcodeVersionResponse is the response from CI Xcode version detail endpoints.
type CiXcodeVersionResponse struct {
	Data     CiXcodeVersionResource   `json:"data"`
	Included []CiMacOsVersionResource `json:"included,omitempty"`
	Links    Links                    `json:"links"`
}

type ciMacOsVersionsQuery struct {
	listQuery
}

// CiMacOsVersionsOption is a functional option for GetCiMacOsVersions.
type CiMacOsVersionsOption func(*ciMacOsVersionsQuery)

// WithCiMacOsVersionsLimit sets the max number of macOS versions to return.
func WithCiMacOsVersionsLimit(limit int) CiMacOsVersionsOption {
	return func(q *ciMacOsVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiMacOsVersionsNextURL uses a next page URL directly.
func WithCiMacOsVersionsNextURL(next string) CiMacOsVersionsOption {
	return func(q *ciMacOsVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildCiMacOsVersionsQuery(query *ciMacOsVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

type ciXcodeVersionsQuery struct {
	listQuery
}

// CiXcodeVersionsOption is a functional option for GetCiXcodeVersions.
type CiXcodeVersionsOption func(*ciXcodeVersionsQuery)

// WithCiXcodeVersionsLimit sets the max number of Xcode versions to return.
func WithCiXcodeVersionsLimit(limit int) CiXcodeVersionsOption {
	return func(q *ciXcodeVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiXcodeVersionsNextURL uses a next page URL directly.
func WithCiXcodeVersionsNextURL(next string) CiXcodeVersionsOption {
	return func(q *ciXcodeVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildCiXcodeVersionsQuery(query *ciXcodeVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GetCiMacOsVersions retrieves the list of CI macOS versions.
func (c *Client) GetCiMacOsVersions(ctx context.Context, opts ...CiMacOsVersionsOption) (*CiMacOsVersionsResponse, error) {
	query := &ciMacOsVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/ciMacOsVersions"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciMacOsVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiMacOsVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiMacOsVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiMacOsVersion retrieves a CI macOS version by ID.
func (c *Client) GetCiMacOsVersion(ctx context.Context, macOsVersionID string) (*CiMacOsVersionResponse, error) {
	path := fmt.Sprintf("/v1/ciMacOsVersions/%s", macOsVersionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiMacOsVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiMacOsVersionXcodeVersions retrieves Xcode versions for a macOS version.
func (c *Client) GetCiMacOsVersionXcodeVersions(ctx context.Context, macOsVersionID string, opts ...CiXcodeVersionsOption) (*CiXcodeVersionsResponse, error) {
	query := &ciXcodeVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciMacOsVersions/%s/xcodeVersions", macOsVersionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciMacOsVersionXcodeVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiXcodeVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiXcodeVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiXcodeVersions retrieves the list of CI Xcode versions.
func (c *Client) GetCiXcodeVersions(ctx context.Context, opts ...CiXcodeVersionsOption) (*CiXcodeVersionsResponse, error) {
	query := &ciXcodeVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/ciXcodeVersions"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciXcodeVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiXcodeVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiXcodeVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiXcodeVersion retrieves a CI Xcode version by ID.
func (c *Client) GetCiXcodeVersion(ctx context.Context, xcodeVersionID string) (*CiXcodeVersionResponse, error) {
	path := fmt.Sprintf("/v1/ciXcodeVersions/%s", xcodeVersionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiXcodeVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiXcodeVersionMacOsVersions retrieves macOS versions for an Xcode version.
func (c *Client) GetCiXcodeVersionMacOsVersions(ctx context.Context, xcodeVersionID string, opts ...CiMacOsVersionsOption) (*CiMacOsVersionsResponse, error) {
	query := &ciMacOsVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciXcodeVersions/%s/macOsVersions", xcodeVersionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciXcodeVersionMacOsVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiMacOsVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiMacOsVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
