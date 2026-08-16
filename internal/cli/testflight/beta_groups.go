package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const buildGroupMembershipTimeout = 5 * time.Minute

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
  asc testflight beta-groups list --build-id "BUILD_ID"
  asc testflight beta-groups list --app "APP_ID" --internal
  asc testflight beta-groups list --global --internal
  asc testflight beta-groups create --app "APP_ID" --name "Beta Testers"
  asc testflight beta-groups create --app "APP_ID" --name "Internal Testers" --internal
  asc testflight beta-groups app view --group-id "GROUP_ID"
  asc testflight beta-groups beta-recruitment-criteria view --group-id "GROUP_ID"
  asc testflight beta-groups beta-recruitment-criterion-compatible-build-check view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaGroupsListCommand(),
			BetaGroupsCreateCommand(),
			BetaGroupsGetCommand(),
			BetaGroupsAppCommand(),
			BetaGroupsRecruitmentCriteriaCommand(),
			BetaGroupsRecruitmentCriterionCompatibleBuildCheckCommand(),
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
	buildID := fs.String("build-id", "", "[experimental] List groups that contain this build ID")
	global := fs.Bool("global", false, "List beta groups across all apps (top-level endpoint)")
	internal := fs.Bool("internal", false, "Filter to internal groups only")
	external := fs.Bool("external", false, "Filter to external groups only")
	output := shared.BindOutputFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-groups list [flags]",
		ShortHelp:  "List TestFlight beta groups for an app or globally.",
		LongHelp: `List TestFlight beta groups for an app or globally.

The --build-id lookup is experimental. It resolves the build's app and
automatically paginates the app's groups, returning both explicit build
relationships and groups with all-build access.
App Store Connect exposes no GET /v1/builds/{id}/relationships/betaGroups. It
does document include=betaGroups on GET /v1/builds/{id}, but that read caps
included groups at limit[betaGroups]=50, has no documented build-side endpoint
for paging past the cap, and reports the same explicit linkage Apple can omit
for all-build groups. So the command prefers the documented betaGroups build
filter. If that filter is rejected, the command falls back to the inverse
group-to-build relationship.
All-build groups omitted by the filter are also checked through that inverse
linkage because Apple can omit their explicit relationships. These checks scan
linkage IDs only; cost scales with the checked groups and their build page count.

A complete lookup with no memberships prints an empty groups array and exits 0.
If an inverse relationship cannot be read, available matches and failures are
printed with complete=false and the command exits nonzero.

Examples:
  asc testflight beta-groups list --app "APP_ID"
  asc testflight beta-groups list --build-id "BUILD_ID"
  asc testflight beta-groups list --build-id "BUILD_ID" --internal
  asc testflight beta-groups list --app "APP_ID" --internal
  asc testflight beta-groups list --app "APP_ID" --external
  asc testflight beta-groups list --app "APP_ID" --limit 10
  asc testflight beta-groups list --app "APP_ID" --paginate
  asc testflight beta-groups list --global
  asc testflight beta-groups list --global --limit 50
  asc testflight beta-groups list --global --internal`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.WithDiagnostic(
					shared.NewValidationError(fmt.Errorf("beta-groups list: --limit must be between 1 and 200")),
					shared.DiagnosticInvalidInput,
					"--limit",
				)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("beta-groups list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			resolvedBuildID := strings.TrimSpace(*buildID)

			if *internal && *external {
				fmt.Fprintln(os.Stderr, "Error: --internal and --external are mutually exclusive")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}
			appIDSet := false
			buildIDSet := false
			membershipPageControlSet := false
			fs.Visit(func(value *flag.Flag) {
				switch value.Name {
				case "app":
					appIDSet = true
				case "build-id":
					buildIDSet = true
				case "global", "limit", "next", "paginate":
					membershipPageControlSet = true
				}
			})
			if buildIDSet && resolvedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-id cannot be empty")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--build-id")
			}
			if resolvedBuildID != "" && appIDSet && strings.TrimSpace(*appID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app cannot be empty when used with --build-id")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--app")
			}
			if resolvedBuildID != "" && membershipPageControlSet {
				fmt.Fprintln(os.Stderr, "Error: --global, --limit, --next, and --paginate cannot be used with --build-id; membership lookup always fetches all required pages")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}

			if resolvedBuildID != "" {
				expectedAppID := ""
				if appIDSet {
					expectedAppID = strings.TrimSpace(*appID)
				}
				var internalFilter *bool
				if *internal {
					value := true
					internalFilter = &value
				} else if *external {
					value := false
					internalFilter = &value
				}

				return runBuildGroupMembershipList(
					ctx,
					resolvedBuildID,
					expectedAppID,
					internalFilter,
					*output.Output,
					*output.Pretty,
					"beta-groups list",
				)
			}

			// Reject --global + --app combination (check explicit flag, not resolved value)
			if *global && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --global and --app are mutually exclusive")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}

			// Require one of --app or --global (unless --next is provided)
			if !*global && resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app or --global is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var internalFilter *bool
			if *internal {
				v := true
				internalFilter = &v
			} else if *external {
				v := false
				internalFilter = &v
			}

			opts := []asc.BetaGroupsOption{
				asc.WithBetaGroupsLimit(*limit),
				asc.WithBetaGroupsNextURL(*next),
			}

			if *global {
				if internalFilter != nil {
					opts = append(opts, asc.WithBetaGroupsIsInternal(*internalFilter))
				}

				if *paginate {
					paginateOpts := append(opts, asc.WithBetaGroupsLimit(200))
					groups, err := shared.PaginateWithSpinner(
						requestCtx,
						func(ctx context.Context) (asc.PaginatedResponse, error) {
							return client.ListBetaGroups(ctx, paginateOpts...)
						},
						func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
							return client.ListBetaGroups(ctx, asc.WithBetaGroupsNextURL(nextURL))
						},
					)
					if err != nil {
						return fmt.Errorf("beta-groups list: %w", err)
					}

					return shared.PrintOutput(groups, *output.Output, *output.Pretty)
				}

				groups, err := client.ListBetaGroups(requestCtx, opts...)
				if err != nil {
					return fmt.Errorf("beta-groups list: failed to fetch: %w", err)
				}

				return shared.PrintOutput(groups, *output.Output, *output.Pretty)
			}

			// The app-scoped endpoint /v1/apps/{id}/betaGroups does not accept
			// filter[isInternalGroup], so we apply the filter client-side.
			if internalFilter != nil {
				var groups *asc.BetaGroupsResponse

				if *paginate {
					paginateOpts := append(opts, asc.WithBetaGroupsLimit(200))
					resp, err := shared.PaginateWithSpinner(
						requestCtx,
						func(ctx context.Context) (asc.PaginatedResponse, error) {
							return client.GetBetaGroups(ctx, resolvedAppID, paginateOpts...)
						},
						func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
							return client.GetBetaGroups(ctx, resolvedAppID, asc.WithBetaGroupsNextURL(nextURL))
						},
					)
					if err != nil {
						return fmt.Errorf("beta-groups list: %w", err)
					}
					var ok bool
					groups, ok = resp.(*asc.BetaGroupsResponse)
					if !ok {
						return fmt.Errorf("beta-groups list: unexpected response type %T", resp)
					}
				} else {
					// To apply the filter correctly, fetch all pages even without --paginate.
					paginateOpts := append(opts, asc.WithBetaGroupsLimit(200))
					firstPage, err := client.GetBetaGroups(requestCtx, resolvedAppID, paginateOpts...)
					if err != nil {
						return fmt.Errorf("beta-groups list: failed to fetch: %w", err)
					}
					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBetaGroups(ctx, resolvedAppID, asc.WithBetaGroupsNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("beta-groups list: %w", err)
					}
					var ok bool
					groups, ok = resp.(*asc.BetaGroupsResponse)
					if !ok {
						return fmt.Errorf("beta-groups list: unexpected response type %T", resp)
					}
				}

				filtered := *groups
				filtered.Data = make([]asc.Resource[asc.BetaGroupAttributes], 0, len(groups.Data))
				for _, g := range groups.Data {
					if g.Attributes.IsInternalGroup == *internalFilter {
						filtered.Data = append(filtered.Data, g)
					}
				}
				if *limit > 0 && len(filtered.Data) > *limit {
					filtered.Data = filtered.Data[:*limit]
				}

				return shared.PrintOutput(&filtered, *output.Output, *output.Pretty)
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBetaGroupsLimit(200))
				groups, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBetaGroups(ctx, resolvedAppID, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBetaGroups(ctx, resolvedAppID, asc.WithBetaGroupsNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("beta-groups list: %w", err)
				}

				return shared.PrintOutput(groups, *output.Output, *output.Pretty)
			}

			groups, err := client.GetBetaGroups(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("beta-groups list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(groups, *output.Output, *output.Pretty)
		},
	}
}

