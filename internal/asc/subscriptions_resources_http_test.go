package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestSubscriptionPricePointClientsRejectBlankIDsBeforeHTTP(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
		want string
	}{
		{
			name: "subscription price points",
			call: func(client *Client) error {
				_, err := client.GetSubscriptionPricePoints(context.Background(), "  ")
				return err
			},
			want: "subscriptionID is required",
		},
		{
			name: "price point detail",
			call: func(client *Client) error {
				_, err := client.GetSubscriptionPricePoint(context.Background(), "  ")
				return err
			},
			want: "pricePointID is required",
		},
		{
			name: "equalizations",
			call: func(client *Client) error {
				_, err := client.GetSubscriptionPricePointEqualizations(context.Background(), "  ")
				return err
			},
			want: "pricePointID is required",
		},
		{
			name: "adjusted equalizations",
			call: func(client *Client) error {
				_, err := client.GetSubscriptionPricePointAdjustedEqualizations(context.Background(), "  ")
				return err
			},
			want: "pricePointID is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestCount := 0
			client := newTestClient(t, func(*http.Request) {
				requestCount++
			}, jsonResponse(http.StatusOK, `{}`))
			err := test.call(client)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
			if requestCount != 0 {
				t.Fatalf("request count = %d, want 0", requestCount)
			}
		})
	}
}

