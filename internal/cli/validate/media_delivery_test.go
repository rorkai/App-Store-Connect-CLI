package validate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func mediaDeliveryLocalizations(ids ...string) []asc.Resource[asc.AppStoreVersionLocalizationAttributes] {
	localizations := make([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], 0, len(ids))
	for _, id := range ids {
		localizations = append(localizations, asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
			Type:       asc.ResourceTypeAppStoreVersionLocalizations,
			ID:         id,
			Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "locale-" + id},
		})
	}
	return localizations
}

func TestFetchMediaDelivery_ReadsEveryPageAndPrefersVideoState(t *testing.T) {
	const nextSets = "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-1/appPreviewSets?cursor=two"
	const nextPlacements = "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-1/placements?cursor=two"
	const nextPreviews = "https://api.appstoreconnect.apple.com/v1/appPreviewSets/set-uninlined/appPreviews?cursor=two"

	var mu sync.Mutex
	var requests []string
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		requests = append(requests, req.URL.Path+"?"+req.URL.RawQuery)
		mu.Unlock()
		query := req.URL.Query()
		switch req.URL.Path {
		case "/v1/appStoreVersionLocalizations/loc-1/appPreviewSets":
			if query.Get("cursor") == "two" {
				return buildsJSONResponse(http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-uninlined","attributes":{"previewType":"IPAD_PRO_3GEN_129"}}]}`)
			}
			return buildsJSONResponse(http.StatusOK, fmt.Sprintf(`{
				"data":[{"type":"appPreviewSets","id":"set-inlined","attributes":{"previewType":"IPHONE_67"},
					"relationships":{"appPreviews":{"data":[{"type":"appPreviews","id":"video"},{"type":"appPreviews","id":"legacy"}]}}}],
				"included":[
					{"type":"appPreviews","id":"video","attributes":{"fileName":"video.mov",
						"assetDeliveryState":{"state":"COMPLETE"},
						"videoDeliveryState":{"state":"FAILED","errors":[{"code":"VIDEO_BAD","description":"Bad codec"}]}}},
					{"type":"appPreviews","id":"legacy","attributes":{"fileName":"legacy.mov","assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}
				],
				"links":{"next":%q}}`, nextSets))
		case "/v1/appPreviewSets/set-uninlined/appPreviews":
			if query.Get("cursor") == "two" {
				return buildsJSONResponse(http.StatusOK, `{"data":[{"type":"appPreviews","id":"second-page","attributes":{"fileName":"second.mov","videoDeliveryState":{"state":"PROCESSING"}}}]}`)
			}
			return buildsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appPreviews","id":"first-page","attributes":{"fileName":"first.mov"}}],"links":{"next":%q}}`, nextPreviews))
		case "/v1/appStoreVersionLocalizations/loc-1/placements":
			if query.Get("cursor") == "two" {
				return buildsJSONResponse(http.StatusOK, `{"data":[{"type":"appAssetLibraryPlacements","id":"placement-2","attributes":{"placementType":"APP_PREVIEW","state":"ASSET_PROCESSING"}}]}`)
			}
			if got := query.Get("filter[placementType]"); got != "APP_SCREENSHOT,IMESSAGE_APP_SCREENSHOT,APP_PREVIEW" {
				return nil, fmt.Errorf("placement type filter = %q", got)
			}
			return buildsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appAssetLibraryPlacements","id":"placement-1","attributes":{"placementType":"APP_SCREENSHOT","placementGroup":"group-1","state":"FAILED","stateDetails":[{"code":"ASSET_BAD","description":"Unreadable"}]}}],"links":{"next":%q}}`, nextPlacements))
		}
		return buildsJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
	}))

	data, err := fetchMediaDelivery(context.Background(), client, mediaDeliveryLocalizations("loc-1"))
	if err != nil {
		t.Fatalf("fetchMediaDelivery() error = %v", err)
	}
	if len(data.gaps) != 0 {
		t.Fatalf("unexpected read gaps: %+v", data.gaps)
	}

	got := map[string]validation.Preview{}
	for _, set := range data.previewSets {
		if set.Locale != "locale-loc-1" || set.LocalizationID != "loc-1" {
			t.Fatalf("preview set lost its locale: %+v", set)
		}
		for _, preview := range set.Previews {
			got[preview.ID] = preview
		}
	}
	wantStates := map[string]string{"video": "FAILED", "legacy": "UPLOAD_COMPLETE", "first-page": "", "second-page": "PROCESSING"}
	if len(got) != len(wantStates) {
		t.Fatalf("previews = %+v, want %v", got, wantStates)
	}
	for id, state := range wantStates {
		if got[id].DeliveryState != state {
			t.Fatalf("preview %s state = %q, want %q", id, got[id].DeliveryState, state)
		}
	}
	if !slices.Equal(got["video"].DeliveryErrors, []string{"VIDEO_BAD: Bad codec"}) {
		t.Fatalf("video errors = %v", got["video"].DeliveryErrors)
	}

	if len(data.placements) != 2 {
		t.Fatalf("placements = %+v, want both pages", data.placements)
	}
	first := data.placements[0]
	if first.ID != "placement-1" || first.Locale != "locale-loc-1" || first.State != "FAILED" || !slices.Equal(first.StateDetails, []string{"ASSET_BAD: Unreadable"}) {
		t.Fatalf("placement = %+v", first)
	}

	includeRequested := slices.ContainsFunc(requests, func(request string) bool {
		return strings.Contains(request, "include=appPreviews") && strings.Contains(request, "limit=200")
	})
	if !includeRequested {
		t.Fatalf("expected an inlined preview set read, got %v", requests)
	}
}

