package validate

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type requestConcurrencyTracker struct {
	active atomic.Int32
	max    atomic.Int32
}

func (tracker *requestConcurrencyTracker) wait(ctx context.Context, delay time.Duration) error {
	active := tracker.active.Add(1)
	defer tracker.active.Add(-1)
	for {
		previous := tracker.max.Load()
		if active <= previous || tracker.max.CompareAndSwap(previous, active) {
			break
		}
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestFetchScreenshotSets_OverlapsRequestsWithinCapAndSortsDeterministically(t *testing.T) {
	const delay = 100 * time.Millisecond
	tracker := &requestConcurrencyTracker{}
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := tracker.wait(req.Context(), delay); err != nil {
			return nil, err
		}

		switch {
		case strings.HasPrefix(req.URL.Path, "/v1/appStoreVersionLocalizations/") && strings.HasSuffix(req.URL.Path, "/appScreenshotSets"):
			localizationID := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/appStoreVersionLocalizations/"), "/appScreenshotSets")
			return buildsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshotSets","id":"set-%s","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`, localizationID))
		case strings.HasPrefix(req.URL.Path, "/v1/appScreenshotSets/") && strings.HasSuffix(req.URL.Path, "/appScreenshots"):
			setID := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/appScreenshotSets/"), "/appScreenshots")
			return buildsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"shot-%s","attributes":{"fileName":"%s.png","imageAsset":{"width":1242,"height":2688}}}]}`, setID, setID))
		default:
			return buildsJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	}))

	localizationIDs := []string{"loc-z", "loc-b", "loc-y", "loc-a", "loc-x", "loc-c"}
	localizations := make([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], 0, len(localizationIDs))
	for _, id := range localizationIDs {
		localizations = append(localizations, asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
			Type: asc.ResourceTypeAppStoreVersionLocalizations,
			ID:   id,
			Attributes: asc.AppStoreVersionLocalizationAttributes{
				Locale: "locale-" + id,
			},
		})
	}

	started := time.Now()
	sets, err := fetchScreenshotSets(context.Background(), client, localizations)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("fetchScreenshotSets() error = %v", err)
	}
	if got := tracker.max.Load(); got > readinessConcurrencyLimit {
		t.Fatalf("maximum in-flight requests = %d, want <= %d", got, readinessConcurrencyLimit)
	} else if got < 2 {
		t.Fatalf("maximum in-flight requests = %d, expected overlap", got)
	}
	if elapsed >= 900*time.Millisecond {
		t.Fatalf("bounded fan-out took %s; serial fixture time is %s", elapsed, time.Duration(len(localizations)*2)*delay)
	}
	if len(sets) != len(localizationIDs) {
		t.Fatalf("got %d screenshot sets, want %d", len(sets), len(localizationIDs))
	}
	wantLocalizationIDs := []string{"loc-a", "loc-b", "loc-c", "loc-x", "loc-y", "loc-z"}
	for index, localizationID := range wantLocalizationIDs {
		if got := sets[index].LocalizationID; got != localizationID {
			t.Fatalf("set %d localization = %q, want %q", index, got, localizationID)
		}
		if got := sets[index].ID; got != "set-"+localizationID {
			t.Fatalf("set %d ID = %q, want %q", index, got, "set-"+localizationID)
		}
		if len(sets[index].Screenshots) != 1 || sets[index].Screenshots[0].ID != "shot-set-"+localizationID {
			t.Fatalf("set %d screenshots = %+v", index, sets[index].Screenshots)
		}
	}
}