func TestSubscriptionListEndpoints_WithLimit(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		path     string
		limit    string
		response string
		call     func(*testing.T, *Client)
	}{
		{
			name:     "GetSubscriptionLocalizations",
			path:     "/v1/subscriptions/sub-1/subscriptionLocalizations",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro","locale":"en-US"}}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionLocalizations(ctx, "sub-1", Limit[SubscriptionLocalizationsOption](5))
				if err != nil {
					t.Fatalf("GetSubscriptionLocalizations() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.Locale != "en-US" {
					t.Fatalf("expected decoded subscription localization, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionImages",
			path:     "/v1/subscriptions/sub-1/images",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":1234}}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionImages(ctx, "sub-1", Limit[SubscriptionImagesOption](5))
				if err != nil {
					t.Fatalf("GetSubscriptionImages() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.FileName != "image.png" {
					t.Fatalf("expected decoded subscription image, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionIntroductoryOffers",
			path:     "/v1/subscriptions/sub-1/introductoryOffers",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-1","attributes":{"duration":"ONE_MONTH","numberOfPeriods":1,"offerMode":"FREE_TRIAL"}}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionIntroductoryOffers(ctx, "sub-1", Limit[SubscriptionIntroductoryOffersOption](5))
				if err != nil {
					t.Fatalf("GetSubscriptionIntroductoryOffers() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.OfferMode != SubscriptionOfferModeFreeTrial {
					t.Fatalf("expected decoded introductory offer, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionPromotionalOffers",
			path:     "/v1/subscriptions/sub-1/promotionalOffers",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionPromotionalOffers","id":"offer-1","attributes":{"name":"Spring","offerCode":"SPRING"}}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionPromotionalOffers(ctx, "sub-1", Limit[SubscriptionPromotionalOffersOption](5))
				if err != nil {
					t.Fatalf("GetSubscriptionPromotionalOffers() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.Name != "Spring" {
					t.Fatalf("expected decoded promotional offer, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionPromotionalOfferPrices",
			path:     "/v1/subscriptionPromotionalOffers/offer-1/prices",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionPromotionalOfferPrices","id":"price-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionPromotionalOfferPrices(ctx, "offer-1", Limit[SubscriptionPromotionalOfferPricesOption](5))
				if err != nil {
					t.Fatalf("GetSubscriptionPromotionalOfferPrices() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "price-1" {
					t.Fatalf("expected decoded promotional offer price, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionOfferCodes",
			path:     "/v1/subscriptions/sub-1/offerCodes",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionOfferCodes","id":"code-1","attributes":{"name":"Spring"}}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionOfferCodes(ctx, "sub-1", Limit[SubscriptionOfferCodesOption](5))
				if err != nil {
					t.Fatalf("GetSubscriptionOfferCodes() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.Name != "Spring" {
					t.Fatalf("expected decoded offer code, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionOfferCodePrices",
			path:     "/v1/subscriptionOfferCodes/code-1/prices",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionOfferCodePrices","id":"price-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionOfferCodePrices(ctx, "code-1", Limit[SubscriptionOfferCodePricesOption](5))
				if err != nil {
					t.Fatalf("GetSubscriptionOfferCodePrices() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "price-1" {
					t.Fatalf("expected decoded offer code price, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionPricePoints",
			path:     "/v1/subscriptions/sub-1/pricePoints",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionPricePoints","id":"point-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionPricePoints(ctx, "sub-1", WithSubscriptionPricePointsLimit(5))
				if err != nil {
					t.Fatalf("GetSubscriptionPricePoints() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "point-1" {
					t.Fatalf("expected decoded price point, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionPricePointEqualizations",
			path:     "/v1/subscriptionPricePoints/price-1/equalizations",
			limit:    "200",
			response: `{"data":[{"type":"subscriptionPricePoints","id":"eq-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionPricePointEqualizations(ctx, "price-1", WithSubscriptionPricePointsLimit(200))
				if err != nil {
					t.Fatalf("GetSubscriptionPricePointEqualizations() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "eq-1" {
					t.Fatalf("expected decoded equalized price point, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionGroupLocalizations",
			path:     "/v1/subscriptionGroups/group-1/subscriptionGroupLocalizations",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionGroupLocalizations","id":"loc-1","attributes":{"name":"Premium","locale":"en-US"}}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionGroupLocalizations(ctx, "group-1", Limit[SubscriptionGroupLocalizationsOption](5))
				if err != nil {
					t.Fatalf("GetSubscriptionGroupLocalizations() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.Name != "Premium" {
					t.Fatalf("expected decoded group localization, got %+v", resp.Data)
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

func TestSubscriptionListEndpoints_UseNextURL(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		next string
		call func(*testing.T, *Client, string)
	}{
		{
			name: "GetSubscriptionLocalizations",
			next: "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/subscriptionLocalizations?cursor=abc",
			call: func(t *testing.T, c *Client, next string) {
				resp, err := c.GetSubscriptionLocalizations(ctx, "sub-1", NextURL[SubscriptionLocalizationsOption](next))
				if err != nil {
					t.Fatalf("GetSubscriptionLocalizations() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.Locale != "en-US" {
					t.Fatalf("expected decoded next-url subscription localization, got %+v", resp.Data)
				}
			},
		},
		{
			name: "GetSubscriptionPricePointEqualizations",
			next: "https://api.appstoreconnect.apple.com/v1/subscriptionPricePoints/price-1/equalizations?cursor=abc&limit=200",
			call: func(t *testing.T, c *Client, next string) {
				resp, err := c.GetSubscriptionPricePointEqualizations(
					ctx,
					"price-1",
					WithSubscriptionPricePointsNextURL(next),
				)
				if err != nil {
					t.Fatalf("GetSubscriptionPricePointEqualizations() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "eq-1" {
					t.Fatalf("expected decoded next-url price point equalization, got %+v", resp.Data)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.URL.String() != tt.next {
					t.Fatalf("expected next URL %q, got %q", tt.next, req.URL.String())
				}
				assertAuthorized(t, req)
			}, jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro","locale":"en-US"}}]}`))

			if tt.name == "GetSubscriptionPricePointEqualizations" {
				client = newTestClient(t, func(req *http.Request) {
					if req.URL.String() != tt.next {
						t.Fatalf("expected next URL %q, got %q", tt.next, req.URL.String())
					}
					assertAuthorized(t, req)
				}, jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"eq-1"}]}`))
			}

			tt.call(t, client, tt.next)
		})
	}
}

func TestSubscriptionPricePointNextURLRejectsQueryModifiers(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		t.Fatalf("unexpected request: %s", req.URL.String())
	}, jsonResponse(http.StatusOK, `{"data":[]}`))
	next := "https://api.appstoreconnect.apple.com/v1/subscriptionPricePoints/price-1/equalizations?cursor=abc&limit=200"

	if _, err := client.GetSubscriptionPricePointEqualizations(
		context.Background(),
		"price-1",
		WithSubscriptionPricePointsNextURL(next),
		WithSubscriptionPricePointsLimit(17),
	); err == nil || !strings.Contains(err.Error(), "next URL cannot be combined") {
		t.Fatalf("error = %v, want next URL conflict", err)
	}
}

func TestCreateSubscriptionLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro","locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionLocalizations" {
			t.Fatalf("expected path /v1/subscriptionLocalizations, got %s", req.URL.Path)
		}
		var payload SubscriptionLocalizationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionLocalizations {
			t.Fatalf("expected type subscriptionLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Pro" || payload.Data.Attributes.Locale != "en-US" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Subscription == nil {
			t.Fatalf("expected subscription relationship")
		}
		if payload.Data.Relationships.Subscription.Data.Type != ResourceTypeSubscriptions || payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.Subscription.Data)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := SubscriptionLocalizationCreateAttributes{
		Name:   "Pro",
		Locale: "en-US",
	}
	if _, err := client.CreateSubscriptionLocalization(context.Background(), "sub-1", attrs); err != nil {
		t.Fatalf("CreateSubscriptionLocalization() error: %v", err)
	}
}

func TestCreateSubscriptionIntroductoryOffer(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-1","attributes":{"duration":"ONE_MONTH","numberOfPeriods":1,"offerMode":"FREE_TRIAL"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionIntroductoryOffers" {
			t.Fatalf("expected path /v1/subscriptionIntroductoryOffers, got %s", req.URL.Path)
		}
		var payload SubscriptionIntroductoryOfferCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionIntroductoryOffers {
			t.Fatalf("expected type subscriptionIntroductoryOffers, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Duration != SubscriptionOfferDurationOneMonth || payload.Data.Attributes.OfferMode != SubscriptionOfferModeFreeTrial || payload.Data.Attributes.NumberOfPeriods != 1 {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Subscription == nil {
			t.Fatalf("expected subscription relationship")
		}
		if payload.Data.Relationships.Subscription.Data.Type != ResourceTypeSubscriptions || payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.Subscription.Data)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := SubscriptionIntroductoryOfferCreateAttributes{
		Duration:        SubscriptionOfferDurationOneMonth,
		OfferMode:       SubscriptionOfferModeFreeTrial,
		NumberOfPeriods: 1,
	}
	if _, err := client.CreateSubscriptionIntroductoryOffer(context.Background(), "sub-1", attrs, "", ""); err != nil {
		t.Fatalf("CreateSubscriptionIntroductoryOffer() error: %v", err)
	}
}

func TestUpdateSubscriptionIntroductoryOffer(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-1","attributes":{"endDate":"2026-02-01"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionIntroductoryOffers/offer-1" {
			t.Fatalf("expected path /v1/subscriptionIntroductoryOffers/offer-1, got %s", req.URL.Path)
		}
		var payload SubscriptionIntroductoryOfferUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionIntroductoryOffers || payload.Data.ID != "offer-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes.EndDate == nil || *payload.Data.Attributes.EndDate != "2026-02-01" {
			t.Fatalf("expected endDate update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	endDate := "2026-02-01"
	attrs := SubscriptionIntroductoryOfferUpdateAttributes{EndDate: &endDate}
	if _, err := client.UpdateSubscriptionIntroductoryOffer(context.Background(), "offer-1", attrs); err != nil {
		t.Fatalf("UpdateSubscriptionIntroductoryOffer() error: %v", err)
	}
}

func TestGetSubscriptionPromotionalOffer(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPromotionalOffers","id":"offer-1","attributes":{"name":"Spring"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPromotionalOffers/offer-1" {
			t.Fatalf("expected path /v1/subscriptionPromotionalOffers/offer-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionPromotionalOffer(context.Background(), "offer-1"); err != nil {
		t.Fatalf("GetSubscriptionPromotionalOffer() error: %v", err)
	}
}

func TestCreateSubscriptionPromotionalOffer(t *testing.T) {
	tests := []struct {
		name         string
		mode         SubscriptionOfferMode
		prices       []SubscriptionPromotionalOfferPrice
		expectedBody string
	}{
		{
			name:         "compound price with both relationships",
			mode:         SubscriptionOfferModePayAsYouGo,
			prices:       []SubscriptionPromotionalOfferPrice{{TerritoryID: "USA", PricePointID: "price-1"}},
			expectedBody: `{"data":{"type":"subscriptionPromotionalOffers","attributes":{"duration":"ONE_MONTH","name":"Spring","numberOfPeriods":1,"offerCode":"SPRING","offerMode":"PAY_AS_YOU_GO"},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}},"prices":{"data":[{"type":"subscriptionPromotionalOfferPrices","id":"${local-price-1}"}]}}},"included":[{"type":"subscriptionPromotionalOfferPrices","id":"${local-price-1}","relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"price-1"}}}}]}`,
		},
		{
			name:         "paid mode with territory only",
			mode:         SubscriptionOfferModePayUpFront,
			prices:       []SubscriptionPromotionalOfferPrice{{TerritoryID: "FRA"}},
			expectedBody: `{"data":{"type":"subscriptionPromotionalOffers","attributes":{"duration":"ONE_MONTH","name":"Spring","numberOfPeriods":1,"offerCode":"SPRING","offerMode":"PAY_UP_FRONT"},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}},"prices":{"data":[{"type":"subscriptionPromotionalOfferPrices","id":"${local-price-1}"}]}}},"included":[{"type":"subscriptionPromotionalOfferPrices","id":"${local-price-1}","relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}}]}`,
		},
		{
			name:         "free trial with price point",
			mode:         SubscriptionOfferModeFreeTrial,
			prices:       []SubscriptionPromotionalOfferPrice{{TerritoryID: "DEU", PricePointID: "price-2"}},
			expectedBody: `{"data":{"type":"subscriptionPromotionalOffers","attributes":{"duration":"ONE_MONTH","name":"Spring","numberOfPeriods":1,"offerCode":"SPRING","offerMode":"FREE_TRIAL"},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}},"prices":{"data":[{"type":"subscriptionPromotionalOfferPrices","id":"${local-price-1}"}]}}},"included":[{"type":"subscriptionPromotionalOfferPrices","id":"${local-price-1}","relationships":{"territory":{"data":{"type":"territories","id":"DEU"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"price-2"}}}}]}`,
		},
		{
			name:         "price point relationship without territory",
			mode:         SubscriptionOfferModeFreeTrial,
			prices:       []SubscriptionPromotionalOfferPrice{{PricePointID: "price-3"}},
			expectedBody: `{"data":{"type":"subscriptionPromotionalOffers","attributes":{"duration":"ONE_MONTH","name":"Spring","numberOfPeriods":1,"offerCode":"SPRING","offerMode":"FREE_TRIAL"},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}},"prices":{"data":[{"type":"subscriptionPromotionalOfferPrices","id":"${local-price-1}"}]}}},"included":[{"type":"subscriptionPromotionalOfferPrices","id":"${local-price-1}","relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"price-3"}}}}]}`,
		},
		{
			name:         "legacy existing price reference",
			mode:         SubscriptionOfferModePayUpFront,
			prices:       []SubscriptionPromotionalOfferPrice{{ID: "price-legacy"}},
			expectedBody: `{"data":{"type":"subscriptionPromotionalOffers","attributes":{"duration":"ONE_MONTH","name":"Spring","numberOfPeriods":1,"offerCode":"SPRING","offerMode":"PAY_UP_FRONT"},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}},"prices":{"data":[{"type":"subscriptionPromotionalOfferPrices","id":"price-legacy"}]}}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPromotionalOffers","id":"offer-1","attributes":{"name":"Spring"}}}`)
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodPost {
					t.Fatalf("expected POST, got %s", req.Method)
				}
				if req.URL.Path != "/v1/subscriptionPromotionalOffers" {
					t.Fatalf("expected path /v1/subscriptionPromotionalOffers, got %s", req.URL.Path)
				}
				var got map[string]any
				if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}
				var want map[string]any
				if err := json.Unmarshal([]byte(test.expectedBody), &want); err != nil {
					t.Fatalf("failed to decode expected body: %v", err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("unexpected request body:\n got: %#v\nwant: %#v", got, want)
				}
				assertAuthorized(t, req)
			}, response)

			attrs := SubscriptionPromotionalOfferCreateAttributes{
				Name:            "Spring",
				OfferCode:       "SPRING",
				Duration:        SubscriptionOfferDurationOneMonth,
				OfferMode:       test.mode,
				NumberOfPeriods: 1,
			}
			if _, err := client.CreateSubscriptionPromotionalOffer(context.Background(), "sub-1", attrs, test.prices); err != nil {
				t.Fatalf("CreateSubscriptionPromotionalOffer() error: %v", err)
			}
		})
	}
}

func TestCreateSubscriptionPromotionalOfferValidatesInlinePricesBeforeHTTP(t *testing.T) {
	tests := []struct {
		name   string
		mode   SubscriptionOfferMode
		prices []SubscriptionPromotionalOfferPrice
		want   string
	}{
		{name: "missing prices", mode: SubscriptionOfferModePayAsYouGo, want: "at least one price is required"},
		{name: "empty price", mode: SubscriptionOfferModePayAsYouGo, prices: []SubscriptionPromotionalOfferPrice{{}}, want: "price reference ID or inline relationship is required"},
		{name: "reference with inline relationships", mode: SubscriptionOfferModePayAsYouGo, prices: []SubscriptionPromotionalOfferPrice{{ID: "price-1", TerritoryID: "USA"}}, want: "price reference ID must not be combined with inline relationships"},
		{name: "mixed reference and inline prices", mode: SubscriptionOfferModePayAsYouGo, prices: []SubscriptionPromotionalOfferPrice{{ID: "price-1"}, {TerritoryID: "USA"}}, want: "price inputs must not mix existing IDs with inline relationships"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &Client{}
			_, err := client.CreateSubscriptionPromotionalOffer(
				context.Background(),
				"sub-1",
				SubscriptionPromotionalOfferCreateAttributes{OfferMode: test.mode},
				test.prices,
			)
			if err == nil || err.Error() != test.want {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}

func TestUpdateSubscriptionPromotionalOffer(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPromotionalOffers","id":"offer-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPromotionalOffers/offer-1" {
			t.Fatalf("expected path /v1/subscriptionPromotionalOffers/offer-1, got %s", req.URL.Path)
		}
		var payload SubscriptionPromotionalOfferUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionPromotionalOffers || payload.Data.ID != "offer-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Relationships == nil || len(payload.Data.Relationships.Prices.Data) != 1 {
			t.Fatalf("expected prices relationship")
		}
		if payload.Data.Relationships.Prices.Data[0].ID != "price-1" {
			t.Fatalf("unexpected price relationship: %+v", payload.Data.Relationships.Prices.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.UpdateSubscriptionPromotionalOffer(context.Background(), "offer-1", []string{"price-1"}); err != nil {
		t.Fatalf("UpdateSubscriptionPromotionalOffer() error: %v", err)
	}
}

func TestGetSubscriptionOfferCode(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionOfferCodes","id":"code-1","attributes":{"name":"Spring"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodes/code-1" {
			t.Fatalf("expected path /v1/subscriptionOfferCodes/code-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionOfferCode(context.Background(), "code-1"); err != nil {
		t.Fatalf("GetSubscriptionOfferCode() error: %v", err)
	}
}

func TestCreateSubscriptionOfferCode(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionOfferCodes","id":"code-1","attributes":{"name":"Spring"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodes" {
			t.Fatalf("expected path /v1/subscriptionOfferCodes, got %s", req.URL.Path)
		}
		var payload SubscriptionOfferCodeCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionOfferCodes {
			t.Fatalf("expected type subscriptionOfferCodes, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Spring" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("unexpected subscription relationship: %+v", payload.Data.Relationships.Subscription.Data)
		}
		if len(payload.Data.Relationships.Prices.Data) != 1 || payload.Data.Relationships.Prices.Data[0].ID != "${local-price-1}" {
			t.Fatalf("unexpected price relationships: %+v", payload.Data.Relationships.Prices.Data)
		}
		if len(payload.Included) != 1 {
			t.Fatalf("expected one included price, got %d", len(payload.Included))
		}
		if payload.Included[0].Relationships.Territory.Data.ID != "USA" {
			t.Fatalf("expected territory USA, got %q", payload.Included[0].Relationships.Territory.Data.ID)
		}
		if payload.Included[0].Relationships.SubscriptionPricePoint.Data.ID != "price-1" {
			t.Fatalf("expected price point price-1, got %q", payload.Included[0].Relationships.SubscriptionPricePoint.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := SubscriptionOfferCodeCreateAttributes{
		Name:                  "Spring",
		OfferEligibility:      SubscriptionOfferEligibilityStackWithIntroOffers,
		CustomerEligibilities: []SubscriptionCustomerEligibility{SubscriptionCustomerEligibilityNew},
		Duration:              SubscriptionOfferDurationOneMonth,
		OfferMode:             SubscriptionOfferModePayAsYouGo,
		NumberOfPeriods:       1,
	}
	prices := []SubscriptionOfferCodePrice{
		{
			TerritoryID:  "USA",
			PricePointID: "price-1",
		},
	}
	if _, err := client.CreateSubscriptionOfferCode(context.Background(), "sub-1", attrs, prices); err != nil {
		t.Fatalf("CreateSubscriptionOfferCode() error: %v", err)
	}
}

func TestCreateSubscriptionOfferCodeFreeTrial(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionOfferCodes","id":"code-ft-1","attributes":{"name":"One Year Free"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodes" {
			t.Fatalf("expected path /v1/subscriptionOfferCodes, got %s", req.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		data, ok := payload["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected data object, got %T", payload["data"])
		}
		attributes, ok := data["attributes"].(map[string]any)
		if !ok {
			t.Fatalf("expected attributes object, got %T", data["attributes"])
		}
		if attributes["offerMode"] != string(SubscriptionOfferModeFreeTrial) {
			t.Fatalf("expected offer mode FREE_TRIAL, got %#v", attributes["offerMode"])
		}
		relationships, ok := data["relationships"].(map[string]any)
		if !ok {
			t.Fatalf("expected relationships object, got %T", data["relationships"])
		}
		pricesRelationship, ok := relationships["prices"].(map[string]any)
		if !ok {
			t.Fatalf("expected prices relationship for FREE_TRIAL, got %#v", relationships["prices"])
		}
		priceRefs, ok := pricesRelationship["data"].([]any)
		if !ok || len(priceRefs) != 1 {
			t.Fatalf("expected one price relationship, got %#v", pricesRelationship["data"])
		}
		included, ok := payload["included"].([]any)
		if !ok || len(included) != 1 {
			t.Fatalf("expected one included price, got %#v", payload["included"])
		}
		includedPrice, ok := included[0].(map[string]any)
		if !ok {
			t.Fatalf("expected included price object, got %T", included[0])
		}
		priceRelationships, ok := includedPrice["relationships"].(map[string]any)
		if !ok {
			t.Fatalf("expected included price relationships, got %T", includedPrice["relationships"])
		}
		territory, ok := priceRelationships["territory"].(map[string]any)
		if !ok {
			t.Fatalf("expected territory relationship, got %#v", priceRelationships["territory"])
		}
		territoryData, ok := territory["data"].(map[string]any)
		if !ok || territoryData["id"] != "DEU" {
			t.Fatalf("expected territory DEU, got %#v", territory["data"])
		}
		if _, ok := priceRelationships["subscriptionPricePoint"]; ok {
			t.Fatalf("expected subscriptionPricePoint to be omitted, got %#v", priceRelationships["subscriptionPricePoint"])
		}
		assertAuthorized(t, req)
	}, response)

	attrs := SubscriptionOfferCodeCreateAttributes{
		Name:                  "One Year Free",
		OfferEligibility:      SubscriptionOfferEligibilityStackWithIntroOffers,
		CustomerEligibilities: []SubscriptionCustomerEligibility{SubscriptionCustomerEligibilityNew},
		Duration:              SubscriptionOfferDurationOneYear,
		OfferMode:             SubscriptionOfferModeFreeTrial,
		NumberOfPeriods:       1,
	}
	prices := []SubscriptionOfferCodePrice{{TerritoryID: "DEU"}}
	if _, err := client.CreateSubscriptionOfferCode(context.Background(), "sub-1", attrs, prices); err != nil {
		t.Fatalf("CreateSubscriptionOfferCode() error: %v", err)
	}
}

func TestCreateSubscriptionOfferCodeFreeTrialRejectsPricePoint(t *testing.T) {
	attrs := SubscriptionOfferCodeCreateAttributes{
		OfferMode: SubscriptionOfferModeFreeTrial,
	}
	prices := []SubscriptionOfferCodePrice{{TerritoryID: "USA", PricePointID: "price-1"}}
	client := &Client{} // no HTTP needed — validation fires before any call
	_, err := client.CreateSubscriptionOfferCode(context.Background(), "sub-1", attrs, prices)
	if err == nil {
		t.Fatal("expected error for FREE_TRIAL with a price point, got nil")
	}
	const want = "price point must not be set for FREE_TRIAL offer mode"
	if err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}
}

func TestCreateSubscriptionOfferCodeFreeTrialRequiresPrice(t *testing.T) {
	attrs := SubscriptionOfferCodeCreateAttributes{
		OfferMode: SubscriptionOfferModeFreeTrial,
	}
	client := &Client{} // no HTTP needed — validation fires before any call
	_, err := client.CreateSubscriptionOfferCode(context.Background(), "sub-1", attrs, nil)
	if err == nil {
		t.Fatal("expected error for FREE_TRIAL without prices, got nil")
	}
	const want = "at least one price is required"
	if err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}
}

func TestUpdateSubscriptionOfferCode(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionOfferCodes","id":"code-1","attributes":{"active":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodes/code-1" {
			t.Fatalf("expected path /v1/subscriptionOfferCodes/code-1, got %s", req.URL.Path)
		}
		var payload SubscriptionOfferCodeUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionOfferCodes || payload.Data.ID != "code-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes.Active == nil || !*payload.Data.Attributes.Active {
			t.Fatalf("expected active=true, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	active := true
	attrs := SubscriptionOfferCodeUpdateAttributes{Active: &active}
	if _, err := client.UpdateSubscriptionOfferCode(context.Background(), "code-1", attrs); err != nil {
		t.Fatalf("UpdateSubscriptionOfferCode() error: %v", err)
	}
}

func TestGetSubscriptionPricePoints_WithTerritoryFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"price-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
			t.Fatalf("expected path /v1/subscriptions/sub-1/pricePoints, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("filter[territory]") != "USA" {
			t.Fatalf("expected filter[territory]=USA, got %q", req.URL.Query().Get("filter[territory]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionPricePoints(
		context.Background(),
		"sub-1",
		WithSubscriptionPricePointsTerritory("USA"),
	); err != nil {
		t.Fatalf("GetSubscriptionPricePoints() error: %v", err)
	}
}

func TestGetSubscriptionPricePoint(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPricePoints","id":"price-1","relationships":{"adjustedEqualizations":{"links":{"related":"https://api.appstoreconnect.apple.com/v1/subscriptionPricePoints/price-1/adjustedEqualizations"}}}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPricePoints/price-1" {
			t.Fatalf("expected path /v1/subscriptionPricePoints/price-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetSubscriptionPricePoint(context.Background(), "price-1")
	if err != nil {
		t.Fatalf("GetSubscriptionPricePoint() error: %v", err)
	}
	var relationships map[string]json.RawMessage
	if err := json.Unmarshal(resp.Data.Relationships, &relationships); err != nil {
		t.Fatalf("decode relationships: %v", err)
	}
	if _, ok := relationships["adjustedEqualizations"]; !ok {
		t.Fatalf("expected adjustedEqualizations relationship, got %s", resp.Data.Relationships)
	}
}

func TestGetSubscriptionPricePointEqualizations(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"eq-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPricePoints/price-1/equalizations" {
			t.Fatalf("expected path /v1/subscriptionPricePoints/price-1/equalizations, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionPricePointEqualizations(context.Background(), "price-1"); err != nil {
		t.Fatalf("GetSubscriptionPricePointEqualizations() error: %v", err)
	}
}

func TestGetSubscriptionPricePointAdjustedEqualizations(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"adjusted-1","attributes":{"customerPrice":"4.99"}}],"links":{}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPricePoints/price-1/adjustedEqualizations" {
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
		query := req.URL.Query()
		for key, want := range map[string]string{
			"filter[territory]":               "USA,FRA",
			"filter[subscription]":            "sub-1,sub-2",
			"filter[upfrontPricePointId]":     "upfront-1,upfront-2",
			"filter[planType]":                "MONTHLY,UPFRONT",
			"fields[subscriptionPricePoints]": "customerPrice,adjustedEqualizations",
			"fields[territories]":             "currency",
			"include":                         "territory",
			"limit":                           "50",
		} {
			if got := query.Get(key); got != want {
				t.Fatalf("%s=%q, want %q", key, got, want)
			}
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetSubscriptionPricePointAdjustedEqualizations(
		context.Background(),
		"price-1",
		WithSubscriptionPricePointsTerritories([]string{"USA", "FRA"}),
		WithSubscriptionPricePointsSubscriptions([]string{"sub-1", "sub-2"}),
		WithSubscriptionPricePointsUpfrontPricePointIDs([]string{"upfront-1", "upfront-2"}),
		WithSubscriptionPricePointsPlanTypes([]string{"MONTHLY", "UPFRONT"}),
		WithSubscriptionPricePointsFields([]string{"customerPrice", "adjustedEqualizations"}),
		WithSubscriptionPricePointsTerritoryFields([]string{"currency"}),
		WithSubscriptionPricePointsInclude([]string{"territory"}),
		WithSubscriptionPricePointsLimit(50),
	)
	if err != nil {
		t.Fatalf("GetSubscriptionPricePointAdjustedEqualizations() error: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "adjusted-1" || resp.Data[0].Attributes.CustomerPrice != "4.99" {
		t.Fatalf("unexpected response: %#v", resp.Data)
	}
}

func TestGetSubscriptionPricePointAdjustedEqualizationsRequiresIDWithoutNext(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		t.Fatalf("unexpected request: %s", req.URL.String())
	}, jsonResponse(http.StatusOK, `{"data":[],"links":{}}`))

	if _, err := client.GetSubscriptionPricePointAdjustedEqualizations(context.Background(), ""); err == nil {
		t.Fatal("expected missing price point ID error")
	}
}

func TestGetSubscriptionAppStoreReviewScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"shot.png","fileSize":1234}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionAppStoreReviewScreenshots/shot-1" {
			t.Fatalf("expected path /v1/subscriptionAppStoreReviewScreenshots/shot-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionAppStoreReviewScreenshot(context.Background(), "shot-1"); err != nil {
		t.Fatalf("GetSubscriptionAppStoreReviewScreenshot() error: %v", err)
	}
}

func TestCreateSubscriptionAppStoreReviewScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"shot.png","fileSize":1234}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionAppStoreReviewScreenshots" {
			t.Fatalf("expected path /v1/subscriptionAppStoreReviewScreenshots, got %s", req.URL.Path)
		}
		var payload SubscriptionAppStoreReviewScreenshotCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionAppStoreReviewScreenshots {
			t.Fatalf("expected type subscriptionAppStoreReviewScreenshots, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.FileName != "shot.png" || payload.Data.Attributes.FileSize != 1234 {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Subscription == nil {
			t.Fatalf("expected subscription relationship")
		}
		if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.Subscription.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateSubscriptionAppStoreReviewScreenshot(context.Background(), "sub-1", "shot.png", 1234); err != nil {
		t.Fatalf("CreateSubscriptionAppStoreReviewScreenshot() error: %v", err)
	}
}

func TestUpdateSubscriptionAppStoreReviewScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"uploaded":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionAppStoreReviewScreenshots/shot-1" {
			t.Fatalf("expected path /v1/subscriptionAppStoreReviewScreenshots/shot-1, got %s", req.URL.Path)
		}
		var payload SubscriptionAppStoreReviewScreenshotUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionAppStoreReviewScreenshots || payload.Data.ID != "shot-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
			t.Fatalf("expected uploaded=true, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	uploaded := true
	attrs := SubscriptionAppStoreReviewScreenshotUpdateAttributes{Uploaded: &uploaded}
	if _, err := client.UpdateSubscriptionAppStoreReviewScreenshot(context.Background(), "shot-1", attrs); err != nil {
		t.Fatalf("UpdateSubscriptionAppStoreReviewScreenshot() error: %v", err)
	}
}

func TestGetSubscriptionAppStoreReviewScreenshotForSubscription(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"shot.png"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/sub-1/appStoreReviewScreenshot" {
			t.Fatalf("expected path /v1/subscriptions/sub-1/appStoreReviewScreenshot, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("fields[subscriptionAppStoreReviewScreenshots]"); got != "fileName,fileSize,sourceFileChecksum,uploadOperations,assetDeliveryState" {
			t.Fatalf("unexpected screenshot fields: %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionAppStoreReviewScreenshotForSubscription(
		context.Background(),
		"sub-1",
		WithSubscriptionAppStoreReviewScreenshotFields([]string{"fileName", "fileSize", "sourceFileChecksum", "uploadOperations", "assetDeliveryState"}),
	); err != nil {
		t.Fatalf("GetSubscriptionAppStoreReviewScreenshotForSubscription() error: %v", err)
	}
}

func TestGetSubscriptionPromotedPurchase(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"promotedPurchases","id":"promo-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/sub-1/promotedPurchase" {
			t.Fatalf("expected path /v1/subscriptions/sub-1/promotedPurchase, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionPromotedPurchase(context.Background(), "sub-1"); err != nil {
		t.Fatalf("GetSubscriptionPromotedPurchase() error: %v", err)
	}
}

func TestCreateSubscriptionGroupLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionGroupLocalizations","id":"loc-1","attributes":{"name":"Premium","locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionGroupLocalizations" {
			t.Fatalf("expected path /v1/subscriptionGroupLocalizations, got %s", req.URL.Path)
		}
		var payload SubscriptionGroupLocalizationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionGroupLocalizations {
			t.Fatalf("expected type subscriptionGroupLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Premium" || payload.Data.Attributes.Locale != "en-US" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.SubscriptionGroup == nil {
			t.Fatalf("expected subscriptionGroup relationship")
		}
		if payload.Data.Relationships.SubscriptionGroup.Data.ID != "group-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.SubscriptionGroup.Data)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := SubscriptionGroupLocalizationCreateAttributes{
		Name:   "Premium",
		Locale: "en-US",
	}
	if _, err := client.CreateSubscriptionGroupLocalization(context.Background(), "group-1", attrs); err != nil {
		t.Fatalf("CreateSubscriptionGroupLocalization() error: %v", err)
	}
}
