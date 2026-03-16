package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestClientDoIrisV1RequestUsesIntegrationsHeaders(t *testing.T) {
	var (
		gotPath    string
		gotAccept  string
		gotCSRF    string
		gotOrigin  string
		gotReferer string
	)
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotPath = r.URL.Path + "?" + r.URL.RawQuery
			gotAccept = r.Header.Get("Accept")
			gotCSRF = r.Header.Get("X-CSRF-ITC")
			gotOrigin = r.Header.Get("Origin")
			gotReferer = r.Header.Get("Referer")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
			}, nil
		})},
	}

	if _, err := client.doIrisV1Request(context.Background(), http.MethodGet, "/apiKeys?limit=2000", nil); err != nil {
		t.Fatalf("doIrisV1Request() error: %v", err)
	}

	if gotPath != "/iris/v1/apiKeys?limit=2000" {
		t.Fatalf("expected path %q, got %q", "/iris/v1/apiKeys?limit=2000", gotPath)
	}
	if gotAccept != "application/vnd.api+json, application/json, text/csv" {
		t.Fatalf("unexpected accept header %q", gotAccept)
	}
	if gotCSRF != "[asc-ui]" {
		t.Fatalf("unexpected csrf header %q", gotCSRF)
	}
	if gotOrigin != appStoreBaseURL {
		t.Fatalf("unexpected origin header %q", gotOrigin)
	}
	if gotReferer != integrationsAPIRefererURL {
		t.Fatalf("unexpected referer header %q", gotReferer)
	}
}

func TestClientDoOlympusRequestUsesOlympusHeaders(t *testing.T) {
	var (
		gotPath      string
		gotRequested string
		gotAccept    string
	)
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotPath = r.URL.Path
			gotRequested = r.Header.Get("X-Requested-With")
			gotAccept = r.Header.Get("Accept")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
			}, nil
		})},
	}

	if _, err := client.doOlympusRequest(context.Background(), http.MethodGet, "/actors/actor-1", nil); err != nil {
		t.Fatalf("doOlympusRequest() error: %v", err)
	}

	if gotPath != "/olympus/v1/actors/actor-1" {
		t.Fatalf("expected path %q, got %q", "/olympus/v1/actors/actor-1", gotPath)
	}
	if gotRequested != "xsdr2$" {
		t.Fatalf("unexpected X-Requested-With %q", gotRequested)
	}
	if gotAccept != "application/json" {
		t.Fatalf("unexpected accept header %q", gotAccept)
	}
}

