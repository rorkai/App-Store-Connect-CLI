package gamecenter

import (
	"context"
	"flag"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newGameCenterMatchmakingCommand returns the matchmaking command group.
func newGameCenterMatchmakingCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("matchmaking", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "matchmaking",
		ShortUsage: "asc game-center matchmaking <subcommand> [flags]",
		ShortHelp:  "Manage Game Center matchmaking resources.",
		LongHelp: `Manage Game Center matchmaking resources.

Examples:
  asc game-center matchmaking queues list
  asc game-center matchmaking rule-sets list
  asc game-center matchmaking rules list --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking teams list --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking metrics queue-requests --queue-id "QUEUE_ID" --granularity P1D`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterMatchmakingQueuesCommand(d),
			newGameCenterMatchmakingRuleSetsCommand(d),
			newGameCenterMatchmakingRulesCommand(d),
			newGameCenterMatchmakingTeamsCommand(d),
			newGameCenterMatchmakingMetricsCommand(d),
			newGameCenterMatchmakingRuleSetTestsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterMatchmakingQueuesCommand returns the matchmaking queues command group.
func newGameCenterMatchmakingQueuesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("queues", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "queues",
		ShortUsage: "asc game-center matchmaking queues <subcommand> [flags]",
		ShortHelp:  "Manage matchmaking queues.",
		LongHelp: `Manage matchmaking queues.

Examples:
  asc game-center matchmaking queues list
  asc game-center matchmaking queues view --id "QUEUE_ID"
  asc game-center matchmaking queues create --reference-name "Queue 1" --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking queues update --id "QUEUE_ID" --classic-bundle-ids "com.example.app"
  asc game-center matchmaking queues delete --id "QUEUE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterMatchmakingQueuesListCommand(d),
			newGameCenterMatchmakingQueuesGetCommand(d),
			newGameCenterMatchmakingQueuesCreateCommand(d),
			newGameCenterMatchmakingQueuesUpdateCommand(d),
			newGameCenterMatchmakingQueuesDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterMatchmakingQueuesListCommand returns the matchmaking queues list subcommand.
func newGameCenterMatchmakingQueuesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center matchmaking queues list [flags]",
		ShortHelp:  "List matchmaking queues.",
		LongHelp: `List matchmaking queues.

Examples:
  asc game-center matchmaking queues list
  asc game-center matchmaking queues list --limit 50
  asc game-center matchmaking queues list --paginate`,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterMatchmakingQueues(ctx, asc.Limit[asc.GCMatchmakingQueuesOption](limit), asc.NextURL[asc.GCMatchmakingQueuesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterMatchmakingQueuesGetCommand returns the matchmaking queues get subcommand.
func newGameCenterMatchmakingQueuesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center matchmaking queues view --id \"QUEUE_ID\"",
		ShortHelp:  "View a matchmaking queue by ID.",
		LongHelp: `View a matchmaking queue by ID.

Examples:
  asc game-center matchmaking queues view --id "QUEUE_ID"`,
		IDUsage:            "Matchmaking queue ID",
		IDType:             "gameCenterMatchmakingQueues",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterMatchmakingQueue(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterMatchmakingQueuesCreateCommand returns the matchmaking queues create subcommand.
func newGameCenterMatchmakingQueuesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	referenceName := fs.String("reference-name", "", "Reference name for the queue")
	ruleSetID := shared.BindResourceIDFlag(fs, "rule-set-id", "gameCenterMatchmakingRuleSets", "Matchmaking rule set ID")
	experimentRuleSetID := shared.BindResourceIDFlag(fs, "experiment-rule-set-id", "gameCenterMatchmakingRuleSets", "Experiment rule set ID")
	classicBundleIDs := shared.BindOnceCSVFlag(fs, "classic-bundle-ids", "Comma-separated bundle IDs for classic matchmaking")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center matchmaking queues create --reference-name \"Queue\" --rule-set-id \"RULE_SET_ID\"",
		ShortHelp:  "Create a matchmaking queue.",
		LongHelp: `Create a matchmaking queue.

Examples:
  asc game-center matchmaking queues create --reference-name "Queue" --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking queues create --reference-name "Queue" --rule-set-id "RULE_SET_ID" --classic-bundle-ids "com.example.app"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			name := strings.TrimSpace(*referenceName)
			if name == "" {
				return d.RequiredFlag("--reference-name")
			}
			ruleSet := strings.TrimSpace(*ruleSetID)
			if ruleSet == "" {
				return d.RequiredFlag("--rule-set-id")
			}

			attrs := asc.GameCenterMatchmakingQueueCreateAttributes{
				ReferenceName:               name,
				ClassicMatchmakingBundleIDs: shared.SplitCSV(classicBundleIDs.String()),
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterMatchmakingQueue(requestCtx, attrs, ruleSet, strings.TrimSpace(*experimentRuleSetID))
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterMatchmakingQueuesUpdateCommand returns the matchmaking queues update subcommand.
func newGameCenterMatchmakingQueuesUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	queueID := shared.BindResourceIDFlag(fs, "id", "gameCenterMatchmakingQueues", "Matchmaking queue ID")
	ruleSetID := shared.BindResourceIDFlag(fs, "rule-set-id", "gameCenterMatchmakingRuleSets", "Matchmaking rule set ID")
	experimentRuleSetID := shared.BindResourceIDFlag(fs, "experiment-rule-set-id", "gameCenterMatchmakingRuleSets", "Experiment rule set ID")
	classicBundleIDs := shared.BindOnceCSVFlag(fs, "classic-bundle-ids", "Comma-separated bundle IDs for classic matchmaking")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center matchmaking queues update --id \"QUEUE_ID\" [flags]",
		ShortHelp:  "Update a matchmaking queue.",
		LongHelp: `Update a matchmaking queue.

Examples:
  asc game-center matchmaking queues update --id "QUEUE_ID" --classic-bundle-ids "com.example.app"
  asc game-center matchmaking queues update --id "QUEUE_ID" --rule-set-id "RULE_SET_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*queueID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			hasUpdate := false
			attrs := asc.GameCenterMatchmakingQueueUpdateAttributes{}
			if strings.TrimSpace(classicBundleIDs.String()) != "" {
				attrs.ClassicMatchmakingBundleIDs = shared.SplitCSV(classicBundleIDs.String())
				hasUpdate = true
			}

			if !hasUpdate && strings.TrimSpace(*ruleSetID) == "" && strings.TrimSpace(*experimentRuleSetID) == "" {
				return d.UpdateFlagRequired(fs, ruleSetID, experimentRuleSetID, classicBundleIDs)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterMatchmakingQueue(requestCtx, id, attrs, strings.TrimSpace(*ruleSetID), strings.TrimSpace(*experimentRuleSetID))
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterMatchmakingQueuesDeleteCommand returns the matchmaking queues delete subcommand.
func newGameCenterMatchmakingQueuesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center matchmaking queues delete --id \"QUEUE_ID\" --confirm",
		ShortHelp:  "Delete a matchmaking queue.",
		LongHelp: `Delete a matchmaking queue.

Examples:
  asc game-center matchmaking queues delete --id "QUEUE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Matchmaking queue ID",
		IDType:             "gameCenterMatchmakingQueues",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterMatchmakingQueue(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterMatchmakingRuleSetsCommand returns the matchmaking rule sets command group.
func newGameCenterMatchmakingRuleSetsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("rule-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "rule-sets",
		ShortUsage: "asc game-center matchmaking rule-sets <subcommand> [flags]",
		ShortHelp:  "Manage matchmaking rule sets.",
		LongHelp: `Manage matchmaking rule sets.

Examples:
  asc game-center matchmaking rule-sets list
  asc game-center matchmaking rule-sets view --id "RULE_SET_ID"
  asc game-center matchmaking rule-sets create --reference-name "Rules" --rule-language-version 1 --min-players 2 --max-players 8
  asc game-center matchmaking rule-sets update --id "RULE_SET_ID" --min-players 2
  asc game-center matchmaking rule-sets delete --id "RULE_SET_ID" --confirm
  asc game-center matchmaking rule-sets queues list --rule-set-id "RULE_SET_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterMatchmakingRuleSetsListCommand(d),
			newGameCenterMatchmakingRuleSetsGetCommand(d),
			newGameCenterMatchmakingRuleSetsCreateCommand(d),
			newGameCenterMatchmakingRuleSetsUpdateCommand(d),
			newGameCenterMatchmakingRuleSetsDeleteCommand(d),
			newGameCenterMatchmakingRuleSetQueuesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterMatchmakingRuleSetsListCommand returns the rule sets list subcommand.
func newGameCenterMatchmakingRuleSetsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center matchmaking rule-sets list [flags]",
		ShortHelp:  "List matchmaking rule sets.",
		LongHelp: `List matchmaking rule sets.

Examples:
  asc game-center matchmaking rule-sets list
  asc game-center matchmaking rule-sets list --limit 50
  asc game-center matchmaking rule-sets list --paginate`,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterMatchmakingRuleSets(ctx, asc.Limit[asc.GCMatchmakingRuleSetsOption](limit), asc.NextURL[asc.GCMatchmakingRuleSetsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterMatchmakingRuleSetsGetCommand returns the rule sets get subcommand.
func newGameCenterMatchmakingRuleSetsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center matchmaking rule-sets view --id \"RULE_SET_ID\"",
		ShortHelp:  "View a matchmaking rule set by ID.",
		LongHelp: `View a matchmaking rule set by ID.

Examples:
  asc game-center matchmaking rule-sets view --id "RULE_SET_ID"`,
		IDUsage:            "Matchmaking rule set ID",
		IDType:             "gameCenterMatchmakingRuleSets",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterMatchmakingRuleSet(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterMatchmakingRuleSetsCreateCommand returns the rule sets create subcommand.
func newGameCenterMatchmakingRuleSetsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	referenceName := fs.String("reference-name", "", "Reference name for the rule set")
	ruleLanguageVersion := fs.Int("rule-language-version", 0, "Rule language version")
	minPlayers := fs.Int("min-players", 0, "Minimum players")
	maxPlayers := fs.Int("max-players", 0, "Maximum players")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center matchmaking rule-sets create --reference-name \"Rules\" --rule-language-version 1 --min-players 2 --max-players 8",
		ShortHelp:  "Create a matchmaking rule set.",
		LongHelp: `Create a matchmaking rule set.

Examples:
  asc game-center matchmaking rule-sets create --reference-name "Rules" --rule-language-version 1 --min-players 2 --max-players 8`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			name := strings.TrimSpace(*referenceName)
			if name == "" {
				return d.RequiredFlag("--reference-name")
			}
			if *ruleLanguageVersion == 0 {
				return d.RequiredFlag("--rule-language-version")
			}
			if *minPlayers == 0 || *maxPlayers == 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: --min-players and --max-players are required")
				return shared.MissingRequiredUsageError("")
			}

			attrs := asc.GameCenterMatchmakingRuleSetCreateAttributes{
				ReferenceName:       name,
				RuleLanguageVersion: *ruleLanguageVersion,
				MinPlayers:          *minPlayers,
				MaxPlayers:          *maxPlayers,
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterMatchmakingRuleSet(requestCtx, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterMatchmakingRuleSetsUpdateCommand returns the rule sets update subcommand.
func newGameCenterMatchmakingRuleSetsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	ruleSetID := shared.BindResourceIDFlag(fs, "id", "gameCenterMatchmakingRuleSets", "Matchmaking rule set ID")
	minPlayers := fs.Int("min-players", 0, "Minimum players")
	maxPlayers := fs.Int("max-players", 0, "Maximum players")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center matchmaking rule-sets update --id \"RULE_SET_ID\" [flags]",
		ShortHelp:  "Update a matchmaking rule set.",
		LongHelp: `Update a matchmaking rule set.

Examples:
  asc game-center matchmaking rule-sets update --id "RULE_SET_ID" --min-players 2
  asc game-center matchmaking rule-sets update --id "RULE_SET_ID" --max-players 8`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*ruleSetID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			attrs := asc.GameCenterMatchmakingRuleSetUpdateAttributes{}
			hasUpdate := false

			if *minPlayers > 0 {
				value := *minPlayers
				attrs.MinPlayers = &value
				hasUpdate = true
			}
			if *maxPlayers > 0 {
				value := *maxPlayers
				attrs.MaxPlayers = &value
				hasUpdate = true
			}

			if !hasUpdate {
				return d.UpdateFlagRequired(fs, minPlayers, maxPlayers)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterMatchmakingRuleSet(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterMatchmakingRuleSetsDeleteCommand returns the rule sets delete subcommand.
func newGameCenterMatchmakingRuleSetsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center matchmaking rule-sets delete --id \"RULE_SET_ID\" --confirm",
		ShortHelp:  "Delete a matchmaking rule set.",
		LongHelp: `Delete a matchmaking rule set.

Examples:
  asc game-center matchmaking rule-sets delete --id "RULE_SET_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Matchmaking rule set ID",
		IDType:             "gameCenterMatchmakingRuleSets",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterMatchmakingRuleSet(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterMatchmakingRuleSetQueuesCommand returns the rule set queues command group.
func newGameCenterMatchmakingRuleSetQueuesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("queues", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "queues",
		ShortUsage: "asc game-center matchmaking rule-sets queues list --rule-set-id \"RULE_SET_ID\"",
		ShortHelp:  "List matchmaking queues for a rule set.",
		LongHelp: `List matchmaking queues for a rule set.

Examples:
  asc game-center matchmaking rule-sets queues list --rule-set-id "RULE_SET_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterMatchmakingRuleSetQueuesListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterMatchmakingRuleSetQueuesListCommand returns the rule set queues list subcommand.
func newGameCenterMatchmakingRuleSetQueuesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center matchmaking rule-sets queues list --rule-set-id \"RULE_SET_ID\"",
		ShortHelp:  "List queues for a matchmaking rule set.",
		LongHelp: `List queues for a matchmaking rule set.

Examples:
  asc game-center matchmaking rule-sets queues list --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking rule-sets queues list --rule-set-id "RULE_SET_ID" --limit 50
  asc game-center matchmaking rule-sets queues list --rule-set-id "RULE_SET_ID" --paginate`,
		ParentFlag:           "rule-set-id",
		ParentUsage:          "Matchmaking rule set ID",
		ParentType:           "gameCenterMatchmakingRuleSets",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, ruleSetId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterMatchmakingRuleSetQueues(ctx, ruleSetId, asc.Limit[asc.GCMatchmakingQueuesOption](limit), asc.NextURL[asc.GCMatchmakingQueuesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterMatchmakingRulesCommand returns the matchmaking rules command group.
func newGameCenterMatchmakingRulesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("rules", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "rules",
		ShortUsage: "asc game-center matchmaking rules <subcommand> [flags]",
		ShortHelp:  "Manage matchmaking rules.",
		LongHelp: `Manage matchmaking rules.

Examples:
  asc game-center matchmaking rules list --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking rules create --rule-set-id "RULE_SET_ID" --reference-name "Rule" --description "Match" --type MATCH --expression "player.level > 1"
  asc game-center matchmaking rules update --id "RULE_ID" --description "New description"
  asc game-center matchmaking rules delete --id "RULE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterMatchmakingRulesListCommand(d),
			newGameCenterMatchmakingRulesCreateCommand(d),
			newGameCenterMatchmakingRulesUpdateCommand(d),
			newGameCenterMatchmakingRulesDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterMatchmakingRulesListCommand returns the rules list subcommand.
func newGameCenterMatchmakingRulesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center matchmaking rules list --rule-set-id \"RULE_SET_ID\"",
		ShortHelp:  "List matchmaking rules for a rule set.",
		LongHelp: `List matchmaking rules for a rule set.

Examples:
  asc game-center matchmaking rules list --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking rules list --rule-set-id "RULE_SET_ID" --limit 50
  asc game-center matchmaking rules list --rule-set-id "RULE_SET_ID" --paginate`,
		ParentFlag:           "rule-set-id",
		ParentUsage:          "Matchmaking rule set ID",
		ParentType:           "gameCenterMatchmakingRuleSets",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, ruleSetId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterMatchmakingRules(ctx, ruleSetId, asc.Limit[asc.GCMatchmakingRulesOption](limit), asc.NextURL[asc.GCMatchmakingRulesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterMatchmakingRulesCreateCommand returns the rules create subcommand.
func newGameCenterMatchmakingRulesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	ruleSetID := shared.BindResourceIDFlag(fs, "rule-set-id", "gameCenterMatchmakingRuleSets", "Matchmaking rule set ID")
	referenceName := fs.String("reference-name", "", "Reference name for the rule")
	description := fs.String("description", "", "Rule description")
	ruleType := fs.String("type", "", "Rule type (COMPATIBLE, DISTANCE, MATCH, TEAM)")
	expression := fs.String("expression", "", "Rule expression")
	weight := fs.String("weight", "", "Rule weight (finite number)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center matchmaking rules create --rule-set-id \"RULE_SET_ID\" --reference-name \"Rule\" --description \"Match\" --type MATCH --expression \"player.level > 1\"",
		ShortHelp:  "Create a matchmaking rule.",
		LongHelp: `Create a matchmaking rule.

Examples:
  asc game-center matchmaking rules create --rule-set-id "RULE_SET_ID" --reference-name "Rule" --description "Match" --type MATCH --expression "player.level > 1"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ruleSet := strings.TrimSpace(*ruleSetID)
			if ruleSet == "" {
				return d.RequiredFlag("--rule-set-id")
			}
			name := strings.TrimSpace(*referenceName)
			if name == "" {
				return d.RequiredFlag("--reference-name")
			}
			desc := strings.TrimSpace(*description)
			if desc == "" {
				return d.RequiredFlag("--description")
			}
			rtype := strings.TrimSpace(*ruleType)
			if rtype == "" {
				return d.RequiredFlag("--type")
			}
			expr := strings.TrimSpace(*expression)
			if expr == "" {
				return d.RequiredFlag("--expression")
			}

			attrs := asc.GameCenterMatchmakingRuleCreateAttributes{
				ReferenceName: name,
				Description:   desc,
				Type:          rtype,
				Expression:    expr,
			}

			if strings.TrimSpace(*weight) != "" {
				val, err := parseMatchmakingRuleWeight(*weight)
				if err != nil {
					return d.UsageError(err.Error())
				}
				attrs.Weight = &val
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rules create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterMatchmakingRule(requestCtx, ruleSet, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rules create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterMatchmakingRulesUpdateCommand returns the rules update subcommand.
func newGameCenterMatchmakingRulesUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	ruleID := shared.BindResourceIDFlag(fs, "id", "gameCenterMatchmakingRules", "Matchmaking rule ID")
	description := fs.String("description", "", "Rule description")
	expression := fs.String("expression", "", "Rule expression")
	weight := fs.String("weight", "", "Rule weight (finite number)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center matchmaking rules update --id \"RULE_ID\" [flags]",
		ShortHelp:  "Update a matchmaking rule.",
		LongHelp: `Update a matchmaking rule.

Examples:
  asc game-center matchmaking rules update --id "RULE_ID" --description "New description"
  asc game-center matchmaking rules update --id "RULE_ID" --weight 0.5`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*ruleID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			attrs := asc.GameCenterMatchmakingRuleUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*description) != "" {
				value := strings.TrimSpace(*description)
				attrs.Description = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*expression) != "" {
				value := strings.TrimSpace(*expression)
				attrs.Expression = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*weight) != "" {
				val, err := parseMatchmakingRuleWeight(*weight)
				if err != nil {
					return d.UsageError(err.Error())
				}
				attrs.Weight = &val
				hasUpdate = true
			}

			if !hasUpdate {
				return d.UpdateFlagRequired(fs, description, expression, weight)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rules update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterMatchmakingRule(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rules update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func parseMatchmakingRuleWeight(value string) (float64, error) {
	weight, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("--weight must be a number")
	}
	if math.IsNaN(weight) || math.IsInf(weight, 0) {
		return 0, fmt.Errorf("--weight must be a finite number")
	}
	return weight, nil
}

// newGameCenterMatchmakingRulesDeleteCommand returns the rules delete subcommand.
func newGameCenterMatchmakingRulesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center matchmaking rules delete --id \"RULE_ID\" --confirm",
		ShortHelp:  "Delete a matchmaking rule.",
		LongHelp: `Delete a matchmaking rule.

Examples:
  asc game-center matchmaking rules delete --id "RULE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Matchmaking rule ID",
		IDType:             "gameCenterMatchmakingRules",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterMatchmakingRule(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterMatchmakingTeamsCommand returns the matchmaking teams command group.
func newGameCenterMatchmakingTeamsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("teams", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "teams",
		ShortUsage: "asc game-center matchmaking teams <subcommand> [flags]",
		ShortHelp:  "Manage matchmaking teams.",
		LongHelp: `Manage matchmaking teams.

Examples:
  asc game-center matchmaking teams list --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking teams create --rule-set-id "RULE_SET_ID" --reference-name "Team" --min-players 1 --max-players 4
  asc game-center matchmaking teams update --id "TEAM_ID" --min-players 1
  asc game-center matchmaking teams delete --id "TEAM_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterMatchmakingTeamsListCommand(d),
			newGameCenterMatchmakingTeamsCreateCommand(d),
			newGameCenterMatchmakingTeamsUpdateCommand(d),
			newGameCenterMatchmakingTeamsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterMatchmakingTeamsListCommand returns the teams list subcommand.
func newGameCenterMatchmakingTeamsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center matchmaking teams list --rule-set-id \"RULE_SET_ID\"",
		ShortHelp:  "List matchmaking teams for a rule set.",
		LongHelp: `List matchmaking teams for a rule set.

Examples:
  asc game-center matchmaking teams list --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking teams list --rule-set-id "RULE_SET_ID" --limit 50
  asc game-center matchmaking teams list --rule-set-id "RULE_SET_ID" --paginate`,
		ParentFlag:           "rule-set-id",
		ParentUsage:          "Matchmaking rule set ID",
		ParentType:           "gameCenterMatchmakingRuleSets",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, ruleSetId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterMatchmakingTeams(ctx, ruleSetId, asc.Limit[asc.GCMatchmakingTeamsOption](limit), asc.NextURL[asc.GCMatchmakingTeamsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterMatchmakingTeamsCreateCommand returns the teams create subcommand.
func newGameCenterMatchmakingTeamsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	ruleSetID := shared.BindResourceIDFlag(fs, "rule-set-id", "gameCenterMatchmakingRuleSets", "Matchmaking rule set ID")
	referenceName := fs.String("reference-name", "", "Reference name for the team")
	minPlayers := fs.Int("min-players", 0, "Minimum players")
	maxPlayers := fs.Int("max-players", 0, "Maximum players")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center matchmaking teams create --rule-set-id \"RULE_SET_ID\" --reference-name \"Team\" --min-players 1 --max-players 4",
		ShortHelp:  "Create a matchmaking team.",
		LongHelp: `Create a matchmaking team.

Examples:
  asc game-center matchmaking teams create --rule-set-id "RULE_SET_ID" --reference-name "Team" --min-players 1 --max-players 4`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ruleSet := strings.TrimSpace(*ruleSetID)
			if ruleSet == "" {
				return d.RequiredFlag("--rule-set-id")
			}
			name := strings.TrimSpace(*referenceName)
			if name == "" {
				return d.RequiredFlag("--reference-name")
			}
			if *minPlayers == 0 || *maxPlayers == 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: --min-players and --max-players are required")
				return shared.MissingRequiredUsageError("")
			}

			attrs := asc.GameCenterMatchmakingTeamCreateAttributes{
				ReferenceName: name,
				MinPlayers:    *minPlayers,
				MaxPlayers:    *maxPlayers,
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center matchmaking teams create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterMatchmakingTeam(requestCtx, ruleSet, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking teams create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterMatchmakingTeamsUpdateCommand returns the teams update subcommand.
func newGameCenterMatchmakingTeamsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	teamID := shared.BindResourceIDFlag(fs, "id", "gameCenterMatchmakingTeams", "Matchmaking team ID")
	minPlayers := fs.Int("min-players", 0, "Minimum players")
	maxPlayers := fs.Int("max-players", 0, "Maximum players")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center matchmaking teams update --id \"TEAM_ID\" [flags]",
		ShortHelp:  "Update a matchmaking team.",
		LongHelp: `Update a matchmaking team.

Examples:
  asc game-center matchmaking teams update --id "TEAM_ID" --min-players 1
  asc game-center matchmaking teams update --id "TEAM_ID" --max-players 4`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*teamID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			attrs := asc.GameCenterMatchmakingTeamUpdateAttributes{}
			hasUpdate := false
			if *minPlayers > 0 {
				value := *minPlayers
				attrs.MinPlayers = &value
				hasUpdate = true
			}
			if *maxPlayers > 0 {
				value := *maxPlayers
				attrs.MaxPlayers = &value
				hasUpdate = true
			}
			if !hasUpdate {
				return d.UpdateFlagRequired(fs, minPlayers, maxPlayers)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center matchmaking teams update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterMatchmakingTeam(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking teams update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterMatchmakingTeamsDeleteCommand returns the teams delete subcommand.
func newGameCenterMatchmakingTeamsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center matchmaking teams delete --id \"TEAM_ID\" --confirm",
		ShortHelp:  "Delete a matchmaking team.",
		LongHelp: `Delete a matchmaking team.

Examples:
  asc game-center matchmaking teams delete --id "TEAM_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Matchmaking team ID",
		IDType:             "gameCenterMatchmakingTeams",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterMatchmakingTeam(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterMatchmakingMetricsCommand returns the matchmaking metrics command group.
func newGameCenterMatchmakingMetricsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("metrics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metrics",
		ShortUsage: "asc game-center matchmaking metrics <subcommand> [flags]",
		ShortHelp:  "Fetch Game Center matchmaking metrics.",
		LongHelp: `Fetch Game Center matchmaking metrics.

Examples:
  asc game-center matchmaking metrics queue-sizes --queue-id "QUEUE_ID" --granularity P1D
  asc game-center matchmaking metrics queue-requests --queue-id "QUEUE_ID" --granularity P1D --group-by result
  asc game-center matchmaking metrics rule-errors --rule-id "RULE_ID" --granularity P1D`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterMatchmakingQueueSizesCommand(d),
			newGameCenterMatchmakingQueueRequestsCommand(d),
			newGameCenterMatchmakingQueueSessionsCommand(d),
			newGameCenterMatchmakingQueueExperimentSizesCommand(d),
			newGameCenterMatchmakingQueueExperimentRequestsCommand(d),
			newGameCenterMatchmakingBooleanRuleResultsCommand(d),
			newGameCenterMatchmakingNumberRuleResultsCommand(d),
			newGameCenterMatchmakingRuleErrorsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterMatchmakingQueueSizesCommand returns the queue sizes metrics subcommand.
func newGameCenterMatchmakingQueueSizesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("queue-sizes", flag.ExitOnError)

	queueID := shared.BindResourceIDFlag(fs, "queue-id", "gameCenterMatchmakingQueues", "Matchmaking queue ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return metricsQueueCommand(d, "queue-sizes", fs, queueID, granularity, sort, limit, next, paginate, output.Output, output.Pretty, func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueSizesResponse, error) {
		return client.GetGameCenterMatchmakingQueueSizes(ctx, id, opts...)
	})
}

// newGameCenterMatchmakingQueueRequestsCommand returns the queue requests metrics subcommand.
func newGameCenterMatchmakingQueueRequestsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("queue-requests", flag.ExitOnError)

	queueID := shared.BindResourceIDFlag(fs, "queue-id", "gameCenterMatchmakingQueues", "Matchmaking queue ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result, gameCenterDetail)")
	filterResult := fs.String("filter-result", "", "Filter result (MATCHED, CANCELED, EXPIRED)")
	filterDetail := shared.BindResourceIDFlag(fs, "filter-detail", "gameCenterDetails", "Filter by Game Center detail ID")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return metricsQueueCommandWithFilters(d, "queue-requests", fs, queueID, granularity, groupBy, filterResult, filterDetail, sort, limit, next, paginate, output.Output, output.Pretty, func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueRequestsResponse, error) {
		return client.GetGameCenterMatchmakingQueueRequests(ctx, id, opts...)
	})
}

// newGameCenterMatchmakingQueueSessionsCommand returns the queue sessions metrics subcommand.
func newGameCenterMatchmakingQueueSessionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("queue-sessions", flag.ExitOnError)

	queueID := shared.BindResourceIDFlag(fs, "queue-id", "gameCenterMatchmakingQueues", "Matchmaking queue ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return metricsQueueCommand(d, "queue-sessions", fs, queueID, granularity, sort, limit, next, paginate, output.Output, output.Pretty, func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueSessionsResponse, error) {
		return client.GetGameCenterMatchmakingQueueSessions(ctx, id, opts...)
	})
}

// newGameCenterMatchmakingQueueExperimentSizesCommand returns the experiment queue sizes metrics subcommand.
func newGameCenterMatchmakingQueueExperimentSizesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiment-queue-sizes", flag.ExitOnError)

	queueID := shared.BindResourceIDFlag(fs, "queue-id", "gameCenterMatchmakingQueues", "Matchmaking queue ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return metricsQueueCommand(d, "experiment-queue-sizes", fs, queueID, granularity, sort, limit, next, paginate, output.Output, output.Pretty, func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueExperimentSizesResponse, error) {
		return client.GetGameCenterMatchmakingQueueExperimentSizes(ctx, id, opts...)
	})
}

// newGameCenterMatchmakingQueueExperimentRequestsCommand returns the experiment queue requests metrics subcommand.
func newGameCenterMatchmakingQueueExperimentRequestsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiment-queue-requests", flag.ExitOnError)

	queueID := shared.BindResourceIDFlag(fs, "queue-id", "gameCenterMatchmakingQueues", "Matchmaking queue ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result, gameCenterDetail)")
	filterResult := fs.String("filter-result", "", "Filter result (MATCHED, CANCELED, EXPIRED)")
	filterDetail := shared.BindResourceIDFlag(fs, "filter-detail", "gameCenterDetails", "Filter by Game Center detail ID")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return metricsQueueCommandWithFilters(d, "experiment-queue-requests", fs, queueID, granularity, groupBy, filterResult, filterDetail, sort, limit, next, paginate, output.Output, output.Pretty, func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueExperimentRequestsResponse, error) {
		return client.GetGameCenterMatchmakingQueueExperimentRequests(ctx, id, opts...)
	})
}

// ruleMetricsSupport describes the query dimensions a matchmaking rule metrics
// endpoint accepts. Only matchmakingBooleanRuleResults documents the result
// dimension; matchmakingNumberRuleResults and matchmakingRuleErrors reject both
// `filter[result]` and `groupBy=result`.
type ruleMetricsSupport struct {
	groupBy      []string
	filterResult bool
}

var (
	booleanRuleMetricsSupport = ruleMetricsSupport{
		groupBy:      []string{"result", "gameCenterMatchmakingQueue"},
		filterResult: true,
	}
	queueOnlyRuleMetricsSupport = ruleMetricsSupport{
		groupBy: []string{"gameCenterMatchmakingQueue"},
	}
)

// newGameCenterMatchmakingBooleanRuleResultsCommand returns the boolean rule results metrics subcommand.
func newGameCenterMatchmakingBooleanRuleResultsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("rule-boolean-results", flag.ExitOnError)

	ruleID := shared.BindResourceIDFlag(fs, "rule-id", "gameCenterMatchmakingRules", "Matchmaking rule ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result, gameCenterMatchmakingQueue)")
	filterResult := fs.String("filter-result", "", "Filter result")
	filterQueue := shared.BindResourceIDFlag(fs, "filter-queue", "gameCenterMatchmakingQueues", "Filter by matchmaking queue ID")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return metricsRuleCommand(d, "rule-boolean-results", fs, booleanRuleMetricsSupport, ruleID, granularity, groupBy, filterResult, filterQueue, sort, limit, next, paginate, output.Output, output.Pretty, func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingBooleanRuleResultsResponse, error) {
		return client.GetGameCenterMatchmakingBooleanRuleResults(ctx, id, opts...)
	})
}

// newGameCenterMatchmakingNumberRuleResultsCommand returns the number rule results metrics subcommand.
func newGameCenterMatchmakingNumberRuleResultsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("rule-number-results", flag.ExitOnError)

	ruleID := shared.BindResourceIDFlag(fs, "rule-id", "gameCenterMatchmakingRules", "Matchmaking rule ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: gameCenterMatchmakingQueue)")
	filterResult := fs.String("filter-result", "", "Filter result (supported only by rule-boolean-results)")
	filterQueue := shared.BindResourceIDFlag(fs, "filter-queue", "gameCenterMatchmakingQueues", "Filter by matchmaking queue ID")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return metricsRuleCommand(d, "rule-number-results", fs, queueOnlyRuleMetricsSupport, ruleID, granularity, groupBy, filterResult, filterQueue, sort, limit, next, paginate, output.Output, output.Pretty, func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingNumberRuleResultsResponse, error) {
		return client.GetGameCenterMatchmakingNumberRuleResults(ctx, id, opts...)
	})
}

// newGameCenterMatchmakingRuleErrorsCommand returns the rule errors metrics subcommand.
func newGameCenterMatchmakingRuleErrorsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("rule-errors", flag.ExitOnError)

	ruleID := shared.BindResourceIDFlag(fs, "rule-id", "gameCenterMatchmakingRules", "Matchmaking rule ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: gameCenterMatchmakingQueue)")
	filterResult := fs.String("filter-result", "", "Filter result (supported only by rule-boolean-results)")
	filterQueue := shared.BindResourceIDFlag(fs, "filter-queue", "gameCenterMatchmakingQueues", "Filter by matchmaking queue ID")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return metricsRuleCommand(d, "rule-errors", fs, queueOnlyRuleMetricsSupport, ruleID, granularity, groupBy, filterResult, filterQueue, sort, limit, next, paginate, output.Output, output.Pretty, func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingRuleErrorsResponse, error) {
		return client.GetGameCenterMatchmakingRuleErrors(ctx, id, opts...)
	})
}

