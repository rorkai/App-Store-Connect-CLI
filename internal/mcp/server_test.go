package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func testRoot() *ffcli.Command {
	listFS := flag.NewFlagSet("list", flag.ContinueOnError)
	listFS.String("app", "", "App ID")
	listFS.Int("limit", 0, "Max results")
	listFS.Bool("paginate", false, "Fetch all pages")
	listFS.String("output", "json", "Output format")

	getFS := flag.NewFlagSet("get", flag.ContinueOnError)
	getFS.String("id", "", "Resource ID")

	deleteFS := flag.NewFlagSet("delete", flag.ContinueOnError)
	deleteFS.String("id", "", "Resource ID")
	deleteFS.Bool("confirm", false, "Confirm deletion")

	gcListFS := flag.NewFlagSet("gc-list", flag.ContinueOnError)
	gcListFS.String("app", "", "App ID")

	return &ffcli.Command{
		Name:    "asc",
		FlagSet: flag.NewFlagSet("asc", flag.ContinueOnError),
		Subcommands: []*ffcli.Command{
			{
				Name:      "apps",
				ShortHelp: "Manage apps",
				FlagSet:   flag.NewFlagSet("apps", flag.ContinueOnError),
				Subcommands: []*ffcli.Command{
					{
						Name:      "list",
						ShortHelp: "List apps",
						FlagSet:   listFS,
						Exec: func(_ context.Context, _ []string) error {
							return nil
						},
					},
					{
						Name:      "get",
						ShortHelp: "Get app by ID",
						FlagSet:   getFS,
						Exec: func(_ context.Context, _ []string) error {
							return nil
						},
					},
					{
						Name:      "delete",
						ShortHelp: "Delete app",
						FlagSet:   deleteFS,
						Exec: func(_ context.Context, _ []string) error {
							return nil
						},
					},
				},
			},
			{
				Name:      "game-center",
				ShortHelp: "Manage Game Center",
				FlagSet:   flag.NewFlagSet("game-center", flag.ContinueOnError),
				Subcommands: []*ffcli.Command{
					{
						Name:      "leaderboards",
						ShortHelp: "List leaderboards",
						FlagSet:   gcListFS,
						Exec: func(_ context.Context, _ []string) error {
							return nil
						},
					},
				},
			},
			{
				Name:      "version",
				ShortHelp: "Print version",
				FlagSet:   flag.NewFlagSet("version", flag.ContinueOnError),
				Exec: func(_ context.Context, _ []string) error {
					return nil
				},
			},
		},
	}
}

func TestDiscoverTools_AllGroups(t *testing.T) {
	root := testRoot()
	tools := DiscoverTools(root, nil)

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	expected := []string{"apps_list", "apps_get", "apps_delete", "game-center_leaderboards", "version"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected tool %q not found in discovered tools", name)
		}
	}
}

func TestDiscoverTools_FilteredGroups(t *testing.T) {
	root := testRoot()
	tools := DiscoverTools(root, []string{"apps"})

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	if !names["apps_list"] {
		t.Error("expected apps_list in filtered results")
	}
	if names["game-center_leaderboards"] {
		t.Error("game-center_leaderboards should be excluded when filtering to 'apps' only")
	}
	if names["version"] {
		t.Error("version should be excluded when filtering to 'apps' only")
	}
}

func TestDiscoverTools_ExtractsFlags(t *testing.T) {
	root := testRoot()
	tools := DiscoverTools(root, nil)

	var listTool *Tool
	for i := range tools {
		if tools[i].Name == "apps_list" {
			listTool = &tools[i]
			break
		}
	}
	if listTool == nil {
		t.Fatal("apps_list tool not found")
	}

	props := listTool.InputSchema.Properties
	if _, ok := props["app"]; !ok {
		t.Error("expected 'app' property")
	}
	if props["limit"].Type != "integer" {
		t.Errorf("limit type = %q, want integer", props["limit"].Type)
	}
	if props["paginate"].Type != "boolean" {
		t.Errorf("paginate type = %q, want boolean", props["paginate"].Type)
	}
}

func TestDiscoverTools_SetsDescription(t *testing.T) {
	root := testRoot()
	tools := DiscoverTools(root, nil)

	for _, tool := range tools {
		if tool.Name == "apps_list" {
			if tool.Description != "List apps" {
				t.Errorf("description = %q, want %q", tool.Description, "List apps")
			}
			return
		}
	}
	t.Fatal("apps_list not found")
}

func TestRunTool_HasRequiredField(t *testing.T) {
	tool := RunTool()
	if tool.Name != "asc_run" {
		t.Errorf("name = %q, want asc_run", tool.Name)
	}
	if _, ok := tool.InputSchema.Properties["command"]; !ok {
		t.Error("expected 'command' property")
	}
	if len(tool.InputSchema.Required) != 1 || tool.InputSchema.Required[0] != "command" {
		t.Errorf("required = %v, want [command]", tool.InputSchema.Required)
	}
}

