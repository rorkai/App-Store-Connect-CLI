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

	"github.com/peterbourgon/ff/v3/ffcli"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestBindJSONOnlyOutputFlagsDefaultsToJSON(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	output := bindJSONOnlyOutputFlags(fs)
	if output.Output == nil {
		t.Fatal("expected output flag pointer to be set")
	}
	if *output.Output != "json" {
		t.Fatalf("expected json default, got %q", *output.Output)
	}
}

func TestWorkflowsOptionsCommandHierarchy(t *testing.T) {
	cmd := webXcodeCloudWorkflowsCommand()
	optionsCmd := findSub(cmd, "options")
	if optionsCmd == nil {
		t.Fatal("expected 'options' subcommand")
	}
	if len(optionsCmd.Subcommands) != 7 {
		t.Fatalf("expected 7 options subcommands, got %d", len(optionsCmd.Subcommands))
	}

	names := map[string]bool{}
	for _, sub := range optionsCmd.Subcommands {
		names[sub.Name] = true
	}
	for _, name := range []string{
		"team-config",
		"build-versions",
		"product-config",
		"schemes",
		"test-destinations",
		"slack-provider",
		"slack-channels",
	} {
		if !names[name] {
			t.Fatalf("expected %q options subcommand", name)
		}
	}
}

func TestWorkflowsOptionsGroupReturnsErrHelp(t *testing.T) {
	cmd := webXcodeCloudWorkflowOptionsCommand()
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestWorkflowsOptionsCommandsSuccess(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	tests := []struct {
		name      string
		build     func() *ffcli.Command
		args      []string
		wantPath  string
		wantQuery map[string]string
		response  string
		assert    func(t *testing.T, raw []byte)
	}{
		{
			name:     "team config",
			build:    webXcodeCloudWorkflowOptionsTeamConfigCommand,
			args:     []string{"--apple-id", "user@example.com", "--output", "json"},
			wantPath: "/ci/api/teams/team-uuid/configuration-options-v10",
			response: `{"default_timezone":{"id":"Asia/Calcutta"},"timezones":[{"id":"Asia/Calcutta"}],"available_platforms":[{"localized_name":"iOS"}]}`,
			assert: func(t *testing.T, raw []byte) {
				t.Helper()
				var got map[string]any
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatalf("unmarshal output: %v", err)
				}
				if _, ok := got["default_timezone"]; !ok {
					t.Fatalf("expected default_timezone in output, got %#v", got)
				}
			},
		},
		{
			name:     "build versions",
			build:    webXcodeCloudWorkflowOptionsBuildVersionsCommand,
			args:     []string{"--apple-id", "user@example.com", "--output", "json"},
			wantPath: "/ci/api/teams/team-uuid/configuration-options/build-versions",
			response: `{"items":[{"build":"2026.03.14.1","is_default":true}]}`,
			assert: func(t *testing.T, raw []byte) {
				t.Helper()
				var got map[string]any
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatalf("unmarshal output: %v", err)
				}
				items, ok := got["items"].([]any)
				if !ok || len(items) != 1 {
					t.Fatalf("expected 1 build version item, got %#v", got["items"])
				}
			},
		},
		{
			name:     "product config",
			build:    webXcodeCloudWorkflowOptionsProductConfigCommand,
			args:     []string{"--apple-id", "user@example.com", "--product-id", "prod-1", "--output", "json"},
			wantPath: "/ci/api/teams/team-uuid/products/prod-1/product-configuration-options-v4",
			response: `{"repo_default_recommendation":"repo-1","all_container_file_paths":["Focus Rail.xcodeproj"]}`,
			assert: func(t *testing.T, raw []byte) {
				t.Helper()
				var got map[string]any
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatalf("unmarshal output: %v", err)
				}
				if got["repo_default_recommendation"] != "repo-1" {
					t.Fatalf("unexpected repo_default_recommendation: %#v", got["repo_default_recommendation"])
				}
			},
		},
		{
			name:     "schemes",
			build:    webXcodeCloudWorkflowOptionsSchemesCommand,
			args:     []string{"--apple-id", "user@example.com", "--product-id", "prod-1", "--container-file-path", "Focus Rail.xcodeproj", "--limit", "20", "--continuation-offset", "next-1", "--output", "json"},
			wantPath: "/ci/api/teams/team-uuid/products/prod-1/schemes",
			wantQuery: map[string]string{
				"container_file_path": "Focus Rail.xcodeproj",
				"limit":               "20",
				"continuation_offset": "next-1",
			},
			response: `{"items":[{"name":"Focus Rail","test_plans":[{"name":"Focus Rail"}]}]}`,
			assert: func(t *testing.T, raw []byte) {
				t.Helper()
				var got map[string]any
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatalf("unmarshal output: %v", err)
				}
				items, ok := got["items"].([]any)
				if !ok || len(items) != 1 {
					t.Fatalf("expected 1 scheme item, got %#v", got["items"])
				}
			},
		},
		{
			name:     "test destinations",
			build:    webXcodeCloudWorkflowOptionsTestDestinationsCommand,
			args:     []string{"--apple-id", "user@example.com", "--xcode-version", "latest:stable", "--output", "json"},
			wantPath: "/ci/api/teams/team-uuid/test-destinations-v3",
			wantQuery: map[string]string{
				"xcode_version": "latest:stable",
			},
			response: `{"platform_test_destinations":[{"platform":{"name":"iOS"},"destinations":[{"option_groups":[]}]}]}`,
			assert: func(t *testing.T, raw []byte) {
				t.Helper()
				var got map[string]any
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatalf("unmarshal output: %v", err)
				}
				if _, ok := got["platform_test_destinations"]; !ok {
					t.Fatalf("expected platform_test_destinations in output, got %#v", got)
				}
			},
		},
		{
			name:     "slack provider",
			build:    webXcodeCloudWorkflowOptionsSlackProviderCommand,
			args:     []string{"--apple-id", "user@example.com", "--output", "json"},
			wantPath: "/ci/api/teams/team-uuid/integrations/slack",
			response: `{"is_user_connected":true,"workspace_name":"Build Ops","bot_name":"XC Cloud"}`,
			assert: func(t *testing.T, raw []byte) {
				t.Helper()
				var got map[string]any
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatalf("unmarshal output: %v", err)
				}
				if got["workspace_name"] != "Build Ops" {
					t.Fatalf("unexpected workspace_name: %#v", got["workspace_name"])
				}
			},
		},
		{
			name:     "slack channels",
			build:    webXcodeCloudWorkflowOptionsSlackChannelsCommand,
			args:     []string{"--apple-id", "user@example.com", "--output", "json"},
			wantPath: "/ci/api/teams/team-uuid/integrations/slack/channels",
			response: `[{"id":"chan-1","channel_name":"builds","is_private":false,"is_enabled":true}]`,
			assert: func(t *testing.T, raw []byte) {
				t.Helper()
				var got []map[string]any
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatalf("unmarshal output: %v", err)
				}
				if len(got) != 1 || got[0]["channel_name"] != "builds" {
					t.Fatalf("unexpected channels output: %#v", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
							if req.URL.Path != tt.wantPath {
								t.Fatalf("unexpected path: %s", req.URL.Path)
							}
							for key, want := range tt.wantQuery {
								if got := req.URL.Query().Get(key); got != want {
									t.Fatalf("expected query %s=%q, got %q", key, want, got)
								}
							}
							return &http.Response{
								StatusCode: http.StatusOK,
								Header:     http.Header{"Content-Type": []string{"application/json"}},
								Body:       io.NopCloser(strings.NewReader(tt.response)),
								Request:    req,
							}, nil
						}),
					},
				}, "cache", nil
			}

			cmd := tt.build()
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}

			stdout, _ := captureOutput(t, func() {
				if err := cmd.Exec(context.Background(), nil); err != nil {
					t.Fatalf("exec error: %v", err)
				}
			})

			tt.assert(t, []byte(stdout))
		})
	}
}

