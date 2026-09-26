package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestListDeveloperServiceIDsUsesServicesFilterAndPreservesRawEnvelope(t *testing.T) {
	const raw = `{"data":[{"type":"bundleIds","id":"service-1","attributes":{"name":"Example Service","identifier":"com.example.service","platform":"SERVICES"}}],"links":{},"meta":{},"unknownTopLevel":{"keep":true}}`
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case developerPortalTeamsPath:
			if r.Method != http.MethodPost {
				t.Fatalf("bootstrap method = %s, want POST", r.Method)
			}
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
		case "/services-account/v1/bundleIds":
			if r.Method != http.MethodPost || r.Header.Get("X-HTTP-Method-Override") != http.MethodGet {
				t.Fatalf("collection transport = %s override=%q, want POST override GET", r.Method, r.Header.Get("X-HTTP-Method-Override"))
			}
			proxy := decodeDeveloperPortalProxyReadRequest(t, r)
			if proxy.URLEncodedQueryParams != "limit=1000&sort=name&filter[platform]=SERVICES" {
				t.Fatalf("collection query string = %q, want captured source order", proxy.URLEncodedQueryParams)
			}
			query, err := url.ParseQuery(proxy.URLEncodedQueryParams)
			if err != nil {
				t.Fatalf("collection query: %v", err)
			}
			if proxy.TeamID != "TEAM123456" || query.Get("filter[platform]") != "SERVICES" || query.Get("limit") != "1000" || query.Get("sort") != "name" {
				t.Fatalf("collection request = team %q query %q", proxy.TeamID, query.Encode())
			}
			return developerPortalTestResponse(http.StatusOK, raw, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.ListDeveloperServiceIDs(context.Background())
	if err != nil {
		t.Fatalf("ListDeveloperServiceIDs() error: %v", err)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "service-1" {
		t.Fatalf("unexpected result: %+v", result.Data)
	}
	if string(result.Raw) != raw {
		t.Fatalf("raw envelope changed: %s", result.Raw)
	}
}

func TestGetDeveloperServiceIDRejectsNonServicesPlatformBeforeMutation(t *testing.T) {
	var requests int
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/v1/bundleIds/service-1" || r.Header.Get("X-HTTP-Method-Override") != http.MethodGet {
				t.Fatalf("detail transport = %s %s override=%q", r.Method, r.URL.String(), r.Header.Get("X-HTTP-Method-Override"))
			}
			return developerPortalTestResponse(http.StatusOK, serviceIDDetailFixture("Example App", "IOS"), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requests, r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.GetDeveloperServiceID(context.Background(), "service-1")
	if err == nil || !strings.Contains(err.Error(), `want "SERVICES"`) {
		t.Fatalf("GetDeveloperServiceID() error = %v, want platform rejection", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want bootstrap and detail only", requests)
	}
}

func TestCreateDeveloperServiceIDUsesPrivatePayloadAndVerifies(t *testing.T) {
	var requests int
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/v1/bundleIds" || r.Header.Get("X-HTTP-Method-Override") != "" {
				t.Fatalf("create transport = %s %s override=%q", r.Method, r.URL.String(), r.Header.Get("X-HTTP-Method-Override"))
			}
			var payload struct {
				Data   developerServiceIDCreateData `json:"data"`
				TeamID *string                      `json:"teamId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode create payload: %v", err)
			}
			if payload.TeamID != nil {
				return developerPortalTestResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_UNPROCESSABLE","title":"Entity is valid json but is not a valid json:api document","detail":"Unrecognized field 'teamId'"}]}`, nil), nil
			}
			if payload.Data.Type != "bundleIds" {
				t.Fatalf("create envelope = %+v", payload)
			}
			want := map[string]string{
				"identifier": "com.example.service",
				"name":       "Example Service",
				"platform":   "SERVICES",
				"seedId":     "TEAM123456",
				"teamId":     "TEAM123456",
			}
			if !mapsEqual(payload.Data.Attributes, want) || len(payload.Data.Relationships.BundleIDCapabilities.Data) != 0 {
				t.Fatalf("create data = %+v", payload.Data)
			}
			return developerPortalTestResponse(http.StatusCreated, serviceIDDetailFixture("Example Service", "SERVICES"), nil), nil
		case 3:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/v1/bundleIds/service-1" || r.Header.Get("X-HTTP-Method-Override") != http.MethodGet {
				t.Fatalf("verification transport = %s %s override=%q", r.Method, r.URL.String(), r.Header.Get("X-HTTP-Method-Override"))
			}
			return developerPortalTestResponse(http.StatusOK, serviceIDDetailFixture("Example Service", "SERVICES"), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requests, r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.CreateDeveloperServiceID(context.Background(), DeveloperServiceIDCreateRequest{Identifier: "com.example.service", Name: "Example Service"})
	if err != nil {
		t.Fatalf("CreateDeveloperServiceID() error: %v", err)
	}
	if result.ServiceID != "service-1" || result.Status != "created" || !result.Verified {
		t.Fatalf("unexpected receipt: %+v", result)
	}
}

func TestRenameDeveloperServiceIDPreservesCapabilityGraph(t *testing.T) {
	var requests int
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, serviceIDDetailFixture("Old Name", "SERVICES"), nil), nil
		case 3:
			if r.Method != http.MethodPatch || r.URL.Path != "/services-account/v1/bundleIds/service-1" || r.Header.Get("X-HTTP-Method-Override") != "" {
				t.Fatalf("rename transport = %s %s override=%q", r.Method, r.URL.String(), r.Header.Get("X-HTTP-Method-Override"))
			}
			var payload developerBundleIDPatchRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode rename payload: %v", err)
			}
			var attrs map[string]json.RawMessage
			if err := json.Unmarshal(payload.Data.Attributes, &attrs); err != nil {
				t.Fatalf("decode rename attributes: %v", err)
			}
			var name, identifier, platform, teamID string
			_ = json.Unmarshal(attrs["name"], &name)
			_ = json.Unmarshal(attrs["identifier"], &identifier)
			_ = json.Unmarshal(attrs["platform"], &platform)
			_ = json.Unmarshal(attrs["teamId"], &teamID)
			if name != "New Name" || identifier != "com.example.service" || platform != "SERVICES" || teamID != "TEAM123456" {
				t.Fatalf("rename attrs = %s", payload.Data.Attributes)
			}
			if _, ok := attrs["~permissions.delete"]; ok {
				t.Fatal("rename payload included read-only delete permission")
			}
			if got := string(payload.Data.Relationships["bundleIdCapabilities"]); !strings.Contains(got, `"id":"cap-1"`) || !strings.Contains(got, `"id":"cap-2"`) {
				t.Fatalf("rename dropped capability graph: %s", got)
			}
			if got := string(payload.Data.Relationships["bundleIdCapabilities"]); !strings.Contains(got, `"meta":{"opaque":"keep"}`) {
				t.Fatalf("rename dropped opaque capability relationship members: %s", got)
			}
			return developerPortalTestResponse(http.StatusOK, `{}`, nil), nil
		case 4:
			return developerPortalTestResponse(http.StatusOK, serviceIDDetailFixture("New Name", "SERVICES"), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requests, r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.RenameDeveloperServiceID(context.Background(), DeveloperServiceIDRenameRequest{ServiceID: "service-1", Name: "New Name"})
	if err != nil {
		t.Fatalf("RenameDeveloperServiceID() error: %v", err)
	}
	if result.Status != "renamed" || !result.Verified || result.Identifier != "com.example.service" {
		t.Fatalf("unexpected receipt: %+v", result)
	}
}

