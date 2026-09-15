package testflight

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
  asc testflight beta-testers view --id "TESTER_ID"
  asc testflight beta-testers beta-groups list --id "TESTER_ID"
  asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta"
  asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta,iOS 27"
  asc testflight beta-testers export --app "APP_ID" --output "./testflight-testers.csv"
  asc testflight beta-testers import --app "APP_ID" --input "./testflight-testers.csv" --dry-run
  asc testflight beta-testers remove --app "APP_ID" --email "tester@example.com" --confirm
  asc testflight beta-testers add-groups --id "TESTER_ID" --group "GROUP_ID"
  asc testflight beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID"
  asc testflight beta-testers add-builds --id "TESTER_ID" --build-id "BUILD_ID"
  asc testflight beta-testers remove-builds --id "TESTER_ID" --build-id "BUILD_ID" --confirm
  asc testflight beta-testers remove-apps --id "TESTER_ID" --app "APP_ID" --confirm
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com"
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com" --group "Beta"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaTestersListCommand(),
			BetaTestersGetCommand(),
			BetaTestersAddCommand(),
			BetaTestersExportCommand(),
			BetaTestersImportCommand(),
			BetaTestersRemoveCommand(),
			BetaTestersAddGroupsCommand(),
			BetaTestersRemoveGroupsCommand(),
			BetaTestersAddBuildsCommand(),
			BetaTestersRemoveBuildsCommand(),
			BetaTestersRemoveAppsCommand(),
			BetaTestersRelationshipsCommand(),
			BetaTestersAppsCommand(),
			BetaTestersBetaGroupsCommand(),
			BetaTestersBuildsCommand(),
			BetaTestersMetricsCommand(),
			BetaTestersInviteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// betaTesterSortValues lists the sort expressions GET /v1/betaTesters accepts.
var betaTesterSortValues = []string{
	"firstName", "-firstName",
	"lastName", "-lastName",
	"email", "-email",
	"inviteType", "-inviteType",
	"state", "-state",
}

// betaTesterIncludeValues lists the relationships GET /v1/betaTesters can include.
var betaTesterIncludeValues = []string{"apps", "betaGroups", "builds"}

// betaTesterInviteTypeValues lists the accepted filter[inviteType] values.
var betaTesterInviteTypeValues = []string{"EMAIL", "PUBLIC_LINK"}

const betaTesterIncludedRelationshipsWarning = "Warning: included relationships can be partial; App Store Connect returns at most 50 related resources per included relationship. --paginate pages the tester collection, not included relationships."

