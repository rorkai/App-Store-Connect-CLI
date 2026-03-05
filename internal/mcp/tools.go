package mcp

import (
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// Tool is an MCP tool descriptor derived from an ffcli command.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema is a JSON Schema object describing tool parameters.
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property describes a single tool parameter.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
}

// DefaultCommandGroups is the curated set of command groups exposed by default.
// These cover the most common agent workflows. Use --commands all for the full
// set, or --commands group1,group2 to customize.
var DefaultCommandGroups = []string{
	"apps",
	"builds",
	"testflight",
	"submit",
	"validate",
	"versions",
	"localizations",
	"metadata",
	"screenshots",
	"certificates",
	"profiles",
	"bundle-ids",
	"devices",
	"users",
	"analytics",
	"finance",
	"feedback",
	"crashes",
	"reviews",
	"iap",
	"subscriptions",
	"pricing",
	"publish",
	"status",
	"auth",
	"signing",
	"xcode-cloud",
	"workflow",
	"app-info",
}

// DiscoverTools walks the ffcli command tree and returns MCP tool descriptors.
// When groups is nil or empty, all leaf commands are included. When groups is
// set, only commands whose top-level parent name matches a group are included.
func DiscoverTools(root *ffcli.Command, groups []string) []Tool {
	filter := buildGroupFilter(groups)

	var tools []Tool
	for _, sub := range root.Subcommands {
		if filter != nil {
			if _, ok := filter[sub.Name]; !ok {
				continue
			}
		}
		walkCommands(sub, nil, &tools)
	}
	return tools
}

// RunTool returns a catch-all tool that lets agents invoke any asc command by
// passing the full command string. This covers commands not in the curated set.
func RunTool() Tool {
	return Tool{
		Name:        "asc_run",
		Description: "Run any asc CLI command by passing the full argument string. Use for commands not exposed as individual tools.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"command": {
					Type:        "string",
					Description: "Full asc command arguments (e.g. \"game-center leaderboards list --app 123456789\")",
				},
			},
			Required: []string{"command"},
		},
	}
}

func buildGroupFilter(groups []string) map[string]struct{} {
	if len(groups) == 0 {
		return nil
	}
	filter := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g != "" {
			filter[g] = struct{}{}
		}
	}
	if len(filter) == 0 {
		return nil
	}
	return filter
}

func walkCommands(cmd *ffcli.Command, parentPath []string, tools *[]Tool) {
	currentPath := append(parentPath, cmd.Name)

	if len(cmd.Subcommands) > 0 {
		if cmd.Exec != nil && cmd.FlagSet != nil && hasFlagsRegistered(cmd.FlagSet) {
			*tools = append(*tools, buildTool(currentPath, cmd))
		}
		for _, sub := range cmd.Subcommands {
			walkCommands(sub, currentPath, tools)
		}
		return
	}

	if cmd.Exec != nil {
		*tools = append(*tools, buildTool(currentPath, cmd))
	}
}

func buildTool(path []string, cmd *ffcli.Command) Tool {
	name := strings.Join(path, "_")

	description := strings.TrimSpace(cmd.ShortHelp)
	if description == "" {
		description = strings.TrimSpace(cmd.LongHelp)
	}

	props := make(map[string]Property)
	if cmd.FlagSet != nil {
		cmd.FlagSet.VisitAll(func(f *flag.Flag) {
			p := Property{
				Description: f.Usage,
			}
			p.Type = inferJSONSchemaType(f)
			if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
				p.Default = f.DefValue
			}
			props[f.Name] = p
		})
	}

	return Tool{
		Name:        name,
		Description: description,
		InputSchema: InputSchema{
			Type:       "object",
			Properties: props,
		},
	}
}

func inferJSONSchemaType(f *flag.Flag) string {
	type boolFlag interface {
		IsBoolFlag() bool
	}
	if v, ok := f.Value.(boolFlag); ok && v.IsBoolFlag() {
		return "boolean"
	}
	typeName := fmt.Sprintf("%T", f.Value)
	if strings.Contains(typeName, "int") || strings.Contains(typeName, "Int") {
		return "integer"
	}
	if strings.Contains(typeName, "float") || strings.Contains(typeName, "Float") {
		return "number"
	}
	if strings.Contains(typeName, "duration") || strings.Contains(typeName, "Duration") {
		return "string"
	}
	return "string"
}

func hasFlagsRegistered(fs *flag.FlagSet) bool {
	hasFlags := false
	fs.VisitAll(func(*flag.Flag) {
		hasFlags = true
	})
	return hasFlags
}
