package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestOpenAPI441SparseAppFields(t *testing.T) {
	tests := []struct {
		name      string
		wantPath  string
		wantQuery map[string]string
		response  string
		call      func(*Client) error
	}{
		{
			name:     "app info localization detail",
			wantPath: "/v1/appInfoLocalizations/loc-1",
			wantQuery: map[string]string{
				"fields[appInfos]": "kidsAgeBand",
				"include":          "appInfo",
			},
			call: func(client *Client) error {
				_, err := client.GetAppInfoLocalization(
					context.Background(), "loc-1",
					WithAppInfoLocalizationAppInfoFields([]string{"kidsAgeBand"}),
					WithAppInfoLocalizationInclude([]string{"appInfo"}),
				)
				return err
			},
		},
		{
			name:     "app info detail",
			wantPath: "/v1/appInfos/info-1",
			wantQuery: map[string]string{
				"fields[appInfos]":              "kidsAgeBand",
				"fields[ageRatingDeclarations]": "socialMedia,socialMediaAgeRestricted",
				"include":                       "ageRatingDeclaration",
			},
			call: func(client *Client) error {
				_, err := client.GetAppInfo(
					context.Background(), "info-1",
					WithAppInfoFields([]string{"kidsAgeBand"}),
					WithAppInfoAgeRatingDeclarationFields([]string{"socialMedia", "socialMediaAgeRestricted"}),
					WithAppInfoInclude([]string{"ageRatingDeclaration"}),
				)
				return err
			},
		},
		{
			name:     "apps list",
			wantPath: "/v1/apps",
			response: `{"data":[]}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]":     "versions",
				"fields[subscriptionGroups]": "versions",
				"include":                    "inAppPurchases,subscriptionGroups",
			},
			call: func(client *Client) error {
				_, err := client.GetApps(
					context.Background(),
					WithAppsInAppPurchaseFields([]string{"versions"}),
					WithAppsSubscriptionGroupFields([]string{"versions"}),
					WithAppsInclude([]string{"inAppPurchases", "subscriptionGroups"}),
				)
				return err
			},
		},
		{
			name:     "app detail",
			wantPath: "/v1/apps/app-1",
			wantQuery: map[string]string{
				"fields[inAppPurchases]":     "versions",
				"fields[subscriptionGroups]": "versions",
				"include":                    "inAppPurchases,subscriptionGroups",
			},
			call: func(client *Client) error {
				_, err := client.GetAppWithOptions(
					context.Background(), "app-1",
					WithAppInAppPurchaseFields([]string{"versions"}),
					WithAppSubscriptionGroupFields([]string{"versions"}),
					WithAppInclude([]string{"inAppPurchases", "subscriptionGroups"}),
				)
				return err
			},
		},
		{
			name:     "age rating for app info",
			wantPath: "/v1/appInfos/info-1/ageRatingDeclaration",
			wantQuery: map[string]string{
				"fields[ageRatingDeclarations]": "socialMedia,socialMediaAgeRestricted",
			},
			call: func(client *Client) error {
				_, err := client.GetAgeRatingDeclarationForAppInfo(
					context.Background(), "info-1",
					WithAgeRatingDeclarationFields([]string{"socialMedia", "socialMediaAgeRestricted"}),
				)
				return err
			},
		},
		{
			name:     "app info localizations",
			wantPath: "/v1/appInfos/info-1/appInfoLocalizations",
			response: `{"data":[]}`,
			wantQuery: map[string]string{
				"fields[appInfos]": "kidsAgeBand",
				"include":          "appInfo",
			},
			call: func(client *Client) error {
				_, err := client.GetAppInfoLocalizations(
					context.Background(), "info-1",
					WithAppInfoLocalizationsAppInfoFields([]string{"kidsAgeBand"}),
					WithAppInfoLocalizationsInclude([]string{"appInfo"}),
				)
				return err
			},
		},
		{
			name:     "app infos",
			wantPath: "/v1/apps/app-1/appInfos",
			response: `{"data":[]}`,
			wantQuery: map[string]string{
				"fields[appInfos]":              "kidsAgeBand",
				"fields[ageRatingDeclarations]": "socialMedia,socialMediaAgeRestricted",
				"include":                       "ageRatingDeclaration",
			},
			call: func(client *Client) error {
				_, err := client.GetAppInfos(
					context.Background(), "app-1",
					WithAppInfoFields([]string{"kidsAgeBand"}),
					WithAppInfoAgeRatingDeclarationFields([]string{"socialMedia", "socialMediaAgeRestricted"}),
					WithAppInfoInclude([]string{"ageRatingDeclaration"}),
				)
				return err
			},
		},
		{
			name:     "ci product app",
			wantPath: "/v1/ciProducts/product-1/app",
			wantQuery: map[string]string{
				"fields[inAppPurchases]":     "versions",
				"fields[subscriptionGroups]": "versions",
				"include":                    "inAppPurchases,subscriptionGroups",
			},
			call: func(client *Client) error {
				_, err := client.GetCiProductApp(
					context.Background(), "product-1",
					WithCiProductAppInAppPurchaseFields([]string{"versions"}),
					WithCiProductAppSubscriptionGroupFields([]string{"versions"}),
					WithCiProductAppInclude([]string{"inAppPurchases", "subscriptionGroups"}),
				)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("method = %s, want GET", req.Method)
				}
				if req.URL.Path != test.wantPath {
					t.Fatalf("path = %q, want %q", req.URL.Path, test.wantPath)
				}
				query := req.URL.Query()
				for key, want := range test.wantQuery {
					if got := query.Get(key); got != want {
						t.Errorf("query %s = %q, want %q", key, got, want)
					}
				}
				if len(query) != len(test.wantQuery) {
					t.Errorf("query = %v, want exactly %v", query, test.wantQuery)
				}
			}, jsonResponse(http.StatusOK, func() string {
				if test.response != "" {
					return test.response
				}
				return `{"data":{"type":"apps","id":"1"}}`
			}()))

			if err := test.call(client); err != nil {
				t.Fatalf("call error: %v", err)
			}
		})
	}
}
