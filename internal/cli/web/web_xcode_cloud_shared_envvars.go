package web

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func webXcodeCloudEnvVarsSharedCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web xcode-cloud env-vars shared", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "shared",
		ShortUsage: "asc web xcode-cloud env-vars shared <subcommand> [flags]",
		ShortHelp:  "Manage shared (product-level) environment variables.",
		LongHelp: `WEB SESSION WORKFLOWS

List, set, and delete shared (product-level) environment variables for
Xcode Cloud products using Apple's CI API. Requires a web session.

Shared env vars are scoped to a product and can be linked to specific workflows.



Examples:
  asc web xcode-cloud env-vars shared list --product-id "UUID" --apple-id "user@example.com"
  asc web xcode-cloud env-vars shared set --product-id "UUID" --name MY_VAR --value hello --apple-id "user@example.com"
  asc web xcode-cloud env-vars shared set --product-id "UUID" --name MY_SECRET --value s3cret --secret --locked --apple-id "user@example.com"
  asc web xcode-cloud env-vars shared delete --product-id "UUID" --name MY_VAR --confirm --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			webXcodeCloudEnvVarsSharedListCommand(d),
			webXcodeCloudEnvVarsSharedSetCommand(d),
			webXcodeCloudEnvVarsSharedDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CISharedEnvVarsListResult is the output type for the env-vars shared list command.
type CISharedEnvVarsListResult struct {
	ProductID string                                 `json:"product_id"`
	Variables []webcore.CIProductEnvironmentVariable `json:"variables"`
}

// CISharedEnvVarsSetResult is the output type for the env-vars shared set command.
type CISharedEnvVarsSetResult struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Locked    bool   `json:"locked"`
	Action    string `json:"action"`
}

// CISharedEnvVarsDeleteResult is the output type for the env-vars shared delete command.
type CISharedEnvVarsDeleteResult struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
}

func webXcodeCloudEnvVarsSharedListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web xcode-cloud env-vars shared list", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web xcode-cloud env-vars shared list --product-id ID [flags]",
		ShortHelp:  "List shared (product-level) environment variables.",
		LongHelp: `WEB SESSION WORKFLOWS

List shared environment variables for an Xcode Cloud product.
Plaintext variables show their values; secret variables show "(redacted)".



Examples:
  asc web xcode-cloud env-vars shared list --product-id "UUID" --apple-id "user@example.com"
  asc web xcode-cloud env-vars shared list --product-id "UUID" --apple-id "user@example.com" --output table`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				return d.RequiredFlag("--product-id")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud env-vars shared list failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			result := &CISharedEnvVarsListResult{}
			err = withWebSpinnerWith(d, "Loading shared Xcode Cloud environment variables", func() error {
				vars, err := client.ListCIProductEnvVars(requestCtx, teamID, pid)
				if err != nil {
					return err
				}

				result = &CISharedEnvVarsListResult{
					ProductID: pid,
					Variables: vars,
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud env-vars shared list")
			}
			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderSharedEnvVarsTable(d, result) },
				func() error { return renderSharedEnvVarsMarkdown(d, result) },
			)
		},
	}
}

func webXcodeCloudEnvVarsSharedSetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web xcode-cloud env-vars shared set", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	name := fs.String("name", "", "Environment variable name (required)")
	value := fs.String("value", "", "Environment variable value (required)")
	secret := fs.Bool("secret", false, "Encrypt the value as a secret (keep value redacted)")
	locked := fs.Bool("locked", false, "Restrict editing of this variable")
	workflowIDs := shared.BindOnceCSVFlag(fs, "workflow-ids", "Comma-separated workflow IDs to link (optional)")

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web xcode-cloud env-vars shared set --product-id ID --name NAME --value VALUE [--secret] [--locked] [--workflow-ids IDS] [flags]",
		ShortHelp:  "Set a shared (product-level) environment variable.",
		LongHelp: `WEB SESSION WORKFLOWS

Set (create or update) a shared environment variable on an Xcode Cloud product.
Use --secret to encrypt the value (the same scheme as the ASC web UI).
Use --locked to restrict editing of this variable.
Use --workflow-ids to link the variable to specific workflows.
If a variable with the same name already exists, it will be updated.



