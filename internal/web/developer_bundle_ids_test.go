package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEnsureDeveloperPortalSessionRejectsHTTPSDowngradeRedirect(t *testing.T) {
	var targetCalled atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		targetCalled.Store(true)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(developerPortalTeamsFixture()))
	}))
	t.Cleanup(target.Close)

	portal := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	t.Cleanup(portal.Close)

	client := &Client{httpClient: portal.Client(), developerPortalURL: portal.URL}
	err := client.ensureDeveloperPortalSession(context.Background())
	if err == nil || !strings.Contains(err.Error(), "authentication redirected to") {
		t.Fatalf("ensureDeveloperPortalSession() error = %v, want redirect rejection", err)
	}
	if targetCalled.Load() {
		t.Fatal("authenticated request followed HTTPS-to-HTTP redirect")
	}
}

func TestEnsureDeveloperPortalSessionRejectsDifferentPortRedirect(t *testing.T) {
	var targetCalled atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		targetCalled.Store(true)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(developerPortalTeamsFixture()))
	}))
	t.Cleanup(target.Close)

	portal := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	t.Cleanup(portal.Close)

	client := &Client{httpClient: portal.Client(), developerPortalURL: portal.URL}
	err := client.ensureDeveloperPortalSession(context.Background())
	if err == nil || !strings.Contains(err.Error(), "authentication redirected to") {
		t.Fatalf("ensureDeveloperPortalSession() error = %v, want redirect rejection", err)
	}
	if targetCalled.Load() {
		t.Fatal("authenticated request followed redirect to a different port")
	}
}