// BuildGroupsListCommandConfig configures the build-centric beta-group lookup
// surface while keeping the membership implementation in one place.
type BuildGroupsListCommandConfig struct {
	ShortUsage  string
	ShortHelp   string
	LongHelp    string
	ErrorPrefix string
}

// BuildGroupsListCommand returns a narrow list command for beta groups that
// contain a required build ID.
func BuildGroupsListCommand(config BuildGroupsListCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	buildID := fs.String("build-id", "", "[experimental] Build ID whose TestFlight groups should be listed")
	output := shared.BindOutputFlags(fs)

	errorPrefix := strings.TrimSpace(config.ErrorPrefix)
	if errorPrefix == "" {
		errorPrefix = "builds groups list"
	}

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedBuildID := strings.TrimSpace(*buildID)
			buildIDSet := false
			fs.Visit(func(value *flag.Flag) {
				if value.Name == "build-id" {
					buildIDSet = true
				}
			})
			if buildIDSet && resolvedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-id cannot be empty")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--build-id")
			}
			if resolvedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-id is required")
				return shared.MissingRequiredUsageError("--build-id")
			}

			return runBuildGroupMembershipList(
				ctx,
				resolvedBuildID,
				"",
				nil,
				*output.Output,
				*output.Pretty,
				errorPrefix,
			)
		},
	}
}

