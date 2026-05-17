package gamecenter

import "testing"

func TestGameCenterCommandConstructors(t *testing.T) {
	top := GameCenterCommand()
	if top == nil {
		t.Fatal("expected game-center command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := GameCenterCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}

	constructors := []func() any{
		func() any { return GameCenterAchievementsCommand() },
		func() any { return GameCenterLeaderboardsCommand() },
		func() any { return GameCenterLeaderboardSetsCommand() },
		func() any { return GameCenterGroupsCommand() },
		func() any { return GameCenterDetailsCommand() },
		func() any { return GameCenterAppVersionsCommand() },
		func() any { return GameCenterEnabledVersionsCommand() },
		func() any { return GameCenterMatchmakingCommand() },
		func() any { return GameCenterChallengesCommand() },
		func() any { return GameCenterActivitiesCommand() },
		func() any { return GameCenterAchievementsV2Command() },
		func() any { return GameCenterLeaderboardsV2Command() },
		func() any { return GameCenterLeaderboardSetsV2Command() },
		func() any { return GameCenterLeaderboardSetImagesCommand() },
	}
	for _, ctor := range constructors {
		if got := ctor(); got == nil {
			t.Fatal("expected constructor to return command")
		}
	}
}
