package gamecenter

import (
	"context"
	"encoding/json"
	"flag"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const noGameCenterDetailWarning = `Warning: no Game Center detail exists for this app. Run "asc game-center details create --app <APP_ID>" to create one.`

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

// isValidLeaderboardFormatter checks if the value is a valid leaderboard formatter.
func isValidLeaderboardFormatter(value string) bool {
	return slices.Contains(asc.ValidLeaderboardFormatters, value)
}

// isValidScoreSortType checks if the value is a valid score sort type.
func isValidScoreSortType(value string) bool {
	return slices.Contains(asc.ValidScoreSortTypes, value)
}

// isValidSubmissionType checks if the value is a valid submission type.
func isValidSubmissionType(value string) bool {
	return slices.Contains(asc.ValidSubmissionTypes, value)
}

func flagPassed(fs *flag.FlagSet, name string) bool {
	passed := false
	fs.Visit(func(item *flag.Flag) {
		passed = passed || item.Name == name
	})
	return passed
}

// parseStringMapFlag decodes a flag holding a JSON object of string values. It
// returns nil when the flag was not passed and a non-nil map otherwise, so an
// empty object still reaches Apple.
func parseStringMapFlag(fs *flag.FlagSet, name string) (map[string]string, error) {
	if !flagPassed(fs, name) {
		return nil, nil
	}
	shapeError := func() error {
		return shared.UsageErrorf("--%s must be a JSON object of string values", name)
	}
	// Pointer values keep a JSON null from decoding to an empty string.
	var decoded map[string]*string
	if err := json.Unmarshal([]byte(fs.Lookup(name).Value.String()), &decoded); err != nil || decoded == nil {
		return nil, shapeError()
	}
	values := make(map[string]string, len(decoded))
	for key, value := range decoded {
		if value == nil {
			return nil, shapeError()
		}
		values[key] = *value
	}
	return values, nil
}

// parseLeaderboardVisibilityFlag returns the normalized --visibility value, or
// an empty string when the flag was not passed.
func parseLeaderboardVisibilityFlag(fs *flag.FlagSet) (string, error) {
	if !flagPassed(fs, "visibility") {
		return "", nil
	}
	value := strings.TrimSpace(strings.ToUpper(fs.Lookup("visibility").Value.String()))
	if !slices.Contains(asc.ValidLeaderboardVisibilities, value) {
		return "", shared.UsageErrorf("--visibility must be one of: %s", strings.Join(asc.ValidLeaderboardVisibilities, ", "))
	}
	return value, nil
}
