package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_GameCenterAchievementVersions(t *testing.T) {
	resp := &GameCenterAchievementVersionsResponse{
		Data: []Resource[GameCenterAchievementVersionV2Attributes]{
			{
				ID: "ver-1",
				Attributes: GameCenterAchievementVersionV2Attributes{
					Version: 2,
					State:   GameCenterVersionStateReadyForReview,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Version") {
		t.Fatalf("expected Version header, got: %s", output)
	}
	if !strings.Contains(output, "READY_FOR_REVIEW") {
		t.Fatalf("expected state in output, got: %s", output)
	}
}

func TestPrintMarkdown_GameCenterLeaderboardVersions(t *testing.T) {
	resp := &GameCenterLeaderboardVersionsResponse{
		Data: []Resource[GameCenterLeaderboardVersionV2Attributes]{
			{
				ID: "ver-2",
				Attributes: GameCenterLeaderboardVersionV2Attributes{
					Version: 5,
					State:   GameCenterVersionStateInReview,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Version") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "IN_REVIEW") {
		t.Fatalf("expected state in output, got: %s", output)
	}
}

func TestPrintTable_GameCenterLeaderboardSetVersions(t *testing.T) {
	resp := &GameCenterLeaderboardSetVersionsResponse{
		Data: []Resource[GameCenterLeaderboardSetVersionV2Attributes]{
			{
				ID: "ver-3",
				Attributes: GameCenterLeaderboardSetVersionV2Attributes{
					Version: 1,
					State:   GameCenterVersionStateLive,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Version") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "LIVE") {
		t.Fatalf("expected state in output, got: %s", output)
	}
}
