package web

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func webXcodeCloudEnvVarsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud env-vars", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "env-vars",
		ShortUsage: "asc web xcode-cloud env-vars <subcommand> [flags]",
		ShortHelp:  "[experimental] Manage Xcode Cloud environment variables.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Manage environment variables on Xcode Cloud workflows and products
using Apple's private CI API. Requires a web session.

Use list/set/delete for workflow-scoped variables.
Use "shared" subcommand for product-level shared variables.

` + webWarningText + `

Examples:
  asc web xcode-cloud env-vars list --product-id "UUID" --workflow-id "WF-UUID" --apple-id "user@example.com"
  asc web xcode-cloud env-vars set --product-id "UUID" --workflow-id "WF-UUID" --name MY_VAR --value hello --apple-id "user@example.com"
  asc web xcode-cloud env-vars set --product-id "UUID" --workflow-id "WF-UUID" --name MY_SECRET --value s3cret --secret --apple-id "user@example.com"
  asc web xcode-cloud env-vars delete --product-id "UUID" --workflow-id "WF-UUID" --name MY_VAR --confirm --apple-id "user@example.com"
  asc web xcode-cloud env-vars shared list --product-id "UUID" --apple-id "user@example.com"
  asc web xcode-cloud env-vars shared set --product-id "UUID" --name MY_VAR --value hello --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			webXcodeCloudEnvVarsListCommand(),
			webXcodeCloudEnvVarsSetCommand(),
			webXcodeCloudEnvVarsDeleteCommand(),
			webXcodeCloudEnvVarsSharedCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CIEnvVarsListResult is the output type for the env-vars list command.
type CIEnvVarsListResult struct {
	WorkflowID string                          `json:"workflow_id"`
	Variables  []webcore.CIEnvironmentVariable `json:"variables"`
}

