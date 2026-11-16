package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestAPIAttributesRoundTripAppleObject(t *testing.T) {
	tests := []struct {
		name   string
		target any
		apple  string
	}{
		{"app store review detail", &AppStoreReviewDetailAttributes{}, `{"demoAccountRequired":false,"demoAccountName":null,"demoAccountPassword":null,"notes":"","unmodeled":1}`},
		{"version localization", &AppStoreVersionLocalizationAttributes{}, `{"locale":"en-US","marketingUrl":null,"whatsNew":"","unmodeled":1}`},
		{"app info localization", &AppInfoLocalizationAttributes{}, `{"locale":"en-US","privacyChoicesUrl":null,"privacyPolicyText":null,"unmodeled":1}`},
		{"beta build localization", &BetaBuildLocalizationAttributes{}, `{"locale":"en-US","whatsNew":null,"unmodeled":1}`},
		{"age rating declaration", &AgeRatingDeclarationAttributes{}, `{"gambling":false,"kidsAgeBand":null,"gracRatingClassificationNumber":null,"unmodeled":1}`},
		{"iap review screenshot", &InAppPurchaseAppStoreReviewScreenshotAttributes{}, `{"fileName":"shot.png","fileSize":0,"uploadOperations":[],"sourceFileChecksum":null,"unmodeled":1}`},
		{"review submission", &ReviewSubmissionAttributes{}, `{"platform":"IOS","state":"READY_FOR_REVIEW","submittedDate":null,"unmodeled":1}`},
		{"review submission item", &ReviewSubmissionItemAttributes{}, `{"state":null,"unmodeled":1}`},
		{"build beta detail", &BuildBetaDetailAttributes{}, `{"autoNotifyEnabled":false,"internalBuildState":null,"unmodeled":1}`},
		{"subscription", &SubscriptionAttributes{}, `{"name":"Pro","familySharable":false,"reviewNote":null,"groupLevel":0,"unmodeled":1}`},
		{"in-app purchase", &InAppPurchaseV2Attributes{}, `{"name":"Coins","contentHosting":false,"reviewNote":null,"familySharable":false,"unmodeled":1}`},
		{"beta app review detail", &BetaAppReviewDetailAttributes{}, `{"demoAccountRequired":false,"demoAccountName":null,"demoAccountPassword":null,"notes":null,"unmodeled":1}`},
		{"beta app localization", &BetaAppLocalizationAttributes{}, `{"locale":"en-US","description":null,"marketingUrl":null,"privacyPolicyUrl":null,"unmodeled":1}`},
		{"app store version", &AppStoreVersionAttributes{}, `{"versionString":"2.5.2","appStoreState":"READY_FOR_SALE","reviewType":"APP_STORE","usesIdfa":null,"earliestReleaseDate":null,"downloadable":false,"unmodeled":1}`},
		{"phased release", &AppStoreVersionPhasedReleaseAttributes{}, `{"phasedReleaseState":"COMPLETE","totalPauseDuration":0,"currentDayNumber":null,"unmodeled":1}`},
		{"build", &BuildAttributes{}, `{"version":"42","uploadedDate":"2026-10-01T00:00:00Z","expired":false,"lsMinimumSystemVersion":null,"usesNonExemptEncryption":null,"unmodeled":1}`},
		{"beta tester", &BetaTesterAttributes{}, `{"firstName":"Ada","lastName":null,"email":null,"appDevices":[],"unmodeled":1}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(test.apple), test.target); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			output, err := json.Marshal(test.target)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got, want map[string]any
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatalf("unmarshal output: %v", err)
			}
			if err := json.Unmarshal([]byte(test.apple), &want); err != nil {
				t.Fatalf("unmarshal input: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("attributes changed on read\n got: %s\nwant: %s", output, test.apple)
			}
		})
	}
}

func TestAPIAttributesRedactUnmodeledSecrets(t *testing.T) {
	const sentinel = "asc-red-sentinel-unmodeled-secret-5f19"
	var attrs BuildAttributes
	apple := `{"version":"42","accessToken":"` + sentinel + `","clientSecret":"` + sentinel + `","adminPassword":null,"apiKeyHint":"","assetToken":"public"}`
	if err := json.Unmarshal([]byte(apple), &attrs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	output, err := json.Marshal(attrs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(output), sentinel) {
		t.Fatalf("read output leaked an unmodeled secret: %s", output)
	}
	var got map[string]any
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	want := map[string]any{"version": "42", "accessToken": RedactedValuePlaceholder, "clientSecret": RedactedValuePlaceholder, "adminPassword": nil, "apiKeyHint": "", "assetToken": "public"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %s", output)
	}
}

func TestRedactedReviewDetailHidesPasswordUnderAnyKeyCase(t *testing.T) {
	const sentinel = "asc-red-sentinel-folded-key-pw-71c2e0"
	tests := []struct {
		name       string
		attributes string
		want       string
	}{
		{
			name:       "only a differently cased key",
			attributes: `{"demoAccountName":null,"DemoAccountPassword":"` + sentinel + `"}`,
			want:       `"demoAccountPassword":"` + RedactedValuePlaceholder + `"`,
		},
		{
			name:       "differently cased key before an empty exact key",
			attributes: `{"DemoAccountPassword":"` + sentinel + `","demoAccountPassword":""}`,
			want:       `"demoAccountPassword":""`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var resp AppStoreReviewDetailResponse
			body := `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":` + test.attributes + `}}`
			if err := json.Unmarshal([]byte(body), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			output, err := json.Marshal(RedactAppStoreReviewDetailResponse(&resp))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if strings.Contains(string(output), sentinel) || strings.Contains(string(output), "DemoAccountPassword") {
				t.Fatalf("redacted output leaked the password or kept its folded key: %s", output)
			}
			if !strings.Contains(string(output), test.want) {
				t.Fatalf("expected %s in output, got %s", test.want, output)
			}
		})
	}
}

