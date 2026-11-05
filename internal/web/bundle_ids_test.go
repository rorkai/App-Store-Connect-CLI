package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestSyncAppClipBundleIDCapabilityAddsParentBundleRelationship(t *testing.T) {
	var patchBody []byte
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/bundleIds/clip-bundle":
				if r.URL.Query().Get("include") != "bundleIdCapabilities" {
					t.Fatalf("expected include=bundleIdCapabilities, got %q", r.URL.RawQuery)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(bytes.NewBufferString(`{
						"data":{
							"id":"clip-bundle",
							"type":"bundleIds",
							"attributes":{
								"name":"Example Clip",
								"identifier":"com.example.app.Clip",
								"seedId":"TEAMID"
							}
						}
					}`)),
				}, nil
			case r.Method == http.MethodPatch && r.URL.Path == "/iris/v1/bundleIds/clip-bundle":
				var err error
				patchBody, err = io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("ReadAll patch body error: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"id":"clip-bundle","type":"bundleIds"}}`)),
				}, nil
			default:
				t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
				return nil, nil
			}
		})},
	}

	enabled := true
	result, err := client.SyncAppClipBundleIDCapability(context.Background(), AppClipBundleIDCapabilitySyncRequest{
		BundleID:       "clip-bundle",
		ParentBundleID: "parent-bundle",
		Capability:     "PUSH_NOTIFICATIONS",
		Enabled:        true,
		Settings: []BundleIDCapabilitySetting{{
			Key: "PUSH_NOTIFICATION_FEATURES",
			Options: []BundleIDCapabilityOption{{
				Key:     "PUSH_NOTIFICATION_FEATURE_BROADCAST",
				Enabled: &enabled,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("SyncAppClipBundleIDCapability error: %v", err)
	}
	if result.ParentBundleID != "parent-bundle" || result.Capability != "PUSH_NOTIFICATIONS" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(patchBody) == 0 {
		t.Fatal("expected patch body")
	}

	var payload struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Name       string `json:"name"`
				Identifier string `json:"identifier"`
				SeedID     string `json:"seedId"`
			} `json:"attributes"`
			Relationships struct {
				BundleIDCapabilities struct {
					Data []struct {
						Type       string `json:"type"`
						Attributes struct {
							Enabled  bool `json:"enabled"`
							Settings []struct {
								Key string `json:"key"`
							} `json:"settings"`
						} `json:"attributes"`
						Relationships struct {
							Capability     webBundleIDRelationshipData `json:"capability"`
							ParentBundleID webBundleIDRelationshipData `json:"parentBundleId"`
						} `json:"relationships"`
					} `json:"data"`
				} `json:"bundleIdCapabilities"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(patchBody, &payload); err != nil {
		t.Fatalf("json.Unmarshal patch body error: %v; body=%s", err, patchBody)
	}
	if payload.Data.Type != "bundleIds" || payload.Data.ID != "clip-bundle" {
		t.Fatalf("unexpected bundle data: %+v", payload.Data)
	}
	if payload.Data.Attributes.Identifier != "com.example.app.Clip" || payload.Data.Attributes.SeedID != "TEAMID" {
		t.Fatalf("expected current bundle attributes in patch, got %+v", payload.Data.Attributes)
	}
	caps := payload.Data.Relationships.BundleIDCapabilities.Data
	if len(caps) != 1 {
		t.Fatalf("expected one capability relationship, got %d", len(caps))
	}
	if caps[0].Relationships.Capability.Data != (relationshipData{Type: "capabilities", ID: "PUSH_NOTIFICATIONS"}) {
		t.Fatalf("unexpected capability relationship: %+v", caps[0].Relationships.Capability.Data)
	}
	if caps[0].Relationships.ParentBundleID.Data != (relationshipData{Type: "bundleIds", ID: "parent-bundle"}) {
		t.Fatalf("unexpected parentBundleId relationship: %+v", caps[0].Relationships.ParentBundleID.Data)
	}
	if len(caps[0].Attributes.Settings) != 1 || caps[0].Attributes.Settings[0].Key != "PUSH_NOTIFICATION_FEATURES" {
		t.Fatalf("expected settings to be preserved, got %+v", caps[0].Attributes.Settings)
	}
}

func TestSyncAppClipBundleIDCapabilityPreservesExistingCapabilities(t *testing.T) {
	var patchBody []byte
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/bundleIds/clip-bundle":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(bytes.NewBufferString(`{
						"data":{
							"id":"clip-bundle",
							"type":"bundleIds",
							"attributes":{
								"name":"Example Clip",
								"identifier":"com.example.app.Clip"
							},
							"relationships":{
								"bundleIdCapabilities":{
									"data":[
										{"id":"existing-icloud","type":"bundleIdCapabilities"},
										{"id":"existing-push","type":"bundleIdCapabilities"}
									]
								}
							}
						},
						"included":[
							{
								"id":"existing-icloud",
								"type":"bundleIdCapabilities",
								"attributes":{"enabled":true,"settings":[]},
								"relationships":{
									"capability":{"data":{"type":"capabilities","id":"ICLOUD"}}
								}
							},
							{
								"id":"existing-push",
								"type":"bundleIdCapabilities",
								"attributes":{"enabled":false,"settings":[{"key":"PUSH_NOTIFICATION_FEATURES"}]},
								"relationships":{
									"capability":{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"}}
								}
							}
						]
					}`)),
				}, nil
			case r.Method == http.MethodPatch && r.URL.Path == "/iris/v1/bundleIds/clip-bundle":
				var err error
				patchBody, err = io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("ReadAll patch body error: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewBufferString(`{"data":{"id":"clip-bundle","type":"bundleIds"}}`)),
				}, nil
			default:
				t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
				return nil, nil
			}
		})},
	}

	result, err := client.SyncAppClipBundleIDCapability(context.Background(), AppClipBundleIDCapabilitySyncRequest{
		BundleID:       "clip-bundle",
		ParentBundleID: "parent-bundle",
		Capability:     "PUSH_NOTIFICATIONS",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("SyncAppClipBundleIDCapability error: %v", err)
	}

	var payload struct {
		Data struct {
			Relationships struct {
				BundleIDCapabilities struct {
					Data []struct {
						ID            string                     `json:"id"`
						Attributes    map[string]any             `json:"attributes"`
						Relationships map[string]json.RawMessage `json:"relationships"`
					} `json:"data"`
				} `json:"bundleIdCapabilities"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(patchBody, &payload); err != nil {
		t.Fatalf("json.Unmarshal patch body error: %v; body=%s", err, patchBody)
	}
	caps := payload.Data.Relationships.BundleIDCapabilities.Data
	if len(caps) != 2 {
		t.Fatalf("expected existing ICLOUD plus synced PUSH_NOTIFICATIONS, got %d: %+v", len(caps), caps)
	}
	if caps[0].ID != "existing-icloud" {
		t.Fatalf("expected existing ICLOUD capability to be preserved first, got %+v", caps[0])
	}
	assertRawJSONEqual(t, caps[0].Relationships["capability"], `{"data":{"type":"capabilities","id":"ICLOUD"}}`)
	if caps[1].ID != "existing-push" {
		t.Fatalf("expected synced PUSH_NOTIFICATIONS capability to replace existing entry, got %+v", caps[1])
	}
	assertRawJSONEqual(t, caps[1].Relationships["capability"], `{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"}}`)
	assertRawJSONEqual(t, caps[1].Relationships["parentBundleId"], `{"data":{"type":"bundleIds","id":"parent-bundle"}}`)
	settings, ok := caps[1].Attributes["settings"].([]any)
	if !ok || len(settings) != 1 {
		t.Fatalf("expected existing settings to be preserved, got %+v", caps[1].Attributes["settings"])
	}
	setting, ok := settings[0].(map[string]any)
	if !ok || setting["key"] != "PUSH_NOTIFICATION_FEATURES" {
		t.Fatalf("expected existing PUSH_NOTIFICATION_FEATURES setting, got %+v", settings[0])
	}
	if got := result.Changed; !got {
		t.Fatalf("expected Changed=true after PATCH, got %+v", result)
	}
	if result.Status != "synced" {
		t.Fatalf("expected status synced, got %+v", result)
	}
}

// newSyncAppClipBundleIDTestServer serves the web-session bundleIds read and
// patch endpoints. It records PATCH bodies and counts PATCH requests so tests
// can assert both the payload and that no write happened.
func newSyncAppClipBundleIDTestServer(t *testing.T, getBody string, patchCount *atomic.Int32, patchBody *[]byte) *Client {
	t.Helper()
	const bundleID = "clip-bundle"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/bundleIds/"+bundleID:
			if r.URL.Query().Get("include") != "bundleIdCapabilities" {
				t.Errorf("expected include=bundleIdCapabilities, got %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(getBody))
		case r.Method == http.MethodPatch && r.URL.Path == "/iris/v1/bundleIds/"+bundleID:
			patchCount.Add(1)
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("ReadAll patch body error: %v", err)
			}
			*patchBody = body
			_, _ = w.Write([]byte(`{"data":{"id":"` + bundleID + `","type":"bundleIds"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	// The iris client hardcodes Apple's origin, so rewrite every request to the
	// in-process server instead of letting anything reach the network.
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rewritten := r.Clone(r.Context())
		rewritten.URL.Scheme = target.Scheme
		rewritten.URL.Host = target.Host
		rewritten.Host = target.Host
		return http.DefaultTransport.RoundTrip(rewritten)
	})
	return &Client{httpClient: &http.Client{Transport: transport}}
}

const syncedAppClipBundleIDGetBody = `{
	"data":{
		"id":"clip-bundle",
		"type":"bundleIds",
		"attributes":{"name":"Example Clip","identifier":"com.example.app.Clip","seedId":"TEAMID"},
		"relationships":{
			"bundleIdCapabilities":{
				"data":[
					{"id":"existing-icloud","type":"bundleIdCapabilities"},
					{"id":"existing-push","type":"bundleIdCapabilities"}
				]
			}
		}
	},
	"included":[
		{
			"id":"existing-icloud",
			"type":"bundleIdCapabilities",
			"attributes":{"enabled":true,"settings":[]},
			"relationships":{
				"capability":{"data":{"type":"capabilities","id":"ICLOUD"}},
				"cloudContainers":{"data":[{"type":"cloudContainers","id":"container-1"},{"type":"cloudContainers","id":"container-2"}]}
			}
		},
		{
			"id":"existing-push",
			"type":"bundleIdCapabilities",
			"attributes":{"enabled":true,"settings":[{"key":"PUSH_NOTIFICATION_FEATURES","options":[{"key":"PUSH_NOTIFICATION_FEATURE_BROADCAST","enabled":true}]}]},
			"relationships":{
				"capability":{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"}},
				"parentBundleId":{"data":{"type":"bundleIds","id":"parent-bundle"}}
			}
		}
	]
}`

func TestSyncAppClipBundleIDCapabilitySkipsPatchWhenParentAlreadySynced(t *testing.T) {
	var patchCount atomic.Int32
	var patchBody []byte
	client := newSyncAppClipBundleIDTestServer(t, syncedAppClipBundleIDGetBody, &patchCount, &patchBody)

	result, err := client.SyncAppClipBundleIDCapability(context.Background(), AppClipBundleIDCapabilitySyncRequest{
		BundleID:       "clip-bundle",
		ParentBundleID: "parent-bundle",
		Capability:     "push_notifications",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("SyncAppClipBundleIDCapability error: %v", err)
	}
	if got := patchCount.Load(); got != 0 {
		t.Fatalf("expected no PATCH when the parent relationship is already synced, got %d: %s", got, patchBody)
	}
	want := &AppClipBundleIDCapabilitySyncResult{
		BundleID:       "clip-bundle",
		ParentBundleID: "parent-bundle",
		Capability:     "PUSH_NOTIFICATIONS",
		Enabled:        true,
		Changed:        false,
		Status:         "already-synced",
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("result = %+v, want %+v", result, want)
	}
}

func TestSyncAppClipBundleIDCapabilitySkipsPatchWhenAnyDuplicateRecordAlreadyMatches(t *testing.T) {
	var patchCount atomic.Int32
	var patchBody []byte
	client := newSyncAppClipBundleIDTestServer(t, `{
		"data":{
			"id":"clip-bundle",
			"type":"bundleIds",
			"attributes":{"name":"Example Clip","identifier":"com.example.app.Clip"},
			"relationships":{
				"bundleIdCapabilities":{
					"data":[
						{"id":"stale-push","type":"bundleIdCapabilities"},
						{"id":"current-push","type":"bundleIdCapabilities"}
					]
				}
			}
		},
		"included":[
			{
				"id":"stale-push",
				"type":"bundleIdCapabilities",
				"attributes":{"enabled":false,"settings":[]},
				"relationships":{
					"capability":{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"}},
					"parentBundleId":{"data":{"type":"bundleIds","id":"old-parent"}}
				}
			},
			{
				"id":"current-push",
				"type":"bundleIdCapabilities",
				"attributes":{"enabled":true,"settings":[]},
				"relationships":{
					"capability":{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"}},
					"parentBundleId":{"data":{"type":"bundleIds","id":"parent-bundle"}}
				}
			}
		]
	}`, &patchCount, &patchBody)

	result, err := client.SyncAppClipBundleIDCapability(context.Background(), AppClipBundleIDCapabilitySyncRequest{
		BundleID:       "clip-bundle",
		ParentBundleID: "parent-bundle",
		Capability:     "PUSH_NOTIFICATIONS",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("SyncAppClipBundleIDCapability error: %v", err)
	}
	if got := patchCount.Load(); got != 0 {
		t.Fatalf("expected no PATCH when a later duplicate record already matches, got %d: %s", got, patchBody)
	}
	if result.Changed || result.Status != "already-synced" {
		t.Fatalf("expected already-synced receipt, got %+v", result)
	}
}

func TestSyncAppClipBundleIDCapabilitySkipsPatchWhenExplicitSettingsMatch(t *testing.T) {
	tests := []struct {
		name       string
		getBody    string
		capability string
		settingKey string
		optionKey  string
	}{
		{name: "exact settings", getBody: syncedAppClipBundleIDGetBody, capability: "PUSH_NOTIFICATIONS", settingKey: "PUSH_NOTIFICATION_FEATURES", optionKey: "PUSH_NOTIFICATION_FEATURE_BROADCAST"},
		{name: "read-only fields enriched by Apple", getBody: enrichedSettingsAppClipBundleIDGetBody, capability: "PUSH_NOTIFICATIONS", settingKey: "PUSH_NOTIFICATION_FEATURES", optionKey: "PUSH_NOTIFICATION_FEATURE_BROADCAST"},
		{name: "extra keyed options", getBody: multiOptionSettingsAppClipBundleIDGetBody, capability: "ICLOUD", settingKey: "ICLOUD_VERSION", optionKey: "XCODE_6"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var patchCount atomic.Int32
			var patchBody []byte
			client := newSyncAppClipBundleIDTestServer(t, tc.getBody, &patchCount, &patchBody)

			enabled := true
			result, err := client.SyncAppClipBundleIDCapability(context.Background(), AppClipBundleIDCapabilitySyncRequest{
				BundleID:         "clip-bundle",
				ParentBundleID:   "parent-bundle",
				Capability:       tc.capability,
				Enabled:          true,
				SettingsProvided: true,
				Settings: []BundleIDCapabilitySetting{{
					Key:     tc.settingKey,
					Options: []BundleIDCapabilityOption{{Key: tc.optionKey, Enabled: &enabled}},
				}},
			})
			if err != nil {
				t.Fatalf("SyncAppClipBundleIDCapability error: %v", err)
			}
			if got := patchCount.Load(); got != 0 {
				t.Fatalf("expected no PATCH when the requested settings already match, got %d: %s", got, patchBody)
			}
			if result.Changed || result.Status != "already-synced" {
				t.Fatalf("expected already-synced receipt, got %+v", result)
			}
		})
	}
}

const enrichedSettingsAppClipBundleIDGetBody = `{
	"data":{
		"id":"clip-bundle",
		"type":"bundleIds",
		"attributes":{"name":"Example Clip","identifier":"com.example.app.Clip"},
		"relationships":{
			"bundleIdCapabilities":{"data":[{"id":"push-capability","type":"bundleIdCapabilities"}]}
		}
	},
	"included":[
		{
			"id":"push-capability",
			"type":"bundleIdCapabilities",
			"attributes":{
				"enabled":true,
				"settings":[{
					"key":"PUSH_NOTIFICATION_FEATURES",
					"name":"Push Notification Features",
					"description":"Choose the push notification features for this App ID.",
					"enabledByDefault":false,
					"visible":true,
					"allowedInstances":"MULTIPLE",
					"minInstances":0,
					"options":[{
						"key":"PUSH_NOTIFICATION_FEATURE_BROADCAST",
						"name":"Broadcast Capability",
						"description":"Send a notification to many devices at once.",
						"enabled":true,
						"enabledByDefault":false,
						"supportsWildcard":false
					}]
				}]
			},
			"relationships":{
				"capability":{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"}},
				"parentBundleId":{"data":{"type":"bundleIds","id":"parent-bundle"}}
			}
		}
	]
}`

const multiOptionSettingsAppClipBundleIDGetBody = `{
	"data":{
		"id":"clip-bundle",
		"type":"bundleIds",
		"attributes":{"name":"Example Clip","identifier":"com.example.app.Clip"},
		"relationships":{
			"bundleIdCapabilities":{"data":[{"id":"icloud-capability","type":"bundleIdCapabilities"}]}
		}
	},
	"included":[
		{
			"id":"icloud-capability",
			"type":"bundleIdCapabilities",
			"attributes":{
				"enabled":true,
				"settings":[{
					"key":"ICLOUD_VERSION",
					"name":"iCloud Version",
					"options":[
						{"key":"XCODE_5","name":"Compatible with Xcode 5","enabled":false},
						{"key":"XCODE_6","name":"Include CloudKit support","enabled":true}
					]
				}]
			},
			"relationships":{
				"capability":{"data":{"type":"capabilities","id":"ICLOUD"}},
				"parentBundleId":{"data":{"type":"bundleIds","id":"parent-bundle"}}
			}
		}
	]
}`

func TestSyncAppClipBundleIDCapabilityPatchesWhenRequestedKeysAreDuplicated(t *testing.T) {
	var patchCount atomic.Int32
	var patchBody []byte
	client := newSyncAppClipBundleIDTestServer(t, multiOptionSettingsAppClipBundleIDGetBody, &patchCount, &patchBody)

	enabled := true
	result, err := client.SyncAppClipBundleIDCapability(context.Background(), AppClipBundleIDCapabilitySyncRequest{
		BundleID:         "clip-bundle",
		ParentBundleID:   "parent-bundle",
		Capability:       "ICLOUD",
		Enabled:          true,
		SettingsProvided: true,
		Settings: []BundleIDCapabilitySetting{{
			Key: "ICLOUD_VERSION",
			Options: []BundleIDCapabilityOption{
				{Key: "XCODE_6", Enabled: &enabled},
				{Key: "XCODE_6", Enabled: &enabled},
			},
		}},
	})
	if err != nil {
		t.Fatalf("SyncAppClipBundleIDCapability error: %v", err)
	}
	if got := patchCount.Load(); got != 1 {
		t.Fatalf("expected duplicate requested keys to never be reported as already synced, got %d PATCH requests", got)
	}
	if !result.Changed || result.Status != "synced" {
		t.Fatalf("expected synced receipt, got %+v", result)
	}
}

func TestSyncAppClipBundleIDCapabilityPatchesWhenRequestedOptionStateDiffers(t *testing.T) {
	var patchCount atomic.Int32
	var patchBody []byte
	client := newSyncAppClipBundleIDTestServer(t, enrichedSettingsAppClipBundleIDGetBody, &patchCount, &patchBody)

	disabled := false
	result, err := client.SyncAppClipBundleIDCapability(context.Background(), AppClipBundleIDCapabilitySyncRequest{
		BundleID:         "clip-bundle",
		ParentBundleID:   "parent-bundle",
		Capability:       "PUSH_NOTIFICATIONS",
		Enabled:          true,
		SettingsProvided: true,
		Settings: []BundleIDCapabilitySetting{{
			Key:     "PUSH_NOTIFICATION_FEATURES",
			Options: []BundleIDCapabilityOption{{Key: "PUSH_NOTIFICATION_FEATURE_BROADCAST", Enabled: &disabled}},
		}},
	})
	if err != nil {
		t.Fatalf("SyncAppClipBundleIDCapability error: %v", err)
	}
	if got := patchCount.Load(); got != 1 {
		t.Fatalf("expected one PATCH when a caller-controlled option differs, got %d", got)
	}
	if !result.Changed || result.Status != "synced" {
		t.Fatalf("expected synced receipt, got %+v", result)
	}
}

func TestSyncAppClipBundleIDCapabilityPatchesWhenCurrentStateDiffers(t *testing.T) {
	tests := []struct {
		name    string
		request AppClipBundleIDCapabilitySyncRequest
	}{
		{
			name: "different parent",
			request: AppClipBundleIDCapabilitySyncRequest{
				BundleID: "clip-bundle", ParentBundleID: "other-parent", Capability: "PUSH_NOTIFICATIONS", Enabled: true,
			},
		},
		{
			name: "capability missing",
			request: AppClipBundleIDCapabilitySyncRequest{
				BundleID: "clip-bundle", ParentBundleID: "parent-bundle", Capability: "GAME_CENTER", Enabled: true,
			},
		},
		{
			name: "explicit settings differ",
			request: AppClipBundleIDCapabilitySyncRequest{
				BundleID: "clip-bundle", ParentBundleID: "parent-bundle", Capability: "PUSH_NOTIFICATIONS", Enabled: true,
				SettingsProvided: true, Settings: []BundleIDCapabilitySetting{},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var patchCount atomic.Int32
			var patchBody []byte
			client := newSyncAppClipBundleIDTestServer(t, syncedAppClipBundleIDGetBody, &patchCount, &patchBody)
			result, err := client.SyncAppClipBundleIDCapability(context.Background(), tc.request)
			if err != nil {
				t.Fatalf("SyncAppClipBundleIDCapability error: %v", err)
			}
			if got := patchCount.Load(); got != 1 {
				t.Fatalf("expected exactly one PATCH, got %d", got)
			}
			if !result.Changed || result.Status != "synced" {
				t.Fatalf("expected synced receipt, got %+v", result)
			}
		})
	}
}

func TestSyncAppClipBundleIDCapabilityPreservesUnrelatedRelationshipsAndSendsWritableAttributes(t *testing.T) {
	var patchCount atomic.Int32
	var patchBody []byte
	client := newSyncAppClipBundleIDTestServer(t, `{
		"data":{
			"id":"clip-bundle",
			"type":"bundleIds",
			"attributes":{"name":"Example Clip","identifier":"com.example.app.Clip","seedId":"TEAMID","platform":"IOS","wildcard":false},
			"relationships":{
				"bundleIdCapabilities":{
					"data":[
						{"id":"existing-icloud","type":"bundleIdCapabilities"},
						{"id":"existing-groups","type":"bundleIdCapabilities"},
						{"id":"existing-push","type":"bundleIdCapabilities"}
					]
				}
			}
		},
		"included":[
			{
				"id":"existing-icloud",
				"type":"bundleIdCapabilities",
				"attributes":{"enabled":true,"settings":[],"editable":true,"responseId":"read-only"},
				"relationships":{
					"capability":{"data":{"type":"capabilities","id":"ICLOUD"}},
					"cloudContainers":{"data":[{"type":"cloudContainers","id":"container-1"},{"type":"cloudContainers","id":"container-2"}]}
				}
			},
			{
				"id":"existing-groups",
				"type":"bundleIdCapabilities",
				"attributes":{"enabled":true,"settings":[]},
				"relationships":{
					"capability":{"data":{"type":"capabilities","id":"APP_GROUPS"}},
					"appGroups":{"data":[{"type":"appGroups","id":"group-1"}]}
				}
			},
			{
				"id":"existing-push",
				"type":"bundleIdCapabilities",
				"attributes":{"enabled":false,"settings":[{"key":"PUSH_NOTIFICATION_FEATURES"}]},
				"relationships":{
					"capability":{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"}},
					"parentBundleId":{"data":{"type":"bundleIds","id":"stale-parent"}}
				}
			}
		]
	}`, &patchCount, &patchBody)

	result, err := client.SyncAppClipBundleIDCapability(context.Background(), AppClipBundleIDCapabilitySyncRequest{
		BundleID:       "clip-bundle",
		ParentBundleID: "parent-bundle",
		Capability:     "PUSH_NOTIFICATIONS",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("SyncAppClipBundleIDCapability error: %v", err)
	}
	if got := patchCount.Load(); got != 1 {
		t.Fatalf("expected exactly one PATCH, got %d", got)
	}
	if !result.Changed || result.Status != "synced" {
		t.Fatalf("expected synced receipt, got %+v", result)
	}

	var payload struct {
		Data struct {
			Attributes    map[string]any `json:"attributes"`
			Relationships struct {
				BundleIDCapabilities struct {
					Data []struct {
						ID            string                     `json:"id"`
						Type          string                     `json:"type"`
						Attributes    map[string]any             `json:"attributes"`
						Relationships map[string]json.RawMessage `json:"relationships"`
					} `json:"data"`
				} `json:"bundleIdCapabilities"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(patchBody, &payload); err != nil {
		t.Fatalf("json.Unmarshal patch body error: %v; body=%s", err, patchBody)
	}
	wantBundleAttributes := map[string]any{"name": "Example Clip", "identifier": "com.example.app.Clip", "seedId": "TEAMID"}
	if !reflect.DeepEqual(payload.Data.Attributes, wantBundleAttributes) {
		t.Fatalf("expected only writable bundle attributes, got %+v", payload.Data.Attributes)
	}
	caps := payload.Data.Relationships.BundleIDCapabilities.Data
	if len(caps) != 3 {
		t.Fatalf("expected ICLOUD, APP_GROUPS, and synced PUSH_NOTIFICATIONS, got %d: %s", len(caps), patchBody)
	}

	icloud := caps[0]
	if icloud.ID != "existing-icloud" {
		t.Fatalf("expected ICLOUD preserved first, got %+v", icloud)
	}
	if !reflect.DeepEqual(icloud.Attributes, map[string]any{"enabled": true, "settings": []any{}}) {
		t.Fatalf("expected only writable ICLOUD attributes, got %+v", icloud.Attributes)
	}
	assertRawJSONEqual(t, icloud.Relationships["capability"], `{"data":{"type":"capabilities","id":"ICLOUD"}}`)
	assertRawJSONEqual(t, icloud.Relationships["cloudContainers"], `{"data":[{"type":"cloudContainers","id":"container-1"},{"type":"cloudContainers","id":"container-2"}]}`)

	groups := caps[1]
	if groups.ID != "existing-groups" {
		t.Fatalf("expected APP_GROUPS preserved second, got %+v", groups)
	}
	assertRawJSONEqual(t, groups.Relationships["appGroups"], `{"data":[{"type":"appGroups","id":"group-1"}]}`)

	synced := caps[2]
	if synced.ID != "existing-push" || synced.Type != "bundleIdCapabilities" {
		t.Fatalf("expected synced PUSH_NOTIFICATIONS to reuse the existing resource ID, got %+v", synced)
	}
	if synced.Attributes["enabled"] != true {
		t.Fatalf("expected synced capability enabled, got %+v", synced.Attributes)
	}
	if _, hasSettings := synced.Attributes["settings"]; !hasSettings {
		t.Fatalf("expected synced capability to carry preserved settings, got %+v", synced.Attributes)
	}
	assertRawJSONEqual(t, synced.Relationships["capability"], `{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"}}`)
	assertRawJSONEqual(t, synced.Relationships["parentBundleId"], `{"data":{"type":"bundleIds","id":"parent-bundle"}}`)
}

func assertRawJSONEqual(t *testing.T, got json.RawMessage, want string) {
	t.Helper()
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("invalid JSON %s: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("invalid expected JSON %s: %v", want, err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON mismatch:\n got %s\nwant %s", got, want)
	}
}
