package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runMarketplaceWebhooksInvalidNextURLCases(
	t *testing.T,
	argsPrefix []string,
	wantErrPrefix string,
) {
	t.Helper()

	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v1/marketplaceWebhooks?cursor=AQ",
			wantErr: wantErrPrefix + " must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: wantErrPrefix + " must be a valid URL:",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string{}, argsPrefix...), "--next", test.next)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.") {
				t.Fatalf("expected deprecation warning in stderr, got %q", stderr)
			}
		})
	}
}

func runMarketplaceWebhooksPaginateFromNext(
	t *testing.T,
	argsPrefix []string,
	firstURL string,
	secondURL string,
	firstBody string,
	secondBody string,
	wantIDs ...string,
) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(firstBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(secondBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	args := append(append([]string{}, argsPrefix...), "--paginate", "--next", firstURL)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.") {
		t.Fatalf("expected deprecation warning in stderr, got %q", stderr)
	}
	for _, id := range wantIDs {
		needle := `"id":"` + id + `"`
		if !strings.Contains(stdout, needle) {
			t.Fatalf("expected output to contain %q, got %q", needle, stdout)
		}
	}
}

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
				if err := root.Parse([]string{"app-info", "get", "--next", test.next}); err != nil {
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

func TestBuildsListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"builds", "list"},
		"builds: --next",
	)
}

func TestBuildsListPaginateFromNextWithoutAppPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/builds?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/builds?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"builds","id":"build-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"builds","id":"build-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"builds", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"build-next-1",
		"build-next-2",
	)
}

func TestExperimentsListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"product-pages", "experiments", "list"},
		"experiments list: --next",
	)
}

func TestExperimentsListPaginateFromNextPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionExperiments?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionExperiments?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appStoreVersionExperiments","id":"experiment-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appStoreVersionExperiments","id":"experiment-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"product-pages", "experiments", "list", "--version-id", "version-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"experiment-next-1",
		"experiment-next-2",
	)
}

func TestExperimentsTreatmentsListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"product-pages", "experiments", "treatments", "list"},
		"experiments treatments list: --next",
	)
}

func TestExperimentsTreatmentsListPaginateFromNextPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperiments/experiment-1/appStoreVersionExperimentTreatments?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperiments/experiment-1/appStoreVersionExperimentTreatments?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appStoreVersionExperimentTreatments","id":"treatment-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appStoreVersionExperimentTreatments","id":"treatment-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"product-pages", "experiments", "treatments", "list", "--experiment-id", "experiment-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"treatment-next-1",
		"treatment-next-2",
	)
}

func TestExperimentsTreatmentsLocalizationsListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"product-pages", "experiments", "treatments", "localizations", "list"},
		"experiments treatments localizations list: --next",
	)
}

func TestExperimentsTreatmentsLocalizationsListPaginateFromNextPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperimentTreatments/treatment-1/appStoreVersionExperimentTreatmentLocalizations?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperimentTreatments/treatment-1/appStoreVersionExperimentTreatmentLocalizations?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appStoreVersionExperimentTreatmentLocalizations","id":"treatment-localization-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appStoreVersionExperimentTreatmentLocalizations","id":"treatment-localization-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"product-pages", "experiments", "treatments", "localizations", "list", "--treatment-id", "treatment-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"treatment-localization-next-1",
		"treatment-localization-next-2",
	)
}

func TestExperimentsTreatmentsLocalizationsPreviewSetsListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "list"},
		"experiments treatments localizations preview-sets list: --next",
	)
}

func TestExperimentsTreatmentsLocalizationsPreviewSetsListPaginateFromNextPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperimentTreatmentLocalizations/localization-1/appPreviewSets?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperimentTreatmentLocalizations/localization-1/appPreviewSets?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appPreviewSets","id":"preview-set-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appPreviewSets","id":"preview-set-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"preview-set-next-1",
		"preview-set-next-2",
	)
}

func TestExperimentsTreatmentsLocalizationsScreenshotSetsListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
		"experiments treatments localizations screenshot-sets list: --next",
	)
}

func TestExperimentsTreatmentsLocalizationsScreenshotSetsListPaginateFromNextPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperimentTreatmentLocalizations/localization-1/appScreenshotSets?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersionExperimentTreatmentLocalizations/localization-1/appScreenshotSets?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appScreenshotSets","id":"screenshot-set-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appScreenshotSets","id":"screenshot-set-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"screenshot-set-next-1",
		"screenshot-set-next-2",
	)
}

func TestLocalizationsDownloadRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"localizations", "download"},
		"localizations download: --next",
	)
}

func TestMarketplaceWebhooksListRejectsInvalidNextURLPhase63(t *testing.T) {
	runMarketplaceWebhooksInvalidNextURLCases(
		t,
		[]string{"marketplace", "webhooks", "list"},
		"marketplace webhooks list: --next",
	)
}

func TestMarketplaceWebhooksListPaginateFromNextPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/marketplaceWebhooks?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/marketplaceWebhooks?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"marketplaceWebhooks","id":"marketplace-webhook-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"marketplaceWebhooks","id":"marketplace-webhook-next-2"}],"links":{"next":""}}`

	runMarketplaceWebhooksPaginateFromNext(
		t,
		[]string{"marketplace", "webhooks", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"marketplace-webhook-next-1",
		"marketplace-webhook-next-2",
	)
}

func TestPricingPricePointsRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"pricing", "price-points"},
		"pricing price-points: --next",
	)
}

