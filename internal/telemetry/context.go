package telemetry

import (
	"os"
	"slices"
	"strings"
)

type ExecutionContext string

const (
	ContextLocal              ExecutionContext = "local"
	ContextClaudeCode         ExecutionContext = "claude_code"
	ContextCursorAgent        ExecutionContext = "cursor_agent"
	ContextCodexDesktop       ExecutionContext = "codex_desktop"
	ContextRorkAgentSetup     ExecutionContext = "rork_agent_setup"
	ContextRorkSandbox        ExecutionContext = "rork_sandbox"
	ContextRorkGitHubWorkflow ExecutionContext = "rork_github_workflow"
	ContextCI                 ExecutionContext = "ci"
)

func DetectExecutionContext(commandPath string, args []string) ExecutionContext {
	switch {
	case os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("GITHUB_REPOSITORY") == "rorkai/user-workflows":
		return ContextRorkGitHubWorkflow
	case commandPath == "asc auth login" && hasArgValue(args, "--name", "rork"):
		return ContextRorkAgentSetup
	case os.Getenv("RORK_SANDBOX_ID") != "":
		return ContextRorkSandbox
	case os.Getenv("CLAUDECODE") == "1":
		return ContextClaudeCode
	case os.Getenv("CURSOR_AGENT") != "":
		return ContextCursorAgent
	case os.Getenv("CODEX_SHELL") == "1" && os.Getenv("CODEX_THREAD_ID") != "":
		return ContextCodexDesktop
	case isKnownCIEnv():
		return ContextCI
	default:
		return ContextLocal
	}
}

func isKnownCIEnv() bool {
	if envTruthy("CI") {
		return true
	}
	for _, key := range []string{
		"GITHUB_ACTIONS",
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
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

func envTruthy(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func hasArgValue(args []string, name, value string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == name {
			return i+1 < len(args) && args[i+1] == value
		}
		if before, after, ok := strings.Cut(arg, "="); ok && before == name && after == value {
			return true
		}
	}
	return false
}

func isLocalContext(ctx ExecutionContext) bool {
	return slices.Contains([]ExecutionContext{ContextLocal}, ctx)
}
