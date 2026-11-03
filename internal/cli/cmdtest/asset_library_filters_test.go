package cmdtest

import (
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"
)

func TestAssetLibraryListFilters(t *testing.T) {
	for _, group := range []string{"images", "videos"} {
		t.Run(group, func(t *testing.T) {
			for _, top := range RootCommand("dev").Subcommands {
				if top.Name != "asset-library" {
					continue
				}
				for _, media := range top.Subcommands {
					if media.Name != group {
						continue
					}
					for _, command := range media.Subcommands {
						if command.Name == "list" && command.FlagSet.Lookup("reference-name") == nil {
							t.Fatal("list missing official --reference-name filter")
						}
					}
				}
			}
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				if req.Method != "GET" || req.URL.Path != "/v1/appAssetLibraries/lib/"+group || q.Get("filter[category]") != "CREATIVE_ASSETS,APP_SCREENSHOTS_AND_PREVIEWS" || q.Get("filter[state]") != "FAILED,FUTURE_STATE" || q.Get("filter[specId]") != "spec-1,spec-2" || q.Get("sort") != "referenceName,-lastModifiedDate,createdDate,-createdDate" || q.Get("filter[referenceName]") != "Header & Search,Spring Campaign" || q.Get("filter[id]") != "asset-1,asset-2" || q.Get("limit") != "2" {
					t.Fatalf("unexpected query: %s", req.URL)
				}
				if strings.Contains(req.URL.RawQuery, "[") {
					t.Fatalf("query not encoded: %s", req.URL.RawQuery)
				}
				return jsonResponse(200, `{"data":[],"meta":{"unknown":{"preserved":true}}}`)
			})
			out, _, err := runAssetLibrary(t, "asset-library", group, "list", "--library-id", "https://api.appstoreconnect.apple.com/v1/appAssetLibraries/lib", "--category", "CREATIVE_ASSETS, APP_SCREENSHOTS_AND_PREVIEWS", "--state", "FAILED,FUTURE_STATE", "--spec-id", "spec-1,spec-2", "--sort", "referenceName,-lastModifiedDate,createdDate,-createdDate", "--reference-name", "Header & Search, Spring Campaign", "--id", "asset-1,asset-2", "--limit", "2", "--output", "json")
			if err != nil || !strings.Contains(out, `"unknown":{"preserved":true}`) {
				t.Fatalf("out=%s err=%v", out, err)
			}
		})
	}
}

func TestAssetLibraryListFilterValidation(t *testing.T) {
	for _, tc := range []struct{ flag, value string }{{"reference-name", ""}, {"reference-name", "Header,,Search"}, {"id", "asset-1,"}, {"id", "../asset"}, {"category", "OTHER"}, {"category", "CREATIVE_ASSETS,"}, {"state", "FAILED,,APPROVED"}, {"state", "bad state"}, {"spec-id", "../spec"}, {"sort", "state"}, {"sort", "referenceName,"}} {
		t.Run(tc.flag+tc.value, func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			calls := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) { calls++; return jsonResponse(200, `{"data":[]}`) })
			_, _, err := runAssetLibrary(t, "asset-library", "images", "list", "--library-id", "lib", "--"+tc.flag, tc.value)
			if err == nil || calls != 0 {
				t.Fatalf("expected zeroHTTP usage error: %v calls%d", err, calls)
			}
		})
	}
	for _, flag := range []string{"category", "state", "spec-id", "sort", "reference-name", "id"} {
		t.Run("next/"+flag, func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			calls := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) { calls++; return jsonResponse(200, `{"data":[]}`) })
			values := map[string]string{"category": "CREATIVE_ASSETS", "state": "FAILED", "spec-id": "spec-1", "sort": "referenceName", "reference-name": "Header & Search", "id": "asset-1"}
			_, _, err := runAssetLibrary(t, "asset-library", "videos", "list", "--next", "https://api.appstoreconnect.apple.com/v1/appAssetLibraries/lib/videos?cursor=next", "--"+flag, values[flag])
			if err == nil || calls != 0 || !strings.Contains(err.Error(), "--next") {
				t.Fatalf("expected specific next conflict: %v", err)
			}
		})
	}
}