func TestPricingPricePointsPaginateFromNextWithoutAppPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/pricePoints?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/pricePoints?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appPricePoints","id":"price-point-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appPricePoints","id":"price-point-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"pricing", "price-points"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"price-point-next-1",
		"price-point-next-2",
	)
}

func TestPricingTerritoriesListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"pricing", "territories", "list"},
		"pricing territories list: --next",
	)
}

func TestPricingTerritoriesListPaginateFromNextPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/territories?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/territories?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"territories","id":"pricing-territory-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"territories","id":"pricing-territory-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"pricing", "territories", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"pricing-territory-next-1",
		"pricing-territory-next-2",
	)
}

func TestReviewAttachmentsListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"review", "attachments-list"},
		"review attachments-list: --next",
	)
}

func TestReviewAttachmentsListPaginateFromNextWithoutReviewDetailPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appStoreReviewDetails/review-detail-1/appStoreReviewAttachments?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appStoreReviewDetails/review-detail-1/appStoreReviewAttachments?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appStoreReviewAttachments","id":"review-attachment-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appStoreReviewAttachments","id":"review-attachment-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"review", "attachments-list", "--review-detail", "review-detail-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"review-attachment-next-1",
		"review-attachment-next-2",
	)
}

func TestReviewItemsListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"review", "items-list"},
		"review items-list: --next",
	)
}

func TestReviewItemsListPaginateFromNextWithoutSubmissionPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"reviewSubmissionItems","id":"review-item-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"reviewSubmissionItems","id":"review-item-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"review", "items-list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"review-item-next-1",
		"review-item-next-2",
	)
}

func TestReviewSubmissionsItemsIDsRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"review", "submissions-items-ids"},
		"review submissions-items-ids: --next",
	)
}

func TestReviewSubmissionsItemsIDsPaginateFromNextWithoutIDPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/relationships/items?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/relationships/items?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"reviewSubmissionItems","id":"review-submission-item-link-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"reviewSubmissionItems","id":"review-submission-item-link-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"review", "submissions-items-ids"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"review-submission-item-link-next-1",
		"review-submission-item-link-next-2",
	)
}

func TestReviewSubmissionsListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"review", "submissions-list"},
		"review submissions-list: --next",
	)
}

func TestReviewSubmissionsListPaginateFromNextWithoutAppPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"reviewSubmissions","id":"review-submission-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"reviewSubmissions","id":"review-submission-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"review", "submissions-list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"review-submission-next-1",
		"review-submission-next-2",
	)
}

func TestUsersInvitesVisibleAppsListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"users", "invites", "visible-apps", "list"},
		"users invites visible-apps list: --next",
	)
}

func TestUsersInvitesVisibleAppsListPaginateFromNextWithoutIDPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/userInvitations/invite-1/visibleApps?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/userInvitations/invite-1/visibleApps?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"apps","id":"invite-visible-app-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"apps","id":"invite-visible-app-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"users", "invites", "visible-apps", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"invite-visible-app-next-1",
		"invite-visible-app-next-2",
	)
}

func TestUsersVisibleAppsGetRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"users", "visible-apps", "get"},
		"users visible-apps get: --next",
	)
}

func TestUsersVisibleAppsGetPaginateFromNextWithoutIDPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/users/user-1/relationships/visibleApps?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/users/user-1/relationships/visibleApps?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"apps","id":"user-visible-app-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"apps","id":"user-visible-app-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"users", "visible-apps", "get"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"user-visible-app-next-1",
		"user-visible-app-next-2",
	)
}

func TestWinBackOffersListRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"win-back-offers", "list"},
		"win-back-offers list: --next",
	)
}

func TestWinBackOffersListPaginateFromNextWithoutSubscriptionPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/winBackOffers?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/winBackOffers?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"winBackOffers","id":"win-back-offer-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"winBackOffers","id":"win-back-offer-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"win-back-offers", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"win-back-offer-next-1",
		"win-back-offer-next-2",
	)
}

func TestWinBackOffersPricesRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"win-back-offers", "prices"},
		"win-back-offers prices: --next",
	)
}

func TestWinBackOffersPricesPaginateFromNextWithoutIDPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/winBackOffers/offer-1/prices?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/winBackOffers/offer-1/prices?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"winBackOfferPrices","id":"win-back-offer-price-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"winBackOfferPrices","id":"win-back-offer-price-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"win-back-offers", "prices"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"win-back-offer-price-next-1",
		"win-back-offer-price-next-2",
	)
}

func TestWinBackOffersPricesRelationshipsRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"win-back-offers", "prices-relationships"},
		"win-back-offers prices-relationships: --next",
	)
}

func TestWinBackOffersPricesRelationshipsPaginateFromNextWithoutIDPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/winBackOffers/offer-1/relationships/prices?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/winBackOffers/offer-1/relationships/prices?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"winBackOfferPrices","id":"win-back-offer-price-link-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"winBackOfferPrices","id":"win-back-offer-price-link-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"win-back-offers", "prices-relationships"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"win-back-offer-price-link-next-1",
		"win-back-offer-price-link-next-2",
	)
}

func TestWinBackOffersRelationshipsRejectsInvalidNextURLPhase63(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"win-back-offers", "relationships"},
		"win-back-offers relationships: --next",
	)
}

func TestWinBackOffersRelationshipsPaginateFromNextWithoutSubscriptionPhase63(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/relationships/winBackOffers?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/relationships/winBackOffers?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"winBackOffers","id":"win-back-offer-link-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"winBackOffers","id":"win-back-offer-link-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"win-back-offers", "relationships"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"win-back-offer-link-next-1",
		"win-back-offer-link-next-2",
	)
}