func runBuildGroupMembershipList(
	ctx context.Context,
	buildID string,
	expectedAppID string,
	internalFilter *bool,
	output string,
	pretty bool,
	errorPrefix string,
) error {
	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("%s: %w", errorPrefix, err)
	}

	requestCtx, cancel := contextWithBuildGroupMembershipTimeout(ctx)
	defer cancel()

	result, usedFallback, lookupErr := lookupBuildGroupMembership(
		requestCtx,
		client,
		buildID,
		expectedAppID,
		internalFilter,
	)
	if usedFallback {
		fmt.Fprintln(os.Stderr, "Apple rejected the documented betaGroups build filter; falling back to inverse group build relationships (cost scales with groups and build pages)")
	}
	if result == nil {
		return fmt.Errorf("%s: %w", errorPrefix, lookupErr)
	}
	if err := shared.PrintOutput(result, output, pretty); err != nil {
		return err
	}
	if lookupErr != nil {
		fmt.Fprintf(os.Stderr, "%d group relationship lookup failed; membership result is incomplete\n", len(result.Failures))
		return shared.NewReportedError(lookupErr)
	}
	return nil
}

func contextWithBuildGroupMembershipTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithResolvedTimeout(ctx, buildGroupMembershipTimeout)
}

// BetaGroupsCreateCommand returns the beta groups create subcommand.
func BetaGroupsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	name := fs.String("name", "", "Beta group name")
	internal := fs.Bool("internal", false, "Create as internal group")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc testflight beta-groups create [flags]",
		ShortHelp:  "Create a TestFlight beta group.",
		LongHelp: `Create a TestFlight beta group.

Examples:
  asc testflight beta-groups create --app "APP_ID" --name "Beta Testers"
  asc testflight beta-groups create --app "APP_ID" --name "Internal Testers" --internal`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}
			if strings.TrimSpace(*name) == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError("--name")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BetaGroupAttributes{
				Name: strings.TrimSpace(*name),
			}
			if *internal {
				attrs.IsInternalGroup = true
			}

			group, err := client.CreateBetaGroupWithAttributes(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("beta-groups create: failed to create: %w", err)
			}

			return shared.PrintOutput(group, *output.Output, *output.Pretty)
		},
	}
}