func TestClientListTeamKeysParsesRolesAndActors(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"data":[
						{
							"id":"39MX87M9Y4",
							"attributes":{
								"lastUsed":"2026-03-15T11:48:57.844-07:00",
								"roles":["APP_MANAGER"],
								"nickname":"asc_cli",
								"isActive":true,
								"keyType":"PUBLIC_API"
							},
							"relationships":{
								"createdBy":{"data":{"id":"user-1"}},
								"revokedBy":{"data":null}
							}
						},
						{
							"id":"8P3JQ8PBFJ",
							"attributes":{
								"lastUsed":"",
								"roles":["APP_MANAGER","FINANCE"],
								"nickname":"codex-probe-team-1",
								"isActive":false,
								"keyType":"PUBLIC_API"
							},
							"relationships":{
								"createdBy":{"data":{"id":"user-1"}},
								"revokedBy":{"data":{"id":"user-2"}}
							}
						}
					],
					"included":[
						{"type":"users","id":"user-1","attributes":{"firstName":"Mithilesh","lastName":"Chellappan"}},
						{"type":"users","id":"user-2","attributes":{"firstName":"Jane","lastName":"Admin"}}
					]
				}`)),
			}, nil
		})},
	}

	keys, err := client.listTeamKeys(context.Background())
	if err != nil {
		t.Fatalf("listTeamKeys() error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0].KeyID != "39MX87M9Y4" || keys[0].Name != "asc_cli" {
		t.Fatalf("unexpected first key: %#v", keys[0])
	}
	if len(keys[0].Roles) != 1 || keys[0].Roles[0] != "APP_MANAGER" {
		t.Fatalf("unexpected first key roles: %#v", keys[0].Roles)
	}
	if keys[0].GeneratedBy == nil || keys[0].GeneratedBy.Name != "Mithilesh Chellappan" {
		t.Fatalf("unexpected generatedBy: %#v", keys[0].GeneratedBy)
	}
	if keys[1].Active {
		t.Fatalf("expected revoked key to be inactive: %#v", keys[1])
	}
	if keys[1].RevokedBy == nil || keys[1].RevokedBy.Name != "Jane Admin" {
		t.Fatalf("unexpected revokedBy: %#v", keys[1].RevokedBy)
	}
}

func TestClientLookupAPIKeyRolesReturnsTeamMatch(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"data":[
						{
							"id":"39MX87M9Y4",
							"attributes":{
								"lastUsed":"2026-03-15T11:48:57.844-07:00",
								"roles":["APP_MANAGER"],
								"nickname":"asc_cli",
								"isActive":true,
								"keyType":"PUBLIC_API"
							},
							"relationships":{
								"createdBy":{"data":{"id":"user-1"}},
								"revokedBy":{"data":null}
							}
						}
					],
					"included":[
						{"type":"users","id":"user-1","attributes":{"firstName":"Mithilesh","lastName":"Chellappan"}}
					]
				}`)),
			}, nil
		})},
	}

	got, err := client.LookupAPIKeyRoles(context.Background(), "39MX87M9Y4")
	if err != nil {
		t.Fatalf("LookupAPIKeyRoles() error: %v", err)
	}
	if got.Kind != "team" || got.Lookup != "team_keys" || got.RoleSource != "key" {
		t.Fatalf("unexpected lookup metadata: %#v", got)
	}
	if len(got.Roles) != 1 || got.Roles[0] != "APP_MANAGER" {
		t.Fatalf("unexpected roles: %#v", got.Roles)
	}
}

func TestClientLookupAPIKeyRolesReturnsTeamMatchFromLaterPage(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/iris/v1/apiKeys":
				if r.URL.Query().Get("cursor") == "page-2" {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body: io.NopCloser(strings.NewReader(`{
							"data":[
								{
									"id":"39MX87M9Y4",
									"attributes":{
										"roles":["APP_MANAGER"],
										"nickname":"late-team-key",
										"isActive":true,
										"keyType":"PUBLIC_API"
									},
									"relationships":{
										"createdBy":{"data":{"id":"user-2"}},
										"revokedBy":{"data":null}
									}
								}
							],
							"included":[
								{"type":"users","id":"user-2","attributes":{"firstName":"Jane","lastName":"Admin"}}
							]
						}`)),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[],
						"included":[],
						"links":{"next":"https://appstoreconnect.apple.com/iris/v1/apiKeys?cursor=page-2"}
					}`)),
				}, nil
			case "/iris/v2/apiKeys":
				t.Fatalf("did not expect individual key lookup when team pagination should find the key")
				return nil, nil
			default:
				t.Fatalf("unexpected request path %q", r.URL.String())
				return nil, nil
			}
		})},
	}

	got, err := client.LookupAPIKeyRoles(context.Background(), "39MX87M9Y4")
	if err != nil {
		t.Fatalf("LookupAPIKeyRoles() error: %v", err)
	}
	if got.Kind != "team" || got.Lookup != "team_keys" {
		t.Fatalf("unexpected lookup metadata: %#v", got)
	}
	if got.GeneratedBy == nil || got.GeneratedBy.Name != "Jane Admin" {
		t.Fatalf("unexpected generatedBy: %#v", got.GeneratedBy)
	}
}

func TestClientLookupAPIKeyRolesReturnsNotFound(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"data":[],"included":[]}`)),
			}, nil
		})},
	}

	_, err := client.LookupAPIKeyRoles(context.Background(), "missing")
	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("expected ErrAPIKeyNotFound, got %v", err)
	}
}

