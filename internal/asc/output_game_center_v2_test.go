package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_GameCenterAchievementVersions(t *testing.T) {
	resp := &GameCenterAchievementVersionsResponse{
		Data: []Resource[GameCenterAchievementVersionAttributes]{
			{
				ID: "ver-1",
				Attributes: GameCenterAchievementVersionAttributes{
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
		Data: []Resource[GameCenterLeaderboardVersionAttributes]{
			{
				ID: "ver-2",
				Attributes: GameCenterLeaderboardVersionAttributes{
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
		Data: []Resource[GameCenterLeaderboardSetVersionAttributes]{
			{
				ID: "ver-3",
				Attributes: GameCenterLeaderboardSetVersionAttributes{
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
