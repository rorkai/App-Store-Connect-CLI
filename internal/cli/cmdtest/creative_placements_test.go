package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func runCreativePlacements(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCommand("dev")
	root.FlagSet.SetOutput(io.Discard)
	var err error
	stdout, stderr := captureOutput(t, func() {
		if err = root.Parse(args); err == nil {
			err = root.Run(context.Background())
		}
	})
	return stdout, stderr, err
}

func TestCreativePlacementsPublicRead(t *testing.T) {
	for _, output := range []string{"json", "table", "markdown"} {
		t.Run(output, func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			body := `{"data":[{"type":"appAssetLibraryPlacements","id":"header-1","attributes":{"mediaType":"IMAGE","placementType":"PRODUCT_PAGE_HEADER_ASSET","placementGroup":"DEFAULT_PROFILE","state":"PREPARE_FOR_SUBMISSION","unknown":null},"relationships":{"image":{"data":{"type":"appAssetLibraryImages","id":"img"}}}}],"included":[{"type":"appAssetLibraryImages","id":"img","attributes":{"future":true}}],"newTopLevel":{"retain":true}}`
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != "GET" || req.URL.Path != "/v1/appStoreVersionLocalizations/loc/placements" {
					t.Fatalf("unexpected %s %s", req.Method, req.URL)
				}
				q := req.URL.Query()
				if q.Get("filter[placementType]") != "PRODUCT_PAGE_HEADER_ASSET,APP_STORE_SEARCH_RESULTS_ASSET" || q.Get("include") != "image,video" || q.Get("sort") != "placementGroupPosition" || q.Get("limit") != "1" {
					t.Fatalf("query=%v", q)
				}
				return jsonResponse(200, body)
			})
			stdout, stderr, err := runCreativePlacements(t, "localizations", "placements", "list", "--localization-id", "loc", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET,APP_STORE_SEARCH_RESULTS_ASSET", "--include", "image,video", "--sort", "placementGroupPosition", "--limit", "1", "--output", output)
			if err != nil || stderr != "" {
				t.Fatalf("error=%v stderr=%q", err, stderr)
			}
			if output == "json" {
				var got, want any
				_ = json.Unmarshal([]byte(stdout), &got)
				_ = json.Unmarshal([]byte(body), &want)
				g, _ := json.Marshal(got)
				w, _ := json.Marshal(want)
				if string(g) != string(w) {
					t.Fatalf("changed envelope: %s", stdout)
				}
			} else if !strings.Contains(stdout, "PRODUCT_PAGE_HEADER_ASSET") || !strings.Contains(stdout, "DEFAULT_PROFILE") {
				t.Fatalf("missing placement: %s", stdout)
			}
		})
	}
}

func TestCreativePlacementsUsage(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"--localization-id", "loc", "--placement-type", "HEADER"},
		{"--localization-id", "loc", "--include", "app"},
		{"--localization-id", "loc", "--sort", "name"},
		{"--localization-id", "loc", "--sort", "referenceName"},
		{"--localization-id", "loc", "--sort", ""},
		{"--localization-id", "loc", "--state", ""},
		{"--localization-id", "loc", "--state", "active"},
		{"--localization-id", "loc", "--image-id", "img,"},
		{"--localization-id", "loc", "--video-id", "../apps"},
		{"--localization-id", "loc", "--placement-id", "a b"},
		{"--localization-id", "loc", "--event-localization", "a/b"},
		{"--localization-id", "loc", "--limit", "-1"},
		{"--localization-id", "../apps"},
		{"--next", "https://evil.example/v1/apps"},
		{"--next", "https://api.appstoreconnect.apple.com/v1/apps", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
		{"--next", "https://api.appstoreconnect.apple.com/v1/apps", "--state", "ASSET_PROCESSING"},
		{"--next", "https://api.appstoreconnect.apple.com/v1/apps", "--sort", "createdDate"},
		{"--next", "https://api.appstoreconnect.apple.com/v1/apps", "--image-id", "img"},
		{"--next", "https://api.appstoreconnect.apple.com/v1/apps", "--video-id", "vid"},
		{"--next", "https://api.appstoreconnect.apple.com/v1/apps", "--placement-id", "place"},
		{"--next", "https://api.appstoreconnect.apple.com/v1/apps", "--event-localization", "event"},
		{"--next", "https://api.appstoreconnect.apple.com/v1/apps", "--custom-page-localization", "page"},
		{"--next", "https://api.appstoreconnect.apple.com/v1/apps", "--treatment-localization", "treatment"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			full := append([]string{"localizations", "placements", "list"}, args...)
			_, stderr, err := runCreativePlacements(t, full...)
			if !isUsageClassError(err) || stderr == "" {
				t.Fatalf("expected usage before auth, got %v %q", err, stderr)
			}
		})
	}
}