func TestSetDeveloperServiceIDDomainsUpdatesOnlyAuthInputsAndVerifies(t *testing.T) {
	preflight := serviceIDDetailCapabilityGraphFixture(" Old Name ", false)
	postRead := strings.Replace(preflight, `"value":"old.example.com"`, `"value":"login.example.net"`, 1)
	postRead = strings.Replace(postRead, `"value":"https://old.example.com/callback"`, `"value":"https://login.example.net/callback"`, 1)
	var requests int
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/v1/bundleIds/service-1" || r.Header.Get("X-HTTP-Method-Override") != http.MethodGet {
				t.Fatalf("preflight transport = %s %s override=%q", r.Method, r.URL.String(), r.Header.Get("X-HTTP-Method-Override"))
			}
			return developerPortalTestResponse(http.StatusOK, preflight, nil), nil
		case 3:
			if r.Method != http.MethodPatch || r.URL.Path != "/services-account/v1/bundleIds/service-1" || r.Header.Get("X-HTTP-Method-Override") != "" {
				t.Fatalf("domains transport = %s %s override=%q", r.Method, r.URL.String(), r.Header.Get("X-HTTP-Method-Override"))
			}
			var payload developerBundleIDPatchRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode domains payload: %v", err)
			}
			if payload.Data.ID != "service-1" || payload.Data.Type != "bundleIds" {
				t.Fatalf("domains resource identity = %+v", payload.Data)
			}
			var parentAttributes map[string]json.RawMessage
			if err := json.Unmarshal(payload.Data.Attributes, &parentAttributes); err != nil {
				t.Fatal(err)
			}
			if string(parentAttributes["name"]) != `" Old Name "` {
				t.Fatalf("name changed: %s", parentAttributes["name"])
			}
			var capabilities struct {
				Data []developerResource `json:"data"`
			}
			if err := json.Unmarshal(payload.Data.Relationships["bundleIdCapabilities"], &capabilities); err != nil {
				t.Fatalf("decode capability replacement: %v", err)
			}
			if len(capabilities.Data) != 2 {
				t.Fatalf("capability count = %d, want both existing capabilities", len(capabilities.Data))
			}
			for _, capability := range capabilities.Data {
				var capabilityLink, appConsentLink struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				}
				if err := json.Unmarshal(capability.Relationships["capability"], &capabilityLink); err != nil {
					t.Fatalf("decode capability relationship: %v", err)
				}
				if err := json.Unmarshal(capability.Relationships["appConsentBundleId"], &appConsentLink); err != nil {
					t.Fatalf("decode primary app relationship: %v", err)
				}
				var attributes map[string]json.RawMessage
				if err := json.Unmarshal(capability.Attributes, &attributes); err != nil {
					t.Fatalf("decode capability attributes: %v", err)
				}
				switch capabilityLink.Data.ID {
				case "APPLE_ID_AUTH":
					var inputs []struct {
						Key    string `json:"key"`
						Values []struct {
							Value string `json:"value"`
						} `json:"values"`
					}
					if err := json.Unmarshal(attributes["inputs"], &inputs); err != nil {
						t.Fatalf("decode Apple ID Auth inputs: %v", err)
					}
					got := make(map[string][]string, len(inputs))
					for _, input := range inputs {
						for _, value := range input.Values {
							got[input.Key] = append(got[input.Key], value.Value)
						}
					}
					if !reflect.DeepEqual(got, map[string][]string{
						"APPLE_ID_AUTH_WEB_DOMAIN":     {"login.example.net"},
						"APPLE_ID_AUTH_WEB_RETURN_URL": {"https://login.example.net/callback"},
						"OTHER_AUTH_INPUT":             {"keep"},
					}) {
						t.Fatalf("Apple ID Auth inputs = %+v", got)
					}
					if string(attributes["settings"]) != `[{"key":"KEEP","value":"one"},{"key":"SECOND","value":"two"}]` {
						t.Fatalf("Apple ID Auth settings changed: %s", attributes["settings"])
					}
					var enabled bool
					if err := json.Unmarshal(attributes["enabled"], &enabled); err != nil || !enabled {
						t.Fatalf("Apple ID Auth enabled state = %s, err=%v", attributes["enabled"], err)
					}
					if appConsentLink.Data.ID != "consent-1" {
						t.Fatalf("primary app relationship changed: %+v", appConsentLink.Data)
					}
				case "PUSH_NOTIFICATIONS":
					if string(attributes["settings"]) != `[{"key":"OTHER","value":"two"}]` {
						t.Fatalf("unrelated capability settings changed: %s", attributes["settings"])
					}
					if string(attributes["inputs"]) != `[{"key":"UNRELATED_INPUT","values":[{"value":"preserve"}]}]` {
						t.Fatalf("unrelated capability inputs changed: %s", attributes["inputs"])
					}
					var enabled bool
					if err := json.Unmarshal(attributes["enabled"], &enabled); err != nil || enabled {
						t.Fatalf("unrelated capability enabled state = %s, err=%v", attributes["enabled"], err)
					}
				default:
					t.Fatalf("unexpected capability %q", capabilityLink.Data.ID)
				}
			}
			if _, ok := payload.Data.Relationships["profiles"]; !ok {
				t.Fatal("domains update dropped the existing profiles relationship")
			}
			return developerPortalTestResponse(http.StatusOK, `{}`, nil), nil
		case 4:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/v1/bundleIds/service-1" || r.Header.Get("X-HTTP-Method-Override") != http.MethodGet {
				t.Fatalf("post-write transport = %s %s override=%q", r.Method, r.URL.String(), r.Header.Get("X-HTTP-Method-Override"))
			}
			return developerPortalTestResponse(http.StatusOK, postRead, nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requests, r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.SetDeveloperServiceIDDomains(context.Background(), DeveloperServiceIDDomainsSetRequest{
		ServiceID:  "service-1",
		Domains:    []string{"login.example.net"},
		ReturnURLs: []string{"https://login.example.net/callback"},
	})
	if err != nil {
		t.Fatalf("SetDeveloperServiceIDDomains() error: %v", err)
	}
	if result.Operation != "domains-set" || result.Status != "updated" || !result.Changed || !result.Verified {
		t.Fatalf("unexpected receipt: %+v", result)
	}
	if requests != 4 {
		t.Fatalf("requests = %d, want bootstrap, preflight, PATCH, and post-write read", requests)
	}
}

