package xcodecloud

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type xcodeCloudActionResourceGroupConfig struct {
	Name       string
	ShortUsage string
	ShortHelp  string
	LongHelp   string
	List       *ffcli.Command
	Get        *ffcli.Command
}

func newXcodeCloudActionResourceGroupCommand(d *shared.Deps, config xcodeCloudActionResourceGroupConfig) *ffcli.Command {
	fs := d.NewFlagSet(config.Name, flag.ExitOnError)

	return &ffcli.Command{
		Name:       config.Name,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			config.List,
			config.Get,
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

type xcodeCloudActionResourceListConfig struct {
	ShortUsage       string
	ShortHelp        string
	LongHelp         string
	ActionUsage      string
	RunUsage         string
	ErrorPrefix      string
	FetchPage        func(context.Context, *asc.Client, string, int, string) (asc.PaginatedResponse, error)
	AggregateFromRun func(context.Context, *shared.Deps, *asc.Client, string, int, bool) (asc.PaginatedResponse, error)
}

func newXcodeCloudActionResourceListCommand(d *shared.Deps, config xcodeCloudActionResourceListConfig) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	actionUsage := strings.TrimSpace(config.ActionUsage)
	if actionUsage == "" {
		actionUsage = "Build action ID to list resources for"
	}
	runUsage := strings.TrimSpace(config.RunUsage)
	if runUsage == "" {
		runUsage = "Build run ID to resolve a single action from"
	}

	actionID := shared.BindResourceIDFlag(fs, "action-id", "ciBuildActions", actionUsage)
	runID := shared.BindResourceIDFlag(fs, "run-id", "ciBuildRuns", runUsage)
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, config.ErrorPrefix, 200); err != nil {
				return err
			}
			return runXcodeCloudActionResourceList(
				ctx,
				d,
				*actionID,
				*runID,
				*limit,
				*next,
				*paginate,
				*output.Output,
				*output.Pretty,
				config.ErrorPrefix,
				config.FetchPage,
				config.AggregateFromRun,
			)
		},
	}
}

type xcodeCloudActionResourceGetConfig struct {
	ShortUsage  string
	ShortHelp   string
	LongHelp    string
	IDUsage     string
	IDType      string
	ErrorPrefix string
	Fetch       func(context.Context, *asc.Client, string) (any, error)
}

func newXcodeCloudActionResourceGetCommand(d *shared.Deps, config xcodeCloudActionResourceGetConfig) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:             "view",
		ShortUsage:       config.ShortUsage,
		ShortHelp:        config.ShortHelp,
		LongHelp:         config.LongHelp,
		IDFlag:           "id",
		IDUsage:          config.IDUsage,
		IDType:           config.IDType,
		ErrorPrefix:      config.ErrorPrefix,
		FetchErrorPrefix: config.ErrorPrefix,
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		Fetch: config.Fetch,
	})
}

type xcodeCloudActionResourceCommandConfig struct {
	Name       string
	ShortUsage string
	ShortHelp  string
	LongHelp   string

	ListShortUsage       string
	ListShortHelp        string
	ListLongHelp         string
	ListActionUsage      string
	ListRunUsage         string
	ListErrorPrefix      string
	ListFetchPage        func(context.Context, *asc.Client, string, int, string) (asc.PaginatedResponse, error)
	ListAggregateFromRun func(context.Context, *shared.Deps, *asc.Client, string, int, bool) (asc.PaginatedResponse, error)

	GetShortUsage  string
	GetShortHelp   string
	GetLongHelp    string
	GetIDUsage     string
	GetIDType      string
	GetErrorPrefix string
	GetFetch       func(context.Context, *asc.Client, string) (any, error)
}

