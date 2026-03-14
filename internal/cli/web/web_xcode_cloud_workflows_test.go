package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWorkflowsCommandHierarchy(t *testing.T) {
	cmd := WebXcodeCloudCommand()
	workflowsCmd := findSub(cmd, "workflows")
	if workflowsCmd == nil {
		t.Fatal("expected 'workflows' subcommand")
	}
	if len(workflowsCmd.Subcommands) != 6 {
		t.Fatalf("expected 6 subcommands (describe, create, options, edit, enable, disable), got %d", len(workflowsCmd.Subcommands))
	}
	names := map[string]bool{}
	for _, sub := range workflowsCmd.Subcommands {
		names[sub.Name] = true
	}
	for _, name := range []string{"describe", "create", "options", "edit", "enable", "disable"} {
		if !names[name] {
			t.Fatalf("expected %q subcommand", name)
		}
	}
}

func TestWorkflowsGroupReturnsErrHelp(t *testing.T) {
	cmd := webXcodeCloudWorkflowsCommand()
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestWorkflowsDescribeSuccess(t *testing.T) {
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
					if req.Method != http.MethodGet {
						t.Fatalf("expected GET, got %s", req.Method)
					}
					if !strings.Contains(req.URL.Path, "/products/prod-1/workflows-v15/wf-1") {
						t.Fatalf("unexpected path: %s", req.URL.Path)
					}
					body := `{
						"id":"wf-1",
						"content":{
							"name":"Default",
							"description":"Main workflow",
							"disabled":true,
							"locked":false,
							"xcode_version":"latest:all",
							"macos_version":"15",
							"start_conditions":[{"type":"branch"}],
							"actions":[{"name":"Archive"}],
							"post_actions":[{"name":"TestFlight"}],
							"clean":true,
							"container_file_path":"FoundationLab.xcodeproj",
							"repo":{"id":"repo-1"},
							"product_environment_variables":["var-1","var-2"]
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

	cmd := webXcodeCloudWorkflowDescribeCommand()
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

	var result CIWorkflowDescribeResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}

	if result.ProductID != "prod-1" || result.WorkflowID != "wf-1" {
		t.Fatalf("unexpected product/workflow IDs: %+v", result)
	}
	if result.Name != "Default" || result.Description != "Main workflow" {
		t.Fatalf("unexpected workflow metadata: %+v", result)
	}
	if !result.Disabled {
		t.Fatalf("expected disabled=true, got false")
	}
	if len(result.Actions) == 0 || len(result.StartConditions) == 0 || len(result.PostActions) == 0 {
		t.Fatalf("expected workflow sections to be present: %+v", result)
	}
	if len(result.ProductEnvironmentVariables) != 2 {
		t.Fatalf("expected 2 shared env var refs, got %d", len(result.ProductEnvironmentVariables))
	}
}

func TestWorkflowsDescribeMissingFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing product-id",
			args:    []string{"--workflow-id", "wf-1"},
			wantErr: "--product-id is required",
		},
		{
			name:    "missing workflow-id",
			args:    []string{"--product-id", "prod-1"},
			wantErr: "--workflow-id is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := webXcodeCloudWorkflowDescribeCommand()
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

func TestWorkflowsCreateSuccess(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	payloadFile, err := os.CreateTemp(t.TempDir(), "workflow-create-*.json")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	payload := `{
		"name":"Nightly Build",
		"description":"Creates builds and notifies testers",
		"disabled":false,
		"locked":true,
		"xcode_version":{"name":"Xcode 16.3"},
		"macos_version":{"name":"macOS 15"},
		"start_conditions":{"manual":{"branch":{"branch":{"kind":"branch","is_all_match":true}}}},
		"actions":[{"default_name":"Archive - iOS","action_type":"archive"}],
		"post_actions":[{"name":"Notify","type":"notification"}],
		"clean":true,
		"container_file_path":"Example.xcodeproj/project.xcworkspace",
		"repo":{"id":"repo-1"},
		"product_environment_variables":["var-1"]
	}`
	if _, err := payloadFile.WriteString(payload); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := payloadFile.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	putCalls := 0
	var (
		putBody []byte
		gotPath string
	)

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.Method {
					case http.MethodGet:
						if req.URL.Path != "/ci/api/teams/team-uuid/products/prod-1/workflows-v15/wf-new" {
							t.Fatalf("unexpected GET path: %s", req.URL.Path)
						}
						return &http.Response{
							StatusCode: http.StatusNotFound,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
							Request:    req,
						}, nil
					case http.MethodPut:
						putCalls++
						gotPath = req.URL.Path
						var err error
						putBody, err = io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("failed reading PUT body: %v", err)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{}`)),
							Request:    req,
						}, nil
					default:
						t.Fatalf("unexpected method: %s", req.Method)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudWorkflowCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-new",
		"--file", payloadFile.Name(),
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if putCalls != 1 {
		t.Fatalf("expected 1 PUT call, got %d", putCalls)
	}
	if gotPath != "/ci/api/teams/team-uuid/products/prod-1/workflows-v15/wf-new" {
		t.Fatalf("unexpected path: %s", gotPath)
	}

	var putPayload map[string]any
	if err := json.Unmarshal(putBody, &putPayload); err != nil {
		t.Fatalf("failed to unmarshal PUT body: %v", err)
	}
	if putPayload["name"] != "Nightly Build" {
		t.Fatalf("expected workflow name in PUT body, got %#v", putPayload["name"])
	}
	if putPayload["locked"] != true {
		t.Fatalf("expected locked=true in PUT body, got %#v", putPayload["locked"])
	}
	if _, ok := putPayload["post_actions"]; !ok {
		t.Fatalf("expected post_actions in PUT body, got %#v", putPayload)
	}

	var result CIWorkflowCreateResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if !result.Created {
		t.Fatal("expected created=true")
	}
	if result.ProductID != "prod-1" || result.WorkflowID != "wf-new" {
		t.Fatalf("unexpected product/workflow IDs: %+v", result)
	}
	if result.Name != "Nightly Build" || result.Description != "Creates builds and notifies testers" {
		t.Fatalf("unexpected workflow metadata: %+v", result)
	}
	if len(result.PostActions) == 0 {
		t.Fatalf("expected post actions in output: %+v", result)
	}
}

func TestWorkflowsCreateMissingFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing product-id",
			args:    []string{"--file", "workflow.json"},
			wantErr: "--product-id is required",
		},
		{
			name:    "missing file",
			args:    []string{"--product-id", "prod-1"},
			wantErr: "--file is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := webXcodeCloudWorkflowCreateCommand()
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

func TestWorkflowsCreateRejectsExistingWorkflowID(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	payloadFile, err := os.CreateTemp(t.TempDir(), "workflow-create-*.json")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if _, err := payloadFile.WriteString(`{"name":"Nightly Build"}`); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := payloadFile.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.Method {
					case http.MethodGet:
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{"id":"wf-existing","content":{"name":"Existing"}}`)),
							Request:    req,
						}, nil
					case http.MethodPut:
						t.Fatal("did not expect PUT when workflow already exists")
						return nil, nil
					default:
						t.Fatalf("unexpected method: %s", req.Method)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudWorkflowCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-existing",
		"--file", payloadFile.Name(),
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err = cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for existing workflow id")
	}
	if !strings.Contains(err.Error(), `workflow "wf-existing" already exists`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkflowsEditSuccess(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	patchFile, err := os.CreateTemp(t.TempDir(), "workflow-patch-*.json")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	patch := `{
		"description":"Updated workflow",
		"clean":false,
		"start_conditions":{
			"branch":{
				"files":{
					"matchers":[{"directory":"Sources","file_extension":"swift"}]
				}
			}
		}
	}`
	if _, err := patchFile.WriteString(patch); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	putCalls := 0
	var putBody []byte

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.Method {
					case http.MethodGet:
						body := `{
							"id":"wf-1",
							"content":{
								"name":"Default",
								"description":"Main workflow",
								"disabled":false,
								"locked":false,
								"xcode_version":"latest:all",
								"macos_version":"15",
								"start_conditions":{
									"branch":{
										"source":{"kind":"branch","value":"main"},
										"files":{
											"mode":"trigger_if_any_file_match",
											"matchers":[{"directory":"Sources","file_name":"App.swift"}]
										}
									}
								},
								"actions":[{"name":"Archive"}],
								"post_actions":[{"name":"Notify"}],
								"clean":true,
								"container_file_path":"FoundationLab.xcodeproj",
								"repo":{"id":"repo-1"},
								"product_environment_variables":["var-1"],
								"custom":"keep"
							}
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					case http.MethodPut:
						putCalls++
						putBody, err = io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("failed reading PUT body: %v", err)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{}`)),
							Request:    req,
						}, nil
					default:
						t.Fatalf("unexpected method: %s", req.Method)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudWorkflowEditCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--patch-file", patchFile.Name(),
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if putCalls != 1 {
		t.Fatalf("expected 1 PUT call, got %d", putCalls)
	}

	var putPayload map[string]any
	if err := json.Unmarshal(putBody, &putPayload); err != nil {
		t.Fatalf("failed to unmarshal PUT body: %v", err)
	}
	if putPayload["custom"] != "keep" {
		t.Fatalf("expected custom field to be preserved, got %#v", putPayload["custom"])
	}
	if putPayload["description"] != "Updated workflow" {
		t.Fatalf("expected description update, got %#v", putPayload["description"])
	}
	if putPayload["clean"] != false {
		t.Fatalf("expected clean=false, got %#v", putPayload["clean"])
	}

	startConditions, ok := putPayload["start_conditions"].(map[string]any)
	if !ok {
		t.Fatalf("expected start_conditions object, got %#v", putPayload["start_conditions"])
	}
	branch, ok := startConditions["branch"].(map[string]any)
	if !ok {
		t.Fatalf("expected branch object, got %#v", startConditions["branch"])
	}
	source, ok := branch["source"].(map[string]any)
	if !ok || source["kind"] != "branch" {
		t.Fatalf("expected branch source to be preserved, got %#v", branch["source"])
	}

	var result CIWorkflowEditResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if !result.Changed {
		t.Fatal("expected changed=true")
	}
	if result.Name != "Default" || result.Description != "Updated workflow" {
		t.Fatalf("unexpected workflow metadata: %+v", result)
	}
}

func TestWorkflowsEditNullDescriptionNormalizesToEmptyString(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	patchFile, err := os.CreateTemp(t.TempDir(), "workflow-patch-*.json")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if _, err := patchFile.WriteString(`{"description":null}`); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	putCalls := 0
	var putBody []byte

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.Method {
					case http.MethodGet:
						body := `{
							"id":"wf-1",
							"content":{
								"name":"Default",
								"description":"Temporary description",
								"disabled":false,
								"locked":false
							}
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					case http.MethodPut:
						putCalls++
						putBody, err = io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("failed reading PUT body: %v", err)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{}`)),
							Request:    req,
						}, nil
					default:
						t.Fatalf("unexpected method: %s", req.Method)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudWorkflowEditCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--patch-file", patchFile.Name(),
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if putCalls != 1 {
		t.Fatalf("expected 1 PUT call, got %d", putCalls)
	}
	if !strings.Contains(string(putBody), `"description":""`) {
		t.Fatalf("expected empty-string description in PUT body, got %q", string(putBody))
	}

	var result CIWorkflowEditResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if !result.Changed {
		t.Fatal("expected changed=true")
	}
	if result.Description != "" {
		t.Fatalf("expected cleared description, got %q", result.Description)
	}
}

func TestWorkflowsEditNoChangeSkipsUpdate(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	patchFile, err := os.CreateTemp(t.TempDir(), "workflow-patch-*.json")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if _, err := patchFile.WriteString(`{"description":"Main workflow"}`); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	putCalls := 0

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.Method {
					case http.MethodGet:
						body := `{
							"id":"wf-1",
							"content":{
								"name":"Default",
								"description":"Main workflow",
								"disabled":false,
								"locked":false
							}
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					case http.MethodPut:
						putCalls++
						t.Fatal("did not expect PUT when patch makes no changes")
						return nil, nil
					default:
						t.Fatalf("unexpected method: %s", req.Method)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudWorkflowEditCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--patch-file", patchFile.Name(),
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if putCalls != 0 {
		t.Fatalf("expected 0 PUT calls, got %d", putCalls)
	}

	var result CIWorkflowEditResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if result.Changed {
		t.Fatal("expected changed=false")
	}
}

func TestWorkflowsEditMissingFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing product-id",
			args:    []string{"--workflow-id", "wf-1", "--patch-file", "workflow.patch.json"},
			wantErr: "--product-id is required",
		},
		{
			name:    "missing workflow-id",
			args:    []string{"--product-id", "prod-1", "--patch-file", "workflow.patch.json"},
			wantErr: "--workflow-id is required",
		},
		{
			name:    "missing patch-file",
			args:    []string{"--product-id", "prod-1", "--workflow-id", "wf-1"},
			wantErr: "--patch-file is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := webXcodeCloudWorkflowEditCommand()
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

func TestWorkflowsEnableSuccess(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	putCalls := 0
	var putBody []byte

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.Method {
					case http.MethodGet:
						body := `{
							"id":"wf-1",
							"content":{"name":"Default","disabled":true,"locked":false,"custom":"keep"}
						}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					case http.MethodPut:
						putCalls++
						var err error
						putBody, err = io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("failed reading PUT body: %v", err)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{}`)),
							Request:    req,
						}, nil
					default:
						t.Fatalf("unexpected method: %s", req.Method)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudWorkflowEnableCommand()
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

	var result CIWorkflowToggleResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if result.Action != "enabled" || !result.Changed {
		t.Fatalf("expected changed enable action, got %+v", result)
	}
	if !result.DisabledBefore || result.DisabledAfter {
		t.Fatalf("unexpected disabled transition: %+v", result)
	}
	if putCalls != 1 {
		t.Fatalf("expected 1 PUT call, got %d", putCalls)
	}
	if !strings.Contains(string(putBody), `"disabled":false`) {
		t.Fatalf("expected disabled:false in PUT body, got %q", string(putBody))
	}
	if !strings.Contains(string(putBody), `"name":"Default"`) || !strings.Contains(string(putBody), `"custom":"keep"`) {
		t.Fatalf("expected unrelated fields preserved in PUT body, got %q", string(putBody))
	}
}

func TestWorkflowsEnableAlreadyEnabledSkipsUpdate(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	putCalls := 0

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.Method {
					case http.MethodGet:
						body := `{"id":"wf-1","content":{"name":"Default","disabled":false}}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					case http.MethodPut:
						putCalls++
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{}`)),
							Request:    req,
						}, nil
					default:
						t.Fatalf("unexpected method: %s", req.Method)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudWorkflowEnableCommand()
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

	var result CIWorkflowToggleResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if result.Action != "already-enabled" || result.Changed {
		t.Fatalf("expected already-enabled no-op, got %+v", result)
	}
	if putCalls != 0 {
		t.Fatalf("expected no PUT calls for idempotent enable, got %d", putCalls)
	}
}

func TestWorkflowsDisableSuccess(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	putCalls := 0
	var putBody []byte

	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.Method {
					case http.MethodGet:
						body := `{"id":"wf-1","content":{"name":"Default","disabled":false}}`
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					case http.MethodPut:
						putCalls++
						var err error
						putBody, err = io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("failed reading PUT body: %v", err)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(`{}`)),
							Request:    req,
						}, nil
					default:
						t.Fatalf("unexpected method: %s", req.Method)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudWorkflowDisableCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--product-id", "prod-1",
		"--workflow-id", "wf-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	var result CIWorkflowToggleResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\noutput: %q", err, stdout)
	}
	if result.Action != "disabled" || !result.Changed {
		t.Fatalf("expected changed disable action, got %+v", result)
	}
	if result.DisabledBefore || !result.DisabledAfter {
		t.Fatalf("unexpected disabled transition: %+v", result)
	}
	if putCalls != 1 {
		t.Fatalf("expected 1 PUT call, got %d", putCalls)
	}
	if !strings.Contains(string(putBody), `"disabled":true`) {
		t.Fatalf("expected disabled:true in PUT body, got %q", string(putBody))
	}
}

