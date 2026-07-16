package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCreateSubscriptionGroupVersionUsesRequiredGroupRelationship(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionGroupVersions" {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
		var payload SubscriptionGroupVersionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionGroupVersions || payload.Data.Relationships.SubscriptionGroup.Data.ID != "group-1" {
			t.Fatalf("unexpected payload %#v", payload)
		}
		assertAuthorized(t, req)
	}, jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionGroupVersions","id":"version-1","attributes":{"version":1,"state":"PREPARE_FOR_SUBMISSION"}}}`))

	resp, err := client.CreateSubscriptionGroupVersion(context.Background(), "group-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Data.ID != "version-1" || resp.Data.Attributes.Version != 1 {
		t.Fatalf("unexpected response %#v", resp)
	}
}

func TestSubscriptionGroupVersionEndpoints(t *testing.T) {
	updatedName := "Premium Plus"
	cleared := NullableString{Value: nil}
	tests := []struct {
		name         string
		method       string
		path         string
		responseCode int
		responseBody string
		bodyContains []string
		query        map[string]string
		call         func(*Client) error
	}{
		{
			name: "get version", method: http.MethodGet, path: "/v1/subscriptionGroupVersions/version-1", responseCode: http.StatusOK,
			responseBody: `{"data":{"type":"subscriptionGroupVersions","id":"version-1","attributes":{"version":2}}}`,
			query:        map[string]string{"include": "localizations", "fields[subscriptionGroupVersions]": "version,state", "limit[localizations]": "5"},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionGroupVersion(context.Background(), "version-1", WithSubscriptionGroupVersionsInclude([]string{"localizations"}), WithSubscriptionGroupVersionsFields([]string{"version", "state"}), WithSubscriptionGroupVersionsLocalizationsLimit(5))
				return err
			},
		},
		{
			name: "list versions", method: http.MethodGet, path: "/v1/subscriptionGroups/group-1/versions", responseCode: http.StatusOK,
			responseBody: `{"data":[{"type":"subscriptionGroupVersions","id":"version-1","attributes":{"state":"READY_FOR_REVIEW"}}]}`,
			query:        map[string]string{"filter[state]": "READY_FOR_REVIEW", "include": "subscriptionGroup,localizations", "limit": "3", "limit[localizations]": "2"},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionGroupVersions(context.Background(), "group-1", WithSubscriptionGroupVersionsStates([]string{"READY_FOR_REVIEW"}), WithSubscriptionGroupVersionsInclude([]string{"subscriptionGroup", "localizations"}), WithSubscriptionGroupVersionsLimit(3), WithSubscriptionGroupVersionsLocalizationsLimit(2))
				return err
			},
		},
		{
			name: "version linkages", method: http.MethodGet, path: "/v1/subscriptionGroups/group-1/relationships/versions", responseCode: http.StatusOK,
			responseBody: `{"data":[{"type":"subscriptionGroupVersions","id":"version-1"}]}`, query: map[string]string{"limit": "4"},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionGroupVersionsRelationships(context.Background(), "group-1", WithLinkagesLimit(4))
				return err
			},
		},
		{
			name: "related localizations", method: http.MethodGet, path: "/v1/subscriptionGroupVersions/version-1/localizations", responseCode: http.StatusOK,
			responseBody: `{"data":[{"type":"subscriptionGroupLocalizations","id":"loc-1"}]}`,
			query:        map[string]string{"include": "version", "fields[subscriptionGroupLocalizations]": "name,locale", "limit": "5"},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionGroupVersionLocalizations(context.Background(), "version-1", WithSubscriptionGroupVersionLocalizationsInclude([]string{"version"}), WithSubscriptionGroupVersionLocalizationsFields([]string{"name", "locale"}), WithSubscriptionGroupVersionLocalizationsLimit(5))
				return err
			},
		},
		{
			name: "localization linkages", method: http.MethodGet, path: "/v1/subscriptionGroupVersions/version-1/relationships/localizations", responseCode: http.StatusOK,
			responseBody: `{"data":[{"type":"subscriptionGroupLocalizations","id":"loc-1"}]}`, query: map[string]string{"limit": "6"},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionGroupVersionLocalizationsRelationships(context.Background(), "version-1", WithLinkagesLimit(6))
				return err
			},
		},
		{
			name: "create localization v2", method: http.MethodPost, path: "/v2/subscriptionGroupLocalizations", responseCode: http.StatusCreated,
			responseBody: `{"data":{"type":"subscriptionGroupLocalizations","id":"loc-1"}}`,
			bodyContains: []string{`"version":{"data":{"type":"subscriptionGroupVersions","id":"version-1"}}`, `"name":"Premium"`, `"locale":"en-US"`, `"customAppName":"Example"`},
			call: func(c *Client) error {
				_, err := c.CreateSubscriptionGroupLocalizationV2(context.Background(), "version-1", SubscriptionGroupLocalizationV2CreateAttributes{Name: "Premium", Locale: "en-US", CustomAppName: "Example"})
				return err
			},
		},
		{
			name: "get localization v2", method: http.MethodGet, path: "/v2/subscriptionGroupLocalizations/loc-1", responseCode: http.StatusOK,
			responseBody: `{"data":{"type":"subscriptionGroupLocalizations","id":"loc-1"}}`, query: map[string]string{"include": "version", "fields[subscriptionGroupVersions]": "state"},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionGroupLocalizationV2(context.Background(), "loc-1", WithSubscriptionGroupVersionLocalizationsInclude([]string{"version"}), WithSubscriptionGroupVersionLocalizationsVersionFields([]string{"state"}))
				return err
			},
		},
		{
			name: "update localization v2 with nullable field", method: http.MethodPatch, path: "/v2/subscriptionGroupLocalizations/loc-1", responseCode: http.StatusOK,
			responseBody: `{"data":{"type":"subscriptionGroupLocalizations","id":"loc-1"}}`, bodyContains: []string{`"name":"Premium Plus"`, `"customAppName":null`},
			call: func(c *Client) error {
				_, err := c.UpdateSubscriptionGroupLocalizationV2(context.Background(), "loc-1", SubscriptionGroupLocalizationV2UpdateAttributes{Name: &NullableString{Value: &updatedName}, CustomAppName: &cleared})
				return err
			},
		},
		{
			name: "delete localization v2", method: http.MethodDelete, path: "/v2/subscriptionGroupLocalizations/loc-1", responseCode: http.StatusNoContent,
			call: func(c *Client) error { return c.DeleteSubscriptionGroupLocalizationV2(context.Background(), "loc-1") },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != test.method || req.URL.Path != test.path {
					t.Fatalf("expected %s %s, got %s %s", test.method, test.path, req.Method, req.URL.Path)
				}
				for key, want := range test.query {
					if got := req.URL.Query().Get(key); got != want {
						t.Fatalf("query %s = %q, want %q", key, got, want)
					}
				}
				if len(test.bodyContains) > 0 {
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatal(err)
					}
					for _, want := range test.bodyContains {
						if !strings.Contains(string(body), want) {
							t.Fatalf("body %s missing %s", body, want)
						}
					}
				}
				assertAuthorized(t, req)
			}, jsonResponse(test.responseCode, test.responseBody))
			if err := test.call(client); err != nil {
				t.Fatalf("call error: %v", err)
			}
		})
	}
}

