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

func TestSharedEnvVarsCommandHierarchy(t *testing.T) {
	cmd := WebXcodeCloudCommand()
	envVarsCmd := findSub(cmd, "env-vars")
	if envVarsCmd == nil {
		t.Fatal("expected 'env-vars' subcommand")
	}
	sharedCmd := findSub(envVarsCmd, "shared")
	if sharedCmd == nil {
		t.Fatal("expected 'shared' subcommand under env-vars")
		return
	}
	if len(sharedCmd.Subcommands) != 3 {
		t.Fatalf("expected 3 subcommands (list, set, delete), got %d", len(sharedCmd.Subcommands))
	}
	names := map[string]bool{}
	for _, sub := range sharedCmd.Subcommands {
		names[sub.Name] = true
	}
	for _, name := range []string{"list", "set", "delete"} {
		if !names[name] {
			t.Fatalf("expected %q subcommand", name)
		}
	}
}

func TestSharedEnvVarsGroupReturnsErrHelp(t *testing.T) {
	cmd := webXcodeCloudEnvVarsSharedCommand()
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestSharedEnvVarsList_Success(t *testing.T) {
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
					body := `[
						{
							"id":"var-1","name":"SHARED_KEY",
							"value":{"plaintext":"abc123"},
							"is_locked":false,
							"related_workflow_summaries":[{"id":"wf-1","name":"Deploy","disabled":false,"locked":false}]
						},
						{
							"id":"var-2","name":"SHARED_SECRET",
							"value":{"redacted_value":""},
							"is_locked":true,
							"related_workflow_summaries":[]
						}
					]`
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

	cmd := webXcodeCloudEnvVarsSharedListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	if !strings.Contains(stdout, "SHARED_KEY") {
		t.Fatalf("expected SHARED_KEY in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "SHARED_SECRET") {
		t.Fatalf("expected SHARED_SECRET in output, got %q", stdout)
	}
}

func TestSharedEnvVarsList_EmptyList(t *testing.T) {
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
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(`[]`)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsSharedListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"variables"`) {
		t.Fatalf("expected variables key in output, got %q", stdout)
	}
}

func TestSharedEnvVarsList_MissingProductID(t *testing.T) {
	cmd := webXcodeCloudEnvVarsSharedListCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
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

func TestSharedEnvVarsList_TableOutput(t *testing.T) {
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
					body := `[
						{
							"id":"var-1","name":"MY_VAR",
							"value":{"plaintext":"hello"},
							"is_locked":false,
							"related_workflow_summaries":[{"id":"wf-1","name":"Deploy","disabled":false,"locked":false}]
						},
						{
							"id":"var-2","name":"MY_SECRET",
							"value":{"redacted_value":""},
							"is_locked":true,
							"related_workflow_summaries":[]
						}
					]`
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

	cmd := webXcodeCloudEnvVarsSharedListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--output", "table",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	for _, token := range []string{"Name", "Type", "Value", "Locked", "Workflows", "MY_VAR", "plaintext", "hello", "MY_SECRET", "secret", "(redacted)", "Deploy"} {
		if !strings.Contains(stdout, token) {
			t.Fatalf("expected table output to include %q, got %q", token, stdout)
		}
	}
}

func TestSharedEnvVarsList_JSONOutput(t *testing.T) {
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
					body := `[{
						"id":"var-1","name":"FOO",
						"value":{"plaintext":"bar"},
						"is_locked":false,
						"related_workflow_summaries":[]
					}]`
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

	cmd := webXcodeCloudEnvVarsSharedListCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	var result CISharedEnvVarsListResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if result.ProductID != "prod-1" {
		t.Fatalf("expected product_id %q, got %q", "prod-1", result.ProductID)
	}
	if len(result.Variables) != 1 || result.Variables[0].Name != "FOO" {
		t.Fatalf("unexpected variables: %+v", result.Variables)
	}
}

func TestSharedEnvVarsSetPlaintext_Success(t *testing.T) {
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
					path := req.URL.Path
					switch {
					case req.Method == http.MethodGet && strings.Contains(path, "/product-environment-variables"):
						// List returns empty (new var)
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`[]`)),
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
							Body:       io.NopCloser(strings.NewReader(`{"id":"new-uuid","name":"MY_VAR","value":{"plaintext":"hello"},"is_locked":false,"related_workflow_summaries":[]}`)),
							Request:    req,
						}, nil
					}
					t.Fatalf("unexpected request: %s %s", req.Method, path)
					return nil, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsSharedSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
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
	var setResult CISharedEnvVarsSetResult
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
	if setResult.Locked {
		t.Fatalf("expected locked=false")
	}
	// Verify PUT body contains plaintext value
	if !strings.Contains(string(putBody), `"plaintext"`) {
		t.Fatalf("expected plaintext in PUT body, got %q", string(putBody))
	}
	if !strings.Contains(string(putBody), "hello") {
		t.Fatalf("expected value 'hello' in PUT body, got %q", string(putBody))
	}
}

func TestSharedEnvVarsSetPlaintext_UpdateExisting(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	var putPath string
	var putBody []byte

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
					case req.Method == http.MethodGet && strings.Contains(path, "/product-environment-variables"):
						body := `[{"id":"existing-id","name":"MY_VAR","value":{"plaintext":"old"},"is_locked":false,"related_workflow_summaries":[{"id":"wf-1","name":"Deploy","disabled":false,"locked":false}]}]`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					case req.Method == http.MethodPut:
						putPath = path
						var err error
						putBody, err = io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("failed to read PUT body: %v", err)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{"id":"existing-id","name":"MY_VAR","value":{"plaintext":"updated"},"is_locked":false,"related_workflow_summaries":[]}`)),
							Request:    req,
						}, nil
					}
					return nil, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsSharedSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
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
	var setResult CISharedEnvVarsSetResult
	if err := json.Unmarshal([]byte(stdout), &setResult); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if setResult.Action != "updated" {
		t.Fatalf("expected action %q, got %q", "updated", setResult.Action)
	}
	// Verify PUT path reuses existing ID
	if !strings.Contains(putPath, "existing-id") {
		t.Fatalf("expected PUT to reuse existing ID, got path %q", putPath)
	}
	// Verify PUT body contains the updated value
	if !strings.Contains(string(putBody), "updated") {
		t.Fatalf("expected 'updated' in PUT body, got %q", string(putBody))
	}
	// Verify preserved workflow IDs
	if !strings.Contains(string(putBody), "wf-1") {
		t.Fatalf("expected preserved workflow ID 'wf-1' in PUT body, got %q", string(putBody))
	}
}