// BetaTestersListCommand returns the beta testers list subcommand.
func BetaTestersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID to filter")
	group := fs.String("group", "", "Beta group name or ID to filter")
	email := fs.String("email", "", "Filter by tester email")
	firstName := fs.String("first-name", "", "Filter by tester first name (exact match)")
	lastName := fs.String("last-name", "", "Filter by tester last name (exact match)")
	inviteType := fs.String("invite-type", "", "Filter by invite type(s), comma-separated: "+strings.Join(betaTesterInviteTypeValues, ", "))
	sortBy := fs.String("sort", "", "Sort by: "+strings.Join(betaTesterSortValues, ", "))
	include := fs.String("include", "", "Include related resources, comma-separated: "+strings.Join(betaTesterIncludeValues, ", "))
	output := shared.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-testers list [flags]",
		ShortHelp:  "List TestFlight beta testers for an app.",
		LongHelp: `List TestFlight beta testers for an app.

--include adds the related resources to the response's top-level "included"
array, which only JSON output renders. App Store Connect returns at most 50
related resources per included relationship. --paginate pages the tester
collection, not included relationships. For complete group membership, run
asc testflight testers groups list --id "TESTER_ID" --paginate.
A --next URL retains any include query from its original request; JSON output
is still required to render those included resources.

--invite-type, --sort, and --include cannot be combined with --next: a
links.next URL already carries the query it was produced from, so those values
would never reach the request. Invalid values and these incompatible flag
combinations exit 2 before making a request.

Examples:
  asc testflight beta-testers list --app "APP_ID"
  asc testflight beta-testers list --app "APP_ID" --build-id "BUILD_ID"
  asc testflight beta-testers list --app "APP_ID" --group "Beta"
  asc testflight beta-testers list --app "APP_ID" --first-name "Ada" --last-name "Lovelace"
  asc testflight beta-testers list --app "APP_ID" --invite-type "PUBLIC_LINK"
  asc testflight beta-testers list --app "APP_ID" --sort "-lastName"
  asc testflight beta-testers list --app "APP_ID" --include "betaGroups" --paginate --output json
  asc testflight beta-testers list --app "APP_ID" --limit 25
  asc testflight beta-testers list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.WithDiagnostic(
					shared.UsageErrorCtx(ctx, "beta-testers list: --limit must be between 1 and 200"),
					shared.DiagnosticInvalidInput,
					"--limit",
				)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorfCtx(ctx, "beta-testers list: %v", err)
			}
			if err := shared.ValidateSort(*sortBy, betaTesterSortValues...); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return shared.InvalidValueUsageError("--sort")
			}
			if err := shared.ValidateInclude(*include, betaTesterIncludeValues...); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return shared.InvalidValueUsageError("--include")
			}
			inviteTypes, err := normalizeBetaTesterInviteTypes(*inviteType)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return shared.InvalidValueUsageError("--invite-type")
			}
			// A links.next URL already carries the query it was produced from, so
			// these flags would be accepted and silently dropped.
			if err := rejectBetaTestersNextFlagConflicts(fs, *next, "invite-type", "sort", "include"); err != nil {
				return err
			}
			if strings.TrimSpace(*group) != "" && strings.TrimSpace(*buildID) != "" && strings.TrimSpace(*next) == "" {
				return shared.WithDiagnostic(
					shared.UsageError("--group cannot be combined with --build-id"),
					shared.DiagnosticConflictingInput,
					"",
				)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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

			if strings.TrimSpace(*firstName) != "" {
				opts = append(opts, asc.WithBetaTestersFirstName(*firstName))
			}

			if strings.TrimSpace(*lastName) != "" {
				opts = append(opts, asc.WithBetaTestersLastName(*lastName))
			}

			if len(inviteTypes) > 0 {
				opts = append(opts, asc.WithBetaTestersInviteTypes(inviteTypes))
			}

			if strings.TrimSpace(*sortBy) != "" {
				opts = append(opts, asc.WithBetaTestersSort(*sortBy))
			}

			includeValues := shared.SplitCSV(*include)
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithBetaTestersInclude(includeValues))
			}
			// Only the JSON renderer emits the envelope's included array. A
			// continuation URL can carry include even though --include itself is
			// rejected beside --next, so cover both request shapes.
			requestHasIncludes := len(includeValues) > 0 || betaTestersNextURLHasInclude(*next)
			if requestHasIncludes {
				fmt.Fprintln(os.Stderr, betaTesterIncludedRelationshipsWarning)
				if shared.NormalizeOutputFormat(*output.Output) != "json" {
					fmt.Fprintln(os.Stderr, "Note: included resources are only rendered in JSON output; re-run with --output json to see them.")
				}
			}

			if strings.TrimSpace(*group) != "" && strings.TrimSpace(*next) == "" {
				groupID, err := resolveBetaGroupID(requestCtx, client, resolvedAppID, *group)
				if err != nil {
					return fmt.Errorf("beta-testers list: %w", err)
				}
				opts = append(opts, asc.WithBetaTestersGroupIDs([]string{groupID}))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBetaTestersLimit(200))
				testers, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBetaTesters(ctx, resolvedAppID, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBetaTesters(ctx, resolvedAppID, asc.WithBetaTestersNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("beta-testers list: %w", err)
				}

				return shared.PrintOutput(testers, *output.Output, *output.Pretty)
			}

			testers, err := client.GetBetaTesters(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("beta-testers list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(testers, *output.Output, *output.Pretty)
		},
	}
}

// normalizeBetaTesterInviteTypes upper-cases and validates a comma-separated
// --invite-type value against the filter[inviteType] enum.
func normalizeBetaTesterInviteTypes(value string) ([]string, error) {
	values := shared.SplitCSVUpper(value)
	for _, item := range values {
		if !slices.Contains(betaTesterInviteTypeValues, item) {
			return nil, fmt.Errorf("--invite-type must be a comma-separated list of: %s", strings.Join(betaTesterInviteTypeValues, ", "))
		}
	}
	return values, nil
}

func betaTestersNextURLHasInclude(next string) bool {
	parsed, err := url.Parse(strings.TrimSpace(next))
	if err != nil {
		return false
	}
	return strings.TrimSpace(parsed.Query().Get("include")) != ""
}

// rejectBetaTestersNextFlagConflicts fails when a caller pairs --next with a
// flag whose value cannot reach the request, because a links.next URL is
// followed verbatim.
func rejectBetaTestersNextFlagConflicts(fs *flag.FlagSet, next string, names ...string) error {
	if strings.TrimSpace(next) == "" {
		return nil
	}
	provided := make(map[string]struct{})
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = struct{}{}
	})
	for _, name := range names {
		if _, ok := provided[name]; ok {
			return shared.WithDiagnostic(
				shared.UsageErrorf("beta-testers list: --next cannot be combined with --%s", name),
				shared.DiagnosticConflictingInput,
				"--"+name,
			)
		}
	}
	return nil
}

// BetaTestersGetCommand returns the beta testers get subcommand.
func BetaTestersGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	id := fs.String("id", "", "Beta tester ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-testers view --id \"TESTER_ID\" [flags]",
		ShortHelp:  "View a TestFlight beta tester by ID.",
		LongHelp: `View a TestFlight beta tester by ID.

