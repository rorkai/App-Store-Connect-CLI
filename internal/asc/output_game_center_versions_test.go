package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_GameCenterAppVersions(t *testing.T) {
	resp := &GameCenterAppVersionsResponse{
		Data: []Resource[GameCenterAppVersionAttributes]{
			{
				ID: "gcav-1",
				Attributes: GameCenterAppVersionAttributes{
					Enabled: true,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Enabled") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "gcav-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}

func TestPrintMarkdown_GameCenterAppVersions(t *testing.T) {
	resp := &GameCenterAppVersionsResponse{
		Data: []Resource[GameCenterAppVersionAttributes]{
			{
				ID: "gcav-1",
				Attributes: GameCenterAppVersionAttributes{
					Enabled: true,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Enabled") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "gcav-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}

func TestPrintTable_GameCenterEnabledVersions(t *testing.T) {
	resp := &GameCenterEnabledVersionsResponse{
		Data: []Resource[GameCenterEnabledVersionAttributes]{
			{
				ID: "gcev-1",
				Attributes: GameCenterEnabledVersionAttributes{
					Platform:      PlatformIOS,
					VersionString: "1.0",
					IconAsset: &ImageAsset{
						TemplateURL: "https://example.com/icon.png",
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Platform") || !strings.Contains(output, "Version") {
		t.Fatalf("expected headers in output, got: %s", output)
	}
	if !strings.Contains(output, "gcev-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}

func TestPrintMarkdown_GameCenterDetails(t *testing.T) {
	resp := &GameCenterDetailsResponse{
		Data: []Resource[GameCenterDetailAttributes]{
			{
				ID: "detail-1",
				Attributes: GameCenterDetailAttributes{
					ArcadeEnabled:             true,
					ChallengeEnabled:          false,
					LeaderboardEnabled:        true,
					LeaderboardSetEnabled:     false,
					AchievementEnabled:        true,
					MultiplayerSessionEnabled: true,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "Arcade Enabled") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "detail-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}