func TestEnableDeveloperBundleIDCapabilityPreservesWritablePayloadAndGraph(t *testing.T) {
	var patchBody []byte
	var err error
	requestCount := 0
	handler := func(r *http.Request) *http.Response {
		requestCount++
		for _, header := range []string{"Accept", "Content-Type", "Referer", "User-Agent", "X-Requested-With"} {
			if strings.TrimSpace(r.Header.Get(header)) == "" {
				t.Fatalf("request %d missing %s header", requestCount, header)
			}
		}

		switch requestCount {
		case 1:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/QH65B2/account/listTeams.action" {
				t.Fatalf("unexpected bootstrap request %s %s", r.Method, r.URL.String())
			}
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": []string{"bootstrap-csrf"}, "csrf_ts": []string{"bootstrap-ts"}})
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/v1/capabilities" {
				t.Fatalf("unexpected metadata request %s %s", r.Method, r.URL.String())
			}
			if got := r.Header.Get("X-HTTP-Method-Override"); got != http.MethodGet {
				t.Fatalf("metadata method override = %q", got)
			}
			proxy := decodeDeveloperPortalProxyReadRequest(t, r)
			if proxy.TeamID != "TEAM123456" {
				t.Fatalf("metadata teamId = %q", proxy.TeamID)
			}
			query, err := url.ParseQuery(proxy.URLEncodedQueryParams)
			if err != nil {
				t.Fatalf("metadata query: %v", err)
			}
			if got := query.Get("filter[capabilityType]"); got != "capability,service" {
				t.Fatalf("filter[capabilityType] = %q", got)
			}
			if got := query.Get("filter[includeRequestable]"); got != "true" {
				t.Fatalf("filter[includeRequestable] = %q", got)
			}
			if r.Header.Get("csrf") != "bootstrap-csrf" || r.Header.Get("csrf_ts") != "bootstrap-ts" {
				t.Fatalf("metadata request missing bootstrap CSRF headers")
			}
			return developerPortalTestResponse(http.StatusOK, `{
						"data":[{
							"type":"capabilities",
							"id":"PRIVATE_CLOUD_COMPUTE",
							"attributes":{
								"name":"Access to models on Private Cloud Compute",
								"entitlement":"com.apple.developer.private-cloud-compute",
								"isPublic":false,
								"editable":true,
								"canRequestFromPortal":false
							}
						}]
					}`, http.Header{"csrf": []string{"secret-csrf"}, "csrf_ts": []string{"secret-ts"}})
		case 3:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/v1/bundleIds/bundle-1" {
				t.Fatalf("unexpected bundle request %s %s", r.Method, r.URL.String())
			}
			if got := r.Header.Get("X-HTTP-Method-Override"); got != http.MethodGet {
				t.Fatalf("bundle method override = %q", got)
			}
			proxy := decodeDeveloperPortalProxyReadRequest(t, r)
			query, err := url.ParseQuery(proxy.URLEncodedQueryParams)
			if err != nil {
				t.Fatalf("bundle query: %v", err)
			}
			if got := query.Get("fields[bundleIds]"); got != "name,identifier,platform,seedId,wildcard,bundleIdCapabilities,~permissions.delete,~permissions.edit" {
				t.Fatalf("fields[bundleIds] = %q", got)
			}
			if got := query.Get("limit[bundleIdCapabilities]"); got != "50" {
				t.Fatalf("limit[bundleIdCapabilities] = %q, want 50", got)
			}
			include := query.Get("include")
			for _, relationship := range []string{
				"bundleIdCapabilities",
				"bundleIdCapabilities.capability",
				"bundleIdCapabilities.appGroups",
				"bundleIdCapabilities.cloudContainers",
			} {
				if !strings.Contains(include, relationship) {
					t.Fatalf("include missing %q: %q", relationship, include)
				}
			}
			return developerPortalTestResponse(http.StatusOK, `{
						"data":{
							"id":"bundle-1",
							"type":"bundleIds",
							"attributes":{
								"name":"Example",
								"identifier":"com.example.app",
								"platform":"IOS",
								"seedId":"TEAMID",
								"wildcard":false,
								"~permissions.delete":true,
								"~permissions.edit":true
							},
							"relationships":{
								"bundleIdCapabilities":{
									"data":[
										{"type":"bundleIdCapabilities","id":"icloud-1"},
										{"type":"bundleIdCapabilities","id":"icloud-1"}
									]
								}
							}
						},
						"included":[{
							"type":"bundleIdCapabilities",
							"id":"icloud-1",
							"attributes":{"enabled":true,"settings":[{"key":"ICLOUD_VERSION","options":[{"key":"XCODE_6","enabled":true}]}],"ownerType":"BUNDLE","editable":true,"inputs":[],"responseId":"response-1"},
							"relationships":{
								"capability":{"data":{"type":"capabilities","id":"ICLOUD"}},
								"appGroups":{"data":[{"type":"appGroups","id":"group-1"}]},
								"cloudContainers":{"data":[{"type":"cloudContainers","id":"cloud-1"}]}
							}
						}]
					}`, nil)
		case 4:
			if r.Method != http.MethodPatch || r.URL.Path != "/services-account/v1/bundleIds/bundle-1" {
				t.Fatalf("unexpected patch request %s %s", r.Method, r.URL.String())
			}
			if r.Header.Get("csrf") != "secret-csrf" || r.Header.Get("csrf_ts") != "secret-ts" {
				t.Fatalf("missing CSRF headers")
			}
			patchBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			return developerPortalTestResponse(http.StatusOK, `{"data":{"type":"bundleIds","id":"bundle-1"}}`, nil)
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		response := handler(request)
		if response == nil {
			t.Error("test Developer Portal handler returned a nil response")
			http.Error(writer, "missing test response", http.StatusInternalServerError)
			return
		}
		for name, values := range response.Header {
			for _, value := range values {
				writer.Header().Add(name, value)
			}
		}
		writer.WriteHeader(response.StatusCode)
		if response.Body != nil {
			defer func() { _ = response.Body.Close() }()
			if _, err := io.Copy(writer, response.Body); err != nil {
				t.Errorf("write test Developer Portal response: %v", err)
			}
		}
	}))
	t.Cleanup(server.Close)
	client := &Client{
		httpClient:         server.Client(),
		developerPortalURL: server.URL,
	}

	result, err := client.EnableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityEnableRequest{
		BundleID:   "bundle-1",
		Capability: "private_cloud_compute",
	})
	if err != nil {
		t.Fatalf("EnableDeveloperBundleIDCapability() error: %v", err)
	}
	if !result.Enabled || !result.Changed || result.Status != "enabled" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if requestCount != 4 {
		t.Fatalf("request count = %d, want 4", requestCount)
	}

	var payload struct {
		Data struct {
			ID            string                     `json:"id"`
			Type          string                     `json:"type"`
			Attributes    map[string]any             `json:"attributes"`
			Relationships map[string]json.RawMessage `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(patchBody, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; body=%s", err, patchBody)
	}
	if payload.Data.ID != "bundle-1" || payload.Data.Type != "bundleIds" {
		t.Fatalf("unexpected resource identity: %+v", payload.Data)
	}
	if payload.Data.Attributes["platform"] != "IOS" || payload.Data.Attributes["seedId"] != "TEAMID" {
		t.Fatalf("writable Bundle ID attributes were not preserved: %+v", payload.Data.Attributes)
	}
	for _, key := range []string{"permissions", "~permissions.delete", "~permissions.edit"} {
		if _, ok := payload.Data.Attributes[key]; ok {
			t.Fatalf("read-only Bundle ID attribute %q was sent in PATCH: %+v", key, payload.Data.Attributes)
		}
	}
	if payload.Data.Attributes["teamId"] != "TEAM123456" {
		t.Fatalf("Developer Portal teamId = %v", payload.Data.Attributes["teamId"])
	}

	var capabilityRelationship struct {
		Data []struct {
			ID            string                     `json:"id"`
			Type          string                     `json:"type"`
			Attributes    map[string]any             `json:"attributes"`
			Relationships map[string]json.RawMessage `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload.Data.Relationships["bundleIdCapabilities"], &capabilityRelationship); err != nil {
		t.Fatalf("decode bundleIdCapabilities: %v", err)
	}
	if len(capabilityRelationship.Data) != 2 {
		t.Fatalf("capability count = %d, want preserved ICLOUD plus PCC: %+v", len(capabilityRelationship.Data), capabilityRelationship.Data)
	}
	existing := capabilityRelationship.Data[0]
	if existing.ID != "icloud-1" || existing.Attributes["enabled"] != true {
		t.Fatalf("existing capability changed: %+v", existing)
	}
	if len(existing.Attributes) != 2 {
		t.Fatalf("PATCH contained read-only capability attributes: %+v", existing.Attributes)
	}
	settings, ok := existing.Attributes["settings"].([]any)
	if !ok {
		t.Fatalf("PATCH omitted or changed existing capability settings: %+v", existing.Attributes)
	}
	encodedSettings, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("encode existing capability settings: %v", err)
	}
	const wantSettings = `[{"key":"ICLOUD_VERSION","options":[{"enabled":true,"key":"XCODE_6"}]}]`
	if string(encodedSettings) != wantSettings {
		t.Fatalf("PATCH changed existing capability settings: got %s, want %s", encodedSettings, wantSettings)
	}
	if _, ok := existing.Relationships["appGroups"]; !ok {
		t.Fatalf("existing appGroups relationship was not preserved: %+v", existing.Relationships)
	}
	if _, ok := existing.Relationships["cloudContainers"]; !ok {
		t.Fatalf("existing cloudContainers relationship was not preserved: %+v", existing.Relationships)
	}
	added := capabilityRelationship.Data[1]
	if added.Type != "bundleIdCapabilities" || added.Attributes["enabled"] != true {
		t.Fatalf("unexpected PCC relationship: %+v", added)
	}
	var capability struct {
		Data relationshipData `json:"data"`
	}
	if err := json.Unmarshal(added.Relationships["capability"], &capability); err != nil {
		t.Fatalf("decode capability relationship: %v", err)
	}
	if capability.Data != (relationshipData{Type: "capabilities", ID: "PRIVATE_CLOUD_COMPUTE"}) {
		t.Fatalf("unexpected capability data: %+v", capability.Data)
	}
}