func TestRenameDeveloperServiceIDRejectsPostWriteCapabilityGraphChanges(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(string) string
	}{
		{
			name: "dropped capability reference",
			mutate: func(body string) string {
				return strings.Replace(body, `,{"type":"bundleIdCapabilities","id":"cap-2"}`, "", 1)
			},
		},
		{
			name: "missing referenced capability details",
			mutate: func(body string) string {
				return strings.Replace(body, `"id":"cap-2","attributes"`, `"id":"cap-3","attributes"`, 1)
			},
		},
		{
			name: "changed enabled state",
			mutate: func(body string) string {
				return strings.Replace(body, `"enabled":false`, `"enabled":true`, 1)
			},
		},
		{
			name: "changed settings",
			mutate: func(body string) string {
				return strings.Replace(body, `"key":"OTHER","value":"two"`, `"key":"OTHER","value":"changed"`, 1)
			},
		},
		{
			name: "changed capability linkage",
			mutate: func(body string) string {
				return strings.Replace(body, `"id":"PUSH_NOTIFICATIONS"`, `"id":"APPLE_ID_AUTH"`, 1)
			},
		},
		{
			name: "changed app consent linkage",
			mutate: func(body string) string {
				return strings.Replace(body, `"id":"consent-2"`, `"id":"consent-3"`, 1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verified, requests, err := renameDeveloperServiceIDWithPostRead(t, tc.mutate(serviceIDDetailCapabilityGraphFixture("New Name", false)))
			var unverified *DeveloperServiceIDUnverifiedError
			if !errors.As(err, &unverified) {
				t.Fatalf("RenameDeveloperServiceID() error = %v, want unverified post-write outcome", err)
			}
			if verified {
				t.Fatal("RenameDeveloperServiceID() returned a verified receipt for a changed capability graph")
			}
			if requests != 4 {
				t.Fatalf("requests = %d, want bootstrap, preflight, PATCH, and post-write read", requests)
			}
		})
	}
}

