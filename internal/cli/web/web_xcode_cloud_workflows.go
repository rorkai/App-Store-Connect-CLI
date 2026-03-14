package web

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func webXcodeCloudWorkflowsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "workflows",
		ShortUsage: "asc web xcode-cloud workflows <subcommand> [flags]",
		ShortHelp:  "[experimental] Describe and toggle Xcode Cloud workflows.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Describe and manage workflow state for Xcode Cloud workflows
using Apple's private CI API. Requires a web session.

Use describe to inspect workflow configuration.
Use enable/disable to toggle workflow state.

` + webWarningText + `

Examples:
  asc web xcode-cloud workflows describe --product-id "UUID" --workflow-id "WF-UUID" --apple-id "user@example.com"
  asc web xcode-cloud workflows enable --product-id "UUID" --workflow-id "WF-UUID" --apple-id "user@example.com"
  asc web xcode-cloud workflows disable --product-id "UUID" --workflow-id "WF-UUID" --confirm --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			webXcodeCloudWorkflowDescribeCommand(),
			webXcodeCloudWorkflowEnableCommand(),
			webXcodeCloudWorkflowDisableCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CIWorkflowDescribeResult is the output type for workflows describe.
type CIWorkflowDescribeResult struct {
	ProductID                   string          `json:"product_id"`
	WorkflowID                  string          `json:"workflow_id"`
	Name                        string          `json:"name"`
	Description                 string          `json:"description,omitempty"`
	Disabled                    bool            `json:"disabled"`
	Locked                      bool            `json:"locked"`
	XcodeVersion                json.RawMessage `json:"xcode_version,omitempty"`
	MacOSVersion                json.RawMessage `json:"macos_version,omitempty"`
	Clean                       json.RawMessage `json:"clean,omitempty"`
	ContainerFilePath           string          `json:"container_file_path,omitempty"`
	ProductEnvironmentVariables []string        `json:"product_environment_variables,omitempty"`
	StartConditions             json.RawMessage `json:"start_conditions,omitempty"`
	Actions                     json.RawMessage `json:"actions,omitempty"`
	PostActions                 json.RawMessage `json:"post_actions,omitempty"`
	Repo                        json.RawMessage `json:"repo,omitempty"`
}

// CIWorkflowToggleResult is the output type for workflows enable/disable.
type CIWorkflowToggleResult struct {
	ProductID      string `json:"product_id"`
	WorkflowID     string `json:"workflow_id"`
	WorkflowName   string `json:"workflow_name"`
	Action         string `json:"action"`
	DisabledBefore bool   `json:"disabled_before"`
	DisabledAfter  bool   `json:"disabled_after"`
	Changed        bool   `json:"changed"`
}

func webXcodeCloudWorkflowDescribeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows describe", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	workflowID := fs.String("workflow-id", "", "Xcode Cloud workflow ID (required)")

	return &ffcli.Command{
		Name:       "describe",
		ShortUsage: "asc web xcode-cloud workflows describe --product-id ID --workflow-id ID [flags]",
		ShortHelp:  "[experimental] Show workflow configuration.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show workflow configuration for a specific Xcode Cloud workflow.
Includes state, toolchain versions, triggers, actions, and linked shared env vars.

` + webWarningText + `

Examples:
  asc web xcode-cloud workflows describe --product-id "UUID" --workflow-id "WF-UUID" --apple-id "user@example.com"
  asc web xcode-cloud workflows describe --product-id "UUID" --workflow-id "WF-UUID" --apple-id "user@example.com" --output table`,
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
				return fmt.Errorf("xcode-cloud workflows describe failed: session has no public provider ID")
			}

			client := newCIClientFn(session)
			result := &CIWorkflowDescribeResult{}
			err = withWebSpinner("Loading Xcode Cloud workflow details", func() error {
				workflow, err := client.GetCIWorkflow(requestCtx, teamID, pid, wfID)
				if err != nil {
					return err
				}

				config, err := webcore.ExtractWorkflowConfig(workflow.Content)
				if err != nil {
					return fmt.Errorf("xcode-cloud workflows describe failed: %w", err)
				}

				result = &CIWorkflowDescribeResult{
					ProductID:                   pid,
					WorkflowID:                  wfID,
					Name:                        config.Name,
					Description:                 config.Description,
					Disabled:                    config.Disabled,
					Locked:                      config.Locked,
					XcodeVersion:                config.XcodeVersion,
					MacOSVersion:                config.MacOSVersion,
					Clean:                       config.Clean,
					ContainerFilePath:           config.ContainerFilePath,
					ProductEnvironmentVariables: config.ProductEnvironmentVariables,
					StartConditions:             config.StartConditions,
					Actions:                     config.Actions,
					PostActions:                 config.PostActions,
					Repo:                        config.Repo,
				}
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "xcode-cloud workflows describe")
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWorkflowDescribeTable(result) },
				func() error { return renderWorkflowDescribeMarkdown(result) },
			)
		},
	}
}

func webXcodeCloudWorkflowEnableCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows enable", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	workflowID := fs.String("workflow-id", "", "Xcode Cloud workflow ID (required)")

	return &ffcli.Command{
		Name:       "enable",
		ShortUsage: "asc web xcode-cloud workflows enable --product-id ID --workflow-id ID [flags]",
		ShortHelp:  "[experimental] Enable a workflow.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Enable an Xcode Cloud workflow by setting disabled=false.
If already enabled, this command reports no change and exits successfully.

` + webWarningText + `

Examples:
  asc web xcode-cloud workflows enable --product-id "UUID" --workflow-id "WF-UUID" --apple-id "user@example.com"`,
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

			result, err := executeWorkflowToggle(ctx, sessionFlags, pid, wfID, false, "xcode-cloud workflows enable")
			if err != nil {
				return err
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWorkflowToggleTable(result) },
				func() error { return renderWorkflowToggleMarkdown(result) },
			)
		},
	}
}

func webXcodeCloudWorkflowDisableCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows disable", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	workflowID := fs.String("workflow-id", "", "Xcode Cloud workflow ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm disabling this workflow (required)")

	return &ffcli.Command{
		Name:       "disable",
		ShortUsage: "asc web xcode-cloud workflows disable --product-id ID --workflow-id ID --confirm [flags]",
		ShortHelp:  "[experimental] Disable a workflow.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Disable an Xcode Cloud workflow by setting disabled=true.
Requires --confirm.
If already disabled, this command reports no change and exits successfully.

` + webWarningText + `

Examples:
  asc web xcode-cloud workflows disable --product-id "UUID" --workflow-id "WF-UUID" --confirm --apple-id "user@example.com"`,
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
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			result, err := executeWorkflowToggle(ctx, sessionFlags, pid, wfID, true, "xcode-cloud workflows disable")
			if err != nil {
				return err
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWorkflowToggleTable(result) },
				func() error { return renderWorkflowToggleMarkdown(result) },
			)
		},
	}
}

func executeWorkflowToggle(
	ctx context.Context,
	sessionFlags webSessionFlags,
	productID, workflowID string,
	disabled bool,
	errorPrefix string,
) (*CIWorkflowToggleResult, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
	if err != nil {
		return nil, err
	}
	teamID := strings.TrimSpace(session.PublicProviderID)
	if teamID == "" {
		return nil, fmt.Errorf("%s failed: session has no public provider ID", errorPrefix)
	}

	client := newCIClientFn(session)
	var result *CIWorkflowToggleResult
	err = withWebSpinner("Updating Xcode Cloud workflow", func() error {
		workflow, err := client.GetCIWorkflow(requestCtx, teamID, productID, workflowID)
		if err != nil {
			return err
		}

		config, err := webcore.ExtractWorkflowConfig(workflow.Content)
		if err != nil {
			return fmt.Errorf("%s failed: %w", errorPrefix, err)
		}

		before := config.Disabled
		changed := before != disabled
		action := "enabled"
		if disabled {
			action = "disabled"
		}

		if changed {
			newContent, err := webcore.SetWorkflowDisabled(workflow.Content, disabled)
			if err != nil {
				return fmt.Errorf("%s failed: %w", errorPrefix, err)
			}
			if err := client.UpdateCIWorkflow(requestCtx, teamID, productID, workflowID, newContent); err != nil {
				return err
			}
		} else if disabled {
			action = "already-disabled"
		} else {
			action = "already-enabled"
		}

		workflowName := strings.TrimSpace(config.Name)
		if workflowName == "" {
			workflowName = "unknown"
		}

		result = &CIWorkflowToggleResult{
			ProductID:      productID,
			WorkflowID:     workflowID,
			WorkflowName:   workflowName,
			Action:         action,
			DisabledBefore: before,
			DisabledAfter:  disabled,
			Changed:        changed,
		}
		return nil
	})
	if err != nil {
		return nil, withWebAuthHint(err, errorPrefix)
	}

	return result, nil
}

func renderWorkflowDescribeTable(result *CIWorkflowDescribeResult) error {
	if result == nil {
		return nil
	}

	asc.RenderTable(
		[]string{
			"Workflow",
			"Workflow ID",
			"Disabled",
			"Locked",
			"Xcode",
			"macOS",
			"Triggers",
			"Actions",
			"Post Actions",
			"Shared Vars",
		},
		[][]string{{
			valueOrNA(strings.TrimSpace(result.Name)),
			result.WorkflowID,
			fmt.Sprintf("%t", result.Disabled),
			fmt.Sprintf("%t", result.Locked),
			valueOrNA(summarizeJSONValue(result.XcodeVersion)),
			valueOrNA(summarizeJSONValue(result.MacOSVersion)),
			summarizeStartConditions(result.StartConditions),
			summarizeActionList(result.Actions),
			summarizeActionList(result.PostActions),
			fmt.Sprintf("%d", len(result.ProductEnvironmentVariables)),
		}},
	)
	return nil
}

func renderWorkflowDescribeMarkdown(result *CIWorkflowDescribeResult) error {
	if result == nil {
		return nil
	}

	asc.RenderMarkdown(
		[]string{
			"Workflow",
			"Workflow ID",
			"Disabled",
			"Locked",
			"Xcode",
			"macOS",
			"Triggers",
			"Actions",
			"Post Actions",
			"Shared Vars",
		},
		[][]string{{
			valueOrNA(strings.TrimSpace(result.Name)),
			result.WorkflowID,
			fmt.Sprintf("%t", result.Disabled),
			fmt.Sprintf("%t", result.Locked),
			valueOrNA(summarizeJSONValue(result.XcodeVersion)),
			valueOrNA(summarizeJSONValue(result.MacOSVersion)),
			summarizeStartConditions(result.StartConditions),
			summarizeActionList(result.Actions),
			summarizeActionList(result.PostActions),
			fmt.Sprintf("%d", len(result.ProductEnvironmentVariables)),
		}},
	)
	return nil
}

func renderWorkflowToggleTable(result *CIWorkflowToggleResult) error {
	if result == nil {
		return nil
	}

	asc.RenderTable(
		[]string{"Action", "Workflow", "Workflow ID", "Disabled Before", "Disabled After", "Changed"},
		[][]string{{
			result.Action,
			result.WorkflowName,
			result.WorkflowID,
			fmt.Sprintf("%t", result.DisabledBefore),
			fmt.Sprintf("%t", result.DisabledAfter),
			fmt.Sprintf("%t", result.Changed),
		}},
	)
	return nil
}

func renderWorkflowToggleMarkdown(result *CIWorkflowToggleResult) error {
	if result == nil {
		return nil
	}

	asc.RenderMarkdown(
		[]string{"Action", "Workflow", "Workflow ID", "Disabled Before", "Disabled After", "Changed"},
		[][]string{{
			result.Action,
			result.WorkflowName,
			result.WorkflowID,
			fmt.Sprintf("%t", result.DisabledBefore),
			fmt.Sprintf("%t", result.DisabledAfter),
			fmt.Sprintf("%t", result.Changed),
		}},
	)
	return nil
}

func countJSONCollection(raw json.RawMessage) int {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return 0
	}

	switch trimmed[0] {
	case '[':
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			return 0
		}
		return len(arr)
	case '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return 0
		}
		return len(obj)
	default:
		return 1
	}
}

func summarizeJSONValue(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}

	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return strings.TrimSpace(str)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, key := range []string{"name", "version", "display_name", "id", "alias"} {
			var v string
			if value, ok := obj[key]; ok && json.Unmarshal(value, &v) == nil && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err == nil {
		return buf.String()
	}
	return trimmed
}

func summarizeStartConditions(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "0"
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		if len(obj) == 0 {
			return "0"
		}

		names := make([]string, 0, len(obj))
		for key := range obj {
			names = append(names, humanizeIdentifier(key))
		}
		sort.Strings(names)
		return summarizeNameList(names)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return fmt.Sprintf("%d", countJSONCollection(raw))
	}
	if len(items) == 0 {
		return "0"
	}

	names := make([]string, 0, len(items))
	for _, item := range items {
		name := summarizeStartConditionItem(item)
		if strings.TrimSpace(name) == "" {
			name = "Unnamed"
		}
		names = append(names, name)
	}
	return summarizeNameList(names)
}

func summarizeStartConditionItem(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}

	var item map[string]json.RawMessage
	if err := json.Unmarshal(raw, &item); err != nil || len(item) == 0 {
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			return humanizeIdentifier(value)
		}
		return ""
	}

	name := firstNonEmptyJSONField(item, "name", "default_name", "display_name", "title")
	if strings.TrimSpace(name) == "" {
		name = humanizeIdentifier(firstNonEmptyJSONField(item, "type", "kind", "trigger_type", "start_condition_type"))
	}
	if strings.TrimSpace(name) != "" {
		return name
	}
	if len(item) == 1 {
		for key := range item {
			return humanizeIdentifier(key)
		}
	}
	return ""
}

func summarizeActionList(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "0"
	}

	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return fmt.Sprintf("%d", countJSONCollection(raw))
	}
	if len(items) == 0 {
		return "0"
	}

	names := make([]string, 0, len(items))
	for _, item := range items {
		name := firstNonEmptyJSONField(item, "name", "default_name", "display_name", "title")
		if name == "" {
			name = humanizeIdentifier(firstNonEmptyJSONField(item, "action_type", "type", "kind"))
		}
		if strings.TrimSpace(name) == "" {
			name = "Unnamed"
		}
		names = append(names, name)
	}
	return summarizeNameList(names)
}

func summarizeNameList(names []string) string {
	const maxPreview = 3
	if len(names) == 0 {
		return "0"
	}

	preview := names
	if len(preview) > maxPreview {
		preview = preview[:maxPreview]
	}

	label := strings.Join(preview, ", ")
	if len(names) > maxPreview {
		label = fmt.Sprintf("%s, +%d more", label, len(names)-maxPreview)
	}
	return fmt.Sprintf("%d (%s)", len(names), label)
}

func firstNonEmptyJSONField(m map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func humanizeIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}

	parts := strings.Split(value, " ")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if len(part) == 1 {
			parts[i] = strings.ToUpper(part)
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}
