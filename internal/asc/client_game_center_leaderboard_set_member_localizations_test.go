package asc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestGetGameCenterLeaderboardSetMemberLocalizations_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSetMemberLocalizations, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		if values.Get("filter[gameCenterLeaderboardSet]") != "set-1" {
			t.Fatalf("expected filter[gameCenterLeaderboardSet]=set-1, got %q", values.Get("filter[gameCenterLeaderboardSet]"))
		}
		if values.Get("filter[gameCenterLeaderboard]") != "lb-1,lb-2" {
			t.Fatalf("expected filter[gameCenterLeaderboard]=lb-1,lb-2, got %q", values.Get("filter[gameCenterLeaderboard]"))
		}
		assertAuthorized(t, req)
	}, response)

	opts := []GCLeaderboardSetMemberLocalizationsOption{
		WithGCLeaderboardSetMemberLocalizationsLimit(5),
		WithGCLeaderboardSetMemberLocalizationsLeaderboardSetIDs([]string{"set-1"}),
		WithGCLeaderboardSetMemberLocalizationsLeaderboardIDs([]string{"lb-1", "lb-2"}),
	}

	if _, err := client.GetGameCenterLeaderboardSetMemberLocalizations(context.Background(), opts...); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetMemberLocalizations() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetMemberLocalizations_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboardSetMemberLocalizations?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetMemberLocalizations(context.Background(), WithGCLeaderboardSetMemberLocalizationsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetMemberLocalizations() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetMemberLocalization_CompositeLookupFindsExactIDOnSecondPage(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboardSetMemberLocalizations?cursor=page-2"
	responses := []*http.Response{
		jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{}}}`),
		jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSets","id":"set-1","attributes":{}}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"gameCenterLeaderboardSetMemberLocalizations","id":"loc-other","attributes":{"name":"Other","locale":"en-US"}}],"links":{"next":"`+next+`"}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"gameCenterLeaderboardSetMemberLocalizations","id":"loc-2","attributes":{"name":"Seasonal","locale":"en-GB"}}],"links":{}}`),
	}

	requestCount := 0
	client := newTestClient(t, func(req *http.Request) {
		requestCount++
		switch requestCount {
		case 1:
			if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations/loc-2/gameCenterLeaderboard" {
				t.Fatalf("request 1 path = %q", req.URL.Path)
			}
		case 2:
			if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations/loc-2/gameCenterLeaderboardSet" {
				t.Fatalf("request 2 path = %q", req.URL.Path)
			}
		case 3:
			if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations" {
				t.Fatalf("request 3 path = %q", req.URL.Path)
			}
			query := req.URL.Query()
			if got := query.Get("filter[gameCenterLeaderboard]"); got != "lb-1" {
				t.Fatalf("leaderboard filter = %q, want lb-1", got)
			}
			if got := query.Get("filter[gameCenterLeaderboardSet]"); got != "set-1" {
				t.Fatalf("leaderboard set filter = %q, want set-1", got)
			}
			if got := query.Get("limit"); got != "200" {
				t.Fatalf("limit = %q, want 200", got)
			}
		case 4:
			if got := req.URL.String(); got != next {
				t.Fatalf("request 4 URL = %q, want %q", got, next)
			}
		default:
			t.Fatalf("unexpected request %d: %s", requestCount, req.URL.String())
		}
		assertAuthorized(t, req)
	}, responses...)

	resp, err := client.GetGameCenterLeaderboardSetMemberLocalization(context.Background(), " loc-2 ")
	if err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetMemberLocalization() error: %v", err)
	}
	if requestCount != 4 {
		t.Fatalf("request count = %d, want 4", requestCount)
	}
	if resp.Data.ID != "loc-2" || resp.Data.Attributes.Name != "Seasonal" || resp.Data.Attributes.Locale != "en-GB" {
		t.Fatalf("unexpected response: %+v", resp.Data)
	}
}

