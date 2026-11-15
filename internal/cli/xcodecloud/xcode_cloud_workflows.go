package xcodecloud

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// XcodeCloudWorkflowsCommand returns the xcode-cloud workflows subcommand.
func XcodeCloudWorkflowsCommand() *ffcli.Command {
	cmd := xcodeCloudWorkflowsListCommand(
		"workflows",
		"asc xcode-cloud workflows [flags]",
		"Manage Xcode Cloud workflows.",
		`Manage Xcode Cloud workflows.

Examples:
  asc xcode-cloud workflows --app "APP_ID"
  asc xcode-cloud workflows list --app "APP_ID"
  asc xcode-cloud workflows view --id "WORKFLOW_ID"
  asc xcode-cloud workflows repository --id "WORKFLOW_ID"
  asc xcode-cloud workflows duplicate --id "WORKFLOW_ID" --name "Nightly"
  asc xcode-cloud workflows --app "APP_ID" --limit 50
  asc xcode-cloud workflows --app "APP_ID" --paginate`,
	)
	cmd.Subcommands = []*ffcli.Command{
		XcodeCloudWorkflowsListCommand(),
		XcodeCloudWorkflowsGetCommand(),
		XcodeCloudWorkflowsRepositoryCommand(),
		XcodeCloudWorkflowsCreateCommand(),
		XcodeCloudWorkflowsUpdateCommand(),
		XcodeCloudWorkflowsDuplicateCommand(),
		XcodeCloudWorkflowsDeleteCommand(),
	}
	return cmd
}

func XcodeCloudWorkflowsListCommand() *ffcli.Command {
	return xcodeCloudWorkflowsListCommand(
		"list",
		"asc xcode-cloud workflows list [flags]",
		"List Xcode Cloud workflows for an app.",
		`List Xcode Cloud workflows for an app.

Examples:
  asc xcode-cloud workflows list --app "APP_ID"
  asc xcode-cloud workflows list --app "APP_ID" --limit 50
  asc xcode-cloud workflows list --app "APP_ID" --paginate`,
	)
}

func xcodeCloudWorkflowsListCommand(name, shortUsage, shortHelp, longHelp string) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:                 name,
		ShortUsage:           shortUsage,
		ShortHelp:            shortHelp,
		LongHelp:             longHelp,
		ParentFlag:           "app",
		ParentUsage:          xcodeCloudAppFlagUsage,
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		ErrorPrefix:          "xcode-cloud workflows",
		FetchErrorPrefix:     "xcode-cloud workflows",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		LookupParent: func(ctx context.Context, client *asc.Client, appID string) (string, error) {
			resolvedAppID, err := resolveXcodeCloudAppID(ctx, client, appID)
			if err != nil {
				return "", fmt.Errorf("xcode-cloud workflows: %w", err)
			}
			product, err := client.ResolveCiProductForApp(ctx, resolvedAppID)
			if err != nil {
				return "", classifyXcodeCloudLookupError(fmt.Errorf("xcode-cloud workflows: %w", err))
			}
			return product.ID, nil
		},
		FetchPage: func(ctx context.Context, client *asc.Client, productID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetCiWorkflows(ctx, productID, asc.Limit[asc.CiWorkflowsOption](limit), asc.NextURL[asc.CiWorkflowsOption](next))
		},
	})
}

func XcodeCloudWorkflowsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc xcode-cloud workflows view --id \"WORKFLOW_ID\"",
		ShortHelp:  "View details for a workflow.",
		LongHelp: `View details for a workflow.

Examples:
  asc xcode-cloud workflows view --id "WORKFLOW_ID"
  asc xcode-cloud workflows view --id "WORKFLOW_ID" --output table`,
		IDFlag:           "id",
		IDUsage:          "Workflow ID",
		IDType:           "ciWorkflows",
		FetchErrorPrefix: "xcode-cloud workflows view",
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
		Name:       "repository",
		ShortUsage: "asc xcode-cloud workflows repository --id \"WORKFLOW_ID\"",
		ShortHelp:  "Get the repository for a workflow.",
		LongHelp: `Get the repository for a workflow.

Examples:
  asc xcode-cloud workflows repository --id "WORKFLOW_ID"
  asc xcode-cloud workflows repository --id "WORKFLOW_ID" --output table`,
		IDFlag:           "id",
		IDUsage:          "Workflow ID",
		IDType:           "ciWorkflows",
		FetchErrorPrefix: "xcode-cloud workflows repository",
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
				return shared.RequiredFlag("--file")
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

	id := shared.BindResourceIDFlag(fs, "id", "ciWorkflows", "Workflow ID")
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
				return shared.RequiredFlag("--id")
			}
			fileValue := strings.TrimSpace(*file)
			if fileValue == "" {
				return shared.RequiredFlag("--file")
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
		Name:       "delete",
		ShortUsage: "asc xcode-cloud workflows delete --id \"WORKFLOW_ID\" --confirm",
		ShortHelp:  "Delete a workflow.",
		LongHelp: `Delete a workflow.

Examples:
  asc xcode-cloud workflows delete --id "WORKFLOW_ID" --confirm`,
		IDFlag:            "id",
		IDUsage:           "Workflow ID",
		IDType:            "ciWorkflows",
		DeleteErrorPrefix: "xcode-cloud workflows delete",
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
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}
