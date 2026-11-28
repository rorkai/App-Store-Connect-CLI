package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestListDeveloperBundleIDCapabilitiesPreservesCatalogAndCapturedRequest(t *testing.T) {
	const envelope = `{"data":[{"type":"capabilities","id":"CARPLAY_PLAYABLE_CONTENT","attributes":{"name":"CarPlay Audio App","editable":true,"newMetadata":{"value":1}}}],"included":[],"meta":{"future":true,"paging":{"total":0,"limit":0}},"futureTopLevel":[]}`
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case developerPortalTeamsPath:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case developerServicesPath + "/capabilities":
			if r.Method != http.MethodPost || r.Header.Get("X-HTTP-Method-Override") != http.MethodGet {
				t.Fatalf("unexpected method %s override %q", r.Method, r.Header.Get("X-HTTP-Method-Override"))
			}
			if !strings.HasSuffix(r.Header.Get("Referer"), "/bundleId/edit/bundle-1") {
				t.Fatalf("incorrect bundle context %q", r.Header.Get("Referer"))
			}
			var request developerPortalProxyReadRequest
			if err := json.Unmarshal(mustReadBody(t, r), &request); err != nil {
				t.Fatal(err)
			}
			query, err := url.ParseQuery(request.URLEncodedQueryParams)
			if err != nil {
				t.Fatal(err)
			}
			if request.TeamID != "TEAM123456" || query.Get("filter[bundleId]") != "bundle-1" || query.Get("filter[platform]") != "IOS,MACOS" || r.URL.Query().Get("filter[capabilityType]") != "capability,service" || r.URL.Query().Get("filter[includeRequestable]") != "true" {
				t.Fatalf("unexpected catalog request %+v query %v", request, query)
			}
			return developerPortalTestResponse(http.StatusOK, envelope, nil), nil
		default:
			t.Fatalf("unexpected request %s", r.URL)
			return nil, nil
		}
	})
	result, err := client.ListDeveloperBundleIDCapabilities(context.Background(), " bundle-1 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "CARPLAY_PLAYABLE_CONTENT" || result.Data[0].Attributes.Editable == nil || !*result.Data[0].Attributes.Editable {
		t.Fatalf("unexpected catalog %+v", result.Data)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != envelope {
		t.Fatalf("Apple envelope changed: %s", encoded)
	}
}

func TestListDeveloperBundleIDCapabilitiesRejectsMalformedOrIncompleteCatalog(t *testing.T) {
	for name, body := range map[string]string{
		"missing data":      `{"meta":{}}`,
		"null data":         `{"data":null}`,
		"object data":       `{"data":{}}`,
		"continued catalog": `{"data":[],"links":{"next":"/capabilities?cursor=two"}}`,
		"partial count":     `{"data":[],"meta":{"paging":{"total":1}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == developerPortalTeamsPath {
					return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
				}
				return developerPortalTestResponse(http.StatusOK, body, nil), nil
			})
			result, err := client.ListDeveloperBundleIDCapabilities(context.Background(), "bundle-1")
			if err == nil || result != nil {
				t.Fatalf("expected rejected catalog, got result=%+v err=%v", result, err)
			}
		})
	}
}
