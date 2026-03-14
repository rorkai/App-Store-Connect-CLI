package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GameCenterLeaderboardEntrySubmissionAttributes describes a leaderboard entry submission.
type GameCenterLeaderboardEntrySubmissionAttributes struct {
	BundleID         string   `json:"bundleId"`
	ChallengeIDs     []string `json:"challengeIds,omitempty"`
	Context          *string  `json:"context,omitempty"`
	PreReleased      *bool    `json:"preReleased,omitempty"`
	ScopedPlayerID   string   `json:"scopedPlayerId"`
	Score            string   `json:"score"`
	SubmittedDate    *string  `json:"submittedDate,omitempty"`
	VendorIdentifier string   `json:"vendorIdentifier"`
}

// GameCenterLeaderboardEntrySubmissionResource represents a leaderboard entry submission resource.
type GameCenterLeaderboardEntrySubmissionResource struct {
	Type       ResourceType                                   `json:"type"`
	ID         string                                         `json:"id"`
	Attributes GameCenterLeaderboardEntrySubmissionAttributes `json:"attributes"`
}

// GameCenterLeaderboardEntrySubmissionResponse is the response from leaderboard entry submissions.
type GameCenterLeaderboardEntrySubmissionResponse struct {
	Data  GameCenterLeaderboardEntrySubmissionResource `json:"data"`
	Links Links                                        `json:"links"`
}

// GameCenterLeaderboardEntrySubmissionCreateData is the data portion of a leaderboard entry submission request.
type GameCenterLeaderboardEntrySubmissionCreateData struct {
	Type       ResourceType                                   `json:"type"`
	Attributes GameCenterLeaderboardEntrySubmissionAttributes `json:"attributes"`
}

// GameCenterLeaderboardEntrySubmissionCreateRequest is a request to submit a leaderboard entry.
type GameCenterLeaderboardEntrySubmissionCreateRequest struct {
	Data GameCenterLeaderboardEntrySubmissionCreateData `json:"data"`
}

// CreateGameCenterLeaderboardEntrySubmission submits a leaderboard entry.
func (c *Client) CreateGameCenterLeaderboardEntrySubmission(ctx context.Context, attrs GameCenterLeaderboardEntrySubmissionAttributes) (*GameCenterLeaderboardEntrySubmissionResponse, error) {
	attrs.VendorIdentifier = strings.TrimSpace(attrs.VendorIdentifier)
	attrs.Score = strings.TrimSpace(attrs.Score)
	attrs.BundleID = strings.TrimSpace(attrs.BundleID)
	attrs.ScopedPlayerID = strings.TrimSpace(attrs.ScopedPlayerID)
	attrs.ChallengeIDs = normalizeList(attrs.ChallengeIDs)

	if attrs.VendorIdentifier == "" {
		return nil, fmt.Errorf("vendorIdentifier is required")
	}
	if attrs.Score == "" {
		return nil, fmt.Errorf("score is required")
	}
	if attrs.BundleID == "" {
		return nil, fmt.Errorf("bundleId is required")
	}
	if attrs.ScopedPlayerID == "" {
		return nil, fmt.Errorf("scopedPlayerId is required")
	}
	if len(attrs.ChallengeIDs) == 0 {
		attrs.ChallengeIDs = nil
	}
	if attrs.Context != nil {
		trimmed := strings.TrimSpace(*attrs.Context)
		if trimmed == "" {
			attrs.Context = nil
		} else {
			attrs.Context = &trimmed
		}
	}
	if attrs.SubmittedDate != nil {
		trimmed := strings.TrimSpace(*attrs.SubmittedDate)
		if trimmed == "" {
			attrs.SubmittedDate = nil
		} else {
			attrs.SubmittedDate = &trimmed
		}
	}

	payload := GameCenterLeaderboardEntrySubmissionCreateRequest{
		Data: GameCenterLeaderboardEntrySubmissionCreateData{
			Type:       ResourceTypeGameCenterLeaderboardEntrySubmissions,
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/gameCenterLeaderboardEntrySubmissions", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardEntrySubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GameCenterPlayerAchievementSubmissionAttributes describes a player achievement submission.
type GameCenterPlayerAchievementSubmissionAttributes struct {
	BundleID           string   `json:"bundleId"`
	ChallengeIDs       []string `json:"challengeIds,omitempty"`
	PercentageAchieved int      `json:"percentageAchieved"`
	PreReleased        *bool    `json:"preReleased,omitempty"`
	ScopedPlayerID     string   `json:"scopedPlayerId"`
	SubmittedDate      *string  `json:"submittedDate,omitempty"`
	VendorIdentifier   string   `json:"vendorIdentifier"`
}

// GameCenterPlayerAchievementSubmissionResource represents a player achievement submission resource.
type GameCenterPlayerAchievementSubmissionResource struct {
	Type       ResourceType                                    `json:"type"`
	ID         string                                          `json:"id"`
	Attributes GameCenterPlayerAchievementSubmissionAttributes `json:"attributes"`
}

// GameCenterPlayerAchievementSubmissionResponse is the response from player achievement submissions.
type GameCenterPlayerAchievementSubmissionResponse struct {
	Data  GameCenterPlayerAchievementSubmissionResource `json:"data"`
	Links Links                                         `json:"links"`
}

// GameCenterPlayerAchievementSubmissionCreateData is the data portion of a player achievement submission request.
type GameCenterPlayerAchievementSubmissionCreateData struct {
	Type       ResourceType                                    `json:"type"`
	Attributes GameCenterPlayerAchievementSubmissionAttributes `json:"attributes"`
}

// GameCenterPlayerAchievementSubmissionCreateRequest is a request to submit player achievements.
type GameCenterPlayerAchievementSubmissionCreateRequest struct {
	Data GameCenterPlayerAchievementSubmissionCreateData `json:"data"`
}

// CreateGameCenterPlayerAchievementSubmission submits a player achievement.
func (c *Client) CreateGameCenterPlayerAchievementSubmission(ctx context.Context, attrs GameCenterPlayerAchievementSubmissionAttributes) (*GameCenterPlayerAchievementSubmissionResponse, error) {
	attrs.VendorIdentifier = strings.TrimSpace(attrs.VendorIdentifier)
	attrs.BundleID = strings.TrimSpace(attrs.BundleID)
	attrs.ScopedPlayerID = strings.TrimSpace(attrs.ScopedPlayerID)
	attrs.ChallengeIDs = normalizeList(attrs.ChallengeIDs)

	if attrs.VendorIdentifier == "" {
		return nil, fmt.Errorf("vendorIdentifier is required")
	}
	if attrs.BundleID == "" {
		return nil, fmt.Errorf("bundleId is required")
	}
	if attrs.ScopedPlayerID == "" {
		return nil, fmt.Errorf("scopedPlayerId is required")
	}
	if len(attrs.ChallengeIDs) == 0 {
		attrs.ChallengeIDs = nil
	}
	if attrs.SubmittedDate != nil {
		trimmed := strings.TrimSpace(*attrs.SubmittedDate)
		if trimmed == "" {
			attrs.SubmittedDate = nil
		} else {
			attrs.SubmittedDate = &trimmed
		}
	}

	payload := GameCenterPlayerAchievementSubmissionCreateRequest{
		Data: GameCenterPlayerAchievementSubmissionCreateData{
			Type:       ResourceTypeGameCenterPlayerAchievementSubmissions,
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/gameCenterPlayerAchievementSubmissions", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterPlayerAchievementSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
