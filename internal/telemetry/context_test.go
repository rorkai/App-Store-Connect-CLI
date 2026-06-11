package telemetry

import "testing"

func TestDetectExecutionContext(t *testing.T) {
	tests := []struct {
		name        string
		commandPath string
		args        []string
		env         map[string]string
		want        ExecutionContext
	}{
		{
			name:        "claude code",
			commandPath: "asc builds list",
			env:         map[string]string{"CLAUDECODE": "1"},
			want:        ContextClaudeCode,
		},
		{
			name:        "cursor agent",
			commandPath: "asc builds list",
			env:         map[string]string{"CURSOR_AGENT": "1"},
			want:        ContextCursorAgent,
		},
		{
			name:        "codex desktop",
			commandPath: "asc builds list",
			env:         map[string]string{"CODEX_SHELL": "1", "CODEX_THREAD_ID": "thread-1"},
			want:        ContextCodexDesktop,
		},
		{
			name:        "rork setup command shape",
			commandPath: "asc auth login",
			args:        []string{"auth", "login", "--name", "rork", "--key-id", "secret"},
			want:        ContextRorkAgentSetup,
		},
		{
			name:        "rork sandbox",
			commandPath: "asc apps list",
			env:         map[string]string{"RORK_SANDBOX_ID": "sandbox-1"},
			want:        ContextRorkSandbox,
		},
		{
			name:        "rork github workflow",
			commandPath: "asc publish appstore",
			env:         map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": "rorkai/user-workflows"},
			want:        ContextRorkGitHubWorkflow,
		},
		{
			name:        "generic ci",
			commandPath: "asc builds list",
			env:         map[string]string{"CI": "true"},
			want:        ContextCI,
		},
		{
			name:        "false ci flags stay local",
			commandPath: "asc builds list",
			env:         map[string]string{"CI": "false", "GITHUB_ACTIONS": "0"},
			want:        ContextLocal,
		},
		{
			name:        "local",
			commandPath: "asc builds list",
			want:        ContextLocal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearContextEnv(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			got := DetectExecutionContext(tt.commandPath, tt.args)
			if got != tt.want {
				t.Fatalf("DetectExecutionContext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func clearContextEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"GITHUB_ACTIONS",
		"GITHUB_REPOSITORY",
		"RORK_SANDBOX_ID",
		"CLAUDECODE",
		"CURSOR_AGENT",
		"CODEX_SHELL",
		"CODEX_THREAD_ID",
		"CI",
		"GITLAB_CI",
		"CIRCLECI",
		"BUILDKITE",
		"BITRISE_IO",
		"TF_BUILD",
		"TEAMCITY_VERSION",
		"JENKINS_URL",
		"TRAVIS",
		"APPVEYOR",
	} {
		t.Setenv(key, "")
	}
}