func TestWorkflowsDisableMissingConfirm(t *testing.T) {
	cmd := webXcodeCloudWorkflowDisableCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--product-id", "prod-1",
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
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected confirm error in stderr, got %q", stderr)
	}
}

func TestWorkflowsCommandsHaveUsageFunc(t *testing.T) {
	cmd := webXcodeCloudWorkflowsCommand()
	if cmd.UsageFunc == nil {
		t.Fatalf("workflows command should have UsageFunc set")
	}
	for _, sub := range cmd.Subcommands {
		if sub.UsageFunc == nil {
			t.Fatalf("subcommand %q should have UsageFunc set", sub.Name)
		}
	}
}

func TestWorkflowSectionSummaries(t *testing.T) {
	start := json.RawMessage(`{"branch":{"branch":"main"},"pull_request":{"target":"main"}}`)
	startSummary := summarizeStartConditions(start)
	if !strings.Contains(startSummary, "2 (") {
		t.Fatalf("expected trigger count in summary, got %q", startSummary)
	}
	for _, token := range []string{"Branch", "Pull Request"} {
		if !strings.Contains(startSummary, token) {
			t.Fatalf("expected trigger summary to contain %q, got %q", token, startSummary)
		}
	}

	startArray := json.RawMessage(`[{"type":"branch"},{"type":"pull_request"}]`)
	startArraySummary := summarizeStartConditions(startArray)
	if !strings.Contains(startArraySummary, "2 (") {
		t.Fatalf("expected trigger count in array summary, got %q", startArraySummary)
	}
	for _, token := range []string{"Branch", "Pull Request"} {
		if !strings.Contains(startArraySummary, token) {
			t.Fatalf("expected array trigger summary to contain %q, got %q", token, startArraySummary)
		}
	}

	actions := json.RawMessage(`[
		{"default_name":"Archive - iOS","action_type":"archive"},
		{"default_name":"Archive - macOS","action_type":"archive"},
		{"action_type":"analyze"},
		{"type":"testFlight_external"}
	]`)
	actionSummary := summarizeActionList(actions)
	if !strings.Contains(actionSummary, "4 (") {
		t.Fatalf("expected action count in summary, got %q", actionSummary)
	}
	if !strings.Contains(actionSummary, "Archive - iOS") {
		t.Fatalf("expected action summary to include default_name, got %q", actionSummary)
	}
	if !strings.Contains(actionSummary, "+1 more") {
		t.Fatalf("expected truncated summary marker, got %q", actionSummary)
	}

	postActions := json.RawMessage(`[{"name":"TestFlight External Testing - iOS","type":"testFlight_external"}]`)
	postSummary := summarizeActionList(postActions)
	if postSummary != "1 (TestFlight External Testing - iOS)" {
		t.Fatalf("unexpected post-action summary: %q", postSummary)
	}
}
