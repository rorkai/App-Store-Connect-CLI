package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetCIUsageSummaryParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teams/team-uuid/usage/summary" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"plan": {
				"name": "Plan",
				"available": 1467,
				"used": 33,
				"total": 1500,
				"reset_date": "2026-03-16",
				"reset_date_time": "2026-03-16T09:43:54Z"
			},
			"links": {
				"manage": "https://developer.apple.com/xcode-cloud/"
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.GetCIUsageSummary(context.Background(), "team-uuid")
	if err != nil {
		t.Fatalf("GetCIUsageSummary() error = %v", err)
	}
	if result.Plan.Name != "Plan" {
		t.Fatalf("expected plan name %q, got %q", "Plan", result.Plan.Name)
	}
	if result.Plan.Available != 1467 {
		t.Fatalf("expected available 1467, got %d", result.Plan.Available)
	}
	if result.Plan.Used != 33 {
		t.Fatalf("expected used 33, got %d", result.Plan.Used)
	}
	if result.Plan.Total != 1500 {
		t.Fatalf("expected total 1500, got %d", result.Plan.Total)
	}
	if result.Plan.ResetDate != "2026-03-16" {
		t.Fatalf("expected reset_date %q, got %q", "2026-03-16", result.Plan.ResetDate)
	}
	if result.Plan.ResetDateTime != "2026-03-16T09:43:54Z" {
		t.Fatalf("expected reset_date_time %q, got %q", "2026-03-16T09:43:54Z", result.Plan.ResetDateTime)
	}
	if result.Links["manage"] != "https://developer.apple.com/xcode-cloud/" {
		t.Fatalf("expected manage link, got %v", result.Links)
	}
}

func TestGetCIUsageSummaryRejectsEmptyTeamID(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	_, err := client.GetCIUsageSummary(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty team ID")
	}
	if !strings.Contains(err.Error(), "team id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetCIUsageMonthsQueryParams(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":[],"product_usage":[],"info":{}}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	_, err := client.GetCIUsageMonths(context.Background(), "team-uuid", 1, 2025, 12, 2025)
	if err != nil {
		t.Fatalf("GetCIUsageMonths() error = %v", err)
	}
	for _, param := range []string{"start_month=1", "start_year=2025", "end_month=12", "end_year=2025"} {
		if !strings.Contains(gotQuery, param) {
			t.Fatalf("expected query to contain %q, got %q", param, gotQuery)
		}
	}
}

func TestGetCIUsageMonthsParsesProductUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"usage": [{"month":1,"year":2026,"minutes":120,"number_of_builds":3}],
			"product_usage": [
				{
					"product_id": "prod-1",
					"usage_in_minutes": 120,
					"usage_in_seconds": 7200,
					"number_of_builds": 3,
					"previous_usage_in_minutes": 80,
					"previous_number_of_builds": 2
				}
			],
			"info": {
				"can_view_all_products": true,
				"current": {"builds":3,"used":120,"average_30_days":60},
				"previous": {"builds":2,"used":80,"average_30_days":40}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.GetCIUsageMonths(context.Background(), "team-uuid", 1, 2026, 1, 2026)
	if err != nil {
		t.Fatalf("GetCIUsageMonths() error = %v", err)
	}
	if len(result.Usage) != 1 || result.Usage[0].Duration != 120 || result.Usage[0].NumberOfBuilds != 3 {
		t.Fatalf("unexpected usage: %+v", result.Usage)
	}
	if len(result.ProductUsage) != 1 {
		t.Fatalf("expected 1 product usage, got %d", len(result.ProductUsage))
	}
	pu := result.ProductUsage[0]
	if pu.ProductID != "prod-1" || pu.UsageInMinutes != 120 || pu.NumberOfBuilds != 3 || pu.PreviousUsageInMinutes != 80 || pu.PreviousNumberOfBuilds != 2 {
		t.Fatalf("unexpected product usage: %+v", pu)
	}
	if !result.Info.CanViewAllProducts || result.Info.Current.Used != 120 || result.Info.Previous.Used != 80 {
		t.Fatalf("unexpected info: %+v", result.Info)
	}
}