func newXcodeCloudActionResourceCommand(d *shared.Deps, config xcodeCloudActionResourceCommandConfig) *ffcli.Command {
	return newXcodeCloudActionResourceGroupCommand(d, xcodeCloudActionResourceGroupConfig{
		Name:       config.Name,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		List: newXcodeCloudActionResourceListCommand(d, xcodeCloudActionResourceListConfig{
			ShortUsage:       config.ListShortUsage,
			ShortHelp:        config.ListShortHelp,
			LongHelp:         config.ListLongHelp,
			ActionUsage:      config.ListActionUsage,
			RunUsage:         config.ListRunUsage,
			ErrorPrefix:      config.ListErrorPrefix,
			FetchPage:        config.ListFetchPage,
			AggregateFromRun: config.ListAggregateFromRun,
		}),
		Get: newXcodeCloudActionResourceGetCommand(d, xcodeCloudActionResourceGetConfig{
			ShortUsage:  config.GetShortUsage,
			ShortHelp:   config.GetShortHelp,
			LongHelp:    config.GetLongHelp,
			IDUsage:     config.GetIDUsage,
			IDType:      config.GetIDType,
			ErrorPrefix: config.GetErrorPrefix,
			Fetch:       config.GetFetch,
		}),
	})
}

var xcodeCloudIssuesCommandConfig = xcodeCloudActionResourceCommandConfig{
	Name:       "issues",
	ShortUsage: "asc xcode-cloud issues <subcommand> [flags]",
	ShortHelp:  "List Xcode Cloud build issues.",
	LongHelp: `List Xcode Cloud build issues.

Examples:
  asc xcode-cloud issues list --action-id "ACTION_ID"
  asc xcode-cloud issues list --run-id "BUILD_RUN_ID"
  asc xcode-cloud issues view --id "ISSUE_ID"`,
	ListShortUsage: "asc xcode-cloud issues list [flags]",
	ListShortHelp:  "List issues for a build action.",
	ListLongHelp: `List issues for a build action.

Examples:
  asc xcode-cloud issues list --action-id "ACTION_ID"
  asc xcode-cloud issues list --run-id "BUILD_RUN_ID"
  asc xcode-cloud issues list --action-id "ACTION_ID" --output table
  asc xcode-cloud issues list --action-id "ACTION_ID" --limit 50
  asc xcode-cloud issues list --action-id "ACTION_ID" --paginate`,
	ListActionUsage: "Build action ID to list issues for",
	ListRunUsage:    "Build run ID to resolve a single issue action from",
	ListErrorPrefix: "xcode-cloud issues list",
	ListFetchPage: func(ctx context.Context, client *asc.Client, actionID string, limit int, next string) (asc.PaginatedResponse, error) {
		return client.GetCiBuildActionIssues(
			ctx, actionID,
			asc.Limit[asc.CiIssuesOption](limit),
			asc.NextURL[asc.CiIssuesOption](next),
		)
	},
	ListAggregateFromRun: aggregateXcodeCloudIssuesFromRun,
	GetShortUsage:        "asc xcode-cloud issues view --id \"ISSUE_ID\"",
	GetShortHelp:         "View details for a build issue.",
	GetLongHelp: `View details for a build issue.

Examples:
  asc xcode-cloud issues view --id "ISSUE_ID"
  asc xcode-cloud issues view --id "ISSUE_ID" --output table`,
	GetIDUsage:     "Issue ID",
	GetIDType:      "ciIssues",
	GetErrorPrefix: "xcode-cloud issues view",
	GetFetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
		return client.GetCiIssue(ctx, id)
	},
}

