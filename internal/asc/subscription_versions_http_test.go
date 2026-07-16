package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestSubscriptionVersionReadOperations(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		path     string
		query    map[string]string
		response string
		call     func(*Client) error
	}{
		{
			name: "version detail", path: "/v1/subscriptionVersions/ver-1",
			query:    map[string]string{"include": "subscription,images,localizations", "limit[images]": "5", "limit[localizations]": "6", "fields[subscriptionVersions]": "version,state"},
			response: `{"data":{"type":"subscriptionVersions","id":"ver-1","attributes":{"version":2,"state":"PREPARE_FOR_SUBMISSION"}},"included":[{"type":"subscriptions","id":"sub-1"}]}`,
			call: func(c *Client) error {
				resp, err := c.GetSubscriptionVersion(ctx, "ver-1", WithSubscriptionVersionInclude([]string{"subscription", "images", "localizations"}), WithSubscriptionVersionImageLimit(5), WithSubscriptionVersionLocalizationLimit(6), WithSubscriptionVersionFields([]string{"version", "state"}))
				if err == nil && (resp.Data.Attributes.Version != 2 || len(resp.Included) == 0) {
					t.Fatalf("version response not decoded: %+v", resp)
				}
				return err
			},
		},
		{
			name: "subscription versions related", path: "/v1/subscriptions/sub-1/versions",
			query:    map[string]string{"filter[state]": "PREPARE_FOR_SUBMISSION", "limit": "7", "include": "localizations"},
			response: `{"data":[{"type":"subscriptionVersions","id":"ver-1","attributes":{"version":1}}],"links":{}}`,
			call: func(c *Client) error {
				_, err := c.GetSubscriptionVersions(ctx, "sub-1", WithSubscriptionVersionsStates([]SubscriptionVersionState{SubscriptionVersionStatePrepareForSubmission}), WithSubscriptionVersionsLimit(7), WithSubscriptionVersionsInclude([]string{"localizations"}))
				return err
			},
		},
		{
			name: "subscription versions linkage", path: "/v1/subscriptions/sub-1/relationships/versions", query: map[string]string{"limit": "8"},
			response: `{"data":[{"type":"subscriptionVersions","id":"ver-1"}],"links":{}}`,
			call: func(c *Client) error {
				_, err := c.GetSubscriptionVersionsRelationships(ctx, "sub-1", WithLinkagesLimit(8))
				return err
			},
		},
		{
			name: "version localizations related", path: "/v1/subscriptionVersions/ver-1/localizations", query: map[string]string{"limit": "9", "include": "version", "fields[subscriptionLocalizations]": "name,locale"},
			response: `{"data":[{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro","locale":"en-US"}}],"links":{}}`,
			call: func(c *Client) error {
				resp, err := c.GetSubscriptionVersionLocalizations(ctx, "ver-1", WithSubscriptionVersionLocalizationsLimit(9), WithSubscriptionVersionLocalizationsInclude([]string{"version"}), WithSubscriptionVersionLocalizationsFields([]string{"name", "locale"}))
				if err == nil && resp.Data[0].Attributes.Locale != "en-US" {
					t.Fatalf("localization not decoded")
				}
				return err
			},
		},
		{
			name: "version localizations linkage", path: "/v1/subscriptionVersions/ver-1/relationships/localizations", query: map[string]string{"limit": "10"},
			response: `{"data":[{"type":"subscriptionLocalizations","id":"loc-1"}],"links":{}}`,
			call: func(c *Client) error {
				_, err := c.GetSubscriptionVersionLocalizationsRelationships(ctx, "ver-1", WithLinkagesLimit(10))
				return err
			},
		},
		{
			name: "version singular image related", path: "/v1/subscriptionVersions/ver-1/image", query: map[string]string{"fields[subscriptionImages]": "fileName,fileSize"},
			response: `{"data":{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":123}}}`,
			call: func(c *Client) error {
				resp, err := c.GetSubscriptionVersionImage(ctx, "ver-1", WithSubscriptionImageV2Fields([]string{"fileName", "fileSize"}))
				if err == nil && resp.Data.Attributes.FileName != "image.png" {
					t.Fatalf("image not decoded")
				}
				return err
			},
		},
		{
			name: "version singular image linkage", path: "/v1/subscriptionVersions/ver-1/relationships/image", query: map[string]string{},
			response: `{"data":{"type":"subscriptionImages","id":"img-1"},"links":{}}`,
			call: func(c *Client) error {
				resp, err := c.GetSubscriptionVersionImageRelationship(ctx, "ver-1")
				if err == nil && resp.Data.ID != "img-1" {
					t.Fatalf("linkage not decoded")
				}
				return err
			},
		},
		{
			name: "version plural images related", path: "/v1/subscriptionVersions/ver-1/images", query: map[string]string{"limit": "11", "fields[subscriptionImages]": "fileName"},
			response: `{"data":[{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png"}}],"links":{}}`,
			call: func(c *Client) error {
				_, err := c.GetSubscriptionVersionImages(ctx, "ver-1", WithSubscriptionVersionImagesLimit(11), WithSubscriptionVersionImagesFields([]string{"fileName"}))
				return err
			},
		},
		{
			name: "version plural images linkage", path: "/v1/subscriptionVersions/ver-1/relationships/images", query: map[string]string{"limit": "12"},
			response: `{"data":[{"type":"subscriptionImages","id":"img-1"}],"links":{}}`,
			call: func(c *Client) error {
				_, err := c.GetSubscriptionVersionImagesRelationships(ctx, "ver-1", WithLinkagesLimit(12))
				return err
			},
		},
		{
			name: "v2 localization detail", path: "/v2/subscriptionLocalizations/loc-1", query: map[string]string{"include": "version", "fields[subscriptionVersions]": "version,state"},
			response: `{"data":{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro","locale":"en-US"}},"included":[{"type":"subscriptionVersions","id":"ver-1","attributes":{"version":1}}]}`,
			call: func(c *Client) error {
				resp, err := c.GetSubscriptionLocalizationV2(ctx, "loc-1", WithSubscriptionLocalizationV2Include([]string{"version"}), WithSubscriptionLocalizationV2VersionFields([]string{"version", "state"}))
				if err == nil && len(resp.Included) == 0 {
					t.Fatalf("included version was dropped")
				}
				return err
			},
		},
		{
			name: "v2 image detail", path: "/v2/subscriptionImages/img-1", query: map[string]string{"fields[subscriptionImages]": "fileName,assetDeliveryState"},
			response: `{"data":{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png","assetDeliveryState":{"state":"COMPLETE"}}}}`,
			call: func(c *Client) error {
				resp, err := c.GetSubscriptionImageV2(ctx, "img-1", WithSubscriptionImageV2Fields([]string{"fileName", "assetDeliveryState"}))
				if err == nil && resp.Data.Attributes.AssetDeliveryState == nil {
					t.Fatalf("delivery state not decoded")
				}
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					t.Fatalf("request = %s %s, want GET %s", req.Method, req.URL.Path, test.path)
				}
				wantQuery := url.Values{}
				for key, want := range test.query {
					wantQuery.Set(key, want)
				}
				if got, want := req.URL.Query().Encode(), wantQuery.Encode(); got != want {
					t.Fatalf("query = %q, want %q", got, want)
				}
			}, jsonResponse(http.StatusOK, test.response))
			if err := test.call(client); err != nil {
				t.Fatalf("call error: %v", err)
			}
		})
	}
}