Examples:
  asc testflight beta-testers view --id "TESTER_ID"

See also: asc testflight beta-testers beta-groups list --id "TESTER_ID" for the tester's group membership.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			tester, err := client.GetBetaTester(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("beta-testers view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(tester, *output.Output, *output.Pretty)
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
	group := shared.BindOnceCSVFlag(fs, "group", "Comma-separated beta group names or IDs")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add",
		ShortUsage: "asc testflight beta-testers add --app APP_ID --email EMAIL --group GROUP[,GROUP...]",
		ShortHelp:  "Add a TestFlight beta tester.",
		LongHelp: `Add a TestFlight beta tester.

The tester is added to every group in the comma-separated --group list. A
value that exactly matches one group name is used as-is, even when the name
contains commas. To combine a comma-containing group name with other groups
in one list, reference that group by its ID instead.

Examples:
  asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta"
  asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta,iOS 27"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}
			if strings.TrimSpace(*email) == "" {
				fmt.Fprintln(os.Stderr, "Error: --email is required")
				return shared.MissingRequiredUsageError("--email")
			}
			if strings.TrimSpace(group.String()) == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return shared.MissingRequiredUsageError("--group")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers add: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			groupIDs, err := resolveBetaGroupIDs(requestCtx, client, resolvedAppID, group.String())
			if err != nil {
				return fmt.Errorf("beta-testers add: %w", err)
			}

			tester, err := client.CreateBetaTester(requestCtx, *email, *firstName, *lastName, groupIDs)
			if err != nil {
				return fmt.Errorf("beta-testers add: failed to create: %w", err)
			}

			return shared.PrintOutput(tester, *output.Output, *output.Pretty)
		},
	}
}

// BetaTestersRemoveCommand returns the beta testers remove subcommand.
func BetaTestersRemoveCommand() *ffcli.Command {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	email := fs.String("email", "", "Tester email address")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	wait := fs.Bool("wait", false, "Wait until the removal is visible (tester is gone or reports REVOKED)")
	pollInterval := fs.Duration("poll-interval", betaTesterRemoveDefaultPollInterval, "Polling interval while waiting for removal visibility")
	timeout := fs.Duration("timeout", betaTesterRemoveDefaultWaitTimeout, "Maximum time to wait for removal visibility")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove",
		ShortUsage: "asc testflight beta-testers remove --app APP_ID --email EMAIL --confirm [--wait]",
		ShortHelp:  "Remove a TestFlight beta tester.",
		LongHelp: `Remove a TestFlight beta tester.

