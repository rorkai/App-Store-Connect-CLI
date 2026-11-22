package schema

import (
	"testing"
)

func TestMatchOperationNormalizesPathParameters(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		wantPath string
		wantOK   bool
	}{
		{name: "collection", method: "GET", path: "/v1/apps", wantPath: "/v1/apps", wantOK: true},
		{name: "resource id", method: "GET", path: "/v1/apps/6759231657", wantPath: "/v1/apps/{id}", wantOK: true},
		{name: "related resource", method: "GET", path: "/v1/apps/6759231657/builds", wantPath: "/v1/apps/{id}/builds", wantOK: true},
		{name: "linkage", method: "GET", path: "/v1/apps/6759231657/relationships/builds", wantPath: "/v1/apps/{id}/relationships/builds", wantOK: true},
		{name: "lowercase method", method: "get", path: "/v1/apps", wantPath: "/v1/apps", wantOK: true},
		{name: "trailing slash", method: "GET", path: "/v1/apps/", wantPath: "/v1/apps", wantOK: true},
		{name: "v2 path", method: "GET", path: "/v2/gameCenterAchievements/abc", wantPath: "/v2/gameCenterAchievements/{id}", wantOK: true},
		{name: "unknown segment", method: "GET", path: "/v1/apps/6759231657/buildz", wantOK: false},
		{name: "method not declared", method: "POST", path: "/v1/apps/6759231657/builds", wantOK: false},
		{name: "id where literal expected", method: "GET", path: "/v1/6759231657", wantOK: false},
		{name: "empty path", method: "GET", path: "", wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint, ok, err := MatchOperation(test.method, test.path)
			if err != nil {
				t.Fatalf("MatchOperation() error: %v", err)
			}
			if ok != test.wantOK {
				t.Fatalf("MatchOperation(%s %s) ok = %v, want %v", test.method, test.path, ok, test.wantOK)
			}
			if ok && endpoint.Path != test.wantPath {
				t.Fatalf("MatchOperation(%s %s) path = %q, want %q", test.method, test.path, endpoint.Path, test.wantPath)
			}
		})
	}
}

func TestNearestOperationsCorrectsVersionAndMisspelledSegments(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		want           string // empty: any first operation
		wantCorrection bool
	}{
		{name: "other API version", method: "GET", path: "/v1/appAvailabilities/123/territoryAvailabilities", want: "GET /v2/appAvailabilities/{id}/territoryAvailabilities", wantCorrection: true},
		{name: "expanded resource name", method: "GET", path: "/v1/appInformations/123/appInfoLocalizations", want: "GET /v1/appInfos/{id}/appInfoLocalizations", wantCorrection: true},
		{name: "reordered words", method: "GET", path: "/v1/betaBuildDetails", want: "GET /v1/buildBetaDetails", wantCorrection: true},
		{name: "plural of a to-one relationship", method: "GET", path: "/v1/appStoreVersions/123/builds", want: "GET /v1/appStoreVersions/{id}/build", wantCorrection: true},
		{name: "missing word", method: "GET", path: "/v1/apps/123/appVersions", want: "GET /v1/apps/{id}/appStoreVersions", wantCorrection: true},
		{name: "similar path under another method is no correction", method: "POST", path: "/v1/builds/123/betaAppReviewSubmissions", wantCorrection: false},
		{name: "typo", method: "GET", path: "/v1/apps/123/buildz", want: "GET /v1/apps/{id}/builds", wantCorrection: true},
		{name: "singular relationships segment", method: "GET", path: "/v1/apps/123/relationship/betaGroups", want: "GET /v1/apps/{id}/relationships/betaGroups", wantCorrection: true},
		{name: "same path under another method", method: "GET", path: "/v1/ageRatingDeclarations/123", want: "PATCH /v1/ageRatingDeclarations/{id}", wantCorrection: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nearest, err := NearestOperations(test.method, test.path, 5)
			if err != nil {
				t.Fatalf("NearestOperations() error: %v", err)
			}
			if len(nearest) == 0 {
				t.Fatalf("NearestOperations(%s %s) returned nothing, want %s first", test.method, test.path, test.want)
			}
			got := nearest[0].Method + " " + nearest[0].Path
			if (test.want != "" && got != test.want) || nearest[0].Correction != test.wantCorrection {
				t.Fatalf("NearestOperations(%s %s)[0] = %s (correction %v), want %s (correction %v)", test.method, test.path, got, nearest[0].Correction, test.want, test.wantCorrection)
			}
		})
	}
}

func TestNearestOperationsListsNothingForAnUnknownResource(t *testing.T) {
	for _, path := range []string{"/v1/agreements", "/v1/betaApps", "/v1/appDataUsages"} {
		nearest, err := NearestOperations("GET", path, 5)
		if err != nil {
			t.Fatalf("NearestOperations() error: %v", err)
		}
		if len(nearest) != 0 {
			t.Fatalf("NearestOperations(GET %s) = %v, want nothing: no indexed resource resembles it", path, nearest)
		}
	}
}

func TestNearestOperationsRanksSharedPrefixAndMethod(t *testing.T) {
	nearest, err := NearestOperations("GET", "/v1/apps/6759231657/buildz", 3)
	if err != nil {
		t.Fatalf("NearestOperations() error: %v", err)
	}
	if len(nearest) != 3 {
		t.Fatalf("NearestOperations() returned %d results, want 3", len(nearest))
	}
	if nearest[0].Method != "GET" || nearest[0].Path != "/v1/apps/{id}/builds" {
		t.Fatalf("NearestOperations()[0] = %s %s, want GET /v1/apps/{id}/builds", nearest[0].Method, nearest[0].Path)
	}
	for _, endpoint := range nearest {
		if endpoint.Method != "GET" {
			t.Fatalf("NearestOperations() returned %s %s before exhausting GET candidates", endpoint.Method, endpoint.Path)
		}
	}
}

func TestNearestOperationsPrefersMethodMatchesOnExactPath(t *testing.T) {
	nearest, err := NearestOperations("POST", "/v1/apps/6759231657/builds", 2)
	if err != nil {
		t.Fatalf("NearestOperations() error: %v", err)
	}
	if len(nearest) == 0 {
		t.Fatal("NearestOperations() returned no results")
	}
	if nearest[0].Path != "/v1/apps/{id}/builds" {
		t.Fatalf("NearestOperations()[0] = %s %s, want the same path under its declared method", nearest[0].Method, nearest[0].Path)
	}
}