func metricsQueueCommand(d *shared.Deps, name string, fs *flag.FlagSet, queueID *string, granularity *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueSizesResponse, error)) *ffcli.Command {
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc game-center matchmaking metrics " + name + " --queue-id \"QUEUE_ID\" --granularity P1D",
		ShortHelp:  "Fetch matchmaking queue metrics.",
		LongHelp: `Fetch matchmaking queue metrics.

Examples:
  asc game-center matchmaking metrics ` + name + ` --queue-id "QUEUE_ID" --granularity P1D`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runMetricsQueue(ctx, d, name, fs, queueID, granularity, sort, limit, next, paginate, output, pretty, fetch, nil, "", "", "")
		},
	}
}

func metricsQueueCommandWithFilters(d *shared.Deps, name string, fs *flag.FlagSet, queueID *string, granularity *string, groupBy *string, filterResult *string, filterDetail *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueRequestsResponse, error)) *ffcli.Command {
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc game-center matchmaking metrics " + name + " --queue-id \"QUEUE_ID\" --granularity P1D",
		ShortHelp:  "Fetch matchmaking queue request metrics.",
		LongHelp: `Fetch matchmaking queue request metrics.

Examples:
  asc game-center matchmaking metrics ` + name + ` --queue-id "QUEUE_ID" --granularity P1D --group-by result`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runMetricsQueue(ctx, d, name, fs, queueID, granularity, sort, limit, next, paginate, output, pretty, nil, fetch, *groupBy, *filterResult, *filterDetail)
		},
	}
}

