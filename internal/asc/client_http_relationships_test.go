package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestGetEndUserLicenseAgreementTerritories_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/endUserLicenseAgreements/eula-1/territories" {
			t.Fatalf("expected path /v1/endUserLicenseAgreements/eula-1/territories, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetEndUserLicenseAgreementTerritories(context.Background(), "eula-1", Limit[EndUserLicenseAgreementTerritoriesOption](5)); err != nil {
		t.Fatalf("GetEndUserLicenseAgreementTerritories() error: %v", err)
	}
}
