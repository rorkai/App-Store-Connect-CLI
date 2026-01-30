package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// BetaGroupsCommand returns the beta groups command with subcommands.
func BetaGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-groups", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-groups",
		ShortUsage: "asc testflight beta-groups <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight beta groups.",
		LongHelp: `Manage TestFlight beta groups.

Examples:
  asc testflight beta-groups list --app "APP_ID"
  asc testflight beta-groups create --app "APP_ID" --name "Beta Testers"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaGroupsListCommand(),
			BetaGroupsCreateCommand(),
			BetaGroupsGetCommand(),
			BetaGroupsUpdateCommand(),
			BetaGroupsAddTestersCommand(),
			BetaGroupsRemoveTestersCommand(),
			BetaGroupsRelationshipsCommand(),
			BetaGroupsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaGroupsListCommand returns the beta groups list subcommand.
func BetaGroupsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-groups list [flags]",
		ShortHelp:  "List TestFlight beta groups for an app.",
		LongHelp: `List TestFlight beta groups for an app.

Examples:
  asc testflight beta-groups list --app "APP_ID"
  asc testflight beta-groups list --app "APP_ID" --limit 10
  asc testflight beta-groups list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("beta-groups list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("beta-groups list: %w", err)
			}

			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaGroupsOption{
				asc.WithBetaGroupsLimit(*limit),
				asc.WithBetaGroupsNextURL(*next),
			}

			if *paginate {
				// Fetch first page with limit set for consistent pagination
				paginateOpts := append(opts, asc.WithBetaGroupsLimit(200))
				firstPage, err := client.GetBetaGroups(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("beta-groups list: failed to fetch: %w", err)
				}

				// Fetch all remaining pages
				groups, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBetaGroups(ctx, resolvedAppID, asc.WithBetaGroupsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("beta-groups list: %w", err)
				}

				return printOutput(groups, *output, *pretty)
			}

			groups, err := client.GetBetaGroups(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("beta-groups list: failed to fetch: %w", err)
			}

			return printOutput(groups, *output, *pretty)
		},
	}
}

// BetaGroupsCreateCommand returns the beta groups create subcommand.
func BetaGroupsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	name := fs.String("name", "", "Beta group name")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc testflight beta-groups create [flags]",
		ShortHelp:  "Create a TestFlight beta group.",
		LongHelp: `Create a TestFlight beta group.

Examples:
  asc testflight beta-groups create --app "APP_ID" --name "Beta Testers"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*name) == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			group, err := client.CreateBetaGroup(requestCtx, resolvedAppID, strings.TrimSpace(*name))
			if err != nil {
				return fmt.Errorf("beta-groups create: failed to create: %w", err)
			}

			return printOutput(group, *output, *pretty)
		},
	}
}

// BetaGroupsGetCommand returns the beta groups get subcommand.
func BetaGroupsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Beta group ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-groups get [flags]",
		ShortHelp:  "Get a TestFlight beta group by ID.",
		LongHelp: `Get a TestFlight beta group by ID.

Examples:
  asc testflight beta-groups get --id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*id) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			group, err := client.GetBetaGroup(requestCtx, strings.TrimSpace(*id))
			if err != nil {
				return fmt.Errorf("beta-groups get: failed to fetch: %w", err)
			}

			return printOutput(group, *output, *pretty)
		},
	}
}