Removal deletes the beta tester record itself: every group membership and
build assignment is removed across all apps the tester belongs to, not only
the app used for the lookup. This cannot be undone, so --confirm is required.

Removed testers can continue to appear in list output with state REVOKED;
verify a removal with view --id, which reports the record as gone. Pass
--wait to block until that signal is observed.

Examples:
  asc testflight beta-testers remove --app "APP_ID" --email "tester@example.com" --confirm
  asc testflight beta-testers remove --app "APP_ID" --email "tester@example.com" --confirm --wait`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}
			if strings.TrimSpace(*email) == "" {
				fmt.Fprintln(os.Stderr, "Error: --email is required")
				return shared.MissingRequiredUsageError("--email")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}
			waitFlagsProvided := make([]string, 0, 2)
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "poll-interval" || f.Name == "timeout" {
					waitFlagsProvided = append(waitFlagsProvided, "--"+f.Name)
				}
			})
			if !*wait && len(waitFlagsProvided) > 0 {
				verb := "requires"
				if len(waitFlagsProvided) > 1 {
					verb = "require"
				}
				return shared.UsageError(strings.Join(waitFlagsProvided, " and ") + " " + verb + " --wait")
			}
			if *wait {
				if *pollInterval <= 0 {
					return shared.UsageError("--poll-interval must be greater than 0")
				}
				if *timeout <= 0 {
					return shared.UsageError("--timeout must be greater than 0")
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers remove: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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

			if *wait {
				if waitErr := waitForBetaTesterRemoval(ctx, client, testerID, *pollInterval, *timeout); waitErr != nil {
					if printErr := shared.PrintOutput(result, *output.Output, *output.Pretty); printErr != nil {
						return printErr
					}
					return fmt.Errorf("beta-testers remove: removal committed but not yet visible: %w", waitErr)
				}
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

const (
	betaTesterRemoveDefaultPollInterval = 5 * time.Second
	betaTesterRemoveDefaultWaitTimeout  = 2 * time.Minute
)

func waitForBetaTesterRemoval(ctx context.Context, client *asc.Client, testerID string, pollInterval, timeout time.Duration) error {
	waitCtx, cancel := shared.ContextWithTimeoutDuration(ctx, timeout)
	defer cancel()

	_, err := asc.PollUntil(waitCtx, pollInterval, func(pollCtx context.Context) (struct{}, bool, error) {
		tester, err := client.GetBetaTester(pollCtx, testerID)
		if err != nil {
			if asc.IsNotFound(err) {
				return struct{}{}, true, nil
			}
			return struct{}{}, false, err
		}
		if tester.Data.Attributes.State == asc.BetaTesterStateRevoked {
			return struct{}{}, true, nil
		}
		return struct{}{}, false, nil
	})
	return err
}

// BetaTestersAddGroupsCommand returns the beta testers add-groups subcommand.
func BetaTestersAddGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("add-groups", flag.ExitOnError)

	id := fs.String("id", "", "Beta tester ID")
	groups := shared.BindOnceCSVFlag(fs, "group", "Comma-separated beta group IDs")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add-groups",
		ShortUsage: "asc testflight beta-testers add-groups --id TESTER_ID --group GROUP_ID[,GROUP_ID...]",
		ShortHelp:  "Add a beta tester to beta groups.",
		LongHelp: `Add a beta tester to beta groups.

