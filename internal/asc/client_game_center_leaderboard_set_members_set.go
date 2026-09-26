package asc

import (
	"context"
	"fmt"
	"strings"
)

type leaderboardSetMembersOperations struct {
	list    func(context.Context, string, ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error)
	add     func(context.Context, string, []string) error
	remove  func(context.Context, string, []string) error
	replace func(context.Context, string, []string) error
}

func setGameCenterLeaderboardSetMembers(ctx context.Context, setID string, leaderboardIDs []string, ops leaderboardSetMembersOperations) error {
	setID = strings.TrimSpace(setID)
	if setID == "" {
		return fmt.Errorf("setID is required")
	}
	if ops.list == nil || ops.add == nil || ops.remove == nil || ops.replace == nil {
		return fmt.Errorf("leaderboard set member operations are incomplete")
	}

	desiredIDs := normalizeUniqueList(leaderboardIDs)
	currentIDs, err := listGameCenterLeaderboardSetMemberIDs(ctx, setID, ops.list)
	if err != nil {
		return err
	}

	currentSet := make(map[string]struct{}, len(currentIDs))
	for _, id := range currentIDs {
		currentSet[id] = struct{}{}
	}

	desiredSet := make(map[string]struct{}, len(desiredIDs))
	for _, id := range desiredIDs {
		desiredSet[id] = struct{}{}
	}

	toAdd := make([]string, 0, len(desiredIDs))
	for _, id := range desiredIDs {
		if _, exists := currentSet[id]; !exists {
			toAdd = append(toAdd, id)
		}
	}

	toRemove := make([]string, 0, len(currentIDs))
	for _, id := range currentIDs {
		if _, exists := desiredSet[id]; !exists {
			toRemove = append(toRemove, id)
		}
	}

	if len(toAdd) > 0 {
		if err := ops.add(ctx, setID, toAdd); err != nil {
			return err
		}
	}
	if len(toRemove) > 0 {
		if err := ops.remove(ctx, setID, toRemove); err != nil {
			return err
		}
	}

	if len(desiredIDs) == 0 {
		return nil
	}

	if len(toAdd) > 0 || len(toRemove) > 0 || !sameOrderedList(currentIDs, desiredIDs) {
		return ops.replace(ctx, setID, desiredIDs)
	}

	return nil
}

func listGameCenterLeaderboardSetMemberIDs(ctx context.Context, setID string, listFn func(context.Context, string, ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error)) ([]string, error) {
	ids := make([]string, 0)
	nextURL := ""
	page := 1
	seenNext := make(map[string]struct{})

	for {
		opts := []GCLeaderboardSetMembersOption{WithGCLeaderboardSetMembersLimit(200)}
		if nextURL != "" {
			opts = append(opts, WithGCLeaderboardSetMembersNextURL(nextURL))
		}

		resp, err := listFn(ctx, setID, opts...)
		if err != nil {
			return nil, err
		}

		for _, item := range resp.Data {
			id := strings.TrimSpace(item.ID)
			if id != "" {
				ids = append(ids, id)
			}
		}

		nextURL = strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			break
		}
		nextIdentity := PaginationURLIdentity(nextURL)
		if _, ok := seenNext[nextIdentity]; ok {
			return nil, fmt.Errorf("page %d: %w", page+1, ErrRepeatedPaginationURL)
		}
		seenNext[nextIdentity] = struct{}{}
		page++
	}

	return ids, nil
}