func TestDisableDeveloperBundleIDCapabilityDisablesPCCAndVerifiesGraph(t *testing.T) {
	requestCount := 0
	var patchBody []byte
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": []string{"bootstrap-csrf"}, "csrf_ts": []string{"bootstrap-ts"}}), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"secret-csrf"}, "csrf_ts": []string{"secret-ts"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, `{
				"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app","platform":"IOS","seedId":"TEAMID"},"relationships":{"bundleIdCapabilities":{"data":[
					{"type":"bundleIdCapabilities","id":"pcc-1"},
					{"type":"bundleIdCapabilities","id":"icloud-1"}
				]}}},
				"included":[
					{"type":"bundleIdCapabilities","id":"pcc-1","attributes":{"enabled":true,"settings":[{"key":"PCC_SETTING"}],"portalOwned":"drop"},"relationships":{"capability":{"data":{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE"}},"associatedBundleIds":{"data":[{"type":"bundleIds","id":"related-1"}]}}},
					{"type":"bundleIdCapabilities","id":"icloud-1","attributes":{"enabled":true,"settings":[{"key":"ICLOUD_VERSION"}]},"relationships":{"capability":{"data":{"type":"capabilities","id":"ICLOUD"}},"cloudContainers":{"data":[{"type":"cloudContainers","id":"cloud-1"}]}}}
				]}`, nil), nil
		case 4:
			if r.Method != http.MethodPatch || r.URL.Path != "/services-account/v1/bundleIds/bundle-1" {
				t.Fatalf("unexpected write request %s %s", r.Method, r.URL.String())
			}
			var err error
			patchBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read patch: %v", err)
			}
			return developerPortalTestResponse(http.StatusOK, ``, nil), nil
		case 5:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithICloudOnly(), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "private_cloud_compute",
	})
	if err != nil {
		t.Fatalf("DisableDeveloperBundleIDCapability() error: %v", err)
	}
	if result == nil || !result.Changed || result.Enabled || result.Status != "disabled" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if requestCount != 5 {
		t.Fatalf("request count = %d, want 5", requestCount)
	}

	var payload struct {
		Data struct {
			Attributes    map[string]any `json:"attributes"`
			Relationships struct {
				Capabilities developerResourceRelationship `json:"bundleIdCapabilities"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(patchBody, &payload); err != nil {
		t.Fatalf("decode patch: %v; body=%s", err, patchBody)
	}
	if payload.Data.Attributes["teamId"] != "TEAM123456" {
		t.Fatalf("teamId = %v", payload.Data.Attributes["teamId"])
	}
	if len(payload.Data.Relationships.Capabilities.Data) != 2 {
		t.Fatalf("capability count = %d, want 2", len(payload.Data.Relationships.Capabilities.Data))
	}
	pcc := payload.Data.Relationships.Capabilities.Data[0]
	var attrs map[string]any
	if err := json.Unmarshal(pcc.Attributes, &attrs); err != nil {
		t.Fatalf("decode PCC attributes: %v", err)
	}
	if attrs["enabled"] != false || attrs["settings"].([]any)[0].(map[string]any)["key"] != "PCC_SETTING" {
		t.Fatalf("PCC payload was not disabled while retaining settings: %+v", attrs)
	}
	if _, ok := attrs["portalOwned"]; ok {
		t.Fatalf("read-only PCC attribute leaked into patch: %+v", attrs)
	}
	if _, ok := pcc.Relationships["associatedBundleIds"]; !ok {
		t.Fatalf("PCC relationship was dropped: %+v", pcc.Relationships)
	}
}

func TestDisableDeveloperBundleIDCapabilityAlreadyDisabledSkipsPatch(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(false), nil), nil
		default:
			t.Fatalf("unexpected PATCH or extra request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err != nil {
		t.Fatalf("DisableDeveloperBundleIDCapability() error: %v", err)
	}
	if result == nil || result.Changed || result.Enabled || result.Status != "already-disabled" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
}

func TestDisableDeveloperBundleIDCapabilityDisablesEveryDuplicateTarget(t *testing.T) {
	requestCount := 0
	var patchBody []byte
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithTwoPCCStates(true, true), nil), nil
		case 4:
			var err error
			patchBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read patch: %v", err)
			}
			return developerPortalTestResponse(http.StatusOK, ``, nil), nil
		case 5:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithTwoPCCStates(false, false), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err != nil {
		t.Fatalf("DisableDeveloperBundleIDCapability() error: %v", err)
	}
	if result == nil || !result.Changed {
		t.Fatalf("unexpected result: %+v", result)
	}
	var payload struct {
		Data struct {
			Relationships struct {
				Capabilities developerResourceRelationship `json:"bundleIdCapabilities"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(patchBody, &payload); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	if got := payload.Data.Relationships.Capabilities.Data; len(got) != 2 {
		t.Fatalf("capability count = %d, want both duplicate targets preserved: %+v", len(got), got)
	} else {
		for _, capability := range got {
			var attributes map[string]any
			if err := json.Unmarshal(capability.Attributes, &attributes); err != nil {
				t.Fatalf("decode %s attributes: %v", capability.ID, err)
			}
			if attributes["enabled"] != false {
				t.Fatalf("%s was not disabled: %+v", capability.ID, attributes)
			}
		}
	}
}

func TestDisableDeveloperBundleIDCapabilityRejectsUnknownTargetStateBeforePatch(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithPCCAttributes(`{"settings":[]}`), nil), nil
		default:
			t.Fatalf("unexpected write after unknown state: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err == nil || !strings.Contains(err.Error(), "missing an enabled state") {
		t.Fatalf("error = %v, want missing enabled state", err)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
}

func TestEnableDeveloperBundleIDCapabilityRejectsWrongResourceIDBeforePatch(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, strings.Replace(developerBundleResponse(false), `"bundle-1"`, `"other-bundle"`, 1), nil), nil
		default:
			t.Fatalf("unexpected write after wrong resource id: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.EnableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityEnableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err == nil || !strings.Contains(err.Error(), "returned resource") {
		t.Fatalf("error = %v, want exact-resource rejection", err)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
}

func TestDisableDeveloperBundleIDCapabilityRejectsWrongResourceIDBeforePatch(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, strings.Replace(developerBundleResponse(true), `"bundle-1"`, `"other-bundle"`, 1), nil), nil
		default:
			t.Fatalf("unexpected write after wrong resource id: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err == nil || !strings.Contains(err.Error(), "returned resource") {
		t.Fatalf("error = %v, want exact-resource rejection", err)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
}

func TestDisableDeveloperBundleIDCapabilityAmbiguousWriteUsesDisabledRead(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(true), nil), nil
		case 4:
			return nil, errors.New("write response lost")
		case 5:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithPCCAttributes(`{"enabled":false,"settings":[]}`), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err != nil {
		t.Fatalf("DisableDeveloperBundleIDCapability() error: %v", err)
	}
	if result == nil || !result.Changed || result.Status != "disabled" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if requestCount != 5 {
		t.Fatalf("request count = %d, want 5 (one write plus one settling read)", requestCount)
	}
}

func TestDisableDeveloperBundleIDCapabilityServerErrorUsesDisabledRead(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusRequestTimeout} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			requestCount := 0
			client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
				requestCount++
				switch requestCount {
				case 1:
					return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
				case 2:
					return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
				case 3:
					return developerPortalTestResponse(http.StatusOK, developerBundleResponse(true), nil), nil
				case 4:
					return developerPortalTestResponse(status, `{"errors":[{"code":"TEMPORARY_FAILURE"}]}`, nil), nil
				case 5:
					return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithPCCAttributes(`{"enabled":false,"settings":[]}`), nil), nil
				default:
					t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
					return nil, nil
				}
			})

			result, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
				BundleID:   "bundle-1",
				Capability: "PRIVATE_CLOUD_COMPUTE",
			})
			if err != nil {
				t.Fatalf("DisableDeveloperBundleIDCapability() error: %v", err)
			}
			if result == nil || !result.Changed || result.Status != "disabled" {
				t.Fatalf("unexpected result: %+v", result)
			}
			if requestCount != 5 {
				t.Fatalf("request count = %d, want 5 (one server error plus one settling read)", requestCount)
			}
		})
	}
}

func TestDisableDeveloperBundleIDCapabilityAllowsCompleteTargetRemoval(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(true), nil), nil
		case 4:
			return developerPortalTestResponse(http.StatusOK, ``, nil), nil
		case 5:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(false), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err != nil {
		t.Fatalf("DisableDeveloperBundleIDCapability() error: %v", err)
	}
	if result == nil || !result.Changed || result.Enabled || result.Status != "disabled" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if requestCount != 5 {
		t.Fatalf("request count = %d, want 5", requestCount)
	}
}

func TestDisableDeveloperBundleIDCapabilityRejectsOmittedIncludedGraphAfterWrite(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(true), nil), nil
		case 4:
			return developerPortalTestResponse(http.StatusOK, ``, nil), nil
		case 5:
			return developerPortalTestResponse(http.StatusOK, `{"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app"}}}`, nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	var unverified *DeveloperBundleIDCapabilityUnverifiedError
	if !errors.As(err, &unverified) {
		t.Fatalf("error = %v, want unverified omitted-graph result", err)
	}
	if !strings.Contains(err.Error(), "did not resolve") {
		t.Fatalf("error = %v, want unresolved ownership diagnostic", err)
	}
	if requestCount != 5 {
		t.Fatalf("request count = %d, want 5", requestCount)
	}
}

func TestDisableDeveloperBundleIDCapabilityRejectsMalformedIncludedGraphAfterWrite(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(true), nil), nil
		case 4:
			return developerPortalTestResponse(http.StatusOK, ``, nil), nil
		case 5:
			return developerPortalTestResponse(http.StatusOK, `{"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app"},"relationships":{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":"icloud-1"}]}}},"included":[{"type":"bundleIdCapabilities","id":"icloud-1","attributes":{"enabled":true}}]}`, nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	var unverified *DeveloperBundleIDCapabilityUnverifiedError
	if !errors.As(err, &unverified) {
		t.Fatalf("error = %v, want unverified malformed-graph result", err)
	}
	if !strings.Contains(err.Error(), "missing capability relationship") {
		t.Fatalf("error = %v, want malformed included-data diagnostic", err)
	}
	if requestCount != 5 {
		t.Fatalf("request count = %d, want 5", requestCount)
	}
}

func TestDisableDeveloperBundleIDCapabilityRejectsDroppedNonTargetResource(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithPCCAndICloud(true), nil), nil
		case 4:
			return developerPortalTestResponse(http.StatusOK, ``, nil), nil
		case 5:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(false), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	var unverified *DeveloperBundleIDCapabilityUnverifiedError
	if !errors.As(err, &unverified) {
		t.Fatalf("error = %v, want unverified dropped-non-target result", err)
	}
	if requestCount != 5 {
		t.Fatalf("request count = %d, want 5", requestCount)
	}
}

