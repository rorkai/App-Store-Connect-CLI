package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAppInfoLocalizationsRejectsUnsafeInitialContinuation(t *testing.T) {
	tests := []struct {
		name string
		next string
	}{
		{
			name: "cross-origin",
			next: "https://example.com/v1/appInfos/info-1/appInfoLocalizations?cursor=AQ",
		},
		{
			name: "protocol-relative",
			next: "//api.appstoreconnect.apple.com/v1/appInfos/info-1/appInfoLocalizations?cursor=AQ",
		},
		{
			name: "userinfo",
			next: "https://user:pass@api.appstoreconnect.apple.com/v1/appInfos/info-1/appInfoLocalizations?cursor=AQ",
		},
		{
			name: "fragment",
			next: "https://api.appstoreconnect.apple.com/v1/appInfos/info-1/appInfoLocalizations?cursor=AQ#fragment",
		},
		{
			name: "different-collection",
			next: "https://api.appstoreconnect.apple.com/v1/appInfos/info-1/territoryAgeRatings?cursor=AQ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()

			stdout, stderr, runErr := runRootWithDeps(
				t,
				stubClient(t, reg),
				"localizations",
				"list",
				"--app",
				"100000001",
				"--type",
				"app-info",
				"--next",
				tt.next,
			)
			if rootcmd.ExitCodeFromError(runErr) != rootcmd.ExitUsage || stdout != "" || !strings.Contains(stderr, "--next") {
				t.Fatalf("run error=%v stdout=%q stderr=%q, want usage refusal", runErr, stdout, stderr)
			}
			if requests := reg.Requests(); len(requests) != 0 {
				t.Fatalf("requests=%v, want zero requests for unsafe continuation", requests)
			}
		})
	}
}

func TestAppInfoLocalizationsCanonicalizesSameParentContinuation(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appInfos/info-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"info-1","relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
	reg.Register(asctest.GET("/v1/appInfos/info-1/appInfoLocalizations", "cursor=AQ"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{}}`))

	nextURL := "HTTPS://api.appstoreconnect.apple.com/v1/appInfos/info-1/appInfoLocalizations?cursor=AQ"
	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "localizations", "list", "--app", "100000001", "--type", "app-info", "--next", nextURL, "--output", "json")
	if runErr != nil || stderr != "" || !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("run error=%v stdout=%q stderr=%q, want selected app-info page", runErr, stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 2 || requests[0].URL.Path != "/v1/appInfos/info-1" || requests[1].URL.Scheme != "https" {
		t.Fatalf("requests=%v, want ownership then canonical same-parent continuation", requests)
	}
}

func TestAppInfoLocalizationsRejectsUnsafePaginatedContinuation(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/100000001/appInfos"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/appInfos/info-1/appInfoLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{"next":"https://example.com/v1/appInfos/info-1/appInfoLocalizations?cursor=BQ"}}`))

	stdout, _, runErr := runRootWithDeps(t, stubClient(t, reg), "localizations", "list", "--app", "100000001", "--type", "app-info", "--paginate", "--output", "json")
	if runErr == nil || stdout != "" || !strings.Contains(runErr.Error(), "pagination URL") {
		t.Fatalf("run error=%v stdout=%q, want unsafe continuation refusal", runErr, stdout)
	}
	requests := reg.Requests()
	if len(requests) != 2 || requests[0].URL.Path != "/v1/apps/100000001/appInfos" || requests[1].URL.Path != "/v1/appInfos/info-1/appInfoLocalizations" {
		t.Fatalf("requests=%v, want ownership and first-page reads only", requests)
	}
}

func TestTerritoryAgeRatingsRejectsUnsafePaginatedContinuation(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appInfos/info-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"info-1","relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
	reg.Register(asctest.GET("/v1/appInfos/info-1/territoryAgeRatings", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"territoryAgeRatings","id":"rating-1"}],"links":{"next":"https://example.com/v1/appInfos/info-1/territoryAgeRatings?cursor=BQ"}}`))

	stdout, _, runErr := runRootWithDeps(t, stubClient(t, reg), "apps", "info", "territory-age-ratings", "list", "--app", "100000001", "--info-id", "info-1", "--paginate", "--output", "json")
	if runErr == nil || stdout != "" || !strings.Contains(runErr.Error(), "pagination URL") {
		t.Fatalf("run error=%v stdout=%q, want unsafe continuation refusal", runErr, stdout)
	}
	requests := reg.Requests()
	if len(requests) != 2 || requests[0].URL.Path != "/v1/appInfos/info-1" || requests[1].URL.Path != "/v1/appInfos/info-1/territoryAgeRatings" {
		t.Fatalf("requests=%v, want ownership and first-page reads only", requests)
	}
}

func TestTerritoryAgeRatingsDirectInfoContinuationRemainsScoped(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appInfos/info-1/territoryAgeRatings", "cursor=AQ"), asctest.JSON(http.StatusOK, `{"data":[{"type":"territoryAgeRatings","id":"rating-1"}],"links":{}}`))

	nextURL := "HTTPS://api.appstoreconnect.apple.com/v1/appInfos/info-1/territoryAgeRatings?cursor=AQ"
	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "apps", "info", "territory-age-ratings", "list", "--info-id", "info-1", "--next", nextURL, "--output", "json")
	if runErr != nil || stderr != "" || !strings.Contains(stdout, `"id":"rating-1"`) {
		t.Fatalf("run error=%v stdout=%q stderr=%q, want direct App Info continuation", runErr, stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 1 || requests[0].URL.Scheme != "https" || requests[0].URL.Path != "/v1/appInfos/info-1/territoryAgeRatings" {
		t.Fatalf("requests=%v, want one canonical direct continuation request", requests)
	}
}
