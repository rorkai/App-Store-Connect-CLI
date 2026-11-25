package shared

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestIsLiveAppStoreVersionPrefersAppVersionState(t *testing.T) {
	tests := []struct {
		name               string
		attributes         asc.AppStoreVersionAttributes
		wantLive           bool
		wantLiveOrPreorder bool
	}{
		{
			name:               "modern only",
			attributes:         asc.AppStoreVersionAttributes{AppVersionState: "READY_FOR_DISTRIBUTION"},
			wantLive:           true,
			wantLiveOrPreorder: true,
		},
		{
			name:               "legacy only",
			attributes:         asc.AppStoreVersionAttributes{AppStoreState: "READY_FOR_SALE"},
			wantLive:           true,
			wantLiveOrPreorder: true,
		},
		{
			name:               "both present",
			attributes:         asc.AppStoreVersionAttributes{AppStoreState: "READY_FOR_SALE", AppVersionState: "READY_FOR_DISTRIBUTION"},
			wantLive:           true,
			wantLiveOrPreorder: true,
		},
		{
			name:       "modern state wins over a stale legacy state",
			attributes: asc.AppStoreVersionAttributes{AppStoreState: "READY_FOR_SALE", AppVersionState: "REPLACED_WITH_NEW_VERSION"},
		},
		{
			name:               "modern live state wins over a stale legacy state",
			attributes:         asc.AppStoreVersionAttributes{AppStoreState: "PENDING_DEVELOPER_RELEASE", AppVersionState: "READY_FOR_DISTRIBUTION"},
			wantLive:           true,
			wantLiveOrPreorder: true,
		},
		{
			name:               "legacy spelling in the modern attribute is still recognized",
			attributes:         asc.AppStoreVersionAttributes{AppVersionState: "ready_for_sale"},
			wantLive:           true,
			wantLiveOrPreorder: true,
		},
		{
			name:               "legacy pre-order has no modern spelling",
			attributes:         asc.AppStoreVersionAttributes{AppStoreState: "PREORDER_READY_FOR_SALE"},
			wantLiveOrPreorder: true,
		},
		{
			name:               "legacy pre-order alongside a modern state",
			attributes:         asc.AppStoreVersionAttributes{AppStoreState: "PREORDER_READY_FOR_SALE", AppVersionState: "PENDING_DEVELOPER_RELEASE"},
			wantLiveOrPreorder: true,
		},
		{
			name:       "not live",
			attributes: asc.AppStoreVersionAttributes{AppStoreState: "PREPARE_FOR_SUBMISSION", AppVersionState: "PREPARE_FOR_SUBMISSION"},
		},
		{
			name:       "no state",
			attributes: asc.AppStoreVersionAttributes{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsLiveAppStoreVersion(test.attributes); got != test.wantLive {
				t.Fatalf("IsLiveAppStoreVersion(%+v) = %t, want %t", test.attributes, got, test.wantLive)
			}
			if got := IsLiveOrPreorderAppStoreVersion(test.attributes); got != test.wantLiveOrPreorder {
				t.Fatalf("IsLiveOrPreorderAppStoreVersion(%+v) = %t, want %t", test.attributes, got, test.wantLiveOrPreorder)
			}
		})
	}
}

func TestLiveAppStoreVersionStateFilterQueriesBothSpellings(t *testing.T) {
	filter := LiveAppStoreVersionStateFilter()
	if got := strings.Join(filter.AppStoreStates, ","); got != "READY_FOR_SALE" {
		t.Fatalf("AppStoreStates = %q, want READY_FOR_SALE", got)
	}
	if got := strings.Join(filter.AppVersionStates, ","); got != "READY_FOR_DISTRIBUTION" {
		t.Fatalf("AppVersionStates = %q, want READY_FOR_DISTRIBUTION", got)
	}

	// Callers extend the filter; that must not leak into later calls.
	filter.AppStoreStates = append(filter.AppStoreStates, "PREORDER_READY_FOR_SALE")
	if got := strings.Join(LiveAppStoreVersionStateFilter().AppStoreStates, ","); got != "READY_FOR_SALE" {
		t.Fatalf("LiveAppStoreVersionStateFilter() was mutated by a caller: %q", got)
	}
}