func TestGetGameCenterLeaderboardSetMemberLocalization_CompositeLookupNotFound(t *testing.T) {
	requestCount := 0
	client := newTestClient(
		t, func(req *http.Request) {
			requestCount++
		},
		jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{}}}`),
		jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSets","id":"set-1","attributes":{}}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"gameCenterLeaderboardSetMemberLocalizations","id":"loc-other","attributes":{"name":"Other","locale":"en-US"}}],"links":{}}`),
	)

	_, err := client.GetGameCenterLeaderboardSetMemberLocalization(context.Background(), "loc-missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), `member localization "loc-missing"`) {
		t.Fatalf("error = %v, want localization ID", err)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
}

func TestGetGameCenterLeaderboardSetMemberLocalization_CompositeLookupRejectsRepeatedNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboardSetMemberLocalizations?cursor=repeat"
	requestCount := 0
	client := newTestClient(
		t, func(req *http.Request) {
			requestCount++
		},
		jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{}}}`),
		jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSets","id":"set-1","attributes":{}}}`),
		jsonResponse(http.StatusOK, `{"data":[],"links":{"next":"`+next+`"}}`),
		jsonResponse(http.StatusOK, `{"data":[],"links":{"next":"`+next+`"}}`),
	)

	_, err := client.GetGameCenterLeaderboardSetMemberLocalization(context.Background(), "loc-missing")
	if !errors.Is(err, ErrRepeatedPaginationURL) {
		t.Fatalf("error = %v, want ErrRepeatedPaginationURL", err)
	}
	if requestCount != 4 {
		t.Fatalf("request count = %d, want 4", requestCount)
	}
}

