package cmdtest

import (
	"encoding/json"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestCreativePlacementCreate(t *testing.T) {
	for _, kind := range []string{"PRODUCT_PAGE_HEADER_ASSET", "APP_STORE_SEARCH_RESULTS_ASSET"} {
		t.Run(kind, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			for range 3 {
				reg.Register(placementCreateRequest(kind), asctest.JSON(201, `{"data":{"id":"placement","type":"appAssetLibraryPlacements","attributes":{"state":"ACTIVE","future":null}}}`))
			}
			stdout, stderr, err := runCreativePlacementsFor(t, reg, "localizations", "placements", "create", "--localization-id", "loc", "--image-id", "image", "--placement-type", kind)
			var receipt map[string]any
			if err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &receipt) != nil {
				t.Fatalf("failed %s %s %v", stdout, stderr, err)
			}
			if receipt["id"] != "placement" || receipt["created"] != true || receipt["state"] != "ACTIVE" || receipt["localizationId"] != "loc" || receipt["imageId"] != "image" || receipt["placementType"] != kind || receipt["placementGroup"] != "DEFAULT_PROFILE" {
				t.Fatalf("receipt=%v", receipt)
			}
			for _, output := range []string{"table", "markdown"} {
				stdout, stderr, err = runCreativePlacementsFor(t, reg, "localizations", "placements", "create", "--localization-id", "loc", "--image-id", "image", "--placement-type", kind, "--output", output)
				if err != nil || stderr != "" || !strings.Contains(stdout, kind) || !strings.Contains(stdout, "ACTIVE") {
					t.Fatalf("human receipt=%s %s %v", stdout, stderr, err)
				}
			}
			if got, want := len(reg.Requests()), 3; got != want {
				t.Fatalf("requests=%d, want %d successful create requests", got, want)
			}
		})
	}
}

func placementCreateRequest(kind string) asctest.Matcher {
	return asctest.POST("/v1/appAssetLibraryPlacements").JSONBody(`{"data":{"attributes":{"placementGroup":"DEFAULT_PROFILE","placementType":"` + kind + `"},"relationships":{"appStoreVersionLocalization":{"data":{"id":"loc","type":"appStoreVersionLocalizations"}},"image":{"data":{"id":"image","type":"appAssetLibraryImages"}}},"type":"appAssetLibraryPlacements"}}`)
}

