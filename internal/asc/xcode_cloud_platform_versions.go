package asc

import (
	"context"
	"fmt"
	"net/url"
)

// CiTestDestinationKind represents the kind of test destination.
type CiTestDestinationKind string

const (
	CiTestDestinationKindSimulator CiTestDestinationKind = "SIMULATOR"
	CiTestDestinationKindMac       CiTestDestinationKind = "MAC"
)

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

func buildCiXcodeVersionsQuery(query *ciXcodeVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GetCiMacOsVersion retrieves a CI macOS version by ID.
func (c *Client) GetCiMacOsVersion(ctx context.Context, macOsVersionID string) (*CiMacOsVersionResponse, error) {
	return getJSON[CiMacOsVersionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/ciMacOsVersions/%s", macOsVersionID)})
}

// GetCiMacOsVersionXcodeVersions retrieves Xcode versions for a macOS version.
func (c *Client) GetCiMacOsVersionXcodeVersions(ctx context.Context, macOsVersionID string, opts ...CiXcodeVersionsOption) (*CiXcodeVersionsResponse, error) {
	query := applyOptions(opts)

	return getJSON[CiXcodeVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/ciMacOsVersions/%s/xcodeVersions", macOsVersionID),
		Query:       buildCiXcodeVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "ciMacOsVersionXcodeVersions",
	})
}

// GetCiXcodeVersions retrieves the list of CI Xcode versions.
func (c *Client) GetCiXcodeVersions(ctx context.Context, opts ...CiXcodeVersionsOption) (*CiXcodeVersionsResponse, error) {
	query := applyOptions(opts)

	return getJSON[CiXcodeVersionsResponse](ctx, c, readRequest{
		Path:        "/v1/ciXcodeVersions",
		Query:       buildCiXcodeVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "ciXcodeVersions",
	})
}

// GetCiXcodeVersion retrieves a CI Xcode version by ID.
func (c *Client) GetCiXcodeVersion(ctx context.Context, xcodeVersionID string) (*CiXcodeVersionResponse, error) {
	return getJSON[CiXcodeVersionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/ciXcodeVersions/%s", xcodeVersionID)})
}

// GetCiXcodeVersionMacOsVersions retrieves macOS versions for an Xcode version.
func (c *Client) GetCiXcodeVersionMacOsVersions(ctx context.Context, xcodeVersionID string, opts ...CiMacOsVersionsOption) (*CiMacOsVersionsResponse, error) {
	query := applyOptions(opts)

	return getJSON[CiMacOsVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/ciXcodeVersions/%s/macOsVersions", xcodeVersionID),
		Query:       buildCiMacOsVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "ciXcodeVersionMacOsVersions",
	})
}
