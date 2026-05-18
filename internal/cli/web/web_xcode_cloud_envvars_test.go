package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestEnvVarsCommandHierarchy(t *testing.T) {
	cmd := WebXcodeCloudCommand()
	envVarsCmd := findSub(cmd, "env-vars")
	if envVarsCmd == nil {
		t.Fatal("expected 'env-vars' subcommand")
		return
	}
	if len(envVarsCmd.Subcommands) != 4 {
		t.Fatalf("expected 4 subcommands (list, set, delete, shared), got %d", len(envVarsCmd.Subcommands))
	}
	names := map[string]bool{}
	for _, sub := range envVarsCmd.Subcommands {
		names[sub.Name] = true
	}
	for _, name := range []string{"list", "set", "delete", "shared"} {
		if !names[name] {
			t.Fatalf("expected %q subcommand", name)
		}
	}
}

func TestEnvVarsGroupReturnsErrHelp(t *testing.T) {
	cmd := webXcodeCloudEnvVarsCommand()
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestEnvVarsList_Success(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					body := `{
						"id": "wf-1",
						"content": {
							"name": "Test WF",
							"environment_variables": [
								{"id":"ev-1","name":"API_KEY","value":{"plaintext":"abc123"}},
								{"id":"ev-2","name":"SECRET","value":{"redacted_value":"***"}}
							]
						}
					}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	// Default output is JSON
	if !strings.Contains(stdout, "API_KEY") {
		t.Fatalf("expected API_KEY in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "SECRET") {
		t.Fatalf("expected SECRET in output, got %q", stdout)
	}
}

func TestEnvVarsList_EmptyList(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					body := `{"id":"wf-1","content":{"name":"Test WF","environment_variables":[]}}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	// JSON output with empty variables
	if !strings.Contains(stdout, `"variables"`) {
		t.Fatalf("expected variables key in output, got %q", stdout)
	}
}

func TestEnvVarsList_MissingProductID(t *testing.T) {
	cmd := webXcodeCloudEnvVarsListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--workflow-id", "wf-1",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--product-id is required") {
		t.Fatalf("expected product-id error in stderr, got %q", stderr)
	}
}

func TestEnvVarsList_MissingWorkflowID(t *testing.T) {
	cmd := webXcodeCloudEnvVarsListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--product-id", "prod-1",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--workflow-id is required") {
		t.Fatalf("expected workflow-id error in stderr, got %q", stderr)
	}
}

func TestEnvVarsList_TableOutput(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					body := `{
						"id": "wf-1",
						"content": {
							"name": "Test WF",
							"environment_variables": [
								{"id":"ev-1","name":"MY_VAR","value":{"plaintext":"hello"}},
								{"id":"ev-2","name":"MY_SECRET","value":{"redacted_value":"***"}}
							]
						}
					}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--output", "table",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	for _, token := range []string{"Name", "Type", "Value", "MY_VAR", "plaintext", "hello", "MY_SECRET", "secret", "(redacted)"} {
		if !strings.Contains(stdout, token) {
			t.Fatalf("expected table output to include %q, got %q", token, stdout)
		}
	}
}

func TestEnvVarsList_JSONOutput(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					body := `{
						"id": "wf-1",
						"content": {
							"name": "Test WF",
							"environment_variables": [
								{"id":"ev-1","name":"FOO","value":{"plaintext":"bar"}}
							]
						}
					}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	var result CIEnvVarsListResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if result.WorkflowID != "wf-1" {
		t.Fatalf("expected workflow_id %q, got %q", "wf-1", result.WorkflowID)
	}
	if len(result.Variables) != 1 || result.Variables[0].Name != "FOO" {
		t.Fatalf("unexpected variables: %+v", result.Variables)
	}
}

func TestEnvVarsSetPlaintext_Success(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	var putBody []byte

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == http.MethodGet {
						body := `{"id":"wf-1","content":{"name":"WF","environment_variables":[]}}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					}
					if req.Method == http.MethodPut {
						var err error
						putBody, err = io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("failed to read PUT body: %v", err)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{}`)),
							Request:    req,
						}, nil
					}
					t.Fatalf("unexpected method: %s", req.Method)
					return nil, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--name", "MY_VAR",
		"--value", "hello",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	var setResult CIEnvVarsSetResult
	if err := json.Unmarshal([]byte(stdout), &setResult); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if setResult.Name != "MY_VAR" {
		t.Fatalf("expected name %q, got %q", "MY_VAR", setResult.Name)
	}
	if setResult.Type != "plaintext" {
		t.Fatalf("expected type %q, got %q", "plaintext", setResult.Type)
	}
	if setResult.Action != "created" {
		t.Fatalf("expected action %q, got %q", "created", setResult.Action)
	}
	if setResult.WorkflowName != "WF" {
		t.Fatalf("expected workflow_name %q, got %q", "WF", setResult.WorkflowName)
	}
	// Verify PUT body contains the plaintext value
	if !strings.Contains(string(putBody), `"plaintext"`) {
		t.Fatalf("expected plaintext in PUT body, got %q", string(putBody))
	}
	if !strings.Contains(string(putBody), "hello") {
		t.Fatalf("expected value 'hello' in PUT body, got %q", string(putBody))
	}
}