Examples:
  asc web xcode-cloud env-vars shared set --product-id "UUID" --name MY_VAR --value hello --apple-id "user@example.com"
  asc web xcode-cloud env-vars shared set --product-id "UUID" --name MY_SECRET --value s3cret --secret --locked --apple-id "user@example.com"
  asc web xcode-cloud env-vars shared set --product-id "UUID" --name MY_VAR --value hello --workflow-ids "wf-1,wf-2" --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				return d.RequiredFlag("--product-id")
			}
			varName := strings.TrimSpace(*name)
			if varName == "" {
				return d.RequiredFlag("--name")
			}
			varValue := *value
			if varValue == "" {
				return d.RequiredFlag("--value")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud env-vars shared set failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			result := &CISharedEnvVarsSetResult{}
			err = withWebSpinnerWith(d, "Updating shared Xcode Cloud environment variable", func() error {
				var envValue webcore.CIEnvironmentVariableValue
				if *secret {
					keyResp, err := client.GetCIEncryptionKey(requestCtx)
					if err != nil {
						return fmt.Errorf("xcode-cloud env-vars shared set failed: could not fetch encryption key: %w", err)
					}
					ct, err := webcore.ECIESEncrypt(keyResp.Key, varValue)
					if err != nil {
						return fmt.Errorf("xcode-cloud env-vars shared set failed: encryption error: %w", err)
					}
					envValue = webcore.CIEnvironmentVariableValue{Ciphertext: &ct}
				} else {
					envValue = webcore.CIEnvironmentVariableValue{Plaintext: &varValue}
				}

				wfIDs := parseWorkflowIDs(workflowIDs.String())
				if wfIDs == nil {
					wfIDs = []string{}
				}

				existing, err := client.ListCIProductEnvVars(requestCtx, teamID, pid)
				if err != nil {
					return err
				}

				varID := ""
				action := "created"
				for _, v := range existing {
					if strings.EqualFold(v.Name, varName) {
						varID = v.ID
						action = "updated"
						if len(wfIDs) == 0 {
							for _, ws := range v.RelatedWorkflowSummaries {
								wfIDs = append(wfIDs, ws.ID)
							}
						}
						break
					}
				}
				if varID == "" {
					varID = newUUID()
				}

				req := webcore.CIProductEnvVarRequest{
					Name:        varName,
					Value:       envValue,
					IsLocked:    *locked,
					WorkflowIDs: wfIDs,
				}
				if _, err := client.SetCIProductEnvVar(requestCtx, teamID, pid, varID, req); err != nil {
					return err
				}

				varType := "plaintext"
				if *secret {
					varType = "secret"
				}
				result = &CISharedEnvVarsSetResult{
					ProductID: pid,
					Name:      varName,
					Type:      varType,
					Locked:    *locked,
					Action:    action,
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud env-vars shared set")
			}
			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderSharedEnvVarsSetTable(d, result) },
				func() error { return renderSharedEnvVarsSetMarkdown(d, result) },
			)
		},
	}
}

func webXcodeCloudEnvVarsSharedDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web xcode-cloud env-vars shared delete", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	name := fs.String("name", "", "Environment variable name to delete (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion (required)")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc web xcode-cloud env-vars shared delete --product-id ID --name NAME --confirm [flags]",
		ShortHelp:  "Delete a shared (product-level) environment variable.",
		LongHelp: `WEB SESSION WORKFLOWS

Delete a shared environment variable from an Xcode Cloud product by name.



Examples:
  asc web xcode-cloud env-vars shared delete --product-id "UUID" --name MY_VAR --confirm --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				return d.RequiredFlag("--product-id")
			}
			varName := strings.TrimSpace(*name)
			if varName == "" {
				return d.RequiredFlag("--name")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			teamID := strings.TrimSpace(session.PublicProviderID)
			if teamID == "" {
				return fmt.Errorf("xcode-cloud env-vars shared delete failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			var existing []webcore.CIProductEnvironmentVariable
			err = withWebSpinnerWith(d, "Loading shared Xcode Cloud environment variables", func() error {
				var err error
				existing, err = client.ListCIProductEnvVars(requestCtx, teamID, pid)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud env-vars shared delete")
			}

			varID := ""
			for _, v := range existing {
				if strings.EqualFold(v.Name, varName) {
					varID = v.ID
					break
				}
			}
			if varID == "" {
				return fmt.Errorf("shared environment variable %q not found in product %s", varName, pid)
			}

			result := &CISharedEnvVarsDeleteResult{}
			err = withWebSpinnerWith(d, "Deleting shared Xcode Cloud environment variable", func() error {
				if err := client.DeleteCIProductEnvVar(requestCtx, teamID, pid, varID); err != nil {
					return err
				}

				result = &CISharedEnvVarsDeleteResult{
					ProductID: pid,
					Name:      varName,
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud env-vars shared delete")
			}
			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderSharedEnvVarsDeleteTable(d, result) },
				func() error { return renderSharedEnvVarsDeleteMarkdown(d, result) },
			)
		},
	}
}

func parseWorkflowIDs(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var ids []string
	for _, part := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	return ids
}

func renderSharedEnvVarsTable(d *shared.Deps, result *CISharedEnvVarsListResult) error {
	if result == nil || len(result.Variables) == 0 {
		fmt.Fprintln(d.IO.Out, "No shared environment variables found.")
		return nil
	}
	asc.RenderTableTo(
		d.IO.Out,
		[]string{"Name", "Type", "Value", "Locked", "Workflows"},
		buildSharedEnvVarRows(result.Variables),
	)
	return nil
}

func renderSharedEnvVarsMarkdown(d *shared.Deps, result *CISharedEnvVarsListResult) error {
	if result == nil || len(result.Variables) == 0 {
		fmt.Fprintln(d.IO.Out, "No shared environment variables found.")
		return nil
	}
	asc.RenderMarkdownTo(
		d.IO.Out,
		[]string{"Name", "Type", "Value", "Locked", "Workflows"},
		buildSharedEnvVarRows(result.Variables),
	)
	return nil
}

func renderSharedEnvVarsSetTable(d *shared.Deps, result *CISharedEnvVarsSetResult) error {
	asc.RenderTableTo(
		d.IO.Out,
		[]string{"Action", "Name", "Type", "Locked", "Product ID"},
		[][]string{{result.Action, result.Name, result.Type, fmt.Sprintf("%t", result.Locked), result.ProductID}},
	)
	return nil
}

func renderSharedEnvVarsSetMarkdown(d *shared.Deps, result *CISharedEnvVarsSetResult) error {
	asc.RenderMarkdownTo(
		d.IO.Out,
		[]string{"Action", "Name", "Type", "Locked", "Product ID"},
		[][]string{{result.Action, result.Name, result.Type, fmt.Sprintf("%t", result.Locked), result.ProductID}},
	)
	return nil
}

func renderSharedEnvVarsDeleteTable(d *shared.Deps, result *CISharedEnvVarsDeleteResult) error {
	asc.RenderTableTo(
		d.IO.Out,
		[]string{"Action", "Name", "Product ID"},
		[][]string{{"deleted", result.Name, result.ProductID}},
	)
	return nil
}

func renderSharedEnvVarsDeleteMarkdown(d *shared.Deps, result *CISharedEnvVarsDeleteResult) error {
	asc.RenderMarkdownTo(
		d.IO.Out,
		[]string{"Action", "Name", "Product ID"},
		[][]string{{"deleted", result.Name, result.ProductID}},
	)
	return nil
}

func buildSharedEnvVarRows(vars []webcore.CIProductEnvironmentVariable) [][]string {
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
		lockedStr := "no"
		if v.IsLocked {
			lockedStr = "yes"
		}
		wfNames := make([]string, 0, len(v.RelatedWorkflowSummaries))
		for _, ws := range v.RelatedWorkflowSummaries {
			wfNames = append(wfNames, ws.Name)
		}
		workflows := strings.Join(wfNames, ", ")
		if workflows == "" {
			workflows = "(none)"
		}
		rows = append(rows, []string{v.Name, varType, varValue, lockedStr, workflows})
	}
	return rows
}
