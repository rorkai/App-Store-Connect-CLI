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

func TestDiscoverTools_FindsLeafCommands(t *testing.T) {
	root := testRoot()
	tools := DiscoverTools(root)

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	expected := []string{"apps_list", "apps_get", "apps_delete", "version"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected tool %q not found in discovered tools", name)
		}
	}

	if names["apps"] {
		t.Error("parent command 'apps' should not be a tool (no Exec with flags)")
	}
}

func TestDiscoverTools_ExtractsFlags(t *testing.T) {
	root := testRoot()
	tools := DiscoverTools(root)

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

	if listTool.InputSchema.Type != "object" {
		t.Errorf("inputSchema.type = %q, want object", listTool.InputSchema.Type)
	}

	props := listTool.InputSchema.Properties
	if _, ok := props["app"]; !ok {
		t.Error("expected 'app' property")
	}
	if _, ok := props["limit"]; !ok {
		t.Error("expected 'limit' property")
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
	tools := DiscoverTools(root)

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

func TestServer_Initialize(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.2.3-test")

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

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion = %v", result["protocolVersion"])
	}
	info := result["serverInfo"].(map[string]any)
	if info["name"] != "asc" {
		t.Errorf("serverInfo.name = %v", info["name"])
	}
	if info["version"] != "1.2.3-test" {
		t.Errorf("serverInfo.version = %v", info["version"])
	}
}

func TestServer_ToolsList(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.0.0")

	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
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
	tools := result["tools"].([]any)
	if len(tools) == 0 {
		t.Fatal("expected at least one tool")
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		m := tool.(map[string]any)
		names[m["name"].(string)] = true
	}
	if !names["apps_list"] {
		t.Error("apps_list not in tools/list result")
	}
}

func TestServer_ToolsCall_UnknownTool(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.0.0")

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
	s := NewServer(root, "1.0.0")

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
	s := NewServer(root, "1.0.0")

	req := "not json\n"
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(req), &out); err != nil {
		t.Fatalf("Serve() error: %v", err)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected parse error")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("error code = %d, want -32700", resp.Error.Code)
	}
}

func TestServer_MultipleRequests(t *testing.T) {
	root := testRoot()
	s := NewServer(root, "1.0.0")

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
`
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(req), &out); err != nil {
		t.Fatalf("Serve() error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d: %q", len(lines), out.String())
	}

	for i, line := range lines {
		var resp jsonRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("line %d: unmarshal error: %v", i, err)
		}
		if resp.Error != nil {
			t.Errorf("line %d: unexpected error: %v", i, resp.Error)
		}
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
	if !strings.Contains(argsStr, "--paginate true") {
		t.Errorf("expected --paginate true in args: %q", argsStr)
	}
}

func TestBuildCLIArgs_SkipsEmptyValues(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("app", "", "App ID")
	fs.String("next", "", "Next URL")

	cmd := &ffcli.Command{FlagSet: fs}
	args := buildCLIArgs(cmd, map[string]any{
		"app":  "123456",
		"next": "",
	})

	for _, arg := range args {
		if arg == "--next" {
			t.Error("should not include --next with empty value")
		}
	}
}
