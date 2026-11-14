package asc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GetGameCenterDetailID retrieves the Game Center detail ID for an app.
func (c *Client) GetGameCenterDetailID(ctx context.Context, appID string) (string, error) {
	resp, err := getJSON[GameCenterDetailResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/apps/%s/gameCenterDetail", strings.TrimSpace(appID))})
	if err != nil {
		return "", err
	}
	return resp.Data.ID, nil
}

// GetGameCenterAchievements retrieves the list of Game Center achievements for a Game Center detail.
func (c *Client) GetGameCenterAchievements(ctx context.Context, gcDetailID string, opts ...GCAchievementsOption) (*GameCenterAchievementsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterAchievementsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterAchievements", strings.TrimSpace(gcDetailID)),
		Query:       buildGCAchievementsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-achievements",
	})
}

// GetGameCenterAchievement retrieves a Game Center achievement by ID.
func (c *Client) GetGameCenterAchievement(ctx context.Context, achievementID string) (*GameCenterAchievementResponse, error) {
	return getJSON[GameCenterAchievementResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterAchievements/%s", strings.TrimSpace(achievementID))})
}

// GetGameCenterAchievementGroupAchievement retrieves the group achievement for an achievement.
func (c *Client) GetGameCenterAchievementGroupAchievement(ctx context.Context, achievementID string) (*GameCenterAchievementResponse, error) {
	return getJSON[GameCenterAchievementResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterAchievements/%s/groupAchievement", strings.TrimSpace(achievementID))})
}

// CreateGameCenterAchievement creates a new Game Center achievement.
func (c *Client) CreateGameCenterAchievement(ctx context.Context, gcDetailID string, attrs GameCenterAchievementCreateAttributes) (*GameCenterAchievementResponse, error) {
	payload := GameCenterAchievementCreateRequest{
		Data: GameCenterAchievementCreateData{
			Type:       ResourceTypeGameCenterAchievements,
			Attributes: attrs,
			Relationships: &GameCenterAchievementRelationships{
				GameCenterDetail: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterDetails,
						ID:   strings.TrimSpace(gcDetailID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterAchievementResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterAchievements", Payload: payload})
}

// UpdateGameCenterAchievement updates an existing Game Center achievement.
func (c *Client) UpdateGameCenterAchievement(ctx context.Context, achievementID string, attrs GameCenterAchievementUpdateAttributes) (*GameCenterAchievementResponse, error) {
	payload := GameCenterAchievementUpdateRequest{
		Data: GameCenterAchievementUpdateData{
			Type:       ResourceTypeGameCenterAchievements,
			ID:         strings.TrimSpace(achievementID),
			Attributes: &attrs,
		},
	}

	return sendJSON[GameCenterAchievementResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterAchievements/%s", strings.TrimSpace(achievementID)), Payload: payload})
}

// DeleteGameCenterAchievement deletes a Game Center achievement.
func (c *Client) DeleteGameCenterAchievement(ctx context.Context, achievementID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterAchievements/%s", strings.TrimSpace(achievementID)), nil)
}

// GetGameCenterLeaderboards retrieves the list of Game Center leaderboards for a Game Center detail.
func (c *Client) GetGameCenterLeaderboards(ctx context.Context, gcDetailID string, opts ...GCLeaderboardsOption) (*GameCenterLeaderboardsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterLeaderboards", strings.TrimSpace(gcDetailID)),
		Query:       buildGCLeaderboardsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboards",
	})
}

// GetGameCenterLeaderboard retrieves a Game Center leaderboard by ID.
func (c *Client) GetGameCenterLeaderboard(ctx context.Context, leaderboardID string) (*GameCenterLeaderboardResponse, error) {
	return getJSON[GameCenterLeaderboardResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID))})
}

// GetGameCenterLeaderboardGroupLeaderboard retrieves the group leaderboard for a leaderboard.
func (c *Client) GetGameCenterLeaderboardGroupLeaderboard(ctx context.Context, leaderboardID string) (*GameCenterLeaderboardResponse, error) {
	return getJSON[GameCenterLeaderboardResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboards/%s/groupLeaderboard", strings.TrimSpace(leaderboardID))})
}

// gameCenterDetailRelationship returns nil for an empty ID so a create that
// inherits its parent from its members sends no gameCenterDetail.
func gameCenterDetailRelationship(gcDetailID string) *Relationship {
	gcDetailID = strings.TrimSpace(gcDetailID)
	if gcDetailID == "" {
		return nil
	}
	return &Relationship{Data: ResourceData{Type: ResourceTypeGameCenterDetails, ID: gcDetailID}}
}

// CreateGameCenterLeaderboard creates a new Game Center leaderboard.
func (c *Client) CreateGameCenterLeaderboard(ctx context.Context, gcDetailID string, attrs GameCenterLeaderboardCreateAttributes, setIDs []string) (*GameCenterLeaderboardResponse, error) {
	payload := GameCenterLeaderboardCreateRequest{
		Data: GameCenterLeaderboardCreateData{
			Type:       ResourceTypeGameCenterLeaderboards,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardRelationships{
				GameCenterDetail:          gameCenterDetailRelationship(gcDetailID),
				GameCenterLeaderboardSets: buildRelationshipList(ResourceTypeGameCenterLeaderboardSets, setIDs),
			},
		},
	}

	return sendJSON[GameCenterLeaderboardResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboards", Payload: payload})
}

// UpdateGameCenterLeaderboard updates an existing Game Center leaderboard.
func (c *Client) UpdateGameCenterLeaderboard(ctx context.Context, leaderboardID string, attrs GameCenterLeaderboardUpdateAttributes) (*GameCenterLeaderboardResponse, error) {
	payload := GameCenterLeaderboardUpdateRequest{
		Data: GameCenterLeaderboardUpdateData{
			Type:       ResourceTypeGameCenterLeaderboards,
			ID:         strings.TrimSpace(leaderboardID),
			Attributes: &attrs,
		},
	}

	return sendJSON[GameCenterLeaderboardResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID)), Payload: payload})
}

// DeleteGameCenterLeaderboard deletes a Game Center leaderboard.
func (c *Client) DeleteGameCenterLeaderboard(ctx context.Context, leaderboardID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID)), nil)
}