func TestDisableDeveloperBundleIDCapabilityRejectsNewTargetResource(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(true), nil), nil
		case 4:
			return developerPortalTestResponse(http.StatusOK, ``, nil), nil
		case 5:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithTwoPCCStates(false, false), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	var unverified *DeveloperBundleIDCapabilityUnverifiedError
	if !errors.As(err, &unverified) {
		t.Fatalf("error = %v, want unverified new-target result", err)
	}
	if requestCount != 5 {
		t.Fatalf("request count = %d, want 5", requestCount)
	}
}

func TestDisableDeveloperBundleIDCapabilityRejectsPartialTargetRemoval(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithTwoPCCStates(true, true), nil), nil
		case 4:
			return developerPortalTestResponse(http.StatusOK, ``, nil), nil
		case 5:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithPCCAttributes(`{"enabled":false,"settings":[]}`), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	var unverified *DeveloperBundleIDCapabilityUnverifiedError
	if !errors.As(err, &unverified) {
		t.Fatalf("error = %v, want unverified partial-removal result", err)
	}
	if requestCount != 5 {
		t.Fatalf("request count = %d, want 5", requestCount)
	}
}

func TestDisableDeveloperBundleIDCapabilityAmbiguousPriorReadIsUnverifiedWithoutRetry(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(true), nil), nil
		case 4:
			return nil, errors.New("write response lost")
		case 5:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(true), nil), nil
		default:
			t.Fatalf("unexpected retry request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	var unverified *DeveloperBundleIDCapabilityUnverifiedError
	if !errors.As(err, &unverified) {
		t.Fatalf("error = %v, want unverified prior-state result", err)
	}
	if !strings.Contains(err.Error(), "no automatic retry was sent") {
		t.Fatalf("error = %v, want no-retry diagnostic", err)
	}
	if requestCount != 5 {
		t.Fatalf("request count = %d, want 5 with no retry", requestCount)
	}
}

