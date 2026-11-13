package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestGetBundleIDApp_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"name":"Demo","bundleId":"com.example.demo","sku":"SKU"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/bundleIds/bid-1/app" {
			t.Fatalf("expected path /v1/bundleIds/bid-1/app, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBundleIDApp(context.Background(), "bid-1"); err != nil {
		t.Fatalf("GetBundleIDApp() error: %v", err)
	}
}

func TestGetBundleIDProfiles_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/bundleIds/bid-1/profiles" {
			t.Fatalf("expected path /v1/bundleIds/bid-1/profiles, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "8" {
			t.Fatalf("expected limit=8, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBundleIDProfiles(context.Background(), "bid-1", WithBundleIDProfilesLimit(8)); err != nil {
		t.Fatalf("GetBundleIDProfiles() error: %v", err)
	}
}

func TestGetBundleIDProfiles_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/bundleIds/bid-1/profiles?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBundleIDProfiles(context.Background(), "bid-1", WithBundleIDProfilesNextURL(next)); err != nil {
		t.Fatalf("GetBundleIDProfiles() error: %v", err)
	}
}

func TestGetUserInvitationVisibleApps_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/userInvitations/invite-1/visibleApps" {
			t.Fatalf("expected path /v1/userInvitations/invite-1/visibleApps, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "12" {
			t.Fatalf("expected limit=12, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetUserInvitationVisibleApps(context.Background(), "invite-1", WithUserInvitationVisibleAppsLimit(12)); err != nil {
		t.Fatalf("GetUserInvitationVisibleApps() error: %v", err)
	}
}

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

	if _, err := client.GetEndUserLicenseAgreementTerritories(context.Background(), "eula-1", WithEndUserLicenseAgreementTerritoriesLimit(5)); err != nil {
		t.Fatalf("GetEndUserLicenseAgreementTerritories() error: %v", err)
	}
}

func TestGetAppCiProduct_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciProducts","id":"prod-1","attributes":{"name":"Demo"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/ciProduct" {
			t.Fatalf("expected path /v1/apps/app-1/ciProduct, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppCiProduct(context.Background(), "app-1"); err != nil {
		t.Fatalf("GetAppCiProduct() error: %v", err)
	}
}
