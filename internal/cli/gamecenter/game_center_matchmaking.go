package gamecenter

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// GameCenterMatchmakingCommand returns the matchmaking command group.
func GameCenterMatchmakingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("matchmaking", flag.ExitOnError)

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
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterMatchmakingQueuesCommand(),
			GameCenterMatchmakingRuleSetsCommand(),
			GameCenterMatchmakingRulesCommand(),
			GameCenterMatchmakingTeamsCommand(),
			GameCenterMatchmakingMetricsCommand(),
			GameCenterMatchmakingRuleSetTestsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterMatchmakingQueuesCommand returns the matchmaking queues command group.
func GameCenterMatchmakingQueuesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("queues", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "queues",
		ShortUsage: "asc game-center matchmaking queues <subcommand> [flags]",
		ShortHelp:  "Manage matchmaking queues.",
		LongHelp: `Manage matchmaking queues.

Examples:
  asc game-center matchmaking queues list
  asc game-center matchmaking queues get --id "QUEUE_ID"
  asc game-center matchmaking queues create --reference-name "Queue 1" --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking queues update --id "QUEUE_ID" --classic-bundle-ids "com.example.app"
  asc game-center matchmaking queues delete --id "QUEUE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterMatchmakingQueuesListCommand(),
			GameCenterMatchmakingQueuesGetCommand(),
			GameCenterMatchmakingQueuesCreateCommand(),
			GameCenterMatchmakingQueuesUpdateCommand(),
			GameCenterMatchmakingQueuesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterMatchmakingQueuesListCommand returns the matchmaking queues list subcommand.
func GameCenterMatchmakingQueuesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center matchmaking queues list [flags]",
		ShortHelp:  "List matchmaking queues.",
		LongHelp: `List matchmaking queues.

Examples:
  asc game-center matchmaking queues list
  asc game-center matchmaking queues list --limit 50
  asc game-center matchmaking queues list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center matchmaking queues list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center matchmaking queues list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCMatchmakingQueuesOption{
				asc.WithGCMatchmakingQueuesLimit(*limit),
				asc.WithGCMatchmakingQueuesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCMatchmakingQueuesLimit(200))
				firstPage, err := client.GetGameCenterMatchmakingQueues(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center matchmaking queues list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterMatchmakingQueues(ctx, asc.WithGCMatchmakingQueuesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center matchmaking queues list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterMatchmakingQueues(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingQueuesGetCommand returns the matchmaking queues get subcommand.
func GameCenterMatchmakingQueuesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	queueID := fs.String("id", "", "Matchmaking queue ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center matchmaking queues get --id \"QUEUE_ID\"",
		ShortHelp:  "Get a matchmaking queue by ID.",
		LongHelp: `Get a matchmaking queue by ID.

Examples:
  asc game-center matchmaking queues get --id "QUEUE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*queueID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterMatchmakingQueue(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingQueuesCreateCommand returns the matchmaking queues create subcommand.
func GameCenterMatchmakingQueuesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	referenceName := fs.String("reference-name", "", "Reference name for the queue")
	ruleSetID := fs.String("rule-set-id", "", "Matchmaking rule set ID")
	experimentRuleSetID := fs.String("experiment-rule-set-id", "", "Experiment rule set ID")
	classicBundleIDs := fs.String("classic-bundle-ids", "", "Comma-separated bundle IDs for classic matchmaking")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center matchmaking queues create --reference-name \"Queue\" --rule-set-id \"RULE_SET_ID\"",
		ShortHelp:  "Create a matchmaking queue.",
		LongHelp: `Create a matchmaking queue.

Examples:
  asc game-center matchmaking queues create --reference-name "Queue" --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking queues create --reference-name "Queue" --rule-set-id "RULE_SET_ID" --classic-bundle-ids "com.example.app"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			name := strings.TrimSpace(*referenceName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --reference-name is required")
				return flag.ErrHelp
			}
			ruleSet := strings.TrimSpace(*ruleSetID)
			if ruleSet == "" {
				fmt.Fprintln(os.Stderr, "Error: --rule-set-id is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterMatchmakingQueueCreateAttributes{
				ReferenceName:               name,
				ClassicMatchmakingBundleIDs: shared.SplitCSV(*classicBundleIDs),
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterMatchmakingQueue(requestCtx, attrs, ruleSet, strings.TrimSpace(*experimentRuleSetID))
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingQueuesUpdateCommand returns the matchmaking queues update subcommand.
func GameCenterMatchmakingQueuesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	queueID := fs.String("id", "", "Matchmaking queue ID")
	ruleSetID := fs.String("rule-set-id", "", "Matchmaking rule set ID")
	experimentRuleSetID := fs.String("experiment-rule-set-id", "", "Experiment rule set ID")
	classicBundleIDs := fs.String("classic-bundle-ids", "", "Comma-separated bundle IDs for classic matchmaking")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center matchmaking queues update --id \"QUEUE_ID\" [flags]",
		ShortHelp:  "Update a matchmaking queue.",
		LongHelp: `Update a matchmaking queue.

Examples:
  asc game-center matchmaking queues update --id "QUEUE_ID" --classic-bundle-ids "com.example.app"
  asc game-center matchmaking queues update --id "QUEUE_ID" --rule-set-id "RULE_SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*queueID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			hasUpdate := false
			attrs := asc.GameCenterMatchmakingQueueUpdateAttributes{}
			if strings.TrimSpace(*classicBundleIDs) != "" {
				attrs.ClassicMatchmakingBundleIDs = shared.SplitCSV(*classicBundleIDs)
				hasUpdate = true
			}

			if !hasUpdate && strings.TrimSpace(*ruleSetID) == "" && strings.TrimSpace(*experimentRuleSetID) == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterMatchmakingQueue(requestCtx, id, attrs, strings.TrimSpace(*ruleSetID), strings.TrimSpace(*experimentRuleSetID))
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingQueuesDeleteCommand returns the matchmaking queues delete subcommand.
func GameCenterMatchmakingQueuesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	queueID := fs.String("id", "", "Matchmaking queue ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center matchmaking queues delete --id \"QUEUE_ID\" --confirm",
		ShortHelp:  "Delete a matchmaking queue.",
		LongHelp: `Delete a matchmaking queue.

Examples:
  asc game-center matchmaking queues delete --id "QUEUE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*queueID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking queues delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterMatchmakingQueue(requestCtx, id); err != nil {
				return fmt.Errorf("game-center matchmaking queues delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterMatchmakingQueueDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingRuleSetsCommand returns the matchmaking rule sets command group.
func GameCenterMatchmakingRuleSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rule-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "rule-sets",
		ShortUsage: "asc game-center matchmaking rule-sets <subcommand> [flags]",
		ShortHelp:  "Manage matchmaking rule sets.",
		LongHelp: `Manage matchmaking rule sets.

Examples:
  asc game-center matchmaking rule-sets list
  asc game-center matchmaking rule-sets get --id "RULE_SET_ID"
  asc game-center matchmaking rule-sets create --reference-name "Rules" --rule-language-version 1 --min-players 2 --max-players 8
  asc game-center matchmaking rule-sets update --id "RULE_SET_ID" --min-players 2
  asc game-center matchmaking rule-sets delete --id "RULE_SET_ID" --confirm
  asc game-center matchmaking rule-sets queues list --rule-set-id "RULE_SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterMatchmakingRuleSetsListCommand(),
			GameCenterMatchmakingRuleSetsGetCommand(),
			GameCenterMatchmakingRuleSetsCreateCommand(),
			GameCenterMatchmakingRuleSetsUpdateCommand(),
			GameCenterMatchmakingRuleSetsDeleteCommand(),
			GameCenterMatchmakingRuleSetQueuesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterMatchmakingRuleSetsListCommand returns the rule sets list subcommand.
func GameCenterMatchmakingRuleSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center matchmaking rule-sets list [flags]",
		ShortHelp:  "List matchmaking rule sets.",
		LongHelp: `List matchmaking rule sets.

Examples:
  asc game-center matchmaking rule-sets list
  asc game-center matchmaking rule-sets list --limit 50
  asc game-center matchmaking rule-sets list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center matchmaking rule-sets list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCMatchmakingRuleSetsOption{
				asc.WithGCMatchmakingRuleSetsLimit(*limit),
				asc.WithGCMatchmakingRuleSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCMatchmakingRuleSetsLimit(200))
				firstPage, err := client.GetGameCenterMatchmakingRuleSets(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center matchmaking rule-sets list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterMatchmakingRuleSets(ctx, asc.WithGCMatchmakingRuleSetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center matchmaking rule-sets list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterMatchmakingRuleSets(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingRuleSetsGetCommand returns the rule sets get subcommand.
func GameCenterMatchmakingRuleSetsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	ruleSetID := fs.String("id", "", "Matchmaking rule set ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center matchmaking rule-sets get --id \"RULE_SET_ID\"",
		ShortHelp:  "Get a matchmaking rule set by ID.",
		LongHelp: `Get a matchmaking rule set by ID.

Examples:
  asc game-center matchmaking rule-sets get --id "RULE_SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*ruleSetID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterMatchmakingRuleSet(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingRuleSetsCreateCommand returns the rule sets create subcommand.
func GameCenterMatchmakingRuleSetsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	referenceName := fs.String("reference-name", "", "Reference name for the rule set")
	ruleLanguageVersion := fs.Int("rule-language-version", 0, "Rule language version")
	minPlayers := fs.Int("min-players", 0, "Minimum players")
	maxPlayers := fs.Int("max-players", 0, "Maximum players")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center matchmaking rule-sets create --reference-name \"Rules\" --rule-language-version 1 --min-players 2 --max-players 8",
		ShortHelp:  "Create a matchmaking rule set.",
		LongHelp: `Create a matchmaking rule set.

Examples:
  asc game-center matchmaking rule-sets create --reference-name "Rules" --rule-language-version 1 --min-players 2 --max-players 8`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			name := strings.TrimSpace(*referenceName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --reference-name is required")
				return flag.ErrHelp
			}
			if *ruleLanguageVersion == 0 {
				fmt.Fprintln(os.Stderr, "Error: --rule-language-version is required")
				return flag.ErrHelp
			}
			if *minPlayers == 0 || *maxPlayers == 0 {
				fmt.Fprintln(os.Stderr, "Error: --min-players and --max-players are required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterMatchmakingRuleSetCreateAttributes{
				ReferenceName:       name,
				RuleLanguageVersion: *ruleLanguageVersion,
				MinPlayers:          *minPlayers,
				MaxPlayers:          *maxPlayers,
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterMatchmakingRuleSet(requestCtx, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingRuleSetsUpdateCommand returns the rule sets update subcommand.
func GameCenterMatchmakingRuleSetsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	ruleSetID := fs.String("id", "", "Matchmaking rule set ID")
	minPlayers := fs.Int("min-players", 0, "Minimum players")
	maxPlayers := fs.Int("max-players", 0, "Maximum players")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center matchmaking rule-sets update --id \"RULE_SET_ID\" [flags]",
		ShortHelp:  "Update a matchmaking rule set.",
		LongHelp: `Update a matchmaking rule set.

Examples:
  asc game-center matchmaking rule-sets update --id "RULE_SET_ID" --min-players 2
  asc game-center matchmaking rule-sets update --id "RULE_SET_ID" --max-players 8`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*ruleSetID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
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
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterMatchmakingRuleSet(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingRuleSetsDeleteCommand returns the rule sets delete subcommand.
func GameCenterMatchmakingRuleSetsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	ruleSetID := fs.String("id", "", "Matchmaking rule set ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center matchmaking rule-sets delete --id \"RULE_SET_ID\" --confirm",
		ShortHelp:  "Delete a matchmaking rule set.",
		LongHelp: `Delete a matchmaking rule set.

Examples:
  asc game-center matchmaking rule-sets delete --id "RULE_SET_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*ruleSetID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterMatchmakingRuleSet(requestCtx, id); err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterMatchmakingRuleSetDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingRuleSetQueuesCommand returns the rule set queues command group.
func GameCenterMatchmakingRuleSetQueuesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("queues", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "queues",
		ShortUsage: "asc game-center matchmaking rule-sets queues list --rule-set-id \"RULE_SET_ID\"",
		ShortHelp:  "List matchmaking queues for a rule set.",
		LongHelp: `List matchmaking queues for a rule set.

Examples:
  asc game-center matchmaking rule-sets queues list --rule-set-id "RULE_SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterMatchmakingRuleSetQueuesListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterMatchmakingRuleSetQueuesListCommand returns the rule set queues list subcommand.
func GameCenterMatchmakingRuleSetQueuesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	ruleSetID := fs.String("rule-set-id", "", "Matchmaking rule set ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center matchmaking rule-sets queues list --rule-set-id \"RULE_SET_ID\"",
		ShortHelp:  "List queues for a matchmaking rule set.",
		LongHelp: `List queues for a matchmaking rule set.

Examples:
  asc game-center matchmaking rule-sets queues list --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking rule-sets queues list --rule-set-id "RULE_SET_ID" --limit 50
  asc game-center matchmaking rule-sets queues list --rule-set-id "RULE_SET_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center matchmaking rule-sets queues list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets queues list: %w", err)
			}

			id := strings.TrimSpace(*ruleSetID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --rule-set-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets queues list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCMatchmakingQueuesOption{
				asc.WithGCMatchmakingQueuesLimit(*limit),
				asc.WithGCMatchmakingQueuesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCMatchmakingQueuesLimit(200))
				firstPage, err := client.GetGameCenterMatchmakingRuleSetQueues(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center matchmaking rule-sets queues list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterMatchmakingRuleSetQueues(ctx, id, asc.WithGCMatchmakingQueuesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center matchmaking rule-sets queues list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterMatchmakingRuleSetQueues(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-sets queues list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingRulesCommand returns the matchmaking rules command group.
func GameCenterMatchmakingRulesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rules", flag.ExitOnError)

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
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterMatchmakingRulesListCommand(),
			GameCenterMatchmakingRulesCreateCommand(),
			GameCenterMatchmakingRulesUpdateCommand(),
			GameCenterMatchmakingRulesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterMatchmakingRulesListCommand returns the rules list subcommand.
func GameCenterMatchmakingRulesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	ruleSetID := fs.String("rule-set-id", "", "Matchmaking rule set ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center matchmaking rules list --rule-set-id \"RULE_SET_ID\"",
		ShortHelp:  "List matchmaking rules for a rule set.",
		LongHelp: `List matchmaking rules for a rule set.

Examples:
  asc game-center matchmaking rules list --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking rules list --rule-set-id "RULE_SET_ID" --limit 50
  asc game-center matchmaking rules list --rule-set-id "RULE_SET_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center matchmaking rules list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center matchmaking rules list: %w", err)
			}

			id := strings.TrimSpace(*ruleSetID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --rule-set-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rules list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCMatchmakingRulesOption{
				asc.WithGCMatchmakingRulesLimit(*limit),
				asc.WithGCMatchmakingRulesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCMatchmakingRulesLimit(200))
				firstPage, err := client.GetGameCenterMatchmakingRules(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center matchmaking rules list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterMatchmakingRules(ctx, id, asc.WithGCMatchmakingRulesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center matchmaking rules list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterMatchmakingRules(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rules list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingRulesCreateCommand returns the rules create subcommand.
func GameCenterMatchmakingRulesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	ruleSetID := fs.String("rule-set-id", "", "Matchmaking rule set ID")
	referenceName := fs.String("reference-name", "", "Reference name for the rule")
	description := fs.String("description", "", "Rule description")
	ruleType := fs.String("type", "", "Rule type (COMPATIBLE, DISTANCE, MATCH, TEAM)")
	expression := fs.String("expression", "", "Rule expression")
	weight := fs.String("weight", "", "Rule weight (float)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center matchmaking rules create --rule-set-id \"RULE_SET_ID\" --reference-name \"Rule\" --description \"Match\" --type MATCH --expression \"player.level > 1\"",
		ShortHelp:  "Create a matchmaking rule.",
		LongHelp: `Create a matchmaking rule.

Examples:
  asc game-center matchmaking rules create --rule-set-id "RULE_SET_ID" --reference-name "Rule" --description "Match" --type MATCH --expression "player.level > 1"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ruleSet := strings.TrimSpace(*ruleSetID)
			if ruleSet == "" {
				fmt.Fprintln(os.Stderr, "Error: --rule-set-id is required")
				return flag.ErrHelp
			}
			name := strings.TrimSpace(*referenceName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --reference-name is required")
				return flag.ErrHelp
			}
			desc := strings.TrimSpace(*description)
			if desc == "" {
				fmt.Fprintln(os.Stderr, "Error: --description is required")
				return flag.ErrHelp
			}
			rtype := strings.TrimSpace(*ruleType)
			if rtype == "" {
				fmt.Fprintln(os.Stderr, "Error: --type is required")
				return flag.ErrHelp
			}
			expr := strings.TrimSpace(*expression)
			if expr == "" {
				fmt.Fprintln(os.Stderr, "Error: --expression is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterMatchmakingRuleCreateAttributes{
				ReferenceName: name,
				Description:   desc,
				Type:          rtype,
				Expression:    expr,
			}

			if strings.TrimSpace(*weight) != "" {
				val, err := strconv.ParseFloat(strings.TrimSpace(*weight), 64)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error: --weight must be a number")
					return flag.ErrHelp
				}
				attrs.Weight = &val
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rules create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterMatchmakingRule(requestCtx, ruleSet, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rules create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingRulesUpdateCommand returns the rules update subcommand.
func GameCenterMatchmakingRulesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	ruleID := fs.String("id", "", "Matchmaking rule ID")
	description := fs.String("description", "", "Rule description")
	expression := fs.String("expression", "", "Rule expression")
	weight := fs.String("weight", "", "Rule weight (float)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center matchmaking rules update --id \"RULE_ID\" [flags]",
		ShortHelp:  "Update a matchmaking rule.",
		LongHelp: `Update a matchmaking rule.

Examples:
  asc game-center matchmaking rules update --id "RULE_ID" --description "New description"
  asc game-center matchmaking rules update --id "RULE_ID" --weight 0.5`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*ruleID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
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
				val, err := strconv.ParseFloat(strings.TrimSpace(*weight), 64)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error: --weight must be a number")
					return flag.ErrHelp
				}
				attrs.Weight = &val
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rules update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterMatchmakingRule(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rules update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingRulesDeleteCommand returns the rules delete subcommand.
func GameCenterMatchmakingRulesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	ruleID := fs.String("id", "", "Matchmaking rule ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center matchmaking rules delete --id \"RULE_ID\" --confirm",
		ShortHelp:  "Delete a matchmaking rule.",
		LongHelp: `Delete a matchmaking rule.

Examples:
  asc game-center matchmaking rules delete --id "RULE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*ruleID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rules delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterMatchmakingRule(requestCtx, id); err != nil {
				return fmt.Errorf("game-center matchmaking rules delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterMatchmakingRuleDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingTeamsCommand returns the matchmaking teams command group.
func GameCenterMatchmakingTeamsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("teams", flag.ExitOnError)

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
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterMatchmakingTeamsListCommand(),
			GameCenterMatchmakingTeamsCreateCommand(),
			GameCenterMatchmakingTeamsUpdateCommand(),
			GameCenterMatchmakingTeamsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterMatchmakingTeamsListCommand returns the teams list subcommand.
func GameCenterMatchmakingTeamsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	ruleSetID := fs.String("rule-set-id", "", "Matchmaking rule set ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center matchmaking teams list --rule-set-id \"RULE_SET_ID\"",
		ShortHelp:  "List matchmaking teams for a rule set.",
		LongHelp: `List matchmaking teams for a rule set.

Examples:
  asc game-center matchmaking teams list --rule-set-id "RULE_SET_ID"
  asc game-center matchmaking teams list --rule-set-id "RULE_SET_ID" --limit 50
  asc game-center matchmaking teams list --rule-set-id "RULE_SET_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center matchmaking teams list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center matchmaking teams list: %w", err)
			}

			id := strings.TrimSpace(*ruleSetID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --rule-set-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking teams list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCMatchmakingTeamsOption{
				asc.WithGCMatchmakingTeamsLimit(*limit),
				asc.WithGCMatchmakingTeamsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCMatchmakingTeamsLimit(200))
				firstPage, err := client.GetGameCenterMatchmakingTeams(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center matchmaking teams list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterMatchmakingTeams(ctx, id, asc.WithGCMatchmakingTeamsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center matchmaking teams list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterMatchmakingTeams(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center matchmaking teams list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingTeamsCreateCommand returns the teams create subcommand.
func GameCenterMatchmakingTeamsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	ruleSetID := fs.String("rule-set-id", "", "Matchmaking rule set ID")
	referenceName := fs.String("reference-name", "", "Reference name for the team")
	minPlayers := fs.Int("min-players", 0, "Minimum players")
	maxPlayers := fs.Int("max-players", 0, "Maximum players")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center matchmaking teams create --rule-set-id \"RULE_SET_ID\" --reference-name \"Team\" --min-players 1 --max-players 4",
		ShortHelp:  "Create a matchmaking team.",
		LongHelp: `Create a matchmaking team.

Examples:
  asc game-center matchmaking teams create --rule-set-id "RULE_SET_ID" --reference-name "Team" --min-players 1 --max-players 4`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ruleSet := strings.TrimSpace(*ruleSetID)
			if ruleSet == "" {
				fmt.Fprintln(os.Stderr, "Error: --rule-set-id is required")
				return flag.ErrHelp
			}
			name := strings.TrimSpace(*referenceName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --reference-name is required")
				return flag.ErrHelp
			}
			if *minPlayers == 0 || *maxPlayers == 0 {
				fmt.Fprintln(os.Stderr, "Error: --min-players and --max-players are required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterMatchmakingTeamCreateAttributes{
				ReferenceName: name,
				MinPlayers:    *minPlayers,
				MaxPlayers:    *maxPlayers,
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking teams create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterMatchmakingTeam(requestCtx, ruleSet, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking teams create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingTeamsUpdateCommand returns the teams update subcommand.
func GameCenterMatchmakingTeamsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	teamID := fs.String("id", "", "Matchmaking team ID")
	minPlayers := fs.Int("min-players", 0, "Minimum players")
	maxPlayers := fs.Int("max-players", 0, "Maximum players")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center matchmaking teams update --id \"TEAM_ID\" [flags]",
		ShortHelp:  "Update a matchmaking team.",
		LongHelp: `Update a matchmaking team.

Examples:
  asc game-center matchmaking teams update --id "TEAM_ID" --min-players 1
  asc game-center matchmaking teams update --id "TEAM_ID" --max-players 4`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*teamID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
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
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking teams update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterMatchmakingTeam(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center matchmaking teams update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingTeamsDeleteCommand returns the teams delete subcommand.
func GameCenterMatchmakingTeamsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	teamID := fs.String("id", "", "Matchmaking team ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center matchmaking teams delete --id \"TEAM_ID\" --confirm",
		ShortHelp:  "Delete a matchmaking team.",
		LongHelp: `Delete a matchmaking team.

Examples:
  asc game-center matchmaking teams delete --id "TEAM_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*teamID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking teams delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterMatchmakingTeam(requestCtx, id); err != nil {
				return fmt.Errorf("game-center matchmaking teams delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterMatchmakingTeamDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterMatchmakingMetricsCommand returns the matchmaking metrics command group.
func GameCenterMatchmakingMetricsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metrics", flag.ExitOnError)

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
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterMatchmakingQueueSizesCommand(),
			GameCenterMatchmakingQueueRequestsCommand(),
			GameCenterMatchmakingQueueSessionsCommand(),
			GameCenterMatchmakingQueueExperimentSizesCommand(),
			GameCenterMatchmakingQueueExperimentRequestsCommand(),
			GameCenterMatchmakingBooleanRuleResultsCommand(),
			GameCenterMatchmakingNumberRuleResultsCommand(),
			GameCenterMatchmakingRuleErrorsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterMatchmakingQueueSizesCommand returns the queue sizes metrics subcommand.
func GameCenterMatchmakingQueueSizesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("queue-sizes", flag.ExitOnError)

	queueID := fs.String("queue-id", "", "Matchmaking queue ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return metricsQueueCommand("queue-sizes", fs, queueID, granularity, sort, limit, next, paginate, output.Output, output.Pretty, func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueSizesResponse, error) {
		return ascClient(ctx).GetGameCenterMatchmakingQueueSizes(ctx, id, opts...)
	})
}

// GameCenterMatchmakingQueueRequestsCommand returns the queue requests metrics subcommand.
func GameCenterMatchmakingQueueRequestsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("queue-requests", flag.ExitOnError)

	queueID := fs.String("queue-id", "", "Matchmaking queue ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result, gameCenterDetail)")
	filterResult := fs.String("filter-result", "", "Filter result (MATCHED, CANCELED, EXPIRED)")
	filterDetail := fs.String("filter-detail", "", "Filter by Game Center detail ID")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return metricsQueueCommandWithFilters("queue-requests", fs, queueID, granularity, groupBy, filterResult, filterDetail, sort, limit, next, paginate, output.Output, output.Pretty, func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueRequestsResponse, error) {
		return ascClient(ctx).GetGameCenterMatchmakingQueueRequests(ctx, id, opts...)
	})
}

// GameCenterMatchmakingQueueSessionsCommand returns the queue sessions metrics subcommand.
func GameCenterMatchmakingQueueSessionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("queue-sessions", flag.ExitOnError)

	queueID := fs.String("queue-id", "", "Matchmaking queue ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return metricsQueueCommand("queue-sessions", fs, queueID, granularity, sort, limit, next, paginate, output.Output, output.Pretty, func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueSessionsResponse, error) {
		return ascClient(ctx).GetGameCenterMatchmakingQueueSessions(ctx, id, opts...)
	})
}

// GameCenterMatchmakingQueueExperimentSizesCommand returns the experiment queue sizes metrics subcommand.
func GameCenterMatchmakingQueueExperimentSizesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiment-queue-sizes", flag.ExitOnError)

	queueID := fs.String("queue-id", "", "Matchmaking queue ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return metricsQueueCommand("experiment-queue-sizes", fs, queueID, granularity, sort, limit, next, paginate, output.Output, output.Pretty, func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueExperimentSizesResponse, error) {
		return ascClient(ctx).GetGameCenterMatchmakingQueueExperimentSizes(ctx, id, opts...)
	})
}

// GameCenterMatchmakingQueueExperimentRequestsCommand returns the experiment queue requests metrics subcommand.
func GameCenterMatchmakingQueueExperimentRequestsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiment-queue-requests", flag.ExitOnError)

	queueID := fs.String("queue-id", "", "Matchmaking queue ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result, gameCenterDetail)")
	filterResult := fs.String("filter-result", "", "Filter result (MATCHED, CANCELED, EXPIRED)")
	filterDetail := fs.String("filter-detail", "", "Filter by Game Center detail ID")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return metricsQueueCommandWithFilters("experiment-queue-requests", fs, queueID, granularity, groupBy, filterResult, filterDetail, sort, limit, next, paginate, output.Output, output.Pretty, func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueExperimentRequestsResponse, error) {
		return ascClient(ctx).GetGameCenterMatchmakingQueueExperimentRequests(ctx, id, opts...)
	})
}

// GameCenterMatchmakingBooleanRuleResultsCommand returns the boolean rule results metrics subcommand.
func GameCenterMatchmakingBooleanRuleResultsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rule-boolean-results", flag.ExitOnError)

	ruleID := fs.String("rule-id", "", "Matchmaking rule ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result, gameCenterMatchmakingQueue)")
	filterResult := fs.String("filter-result", "", "Filter result")
	filterQueue := fs.String("filter-queue", "", "Filter by matchmaking queue ID")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return metricsRuleCommand("rule-boolean-results", fs, ruleID, granularity, groupBy, filterResult, filterQueue, sort, limit, next, paginate, output.Output, output.Pretty, func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingBooleanRuleResultsResponse, error) {
		return ascClient(ctx).GetGameCenterMatchmakingBooleanRuleResults(ctx, id, opts...)
	})
}

// GameCenterMatchmakingNumberRuleResultsCommand returns the number rule results metrics subcommand.
func GameCenterMatchmakingNumberRuleResultsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rule-number-results", flag.ExitOnError)

	ruleID := fs.String("rule-id", "", "Matchmaking rule ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result, gameCenterMatchmakingQueue)")
	filterResult := fs.String("filter-result", "", "Filter result")
	filterQueue := fs.String("filter-queue", "", "Filter by matchmaking queue ID")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return metricsRuleCommand("rule-number-results", fs, ruleID, granularity, groupBy, filterResult, filterQueue, sort, limit, next, paginate, output.Output, output.Pretty, func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingNumberRuleResultsResponse, error) {
		return ascClient(ctx).GetGameCenterMatchmakingNumberRuleResults(ctx, id, opts...)
	})
}

// GameCenterMatchmakingRuleErrorsCommand returns the rule errors metrics subcommand.
func GameCenterMatchmakingRuleErrorsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rule-errors", flag.ExitOnError)

	ruleID := fs.String("rule-id", "", "Matchmaking rule ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result, gameCenterMatchmakingQueue)")
	filterResult := fs.String("filter-result", "", "Filter result")
	filterQueue := fs.String("filter-queue", "", "Filter by matchmaking queue ID")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return metricsRuleCommand("rule-errors", fs, ruleID, granularity, groupBy, filterResult, filterQueue, sort, limit, next, paginate, output.Output, output.Pretty, func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingRuleErrorsResponse, error) {
		return ascClient(ctx).GetGameCenterMatchmakingRuleErrors(ctx, id, opts...)
	})
}

func metricsQueueCommand(name string, fs *flag.FlagSet, queueID *string, granularity *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueSizesResponse, error)) *ffcli.Command {
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc game-center matchmaking metrics " + name + " --queue-id \"QUEUE_ID\" --granularity P1D",
		ShortHelp:  "Fetch matchmaking queue metrics.",
		LongHelp: `Fetch matchmaking queue metrics.

Examples:
  asc game-center matchmaking metrics ` + name + ` --queue-id "QUEUE_ID" --granularity P1D`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runMetricsQueue(ctx, name, queueID, granularity, sort, limit, next, paginate, output, pretty, fetch, nil, "", "", "")
		},
	}
}

func metricsQueueCommandWithFilters(name string, fs *flag.FlagSet, queueID *string, granularity *string, groupBy *string, filterResult *string, filterDetail *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueRequestsResponse, error)) *ffcli.Command {
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc game-center matchmaking metrics " + name + " --queue-id \"QUEUE_ID\" --granularity P1D",
		ShortHelp:  "Fetch matchmaking queue request metrics.",
		LongHelp: `Fetch matchmaking queue request metrics.

Examples:
  asc game-center matchmaking metrics ` + name + ` --queue-id "QUEUE_ID" --granularity P1D --group-by result`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runMetricsQueue(ctx, name, queueID, granularity, sort, limit, next, paginate, output, pretty, nil, fetch, *groupBy, *filterResult, *filterDetail)
		},
	}
}

func metricsRuleCommand(name string, fs *flag.FlagSet, ruleID *string, granularity *string, groupBy *string, filterResult *string, filterQueue *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingBooleanRuleResultsResponse, error)) *ffcli.Command {
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc game-center matchmaking metrics " + name + " --rule-id \"RULE_ID\" --granularity P1D",
		ShortHelp:  "Fetch matchmaking rule metrics.",
		LongHelp: `Fetch matchmaking rule metrics.

Examples:
  asc game-center matchmaking metrics ` + name + ` --rule-id "RULE_ID" --granularity P1D --group-by result`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runMetricsRule(ctx, name, ruleID, granularity, groupBy, filterResult, filterQueue, sort, limit, next, paginate, output, pretty, fetch)
		},
	}
}

func runMetricsQueue(ctx context.Context, name string, queueID *string, granularity *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetchSizes func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueSizesResponse, error), fetchRequests func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingQueueRequestsResponse, error), groupBy string, filterResult string, filterDetail string) error {
	if *limit != 0 && (*limit < 1 || *limit > 200) {
		return fmt.Errorf("game-center matchmaking metrics %s: --limit must be between 1 and 200", name)
	}
	if err := shared.ValidateNextURL(*next); err != nil {
		return fmt.Errorf("game-center matchmaking metrics %s: %w", name, err)
	}

	id := strings.TrimSpace(*queueID)
	if id == "" && strings.TrimSpace(*next) == "" {
		fmt.Fprintln(os.Stderr, "Error: --queue-id is required")
		return flag.ErrHelp
	}
	gran := strings.TrimSpace(*granularity)
	if gran == "" && strings.TrimSpace(*next) == "" {
		fmt.Fprintln(os.Stderr, "Error: --granularity is required")
		return flag.ErrHelp
	}

	var err error
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	opts := []asc.GCMatchmakingMetricsOption{
		asc.WithGCMatchmakingMetricsGranularity(gran),
		asc.WithGCMatchmakingMetricsSort(shared.SplitCSV(*sort)),
		asc.WithGCMatchmakingMetricsLimit(*limit),
		asc.WithGCMatchmakingMetricsNextURL(*next),
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
		paginateOpts := append(opts, asc.WithGCMatchmakingMetricsLimit(200))
		var firstPage asc.PaginatedResponse
		var err error
		if fetchRequests != nil {
			firstPage, err = fetchRequests(requestCtx, id, paginateOpts...)
		} else {
			firstPage, err = fetchSizes(requestCtx, id, paginateOpts...)
		}
		if err != nil {
			return fmt.Errorf("game-center matchmaking metrics %s: failed to fetch: %w", name, err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			if fetchRequests != nil {
				return fetchRequests(ctx, id, asc.WithGCMatchmakingMetricsNextURL(nextURL))
			}
			return fetchSizes(ctx, id, asc.WithGCMatchmakingMetricsNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("game-center matchmaking metrics %s: %w", name, err)
		}

		return shared.PrintOutput(resp, *output, *pretty)
	}

	var resp any
	if fetchRequests != nil {
		resp, err = fetchRequests(requestCtx, id, opts...)
	} else {
		resp, err = fetchSizes(requestCtx, id, opts...)
	}
	if err != nil {
		return fmt.Errorf("game-center matchmaking metrics %s: failed to fetch: %w", name, err)
	}

	return shared.PrintOutput(resp, *output, *pretty)
}

func runMetricsRule(ctx context.Context, name string, ruleID *string, granularity *string, groupBy *string, filterResult *string, filterQueue *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMatchmakingBooleanRuleResultsResponse, error)) error {
	if *limit != 0 && (*limit < 1 || *limit > 200) {
		return fmt.Errorf("game-center matchmaking metrics %s: --limit must be between 1 and 200", name)
	}
	if err := shared.ValidateNextURL(*next); err != nil {
		return fmt.Errorf("game-center matchmaking metrics %s: %w", name, err)
	}

	id := strings.TrimSpace(*ruleID)
	if id == "" && strings.TrimSpace(*next) == "" {
		fmt.Fprintln(os.Stderr, "Error: --rule-id is required")
		return flag.ErrHelp
	}
	gran := strings.TrimSpace(*granularity)
	if gran == "" && strings.TrimSpace(*next) == "" {
		fmt.Fprintln(os.Stderr, "Error: --granularity is required")
		return flag.ErrHelp
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	opts := []asc.GCMatchmakingMetricsOption{
		asc.WithGCMatchmakingMetricsGranularity(gran),
		asc.WithGCMatchmakingMetricsGroupBy(shared.SplitCSV(*groupBy)),
		asc.WithGCMatchmakingMetricsFilterResult(strings.TrimSpace(*filterResult)),
		asc.WithGCMatchmakingMetricsFilterQueue(strings.TrimSpace(*filterQueue)),
		asc.WithGCMatchmakingMetricsSort(shared.SplitCSV(*sort)),
		asc.WithGCMatchmakingMetricsLimit(*limit),
		asc.WithGCMatchmakingMetricsNextURL(*next),
	}

	if *paginate {
		paginateOpts := append(opts, asc.WithGCMatchmakingMetricsLimit(200))
		firstPage, err := fetch(requestCtx, id, paginateOpts...)
		if err != nil {
			return fmt.Errorf("game-center matchmaking metrics %s: failed to fetch: %w", name, err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return fetch(ctx, id, asc.WithGCMatchmakingMetricsNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("game-center matchmaking metrics %s: %w", name, err)
		}

		return shared.PrintOutput(resp, *output, *pretty)
	}

	resp, err := fetch(requestCtx, id, opts...)
	if err != nil {
		return fmt.Errorf("game-center matchmaking metrics %s: failed to fetch: %w", name, err)
	}

	return shared.PrintOutput(resp, *output, *pretty)
}

// GameCenterMatchmakingRuleSetTestsCommand returns the rule set tests command group.
func GameCenterMatchmakingRuleSetTestsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rule-set-tests", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "rule-set-tests",
		ShortUsage: "asc game-center matchmaking rule-set-tests create --file payload.json",
		ShortHelp:  "Run matchmaking rule set tests.",
		LongHelp: `Run matchmaking rule set tests.

Examples:
  asc game-center matchmaking rule-set-tests create --file payload.json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterMatchmakingRuleSetTestsCreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterMatchmakingRuleSetTestsCreateCommand returns the rule set tests create subcommand.
func GameCenterMatchmakingRuleSetTestsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	filePath := fs.String("file", "", "Path to rule set test JSON payload")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center matchmaking rule-set-tests create --file payload.json",
		ShortHelp:  "Create a matchmaking rule set test.",
		LongHelp: `Create a matchmaking rule set test from a JSON payload.

Examples:
  asc game-center matchmaking rule-set-tests create --file payload.json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			path := strings.TrimSpace(*filePath)
			if path == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			payload, err := readJSONFilePayload(path)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-set-tests create: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-set-tests create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterMatchmakingRuleSetTest(requestCtx, payload)
			if err != nil {
				return fmt.Errorf("game-center matchmaking rule-set-tests create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func ascClient(ctx context.Context) *asc.Client {
	client, _ := shared.GetASCClient()
	return client
}

func readJSONFilePayload(path string) (json.RawMessage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("payload path must be a file")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, fmt.Errorf("payload file is empty")
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return json.RawMessage(data), nil
}
