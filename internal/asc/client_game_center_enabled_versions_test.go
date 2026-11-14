package asc

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestGetAppGameCenterEnabledVersions_WithLimitAndFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/gameCenterEnabledVersions" {
			t.Fatalf("expected path /v1/apps/app-1/gameCenterEnabledVersions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "40" {
			t.Fatalf("expected limit=40, got %q", values.Get("limit"))
		}
		if values.Get("filter[platform]") != "IOS" {
			t.Fatalf("expected filter[platform]=IOS, got %q", values.Get("filter[platform]"))
		}
		if values.Get("filter[versionString]") != "1.0" {
			t.Fatalf("expected filter[versionString]=1.0, got %q", values.Get("filter[versionString]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppGameCenterEnabledVersions(
		context.Background(),
		"app-1",
		WithGCEnabledVersionsLimit(40),
		WithGCEnabledVersionsPlatforms([]string{"IOS"}),
		WithGCEnabledVersionsVersionStrings([]string{"1.0"}),
	); err != nil {
		t.Fatalf("GetAppGameCenterEnabledVersions() error: %v", err)
	}
}

func TestGetAppGameCenterEnabledVersions_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps/app-1/gameCenterEnabledVersions?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppGameCenterEnabledVersions(context.Background(), "", WithGCEnabledVersionsNextURL(next)); err != nil {
		t.Fatalf("GetAppGameCenterEnabledVersions() error: %v", err)
	}
}

func TestGetGameCenterEnabledVersionCompatibleVersions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterEnabledVersions/enabled-1/compatibleVersions" {
			t.Fatalf("expected path /v1/gameCenterEnabledVersions/enabled-1/compatibleVersions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "12" {
			t.Fatalf("expected limit=12, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterEnabledVersionCompatibleVersions(context.Background(), "enabled-1", WithGCEnabledVersionsLimit(12)); err != nil {
		t.Fatalf("GetGameCenterEnabledVersionCompatibleVersions() error: %v", err)
	}
}

func TestGetGameCenterEnabledVersionCompatibleVersions_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterEnabledVersions/enabled-1/compatibleVersions?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterEnabledVersionCompatibleVersions(context.Background(), "", WithGCEnabledVersionsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterEnabledVersionCompatibleVersions() error: %v", err)
	}
}

func TestGCEnabledVersionsOptions(t *testing.T) {
	query := &gcEnabledVersionsQuery{}
	WithGCEnabledVersionsLimit(9)(query)
	WithGCEnabledVersionsNextURL("next")(query)
	WithGCEnabledVersionsPlatforms([]string{" IOS ", "", "MAC_OS"})(query)
	WithGCEnabledVersionsVersionStrings([]string{"1.0", "  ", "1.1"})(query)
	WithGCEnabledVersionsIDs([]string{"id-1", "id-2"})(query)
	WithGCEnabledVersionsSort([]string{"versionString"})(query)

	values, err := url.ParseQuery(buildGCEnabledVersionsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "9" {
		t.Fatalf("expected limit=9, got %q", values.Get("limit"))
	}
	if values.Get("filter[platform]") != "IOS,MAC_OS" {
		t.Fatalf("expected filter[platform]=IOS,MAC_OS, got %q", values.Get("filter[platform]"))
	}
	if values.Get("filter[versionString]") != "1.0,1.1" {
		t.Fatalf("expected filter[versionString]=1.0,1.1, got %q", values.Get("filter[versionString]"))
	}
	if values.Get("filter[id]") != "id-1,id-2" {
		t.Fatalf("expected filter[id]=id-1,id-2, got %q", values.Get("filter[id]"))
	}
	if values.Get("sort") != "versionString" {
		t.Fatalf("expected sort=versionString, got %q", values.Get("sort"))
	}
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
}
