package cmdtest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestCreativePlacementCreate(t *testing.T) {
	for _, kind := range []string{"PRODUCT_PAGE_HEADER_ASSET", "APP_STORE_SEARCH_RESULTS_ASSET"} {
		t.Run(kind, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			for range 3 {
				reg.Register(placementCreateRequest(kind), asctest.JSON(201, `{"data":{"id":"placement","type":"appAssetLibraryPlacements","attributes":{"state":"ACTIVE","future":null}}}`))
			}
			stdout, stderr, err := runCreativePlacements(t, "localizations", "placements", "create", "--localization-id", "loc", "--image-id", "image", "--placement-type", kind)
			var receipt map[string]any
			if err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &receipt) != nil {
				t.Fatalf("failed %s %s %v", stdout, stderr, err)
			}
			if receipt["id"] != "placement" || receipt["created"] != true || receipt["state"] != "ACTIVE" || receipt["localizationId"] != "loc" || receipt["imageId"] != "image" || receipt["placementType"] != kind || receipt["placementGroup"] != "DEFAULT_PROFILE" {
				t.Fatalf("receipt=%v", receipt)
			}
			for _, output := range []string{"table", "markdown"} {
				stdout, stderr, err = runCreativePlacements(t, "localizations", "placements", "create", "--localization-id", "loc", "--image-id", "image", "--placement-type", kind, "--output", output)
				if err != nil || stderr != "" || !strings.Contains(stdout, kind) || !strings.Contains(stdout, "ACTIVE") {
					t.Fatalf("human receipt=%s %s %v", stdout, stderr, err)
				}
			}
		})
	}
}

func placementCreateRequest(kind string) asctest.Matcher {
	return asctest.POST("/v1/appAssetLibraryPlacements").JSONBody(`{"data":{"attributes":{"placementGroup":"DEFAULT_PROFILE","placementType":"` + kind + `"},"relationships":{"appStoreVersionLocalization":{"data":{"id":"loc","type":"appStoreVersionLocalizations"}},"image":{"data":{"id":"image","type":"appAssetLibraryImages"}}},"type":"appAssetLibraryPlacements"}}`)
}

func TestCreativePlacementCreateRefusesFalseSuccess(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	for _, response := range []struct {
		status int
		body   string
	}{
		{403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`},
		{201, `{"data":{}}`},
	} {
		reg.Register(placementCreateRequest("PRODUCT_PAGE_HEADER_ASSET"), asctest.JSON(response.status, response.body))
		stdout, _, err := runCreativePlacements(t, "localizations", "placements", "create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET")
		if err == nil || stdout != "" {
			t.Fatalf("false create success status=%d output=%s error=%v", response.status, stdout, err)
		}
	}
}

func TestCreativePlacementWriteUsageBeforeAuth(t *testing.T) {
	for _, args := range [][]string{
		{"create"},
		{"create", "--localization-id", "loc", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
		{"create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "APP_SCREENSHOT"},
		{"create", "--localization-id", "../loc", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
		{"delete", "--id", "placement"},
		{"delete", "--confirm"},
		{"delete", "--id", "../placement", "--confirm"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, err := runCreativePlacements(t, append([]string{"localizations", "placements"}, args...)...)
			if !isUsageClassError(err) || stdout != "" || stderr == "" {
				t.Fatalf("expected usage before auth: %s %s %v", stdout, stderr, err)
			}
		})
	}
}

// Read filter support does not establish creation support, so the wider list
// filter must not widen create or swap.
func TestCreativePlacementWritesRejectFilterOnlyTypes(t *testing.T) {
	asctest.NewRegistry(t).Install()
	for _, tc := range []struct{ command, operation, kind string }{
		{"localizations", "create", "SEARCH_RESULTS_ADS_ASSET"},
		{"app-events localizations", "create", "PRODUCT_PAGE_HEADER_ASSET"},
		{"product-pages custom-pages localizations", "create", "IMESSAGE_APP_SCREENSHOT"},
		{"product-pages experiments treatments localizations", "swap", "IMESSAGE_APP_SCREENSHOT"},
	} {
		t.Run(tc.command+" "+tc.operation, func(t *testing.T) {
			args := append(strings.Fields(tc.command), "placements", tc.operation, "--localization-id", "loc", "--image-id", "image", "--placement-type", tc.kind)
			stdout, stderr, err := runCreativePlacements(t, args...)
			if !isUsageClassError(err) || stdout != "" || !strings.Contains(stderr, "--placement-type must be one of") {
				t.Fatalf("expected placement type usage error: %s %s %v", stdout, stderr, err)
			}
		})
	}
}

func TestCreativePlacementDeleteReceiptAndFailure(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	for range 3 {
		reg.Register(asctest.DELETE("/v1/appAssetLibraryPlacements/placement"), asctest.JSON(204, ""))
	}
	reg.Register(asctest.DELETE("/v1/appAssetLibraryPlacements/placement"),
		asctest.JSON(403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`))
	for _, output := range []string{"json", "table", "markdown"} {
		stdout, stderr, err := runCreativePlacements(t, "localizations", "placements", "delete", "--id", "placement", "--confirm", "--output", output)
		if err != nil || stderr != "" || !strings.Contains(stdout, "placement") {
			t.Fatalf("output=%s %s %v", stdout, stderr, err)
		}
		if output == "json" {
			var receipt map[string]any
			if json.Unmarshal([]byte(stdout), &receipt) != nil || receipt["deleted"] != true {
				t.Fatalf("receipt=%s", stdout)
			}
		}
	}
	stdout, _, err := runCreativePlacements(t, "localizations", "placements", "delete", "--id", "placement", "--confirm")
	if err == nil || stdout != "" {
		t.Fatalf("false success %s %v", stdout, err)
	}
}
