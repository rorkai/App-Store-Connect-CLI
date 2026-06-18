package telemetry

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildEventSanitizesCommand(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	ev, ok := BuildEvent(
		"asc apps info edit",
		"1.2.3",
		450*time.Millisecond,
		0,
	)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.CommandPath != "asc apps info edit" {
		t.Fatalf("CommandPath = %q", ev.CommandPath)
	}
	if ev.CommandFamily != "apps" {
		t.Fatalf("CommandFamily = %q", ev.CommandFamily)
	}
	if ev.DurationBucket != "100ms_500ms" {
		t.Fatalf("DurationBucket = %q", ev.DurationBucket)
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if payload["runtime_context"] != string(RuntimeLocal) {
		t.Fatalf("runtime_context = %v, want %q", payload["runtime_context"], RuntimeLocal)
	}
	if payload["invocation_source"] != string(SourceTerminal) {
		t.Fatalf("invocation_source = %v, want %q", payload["invocation_source"], SourceTerminal)
	}
	if _, exists := payload["execution_context"]; exists {
		t.Fatal("legacy execution_context field should not be emitted")
	}
	for _, forbiddenField := range []string{"args", "argv", "raw_args", "raw_argv"} {
		if _, exists := payload[forbiddenField]; exists {
			t.Fatalf("event contains forbidden raw-argument field %q", forbiddenField)
		}
	}
}

func TestBuildEventReusesProcessSessionID(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	first, ok := BuildEvent("asc builds list", "1.2.3", time.Second, 0)
	if !ok {
		t.Fatal("expected first event")
	}
	second, ok := BuildEvent("asc apps list", "1.2.3", time.Second, 0)
	if !ok {
		t.Fatal("expected second event")
	}
	if first.SessionID != second.SessionID {
		t.Fatalf("session IDs differ within one process: %q != %q", first.SessionID, second.SessionID)
	}
	if first.EventID == second.EventID {
		t.Fatalf("event IDs must remain unique, both were %q", first.EventID)
	}
}

func TestBuildEventReusesInstallIDAcrossLocalInvocationSources(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	terminalEvent, ok := BuildEvent("asc builds list", "1.2.3", time.Second, 0)
	if !ok {
		t.Fatal("expected terminal event")
	}
	if terminalEvent.InstallID == nil {
		t.Fatal("expected terminal install ID")
	}
	if terminalEvent.RuntimeContext != RuntimeLocal {
		t.Fatalf("terminal RuntimeContext = %q, want %q", terminalEvent.RuntimeContext, RuntimeLocal)
	}
	if terminalEvent.InvocationSource != SourceTerminal {
		t.Fatalf("terminal InvocationSource = %q, want %q", terminalEvent.InvocationSource, SourceTerminal)
	}

	tests := []struct {
		name       string
		env        map[string]string
		wantSource InvocationSource
	}{
		{name: "Claude Code", env: map[string]string{"CLAUDECODE": "1"}, wantSource: SourceClaudeCode},
		{name: "Cursor Agent", env: map[string]string{"CURSOR_AGENT": "1"}, wantSource: SourceCursorAgent},
		{
			name:       "Codex Desktop",
			env:        map[string]string{"CODEX_SHELL": "1", "CODEX_THREAD_ID": "thread-1"},
			wantSource: SourceCodexDesktop,
		},
		{name: "OpenCode", env: map[string]string{"OPENCODE": "1"}, wantSource: SourceOpenCode},
		{name: "Pi", env: map[string]string{"PI_CODING_AGENT": "true"}, wantSource: SourcePi},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearContextEnv(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			agentEvent, ok := BuildEvent("asc builds list", "1.2.3", time.Second, 0)
			if !ok {
				t.Fatal("expected agent event")
			}
			if agentEvent.InstallID == nil {
				t.Fatal("expected local agent install ID")
			}
			if *agentEvent.InstallID != *terminalEvent.InstallID {
				t.Fatalf("agent install ID = %q, want %q", *agentEvent.InstallID, *terminalEvent.InstallID)
			}
			if agentEvent.RuntimeContext != RuntimeLocal {
				t.Fatalf("RuntimeContext = %q, want %q", agentEvent.RuntimeContext, RuntimeLocal)
			}
			if agentEvent.InvocationSource != tt.wantSource {
				t.Fatalf("InvocationSource = %q, want %q", agentEvent.InvocationSource, tt.wantSource)
			}
		})
	}
}