func TestSubscriptionGroupGetEndpointsExposeVersionsQuerySurface(t *testing.T) {
	tests := []struct {
		name string
		path string
		call func(*Client) error
	}{
		{"app group collection", "/v1/apps/app-1/subscriptionGroups", func(c *Client) error {
			_, err := c.GetSubscriptionGroups(context.Background(), "app-1", WithSubscriptionGroupsInclude([]string{"versions"}), WithSubscriptionGroupsFields([]string{"referenceName", "versions"}), WithSubscriptionGroupsVersionFields([]string{"version", "state"}), WithSubscriptionGroupsVersionsLimit(5))
			return err
		}},
		{"group detail", "/v1/subscriptionGroups/group-1", func(c *Client) error {
			_, err := c.GetSubscriptionGroup(context.Background(), "group-1", WithSubscriptionGroupsInclude([]string{"versions"}), WithSubscriptionGroupsFields([]string{"versions"}), WithSubscriptionGroupsVersionFields([]string{"state"}), WithSubscriptionGroupsVersionsLimit(4))
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.URL.Path != test.path {
					t.Fatalf("path = %s, want %s", req.URL.Path, test.path)
				}
				if got := req.URL.Query().Get("include"); got != "versions" {
					t.Fatalf("include = %q", got)
				}
				if got := req.URL.Query().Get("fields[subscriptionGroupVersions]"); got == "" {
					t.Fatal("missing version sparse fields")
				}
				if got := req.URL.Query().Get("limit[versions]"); got == "" {
					t.Fatal("missing versions relationship limit")
				}
			}, jsonResponse(http.StatusOK, func() string {
				if strings.Contains(test.path, "/apps/") {
					return `{"data":[]}`
				}
				return `{"data":{"type":"subscriptionGroups","id":"group-1"}}`
			}()))
			if err := test.call(client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSubscriptionGroupVersionsDecodeAcrossSharedResponseShapes(t *testing.T) {
	groupResource := `{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"},"relationships":{"versions":{"data":[{"type":"subscriptionGroupVersions","id":"version-1"}]}}}`
	groupResponse := `{"data":` + groupResource + `}`
	includedGroupResponse := `{"data":{"type":"subscriptionGroupLocalizations","id":"loc-1"},"included":[` + groupResource + `]}`
	appResponse := `{"data":{"type":"apps","id":"app-1"},"included":[` + groupResource + `]}`
	updatedName := "Updated"

	decodeIncludedGroup := func(t *testing.T, included json.RawMessage) json.RawMessage {
		t.Helper()
		var groups []Resource[SubscriptionGroupAttributes]
		if err := json.Unmarshal(included, &groups); err != nil {
			t.Fatalf("decode included subscription groups: %v", err)
		}
		for _, group := range groups {
			if group.Type == ResourceTypeSubscriptionGroups {
				return group.Relationships
			}
		}
		t.Fatal("included response did not contain a subscription group")
		return nil
	}

	tests := []struct {
		name         string
		method       string
		path         string
		responseBody string
		call         func(*testing.T, *Client) json.RawMessage
	}{
		{
			name: "direct group", method: http.MethodGet, path: "/v1/subscriptionGroups/group-1", responseBody: groupResponse,
			call: func(t *testing.T, c *Client) json.RawMessage {
				resp, err := c.GetSubscriptionGroup(context.Background(), "group-1")
				if err != nil {
					t.Fatal(err)
				}
				return resp.Data.Relationships
			},
		},
		{
			name: "group create", method: http.MethodPost, path: "/v1/subscriptionGroups", responseBody: groupResponse,
			call: func(t *testing.T, c *Client) json.RawMessage {
				resp, err := c.CreateSubscriptionGroup(context.Background(), "app-1", SubscriptionGroupCreateAttributes{ReferenceName: "Premium"})
				if err != nil {
					t.Fatal(err)
				}
				return resp.Data.Relationships
			},
		},
		{
			name: "group update", method: http.MethodPatch, path: "/v1/subscriptionGroups/group-1", responseBody: groupResponse,
			call: func(t *testing.T, c *Client) json.RawMessage {
				resp, err := c.UpdateSubscriptionGroup(context.Background(), "group-1", SubscriptionGroupUpdateAttributes{ReferenceName: &updatedName})
				if err != nil {
					t.Fatal(err)
				}
				return resp.Data.Relationships
			},
		},
		{
			name: "legacy localization included group", method: http.MethodGet, path: "/v1/subscriptionGroupLocalizations/loc-1", responseBody: includedGroupResponse,
			call: func(t *testing.T, c *Client) json.RawMessage {
				resp, err := c.GetSubscriptionGroupLocalization(context.Background(), "loc-1")
				if err != nil {
					t.Fatal(err)
				}
				return decodeIncludedGroup(t, resp.Included)
			},
		},
		{
			name: "app included group", method: http.MethodGet, path: "/v1/apps/app-1", responseBody: appResponse,
			call: func(t *testing.T, c *Client) json.RawMessage {
				resp, err := c.GetApp(context.Background(), "app-1")
				if err != nil {
					t.Fatal(err)
				}
				return decodeIncludedGroup(t, resp.Included)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != test.method || req.URL.Path != test.path {
					t.Fatalf("expected %s %s, got %s %s", test.method, test.path, req.Method, req.URL.Path)
				}
			}, jsonResponse(http.StatusOK, test.responseBody))

			var relationships SubscriptionGroupRelationships
			if err := json.Unmarshal(test.call(t, client), &relationships); err != nil {
				t.Fatalf("decode subscription group relationships: %v", err)
			}
			if relationships.Versions == nil || len(relationships.Versions.Data) != 1 {
				t.Fatalf("versions relationship was not preserved: %#v", relationships.Versions)
			}
			version := relationships.Versions.Data[0]
			if version.Type != ResourceTypeSubscriptionGroupVersions || version.ID != "version-1" {
				t.Fatalf("unexpected version linkage: %#v", version)
			}
		})
	}
}

func TestSubscriptionGroupVersionsUsesValidatedNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/versions?cursor=next"
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("URL = %q, want %q", req.URL.String(), next)
		}
	}, jsonResponse(http.StatusOK, `{"data":[]}`))
	if _, err := client.GetSubscriptionGroupVersions(context.Background(), "", WithSubscriptionGroupVersionsNextURL(next)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetSubscriptionGroupVersions(context.Background(), "", WithSubscriptionGroupVersionsNextURL("https://example.com/steal")); err == nil {
		t.Fatal("expected untrusted next URL error")
	}
}

func TestGetSubscriptionGroupRejectsEmptyIDBeforeHTTP(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL)
	}, jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionGroups","id":"group-1"}}`))
	if _, err := client.GetSubscriptionGroup(context.Background(), "   "); err == nil || !strings.Contains(err.Error(), "groupID is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateSubscriptionGroupVersionPropagatesAPIError(t *testing.T) {
	client := newTestClient(t, nil, jsonResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","detail":"A draft version already exists."}]}`))
	_, err := client.CreateSubscriptionGroupVersion(context.Background(), "group-1")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "draft version already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}