func TestDeveloperBundleIDCapabilityEnabledKeepsLegacyNullBehavior(t *testing.T) {
	resource := developerResource{ID: "pcc-1", Attributes: json.RawMessage(`{"enabled":null}`)}
	enabled, err := developerBundleIDCapabilityEnabled(resource)
	if err != nil || enabled {
		t.Fatalf("legacy enabled parser = %t, %v; want false, nil", enabled, err)
	}
	strictEnabled, present, strictErr := developerBundleIDCapabilityEnabledValue(resource)
	if strictErr == nil || !present || strictEnabled {
		t.Fatalf("disable-only strict parser = %t, %t, %v; want false, true, error", strictEnabled, present, strictErr)
	}
}

func TestDisableDeveloperBundleIDCapabilityAmbiguousMixedReadIsUnverifiedWithoutRetry(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithTwoPCCStates(true, true), nil), nil
		case 4:
			return nil, errors.New("write response lost")
		case 5:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponseWithTwoPCCStates(false, true), nil), nil
		default:
			t.Fatalf("unexpected retry request %d: %s %s", requestCount, r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	var unverified *DeveloperBundleIDCapabilityUnverifiedError
	if !errors.As(err, &unverified) {
		t.Fatalf("error = %v, want DeveloperBundleIDCapabilityUnverifiedError", err)
	}
	if !strings.Contains(err.Error(), "no automatic retry was sent") {
		t.Fatalf("error = %v, want no-retry diagnostic", err)
	}
	if requestCount != 5 {
		t.Fatalf("request count = %d, want 5 with no retry", requestCount)
	}
}

func TestSelectDeveloperPortalTeam(t *testing.T) {
	teams := []developerPortalTeam{
		{TeamID: "TEAMONE123", Name: "Example"},
		{TeamID: "TEAMTWO456", Name: "Example Company"},
	}

	t.Run("exact public provider id wins", func(t *testing.T) {
		team, err := selectDeveloperPortalTeam(teams, "TEAMONE123", "Example Company")
		if err != nil {
			t.Fatalf("selectDeveloperPortalTeam() error: %v", err)
		}
		if team.TeamID != "TEAMONE123" {
			t.Fatalf("team = %+v", team)
		}
	})

	t.Run("exact provider name", func(t *testing.T) {
		team, err := selectDeveloperPortalTeam(teams, "", "Example Company")
		if err != nil {
			t.Fatalf("selectDeveloperPortalTeam() error: %v", err)
		}
		if team.TeamID != "TEAMTWO456" {
			t.Fatalf("team = %+v", team)
		}
	})

	t.Run("unique provider name prefix", func(t *testing.T) {
		unique := []developerPortalTeam{
			{TeamID: "ACME123", Name: "Acme"},
			{TeamID: "OTHER456", Name: "Other"},
		}
		team, err := selectDeveloperPortalTeam(unique, "", "Acme Inc")
		if err != nil {
			t.Fatalf("selectDeveloperPortalTeam() error: %v", err)
		}
		if team.TeamID != "ACME123" {
			t.Fatalf("team = %+v", team)
		}
	})

	t.Run("single team fallback", func(t *testing.T) {
		team, err := selectDeveloperPortalTeam(teams[:1], "", "Different Provider")
		if err != nil {
			t.Fatalf("selectDeveloperPortalTeam() error: %v", err)
		}
		if team.TeamID != "TEAMONE123" {
			t.Fatalf("team = %+v", team)
		}
	})

	t.Run("ambiguous teams", func(t *testing.T) {
		_, err := selectDeveloperPortalTeam(teams, "", "Different Provider")
		if err == nil {
			t.Fatal("expected provider matching error")
		}
		if !strings.Contains(err.Error(), "--developer-team") || !strings.Contains(err.Error(), "TEAMONE123") {
			t.Fatalf("error %q does not mention --developer-team or available teams", err)
		}
	})
}

