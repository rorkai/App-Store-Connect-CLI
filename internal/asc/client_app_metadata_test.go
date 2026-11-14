package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestGetAppSearchKeywords_SendsRequestWithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appKeywords","id":"keyword-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/searchKeywords" {
			t.Fatalf("expected path /v1/apps/app-1/searchKeywords, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[platform]") != "IOS,MAC_OS" {
			t.Fatalf("expected filter[platform]=IOS,MAC_OS, got %q", values.Get("filter[platform]"))
		}
		if values.Get("filter[locale]") != "en-US,ja" {
			t.Fatalf("expected filter[locale]=en-US,ja, got %q", values.Get("filter[locale]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetAppSearchKeywords(
		context.Background(),
		"app-1",
		WithAppSearchKeywordsPlatforms([]string{"ios", "MAC_OS"}),
		WithAppSearchKeywordsLocales([]string{"en-US", "ja"}),
		Limit[AppSearchKeywordsOption](5),
	)
	if err != nil {
		t.Fatalf("GetAppSearchKeywords() error: %v", err)
	}
}

func TestGetAppSearchKeywords_RequiresAppID(t *testing.T) {
	client := newTestClient(t, nil, jsonResponse(http.StatusOK, `{"data":[]}`))
	if _, err := client.GetAppSearchKeywords(context.Background(), ""); err == nil {
		t.Fatal("expected error for missing app ID")
	}
}

func TestAddAppStoreVersionLocalizationSearchKeywords_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, `{}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1/relationships/searchKeywords" {
			t.Fatalf("expected path /v1/appStoreVersionLocalizations/loc-1/relationships/searchKeywords, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		var payload RelationshipRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 keywords, got %d", len(payload.Data))
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AddAppStoreVersionLocalizationSearchKeywords(context.Background(), "loc-1", []string{"kw-1", "kw-2"}); err != nil {
		t.Fatalf("AddAppStoreVersionLocalizationSearchKeywords() error: %v", err)
	}
}

func TestDeleteAppStoreVersionLocalizationSearchKeywords_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, `{}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1/relationships/searchKeywords" {
			t.Fatalf("expected path /v1/appStoreVersionLocalizations/loc-1/relationships/searchKeywords, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		var payload RelationshipRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		if len(payload.Data) != 1 {
			t.Fatalf("expected 1 keyword, got %d", len(payload.Data))
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppStoreVersionLocalizationSearchKeywords(context.Background(), "loc-1", []string{"kw-1"}); err != nil {
		t.Fatalf("DeleteAppStoreVersionLocalizationSearchKeywords() error: %v", err)
	}
}

func TestGetAppStoreVersionAgeRatingDeclarationRelationship_UsesAppInfoRelationship(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	var seenPaths []string
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", req.Method)
				}
				assertAuthorized(t, req)

				seenPaths = append(seenPaths, req.URL.Path)
				switch req.URL.Path {
				case "/v1/appStoreVersions/version-1":
					if got := req.URL.Query().Get("include"); got != "app" {
						t.Fatalf("expected include=app, got %q", got)
					}
					return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"appVersionState":"REPLACED_WITH_NEW_VERSION"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`), nil
				case "/v1/apps/app-1/appInfos":
					return jsonResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-draft","attributes":{"state":"PREPARE_FOR_SUBMISSION"}},{"type":"appInfos","id":"info-live","attributes":{"state":"REPLACED_WITH_NEW_INFO"}}]}`), nil
				case "/v1/appInfos/info-live/relationships/ageRatingDeclaration":
					return jsonResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"age-1"}}`), nil
				default:
					t.Fatalf("unexpected path %s", req.URL.Path)
					return nil, nil
				}
			}),
		},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}

	resp, err := client.GetAppStoreVersionAgeRatingDeclarationRelationship(context.Background(), "version-1")
	if err != nil {
		t.Fatalf("GetAppStoreVersionAgeRatingDeclarationRelationship() error: %v", err)
	}
	if resp.Data.ID != "age-1" {
		t.Fatalf("expected linkage id age-1, got %q", resp.Data.ID)
	}
	if len(seenPaths) != 3 {
		t.Fatalf("expected 3 requests, got %d (%v)", len(seenPaths), seenPaths)
	}
}

func TestGetAppStoreVersionRelationshipLists_SendsRequestWithLimit(t *testing.T) {
	tests := []struct {
		name string
		call func(ctx context.Context, client *Client) error
		path string
	}{
		{
			name: "experiments v1",
			call: func(ctx context.Context, client *Client) error {
				_, err := client.GetAppStoreVersionExperimentsRelationships(ctx, "version-1", Limit[LinkagesOption](20))
				return err
			},
			path: "/v1/appStoreVersions/version-1/relationships/appStoreVersionExperiments",
		},
		{
			name: "experiments v2",
			call: func(ctx context.Context, client *Client) error {
				_, err := client.GetAppStoreVersionExperimentsV2Relationships(ctx, "version-1", Limit[LinkagesOption](20))
				return err
			},
			path: "/v1/appStoreVersions/version-1/relationships/appStoreVersionExperimentsV2",
		},
		{
			name: "customer reviews",
			call: func(ctx context.Context, client *Client) error {
				_, err := client.GetAppStoreVersionCustomerReviewsRelationships(ctx, "version-1", Limit[LinkagesOption](20))
				return err
			},
			path: "/v1/appStoreVersions/version-1/relationships/customerReviews",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"rel-1"}]}`)
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", req.Method)
				}
				if req.URL.Path != test.path {
					t.Fatalf("expected path %s, got %s", test.path, req.URL.Path)
				}
				if req.URL.Query().Get("limit") != "20" {
					t.Fatalf("expected limit=20, got %q", req.URL.Query().Get("limit"))
				}
				assertAuthorized(t, req)
			}, response)

			if err := test.call(context.Background(), client); err != nil {
				t.Fatalf("call error: %v", err)
			}
		})
	}
}

