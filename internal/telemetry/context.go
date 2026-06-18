package telemetry

import (
	"os"
	"strings"
)

const telemetryEphemeralEnvVar = "ASC_TELEMETRY_EPHEMERAL"

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
	SourceTerminal       InvocationSource = "terminal"
	SourceClaudeCode     InvocationSource = "claude_code"
	SourceCursorAgent    InvocationSource = "cursor_agent"
	SourceCodexDesktop   InvocationSource = "codex_desktop"
	SourceOpenCode       InvocationSource = "opencode"
	SourcePi             InvocationSource = "pi"
	SourceRorkAgentSetup InvocationSource = "rork_agent_setup"
)

func DetectRuntimeContext() RuntimeContext {
	switch {
	case os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("GITHUB_REPOSITORY") == "rorkai/user-workflows":
		return RuntimeRorkGitHubWorkflow
	case os.Getenv("RORK_SANDBOX_ID") != "":
		return RuntimeRorkSandbox
	case envTruthy(telemetryEphemeralEnvVar):
		return RuntimeEphemeral
	case isKnownCIEnv():
		return RuntimeCI
	default:
		return RuntimeLocal
	}
}

func DetectInvocationSource(commandPath string, args []string) InvocationSource {
	switch {
	case commandPath == "asc auth login" && hasArgValue(args, "--name", "rork"):
		return SourceRorkAgentSetup
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
	default:
		return SourceTerminal
	}
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

func shouldAttachInstallID(ctx RuntimeContext) bool {
	return ctx == RuntimeLocal
}