func TestClientLookupAPIKeyRolesFallsBackToIndividualKeysWhenTeamListForbidden(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/iris/v1/apiKeys":
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"403"}]}`)),
					Request:    r,
				}, nil
			case "/iris/v2/apiKeys":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[
							{
								"id":"ind-1",
								"attributes":{
									"lastUsed":"2026-03-16T00:00:00Z",
									"roles":["ADMIN"],
									"nickname":"personal-key",
									"isActive":true,
									"keyType":"PUBLIC_API"
								},
								"relationships":{
									"createdByActor":{"data":{"id":"actor-1"}},
									"revokedByActor":{"data":null}
								}
							}
						]
					}`)),
					Request: r,
				}, nil
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
				return nil, nil
			}
		})},
	}

	got, err := client.LookupAPIKeyRoles(context.Background(), "ind-1")
	if err != nil {
		t.Fatalf("LookupAPIKeyRoles() error: %v", err)
	}
	if got.Kind != "individual" || got.Lookup != "individual_keys" || got.RoleSource != "key" {
		t.Fatalf("unexpected lookup metadata: %#v", got)
	}
	if len(got.Roles) != 1 || got.Roles[0] != "ADMIN" {
		t.Fatalf("unexpected roles: %#v", got.Roles)
	}
}

func TestClientLookupAPIKeyRolesReturnsTeamListErrorWhenFallbackMisses(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/iris/v1/apiKeys":
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"403"}]}`)),
					Request:    r,
				}, nil
			case "/iris/v2/apiKeys":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
					Request:    r,
				}, nil
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
				return nil, nil
			}
		})},
	}

	_, err := client.LookupAPIKeyRoles(context.Background(), "missing")
	if !errors.Is(err, ErrAPIKeyNotVisible) {
		t.Fatalf("expected ErrAPIKeyNotVisible, got %v", err)
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected key id in error, got %v", err)
	}
}

func TestClientLookupAPIKeyRolesReturnsNotVisibleWhenIndividualListForbiddenAfterTeamMiss(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/iris/v1/apiKeys":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"included":[]}`)),
					Request:    r,
				}, nil
			case "/iris/v2/apiKeys":
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"403"}]}`)),
					Request:    r,
				}, nil
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
				return nil, nil
			}
		})},
	}

	_, err := client.LookupAPIKeyRoles(context.Background(), "missing")
	if !errors.Is(err, ErrAPIKeyNotVisible) {
		t.Fatalf("expected ErrAPIKeyNotVisible, got %v", err)
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected key id in error, got %v", err)
	}
}

func TestClientListIndividualKeysParsesActors(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"data":[
						{
							"id":"ind-1",
							"attributes":{
								"lastUsed":"2026-03-16T00:00:00Z",
								"roles":[],
								"nickname":"personal-key",
								"isActive":true,
								"keyType":"PUBLIC_API"
							},
							"relationships":{
								"createdByActor":{"data":{"id":"actor-1"}},
								"revokedByActor":{"data":null}
							}
						}
					]
				}`)),
			}, nil
		})},
	}

	keys, err := client.listIndividualKeys(context.Background())
	if err != nil {
		t.Fatalf("listIndividualKeys() error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if keys[0].KeyID != "ind-1" || keys[0].CreatedByActorID != "actor-1" {
		t.Fatalf("unexpected individual key: %#v", keys[0])
	}
	if keys[0].Name != "personal-key" {
		t.Fatalf("expected nickname to populate name, got %#v", keys[0])
	}
}