func metricsRuleCommand(d *shared.Deps, name string, fs *flag.FlagSet, support ruleMetricsSupport, ruleID *string, granularity *string, groupBy *string, filterResult *string, filterQueue *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingBooleanRuleResultsResponse, error)) *ffcli.Command {
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc game-center matchmaking metrics " + name + " --rule-id \"RULE_ID\" --granularity P1D",
		ShortHelp:  "Fetch matchmaking rule metrics.",
		LongHelp: `Fetch matchmaking rule metrics.

Examples:
  asc game-center matchmaking metrics ` + name + ` --rule-id "RULE_ID" --granularity P1D --group-by ` + support.groupBy[0],
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runMetricsRule(ctx, d, name, fs, support, ruleID, granularity, groupBy, filterResult, filterQueue, sort, limit, next, paginate, output, pretty, fetch)
		},
	}
}

// resolveRuleMetricsDimensions rejects group-by and filter values the endpoint
// does not document, and returns the canonical group-by dimensions to send.
func resolveRuleMetricsDimensions(d *shared.Deps, name string, support ruleMetricsSupport, groupBy string, filterResult string) ([]string, error) {
	if strings.TrimSpace(filterResult) != "" && !support.filterResult {
		return nil, d.UsageErrorf("game-center matchmaking metrics %s: --filter-result is not supported by this endpoint (supported filters: --filter-queue)", name)
	}

	requested := shared.SplitCSV(groupBy)
	dimensions := make([]string, 0, len(requested))
	for _, value := range requested {
		canonical, ok := canonicalRuleMetricsGroupBy(support.groupBy, value)
		if !ok {
			return nil, d.UsageErrorf("game-center matchmaking metrics %s: unsupported --group-by value %q (supported values: %s)", name, value, strings.Join(support.groupBy, ", "))
		}
		dimensions = append(dimensions, canonical)
	}
	return dimensions, nil
}