func TestEnableDeveloperBundleIDCapabilityAlreadyEnabledSkipsPatch(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(true), nil), nil
		default:
			t.Fatalf("unexpected PATCH or extra request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.EnableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityEnableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err != nil {
		t.Fatalf("EnableDeveloperBundleIDCapability() error: %v", err)
	}
	if !result.Enabled || result.Changed || result.Status != "already-enabled" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestEnableDeveloperBundleIDCapabilityFailsClosedWhenSelectedRelationshipIsOmitted(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, `{
				"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app"}},
				"included":[
					{"type":"bundleIdCapabilities","id":"iap-1","attributes":{"enabled":true,"settings":[]},"relationships":{"capability":{"data":{"type":"capabilities","id":"IN_APP_PURCHASE"}}}},
					{"type":"bundleIdCapabilities","id":"pcc-1","attributes":{"enabled":true,"settings":[]},"relationships":{"capability":{"data":{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE"}}}}
				]
			}`, nil), nil
		default:
			t.Fatalf("unexpected PATCH or extra request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.EnableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityEnableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err == nil || result != nil || !strings.Contains(err.Error(), "did not resolve") {
		t.Fatalf("result = %+v, error = %v; want fail-closed unresolved relationship", result, err)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
}

func TestDeveloperBundleIDCapabilityMutationsRejectPotentiallyTruncatedGraph(t *testing.T) {
	references := make([]string, 0, developerBundleIDCapabilitiesIncludeLimit)
	included := make([]string, 0, developerBundleIDCapabilitiesIncludeLimit)
	for index := 0; index < developerBundleIDCapabilitiesIncludeLimit; index++ {
		references = append(references, fmt.Sprintf(`{"type":"bundleIdCapabilities","id":"cap-%d"}`, index))
		included = append(included, fmt.Sprintf(`{"type":"bundleIdCapabilities","id":"cap-%d","attributes":{"enabled":true,"settings":[]},"relationships":{"capability":{"data":{"type":"capabilities","id":"CAPABILITY_%d"}}}}`, index, index))
	}
	var response developerBundleIDResponse
	body := fmt.Sprintf(`{"data":{"id":"bundle-1","type":"bundleIds","attributes":{"identifier":"com.example.app"},"relationships":{"bundleIdCapabilities":{"data":[%s]}}},"included":[%s]}`, strings.Join(references, ","), strings.Join(included, ","))
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatal(err)
	}

	_, _, err := buildDeveloperBundleIDCapabilityPatchRequest(response, DeveloperBundleIDCapabilityEnableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err == nil || !strings.Contains(err.Error(), "may be truncated") {
		t.Fatalf("enable error = %v, want truncation refusal", err)
	}
	_, err = developerBundleIDCapabilityDisableState(response, "PRIVATE_CLOUD_COMPUTE")
	if err == nil || !strings.Contains(err.Error(), "may be truncated") {
		t.Fatalf("disable error = %v, want truncation refusal", err)
	}
}