var xcodeCloudTestResultsCommandConfig = xcodeCloudActionResourceCommandConfig{
	Name:       "test-results",
	ShortUsage: "asc xcode-cloud test-results <subcommand> [flags]",
	ShortHelp:  "List Xcode Cloud test results.",
	LongHelp: `List Xcode Cloud test results.

Examples:
  asc xcode-cloud test-results list --action-id "ACTION_ID"
  asc xcode-cloud test-results list --run-id "BUILD_RUN_ID"
  asc xcode-cloud test-results view --id "TEST_RESULT_ID"`,
	ListShortUsage: "asc xcode-cloud test-results list [flags]",
	ListShortHelp:  "List test results for a build action.",
	ListLongHelp: `List test results for a build action.

Examples:
  asc xcode-cloud test-results list --action-id "ACTION_ID"
  asc xcode-cloud test-results list --run-id "BUILD_RUN_ID"
  asc xcode-cloud test-results list --action-id "ACTION_ID" --output table
  asc xcode-cloud test-results list --action-id "ACTION_ID" --limit 50
  asc xcode-cloud test-results list --action-id "ACTION_ID" --paginate`,
	ListActionUsage: "Build action ID to list test results for",
	ListRunUsage:    "Build run ID to resolve a single test-result action from",
	ListErrorPrefix: "xcode-cloud test-results list",
	ListFetchPage: func(ctx context.Context, client *asc.Client, actionID string, limit int, next string) (asc.PaginatedResponse, error) {
		return client.GetCiBuildActionTestResults(
			ctx, actionID,
			asc.Limit[asc.CiTestResultsOption](limit),
			asc.NextURL[asc.CiTestResultsOption](next),
		)
	},
	ListAggregateFromRun: aggregateXcodeCloudTestResultsFromRun,
	GetShortUsage:        "asc xcode-cloud test-results view --id \"TEST_RESULT_ID\"",
	GetShortHelp:         "View details for a test result.",
	GetLongHelp: `View details for a test result.

Examples:
  asc xcode-cloud test-results view --id "TEST_RESULT_ID"
  asc xcode-cloud test-results view --id "TEST_RESULT_ID" --output table`,
	GetIDUsage:     "Test result ID",
	GetIDType:      "ciTestResults",
	GetErrorPrefix: "xcode-cloud test-results view",
	GetFetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
		return client.GetCiTestResult(ctx, id)
	},
}

func runXcodeCloudActionResourceList(
	ctx context.Context,
	d *shared.Deps,
	actionID string,
	runID string,
	limit int,
	next string,
	paginate bool,
	output string,
	pretty bool,
	errorPrefix string,
	fetchPage func(context.Context, *asc.Client, string, int, string) (asc.PaginatedResponse, error),
	aggregateFromRun func(context.Context, *shared.Deps, *asc.Client, string, int, bool) (asc.PaginatedResponse, error),
) error {
	if strings.TrimSpace(actionID) != "" && strings.TrimSpace(runID) != "" {
		return d.UsageError("--action-id and --run-id are mutually exclusive")
	}
	nextURL := strings.TrimSpace(next)

	resolvedActionID := strings.TrimSpace(actionID)
	resolvedRunID := strings.TrimSpace(runID)
	if resolvedActionID == "" && resolvedRunID == "" && nextURL == "" {
		return d.UsageError("--action-id or --run-id is required")
	}
	if resolvedRunID != "" && nextURL != "" {
		return d.UsageError("--next is not supported with --run-id; use --action-id with a links.next URL")
	}

	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("%s: %w", errorPrefix, err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(d, ctx, 0)
	defer cancel()

	if nextURL == "" && resolvedRunID != "" && resolvedActionID == "" && aggregateFromRun != nil {
		resp, err := aggregateFromRun(requestCtx, d, client, resolvedRunID, limit, paginate)
		if err != nil {
			return fmt.Errorf("%s: %w", errorPrefix, err)
		}
		return d.PrintOutput(resp, output, pretty)
	}

	if nextURL == "" && resolvedActionID == "" {
		resolvedActionID, err = resolveSingleBuildActionIDForRun(requestCtx, d, client, resolvedRunID)
		if err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return err
			}
			return fmt.Errorf("%s: %w", errorPrefix, err)
		}
	}

	if paginate {
		resp, err := d.PaginateWithSpinner(
			requestCtx,
			func(ctx context.Context) (asc.PaginatedResponse, error) {
				return fetchPage(ctx, client, resolvedActionID, 200, nextURL)
			},
			func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return fetchPage(ctx, client, resolvedActionID, 0, nextURL)
			},
		)
		if err != nil {
			return fmt.Errorf("%s: %w", errorPrefix, err)
		}
		return d.PrintOutput(resp, output, pretty)
	}

	resp, err := fetchPage(requestCtx, client, resolvedActionID, limit, nextURL)
	if err != nil {
		return fmt.Errorf("%s: %w", errorPrefix, err)
	}

	return d.PrintOutput(resp, output, pretty)
}