func TestWriteRequestsDoNotEchoAppleResponseAttributes(t *testing.T) {
	const apple = `{"locale":"en-US","whatsNew":"Fixes","marketingUrl":null,"privacyChoicesUrl":null,"kidsAgeBand":null,"unmodeled":true}`
	decode := func(t *testing.T, target any) {
		t.Helper()
		if err := json.Unmarshal([]byte(apple), target); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
	}
	// Each body is what the client sent for the same decoded attributes before
	// read snapshots existed.
	tests := []struct {
		name string
		want string
		call func(*testing.T, *Client) error
	}{
		{"create version localization", `{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"en-US","whatsNew":"Fixes"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`, func(t *testing.T, c *Client) error {
			var attrs AppStoreVersionLocalizationAttributes
			decode(t, &attrs)
			_, err := c.CreateAppStoreVersionLocalization(context.Background(), "version-1", attrs.CreateAttributes())
			return err
		}},
		{"update version localization", `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"whatsNew":"Fixes"}}}`, func(t *testing.T, c *Client) error {
			var attrs AppStoreVersionLocalizationAttributes
			decode(t, &attrs)
			_, err := c.UpdateAppStoreVersionLocalization(context.Background(), "loc-1", attrs.UpdateAttributes())
			return err
		}},
		{"create test notes", `{"data":{"type":"betaBuildLocalizations","attributes":{"locale":"en-US","whatsNew":"Fixes"},"relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`, func(t *testing.T, c *Client) error {
			var attrs BetaBuildLocalizationAttributes
			decode(t, &attrs)
			_, err := c.CreateBetaBuildLocalization(context.Background(), "build-1", attrs)
			return err
		}},
		{"update test notes", `{"data":{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Fixes"}}}`, func(t *testing.T, c *Client) error {
			var attrs BetaBuildLocalizationAttributes
			decode(t, &attrs)
			_, err := c.UpdateBetaBuildLocalization(context.Background(), "loc-1", attrs)
			return err
		}},
		{"create app info localization", `{"data":{"type":"appInfoLocalizations","attributes":{"locale":"en-US"},"relationships":{"appInfo":{"data":{"type":"appInfos","id":"info-1"}}}}}`, func(t *testing.T, c *Client) error {
			var attrs AppInfoLocalizationAttributes
			decode(t, &attrs)
			_, err := c.CreateAppInfoLocalization(context.Background(), "info-1", attrs.CreateAttributes())
			return err
		}},
		{"update app info localization", `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{}}}`, func(t *testing.T, c *Client) error {
			var attrs AppInfoLocalizationAttributes
			decode(t, &attrs)
			_, err := c.UpdateAppInfoLocalization(context.Background(), "loc-1", attrs.UpdateAttributes())
			return err
		}},
		{"create beta app localization", `{"data":{"type":"betaAppLocalizations","attributes":{"locale":"en-US"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`, func(t *testing.T, c *Client) error {
			var attrs BetaAppLocalizationAttributes
			decode(t, &attrs)
			_, err := c.CreateBetaAppLocalization(context.Background(), "app-1", attrs.CreateAttributes())
			return err
		}},
		{"update age rating", `{"data":{"type":"ageRatingDeclarations","id":"age-1","attributes":{}}}`, func(t *testing.T, c *Client) error {
			var attrs AgeRatingDeclarationAttributes
			decode(t, &attrs)
			_, err := c.UpdateAgeRatingDeclaration(context.Background(), "age-1", attrs)
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				if string(body) != test.want+"\n" {
					t.Fatalf("request body changed\n got: %s\nwant: %s", body, test.want)
				}
			}, jsonResponse(http.StatusOK, `{"data":{"type":"x","id":"1","attributes":{}}}`))
			if err := test.call(t, client); err != nil {
				t.Fatalf("request error: %v", err)
			}
		})
	}
}