func TestAssetLibraryListFilterPagination(t *testing.T) {
	setupAuth(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	calls := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			if req.URL.Query().Get("filter[state]") != "FAILED" {
				t.Fatalf("missing initial filter: %s", req.URL)
			}
			return jsonResponse(200, `{"data":[{"type":"appAssetLibraryImages","id":"a","attributes":{"future":true}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appAssetLibraries/lib/images?cursor=opaque&filter%5Bstate%5D=FAILED"}}`)
		}
		if req.URL.Query().Get("cursor") != "opaque" || req.URL.Query().Get("filter[state]") != "FAILED" {
			t.Fatalf("next changed: %s", req.URL)
		}
		return jsonResponse(200, `{"data":[{"type":"appAssetLibraryImages","id":"b","attributes":{"future":null}}],"links":{"next":null}}`)
	})
	out, _, err := runAssetLibrary(t, "asset-library", "images", "list", "--library-id", "lib", "--state", "FAILED", "--paginate", "--output", "json")
	if err != nil || calls != 2 || !strings.Contains(out, `"id":"b"`) || !strings.Contains(out, `"future":null`) {
		t.Fatalf("pagination lost data: %s calls%d %v", out, calls, err)
	}
}

func TestAssetLibrarySpecsFilters(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want map[string]string
	}{
		{name: "omitted flags send no query"},
		{
			name: "every filter encodes as a comma-separated list",
			args: []string{"--placement-type", "APP_SCREENSHOT, FUTURE_PLACEMENT", "--placement-profile-group", "IPHONE_DUO_PROFILE,future.group", "--feature", "APP_STORE_VERSIONS,FUTURE_FEATURE", "--spec", "duo-shot,future.spec"},
			want: map[string]string{
				"filter[placementTypes]":         "APP_SCREENSHOT,FUTURE_PLACEMENT",
				"filter[placementProfileGroups]": "IPHONE_DUO_PROFILE,future.group",
				"filter[features]":               "APP_STORE_VERSIONS,FUTURE_FEATURE",
				"filter[specs]":                  "duo-shot,future.spec",
			},
		},
		{name: "one filter sends only its key", args: []string{"--feature", "GAME_CENTER"}, want: map[string]string{"filter[features]": "GAME_CENTER"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			calls := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				q := req.URL.Query()
				if req.Method != "GET" || req.URL.Path != "/v1/appAssetLibraryRefData" || len(q) != len(tc.want) {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
				}
				for key, value := range tc.want {
					if q.Get(key) != value {
						t.Fatalf("%s=%q want %q in %s", key, q.Get(key), value, req.URL)
					}
				}
				if strings.Contains(req.URL.RawQuery, "[") {
					t.Fatalf("query not encoded: %s", req.URL.RawQuery)
				}
				return jsonResponse(200, `{"data":[],"links":{"self":"https://example.test"},"future":true}`)
			})
			out, _, err := runAssetLibrary(t, append([]string{"asset-library", "specs", "--output", "json"}, tc.args...)...)
			if err != nil || calls != 1 || !strings.Contains(out, `"future":true`) {
				t.Fatalf("out=%s calls=%d err=%v", out, calls, err)
			}
		})
	}
}

func TestAssetLibrarySpecsFilterValidation(t *testing.T) {
	for _, tc := range []struct{ flag, value string }{{"placement-type", ""}, {"placement-profile-group", "A,,B"}, {"feature", "GAME_CENTER,"}, {"spec", " "}} {
		t.Run(tc.flag+"/"+tc.value, func(t *testing.T) {
			setupAuth(t)
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			calls := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) { calls++; return jsonResponse(200, `{"data":[]}`) })
			_, _, err := runAssetLibrary(t, "asset-library", "specs", "--"+tc.flag, tc.value)
			if !errors.Is(err, flag.ErrHelp) || calls != 0 || !strings.Contains(err.Error(), "--"+tc.flag) {
				t.Fatalf("expected usage error naming --%s with zero HTTP: %v calls%d", tc.flag, err, calls)
			}
		})
	}
}
