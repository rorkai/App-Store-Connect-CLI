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

func TestGetGameCenterMatchmakingQueues_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingQueues" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingQueues, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterMatchmakingQueues(context.Background(), WithGCMatchmakingQueuesLimit(10)); err != nil {
		t.Fatalf("GetGameCenterMatchmakingQueues() error: %v", err)
	}
}

func TestGetGameCenterMatchmakingQueues_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterMatchmakingQueues?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterMatchmakingQueues(context.Background(), WithGCMatchmakingQueuesNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterMatchmakingQueues() error: %v", err)
	}
}

func TestGetGameCenterMatchmakingQueue(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterMatchmakingQueues","id":"queue-1","attributes":{"referenceName":"Primary"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingQueues/queue-1" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingQueues/queue-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterMatchmakingQueue(context.Background(), "queue-1"); err != nil {
		t.Fatalf("GetGameCenterMatchmakingQueue() error: %v", err)
	}
}

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

func TestDeleteGameCenterMatchmakingQueue(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingQueues/queue-1" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingQueues/queue-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterMatchmakingQueue(context.Background(), "queue-1"); err != nil {
		t.Fatalf("DeleteGameCenterMatchmakingQueue() error: %v", err)
	}
}

func TestGetGameCenterMatchmakingRuleSets_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRuleSets" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRuleSets, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterMatchmakingRuleSets(context.Background(), WithGCMatchmakingRuleSetsLimit(5)); err != nil {
		t.Fatalf("GetGameCenterMatchmakingRuleSets() error: %v", err)
	}
}

func TestGetGameCenterMatchmakingRuleSet(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterMatchmakingRuleSets","id":"rule-set-1","attributes":{"referenceName":"Rules"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRuleSets/rule-set-1" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRuleSets/rule-set-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterMatchmakingRuleSet(context.Background(), "rule-set-1"); err != nil {
		t.Fatalf("GetGameCenterMatchmakingRuleSet() error: %v", err)
	}
}

func TestCreateGameCenterMatchmakingRuleSet(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterMatchmakingRuleSets","id":"rule-set-1","attributes":{"referenceName":"Rules"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRuleSets" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRuleSets, got %s", req.URL.Path)
		}
		var payload GameCenterMatchmakingRuleSetCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterMatchmakingRuleSets {
			t.Fatalf("expected type gameCenterMatchmakingRuleSets, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.ReferenceName != "Rules" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterMatchmakingRuleSetCreateAttributes{
		ReferenceName:       "Rules",
		RuleLanguageVersion: 1,
		MinPlayers:          2,
		MaxPlayers:          8,
	}
	if _, err := client.CreateGameCenterMatchmakingRuleSet(context.Background(), attrs); err != nil {
		t.Fatalf("CreateGameCenterMatchmakingRuleSet() error: %v", err)
	}
}

func TestUpdateGameCenterMatchmakingRuleSet(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterMatchmakingRuleSets","id":"rule-set-1","attributes":{"referenceName":"Rules"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRuleSets/rule-set-1" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRuleSets/rule-set-1, got %s", req.URL.Path)
		}
		var payload GameCenterMatchmakingRuleSetUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "rule-set-1" || payload.Data.Type != ResourceTypeGameCenterMatchmakingRuleSets {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.MinPlayers == nil || *payload.Data.Attributes.MinPlayers != 3 {
			t.Fatalf("expected minPlayers update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	minPlayers := 3
	attrs := GameCenterMatchmakingRuleSetUpdateAttributes{MinPlayers: &minPlayers}
	if _, err := client.UpdateGameCenterMatchmakingRuleSet(context.Background(), "rule-set-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterMatchmakingRuleSet() error: %v", err)
	}
}

func TestDeleteGameCenterMatchmakingRuleSet(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRuleSets/rule-set-1" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRuleSets/rule-set-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterMatchmakingRuleSet(context.Background(), "rule-set-1"); err != nil {
		t.Fatalf("DeleteGameCenterMatchmakingRuleSet() error: %v", err)
	}
}

