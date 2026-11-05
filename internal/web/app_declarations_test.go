package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListAppDeclarationsReturnsRequirements(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/ppm/complianceform/v1/accounts/account-123/requirements" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if got := r.URL.Query().Get("contentId"); got != "app-123" {
			t.Fatalf("expected contentId app-123, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"accountId":"account-123",
			"requirementData":[{
				"contentId":"app-123",
				"requirements":[
					{
						"id":"req-1",
						"name":"MEDICAL_DEVICE",
						"ref":"medical",
						"status":"COLLECTED",
						"formId":"form-1",
						"dateSigned":"2026-09-01T00:00:00Z",
						"isRequired":true
					},
					{
						"id":"req-2",
						"name":"OTHER_REQUIREMENT",
						"status":"PENDING_COLLECTION",
						"isRequired":false
					}
				]
			}]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.ListAppDeclarations(context.Background(), "account-123", "app-123")
	if err != nil {
		t.Fatalf("ListAppDeclarations() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(got))
	}
	first := got[0]
	if first.AppID != "app-123" || first.RequirementID != "req-1" || first.RequirementName != "MEDICAL_DEVICE" {
		t.Fatalf("unexpected first declaration: %#v", first)
	}
	if first.Status != "COLLECTED" || first.FormID != "form-1" || first.DateSigned != "2026-09-01T00:00:00Z" {
		t.Fatalf("unexpected first declaration metadata: %#v", first)
	}
	if !first.Required {
		t.Fatalf("expected first declaration to be required: %#v", first)
	}
	if got[1].RequirementName != "OTHER_REQUIREMENT" || got[1].Required {
		t.Fatalf("unexpected second declaration: %#v", got[1])
	}
}

func TestListAppDeclarationsRequiresIdentifiers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
	}))
	t.Cleanup(server.Close)
	client := testWebClient(server)

	if _, err := client.ListAppDeclarations(context.Background(), "", "app-123"); err == nil ||
		!strings.Contains(err.Error(), "account id is required") {
		t.Fatalf("expected account id error, got %v", err)
	}
	if _, err := client.ListAppDeclarations(context.Background(), "account-123", " "); err == nil ||
		!strings.Contains(err.Error(), "app id is required") {
		t.Fatalf("expected app id error, got %v", err)
	}
}

func TestListAppDeclarationsSurfacesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"detail":"forbidden"}]}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	if _, err := client.ListAppDeclarations(context.Background(), "account-123", "app-123"); err == nil {
		t.Fatal("expected error for forbidden response")
	}
}

func medicalDeclarationServer(t *testing.T, formBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"requirementData":[{
					"contentId":"app-123",
					"requirements":[{
						"id":"req-1",
						"name":"MEDICAL_DEVICE",
						"status":"COLLECTED",
						"formId":"form-1",
						"isRequired":true
					}]
				}]
			}`))
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-1/forms":
			if got := r.URL.Query().Get("contentId"); got != "app-123" {
				t.Fatalf("expected contentId app-123, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(formBody))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
}

func TestGetMedicalDeviceDeclarationReadsArrayFormPayload(t *testing.T) {
	server := medicalDeclarationServer(t, `{
		"data":[{
			"requirementName":"MEDICAL_DEVICE",
			"countriesOrRegions":["USA","EEA"],
			"medicalDeviceData":{"declaration":"no"}
		}],
		"constraints":{}
	}`)
	defer server.Close()

	got, err := testWebClient(server).GetMedicalDeviceDeclaration(context.Background(), "account-123", "app-123")
	if err != nil {
		t.Fatalf("GetMedicalDeviceDeclaration() error = %v", err)
	}
	if got.Declaration != "no" {
		t.Fatalf("expected declaration no, got %q", got.Declaration)
	}
	if got.RequirementID != "req-1" || got.RequirementName != "MEDICAL_DEVICE" || got.Status != "COLLECTED" {
		t.Fatalf("unexpected requirement metadata: %#v", got)
	}
	if !got.Required {
		t.Fatalf("expected required declaration: %#v", got)
	}
	if strings.Join(got.CountriesOrRegions, ",") != "EEA,USA" {
		t.Fatalf("unexpected countries: %#v", got.CountriesOrRegions)
	}
}

func TestGetMedicalDeviceDeclarationReadsFormPayloadShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "object form payload", body: `{"data":{"medicalDeviceData":{"declaration":"yes"}},"constraints":{}}`, want: "yes"},
		{name: "top-level form payload", body: `{"medicalDeviceData":{"declaration":"no"}}`, want: "no"},
		{name: "unanswered form", body: `{"data":[],"constraints":{}}`, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := medicalDeclarationServer(t, tc.body)
			defer server.Close()

			got, err := testWebClient(server).GetMedicalDeviceDeclaration(context.Background(), "account-123", "app-123")
			if err != nil {
				t.Fatalf("GetMedicalDeviceDeclaration() error = %v", err)
			}
			if got.Declaration != tc.want {
				t.Fatalf("declaration = %q, want %q", got.Declaration, tc.want)
			}
		})
	}
}

func TestGetMedicalDeviceDeclarationMissingRequirement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"requirementData":[{"contentId":"app-123","requirements":[]}]}`))
	}))
	defer server.Close()

	_, err := testWebClient(server).GetMedicalDeviceDeclaration(context.Background(), "account-123", "app-123")
	if err == nil || !strings.Contains(err.Error(), "regulated medical device requirement was not found") {
		t.Fatalf("expected missing requirement error, got %v", err)
	}
}

func TestSetMedicalDeviceDeclarationSkipsWhenAlreadyDeclared(t *testing.T) {
	server := medicalDeclarationServer(t, `{
		"data":[{"medicalDeviceData":{"declaration":"no"},"countriesOrRegions":["USA"]}],
		"constraints":{
			"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"USA"}]}
		}
	}`)
	defer server.Close()

	got, err := testWebClient(server).SetMedicalDeviceDeclaration(context.Background(), "account-123", "app-123", false)
	if err != nil {
		t.Fatalf("SetMedicalDeviceDeclaration() error = %v", err)
	}
	if got.Changed {
		t.Fatal("expected changed=false when the declaration already matches")
	}
	if got.Declared {
		t.Fatalf("expected declared false, got true")
	}
	if got.RequirementID != "req-1" || got.Status != "COLLECTED" {
		t.Fatalf("unexpected requirement metadata: %#v", got)
	}
}

func TestSetMedicalDeviceDeclarationSkipsWhenAlreadyDeclaredWithoutConstraints(t *testing.T) {
	server := medicalDeclarationServer(t, `{
		"data":[{"medicalDeviceData":{"declaration":"no"},"countriesOrRegions":["USA"]}]
	}`)
	defer server.Close()

	got, err := testWebClient(server).SetMedicalDeviceDeclaration(context.Background(), "account-123", "app-123", false)
	if err != nil {
		t.Fatalf("SetMedicalDeviceDeclaration() error = %v", err)
	}
	if got.Changed {
		t.Fatal("expected changed=false when the declaration already matches")
	}
	if got.Declared {
		t.Fatalf("expected declared false, got true")
	}
	if strings.Join(got.CountriesOrRegions, ",") != "USA" {
		t.Fatalf("expected existing country selection USA, got %#v", got.CountriesOrRegions)
	}
}
