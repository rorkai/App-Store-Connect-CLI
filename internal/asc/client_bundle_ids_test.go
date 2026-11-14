package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestGetBundleIDs_WithQuerySurface(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"name":"Example","identifier":"com.example.app"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/bundleIds" {
			t.Fatalf("expected GET /v1/bundleIds, got %s %s", req.Method, req.URL.Path)
		}
		values := req.URL.Query()
		want := map[string]string{
			"filter[name]":                 "Example,Other",
			"filter[platform]":             "IOS,UNIVERSAL",
			"filter[identifier]":           "com.example.app,com.example.other",
			"filter[seedId]":               "seed-1,seed-2",
			"filter[id]":                   "bundle-1,bundle-2",
			"sort":                         "-identifier",
			"fields[bundleIds]":            "name,identifier,app",
			"fields[profiles]":             "name,expirationDate",
			"fields[bundleIdCapabilities]": "capabilityType,settings",
			"fields[apps]":                 "name,bundleId",
			"include":                      "profiles,bundleIdCapabilities,app",
			"limit[profiles]":              "7",
			"limit[bundleIdCapabilities]":  "8",
			"limit":                        "25",
		}
		for key, expected := range want {
			if got := values.Get(key); got != expected {
				t.Errorf("%s = %q, want %q", key, got, expected)
			}
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetBundleIDs(
		context.Background(),
		WithBundleIDsFilterNames([]string{" Example ", "Other"}),
		WithBundleIDsFilterPlatforms([]string{"ios", "universal"}),
		WithBundleIDsFilterIdentifier("com.example.app, com.example.other"),
		WithBundleIDsFilterSeedIDs([]string{"seed-1", " seed-2 "}),
		WithBundleIDsFilterIDs([]string{"bundle-1", " bundle-2 "}),
		WithBundleIDsSort("-identifier"),
		WithBundleIDsFields([]string{"name", "identifier", "app"}),
		WithBundleIDsProfilesFields([]string{"name", "expirationDate"}),
		WithBundleIDsCapabilitiesFields([]string{"capabilityType", "settings"}),
		WithBundleIDsAppFields([]string{"name", "bundleId"}),
		WithBundleIDsInclude([]string{"profiles", "bundleIdCapabilities", "app"}),
		WithBundleIDsProfilesLimit(7),
		WithBundleIDsCapabilitiesLimit(8),
		Limit[BundleIDsOption](25),
	)
	if err != nil {
		t.Fatalf("GetBundleIDs() error: %v", err)
	}
}