func TestGetGameCenterMatchmakingRules_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRuleSets/rule-set-1/rules" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRuleSets/rule-set-1/rules, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "4" {
			t.Fatalf("expected limit=4, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterMatchmakingRules(context.Background(), "rule-set-1", WithGCMatchmakingRulesLimit(4)); err != nil {
		t.Fatalf("GetGameCenterMatchmakingRules() error: %v", err)
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

func TestUpdateGameCenterMatchmakingRule(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterMatchmakingRules","id":"rule-1","attributes":{"referenceName":"Rule"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRules/rule-1" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRules/rule-1, got %s", req.URL.Path)
		}
		var payload GameCenterMatchmakingRuleUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "rule-1" || payload.Data.Type != ResourceTypeGameCenterMatchmakingRules {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Expression == nil || *payload.Data.Attributes.Expression != "false" {
			t.Fatalf("expected expression update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	expression := "false"
	attrs := GameCenterMatchmakingRuleUpdateAttributes{Expression: &expression}
	if _, err := client.UpdateGameCenterMatchmakingRule(context.Background(), "rule-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterMatchmakingRule() error: %v", err)
	}
}

func TestDeleteGameCenterMatchmakingRule(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRules/rule-1" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRules/rule-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterMatchmakingRule(context.Background(), "rule-1"); err != nil {
		t.Fatalf("DeleteGameCenterMatchmakingRule() error: %v", err)
	}
}

func TestGetGameCenterMatchmakingTeams_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRuleSets/rule-set-1/teams" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRuleSets/rule-set-1/teams, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "8" {
			t.Fatalf("expected limit=8, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterMatchmakingTeams(context.Background(), "rule-set-1", WithGCMatchmakingTeamsLimit(8)); err != nil {
		t.Fatalf("GetGameCenterMatchmakingTeams() error: %v", err)
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

func TestUpdateGameCenterMatchmakingTeam(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterMatchmakingTeams","id":"team-1","attributes":{"referenceName":"Team"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingTeams/team-1" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingTeams/team-1, got %s", req.URL.Path)
		}
		var payload GameCenterMatchmakingTeamUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "team-1" || payload.Data.Type != ResourceTypeGameCenterMatchmakingTeams {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.MinPlayers == nil || *payload.Data.Attributes.MinPlayers != 2 {
			t.Fatalf("expected minPlayers update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	minPlayers := 2
	attrs := GameCenterMatchmakingTeamUpdateAttributes{MinPlayers: &minPlayers}
	if _, err := client.UpdateGameCenterMatchmakingTeam(context.Background(), "team-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterMatchmakingTeam() error: %v", err)
	}
}

func TestDeleteGameCenterMatchmakingTeam(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingTeams/team-1" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingTeams/team-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterMatchmakingTeam(context.Background(), "team-1"); err != nil {
		t.Fatalf("DeleteGameCenterMatchmakingTeam() error: %v", err)
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
		WithGCMatchmakingMetricsLimit(30),
	}
	if _, err := client.GetGameCenterMatchmakingQueueRequests(context.Background(), "queue-1", opts...); err != nil {
		t.Fatalf("GetGameCenterMatchmakingQueueRequests() error: %v", err)
	}
}

func TestGameCenterMatchmakingMetricsPaths(t *testing.T) {
	tests := []struct {
		name     string
		call     func(*Client) error
		wantPath string
	}{
		{
			name: "queue sizes",
			call: func(c *Client) error {
				_, err := c.GetGameCenterMatchmakingQueueSizes(context.Background(), "queue-1")
				return err
			},
			wantPath: "/v1/gameCenterMatchmakingQueues/queue-1/metrics/matchmakingQueueSizes",
		},
		{
			name: "queue sessions",
			call: func(c *Client) error {
				_, err := c.GetGameCenterMatchmakingQueueSessions(context.Background(), "queue-1")
				return err
			},
			wantPath: "/v1/gameCenterMatchmakingQueues/queue-1/metrics/matchmakingSessions",
		},
		{
			name: "experiment sizes",
			call: func(c *Client) error {
				_, err := c.GetGameCenterMatchmakingQueueExperimentSizes(context.Background(), "queue-1")
				return err
			},
			wantPath: "/v1/gameCenterMatchmakingQueues/queue-1/metrics/experimentMatchmakingQueueSizes",
		},
		{
			name: "experiment requests",
			call: func(c *Client) error {
				_, err := c.GetGameCenterMatchmakingQueueExperimentRequests(context.Background(), "queue-1")
				return err
			},
			wantPath: "/v1/gameCenterMatchmakingQueues/queue-1/metrics/experimentMatchmakingRequests",
		},
		{
			name: "boolean rule results",
			call: func(c *Client) error {
				_, err := c.GetGameCenterMatchmakingBooleanRuleResults(context.Background(), "rule-1")
				return err
			},
			wantPath: "/v1/gameCenterMatchmakingRules/rule-1/metrics/matchmakingBooleanRuleResults",
		},
		{
			name: "number rule results",
			call: func(c *Client) error {
				_, err := c.GetGameCenterMatchmakingNumberRuleResults(context.Background(), "rule-1")
				return err
			},
			wantPath: "/v1/gameCenterMatchmakingRules/rule-1/metrics/matchmakingNumberRuleResults",
		},
		{
			name: "rule errors",
			call: func(c *Client) error {
				_, err := c.GetGameCenterMatchmakingRuleErrors(context.Background(), "rule-1")
				return err
			},
			wantPath: "/v1/gameCenterMatchmakingRules/rule-1/metrics/matchmakingRuleErrors",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := jsonResponse(http.StatusOK, `{"data":[]}`)
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", req.Method)
				}
				if req.URL.Path != test.wantPath {
					t.Fatalf("expected path %s, got %s", test.wantPath, req.URL.Path)
				}
				assertAuthorized(t, req)
			}, response)

			if err := test.call(client); err != nil {
				t.Fatalf("call error: %v", err)
			}
		})
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