func TestRenameDeveloperServiceIDAcceptsReorderedCapabilityGraph(t *testing.T) {
	verified, requests, err := renameDeveloperServiceIDWithPostRead(t, serviceIDDetailCapabilityGraphFixture("New Name", true))
	if err != nil {
		t.Fatalf("RenameDeveloperServiceID() error: %v", err)
	}
	if !verified {
		t.Fatal("RenameDeveloperServiceID() returned an unverified receipt for an equivalent reordered capability graph")
	}
	if requests != 4 {
		t.Fatalf("requests = %d, want bootstrap, preflight, PATCH, and post-write read", requests)
	}
}

func renameDeveloperServiceIDWithPostRead(t *testing.T, postReadBody string) (bool, int, error) {
	return renameDeveloperServiceIDWithResponses(t, serviceIDDetailCapabilityGraphFixture("Old Name", false), postReadBody)
}

func TestRenameDeveloperServiceIDRejectsIncompletePreWriteCapabilityGraphBeforeMutation(t *testing.T) {
	preflight := strings.Replace(serviceIDDetailCapabilityGraphFixture("Old Name", false), `"id":"cap-2","attributes"`, `"id":"cap-3","attributes"`, 1)
	verified, requests, err := renameDeveloperServiceIDWithResponses(t, preflight, serviceIDDetailCapabilityGraphFixture("New Name", false))
	if err == nil || !strings.Contains(err.Error(), "included capability") {
		t.Fatalf("RenameDeveloperServiceID() error = %v, want pre-write capability graph rejection", err)
	}
	if verified {
		t.Fatal("RenameDeveloperServiceID() returned a verified receipt after rejecting its pre-write graph")
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want bootstrap and preflight only", requests)
	}
}

