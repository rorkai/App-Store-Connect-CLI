package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestIAPRelatedReadsPropagateVersionSparseFields441(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		response  string
		wantQuery map[string]string
		invoke    func(context.Context, *Client) error
	}{
		{
			name:     "review screenshot detail",
			path:     "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1",
			response: `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1"}}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"include":                "inAppPurchaseV2",
			},
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetInAppPurchaseAppStoreReviewScreenshot(ctx, "shot-1", WithIAPReviewScreenshotIAPFields([]string{"versions"}))
				return err
			},
		},
		{
			name:     "review screenshot relationship",
			path:     "/v2/inAppPurchases/iap-1/appStoreReviewScreenshot",
			response: `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1"}}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"include":                "inAppPurchaseV2",
			},
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetInAppPurchaseAppStoreReviewScreenshotForIAP(ctx, "iap-1", WithIAPReviewScreenshotIAPFields([]string{"versions"}))
				return err
			},
		},
		{
			name:     "content detail",
			path:     "/v1/inAppPurchaseContents/content-1",
			response: `{"data":{"type":"inAppPurchaseContents","id":"content-1"}}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"include":                "inAppPurchaseV2",
			},
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetInAppPurchaseContentByID(ctx, "content-1", WithIAPContentIAPFields([]string{"versions"}))
				return err
			},
		},
		{
			name:     "content relationship",
			path:     "/v2/inAppPurchases/iap-1/content",
			response: `{"data":{"type":"inAppPurchaseContents","id":"content-1"}}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"include":                "inAppPurchaseV2",
			},
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetInAppPurchaseContent(ctx, "iap-1", WithIAPContentIAPFields([]string{"versions"}))
				return err
			},
		},
		{
			name:     "image detail",
			path:     "/v1/inAppPurchaseImages/image-1",
			response: `{"data":{"type":"inAppPurchaseImages","id":"image-1"}}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"include":                "inAppPurchase",
			},
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetInAppPurchaseImage(ctx, "image-1", WithIAPImageIAPFields([]string{"versions"}))
				return err
			},
		},
		{
			name:     "images relationship",
			path:     "/v2/inAppPurchases/iap-1/images",
			response: `{"data":[]}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"include":                "inAppPurchase",
			},
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetInAppPurchaseImages(ctx, "iap-1", WithIAPImagesIAPFields([]string{"versions"}))
				return err
			},
		},
		{
			name:     "localization detail",
			path:     "/v1/inAppPurchaseLocalizations/localization-1",
			response: `{"data":{"type":"inAppPurchaseLocalizations","id":"localization-1"}}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"include":                "inAppPurchaseV2",
			},
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetInAppPurchaseLocalization(ctx, "localization-1", WithIAPLocalizationIAPFields([]string{"versions"}))
				return err
			},
		},
		{
			name:     "localizations relationship",
			path:     "/v2/inAppPurchases/iap-1/inAppPurchaseLocalizations",
			response: `{"data":[]}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"include":                "inAppPurchaseV2",
			},
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetInAppPurchaseLocalizations(ctx, "iap-1", WithIAPLocalizationsIAPFields([]string{"versions"}))
				return err
			},
		},
		{
			name:     "promoted purchase detail",
			path:     "/v1/promotedPurchases/promo-1",
			response: `{"data":{"type":"promotedPurchases","id":"promo-1"}}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"fields[subscriptions]":  "versions",
				"include":                "inAppPurchaseV2,subscription",
			},
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetPromotedPurchase(ctx, "promo-1", WithPromotedPurchaseIAPFields([]string{"versions"}), WithPromotedPurchaseSubscriptionFields([]string{"versions"}))
				return err
			},
		},
		{
			name:     "app promoted purchases",
			path:     "/v1/apps/app-1/promotedPurchases",
			response: `{"data":[]}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"fields[subscriptions]":  "versions",
				"include":                "inAppPurchaseV2,subscription",
			},
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetAppPromotedPurchases(ctx, "app-1", WithPromotedPurchasesIAPFields([]string{"versions"}), WithPromotedPurchasesSubscriptionFields([]string{"versions"}))
				return err
			},
		},
		{
			name:     "iap promoted purchase relationship",
			path:     "/v2/inAppPurchases/iap-1/promotedPurchase",
			response: `{"data":{"type":"promotedPurchases","id":"promo-1"}}`,
			wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"fields[subscriptions]":  "versions",
				"include":                "inAppPurchaseV2,subscription",
			},
			invoke: func(ctx context.Context, client *Client) error {
				_, err := client.GetInAppPurchasePromotedPurchase(ctx, "iap-1", WithPromotedPurchaseIAPFields([]string{"versions"}), WithPromotedPurchaseSubscriptionFields([]string{"versions"}))
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("method = %s, want GET", req.Method)
				}
				if req.URL.Path != tt.path {
					t.Fatalf("path = %q, want %q", req.URL.Path, tt.path)
				}
				if len(req.URL.Query()) != len(tt.wantQuery) {
					t.Fatalf("query = %v, want exactly %v", req.URL.Query(), tt.wantQuery)
				}
				for key, want := range tt.wantQuery {
					values := req.URL.Query()[key]
					if len(values) != 1 || values[0] != want {
						t.Fatalf("query %s = %v, want exactly [%q]", key, values, want)
					}
				}
			}, jsonResponse(http.StatusOK, tt.response))

			if err := tt.invoke(context.Background(), client); err != nil {
				t.Fatalf("invoke() error: %v", err)
			}
		})
	}
}

func TestIAPRelatedSparseFieldsExplicitIncludesAreDeduplicated441(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		values := req.URL.Query()["include"]
		if len(values) != 1 || values[0] != "inAppPurchaseV2" {
			t.Fatalf("include = %v, want exactly [inAppPurchaseV2]", values)
		}
	}, jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseContents","id":"content-1"}}`))

	_, err := client.GetInAppPurchaseContent(
		context.Background(),
		"iap-1",
		WithIAPContentInclude([]string{"inAppPurchaseV2"}),
		WithIAPContentIAPFields([]string{"versions"}),
	)
	if err != nil {
		t.Fatalf("GetInAppPurchaseContent() error: %v", err)
	}
}