func TestGetGameCenterLeaderboardSetMemberLocalizationLeaderboard(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{"referenceName":"Leaderboard"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations/loc-1/gameCenterLeaderboard" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSetMemberLocalizations/loc-1/gameCenterLeaderboard, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetMemberLocalizationLeaderboard(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetMemberLocalizationLeaderboard() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetMemberLocalizationLeaderboardSet(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSets","id":"set-1","attributes":{"referenceName":"Set"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations/loc-1/gameCenterLeaderboardSet" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSetMemberLocalizations/loc-1/gameCenterLeaderboardSet, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetMemberLocalizationLeaderboardSet(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetMemberLocalizationLeaderboardSet() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardSetMemberLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboardSetMemberLocalizations","id":"loc-new","attributes":{"name":"Top Score","locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSetMemberLocalizations, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload GameCenterLeaderboardSetMemberLocalizationCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeGameCenterLeaderboardSetMemberLocalizations {
			t.Fatalf("expected type gameCenterLeaderboardSetMemberLocalizations, got %s", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Top Score" {
			t.Fatalf("expected name 'Top Score', got %s", payload.Data.Attributes.Name)
		}
		if payload.Data.Attributes.Locale != "en-US" {
			t.Fatalf("expected locale en-US, got %s", payload.Data.Attributes.Locale)
		}
		if payload.Data.Relationships == nil {
			t.Fatalf("expected relationships to be set")
		}
		if payload.Data.Relationships.GameCenterLeaderboardSet.Data.ID != "set-1" {
			t.Fatalf("expected leaderboardSet ID set-1, got %s", payload.Data.Relationships.GameCenterLeaderboardSet.Data.ID)
		}
		if payload.Data.Relationships.GameCenterLeaderboard.Data.ID != "lb-1" {
			t.Fatalf("expected leaderboard ID lb-1, got %s", payload.Data.Relationships.GameCenterLeaderboard.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterLeaderboardSetMemberLocalizationCreateAttributes{
		Name:   "Top Score",
		Locale: "en-US",
	}
	resp, err := client.CreateGameCenterLeaderboardSetMemberLocalization(context.Background(), "set-1", "lb-1", attrs)
	if err != nil {
		t.Fatalf("CreateGameCenterLeaderboardSetMemberLocalization() error: %v", err)
	}

	if resp.Data.ID != "loc-new" {
		t.Fatalf("expected ID loc-new, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.Name != "Top Score" {
		t.Fatalf("expected name 'Top Score', got %s", resp.Data.Attributes.Name)
	}
	if resp.Data.Attributes.Locale != "en-US" {
		t.Fatalf("expected locale en-US, got %s", resp.Data.Attributes.Locale)
	}
}

func TestUpdateGameCenterLeaderboardSetMemberLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSetMemberLocalizations","id":"loc-1","attributes":{"name":"Updated Name","locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations/loc-1" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSetMemberLocalizations/loc-1, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload GameCenterLeaderboardSetMemberLocalizationUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeGameCenterLeaderboardSetMemberLocalizations {
			t.Fatalf("expected type gameCenterLeaderboardSetMemberLocalizations, got %s", payload.Data.Type)
		}
		if payload.Data.ID != "loc-1" {
			t.Fatalf("expected id loc-1, got %s", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil {
			t.Fatalf("expected name attribute to be set")
		}
		if *payload.Data.Attributes.Name != "Updated Name" {
			t.Fatalf("expected name 'Updated Name', got %s", *payload.Data.Attributes.Name)
		}
		assertAuthorized(t, req)
	}, response)

	newName := "Updated Name"
	attrs := GameCenterLeaderboardSetMemberLocalizationUpdateAttributes{Name: &newName}
	resp, err := client.UpdateGameCenterLeaderboardSetMemberLocalization(context.Background(), "loc-1", attrs)
	if err != nil {
		t.Fatalf("UpdateGameCenterLeaderboardSetMemberLocalization() error: %v", err)
	}

	if resp.Data.ID != "loc-1" {
		t.Fatalf("expected ID loc-1, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.Name != "Updated Name" {
		t.Fatalf("expected name 'Updated Name', got %s", resp.Data.Attributes.Name)
	}
}

func TestDeleteGameCenterLeaderboardSetMemberLocalization(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusNoContent,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader("")),
	}

	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSetMemberLocalizations/loc-1" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSetMemberLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	err := client.DeleteGameCenterLeaderboardSetMemberLocalization(context.Background(), "loc-1")
	if err != nil {
		t.Fatalf("DeleteGameCenterLeaderboardSetMemberLocalization() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardSetMemberLocalization_ValidationErrors(t *testing.T) {
	client := newTestClient(t, nil, nil)

	tests := []struct {
		name           string
		leaderboardSet string
		leaderboard    string
		attrs          GameCenterLeaderboardSetMemberLocalizationCreateAttributes
	}{
		{
			name:           "missing leaderboard set ID",
			leaderboardSet: " ",
			leaderboard:    "lb-1",
			attrs:          GameCenterLeaderboardSetMemberLocalizationCreateAttributes{Name: "Top Score", Locale: "en-US"},
		},
		{
			name:           "missing leaderboard ID",
			leaderboardSet: "set-1",
			leaderboard:    " ",
			attrs:          GameCenterLeaderboardSetMemberLocalizationCreateAttributes{Name: "Top Score", Locale: "en-US"},
		},
		{
			name:           "missing name",
			leaderboardSet: "set-1",
			leaderboard:    "lb-1",
			attrs:          GameCenterLeaderboardSetMemberLocalizationCreateAttributes{Name: " ", Locale: "en-US"},
		},
		{
			name:           "missing locale",
			leaderboardSet: "set-1",
			leaderboard:    "lb-1",
			attrs:          GameCenterLeaderboardSetMemberLocalizationCreateAttributes{Name: "Top Score", Locale: " "},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.CreateGameCenterLeaderboardSetMemberLocalization(context.Background(), test.leaderboardSet, test.leaderboard, test.attrs)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestCreateGameCenterLeaderboardSetMemberLocalization_ReturnsAPIError(t *testing.T) {
	response := jsonResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
	client := newTestClient(t, nil, response)

	attrs := GameCenterLeaderboardSetMemberLocalizationCreateAttributes{Name: "Top Score", Locale: "en-US"}
	_, err := client.CreateGameCenterLeaderboardSetMemberLocalization(context.Background(), "set-1", "lb-1", attrs)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status code %d, got %d", http.StatusForbidden, apiErr.StatusCode)
	}
}

func TestUpdateGameCenterLeaderboardSetMemberLocalization_ValidationErrors(t *testing.T) {
	client := newTestClient(t, nil, nil)

	newName := "Updated Name"
	tests := []struct {
		name string
		id   string
		attr GameCenterLeaderboardSetMemberLocalizationUpdateAttributes
	}{
		{
			name: "missing localization ID",
			id:   " ",
			attr: GameCenterLeaderboardSetMemberLocalizationUpdateAttributes{Name: &newName},
		},
		{
			name: "missing attributes",
			id:   "loc-1",
			attr: GameCenterLeaderboardSetMemberLocalizationUpdateAttributes{},
		},
		{
			name: "empty name",
			id:   "loc-1",
			attr: GameCenterLeaderboardSetMemberLocalizationUpdateAttributes{Name: func() *string { s := " "; return &s }()},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.UpdateGameCenterLeaderboardSetMemberLocalization(context.Background(), test.id, test.attr)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestUpdateGameCenterLeaderboardSetMemberLocalization_ReturnsAPIError(t *testing.T) {
	response := jsonResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
	client := newTestClient(t, nil, response)

	newName := "Updated Name"
	_, err := client.UpdateGameCenterLeaderboardSetMemberLocalization(context.Background(), "loc-1", GameCenterLeaderboardSetMemberLocalizationUpdateAttributes{Name: &newName})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status code %d, got %d", http.StatusForbidden, apiErr.StatusCode)
	}
}

func TestDeleteGameCenterLeaderboardSetMemberLocalization_RequiresID(t *testing.T) {
	client := newTestClient(t, nil, nil)

	err := client.DeleteGameCenterLeaderboardSetMemberLocalization(context.Background(), " ")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDeleteGameCenterLeaderboardSetMemberLocalization_ReturnsAPIError(t *testing.T) {
	response := jsonResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
	client := newTestClient(t, nil, response)

	err := client.DeleteGameCenterLeaderboardSetMemberLocalization(context.Background(), "loc-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status code %d, got %d", http.StatusForbidden, apiErr.StatusCode)
	}
}

func TestGCLeaderboardSetMemberLocalizationsOptions(t *testing.T) {
	query := &gcLeaderboardSetMemberLocalizationsQuery{}
	WithGCLeaderboardSetMemberLocalizationsLimit(10)(query)
	if query.limit != 10 {
		t.Fatalf("expected limit 10, got %d", query.limit)
	}
	WithGCLeaderboardSetMemberLocalizationsNextURL("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	WithGCLeaderboardSetMemberLocalizationsLeaderboardSetIDs([]string{" set-1 ", "", "set-2"})(query)
	if len(query.leaderboardSetIDs) != 2 {
		t.Fatalf("expected 2 set IDs, got %d", len(query.leaderboardSetIDs))
	}
	WithGCLeaderboardSetMemberLocalizationsLeaderboardIDs([]string{" lb-1 ", "lb-2"})(query)
	if len(query.leaderboardIDs) != 2 {
		t.Fatalf("expected 2 leaderboard IDs, got %d", len(query.leaderboardIDs))
	}
	values, err := url.ParseQuery(buildGCLeaderboardSetMemberLocalizationsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "10" {
		t.Fatalf("expected limit=10, got %q", values.Get("limit"))
	}
	if values.Get("filter[gameCenterLeaderboardSet]") != "set-1,set-2" {
		t.Fatalf("expected filter[gameCenterLeaderboardSet]=set-1,set-2, got %q", values.Get("filter[gameCenterLeaderboardSet]"))
	}
	if values.Get("filter[gameCenterLeaderboard]") != "lb-1,lb-2" {
		t.Fatalf("expected filter[gameCenterLeaderboard]=lb-1,lb-2, got %q", values.Get("filter[gameCenterLeaderboard]"))
	}
}