func TestListAppStoreVersionsWithStateBindsScopedContinuation(t *testing.T) {
	const expectedPath = "/v1/apps/app-1/appStoreVersions"
	for _, test := range []struct {
		name          string
		wantCalls     int
		wantVersions  int
		firstResponse string
		location      string
	}{
		{
			name:          "same app pagination succeeds",
			wantCalls:     2,
			wantVersions:  2,
			firstResponse: `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=next"}}`,
		},
		{
			name:          "foreign collection pagination refuses before fetch",
			wantCalls:     1,
			firstResponse: `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/other/appStoreVersions?cursor=next"}}`,
		},
		{
			name:      "redirect refuses before follow-up",
			wantCalls: 1,
			location:  "https://api.appstoreconnect.apple.com/fixture/scoped-version-refusal",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client := newSubmitReadinessTestClient(t, func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodGet || req.URL.Path != expectedPath {
					return submitReadinessJSONResponse(http.StatusInternalServerError, `{"errors":[{"title":"unexpected version request"}]}`)
				}
				switch {
				case test.location != "" && requests == 1:
					response, err := submitReadinessJSONResponse(http.StatusTemporaryRedirect, `{"errors":[{"title":"synthetic redirect"}]}`)
					if err != nil {
						return nil, err
					}
					response.Header.Set("Location", test.location)
					return response, nil
				case test.firstResponse != "" && requests == 1:
					return submitReadinessJSONResponse(http.StatusOK, test.firstResponse)
				case test.name == "same app pagination succeeds" && req.URL.Query().Get("cursor") == "next":
					return submitReadinessJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-2","attributes":{"platform":"IOS"}}],"links":{}}`)
				default:
					return submitReadinessJSONResponse(http.StatusInternalServerError, `{"errors":[{"title":"unexpected follow-up"}]}`)
				}
			})

			versions, err := listAppStoreVersionsWithState(asc.WithNoRedirectReads(context.Background()), client, "app-1", "IOS", asc.WithAppStoreVersionsStates([]string{"READY_FOR_SALE"}), callerRequestContext)
			if test.wantVersions == 0 {
				if err == nil {
					t.Fatal("expected scoped continuation refusal")
				}
			} else if err != nil || len(versions) != test.wantVersions {
				t.Fatalf("versions=%d err=%v, want %d versions", len(versions), err, test.wantVersions)
			}
			if requests != test.wantCalls {
				t.Fatalf("requests=%d, want %d and no foreign redirect follow-up", requests, test.wantCalls)
			}
		})
	}
}

// liveVersionTestClient answers app store version list requests keyed by
// "filter[appStoreState]|filter[appVersionState]|filter[platform]|limit".
func liveVersionTestClient(t *testing.T, responses map[string]string, log *[]string) *asc.Client {
	t.Helper()
	return newSubmitReadinessTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
		}
		query := req.URL.Query()
		key := query.Get("filter[appStoreState]") + "|" + query.Get("filter[appVersionState]") + "|" + query.Get("filter[platform]") + "|" + query.Get("limit")
		if log != nil {
			*log = append(*log, key)
		}
		body, ok := responses[key]
		if !ok {
			t.Fatalf("unexpected version query %q", key)
		}
		return submitReadinessJSONResponse(http.StatusOK, body)
	})
}

func TestListAppStoreVersionsInStatesMergesBothSpellings(t *testing.T) {
	var log []string
	client := liveVersionTestClient(t, map[string]string{
		"READY_FOR_SALE||IOS|200": `{"data":[
			{"type":"appStoreVersions","id":"ver-legacy","attributes":{"platform":"IOS","versionString":"1.0","appStoreState":"READY_FOR_SALE"}},
			{"type":"appStoreVersions","id":"ver-both","attributes":{"platform":"IOS","versionString":"2.0","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION"}}
		],"links":{}}`,
		"|READY_FOR_DISTRIBUTION|IOS|200": `{"data":[
			{"type":"appStoreVersions","id":"ver-both","attributes":{"platform":"IOS","versionString":"2.0","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION"}},
			{"type":"appStoreVersions","id":"ver-modern","attributes":{"platform":"IOS","versionString":"3.0","appVersionState":"READY_FOR_DISTRIBUTION"}}
		],"links":{}}`,
	}, &log)

	versions, err := ListAppStoreVersionsInStates(context.Background(), client, "app-1", "IOS", LiveAppStoreVersionStateFilter())
	if err != nil {
		t.Fatalf("ListAppStoreVersionsInStates() error: %v", err)
	}
	ids := make([]string, 0, len(versions))
	for _, version := range versions {
		ids = append(ids, version.ID)
	}
	if got, want := strings.Join(ids, ","), "ver-legacy,ver-both,ver-modern"; got != want {
		t.Fatalf("versions = %q, want %q", got, want)
	}
	if got, want := strings.Join(log, ","), "READY_FOR_SALE||IOS|200,|READY_FOR_DISTRIBUTION|IOS|200"; got != want {
		t.Fatalf("request sequence = %q, want %q", got, want)
	}
}

