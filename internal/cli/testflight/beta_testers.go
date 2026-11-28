package testflight

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBetaTestersCommand returns the beta testers command with subcommands built on d.
func newBetaTestersCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("beta-testers", flag.ExitOnError)

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
  asc testflight beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID" --confirm
  asc testflight beta-testers add-builds --id "TESTER_ID" --build-id "BUILD_ID"
  asc testflight beta-testers remove-builds --id "TESTER_ID" --build-id "BUILD_ID" --confirm
  asc testflight beta-testers remove-apps --id "TESTER_ID" --app "APP_ID" --confirm
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com"
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com" --group "Beta"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBetaTestersListCommand(d),
			newBetaTestersGetCommand(d),
			newBetaTestersAddCommand(d),
			newBetaTestersExportCommand(d),
			newBetaTestersImportCommand(d),
			newBetaTestersRemoveCommand(d),
			newBetaTestersAddGroupsCommand(d),
			newBetaTestersRemoveGroupsCommand(d),
			newBetaTestersAddBuildsCommand(d),
			newBetaTestersRemoveBuildsCommand(d),
			newBetaTestersRemoveAppsCommand(d),
			newBetaTestersRelationshipsCommand(d),
			newBetaTestersAppsCommand(d),
			newBetaTestersBetaGroupsCommand(d),
			newBetaTestersBuildsCommand(d),
			newBetaTestersMetricsCommand(d),
			newBetaTestersInviteCommand(d),
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

