package cmdtest

import (
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/appAssetLibraries/lib/"+group, "filter[category]=CREATIVE_ASSETS,APP_SCREENSHOTS_AND_PREVIEWS", "filter[state]=FAILED,FUTURE_STATE", "filter[specId]=spec-1,spec-2", "sort=referenceName,-lastModifiedDate,createdDate,-createdDate", "filter[referenceName]=Header %26 Search,Spring Campaign", "filter[id]=asset-1,asset-2", "limit=2"),
				asctest.JSON(200, `{"data":[],"meta":{"unknown":{"preserved":true}}}`))
			out, _, err := runAssetLibrary(t, "asset-library", group, "list", "--library-id", "https://api.appstoreconnect.apple.com/v1/appAssetLibraries/lib", "--category", "CREATIVE_ASSETS, APP_SCREENSHOTS_AND_PREVIEWS", "--state", "FAILED,FUTURE_STATE", "--spec-id", "spec-1,spec-2", "--sort", "referenceName,-lastModifiedDate,createdDate,-createdDate", "--reference-name", "Header & Search, Spring Campaign", "--id", "asset-1,asset-2", "--limit", "2", "--output", "json")
			for _, req := range reg.Requests() {
				if strings.Contains(req.URL.RawQuery, "[") {
					t.Fatalf("query not encoded: %s", req.URL.RawQuery)
				}
			}
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
			reg := asctest.NewRegistry(t)
			reg.Install()
			_, _, err := runAssetLibrary(t, "asset-library", "images", "list", "--library-id", "lib", "--"+tc.flag, tc.value)
			calls := len(reg.Requests())
			if err == nil || calls != 0 {
				t.Fatalf("expected zeroHTTP usage error: %v calls%d", err, calls)
			}
		})
	}
	for _, flag := range []string{"category", "state", "spec-id", "sort", "reference-name", "id"} {
		t.Run("next/"+flag, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			values := map[string]string{"category": "CREATIVE_ASSETS", "state": "FAILED", "spec-id": "spec-1", "sort": "referenceName", "reference-name": "Header & Search", "id": "asset-1"}
			_, _, err := runAssetLibrary(t, "asset-library", "videos", "list", "--next", "https://api.appstoreconnect.apple.com/v1/appAssetLibraries/lib/videos?cursor=next", "--"+flag, values[flag])
			calls := len(reg.Requests())
			if err == nil || calls != 0 || !strings.Contains(err.Error(), "--next") {
				t.Fatalf("expected specific next conflict: %v", err)
			}
		})
	}
}

func TestAssetLibraryListFilterPagination(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appAssetLibraries/lib/images", "filter[state]=FAILED", "limit=200"),
		asctest.JSON(200, `{"data":[{"type":"appAssetLibraryImages","id":"a","attributes":{"future":true}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appAssetLibraries/lib/images?cursor=opaque&filter%5Bstate%5D=FAILED"}}`))
	reg.Register(asctest.GET("/v1/appAssetLibraries/lib/images", "cursor=opaque", "filter[state]=FAILED"),
		asctest.JSON(200, `{"data":[{"type":"appAssetLibraryImages","id":"b","attributes":{"future":null}}],"links":{"next":null}}`))
	out, _, err := runAssetLibrary(t, "asset-library", "images", "list", "--library-id", "lib", "--state", "FAILED", "--paginate", "--output", "json")
	calls := len(reg.Requests())
	if err != nil || calls != 2 || !strings.Contains(out, `"id":"b"`) || !strings.Contains(out, `"future":null`) {
		t.Fatalf("pagination lost data: %s calls%d %v", out, calls, err)
	}
}

func TestAssetLibrarySpecsFilters(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{name: "omitted flags send no query"},
		{
			name: "every filter encodes as a comma-separated list",
			args: []string{"--placement-type", "APP_SCREENSHOT, FUTURE_PLACEMENT", "--placement-profile-group", "IPHONE_DUO_PROFILE,future.group", "--feature", "APP_STORE_VERSIONS,FUTURE_FEATURE", "--spec", "duo-shot,future.spec"},
			want: []string{
				"filter[placementTypes]=APP_SCREENSHOT,FUTURE_PLACEMENT",
				"filter[placementProfileGroups]=IPHONE_DUO_PROFILE,future.group",
				"filter[features]=APP_STORE_VERSIONS,FUTURE_FEATURE",
				"filter[specs]=duo-shot,future.spec",
			},
		},
		{name: "one filter sends only its key", args: []string{"--feature", "GAME_CENTER"}, want: []string{"filter[features]=GAME_CENTER"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/appAssetLibraryRefData", tc.want...),
				asctest.JSON(200, `{"data":[],"links":{"self":"https://example.test"},"future":true}`))
			out, _, err := runAssetLibrary(t, append([]string{"asset-library", "specs", "--output", "json"}, tc.args...)...)
			requests := reg.Requests()
			for _, req := range requests {
				if strings.Contains(req.URL.RawQuery, "[") {
					t.Fatalf("query not encoded: %s", req.URL.RawQuery)
				}
			}
			calls := len(requests)
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
			reg := asctest.NewRegistry(t)
			reg.Install()
			_, _, err := runAssetLibrary(t, "asset-library", "specs", "--"+tc.flag, tc.value)
			calls := len(reg.Requests())
			if !errors.Is(err, flag.ErrHelp) || calls != 0 || !strings.Contains(err.Error(), "--"+tc.flag) {
				t.Fatalf("expected usage error naming --%s with zero HTTP: %v calls%d", tc.flag, err, calls)
			}
		})
	}
}
