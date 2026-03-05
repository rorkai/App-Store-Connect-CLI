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
}

// Property describes a single tool parameter.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
}

// DiscoverTools walks the ffcli command tree and returns MCP tool descriptors
// for every leaf command (commands with an Exec function that either have no
// subcommands or represent a directly-executable group).
func DiscoverTools(root *ffcli.Command) []Tool {
	var tools []Tool
	for _, sub := range root.Subcommands {
		walkCommands(sub, nil, &tools)
	}
	return tools
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