func TestGetCIUsageMonthsRejectsEmptyTeamID(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	_, err := client.GetCIUsageMonths(context.Background(), "  ", 1, 2026, 1, 2026)
	if err == nil {
		t.Fatal("expected error for empty team ID")
	}
	if !strings.Contains(err.Error(), "team id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetCIUsageDaysParsesWorkflowUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/products/prod-1/usage/days") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("start") != "2026-01-01" || r.URL.Query().Get("end") != "2026-01-31" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"usage": [{"date":"2026-01-01","minutes":60,"number_of_builds":2}],
			"workflow_usage": [
				{
					"workflow_id": "wf-1",
					"usage_in_minutes": 60,
					"number_of_builds": 2,
					"previous_usage_in_minutes": 30,
					"previous_number_of_builds": 1
				}
			],
			"info": {"current":{"builds":2,"used":60,"average_30_days":45}}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.GetCIUsageDays(context.Background(), "team-uuid", "prod-1", "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("GetCIUsageDays() error = %v", err)
	}
	if len(result.Usage) != 1 || result.Usage[0].Duration != 60 || result.Usage[0].NumberOfBuilds != 2 {
		t.Fatalf("unexpected usage: %+v", result.Usage)
	}
	if len(result.WorkflowUsage) != 1 {
		t.Fatalf("expected 1 workflow usage, got %d", len(result.WorkflowUsage))
	}
	wf := result.WorkflowUsage[0]
	if wf.WorkflowID != "wf-1" || wf.UsageInMinutes != 60 || wf.NumberOfBuilds != 2 || wf.PreviousUsageInMinutes != 30 || wf.PreviousNumberOfBuilds != 1 {
		t.Fatalf("unexpected workflow usage: %+v", wf)
	}
	if result.Info.Current.Used != 60 {
		t.Fatalf("unexpected current usage info: %+v", result.Info.Current)
	}
}

func TestGetCIUsageDaysOverallParsesProductUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/teams/team-uuid/usage/days") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("start") != "2026-01-01" || r.URL.Query().Get("end") != "2026-01-31" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"usage": [{"date":"2026-01-01","minutes":90,"number_of_builds":4}],
			"product_usage": [
				{
					"product_id":"prod-1",
					"usage_in_minutes":60,
					"number_of_builds":2,
					"previous_usage_in_minutes":30,
					"previous_number_of_builds":1
				}
			],
			"info": {"current":{"builds":4,"used":90,"average_30_days":75}}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.GetCIUsageDaysOverall(context.Background(), "team-uuid", "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("GetCIUsageDaysOverall() error = %v", err)
	}
	if len(result.Usage) != 1 || result.Usage[0].Duration != 90 {
		t.Fatalf("unexpected usage: %+v", result.Usage)
	}
	if len(result.ProductUsage) != 1 {
		t.Fatalf("expected 1 product usage row, got %d", len(result.ProductUsage))
	}
	pu := result.ProductUsage[0]
	if pu.ProductID != "prod-1" || pu.UsageInMinutes != 60 || pu.NumberOfBuilds != 2 || pu.PreviousUsageInMinutes != 30 || pu.PreviousNumberOfBuilds != 1 {
		t.Fatalf("unexpected product usage: %+v", pu)
	}
	if result.Info.Current.Used != 90 || result.Info.Current.Builds != 4 {
		t.Fatalf("unexpected current info: %+v", result.Info.Current)
	}
}

func TestGetCIUsageDaysOverallRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name    string
		teamID  string
		start   string
		end     string
		wantErr string
	}{
		{"empty team", "", "2026-01-01", "2026-01-31", "team id is required"},
		{"empty start", "team", "", "2026-01-31", "start date is required"},
		{"empty end", "team", "2026-01-01", "", "end date is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetCIUsageDaysOverall(context.Background(), tt.teamID, tt.start, tt.end)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestCIMonthUsageUnmarshalSupportsDurationAlias(t *testing.T) {
	var usage CIMonthUsage
	if err := json.Unmarshal([]byte(`{"year":2026,"month":2,"duration":33}`), &usage); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if usage.Duration != 33 {
		t.Fatalf("expected duration 33, got %d", usage.Duration)
	}
}

func TestCIDayUsageUnmarshalSupportsDurationAlias(t *testing.T) {
	var usage CIDayUsage
	if err := json.Unmarshal([]byte(`{"date":"2026-02-20","duration":17}`), &usage); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if usage.Duration != 17 {
		t.Fatalf("expected duration 17, got %d", usage.Duration)
	}
}

func TestGetCIUsageDaysRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name      string
		teamID    string
		productID string
		start     string
		end       string
		wantErr   string
	}{
		{"empty team", "", "prod", "2026-01-01", "2026-01-31", "team id is required"},
		{"empty product", "team", "", "2026-01-01", "2026-01-31", "product id is required"},
		{"empty start", "team", "prod", "", "2026-01-31", "start date is required"},
		{"empty end", "team", "prod", "2026-01-01", "", "end date is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetCIUsageDays(context.Background(), tt.teamID, tt.productID, tt.start, tt.end)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestListCIProductsParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/products-v4") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "100" {
			t.Fatalf("expected limit=100, got %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id":"prod-1","name":"My App","bundle_id":"com.example.app","type":"solo"},
				{"id":"prod-2","name":"Other App","bundle_id":"com.other.app","type":"solo","icon_url":"https://example.com/icon.png"}
			]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.ListCIProducts(context.Background(), "team-uuid")
	if err != nil {
		t.Fatalf("ListCIProducts() error = %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 products, got %d", len(result.Items))
	}
	if result.Items[0].ID != "prod-1" || result.Items[0].BundleID != "com.example.app" {
		t.Fatalf("unexpected first product: %+v", result.Items[0])
	}
	if result.Items[1].IconURL != "https://example.com/icon.png" {
		t.Fatalf("expected icon_url, got %q", result.Items[1].IconURL)
	}
}