// GetGameCenterLeaderboardSets retrieves the list of Game Center leaderboard sets for a Game Center detail.
func (c *Client) GetGameCenterLeaderboardSets(ctx context.Context, gcDetailID string, opts ...GCLeaderboardSetsOption) (*GameCenterLeaderboardSetsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardSetsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterLeaderboardSets", strings.TrimSpace(gcDetailID)),
		Query:       buildGCLeaderboardSetsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-sets",
	})
}

// GetGameCenterLeaderboardSet retrieves a Game Center leaderboard set by ID.
func (c *Client) GetGameCenterLeaderboardSet(ctx context.Context, setID string) (*GameCenterLeaderboardSetResponse, error) {
	return getJSON[GameCenterLeaderboardSetResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID))})
}

// GetGameCenterLeaderboardSetGroupLeaderboardSet retrieves the group leaderboard set for a leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetGroupLeaderboardSet(ctx context.Context, setID string) (*GameCenterLeaderboardSetResponse, error) {
	return getJSON[GameCenterLeaderboardSetResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/groupLeaderboardSet", strings.TrimSpace(setID))})
}

// CreateGameCenterLeaderboardSet creates a new Game Center leaderboard set.
func (c *Client) CreateGameCenterLeaderboardSet(ctx context.Context, gcDetailID string, attrs GameCenterLeaderboardSetCreateAttributes, leaderboardIDs []string) (*GameCenterLeaderboardSetResponse, error) {
	payload := GameCenterLeaderboardSetCreateRequest{
		Data: GameCenterLeaderboardSetCreateData{
			Type:       ResourceTypeGameCenterLeaderboardSets,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardSetRelationships{
				GameCenterDetail:       gameCenterDetailRelationship(gcDetailID),
				GameCenterLeaderboards: buildRelationshipList(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
			},
		},
	}

	return sendJSON[GameCenterLeaderboardSetResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboardSets", Payload: payload})
}

// UpdateGameCenterLeaderboardSet updates an existing Game Center leaderboard set.
func (c *Client) UpdateGameCenterLeaderboardSet(ctx context.Context, setID string, attrs GameCenterLeaderboardSetUpdateAttributes) (*GameCenterLeaderboardSetResponse, error) {
	payload := GameCenterLeaderboardSetUpdateRequest{
		Data: GameCenterLeaderboardSetUpdateData{
			Type:       ResourceTypeGameCenterLeaderboardSets,
			ID:         strings.TrimSpace(setID),
			Attributes: &attrs,
		},
	}

	return sendJSON[GameCenterLeaderboardSetResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID)), Payload: payload})
}

// DeleteGameCenterLeaderboardSet deletes a Game Center leaderboard set.
func (c *Client) DeleteGameCenterLeaderboardSet(ctx context.Context, setID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID)), nil)
}

// GetGameCenterLeaderboardLocalizations retrieves the list of localizations for a Game Center leaderboard.
func (c *Client) GetGameCenterLeaderboardLocalizations(ctx context.Context, leaderboardID string, opts ...GCLeaderboardLocalizationsOption) (*GameCenterLeaderboardLocalizationsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardLocalizationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterLeaderboards/%s/localizations", strings.TrimSpace(leaderboardID)),
		Query:       buildGCLeaderboardLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-localizations",
	})
}

// GetGameCenterLeaderboardLocalization retrieves a Game Center leaderboard localization by ID.
func (c *Client) GetGameCenterLeaderboardLocalization(ctx context.Context, localizationID string) (*GameCenterLeaderboardLocalizationResponse, error) {
	return getJSON[GameCenterLeaderboardLocalizationResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID))})
}