func TestBuildReadinessReport_OverlapsSixIndependentReadGroups(t *testing.T) {
	const (
		slowDelay  = 300 * time.Millisecond
		shortDelay = 100 * time.Millisecond
	)
	delays := map[string]time.Duration{
		"/v1/apps/app-1/appPriceSchedule":                              slowDelay,
		"/v1/apps/app-1/appAvailabilityV2":                             shortDelay,
		"/v1/territories":                                              shortDelay,
		"/v1/appStoreVersionLocalizations/ver-loc-1/appScreenshotSets": shortDelay,
		"/v1/apps/app-1/subscriptionGroups":                            shortDelay,
		"/v1/apps/app-1/inAppPurchasesV2":                              shortDelay,
	}
	serialDelay := slowDelay + 5*shortDelay

	tracker := &requestConcurrencyTracker{}
	var mu sync.Mutex
	requestCounts := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if delay, ok := delays[req.URL.Path]; ok {
			mu.Lock()
			requestCounts[req.URL.Path]++
			mu.Unlock()
			if err := tracker.wait(req.Context(), delay); err != nil {
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/v1/appStoreVersions/ver-1":
			fmt.Fprint(w, `{
				"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0"},"relationships":{
					"appStoreVersionLocalizations":{"data":[{"type":"appStoreVersionLocalizations","id":"ver-loc-1"}],"meta":{"paging":{"total":1,"limit":50}}},
					"build":{"data":null},
					"appStoreReviewDetail":{"data":null}
				}},
				"included":[{"type":"appStoreVersionLocalizations","id":"ver-loc-1","attributes":{"locale":"en-US"}}]
			}`)
		case "/v1/apps/app-1/appInfos":
			fmt.Fprint(w, `{
				"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"},"relationships":{
					"app":{"data":{"type":"apps","id":"app-1"}},
					"ageRatingDeclaration":{"data":null},
					"appInfoLocalizations":{"data":[],"meta":{"paging":{"total":0,"limit":50}}},
					"primaryCategory":{"data":null}
				}}],
				"included":[{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}}]
			}`)
		case "/v1/apps/app-1/appPriceSchedule":
			fmt.Fprint(w, `{"data":{"type":"appPriceSchedules","id":"schedule-1"}}`)
		case "/v1/apps/app-1/appAvailabilityV2":
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`)
		case "/v1/territories":
			fmt.Fprint(w, `{"data":[{"type":"territories","id":"USA"}],"links":{"next":""}}`)
		case "/v1/appStoreVersionLocalizations/ver-loc-1/appScreenshotSets",
			"/v1/apps/app-1/subscriptionGroups",
			"/v1/apps/app-1/inAppPurchasesV2":
			fmt.Fprint(w, `{"data":[]}`)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"errors":[{"status":"500","code":"UNEXPECTED_REQUEST","detail":%q}]}`, req.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse fixture URL: %v", err)
	}
	transport := buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		fixtureReq := req.Clone(req.Context())
		fixtureReq.URL.Scheme = target.Scheme
		fixtureReq.URL.Host = target.Host
		fixtureReq.Host = target.Host
		return server.Client().Transport.RoundTrip(fixtureReq)
	})
	client := newBuildsTestClient(t, transport)
	restoreClient := SetClientFactory(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	started := time.Now()
	if _, err := BuildReadinessReport(context.Background(), ReadinessOptions{AppID: "app-1", VersionID: "ver-1"}); err != nil {
		t.Fatalf("BuildReadinessReport() error = %v", err)
	}
	elapsed := time.Since(started)

	mu.Lock()
	defer mu.Unlock()
	for path := range delays {
		if got := requestCounts[path]; got != 1 {
			t.Fatalf("request count for %s = %d, want 1", path, got)
		}
	}
	if got := tracker.max.Load(); got != readinessConcurrencyLimit {
		t.Fatalf("maximum in-flight top-level reads = %d, want %d", got, readinessConcurrencyLimit)
	}
	if elapsed >= 550*time.Millisecond {
		t.Fatalf("six independent read groups took %s; slowest group is %s and serial fixture time is %s", elapsed, slowDelay, serialDelay)
	}
	t.Logf("six independent read groups completed in %s (slowest=%s, serial=%s, max-in-flight=%d)", elapsed, slowDelay, serialDelay, tracker.max.Load())
}

