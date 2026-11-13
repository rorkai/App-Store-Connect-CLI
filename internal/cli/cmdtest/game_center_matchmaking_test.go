package cmdtest

import (
	"testing"
)

func TestGameCenterMatchmakingValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "matchmaking", "queues", "create", "--rule-set-id", "RULE_SET_ID"}, "--reference-name is required"},
		{[]string{"game-center", "matchmaking", "queues", "create", "--reference-name", "Queue"}, "--rule-set-id is required"},
		{[]string{"game-center", "matchmaking", "queues", "update"}, "--id is required"},
		{[]string{"game-center", "matchmaking", "queues", "update", "--id", "QUEUE_ID"}, "at least one update flag is required"},
		{[]string{"game-center", "matchmaking", "queues", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "matchmaking", "queues", "delete", "--id", "QUEUE_ID"}, "--confirm is required"},
		{[]string{"game-center", "matchmaking", "rule-sets", "create", "--rule-language-version", "1", "--min-players", "2", "--max-players", "8"}, "--reference-name is required"},
		{[]string{"game-center", "matchmaking", "rule-sets", "create", "--reference-name", "Rules", "--min-players", "2", "--max-players", "8"}, "--rule-language-version is required"},
		{[]string{"game-center", "matchmaking", "rule-sets", "create", "--reference-name", "Rules", "--rule-language-version", "1"}, "--min-players and --max-players are required"},
		{[]string{"game-center", "matchmaking", "rule-sets", "update"}, "--id is required"},
		{[]string{"game-center", "matchmaking", "rule-sets", "update", "--id", "RULE_SET_ID"}, "at least one update flag is required"},
		{[]string{"game-center", "matchmaking", "rule-sets", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "matchmaking", "rule-sets", "delete", "--id", "RULE_SET_ID"}, "--confirm is required"},
		{[]string{"game-center", "matchmaking", "rules", "list"}, "--rule-set-id is required"},
		{[]string{"game-center", "matchmaking", "rules", "create", "--reference-name", "Rule", "--description", "Desc", "--type", "MATCH", "--expression", "true"}, "--rule-set-id is required"},
		{[]string{"game-center", "matchmaking", "rules", "create", "--rule-set-id", "RULE_SET_ID", "--description", "Desc", "--type", "MATCH", "--expression", "true"}, "--reference-name is required"},
		{[]string{"game-center", "matchmaking", "rules", "create", "--rule-set-id", "RULE_SET_ID", "--reference-name", "Rule", "--type", "MATCH", "--expression", "true"}, "--description is required"},
		{[]string{"game-center", "matchmaking", "rules", "create", "--rule-set-id", "RULE_SET_ID", "--reference-name", "Rule", "--description", "Desc", "--expression", "true"}, "--type is required"},
		{[]string{"game-center", "matchmaking", "rules", "create", "--rule-set-id", "RULE_SET_ID", "--reference-name", "Rule", "--description", "Desc", "--type", "MATCH"}, "--expression is required"},
		{[]string{"game-center", "matchmaking", "rules", "update"}, "--id is required"},
		{[]string{"game-center", "matchmaking", "rules", "update", "--id", "RULE_ID"}, "at least one update flag is required"},
		{[]string{"game-center", "matchmaking", "rules", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "matchmaking", "rules", "delete", "--id", "RULE_ID"}, "--confirm is required"},
		{[]string{"game-center", "matchmaking", "teams", "list"}, "--rule-set-id is required"},
		{[]string{"game-center", "matchmaking", "teams", "create", "--reference-name", "Team", "--min-players", "1", "--max-players", "4"}, "--rule-set-id is required"},
		{[]string{"game-center", "matchmaking", "teams", "create", "--rule-set-id", "RULE_SET_ID", "--min-players", "1", "--max-players", "4"}, "--reference-name is required"},
		{[]string{"game-center", "matchmaking", "teams", "create", "--rule-set-id", "RULE_SET_ID", "--reference-name", "Team"}, "--min-players and --max-players are required"},
		{[]string{"game-center", "matchmaking", "teams", "update"}, "--id is required"},
		{[]string{"game-center", "matchmaking", "teams", "update", "--id", "TEAM_ID"}, "at least one update flag is required"},
		{[]string{"game-center", "matchmaking", "teams", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "matchmaking", "teams", "delete", "--id", "TEAM_ID"}, "--confirm is required"},
		{[]string{"game-center", "matchmaking", "metrics", "queue-sizes", "--granularity", "P1D"}, "--queue-id is required"},
		{[]string{"game-center", "matchmaking", "metrics", "queue-requests", "--granularity", "P1D"}, "--queue-id is required"},
		{[]string{"game-center", "matchmaking", "metrics", "queue-sessions", "--granularity", "P1D"}, "--queue-id is required"},
		{[]string{"game-center", "matchmaking", "metrics", "experiment-queue-sizes", "--granularity", "P1D"}, "--queue-id is required"},
		{[]string{"game-center", "matchmaking", "metrics", "experiment-queue-requests", "--granularity", "P1D"}, "--queue-id is required"},
		{[]string{"game-center", "matchmaking", "metrics", "queue-sizes", "--queue-id", "QUEUE_ID"}, "--granularity is required"},
		{[]string{"game-center", "matchmaking", "metrics", "rule-boolean-results", "--granularity", "P1D"}, "--rule-id is required"},
		{[]string{"game-center", "matchmaking", "metrics", "rule-number-results", "--granularity", "P1D"}, "--rule-id is required"},
		{[]string{"game-center", "matchmaking", "metrics", "rule-errors", "--granularity", "P1D"}, "--rule-id is required"},
		{[]string{"game-center", "matchmaking", "metrics", "rule-boolean-results", "--rule-id", "RULE_ID"}, "--granularity is required"},
		{[]string{"game-center", "matchmaking", "rule-set-tests", "create"}, "--file is required"},
		{[]string{"game-center", "matchmaking", "rule-sets", "queues", "list"}, "--rule-set-id is required"},
	})
}
