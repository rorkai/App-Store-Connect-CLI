package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

func TestGetGameCenterGroups_WithLimitAndFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups" {
			t.Fatalf("expected path /v1/gameCenterGroups, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "15" {
			t.Fatalf("expected limit=15, got %q", values.Get("limit"))
		}
		if values.Get("filter[gameCenterDetails]") != "gc-1,gc-2" {
			t.Fatalf("expected filter[gameCenterDetails], got %q", values.Get("filter[gameCenterDetails]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroups(context.Background(), WithGCGroupsLimit(15), WithGCGroupsGameCenterDetailIDs([]string{"gc-1", "gc-2"})); err != nil {
		t.Fatalf("GetGameCenterGroups() error: %v", err)
	}
}

func TestGetGameCenterGroups_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterGroups?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroups(context.Background(), WithGCGroupsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterGroups() error: %v", err)
	}
}

func TestGetGameCenterGroup(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterGroups","id":"group-1","attributes":{"referenceName":"Group"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups/group-1" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroup(context.Background(), "group-1"); err != nil {
		t.Fatalf("GetGameCenterGroup() error: %v", err)
	}
}

func TestCreateGameCenterGroup(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterGroups","id":"group-1","attributes":{"referenceName":"Group"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups" {
			t.Fatalf("expected path /v1/gameCenterGroups, got %s", req.URL.Path)
		}
		var payload GameCenterGroupCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterGroups {
			t.Fatalf("expected type gameCenterGroups, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.ReferenceName == nil || *payload.Data.Attributes.ReferenceName != "Group" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Group"
	if _, err := client.CreateGameCenterGroup(context.Background(), &name); err != nil {
		t.Fatalf("CreateGameCenterGroup() error: %v", err)
	}
}

func TestUpdateGameCenterGroup(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterGroups","id":"group-1","attributes":{"referenceName":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups/group-1" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1, got %s", req.URL.Path)
		}
		var payload GameCenterGroupUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "group-1" || payload.Data.Type != ResourceTypeGameCenterGroups {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.ReferenceName == nil || *payload.Data.Attributes.ReferenceName != "Updated" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	if _, err := client.UpdateGameCenterGroup(context.Background(), "group-1", &name); err != nil {
		t.Fatalf("UpdateGameCenterGroup() error: %v", err)
	}
}

func TestDeleteGameCenterGroup(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups/group-1" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterGroup(context.Background(), "group-1"); err != nil {
		t.Fatalf("DeleteGameCenterGroup() error: %v", err)
	}
}

func TestUpdateGameCenterGroupAchievements(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups/group-1/relationships/gameCenterAchievements" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/relationships/gameCenterAchievements, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 relationships, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeGameCenterAchievements {
			t.Fatalf("expected type gameCenterAchievements, got %q", payload.Data[0].Type)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.UpdateGameCenterGroupAchievements(context.Background(), "group-1", []string{"ach-1", "ach-2"}); err != nil {
		t.Fatalf("UpdateGameCenterGroupAchievements() error: %v", err)
	}
}

func TestUpdateGameCenterGroupAchievementsV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups/group-1/relationships/gameCenterAchievementsV2" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/relationships/gameCenterAchievementsV2, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(payload.Data) != 1 || payload.Data[0].ID != "ach-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.UpdateGameCenterGroupAchievementsV2(context.Background(), "group-1", []string{"ach-1"}); err != nil {
		t.Fatalf("UpdateGameCenterGroupAchievementsV2() error: %v", err)
	}
}

func TestUpdateGameCenterGroupLeaderboards(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups/group-1/relationships/gameCenterLeaderboards" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/relationships/gameCenterLeaderboards, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(payload.Data) != 1 || payload.Data[0].ID != "lb-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.UpdateGameCenterGroupLeaderboards(context.Background(), "group-1", []string{"lb-1"}); err != nil {
		t.Fatalf("UpdateGameCenterGroupLeaderboards() error: %v", err)
	}
}

func TestUpdateGameCenterGroupLeaderboardsV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups/group-1/relationships/gameCenterLeaderboardsV2" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/relationships/gameCenterLeaderboardsV2, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.UpdateGameCenterGroupLeaderboardsV2(context.Background(), "group-1", []string{"lb-1", "lb-2"}); err != nil {
		t.Fatalf("UpdateGameCenterGroupLeaderboardsV2() error: %v", err)
	}
}