func TestFetchMediaDelivery_ReportsUnreadSourcesPerLocale(t *testing.T) {
	tests := []struct {
		name       string
		failPath   string
		status     int
		wantMedia  string
		wantReason string
	}{
		{name: "previews forbidden", failPath: "/appPreviewSets", status: http.StatusForbidden, wantMedia: mediaLabelPreviews, wantReason: "cannot read them"},
		{name: "placements rate limited", failPath: "/placements", status: http.StatusTooManyRequests, wantMedia: mediaLabelPlacements, wantReason: "rate limited"},
		{name: "placements missing", failPath: "/placements", status: http.StatusNotFound, wantMedia: mediaLabelPlacements, wantReason: "not found"},
		{name: "previews bad request", failPath: "/appPreviewSets", status: http.StatusBadRequest, wantMedia: mediaLabelPreviews, wantReason: "could not be read"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.HasPrefix(req.URL.Path, "/v1/appStoreVersionLocalizations/loc-bad/") && strings.HasSuffix(req.URL.Path, test.failPath) {
					return buildsJSONResponse(test.status, fmt.Sprintf(`{"errors":[{"status":"%d","title":"failed","detail":"failed"}]}`, test.status))
				}
				return buildsJSONResponse(http.StatusOK, `{"data":[]}`)
			}))

			data, err := fetchMediaDelivery(context.Background(), client, mediaDeliveryLocalizations("loc-good", "loc-bad"))
			if err != nil {
				t.Fatalf("fetchMediaDelivery() error = %v", err)
			}
			if len(data.gaps) != 1 {
				t.Fatalf("gaps = %+v, want one", data.gaps)
			}
			gap := data.gaps[0]
			if gap.Media != test.wantMedia || gap.Locale != "locale-loc-bad" || gap.LocalizationID != "loc-bad" {
				t.Fatalf("gap = %+v", gap)
			}
			if !strings.Contains(gap.Reason, test.wantReason) {
				t.Fatalf("gap reason = %q, want %q", gap.Reason, test.wantReason)
			}
		})
	}
}

func TestFetchMediaDelivery_PropagatesCancellation(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.Canceled
	}))

	if _, err := fetchMediaDelivery(context.Background(), client, mediaDeliveryLocalizations("loc-1")); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

func TestFetchMediaDelivery_StaysWithinReadinessConcurrency(t *testing.T) {
	tracker := &requestConcurrencyTracker{}
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := tracker.wait(req.Context(), 20*time.Millisecond); err != nil {
			return nil, err
		}
		return buildsJSONResponse(http.StatusOK, `{"data":[]}`)
	}))

	if _, err := fetchMediaDelivery(context.Background(), client, mediaDeliveryLocalizations("a", "b", "c", "d", "e", "f")); err != nil {
		t.Fatalf("fetchMediaDelivery() error = %v", err)
	}
	if got := tracker.max.Load(); got > readinessConcurrencyLimit {
		t.Fatalf("maximum in-flight requests = %d, want <= %d", got, readinessConcurrencyLimit)
	}
}

func TestFetchScreenshotSets_CarriesAssetDeliveryState(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return buildsJSONResponse(http.StatusOK, `{
			"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_67"},
				"relationships":{"appScreenshots":{"data":[{"type":"appScreenshots","id":"shot-1"}]}}}],
			"included":[{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"one.png",
				"assetDeliveryState":{"state":"FAILED","errors":[{"code":"IMAGE_BAD","description":"Corrupt"}]}}}]}`)
	}))

	sets, err := fetchScreenshotSets(context.Background(), client, mediaDeliveryLocalizations("loc-1"))
	if err != nil {
		t.Fatalf("fetchScreenshotSets() error = %v", err)
	}
	if len(sets) != 1 || len(sets[0].Screenshots) != 1 {
		t.Fatalf("sets = %+v", sets)
	}
	shot := sets[0].Screenshots[0]
	if shot.DeliveryState != "FAILED" || !slices.Equal(shot.DeliveryErrors, []string{"IMAGE_BAD: Corrupt"}) {
		t.Fatalf("screenshot = %+v", shot)
	}
}