// GetGameCenterLeaderboardLocalizationImage retrieves the image for a leaderboard localization.
func (c *Client) GetGameCenterLeaderboardLocalizationImage(ctx context.Context, localizationID string) (*GameCenterLeaderboardImageResponse, error) {
	return getJSON[GameCenterLeaderboardImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboardLocalizations/%s/gameCenterLeaderboardImage", strings.TrimSpace(localizationID))})
}

// CreateGameCenterLeaderboardLocalization creates a new Game Center leaderboard localization.
func (c *Client) CreateGameCenterLeaderboardLocalization(ctx context.Context, leaderboardID string, attrs GameCenterLeaderboardLocalizationCreateAttributes) (*GameCenterLeaderboardLocalizationResponse, error) {
	payload := GameCenterLeaderboardLocalizationCreateRequest{
		Data: GameCenterLeaderboardLocalizationCreateData{
			Type:       ResourceTypeGameCenterLeaderboardLocalizations,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardLocalizationRelationships{
				GameCenterLeaderboard: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboards,
						ID:   strings.TrimSpace(leaderboardID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterLeaderboardLocalizationResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboardLocalizations", Payload: payload})
}

// UpdateGameCenterLeaderboardLocalization updates an existing Game Center leaderboard localization.
func (c *Client) UpdateGameCenterLeaderboardLocalization(ctx context.Context, localizationID string, attrs GameCenterLeaderboardLocalizationUpdateAttributes) (*GameCenterLeaderboardLocalizationResponse, error) {
	payload := GameCenterLeaderboardLocalizationUpdateRequest{
		Data: GameCenterLeaderboardLocalizationUpdateData{
			Type:       ResourceTypeGameCenterLeaderboardLocalizations,
			ID:         strings.TrimSpace(localizationID),
			Attributes: &attrs,
		},
	}

	return sendJSON[GameCenterLeaderboardLocalizationResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID)), Payload: payload})
}

// DeleteGameCenterLeaderboardLocalization deletes a Game Center leaderboard localization.
func (c *Client) DeleteGameCenterLeaderboardLocalization(ctx context.Context, localizationID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID)), nil)
}

// GetAllGameCenterLeaderboardLocalizations retrieves all leaderboard localizations using automatic pagination.
func (c *Client) GetAllGameCenterLeaderboardLocalizations(ctx context.Context, leaderboardID string, opts ...GCLeaderboardLocalizationsOption) (*GameCenterLeaderboardLocalizationsResponse, error) {
	firstPage, err := c.GetGameCenterLeaderboardLocalizations(ctx, leaderboardID, opts...)
	if err != nil {
		return nil, err
	}

	result, err := PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return c.GetGameCenterLeaderboardLocalizations(ctx, leaderboardID, WithGCLeaderboardLocalizationsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}

	resp, ok := result.(*GameCenterLeaderboardLocalizationsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected paginated response type %T", result)
	}
	return resp, nil
}

// GetGameCenterLeaderboardReleases retrieves the list of releases for a Game Center leaderboard.
func (c *Client) GetGameCenterLeaderboardReleases(ctx context.Context, leaderboardID string, opts ...GCLeaderboardReleasesOption) (*GameCenterLeaderboardReleasesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardReleasesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterLeaderboards/%s/releases", strings.TrimSpace(leaderboardID)),
		Query:       buildGCLeaderboardReleasesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-releases",
	})
}

// GetGameCenterLeaderboardRelease retrieves a leaderboard release by ID.
func (c *Client) GetGameCenterLeaderboardRelease(ctx context.Context, releaseID string) (*GameCenterLeaderboardReleaseResponse, error) {
	return getJSON[GameCenterLeaderboardReleaseResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboardReleases/%s", strings.TrimSpace(releaseID))})
}

// CreateGameCenterLeaderboardRelease creates a new Game Center leaderboard release.
func (c *Client) CreateGameCenterLeaderboardRelease(ctx context.Context, gcDetailID, leaderboardID string) (*GameCenterLeaderboardReleaseResponse, error) {
	payload := GameCenterLeaderboardReleaseCreateRequest{
		Data: GameCenterLeaderboardReleaseCreateData{
			Type: ResourceTypeGameCenterLeaderboardReleases,
			Relationships: &GameCenterLeaderboardReleaseRelationships{
				GameCenterDetail: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterDetails,
						ID:   strings.TrimSpace(gcDetailID),
					},
				},
				GameCenterLeaderboard: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboards,
						ID:   strings.TrimSpace(leaderboardID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterLeaderboardReleaseResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboardReleases", Payload: payload})
}

// DeleteGameCenterLeaderboardRelease deletes a Game Center leaderboard release.
func (c *Client) DeleteGameCenterLeaderboardRelease(ctx context.Context, releaseID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterLeaderboardReleases/%s", strings.TrimSpace(releaseID)), nil)
}

// GetGameCenterAchievementReleases retrieves the list of releases for a Game Center achievement.
func (c *Client) GetGameCenterAchievementReleases(ctx context.Context, achievementID string, opts ...GCAchievementReleasesOption) (*GameCenterAchievementReleasesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterAchievementReleasesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterAchievements/%s/releases", strings.TrimSpace(achievementID)),
		Query:       buildGCAchievementReleasesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-achievement-releases",
	})
}

// GetGameCenterAchievementRelease retrieves an achievement release by ID.
func (c *Client) GetGameCenterAchievementRelease(ctx context.Context, releaseID string) (*GameCenterAchievementReleaseResponse, error) {
	return getJSON[GameCenterAchievementReleaseResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterAchievementReleases/%s", strings.TrimSpace(releaseID))})
}

// CreateGameCenterAchievementRelease creates a new Game Center achievement release.
func (c *Client) CreateGameCenterAchievementRelease(ctx context.Context, gcDetailID, achievementID string) (*GameCenterAchievementReleaseResponse, error) {
	payload := GameCenterAchievementReleaseCreateRequest{
		Data: GameCenterAchievementReleaseCreateData{
			Type: ResourceTypeGameCenterAchievementReleases,
			Relationships: &GameCenterAchievementReleaseRelationships{
				GameCenterDetail: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterDetails,
						ID:   strings.TrimSpace(gcDetailID),
					},
				},
				GameCenterAchievement: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterAchievements,
						ID:   strings.TrimSpace(achievementID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterAchievementReleaseResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterAchievementReleases", Payload: payload})
}

// DeleteGameCenterAchievementRelease deletes a Game Center achievement release.
func (c *Client) DeleteGameCenterAchievementRelease(ctx context.Context, releaseID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterAchievementReleases/%s", strings.TrimSpace(releaseID)), nil)
}

// GetGameCenterLeaderboardSetMembers retrieves the leaderboards in a leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetMembers(ctx context.Context, setID string, opts ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/gameCenterLeaderboards", strings.TrimSpace(setID)),
		Query:       buildGCLeaderboardSetMembersQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-set-members",
	})
}

// UpdateGameCenterLeaderboardSetMembers replaces all leaderboard members in a leaderboard set.
func (c *Client) UpdateGameCenterLeaderboardSetMembers(ctx context.Context, setID string, leaderboardIDs []string) error {
	leaderboardIDs = normalizeList(leaderboardIDs)

	payload := GameCenterLeaderboardSetMembersUpdateRequest{
		Data: make([]RelationshipData, 0, len(leaderboardIDs)),
	}
	for _, leaderboardID := range leaderboardIDs {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeGameCenterLeaderboards,
			ID:   leaderboardID,
		})
	}

	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/relationships/gameCenterLeaderboards", strings.TrimSpace(setID)), payload)
}