func TestListCIProductsRejectsEmptyTeamID(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	_, err := client.ListCIProducts(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty team ID")
	}
	if !strings.Contains(err.Error(), "team id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetCIUsageSummaryHandles4xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	_, err := client.GetCIUsageSummary(context.Background(), "team-uuid")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Status != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", apiErr.Status)
	}
}

func TestCIUsagePlanJSONRoundTrip(t *testing.T) {
	raw := `{"name":"Plan","reset_date":"2026-03-16","reset_date_time":"2026-03-16T09:43:54Z","available":1467,"used":33,"total":1500}`
	var plan CIUsagePlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if plan.ResetDate != "2026-03-16" {
		t.Fatalf("expected reset_date %q, got %q", "2026-03-16", plan.ResetDate)
	}
	if plan.ResetDateTime != "2026-03-16T09:43:54Z" {
		t.Fatalf("expected reset_date_time %q, got %q", "2026-03-16T09:43:54Z", plan.ResetDateTime)
	}

	out, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if !strings.Contains(string(out), `"reset_date":"2026-03-16"`) {
		t.Fatalf("expected reset_date in output, got %s", out)
	}
}

func TestNewCIClientSetsBaseURL(t *testing.T) {
	session := &AuthSession{Client: http.DefaultClient}
	client := NewCIClient(session)
	if !strings.HasSuffix(client.baseURL, "/ci/api") {
		t.Fatalf("expected base URL ending in /ci/api, got %q", client.baseURL)
	}
}

func TestListCIWorkflowsParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/products/prod-1/workflows-v15") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "100" {
			t.Fatalf("expected limit=100, got %q", r.URL.Query().Get("limit"))
		}
		if r.URL.Query().Get("include_deleted") != "false" {
			t.Fatalf("expected include_deleted=false, got %q", r.URL.Query().Get("include_deleted"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id":"wf-1","content":{"name":"TestFlight Deploy","description":"Build on main"}},
				{"id":"wf-2","content":{"name":"PR Check"}}
			]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.ListCIWorkflows(context.Background(), "team-uuid", "prod-1")
	if err != nil {
		t.Fatalf("ListCIWorkflows() error = %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(result.Items))
	}
	if result.Items[0].ID != "wf-1" || result.Items[0].Content.Name != "TestFlight Deploy" {
		t.Fatalf("unexpected first workflow: %+v", result.Items[0])
	}
	if result.Items[1].Content.Name != "PR Check" {
		t.Fatalf("unexpected second workflow name: %q", result.Items[1].Content.Name)
	}
}

func TestListCIWorkflowsRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name      string
		teamID    string
		productID string
		wantErr   string
	}{
		{"empty team", "", "prod", "team id is required"},
		{"empty product", "team", "", "product id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.ListCIWorkflows(context.Background(), tt.teamID, tt.productID)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestGetCIWorkflowParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teams/team-uuid/products/prod-1/workflows-v15/wf-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "wf-1",
			"content": {
				"name": "TestFlight Deploy",
				"environment_variables": [
					{"id":"ev-1","name":"API_KEY","value":{"plaintext":"abc123"}},
					{"id":"ev-2","name":"SECRET","value":{"redacted_value":"***"}}
				]
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.GetCIWorkflow(context.Background(), "team-uuid", "prod-1", "wf-1")
	if err != nil {
		t.Fatalf("GetCIWorkflow() error = %v", err)
	}
	if result.ID != "wf-1" {
		t.Fatalf("expected id %q, got %q", "wf-1", result.ID)
	}
	if result.Content == nil {
		t.Fatal("expected content to be non-nil")
	}
	// Verify content is valid JSON that contains expected fields.
	vars, err := ExtractEnvVars(result.Content)
	if err != nil {
		t.Fatalf("ExtractEnvVars() error = %v", err)
	}
	if len(vars) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(vars))
	}
	if vars[0].Name != "API_KEY" || vars[0].Value.Plaintext == nil || *vars[0].Value.Plaintext != "abc123" {
		t.Fatalf("unexpected first env var: %+v", vars[0])
	}
	if vars[1].Name != "SECRET" || vars[1].Value.RedactedValue == nil || *vars[1].Value.RedactedValue != "***" {
		t.Fatalf("unexpected second env var: %+v", vars[1])
	}
}

func TestGetCIWorkflowRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name       string
		teamID     string
		productID  string
		workflowID string
		wantErr    string
	}{
		{"empty team", "", "prod", "wf", "team id is required"},
		{"empty product", "team", "", "wf", "product id is required"},
		{"empty workflow", "team", "prod", "", "workflow id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetCIWorkflow(context.Background(), tt.teamID, tt.productID, tt.workflowID)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestUpdateCIWorkflowSendsBody(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	content := json.RawMessage(`{"name":"Updated","environment_variables":[]}`)
	err := client.UpdateCIWorkflow(context.Background(), "team-uuid", "prod-1", "wf-1", content)
	if err != nil {
		t.Fatalf("UpdateCIWorkflow() error = %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("expected PUT, got %s", gotMethod)
	}
	if gotPath != "/teams/team-uuid/products/prod-1/workflows-v15/wf-1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	// Verify the body is just the content (not wrapped in {id, content})
	if !strings.Contains(string(gotBody), "Updated") {
		t.Fatalf("expected body to contain 'Updated', got %s", gotBody)
	}
	if strings.Contains(string(gotBody), `"id"`) {
		t.Fatalf("body should not contain id wrapper, got %s", gotBody)
	}
}

func TestUpdateCIWorkflowRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name       string
		teamID     string
		productID  string
		workflowID string
		wantErr    string
	}{
		{"empty team", "", "prod", "wf", "team id is required"},
		{"empty product", "team", "", "wf", "product id is required"},
		{"empty workflow", "team", "prod", "", "workflow id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.UpdateCIWorkflow(context.Background(), tt.teamID, tt.productID, tt.workflowID, json.RawMessage(`{}`))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestGetCIEncryptionKeyParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/keys/client-encryption" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"0xm9f0gX7lzArxrChNrDVUR3MKxueb1DdheWBeLndCVOqoiEsT2jxqZW6cHsIuDGDykvYWgQ1qaPBSxCNFXEUg=="}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.GetCIEncryptionKey(context.Background())
	if err != nil {
		t.Fatalf("GetCIEncryptionKey() error = %v", err)
	}
	if result.Key != "0xm9f0gX7lzArxrChNrDVUR3MKxueb1DdheWBeLndCVOqoiEsT2jxqZW6cHsIuDGDykvYWgQ1qaPBSxCNFXEUg==" {
		t.Fatalf("unexpected key: %q", result.Key)
	}
}

func TestExtractEnvVars(t *testing.T) {
	content := json.RawMessage(`{
		"name":"Test",
		"environment_variables":[
			{"id":"1","name":"FOO","value":{"plaintext":"bar"}},
			{"id":"2","name":"SECRET","value":{"redacted_value":"***"}}
		]
	}`)
	vars, err := ExtractEnvVars(content)
	if err != nil {
		t.Fatalf("ExtractEnvVars() error = %v", err)
	}
	if len(vars) != 2 {
		t.Fatalf("expected 2 vars, got %d", len(vars))
	}
	if vars[0].Name != "FOO" {
		t.Fatalf("expected name FOO, got %q", vars[0].Name)
	}
	if vars[1].Name != "SECRET" {
		t.Fatalf("expected name SECRET, got %q", vars[1].Name)
	}
}

func TestExtractEnvVarsNoKey(t *testing.T) {
	content := json.RawMessage(`{"name":"Test"}`)
	vars, err := ExtractEnvVars(content)
	if err != nil {
		t.Fatalf("ExtractEnvVars() error = %v", err)
	}
	if len(vars) != 0 {
		t.Fatalf("expected 0 vars, got %d", len(vars))
	}
}

func TestSetEnvVars(t *testing.T) {
	content := json.RawMessage(`{"name":"Test","environment_variables":[{"id":"1","name":"OLD","value":{"plaintext":"old"}}]}`)
	pt := "new-value"
	newVars := []CIEnvironmentVariable{
		{ID: "2", Name: "NEW", Value: CIEnvironmentVariableValue{Plaintext: &pt}},
	}
	result, err := SetEnvVars(content, newVars)
	if err != nil {
		t.Fatalf("SetEnvVars() error = %v", err)
	}
	// Verify the result has the new vars and preserves name.
	vars, err := ExtractEnvVars(result)
	if err != nil {
		t.Fatalf("ExtractEnvVars() error = %v", err)
	}
	if len(vars) != 1 || vars[0].Name != "NEW" {
		t.Fatalf("expected 1 var named NEW, got %+v", vars)
	}
	// Verify "name" field is preserved.
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	var name string
	if err := json.Unmarshal(m["name"], &name); err != nil {
		t.Fatalf("unmarshal name error: %v", err)
	}
	if name != "Test" {
		t.Fatalf("expected name %q, got %q", "Test", name)
	}
}

func TestSetEnvVarsPreservesUnknownFields(t *testing.T) {
	content := json.RawMessage(`{"name":"WF","description":"desc","custom_field":42,"environment_variables":[]}`)
	pt := "val"
	newVars := []CIEnvironmentVariable{
		{ID: "1", Name: "X", Value: CIEnvironmentVariableValue{Plaintext: &pt}},
	}
	result, err := SetEnvVars(content, newVars)
	if err != nil {
		t.Fatalf("SetEnvVars() error = %v", err)
	}
	// Verify all original fields are preserved.
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	for _, key := range []string{"name", "description", "custom_field", "environment_variables"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("expected key %q to be preserved, got keys: %v", key, keysOf(m))
		}
	}
	// Verify custom_field value is preserved.
	var customField int
	if err := json.Unmarshal(m["custom_field"], &customField); err != nil {
		t.Fatalf("unmarshal custom_field error: %v", err)
	}
	if customField != 42 {
		t.Fatalf("expected custom_field 42, got %d", customField)
	}
}

