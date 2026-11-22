package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestListAppComplianceRequirements(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []AppComplianceRequirement
	}{
		{
			name: "answered and unanswered requirements",
			body: `{"data":[
				{"id":"req-1","formName":"SECURITYASF395E","entityId":"app-123","entityType":"APP",
				 "status":"COLLECTED_VERIFY_PENDING","isRequired":true,"countries":["CHN"],"formId":"form-1",
				 "dateSigned":"2026-01-01T00:00:00.000Z","lastUpdatedDate":"2026-01-02T00:00:00.000Z",
				 "requiredActions":["APP_BINARY_SUBMISSION"]},
				{"id":"req-2","formName":"AIGENERATEFD1DE","entityId":"app-123","entityType":"APP",
				 "status":"PENDING_COLLECT","isRequired":true,"countries":["CHN"]},
				{"id":"req-3","formName":"FUTUREFORM123","status":"PENDING_COLLECT","isRequired":false}
			]}`,
			want: []AppComplianceRequirement{
				{
					AppID: "app-123", RequirementID: "req-1", RequirementName: "SECURITY_ASSESSMENT", FormName: "SECURITYASF395E",
					Status: "COLLECTED_VERIFY_PENDING", Required: true, CountriesOrRegions: []string{"CHN"}, FormID: "form-1",
					DateSigned: "2026-01-01T00:00:00.000Z", LastUpdatedDate: "2026-01-02T00:00:00.000Z",
					RequiredActions: []string{"APP_BINARY_SUBMISSION"},
				},
				{
					AppID: "app-123", RequirementID: "req-2", RequirementName: "AI_GENERATED_CONTENT", FormName: "AIGENERATEFD1DE",
					Status: "PENDING_COLLECT", Required: true, CountriesOrRegions: []string{"CHN"},
				},
				{
					AppID: "app-123", RequirementID: "req-3", RequirementName: "FUTUREFORM123", FormName: "FUTUREFORM123",
					Status: "PENDING_COLLECT",
				},
			},
		},
		{
			name: "no requirements omits data",
			body: `{}`,
			want: []AppComplianceRequirement{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/ppm/compliancerequirements/v1/accounts/account-123/requirements" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				}
				if got := r.URL.RawQuery; got != "entity_id=app-123" {
					t.Fatalf("expected query entity_id=app-123, got %q", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			got, err := testWebClient(server).ListAppComplianceRequirements(context.Background(), "account-123", " app-123 ")
			if err != nil {
				t.Fatalf("ListAppComplianceRequirements() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected requirements:\n got %#v\nwant %#v", got, tc.want)
			}
		})
	}
}