// SetGameCenterLeaderboardSetMembers sets leaderboard members for a leaderboard set.
func (c *Client) SetGameCenterLeaderboardSetMembers(ctx context.Context, setID string, leaderboardIDs []string) error {
	return setGameCenterLeaderboardSetMembers(ctx, setID, leaderboardIDs, leaderboardSetMembersOperations{
		list:    c.GetGameCenterLeaderboardSetMembers,
		add:     c.AddGameCenterLeaderboardSetMembers,
		remove:  c.RemoveGameCenterLeaderboardSetMembers,
		replace: c.UpdateGameCenterLeaderboardSetMembers,
	})
}

// GetGameCenterLeaderboardSetReleases retrieves the list of releases for a Game Center leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetReleases(ctx context.Context, setID string, opts ...GCLeaderboardSetReleasesOption) (*GameCenterLeaderboardSetReleasesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardSetReleasesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/releases", strings.TrimSpace(setID)),
		Query:       buildGCLeaderboardSetReleasesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-set-releases",
	})
}

// GetGameCenterLeaderboardSetRelease retrieves a leaderboard set release by ID.
func (c *Client) GetGameCenterLeaderboardSetRelease(ctx context.Context, releaseID string) (*GameCenterLeaderboardSetReleaseResponse, error) {
	return getJSON[GameCenterLeaderboardSetReleaseResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboardSetReleases/%s", strings.TrimSpace(releaseID))})
}

// CreateGameCenterLeaderboardSetRelease creates a new Game Center leaderboard set release.
func (c *Client) CreateGameCenterLeaderboardSetRelease(ctx context.Context, gcDetailID, setID string) (*GameCenterLeaderboardSetReleaseResponse, error) {
	payload := GameCenterLeaderboardSetReleaseCreateRequest{
		Data: GameCenterLeaderboardSetReleaseCreateData{
			Type: ResourceTypeGameCenterLeaderboardSetReleases,
			Relationships: &GameCenterLeaderboardSetReleaseRelationships{
				GameCenterDetail: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterDetails,
						ID:   strings.TrimSpace(gcDetailID),
					},
				},
				GameCenterLeaderboardSet: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboardSets,
						ID:   strings.TrimSpace(setID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterLeaderboardSetReleaseResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboardSetReleases", Payload: payload})
}

// DeleteGameCenterLeaderboardSetRelease deletes a Game Center leaderboard set release.
func (c *Client) DeleteGameCenterLeaderboardSetRelease(ctx context.Context, releaseID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterLeaderboardSetReleases/%s", strings.TrimSpace(releaseID)), nil)
}

// GetGameCenterLeaderboardSetLocalizations retrieves the list of localizations for a Game Center leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetLocalizations(ctx context.Context, setID string, opts ...GCLeaderboardSetLocalizationsOption) (*GameCenterLeaderboardSetLocalizationsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardSetLocalizationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/localizations", strings.TrimSpace(setID)),
		Query:       buildGCLeaderboardSetLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-set-localizations",
	})
}

// GetGameCenterLeaderboardSetLocalization retrieves a Game Center leaderboard set localization by ID.
func (c *Client) GetGameCenterLeaderboardSetLocalization(ctx context.Context, localizationID string) (*GameCenterLeaderboardSetLocalizationResponse, error) {
	return getJSON[GameCenterLeaderboardSetLocalizationResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID))})
}