func TestSharedEnvVarsSetSecret_Success(t *testing.T) {
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
					case req.Method == http.MethodGet && strings.Contains(path, "/product-environment-variables"):
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`[]`)),
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
							Body:       io.NopCloser(strings.NewReader(`{"id":"new-uuid","name":"MY_SECRET","value":{"redacted_value":""},"is_locked":true,"related_workflow_summaries":[]}`)),
							Request:    req,
						}, nil
					}
					t.Fatalf("unexpected request: %s %s", req.Method, path)
					return nil, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsSharedSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--name", "MY_SECRET",
		"--value", "s3cret",
		"--secret",
		"--locked",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	var setResult CISharedEnvVarsSetResult
	if err := json.Unmarshal([]byte(stdout), &setResult); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if setResult.Name != "MY_SECRET" {
		t.Fatalf("expected name %q, got %q", "MY_SECRET", setResult.Name)
	}
	if setResult.Type != "secret" {
		t.Fatalf("expected type %q, got %q", "secret", setResult.Type)
	}
	if !setResult.Locked {
		t.Fatalf("expected locked=true")
	}
	// Verify PUT body contains ciphertext (not plaintext)
	if !strings.Contains(string(putBody), `"ciphertext"`) {
		t.Fatalf("expected ciphertext in PUT body, got %q", string(putBody))
	}
	if strings.Contains(string(putBody), "s3cret") {
		t.Fatalf("plaintext value should not appear in PUT body")
	}
	if !strings.Contains(string(putBody), `"is_locked":true`) {
		t.Fatalf("expected is_locked:true in PUT body, got %q", string(putBody))
	}
}

