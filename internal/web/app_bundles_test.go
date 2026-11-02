package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestListAppBundlesJoinsReviewStateAcrossPages(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/appBundles" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("include"); got != "appBundleVersions" {
			t.Errorf("include = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "" {
			_, _ = w.Write([]byte(`{
				"data": [{"type": "appBundles", "id": "1501593771",
					"attributes": {"name": "Colorway", "platforms": ["IOS", "MAC_OS"]},
					"relationships": {"appBundleVersions": {"data": [{"type": "appBundleVersions", "id": "v-1"}]}}}],
				"included": [{"type": "appBundleVersions", "id": "v-1", "attributes": {"bundleReviewState": "DEVELOPER_REMOVED_FROM_SALE"}}],
				"links": {"next": "` + server.URL + `/appBundles?include=appBundleVersions&limit=300&cursor=abc"}
			}`))
			return
		}
		_, _ = w.Write([]byte(`{"data": [{"type": "appBundles", "id": "2", "attributes": {"name": "No Versions"}}]}`))
	}))
	defer server.Close()

	got, err := testWebClient(server).ListAppBundles(context.Background())
	if err != nil {
		t.Fatalf("ListAppBundles() error = %v", err)
	}
	want := []AppBundle{
		{
			ID: "1501593771", Name: "Colorway", Platforms: []string{"IOS", "MAC_OS"}, Status: "DEVELOPER_REMOVED_FROM_SALE",
			Versions: []AppBundleVersion{{ID: "v-1", ReviewState: "DEVELOPER_REMOVED_FROM_SALE"}},
		},
		{ID: "2", Name: "No Versions"},
	}
	if !reflect.DeepEqual(got.Data, want) {
		t.Fatalf("bundles = %+v, want %+v", got.Data, want)
	}
}

func TestGetAppBundleKeepsMemberOrderAndRenamesReviewFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/WebObjects/iTunesConnect.woa/ra/appbundles/metadetail/1501593771" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {
			"state": "Developer Removed from Sale", "stateKey": "developerRemovedFromSale", "inFlightState": null,
			"primaryLocaleCode": "en-GB", "priceTierStem": "530",
			"canSubmit": false, "canDevReject": true, "clearedForSale": false, "clearedForSaleEditable": true,
			"memberApps": [
				{"adamId": 1500196580, "name": "Second Listed First", "appType": "ios", "priceTier": "Free"},
				{"adamId": 1479784361, "name": "Gradient Game", "appType": "ios", "priceTier": "Free"}
			]
		}, "messages": {"warn": null, "error": null, "info": null}, "statusCode": "SUCCESS"}`))
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppBundle(context.Background(), " 1501593771 ")
	if err != nil {
		t.Fatalf("GetAppBundle() error = %v", err)
	}
	if got.ID != "1501593771" || got.StateKey != "developerRemovedFromSale" || got.PriceTier != "530" || !got.CanRemoveFromReview || !got.ClearedForSaleEditable {
		t.Fatalf("unexpected detail: %+v", got)
	}
	if len(got.Apps) != 2 || got.Apps[0].ID != "1500196580" || got.Apps[1].ID != "1479784361" {
		t.Fatalf("apps must keep Apple's order: %+v", got.Apps)
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fields["canRemoveFromReview"] != true || fields["clearedForSale"] != false {
		t.Fatalf("false booleans must stay in the JSON contract: %s", encoded)
	}
}

func TestGetAppBundleReportsLegacyFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"error on HTTP 200", http.StatusOK, `{"data": null, "messages": {"error": ["Bundle unavailable"]}, "statusCode": "ERROR"}`, "Bundle unavailable"},
		{"status without message", http.StatusOK, `{"data": null, "messages": {"error": null}, "statusCode": "ERROR"}`, `"ERROR"`},
		{"success without data", http.StatusOK, `{"data": null, "messages": {"error": null}, "statusCode": "SUCCESS"}`, "missing data"},
		{"unknown bundle", http.StatusNotFound, `{"data": null, "messages": {"error": ["Not Found"]}, "statusCode": "ERROR"}`, "404"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			_, err := testWebClient(server).GetAppBundle(context.Background(), "1")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