// GetGameCenterLeaderboardSetLocalizationImage retrieves the image for a leaderboard set localization.
func (c *Client) GetGameCenterLeaderboardSetLocalizationImage(ctx context.Context, localizationID string) (*GameCenterLeaderboardSetImageResponse, error) {
	return getJSON[GameCenterLeaderboardSetImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboardSetLocalizations/%s/gameCenterLeaderboardSetImage", strings.TrimSpace(localizationID))})
}

// CreateGameCenterLeaderboardSetLocalization creates a new Game Center leaderboard set localization.
func (c *Client) CreateGameCenterLeaderboardSetLocalization(ctx context.Context, setID string, attrs GameCenterLeaderboardSetLocalizationCreateAttributes) (*GameCenterLeaderboardSetLocalizationResponse, error) {
	payload := GameCenterLeaderboardSetLocalizationCreateRequest{
		Data: GameCenterLeaderboardSetLocalizationCreateData{
			Type:       ResourceTypeGameCenterLeaderboardSetLocalizations,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardSetLocalizationRelationships{
				GameCenterLeaderboardSet: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboardSets,
						ID:   strings.TrimSpace(setID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterLeaderboardSetLocalizationResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboardSetLocalizations", Payload: payload})
}

// UpdateGameCenterLeaderboardSetLocalization updates an existing Game Center leaderboard set localization.
func (c *Client) UpdateGameCenterLeaderboardSetLocalization(ctx context.Context, localizationID string, attrs GameCenterLeaderboardSetLocalizationUpdateAttributes) (*GameCenterLeaderboardSetLocalizationResponse, error) {
	payload := GameCenterLeaderboardSetLocalizationUpdateRequest{
		Data: GameCenterLeaderboardSetLocalizationUpdateData{
			Type:       ResourceTypeGameCenterLeaderboardSetLocalizations,
			ID:         strings.TrimSpace(localizationID),
			Attributes: &attrs,
		},
	}

	return sendJSON[GameCenterLeaderboardSetLocalizationResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID)), Payload: payload})
}

// DeleteGameCenterLeaderboardSetLocalization deletes a Game Center leaderboard set localization.
func (c *Client) DeleteGameCenterLeaderboardSetLocalization(ctx context.Context, localizationID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID)), nil)
}

// GetGameCenterAchievementLocalizations retrieves the list of localizations for a Game Center achievement.
func (c *Client) GetGameCenterAchievementLocalizations(ctx context.Context, achievementID string, opts ...GCAchievementLocalizationsOption) (*GameCenterAchievementLocalizationsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterAchievementLocalizationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterAchievements/%s/localizations", strings.TrimSpace(achievementID)),
		Query:       buildGCAchievementLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-achievement-localizations",
	})
}

// GetGameCenterAchievementLocalization retrieves a Game Center achievement localization by ID.
func (c *Client) GetGameCenterAchievementLocalization(ctx context.Context, localizationID string) (*GameCenterAchievementLocalizationResponse, error) {
	return getJSON[GameCenterAchievementLocalizationResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID))})
}

// GetGameCenterAchievementLocalizationAchievement retrieves the achievement for a localization.
func (c *Client) GetGameCenterAchievementLocalizationAchievement(ctx context.Context, localizationID string) (*GameCenterAchievementResponse, error) {
	return getJSON[GameCenterAchievementResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s/gameCenterAchievement", strings.TrimSpace(localizationID))})
}

// GetGameCenterAchievementLocalizationImage retrieves the image for a localization.
func (c *Client) GetGameCenterAchievementLocalizationImage(ctx context.Context, localizationID string) (*GameCenterAchievementImageResponse, error) {
	return getJSON[GameCenterAchievementImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s/gameCenterAchievementImage", strings.TrimSpace(localizationID))})
}

// CreateGameCenterAchievementLocalization creates a new Game Center achievement localization.
func (c *Client) CreateGameCenterAchievementLocalization(ctx context.Context, achievementID string, attrs GameCenterAchievementLocalizationCreateAttributes) (*GameCenterAchievementLocalizationResponse, error) {
	payload := GameCenterAchievementLocalizationCreateRequest{
		Data: GameCenterAchievementLocalizationCreateData{
			Type:       ResourceTypeGameCenterAchievementLocalizations,
			Attributes: attrs,
			Relationships: &GameCenterAchievementLocalizationRelationships{
				GameCenterAchievement: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterAchievements,
						ID:   strings.TrimSpace(achievementID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterAchievementLocalizationResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterAchievementLocalizations", Payload: payload})
}

// UpdateGameCenterAchievementLocalization updates an existing Game Center achievement localization.
func (c *Client) UpdateGameCenterAchievementLocalization(ctx context.Context, localizationID string, attrs GameCenterAchievementLocalizationUpdateAttributes) (*GameCenterAchievementLocalizationResponse, error) {
	payload := GameCenterAchievementLocalizationUpdateRequest{
		Data: GameCenterAchievementLocalizationUpdateData{
			Type:       ResourceTypeGameCenterAchievementLocalizations,
			ID:         strings.TrimSpace(localizationID),
			Attributes: &attrs,
		},
	}

	return sendJSON[GameCenterAchievementLocalizationResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID)), Payload: payload})
}

// DeleteGameCenterAchievementLocalization deletes a Game Center achievement localization.
func (c *Client) DeleteGameCenterAchievementLocalization(ctx context.Context, localizationID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID)), nil)
}