func TestClientGetActorParsesRolesAndName(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.Path + "?" + r.URL.RawQuery; got != "/olympus/v1/actors/actor-1?include=provider,person" {
				t.Fatalf("unexpected actor path %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"data":{
						"id":"actor-1",
						"attributes":{"roles":["ADMIN","CIPS"]},
						"relationships":{
							"provider":{"data":{"id":"prov-1"}},
							"person":{"data":{"id":"person-1"}}
						}
					},
					"included":[
						{"type":"people","id":"person-1","attributes":{"firstName":"Mithilesh","lastName":"Chellappan"}},
						{"type":"providers","id":"prov-1","attributes":{"name":"Mithilesh Chellappan"}}
					]
				}`)),
			}, nil
		})},
	}

	actor, err := client.getActor(context.Background(), "actor-1")
	if err != nil {
		t.Fatalf("getActor() error: %v", err)
	}
	if actor.ID != "actor-1" || actor.Name != "Mithilesh Chellappan" {
		t.Fatalf("unexpected actor: %#v", actor)
	}
	if len(actor.Roles) != 2 || actor.Roles[0] != "ADMIN" {
		t.Fatalf("unexpected actor roles: %#v", actor.Roles)
	}
}

func TestClientListActorsParsesIncludedNames(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"data":[
						{
							"id":"actor-1",
							"attributes":{"roles":["ADMIN"]},
							"relationships":{
								"provider":{"data":{"id":"prov-1"}},
								"person":{"data":{"id":"person-1"}}
							}
						}
					],
					"included":[
						{"type":"people","id":"person-1","attributes":{"firstName":"Mithilesh","lastName":"Chellappan"}},
						{"type":"providers","id":"prov-1","attributes":{"name":"Ignored Provider"}}
					]
				}`)),
			}, nil
		})},
	}

	actors, err := client.listActors(context.Background())
	if err != nil {
		t.Fatalf("listActors() error: %v", err)
	}
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(actors))
	}
	if actors[0].Name != "Mithilesh Chellappan" {
		t.Fatalf("unexpected actor name: %#v", actors[0])
	}
}

func TestClientLookupAPIKeyRolesReturnsIndividualMatchFromKey(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Path, "/iris/v1/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"included":[]}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/iris/v2/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[
							{
								"id":"ind-1",
								"attributes":{
									"roles":["ADMIN"],
									"nickname":"individual-key",
									"isActive":true,
									"keyType":"PUBLIC_API"
								},
								"relationships":{
									"createdByActor":{"data":{"id":"actor-1"}},
									"revokedByActor":{"data":null}
								}
							}
						]
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected request URL %q", r.URL.String())
				return nil, nil
			}
		})},
	}

	got, err := client.LookupAPIKeyRoles(context.Background(), "ind-1")
	if err != nil {
		t.Fatalf("LookupAPIKeyRoles() error: %v", err)
	}
	if got.Kind != "individual" || got.RoleSource != "key" {
		t.Fatalf("unexpected lookup metadata: %#v", got)
	}
	if len(got.Roles) != 1 || got.Roles[0] != "ADMIN" {
		t.Fatalf("unexpected roles: %#v", got.Roles)
	}
	if got.GeneratedBy == nil || got.GeneratedBy.ID != "actor-1" {
		t.Fatalf("unexpected generatedBy: %#v", got.GeneratedBy)
	}
}

func TestClientLookupAPIKeyRolesReturnsIndividualMatchFromLaterPage(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/iris/v1/apiKeys":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"included":[]}`)),
				}, nil
			case "/iris/v2/apiKeys":
				if r.URL.Query().Get("cursor") == "page-2" {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body: io.NopCloser(strings.NewReader(`{
							"data":[
								{
									"id":"ind-late",
									"attributes":{
										"roles":["ADMIN"],
										"nickname":"late-individual-key",
										"isActive":true,
										"keyType":"PUBLIC_API"
									},
									"relationships":{
										"createdByActor":{"data":{"id":"actor-1"}},
										"revokedByActor":{"data":null}
									}
								}
							]
						}`)),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[],
						"links":{"next":"https://appstoreconnect.apple.com/iris/v2/apiKeys?cursor=page-2"}
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected request URL %q", r.URL.String())
				return nil, nil
			}
		})},
	}

	got, err := client.LookupAPIKeyRoles(context.Background(), "ind-late")
	if err != nil {
		t.Fatalf("LookupAPIKeyRoles() error: %v", err)
	}
	if got.Kind != "individual" || got.Lookup != "individual_keys" || got.RoleSource != "key" {
		t.Fatalf("unexpected lookup metadata: %#v", got)
	}
	if got.Name != "late-individual-key" {
		t.Fatalf("unexpected key payload: %#v", got)
	}
}