// BetaGroupsGetCommand returns the beta groups get subcommand.
func BetaGroupsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	id := fs.String("id", "", "Beta group ID")
	legacyGroupID := shared.BindDeprecatedStringFlagAlias(fs, "group-id", "id")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-groups view [flags]",
		ShortHelp:  "View a TestFlight beta group by ID.",
		LongHelp: `View a TestFlight beta group by ID.

Examples:
  asc testflight beta-groups view --id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := legacyGroupID.Apply(id); err != nil {
				return err
			}
			if strings.TrimSpace(*id) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			group, err := client.GetBetaGroup(requestCtx, strings.TrimSpace(*id))
			if err != nil {
				return fmt.Errorf("beta-groups view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(group, *output.Output, *output.Pretty)
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
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc testflight beta-groups update [flags]",
		ShortHelp:  "Update a TestFlight beta group.",
		LongHelp: `Update a TestFlight beta group.

Examples:
  asc testflight beta-groups update --id "GROUP_ID" --name "New Name"
  asc testflight beta-groups update --id "GROUP_ID" --public-link-enabled --public-link-limit 100
  asc testflight beta-groups update --id "GROUP_ID" --feedback-enabled`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*id)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if visited["public-link-limit"] && (*publicLinkLimit < 1 || *publicLinkLimit > 10000) {
				fmt.Fprintln(os.Stderr, "Error: --public-link-limit must be between 1 and 10000")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--public-link-limit")
			}

			hasUpdates := strings.TrimSpace(*name) != "" ||
				visited["public-link-enabled"] ||
				visited["public-link-limit-enabled"] ||
				visited["public-link-limit"] ||
				visited["feedback-enabled"]
			if !hasUpdates {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("")
			}

			if visited["public-link-limit-enabled"] && *publicLinkLimitEnabled && !visited["public-link-limit"] {
				fmt.Fprintln(os.Stderr, "Error: --public-link-limit is required when enabling public link limit")
				return shared.MissingRequiredUsageError("--public-link-limit")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var publicLinkEnabledAttr *bool
			var publicLinkLimitEnabledAttr *bool
			var feedbackEnabledAttr *bool

			if visited["public-link-enabled"] {
				publicLinkEnabledAttr = publicLinkEnabled
			}
			if visited["public-link-limit-enabled"] {
				publicLinkLimitEnabledAttr = publicLinkLimitEnabled
			}
			if visited["feedback-enabled"] {
				feedbackEnabledAttr = feedbackEnabled
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
					},
				},
			}

			group, err := client.UpdateBetaGroup(requestCtx, trimmedID, req)
			if err != nil {
				return fmt.Errorf("beta-groups update: failed to update: %w", err)
			}

			return shared.PrintOutput(group, *output.Output, *output.Pretty)
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
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*id) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaGroup(requestCtx, strings.TrimSpace(*id)); err != nil {
				return fmt.Errorf("beta-groups delete: failed to delete: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully deleted group %s\n", strings.TrimSpace(*id))
			return nil
		},
	}
}

// BetaGroupsAddTestersCommand returns the beta groups add-testers subcommand.
func BetaGroupsAddTestersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("add-testers", flag.ExitOnError)

	group := fs.String("group", "", "Beta group ID")
	tester := fs.String("tester", "", "Beta tester ID(s), comma-separated")
	email := fs.String("email", "", "Beta tester email(s), comma-separated")

	return &ffcli.Command{
		Name:       "add-testers",
		ShortUsage: "asc testflight beta-groups add-testers --group \"GROUP_ID\" [--tester \"TESTER_ID[,TESTER_ID...]\" | --email \"EMAIL[,EMAIL...]\"]",
		ShortHelp:  "Add beta testers to a beta group.",
		LongHelp: `Add beta testers to a beta group.