func TestBuildReadinessReport_CancelsSiblingCompoundReadOnHardError(t *testing.T) {
	appInfoStarted := make(chan struct{})
	appInfoCanceled := make(chan struct{})
	var startedOnce sync.Once
	var canceledOnce sync.Once

	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			startedOnce.Do(func() { close(appInfoStarted) })
			<-req.Context().Done()
			canceledOnce.Do(func() { close(appInfoCanceled) })
			return nil, req.Context().Err()
		case "/v1/appStoreVersions/ver-1":
			<-appInfoStarted
			return buildsJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"INVALID_REQUEST","title":"Invalid Request"}]}`)
		default:
			return buildsJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	}))
	restoreClient := SetClientFactory(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	_, err := BuildReadinessReport(context.Background(), ReadinessOptions{AppID: "app-1", VersionID: "ver-1"})
	if err == nil || !strings.Contains(err.Error(), "failed to fetch app store version") {
		t.Fatalf("BuildReadinessReport() error = %v", err)
	}
	select {
	case <-appInfoCanceled:
	case <-time.After(time.Second):
		t.Fatal("app-info compound request was not canceled after sibling hard error")
	}
}

func TestBuildReadinessReport_SharedGateCapsNestedSubscriptionRequests(t *testing.T) {
	tracker := &requestConcurrencyTracker{}
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		delay := 10 * time.Millisecond
		switch req.URL.Path {
		case "/v1/apps/app-1/appPriceSchedule", "/v1/apps/app-1/appAvailabilityV2", "/v1/apps/app-1/inAppPurchasesV2":
			delay = 200 * time.Millisecond
		case "/v1/apps/app-1/subscriptionGroups":
			delay = 20 * time.Millisecond
		case "/v1/subscriptionGroups/group-1/subscriptions",
			"/v1/subscriptionGroups/group-2/subscriptions",
			"/v1/subscriptionGroups/group-3/subscriptions",
			"/v1/subscriptionGroups/group-4/subscriptions":
			delay = 100 * time.Millisecond
		}
		if err := tracker.wait(req.Context(), delay); err != nil {
			return nil, err
		}

		switch req.URL.Path {
		case "/v1/appStoreVersions/ver-1":
			return buildsJSONResponse(http.StatusOK, `{
				"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0"},"relationships":{
					"appStoreVersionLocalizations":{"data":[],"meta":{"paging":{"total":0,"limit":50}}},
					"build":{"data":null},
					"appStoreReviewDetail":{"data":null}
				}},
				"included":[]
			}`)
		case "/v1/apps/app-1/appInfos":
			return buildsJSONResponse(http.StatusOK, `{
				"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"},"relationships":{
					"app":{"data":{"type":"apps","id":"app-1"}},
					"ageRatingDeclaration":{"data":null},
					"appInfoLocalizations":{"data":[],"meta":{"paging":{"total":0,"limit":50}}},
					"primaryCategory":{"data":null}
				}}],
				"included":[{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}}]
			}`)
		case "/v1/apps/app-1/appPriceSchedule", "/v1/apps/app-1/appAvailabilityV2":
			return buildsJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`)
		case "/v1/apps/app-1/inAppPurchasesV2":
			return buildsJSONResponse(http.StatusOK, `{"data":[]}`)
		case "/v1/apps/app-1/subscriptionGroups":
			return buildsJSONResponse(http.StatusOK, `{"data":[
				{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"One"}},
				{"type":"subscriptionGroups","id":"group-2","attributes":{"referenceName":"Two"}},
				{"type":"subscriptionGroups","id":"group-3","attributes":{"referenceName":"Three"}},
				{"type":"subscriptionGroups","id":"group-4","attributes":{"referenceName":"Four"}}
			]}`)
		case "/v1/subscriptionGroups/group-1/subscriptions",
			"/v1/subscriptionGroups/group-2/subscriptions",
			"/v1/subscriptionGroups/group-3/subscriptions",
			"/v1/subscriptionGroups/group-4/subscriptions":
			groupID := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/subscriptionGroups/"), "/subscriptions")
			return buildsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"subscriptions","id":"sub-%s","attributes":{"name":"%s","productId":"%s","state":"REMOVED_FROM_SALE"}}]}`, groupID, groupID, groupID))
		case "/v1/subscriptions/sub-group-1/images",
			"/v1/subscriptions/sub-group-2/images",
			"/v1/subscriptions/sub-group-3/images",
			"/v1/subscriptions/sub-group-4/images":
			return buildsJSONResponse(http.StatusOK, `{"data":[]}`)
		default:
			return buildsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_REQUEST"}]}`)
		}
	}))
	restoreClient := SetClientFactory(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	if _, err := BuildReadinessReport(context.Background(), ReadinessOptions{AppID: "app-1", VersionID: "ver-1"}); err != nil {
		t.Fatalf("BuildReadinessReport() error = %v", err)
	}
	if got := tracker.max.Load(); got != readinessConcurrencyLimit {
		t.Fatalf("maximum in-flight requests = %d, want exactly %d across nested tasks", got, readinessConcurrencyLimit)
	}
}