// Every placement read shares Apple's enums, so a value must reach the server
// in any localization context rather than stop as a usage error.
func TestCreativePlacementsListSendsAppleEnumValues(t *testing.T) {
	for _, tc := range []struct{ name, command, resource, flag, value, query string }{
		{"ad types", "localizations", "appStoreVersionLocalizations", "--placement-type", "SEARCH_RESULTS_ADS_ASSET,TODAY_TAB_ADS_ASSET", "filter%5BplacementType%5D=SEARCH_RESULTS_ADS_ASSET%2CTODAY_TAB_ADS_ASSET"},
		{"event type outside events", "localizations", "appStoreVersionLocalizations", "--placement-type", "EVENT_CARD_ASSET", "filter%5BplacementType%5D=EVENT_CARD_ASSET"},
		{"store type on events", "app-events localizations", "appEventLocalizations", "--placement-type", "APP_SCREENSHOT", "filter%5BplacementType%5D=APP_SCREENSHOT"},
		{"iMessage type on treatments", "product-pages experiments treatments localizations", "appStoreVersionExperimentTreatmentLocalizations", "--placement-type", "IMESSAGE_APP_SCREENSHOT", "filter%5BplacementType%5D=IMESSAGE_APP_SCREENSHOT"},
		{"parent includes", "localizations", "appStoreVersionLocalizations", "--include", "appEventLocalization,appStoreVersionLocalization,appCustomProductPageLocalization,appStoreVersionExperimentTreatmentLocalization,retentionMessageVersion", "include=appEventLocalization%2CappStoreVersionLocalization%2CappCustomProductPageLocalization%2CappStoreVersionExperimentTreatmentLocalization%2CretentionMessageVersion"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			requests := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != "GET" || req.URL.Path != "/v1/"+tc.resource+"/loc/placements" || req.URL.RawQuery != tc.query {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL)
				}
				return jsonResponse(200, `{"data":[]}`)
			})
			args := append(strings.Fields(tc.command), "placements", "list", "--localization-id", "loc", tc.flag, tc.value)
			stdout, stderr, err := runCreativePlacements(t, args...)
			if err != nil || stderr != "" || requests != 1 || !strings.Contains(stdout, `"data":[]`) {
				t.Fatalf("requests=%d stdout=%s stderr=%s err=%v", requests, stdout, stderr, err)
			}
		})
	}
}

