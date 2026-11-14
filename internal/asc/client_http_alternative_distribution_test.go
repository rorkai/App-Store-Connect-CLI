package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestAlternativeDistributionReadEndpoints_SendsRequest(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		path     string
		response string
		call     func(*Client) error
	}{
		{
			name:     "GetAlternativeDistributionKey",
			path:     "/v1/alternativeDistributionKeys/key-1",
			response: `{"data":{"type":"alternativeDistributionKeys","id":"key-1"}}`,
			call: func(c *Client) error {
				_, err := c.GetAlternativeDistributionKey(ctx, "key-1")
				return err
			},
		},
		{
			name:     "GetAlternativeDistributionPackage",
			path:     "/v1/alternativeDistributionPackages/pkg-1",
			response: `{"data":{"type":"alternativeDistributionPackages","id":"pkg-1"}}`,
			call: func(c *Client) error {
				_, err := c.GetAlternativeDistributionPackage(ctx, "pkg-1")
				return err
			},
		},
		{
			name:     "GetAlternativeDistributionPackageVersion",
			path:     "/v1/alternativeDistributionPackageVersions/ver-1",
			response: `{"data":{"type":"alternativeDistributionPackageVersions","id":"ver-1"}}`,
			call: func(c *Client) error {
				_, err := c.GetAlternativeDistributionPackageVersion(ctx, "ver-1")
				return err
			},
		},
		{
			name:     "GetAlternativeDistributionPackageVariant",
			path:     "/v1/alternativeDistributionPackageVariants/var-1",
			response: `{"data":{"type":"alternativeDistributionPackageVariants","id":"var-1"}}`,
			call: func(c *Client) error {
				_, err := c.GetAlternativeDistributionPackageVariant(ctx, "var-1")
				return err
			},
		},
		{
			name:     "GetAlternativeDistributionPackageDelta",
			path:     "/v1/alternativeDistributionPackageDeltas/delta-1",
			response: `{"data":{"type":"alternativeDistributionPackageDeltas","id":"delta-1"}}`,
			call: func(c *Client) error {
				_, err := c.GetAlternativeDistributionPackageDelta(ctx, "delta-1")
				return err
			},
		},
		{
			name:     "GetAppAlternativeDistributionKey",
			path:     "/v1/apps/app-1/alternativeDistributionKey",
			response: `{"data":{"type":"alternativeDistributionKeys","id":"key-1"}}`,
			call: func(c *Client) error {
				_, err := c.GetAppAlternativeDistributionKey(ctx, "app-1")
				return err
			},
		},
		{
			name:     "GetAppStoreVersionAlternativeDistributionPackage",
			path:     "/v1/appStoreVersions/ver-1/alternativeDistributionPackage",
			response: `{"data":{"type":"alternativeDistributionPackages","id":"pkg-1"}}`,
			call: func(c *Client) error {
				_, err := c.GetAppStoreVersionAlternativeDistributionPackage(ctx, "ver-1")
				return err
			},
		},
		{
			name:     "GetAppStoreVersionAlternativeDistributionPackageRelationship",
			path:     "/v1/appStoreVersions/ver-1/relationships/alternativeDistributionPackage",
			response: `{"data":{"type":"alternativeDistributionPackages","id":"pkg-1"}}`,
			call: func(c *Client) error {
				_, err := c.GetAppStoreVersionAlternativeDistributionPackageRelationship(ctx, "ver-1")
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", req.Method)
				}
				if req.URL.Path != tt.path {
					t.Fatalf("expected path %s, got %s", tt.path, req.URL.Path)
				}
				assertAuthorized(t, req)
			}, jsonResponse(http.StatusOK, tt.response))

			if err := tt.call(client); err != nil {
				t.Fatalf("%s() error: %v", tt.name, err)
			}
		})
	}
}