func TestGCGroupsOptions(t *testing.T) {
	query := &gcGroupsQuery{}
	WithGCGroupsLimit(10)(query)
	if query.limit != 10 {
		t.Fatalf("expected limit 10, got %d", query.limit)
	}
	WithGCGroupsNextURL("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	WithGCGroupsGameCenterDetailIDs([]string{" gc-1 ", "", "gc-2"})(query)
	if len(query.gameCenterDetailIDs) != 2 {
		t.Fatalf("expected 2 detail IDs, got %d", len(query.gameCenterDetailIDs))
	}
	values, err := url.ParseQuery(buildGCGroupsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "10" {
		t.Fatalf("expected limit=10, got %q", values.Get("limit"))
	}
	if values.Get("filter[gameCenterDetails]") != "gc-1,gc-2" {
		t.Fatalf("expected filter[gameCenterDetails]=gc-1,gc-2, got %q", values.Get("filter[gameCenterDetails]"))
	}
}

func TestGetGameCenterGroupAchievements_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups/group-1/gameCenterAchievements" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/gameCenterAchievements, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroupAchievements(context.Background(), "group-1", WithGCAchievementsLimit(10)); err != nil {
		t.Fatalf("GetGameCenterGroupAchievements() error: %v", err)
	}
}

func TestGetGameCenterGroupAchievements_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterGroups/group-1/gameCenterAchievements?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroupAchievements(context.Background(), "group-1", WithGCAchievementsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterGroupAchievements() error: %v", err)
	}
}

func TestGetGameCenterGroupAchievementsV2_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/gameCenterGroups/group-1/gameCenterAchievementsV2" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/gameCenterAchievementsV2, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "15" {
			t.Fatalf("expected limit=15, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroupAchievementsV2(context.Background(), "group-1", WithGCAchievementsLimit(15)); err != nil {
		t.Fatalf("GetGameCenterGroupAchievementsV2() error: %v", err)
	}
}

func TestGetGameCenterGroupActivities_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/gameCenterGroups/group-1/gameCenterActivities" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/gameCenterActivities, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "20" {
			t.Fatalf("expected limit=20, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroupActivities(context.Background(), "group-1", WithGCActivitiesLimit(20)); err != nil {
		t.Fatalf("GetGameCenterGroupActivities() error: %v", err)
	}
}

func TestGetGameCenterGroupChallenges_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/gameCenterGroups/group-1/gameCenterChallenges" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/gameCenterChallenges, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "25" {
			t.Fatalf("expected limit=25, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroupChallenges(context.Background(), "group-1", WithGCChallengesLimit(25)); err != nil {
		t.Fatalf("GetGameCenterGroupChallenges() error: %v", err)
	}
}

func TestGetGameCenterGroupLeaderboards_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/gameCenterGroups/group-1/gameCenterLeaderboards" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/gameCenterLeaderboards, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "30" {
			t.Fatalf("expected limit=30, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroupLeaderboards(context.Background(), "group-1", WithGCLeaderboardsLimit(30)); err != nil {
		t.Fatalf("GetGameCenterGroupLeaderboards() error: %v", err)
	}
}

func TestGetGameCenterGroupLeaderboardsV2_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/gameCenterGroups/group-1/gameCenterLeaderboardsV2" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/gameCenterLeaderboardsV2, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "35" {
			t.Fatalf("expected limit=35, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroupLeaderboardsV2(context.Background(), "group-1", WithGCLeaderboardsLimit(35)); err != nil {
		t.Fatalf("GetGameCenterGroupLeaderboardsV2() error: %v", err)
	}
}

func TestGetGameCenterGroupLeaderboardSets_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/gameCenterGroups/group-1/gameCenterLeaderboardSets" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/gameCenterLeaderboardSets, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "40" {
			t.Fatalf("expected limit=40, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroupLeaderboardSets(context.Background(), "group-1", WithGCLeaderboardSetsLimit(40)); err != nil {
		t.Fatalf("GetGameCenterGroupLeaderboardSets() error: %v", err)
	}
}

func TestGetGameCenterGroupLeaderboardSetsV2_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/gameCenterGroups/group-1/gameCenterLeaderboardSetsV2" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/gameCenterLeaderboardSetsV2, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "45" {
			t.Fatalf("expected limit=45, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterGroupLeaderboardSetsV2(context.Background(), "group-1", WithGCLeaderboardSetsLimit(45)); err != nil {
		t.Fatalf("GetGameCenterGroupLeaderboardSetsV2() error: %v", err)
	}
}
