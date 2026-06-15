package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

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
				resp, err := c.GetSubscriptionLocalizations(ctx, "sub-1", WithSubscriptionLocalizationsLimit(5))
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
				resp, err := c.GetSubscriptionImages(ctx, "sub-1", WithSubscriptionImagesLimit(5))
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
				resp, err := c.GetSubscriptionIntroductoryOffers(ctx, "sub-1", WithSubscriptionIntroductoryOffersLimit(5))
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
				resp, err := c.GetSubscriptionPromotionalOffers(ctx, "sub-1", WithSubscriptionPromotionalOffersLimit(5))
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
				resp, err := c.GetSubscriptionPromotionalOfferPrices(ctx, "offer-1", WithSubscriptionPromotionalOfferPricesLimit(5))
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
				resp, err := c.GetSubscriptionOfferCodes(ctx, "sub-1", WithSubscriptionOfferCodesLimit(5))
				if err != nil {
					t.Fatalf("GetSubscriptionOfferCodes() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.Name != "Spring" {
					t.Fatalf("expected decoded offer code, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionOfferCodeCustomCodes",
			path:     "/v1/subscriptionOfferCodes/code-1/customCodes",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionOfferCodeCustomCodes","id":"custom-1","attributes":{"customCode":"SPRING"}}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionOfferCodeCustomCodes(ctx, "code-1", WithSubscriptionOfferCodeCustomCodesLimit(5))
				if err != nil {
					t.Fatalf("GetSubscriptionOfferCodeCustomCodes() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.CustomCode != "SPRING" {
					t.Fatalf("expected decoded custom code, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionOfferCodePrices",
			path:     "/v1/subscriptionOfferCodes/code-1/prices",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionOfferCodePrices","id":"price-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionOfferCodePrices(ctx, "code-1", WithSubscriptionOfferCodePricesLimit(5))
				if err != nil {
					t.Fatalf("GetSubscriptionOfferCodePrices() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "price-1" {
					t.Fatalf("expected decoded offer code price, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetSubscriptionPrices",
			path:     "/v1/subscriptions/sub-1/prices",
			limit:    "5",
			response: `{"data":[{"type":"subscriptionPrices","id":"price-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetSubscriptionPrices(ctx, "sub-1", WithSubscriptionPricesLimit(5))
				if err != nil {
					t.Fatalf("GetSubscriptionPrices() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "price-1" {
					t.Fatalf("expected decoded subscription price, got %+v", resp.Data)
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
				resp, err := c.GetSubscriptionGroupLocalizations(ctx, "group-1", WithSubscriptionGroupLocalizationsLimit(5))
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
				resp, err := c.GetSubscriptionLocalizations(ctx, "sub-1", WithSubscriptionLocalizationsNextURL(next))
				if err != nil {
					t.Fatalf("GetSubscriptionLocalizations() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.Locale != "en-US" {
					t.Fatalf("expected decoded next-url subscription localization, got %+v", resp.Data)
				}
			},
		},
		{
			name: "GetSubscriptionPrices",
			next: "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/prices?cursor=abc",
			call: func(t *testing.T, c *Client, next string) {
				resp, err := c.GetSubscriptionPrices(ctx, "sub-1", WithSubscriptionPricesNextURL(next))
				if err != nil {
					t.Fatalf("GetSubscriptionPrices() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "price-1" {
					t.Fatalf("expected decoded next-url subscription price, got %+v", resp.Data)
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
					WithSubscriptionPricePointsLimit(200),
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

			if tt.name == "GetSubscriptionPrices" {
				client = newTestClient(t, func(req *http.Request) {
					if req.URL.String() != tt.next {
						t.Fatalf("expected next URL %q, got %q", tt.next, req.URL.String())
					}
					assertAuthorized(t, req)
				}, jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-1"}]}`))
			}
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

func TestGetSubscriptionLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro","locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionLocalizations/loc-1" {
			t.Fatalf("expected path /v1/subscriptionLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetSubscriptionLocalization() error: %v", err)
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

func TestUpdateSubscriptionLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro+"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionLocalizations/loc-1" {
			t.Fatalf("expected path /v1/subscriptionLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload SubscriptionLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionLocalizations || payload.Data.ID != "loc-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes.Name == nil || *payload.Data.Attributes.Name != "Pro+" {
			t.Fatalf("expected name update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Pro+"
	attrs := SubscriptionLocalizationUpdateAttributes{Name: &name}
	if _, err := client.UpdateSubscriptionLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateSubscriptionLocalization() error: %v", err)
	}
}

func TestDeleteSubscriptionLocalization(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, `{}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionLocalizations/loc-1" {
			t.Fatalf("expected path /v1/subscriptionLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteSubscriptionLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteSubscriptionLocalization() error: %v", err)
	}
}

func TestGetSubscriptionImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":1234}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionImages/img-1" {
			t.Fatalf("expected path /v1/subscriptionImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionImage(context.Background(), "img-1"); err != nil {
		t.Fatalf("GetSubscriptionImage() error: %v", err)
	}
}

func TestCreateSubscriptionImage(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":1234}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionImages" {
			t.Fatalf("expected path /v1/subscriptionImages, got %s", req.URL.Path)
		}
		var payload SubscriptionImageCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionImages {
			t.Fatalf("expected type subscriptionImages, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.FileName != "image.png" || payload.Data.Attributes.FileSize != 1234 {
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

	if _, err := client.CreateSubscriptionImage(context.Background(), "sub-1", "image.png", 1234); err != nil {
		t.Fatalf("CreateSubscriptionImage() error: %v", err)
	}
}

func TestUpdateSubscriptionImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionImages","id":"img-1","attributes":{"uploaded":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionImages/img-1" {
			t.Fatalf("expected path /v1/subscriptionImages/img-1, got %s", req.URL.Path)
		}
		var payload SubscriptionImageUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionImages || payload.Data.ID != "img-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
			t.Fatalf("expected uploaded=true, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	uploaded := true
	attrs := SubscriptionImageUpdateAttributes{Uploaded: &uploaded}
	if _, err := client.UpdateSubscriptionImage(context.Background(), "img-1", attrs); err != nil {
		t.Fatalf("UpdateSubscriptionImage() error: %v", err)
	}
}

func TestDeleteSubscriptionImage(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, `{}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionImages/img-1" {
			t.Fatalf("expected path /v1/subscriptionImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteSubscriptionImage(context.Background(), "img-1"); err != nil {
		t.Fatalf("DeleteSubscriptionImage() error: %v", err)
	}
}

func TestGetSubscriptionIntroductoryOffer(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-1","attributes":{"duration":"ONE_MONTH","numberOfPeriods":1,"offerMode":"FREE_TRIAL"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionIntroductoryOffers/offer-1" {
			t.Fatalf("expected path /v1/subscriptionIntroductoryOffers/offer-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionIntroductoryOffer(context.Background(), "offer-1"); err != nil {
		t.Fatalf("GetSubscriptionIntroductoryOffer() error: %v", err)
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

func TestDeleteSubscriptionIntroductoryOffer(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, `{}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionIntroductoryOffers/offer-1" {
			t.Fatalf("expected path /v1/subscriptionIntroductoryOffers/offer-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteSubscriptionIntroductoryOffer(context.Background(), "offer-1"); err != nil {
		t.Fatalf("DeleteSubscriptionIntroductoryOffer() error: %v", err)
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
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPromotionalOffers","id":"offer-1","attributes":{"name":"Spring"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPromotionalOffers" {
			t.Fatalf("expected path /v1/subscriptionPromotionalOffers, got %s", req.URL.Path)
		}
		var payload SubscriptionPromotionalOfferCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionPromotionalOffers {
			t.Fatalf("expected type subscriptionPromotionalOffers, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Spring" || payload.Data.Attributes.OfferCode != "SPRING" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("unexpected subscription relationship: %+v", payload.Data.Relationships.Subscription.Data)
		}
		if len(payload.Data.Relationships.Prices.Data) != 1 || payload.Data.Relationships.Prices.Data[0].ID != "price-1" {
			t.Fatalf("unexpected price relationships: %+v", payload.Data.Relationships.Prices.Data)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := SubscriptionPromotionalOfferCreateAttributes{
		Name:            "Spring",
		OfferCode:       "SPRING",
		Duration:        SubscriptionOfferDurationOneMonth,
		OfferMode:       SubscriptionOfferModeFreeTrial,
		NumberOfPeriods: 1,
	}
	if _, err := client.CreateSubscriptionPromotionalOffer(context.Background(), "sub-1", attrs, []string{"price-1"}); err != nil {
		t.Fatalf("CreateSubscriptionPromotionalOffer() error: %v", err)
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

func TestDeleteSubscriptionPromotionalOffer(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, `{}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPromotionalOffers/offer-1" {
			t.Fatalf("expected path /v1/subscriptionPromotionalOffers/offer-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteSubscriptionPromotionalOffer(context.Background(), "offer-1"); err != nil {
		t.Fatalf("DeleteSubscriptionPromotionalOffer() error: %v", err)
	}
}

func TestDeleteSubscriptionPrice(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, `{}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPrices/price-1" {
			t.Fatalf("expected path /v1/subscriptionPrices/price-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteSubscriptionPrice(context.Background(), "price-1"); err != nil {
		t.Fatalf("DeleteSubscriptionPrice() error: %v", err)
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
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPricePoints","id":"price-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPricePoints/price-1" {
			t.Fatalf("expected path /v1/subscriptionPricePoints/price-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionPricePoint(context.Background(), "price-1"); err != nil {
		t.Fatalf("GetSubscriptionPricePoint() error: %v", err)
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

func TestCreateSubscriptionSubmission(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionSubmissions","id":"submit-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionSubmissions" {
			t.Fatalf("expected path /v1/subscriptionSubmissions, got %s", req.URL.Path)
		}
		var payload SubscriptionSubmissionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionSubmissions {
			t.Fatalf("expected type subscriptionSubmissions, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Subscription == nil {
			t.Fatalf("expected subscription relationship")
		}
		if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.Subscription.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateSubscriptionSubmission(context.Background(), "sub-1"); err != nil {
		t.Fatalf("CreateSubscriptionSubmission() error: %v", err)
	}
}

func TestCreateSubscriptionGroupSubmission(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionGroupSubmissions","id":"submit-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionGroupSubmissions" {
			t.Fatalf("expected path /v1/subscriptionGroupSubmissions, got %s", req.URL.Path)
		}
		var payload SubscriptionGroupSubmissionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionGroupSubmissions {
			t.Fatalf("expected type subscriptionGroupSubmissions, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.SubscriptionGroup == nil {
			t.Fatalf("expected subscriptionGroup relationship")
		}
		if payload.Data.Relationships.SubscriptionGroup.Data.ID != "group-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.SubscriptionGroup.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateSubscriptionGroupSubmission(context.Background(), "group-1"); err != nil {
		t.Fatalf("CreateSubscriptionGroupSubmission() error: %v", err)
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

func TestDeleteSubscriptionAppStoreReviewScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, `{}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionAppStoreReviewScreenshots/shot-1" {
			t.Fatalf("expected path /v1/subscriptionAppStoreReviewScreenshots/shot-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteSubscriptionAppStoreReviewScreenshot(context.Background(), "shot-1"); err != nil {
		t.Fatalf("DeleteSubscriptionAppStoreReviewScreenshot() error: %v", err)
	}
}

func TestGetSubscriptionAvailability(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionAvailabilities/avail-1" {
			t.Fatalf("expected path /v1/subscriptionAvailabilities/avail-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionAvailability(context.Background(), "avail-1"); err != nil {
		t.Fatalf("GetSubscriptionAvailability() error: %v", err)
	}
}

func TestGetSubscriptionAvailabilityForSubscription(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/sub-1/subscriptionAvailability" {
			t.Fatalf("expected path /v1/subscriptions/sub-1/subscriptionAvailability, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionAvailabilityForSubscription(context.Background(), "sub-1"); err != nil {
		t.Fatalf("GetSubscriptionAvailabilityForSubscription() error: %v", err)
	}
}

func TestGetSubscriptionAvailabilityAvailableTerritories(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionAvailabilities/avail-1/availableTerritories" {
			t.Fatalf("expected path /v1/subscriptionAvailabilities/avail-1/availableTerritories, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionAvailabilityAvailableTerritories(context.Background(), "avail-1", WithSubscriptionAvailabilityTerritoriesLimit(5)); err != nil {
		t.Fatalf("GetSubscriptionAvailabilityAvailableTerritories() error: %v", err)
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
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionAppStoreReviewScreenshotForSubscription(context.Background(), "sub-1"); err != nil {
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

func TestGetSubscriptionGroupLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionGroupLocalizations","id":"loc-1","attributes":{"name":"Premium","locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionGroupLocalizations/loc-1" {
			t.Fatalf("expected path /v1/subscriptionGroupLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionGroupLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetSubscriptionGroupLocalization() error: %v", err)
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

func TestUpdateSubscriptionGroupLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionGroupLocalizations","id":"loc-1","attributes":{"name":"Premium+"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionGroupLocalizations/loc-1" {
			t.Fatalf("expected path /v1/subscriptionGroupLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload SubscriptionGroupLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionGroupLocalizations || payload.Data.ID != "loc-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes.Name == nil || *payload.Data.Attributes.Name != "Premium+" {
			t.Fatalf("expected name update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Premium+"
	attrs := SubscriptionGroupLocalizationUpdateAttributes{Name: &name}
	if _, err := client.UpdateSubscriptionGroupLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateSubscriptionGroupLocalization() error: %v", err)
	}
}

func TestDeleteSubscriptionGroupLocalization(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, `{}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionGroupLocalizations/loc-1" {
			t.Fatalf("expected path /v1/subscriptionGroupLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteSubscriptionGroupLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteSubscriptionGroupLocalization() error: %v", err)
	}
}
