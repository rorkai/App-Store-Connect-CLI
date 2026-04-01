package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

type LaunchSpec struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
}

type SessionConfig struct {
	CWD       string `json:"cwd,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
}

type SessionResult struct {
	SessionID string `json:"sessionId"`
}

type PromptResult struct {
	Status   string `json:"status,omitempty"`
	Summary_ string `json:"summary,omitempty"`
}

func (p PromptResult) Summary() string {
	return p.Summary_
}

type Event struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	events chan Event

	nextID  atomic.Int64
	done    chan struct{}
	once    sync.Once
	mu      sync.Mutex
	pending map[int64]chan rpcResponse
}

func Start(ctx context.Context, spec LaunchSpec) (*Client, error) {
	if spec.Command == "" {
		return nil, errors.New("agent command is required")
	}

	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Dir = spec.Dir
	if len(spec.Env) > 0 {
		cmd.Env = append(cmd.Environ(), spec.Env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	client := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		events:  make(chan Event, 32),
		done:    make(chan struct{}),
		pending: make(map[int64]chan rpcResponse),
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start agent process: %w", err)
	}

	go client.readLoop()
	go client.drainStderr()
	return client, nil
}

func (c *Client) Close() error {
	var err error
	c.once.Do(func() {
		close(c.done)
		if c.stdin != nil {
			_ = c.stdin.Close()
		}
		if c.stderr != nil {
			_ = c.stderr.Close()
		}
		if c.cmd != nil && c.cmd.Process != nil {
			err = c.cmd.Process.Kill()
			_, _ = c.cmd.Process.Wait()
		}
	})
	return err
}

func (c *Client) Events() <-chan Event {
	return c.events
}

func (c *Client) Bootstrap(ctx context.Context, cfg SessionConfig) (string, error) {
	var initializeResult map[string]any
	if err := c.Call(ctx, "initialize", map[string]any{
		"clientInfo": map[string]any{
			"name":    "ASC Studio",
			"version": "0.1.0",
		},
	}, &initializeResult); err != nil {
		return "", err
	}

	var session SessionResult
	if err := c.Call(ctx, "session/new", cfg, &session); err != nil {
		return "", err
	}
	if session.SessionID == "" {
		return "", errors.New("agent returned empty session ID")
	}
	return session.SessionID, nil
}

func (c *Client) Prompt(ctx context.Context, sessionID string, prompt string) (PromptResult, []Event, error) {
	var result PromptResult
	if err := c.Call(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt": map[string]any{
			"role":    "user",
			"content": prompt,
		},
	}, &result); err != nil {
		return PromptResult{}, nil, err
	}

	var events []Event
	for {
		select {
		case event := <-c.events:
			events = append(events, event)
		default:
			return result, events, nil
		}
	}
}

func (c *Client) Call(ctx context.Context, method string, params interface{}, out interface{}) error {
	id := c.nextID.Add(1)
	responseCh := make(chan rpcResponse, 1)
	cleanupPending := func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}

	c.mu.Lock()
	c.pending[id] = responseCh
	c.mu.Unlock()

	request := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		cleanupPending()
		return fmt.Errorf("marshal request: %w", err)
	}
	payload = append(payload, '\n')
	if _, err := c.stdin.Write(payload); err != nil {
		cleanupPending()
		return fmt.Errorf("write request: %w", err)
	}

	select {
	case <-ctx.Done():
		cleanupPending()
		return ctx.Err()
	case response := <-responseCh:
		if response.Error != nil {
			return fmt.Errorf("%s: %s", method, response.Error.Message)
		}
		if out == nil || len(response.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(response.Result, out); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
		return nil
	}
}

func (c *Client) drainStderr() {
	if c.stderr == nil {
		return
	}
	_, _ = io.Copy(io.Discard, c.stderr)
}

func (c *Client) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)

		var envelope struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}

		if envelope.ID == nil && envelope.Method != "" {
			select {
			case c.events <- Event{Method: envelope.Method, Params: envelope.Params}:
			default:
			}
			continue
		}
		if envelope.ID == nil {
			continue
		}

		var response rpcResponse
		if err := json.Unmarshal(line, &response); err != nil {
			continue
		}

		c.mu.Lock()
		responseCh := c.pending[response.ID]
		delete(c.pending, response.ID)
		c.mu.Unlock()
		if responseCh != nil {
			responseCh <- response
		}
	}

	// Scanner finished (EOF or error): the agent process has exited or the
	// pipe broke. Send an error response to every pending caller so they
	// unblock immediately instead of waiting for their context to expire.
	c.mu.Lock()
	stale := c.pending
	c.pending = make(map[int64]chan rpcResponse)
	c.mu.Unlock()

	for _, ch := range stale {
		ch <- rpcResponse{
			Error: &rpcError{Code: -32000, Message: "agent process exited"},
		}
	}
}
