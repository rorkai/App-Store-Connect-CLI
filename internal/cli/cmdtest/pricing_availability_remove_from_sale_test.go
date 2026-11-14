package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestPricingAvailabilityRemoveFromSaleUpdatesAndVerifiesTerritories(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerRemoveFromSaleReads(reg, singlePlatformAppStoreVersions, true, territoryAvailabilityBody(true, false))
	registerRemoveFromSalePatch(reg, "ta-usa", http.StatusOK)
	registerTerritoryAvailabilitiesRead(reg, territoryAvailabilityBody(false, false))

	stdout, stderr, err := runCommand(t, []string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if got := countPatches(reg); got != 1 {
		t.Fatalf("PATCH count = %d, want 1", got)
	}
	var result struct {
		AppID                          string   `json:"appId"`
		AvailabilityID                 string   `json:"availabilityId"`
		Status                         string   `json:"status"`
		AvailableInNewTerritories      bool     `json:"availableInNewTerritories"`
		TotalTerritories               int      `json:"totalTerritories"`
		UpdatedTerritories             int      `json:"updatedTerritories"`
		AlreadyUnavailableTerritories  int      `json:"alreadyUnavailableTerritories"`
		VerifiedUnavailableTerritories int      `json:"verifiedUnavailableTerritories"`
		FailedTerritories              []string `json:"failedTerritories"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode result %q: %v", stdout, err)
	}
	if result.AppID != "app-1" || result.AvailabilityID != "availability-1" || result.Status != "removedFromSale" {
		t.Fatalf("unexpected identity result: %+v", result)
	}
	if !result.AvailableInNewTerritories {
		t.Fatalf("expected preserved new-territory policy, got %+v", result)
	}
	if result.TotalTerritories != 2 || result.UpdatedTerritories != 1 || result.AlreadyUnavailableTerritories != 1 || result.VerifiedUnavailableTerritories != 2 || len(result.FailedTerritories) != 0 {
		t.Fatalf("unexpected counts: %+v", result)
	}
	if !strings.Contains(stderr, "preserved availableInNewTerritories=true") {
		t.Fatalf("expected preserved-policy caveat, got %q", stderr)
	}
}

func TestPricingAvailabilityRemoveFromSaleRequiresConfirmWithUsageExit(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"pricing", "availability", "remove-from-sale", "--app", "app-1"}, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected confirmation diagnostic, got %q", stderr)
	}
}

func TestPricingAvailabilityRemoveFromSaleOutputFormats(t *testing.T) {
	setupAuth(t)

	tests := []struct {
		format string
		want   string
	}{
		{format: "table", want: "Availability ID"},
		{format: "markdown", want: "| Field"},
	}
	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			registerRemoveFromSaleReads(reg, singlePlatformAppStoreVersions, false, territoryAvailabilityBody(false, false))

			stdout, _, err := runCommand(t, []string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm", "--output", test.format})
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if !strings.Contains(stdout, test.want) {
				t.Fatalf("%s output missing %q: %s", test.format, test.want, stdout)
			}
			if !strings.Contains(stdout, "Platform listings verified") {
				t.Fatalf("%s output missing platform verification field: %s", test.format, stdout)
			}
		})
	}
}

func TestPricingAvailabilityRemoveFromSaleNoOp(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerRemoveFromSaleReads(reg, singlePlatformAppStoreVersions, false, territoryAvailabilityBody(false, false))

	stdout, _, err := runCommand(t, []string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if got := countPatches(reg); got != 0 {
		t.Fatalf("PATCH count = %d, want 0", got)
	}
	if !strings.Contains(stdout, `"updatedTerritories":0`) || !strings.Contains(stdout, `"alreadyUnavailableTerritories":2`) {
		t.Fatalf("unexpected no-op result: %s", stdout)
	}
}

func TestPricingAvailabilityRemoveFromSaleContinuesAfterPartialFailure(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerRemoveFromSaleReads(reg, singlePlatformAppStoreVersions, false, territoryAvailabilityBody(true, true))
	registerRemoveFromSalePatch(reg, "ta-usa", http.StatusOK)
	registerRemoveFromSalePatch(reg, "ta-fra", http.StatusUnprocessableEntity)
	registerTerritoryAvailabilitiesRead(reg, territoryAvailabilityBody(false, true))

	stdout, _, err := runCommand(t, []string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"})
	if err == nil {
		t.Fatal("expected partial failure")
	}
	if !strings.Contains(err.Error(), "updated 1, skipped 0, failed 1 (FRA)") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitError)
	}
	if got := countPatches(reg); got != 2 {
		t.Fatalf("PATCH count = %d, want 2", got)
	}
	var result struct {
		Status                         string   `json:"status"`
		TotalTerritories               int      `json:"totalTerritories"`
		UpdatedTerritories             int      `json:"updatedTerritories"`
		VerifiedUnavailableTerritories int      `json:"verifiedUnavailableTerritories"`
		FailedTerritories              []string `json:"failedTerritories"`
		RemovedPlatformListings        []struct {
			Platform string `json:"platform"`
		} `json:"removedPlatformListings"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode partial-failure result %q: %v", stdout, err)
	}
	if result.Status != "partialFailure" ||
		result.TotalTerritories != 2 ||
		result.UpdatedTerritories != 1 ||
		result.VerifiedUnavailableTerritories != 1 ||
		len(result.FailedTerritories) != 1 ||
		result.FailedTerritories[0] != "FRA" {
		t.Fatalf("unexpected partial-failure result: %+v", result)
	}
	if len(result.RemovedPlatformListings) != 0 {
		t.Fatalf("partial failure claimed removed platform listings: %+v", result.RemovedPlatformListings)
	}
}

func TestPricingAvailabilityRemoveFromSaleFinalReadbackIncludesInitiallyUnavailableTerritories(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerRemoveFromSaleReads(reg, singlePlatformAppStoreVersions, false, territoryAvailabilityBody(true, false))
	registerRemoveFromSalePatch(reg, "ta-usa", http.StatusOK)
	registerTerritoryAvailabilitiesRead(reg, territoryAvailabilityBody(false, true))

	_, _, err := runCommand(t, []string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"})
	if err == nil {
		t.Fatal("expected final-readback failure")
	}
	if !strings.Contains(err.Error(), "updated 1, skipped 1, failed 1 (FRA)") || !strings.Contains(err.Error(), "state changed during verification") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func territoryAvailabilityBody(usaAvailable, fraAvailable bool) string {
	return fmt.Sprintf(`{"data":[{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":%t},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},{"type":"territoryAvailabilities","id":"ta-fra","attributes":{"available":%t},"relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}}],"links":{"next":""}}`, usaAvailable, fraAvailable)
}

// registerRemoveFromSaleReads stubs the platform lookup, the availability and
// the first territory read that remove-from-sale makes before any PATCH.
func registerRemoveFromSaleReads(reg *asctest.Registry, versions string, availableInNewTerritories bool, territories string) {
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, versions))
	reg.Register(asctest.GET("/v1/apps/app-1/appAvailabilityV2"),
		asctest.JSON(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":%t}}}`, availableInNewTerritories)))
	registerTerritoryAvailabilitiesRead(reg, territories)
}

func registerRemoveFromSalePatch(reg *asctest.Registry, id string, status int) {
	resp := asctest.JSON(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":false}}}`)
	if status != http.StatusOK {
		resp = asctest.JSON(status, `{"errors":[{"status":"422","code":"ENTITY_ERROR","title":"Invalid"}]}`)
	}
	reg.Register(asctest.PATCH("/v1/territoryAvailabilities/"+id).JSONBody(`{"data":{"type":"territoryAvailabilities","id":"`+id+`","attributes":{"available":false}}}`), resp)
}

func countPatches(reg *asctest.Registry) int {
	patches := 0
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPatch {
			patches++
		}
	}
	return patches
}

const singlePlatformAppStoreVersions = `{"data":[{"type":"appStoreVersions","id":"ver-ios","attributes":{"platform":"IOS","versionString":"1.0","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-01-01T00:00:00Z"}}],"links":{}}`

const multiPlatformAppStoreVersions = `{"data":[{"type":"appStoreVersions","id":"ver-ios","attributes":{"platform":"IOS","versionString":"4.1.0","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-07-14T00:00:00Z"}},{"type":"appStoreVersions","id":"ver-vision","attributes":{"platform":"VISION_OS","versionString":"1.3.1","appStoreState":"PREORDER_READY_FOR_SALE","appVersionState":"PREORDER_READY_FOR_SALE","createdDate":"2024-07-06T00:00:00Z"}}],"links":{}}`

const unknownPlatformStateAppStoreVersions = `{"data":[{"type":"appStoreVersions","id":"ver-ios","attributes":{"platform":"IOS","versionString":"4.1.0","appStoreState":"FUTURE_SALE_STATE","createdDate":"2026-07-14T00:00:00Z"}}],"links":{}}`

func TestPricingAvailabilityRemoveFromSaleRequiresAllPlatformsWhenMultiplePlatformsLive(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, multiPlatformAppStoreVersions))

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"}, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if got := countPatches(reg); got != 0 {
		t.Fatalf("PATCH count = %d, want 0", got)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{"IOS 4.1.0", "VISION_OS 1.3.1", "--all-platforms", "App Store Connect support"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q, got %q", want, stderr)
		}
	}
}