func TestClientLookupAPIKeyRolesFallsBackToActorRoles(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Path, "/iris/v1/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"included":[]}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/iris/v2/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[
							{
								"id":"ind-2",
								"attributes":{
									"roles":[],
									"nickname":"individual-key",
									"isActive":true,
									"keyType":"PUBLIC_API"
								},
								"relationships":{
									"createdByActor":{"data":{"id":"actor-1"}},
									"revokedByActor":{"data":{"id":"actor-2"}}
								}
							}
						]
					}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors/actor-1"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":{
							"id":"actor-1",
							"attributes":{"roles":["ADMIN","CIPS"]},
							"relationships":{
								"provider":{"data":{"id":"prov-1"}},
								"person":{"data":{"id":"person-1"}}
							}
						},
						"included":[
							{"type":"people","id":"person-1","attributes":{"firstName":"Mithilesh","lastName":"Chellappan"}},
							{"type":"providers","id":"prov-1","attributes":{"name":"Ignored Provider"}}
						]
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected request URL %q", r.URL.String())
				return nil, nil
			}
		})},
	}

	got, err := client.LookupAPIKeyRoles(context.Background(), "ind-2")
	if err != nil {
		t.Fatalf("LookupAPIKeyRoles() error: %v", err)
	}
	if got.RoleSource != "actor" {
		t.Fatalf("expected actor role source, got %#v", got)
	}
	if len(got.Roles) != 2 || got.Roles[0] != "ADMIN" {
		t.Fatalf("unexpected actor roles: %#v", got.Roles)
	}
	if got.GeneratedBy == nil || got.GeneratedBy.Name != "Mithilesh Chellappan" {
		t.Fatalf("unexpected generatedBy: %#v", got.GeneratedBy)
	}
}

func TestClientLookupAPIKeyRolesFallsBackToActorList(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Path, "/iris/v1/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"included":[]}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/iris/v2/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[
							{
								"id":"ind-3",
								"attributes":{"roles":[],"nickname":"individual-key","isActive":true,"keyType":"PUBLIC_API"},
								"relationships":{"createdByActor":{"data":{"id":"actor-1"}},"revokedByActor":{"data":null}}
							}
						]
					}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors/actor-1"):
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[]}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[
							{
								"id":"actor-1",
								"attributes":{"roles":["LEGAL","ADMIN"]},
								"relationships":{"provider":{"data":{"id":"prov-1"}},"person":{"data":{"id":"person-1"}}}
							}
						],
						"included":[
							{"type":"people","id":"person-1","attributes":{"firstName":"Mithilesh","lastName":"Chellappan"}},
							{"type":"providers","id":"prov-1","attributes":{"name":"Ignored Provider"}}
						]
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected request URL %q", r.URL.String())
				return nil, nil
			}
		})},
	}

	got, err := client.LookupAPIKeyRoles(context.Background(), "ind-3")
	if err != nil {
		t.Fatalf("LookupAPIKeyRoles() error: %v", err)
	}
	if got.RoleSource != "actor" || len(got.Roles) != 2 || got.Roles[0] != "LEGAL" {
		t.Fatalf("unexpected fallback result: %#v", got)
	}
}

func TestClientLookupAPIKeyRolesFallsBackToPaginatedActorList(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Path, "/iris/v1/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"included":[]}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/iris/v2/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[
							{
								"id":"ind-paginated-actor",
								"attributes":{"roles":[],"nickname":"individual-key","isActive":true,"keyType":"PUBLIC_API"},
								"relationships":{"createdByActor":{"data":{"id":"actor-9"}},"revokedByActor":{"data":null}}
							}
						]
					}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors/actor-9"):
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[]}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors") && r.URL.Query().Get("cursor") == "page-2":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[
							{
								"id":"actor-9",
								"attributes":{"roles":["ADMIN","LEGAL"]},
								"relationships":{"provider":{"data":{"id":"prov-1"}},"person":{"data":{"id":"person-9"}}}
							}
						],
						"included":[
							{"type":"people","id":"person-9","attributes":{"firstName":"Late","lastName":"Actor"}},
							{"type":"providers","id":"prov-1","attributes":{"name":"Ignored Provider"}}
						]
					}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[],
						"included":[],
						"links":{"next":"https://appstoreconnect.apple.com/olympus/v1/actors?cursor=page-2"}
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected request URL %q", r.URL.String())
				return nil, nil
			}
		})},
	}

	got, err := client.LookupAPIKeyRoles(context.Background(), "ind-paginated-actor")
	if err != nil {
		t.Fatalf("LookupAPIKeyRoles() error: %v", err)
	}
	if got.RoleSource != "actor" || len(got.Roles) != 2 || got.Roles[0] != "ADMIN" {
		t.Fatalf("unexpected fallback result: %#v", got)
	}
	if got.GeneratedBy == nil || got.GeneratedBy.Name != "Late Actor" {
		t.Fatalf("unexpected generatedBy: %#v", got.GeneratedBy)
	}
}

