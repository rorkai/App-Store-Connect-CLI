package xcodecloud

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func xcodeCloudWorkflowsListFlags(fs *flag.FlagSet) (appID *string, limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	appID = fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
	next = fs.String("next", "", "Fetch next page using a links.next URL")
	paginate = fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	outputFlags := shared.BindOutputFlags(fs)
	output = outputFlags.Output
	pretty = outputFlags.Pretty
	return
}

// XcodeCloudWorkflowsCommand returns the xcode-cloud workflows subcommand.
func XcodeCloudWorkflowsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("workflows", flag.ExitOnError)

	appID, limit, next, paginate, output, pretty := xcodeCloudWorkflowsListFlags(fs)

	return &ffcli.Command{
		Name:       "workflows",
		ShortUsage: "asc xcode-cloud workflows [flags]",
		ShortHelp:  "Manage Xcode Cloud workflows.",
		LongHelp: `Manage Xcode Cloud workflows.

Examples:
  asc xcode-cloud workflows --app "APP_ID"
  asc xcode-cloud workflows list --app "APP_ID"
  asc xcode-cloud workflows get --id "WORKFLOW_ID"
  asc xcode-cloud workflows repository --id "WORKFLOW_ID"
  asc xcode-cloud workflows --app "APP_ID" --limit 50
  asc xcode-cloud workflows --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudWorkflowsListCommand(),
			XcodeCloudWorkflowsGetCommand(),
			XcodeCloudWorkflowsRepositoryCommand(),
			XcodeCloudWorkflowsCreateCommand(),
			XcodeCloudWorkflowsUpdateCommand(),
			XcodeCloudWorkflowsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudWorkflowsList(ctx, *appID, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudWorkflowsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID, limit, next, paginate, output, pretty := xcodeCloudWorkflowsListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud workflows list [flags]",
		ShortHelp:  "List Xcode Cloud workflows for an app.",
		LongHelp: `List Xcode Cloud workflows for an app.

Examples:
  asc xcode-cloud workflows list --app "APP_ID"
  asc xcode-cloud workflows list --app "APP_ID" --limit 50
  asc xcode-cloud workflows list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudWorkflowsList(ctx, *appID, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudWorkflowsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Workflow ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud workflows get --id \"WORKFLOW_ID\"",
		ShortHelp:  "Get details for a workflow.",
		LongHelp: `Get details for a workflow.

Examples:
  asc xcode-cloud workflows get --id "WORKFLOW_ID"
  asc xcode-cloud workflows get --id "WORKFLOW_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetCiWorkflow(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudWorkflowsRepositoryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("repository", flag.ExitOnError)

	id := fs.String("id", "", "Workflow ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "repository",
		ShortUsage: "asc xcode-cloud workflows repository --id \"WORKFLOW_ID\"",
		ShortHelp:  "Get the repository for a workflow.",
		LongHelp: `Get the repository for a workflow.

Examples:
  asc xcode-cloud workflows repository --id "WORKFLOW_ID"
  asc xcode-cloud workflows repository --id "WORKFLOW_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows repository: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			repo, err := client.GetCiWorkflowRepository(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows repository: %w", err)
			}

			resp := &asc.ScmRepositoriesResponse{Data: []asc.ScmRepositoryResource{*repo}}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudWorkflowsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	file := fs.String("file", "", "Path to workflow JSON payload")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc xcode-cloud workflows create --file ./workflow.json",
		ShortHelp:  "Create a workflow.",
		LongHelp: `Create a workflow.

Examples:
  asc xcode-cloud workflows create --file ./workflow.json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			fileValue := strings.TrimSpace(*file)
			if fileValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			payload, err := readJSONFilePayload(fileValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows create: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows create: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.CreateCiWorkflow(requestCtx, payload)
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudWorkflowsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Workflow ID")
	file := fs.String("file", "", "Path to workflow JSON payload")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc xcode-cloud workflows update --id \"WORKFLOW_ID\" --file ./workflow.json",
		ShortHelp:  "Update a workflow.",
		LongHelp: `Update a workflow.

Examples:
  asc xcode-cloud workflows update --id "WORKFLOW_ID" --file ./workflow.json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			fileValue := strings.TrimSpace(*file)
			if fileValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			payload, err := readJSONFilePayload(fileValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows update: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows update: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.UpdateCiWorkflow(requestCtx, idValue, payload)
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudWorkflowsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Workflow ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc xcode-cloud workflows delete --id \"WORKFLOW_ID\" --confirm",
		ShortHelp:  "Delete a workflow.",
		LongHelp: `Delete a workflow.

Examples:
  asc xcode-cloud workflows delete --id "WORKFLOW_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud workflows delete: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			if err := client.DeleteCiWorkflow(requestCtx, idValue); err != nil {
				return fmt.Errorf("xcode-cloud workflows delete: failed to delete: %w", err)
			}

			result := &asc.CiWorkflowDeleteResult{ID: idValue, Deleted: true}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func xcodeCloudWorkflowsList(ctx context.Context, appID string, limit int, next string, paginate bool, output string, pretty bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return fmt.Errorf("xcode-cloud workflows: --limit must be between 1 and 200")
	}
	nextURL := strings.TrimSpace(next)
	if err := shared.ValidateNextURL(nextURL); err != nil {
		return fmt.Errorf("xcode-cloud workflows: %w", err)
	}

	resolvedAppID := shared.ResolveAppID(appID)
	if resolvedAppID == "" && nextURL == "" {
		fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
		return flag.ErrHelp
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("xcode-cloud workflows: %w", err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
	defer cancel()

	productID := ""
	if nextURL == "" && resolvedAppID != "" {
		product, err := client.ResolveCiProductForApp(requestCtx, resolvedAppID)
		if err != nil {
			return fmt.Errorf("xcode-cloud workflows: %w", err)
		}
		productID = product.ID
	}

	opts := []asc.CiWorkflowsOption{
		asc.WithCiWorkflowsLimit(limit),
		asc.WithCiWorkflowsNextURL(nextURL),
	}

	if paginate {
		paginateOpts := append(opts, asc.WithCiWorkflowsLimit(200))
		firstPage, err := client.GetCiWorkflows(requestCtx, productID, paginateOpts...)
		if err != nil {
			return fmt.Errorf("xcode-cloud workflows: failed to fetch: %w", err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetCiWorkflows(ctx, productID, asc.WithCiWorkflowsNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("xcode-cloud workflows: %w", err)
		}

		return shared.PrintOutput(resp, output, pretty)
	}

	resp, err := client.GetCiWorkflows(requestCtx, productID, opts...)
	if err != nil {
		return fmt.Errorf("xcode-cloud workflows: %w", err)
	}

	return shared.PrintOutput(resp, output, pretty)
}