func TestEnvVarsSetPlaintext_UpdateExisting(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	var putBody []byte

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == http.MethodGet {
						body := `{"id":"wf-1","content":{"name":"WF","environment_variables":[{"id":"existing-id","name":"MY_VAR","value":{"plaintext":"old"}}]}}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					}
					if req.Method == http.MethodPut {
						var err error
						putBody, err = io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("failed to read PUT body: %v", err)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{}`)),
							Request:    req,
						}, nil
					}
					return nil, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--name", "MY_VAR",
		"--value", "updated",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	var setResult CIEnvVarsSetResult
	if err := json.Unmarshal([]byte(stdout), &setResult); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if setResult.Action != "updated" {
		t.Fatalf("expected action %q, got %q", "updated", setResult.Action)
	}
	// Verify the PUT body contains the updated value and reuses the existing ID
	if !strings.Contains(string(putBody), "updated") {
		t.Fatalf("expected 'updated' in PUT body, got %q", string(putBody))
	}
	if !strings.Contains(string(putBody), "existing-id") {
		t.Fatalf("expected existing ID preserved in PUT body, got %q", string(putBody))
	}
	// The PUT body is the raw content (not wrapped), verify no duplication
	vars, _ := webcore.ExtractEnvVars(json.RawMessage(putBody))
	if len(vars) != 1 {
		t.Fatalf("expected 1 var (no duplication), got %d", len(vars))
	}
}

func TestEnvVarsSet_MissingFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing product-id",
			args:    []string{"--workflow-id", "wf-1", "--name", "X", "--value", "Y"},
			wantErr: "--product-id is required",
		},
		{
			name:    "missing workflow-id",
			args:    []string{"--product-id", "prod-1", "--name", "X", "--value", "Y"},
			wantErr: "--workflow-id is required",
		},
		{
			name:    "missing name",
			args:    []string{"--product-id", "prod-1", "--workflow-id", "wf-1", "--value", "Y"},
			wantErr: "--name is required",
		},
		{
			name:    "missing value",
			args:    []string{"--product-id", "prod-1", "--workflow-id", "wf-1", "--name", "X"},
			wantErr: "--value is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := webXcodeCloudEnvVarsSetCommand()
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			_, stderr := captureOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", tt.wantErr, stderr)
			}
		})
	}
}