func TestPlacementListsSendEachFilterAndSortTheirOperationAllows(t *testing.T) {
	version := []string{"localizations", "placements", "list", "--localization-id", "loc"}
	customPage := []string{"product-pages", "custom-pages", "localizations", "placements", "list", "--localization-id", "loc"}
	treatment := []string{"product-pages", "experiments", "treatments", "localizations", "placements", "list", "--localization-id", "loc"}
	event := []string{"app-events", "localizations", "placements", "list", "--localization-id", "loc"}
	image := []string{"asset-library", "images", "placements", "--id", "asset"}
	video := []string{"asset-library", "videos", "placements", "--id", "asset"}
	for _, tc := range []struct {
		base  []string
		path  string
		flags []string
		query url.Values
	}{
		{version, "/v1/appStoreVersionLocalizations/loc/placements", nil, url.Values{}},
		{version, "/v1/appStoreVersionLocalizations/loc/placements", []string{"--state", "ASSET_PROCESSING, FAILED", "--image-id", "i1,i2", "--video-id", "v1", "--placement-id", "p1,p2", "--sort", "-createdDate,lastModifiedDate"}, url.Values{"filter[state]": {"ASSET_PROCESSING,FAILED"}, "filter[image]": {"i1,i2"}, "filter[video]": {"v1"}, "filter[id]": {"p1,p2"}, "sort": {"-createdDate,lastModifiedDate"}}},
		{version, "/v1/appStoreVersionLocalizations/loc/placements", []string{"--state", "ACTIVE"}, url.Values{"filter[state]": {"ACTIVE"}}},
		{version, "/v1/appStoreVersionLocalizations/loc/placements", []string{"--event-localization", "e", "--custom-page-localization", "c", "--treatment-localization", "t"}, url.Values{"filter[appEventLocalization]": {"e"}, "filter[appCustomProductPageLocalization]": {"c"}, "filter[appStoreVersionExperimentTreatmentLocalization]": {"t"}}},
		{customPage, "/v1/appCustomProductPageLocalizations/loc/placements", []string{"--event-localization", "e", "--version-localization", "s", "--treatment-localization", "t", "--sort", "-placementGroupPosition"}, url.Values{"filter[appEventLocalization]": {"e"}, "filter[appStoreVersionLocalization]": {"s"}, "filter[appStoreVersionExperimentTreatmentLocalization]": {"t"}, "sort": {"-placementGroupPosition"}}},
		{treatment, "/v1/appStoreVersionExperimentTreatmentLocalizations/loc/placements", []string{"--event-localization", "e", "--version-localization", "s", "--custom-page-localization", "c"}, url.Values{"filter[appEventLocalization]": {"e"}, "filter[appStoreVersionLocalization]": {"s"}, "filter[appCustomProductPageLocalization]": {"c"}}},
		{event, "/v1/appEventLocalizations/loc/placements", []string{"--version-localization", "s", "--custom-page-localization", "c", "--treatment-localization", "t"}, url.Values{"filter[appStoreVersionLocalization]": {"s"}, "filter[appCustomProductPageLocalization]": {"c"}, "filter[appStoreVersionExperimentTreatmentLocalization]": {"t"}}},
		{image, "/v1/appAssetLibraryImages/asset/placements", nil, url.Values{}},
		{image, "/v1/appAssetLibraryImages/asset/placements", []string{"--placement-type", "RETENTION_MESSAGE_ASSET,APP_SCREENSHOT", "--placement-group", "IPHONE_DUO_PROFILE", "--state", "FAILED", "--video-id", "v", "--placement-id", "p", "--sort", "placementGroupPosition"}, url.Values{"filter[placementType]": {"RETENTION_MESSAGE_ASSET,APP_SCREENSHOT"}, "filter[placementGroup]": {"IPHONE_DUO_PROFILE"}, "filter[state]": {"FAILED"}, "filter[video]": {"v"}, "filter[id]": {"p"}, "sort": {"placementGroupPosition"}}},
		{image, "/v1/appAssetLibraryImages/asset/placements", []string{"--event-localization", "e", "--version-localization", "s", "--custom-page-localization", "c", "--treatment-localization", "t"}, url.Values{"filter[appEventLocalization]": {"e"}, "filter[appStoreVersionLocalization]": {"s"}, "filter[appCustomProductPageLocalization]": {"c"}, "filter[appStoreVersionExperimentTreatmentLocalization]": {"t"}}},
		{video, "/v1/appAssetLibraryVideos/asset/placements", []string{"--placement-type", "TODAY_TAB_ADS_ASSET", "--image-id", "i", "--sort", "-lastModifiedDate"}, url.Values{"filter[placementType]": {"TODAY_TAB_ADS_ASSET"}, "filter[image]": {"i"}, "sort": {"-lastModifiedDate"}}},
	} {
		t.Run(strings.Join(append(tc.base[:len(tc.base)-2:len(tc.base)-2], tc.flags...), " "), func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			calls := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Method != "GET" || req.URL.Path != tc.path {
					t.Fatalf("unexpected %s %s", req.Method, req.URL)
				}
				if got := req.URL.Query(); !reflect.DeepEqual(got, tc.query) {
					t.Fatalf("query=%v want %v", got, tc.query)
				}
				return jsonResponse(200, `{"data":[]}`)
			})
			_, stderr, err := runCreativePlacements(t, append(tc.base[:len(tc.base):len(tc.base)], tc.flags...)...)
			if err != nil || stderr != "" || calls != 1 {
				t.Fatalf("error=%v stderr=%q calls=%d", err, stderr, calls)
			}
		})
	}
}