// newBetaTestersListCommand returns the beta testers list subcommand built on d.
func newBetaTestersListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID to filter")
	shared.BindFlagAlias(fs, "build", "build-id")
	group := shared.BindResourceIDFlag(fs, "group", "betaGroups", "Beta group name or ID to filter")
	email := fs.String("email", "", "Filter by tester email")
	firstName := fs.String("first-name", "", "Filter by tester first name (exact match)")
	lastName := fs.String("last-name", "", "Filter by tester last name (exact match)")
	inviteType := fs.String("invite-type", "", "Filter by invite type(s), comma-separated: "+strings.Join(betaTesterInviteTypeValues, ", "))
	sortBy := fs.String("sort", "", "Sort by: "+strings.Join(betaTesterSortValues, ", "))
	include := fs.String("include", "", "Include related resources, comma-separated: "+strings.Join(betaTesterIncludeValues, ", "))
	includedFields := shared.BindIncludedFieldFlags(fs, map[string]string{"apps": "apps", "betaGroups": "betaGroups", "builds": "builds"})
	fields := fs.String("fields", "", "Fields to include for beta testers, comma-separated: "+strings.Join(betaGroupTesterFieldsValues, ", "))
	devices := fs.Bool("devices", false, "Request appDevices and show one row per reported device in table or markdown output")
	output := d.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-testers list [flags]",
		ShortHelp:  "List TestFlight beta testers for an app.",
		LongHelp: `List TestFlight beta testers for an app.

To list one group's testers without --app, run
asc testflight groups testers list --group-id "GROUP_ID".

--include adds the related resources to the response's top-level "included"
array, which only JSON output renders. App Store Connect returns at most 50
related resources per included relationship. --paginate pages the tester
collection, not included relationships. For complete group membership, run
asc testflight testers groups list --id "TESTER_ID" --paginate.
A --next URL retains any include query from its original request; JSON output
is still required to render those included resources.

--devices requests appDevices (or adds it to --fields) and shows tester ID,
model, platform, OS, and app build in table or markdown output. Table and markdown
omit names and emails; JSON keeps Apple's response envelope. Device entries
have no timestamp or app ID, so this view does not infer a latest installation
or associate entries with individual apps. Filters select testers, not device
entries. --paginate pages testers. With --devices, a --next URL must already
select appDevices in fields[betaTesters]. CSV export is unchanged.

Filter, --fields, --sort, and --include flags cannot be combined with --next: a
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
  asc testflight beta-testers list --app "APP_ID" --fields "email,state"
  asc testflight beta-testers list --app "APP_ID" --devices --paginate --output table
  asc testflight beta-testers list --app "APP_ID" --limit 25
  asc testflight beta-testers list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "testflight beta-testers list", 200); err != nil {
				return err
			}

			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			if err := shared.ValidateSort(*sortBy, betaTesterSortValues...); err != nil {
				fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", err.Error())
				return shared.InvalidValueUsageError("--sort")
			}
			if err := shared.ValidateInclude(*include, betaTesterIncludeValues...); err != nil {
				fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", err.Error())
				return shared.InvalidValueUsageError("--include")
			}
			fieldValues, err := shared.NormalizeSelection(*fields, betaGroupTesterFieldsValues, "--fields")
			if err != nil {
				fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", err.Error())
				return shared.InvalidValueUsageError("--fields")
			}
			inviteTypes, err := normalizeBetaTesterInviteTypes(*inviteType)
			if err != nil {
				fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", err.Error())
				return shared.InvalidValueUsageError("--invite-type")
			}
			command := shared.RewriteUsageMessage(ctx, "beta-testers list")
			// A links.next URL already carries the query it was produced from, so
			// these flags would be accepted and silently dropped.
			if err := rejectBetaTestersNextFlagConflicts(d, fs, *next, command, "build-id", "group", "email", "first-name", "last-name", "invite-type", "sort", "include", "fields"); err != nil {
				return err
			}
			if *devices {
				if strings.TrimSpace(*next) != "" {
					nextURL, err := url.Parse(strings.TrimSpace(*next))
					if err != nil || !slices.Contains(shared.SplitCSV(nextURL.Query().Get("fields[betaTesters]")), "appDevices") {
						return d.UsageError("--devices requires --next to include appDevices in fields[betaTesters]")
					}
				} else if !slices.Contains(fieldValues, "appDevices") {
					fieldValues = append(fieldValues, "appDevices")
				}
			}
			included, err := includedFields.ResolveWith(d, command, *next, shared.SplitCSV(*include))
			if err != nil {
				return err
			}
			if strings.TrimSpace(*group) != "" && strings.TrimSpace(*buildID) != "" {
				return shared.WithDiagnostic(
					d.UsageError("--group cannot be combined with --build-id"),
					shared.DiagnosticConflictingInput,
					"",
				)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-testers list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BetaTestersOption{
				asc.Limit[asc.BetaTestersOption](*limit),
				asc.NextURL[asc.BetaTestersOption](*next),
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

			if len(fieldValues) > 0 {
				opts = append(opts, asc.WithBetaTestersFields(fieldValues))
			}

			includeValues := shared.SplitCSV(*include)
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithBetaTestersInclude(includeValues), asc.WithIncludedFields[asc.BetaTestersOption](included))
			}
			// Only the JSON renderer emits the envelope's included array. A
			// continuation URL can carry include even though --include itself is
			// rejected beside --next, so cover both request shapes.
			requestHasIncludes := len(includeValues) > 0 || betaTestersNextURLHasInclude(*next)
			if requestHasIncludes {
				fmt.Fprintln(d.IO.ErrOut, betaTesterIncludedRelationshipsWarning)
				if shared.NormalizeOutputFormat(*output.Output) != "json" {
					fmt.Fprintln(d.IO.ErrOut, "Note: included resources are only rendered in JSON output; re-run with --output json to see them.")
				}
			}

			if strings.TrimSpace(*group) != "" {
				groupID, err := resolveBetaGroupID(requestCtx, client, resolvedAppID, *group)
				if err != nil {
					return fmt.Errorf("beta-testers list: %w", err)
				}
				opts = append(opts, asc.WithBetaTestersGroupIDs([]string{groupID}))
			}

			printTesters := func(response any) error {
				if *devices {
					testers, ok := response.(*asc.BetaTestersResponse)
					if !ok {
						return fmt.Errorf("beta-testers list: unexpected response type %T", response)
					}
					response = &asc.BetaTesterDevicesResponse{BetaTestersResponse: *testers}
				}
				return d.PrintOutput(response, *output.Output, *output.Pretty)
			}
			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.BetaTestersOption](200))
				testers, err := d.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBetaTesters(ctx, resolvedAppID, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBetaTesters(ctx, resolvedAppID, asc.NextURL[asc.BetaTestersOption](nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("beta-testers list: %w", err)
				}

				return printTesters(testers)
			}

			testers, err := client.GetBetaTesters(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("beta-testers list: failed to fetch: %w", err)
			}

			return printTesters(testers)
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
func rejectBetaTestersNextFlagConflicts(d *shared.Deps, fs *flag.FlagSet, next, command string, names ...string) error {
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
				d.UsageErrorf("%s: --next cannot be combined with --%s", command, name),
				shared.DiagnosticConflictingInput,
				"--"+name,
			)
		}
	}
	return nil
}

// newBetaTestersGetCommand returns the beta testers get subcommand built on d.
func newBetaTestersGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc testflight beta-testers view --id \"TESTER_ID\" [flags]",
		ShortHelp:  "View a TestFlight beta tester by ID.",
		LongHelp: `View a TestFlight beta tester by ID.

