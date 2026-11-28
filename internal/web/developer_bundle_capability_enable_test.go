package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEnableDeveloperBundleIDCapabilityUsesMetadataAndVerifiesSave(t *testing.T) {
	enabled := true
	settings := []BundleIDCapabilitySetting{{Key: "DATA_PROTECTION_PERMISSION_LEVEL", Options: []BundleIDCapabilityOption{{Key: "COMPLETE_PROTECTION", Enabled: &enabled}}}}
	settingMetadata := `,"settings":[{"key":"DATA_PROTECTION_PERMISSION_LEVEL","minInstances":1,"allowedInstances":"SINGLE","options":[{"key":"COMPLETE_PROTECTION","enabledByDefault":true,"enabled":true}]}]`
	associationMetadata := `,"associations":[{"relationshipName":"associatedBundleIds","relationshipType":"bundleIds","minInstances":1,"maxLimit":2,"allowedInstances":"MULTIPLE"}]`
	reorderedCurrent := `{"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app"},"relationships":{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":"icloud-1"},{"type":"bundleIdCapabilities","id":"target-1"}]}}},"included":[{"type":"bundleIdCapabilities","id":"icloud-1","attributes":{"enabled":true,"settings":[{"key":"ICLOUD_VERSION"}]},"relationships":{"capability":{"data":{"type":"capabilities","id":"ICLOUD"}}}},{"type":"bundleIdCapabilities","id":"target-1","attributes":{"enabled":true,"settings":[]},"relationships":{"capability":{"data":{"type":"capabilities","id":"CARPLAY_PLAYABLE_CONTENT"}},"associatedBundleIds":{"data":[{"type":"bundleIds","id":"related-1"},{"type":"bundleIds","id":"related-2"}]}}}]}`
	tests := []struct {
		name         string
		capability   string
		metadata     string
		settings     []BundleIDCapabilitySetting
		associations map[string][]string
		verifyBody   string
		currentBody  string
		reverseLinks bool
		patchStatus  int
		wantErr      string
		wantPatch    bool
	}{
		{name: "CarPlay Audio", capability: "CARPLAY_PLAYABLE_CONTENT", wantPatch: true},
		{name: "association order no-op", capability: "CARPLAY_PLAYABLE_CONTENT", metadata: associationMetadata, currentBody: reorderedCurrent, associations: map[string][]string{"associatedBundleIds": {"related-2", "related-1"}}},
		{name: "association order readback", capability: "CARPLAY_PLAYABLE_CONTENT", metadata: associationMetadata, reverseLinks: true, associations: map[string][]string{"associatedBundleIds": {"related-1", "related-2"}}, wantPatch: true},
		{name: "future capability", capability: "Future_Simple_Capability", wantPatch: true},
		{name: "unrelated linkage duplicate", capability: "CARPLAY_PLAYABLE_CONTENT", currentBody: strings.ReplaceAll(developerBundleResponseWithICloudOnly(), `{"type":"cloudContainers","id":"cloud-1"}`, `{"type":"cloudContainers","id":"cloud-1"},{"type":"cloudContainers","id":"cloud-1"}`), wantErr: "invalid or duplicate relationship linkage"},
		{name: "unknown unrelated linkage preserved", capability: "CARPLAY_PLAYABLE_CONTENT", currentBody: strings.ReplaceAll(developerBundleResponseWithICloudOnly(), `{"type":"cloudContainers","id":"cloud-1"}`, `{"type":"cloudContainers","id":"cloud-1"},{"unknown":"kept"}`), wantPatch: true},
		{name: "required settings supplied", capability: "DATA_PROTECTION", metadata: settingMetadata, settings: settings, wantPatch: true},
		{name: "required settings missing", capability: "DATA_PROTECTION", metadata: settingMetadata, wantErr: "DATA_PROTECTION_PERMISSION_LEVEL"},
		{name: "required association missing", capability: "DATA_CONTAINER_ACCESS", metadata: `,"associations":[{"relationshipName":"associatedBundleIds","relationshipType":"bundleIds","minInstances":1,"maxLimit":1,"allowedInstances":"SINGLE"}]`, wantErr: "associatedBundleIds"},
		{name: "required association supplied", capability: "DATA_CONTAINER_ACCESS", metadata: `,"associations":[{"relationshipName":"associatedBundleIds","relationshipType":"bundleIds","minInstances":1,"maxLimit":1,"allowedInstances":"SINGLE"}]`, associations: map[string][]string{"associatedBundleIds": {"related-1"}}, wantPatch: true},
		{name: "opt-in required", capability: "APPROVED_BUT_OPT_IN_REQUIRED", metadata: `,"optInRequired":true`, wantErr: "opt-in"},
		{name: "save did not enable target", capability: "CARPLAY_PLAYABLE_CONTENT", verifyBody: developerBundleResponseWithPCCAndICloud(false), wantErr: "could not be verified", wantPatch: true},
		{name: "save dropped unrelated capability", capability: "CARPLAY_PLAYABLE_CONTENT", verifyBody: developerBundleResponse(true), wantErr: "could not be verified", wantPatch: true},
		{name: "ambiguous save applied", capability: "CARPLAY_PLAYABLE_CONTENT", patchStatus: http.StatusServiceUnavailable, wantPatch: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var patchBody []byte
			patches := 0
			reads := 0
			client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
				switch {
				case r.URL.Path == developerPortalTeamsPath:
					return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
				case r.URL.Path == "/services-account/v1/capabilities":
					body := `{"data":[{"type":"capabilities","id":"` + tc.capability + `","attributes":{"name":"Capability","editable":true` + tc.metadata + `}}]}`
					return developerPortalTestResponse(http.StatusOK, body, http.Header{"csrf": []string{"token"}, "csrf_ts": []string{"time"}}), nil
				case r.URL.Path == "/services-account/v1/bundleIds/bundle-1" && r.Method == http.MethodPatch:
					patches++
					var err error
					patchBody, err = io.ReadAll(r.Body)
					if err != nil {
						t.Fatal(err)
					}
					status := tc.patchStatus
					if status == 0 {
						status = http.StatusOK
					}
					return developerPortalTestResponse(status, `{}`, nil), nil
				case r.URL.Path == "/services-account/v1/bundleIds/bundle-1" && r.Header.Get("X-HTTP-Method-Override") == http.MethodGet:
					reads++
					body := tc.currentBody
					if body == "" {
						body = developerBundleResponseWithICloudOnly()
					}
					if reads > 1 {
						body = tc.verifyBody
						if body == "" {
							body = developerBundleResponseAfterPatch(t, patchBody)
						}
						if tc.reverseLinks {
							original := body
							body = strings.ReplaceAll(body, `[{"id":"related-1","type":"bundleIds"},{"id":"related-2","type":"bundleIds"}]`, `[{"id":"related-2","type":"bundleIds"},{"id":"related-1","type":"bundleIds"}]`)
							if body == original {
								t.Fatal("readback fixture did not reorder association linkage")
							}
						}
					}
					body = strings.ReplaceAll(body, "PRIVATE_CLOUD_COMPUTE", tc.capability)
					return developerPortalTestResponse(http.StatusOK, body, nil), nil
				default:
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
					return nil, nil
				}
			})
			result, err := client.EnableDeveloperBundleIDCapability(context.Background(), DeveloperBundleIDCapabilityEnableRequest{BundleID: "bundle-1", Capability: strings.ToLower(tc.capability), Settings: tc.settings, Associations: tc.associations})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
				if tc.wantPatch {
					var unverified *DeveloperBundleIDCapabilityUnverifiedError
					if !errors.As(err, &unverified) {
						t.Fatalf("expected inspect-before-retry error, got %T", err)
					}
				}
			} else if err != nil || result == nil || !result.Enabled || result.Changed != tc.wantPatch || result.Capability != tc.capability {
				t.Fatalf("result = %+v, error = %v", result, err)
			}
			wantPatches := 0
			if tc.wantPatch {
				wantPatches = 1
			}
			if patches != wantPatches {
				t.Fatalf("PATCH count = %d, want %d", patches, wantPatches)
			}
			if !tc.wantPatch {
				return
			}
			var payload developerBundleIDPatchRequest
			if err := json.Unmarshal(patchBody, &payload); err != nil {
				t.Fatal(err)
			}
			var graph developerResourceRelationship
			if err := json.Unmarshal(payload.Data.Relationships["bundleIdCapabilities"], &graph); err != nil {
				t.Fatal(err)
			}
			if len(graph.Data) != 2 || graph.Data[0].ID != "icloud-1" || graph.Data[1].ID != "" {
				t.Fatalf("expected preserved iCloud and new capability, got %+v", graph.Data)
			}
			if !developerCapabilityResourceSettingsApplied(graph.Data[0], []BundleIDCapabilitySetting{{Key: "ICLOUD_VERSION"}}) {
				t.Fatal("target settings replaced unrelated iCloud settings")
			}
			if tc.settings != nil && !developerCapabilityResourceSettingsApplied(graph.Data[1], tc.settings) {
				t.Fatal("requested settings missing from target capability")
			}
			if tc.associations != nil {
				resources, _, err := developerCapabilityAssociationResources(graph.Data[1].Relationships["associatedBundleIds"])
				if err != nil || len(resources) != len(tc.associations["associatedBundleIds"]) || resources[0].ID != tc.associations["associatedBundleIds"][0] || resources[0].Type != "bundleIds" {
					t.Fatalf("requested association was not saved: %+v, error = %v", resources, err)
				}
			}
			id, err := developerBundleIDCapabilityID(graph.Data[1])
			if err != nil || id != tc.capability {
				t.Fatalf("target capability = %q, error = %v", id, err)
			}
		})
	}
}