func TestCreativePlacementCreateRefusesFalseSuccess(t *testing.T) {
	reg := asctest.NewRegistry(t)
	for _, response := range []struct {
		status int
		body   string
	}{
		{403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`},
		{201, `{"data":{}}`},
	} {
		reg.Register(placementCreateRequest("PRODUCT_PAGE_HEADER_ASSET"), asctest.JSON(response.status, response.body))
		stdout, _, err := runCreativePlacementsFor(t, reg, "localizations", "placements", "create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET")
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
			stdout, stderr, err := runCreativePlacementsFor(t, asctest.NewRegistry(t), append([]string{"localizations", "placements"}, args...)...)
			if !isUsageClassError(err) || stdout != "" || stderr == "" {
				t.Fatalf("expected usage before auth: %s %s %v", stdout, stderr, err)
			}
		})
	}
}

// Read filter support does not establish creation support, so the wider list
// filter must not widen create or swap.
func TestCreativePlacementWritesRejectFilterOnlyTypes(t *testing.T) {
	for _, tc := range []struct{ command, operation, kind string }{
		{"localizations", "create", "SEARCH_RESULTS_ADS_ASSET"},
		{"app-events localizations", "create", "PRODUCT_PAGE_HEADER_ASSET"},
		{"product-pages custom-pages localizations", "create", "IMESSAGE_APP_SCREENSHOT"},
		{"product-pages experiments treatments localizations", "swap", "IMESSAGE_APP_SCREENSHOT"},
	} {
		t.Run(tc.command+" "+tc.operation, func(t *testing.T) {
			args := append(strings.Fields(tc.command), "placements", tc.operation, "--localization-id", "loc", "--image-id", "image", "--placement-type", tc.kind)
			stdout, stderr, err := runCreativePlacementsFor(t, asctest.NewRegistry(t), args...)
			if !isUsageClassError(err) || stdout != "" || !strings.Contains(stderr, "--placement-type must be one of") {
				t.Fatalf("expected placement type usage error: %s %s %v", stdout, stderr, err)
			}
		})
	}
}

func TestCreativePlacementWriteDiagnosticsBeforeClient(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		code      shared.DiagnosticCode
		parameter string
		message   string
	}{
		{
			name:      "missing localization",
			args:      []string{"create", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
			code:      shared.DiagnosticRequiredInputMissing,
			parameter: "--localization-id",
			message:   "--localization-id requires a resource ID",
		},
		{
			name:      "invalid localization",
			args:      []string{"create", "--localization-id", "../loc", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
			code:      shared.DiagnosticInvalidInput,
			parameter: "--localization-id",
			message:   "--localization-id requires a resource ID",
		},
		{
			name:    "missing media",
			args:    []string{"create", "--localization-id", "loc", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
			code:    shared.DiagnosticRequiredInputMissing,
			message: "supply exactly one of --image-id or --video-id",
		},
		{
			name:    "conflicting media",
			args:    []string{"create", "--localization-id", "loc", "--image-id", "image", "--video-id", "video", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
			code:    shared.DiagnosticConflictingInput,
			message: "supply exactly one of --image-id or --video-id",
		},
		{
			name:      "invalid media",
			args:      []string{"create", "--localization-id", "loc", "--image-id", "../image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET"},
			code:      shared.DiagnosticInvalidInput,
			parameter: "--image-id",
			message:   "media ID must be a resource ID",
		},
		{
			name:      "missing placement type",
			args:      []string{"create", "--localization-id", "loc", "--image-id", "image"},
			code:      shared.DiagnosticRequiredInputMissing,
			parameter: "--placement-type",
			message:   "--placement-type must be one of",
		},
		{
			name:      "invalid placement type",
			args:      []string{"create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "NOT_A_PLACEMENT"},
			code:      shared.DiagnosticInvalidInput,
			parameter: "--placement-type",
			message:   "--placement-type must be one of",
		},
		{
			name:      "preview requires video",
			args:      []string{"create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "APP_PREVIEW", "--placement-group", "IPHONE_DUO_PROFILE"},
			code:      shared.DiagnosticRequiredInputMissing,
			parameter: "--video-id",
			message:   "APP_PREVIEW requires --video-id",
		},
		{
			name:      "screenshot requires image",
			args:      []string{"create", "--localization-id", "loc", "--video-id", "video", "--placement-type", "APP_SCREENSHOT", "--placement-group", "IPHONE_DUO_PROFILE"},
			code:      shared.DiagnosticRequiredInputMissing,
			parameter: "--image-id",
			message:   "screenshots require --image-id",
		},
		{
			name:      "missing native group",
			args:      []string{"create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "APP_SCREENSHOT"},
			code:      shared.DiagnosticRequiredInputMissing,
			parameter: "--placement-group",
			message:   "asc asset-library specs --placement-type APP_SCREENSHOT",
		},
		{
			name:      "invalid native group",
			args:      []string{"create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "APP_SCREENSHOT", "--placement-group", "DEFAULT_PROFILE"},
			code:      shared.DiagnosticInvalidInput,
			parameter: "--placement-group",
			message:   "screenshots/previews require a device --placement-group",
		},
		{
			name:      "invalid creative group",
			args:      []string{"create", "--localization-id", "loc", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET", "--placement-group", "IPHONE_DUO_PROFILE"},
			code:      shared.DiagnosticInvalidInput,
			parameter: "--placement-group",
			message:   "creative assets require DEFAULT_PROFILE",
		},
		{
			name:      "invalid swap selector",
			args:      []string{"swap", "--localization-id", "loc", "--placement-id", "../old", "--image-id", "image", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET", "--confirm"},
			code:      shared.DiagnosticInvalidInput,
			parameter: "--placement-id",
			message:   "--placement-id requires a resource ID",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			stdout, stderr, err := runCreativePlacementsFor(t, reg, append([]string{"localizations", "placements"}, tc.args...)...)
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code=%d, want %d; err=%v", got, rootcmd.ExitUsage, err)
			}
			if stdout != "" || !strings.Contains(stderr, tc.message) {
				t.Fatalf("stdout=%q stderr=%q, want message containing %q", stdout, stderr, tc.message)
			}
			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok || diagnostic.Code != tc.code || diagnostic.Parameter != tc.parameter {
				t.Fatalf("diagnostic=%+v found=%t, want code=%q parameter=%q", diagnostic, ok, tc.code, tc.parameter)
			}
			if got := len(reg.Requests()); got != 0 {
				t.Fatalf("requests=%d, want zero before client creation", got)
			}
			if tc.name == "missing native group" {
				setupAuth(t)
				specs := asctest.NewRegistry(t)
				specs.Register(asctest.GET("/v1/appAssetLibraryRefData", "filter[placementTypes]=APP_SCREENSHOT"), asctest.JSON(200, `{"data":[]}`))
				if stdout, stderr, err := runAssetLibrary(t, stubClient(t, specs), "asset-library", "specs", "--placement-type", "APP_SCREENSHOT"); err != nil || stderr != "" || stdout == "" || len(specs.Requests()) != 1 {
					t.Fatalf("suggested specs command failed: stdout=%q stderr=%q err=%v requests=%d", stdout, stderr, err, len(specs.Requests()))
				}
			}
		})
	}
}

func TestCreativePlacementDeleteReceiptAndFailure(t *testing.T) {
	reg := asctest.NewRegistry(t)
	for range 3 {
		reg.Register(asctest.DELETE("/v1/appAssetLibraryPlacements/placement"), asctest.JSON(204, ""))
	}
	reg.Register(asctest.DELETE("/v1/appAssetLibraryPlacements/placement"),
		asctest.JSON(403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`))
	for _, output := range []string{"json", "table", "markdown"} {
		stdout, stderr, err := runCreativePlacementsFor(t, reg, "localizations", "placements", "delete", "--id", "placement", "--confirm", "--output", output)
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
	stdout, _, err := runCreativePlacementsFor(t, reg, "localizations", "placements", "delete", "--id", "placement", "--confirm")
	if err == nil || stdout != "" {
		t.Fatalf("false success %s %v", stdout, err)
	}
}