func TestSetEnvVarsRejectsNullContent(t *testing.T) {
	pt := "value"
	_, err := SetEnvVars(json.RawMessage(`null`), []CIEnvironmentVariable{
		{ID: "1", Name: "X", Value: CIEnvironmentVariableValue{Plaintext: &pt}},
	})
	if err == nil {
		t.Fatal("expected error for null workflow content")
	}
	if !strings.Contains(err.Error(), "expected JSON object") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractWorkflowConfig(t *testing.T) {
	content := json.RawMessage(`{
		"name":"Default",
		"description":"Main branch",
		"disabled":true,
		"locked":false,
		"xcode_version":"latest:all",
		"macos_version":"15",
		"start_conditions":[{"type":"branch"}],
		"actions":[{"name":"Archive"}],
		"post_actions":[{"name":"TestFlight"}],
		"clean":true,
		"container_file_path":"FoundationLab.xcodeproj",
		"repo":{"id":"repo-1"},
		"product_environment_variables":["var-1","var-2"]
	}`)

	cfg, err := ExtractWorkflowConfig(content)
	if err != nil {
		t.Fatalf("ExtractWorkflowConfig() error = %v", err)
	}

	if cfg.Name != "Default" || cfg.Description != "Main branch" {
		t.Fatalf("unexpected name/description: %+v", cfg)
	}
	if !cfg.Disabled || cfg.Locked {
		t.Fatalf("unexpected disabled/locked state: %+v", cfg)
	}
	var xcodeVersion string
	if err := json.Unmarshal(cfg.XcodeVersion, &xcodeVersion); err != nil {
		t.Fatalf("failed to decode xcode version: %v", err)
	}
	var macosVersion string
	if err := json.Unmarshal(cfg.MacOSVersion, &macosVersion); err != nil {
		t.Fatalf("failed to decode macos version: %v", err)
	}
	if xcodeVersion != "latest:all" || macosVersion != "15" {
		t.Fatalf("unexpected toolchain versions: xcode=%q macos=%q", xcodeVersion, macosVersion)
	}
	if len(cfg.ProductEnvironmentVariables) != 2 {
		t.Fatalf("expected 2 shared env var refs, got %d", len(cfg.ProductEnvironmentVariables))
	}
	if len(cfg.StartConditions) == 0 || len(cfg.Actions) == 0 || len(cfg.PostActions) == 0 || len(cfg.Repo) == 0 || len(cfg.Clean) == 0 {
		t.Fatalf("expected nested fields to be populated: %+v", cfg)
	}
}

func TestSetWorkflowDisabled(t *testing.T) {
	content := json.RawMessage(`{
		"name":"Default",
		"disabled":false,
		"custom_field":{"keep":true},
		"environment_variables":[{"id":"ev-1","name":"FOO","value":{"plaintext":"bar"}}]
	}`)

	result, err := SetWorkflowDisabled(content, true)
	if err != nil {
		t.Fatalf("SetWorkflowDisabled() error = %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("result unmarshal error: %v", err)
	}

	var disabled bool
	if err := json.Unmarshal(m["disabled"], &disabled); err != nil {
		t.Fatalf("disabled unmarshal error: %v", err)
	}
	if !disabled {
		t.Fatalf("expected disabled=true, got false")
	}

	if _, ok := m["custom_field"]; !ok {
		t.Fatalf("expected custom_field to be preserved")
	}
	if _, ok := m["environment_variables"]; !ok {
		t.Fatalf("expected environment_variables to be preserved")
	}
}

func TestSetWorkflowDisabledRejectsNullContent(t *testing.T) {
	_, err := SetWorkflowDisabled(json.RawMessage(`null`), true)
	if err == nil {
		t.Fatal("expected error for null workflow content")
	}
	if !strings.Contains(err.Error(), "expected JSON object") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyJSONMergePatch(t *testing.T) {
	content := json.RawMessage(`{
		"name":"Default",
		"description":"Main workflow",
		"clean":true,
		"start_conditions":{
			"branch":{
				"source":{"kind":"branch","value":"main"},
				"files":{
					"mode":"trigger_if_any_file_match",
					"matchers":[{"directory":"Sources","file_name":"App.swift"}]
				}
			}
		},
		"custom_field":{"keep":true}
	}`)
	patch := json.RawMessage(`{
		"description":"Updated workflow",
		"clean":false,
		"start_conditions":{
			"branch":{
				"files":{
					"matchers":[{"directory":"Sources","file_extension":"swift"}]
				}
			}
		}
	}`)

	result, changed, err := ApplyJSONMergePatch(content, patch)
	if err != nil {
		t.Fatalf("ApplyJSONMergePatch() error = %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	var got map[string]any
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("result unmarshal error: %v", err)
	}

	if got["description"] != "Updated workflow" {
		t.Fatalf("expected description to update, got %#v", got["description"])
	}
	if got["clean"] != false {
		t.Fatalf("expected clean=false, got %#v", got["clean"])
	}
	if _, ok := got["custom_field"]; !ok {
		t.Fatalf("expected custom_field to be preserved")
	}

	startConditions, ok := got["start_conditions"].(map[string]any)
	if !ok {
		t.Fatalf("expected start_conditions object, got %#v", got["start_conditions"])
	}
	branch, ok := startConditions["branch"].(map[string]any)
	if !ok {
		t.Fatalf("expected branch object, got %#v", startConditions["branch"])
	}
	source, ok := branch["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source object, got %#v", branch["source"])
	}
	if source["kind"] != "branch" || source["value"] != "main" {
		t.Fatalf("expected source to be preserved, got %#v", source)
	}

	files, ok := branch["files"].(map[string]any)
	if !ok {
		t.Fatalf("expected files object, got %#v", branch["files"])
	}
	if files["mode"] != "trigger_if_any_file_match" {
		t.Fatalf("expected files.mode to be preserved, got %#v", files["mode"])
	}
	matchers, ok := files["matchers"].([]any)
	if !ok || len(matchers) != 1 {
		t.Fatalf("expected 1 matcher, got %#v", files["matchers"])
	}
	matcher, ok := matchers[0].(map[string]any)
	if !ok {
		t.Fatalf("expected matcher object, got %#v", matchers[0])
	}
	if matcher["file_extension"] != "swift" {
		t.Fatalf("expected file_extension matcher, got %#v", matcher)
	}
}

func TestApplyJSONMergePatchRemovesFieldWithNull(t *testing.T) {
	content := json.RawMessage(`{"name":"Default","description":"Main workflow","clean":true}`)
	patch := json.RawMessage(`{"description":null}`)

	result, changed, err := ApplyJSONMergePatch(content, patch)
	if err != nil {
		t.Fatalf("ApplyJSONMergePatch() error = %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("result unmarshal error: %v", err)
	}
	if _, ok := got["description"]; ok {
		t.Fatalf("expected description to be removed")
	}
}

func TestApplyJSONMergePatchNoChange(t *testing.T) {
	content := json.RawMessage(`{"name":"Default","clean":true}`)
	patch := json.RawMessage(`{"clean":true}`)

	result, changed, err := ApplyJSONMergePatch(content, patch)
	if err != nil {
		t.Fatalf("ApplyJSONMergePatch() error = %v", err)
	}
	if changed {
		t.Fatal("expected changed=false")
	}
	if string(result) != `{"name":"Default","clean":true}` && string(result) != `{"clean":true,"name":"Default"}` {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestApplyJSONMergePatchRejectsNullContent(t *testing.T) {
	_, _, err := ApplyJSONMergePatch(json.RawMessage(`null`), json.RawMessage(`{"clean":false}`))
	if err == nil {
		t.Fatal("expected error for null workflow content")
	}
	if !strings.Contains(err.Error(), "expected JSON object") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyJSONMergePatchRejectsNonObjectPatch(t *testing.T) {
	_, _, err := ApplyJSONMergePatch(json.RawMessage(`{"name":"Default"}`), json.RawMessage(`[]`))
	if err == nil {
		t.Fatal("expected error for non-object workflow patch")
	}
	if !strings.Contains(err.Error(), "expected JSON object") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCIProductEnvVarsParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teams/team-uuid/products/prod-1/product-environment-variables" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": "var-1",
				"name": "SHARED_KEY",
				"value": {"plaintext": "shared-val"},
				"is_locked": false,
				"related_workflow_summaries": [
					{"id": "wf-1", "name": "Deploy", "disabled": false, "locked": false}
				]
			},
			{
				"id": "var-2",
				"name": "SHARED_SECRET",
				"value": {"redacted_value": ""},
				"is_locked": true,
				"related_workflow_summaries": []
			}
		]`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.ListCIProductEnvVars(context.Background(), "team-uuid", "prod-1")
	if err != nil {
		t.Fatalf("ListCIProductEnvVars() error = %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 vars, got %d", len(result))
	}
	if result[0].ID != "var-1" || result[0].Name != "SHARED_KEY" {
		t.Fatalf("unexpected first var: %+v", result[0])
	}
	if result[0].Value.Plaintext == nil || *result[0].Value.Plaintext != "shared-val" {
		t.Fatalf("expected plaintext value, got %+v", result[0].Value)
	}
	if result[0].IsLocked {
		t.Fatalf("expected is_locked=false for first var")
	}
	if len(result[0].RelatedWorkflowSummaries) != 1 || result[0].RelatedWorkflowSummaries[0].Name != "Deploy" {
		t.Fatalf("unexpected workflow summaries: %+v", result[0].RelatedWorkflowSummaries)
	}
	if result[1].Name != "SHARED_SECRET" || !result[1].IsLocked {
		t.Fatalf("unexpected second var: %+v", result[1])
	}
	if result[1].Value.RedactedValue == nil {
		t.Fatalf("expected redacted_value for second var")
	}
}

func TestListCIProductEnvVarsEmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.ListCIProductEnvVars(context.Background(), "team-uuid", "prod-1")
	if err != nil {
		t.Fatalf("ListCIProductEnvVars() error = %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 vars, got %d", len(result))
	}
}

func TestListCIProductEnvVarsRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name      string
		teamID    string
		productID string
		wantErr   string
	}{
		{"empty team", "", "prod", "team id is required"},
		{"empty product", "team", "", "product id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.ListCIProductEnvVars(context.Background(), tt.teamID, tt.productID)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestSetCIProductEnvVarSendsBody(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "var-1",
			"name": "MY_VAR",
			"value": {"plaintext": "hello"},
			"is_locked": false,
			"related_workflow_summaries": []
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	pt := "hello"
	req := CIProductEnvVarRequest{
		Name:        "MY_VAR",
		Value:       CIEnvironmentVariableValue{Plaintext: &pt},
		IsLocked:    false,
		WorkflowIDs: []string{"wf-1"},
	}
	result, err := client.SetCIProductEnvVar(context.Background(), "team-uuid", "prod-1", "var-1", req)
	if err != nil {
		t.Fatalf("SetCIProductEnvVar() error = %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("expected PUT, got %s", gotMethod)
	}
	if gotPath != "/teams/team-uuid/products/prod-1/product-environment-variables/var-1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if !strings.Contains(string(gotBody), `"name":"MY_VAR"`) {
		t.Fatalf("expected name in body, got %s", gotBody)
	}
	if !strings.Contains(string(gotBody), `"workflow_ids":["wf-1"]`) {
		t.Fatalf("expected workflow_ids in body, got %s", gotBody)
	}
	if result.ID != "var-1" || result.Name != "MY_VAR" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSetCIProductEnvVarRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name      string
		teamID    string
		productID string
		varID     string
		wantErr   string
	}{
		{"empty team", "", "prod", "var", "team id is required"},
		{"empty product", "team", "", "var", "product id is required"},
		{"empty var", "team", "prod", "", "variable id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.SetCIProductEnvVar(context.Background(), tt.teamID, tt.productID, tt.varID, CIProductEnvVarRequest{})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestDeleteCIProductEnvVar(t *testing.T) {
	var gotMethod string
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	err := client.DeleteCIProductEnvVar(context.Background(), "team-uuid", "prod-1", "var-1")
	if err != nil {
		t.Fatalf("DeleteCIProductEnvVar() error = %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/teams/team-uuid/products/prod-1/product-environment-variables/var-1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
}

func TestDeleteCIProductEnvVarRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name      string
		teamID    string
		productID string
		varID     string
		wantErr   string
	}{
		{"empty team", "", "prod", "var", "team id is required"},
		{"empty product", "team", "", "var", "product id is required"},
		{"empty var", "team", "prod", "", "variable id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.DeleteCIProductEnvVar(context.Background(), tt.teamID, tt.productID, tt.varID)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestGetCIConfigurationOptions(t *testing.T) {
	client := testWebClientWithHandler(t, func(r *http.Request) *http.Response {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/teams/team-uuid/configuration-options-v10" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonTestResponse(r, `{"default_timezone":{"id":"Asia/Calcutta"}}`)
	})
	result, err := client.GetCIConfigurationOptions(context.Background(), "team-uuid")
	if err != nil {
		t.Fatalf("GetCIConfigurationOptions() error = %v", err)
	}
	if string(result) != `{"default_timezone":{"id":"Asia/Calcutta"}}` {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestGetCIConfigurationOptionsRejectsEmptyTeam(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	_, err := client.GetCIConfigurationOptions(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "team id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetCIBuildVersions(t *testing.T) {
	client := testWebClientWithHandler(t, func(r *http.Request) *http.Response {
		if r.URL.Path != "/teams/team-uuid/configuration-options/build-versions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonTestResponse(r, `{"items":[{"build":"2026.03.14.1"}]}`)
	})
	result, err := client.GetCIBuildVersions(context.Background(), "team-uuid")
	if err != nil {
		t.Fatalf("GetCIBuildVersions() error = %v", err)
	}
	if string(result) != `{"items":[{"build":"2026.03.14.1"}]}` {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestGetCIProductConfigurationOptions(t *testing.T) {
	client := testWebClientWithHandler(t, func(r *http.Request) *http.Response {
		if r.URL.Path != "/teams/team-uuid/products/prod-1/product-configuration-options-v4" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonTestResponse(r, `{"repo_default_recommendation":"repo-1"}`)
	})
	result, err := client.GetCIProductConfigurationOptions(context.Background(), "team-uuid", "prod-1")
	if err != nil {
		t.Fatalf("GetCIProductConfigurationOptions() error = %v", err)
	}
	if string(result) != `{"repo_default_recommendation":"repo-1"}` {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestGetCIProductConfigurationOptionsRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name      string
		teamID    string
		productID string
		wantErr   string
	}{
		{"empty team", "", "prod-1", "team id is required"},
		{"empty product", "team-uuid", "", "product id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetCIProductConfigurationOptions(context.Background(), tt.teamID, tt.productID)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetCISchemes(t *testing.T) {
	client := testWebClientWithHandler(t, func(r *http.Request) *http.Response {
		if r.URL.Path != "/teams/team-uuid/products/prod-1/schemes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("container_file_path") != "Focus Rail.xcodeproj" {
			t.Fatalf("unexpected container_file_path: %q", r.URL.Query().Get("container_file_path"))
		}
		if r.URL.Query().Get("limit") != "20" {
			t.Fatalf("unexpected limit: %q", r.URL.Query().Get("limit"))
		}
		if r.URL.Query().Get("continuation_offset") != "next-1" {
			t.Fatalf("unexpected continuation_offset: %q", r.URL.Query().Get("continuation_offset"))
		}
		return jsonTestResponse(r, `{"items":[{"name":"Focus Rail"}]}`)
	})
	result, err := client.GetCISchemes(context.Background(), "team-uuid", "prod-1", "Focus Rail.xcodeproj", 20, "next-1")
	if err != nil {
		t.Fatalf("GetCISchemes() error = %v", err)
	}
	if string(result) != `{"items":[{"name":"Focus Rail"}]}` {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestGetCISchemesRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name      string
		teamID    string
		productID string
		wantErr   string
	}{
		{"empty team", "", "prod-1", "team id is required"},
		{"empty product", "team-uuid", "", "product id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetCISchemes(context.Background(), tt.teamID, tt.productID, "", 0, "")
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetCISchemesRejectsNegativeLimit(t *testing.T) {
	client := testWebClientWithHandler(t, func(r *http.Request) *http.Response {
		t.Fatal("did not expect request for negative limit")
		return nil
	})

	_, err := client.GetCISchemes(context.Background(), "team-uuid", "prod-1", "", -1, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "limit must be zero or greater") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetCITestDestinations(t *testing.T) {
	client := testWebClientWithHandler(t, func(r *http.Request) *http.Response {
		if r.URL.Path != "/teams/team-uuid/test-destinations-v3" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("xcode_version") != "latest:stable" {
			t.Fatalf("unexpected xcode_version: %q", r.URL.Query().Get("xcode_version"))
		}
		return jsonTestResponse(r, `{"platform_test_destinations":[]}`)
	})
	result, err := client.GetCITestDestinations(context.Background(), "team-uuid", "latest:stable")
	if err != nil {
		t.Fatalf("GetCITestDestinations() error = %v", err)
	}
	if string(result) != `{"platform_test_destinations":[]}` {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestGetCITestDestinationsRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name         string
		teamID       string
		xcodeVersion string
		wantErr      string
	}{
		{"empty team", "", "latest:stable", "team id is required"},
		{"empty xcode version", "team-uuid", "", "xcode version is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetCITestDestinations(context.Background(), tt.teamID, tt.xcodeVersion)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetCISlackProvider(t *testing.T) {
	client := testWebClientWithHandler(t, func(r *http.Request) *http.Response {
		if r.URL.Path != "/teams/team-uuid/integrations/slack" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonTestResponse(r, `{"is_user_connected":true,"workspace_name":"Build Ops"}`)
	})
	result, err := client.GetCISlackProvider(context.Background(), "team-uuid")
	if err != nil {
		t.Fatalf("GetCISlackProvider() error = %v", err)
	}
	if string(result) != `{"is_user_connected":true,"workspace_name":"Build Ops"}` {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestGetCISlackChannels(t *testing.T) {
	client := testWebClientWithHandler(t, func(r *http.Request) *http.Response {
		if r.URL.Path != "/teams/team-uuid/integrations/slack/channels" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonTestResponse(r, `[{"id":"chan-1","channel_name":"builds"}]`)
	})
	result, err := client.GetCISlackChannels(context.Background(), "team-uuid")
	if err != nil {
		t.Fatalf("GetCISlackChannels() error = %v", err)
	}
	if string(result) != `[{"id":"chan-1","channel_name":"builds"}]` {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestGetCISlackEndpointsRejectEmptyTeam(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name string
		call func() (json.RawMessage, error)
	}{
		{
			name: "provider",
			call: func() (json.RawMessage, error) {
				return client.GetCISlackProvider(context.Background(), "")
			},
		},
		{
			name: "channels",
			call: func() (json.RawMessage, error) {
				return client.GetCISlackChannels(context.Background(), "")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.call()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "team id is required") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

type jsonTestRoundTripper func(*http.Request) *http.Response

func (fn jsonTestRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r), nil
}

func testWebClientWithHandler(t *testing.T, fn func(*http.Request) *http.Response) *Client {
	t.Helper()

	return &Client{
		httpClient: &http.Client{Transport: jsonTestRoundTripper(fn)},
		baseURL:    "https://example.test",
	}
}

func jsonTestResponse(r *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