func TestCreateSubscriptionVersionRequest(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionVersions" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		var payload SubscriptionVersionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionVersions || payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("unexpected payload: %+v", payload)
		}
	}, jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionVersions","id":"ver-1","attributes":{"version":1}}}`))
	resp, err := client.CreateSubscriptionVersion(context.Background(), "sub-1")
	if err != nil || resp.Data.ID != "ver-1" {
		t.Fatalf("CreateSubscriptionVersion() = %+v, %v", resp, err)
	}
}

func TestSubscriptionLocalizationV2Mutations(t *testing.T) {
	description := "Access every feature"
	updatedName := "Pro Plus"
	client := newTestClient(
		t, func(req *http.Request) {
			switch req.Method {
			case http.MethodPost:
				if req.URL.Path != "/v2/subscriptionLocalizations" {
					t.Fatalf("POST path = %s", req.URL.Path)
				}
				var payload SubscriptionLocalizationV2CreateRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload.Data.Relationships.Version.Data.ID != "ver-1" || payload.Data.Attributes.Locale != "en-US" {
					t.Fatalf("unexpected create payload: %+v", payload)
				}
			case http.MethodPatch:
				if req.URL.Path != "/v2/subscriptionLocalizations/loc-1" {
					t.Fatalf("PATCH path = %s", req.URL.Path)
				}
				var payload struct {
					Data struct {
						Attributes map[string]json.RawMessage `json:"attributes"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if got := string(payload.Data.Attributes["name"]); got != `"Pro Plus"` {
					t.Fatalf("name JSON = %s", got)
				}
				if got := string(payload.Data.Attributes["description"]); got != "null" {
					t.Fatalf("description JSON = %s, want null", got)
				}
			case http.MethodDelete:
				if req.URL.Path != "/v2/subscriptionLocalizations/loc-1" {
					t.Fatalf("DELETE path = %s", req.URL.Path)
				}
			default:
				t.Fatalf("unexpected localization mutation method: %s", req.Method)
			}
		},
		jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro","locale":"en-US"}}}`),
		jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"description":""}}}`),
		jsonResponse(http.StatusNoContent, ``),
	)
	ctx := context.Background()
	if _, err := client.CreateSubscriptionLocalizationV2(ctx, "ver-1", SubscriptionLocalizationV2CreateAttributes{Name: "Pro", Locale: "en-US", Description: &description}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdateSubscriptionLocalizationV2(ctx, "loc-1", SubscriptionLocalizationV2UpdateAttributes{
		Name:        &NullableString{Value: &updatedName},
		Description: &NullableString{Value: nil},
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteSubscriptionLocalizationV2(ctx, "loc-1"); err != nil {
		t.Fatal(err)
	}
}

func TestSubscriptionLocalizationV2UpdateNullableEncoding(t *testing.T) {
	t.Parallel()

	name := "Pro Plus"
	empty := ""
	tests := []struct {
		name  string
		attrs SubscriptionLocalizationV2UpdateAttributes
		want  map[string]string
	}{
		{name: "omitted", attrs: SubscriptionLocalizationV2UpdateAttributes{}, want: map[string]string{}},
		{
			name: "string values",
			attrs: SubscriptionLocalizationV2UpdateAttributes{
				Name:        &NullableString{Value: &name},
				Description: &NullableString{Value: &empty},
			},
			want: map[string]string{"name": `"Pro Plus"`, "description": `""`},
		},
		{
			name: "explicit nulls",
			attrs: SubscriptionLocalizationV2UpdateAttributes{
				Name:        &NullableString{Value: nil},
				Description: &NullableString{Value: nil},
			},
			want: map[string]string{"name": "null", "description": "null"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(SubscriptionLocalizationV2UpdateRequest{Data: SubscriptionLocalizationV2UpdateData{
				Type: ResourceTypeSubscriptionLocalizations, ID: "loc-1", Attributes: test.attrs,
			}})
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			var payload struct {
				Data struct {
					Attributes map[string]json.RawMessage `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if len(payload.Data.Attributes) != len(test.want) {
				t.Fatalf("attributes = %s, want %v", payload.Data.Attributes, test.want)
			}
			for key, want := range test.want {
				if got := string(payload.Data.Attributes[key]); got != want {
					t.Fatalf("%s JSON = %s, want %s", key, got, want)
				}
			}
		})
	}
}