// BetaGroupsUpdateCommand returns the beta groups update subcommand.
func BetaGroupsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Beta group ID")
	name := fs.String("name", "", "Beta group name")
	publicLinkEnabled := fs.Bool("public-link-enabled", false, "Enable public link")
	publicLinkLimitEnabled := fs.Bool("public-link-limit-enabled", false, "Enable public link limit")
	publicLinkLimit := fs.Int("public-link-limit", 0, "Public link limit (1-10000)")
	feedbackEnabled := fs.Bool("feedback-enabled", false, "Enable feedback")
	internal := fs.Bool("internal", false, "Set as internal group")
	allBuilds := fs.Bool("all-builds", false, "Grant access to all builds")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc testflight beta-groups update [flags]",
		ShortHelp:  "Update a TestFlight beta group.",
		LongHelp: `Update a TestFlight beta group.

Examples:
  asc testflight beta-groups update --id "GROUP_ID" --name "New Name"
  asc testflight beta-groups update --id "GROUP_ID" --public-link-enabled --public-link-limit 100
  asc testflight beta-groups update --id "GROUP_ID" --feedback-enabled --internal`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*id)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if visited["public-link-limit"] && (*publicLinkLimit < 1 || *publicLinkLimit > 10000) {
				fmt.Fprintln(os.Stderr, "Error: --public-link-limit must be between 1 and 10000")
				return flag.ErrHelp
			}

			hasUpdates := strings.TrimSpace(*name) != "" ||
				visited["public-link-enabled"] ||
				visited["public-link-limit-enabled"] ||
				visited["public-link-limit"] ||
				visited["feedback-enabled"] ||
				visited["internal"] ||
				visited["all-builds"]
			if !hasUpdates {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			if visited["public-link-limit-enabled"] && *publicLinkLimitEnabled && !visited["public-link-limit"] {
				fmt.Fprintln(os.Stderr, "Error: --public-link-limit is required when enabling public link limit")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			var publicLinkEnabledAttr *bool
			var publicLinkLimitEnabledAttr *bool
			var feedbackEnabledAttr *bool
			var internalAttr *bool
			var allBuildsAttr *bool

			if visited["public-link-enabled"] {
				publicLinkEnabledAttr = publicLinkEnabled
			}
			if visited["public-link-limit-enabled"] {
				publicLinkLimitEnabledAttr = publicLinkLimitEnabled
			}
			if visited["feedback-enabled"] {
				feedbackEnabledAttr = feedbackEnabled
			}
			if visited["internal"] {
				internalAttr = internal
			}
			if visited["all-builds"] {
				allBuildsAttr = allBuilds
			}

			req := asc.BetaGroupUpdateRequest{
				Data: asc.BetaGroupUpdateData{
					Type: asc.ResourceTypeBetaGroups,
					ID:   trimmedID,
					Attributes: &asc.BetaGroupUpdateAttributes{
						Name:                   strings.TrimSpace(*name),
						PublicLinkEnabled:      publicLinkEnabledAttr,
						PublicLinkLimitEnabled: publicLinkLimitEnabledAttr,
						PublicLinkLimit:        *publicLinkLimit,
						FeedbackEnabled:        feedbackEnabledAttr,
						IsInternalGroup:        internalAttr,
						HasAccessToAllBuilds:   allBuildsAttr,
					},
				},
			}

			group, err := client.UpdateBetaGroup(requestCtx, trimmedID, req)
			if err != nil {
				return fmt.Errorf("beta-groups update: failed to update: %w", err)
			}

			return printOutput(group, *output, *pretty)
		},
	}
}

// BetaGroupsDeleteCommand returns the beta groups delete subcommand.
func BetaGroupsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Beta group ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc testflight beta-groups delete --id \"GROUP_ID\" --confirm",
		ShortHelp:  "Delete a TestFlight beta group.",
		LongHelp: `Delete a TestFlight beta group.

Examples:
  asc testflight beta-groups delete --id "GROUP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*id) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaGroup(requestCtx, strings.TrimSpace(*id)); err != nil {
				return fmt.Errorf("beta-groups delete: failed to delete: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully deleted beta group %s\n", strings.TrimSpace(*id))
			return nil
		},
	}
}

// BetaGroupsAddTestersCommand returns the beta groups add-testers subcommand.
func BetaGroupsAddTestersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("add-testers", flag.ExitOnError)

	group := fs.String("group", "", "Beta group ID")
	tester := fs.String("tester", "", "Beta tester ID(s), comma-separated")

	return &ffcli.Command{
		Name:       "add-testers",
		ShortUsage: "asc testflight beta-groups add-testers --group \"GROUP_ID\" --tester \"TESTER_ID[,TESTER_ID...]\"",
		ShortHelp:  "Add beta testers to a beta group.",
		LongHelp: `Add beta testers to a beta group.

Examples:
  asc testflight beta-groups add-testers --group "GROUP_ID" --tester "TESTER_ID"
  asc testflight beta-groups add-testers --group "GROUP_ID" --tester "TESTER_ID1,TESTER_ID2"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupID := strings.TrimSpace(*group)
			if groupID == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			testerIDs := parseCommaSeparatedIDs(*tester)
			if len(testerIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --tester is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups add-testers: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.AddBetaTestersToGroup(requestCtx, groupID, testerIDs); err != nil {
				return fmt.Errorf("beta-groups add-testers: failed to add testers: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully added %d tester(s) to group %s\n", len(testerIDs), groupID)
			return nil
		},
	}
}

// BetaGroupsRemoveTestersCommand returns the beta groups remove-testers subcommand.
func BetaGroupsRemoveTestersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("remove-testers", flag.ExitOnError)

	group := fs.String("group", "", "Beta group ID")
	tester := fs.String("tester", "", "Beta tester ID(s), comma-separated")

	return &ffcli.Command{
		Name:       "remove-testers",
		ShortUsage: "asc testflight beta-groups remove-testers --group \"GROUP_ID\" --tester \"TESTER_ID[,TESTER_ID...]\"",
		ShortHelp:  "Remove beta testers from a beta group.",
		LongHelp: `Remove beta testers from a beta group.

Examples:
  asc testflight beta-groups remove-testers --group "GROUP_ID" --tester "TESTER_ID"
  asc testflight beta-groups remove-testers --group "GROUP_ID" --tester "TESTER_ID1,TESTER_ID2"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupID := strings.TrimSpace(*group)
			if groupID == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			testerIDs := parseCommaSeparatedIDs(*tester)
			if len(testerIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --tester is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups remove-testers: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.RemoveBetaTestersFromGroup(requestCtx, groupID, testerIDs); err != nil {
				return fmt.Errorf("beta-groups remove-testers: failed to remove testers: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully removed %d tester(s) from group %s\n", len(testerIDs), groupID)
			return nil
		},
	}
}