func TestListAppStoreVersionsInStatesWithRequestTimeoutsUsesAFreshTimeoutPerRequest(t *testing.T) {
	var deadlines []time.Time
	client := newSubmitReadinessTestClient(t, func(req *http.Request) (*http.Response, error) {
		deadline, ok := req.Context().Deadline()
		if !ok {
			t.Fatalf("request %s has no deadline", req.URL.String())
		}
		deadlines = append(deadlines, deadline)
		query := req.URL.Query()
		switch {
		case query.Get("filter[appStoreState]") == "READY_FOR_SALE" && query.Get("cursor") == "":
			return submitReadinessJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"appStoreState":"READY_FOR_SALE"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?filter%5BappStoreState%5D=READY_FOR_SALE&cursor=2"}}`)
		case query.Get("cursor") == "2":
			return submitReadinessJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-2","attributes":{"appStoreState":"READY_FOR_SALE"}}],"links":{}}`)
		case query.Get("filter[appVersionState]") == "READY_FOR_DISTRIBUTION":
			return submitReadinessJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Fatalf("unexpected request %s", req.URL.String())
			return nil, nil
		}
	})

	versions, err := ListAppStoreVersionsInStatesWithRequestTimeouts(context.Background(), client, "app-1", "", LiveAppStoreVersionStateFilter())
	if err != nil {
		t.Fatalf("ListAppStoreVersionsInStatesWithRequestTimeouts() error: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected two versions across pages, got %d", len(versions))
	}
	if len(deadlines) != 3 {
		t.Fatalf("expected three requests, got %d", len(deadlines))
	}
	// A deadline shared across the whole lookup would be identical on every
	// request; a fresh per-request timeout is created later for each one.
	for i := 1; i < len(deadlines); i++ {
		if !deadlines[i].After(deadlines[i-1]) {
			t.Fatalf("request %d reused an earlier deadline: %v then %v", i, deadlines[i-1], deadlines[i])
		}
	}
}

// Callers that own a longer budget (for example a publish --timeout override)
// must not have each request capped at the default request timeout.
func TestListAndHasAppStoreVersionsInStatesUseTheCallerDeadline(t *testing.T) {
	callerDeadline := time.Now().Add(time.Hour)
	ctx, cancel := context.WithDeadline(context.Background(), callerDeadline)
	defer cancel()

	requests := 0
	client := newSubmitReadinessTestClient(t, func(req *http.Request) (*http.Response, error) {
		requests++
		deadline, ok := req.Context().Deadline()
		if !ok || !deadline.Equal(callerDeadline) {
			t.Fatalf("request %s deadline = %v (set=%t), want caller deadline %v", req.URL.String(), deadline, ok, callerDeadline)
		}
		return submitReadinessJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
	})

	if _, err := ListAppStoreVersionsInStates(ctx, client, "app-1", "IOS", LiveAppStoreVersionStateFilter()); err != nil {
		t.Fatalf("ListAppStoreVersionsInStates() error: %v", err)
	}
	if _, err := HasAppStoreVersionInStates(ctx, client, "app-1", "IOS", LiveAppStoreVersionStateFilter()); err != nil {
		t.Fatalf("HasAppStoreVersionInStates() error: %v", err)
	}
	if requests != 4 {
		t.Fatalf("expected two requests per helper, got %d", requests)
	}
}

func TestAppUpdateRequiresWhatsNewDetectsReleasedVersions(t *testing.T) {
	const (
		legacyQuery = "READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE||IOS|1"
		modernQuery = "|READY_FOR_DISTRIBUTION|IOS|1"
		empty       = `{"data":[],"links":{}}`
	)
	tests := []struct {
		name      string
		responses map[string]string
		want      bool
		wantLog   string
	}{
		{
			name: "modern only",
			responses: map[string]string{
				legacyQuery: empty,
				modernQuery: `{"data":[{"type":"appStoreVersions","id":"ver-live","attributes":{"platform":"IOS","appVersionState":"READY_FOR_DISTRIBUTION"}}],"links":{}}`,
			},
			want:    true,
			wantLog: legacyQuery + "," + modernQuery,
		},
		{
			name: "legacy only",
			responses: map[string]string{
				legacyQuery: `{"data":[{"type":"appStoreVersions","id":"ver-live","attributes":{"platform":"IOS","appStoreState":"READY_FOR_SALE"}}],"links":{}}`,
			},
			want:    true,
			wantLog: legacyQuery,
		},
		{
			name: "both present",
			responses: map[string]string{
				legacyQuery: `{"data":[{"type":"appStoreVersions","id":"ver-live","attributes":{"platform":"IOS","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION"}}],"links":{}}`,
			},
			want:    true,
			wantLog: legacyQuery,
		},
		{
			name: "never released",
			responses: map[string]string{
				legacyQuery: empty,
				modernQuery: empty,
			},
			want:    false,
			wantLog: legacyQuery + "," + modernQuery,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var log []string
			client := liveVersionTestClient(t, test.responses, &log)
			got, err := AppUpdateRequiresWhatsNew(context.Background(), client, "app-1", "IOS")
			if err != nil {
				t.Fatalf("AppUpdateRequiresWhatsNew() error: %v", err)
			}
			if got != test.want {
				t.Fatalf("AppUpdateRequiresWhatsNew() = %t, want %t", got, test.want)
			}
			if gotLog := strings.Join(log, ","); gotLog != test.wantLog {
				t.Fatalf("request sequence = %q, want %q", gotLog, test.wantLog)
			}
		})
	}
}
