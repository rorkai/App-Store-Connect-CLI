package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestListDeveloperAppGroupsUsesPortalFormEndpoint(t *testing.T) {
	client := newDeveloperAppGroupsTestClient(t, func(requestNumber int, request *http.Request) (*http.Response, error) {
		switch requestNumber {
		case 1:
			return assertDeveloperPortalBootstrap(t, request), nil
		case 2:
			if request.Method != http.MethodPost || request.URL.Path != "/services-account/QH65B2/account/ios/identifiers/listApplicationGroups.action" {
				t.Fatalf("unexpected list request %s %s", request.Method, request.URL.String())
			}
			assertDeveloperPortalForm(t, request, url.Values{
				"teamId":     {"TEAM123456"},
				"pageNumber": {"1"},
				"pageSize":   {"500"},
				"sort":       {"name=asc"},
			})
			return developerPortalTestResponse(http.StatusOK, `{
				"resultCode":0,
				"pageNumber":1,
				"pageSize":500,
				"totalRecords":2,
				"applicationGroupList":[
					{"name":"Shared","prefix":"TEAM123456","identifier":"group.com.example.shared","status":"current","applicationGroup":"GROUP12345"},
					{"name":"Preview","prefix":"TEAM123456","identifier":"group.com.example.preview","status":"current","applicationGroup":"GROUP67890"}
				]
			}`, nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestNumber, request.Method, request.URL.String())
			return nil, nil
		}
	})

	result, err := client.ListDeveloperAppGroups(context.Background(), DeveloperAppGroupsListOptions{})
	if err != nil {
		t.Fatalf("ListDeveloperAppGroups() error: %v", err)
	}
	if len(result.Data) != 2 || result.Data[0].ID != "GROUP12345" || result.Data[0].Identifier != "group.com.example.shared" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCreateDeveloperAppGroupUsesPortalFormEndpoint(t *testing.T) {
	client := newDeveloperAppGroupsTestClient(t, func(requestNumber int, request *http.Request) (*http.Response, error) {
		switch requestNumber {
		case 1:
			return assertDeveloperPortalBootstrapWithoutCSRF(t, request), nil
		case 2:
			if request.URL.Path != "/services-account/QH65B2/account/ios/identifiers/listApplicationGroups.action" {
				t.Fatalf("unexpected CSRF priming request %s %s", request.Method, request.URL.String())
			}
			return developerPortalTestResponse(http.StatusOK, `{"resultCode":0,"applicationGroupList":[]}`, http.Header{"csrf": {"primed-csrf"}, "csrf_ts": {"primed-ts"}}), nil
		case 3:
			if request.Method != http.MethodPost || request.URL.Path != "/services-account/QH65B2/account/ios/identifiers/addApplicationGroup.action" {
				t.Fatalf("unexpected create request %s %s", request.Method, request.URL.String())
			}
			if request.Header.Get("csrf") != "primed-csrf" || request.Header.Get("csrf_ts") != "primed-ts" {
				t.Fatalf("create request missing CSRF headers")
			}
			assertDeveloperPortalForm(t, request, url.Values{
				"teamId":     {"TEAM123456"},
				"name":       {"Example Preview"},
				"identifier": {"group.com.example.preview"},
			})
			return developerPortalTestResponse(http.StatusOK, `{
				"resultCode":0,
				"applicationGroup":{"name":"Example Preview","prefix":"TEAM123456","identifier":"group.com.example.preview","status":"current","applicationGroup":"GROUP12345"}
			}`, nil), nil
		default:
			t.Fatalf("unexpected request %d", requestNumber)
			return nil, nil
		}
	})

	result, err := client.CreateDeveloperAppGroup(context.Background(), DeveloperAppGroupCreateRequest{
		Name:       "Example Preview",
		Identifier: "group.com.example.preview",
	})
	if err != nil {
		t.Fatalf("CreateDeveloperAppGroup() error: %v", err)
	}
	if result.ID != "GROUP12345" || result.Name != "Example Preview" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestListDeveloperAppGroupsPaginates(t *testing.T) {
	client := newDeveloperAppGroupsTestClient(t, func(requestNumber int, request *http.Request) (*http.Response, error) {
		switch requestNumber {
		case 1:
			return assertDeveloperPortalBootstrap(t, request), nil
		case 2, 3:
			if request.URL.Path != "/services-account/QH65B2/account/ios/identifiers/listApplicationGroups.action" {
				t.Fatalf("unexpected path: %s", request.URL.Path)
			}
			if err := request.ParseForm(); err != nil {
				t.Fatalf("ParseForm() error: %v", err)
			}
			wantPage := strconv.Itoa(requestNumber - 1)
			if request.PostForm.Get("pageNumber") != wantPage {
				t.Fatalf("pageNumber = %q, want %q", request.PostForm.Get("pageNumber"), wantPage)
			}
			id := "GROUP" + wantPage
			return developerPortalTestResponse(http.StatusOK, fmt.Sprintf(`{
				"resultCode":0,"pageNumber":%s,"pageSize":1,"totalRecords":2,
				"applicationGroupList":[{"name":"Group %s","identifier":"group.com.example.%s","applicationGroup":"%s"}]
			}`, wantPage, wantPage, wantPage, id), nil), nil
		default:
			t.Fatalf("unexpected request %d", requestNumber)
			return nil, nil
		}
	})

	result, err := client.ListDeveloperAppGroups(context.Background(), DeveloperAppGroupsListOptions{Paginate: true})
	if err != nil {
		t.Fatalf("ListDeveloperAppGroups() error: %v", err)
	}
	if len(result.Data) != 2 || result.Data[1].ID != "GROUP2" {
		t.Fatalf("unexpected paginated result: %+v", result)
	}
}

func TestListDeveloperAppGroupsStopsAfterFirstPageByDefault(t *testing.T) {
	client := newDeveloperAppGroupsTestClient(t, func(requestNumber int, request *http.Request) (*http.Response, error) {
		switch requestNumber {
		case 1:
			return assertDeveloperPortalBootstrap(t, request), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, `{
				"resultCode":0,"pageNumber":1,"pageSize":1,"totalRecords":2,
				"applicationGroupList":[{"name":"Group 1","identifier":"group.com.example.1","applicationGroup":"GROUP1"}]
			}`, nil), nil
		default:
			t.Fatalf("default list fetched unexpected request %d", requestNumber)
			return nil, nil
		}
	})

	result, err := client.ListDeveloperAppGroups(context.Background(), DeveloperAppGroupsListOptions{})
	if err != nil {
		t.Fatalf("ListDeveloperAppGroups() error: %v", err)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "GROUP1" {
		t.Fatalf("unexpected first-page result: %+v", result)
	}
}

func TestDeveloperAppGroupsRejectsPortalErrorEnvelope(t *testing.T) {
	client := newDeveloperAppGroupsTestClient(t, func(requestNumber int, request *http.Request) (*http.Response, error) {
		if requestNumber == 1 {
			return assertDeveloperPortalBootstrap(t, request), nil
		}
		return developerPortalTestResponse(http.StatusOK, `{"resultCode":35,"userString":"Identifier already exists","requestId":"request-1"}`, nil), nil
	})

	_, err := client.CreateDeveloperAppGroup(context.Background(), DeveloperAppGroupCreateRequest{Name: "Example", Identifier: "group.com.example.shared"})
	if err == nil || !strings.Contains(err.Error(), "result code 35") || !strings.Contains(err.Error(), "Identifier already exists") || !strings.Contains(err.Error(), "request-1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeveloperAppGroupsRejectsMalformedSuccessEnvelope(t *testing.T) {
	client := newDeveloperAppGroupsTestClient(t, func(requestNumber int, request *http.Request) (*http.Response, error) {
		if requestNumber == 1 {
			return assertDeveloperPortalBootstrap(t, request), nil
		}
		return developerPortalTestResponse(http.StatusOK, `{"applicationGroupList":[]}`, nil), nil
	})

	_, err := client.ListDeveloperAppGroups(context.Background(), DeveloperAppGroupsListOptions{})
	if err == nil || !strings.Contains(err.Error(), "missing resultCode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeveloperAppGroupsSurfacesHTTPError(t *testing.T) {
	client := newDeveloperAppGroupsTestClient(t, func(requestNumber int, request *http.Request) (*http.Response, error) {
		if requestNumber == 1 {
			return assertDeveloperPortalBootstrap(t, request), nil
		}
		return developerPortalTestResponse(http.StatusInternalServerError, `{"error":"portal unavailable"}`, http.Header{"X-Apple-Request-UUID": {"apple-request-1"}}), nil
	})

	_, err := client.ListDeveloperAppGroups(context.Background(), DeveloperAppGroupsListOptions{})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAssignDeveloperAppGroupPreservesBundleGraph(t *testing.T) {
	var patchBody []byte
	client := newDeveloperAppGroupsTestClient(t, func(requestNumber int, request *http.Request) (*http.Response, error) {
		switch requestNumber {
		case 1:
			return assertDeveloperPortalBootstrapWithoutCSRF(t, request), nil
		case 2:
			if request.URL.Path != "/services-account/QH65B2/account/ios/identifiers/listApplicationGroups.action" {
				t.Fatalf("unexpected CSRF priming request %s %s", request.Method, request.URL.String())
			}
			return developerPortalTestResponse(http.StatusOK, `{"resultCode":0,"applicationGroupList":[]}`, http.Header{"csrf": {"primed-csrf"}, "csrf_ts": {"primed-ts"}}), nil
		case 3:
			if request.Method != http.MethodPost || request.URL.Path != "/services-account/v1/bundleIds/bundle-1" || request.Header.Get("X-HTTP-Method-Override") != http.MethodGet {
				t.Fatalf("unexpected bundle read %s %s", request.Method, request.URL.String())
			}
			return developerPortalTestResponse(http.StatusOK, `{
				"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app","platform":"IOS"},"relationships":{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":"push-1"}]}}},
				"included":[{"type":"bundleIdCapabilities","id":"push-1","attributes":{"enabled":true,"settings":[]},"relationships":{"capability":{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"}}}}]
			}`, nil), nil
		case 4:
			if request.Method != http.MethodPatch || request.URL.Path != "/services-account/v1/bundleIds/bundle-1" {
				t.Fatalf("unexpected bundle patch %s %s", request.Method, request.URL.String())
			}
			if request.Header.Get("csrf") != "primed-csrf" || request.Header.Get("csrf_ts") != "primed-ts" {
				t.Fatalf("bundle patch missing primed CSRF headers")
			}
			var err error
			patchBody, err = io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			return developerPortalTestResponse(http.StatusOK, `{"data":{"type":"bundleIds","id":"bundle-1"}}`, nil), nil
		default:
			t.Fatalf("unexpected request %d", requestNumber)
			return nil, nil
		}
	})

	result, err := client.AssignDeveloperAppGroup(context.Background(), DeveloperAppGroupAssignRequest{BundleID: "bundle-1", GroupID: "GROUP12345"})
	if err != nil {
		t.Fatalf("AssignDeveloperAppGroup() error: %v", err)
	}
	if !result.Changed || result.Status != "assigned" {
		t.Fatalf("unexpected result: %+v", result)
	}

	var payload developerBundleIDPatchRequest
	if err := json.Unmarshal(patchBody, &payload); err != nil {
		t.Fatalf("decode patch: %v; body=%s", err, patchBody)
	}
	if payload.Data.Attributes == nil || !strings.Contains(string(payload.Data.Attributes), `"teamId":"TEAM123456"`) {
		t.Fatalf("team and existing attributes not preserved: %s", payload.Data.Attributes)
	}
	var capabilities developerResourceRelationship
	if err := json.Unmarshal(payload.Data.Relationships["bundleIdCapabilities"], &capabilities); err != nil {
		t.Fatalf("decode capabilities: %v", err)
	}
	if len(capabilities.Data) != 2 {
		t.Fatalf("capability count = %d, want existing plus APP_GROUPS", len(capabilities.Data))
	}
	appGroupsCapability := capabilities.Data[1]
	if capability, err := developerBundleIDCapabilityID(appGroupsCapability); err != nil || capability != "APP_GROUPS" {
		t.Fatalf("unexpected app groups capability: %+v, err=%v", appGroupsCapability, err)
	}
	var groups developerResourceRelationship
	if err := json.Unmarshal(appGroupsCapability.Relationships["appGroups"], &groups); err != nil {
		t.Fatalf("decode appGroups: %v", err)
	}
	if len(groups.Data) != 1 || groups.Data[0].ID != "GROUP12345" || groups.Data[0].Type != "appGroups" {
		t.Fatalf("unexpected appGroups relationship: %+v", groups.Data)
	}
}

func TestAssignDeveloperAppGroupIsIdempotent(t *testing.T) {
	client := newDeveloperAppGroupsTestClient(t, func(requestNumber int, request *http.Request) (*http.Response, error) {
		switch requestNumber {
		case 1:
			return assertDeveloperPortalBootstrap(t, request), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, `{
				"data":{"id":"bundle-1","type":"bundleIds","attributes":{"identifier":"com.example.app"},"relationships":{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":"groups-1"}]}}},
				"included":[{"type":"bundleIdCapabilities","id":"groups-1","attributes":{"enabled":true,"settings":[]},"relationships":{"capability":{"data":{"type":"capabilities","id":"APP_GROUPS"}},"appGroups":{"data":[{"type":"appGroups","id":"GROUP12345"}]}}}]
			}`, nil), nil
		default:
			t.Fatalf("idempotent assignment sent unexpected request %d", requestNumber)
			return nil, nil
		}
	})

	result, err := client.AssignDeveloperAppGroup(context.Background(), DeveloperAppGroupAssignRequest{BundleID: "bundle-1", GroupID: "GROUP12345"})
	if err != nil {
		t.Fatalf("AssignDeveloperAppGroup() error: %v", err)
	}
	if result.Changed || result.Status != "already-assigned" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func newDeveloperAppGroupsTestClient(t *testing.T, handler func(int, *http.Request) (*http.Response, error)) *Client {
	t.Helper()
	requestNumber := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestNumber++
		response, err := handler(requestNumber, request)
		if err != nil {
			t.Errorf("test Developer Portal handler: %v", err)
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
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
	return &Client{httpClient: server.Client(), developerPortalURL: server.URL}
}

func assertDeveloperPortalBootstrap(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	if request.Method != http.MethodPost || request.URL.Path != "/services-account/QH65B2/account/listTeams.action" {
		t.Fatalf("unexpected bootstrap request %s %s", request.Method, request.URL.String())
	}
	return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"bootstrap-csrf"}, "csrf_ts": {"bootstrap-ts"}})
}

func assertDeveloperPortalBootstrapWithoutCSRF(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	if request.Method != http.MethodPost || request.URL.Path != "/services-account/QH65B2/account/listTeams.action" {
		t.Fatalf("unexpected bootstrap request %s %s", request.Method, request.URL.String())
	}
	return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil)
}

func assertDeveloperPortalForm(t *testing.T, request *http.Request, expected url.Values) {
	t.Helper()
	if got := request.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-www-form-urlencoded") {
		t.Fatalf("Content-Type = %q", got)
	}
	if err := request.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error: %v", err)
	}
	for key, values := range expected {
		if got := request.PostForm[key]; strings.Join(got, "\x00") != strings.Join(values, "\x00") {
			t.Fatalf("form %s = %q, want %q", key, got, values)
		}
	}
}