// GetGameCenterLeaderboardImage retrieves a Game Center leaderboard image by ID.
func (c *Client) GetGameCenterLeaderboardImage(ctx context.Context, imageID string) (*GameCenterLeaderboardImageResponse, error) {
	return getJSON[GameCenterLeaderboardImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID))})
}

// CreateGameCenterLeaderboardImage reserves an image upload for a leaderboard localization.
func (c *Client) CreateGameCenterLeaderboardImage(ctx context.Context, localizationID string, fileName string, fileSize int64) (*GameCenterLeaderboardImageResponse, error) {
	payload := GameCenterLeaderboardImageCreateRequest{
		Data: GameCenterLeaderboardImageCreateData{
			Type: ResourceTypeGameCenterLeaderboardImages,
			Attributes: GameCenterLeaderboardImageCreateAttributes{
				FileSize: fileSize,
				FileName: fileName,
			},
			Relationships: &GameCenterLeaderboardImageRelationships{
				GameCenterLeaderboardLocalization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboardLocalizations,
						ID:   strings.TrimSpace(localizationID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterLeaderboardImageResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboardImages", Payload: payload})
}

// UpdateGameCenterLeaderboardImage commits an image upload.
func (c *Client) UpdateGameCenterLeaderboardImage(ctx context.Context, imageID string, uploaded bool) (*GameCenterLeaderboardImageResponse, error) {
	payload := GameCenterLeaderboardImageUpdateRequest{
		Data: GameCenterLeaderboardImageUpdateData{
			Type: ResourceTypeGameCenterLeaderboardImages,
			ID:   strings.TrimSpace(imageID),
			Attributes: &GameCenterLeaderboardImageUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	}

	return sendJSON[GameCenterLeaderboardImageResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID)), Payload: payload})
}

// DeleteGameCenterLeaderboardImage deletes a Game Center leaderboard image.
func (c *Client) DeleteGameCenterLeaderboardImage(ctx context.Context, imageID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID)), nil)
}

// UploadGameCenterLeaderboardImage performs the full upload flow for a leaderboard image.
// It reserves the upload, uploads the file, and commits the upload.
func (c *Client) UploadGameCenterLeaderboardImage(ctx context.Context, localizationID string, filePath string) (*GameCenterLeaderboardImageUploadResult, error) {
	// Validate the file
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	// Step 1: Reserve the upload
	reservation, err := c.CreateGameCenterLeaderboardImage(ctx, localizationID, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve image upload: %w", err)
	}

	imageID := reservation.Data.ID
	operations := reservation.Data.Attributes.UploadOperations

	if len(operations) == 0 {
		return nil, fmt.Errorf("no upload operations returned from API")
	}

	// Step 2: Upload the file
	if err := UploadAsset(ctx, filePath, operations); err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	// Step 3: Commit the upload
	committed, err := c.UpdateGameCenterLeaderboardImage(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit image upload: %w", err)
	}

	state := ""
	if committed.Data.Attributes.AssetDeliveryState != nil {
		state = committed.Data.Attributes.AssetDeliveryState.State
	}

	return &GameCenterLeaderboardImageUploadResult{
		ID:                 committed.Data.ID,
		LocalizationID:     localizationID,
		FileName:           fileName,
		FileSize:           fileSize,
		AssetDeliveryState: state,
		Uploaded:           true,
	}, nil
}

