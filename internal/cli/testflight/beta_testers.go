package testflight

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// BetaTestersCommand returns the beta testers command with subcommands.
func BetaTestersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-testers", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-testers",
		ShortUsage: "asc testflight beta-testers <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight beta testers.",
		LongHelp: `Manage TestFlight beta testers.

Examples:
  asc testflight beta-testers list --app "APP_ID"
  asc testflight beta-testers get --id "TESTER_ID"
  asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta"
  asc testflight beta-testers remove --app "APP_ID" --email "tester@example.com"
  asc testflight beta-testers add-groups --id "TESTER_ID" --group "GROUP_ID"
  asc testflight beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID"
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com"
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com" --group "Beta"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaTestersListCommand(),
			BetaTestersGetCommand(),
			BetaTestersAddCommand(),
			BetaTestersRemoveCommand(),
			BetaTestersAddGroupsCommand(),
			BetaTestersRemoveGroupsCommand(),
			BetaTestersRelationshipsCommand(),
			BetaTestersMetricsCommand(),
			BetaTestersInviteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaTestersListCommand returns the beta testers list subcommand.
func BetaTestersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	buildID := fs.String("build", "", "Build ID to filter")
	group := fs.String("group", "", "Beta group name or ID to filter")
	email := fs.String("email", "", "Filter by tester email")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-testers list [flags]",
		ShortHelp:  "List TestFlight beta testers for an app.",
		LongHelp: `List TestFlight beta testers for an app.

Examples:
  asc testflight beta-testers list --app "APP_ID"
  asc testflight beta-testers list --app "APP_ID" --build "BUILD_ID"
  asc testflight beta-testers list --app "APP_ID" --group "Beta"
  asc testflight beta-testers list --app "APP_ID" --limit 25
  asc testflight beta-testers list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("beta-testers list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("beta-testers list: %w", err)
			}

			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaTestersOption{
				asc.WithBetaTestersLimit(*limit),
				asc.WithBetaTestersNextURL(*next),
			}

			if strings.TrimSpace(*buildID) != "" {
				opts = append(opts, asc.WithBetaTestersBuildID(strings.TrimSpace(*buildID)))
			}

			if strings.TrimSpace(*email) != "" {
				opts = append(opts, asc.WithBetaTestersEmail(*email))
			}

			if strings.TrimSpace(*group) != "" && strings.TrimSpace(*next) == "" {
				groupID, err := resolveBetaGroupID(requestCtx, client, resolvedAppID, *group)
				if err != nil {
					return fmt.Errorf("beta-testers list: %w", err)
				}
				opts = append(opts, asc.WithBetaTestersGroupIDs([]string{groupID}))
			}

			if *paginate {
				// Fetch first page with limit set for consistent pagination
				paginateOpts := append(opts, asc.WithBetaTestersLimit(200))
				firstPage, err := client.GetBetaTesters(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("beta-testers list: failed to fetch: %w", err)
				}

				// Fetch all remaining pages
				testers, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBetaTesters(ctx, resolvedAppID, asc.WithBetaTestersNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("beta-testers list: %w", err)
				}

				return printOutput(testers, *output, *pretty)
			}

			testers, err := client.GetBetaTesters(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("beta-testers list: failed to fetch: %w", err)
			}

			return printOutput(testers, *output, *pretty)
		},
	}
}

// BetaTestersGetCommand returns the beta testers get subcommand.
func BetaTestersGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Beta tester ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-testers get [flags]",
		ShortHelp:  "Get a TestFlight beta tester by ID.",
		LongHelp: `Get a TestFlight beta tester by ID.

Examples:
  asc testflight beta-testers get --id "TESTER_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			tester, err := client.GetBetaTester(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("beta-testers get: failed to fetch: %w", err)
			}

			return printOutput(tester, *output, *pretty)
		},
	}
}

// BetaTestersAddCommand returns the beta testers add subcommand.
func BetaTestersAddCommand() *ffcli.Command {
	fs := flag.NewFlagSet("add", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	email := fs.String("email", "", "Tester email address")
	firstName := fs.String("first-name", "", "Tester first name")
	lastName := fs.String("last-name", "", "Tester last name")
	group := fs.String("group", "", "Beta group name or ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "add",
		ShortUsage: "asc testflight beta-testers add [flags]",
		ShortHelp:  "Add a TestFlight beta tester.",
		LongHelp: `Add a TestFlight beta tester.

Examples:
  asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*email) == "" {
				fmt.Fprintln(os.Stderr, "Error: --email is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*group) == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers add: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			groupID, err := resolveBetaGroupID(requestCtx, client, resolvedAppID, *group)
			if err != nil {
				return fmt.Errorf("beta-testers add: %w", err)
			}

			tester, err := client.CreateBetaTester(requestCtx, *email, *firstName, *lastName, []string{groupID})
			if err != nil {
				return fmt.Errorf("beta-testers add: failed to create: %w", err)
			}

			return printOutput(tester, *output, *pretty)
		},
	}
}