func renameDeveloperServiceIDWithResponses(t *testing.T, preflightBody, postReadBody string) (bool, int, error) {
	t.Helper()
	var requests int
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/v1/bundleIds/service-1" || r.Header.Get("X-HTTP-Method-Override") != http.MethodGet {
				t.Fatalf("preflight transport = %s %s override=%q", r.Method, r.URL.String(), r.Header.Get("X-HTTP-Method-Override"))
			}
			return developerPortalTestResponse(http.StatusOK, preflightBody, nil), nil
		case 3:
			if r.Method != http.MethodPatch || r.URL.Path != "/services-account/v1/bundleIds/service-1" || r.Header.Get("X-HTTP-Method-Override") != "" {
				t.Fatalf("rename transport = %s %s override=%q", r.Method, r.URL.String(), r.Header.Get("X-HTTP-Method-Override"))
			}
			var payload developerBundleIDPatchRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode rename payload: %v", err)
			}
			var relationship struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
				Meta map[string]any `json:"meta"`
			}
			if err := json.Unmarshal(payload.Data.Relationships["bundleIdCapabilities"], &relationship); err != nil {
				t.Fatalf("decode capability relationship: %v", err)
			}
			if len(relationship.Data) != 2 || relationship.Data[0].ID != "cap-1" || relationship.Data[1].ID != "cap-2" {
				t.Fatalf("rename payload lost capability references: %+v", relationship.Data)
			}
			if relationship.Meta["opaque"] != "keep" {
				t.Fatalf("rename payload lost opaque capability relationship metadata: %+v", relationship.Meta)
			}
			return developerPortalTestResponse(http.StatusOK, `{}`, nil), nil
		case 4:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/v1/bundleIds/service-1" || r.Header.Get("X-HTTP-Method-Override") != http.MethodGet {
				t.Fatalf("post-write transport = %s %s override=%q", r.Method, r.URL.String(), r.Header.Get("X-HTTP-Method-Override"))
			}
			return developerPortalTestResponse(http.StatusOK, postReadBody, nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requests, r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.RenameDeveloperServiceID(context.Background(), DeveloperServiceIDRenameRequest{ServiceID: "service-1", Name: "New Name"})
	return result != nil && result.Verified, requests, err
}

func TestRenameDeveloperServiceIDRejectsIncompleteIdentityBeforeMutation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		wantErr string
	}{
		{
			name: "missing identifier",
			mutate: func(body string) string {
				return strings.Replace(body, `"identifier":"com.example.service",`, "", 1)
			},
			wantErr: "missing its identifier attribute",
		},
		{
			name: "non-string name",
			mutate: func(body string) string {
				return strings.Replace(body, `"name":"Old Name"`, `"name":123`, 1)
			},
			wantErr: "non-string name attribute",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var requests int
			client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
				requests++
				switch requests {
				case 1:
					return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
				case 2:
					body := tc.mutate(serviceIDDetailFixture("Old Name", "SERVICES"))
					return developerPortalTestResponse(http.StatusOK, body, nil), nil
				default:
					t.Fatalf("unexpected mutation request %d: %s %s", requests, r.Method, r.URL.String())
					return nil, nil
				}
			})

			_, err := client.RenameDeveloperServiceID(context.Background(), DeveloperServiceIDRenameRequest{ServiceID: "service-1", Name: "New Name"})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("RenameDeveloperServiceID() error = %v, want %q", err, tc.wantErr)
			}
			if requests != 2 {
				t.Fatalf("requests = %d, want bootstrap and preflight only", requests)
			}
		})
	}
}

