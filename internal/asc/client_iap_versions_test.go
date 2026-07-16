package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestCreateInAppPurchaseVersionUsesVersionRelationship(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseVersions","id":"version-1","attributes":{"version":1,"state":"PREPARE_FOR_SUBMISSION"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/inAppPurchaseVersions" {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
		var payload InAppPurchaseVersionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Data.Relationships.InAppPurchase.Data.ID != "iap-1" {
			t.Fatalf("expected iap-1 relationship, got %#v", payload)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.CreateInAppPurchaseVersion(context.Background(), "iap-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Data.ID != "version-1" || resp.Data.Attributes.Version != 1 {
		t.Fatalf("unexpected response %#v", resp)
	}
}

func TestInAppPurchaseVersionEndpoints(t *testing.T) {
	uploaded := true
	name := "Updated"
	tests := []struct {
		name         string
		method       string
		path         string
		responseCode int
		responseBody string
		wantBody     string
		query        map[string]string
		call         func(*Client) error
	}{
		{
			name: "get version", method: http.MethodGet, path: "/v1/inAppPurchaseVersions/version-1", responseCode: http.StatusOK,
			responseBody: `{"data":{"type":"inAppPurchaseVersions","id":"version-1","attributes":{"version":2}}}`,
			query: map[string]string{
				"fields[inAppPurchaseVersions]":      "version,state",
				"fields[inAppPurchases]":             "name,versions",
				"fields[inAppPurchaseImages]":        "fileName",
				"fields[inAppPurchaseLocalizations]": "name,version",
				"include":                            "localizations",
				"limit[localizations]":               "5",
			},
			call: func(c *Client) error {
				_, err := c.GetInAppPurchaseVersion(
					context.Background(), "version-1",
					WithIAPVersionGetFields([]string{"version", "state"}),
					WithIAPVersionGetIAPFields([]string{"name", "versions"}),
					WithIAPVersionGetImageFields([]string{"fileName"}),
					WithIAPVersionGetLocalizationFields([]string{"name", "version"}),
					WithIAPVersionGetInclude([]string{"localizations"}),
					WithIAPVersionGetLocalizationsLimit(5),
				)
				return err
			},
		},
		{
			name: "list versions", method: http.MethodGet, path: "/v2/inAppPurchases/iap-1/versions", responseCode: http.StatusOK,
			responseBody: `{"data":[{"type":"inAppPurchaseVersions","id":"version-1","attributes":{"state":"READY_FOR_REVIEW"}}]}`,
			query: map[string]string{
				"filter[state]":                      "READY_FOR_REVIEW",
				"fields[inAppPurchaseVersions]":      "version,state",
				"fields[inAppPurchases]":             "name",
				"fields[inAppPurchaseImages]":        "fileName",
				"fields[inAppPurchaseLocalizations]": "locale",
				"include":                            "images",
				"limit":                              "3",
				"limit[images]":                      "2",
			},
			call: func(c *Client) error {
				_, err := c.GetInAppPurchaseVersions(
					context.Background(), "iap-1",
					WithIAPVersionsStates([]string{"READY_FOR_REVIEW"}),
					WithIAPVersionsFields([]string{"version", "state"}),
					WithIAPVersionsIAPFields([]string{"name"}),
					WithIAPVersionsImageFields([]string{"fileName"}),
					WithIAPVersionsLocalizationFields([]string{"locale"}),
					WithIAPVersionsInclude([]string{"images"}),
					WithIAPVersionsLimit(3),
					WithIAPVersionsImagesLimit(2),
				)
				return err
			},
		},
		{
			name: "versions linkages", method: http.MethodGet, path: "/v2/inAppPurchases/iap-1/relationships/versions", responseCode: http.StatusOK,
			responseBody: `{"data":[{"type":"inAppPurchaseVersions","id":"version-1"}]}`, query: map[string]string{"limit": "4"},
			call: func(c *Client) error {
				_, err := c.GetInAppPurchaseVersionsRelationships(context.Background(), "iap-1", WithLinkagesLimit(4))
				return err
			},
		},
		{
			name: "related image", method: http.MethodGet, path: "/v1/inAppPurchaseVersions/version-1/image", responseCode: http.StatusOK,
			responseBody: `{"data":{"type":"inAppPurchaseImages","id":"image-1"}}`,
			query:        map[string]string{"fields[inAppPurchaseImages]": "fileName"},
			call: func(c *Client) error {
				_, err := c.GetInAppPurchaseVersionImage(context.Background(), "version-1", WithIAPVersionImageFields([]string{"fileName"}))
				return err
			},
		},
		{
			name: "image linkage", method: http.MethodGet, path: "/v1/inAppPurchaseVersions/version-1/relationships/image", responseCode: http.StatusOK,
			responseBody: `{"data":{"type":"inAppPurchaseImages","id":"image-1"}}`,
			call: func(c *Client) error {
				_, err := c.GetInAppPurchaseVersionImageRelationship(context.Background(), "version-1")
				return err
			},
		},
		{
			name: "related images", method: http.MethodGet, path: "/v1/inAppPurchaseVersions/version-1/images", responseCode: http.StatusOK,
			responseBody: `{"data":[{"type":"inAppPurchaseImages","id":"image-1"}]}`, query: map[string]string{"fields[inAppPurchaseImages]": "fileName,assetDeliveryState", "limit": "7"},
			call: func(c *Client) error {
				_, err := c.GetInAppPurchaseVersionImages(context.Background(), "version-1", WithIAPVersionImagesFields([]string{"fileName", "assetDeliveryState"}), WithIAPVersionImagesLimit(7))
				return err
			},
		},
		{
			name: "images linkages", method: http.MethodGet, path: "/v1/inAppPurchaseVersions/version-1/relationships/images", responseCode: http.StatusOK,
			responseBody: `{"data":[{"type":"inAppPurchaseImages","id":"image-1"}]}`, query: map[string]string{"limit": "8"},
			call: func(c *Client) error {
				_, err := c.GetInAppPurchaseVersionImagesRelationships(context.Background(), "version-1", WithLinkagesLimit(8))
				return err
			},
		},
		{
			name: "related localizations", method: http.MethodGet, path: "/v1/inAppPurchaseVersions/version-1/localizations", responseCode: http.StatusOK,
			responseBody: `{"data":[{"type":"inAppPurchaseLocalizations","id":"loc-1"}]}`, query: map[string]string{"fields[inAppPurchaseLocalizations]": "name,locale", "fields[inAppPurchaseVersions]": "version,state", "include": "version", "limit": "9"},
			call: func(c *Client) error {
				_, err := c.GetInAppPurchaseVersionLocalizations(context.Background(), "version-1", WithIAPVersionLocalizationsFields([]string{"name", "locale"}), WithIAPVersionLocalizationsVersionFields([]string{"version", "state"}), WithIAPVersionLocalizationsInclude([]string{"version"}), WithIAPVersionLocalizationsLimit(9))
				return err
			},
		},
		{
			name: "localizations linkages", method: http.MethodGet, path: "/v1/inAppPurchaseVersions/version-1/relationships/localizations", responseCode: http.StatusOK,
			responseBody: `{"data":[{"type":"inAppPurchaseLocalizations","id":"loc-1"}]}`, query: map[string]string{"limit": "10"},
			call: func(c *Client) error {
				_, err := c.GetInAppPurchaseVersionLocalizationsRelationships(context.Background(), "version-1", WithLinkagesLimit(10))
				return err
			},
		},
		{
			name: "create localization v2", method: http.MethodPost, path: "/v2/inAppPurchaseLocalizations", responseCode: http.StatusCreated,
			responseBody: `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1"}}`,
			wantBody:     `{"data":{"type":"inAppPurchaseLocalizations","attributes":{"name":"Name","locale":"en-US"},"relationships":{"version":{"data":{"type":"inAppPurchaseVersions","id":"version-1"}}}}}`,
			call: func(c *Client) error {
				_, err := c.CreateInAppPurchaseLocalizationV2(context.Background(), "version-1", InAppPurchaseLocalizationCreateAttributes{Name: "Name", Locale: "en-US"})
				return err
			},
		},
		{
			name: "get localization v2", method: http.MethodGet, path: "/v2/inAppPurchaseLocalizations/loc-1", responseCode: http.StatusOK,
			responseBody: `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1"}}`,
			query:        map[string]string{"fields[inAppPurchaseLocalizations]": "name,description", "fields[inAppPurchaseVersions]": "version,state", "include": "version"},
			call: func(c *Client) error {
				_, err := c.GetInAppPurchaseLocalizationV2(context.Background(), "loc-1", WithIAPLocalizationV2Fields([]string{"name", "description"}), WithIAPLocalizationV2VersionFields([]string{"version", "state"}), WithIAPLocalizationV2Include([]string{"version"}))
				return err
			},
		},
		{
			name: "update localization v2", method: http.MethodPatch, path: "/v2/inAppPurchaseLocalizations/loc-1", responseCode: http.StatusOK,
			responseBody: `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1"}}`,
			wantBody:     `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{"name":"Updated"}}}`,
			call: func(c *Client) error {
				_, err := c.UpdateInAppPurchaseLocalizationV2(context.Background(), "loc-1", InAppPurchaseLocalizationUpdateAttributes{Name: &name})
				return err
			},
		},
		{
			name: "delete localization v2", method: http.MethodDelete, path: "/v2/inAppPurchaseLocalizations/loc-1", responseCode: http.StatusNoContent,
			call: func(c *Client) error { return c.DeleteInAppPurchaseLocalizationV2(context.Background(), "loc-1") },
		},
		{
			name: "create image v2", method: http.MethodPost, path: "/v2/inAppPurchaseImages", responseCode: http.StatusCreated,
			responseBody: `{"data":{"type":"inAppPurchaseImages","id":"image-1"}}`,
			wantBody:     `{"data":{"type":"inAppPurchaseImages","attributes":{"fileSize":123,"fileName":"image.png"},"relationships":{"version":{"data":{"type":"inAppPurchaseVersions","id":"version-1"}}}}}`,
			call: func(c *Client) error {
				_, err := c.CreateInAppPurchaseImageV2(context.Background(), "version-1", "image.png", 123)
				return err
			},
		},
		{
			name: "get image v2", method: http.MethodGet, path: "/v2/inAppPurchaseImages/image-1", responseCode: http.StatusOK,
			responseBody: `{"data":{"type":"inAppPurchaseImages","id":"image-1"}}`,
			query:        map[string]string{"fields[inAppPurchaseImages]": "fileName,assetDeliveryState"},
			call: func(c *Client) error {
				_, err := c.GetInAppPurchaseImageV2(context.Background(), "image-1", WithIAPImageV2Fields([]string{"fileName", "assetDeliveryState"}))
				return err
			},
		},
		{
			name: "update image v2", method: http.MethodPatch, path: "/v2/inAppPurchaseImages/image-1", responseCode: http.StatusOK,
			responseBody: `{"data":{"type":"inAppPurchaseImages","id":"image-1"}}`,
			wantBody:     `{"data":{"type":"inAppPurchaseImages","id":"image-1","attributes":{"uploaded":true}}}`,
			call: func(c *Client) error {
				_, err := c.UpdateInAppPurchaseImageV2(context.Background(), "image-1", InAppPurchaseImageV2UpdateAttributes{Uploaded: &uploaded})
				return err
			},
		},
		{
			name: "delete image v2", method: http.MethodDelete, path: "/v2/inAppPurchaseImages/image-1", responseCode: http.StatusNoContent,
			call: func(c *Client) error { return c.DeleteInAppPurchaseImageV2(context.Background(), "image-1") },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := jsonResponse(test.responseCode, test.responseBody)
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != test.method || req.URL.Path != test.path {
					t.Fatalf("expected %s %s, got %s %s", test.method, test.path, req.Method, req.URL.Path)
				}
				wantQuery := url.Values{}
				for key, value := range test.query {
					wantQuery.Set(key, value)
				}
				if got := req.URL.Query(); !reflect.DeepEqual(got, wantQuery) {
					t.Fatalf("query = %v, want %v", got, wantQuery)
				}
				if test.wantBody != "" {
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatal(err)
					}
					var gotJSON, wantJSON any
					if err := json.Unmarshal(body, &gotJSON); err != nil {
						t.Fatalf("invalid request JSON %q: %v", body, err)
					}
					if err := json.Unmarshal([]byte(test.wantBody), &wantJSON); err != nil {
						t.Fatalf("invalid expected JSON %q: %v", test.wantBody, err)
					}
					if !reflect.DeepEqual(gotJSON, wantJSON) {
						t.Fatalf("request body = %s, want %s", body, test.wantBody)
					}
				}
				assertAuthorized(t, req)
			}, response)
			if err := test.call(client); err != nil {
				t.Fatalf("call error: %v", err)
			}
		})
	}
}