func resolveSingleBuildActionIDForRun(ctx context.Context, d *shared.Deps, client *asc.Client, runID string) (string, error) {
	actions, err := listBuildActionsForRun(ctx, d, client, runID)
	if err != nil {
		return "", err
	}

	if len(actions) > 1 {
		return "", d.UsageErrorf("--run-id %q matched multiple build actions; use --action-id or inspect asc xcode-cloud actions --run-id %q", runID, runID)
	}

	return strings.TrimSpace(actions[0].ID), nil
}

func listBuildActionsForRun(ctx context.Context, d *shared.Deps, client *asc.Client, runID string) ([]asc.CiBuildActionResource, error) {
	actions, err := listBuildActionsForRunAllowEmpty(ctx, client, runID)
	if err != nil {
		return nil, err
	}
	if len(actions) == 0 {
		return nil, d.UsageErrorf("no build actions found for --run-id %q", runID)
	}
	return actions, nil
}

func listBuildActionsForRunAllowEmpty(ctx context.Context, client *asc.Client, runID string) ([]asc.CiBuildActionResource, error) {
	resp, err := client.GetCiBuildActions(ctx, runID, asc.Limit[asc.CiBuildActionsOption](200))
	if err != nil {
		return nil, fmt.Errorf("resolve build actions for run %q: %w", runID, err)
	}
	if strings.TrimSpace(resp.Links.Next) == "" {
		return resp.Data, nil
	}

	allPages, err := asc.PaginateAll(ctx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetCiBuildActions(ctx, runID, asc.NextURL[asc.CiBuildActionsOption](nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("resolve build actions for run %q: %w", runID, err)
	}

	allActions, ok := allPages.(*asc.CiBuildActionsResponse)
	if !ok {
		return nil, fmt.Errorf("resolve build actions for run %q: unexpected response type %T", runID, allPages)
	}
	return allActions.Data, nil
}

func matchingBuildActionIDsByType(actions []asc.CiBuildActionResource, actionType string) []string {
	ids := make([]string, 0, len(actions))
	for _, action := range actions {
		if !strings.EqualFold(strings.TrimSpace(action.Attributes.ActionType), actionType) {
			continue
		}

		id := strings.TrimSpace(action.ID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func aggregateXcodeCloudIssuesFromRun(ctx context.Context, d *shared.Deps, client *asc.Client, runID string, limit int, paginate bool) (asc.PaginatedResponse, error) {
	actions, err := listBuildActionsForRun(ctx, d, client, runID)
	if err != nil {
		return nil, err
	}

	combined := &asc.CiIssuesResponse{Data: make([]asc.CiIssueResource, 0)}
	remaining := limit
	for _, action := range actions {
		actionID := strings.TrimSpace(action.ID)
		if actionID == "" {
			continue
		}

		pageLimit := remaining
		if paginate {
			pageLimit = 200
		}
		resp, err := client.GetCiBuildActionIssues(ctx, actionID, asc.Limit[asc.CiIssuesOption](pageLimit))
		if err != nil {
			return nil, fmt.Errorf("list issues for action %q: %w", actionID, err)
		}
		if paginate {
			allPages, err := asc.PaginateAll(ctx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetCiBuildActionIssues(ctx, actionID, asc.NextURL[asc.CiIssuesOption](nextURL))
			})
			if err != nil {
				return nil, fmt.Errorf("list issues for action %q: %w", actionID, err)
			}
			resp = allPages.(*asc.CiIssuesResponse)
		}

		combined.Data = append(combined.Data, resp.Data...)
		if !paginate && limit > 0 && len(combined.Data) >= limit {
			combined.Data = combined.Data[:limit]
			break
		}
		if !paginate && limit > 0 {
			remaining = limit - len(combined.Data)
		}
	}

	return combined, nil
}

func aggregateXcodeCloudTestResultsFromRun(ctx context.Context, d *shared.Deps, client *asc.Client, runID string, limit int, paginate bool) (asc.PaginatedResponse, error) {
	actions, err := listBuildActionsForRun(ctx, d, client, runID)
	if err != nil {
		return nil, err
	}

	testActionIDs := matchingBuildActionIDsByType(actions, "TEST")
	if len(testActionIDs) == 0 {
		return nil, d.UsageErrorf("no TEST build actions found for --run-id %q", runID)
	}

	combined := &asc.CiTestResultsResponse{Data: make([]asc.CiTestResultResource, 0)}
	remaining := limit
	for _, actionID := range testActionIDs {
		pageLimit := remaining
		if paginate {
			pageLimit = 200
		}
		resp, err := client.GetCiBuildActionTestResults(ctx, actionID, asc.Limit[asc.CiTestResultsOption](pageLimit))
		if err != nil {
			return nil, fmt.Errorf("list test results for action %q: %w", actionID, err)
		}
		if paginate {
			allPages, err := asc.PaginateAll(ctx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetCiBuildActionTestResults(ctx, actionID, asc.NextURL[asc.CiTestResultsOption](nextURL))
			})
			if err != nil {
				return nil, fmt.Errorf("list test results for action %q: %w", actionID, err)
			}
			resp = allPages.(*asc.CiTestResultsResponse)
		}

		combined.Data = append(combined.Data, resp.Data...)
		if !paginate && limit > 0 && len(combined.Data) >= limit {
			combined.Data = combined.Data[:limit]
			break
		}
		if !paginate && limit > 0 {
			remaining = limit - len(combined.Data)
		}
	}

	return combined, nil
}

func aggregateXcodeCloudArtifactsFromRun(ctx context.Context, d *shared.Deps, client *asc.Client, runID string, limit int, paginate bool) (asc.PaginatedResponse, error) {
	actions, err := listBuildActionsForRun(ctx, d, client, runID)
	if err != nil {
		return nil, err
	}

	combined := &asc.CiArtifactsResponse{Data: make([]asc.CiArtifactResource, 0)}
	remaining := limit
	for _, action := range actions {
		actionID := strings.TrimSpace(action.ID)
		if actionID == "" {
			continue
		}

		pageLimit := remaining
		if paginate {
			pageLimit = 200
		}
		resp, err := client.GetCiBuildActionArtifacts(ctx, actionID, asc.Limit[asc.CiArtifactsOption](pageLimit))
		if err != nil {
			return nil, fmt.Errorf("list artifacts for action %q: %w", actionID, err)
		}
		if paginate {
			allPages, err := asc.PaginateAll(ctx, resp, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetCiBuildActionArtifacts(ctx, actionID, asc.NextURL[asc.CiArtifactsOption](nextURL))
			})
			if err != nil {
				return nil, fmt.Errorf("list artifacts for action %q: %w", actionID, err)
			}
			resp = allPages.(*asc.CiArtifactsResponse)
		}

		combined.Data = append(combined.Data, resp.Data...)
		if !paginate && limit > 0 && len(combined.Data) >= limit {
			combined.Data = combined.Data[:limit]
			break
		}
		if !paginate && limit > 0 {
			remaining = limit - len(combined.Data)
		}
	}

	return combined, nil
}