// GetGameCenterAchievementImage retrieves a Game Center achievement image by ID.
func (c *Client) GetGameCenterAchievementImage(ctx context.Context, imageID string) (*GameCenterAchievementImageResponse, error) {
	return getJSON[GameCenterAchievementImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterAchievementImages/%s", strings.TrimSpace(imageID))})
}

// CreateGameCenterAchievementImage reserves a new Game Center achievement image upload.
func (c *Client) CreateGameCenterAchievementImage(ctx context.Context, localizationID string, fileName string, fileSize int64) (*GameCenterAchievementImageResponse, error) {
	payload := GameCenterAchievementImageCreateRequest{
		Data: GameCenterAchievementImageCreateData{
			Type: ResourceTypeGameCenterAchievementImages,
			Attributes: GameCenterAchievementImageCreateAttributes{
				FileSize: fileSize,
				FileName: fileName,
			},
			Relationships: &GameCenterAchievementImageRelationships{
				GameCenterAchievementLocalization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterAchievementLocalizations,
						ID:   strings.TrimSpace(localizationID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterAchievementImageResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterAchievementImages", Payload: payload})
}

// UpdateGameCenterAchievementImage updates a Game Center achievement image (used to commit upload).
func (c *Client) UpdateGameCenterAchievementImage(ctx context.Context, imageID string, uploaded bool) (*GameCenterAchievementImageResponse, error) {
	payload := GameCenterAchievementImageUpdateRequest{
		Data: GameCenterAchievementImageUpdateData{
			Type: ResourceTypeGameCenterAchievementImages,
			ID:   strings.TrimSpace(imageID),
			Attributes: &GameCenterAchievementImageUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	}

	return sendJSON[GameCenterAchievementImageResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterAchievementImages/%s", strings.TrimSpace(imageID)), Payload: payload})
}

// DeleteGameCenterAchievementImage deletes a Game Center achievement image.
func (c *Client) DeleteGameCenterAchievementImage(ctx context.Context, imageID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterAchievementImages/%s", strings.TrimSpace(imageID)), nil)
}

// UploadGameCenterAchievementImage performs the complete upload flow: reserve, upload chunks, commit.
func (c *Client) UploadGameCenterAchievementImage(ctx context.Context, localizationID string, filePath string) (*GameCenterAchievementImageUploadResult, error) {
	// Validate the file
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	// Step 1: Reserve the upload
	reservation, err := c.CreateGameCenterAchievementImage(ctx, localizationID, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve image upload: %w", err)
	}

	imageID := reservation.Data.ID
	operations := reservation.Data.Attributes.UploadOperations

	if len(operations) == 0 {
		return nil, fmt.Errorf("no upload operations returned from API")
	}

	// Step 2: Upload the file
	if err := UploadAsset(ctx, filePath, operations); err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	// Step 3: Commit the upload
	committed, err := c.UpdateGameCenterAchievementImage(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit image upload: %w", err)
	}

	state := ""
	if committed.Data.Attributes.AssetDeliveryState != nil {
		state = committed.Data.Attributes.AssetDeliveryState.State
	}

	return &GameCenterAchievementImageUploadResult{
		ID:                 committed.Data.ID,
		LocalizationID:     localizationID,
		FileName:           fileName,
		FileSize:           fileSize,
		AssetDeliveryState: state,
		Uploaded:           true,
	}, nil
}

// GetGameCenterLeaderboardSetImage retrieves a Game Center leaderboard set image by ID.
func (c *Client) GetGameCenterLeaderboardSetImage(ctx context.Context, imageID string) (*GameCenterLeaderboardSetImageResponse, error) {
	return getJSON[GameCenterLeaderboardSetImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID))})
}

// CreateGameCenterLeaderboardSetImage reserves an image upload slot for a leaderboard set localization.
func (c *Client) CreateGameCenterLeaderboardSetImage(ctx context.Context, localizationID string, fileName string, fileSize int64) (*GameCenterLeaderboardSetImageResponse, error) {
	payload := GameCenterLeaderboardSetImageCreateRequest{
		Data: GameCenterLeaderboardSetImageCreateData{
			Type: ResourceTypeGameCenterLeaderboardSetImages,
			Attributes: GameCenterLeaderboardSetImageCreateAttributes{
				FileSize: fileSize,
				FileName: fileName,
			},
			Relationships: &GameCenterLeaderboardSetImageRelationships{
				GameCenterLeaderboardSetLocalization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboardSetLocalizations,
						ID:   strings.TrimSpace(localizationID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterLeaderboardSetImageResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboardSetImages", Payload: payload})
}

// UpdateGameCenterLeaderboardSetImage commits an image upload (sets uploaded=true).
func (c *Client) UpdateGameCenterLeaderboardSetImage(ctx context.Context, imageID string, uploaded bool) (*GameCenterLeaderboardSetImageResponse, error) {
	payload := GameCenterLeaderboardSetImageUpdateRequest{
		Data: GameCenterLeaderboardSetImageUpdateData{
			Type: ResourceTypeGameCenterLeaderboardSetImages,
			ID:   strings.TrimSpace(imageID),
			Attributes: &GameCenterLeaderboardSetImageUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	}

	return sendJSON[GameCenterLeaderboardSetImageResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID)), Payload: payload})
}

// DeleteGameCenterLeaderboardSetImage deletes a Game Center leaderboard set image.
func (c *Client) DeleteGameCenterLeaderboardSetImage(ctx context.Context, imageID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID)), nil)
}

// UploadGameCenterLeaderboardSetImage performs the full upload flow for a leaderboard set image.
// It reserves an upload slot, uploads the file, and commits the upload.
func (c *Client) UploadGameCenterLeaderboardSetImage(ctx context.Context, localizationID, filePath string) (*GameCenterLeaderboardSetImageUploadResult, error) {
	// Validate the image file
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	// Step 1: Reserve upload slot
	reservation, err := c.CreateGameCenterLeaderboardSetImage(ctx, localizationID, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve upload: %w", err)
	}

	imageID := reservation.Data.ID
	operations := reservation.Data.Attributes.UploadOperations

	if len(operations) == 0 {
		return nil, fmt.Errorf("no upload operations returned from reservation")
	}

	// Step 2: Upload the file
	if err := UploadAsset(ctx, filePath, operations); err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	// Step 3: Commit the upload
	committed, err := c.UpdateGameCenterLeaderboardSetImage(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit upload: %w", err)
	}

	result := &GameCenterLeaderboardSetImageUploadResult{
		ID:             committed.Data.ID,
		LocalizationID: localizationID,
		FileName:       committed.Data.Attributes.FileName,
		FileSize:       committed.Data.Attributes.FileSize,
		Uploaded:       true,
	}

	if committed.Data.Attributes.AssetDeliveryState != nil {
		result.AssetDeliveryState = committed.Data.Attributes.AssetDeliveryState.State
	}

	return result, nil
}

// GetGameCenterAchievementV2 retrieves a v2 Game Center achievement by ID.
func (c *Client) GetGameCenterAchievementV2(ctx context.Context, achievementID string) (*GameCenterAchievementResponse, error) {
	return getJSON[GameCenterAchievementResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterAchievements/%s", strings.TrimSpace(achievementID))})
}

// CreateGameCenterAchievementV2 creates a new v2 Game Center achievement.
func (c *Client) CreateGameCenterAchievementV2(ctx context.Context, gcDetailID string, groupID string, attrs GameCenterAchievementCreateAttributes) (*GameCenterAchievementResponse, error) {
	const initialVersionID = "${achVer1}"

	relationships := &GameCenterAchievementV2Relationships{
		Versions: &RelationshipList{Data: []ResourceData{{
			Type: ResourceTypeGameCenterAchievementVersions,
			ID:   initialVersionID,
		}}},
	}

	if strings.TrimSpace(gcDetailID) != "" {
		relationships.GameCenterDetail = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterDetails,
				ID:   strings.TrimSpace(gcDetailID),
			},
		}
	}
	if strings.TrimSpace(groupID) != "" {
		relationships.GameCenterGroup = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterGroups,
				ID:   strings.TrimSpace(groupID),
			},
		}
	}

	payload := GameCenterAchievementV2CreateRequest{
		Data: GameCenterAchievementV2CreateData{
			Type:          ResourceTypeGameCenterAchievements,
			Attributes:    attrs,
			Relationships: relationships,
		},
		Included: []GameCenterAchievementVersionInlineCreate{{
			Type: ResourceTypeGameCenterAchievementVersions,
			ID:   initialVersionID,
		}},
	}

	return sendJSON[GameCenterAchievementResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v2/gameCenterAchievements", Payload: payload})
}

// UpdateGameCenterAchievementV2 updates an existing v2 Game Center achievement.
func (c *Client) UpdateGameCenterAchievementV2(ctx context.Context, achievementID string, attrs GameCenterAchievementUpdateAttributes) (*GameCenterAchievementResponse, error) {
	payload := GameCenterAchievementUpdateRequest{
		Data: GameCenterAchievementUpdateData{
			Type:       ResourceTypeGameCenterAchievements,
			ID:         strings.TrimSpace(achievementID),
			Attributes: &attrs,
		},
	}

	return sendJSON[GameCenterAchievementResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v2/gameCenterAchievements/%s", strings.TrimSpace(achievementID)), Payload: payload})
}

// DeleteGameCenterAchievementV2 deletes a v2 Game Center achievement.
func (c *Client) DeleteGameCenterAchievementV2(ctx context.Context, achievementID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/gameCenterAchievements/%s", strings.TrimSpace(achievementID)), nil)
}

// GetGameCenterLeaderboardV2 retrieves a v2 Game Center leaderboard by ID.
func (c *Client) GetGameCenterLeaderboardV2(ctx context.Context, leaderboardID string) (*GameCenterLeaderboardResponse, error) {
	return getJSON[GameCenterLeaderboardResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID))})
}