func TestClientLookupAPIKeyRolesFallsBackToActorListWhenActorGetForbidden(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Path, "/iris/v1/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"included":[]}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/iris/v2/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[
							{
								"id":"ind-5",
								"attributes":{"roles":[],"nickname":"individual-key","isActive":true,"keyType":"PUBLIC_API"},
								"relationships":{"createdByActor":{"data":{"id":"actor-1"}},"revokedByActor":{"data":null}}
							}
						]
					}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors/actor-1"):
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[]}`)),
					Request:    r,
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[
							{
								"id":"actor-1",
								"attributes":{"roles":["ADMIN","CIPS"]},
								"relationships":{"provider":{"data":{"id":"prov-1"}},"person":{"data":{"id":"person-1"}}}
							}
						],
						"included":[
							{"type":"people","id":"person-1","attributes":{"firstName":"Mithilesh","lastName":"Chellappan"}},
							{"type":"providers","id":"prov-1","attributes":{"name":"Ignored Provider"}}
						]
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected request URL %q", r.URL.String())
				return nil, nil
			}
		})},
	}

	got, err := client.LookupAPIKeyRoles(context.Background(), "ind-5")
	if err != nil {
		t.Fatalf("LookupAPIKeyRoles() error: %v", err)
	}
	if got.RoleSource != "actor" || len(got.Roles) != 2 || got.Roles[0] != "ADMIN" {
		t.Fatalf("unexpected fallback result: %#v", got)
	}
}

func TestClientLookupAPIKeyRolesFailsWhenIndividualRolesCannotBeResolved(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Path, "/iris/v1/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"included":[]}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/iris/v2/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[
							{
								"id":"ind-4",
								"attributes":{"roles":[],"nickname":"individual-key","isActive":true,"keyType":"PUBLIC_API"},
								"relationships":{"createdByActor":{"data":{"id":"actor-missing"}},"revokedByActor":{"data":null}}
							}
						]
					}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors/actor-missing"):
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[]}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"included":[]}`)),
				}, nil
			default:
				t.Fatalf("unexpected request URL %q", r.URL.String())
				return nil, nil
			}
		})},
	}

	_, err := client.LookupAPIKeyRoles(context.Background(), "ind-4")
	if !errors.Is(err, ErrAPIKeyRolesUnresolved) {
		t.Fatalf("expected ErrAPIKeyRolesUnresolved, got %v", err)
	}
}

func TestClientLookupAPIKeyRolesFailsWhenActorListIsForbidden(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(r.URL.Path, "/iris/v1/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"included":[]}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/iris/v2/apiKeys"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"data":[
							{
								"id":"ind-5",
								"attributes":{"roles":[],"nickname":"individual-key","isActive":true,"keyType":"PUBLIC_API"},
								"relationships":{"createdByActor":{"data":{"id":"actor-hidden"}},"revokedByActor":{"data":null}}
							}
						]
					}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors/actor-hidden"):
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"403"}]}`)),
				}, nil
			case strings.Contains(r.URL.Path, "/olympus/v1/actors"):
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"403"}]}`)),
				}, nil
			default:
				t.Fatalf("unexpected request URL %q", r.URL.String())
				return nil, nil
			}
		})},
	}

	_, err := client.LookupAPIKeyRoles(context.Background(), "ind-5")
	if !errors.Is(err, ErrAPIKeyRolesUnresolved) {
		t.Fatalf("expected ErrAPIKeyRolesUnresolved, got %v", err)
	}
}
