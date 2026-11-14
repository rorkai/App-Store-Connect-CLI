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
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppGameCenterEnabledVersions(
		context.Background(),
		"app-1",
		Limit[GCEnabledVersionsOption](40),
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

	if _, err := client.GetAppGameCenterEnabledVersions(context.Background(), "", NextURL[GCEnabledVersionsOption](next)); err != nil {
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

	if _, err := client.GetGameCenterEnabledVersionCompatibleVersions(context.Background(), "enabled-1", Limit[GCEnabledVersionsOption](12)); err != nil {
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

	if _, err := client.GetGameCenterEnabledVersionCompatibleVersions(context.Background(), "", NextURL[GCEnabledVersionsOption](next)); err != nil {
		t.Fatalf("GetGameCenterEnabledVersionCompatibleVersions() error: %v", err)
	}
}

func TestGCEnabledVersionsOptions(t *testing.T) {
	query := &gcEnabledVersionsQuery{}
	Limit[GCEnabledVersionsOption](9)(query)
	NextURL[GCEnabledVersionsOption]("next")(query)

	values, err := url.ParseQuery(buildGCEnabledVersionsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "9" {
		t.Fatalf("expected limit=9, got %q", values.Get("limit"))
	}
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
}
