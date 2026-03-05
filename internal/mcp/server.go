// Package mcp implements a Model Context Protocol server for the asc CLI.
//
// It exposes every CLI command as a typed MCP tool over a stdio JSON-RPC 2.0
// transport. Agents call tools/list to discover available commands and
// tools/call to invoke them, receiving structured JSON output without shell
// escaping or output parsing.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// jsonRPCRequest is a JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

// jsonRPCError is a JSON-RPC 2.0 error object.
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server is an MCP server backed by an ffcli command tree.
type Server struct {
	root    *ffcli.Command
	tools   []Tool
	toolMap map[string]*ffcli.Command
	version string
}

// NewServer creates an MCP server from a root ffcli command.
func NewServer(root *ffcli.Command, version string) *Server {
	tools := DiscoverTools(root)
	toolMap := buildToolMap(root)
	return &Server{
		root:    root,
		tools:   tools,
		toolMap: toolMap,
		version: version,
	}
}

// Run reads JSON-RPC requests from stdin and writes responses to stdout.
func (s *Server) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	return s.Serve(ctx, os.Stdin, os.Stdout)
}

// Serve reads JSON-RPC requests from r and writes responses to w. Exported for
// testing without touching process-level stdin/stdout.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				Error:   &jsonRPCError{Code: -32700, Message: "parse error"},
			}
			writeResponse(w, resp)
			continue
		}

		resp := s.handle(ctx, req)
		writeResponse(w, resp)
	}

	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, req jsonRPCRequest) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID}
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func (s *Server) handleInitialize(req jsonRPCRequest) jsonRPCResponse {
	result := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "asc",
			"version": s.version,
		},
	}
	return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *Server) handleToolsList(req jsonRPCRequest) jsonRPCResponse {
	return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
		"tools": s.tools,
	}}
}

type toolsCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

func (s *Server) handleToolsCall(ctx context.Context, req jsonRPCRequest) jsonRPCResponse {
	var params toolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32602, Message: "invalid params"},
		}
	}

	cmd, ok := s.toolMap[params.Name]
	if !ok {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32602, Message: fmt.Sprintf("unknown tool: %s", params.Name)},
		}
	}

	args := buildCLIArgs(cmd, params.Arguments)

	var stdout, stderr bytes.Buffer
	exitCode := runCommand(ctx, s.root, cmd, args, &stdout, &stderr)

	isError := exitCode != 0
	text := stdout.String()
	if text == "" {
		text = stderr.String()
	}

	content := []map[string]string{
		{"type": "text", "text": text},
	}

	result := map[string]any{
		"content": content,
		"isError": isError,
	}
	return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func buildCLIArgs(cmd *ffcli.Command, arguments map[string]any) []string {
	var args []string
	if cmd.FlagSet != nil {
		cmd.FlagSet.VisitAll(func(f *flag.Flag) {
			val, ok := arguments[f.Name]
			if !ok {
				return
			}
			strVal := fmt.Sprintf("%v", val)
			if strVal == "" {
				return
			}
			args = append(args, fmt.Sprintf("--%s", f.Name), strVal)
		})
	}
	return args
}

// runCommand executes a single CLI command by reconstructing the full
// subcommand path and running through the root command's parse+exec flow.
func runCommand(ctx context.Context, root *ffcli.Command, target *ffcli.Command, flagArgs []string, stdout, stderr *bytes.Buffer) int {
	cmdPath := resolveCommandPath(root, target)

	fullArgs := append(cmdPath, flagArgs...)

	origStdout := os.Stdout
	origStderr := os.Stderr
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout = outW
	os.Stderr = errW

	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(stdout, outR)
		close(done)
	}()
	doneErr := make(chan struct{})
	go func() {
		_, _ = io.Copy(stderr, errR)
		close(doneErr)
	}()

	exitCode := 0

	freshRoot := cloneRootForMCP(root)
	if err := freshRoot.Parse(fullArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			exitCode = 0
		} else {
			fmt.Fprintf(errW, "Error: %v\n", err)
			exitCode = 2
		}
	} else {
		if err := freshRoot.Run(ctx); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				exitCode = 0
			} else {
				fmt.Fprintf(errW, "Error: %v\n", err)
				exitCode = 1
			}
		}
	}

	outW.Close()
	errW.Close()
	<-done
	<-doneErr

	return exitCode
}

// resolveCommandPath walks the root tree to find the subcommand path tokens
// needed to reach the target command.
func resolveCommandPath(root *ffcli.Command, target *ffcli.Command) []string {
	var path []string
	if findPath(root, target, &path) {
		return path
	}
	return nil
}

func findPath(current *ffcli.Command, target *ffcli.Command, path *[]string) bool {
	if current == target {
		return true
	}
	for _, sub := range current.Subcommands {
		*path = append(*path, sub.Name)
		if findPath(sub, target, path) {
			return true
		}
		*path = (*path)[:len(*path)-1]
	}
	return false
}

// cloneRootForMCP creates a fresh root command tree for isolated execution.
// We import the registry to rebuild the tree so each invocation gets fresh
// flag state.
var CloneRoot func(version string) *ffcli.Command

func cloneRootForMCP(root *ffcli.Command) *ffcli.Command {
	if CloneRoot != nil {
		return CloneRoot(root.Name)
	}
	return root
}

func buildToolMap(root *ffcli.Command) map[string]*ffcli.Command {
	m := make(map[string]*ffcli.Command)
	for _, sub := range root.Subcommands {
		walkToolMap(sub, nil, m)
	}
	return m
}

func walkToolMap(cmd *ffcli.Command, parentPath []string, m map[string]*ffcli.Command) {
	currentPath := append(parentPath, cmd.Name)
	name := strings.Join(currentPath, "_")

	if len(cmd.Subcommands) > 0 {
		if cmd.Exec != nil && cmd.FlagSet != nil && hasFlagsRegistered(cmd.FlagSet) {
			m[name] = cmd
		}
		for _, sub := range cmd.Subcommands {
			walkToolMap(sub, currentPath, m)
		}
		return
	}

	if cmd.Exec != nil {
		m[name] = cmd
	}
}

func writeResponse(w io.Writer, resp jsonRPCResponse) {
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	_, _ = w.Write(data)
}