func TestEnvVarsSetSecret_Success(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	var putBody []byte
	serverKeyB64 := "0xm9f0gX7lzArxrChNrDVUR3MKxueb1DdheWBeLndCVOqoiEsT2jxqZW6cHsIuDGDykvYWgQ1qaPBSxCNFXEUg=="

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					path := req.URL.Path
					switch {
					case req.Method == http.MethodGet && strings.Contains(path, "/workflows-v15/"):
						body := `{"id":"wf-1","content":{"name":"WF","environment_variables":[]}}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					case req.Method == http.MethodGet && strings.Contains(path, "/keys/client-encryption"):
						body := `{"key":"` + serverKeyB64 + `"}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					case req.Method == http.MethodPut:
						var err error
						putBody, err = io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("failed to read PUT body: %v", err)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{}`)),
							Request:    req,
						}, nil
					}
					t.Fatalf("unexpected request: %s %s", req.Method, path)
					return nil, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--name", "MY_SECRET",
		"--value", "s3cret",
		"--secret",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	var setResult CIEnvVarsSetResult
	if err := json.Unmarshal([]byte(stdout), &setResult); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if setResult.Name != "MY_SECRET" {
		t.Fatalf("expected name %q, got %q", "MY_SECRET", setResult.Name)
	}
	if setResult.Type != "secret" {
		t.Fatalf("expected type %q, got %q", "secret", setResult.Type)
	}
	if setResult.WorkflowName != "WF" {
		t.Fatalf("expected workflow_name %q, got %q", "WF", setResult.WorkflowName)
	}
	// Verify PUT body contains ciphertext (not plaintext)
	if !strings.Contains(string(putBody), `"ciphertext"`) {
		t.Fatalf("expected ciphertext in PUT body, got %q", string(putBody))
	}
	if strings.Contains(string(putBody), "s3cret") {
		t.Fatalf("plaintext value should not appear in PUT body")
	}
}

func TestEnvVarsSetSecret_EncryptionKeyFetchFails(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					path := req.URL.Path
					if strings.Contains(path, "/workflows-v15/") {
						body := `{"id":"wf-1","content":{"name":"WF","environment_variables":[]}}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					}
					if strings.Contains(path, "/keys/client-encryption") {
						return &http.Response{
							StatusCode: http.StatusInternalServerError,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{"error":"server error"}`)),
							Request:    req,
						}, nil
					}
					return nil, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--name", "MY_SECRET",
		"--value", "s3cret",
		"--secret",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if err == nil {
			t.Fatal("expected error when encryption key fetch fails")
		}
		if !strings.Contains(err.Error(), "encryption key") {
			t.Fatalf("expected encryption key error, got %v", err)
		}
	})
}

func TestEnvVarsDelete_Success(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	var putBody []byte

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == http.MethodGet {
						body := `{"id":"wf-1","content":{"name":"WF","environment_variables":[{"id":"ev-1","name":"DELETE_ME","value":{"plaintext":"bye"}},{"id":"ev-2","name":"KEEP_ME","value":{"plaintext":"stay"}}]}}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					}
					if req.Method == http.MethodPut {
						var err error
						putBody, err = io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("failed to read PUT body: %v", err)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{}`)),
							Request:    req,
						}, nil
					}
					return nil, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--name", "DELETE_ME",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	var delResult CIEnvVarsDeleteResult
	if err := json.Unmarshal([]byte(stdout), &delResult); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if delResult.Name != "DELETE_ME" {
		t.Fatalf("expected name %q, got %q", "DELETE_ME", delResult.Name)
	}
	if delResult.WorkflowName != "WF" {
		t.Fatalf("expected workflow_name %q, got %q", "WF", delResult.WorkflowName)
	}
	if delResult.WorkflowID != "wf-1" {
		t.Fatalf("expected workflow_id %q, got %q", "wf-1", delResult.WorkflowID)
	}
	// Verify PUT body does not contain deleted var but keeps the other
	if strings.Contains(string(putBody), "DELETE_ME") {
		t.Fatalf("deleted var should not appear in PUT body")
	}
	if !strings.Contains(string(putBody), "KEEP_ME") {
		t.Fatalf("kept var should appear in PUT body, got %q", string(putBody))
	}
}

func TestEnvVarsDelete_NotFound(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					body := `{"id":"wf-1","content":{"name":"WF","environment_variables":[{"id":"ev-1","name":"OTHER","value":{"plaintext":"val"}}]}}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--name", "NONEXISTENT",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if err == nil {
			t.Fatal("expected error for nonexistent var")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected 'not found' error, got %v", err)
		}
		if strings.Contains(err.Error(), "xcode-cloud env-vars delete failed:") {
			t.Fatalf("expected raw not-found error, got %v", err)
		}
		if strings.Contains(err.Error(), "web session is unauthorized or expired") {
			t.Fatalf("expected no auth hint for not-found error, got %v", err)
		}
	})
}

func TestEnvVarsDelete_MissingFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing product-id",
			args:    []string{"--workflow-id", "wf-1", "--name", "X"},
			wantErr: "--product-id is required",
		},
		{
			name:    "missing workflow-id",
			args:    []string{"--product-id", "prod-1", "--name", "X"},
			wantErr: "--workflow-id is required",
		},
		{
			name:    "missing name",
			args:    []string{"--product-id", "prod-1", "--workflow-id", "wf-1", "--confirm"},
			wantErr: "--name is required",
		},
		{
			name:    "missing confirm",
			args:    []string{"--product-id", "prod-1", "--workflow-id", "wf-1", "--name", "X"},
			wantErr: "--confirm is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := webXcodeCloudEnvVarsDeleteCommand()
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			_, stderr := captureOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", tt.wantErr, stderr)
			}
		})
	}
}

func TestEnvVarsAllCommandsHaveUsageFunc(t *testing.T) {
	cmd := webXcodeCloudEnvVarsCommand()
	if cmd.UsageFunc == nil {
		t.Fatalf("env-vars command should have UsageFunc set")
	}
	for _, sub := range cmd.Subcommands {
		if sub.UsageFunc == nil {
			t.Fatalf("subcommand %q should have UsageFunc set", sub.Name)
		}
	}
}