// Apple's spec lists RETENTION_MESSAGE_ASSET, but production rejects it as a
// placements filter with HTTP 400, so it must stop as a usage error locally.
func TestCreativePlacementsListRejectsRetentionMessageType(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s", req.Method, req.URL)
		return nil, nil
	})
	for _, command := range []string{"localizations", "app-events localizations", "product-pages custom-pages localizations", "product-pages experiments treatments localizations"} {
		t.Run(command, func(t *testing.T) {
			setupAuth(t)
			args := append(strings.Fields(command), "placements", "list", "--localization-id", "loc", "--placement-type", "APP_SCREENSHOT,RETENTION_MESSAGE_ASSET")
			stdout, stderr, err := runCreativePlacements(t, args...)
			if !isUsageClassError(err) || stdout != "" || !strings.Contains(stderr, "--placement-type") {
				t.Fatalf("expected placement type usage error: %s %s %v", stdout, stderr, err)
			}
		})
	}
}

func TestPlacementListsOmitFiltersTheirOperationRejects(t *testing.T) {
	root := RootCommand("dev")
	for _, tc := range []struct {
		path   []string
		absent []string
	}{
		{[]string{"localizations", "placements", "list"}, []string{"version-localization"}},
		{[]string{"product-pages", "custom-pages", "localizations", "placements", "list"}, []string{"custom-page-localization"}},
		{[]string{"product-pages", "experiments", "treatments", "localizations", "placements", "list"}, []string{"treatment-localization"}},
		{[]string{"app-events", "localizations", "placements", "list"}, []string{"event-localization"}},
		{[]string{"asset-library", "images", "placements"}, []string{"image-id"}},
		{[]string{"asset-library", "videos", "placements"}, []string{"video-id"}},
	} {
		command := findSubcommand(root, tc.path...)
		if command == nil {
			t.Fatalf("missing command %v", tc.path)
		}
		for _, name := range tc.absent {
			if command.FlagSet.Lookup(name) != nil {
				t.Fatalf("%v binds --%s, which Apple does not accept on this operation", tc.path, name)
			}
		}
	}
}

func TestCreativePlacementsRejectOverLimitBeforeAuth(t *testing.T) {
	for _, path := range [][]string{
		{"localizations", "placements", "list"},
		{"product-pages", "custom-pages", "localizations", "placements", "list"},
	} {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			args := append(path, "--localization-id", "loc", "--limit", "201")
			stdout, stderr, err := runCreativePlacements(t, args...)
			if !isUsageClassError(err) || stdout != "" || !strings.Contains(stderr, "--limit") {
				t.Fatalf("expected over-limit usage before auth, got stdout=%q stderr=%q error=%v", stdout, stderr, err)
			}
		})
	}
}

