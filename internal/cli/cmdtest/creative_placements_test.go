package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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
		{"--localization-id", "loc", "--limit", "-1"},
		{"--localization-id", "../apps"},
		{"--next", "https://evil.example/v1/apps"},
		{"--next", "https://api.appstoreconnect.apple.com/v1/apps", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
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
		{"retention type", "localizations", "appStoreVersionLocalizations", "--placement-type", "RETENTION_MESSAGE_ASSET", "filter%5BplacementType%5D=RETENTION_MESSAGE_ASSET"},
		{"event type outside events", "localizations", "appStoreVersionLocalizations", "--placement-type", "EVENT_CARD_ASSET", "filter%5BplacementType%5D=EVENT_CARD_ASSET"},
		{"store type on events", "app-events localizations", "appEventLocalizations", "--placement-type", "APP_SCREENSHOT", "filter%5BplacementType%5D=APP_SCREENSHOT"},
		{"iMessage type on treatments", "product-pages experiments treatments localizations", "appStoreVersionExperimentTreatmentLocalizations", "--placement-type", "IMESSAGE_APP_SCREENSHOT", "filter%5BplacementType%5D=IMESSAGE_APP_SCREENSHOT"},
		{"parent includes", "localizations", "appStoreVersionLocalizations", "--include", "appEventLocalization,appStoreVersionLocalization,appCustomProductPageLocalization,appStoreVersionExperimentTreatmentLocalization", "include=appEventLocalization%2CappStoreVersionLocalization%2CappCustomProductPageLocalization%2CappStoreVersionExperimentTreatmentLocalization"},
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
