package asc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestListGameCenterLeaderboardSetMemberIDsRejectsEquivalentRepeatedNextURL(t *testing.T) {
	path := "/v1/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards"
	tests := []struct {
		name       string
		firstNext  string
		repeatNext string
	}{
		{
			name:       "identical",
			firstNext:  path + "?cursor=repeat",
			repeatNext: path + "?cursor=repeat",
		},
		{
			name:       "relative and absolute",
			firstNext:  BaseURL + path + "?cursor=repeat",
			repeatNext: path + "?cursor=repeat",
		},
		{
			name:       "query order",
			firstNext:  path + "?cursor=repeat&limit=200",
			repeatNext: path + "?limit=200&cursor=repeat",
		},
		{
			name:       "surrounding whitespace",
			firstNext:  "  " + path + "?cursor=repeat  ",
			repeatNext: "\t" + path + "?cursor=repeat\n",
		},
		{
			name:       "empty query",
			firstNext:  path + "?",
			repeatNext: path,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			_, err := listGameCenterLeaderboardSetMemberIDs(
				context.Background(),
				"set-1",
				func(_ context.Context, setID string, opts ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error) {
					requests++
					if setID != "set-1" {
						t.Fatalf("setID = %q, want set-1", setID)
					}
					query := applyGCLeaderboardSetMembersOptions(opts...)
					switch requests {
					case 1:
						if query.limit != 200 || query.nextURL != "" {
							t.Fatalf("first query = %+v, want limit 200 and no next URL", query)
						}
						return &GameCenterLeaderboardsResponse{Links: Links{Next: tt.firstNext}}, nil
					case 2:
						if query.limit != 200 || query.nextURL != strings.TrimSpace(tt.firstNext) {
							t.Fatalf("second query = %+v, want limit 200 and next %q", query, strings.TrimSpace(tt.firstNext))
						}
						return &GameCenterLeaderboardsResponse{Links: Links{Next: tt.repeatNext}}, nil
					default:
						return nil, errors.New("unexpected third request")
					}
				},
			)

			if !errors.Is(err, ErrRepeatedPaginationURL) {
				t.Fatalf("error = %v, want ErrRepeatedPaginationURL", err)
			}
			if !strings.Contains(err.Error(), "page 3") {
				t.Fatalf("error = %q, want page 3 context", err)
			}
			if requests != 2 {
				t.Fatalf("requests = %d, want 2", requests)
			}
		})
	}
}

func TestListGameCenterLeaderboardSetMemberIDsAggregatesPages(t *testing.T) {
	next := "/v1/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards?cursor=second"
	requests := 0

	ids, err := listGameCenterLeaderboardSetMemberIDs(
		context.Background(),
		"set-1",
		func(_ context.Context, _ string, opts ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error) {
			requests++
			query := applyGCLeaderboardSetMembersOptions(opts...)
			switch requests {
			case 1:
				if query.limit != 200 || query.nextURL != "" {
					t.Fatalf("first query = %+v, want limit 200 and no next URL", query)
				}
				return gameCenterLeaderboardSetMembersResponse([]string{"first", " "}, next), nil
			case 2:
				if query.limit != 200 || query.nextURL != next {
					t.Fatalf("second query = %+v, want limit 200 and next %q", query, next)
				}
				return gameCenterLeaderboardSetMembersResponse([]string{"second"}, ""), nil
			default:
				return nil, errors.New("unexpected third request")
			}
		},
	)
	if err != nil {
		t.Fatalf("listGameCenterLeaderboardSetMemberIDs() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if want := []string{"first", "second"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestSetGameCenterLeaderboardSetMembersDoesNotMutateAfterListFailure(t *testing.T) {
	tests := []struct {
		name string
		list func(context.Context, string, ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error)
	}{
		{
			name: "page error",
			list: func(context.Context, string, ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error) {
				return nil, errors.New("list failed")
			},
		},
		{
			name: "pagination cycle",
			list: func() func(context.Context, string, ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error) {
				requests := 0
				next := "/v1/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards?cursor=repeat"
				return func(context.Context, string, ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error) {
					requests++
					if requests > 2 {
						return nil, fmt.Errorf("unexpected request %d", requests)
					}
					return gameCenterLeaderboardSetMembersResponse([]string{"current"}, next), nil
				}
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutations := 0
			mutate := func(context.Context, string, []string) error {
				mutations++
				return nil
			}
			err := setGameCenterLeaderboardSetMembers(context.Background(), "set-1", []string{"desired"}, leaderboardSetMembersOperations{
				list:    tt.list,
				add:     mutate,
				remove:  mutate,
				replace: mutate,
			})
			if err == nil {
				t.Fatal("setGameCenterLeaderboardSetMembers() error = nil, want failure")
			}
			if mutations != 0 {
				t.Fatalf("mutations = %d, want 0", mutations)
			}
		})
	}
}

func applyGCLeaderboardSetMembersOptions(opts ...GCLeaderboardSetMembersOption) gcLeaderboardSetMembersQuery {
	query := gcLeaderboardSetMembersQuery{}
	for _, opt := range opts {
		opt(&query)
	}
	return query
}

func gameCenterLeaderboardSetMembersResponse(ids []string, next string) *GameCenterLeaderboardsResponse {
	data := make([]Resource[GameCenterLeaderboardAttributes], 0, len(ids))
	for _, id := range ids {
		data = append(data, Resource[GameCenterLeaderboardAttributes]{ID: id})
	}
	return &GameCenterLeaderboardsResponse{Data: data, Links: Links{Next: next}}
}