func TestCreativePlacementsPaginateEmptyAndAPIError(t *testing.T) {
	setupAuth(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	calls := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return jsonResponse(200, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc/placements?cursor=2"}}`)
		}
		return jsonResponse(200, `{"data":[]}`)
	})
	stdout, _, err := runCreativePlacements(t, "localizations", "placements", "list", "--localization-id", "loc", "--paginate")
	if err != nil || calls != 2 || !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("empty pages failed %s %v %d", stdout, err, calls)
	}
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`)
	})
	stdout, _, err = runCreativePlacements(t, "localizations", "placements", "list", "--localization-id", "loc")
	if err == nil || stdout != "" {
		t.Fatalf("API error accepted: %s %v", stdout, err)
	}
}

func TestCustomPageCreativePlacements(t *testing.T) {
	setupAuth(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != "GET" || req.URL.Path != "/v1/appCustomProductPageLocalizations/loc/placements" || req.URL.Query().Get("filter[placementType]") != "PRODUCT_PAGE_HEADER_ASSET" {
			t.Fatalf("unexpected route: %s", req.URL)
		}
		return jsonResponse(200, `{"data":[]}`)
	})
	stdout, stderr, err := runCreativePlacements(t, "product-pages", "custom-pages", "localizations", "placements", "list", "--localization-id", "loc", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET")
	if err != nil || stderr != "" || !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("failed: %s %s %v", stdout, stderr, err)
	}
}

var creativePlacementGroups = []string{
	"localizations placements",
	"product-pages custom-pages localizations placements",
	"product-pages experiments treatments localizations placements",
	"app-events localizations placements",
}

// Regression: a group loses view, the query params drift from the spec, or JSON stops being Apple's envelope.
func TestCreativePlacementViewReadsOneResourceInEveryContext(t *testing.T) {
	const body = `{"data":{"type":"appAssetLibraryPlacements","id":"placement-1","attributes":{"mediaType":"IMAGE","placementType":"PRODUCT_PAGE_HEADER_ASSET","placementGroup":"DEFAULT_PROFILE","state":"READY","unknown":null}},"included":[{"type":"appAssetLibraryImages","id":"img","attributes":{"future":true}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/appAssetLibraryPlacements/placement-1"},"newTopLevel":{"retain":true}}`
	for _, group := range creativePlacementGroups {
		for _, output := range []string{"json", "table"} {
			t.Run(group+" "+output, func(t *testing.T) {
				setupAuth(t)
				original := http.DefaultTransport
				t.Cleanup(func() { http.DefaultTransport = original })
				http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != "GET" || req.URL.Path != "/v1/appAssetLibraryPlacements/placement-1" {
						t.Fatalf("unexpected %s %s", req.Method, req.URL)
					}
					q := req.URL.Query()
					if q.Get("include") != "image,appEventLocalization,retentionMessageVersion" || q.Get("fields[appAssetLibraryPlacements]") != "state,image,retentionMessageVersion" || len(q) != 2 {
						t.Fatalf("query=%v", q)
					}
					return jsonResponse(200, body)
				})
				args := append(strings.Fields(group), "view", "--id", "placement-1", "--include", "image,appEventLocalization,retentionMessageVersion", "--fields", "state,image,retentionMessageVersion", "--output", output)
				stdout, stderr, err := runCreativePlacements(t, args...)
				if err != nil || stderr != "" {
					t.Fatalf("error=%v stderr=%q", err, stderr)
				}
				if output == "table" {
					if !strings.Contains(stdout, "placement-1") || !strings.Contains(stdout, "PRODUCT_PAGE_HEADER_ASSET") {
						t.Fatalf("missing placement: %s", stdout)
					}
					return
				}
				var got, want any
				if err := json.Unmarshal([]byte(stdout), &got); err != nil {
					t.Fatalf("stdout=%q: %v", stdout, err)
				}
				_ = json.Unmarshal([]byte(body), &want)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("changed envelope: %s", stdout)
				}
			})
		}
	}
}

// Regression: view prints a collection, an empty body, or an API failure as a successful read.
func TestCreativePlacementViewRefusesFalseSuccess(t *testing.T) {
	for name, response := range map[string]struct {
		status int
		body   string
	}{
		"collection": {200, `{"data":[{"type":"appAssetLibraryPlacements","id":"placement-1"}]}`},
		"wrong type": {200, `{"data":{"type":"apps","id":"placement-1"}}`},
		"empty":      {200, `{}`},
		"not found":  {404, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not found"}]}`},
	} {
		t.Run(name, func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(response.status, response.body)
			})
			stdout, _, err := runCreativePlacements(t, "localizations", "placements", "view", "--id", "placement-1")
			if err == nil || isUsageClassError(err) || stdout != "" {
				t.Fatalf("stdout=%q err=%v", stdout, err)
			}
		})
	}
}

// Regression: bad input reaches auth or the network, or an unsupported value is silently dropped.
func TestCreativePlacementViewUsage(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"--id", "   "},
		{"--id", "../apps"},
		{"--id", "placement-1", "extra"},
		{"--id", "placement-1", "--include", "app"},
		{"--id", "placement-1", "--include", "image,"},
		{"--id", "placement-1", "--fields", "placementGroupPosition"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			full := append([]string{"localizations", "placements", "view"}, args...)
			_, stderr, err := runCreativePlacements(t, full...)
			if !isUsageClassError(err) || !strings.Contains(stderr, "localizations placements view: ") {
				t.Fatalf("expected usage before auth, got %v %q", err, stderr)
			}
		})
	}
}
