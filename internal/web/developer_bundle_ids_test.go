package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

func TestEnableDeveloperBundleIDCapabilityPreservesFullPayload(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}

	var patchBody []byte
	requestCount := 0
	client := &Client{
		httpClient: &http.Client{
			Jar: jar,
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
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
					return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": []string{"bootstrap-csrf"}, "csrf_ts": []string{"bootstrap-ts"}}), nil
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
					}`, http.Header{"csrf": []string{"secret-csrf"}, "csrf_ts": []string{"secret-ts"}}), nil
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
					if got := query.Get("fields[bundleIds]"); got != "name,identifier,platform,seedId,wildcard,~permissions.delete,~permissions.edit" {
						t.Fatalf("fields[bundleIds] = %q", got)
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
							"attributes":{"enabled":true,"settings":[{"key":"ICLOUD_VERSION","options":[{"key":"XCODE_6","enabled":true}]}]},
							"relationships":{
								"capability":{"data":{"type":"capabilities","id":"ICLOUD"}},
								"appGroups":{"data":[{"type":"appGroups","id":"group-1"}]},
								"cloudContainers":{"data":[{"type":"cloudContainers","id":"cloud-1"}]}
							}
						}]
					}`, nil), nil
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
					return developerPortalTestResponse(http.StatusOK, `{"data":{"type":"bundleIds","id":"bundle-1"}}`, nil), nil
				default:
					t.Fatalf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.String())
					return nil, nil
				}
			}),
		},
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
	if payload.Data.Attributes["platform"] != "IOS" || payload.Data.Attributes["seedId"] != "TEAMID" || payload.Data.Attributes["~permissions.edit"] != true {
		t.Fatalf("bundle attributes were not preserved: %+v", payload.Data.Attributes)
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

	t.Run("provider name suffix", func(t *testing.T) {
		team, err := selectDeveloperPortalTeam(teams, "", "Example Company (App Store Connect)")
		if err != nil {
			t.Fatalf("selectDeveloperPortalTeam() error: %v", err)
		}
		if team.TeamID != "TEAMTWO456" {
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
		if _, err := selectDeveloperPortalTeam(teams, "", "Different Provider"); err == nil {
			t.Fatal("expected provider matching error")
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

func TestEnableDeveloperBundleIDCapabilityUsesIncludedCapabilitiesWhenTopLevelRelationshipsAreOmitted(t *testing.T) {
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
	if err != nil {
		t.Fatalf("EnableDeveloperBundleIDCapability() error: %v", err)
	}
	if !result.Enabled || result.Changed || result.Status != "already-enabled" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
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
	if attributes["enabled"] != true || attributes["portalOwned"] != "keep" {
		t.Fatalf("target attributes not preserved: %+v", attributes)
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