Examples:
  asc testflight beta-testers add-groups --id "TESTER_ID" --group "GROUP_ID"
  asc testflight beta-testers add-groups --id "TESTER_ID" --group "GROUP_ID_1,GROUP_ID_2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			groupIDs := shared.SplitCSV(groups.String())
			if len(groupIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return shared.MissingRequiredUsageError("--group")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers add-groups: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.AddBetaTesterToGroups(requestCtx, testerID, groupIDs); err != nil {
				return fmt.Errorf("beta-testers add-groups: failed to add groups: %w", err)
			}

			result := &asc.BetaTesterGroupsUpdateResult{
				TesterID: testerID,
				GroupIDs: groupIDs,
				Action:   "added",
			}

			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
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
	groups := shared.BindOnceCSVFlag(fs, "group", "Comma-separated beta group IDs")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-groups",
		ShortUsage: "asc testflight beta-testers remove-groups --id TESTER_ID --group GROUP_ID[,GROUP_ID...] --confirm",
		ShortHelp:  "Remove a beta tester from beta groups.",
		LongHelp: `Remove a beta tester from beta groups.

Examples:
  asc testflight beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID" --confirm
  asc testflight beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID_1,GROUP_ID_2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			groupIDs := shared.SplitCSV(groups.String())
			if len(groupIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return shared.MissingRequiredUsageError("--group")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers remove-groups: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.RemoveBetaTesterFromGroups(requestCtx, testerID, groupIDs); err != nil {
				return fmt.Errorf("beta-testers remove-groups: failed to remove groups: %w", err)
			}

			result := &asc.BetaTesterGroupsUpdateResult{
				TesterID: testerID,
				GroupIDs: groupIDs,
				Action:   "removed",
			}

			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Successfully removed tester %s from %d group(s)\n", testerID, len(groupIDs))
			return nil
		},
	}
}

// BetaTestersAddBuildsCommand returns the beta testers add-builds subcommand.
func BetaTestersAddBuildsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("add-builds", flag.ExitOnError)

	id := fs.String("id", "", "Beta tester ID")
	buildIDs := fs.String("build-id", "", "Comma-separated build IDs")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add-builds",
		ShortUsage: "asc testflight beta-testers add-builds --id TESTER_ID --build-id BUILD_ID[,BUILD_ID...]",
		ShortHelp:  "Add builds to a beta tester.",
		LongHelp: `Add builds to a beta tester.

Examples:
  asc testflight beta-testers add-builds --id "TESTER_ID" --build-id "BUILD_ID"
  asc testflight beta-testers add-builds --id "TESTER_ID" --build-id "BUILD_ID1,BUILD_ID2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			parsedBuildIDs := shared.SplitCSV(*buildIDs)
			if len(parsedBuildIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --build-id is required")
				return shared.MissingRequiredUsageError("--build-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers add-builds: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.AddBuildsToBetaTester(requestCtx, testerID, parsedBuildIDs); err != nil {
				return fmt.Errorf("beta-testers add-builds: failed to add builds: %w", err)
			}

			result := &asc.BetaTesterBuildsUpdateResult{
				TesterID: testerID,
				BuildIDs: parsedBuildIDs,
				Action:   "added",
			}

			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Successfully added tester %s to %d build(s)\n", testerID, len(parsedBuildIDs))
			return nil
		},
	}
}