Examples:
  asc testflight beta-testers view --id "TESTER_ID"

See also: asc testflight beta-testers beta-groups list --id "TESTER_ID" for the tester's group membership.`,
		IDUsage:            "Beta tester ID",
		IDType:             "betaTesters",
		RequiredDiagnostic: true,
		ErrorPrefix:        "beta-testers view",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBetaTester(ctx, id)
		},
	})
}

// newBetaTestersAddCommand returns the beta testers add subcommand built on d.
func newBetaTestersAddCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("add", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, scopes groups and builds (or ASC_APP_ID env)")
	email := fs.String("email", "", "Tester email address")
	firstName := fs.String("first-name", "", "Tester first name")
	lastName := fs.String("last-name", "", "Tester last name")
	group := shared.BindOnceCSVFlag(fs, "group", "Comma-separated beta group names or IDs")
	buildID := shared.BindOnceCSVFlag(fs, "build-id", "Build ID(s) to attach, comma-separated")
	createMissingGroups := fs.Bool("create-missing-groups", false, "Create unresolved group names as external groups with public links disabled (requires --app and --group)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add",
		ShortUsage: "asc testflight beta-testers add --email EMAIL [--app APP_ID --group GROUP[,GROUP...]] [--build-id BUILD_ID[,BUILD_ID...]]",
		ShortHelp:  "Add a TestFlight beta tester.",
		LongHelp: `Add a TestFlight beta tester.

The tester is added to every group in the comma-separated --group list. A
value that exactly matches one group name is used as-is, even when the name
contains commas. To combine a comma-containing group name with other groups
in one list, reference that group by its ID instead.

--build-id attaches builds in the same create request. At least one of --group
or --build-id is required, and --app is required only to resolve --group.
When an app is resolved, every build must belong to it before any mutation.

Add first looks up the input email within the selected app, adds only
missing groups and builds, and prints the existing tester when found. Repeating
the same request sends no writes. First and last names apply only on creation.
--create-missing-groups creates unresolved group names as external groups with
public links disabled; --app and --group are required.
The build-only form resolves each named build's app before looking up the tester.
Its output retains the tester resource envelope with only ID, type and email;
other profile fields are omitted. Only named builds are read or assigned.
An email outside your team cannot join an internal group: add the
person to an external group instead. Inviting them to the team (asc users
invite) gives them a team role and is for the Account Holder or an Admin to
decide.

Examples:
  asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta"
  asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta,iOS 27"
  asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta" --build-id "BUILD_ID1,BUILD_ID2"
  asc testflight beta-testers add --email "tester@example.com" --build-id "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildIDs, err := parseCreateRelationshipIDs(d, buildID, "--build-id")
			if err != nil {
				return err
			}
			hasGroup := strings.TrimSpace(group.String()) != ""
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" && (hasGroup || len(buildIDs) == 0) {
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}
			if strings.TrimSpace(*email) == "" {
				return d.RequiredFlag("--email")
			}
			if !hasGroup && (group.Provided() || len(buildIDs) == 0) {
				fmt.Fprintln(d.IO.ErrOut, "Error: --group is required unless --build-id is set")
				return shared.MissingRequiredUsageError("--group")
			}

			if *createMissingGroups && (resolvedAppID == "" || !hasGroup) {
				return d.UsageError("--create-missing-groups requires --app and --group")
			}
			if !isValidTesterEmail(strings.TrimSpace(*email)) {
				return d.UsageError("invalid email format")
			}

			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}

			ctx = asc.WithNoRedirectReads(ctx)
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-testers add: %w", err)
			}

			lookupAppIDs := []string{}
			if resolvedAppID != "" {
				lookupAppIDs = append(lookupAppIDs, resolvedAppID)
			}
			for _, id := range buildIDs {
				requestCtx, cancel := d.Timeout(ctx)
				owner, err := client.GetBuildAppRelationship(requestCtx, id)
				cancel()
				if err != nil {
					return fmt.Errorf("beta-testers add: read app for build %q: %w", id, err)
				}
				if owner == nil || owner.Data.Type != asc.ResourceTypeApps || strings.TrimSpace(owner.Data.ID) == "" {
					return fmt.Errorf("beta-testers add: build %q returned no verified owning app", id)
				}
				ownerID := strings.TrimSpace(owner.Data.ID)
				if resolvedAppID != "" && ownerID != resolvedAppID {
					return d.UsageErrorf("--build-id %q does not belong to --app %q", id, resolvedAppID)
				}
				lookupAppIDs = append(lookupAppIDs, ownerID)
			}
			lookupAppIDs = uniqueSortedStrings(lookupAppIDs)
			printTester := func(tester *asc.BetaTesterResponse) error {
				if !hasGroup {
					if tester == nil || strings.TrimSpace(tester.Data.ID) == "" {
						return fmt.Errorf("beta-testers add: tester returned no ID")
					}
					tester = &asc.BetaTesterResponse{Data: asc.Resource[asc.BetaTesterAttributes]{
						Type:       asc.ResourceTypeBetaTesters,
						ID:         tester.Data.ID,
						Attributes: asc.BetaTesterAttributes{Email: strings.TrimSpace(*email)},
					}}
				}
				return d.PrintOutput(tester, *output.Output, *output.Pretty)
			}

			var groupIDs []string
			var resolver *betaGroupResolver
			var requested, missing []string
			if hasGroup {
				resolver, err = newBetaGroupResolver(ctx, d, client, resolvedAppID)
				if err != nil {
					return fmt.Errorf("beta-testers add: %w", err)
				}
				if *createMissingGroups {
					requested, missing, err = planBetaTesterAddGroups(resolver, group.String())
				} else {
					groupIDs, err = matchBetaGroupIDs(resolver.groups, group.String())
				}
				if err != nil {
					return fmt.Errorf("beta-testers add: %w", err)
				}
			}
			var fields []string
			if !hasGroup {
				fields = []string{"email"}
			}
			found, err := fetchExistingTestersByEmail(ctx, d, client, lookupAppIDs, []betaTestersCSVRow{{email: *email}}, fields...)
			if err != nil {
				return fmt.Errorf("beta-testers add: look up tester: %w", err)
			}
			existing := found[strings.ToLower(strings.TrimSpace(*email))]
			for _, name := range missing {
				id, err := ensureBetaTesterImportGroup(ctx, d, client, resolvedAppID, name)
				if err != nil {
					return fmt.Errorf("beta-testers add: create group %q: %w", name, err)
				}
				resolver.addImportGroup(name, id)
				fmt.Fprintf(d.IO.ErrOut, "Created external beta group %q (%s)\n", shared.SanitizeTerminal(name), shared.SanitizeTerminal(id))
			}
			if *createMissingGroups {
				groupIDs, err = resolver.ResolveAll(requested)
				if err != nil {
					return fmt.Errorf("beta-testers add: %w", err)
				}
			}
			if existing != nil {
				outcome, err := reconcileBetaTesterRelationships(ctx, d, client, existing, groupIDs, buildIDs)
				if err != nil {
					return fmt.Errorf("beta-testers add: %w", err)
				}
				reportBetaTesterAddConflict(d, strings.TrimSpace(*email), outcome)
				return printTester(outcome.tester)
			}

			requestCtx, cancel := d.Timeout(ctx)
			tester, err := client.CreateBetaTester(requestCtx, *email, *firstName, *lastName, groupIDs, buildIDs)
			cancel()
			if err != nil {
				if !isHTTPConflict(err) {
					return fmt.Errorf("beta-testers add: failed to create: %w", err)
				}
				outcome, resolveErr := resolveBetaTesterAddConflict(ctx, d, client, lookupAppIDs, strings.TrimSpace(*email), groupIDs, buildIDs)
				if resolveErr != nil {
					return fmt.Errorf("beta-testers add: failed to create: %w (%w)", err, resolveErr)
				}
				if outcome == nil || outcome.tester == nil {
					if outcome != nil && len(outcome.internalGroups) > 0 {
						return &shared.HintedError{
							Err: fmt.Errorf("beta-testers add: failed to create: %w", err),
							// No runnable invite here: joining the team grants a
							// team role, which is not what adding a tester asks for.
							Hint: fmt.Sprintf("%s is not a tester of app %s, and %s is an internal group: internal groups take only users of your App Store Connect team who can see this app. To let the person test without joining the team, add them to an external group. Inviting them to the team (asc users invite) is a different, bigger action that gives them a team role; the Account Holder or an Admin decides it.",
								strings.TrimSpace(*email), resolvedAppID, strings.Join(outcome.internalGroups, ", ")),
						}
					}
					return fmt.Errorf("beta-testers add: failed to create: %w", err)
				}
				reportBetaTesterAddConflict(d, strings.TrimSpace(*email), outcome)
				tester = outcome.tester
			}

			return printTester(tester)
		},
	}
}

// newBetaTestersRemoveCommand returns the beta testers remove subcommand built on d.
func newBetaTestersRemoveCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("remove", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	email := fs.String("email", "", "Tester email address")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	wait := fs.Bool("wait", false, "Wait until the removal is visible (tester is gone or reports REVOKED)")
	pollInterval := fs.Duration("poll-interval", betaTesterRemoveDefaultPollInterval, "Polling interval while waiting for removal visibility")
	timeout := fs.Duration("timeout", betaTesterRemoveDefaultWaitTimeout, "Maximum time to wait for removal visibility")
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}
			if strings.TrimSpace(*email) == "" {
				return d.RequiredFlag("--email")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
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
				return d.UsageError(strings.Join(waitFlagsProvided, " and ") + " " + verb + " --wait")
			}
			if *wait {
				if *pollInterval <= 0 {
					return d.UsageError("--poll-interval must be greater than 0")
				}
				if *timeout <= 0 {
					return d.UsageError("--timeout must be greater than 0")
				}
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-testers remove: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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
					if printErr := d.PrintOutput(result, *output.Output, *output.Pretty); printErr != nil {
						return printErr
					}
					return fmt.Errorf("beta-testers remove: removal committed but not yet visible: %w", waitErr)
				}
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
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

// newBetaTestersAddGroupsCommand returns the beta testers add-groups subcommand built on d.
func newBetaTestersAddGroupsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("add-groups", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaTesters", "Beta tester ID")
	groups := shared.BindOnceCSVFlag(fs, "group", "Comma-separated beta group IDs")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add-groups",
		ShortUsage: "asc testflight beta-testers add-groups --id TESTER_ID --group GROUP_ID[,GROUP_ID...]",
		ShortHelp:  "Add a beta tester to beta groups.",
		LongHelp: `Add a beta tester to beta groups.

Examples:
  asc testflight beta-testers add-groups --id "TESTER_ID" --group "GROUP_ID"
  asc testflight beta-testers add-groups --id "TESTER_ID" --group "GROUP_ID_1,GROUP_ID_2"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				return d.RequiredFlag("--id")
			}

			groupIDs := shared.SplitCSV(groups.String())
			if len(groupIDs) == 0 {
				return d.RequiredFlag("--group")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-testers add-groups: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			action := asc.BetaGroupTestersActionAdded
			if err := client.AddBetaTesterToGroups(requestCtx, testerID, groupIDs); err != nil {
				alreadySatisfied, verificationErr := betaTesterGroupConflictAlreadySatisfied(ctx, d, client, testerID, groupIDs, err)
				if verificationErr != nil {
					return fmt.Errorf("beta-testers add-groups: failed to add groups: %w (failed to verify beta group membership: %w)", err, verificationErr)
				}
				if !alreadySatisfied {
					return fmt.Errorf("beta-testers add-groups: failed to add groups: %w", err)
				}
				action = asc.BetaGroupTestersActionSkipped
			}

			result := &asc.BetaTesterGroupsUpdateResult{
				TesterID: testerID,
				GroupIDs: groupIDs,
				Action:   action,
			}

			if err := d.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}

			if action == asc.BetaGroupTestersActionSkipped {
				fmt.Fprintf(d.IO.ErrOut, "Skipped: tester %s already in %d group(s)\n", testerID, len(groupIDs))
				return nil
			}

			fmt.Fprintf(d.IO.ErrOut, "Successfully added tester %s to %d group(s)\n", testerID, len(groupIDs))
			return nil
		},
	}
}