func TestRenameDeveloperServiceIDPreservesValidEmptyCapabilityRelationship(t *testing.T) {
	const relationship = `{"bundleIdCapabilities":{"data":[],"meta":{"opaque":"keep"}}}`
	var requests int
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, serviceIDDetailFixtureWithRelationships("Old Name", "SERVICES", relationship), nil), nil
		case 3:
			var payload developerBundleIDPatchRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode rename payload: %v", err)
			}
			if got := string(payload.Data.Relationships["bundleIdCapabilities"]); got != `{"data":[],"meta":{"opaque":"keep"}}` {
				t.Fatalf("rename changed valid empty capability relationship: %s", got)
			}
			return developerPortalTestResponse(http.StatusOK, `{}`, nil), nil
		case 4:
			return developerPortalTestResponse(http.StatusOK, serviceIDDetailFixtureWithRelationships("New Name", "SERVICES", relationship), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requests, r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.RenameDeveloperServiceID(context.Background(), DeveloperServiceIDRenameRequest{ServiceID: "service-1", Name: "New Name"})
	if err != nil {
		t.Fatalf("RenameDeveloperServiceID() error: %v", err)
	}
	if result.Status != "renamed" || !result.Verified {
		t.Fatalf("unexpected receipt: %+v", result)
	}
}

func TestRenameDeveloperServiceIDRejectsIncompleteCapabilityRelationshipBeforeMutation(t *testing.T) {
	tests := []struct {
		name          string
		relationships string
	}{
		{name: "missing", relationships: `{}`},
		{name: "null data", relationships: `{"bundleIdCapabilities":{"data":null}}`},
		{name: "object data", relationships: `{"bundleIdCapabilities":{"data":{"type":"bundleIdCapabilities","id":"cap-1"}}}`},
		{name: "wrong type", relationships: `{"bundleIdCapabilities":{"data":[{"type":"capabilities","id":"cap-1"}]}}`},
		{name: "missing id", relationships: `{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities"}]}}`},
		{name: "non-string id", relationships: `{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":42}]}}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var requests int
			client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
				requests++
				switch requests {
				case 1:
					return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
				case 2:
					return developerPortalTestResponse(http.StatusOK, serviceIDDetailFixtureWithRelationships("Old Name", "SERVICES", tc.relationships), nil), nil
				default:
					t.Fatalf("unexpected mutation request %d: %s %s", requests, r.Method, r.URL.String())
					return nil, nil
				}
			})

			_, err := client.RenameDeveloperServiceID(context.Background(), DeveloperServiceIDRenameRequest{ServiceID: "service-1", Name: "New Name"})
			if err == nil {
				t.Fatal("RenameDeveloperServiceID() unexpectedly accepted incomplete capability relationship")
			}
			if requests != 2 {
				t.Fatalf("requests = %d, want bootstrap and preflight only", requests)
			}
		})
	}
}

func TestDeleteDeveloperServiceIDUsesLogicalDeleteAndVerifies404(t *testing.T) {
	var requests int
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, serviceIDDetailFixture("Example Service", "SERVICES"), nil), nil
		case 3:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/v1/bundleIds/service-1" || r.Header.Get("X-HTTP-Method-Override") != http.MethodDelete {
				t.Fatalf("delete transport = %s %s override=%q", r.Method, r.URL.String(), r.Header.Get("X-HTTP-Method-Override"))
			}
			var body struct {
				TeamID string `json:"teamId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode delete body: %v", err)
			}
			if body.TeamID != "TEAM123456" {
				return developerPortalTestResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","detail":"Please select a team."}]}`, nil), nil
			}
			return developerPortalTestResponse(http.StatusOK, `{}`, nil), nil
		case 4:
			return developerPortalTestResponse(http.StatusNotFound, `{"errors":[{"code":"NOT_FOUND"}]}`, nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requests, r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.DeleteDeveloperServiceID(context.Background(), DeveloperServiceIDDeleteRequest{ServiceID: "service-1"})
	if err != nil {
		t.Fatalf("DeleteDeveloperServiceID() error: %v", err)
	}
	if result.Status != "deleted" || !result.Verified || !result.Changed {
		t.Fatalf("unexpected receipt: %+v", result)
	}
}

func TestRenameDeveloperServiceIDMarksAmbiguousHTTPAsUnknownWithoutRetry(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusRequestTimeout} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requests int
			client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
				requests++
				switch requests {
				case 1:
					return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
				case 2:
					return developerPortalTestResponse(http.StatusOK, serviceIDDetailFixture("Old Name", "SERVICES"), nil), nil
				case 3:
					return developerPortalTestResponse(status, `{}`, nil), nil
				default:
					t.Fatalf("unexpected retry/request %d: %s %s", requests, r.Method, r.URL.String())
					return nil, nil
				}
			})

			_, err := client.RenameDeveloperServiceID(context.Background(), DeveloperServiceIDRenameRequest{ServiceID: "service-1", Name: "New Name"})
			var unverified *DeveloperServiceIDUnverifiedError
			if !errors.As(err, &unverified) || !strings.Contains(err.Error(), "unknown") {
				t.Fatalf("RenameDeveloperServiceID() error = %v, want unknown write error", err)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Status != status {
				t.Fatalf("error = %v, want wrapped HTTP %d", err, status)
			}
			if requests != 3 {
				t.Fatalf("requests = %d, want no automatic retry or post-read after an ambiguous HTTP response", requests)
			}
		})
	}
}