func TestEnableDeveloperBundleIDCapabilityUpdatesDisabledTargetOnce(t *testing.T) {
	requestCount := 0
	var patchBody []byte
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, `{
				"data":{
					"id":"bundle-1",
					"type":"bundleIds",
					"attributes":{"name":"Example","identifier":"com.example.app"},
					"relationships":{"bundleIdCapabilities":{"data":[
						{"type":"bundleIdCapabilities","id":"pcc-disabled-1"},
						{"type":"bundleIdCapabilities","id":"pcc-disabled-2"}
					]}}
				},
				"included":[
					{
						"type":"bundleIdCapabilities",
						"id":"pcc-disabled-1",
						"attributes":{"enabled":false,"settings":[{"key":"EXISTING_SETTING"}],"portalOwned":"keep"},
						"relationships":{
							"capability":{"data":{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE"}},
							"associatedBundleIds":{"data":[{"type":"bundleIds","id":"related-1"}]}
						}
					},
					{
						"type":"bundleIdCapabilities",
						"id":"pcc-disabled-2",
						"attributes":{"enabled":false,"settings":[]},
						"relationships":{"capability":{"data":{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE"}}}
					}
				]
			}`, nil), nil
		case 4:
			var err error
			patchBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			return developerPortalTestResponse(http.StatusOK, `{"data":{"type":"bundleIds","id":"bundle-1"}}`, nil), nil
		default:
			t.Fatalf("unexpected request")
			return nil, nil
		}
	})

	result, err := client.EnableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityEnableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err != nil {
		t.Fatalf("EnableDeveloperBundleIDCapability() error: %v", err)
	}
	if !result.Changed {
		t.Fatalf("expected changed result: %+v", result)
	}

	var payload struct {
		Data struct {
			Relationships struct {
				BundleIDCapabilities developerResourceRelationship `json:"bundleIdCapabilities"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(patchBody, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	caps := payload.Data.Relationships.BundleIDCapabilities.Data
	if len(caps) != 1 {
		t.Fatalf("target capability count = %d, want 1: %+v", len(caps), caps)
	}
	if caps[0].ID != "pcc-disabled-1" {
		t.Fatalf("first target identity not preserved: %+v", caps[0])
	}
	var attributes map[string]any
	if err := json.Unmarshal(caps[0].Attributes, &attributes); err != nil {
		t.Fatalf("decode attributes: %v", err)
	}
	if attributes["enabled"] != true {
		t.Fatalf("target enabled state not updated: %+v", attributes)
	}
	if _, ok := attributes["portalOwned"]; ok {
		t.Fatalf("read-only target attribute was sent in PATCH: %+v", attributes)
	}
	settings, ok := attributes["settings"].([]any)
	if !ok || len(settings) != 1 {
		t.Fatalf("target settings not preserved: %+v", attributes["settings"])
	}
	if _, ok := caps[0].Relationships["associatedBundleIds"]; !ok {
		t.Fatalf("target relationships not preserved: %+v", caps[0].Relationships)
	}
}

func TestBuildDeveloperBundleIDCapabilityPatchPrefersEnabledDuplicate(t *testing.T) {
	current := developerBundleIDResponse{}
	current.Data.ID = "bundle-1"
	current.Data.Type = "bundleIds"
	current.Data.Attributes = json.RawMessage(`{"name":"Example","identifier":"com.example.app"}`)
	current.Data.Relationships = map[string]json.RawMessage{
		"bundleIdCapabilities": json.RawMessage(`{"data":[
			{"type":"bundleIdCapabilities","id":"pcc-disabled","attributes":{"enabled":false,"settings":[{"key":"DISABLED"}]},"relationships":{"capability":{"data":{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE"}}}},
			{"type":"bundleIdCapabilities","id":"pcc-enabled","attributes":{"enabled":true,"settings":[{"key":"ENABLED"}]},"relationships":{"capability":{"data":{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE"}},"associatedBundleIds":{"data":[{"type":"bundleIds","id":"related-1"}]}}}
		]}`),
	}

	_, alreadyEnabled, err := buildDeveloperBundleIDCapabilityPatchRequest(current, DeveloperBundleIDCapabilityEnableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err != nil {
		t.Fatalf("buildDeveloperBundleIDCapabilityPatchRequest() error: %v", err)
	}
	if !alreadyEnabled {
		t.Fatal("enabled duplicate was not recognized")
	}
}

func TestEnableDeveloperBundleIDCapabilityRejectsUnavailableAndNonEditable(t *testing.T) {
	tests := []struct {
		name         string
		metadataBody string
		wantErr      string
	}{
		{name: "unavailable", metadataBody: `{"data":[]}`, wantErr: "not available"},
		{name: "non-editable", metadataBody: developerCapabilityMetadata(false), wantErr: "not editable"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requestCount := 0
			client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
				requestCount++
				if requestCount == 1 {
					return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
				}
				return developerPortalTestResponse(http.StatusOK, tc.metadataBody, nil), nil
			})

			_, err := client.EnableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityEnableRequest{
				BundleID:   "bundle-1",
				Capability: "PRIVATE_CLOUD_COMPUTE",
			})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestEnableDeveloperBundleIDCapabilityRejectsUnsupportedCapabilityBeforeHTTP(t *testing.T) {
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", r.Method, r.URL.String())
		return nil, nil
	})

	_, err := client.EnableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityEnableRequest{
		BundleID:   "bundle-1",
		Capability: "ICLOUD",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported Developer Portal capability") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnableDeveloperBundleIDCapabilityRequiresDeveloperPortalSession(t *testing.T) {
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		return developerPortalTestResponse(http.StatusForbidden, `forbidden`, nil), nil
	})

	_, err := client.EnableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityEnableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err == nil || !strings.Contains(err.Error(), "Developer Portal") || !strings.Contains(err.Error(), "asc web auth login") || strings.Contains(err.Error(), "--reauthenticate") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnableDeveloperBundleIDCapabilityRequiresCSRFTokensBeforePatch(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), nil), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(false), nil), nil
		default:
			t.Fatalf("unexpected PATCH without CSRF headers")
			return nil, nil
		}
	})

	_, err := client.EnableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityEnableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err == nil || !strings.Contains(err.Error(), "CSRF") || !strings.Contains(err.Error(), "asc web auth login") || strings.Contains(err.Error(), "--reauthenticate") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnableDeveloperBundleIDCapabilitySurfacesAppleError(t *testing.T) {
	requestCount := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, developerBundleResponse(false), nil), nil
		case 4:
			return developerPortalTestResponse(http.StatusUnprocessableEntity, `{"errors":[{"code":"CAPABILITY_NOT_ALLOWED"}]}`, http.Header{"X-Apple-Request-UUID": []string{"request-1"}}), nil
		default:
			t.Fatalf("unexpected request")
			return nil, nil
		}
	})

	_, err := client.EnableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityEnableRequest{
		BundleID:   "bundle-1",
		Capability: "PRIVATE_CLOUD_COMPUTE",
	})
	if err == nil || !strings.Contains(err.Error(), "status 422") || !strings.Contains(err.Error(), "CAPABILITY_NOT_ALLOWED") || !strings.Contains(err.Error(), "request-1") {
		t.Fatalf("error = %v", err)
	}
}

func developerPortalTestClient(t *testing.T, fn roundTripFunc) *Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	return &Client{httpClient: &http.Client{Jar: jar, Transport: fn}}
}

func developerPortalTestResponse(status int, body string, headers http.Header) *http.Response {
	if headers == nil {
		headers = make(http.Header)
	}
	return &http.Response{
		StatusCode: status,
		Header:     headers,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func developerPortalTeamsFixture() string {
	return `{"teams":[{"teamId":"TEAM123456","name":"Example Team","status":"active"}]}`
}

func decodeDeveloperPortalProxyReadRequest(t *testing.T, r *http.Request) developerPortalProxyReadRequest {
	t.Helper()
	var request developerPortalProxyReadRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		t.Fatalf("decode Developer Portal proxy request: %v", err)
	}
	return request
}

func developerCapabilityMetadata(editable bool) string {
	if editable {
		return `{"data":[{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE","attributes":{"name":"Access to models on Private Cloud Compute","editable":true}}]}`
	}
	return `{"data":[{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE","attributes":{"name":"Access to models on Private Cloud Compute","editable":false}}]}`
}

func developerBundleResponse(enabled bool) string {
	if enabled {
		return `{"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app"},"relationships":{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":"pcc-1"}]}}},"included":[{"type":"bundleIdCapabilities","id":"pcc-1","attributes":{"enabled":true,"settings":[]},"relationships":{"capability":{"data":{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE"}}}}]}`
	}
	return `{"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app"},"relationships":{"bundleIdCapabilities":{"data":[]}}},"included":[]}`
}

func developerBundleResponseWithPCCAttributes(attributes string) string {
	return fmt.Sprintf(`{"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app"},"relationships":{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":"pcc-1"}]}}},"included":[{"type":"bundleIdCapabilities","id":"pcc-1","attributes":%s,"relationships":{"capability":{"data":{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE"}}}}]}`, attributes)
}

func developerBundleResponseWithTwoPCCStates(firstEnabled, secondEnabled bool) string {
	return fmt.Sprintf(`{"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app"},"relationships":{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":"pcc-1"},{"type":"bundleIdCapabilities","id":"pcc-2"}]}}},"included":[{"type":"bundleIdCapabilities","id":"pcc-1","attributes":{"enabled":%t,"settings":[]},"relationships":{"capability":{"data":{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE"}}}},{"type":"bundleIdCapabilities","id":"pcc-2","attributes":{"enabled":%t,"settings":[]},"relationships":{"capability":{"data":{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE"}}}}]}`, firstEnabled, secondEnabled)
}

func developerBundleResponseWithICloudOnly() string {
	return `{"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app","platform":"IOS","seedId":"TEAMID"},"relationships":{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":"icloud-1"}]}}},"included":[{"type":"bundleIdCapabilities","id":"icloud-1","attributes":{"enabled":true,"settings":[{"key":"ICLOUD_VERSION"}]},"relationships":{"capability":{"data":{"type":"capabilities","id":"ICLOUD"}},"cloudContainers":{"data":[{"type":"cloudContainers","id":"cloud-1"}]}}}]}`
}

func developerBundleResponseWithPCCAndICloud(pccEnabled bool) string {
	return fmt.Sprintf(`{"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app","platform":"IOS","seedId":"TEAMID"},"relationships":{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":"pcc-1"},{"type":"bundleIdCapabilities","id":"icloud-1"}]}}},"included":[{"type":"bundleIdCapabilities","id":"pcc-1","attributes":{"enabled":%t,"settings":[]},"relationships":{"capability":{"data":{"type":"capabilities","id":"PRIVATE_CLOUD_COMPUTE"}}}},{"type":"bundleIdCapabilities","id":"icloud-1","attributes":{"enabled":true,"settings":[{"key":"ICLOUD_VERSION"}]},"relationships":{"capability":{"data":{"type":"capabilities","id":"ICLOUD"}},"cloudContainers":{"data":[{"type":"cloudContainers","id":"cloud-1"}]}}}]}`, pccEnabled)
}

func TestDisableDeveloperBundleIDCapabilityRejectsContradictoryIncludedGraph(t *testing.T) {
	for _, scenario := range []string{"conflicting duplicate", "explicit empty relationship", "unreferenced capability"} {
		t.Run(scenario, func(t *testing.T) {
			var response developerBundleIDResponse
			if err := json.Unmarshal([]byte(developerBundleResponse(true)), &response); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "conflicting duplicate":
				duplicate := response.Included[0]
				duplicate.Attributes = json.RawMessage(`{"enabled":false,"settings":[]}`)
				response.Included = append(response.Included, duplicate)
			case "explicit empty relationship":
				response.Data.Relationships["bundleIdCapabilities"] = json.RawMessage(`{"data":[]}`)
			case "unreferenced capability":
				extra := response.Included[0]
				extra.ID = "unreferenced-pcc"
				response.Included = append(response.Included, extra)
			}
			encoded, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			requests := 0
			client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
				requests++
				switch requests {
				case 1:
					return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
				case 2:
					return developerPortalTestResponse(http.StatusOK, developerCapabilityMetadata(true), http.Header{"csrf": {"token"}, "csrf_ts": {"time"}}), nil
				case 3:
					return developerPortalTestResponse(http.StatusOK, string(encoded), nil), nil
				default:
					return nil, fmt.Errorf("unexpected request after contradictory preflight: %s %s", r.Method, r.URL.Path)
				}
			})
			result, err := client.DisableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityDisableRequest{BundleID: "bundle-1", Capability: "PRIVATE_CLOUD_COMPUTE"})
			if err == nil || result != nil || requests != 3 {
				t.Fatalf("expected preflight refusal without write: result=%+v error=%v requests=%d", result, err, requests)
			}
		})
	}
}
