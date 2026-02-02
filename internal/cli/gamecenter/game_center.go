package gamecenter

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// GameCenterCommand returns the game-center command group.
func GameCenterCommand() *ffcli.Command {
	fs := flag.NewFlagSet("game-center", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "game-center",
		ShortUsage: "asc game-center <subcommand> [flags]",
		ShortHelp:  "Manage Game Center resources in App Store Connect.",
		LongHelp: `Manage Game Center resources in App Store Connect.

Examples:
  asc game-center achievements list --app "APP_ID"
  asc game-center achievements create --app "APP_ID" --reference-name "First Win" --vendor-id "com.example.firstwin" --points 10
  asc game-center leaderboards list --app "APP_ID"
  asc game-center leaderboards create --app "APP_ID" --reference-name "High Score" --vendor-id "com.example.highscore" --formatter INTEGER --sort DESC --submission-type BEST_SCORE
  asc game-center leaderboard-sets list --app "APP_ID"
  asc game-center leaderboard-sets create --app "APP_ID" --reference-name "Season 1" --vendor-id "com.example.season1"
  asc game-center challenges list --app "APP_ID"
  asc game-center activities list --app "APP_ID"
  asc game-center groups list --app "APP_ID"
  asc game-center app-versions list --app "APP_ID"
  asc game-center enabled-versions list --app "APP_ID"
  asc game-center enabled-versions compatible-versions --id "ENABLED_VERSION_ID"
  asc game-center details list --app "APP_ID"
  asc game-center details achievements-v2 list --id "DETAILS_ID"
  asc game-center matchmaking queues list`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementsCommand(),
			GameCenterLeaderboardsCommand(),
			GameCenterLeaderboardSetsCommand(),
			GameCenterChallengesCommand(),
			GameCenterActivitiesCommand(),
			GameCenterGroupsCommand(),
			GameCenterAppVersionsCommand(),
			GameCenterEnabledVersionsCommand(),
			GameCenterDetailsCommand(),
			GameCenterMatchmakingCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// parseBool parses a string boolean value with a descriptive flag name for errors.
func parseBool(value, flagName string) (bool, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false", flagName)
	}
}

// isValidLeaderboardFormatter checks if the value is a valid leaderboard formatter.
func isValidLeaderboardFormatter(value string) bool {
	for _, v := range asc.ValidLeaderboardFormatters {
		if value == v {
			return true
		}
	}
	return false
}

// isValidScoreSortType checks if the value is a valid score sort type.
func isValidScoreSortType(value string) bool {
	for _, v := range asc.ValidScoreSortTypes {
		if value == v {
			return true
		}
	}
	return false
}

// isValidSubmissionType checks if the value is a valid submission type.
func isValidSubmissionType(value string) bool {
	for _, v := range asc.ValidSubmissionTypes {
		if value == v {
			return true
		}
	}
	return false
}