// newBetaTestersRemoveGroupsCommand returns the beta testers remove-groups subcommand built on d.
func newBetaTestersRemoveGroupsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("remove-groups", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaTesters", "Beta tester ID")
	groups := shared.BindOnceCSVFlag(fs, "group", "Comma-separated beta group IDs")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-groups",
		ShortUsage: "asc testflight beta-testers remove-groups --id TESTER_ID --group GROUP_ID[,GROUP_ID...] --confirm",
		ShortHelp:  "Remove a beta tester from beta groups.",
		LongHelp: `Remove a beta tester from beta groups.

Examples:
  asc testflight beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID" --confirm
  asc testflight beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID_1,GROUP_ID_2" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				return d.RequiredFlag("--id")
			}

			groupIDs := shared.SplitCSV(groups.String())
			if len(groupIDs) == 0 {
				return d.RequiredFlag("--group")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-testers remove-groups: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.RemoveBetaTesterFromGroups(requestCtx, testerID, groupIDs); err != nil {
				return fmt.Errorf("beta-testers remove-groups: failed to remove groups: %w", err)
			}

			result := &asc.BetaTesterGroupsUpdateResult{
				TesterID: testerID,
				GroupIDs: groupIDs,
				Action:   "removed",
			}

			if err := d.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}

			fmt.Fprintf(d.IO.ErrOut, "Successfully removed tester %s from %d group(s)\n", testerID, len(groupIDs))
			return nil
		},
	}
}

// newBetaTestersAddBuildsCommand returns the beta testers add-builds subcommand built on d.
func newBetaTestersAddBuildsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("add-builds", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaTesters", "Beta tester ID")
	buildIDs := fs.String("build-id", "", "Comma-separated build IDs")
	shared.BindFlagAlias(fs, "build", "build-id")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add-builds",
		ShortUsage: "asc testflight beta-testers add-builds --id TESTER_ID --build-id BUILD_ID[,BUILD_ID...]",
		ShortHelp:  "Add builds to a beta tester.",
		LongHelp: `Add builds to a beta tester.

Examples:
  asc testflight beta-testers add-builds --id "TESTER_ID" --build-id "BUILD_ID"
  asc testflight beta-testers add-builds --id "TESTER_ID" --build-id "BUILD_ID1,BUILD_ID2"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				return d.RequiredFlag("--id")
			}

			parsedBuildIDs := shared.SplitCSV(*buildIDs)
			if len(parsedBuildIDs) == 0 {
				return d.RequiredFlag("--build-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-testers add-builds: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.AddBuildsToBetaTester(requestCtx, testerID, parsedBuildIDs); err != nil {
				return fmt.Errorf("beta-testers add-builds: failed to add builds: %w", err)
			}

			result := &asc.BetaTesterBuildsUpdateResult{
				TesterID: testerID,
				BuildIDs: parsedBuildIDs,
				Action:   "added",
			}

			if err := d.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}

			fmt.Fprintf(d.IO.ErrOut, "Successfully added tester %s to %d build(s)\n", testerID, len(parsedBuildIDs))
			return nil
		},
	}
}