// BetaTestersRemoveCommand returns the beta testers remove subcommand.
func BetaTestersRemoveCommand() *ffcli.Command {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	email := fs.String("email", "", "Tester email address")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "remove",
		ShortUsage: "asc testflight beta-testers remove [flags]",
		ShortHelp:  "Remove a TestFlight beta tester.",
		LongHelp: `Remove a TestFlight beta tester.

Examples:
  asc testflight beta-testers remove --app "APP_ID" --email "tester@example.com"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*email) == "" {
				fmt.Fprintln(os.Stderr, "Error: --email is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers remove: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			testerID, err := findBetaTesterIDByEmail(requestCtx, client, resolvedAppID, *email)
			if err != nil {
				if errors.Is(err, errBetaTesterNotFound) {
					return fmt.Errorf("beta-testers remove: no tester found for %q", strings.TrimSpace(*email))
				}
				return fmt.Errorf("beta-testers remove: %w", err)
			}

			if err := client.DeleteBetaTester(requestCtx, testerID); err != nil {
				return fmt.Errorf("beta-testers remove: failed to remove: %w", err)
			}

			result := &asc.BetaTesterDeleteResult{
				ID:      testerID,
				Email:   strings.TrimSpace(*email),
				Deleted: true,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}

// BetaTestersAddGroupsCommand returns the beta testers add-groups subcommand.
func BetaTestersAddGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("add-groups", flag.ExitOnError)

	id := fs.String("id", "", "Beta tester ID")
	groups := fs.String("group", "", "Comma-separated beta group IDs")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "add-groups",
		ShortUsage: "asc testflight beta-testers add-groups --id TESTER_ID --group GROUP_ID[,GROUP_ID...]",
		ShortHelp:  "Add a beta tester to beta groups.",
		LongHelp: `Add a beta tester to beta groups.

Examples:
  asc testflight beta-testers add-groups --id "TESTER_ID" --group "GROUP_ID"
  asc testflight beta-testers add-groups --id "TESTER_ID" --group "GROUP_ID_1,GROUP_ID_2"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			groupIDs := parseCommaSeparatedIDs(*groups)
			if len(groupIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers add-groups: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.AddBetaTesterToGroups(requestCtx, testerID, groupIDs); err != nil {
				return fmt.Errorf("beta-testers add-groups: failed to add groups: %w", err)
			}

			result := &asc.BetaTesterGroupsUpdateResult{
				TesterID: testerID,
				GroupIDs: groupIDs,
				Action:   "added",
			}

			if err := printOutput(result, *output, *pretty); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Successfully added tester %s to %d group(s)\n", testerID, len(groupIDs))
			return nil
		},
	}
}

// BetaTestersRemoveGroupsCommand returns the beta testers remove-groups subcommand.
func BetaTestersRemoveGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("remove-groups", flag.ExitOnError)

	id := fs.String("id", "", "Beta tester ID")
	groups := fs.String("group", "", "Comma-separated beta group IDs")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "remove-groups",
		ShortUsage: "asc testflight beta-testers remove-groups --id TESTER_ID --group GROUP_ID[,GROUP_ID...]",
		ShortHelp:  "Remove a beta tester from beta groups.",
		LongHelp: `Remove a beta tester from beta groups.

Examples:
  asc testflight beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID"
  asc testflight beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID_1,GROUP_ID_2"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			groupIDs := parseCommaSeparatedIDs(*groups)
			if len(groupIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers remove-groups: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.RemoveBetaTesterFromGroups(requestCtx, testerID, groupIDs); err != nil {
				return fmt.Errorf("beta-testers remove-groups: failed to remove groups: %w", err)
			}

			result := &asc.BetaTesterGroupsUpdateResult{
				TesterID: testerID,
				GroupIDs: groupIDs,
				Action:   "removed",
			}

			if err := printOutput(result, *output, *pretty); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Successfully removed tester %s from %d group(s)\n", testerID, len(groupIDs))
			return nil
		},
	}
}

// BetaTestersInviteCommand returns the beta testers invite subcommand.
func BetaTestersInviteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("invite", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	email := fs.String("email", "", "Tester email address")
	group := fs.String("group", "", "Beta group name or ID (optional, creates tester if missing)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "invite",
		ShortUsage: "asc testflight beta-testers invite [flags]",
		ShortHelp:  "Invite a TestFlight beta tester.",
		LongHelp: `Invite a TestFlight beta tester.

Examples:
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com"
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com" --group "Beta"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*email) == "" {
				fmt.Fprintln(os.Stderr, "Error: --email is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers invite: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			emailValue := strings.TrimSpace(*email)
			groupValue := strings.TrimSpace(*group)
			testerID, err := findBetaTesterIDByEmail(requestCtx, client, resolvedAppID, emailValue)
			if err != nil {
				if errors.Is(err, errBetaTesterNotFound) {
					if groupValue == "" {
						return fmt.Errorf("beta-testers invite: no tester found for %q (use beta-testers add --group ... or pass --group here)", emailValue)
					}

					groupID, resolveErr := resolveBetaGroupID(requestCtx, client, resolvedAppID, groupValue)
					if resolveErr != nil {
						return fmt.Errorf("beta-testers invite: %w", resolveErr)
					}

					created, createErr := client.CreateBetaTester(requestCtx, emailValue, "", "", []string{groupID})
					if createErr != nil {
						return fmt.Errorf("beta-testers invite: failed to create tester: %w", createErr)
					}
					testerID = created.Data.ID
				} else {
					return fmt.Errorf("beta-testers invite: %w", err)
				}
			}

			invitation, err := client.CreateBetaTesterInvitation(requestCtx, resolvedAppID, testerID)
			if err != nil {
				return fmt.Errorf("beta-testers invite: failed to create invitation: %w", err)
			}

			result := &asc.BetaTesterInvitationResult{
				InvitationID: invitation.Data.ID,
				TesterID:     testerID,
				AppID:        resolvedAppID,
				Email:        emailValue,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}