func TestPricingAvailabilityRemoveFromSaleFailsClosedWhenPlatformsCannotBeVerified(t *testing.T) {
	tests := []struct {
		name     string
		response asctest.Responder
		wantErr  []string
	}{
		{
			name:     "platform lookup fails",
			response: asctest.JSON(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`),
			wantErr:  []string{"could not verify live platform listings", "--all-platforms", "Forbidden"},
		},
		{
			name:     "platform state is unknown",
			response: asctest.JSON(http.StatusOK, unknownPlatformStateAppStoreVersions),
			wantErr:  []string{"could not verify live platform listings", "FUTURE_SALE_STATE", "--all-platforms"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"), test.response)

			stdout, _, err := runRootCommand(t, []string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"})
			if err == nil {
				t.Fatal("expected platform verification failure")
			}
			for _, want := range test.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error missing %q: %v", want, err)
				}
			}
			if stdout != "" {
				t.Fatalf("expected no stdout before mutation, got %q", stdout)
			}
			if got := len(reg.Requests()); got != 1 {
				t.Fatalf("request count = %d, want only the platform lookup", got)
			}
		})
	}
}

func TestPricingAvailabilityRemoveFromSaleAllPlatformsAcknowledged(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerRemoveFromSaleReads(reg, multiPlatformAppStoreVersions, false, territoryAvailabilityBody(true, false))
	registerRemoveFromSalePatch(reg, "ta-usa", http.StatusOK)
	registerTerritoryAvailabilitiesRead(reg, territoryAvailabilityBody(false, false))

	stdout, _, err := runCommand(t, []string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm", "--all-platforms", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if got := countPatches(reg); got != 1 {
		t.Fatalf("PATCH count = %d, want 1", got)
	}
	var result struct {
		Status                    string `json:"status"`
		PlatformListingsVerified  bool   `json:"platformListingsVerified"`
		PlatformListingsVerifyErr string `json:"platformListingsVerificationError"`
		RemovedPlatformListings   []struct {
			Platform      string `json:"platform"`
			VersionString string `json:"versionString"`
			Live          bool   `json:"live"`
		} `json:"removedPlatformListings"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if result.Status != "removedFromSale" || !result.PlatformListingsVerified || result.PlatformListingsVerifyErr != "" {
		t.Fatalf("status = %q, want removedFromSale", result.Status)
	}
	if len(result.RemovedPlatformListings) != 2 {
		t.Fatalf("removedPlatformListings = %d, want 2", len(result.RemovedPlatformListings))
	}
	wantListings := []struct {
		platform string
		version  string
		live     bool
	}{
		{platform: "IOS", version: "4.1.0", live: true},
		{platform: "VISION_OS", version: "1.3.1", live: true},
	}
	for i, want := range wantListings {
		got := result.RemovedPlatformListings[i]
		if got.Platform != want.platform || got.VersionString != want.version || got.Live != want.live {
			t.Fatalf("unexpected removed listing at index %d: %+v", i, got)
		}
	}
}

func TestPricingAvailabilityRemoveFromSaleAllPlatformsAcknowledgedReportsUnverifiedListings(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerRemoveFromSaleReads(reg, unknownPlatformStateAppStoreVersions, false, territoryAvailabilityBody(true, false))
	registerRemoveFromSalePatch(reg, "ta-usa", http.StatusOK)
	registerTerritoryAvailabilitiesRead(reg, territoryAvailabilityBody(false, false))

	stdout, _, err := runCommand(t, []string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm", "--all-platforms", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	var result struct {
		Status                   string `json:"status"`
		PlatformListingsVerified bool   `json:"platformListingsVerified"`
		VerificationError        string `json:"platformListingsVerificationError"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if result.Status != "removedFromSale" || result.PlatformListingsVerified || !strings.Contains(result.VerificationError, "FUTURE_SALE_STATE") {
		t.Fatalf("unexpected verification receipt: %+v", result)
	}
}

func TestPricingAvailabilityPlatformsPreservesUnverifiableListingVisibility(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, unknownPlatformStateAppStoreVersions))

	stdout, _, err := runCommand(t, []string{"pricing", "availability", "platforms", "--app", "app-1", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	var result struct {
		Platforms []struct {
			Platform   string `json:"platform"`
			State      string `json:"state"`
			Live       bool   `json:"live"`
			StateKnown bool   `json:"stateKnown"`
		} `json:"platforms"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Platforms) != 1 || result.Platforms[0].Platform != "IOS" || result.Platforms[0].State != "FUTURE_SALE_STATE" || result.Platforms[0].Live || result.Platforms[0].StateKnown {
		t.Fatalf("unexpected unverifiable platform listing: %+v", result.Platforms)
	}
}

func TestPricingAvailabilityPlatformsPrefersLiveListingOverNewerNonLive(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-rejected","attributes":{"platform":"IOS","versionString":"2.2.1","appStoreState":"REJECTED","appVersionState":"REJECTED","createdDate":"2026-02-01T00:00:00Z"}},{"type":"appStoreVersions","id":"ver-live","attributes":{"platform":"IOS","versionString":"2.2.0","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2025-09-29T00:00:00Z"}},{"type":"appStoreVersions","id":"ver-mac","attributes":{"platform":"MAC_OS","versionString":"1.0","appStoreState":"PREPARE_FOR_SUBMISSION","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2025-01-01T00:00:00Z"}}],"links":{}}`))

	stdout, _, err := runCommand(t, []string{"pricing", "availability", "platforms", "--app", "app-1", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	var result struct {
		AppID     string `json:"appId"`
		Platforms []struct {
			Platform      string `json:"platform"`
			VersionString string `json:"versionString"`
			State         string `json:"state"`
			Live          bool   `json:"live"`
		} `json:"platforms"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Platforms) != 2 {
		t.Fatalf("platforms = %d, want 2", len(result.Platforms))
	}
	ios := result.Platforms[0]
	if ios.Platform != "IOS" || ios.VersionString != "2.2.0" || !ios.Live || ios.State != "READY_FOR_DISTRIBUTION" {
		t.Fatalf("unexpected iOS listing: %+v", ios)
	}
	mac := result.Platforms[1]
	if mac.Platform != "MAC_OS" || mac.VersionString != "1.0" || mac.Live {
		t.Fatalf("unexpected macOS listing: %+v", mac)
	}
}