// newBetaTestersRemoveBuildsCommand returns the beta testers remove-builds subcommand built on d.
func newBetaTestersRemoveBuildsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("remove-builds", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaTesters", "Beta tester ID")
	buildIDs := fs.String("build-id", "", "Comma-separated build IDs")
	shared.BindFlagAlias(fs, "build", "build-id")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-builds",
		ShortUsage: "asc testflight beta-testers remove-builds --id TESTER_ID --build-id BUILD_ID[,BUILD_ID...] --confirm",
		ShortHelp:  "Remove builds from a beta tester.",
		LongHelp: `Remove builds from a beta tester.

Examples:
  asc testflight beta-testers remove-builds --id "TESTER_ID" --build-id "BUILD_ID" --confirm
  asc testflight beta-testers remove-builds --id "TESTER_ID" --build-id "BUILD_ID1,BUILD_ID2" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				return d.RequiredFlag("--id")
			}

			parsedBuildIDs := shared.SplitCSV(*buildIDs)
			if len(parsedBuildIDs) == 0 {
				return d.RequiredFlag("--build-id")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-testers remove-builds: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.RemoveBuildsFromBetaTester(requestCtx, testerID, parsedBuildIDs); err != nil {
				return fmt.Errorf("beta-testers remove-builds: failed to remove builds: %w", err)
			}

			result := &asc.BetaTesterBuildsUpdateResult{
				TesterID: testerID,
				BuildIDs: parsedBuildIDs,
				Action:   "removed",
			}

			if err := d.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}

			fmt.Fprintf(d.IO.ErrOut, "Successfully removed tester %s from %d build(s)\n", testerID, len(parsedBuildIDs))
			return nil
		},
	}
}

// newBetaTestersRemoveAppsCommand returns the beta testers remove-apps subcommand built on d.
func newBetaTestersRemoveAppsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("remove-apps", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaTesters", "Beta tester ID")
	apps := shared.BindOnceCSVFlag(fs, "app", "Comma-separated app IDs")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-apps",
		ShortUsage: "asc testflight beta-testers remove-apps --id TESTER_ID --app APP_ID[,APP_ID...] --confirm",
		ShortHelp:  "Remove apps from a beta tester.",
		LongHelp: `Remove apps from a beta tester.

Examples:
  asc testflight beta-testers remove-apps --id "TESTER_ID" --app "APP_ID" --confirm
  asc testflight beta-testers remove-apps --id "TESTER_ID" --app "APP_ID1,APP_ID2" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			testerID := strings.TrimSpace(*id)
			if testerID == "" {
				return d.RequiredFlag("--id")
			}

			appIDs := shared.SplitCSV(apps.String())
			if len(appIDs) == 0 {
				return d.RequiredFlag("--app")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-testers remove-apps: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.RemoveBetaTesterFromApps(requestCtx, testerID, appIDs); err != nil {
				return fmt.Errorf("beta-testers remove-apps: failed to remove apps: %w", err)
			}

			result := &asc.BetaTesterAppsUpdateResult{
				TesterID: testerID,
				AppIDs:   appIDs,
				Action:   "removed",
			}

			if err := d.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}

			fmt.Fprintf(d.IO.ErrOut, "Successfully removed tester %s from %d app(s)\n", testerID, len(appIDs))
			return nil
		},
	}
}

// newBetaTestersInviteCommand returns the beta testers invite subcommand built on d.
func newBetaTestersInviteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("invite", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	email := fs.String("email", "", "Tester email address")
	group := shared.BindOnceCSVFlag(fs, "group", "Comma-separated beta group names or IDs (optional, creates tester if missing)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "invite",
		ShortUsage: "asc testflight beta-testers invite [flags]",
		ShortHelp:  "Invite a TestFlight beta tester.",
		LongHelp: `Invite a TestFlight beta tester.

Examples:
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com"
  asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com" --group "Beta"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}
			if strings.TrimSpace(*email) == "" {
				return d.RequiredFlag("--email")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-testers invite: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

					created, createErr := client.CreateBetaTester(requestCtx, emailValue, "", "", groupIDs, nil)
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

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