func TestGetGameCenterMatchmakingRuleSetQueues_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterMatchmakingRuleSets/rules-1/matchmakingQueues" {
			t.Fatalf("expected path /v1/gameCenterMatchmakingRuleSets/rules-1/matchmakingQueues, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "22" {
			t.Fatalf("expected limit=22, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterMatchmakingRuleSetQueues(context.Background(), "rules-1", WithGCMatchmakingQueuesLimit(22)); err != nil {
		t.Fatalf("GetGameCenterMatchmakingRuleSetQueues() error: %v", err)
	}
}

func TestGetGameCenterMatchmakingRuleSetQueues_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterMatchmakingRuleSets/rules-1/matchmakingQueues?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterMatchmakingRuleSetQueues(context.Background(), "rules-1", WithGCMatchmakingQueuesNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterMatchmakingRuleSetQueues() error: %v", err)
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
	WithGCMatchmakingQueuesLimit(12)(query)
	if query.limit != 12 {
		t.Fatalf("expected limit 12, got %d", query.limit)
	}
	WithGCMatchmakingQueuesNextURL("next")(query)
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
	WithGCMatchmakingRuleSetsLimit(3)(query)
	if query.limit != 3 {
		t.Fatalf("expected limit 3, got %d", query.limit)
	}
	WithGCMatchmakingRuleSetsNextURL("next")(query)
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
	WithGCMatchmakingRulesLimit(7)(query)
	if query.limit != 7 {
		t.Fatalf("expected limit 7, got %d", query.limit)
	}
	WithGCMatchmakingRulesNextURL("next")(query)
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
	WithGCMatchmakingTeamsLimit(9)(query)
	if query.limit != 9 {
		t.Fatalf("expected limit 9, got %d", query.limit)
	}
	WithGCMatchmakingTeamsNextURL("next")(query)
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
	WithGCMatchmakingMetricsLimit(20)(query)
	WithGCMatchmakingMetricsNextURL("next")(query)

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