Examples:
  asc testflight beta-groups add-testers --group "GROUP_ID" --tester "TESTER_ID"
  asc testflight beta-groups add-testers --group "GROUP_ID" --tester "TESTER_ID1,TESTER_ID2"
  asc testflight beta-groups add-testers --group "GROUP_ID" --email "tester@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupID := strings.TrimSpace(*group)
			if groupID == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return shared.MissingRequiredUsageError("--group")
			}

			testerIDs := shared.SplitCSV(*tester)
			testerEmails := shared.SplitCSV(*email)
			if len(testerIDs) == 0 && len(testerEmails) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --tester or --email is required")
				return shared.MissingRequiredUsageError("")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups add-testers: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if len(testerEmails) > 0 {
				groupApp, err := client.GetBetaGroupApp(requestCtx, groupID)
				if err != nil {
					return fmt.Errorf("beta-groups add-testers: failed to resolve app for group: %w", err)
				}
				appID := strings.TrimSpace(groupApp.Data.ID)
				if appID == "" {
					return fmt.Errorf("beta-groups add-testers: group %q has empty app ID", groupID)
				}

				for _, testerEmail := range testerEmails {
					resp, err := client.GetBetaTesters(
						requestCtx,
						appID,
						asc.WithBetaTestersEmail(testerEmail),
						asc.WithBetaTestersLimit(2),
					)
					if err != nil {
						return fmt.Errorf("beta-groups add-testers: failed to resolve tester email %q: %w", testerEmail, err)
					}
					if len(resp.Data) == 0 {
						return fmt.Errorf("beta-groups add-testers: tester email %q not found for app %q", testerEmail, appID)
					}
					if len(resp.Data) > 1 {
						return fmt.Errorf("beta-groups add-testers: multiple testers found for email %q; use --tester ID", testerEmail)
					}
					testerIDs = append(testerIDs, resp.Data[0].ID)
				}
			}

			if len(testerIDs) == 0 {
				return fmt.Errorf("beta-groups add-testers: no tester IDs resolved")
			}
			seen := make(map[string]struct{}, len(testerIDs))
			deduped := make([]string, 0, len(testerIDs))
			for _, testerID := range testerIDs {
				trimmed := strings.TrimSpace(testerID)
				if trimmed == "" {
					continue
				}
				if _, ok := seen[trimmed]; ok {
					continue
				}
				seen[trimmed] = struct{}{}
				deduped = append(deduped, trimmed)
			}
			testerIDs = deduped

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
	confirm := fs.Bool("confirm", false, "Confirm removal")

	return &ffcli.Command{
		Name:       "remove-testers",
		ShortUsage: "asc testflight beta-groups remove-testers --group \"GROUP_ID\" --tester \"TESTER_ID[,TESTER_ID...]\" --confirm",
		ShortHelp:  "Remove beta testers from a beta group.",
		LongHelp: `Remove beta testers from a beta group.

Examples:
  asc testflight beta-groups remove-testers --group "GROUP_ID" --tester "TESTER_ID" --confirm
  asc testflight beta-groups remove-testers --group "GROUP_ID" --tester "TESTER_ID1,TESTER_ID2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupID := strings.TrimSpace(*group)
			if groupID == "" {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return shared.MissingRequiredUsageError("--group")
			}

			testerIDs := shared.SplitCSV(*tester)
			if len(testerIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --tester is required")
				return shared.MissingRequiredUsageError("--tester")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-groups remove-testers: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.RemoveBetaTestersFromGroup(requestCtx, groupID, testerIDs); err != nil {
				return fmt.Errorf("beta-groups remove-testers: failed to remove testers: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully removed %d tester(s) from group %s\n", len(testerIDs), groupID)
			return nil
		},
	}
}