func TestBuildEventOmitsInstallIDForEphemeralAgentRuntime(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantRuntime RuntimeContext
		wantSource  InvocationSource
	}{
		{
			name:        "Claude Code in CI",
			env:         map[string]string{"CI": "true", "CLAUDECODE": "1"},
			wantRuntime: RuntimeCI,
			wantSource:  SourceClaudeCode,
		},
		{
			name: "Codex Desktop marker in Rork sandbox",
			env: map[string]string{
				"RORK_SANDBOX_ID": "sandbox-1",
				"CODEX_SHELL":     "1",
				"CODEX_THREAD_ID": "thread-1",
			},
			wantRuntime: RuntimeRorkSandbox,
			wantSource:  SourceCodexDesktop,
		},
		{
			name:        "Rork agent in Rork sandbox",
			env:         map[string]string{"RORK_SANDBOX_ID": "sandbox-1"},
			wantRuntime: RuntimeRorkSandbox,
			wantSource:  SourceRorkAgent,
		},
		{
			name: "Rork agent in GitHub workflow",
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "rorkai/user-workflows",
			},
			wantRuntime: RuntimeRorkGitHubWorkflow,
			wantSource:  SourceRorkAgent,
		},
		{
			name:        "Pi in explicitly ephemeral runtime",
			env:         map[string]string{telemetryEphemeralEnvVar: "1", "PI_CODING_AGENT": "true"},
			wantRuntime: RuntimeEphemeral,
			wantSource:  SourcePi,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearContextEnv(t)
			setTelemetryTestHome(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			ev, ok := BuildEvent("asc builds list", "1.2.3", time.Second, 1)
			if !ok {
				t.Fatal("expected event")
			}
			if ev.RuntimeContext != tt.wantRuntime {
				t.Fatalf("RuntimeContext = %q, want %q", ev.RuntimeContext, tt.wantRuntime)
			}
			if ev.InvocationSource != tt.wantSource {
				t.Fatalf("InvocationSource = %q, want %q", ev.InvocationSource, tt.wantSource)
			}
			if ev.InstallID != nil {
				t.Fatalf("expected nil install ID for %s, got %q", tt.wantRuntime, *ev.InstallID)
			}
		})
	}
}

func TestBuildEventTreatsLocalRorkProfileAsTerminal(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	ev, ok := BuildEvent(
		"asc auth login",
		"1.2.3",
		time.Second,
		0,
	)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.RuntimeContext != RuntimeLocal {
		t.Fatalf("RuntimeContext = %q, want %q", ev.RuntimeContext, RuntimeLocal)
	}
	if ev.InvocationSource != SourceTerminal {
		t.Fatalf("InvocationSource = %q, want %q", ev.InvocationSource, SourceTerminal)
	}
	if ev.InstallID == nil {
		t.Fatal("expected install ID for local profile usage")
	}
}

func TestBuildEventDoesNotWaitForInstallIDLock(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	unlock, err := lockState(path, lockTimeout)
	if err != nil {
		t.Fatalf("lockState() error = %v", err)
	}
	defer unlock()

	start := time.Now()
	ev, ok := BuildEvent("asc builds list", "1.2.3", time.Second, 0)
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("expected event")
	}
	if ev.InstallID != nil {
		t.Fatalf("expected nil install ID while state is locked, got %q", *ev.InstallID)
	}
	if elapsed >= 500*time.Millisecond {
		t.Fatalf("BuildEvent() elapsed = %s, want lock contention skipped before 500ms", elapsed)
	}
}

func TestBuildEventSkipsControlCommands(t *testing.T) {
	for _, commandPath := range []string{"asc", "asc completion", "asc version", "asc telemetry", "asc telemetry status"} {
		t.Run(commandPath, func(t *testing.T) {
			if _, ok := BuildEvent(commandPath, "1.2.3", 0, 0); ok {
				t.Fatalf("expected %q to be skipped", commandPath)
			}
		})
	}
}

func TestNegativeDurationIsClamped(t *testing.T) {
	if got := durationMillis(-time.Second); got != 0 {
		t.Fatalf("durationMillis() = %d, want 0", got)
	}
	if got := durationBucket(-time.Second); got != "lt_100ms" {
		t.Fatalf("durationBucket() = %q, want %q", got, "lt_100ms")
	}
}