func canonicalRuleMetricsGroupBy(supported []string, value string) (string, bool) {
	for _, candidate := range supported {
		if strings.EqualFold(candidate, value) {
			return candidate, true
		}
	}
	return "", false
}

func runMetricsQueue(ctx context.Context, d *shared.Deps, name string, fs *flag.FlagSet, queueID *string, granularity *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetchSizes func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueSizesResponse, error), fetchRequests func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueRequestsResponse, error), groupBy string, filterResult string, filterDetail string) error {
	if err := d.ValidatePaginationFlags(ctx, fs, "game-center matchmaking metrics "+name, 200, "granularity", "group-by", "filter-result", "filter-detail", "filter-queue", "sort"); err != nil {
		return err
	}

	id := strings.TrimSpace(*queueID)
	if id == "" && strings.TrimSpace(*next) == "" {
		return d.RequiredFlag("--queue-id")
	}
	gran := strings.TrimSpace(*granularity)
	if gran == "" && strings.TrimSpace(*next) == "" {
		return d.RequiredFlag("--granularity")
	}

	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("game-center matchmaking metrics %s: %w", name, err)
	}

	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	opts := []asc.GCMatchmakingMetricsOption{
		asc.WithGCMatchmakingMetricsGranularity(gran),
		asc.WithGCMatchmakingMetricsSort(shared.SplitCSV(*sort)),
		asc.Limit[asc.GCMatchmakingMetricsOption](*limit),
		asc.NextURL[asc.GCMatchmakingMetricsOption](*next),
	}
	if strings.TrimSpace(groupBy) != "" {
		opts = append(opts, asc.WithGCMatchmakingMetricsGroupBy(shared.SplitCSV(groupBy)))
	}
	if strings.TrimSpace(filterResult) != "" {
		opts = append(opts, asc.WithGCMatchmakingMetricsFilterResult(filterResult))
	}
	if strings.TrimSpace(filterDetail) != "" {
		opts = append(opts, asc.WithGCMatchmakingMetricsFilterGameCenterDetail(filterDetail))
	}

	if *paginate {
		paginateOpts := append(opts, asc.Limit[asc.GCMatchmakingMetricsOption](200))
		var firstPage asc.PaginatedResponse
		var err error
		if fetchRequests != nil {
			firstPage, err = fetchRequests(client, requestCtx, id, paginateOpts...)
		} else {
			firstPage, err = fetchSizes(client, requestCtx, id, paginateOpts...)
		}
		if err != nil {
			return fmt.Errorf("game-center matchmaking metrics %s: failed to fetch: %w", name, err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			if fetchRequests != nil {
				return fetchRequests(client, ctx, id, asc.NextURL[asc.GCMatchmakingMetricsOption](nextURL))
			}
			return fetchSizes(client, ctx, id, asc.NextURL[asc.GCMatchmakingMetricsOption](nextURL))
		})
		if err != nil {
			return fmt.Errorf("game-center matchmaking metrics %s: %w", name, err)
		}

		return d.PrintOutput(resp, *output, *pretty)
	}

	var resp any
	if fetchRequests != nil {
		resp, err = fetchRequests(client, requestCtx, id, opts...)
	} else {
		resp, err = fetchSizes(client, requestCtx, id, opts...)
	}
	if err != nil {
		return fmt.Errorf("game-center matchmaking metrics %s: failed to fetch: %w", name, err)
	}

	return d.PrintOutput(resp, *output, *pretty)
}

