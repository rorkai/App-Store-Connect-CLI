package asc

import (
	"context"
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

	return sendJSON[GameCenterLeaderboardEntrySubmissionResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/gameCenterLeaderboardEntrySubmissions", Payload: payload})
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

	return sendJSON[GameCenterPlayerAchievementSubmissionResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/gameCenterPlayerAchievementSubmissions", Payload: payload})
}