func TestSubscriptionImageV2Mutations(t *testing.T) {
	client := newTestClient(
		t, func(req *http.Request) {
			switch req.Method {
			case http.MethodPost:
				if req.URL.Path != "/v2/subscriptionImages" {
					t.Fatalf("POST path = %s", req.URL.Path)
				}
				var payload SubscriptionImageV2CreateRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload.Data.Relationships.Version.Data.ID != "ver-1" || payload.Data.Attributes.FileSize != 123 {
					t.Fatalf("unexpected create payload: %+v", payload)
				}
			case http.MethodPatch:
				if req.URL.Path != "/v2/subscriptionImages/img-1" {
					t.Fatalf("PATCH path = %s", req.URL.Path)
				}
				var payload SubscriptionImageV2UpdateRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
					t.Fatalf("expected uploaded=true: %+v", payload)
				}
			case http.MethodDelete:
				if req.URL.Path != "/v2/subscriptionImages/img-1" {
					t.Fatalf("DELETE path = %s", req.URL.Path)
				}
			default:
				t.Fatalf("unexpected image mutation method: %s", req.Method)
			}
		},
		jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":123,"uploadOperations":[{"method":"PUT","url":"https://example.com"}]}}}`),
		jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png"}}}`),
		jsonResponse(http.StatusNoContent, ``),
	)
	ctx := context.Background()
	if _, err := client.CreateSubscriptionImageV2(ctx, "ver-1", "image.png", 123); err != nil {
		t.Fatal(err)
	}
	uploaded := true
	if _, err := client.UpdateSubscriptionImageV2(ctx, "img-1", SubscriptionImageV2UpdateAttributes{Uploaded: &uploaded}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteSubscriptionImageV2(ctx, "img-1"); err != nil {
		t.Fatal(err)
	}
}