func TestNewServer_DefaultGroups_IncludesRunTool(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.0.0", nil)

	hasRunTool := false
	for _, tool := range s.tools {
		if tool.Name == "asc_run" {
			hasRunTool = true
			break
		}
	}
	if !hasRunTool {
		t.Error("expected asc_run tool in server tools")
	}
}

func TestNewServer_AllGroups(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.0.0", []string{"all"})

	names := make(map[string]bool)
	for _, tool := range s.tools {
		names[tool.Name] = true
	}
	if !names["apps_list"] {
		t.Error("expected apps_list")
	}
	if !names["game-center_leaderboards"] {
		t.Error("expected game-center_leaderboards with 'all'")
	}
	if !names["asc_run"] {
		t.Error("expected asc_run")
	}
}

func TestServer_Initialize(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.2.3-test", []string{"all"})

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(req), &out); err != nil {
		t.Fatalf("Serve() error: %v", err)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result := resp.Result.(map[string]any)
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion = %v", result["protocolVersion"])
	}
	info := result["serverInfo"].(map[string]any)
	if info["version"] != "1.2.3-test" {
		t.Errorf("serverInfo.version = %v", info["version"])
	}
}

func TestServer_ToolsList(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.0.0", []string{"all"})

	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(req), &out); err != nil {
		t.Fatalf("Serve() error: %v", err)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	result := resp.Result.(map[string]any)
	tools := result["tools"].([]any)

	names := make(map[string]bool)
	for _, tool := range tools {
		m := tool.(map[string]any)
		names[m["name"].(string)] = true
	}
	if !names["apps_list"] {
		t.Error("apps_list not in tools/list result")
	}
	if !names["asc_run"] {
		t.Error("asc_run not in tools/list result")
	}
}

func TestServer_ToolsCall_UnknownTool(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.0.0", []string{"all"})

	req := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nonexistent","arguments":{}}}` + "\n"
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(req), &out); err != nil {
		t.Fatalf("Serve() error: %v", err)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("error code = %d, want -32602", resp.Error.Code)
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.0.0", []string{"all"})

	req := `{"jsonrpc":"2.0","id":5,"method":"resources/list"}` + "\n"
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(req), &out); err != nil {
		t.Fatalf("Serve() error: %v", err)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", resp.Error.Code)
	}
}

func TestServer_ParseError(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.0.0", []string{"all"})

	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader("not json\n"), &out); err != nil {
		t.Fatalf("Serve() error: %v", err)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32700 {
		t.Errorf("expected parse error (-32700), got %v", resp.Error)
	}
}

func TestServer_MultipleRequests(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.0.0", []string{"all"})

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
`
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(req), &out); err != nil {
		t.Fatalf("Serve() error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d", len(lines))
	}
}

func TestBuildCLIArgs_ConvertsArguments(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("app", "", "App ID")
	fs.Int("limit", 0, "Limit")
	fs.Bool("paginate", false, "Paginate")

	cmd := &ffcli.Command{FlagSet: fs}
	args := buildCLIArgs(cmd, map[string]any{
		"app":      "123456",
		"limit":    float64(10),
		"paginate": true,
	})

	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "--app 123456") {
		t.Errorf("expected --app 123456 in args: %q", argsStr)
	}
	if !strings.Contains(argsStr, "--limit 10") {
		t.Errorf("expected --limit 10 in args: %q", argsStr)
	}
}

func TestBuildCLIArgs_SkipsEmptyValues(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("app", "", "App ID")
	fs.String("next", "", "Next URL")

	cmd := &ffcli.Command{FlagSet: fs}
	args := buildCLIArgs(cmd, map[string]any{"app": "123456", "next": ""})

	for _, arg := range args {
		if arg == "--next" {
			t.Error("should not include --next with empty value")
		}
	}
}

func TestSplitCommandString(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"apps list --app 123", []string{"apps", "list", "--app", "123"}},
		{`apps list --name "My App"`, []string{"apps", "list", "--name", "My App"}},
		{"  apps  list  ", []string{"apps", "list"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := splitCommandString(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCommandString(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCommandString(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestResolveGroups(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		isNil  bool
		length int
	}{
		{"nil defaults", nil, false, len(DefaultCommandGroups)},
		{"empty defaults", []string{}, false, len(DefaultCommandGroups)},
		{"all returns nil", []string{"all"}, true, 0},
		{"custom groups", []string{"apps", "builds"}, false, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveGroups(tt.input)
			if tt.isNil && got != nil {
				t.Errorf("expected nil, got %v", got)
			}
			if !tt.isNil && len(got) != tt.length {
				t.Errorf("length = %d, want %d", len(got), tt.length)
			}
		})
	}
}
