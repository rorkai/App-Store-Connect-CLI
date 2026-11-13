package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

const marketplaceWebhooksDeprecationWarning = "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API."

func TestAppInfoGetRejectsInvalidNextURLPhase63(t *testing.T) {
	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v1/appInfos?cursor=AQ",
			wantErr: "--next must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: "--next must be a valid URL:",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"apps", "info", "view", "--next", test.next}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestRemainingListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"builds", "list"}, wantErr: "builds: --next"},
		{args: []string{"marketplace", "webhooks", "list"}, wantErr: "marketplace webhooks list: --next", wantWarning: marketplaceWebhooksDeprecationWarning},
		{args: []string{"pricing", "price-points"}, wantErr: "pricing price-points: --next"},
		{args: []string{"review", "items-list"}, wantErr: "review items-list: --next"},
		{args: []string{"review", "submissions-items-ids"}, wantErr: "review submissions-items-ids: --next"},
		{args: []string{"review", "submissions-list"}, wantErr: "review submissions-list: --next"},
		{args: []string{"users", "invites", "visible-apps", "list"}, wantErr: "users invites visible-apps list: --next"},
		{args: []string{"users", "visible-apps", "view"}, wantErr: "users visible-apps view: --next"},
		{args: []string{"subscriptions", "offers", "win-back", "list"}, wantErr: "subscriptions offers win-back list: --next"},
		{args: []string{"subscriptions", "offers", "win-back", "prices"}, wantErr: "subscriptions offers win-back prices: --next"},
		{args: []string{"subscriptions", "offers", "win-back", "prices-links"}, wantErr: "subscriptions offers win-back prices-links: --next"},
		{args: []string{"subscriptions", "offers", "win-back", "links"}, wantErr: "subscriptions offers win-back links: --next"},
	})
}

func TestRemainingListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"builds", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/builds?cursor=AQ&limit=200",
			dataType: "builds",
		},
		{
			args:     []string{"product-pages", "experiments", "list", "--version-id", "version-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionExperiments?cursor=AQ&limit=200",
			dataType: "appStoreVersionExperiments",
		},
		{
			args:     []string{"product-pages", "experiments", "treatments", "list", "--experiment-id", "experiment-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperiments/experiment-1/appStoreVersionExperimentTreatments?cursor=AQ&limit=200",
			dataType: "appStoreVersionExperimentTreatments",
		},
		{
			args:     []string{"product-pages", "experiments", "treatments", "localizations", "list", "--treatment-id", "treatment-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperimentTreatments/treatment-1/appStoreVersionExperimentTreatmentLocalizations?cursor=AQ&limit=200",
			dataType: "appStoreVersionExperimentTreatmentLocalizations",
		},
		{
			args:     []string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperimentTreatmentLocalizations/localization-1/appPreviewSets?cursor=AQ&limit=200",
			dataType: "appPreviewSets",
		},
		{
			args:     []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperimentTreatmentLocalizations/localization-1/appScreenshotSets?cursor=AQ&limit=200",
			dataType: "appScreenshotSets",
		},
		{
			args:       []string{"marketplace", "webhooks", "list"},
			firstURL:   "https://api.appstoreconnect.apple.com/v1/marketplaceWebhooks?cursor=AQ&limit=200",
			dataType:   "marketplaceWebhooks",
			wantStderr: marketplaceWebhooksDeprecationWarning + "\n",
		},
		{
			args:     []string{"pricing", "price-points"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/pricePoints?cursor=AQ&limit=200",
			dataType: "appPricePoints",
		},
		{
			args:     []string{"pricing", "territories", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/territories?cursor=AQ&limit=200",
			dataType: "territories",
		},
		{
			args:     []string{"review", "attachments-list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreReviewDetails/review-detail-1/appStoreReviewAttachments?cursor=AQ&limit=200",
			dataType: "appStoreReviewAttachments",
		},
		{
			args:     []string{"review", "items-list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items?cursor=AQ&limit=200",
			dataType: "reviewSubmissionItems",
		},
		{
			args:     []string{"review", "submissions-list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=AQ&limit=200",
			dataType: "reviewSubmissions",
		},
		{
			args:     []string{"review", "submissions-items-ids"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/relationships/items?cursor=AQ&limit=200",
			dataType: "reviewSubmissionItems",
		},
		{
			args:     []string{"users", "invites", "visible-apps", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/userInvitations/invite-1/visibleApps?cursor=AQ&limit=200",
			dataType: "apps",
		},
		{
			args:     []string{"users", "visible-apps", "view"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/users/user-1/relationships/visibleApps?cursor=AQ&limit=200",
			dataType: "apps",
		},
		{
			args:     []string{"subscriptions", "offers", "win-back", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/winBackOffers?cursor=AQ&limit=200",
			dataType: "winBackOffers",
		},
		{
			args:     []string{"subscriptions", "offers", "win-back", "prices"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/winBackOffers/offer-1/prices?cursor=AQ&limit=200",
			dataType: "winBackOfferPrices",
		},
		{
			args:     []string{"subscriptions", "offers", "win-back", "prices-links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/winBackOffers/offer-1/relationships/prices?cursor=AQ&limit=200",
			dataType: "winBackOfferPrices",
		},
		{
			args:     []string{"subscriptions", "offers", "win-back", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/relationships/winBackOffers?cursor=AQ&limit=200",
			dataType: "winBackOffers",
		},
	})
}
