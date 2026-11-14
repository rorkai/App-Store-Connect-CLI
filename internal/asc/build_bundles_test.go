package asc

import (
	"encoding/json"
	"net/url"
	"testing"
)

func TestBuildBuildBundlesQuery(t *testing.T) {
	query := &buildBundlesQuery{}
	WithBuildBundlesLimit(25)(query)

	values, err := url.ParseQuery(buildBuildBundlesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("include"); got != "buildBundles" {
		t.Fatalf("expected include=buildBundles, got %q", got)
	}
	if got := values.Get("limit[buildBundles]"); got != "25" {
		t.Fatalf("expected limit[buildBundles]=25, got %q", got)
	}
}

func TestBuildBuildBundleFileSizesQuery(t *testing.T) {
	query := &buildBundleFileSizesQuery{}
	Limit[BuildBundleFileSizesOption](100)(query)

	values, err := url.ParseQuery(buildBuildBundleFileSizesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "100" {
		t.Fatalf("expected limit=100, got %q", got)
	}
}

func TestBuildBetaAppClipInvocationsQuery(t *testing.T) {
	query := &betaAppClipInvocationsQuery{}
	Limit[BetaAppClipInvocationsOption](50)(query)

	values, err := url.ParseQuery(buildBetaAppClipInvocationsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("limit"); got != "50" {
		t.Fatalf("expected limit=50, got %q", got)
	}
}

func TestExtractBuildBundles(t *testing.T) {
	included := json.RawMessage(`[
		{
			"type": "buildBundles",
			"id": "bundle-1",
			"attributes": {
				"bundleId": "com.example.app",
				"bundleType": "APP",
				"minimumOsVersion": "18.0"
			}
		},
		{
			"type": "apps",
			"id": "app-1"
		}
	]`)

	bundles, err := extractBuildBundles(included)
	if err != nil {
		t.Fatalf("extractBuildBundles() error: %v", err)
	}
	if len(bundles) != 1 {
		t.Fatalf("expected 1 build bundle, got %d", len(bundles))
	}
	if bundles[0].ID != "bundle-1" {
		t.Fatalf("expected build bundle ID bundle-1, got %q", bundles[0].ID)
	}
	if bundles[0].Attributes.BundleID == nil || *bundles[0].Attributes.BundleID != "com.example.app" {
		t.Fatalf("expected bundleId com.example.app, got %v", bundles[0].Attributes.BundleID)
	}
	if bundles[0].Attributes.BundleType == nil || *bundles[0].Attributes.BundleType != BuildBundleTypeApp {
		t.Fatalf("expected bundleType APP, got %v", bundles[0].Attributes.BundleType)
	}
	if bundles[0].Attributes.MinimumOSVersion == nil || *bundles[0].Attributes.MinimumOSVersion != "18.0" {
		t.Fatalf("expected minimumOsVersion 18.0, got %v", bundles[0].Attributes.MinimumOSVersion)
	}
}