func TestSubscriptionVersionsNextURLAndAPIError(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/versions?cursor=next"
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("next URL = %q", req.URL.String())
		}
	}, jsonResponse(http.StatusBadRequest, `{"errors":[{"status":"400","title":"Invalid request"}]}`))
	_, err := client.GetSubscriptionVersions(context.Background(), "", WithSubscriptionVersionsNextURL(next))
	if err == nil || !strings.Contains(err.Error(), "Invalid request") {
		t.Fatalf("expected API error, got %v", err)
	}
}

func TestSubscriptionGetContractsSupportVersions(t *testing.T) {
	responses := []*http.Response{
		jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1"}],"included":[{"type":"subscriptionVersions","id":"ver-1","attributes":{"version":1}}],"links":{}}`),
		jsonResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1"},"included":[{"type":"subscriptionVersions","id":"ver-1","attributes":{"version":1}}]}`),
	}
	client := newTestClient(t, func(req *http.Request) {
		want := url.Values{
			"fields[subscriptionVersions]": {"version,state"},
			"include":                      {"versions"},
			"limit[versions]":              {"5"},
		}.Encode()
		if got := req.URL.Query().Encode(); got != want {
			t.Fatalf("query = %q, want %q", got, want)
		}
	}, responses...)
	ctx := context.Background()
	list, err := client.GetSubscriptions(ctx, "group-1", WithSubscriptionsInclude([]string{"versions"}), WithSubscriptionsVersionFields([]string{"version", "state"}), WithSubscriptionsVersionLimit(5))
	if err != nil || len(list.Included) == 0 {
		t.Fatalf("list included versions = %s, err=%v", list.Included, err)
	}
	detail, err := client.GetSubscription(ctx, "sub-1", WithSubscriptionInclude([]string{"versions"}), WithSubscriptionIncludedVersionFields([]string{"version", "state"}), WithSubscriptionVersionLimit(5))
	if err != nil || len(detail.Included) == 0 {
		t.Fatalf("detail included versions = %s, err=%v", detail.Included, err)
	}
}

func TestSubscriptionVersionCreateRequestOmitsAttributes(t *testing.T) {
	request := SubscriptionVersionCreateRequest{Data: SubscriptionVersionCreateData{Type: ResourceTypeSubscriptionVersions, Relationships: &SubscriptionVersionRelationships{Subscription: &Relationship{Data: ResourceData{Type: ResourceTypeSubscriptions, ID: "sub-1"}}}}}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "attributes") {
		t.Fatalf("version create must not contain attributes: %s", data)
	}
	if _, err := io.ReadAll(strings.NewReader(string(data))); err != nil {
		t.Fatal(err)
	}
}

