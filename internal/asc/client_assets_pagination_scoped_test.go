package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestValidateNoRedirectReadContinuationCanonicalizesOnlyScopedURL(t *testing.T) {
	client := &Client{}
	const path = "/v1/appScreenshotSets/set-1/appScreenshots"

	canonical, err := client.ValidateNoRedirectReadContinuation(
		WithNoRedirectReads(context.Background()),
		"HTTPS://API.APPSTORECONNECT.APPLE.COM"+path+"?cursor=next",
		path,
	)
	if err != nil {
		t.Fatalf("scoped URL validation error: %v", err)
	}
	if want := BaseURL + path + "?cursor=next"; canonical != want {
		t.Fatalf("canonical URL = %q, want %q", canonical, want)
	}

	original := " https://evil.example" + path + "?cursor=next "
	unchanged, err := client.ValidateNoRedirectReadContinuation(context.Background(), original, path)
	if err != nil {
		t.Fatalf("unmarked URL validation error: %v", err)
	}
	if unchanged != original {
		t.Fatalf("unmarked URL = %q, want original %q", unchanged, original)
	}
}

func TestScopedAppScreenshotPaginationBindsNextURLToCollection(t *testing.T) {
	const (
		setsPath        = "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets"
		screenshotsPath = "/v1/appScreenshotSets/set-1/appScreenshots"
	)

	tests := []struct {
		name    string
		nextURL string
		wantErr bool
		call    func(*Client, context.Context, RequestContextFunc) error
		first   string
		second  string
	}{
		{
			name:    "sets same-origin absolute",
			nextURL: BaseURL + setsPath + "?cursor=next",
			call: func(client *Client, ctx context.Context, requestContext RequestContextFunc) error {
				_, err := client.GetAllAppScreenshotSets(ctx, "loc-1", WithAppScreenshotSetsRequestContext(requestContext))
				return err
			},
			first:  `{"data":[{"type":"appScreenshotSets","id":"set-1"}],"links":{"next":"` + BaseURL + setsPath + `?cursor=next"}}`,
			second: `{"data":[{"type":"appScreenshotSets","id":"set-2"}],"links":{}}`,
		},
		{
			name:    "sets root-relative",
			nextURL: setsPath + "?cursor=next",
			call: func(client *Client, ctx context.Context, requestContext RequestContextFunc) error {
				_, err := client.GetAllAppScreenshotSets(ctx, "loc-1", WithAppScreenshotSetsRequestContext(requestContext))
				return err
			},
			first:  `{"data":[{"type":"appScreenshotSets","id":"set-1"}],"links":{"next":"` + setsPath + `?cursor=next"}}`,
			second: `{"data":[{"type":"appScreenshotSets","id":"set-2"}],"links":{}}`,
		},
		{
			name:    "sets off-collection",
			nextURL: "/v1/appScreenshotSets/other/appScreenshots?cursor=next",
			wantErr: true,
			call: func(client *Client, ctx context.Context, requestContext RequestContextFunc) error {
				_, err := client.GetAllAppScreenshotSets(ctx, "loc-1", WithAppScreenshotSetsRequestContext(requestContext))
				return err
			},
			first: `{"data":[{"type":"appScreenshotSets","id":"set-1"}],"links":{"next":"` + "/v1/appScreenshotSets/other/appScreenshots?cursor=next" + `"}}`,
		},
		{
			name:    "sets wrong origin",
			nextURL: "https://evil.example" + setsPath + "?cursor=next",
			wantErr: true,
			call: func(client *Client, ctx context.Context, requestContext RequestContextFunc) error {
				_, err := client.GetAllAppScreenshotSets(ctx, "loc-1", WithAppScreenshotSetsRequestContext(requestContext))
				return err
			},
			first: `{"data":[{"type":"appScreenshotSets","id":"set-1"}],"links":{"next":"https://evil.example` + setsPath + `?cursor=next"}}`,
		},
		{
			name:    "screenshots same-origin absolute",
			nextURL: BaseURL + screenshotsPath + "?cursor=next",
			call: func(client *Client, ctx context.Context, requestContext RequestContextFunc) error {
				_, err := client.GetAllAppScreenshots(ctx, "set-1", WithAppScreenshotsRequestContext(requestContext))
				return err
			},
			first:  `{"data":[{"type":"appScreenshots","id":"shot-1"}],"links":{"next":"` + BaseURL + screenshotsPath + `?cursor=next"}}`,
			second: `{"data":[{"type":"appScreenshots","id":"shot-2"}],"links":{}}`,
		},
		{
			name:    "screenshots root-relative",
			nextURL: screenshotsPath + "?cursor=next",
			call: func(client *Client, ctx context.Context, requestContext RequestContextFunc) error {
				_, err := client.GetAllAppScreenshots(ctx, "set-1", WithAppScreenshotsRequestContext(requestContext))
				return err
			},
			first:  `{"data":[{"type":"appScreenshots","id":"shot-1"}],"links":{"next":"` + screenshotsPath + `?cursor=next"}}`,
			second: `{"data":[{"type":"appScreenshots","id":"shot-2"}],"links":{}}`,
		},
		{
			name:    "screenshots off-collection",
			nextURL: setsPath + "?cursor=next",
			wantErr: true,
			call: func(client *Client, ctx context.Context, requestContext RequestContextFunc) error {
				_, err := client.GetAllAppScreenshots(ctx, "set-1", WithAppScreenshotsRequestContext(requestContext))
				return err
			},
			first: `{"data":[{"type":"appScreenshots","id":"shot-1"}],"links":{"next":"` + setsPath + `?cursor=next"}}`,
		},
		{
			name:    "screenshots wrong origin",
			nextURL: "https://evil.example" + screenshotsPath + "?cursor=next",
			wantErr: true,
			call: func(client *Client, ctx context.Context, requestContext RequestContextFunc) error {
				_, err := client.GetAllAppScreenshots(ctx, "set-1", WithAppScreenshotsRequestContext(requestContext))
				return err
			},
			first: `{"data":[{"type":"appScreenshots","id":"shot-1"}],"links":{"next":"https://evil.example` + screenshotsPath + `?cursor=next"}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestCount := 0
			client := newTestClient(
				t,
				func(*http.Request) { requestCount++ },
				jsonResponse(http.StatusOK, test.first),
				jsonResponse(http.StatusOK, test.second),
			)
			ctx := WithNoRedirectReads(context.Background())
			requestContext := func(parent context.Context) (context.Context, context.CancelFunc) {
				return context.WithCancel(parent)
			}

			err := test.call(client, ctx, requestContext)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected scoped continuation error")
				}
				if requestCount != 1 {
					t.Fatalf("request count = %d, want 1 before refusal", requestCount)
				}
				return
			}
			if err != nil {
				t.Fatalf("scoped pagination error: %v", err)
			}
			if requestCount != 2 {
				t.Fatalf("request count = %d, want 2", requestCount)
			}
		})
	}
}