func TestAlternativeDistributionListEndpoints_WithLimit(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		path     string
		limit    string
		response string
		call     func(*testing.T, *Client)
	}{
		{
			name:     "GetAlternativeDistributionKeys",
			path:     "/v1/alternativeDistributionKeys",
			limit:    "10",
			response: `{"data":[{"type":"alternativeDistributionKeys","id":"key-1","attributes":{"publicKey":"KEY"}}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetAlternativeDistributionKeys(ctx, Limit[AlternativeDistributionKeysOption](10))
				if err != nil {
					t.Fatalf("GetAlternativeDistributionKeys() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.PublicKey != "KEY" {
					t.Fatalf("expected decoded key response, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetAlternativeDistributionPackageVersions",
			path:     "/v1/alternativeDistributionPackages/pkg-1/versions",
			limit:    "2",
			response: `{"data":[{"type":"alternativeDistributionPackageVersions","id":"ver-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetAlternativeDistributionPackageVersions(ctx, "pkg-1", Limit[AlternativeDistributionPackageVersionsOption](2))
				if err != nil {
					t.Fatalf("GetAlternativeDistributionPackageVersions() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "ver-1" {
					t.Fatalf("expected decoded package version response, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetAlternativeDistributionPackageVersionDeltas",
			path:     "/v1/alternativeDistributionPackageVersions/ver-1/deltas",
			limit:    "4",
			response: `{"data":[{"type":"alternativeDistributionPackageDeltas","id":"delta-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetAlternativeDistributionPackageVersionDeltas(ctx, "ver-1", Limit[AlternativeDistributionPackageDeltasOption](4))
				if err != nil {
					t.Fatalf("GetAlternativeDistributionPackageVersionDeltas() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "delta-1" {
					t.Fatalf("expected decoded package delta response, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetAlternativeDistributionPackageVersionVariants",
			path:     "/v1/alternativeDistributionPackageVersions/ver-1/variants",
			limit:    "6",
			response: `{"data":[{"type":"alternativeDistributionPackageVariants","id":"variant-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetAlternativeDistributionPackageVersionVariants(ctx, "ver-1", Limit[AlternativeDistributionPackageVariantsOption](6))
				if err != nil {
					t.Fatalf("GetAlternativeDistributionPackageVersionVariants() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "variant-1" {
					t.Fatalf("expected decoded package variant response, got %+v", resp.Data)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", req.Method)
				}
				if req.URL.Path != tt.path {
					t.Fatalf("expected path %s, got %s", tt.path, req.URL.Path)
				}
				if req.URL.Query().Get("limit") != tt.limit {
					t.Fatalf("expected limit=%s, got %q", tt.limit, req.URL.Query().Get("limit"))
				}
				assertAuthorized(t, req)
			}, jsonResponse(http.StatusOK, tt.response))

			tt.call(t, client)
		})
	}
}

func TestAlternativeDistributionDeleteEndpoints_SendsRequest(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		path string
		call func(*Client) error
	}{
		{
			name: "DeleteAlternativeDistributionKey",
			path: "/v1/alternativeDistributionKeys/key-1",
			call: func(c *Client) error {
				return c.DeleteAlternativeDistributionKey(ctx, "key-1")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodDelete {
					t.Fatalf("expected DELETE, got %s", req.Method)
				}
				if req.URL.Path != tt.path {
					t.Fatalf("expected path %s, got %s", tt.path, req.URL.Path)
				}
				assertAuthorized(t, req)
			}, jsonResponse(http.StatusNoContent, ""))

			if err := tt.call(client); err != nil {
				t.Fatalf("%s() error: %v", tt.name, err)
			}
		})
	}
}

func TestCreateAlternativeDistributionDomain_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"alternativeDistributionDomains","id":"domain-1","attributes":{"domain":"example.com","referenceName":"Example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionDomains" {
			t.Fatalf("expected path /v1/alternativeDistributionDomains, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AlternativeDistributionDomainCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAlternativeDistributionDomains {
			t.Fatalf("expected type alternativeDistributionDomains, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Domain != "example.com" {
			t.Fatalf("expected domain example.com, got %q", payload.Data.Attributes.Domain)
		}
		if payload.Data.Attributes.ReferenceName != "Example" {
			t.Fatalf("expected reference name Example, got %q", payload.Data.Attributes.ReferenceName)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAlternativeDistributionDomain(context.Background(), "example.com", "Example"); err != nil {
		t.Fatalf("CreateAlternativeDistributionDomain() error: %v", err)
	}
}

func TestCreateAlternativeDistributionKey_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"alternativeDistributionKeys","id":"key-1","attributes":{"publicKey":"KEY"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionKeys" {
			t.Fatalf("expected path /v1/alternativeDistributionKeys, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AlternativeDistributionKeyCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAlternativeDistributionKeys {
			t.Fatalf("expected type alternativeDistributionKeys, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.PublicKey != "KEY" {
			t.Fatalf("expected public key KEY, got %q", payload.Data.Attributes.PublicKey)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship to be set")
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app id app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAlternativeDistributionKey(context.Background(), "app-1", "KEY"); err != nil {
		t.Fatalf("CreateAlternativeDistributionKey() error: %v", err)
	}
}

func TestCreateAlternativeDistributionPackage_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"alternativeDistributionPackages","id":"pkg-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/alternativeDistributionPackages" {
			t.Fatalf("expected path /v1/alternativeDistributionPackages, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AlternativeDistributionPackageCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAlternativeDistributionPackages {
			t.Fatalf("expected type alternativeDistributionPackages, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships.AppStoreVersion.Data.ID != "version-1" {
			t.Fatalf("expected app store version id version-1, got %q", payload.Data.Relationships.AppStoreVersion.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAlternativeDistributionPackage(context.Background(), "version-1"); err != nil {
		t.Fatalf("CreateAlternativeDistributionPackage() error: %v", err)
	}
}
