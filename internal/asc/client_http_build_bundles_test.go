package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestGetBuildBundleAppClipDomainCacheStatus(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClipDomainStatuses","id":"status-1","attributes":{"domains":[]}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildBundles/bundle-1/appClipDomainCacheStatus" {
			t.Fatalf("expected path /v1/buildBundles/bundle-1/appClipDomainCacheStatus, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBuildBundleAppClipDomainCacheStatus(context.Background(), "bundle-1"); err != nil {
		t.Fatalf("GetBuildBundleAppClipDomainCacheStatus() error: %v", err)
	}
}

func TestGetBuildBundleAppClipDomainDebugStatus(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appClipDomainStatuses","id":"status-2","attributes":{"domains":[]}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildBundles/bundle-1/appClipDomainDebugStatus" {
			t.Fatalf("expected path /v1/buildBundles/bundle-1/appClipDomainDebugStatus, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBuildBundleAppClipDomainDebugStatus(context.Background(), "bundle-1"); err != nil {
		t.Fatalf("GetBuildBundleAppClipDomainDebugStatus() error: %v", err)
	}
}
