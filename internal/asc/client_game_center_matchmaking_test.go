package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
)

func TestCreateGameCenterMatchmakingQueue(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterMatchmakingQueues","id":"queue-1","attributes":{"referenceName":"Primary"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingQueues" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingQueues, got %s", req.URL.Path)
		}
		var payload GameCenterMatchmakingQueueCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterMatchmakingQueues {
			t.Fatalf("expected type gameCenterMatchmakingQueues, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.RuleSet == nil {
			t.Fatalf("expected rule set relationship")
		}
		if payload.Data.Relationships.RuleSet.Data.ID != "rule-set-1" {
			t.Fatalf("expected rule-set-1, got %s", payload.Data.Relationships.RuleSet.Data.ID)
		}
		if payload.Data.Relationships.ExperimentRuleSet == nil || payload.Data.Relationships.ExperimentRuleSet.Data.ID != "rule-set-2" {
			t.Fatalf("expected experiment rule set rule-set-2")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterMatchmakingQueueCreateAttributes{
		ReferenceName: "Primary",
	}
	if _, err := client.CreateGameCenterMatchmakingQueue(context.Background(), attrs, "rule-set-1", "rule-set-2"); err != nil {
		t.Fatalf("CreateGameCenterMatchmakingQueue() error: %v", err)
	}
}

func TestUpdateGameCenterMatchmakingQueue(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterMatchmakingQueues","id":"queue-1","attributes":{"referenceName":"Primary"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingQueues/queue-1" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingQueues/queue-1, got %s", req.URL.Path)
		}
		var payload GameCenterMatchmakingQueueUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "queue-1" || payload.Data.Type != ResourceTypeGameCenterMatchmakingQueues {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.RuleSet == nil {
			t.Fatalf("expected rule set relationship")
		}
		if payload.Data.Relationships.RuleSet.Data.ID != "rule-set-1" {
			t.Fatalf("expected rule-set-1, got %s", payload.Data.Relationships.RuleSet.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterMatchmakingQueueUpdateAttributes{
		ClassicMatchmakingBundleIDs: []string{"com.example.bundle"},
	}
	if _, err := client.UpdateGameCenterMatchmakingQueue(context.Background(), "queue-1", attrs, "rule-set-1", ""); err != nil {
		t.Fatalf("UpdateGameCenterMatchmakingQueue() error: %v", err)
	}
}

func TestCreateGameCenterMatchmakingRule(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterMatchmakingRules","id":"rule-1","attributes":{"referenceName":"Rule"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRules" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRules, got %s", req.URL.Path)
		}
		var payload GameCenterMatchmakingRuleCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterMatchmakingRules {
			t.Fatalf("expected type gameCenterMatchmakingRules, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.RuleSet == nil || payload.Data.Relationships.RuleSet.Data.ID != "rule-set-1" {
			t.Fatalf("expected rule set relationship rule-set-1")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterMatchmakingRuleCreateAttributes{
		ReferenceName: "Rule",
		Description:   "Rule desc",
		Type:          "BOOLEAN",
		Expression:    "true",
	}
	if _, err := client.CreateGameCenterMatchmakingRule(context.Background(), "rule-set-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterMatchmakingRule() error: %v", err)
	}
}

func TestCreateGameCenterMatchmakingTeam(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterMatchmakingTeams","id":"team-1","attributes":{"referenceName":"Team"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingTeams" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingTeams, got %s", req.URL.Path)
		}
		var payload GameCenterMatchmakingTeamCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterMatchmakingTeams {
			t.Fatalf("expected type gameCenterMatchmakingTeams, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.RuleSet == nil || payload.Data.Relationships.RuleSet.Data.ID != "rule-set-1" {
			t.Fatalf("expected rule set relationship rule-set-1")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterMatchmakingTeamCreateAttributes{
		ReferenceName: "Team",
		MinPlayers:    1,
		MaxPlayers:    4,
	}
	if _, err := client.CreateGameCenterMatchmakingTeam(context.Background(), "rule-set-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterMatchmakingTeam() error: %v", err)
	}
}

func TestGetGameCenterMatchmakingQueueRequests_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		values := req.URL.Query()
		if values.Get("granularity") != "P1D" {
			t.Fatalf("expected granularity=P1D, got %q", values.Get("granularity"))
		}
		if values.Get("groupBy") != "result" {
			t.Fatalf("expected groupBy=result, got %q", values.Get("groupBy"))
		}
		if values.Get("filter[result]") != "MATCHED" {
			t.Fatalf("expected filter[result]=MATCHED, got %q", values.Get("filter[result]"))
		}
		if values.Get("filter[gameCenterDetail]") != "gc-1" {
			t.Fatalf("expected filter[gameCenterDetail]=gc-1, got %q", values.Get("filter[gameCenterDetail]"))
		}
		if values.Get("sort") != "-count" {
			t.Fatalf("expected sort=-count, got %q", values.Get("sort"))
		}
		if values.Get("limit") != "30" {
			t.Fatalf("expected limit=30, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	opts := []GCMatchmakingMetricsOption{
		WithGCMatchmakingMetricsGranularity("P1D"),
		WithGCMatchmakingMetricsGroupBy([]string{"result"}),
		WithGCMatchmakingMetricsFilterResult("MATCHED"),
		WithGCMatchmakingMetricsFilterGameCenterDetail("gc-1"),
		WithGCMatchmakingMetricsSort([]string{"-count"}),
		Limit[GCMatchmakingMetricsOption](30),
	}
	if _, err := client.GetGameCenterMatchmakingQueueRequests(context.Background(), "queue-1", opts...); err != nil {
		t.Fatalf("GetGameCenterMatchmakingQueueRequests() error: %v", err)
	}
}

func TestCreateGameCenterMatchmakingRuleSetTest_EmptyPayload(t *testing.T) {
	client := &Client{}
	if _, err := client.CreateGameCenterMatchmakingRuleSetTest(context.Background(), nil); err == nil {
		t.Fatalf("expected error for empty payload")
	}
}

func TestCreateGameCenterMatchmakingRuleSetTest(t *testing.T) {
	payload := json.RawMessage(`{"data":{"type":"gameCenterMatchmakingRuleSetTests"}}`)
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterMatchmakingRuleSetTests","id":"test-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRuleSetTests" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRuleSetTests, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !bytes.Equal(bytes.TrimSpace(body), payload) {
			t.Fatalf("unexpected payload: %s", string(body))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterMatchmakingRuleSetTest(context.Background(), payload); err != nil {
		t.Fatalf("CreateGameCenterMatchmakingRuleSetTest() error: %v", err)
	}
}

func TestGameCenterMetricsResponseAccessors(t *testing.T) {
	resp := GameCenterMetricsResponse{
		Data: []GameCenterMetricsData{{Granularity: "P1D"}},
		Links: Links{
			Self: "https://example.com",
		},
	}
	if resp.GetLinks() == nil || resp.GetLinks().Self != "https://example.com" {
		t.Fatalf("unexpected links: %+v", resp.GetLinks())
	}
	data, ok := resp.GetData().([]GameCenterMetricsData)
	if !ok {
		t.Fatalf("expected []GameCenterMetricsData")
	}
	if len(data) != 1 || data[0].Granularity != "P1D" {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestGCMatchmakingQueuesOptions(t *testing.T) {
	query := &gcMatchmakingQueuesQuery{}
	Limit[GCMatchmakingQueuesOption](12)(query)
	if query.limit != 12 {
		t.Fatalf("expected limit 12, got %d", query.limit)
	}
	NextURL[GCMatchmakingQueuesOption]("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCMatchmakingQueuesQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "12" {
		t.Fatalf("expected limit=12, got %q", values.Get("limit"))
	}
}

func TestGCMatchmakingRuleSetsOptions(t *testing.T) {
	query := &gcMatchmakingRuleSetsQuery{}
	Limit[GCMatchmakingRuleSetsOption](3)(query)
	if query.limit != 3 {
		t.Fatalf("expected limit 3, got %d", query.limit)
	}
	NextURL[GCMatchmakingRuleSetsOption]("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCMatchmakingRuleSetsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "3" {
		t.Fatalf("expected limit=3, got %q", values.Get("limit"))
	}
}

func TestGCMatchmakingRulesOptions(t *testing.T) {
	query := &gcMatchmakingRulesQuery{}
	Limit[GCMatchmakingRulesOption](7)(query)
	if query.limit != 7 {
		t.Fatalf("expected limit 7, got %d", query.limit)
	}
	NextURL[GCMatchmakingRulesOption]("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCMatchmakingRulesQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "7" {
		t.Fatalf("expected limit=7, got %q", values.Get("limit"))
	}
}

func TestGCMatchmakingTeamsOptions(t *testing.T) {
	query := &gcMatchmakingTeamsQuery{}
	Limit[GCMatchmakingTeamsOption](9)(query)
	if query.limit != 9 {
		t.Fatalf("expected limit 9, got %d", query.limit)
	}
	NextURL[GCMatchmakingTeamsOption]("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCMatchmakingTeamsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "9" {
		t.Fatalf("expected limit=9, got %q", values.Get("limit"))
	}
}

func TestGCMatchmakingMetricsOptionsAndQueries(t *testing.T) {
	query := &gcMatchmakingMetricsQuery{}
	WithGCMatchmakingMetricsGranularity("P1D")(query)
	WithGCMatchmakingMetricsSort([]string{"-count"})(query)
	WithGCMatchmakingMetricsGroupBy([]string{"result"})(query)
	WithGCMatchmakingMetricsFilterResult("MATCHED")(query)
	WithGCMatchmakingMetricsFilterGameCenterDetail("gc-1")(query)
	WithGCMatchmakingMetricsFilterQueue("queue-1")(query)
	Limit[GCMatchmakingMetricsOption](20)(query)
	NextURL[GCMatchmakingMetricsOption]("next")(query)

	if query.granularity != "P1D" {
		t.Fatalf("expected granularity P1D, got %q", query.granularity)
	}
	if query.filterGameCenterDetail != "gc-1" || query.filterGameCenterMatchmakingQueue != "queue-1" {
		t.Fatalf("unexpected filters: %+v", query)
	}

	queueValues, err := url.ParseQuery(buildGCMatchmakingQueueSizesQuery(query))
	if err != nil {
		t.Fatalf("parse queue sizes query: %v", err)
	}
	if queueValues.Get("granularity") != "P1D" {
		t.Fatalf("expected granularity P1D, got %q", queueValues.Get("granularity"))
	}
	if queueValues.Get("sort") != "-count" {
		t.Fatalf("expected sort -count, got %q", queueValues.Get("sort"))
	}

	requestValues, err := url.ParseQuery(buildGCMatchmakingQueueRequestsQuery(query))
	if err != nil {
		t.Fatalf("parse queue requests query: %v", err)
	}
	if requestValues.Get("groupBy") != "result" {
		t.Fatalf("expected groupBy result, got %q", requestValues.Get("groupBy"))
	}
	if requestValues.Get("filter[result]") != "MATCHED" {
		t.Fatalf("expected filter[result]=MATCHED, got %q", requestValues.Get("filter[result]"))
	}
	if requestValues.Get("filter[gameCenterDetail]") != "gc-1" {
		t.Fatalf("expected filter[gameCenterDetail]=gc-1, got %q", requestValues.Get("filter[gameCenterDetail]"))
	}

	ruleValues, err := url.ParseQuery(buildGCMatchmakingRuleMetricsQuery(query))
	if err != nil {
		t.Fatalf("parse rule metrics query: %v", err)
	}
	if ruleValues.Get("filter[gameCenterMatchmakingQueue]") != "queue-1" {
		t.Fatalf("expected filter[gameCenterMatchmakingQueue]=queue-1, got %q", ruleValues.Get("filter[gameCenterMatchmakingQueue]"))
	}
}
