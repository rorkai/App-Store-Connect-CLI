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
	appID = fs.String("app", "", xcodeCloudAppFlagUsage)
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
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "get",
		Name:        "get",
		ShortUsage:  "asc xcode-cloud workflows get --id \"WORKFLOW_ID\"",
		ShortHelp:   "Get details for a workflow.",
		LongHelp: `Get details for a workflow.

Examples:
  asc xcode-cloud workflows get --id "WORKFLOW_ID"
  asc xcode-cloud workflows get --id "WORKFLOW_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "Workflow ID",
		ErrorPrefix: "xcode-cloud workflows get",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiWorkflow(ctx, id)
		},
	})
}

func XcodeCloudWorkflowsRepositoryCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "repository",
		Name:        "repository",
		ShortUsage:  "asc xcode-cloud workflows repository --id \"WORKFLOW_ID\"",
		ShortHelp:   "Get the repository for a workflow.",
		LongHelp: `Get the repository for a workflow.

Examples:
  asc xcode-cloud workflows repository --id "WORKFLOW_ID"
  asc xcode-cloud workflows repository --id "WORKFLOW_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "Workflow ID",
		ErrorPrefix: "xcode-cloud workflows repository",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			repo, err := client.GetCiWorkflowRepository(ctx, id)
			if err != nil {
				return nil, err
			}
			return &asc.ScmRepositoriesResponse{Data: []asc.ScmRepositoryResource{*repo}}, nil
		},
	})
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
				return shared.MissingRequiredUsageError()
			}

			payload, err := shared.ReadJSONFilePayload(fileValue)
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
				return shared.MissingRequiredUsageError()
			}
			fileValue := strings.TrimSpace(*file)
			if fileValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return shared.MissingRequiredUsageError()
			}

			payload, err := shared.ReadJSONFilePayload(fileValue)
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
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc xcode-cloud workflows delete --id \"WORKFLOW_ID\" --confirm",
		ShortHelp:   "Delete a workflow.",
		LongHelp: `Delete a workflow.

Examples:
  asc xcode-cloud workflows delete --id "WORKFLOW_ID" --confirm`,
		IDFlag:      "id",
		IDUsage:     "Workflow ID",
		ErrorPrefix: "xcode-cloud workflows delete",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			if err := client.DeleteCiWorkflow(ctx, id); err != nil {
				return fmt.Errorf("failed to delete: %w", err)
			}
			return nil
		},
		Result: func(id string) any {
			return &asc.CiWorkflowDeleteResult{ID: id, Deleted: true}
		},
	})
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
		return shared.MissingRequiredUsageError()
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("xcode-cloud workflows: %w", err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
	defer cancel()

	productID := ""
	if nextURL == "" && resolvedAppID != "" {
		resolvedAppID, err = resolveXcodeCloudAppID(requestCtx, client, resolvedAppID)
		if err != nil {
			return fmt.Errorf("xcode-cloud workflows: %w", err)
		}

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
		resp, err := shared.PaginateWithSpinner(
			requestCtx,
			func(ctx context.Context) (asc.PaginatedResponse, error) {
				return client.GetCiWorkflows(ctx, productID, paginateOpts...)
			},
			func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetCiWorkflows(ctx, productID, asc.WithCiWorkflowsNextURL(nextURL))
			},
		)
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