func runMetricsRule(ctx context.Context, d *shared.Deps, name string, fs *flag.FlagSet, support ruleMetricsSupport, ruleID *string, granularity *string, groupBy *string, filterResult *string, filterQueue *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingBooleanRuleResultsResponse, error)) error {
	if err := d.ValidatePaginationFlags(ctx, fs, "game-center matchmaking metrics "+name, 200, "granularity", "group-by", "filter-result", "filter-detail", "filter-queue", "sort"); err != nil {
		return err
	}

	id := strings.TrimSpace(*ruleID)
	if id == "" && strings.TrimSpace(*next) == "" {
		return d.RequiredFlag("--rule-id")
	}
	gran := strings.TrimSpace(*granularity)
	if gran == "" && strings.TrimSpace(*next) == "" {
		return d.RequiredFlag("--granularity")
	}

	dimensions, err := resolveRuleMetricsDimensions(d, name, support, *groupBy, *filterResult)
	if err != nil {
		return err
	}

	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("game-center matchmaking metrics %s: %w", name, err)
	}

	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	opts := []asc.GCMatchmakingMetricsOption{
		asc.WithGCMatchmakingMetricsGranularity(gran),
		asc.WithGCMatchmakingMetricsGroupBy(dimensions),
		asc.WithGCMatchmakingMetricsFilterQueue(strings.TrimSpace(*filterQueue)),
		asc.WithGCMatchmakingMetricsSort(shared.SplitCSV(*sort)),
		asc.Limit[asc.GCMatchmakingMetricsOption](*limit),
		asc.NextURL[asc.GCMatchmakingMetricsOption](*next),
	}
	if support.filterResult && strings.TrimSpace(*filterResult) != "" {
		opts = append(opts, asc.WithGCMatchmakingMetricsFilterResult(strings.TrimSpace(*filterResult)))
	}

	if *paginate {
		paginateOpts := append(opts, asc.Limit[asc.GCMatchmakingMetricsOption](200))
		firstPage, err := fetch(client, requestCtx, id, paginateOpts...)
		if err != nil {
			return fmt.Errorf("game-center matchmaking metrics %s: failed to fetch: %w", name, err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return fetch(client, ctx, id, asc.NextURL[asc.GCMatchmakingMetricsOption](nextURL))
		})
		if err != nil {
			return fmt.Errorf("game-center matchmaking metrics %s: %w", name, err)
		}

		return d.PrintOutput(resp, *output, *pretty)
	}

	resp, err := fetch(client, requestCtx, id, opts...)
	if err != nil {
		return fmt.Errorf("game-center matchmaking metrics %s: failed to fetch: %w", name, err)
	}

	return d.PrintOutput(resp, *output, *pretty)
}