func TestSubscriptionVersionClientRequiredValuesFailBeforeRequest(t *testing.T) {
	requestCount := 0
	client := newTestClient(t, func(req *http.Request) {
		requestCount++
	}, jsonResponse(http.StatusOK, `{}`))
	empty := ""
	falseValue := false
	tests := []struct {
		name string
		call func() error
	}{
		{name: "create version subscription ID", call: func() error { _, err := client.CreateSubscriptionVersion(context.Background(), " "); return err }},
		{name: "get subscription ID", call: func() error { _, err := client.GetSubscription(context.Background(), " "); return err }},
		{name: "get version ID", call: func() error { _, err := client.GetSubscriptionVersion(context.Background(), " "); return err }},
		{name: "list subscription ID", call: func() error { _, err := client.GetSubscriptionVersions(context.Background(), " "); return err }},
		{name: "version relationships subscription ID", call: func() error {
			_, err := client.GetSubscriptionVersionsRelationships(context.Background(), " ")
			return err
		}},
		{name: "localizations version ID", call: func() error {
			_, err := client.GetSubscriptionVersionLocalizations(context.Background(), " ")
			return err
		}},
		{name: "localization relationships version ID", call: func() error {
			_, err := client.GetSubscriptionVersionLocalizationsRelationships(context.Background(), " ")
			return err
		}},
		{name: "primary image version ID", call: func() error { _, err := client.GetSubscriptionVersionImage(context.Background(), " "); return err }},
		{name: "primary image relationship version ID", call: func() error {
			_, err := client.GetSubscriptionVersionImageRelationship(context.Background(), " ")
			return err
		}},
		{name: "images version ID", call: func() error { _, err := client.GetSubscriptionVersionImages(context.Background(), " "); return err }},
		{name: "image relationships version ID", call: func() error {
			_, err := client.GetSubscriptionVersionImagesRelationships(context.Background(), " ")
			return err
		}},
		{name: "create localization version ID", call: func() error {
			_, err := client.CreateSubscriptionLocalizationV2(context.Background(), " ", SubscriptionLocalizationV2CreateAttributes{Name: "Pro", Locale: "en-US"})
			return err
		}},
		{name: "create localization name", call: func() error {
			_, err := client.CreateSubscriptionLocalizationV2(context.Background(), "ver-1", SubscriptionLocalizationV2CreateAttributes{Name: " ", Locale: "en-US"})
			return err
		}},
		{name: "create localization locale", call: func() error {
			_, err := client.CreateSubscriptionLocalizationV2(context.Background(), "ver-1", SubscriptionLocalizationV2CreateAttributes{Name: "Pro", Locale: " "})
			return err
		}},
		{name: "get localization ID", call: func() error { _, err := client.GetSubscriptionLocalizationV2(context.Background(), " "); return err }},
		{name: "update localization ID", call: func() error {
			_, err := client.UpdateSubscriptionLocalizationV2(context.Background(), " ", SubscriptionLocalizationV2UpdateAttributes{Name: &NullableString{Value: &empty}})
			return err
		}},
		{name: "delete localization ID", call: func() error { return client.DeleteSubscriptionLocalizationV2(context.Background(), " ") }},
		{name: "create image version ID", call: func() error {
			_, err := client.CreateSubscriptionImageV2(context.Background(), " ", "image.png", 1)
			return err
		}},
		{name: "create image file name", call: func() error {
			_, err := client.CreateSubscriptionImageV2(context.Background(), "ver-1", " ", 1)
			return err
		}},
		{name: "create image zero size", call: func() error {
			_, err := client.CreateSubscriptionImageV2(context.Background(), "ver-1", "image.png", 0)
			return err
		}},
		{name: "create image negative size", call: func() error {
			_, err := client.CreateSubscriptionImageV2(context.Background(), "ver-1", "image.png", -1)
			return err
		}},
		{name: "get image ID", call: func() error { _, err := client.GetSubscriptionImageV2(context.Background(), " "); return err }},
		{name: "update image ID", call: func() error {
			_, err := client.UpdateSubscriptionImageV2(context.Background(), " ", SubscriptionImageV2UpdateAttributes{Uploaded: &falseValue})
			return err
		}},
		{name: "delete image ID", call: func() error { return client.DeleteSubscriptionImageV2(context.Background(), " ") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if requestCount != 0 {
		t.Fatalf("requests = %d, want 0", requestCount)
	}
}