// BetaTestersRemoveBuildsCommand returns the beta testers remove-builds subcommand.
func BetaTestersRemoveBuildsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("remove-builds", flag.ExitOnError)

	id := fs.String("id", "", "Beta tester ID")
	buildIDs := fs.String("build-id", "", "Comma-separated build IDs")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-builds",
		ShortUsage: "asc testflight beta-testers remove-builds --id TESTER_ID --build-id BUILD_ID[,BUILD_ID...] --confirm",
		ShortHelp:  "Remove builds from a beta tester.",
		LongHelp: `Remove builds from a beta tester.

Examples:
  asc testflight beta-testers remove-builds --id "TESTER_ID" --build-id "BUILD_ID" --confirm
  asc testflight beta-testers remove-builds --id "TESTER_ID" --build-id "BUILD_ID1,BUILD_ID2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			parsedBuildIDs := shared.SplitCSV(*buildIDs)
			if len(parsedBuildIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --build-id is required")
				return shared.MissingRequiredUsageError("--build-id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers remove-builds: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.RemoveBuildsFromBetaTester(requestCtx, testerID, parsedBuildIDs); err != nil {
				return fmt.Errorf("beta-testers remove-builds: failed to remove builds: %w", err)
			}

			result := &asc.BetaTesterBuildsUpdateResult{
				TesterID: testerID,
				BuildIDs: parsedBuildIDs,
				Action:   "removed",
			}

			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Successfully removed tester %s from %d build(s)\n", testerID, len(parsedBuildIDs))
			return nil
		},
	}
}

// BetaTestersRemoveAppsCommand returns the beta testers remove-apps subcommand.
func BetaTestersRemoveAppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("remove-apps", flag.ExitOnError)

	id := fs.String("id", "", "Beta tester ID")
	apps := shared.BindOnceCSVFlag(fs, "app", "Comma-separated app IDs")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-apps",
		ShortUsage: "asc testflight beta-testers remove-apps --id TESTER_ID --app APP_ID[,APP_ID...] --confirm",
		ShortHelp:  "Remove apps from a beta tester.",
		LongHelp: `Remove apps from a beta tester.

Examples:
  asc testflight beta-testers remove-apps --id "TESTER_ID" --app "APP_ID" --confirm
  asc testflight beta-testers remove-apps --id "TESTER_ID" --app "APP_ID1,APP_ID2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			appIDs := shared.SplitCSV(apps.String())
			if len(appIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --app is required")
				return shared.MissingRequiredUsageError("--app")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers remove-apps: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.RemoveBetaTesterFromApps(requestCtx, testerID, appIDs); err != nil {
				return fmt.Errorf("beta-testers remove-apps: failed to remove apps: %w", err)
			}

			result := &asc.BetaTesterAppsUpdateResult{
				TesterID: testerID,
				AppIDs:   appIDs,
				Action:   "removed",
			}

			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Successfully removed tester %s from %d app(s)\n", testerID, len(appIDs))
			return nil
		},
	}
}

// BetaTestersInviteCommand returns the beta testers invite subcommand.
func BetaTestersInviteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("invite", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	email := fs.String("email", "", "Tester email address")
	group := shared.BindOnceCSVFlag(fs, "group", "Comma-separated beta group names or IDs (optional, creates tester if missing)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "invite",
		ShortUsage: "asc testflight beta-testers invite [flags]",
		ShortHelp:  "Invite a TestFlight beta tester.",
		LongHelp: `Invite a TestFlight beta tester.

Examples:
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com"
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com" --group "Beta"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}
			if strings.TrimSpace(*email) == "" {
				fmt.Fprintln(os.Stderr, "Error: --email is required")
				return shared.MissingRequiredUsageError("--email")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-testers invite: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			emailValue := strings.TrimSpace(*email)
			groupValue := strings.TrimSpace(group.String())
			testerID, err := findBetaTesterIDByEmail(requestCtx, client, resolvedAppID, emailValue)
			if err != nil {
				if errors.Is(err, errBetaTesterNotFound) {
					if groupValue == "" {
						return fmt.Errorf("beta-testers invite: no tester found for %q (use beta-testers add --group ... or pass --group here)", emailValue)
					}

					groupIDs, resolveErr := resolveBetaGroupIDs(requestCtx, client, resolvedAppID, groupValue)
					if resolveErr != nil {
						return fmt.Errorf("beta-testers invite: %w", resolveErr)
					}

					created, createErr := client.CreateBetaTester(requestCtx, emailValue, "", "", groupIDs)
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

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