func TestWorkflowsOptionsMissingFlags(t *testing.T) {
	tests := []struct {
		name    string
		build   func() *ffcli.Command
		args    []string
		wantErr string
	}{
		{
			name:    "product config missing product-id",
			build:   webXcodeCloudWorkflowOptionsProductConfigCommand,
			args:    []string{},
			wantErr: "--product-id is required",
		},
		{
			name:    "schemes missing product-id",
			build:   webXcodeCloudWorkflowOptionsSchemesCommand,
			args:    []string{},
			wantErr: "--product-id is required",
		},
		{
			name:    "test destinations missing xcode-version",
			build:   webXcodeCloudWorkflowOptionsTestDestinationsCommand,
			args:    []string{},
			wantErr: "--xcode-version is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.build()
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

func TestWorkflowsOptionsRejectUnsupportedOutput(t *testing.T) {
	tests := []struct {
		name  string
		build func() *ffcli.Command
		args  []string
	}{
		{
			name:  "team config",
			build: webXcodeCloudWorkflowOptionsTeamConfigCommand,
			args:  []string{"--output", "table"},
		},
		{
			name:  "build versions",
			build: webXcodeCloudWorkflowOptionsBuildVersionsCommand,
			args:  []string{"--output", "table"},
		},
		{
			name:  "product config",
			build: webXcodeCloudWorkflowOptionsProductConfigCommand,
			args:  []string{"--product-id", "prod-1", "--output", "table"},
		},
		{
			name:  "schemes",
			build: webXcodeCloudWorkflowOptionsSchemesCommand,
			args:  []string{"--product-id", "prod-1", "--output", "table"},
		},
		{
			name:  "test destinations",
			build: webXcodeCloudWorkflowOptionsTestDestinationsCommand,
			args:  []string{"--xcode-version", "latest:stable", "--output", "table"},
		},
		{
			name:  "slack provider",
			build: webXcodeCloudWorkflowOptionsSlackProviderCommand,
			args:  []string{"--output", "table"},
		},
		{
			name:  "slack channels",
			build: webXcodeCloudWorkflowOptionsSlackChannelsCommand,
			args:  []string{"--output", "table"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.build()
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}

			err := cmd.Exec(context.Background(), nil)
			if err == nil {
				t.Fatal("expected unsupported output error")
			}
			if !strings.Contains(err.Error(), "unsupported format: table") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWorkflowsOptionsSchemesRejectsExplicitNonPositiveLimit(t *testing.T) {
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
					t.Fatal("did not expect request for invalid limit")
					return nil, nil
				}),
			},
		}, "cache", nil
	}

	tests := []struct {
		name  string
		limit string
	}{
		{name: "zero", limit: "0"},
		{name: "negative", limit: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := webXcodeCloudWorkflowOptionsSchemesCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--apple-id", "user@example.com",
				"--product-id", "prod-1",
				"--limit", tt.limit,
			}); err != nil {
				t.Fatalf("parse error: %v", err)
			}

			_, stderr := captureOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if !strings.Contains(stderr, "--limit must be greater than 0 when provided") {
				t.Fatalf("expected limit validation in stderr, got %q", stderr)
			}
		})
	}
}