// CreateGameCenterLeaderboardV2 creates a new v2 Game Center leaderboard.
func (c *Client) CreateGameCenterLeaderboardV2(ctx context.Context, gcDetailID string, groupID string, attrs GameCenterLeaderboardCreateAttributes, setIDs []string) (*GameCenterLeaderboardResponse, error) {
	const initialVersionID = "${lbVer1}"

	relationships := &GameCenterLeaderboardV2Relationships{
		GameCenterLeaderboardSets: buildRelationshipList(ResourceTypeGameCenterLeaderboardSets, setIDs),
		Versions: &RelationshipList{Data: []ResourceData{{
			Type: ResourceTypeGameCenterLeaderboardVersions,
			ID:   initialVersionID,
		}}},
	}

	if strings.TrimSpace(gcDetailID) != "" {
		relationships.GameCenterDetail = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterDetails,
				ID:   strings.TrimSpace(gcDetailID),
			},
		}
	}
	if strings.TrimSpace(groupID) != "" {
		relationships.GameCenterGroup = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterGroups,
				ID:   strings.TrimSpace(groupID),
			},
		}
	}

	payload := GameCenterLeaderboardV2CreateRequest{
		Data: GameCenterLeaderboardV2CreateData{
			Type:          ResourceTypeGameCenterLeaderboards,
			Attributes:    attrs,
			Relationships: relationships,
		},
		Included: []GameCenterLeaderboardVersionInlineCreate{{
			Type: ResourceTypeGameCenterLeaderboardVersions,
			ID:   initialVersionID,
		}},
	}

	return sendJSON[GameCenterLeaderboardResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v2/gameCenterLeaderboards", Payload: payload})
}

// UpdateGameCenterLeaderboardV2 updates an existing v2 Game Center leaderboard.
func (c *Client) UpdateGameCenterLeaderboardV2(ctx context.Context, leaderboardID string, attrs GameCenterLeaderboardUpdateAttributes) (*GameCenterLeaderboardResponse, error) {
	payload := GameCenterLeaderboardUpdateRequest{
		Data: GameCenterLeaderboardUpdateData{
			Type:       ResourceTypeGameCenterLeaderboards,
			ID:         strings.TrimSpace(leaderboardID),
			Attributes: &attrs,
		},
	}

	return sendJSON[GameCenterLeaderboardResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v2/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID)), Payload: payload})
}

// DeleteGameCenterLeaderboardV2 deletes a v2 Game Center leaderboard.
func (c *Client) DeleteGameCenterLeaderboardV2(ctx context.Context, leaderboardID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID)), nil)
}