// newGameCenterMatchmakingRuleSetTestsCommand returns the rule set tests command group.
func newGameCenterMatchmakingRuleSetTestsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("rule-set-tests", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "rule-set-tests",
		ShortUsage: "asc game-center matchmaking rule-set-tests create --file payload.json",
		ShortHelp:  "Run matchmaking rule set tests.",
		LongHelp: `Run matchmaking rule set tests.

Examples:
  asc game-center matchmaking rule-set-tests create --file payload.json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterMatchmakingRuleSetTestsCreateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterMatchmakingRuleSetTestsCreateCommand returns the rule set tests create subcommand.
func newGameCenterMatchmakingRuleSetTestsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	filePath := fs.String("file", "", "Path to rule set test JSON payload")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center matchmaking rule-set-tests create --file payload.json",
		ShortHelp:  "Create a matchmaking rule set test.",
		LongHelp: `Create a matchmaking rule set test from a JSON payload.

Examples:
  asc game-center matchmaking rule-set-tests create --file payload.json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			path := strings.TrimSpace(*filePath)
			if path == "" {
				return d.RequiredFlag("--file")
			}

			payload, err := d.ReadJSONFilePayload(path)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-set-tests create: %w", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-set-tests create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterMatchmakingRuleSetTest(requestCtx, payload)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-set-tests create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