// CIEnvVarsSetResult is the output type for the env-vars set command.
type CIEnvVarsSetResult struct {
	WorkflowID   string `json:"workflow_id"`
	WorkflowName string `json:"workflow_name"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Action       string `json:"action"`
}

// CIEnvVarsDeleteResult is the output type for the env-vars delete command.
type CIEnvVarsDeleteResult struct {
	WorkflowID   string `json:"workflow_id"`
	WorkflowName string `json:"workflow_name"`
	Name         string `json:"name"`
}

func webXcodeCloudEnvVarsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud env-vars list", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	workflowID := fs.String("workflow-id", "", "Xcode Cloud workflow ID (required)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web xcode-cloud env-vars list --product-id ID --workflow-id ID [flags]",
		ShortHelp:  "[experimental] List workflow environment variables.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

List environment variables for an Xcode Cloud workflow.
Plaintext variables show their values; secret variables show "(redacted)".

` + webWarningText + `

Examples:
  asc web xcode-cloud env-vars list --product-id "UUID" --workflow-id "WF-UUID" --apple-id "user@example.com"
  asc web xcode-cloud env-vars list --product-id "UUID" --workflow-id "WF-UUID" --apple-id "user@example.com" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return flag.ErrHelp
			}
			wfID := strings.TrimSpace(*workflowID)
			if wfID == "" {
				fmt.Fprintln(os.Stderr, "Error: --workflow-id is required")
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud env-vars list failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			result := &CIEnvVarsListResult{}
			err = withWebSpinner("Loading Xcode Cloud workflow environment variables", func() error {
				workflow, err := client.GetCIWorkflow(requestCtx, teamID, pid, wfID)
				if err != nil {
					return err
				}
				vars, err := webcore.ExtractEnvVars(workflow.Content)
				if err != nil {
					return fmt.Errorf("xcode-cloud env-vars list failed: %w", err)
				}

				result = &CIEnvVarsListResult{
					WorkflowID: wfID,
					Variables:  vars,
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud env-vars list")
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderEnvVarsTable(result) },
				func() error { return renderEnvVarsMarkdown(result) },
			)
		},
	}
}

func webXcodeCloudEnvVarsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud env-vars set", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	workflowID := fs.String("workflow-id", "", "Xcode Cloud workflow ID (required)")
	name := fs.String("name", "", "Environment variable name (required)")
	value := fs.String("value", "", "Environment variable value (required)")
	secret := fs.Bool("secret", false, "Encrypt the value as a secret")

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web xcode-cloud env-vars set --product-id ID --workflow-id ID --name NAME --value VALUE [--secret] [flags]",
		ShortHelp:  "[experimental] Set a workflow environment variable.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Set (create or update) an environment variable on an Xcode Cloud workflow.
Use --secret to encrypt the value using ECIES (the same scheme as the ASC web UI).
If a variable with the same name already exists, it will be updated.

` + webWarningText + `

Examples:
  asc web xcode-cloud env-vars set --product-id "UUID" --workflow-id "WF-UUID" --name MY_VAR --value hello --apple-id "user@example.com"
  asc web xcode-cloud env-vars set --product-id "UUID" --workflow-id "WF-UUID" --name MY_SECRET --value s3cret --secret --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return flag.ErrHelp
			}
			wfID := strings.TrimSpace(*workflowID)
			if wfID == "" {
				fmt.Fprintln(os.Stderr, "Error: --workflow-id is required")
				return flag.ErrHelp
			}
			varName := strings.TrimSpace(*name)
			if varName == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}
			varValue := *value
			if varValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --value is required")
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud env-vars set failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			result := &CIEnvVarsSetResult{}
			err = withWebSpinner("Updating Xcode Cloud workflow environment variables", func() error {
				workflow, err := client.GetCIWorkflow(requestCtx, teamID, pid, wfID)
				if err != nil {
					return err
				}
				vars, err := webcore.ExtractEnvVars(workflow.Content)
				if err != nil {
					return fmt.Errorf("xcode-cloud env-vars set failed: %w", err)
				}

				var envVar webcore.CIEnvironmentVariable
				envVar.Name = varName

				if *secret {
					keyResp, err := client.GetCIEncryptionKey(requestCtx)
					if err != nil {
						return fmt.Errorf("xcode-cloud env-vars set failed: could not fetch encryption key: %w", err)
					}
					ct, err := webcore.ECIESEncrypt(keyResp.Key, varValue)
					if err != nil {
						return fmt.Errorf("xcode-cloud env-vars set failed: encryption error: %w", err)
					}
					envVar.Value = webcore.CIEnvironmentVariableValue{Ciphertext: &ct}
				} else {
					envVar.Value = webcore.CIEnvironmentVariableValue{Plaintext: &varValue}
				}

				found := false
				for i, v := range vars {
					if strings.EqualFold(v.Name, varName) {
						envVar.ID = v.ID
						vars[i] = envVar
						found = true
						break
					}
				}
				if !found {
					envVar.ID = newUUID()
					vars = append(vars, envVar)
				}

				newContent, err := webcore.SetEnvVars(workflow.Content, vars)
				if err != nil {
					return fmt.Errorf("xcode-cloud env-vars set failed: %w", err)
				}
				if err := client.UpdateCIWorkflow(requestCtx, teamID, pid, wfID, newContent); err != nil {
					return err
				}

				varType := "plaintext"
				if *secret {
					varType = "secret"
				}
				action := "created"
				if found {
					action = "updated"
				}
				wfName := extractWorkflowName(workflow.Content)
				result = &CIEnvVarsSetResult{
					WorkflowID:   wfID,
					WorkflowName: wfName,
					Name:         varName,
					Type:         varType,
					Action:       action,
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud env-vars set")
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderEnvVarsSetTable(result) },
				func() error { return renderEnvVarsSetMarkdown(result) },
			)
		},
	}
}

func webXcodeCloudEnvVarsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud env-vars delete", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	workflowID := fs.String("workflow-id", "", "Xcode Cloud workflow ID (required)")
	name := fs.String("name", "", "Environment variable name to delete (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion (required)")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc web xcode-cloud env-vars delete --product-id ID --workflow-id ID --name NAME --confirm [flags]",
		ShortHelp:  "[experimental] Delete a workflow environment variable.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Delete an environment variable from an Xcode Cloud workflow by name.

` + webWarningText + `

Examples:
  asc web xcode-cloud env-vars delete --product-id "UUID" --workflow-id "WF-UUID" --name MY_VAR --confirm --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return flag.ErrHelp
			}
			wfID := strings.TrimSpace(*workflowID)
			if wfID == "" {
				fmt.Fprintln(os.Stderr, "Error: --workflow-id is required")
				return flag.ErrHelp
			}
			varName := strings.TrimSpace(*name)
			if varName == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud env-vars delete failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			var (
				workflow *webcore.CIWorkflowFull
				vars     []webcore.CIEnvironmentVariable
			)
			err = withWebSpinner("Loading Xcode Cloud workflow environment variables", func() error {
				var err error
				workflow, err = client.GetCIWorkflow(requestCtx, teamID, pid, wfID)
				if err != nil {
					return err
				}
				vars, err = webcore.ExtractEnvVars(workflow.Content)
				if err != nil {
					return fmt.Errorf("xcode-cloud env-vars delete failed: %w", err)
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud env-vars delete")
			}

			found := false
			filtered := make([]webcore.CIEnvironmentVariable, 0, len(vars))
			for _, v := range vars {
				if strings.EqualFold(v.Name, varName) {
					found = true
					continue
				}
				filtered = append(filtered, v)
			}
			if !found {
				return fmt.Errorf("environment variable %q not found in workflow %s", varName, wfID)
			}

			result := &CIEnvVarsDeleteResult{}
			err = withWebSpinner("Deleting Xcode Cloud workflow environment variable", func() error {
				newContent, err := webcore.SetEnvVars(workflow.Content, filtered)
				if err != nil {
					return fmt.Errorf("xcode-cloud env-vars delete failed: %w", err)
				}
				if err := client.UpdateCIWorkflow(requestCtx, teamID, pid, wfID, newContent); err != nil {
					return err
				}

				wfName := extractWorkflowName(workflow.Content)
				result = &CIEnvVarsDeleteResult{
					WorkflowID:   wfID,
					WorkflowName: wfName,
					Name:         varName,
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud env-vars delete")
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderEnvVarsDeleteTable(result) },
				func() error { return renderEnvVarsDeleteMarkdown(result) },
			)
		},
	}
}

func renderEnvVarsTable(result *CIEnvVarsListResult) error {
	if result == nil || len(result.Variables) == 0 {
		fmt.Println("No environment variables found.")
		return nil
	}
	asc.RenderTable(
		[]string{"Name", "Type", "Value"},
		buildEnvVarRows(result.Variables),
	)
	return nil
}

func renderEnvVarsMarkdown(result *CIEnvVarsListResult) error {
	if result == nil || len(result.Variables) == 0 {
		fmt.Println("No environment variables found.")
		return nil
	}
	asc.RenderMarkdown(
		[]string{"Name", "Type", "Value"},
		buildEnvVarRows(result.Variables),
	)
	return nil
}

func renderEnvVarsSetTable(result *CIEnvVarsSetResult) error {
	asc.RenderTable(
		[]string{"Action", "Name", "Type", "Workflow", "Workflow ID"},
		[][]string{{result.Action, result.Name, result.Type, result.WorkflowName, result.WorkflowID}},
	)
	return nil
}

func renderEnvVarsSetMarkdown(result *CIEnvVarsSetResult) error {
	asc.RenderMarkdown(
		[]string{"Action", "Name", "Type", "Workflow", "Workflow ID"},
		[][]string{{result.Action, result.Name, result.Type, result.WorkflowName, result.WorkflowID}},
	)
	return nil
}

func renderEnvVarsDeleteTable(result *CIEnvVarsDeleteResult) error {
	asc.RenderTable(
		[]string{"Action", "Name", "Workflow", "Workflow ID"},
		[][]string{{"deleted", result.Name, result.WorkflowName, result.WorkflowID}},
	)
	return nil
}

func renderEnvVarsDeleteMarkdown(result *CIEnvVarsDeleteResult) error {
	asc.RenderMarkdown(
		[]string{"Action", "Name", "Workflow", "Workflow ID"},
		[][]string{{"deleted", result.Name, result.WorkflowName, result.WorkflowID}},
	)
	return nil
}

func buildEnvVarRows(vars []webcore.CIEnvironmentVariable) [][]string {
	rows := make([][]string, 0, len(vars))
	for _, v := range vars {
		varType := "plaintext"
		varValue := ""
		switch {
		case v.Value.Plaintext != nil:
			varType = "plaintext"
			varValue = *v.Value.Plaintext
		case v.Value.Ciphertext != nil || v.Value.RedactedValue != nil:
			varType = "secret"
			varValue = "(redacted)"
		}
		rows = append(rows, []string{v.Name, varType, varValue})
	}
	return rows
}

// extractWorkflowName extracts the "name" field from raw workflow content JSON.
func extractWorkflowName(content json.RawMessage) string {
	var m struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(content, &m); err != nil || m.Name == "" {
		return "unknown"
	}
	return m.Name
}

// newUUID generates a random UUID v4 string.
func newUUID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