func serviceIDDetailFixture(name, platform string) string {
	return serviceIDDetailFixtureWithRelationships(name, platform, `{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":"cap-1"},{"type":"bundleIdCapabilities","id":"cap-2"}],"meta":{"opaque":"keep"}}}`)
}

func serviceIDDetailFixtureWithRelationships(name, platform, relationships string) string {
	return `{"data":{"id":"service-1","type":"bundleIds","attributes":{"name":"` + name + `","identifier":"com.example.service","platform":"` + platform + `","seedId":"TEAM123456","~permissions.delete":true,"~permissions.edit":true},"relationships":` + relationships + `},"included":[{"type":"bundleIdCapabilities","id":"cap-1","attributes":{"enabled":true,"settings":[{"key":"KEEP","value":"one"}]},"relationships":{"capability":{"data":{"type":"capabilities","id":"APPLE_ID_AUTH"}}}},{"type":"bundleIdCapabilities","id":"cap-2","attributes":{"enabled":false,"settings":[]},"relationships":{"capability":{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"}}}}]}`
}

func serviceIDDetailCapabilityGraphFixture(name string, reverse bool) string {
	marker := "preflight"
	references := `[{"type":"bundleIdCapabilities","id":"cap-1"},{"type":"bundleIdCapabilities","id":"cap-2"}]`
	capabilityOne := `{"type":"bundleIdCapabilities","id":"cap-1","attributes":{"enabled":true,"settings":[{"key":"KEEP","value":"one"},{"key":"SECOND","value":"two"}],"inputs":[{"key":"APPLE_ID_AUTH_WEB_DOMAIN","values":[{"value":"old.example.com"}]},{"key":"APPLE_ID_AUTH_WEB_RETURN_URL","values":[{"value":"https://old.example.com/callback"}]},{"key":"OTHER_AUTH_INPUT","values":[{"value":"keep"}]}],"providerOpaque":"stable-one"},"relationships":{"capability":{"data":{"type":"capabilities","id":"APPLE_ID_AUTH"},"links":{"related":"/capability/preflight"},"meta":{"request":"preflight"}},"appConsentBundleId":{"data":{"type":"bundleIds","id":"consent-1"},"links":{"related":"/consent/preflight"},"meta":{"request":"preflight"}}},"links":{"self":"/bundleIdCapabilities/cap-1/preflight"},"meta":{"request":"preflight"}}`
	capabilityTwo := `{"type":"bundleIdCapabilities","id":"cap-2","attributes":{"enabled":false,"settings":[{"key":"OTHER","value":"two"}],"inputs":[{"key":"UNRELATED_INPUT","values":[{"value":"preserve"}]}],"providerOpaque":"stable-two"},"relationships":{"capability":{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"},"links":{"related":"/capability/preflight"},"meta":{"request":"preflight"}},"appConsentBundleId":{"data":{"type":"bundleIds","id":"consent-2"},"links":{"related":"/consent/preflight"},"meta":{"request":"preflight"}}},"links":{"self":"/bundleIdCapabilities/cap-2/preflight"},"meta":{"request":"preflight"}}`
	included := "[" + capabilityOne + "," + capabilityTwo + "]"
	if reverse {
		marker = "postwrite"
		references = `[{"type":"bundleIdCapabilities","id":"cap-2"},{"type":"bundleIdCapabilities","id":"cap-1"}]`
		capabilityOne = strings.ReplaceAll(capabilityOne, "preflight", marker)
		capabilityTwo = strings.ReplaceAll(capabilityTwo, "preflight", marker)
		capabilityOne = strings.Replace(capabilityOne, `"settings":[{"key":"KEEP","value":"one"},{"key":"SECOND","value":"two"}]`, `"settings":[{"key":"SECOND","value":"two"},{"key":"KEEP","value":"one"}]`, 1)
		included = "[" + capabilityTwo + "," + capabilityOne + "]"
	}
	return `{"links":{"self":"/bundleIds/service-1/` + marker + `"},"meta":{"request":"` + marker + `"},"data":{"id":"service-1","type":"bundleIds","attributes":{"name":"` + name + `","identifier":"com.example.service","platform":"SERVICES","seedId":"TEAM123456","teamId":"TEAM123456","~permissions.delete":true,"~permissions.edit":true},"relationships":{"bundleIdCapabilities":{"data":` + references + `,"links":{"self":"/relationships/` + marker + `"},"meta":{"opaque":"keep","request":"` + marker + `","paging":{"total":0,"limit":2147483647}}},"profiles":{"data":[],"links":{"self":"/profiles/` + marker + `"},"meta":{"opaque":"profiles"}}}},"included":` + included + `}`
}