func TestSharedEnvVarsSet_MissingFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing product-id",
			args:    []string{"--name", "X", "--value", "Y"},
			wantErr: "--product-id is required",
		},
		{
			name:    "missing name",
			args:    []string{"--product-id", "prod-1", "--value", "Y"},
			wantErr: "--name is required",
		},
		{
			name:    "missing value",
			args:    []string{"--product-id", "prod-1", "--name", "X"},
			wantErr: "--value is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := webXcodeCloudEnvVarsSharedSetCommand()
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

func TestSharedEnvVarsSetSecret_EncryptionKeyFetchFails(t *testing.T) {
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
					if strings.Contains(path, "/product-environment-variables") {
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`[]`)),
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

	cmd := webXcodeCloudEnvVarsSharedSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
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

func TestSharedEnvVarsDelete_Success(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	var deletePath string

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
					case req.Method == http.MethodGet && strings.Contains(path, "/product-environment-variables"):
						body := `[
							{"id":"var-1","name":"DELETE_ME","value":{"plaintext":"bye"},"is_locked":false,"related_workflow_summaries":[]},
							{"id":"var-2","name":"KEEP_ME","value":{"plaintext":"stay"},"is_locked":false,"related_workflow_summaries":[]}
						]`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					case req.Method == http.MethodDelete:
						deletePath = path
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

	cmd := webXcodeCloudEnvVarsSharedDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
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
	var delResult CISharedEnvVarsDeleteResult
	if err := json.Unmarshal([]byte(stdout), &delResult); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if delResult.Name != "DELETE_ME" {
		t.Fatalf("expected name %q, got %q", "DELETE_ME", delResult.Name)
	}
	if delResult.ProductID != "prod-1" {
		t.Fatalf("expected product_id %q, got %q", "prod-1", delResult.ProductID)
	}
	// Verify DELETE was called with the correct var ID
	if !strings.Contains(deletePath, "var-1") {
		t.Fatalf("expected DELETE path to contain var-1, got %q", deletePath)
	}
}

func TestSharedEnvVarsDelete_NotFound(t *testing.T) {
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
					body := `[{"id":"var-1","name":"OTHER","value":{"plaintext":"val"},"is_locked":false,"related_workflow_summaries":[]}]`
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

	cmd := webXcodeCloudEnvVarsSharedDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
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
		if strings.Contains(err.Error(), "xcode-cloud env-vars shared delete failed:") {
			t.Fatalf("expected raw not-found error, got %v", err)
		}
		if strings.Contains(err.Error(), "web session is unauthorized or expired") {
			t.Fatalf("expected no auth hint for not-found error, got %v", err)
		}
	})
}

func TestSharedEnvVarsDelete_MissingFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing product-id",
			args:    []string{"--name", "X"},
			wantErr: "--product-id is required",
		},
		{
			name:    "missing name",
			args:    []string{"--product-id", "prod-1", "--confirm"},
			wantErr: "--name is required",
		},
		{
			name:    "missing confirm",
			args:    []string{"--product-id", "prod-1", "--name", "X"},
			wantErr: "--confirm is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := webXcodeCloudEnvVarsSharedDeleteCommand()
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

func TestSharedEnvVarsAllCommandsHaveUsageFunc(t *testing.T) {
	cmd := webXcodeCloudEnvVarsSharedCommand()
	if cmd.UsageFunc == nil {
		t.Fatalf("shared command should have UsageFunc set")
	}
	for _, sub := range cmd.Subcommands {
		if sub.UsageFunc == nil {
			t.Fatalf("subcommand %q should have UsageFunc set", sub.Name)
		}
	}
}

func TestSharedEnvVarsSetWithWorkflowIDs(t *testing.T) {
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
					path := req.URL.Path
					switch {
					case req.Method == http.MethodGet && strings.Contains(path, "/product-environment-variables"):
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`[]`)),
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
							Body:       io.NopCloser(strings.NewReader(`{"id":"new-uuid","name":"MY_VAR","value":{"plaintext":"hello"},"is_locked":false,"related_workflow_summaries":[]}`)),
							Request:    req,
						}, nil
					}
					return nil, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudEnvVarsSharedSetCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--name", "MY_VAR",
		"--value", "hello",
		"--workflow-ids", "wf-1,wf-2",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	// Verify PUT body contains workflow IDs
	if !strings.Contains(string(putBody), "wf-1") || !strings.Contains(string(putBody), "wf-2") {
		t.Fatalf("expected workflow IDs in PUT body, got %q", string(putBody))
	}
}