func TestAppStoreVersionListEndpoints_WithLimit(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		path     string
		limit    string
		response string
		call     func(*testing.T, *Client)
	}{
		{
			name:     "GetAppStoreVersionCustomerReviews",
			path:     "/v1/appStoreVersions/version-1/customerReviews",
			limit:    "5",
			response: `{"data":[{"type":"customerReviews","id":"review-1","attributes":{"rating":5,"title":"Great"}}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetAppStoreVersionCustomerReviews(ctx, "version-1", Limit[ReviewOption](5))
				if err != nil {
					t.Fatalf("GetAppStoreVersionCustomerReviews() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].Attributes.Rating != 5 {
					t.Fatalf("expected decoded customer review, got %+v", resp.Data)
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

func TestGetAppCategorySubcategories_SendsRequestWithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCategories/GAMES/subcategories" {
			t.Fatalf("expected path /v1/appCategories/GAMES/subcategories, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppCategorySubcategories(context.Background(), "GAMES", WithAppCategoriesLimit(50)); err != nil {
		t.Fatalf("GetAppCategorySubcategories() error: %v", err)
	}
}

func TestGetAppInfo_SendsRequestWithInclude(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appInfos","id":"info-1","attributes":{"state":"READY"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appInfos/info-1" {
			t.Fatalf("expected path /v1/appInfos/info-1, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("include") != "ageRatingDeclaration" {
			t.Fatalf("expected include=ageRatingDeclaration, got %q", req.URL.Query().Get("include"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppInfo(context.Background(), "info-1", WithAppInfoInclude([]string{"ageRatingDeclaration"})); err != nil {
		t.Fatalf("GetAppInfo() error: %v", err)
	}
}

func TestUpdateAppInfoCategories_SendsSubcategoryRelationships(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appInfos","id":"info-1","attributes":{"state":"READY"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appInfos/info-1" {
			t.Fatalf("expected path /v1/appInfos/info-1, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}

		var payload AppInfoUpdateCategoriesRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}

		if payload.Data.Type != ResourceTypeAppInfos {
			t.Fatalf("expected type appInfos, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "info-1" {
			t.Fatalf("expected id info-1, got %q", payload.Data.ID)
		}
		if payload.Data.Relationships == nil {
			t.Fatal("expected relationships payload")
		}

		rels := payload.Data.Relationships
		if rels.PrimaryCategory == nil || rels.PrimaryCategory.Data.ID != "GAMES" {
			t.Fatalf("expected primaryCategory GAMES, got %+v", rels.PrimaryCategory)
		}
		if rels.SecondaryCategory == nil || rels.SecondaryCategory.Data.ID != "ENTERTAINMENT" {
			t.Fatalf("expected secondaryCategory ENTERTAINMENT, got %+v", rels.SecondaryCategory)
		}
		if rels.PrimarySubcategoryOne == nil || rels.PrimarySubcategoryOne.Data.ID != "GAMES_ACTION" {
			t.Fatalf("expected primarySubcategoryOne GAMES_ACTION, got %+v", rels.PrimarySubcategoryOne)
		}
		if rels.PrimarySubcategoryTwo == nil || rels.PrimarySubcategoryTwo.Data.ID != "GAMES_SIMULATION" {
			t.Fatalf("expected primarySubcategoryTwo GAMES_SIMULATION, got %+v", rels.PrimarySubcategoryTwo)
		}
		if rels.SecondarySubcategoryOne == nil || rels.SecondarySubcategoryOne.Data.ID != "ENTERTAINMENT_KIDS" {
			t.Fatalf("expected secondarySubcategoryOne ENTERTAINMENT_KIDS, got %+v", rels.SecondarySubcategoryOne)
		}
		if rels.SecondarySubcategoryTwo == nil || rels.SecondarySubcategoryTwo.Data.ID != "ENTERTAINMENT_MUSIC" {
			t.Fatalf("expected secondarySubcategoryTwo ENTERTAINMENT_MUSIC, got %+v", rels.SecondarySubcategoryTwo)
		}

		assertAuthorized(t, req)
	}, response)

	_, err := client.UpdateAppInfoCategories(
		context.Background(),
		"info-1",
		"GAMES",
		"ENTERTAINMENT",
		"GAMES_ACTION",
		"GAMES_SIMULATION",
		"ENTERTAINMENT_KIDS",
		"ENTERTAINMENT_MUSIC",
	)
	if err != nil {
		t.Fatalf("UpdateAppInfoCategories() error: %v", err)
	}
}

func TestGetAppInfoTerritoryAgeRatings_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appInfos/info-1/territoryAgeRatings" {
			t.Fatalf("expected path /v1/appInfos/info-1/territoryAgeRatings, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if got := values.Get("fields[territoryAgeRatings]"); got != "appStoreAgeRating,territory" {
			t.Fatalf("expected fields[territoryAgeRatings]=appStoreAgeRating,territory, got %q", got)
		}
		if got := values.Get("fields[territories]"); got != "currency" {
			t.Fatalf("expected fields[territories]=currency, got %q", got)
		}
		if got := values.Get("include"); got != "territory" {
			t.Fatalf("expected include=territory, got %q", got)
		}
		if got := values.Get("limit"); got != "10" {
			t.Fatalf("expected limit=10, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetAppInfoTerritoryAgeRatings(
		context.Background(), "info-1",
		WithTerritoryAgeRatingsFields([]string{"appStoreAgeRating", "territory"}),
		WithTerritoryAgeRatingsTerritoryFields([]string{"currency"}),
		WithTerritoryAgeRatingsInclude([]string{"territory"}),
		Limit[TerritoryAgeRatingsOption](10),
	)
	if err != nil {
		t.Fatalf("GetAppInfoTerritoryAgeRatings() error: %v", err)
	}
}