func mapsEqual(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for key, value := range want {
		if got[key] != value {
			return false
		}
	}
	return true
}

func TestSetDeveloperServiceIDDomainsRejectsUnsafePreflight(t *testing.T) {
	for _, tc := range []struct {
		name        string
		old         string
		replacement string
	}{
		{"disabled", "\"enabled\":true", "\"enabled\":false"},
		{"null input value", `"value":"preserve"`, `"value":null`},
		{"unknown value field", `{"value":"preserve"}`, `{"value":"preserve","opaque":true}`},
		{"missing primary", "\"id\":\"consent-1\"", "\"id\":\"\""},
		{"partial graph", "\"total\":0", "\"total\":3"},
		{"unrecognized zero total", "\"limit\":2147483647", "\"limit\":50"},
		{"next page", "\"self\":\"/relationships/preflight\"", "\"next\":\"/next\""},
		{"missing included", "\"id\":\"cap-2\",\"attributes\"", "\"id\":\"cap-3\",\"attributes\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			body := strings.Replace(serviceIDDetailCapabilityGraphFixture("Service", false), tc.old, tc.replacement, 1)
			client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return developerPortalTestResponse(200, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
				}
				if calls != 2 || r.Method != http.MethodPost {
					t.Fatalf("unexpected write: %s %s", r.Method, r.URL)
				}
				return developerPortalTestResponse(200, body, nil), nil
			})
			_, err := client.SetDeveloperServiceIDDomains(context.Background(), DeveloperServiceIDDomainsSetRequest{ServiceID: "service-1", Domains: []string{"example.com"}, ReturnURLs: []string{"https://example.com/cb"}})
			if err == nil || calls != 2 {
				t.Fatalf("error=%v calls=%d", err, calls)
			}
		})
	}
}

func TestSetDeveloperServiceIDDomainsNoOpAndUnverifiedWrites(t *testing.T) {
	for _, mode := range []string{"unchanged", "stale readback", "server failure", "readback failure"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			writes := 0
			body := serviceIDDetailCapabilityGraphFixture("Service", false)
			client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return developerPortalTestResponse(200, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
				}
				if r.Method == http.MethodPatch {
					writes++
					if mode == "server failure" {
						return developerPortalTestResponse(500, `{"errors":[{"detail":"failed"}]}`, nil), nil
					}
					return developerPortalTestResponse(200, `{}`, nil), nil
				}
				if calls > 2 && mode == "readback failure" {
					return developerPortalTestResponse(403, `{"errors":[{"detail":"denied"}]}`, nil), nil
				}
				return developerPortalTestResponse(200, body, nil), nil
			})
			domain := "example.com"
			callback := "https://example.com/cb"
			if mode == "unchanged" {
				domain = "old.example.com"
				callback = "https://old.example.com/callback"
			}
			result, err := client.SetDeveloperServiceIDDomains(context.Background(), DeveloperServiceIDDomainsSetRequest{ServiceID: "service-1", Domains: []string{domain}, ReturnURLs: []string{callback}})
			if mode == "unchanged" {
				if err != nil || result.Changed || !result.Verified || writes != 0 {
					t.Fatalf("result=%+v err=%v writes=%d", result, err, writes)
				}
				return
			}
			var unknown *DeveloperServiceIDUnverifiedError
			if !errors.As(err, &unknown) || result != nil || writes != 1 {
				t.Fatalf("result=%+v err=%v writes=%d", result, err, writes)
			}
		})
	}
}
