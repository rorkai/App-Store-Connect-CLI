package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTerritoryAgeRatingsRejectsForeignContinuationBeforeRead(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appInfos/info-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"info-1","relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
	reg.Register(asctest.GET("/v1/appInfos/info-1/territoryAgeRatings", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"territoryAgeRatings","id":"rating-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appInfos/info-foreign/territoryAgeRatings?cursor=next"}}`))

	stdout, _, runErr := runRootWithDeps(t, stubClient(t, reg), "apps", "info", "territory-age-ratings", "list", "--app", "100000001", "--info-id", "info-1", "--paginate", "--output", "json")
	if runErr == nil || stdout != "" || !strings.Contains(runErr.Error(), `does not match selected app info "info-1"`) {
		t.Fatalf("run error=%v stdout=%q, want foreign continuation refusal", runErr, stdout)
	}
	requests := reg.Requests()
	if len(requests) != 2 || requests[0].URL.Path != "/v1/appInfos/info-1" || requests[1].URL.Path != "/v1/appInfos/info-1/territoryAgeRatings" {
		t.Fatalf("requests=%v, want ownership and first-page reads only", requests)
	}
}
