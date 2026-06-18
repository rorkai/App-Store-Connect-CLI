package telemetry

import "testing"

func TestDetectRuntimeContext(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want RuntimeContext
	}{
		{
			name: "local agent stays local",
			env:  map[string]string{"CLAUDECODE": "1"},
			want: RuntimeLocal,
		},
		{
			name: "explicit ephemeral runtime",
			env:  map[string]string{telemetryEphemeralEnvVar: "true"},
			want: RuntimeEphemeral,
		},
		{
			name: "rork sandbox",
			env:  map[string]string{"RORK_SANDBOX_ID": "sandbox-1"},
			want: RuntimeRorkSandbox,
		},
		{
			name: "rork github workflow",
			env:  map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": "rorkai/user-workflows"},
			want: RuntimeRorkGitHubWorkflow,
		},
		{
			name: "generic ci",
			env:  map[string]string{"CI": "true"},
			want: RuntimeCI,
		},
		{
			name: "ci wins independently of agent source",
			env:  map[string]string{"CI": "true", "CLAUDECODE": "1"},
			want: RuntimeCI,
		},
		{
			name: "false ephemeral flag stays local",
			env:  map[string]string{telemetryEphemeralEnvVar: "false"},
			want: RuntimeLocal,
		},
		{
			name: "false ci flags stay local",
			env:  map[string]string{"CI": "false", "GITHUB_ACTIONS": "0"},
			want: RuntimeLocal,
		},
		{
			name: "local",
			want: RuntimeLocal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearContextEnv(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			got := DetectRuntimeContext()
			if got != tt.want {
				t.Fatalf("DetectRuntimeContext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectInvocationSource(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want InvocationSource
	}{
		{
			name: "rork agent in sandbox",
			env:  map[string]string{"RORK_SANDBOX_ID": "sandbox-1"},
			want: SourceRorkAgent,
		},
		{
			name: "rork agent in user workflow",
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "rorkai/user-workflows",
			},
			want: SourceRorkAgent,
		},
		{
			name: "pi",
			env:  map[string]string{"PI_CODING_AGENT": "true"},
			want: SourcePi,
		},
		{
			name: "pi config directory is not an invocation marker",
			env:  map[string]string{"PI_CODING_AGENT_DIR": "/tmp/pi"},
			want: SourceTerminal,
		},
		{
			name: "opencode",
			env:  map[string]string{"OPENCODE": "1", "AGENT": "1"},
			want: SourceOpenCode,
		},
		{
			name: "generic agent marker is not enough",
			env:  map[string]string{"AGENT": "1"},
			want: SourceTerminal,
		},
		{
			name: "claude code",
			env:  map[string]string{"CLAUDECODE": "1"},
			want: SourceClaudeCode,
		},
		{
			name: "cursor agent",
			env:  map[string]string{"CURSOR_AGENT": "1"},
			want: SourceCursorAgent,
		},
		{
			name: "codex desktop",
			env:  map[string]string{"CODEX_SHELL": "1", "CODEX_THREAD_ID": "thread-1"},
			want: SourceCodexDesktop,
		},
		{
			name: "terminal",
			want: SourceTerminal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearContextEnv(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			got := DetectInvocationSource()
			if got != tt.want {
				t.Fatalf("DetectInvocationSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

func clearContextEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		telemetryEphemeralEnvVar,
		"GITHUB_ACTIONS",
		"GITHUB_REPOSITORY",
		"RORK_SANDBOX_ID",
		"PI_CODING_AGENT",
		"PI_CODING_AGENT_DIR",
		"OPENCODE",
		"AGENT",
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
