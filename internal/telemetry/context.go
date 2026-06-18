package telemetry

import (
	"os"
	"strings"
)

const (
	telemetryEphemeralEnvVar    = "ASC_TELEMETRY_EPHEMERAL"
	rorkUserWorkflowsRepository = "rorkai/user-workflows"
)

type RuntimeContext string

const (
	RuntimeLocal              RuntimeContext = "local"
	RuntimeEphemeral          RuntimeContext = "ephemeral"
	RuntimeRorkSandbox        RuntimeContext = "rork_sandbox"
	RuntimeRorkGitHubWorkflow RuntimeContext = "rork_github_workflow"
	RuntimeCI                 RuntimeContext = "ci"
)

type InvocationSource string

const (
	SourceTerminal     InvocationSource = "terminal"
	SourceClaudeCode   InvocationSource = "claude_code"
	SourceCursorAgent  InvocationSource = "cursor_agent"
	SourceCodexDesktop InvocationSource = "codex_desktop"
	SourceOpenCode     InvocationSource = "opencode"
	SourcePi           InvocationSource = "pi"
	SourceRorkAgent    InvocationSource = "rork_agent"
)

func DetectRuntimeContext() RuntimeContext {
	switch {
	case isRorkGitHubWorkflow():
		return RuntimeRorkGitHubWorkflow
	case isRorkSandbox():
		return RuntimeRorkSandbox
	case envTruthy(telemetryEphemeralEnvVar):
		return RuntimeEphemeral
	case isKnownCIEnv():
		return RuntimeCI
	default:
		return RuntimeLocal
	}
}

func DetectInvocationSource() InvocationSource {
	switch {
	case envTruthy("PI_CODING_AGENT"):
		return SourcePi
	case envTruthy("OPENCODE"):
		return SourceOpenCode
	case os.Getenv("CLAUDECODE") == "1":
		return SourceClaudeCode
	case os.Getenv("CURSOR_AGENT") != "":
		return SourceCursorAgent
	case os.Getenv("CODEX_SHELL") == "1" && os.Getenv("CODEX_THREAD_ID") != "":
		return SourceCodexDesktop
	case isRorkSandbox() || isRorkGitHubWorkflow():
		return SourceRorkAgent
	default:
		return SourceTerminal
	}
}

func isRorkSandbox() bool {
	return strings.TrimSpace(os.Getenv("RORK_SANDBOX_ID")) != ""
}

func isRorkGitHubWorkflow() bool {
	return envTruthy("GITHUB_ACTIONS") && strings.EqualFold(
		strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")),
		rorkUserWorkflowsRepository,
	)
}

func isKnownCIEnv() bool {
	for _, key := range []string{
		"CI",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"BUILDKITE",
		"BITRISE_IO",
		"TF_BUILD",
		"TRAVIS",
		"APPVEYOR",
	} {
		if envTruthy(key) {
			return true
		}
	}
	return os.Getenv("TEAMCITY_VERSION") != "" || os.Getenv("JENKINS_URL") != ""
}

func envTruthy(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func shouldAttachInstallID(ctx RuntimeContext) bool {
	return ctx == RuntimeLocal
}
