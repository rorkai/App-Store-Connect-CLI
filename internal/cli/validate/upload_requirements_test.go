package validate

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func TestMapBuild_CarriesMinOSVersion(t *testing.T) {
	build := mapBuild(asc.Resource[asc.BuildAttributes]{ID: "build-1", Attributes: asc.BuildAttributes{MinOSVersion: " 14.0 "}})
	if build.MinOSVersion != "14.0" {
		t.Fatalf("MinOSVersion = %q", build.MinOSVersion)
	}
	if copied := copyBuild(build); copied.MinOSVersion != "14.0" {
		t.Fatalf("copyBuild dropped MinOSVersion: %+v", copied)
	}
}

func TestPopulateBuildBundles_ReadsSDKEvidence(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds/build-1" || req.URL.Query().Get("include") != "buildBundles" || req.URL.Query().Get("limit[buildBundles]") != "50" {
			return buildsJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
		return buildsJSONResponse(http.StatusOK, `{
			"data":{"type":"builds","id":"build-1","attributes":{}},
			"included":[
				{"type":"buildBundles","id":"bundle-1","attributes":{"bundleType":"APP","sdkBuild":"23A339"}},
				{"type":"buildBundles","id":"bundle-2","attributes":{"bundleType":"WATCH_APP","sdkBuild":"23R351"}}
			]}`)
	}))

	build := &validation.Build{ID: "build-1"}
	if err := populateBuildBundles(context.Background(), client, build); err != nil {
		t.Fatalf("populateBuildBundles() error = %v", err)
	}
	want := []validation.BuildBundle{{Type: "APP", SDKBuild: "23A339"}, {Type: "WATCH_APP", SDKBuild: "23R351"}}
	if !slices.Equal(build.Bundles, want) {
		t.Fatalf("Bundles = %+v, want %+v", build.Bundles, want)
	}
}

func TestPopulateBuildBundles_RecordsAReadFailure(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   string
	}{
		{name: "forbidden", status: http.StatusForbidden, want: "cannot read them"},
		{name: "bad request", status: http.StatusBadRequest, want: "could not be read"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newBuildsTestClient(t, buildsRoundTripFunc(func(*http.Request) (*http.Response, error) {
				return buildsJSONResponse(test.status, `{"errors":[{"title":"failed","detail":"failed"}]}`)
			}))
			build := &validation.Build{ID: "build-1"}
			if err := populateBuildBundles(context.Background(), client, build); err != nil {
				t.Fatalf("populateBuildBundles() error = %v", err)
			}
			if !strings.Contains(build.BundlesUnreadReason, test.want) {
				t.Fatalf("BundlesUnreadReason = %q, want %q", build.BundlesUnreadReason, test.want)
			}
		})
	}
}

func TestPopulateBuildBundles_PropagatesCancellation(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.Canceled
	}))
	if err := populateBuildBundles(context.Background(), client, &validation.Build{ID: "build-1"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}