func TestInAppPurchaseExistingEndpointsExposeVersionsQuerySurface(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		response string
		call     func(*Client) error
	}{
		{"app IAP collection", "/v1/apps/app-1/inAppPurchasesV2", `{"data":[]}`, func(c *Client) error {
			_, err := c.GetInAppPurchasesV2(context.Background(), "app-1", WithIAPInclude([]string{"versions"}), WithIAPVersionFields([]string{"version", "state"}), WithIAPNestedVersionsLimit(5))
			return err
		}},
		{"IAP detail", "/v2/inAppPurchases/iap-1", `{"data":{"type":"inAppPurchases","id":"iap-1"}}`, func(c *Client) error {
			_, err := c.GetInAppPurchaseV2(context.Background(), "iap-1", WithIAPInclude([]string{"versions"}), WithIAPVersionFields([]string{"version", "state"}), WithIAPNestedVersionsLimit(5))
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
				if got := req.URL.Query().Get("fields[inAppPurchaseVersions]"); got != "version,state" {
					t.Fatalf("version fields = %q", got)
				}
				if got := req.URL.Query().Get("limit[versions]"); got != "5" {
					t.Fatalf("version limit = %q", got)
				}
			}, jsonResponse(http.StatusOK, test.response))
			if err := test.call(client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInAppPurchaseVersionsUsesValidatedNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-1/versions?cursor=next"
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("URL = %q, want %q", req.URL.String(), next)
		}
	}, jsonResponse(http.StatusOK, `{"data":[]}`))
	if _, err := client.GetInAppPurchaseVersions(context.Background(), "", WithIAPVersionsNextURL(next)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetInAppPurchaseVersions(context.Background(), "", WithIAPVersionsNextURL("https://example.com/steal")); err == nil {
		t.Fatal("expected untrusted next URL error")
	}
}

func TestCreateInAppPurchaseVersionPropagatesAPIError(t *testing.T) {
	client := newTestClient(t, nil, jsonResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","detail":"A draft version already exists."}]}`))
	_, err := client.CreateInAppPurchaseVersion(context.Background(), "iap-1")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "draft version already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}
