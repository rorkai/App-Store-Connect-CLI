package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
	if payload["outcome_kind"] != string(OutcomeSuccess) {
		t.Fatalf("unexpected v4 event context: %v", payload)
	}
	if value, exists := payload["http_status"]; !exists || value != nil {
		t.Fatalf("http_status = %v (exists=%t), want explicit null", value, exists)
	}
	if value, exists := payload["diagnostic_code"]; !exists || value != nil {
		t.Fatalf("diagnostic_code = %v (exists=%t), want explicit null", value, exists)
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

func TestBuildEventWithContextEmitsAllowlistedDiagnosticCode(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	ev, ok := BuildEventWithContext(
		"asc auth login",
		"1.2.3",
		0,
		2,
		EventContext{
			DiagnosticCode: string(shared.DiagnosticFileInvalidFormat),
		},
	)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.DiagnosticCode == nil || *ev.DiagnosticCode != string(shared.DiagnosticFileInvalidFormat) {
		t.Fatalf("DiagnosticCode = %v, want %q", ev.DiagnosticCode, shared.DiagnosticFileInvalidFormat)
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if payload["diagnostic_code"] != string(shared.DiagnosticFileInvalidFormat) {
		t.Fatalf("diagnostic_code = %v, want %q", payload["diagnostic_code"], shared.DiagnosticFileInvalidFormat)
	}
}

func TestBuildEventWithContextRejectsUnboundedDiagnosticCode(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	ev, ok := BuildEventWithContext(
		"asc auth login",
		"1.2.3",
		0,
		2,
		EventContext{DiagnosticCode: "private-key-at-/Users/example/AuthKey.p8"},
	)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.DiagnosticCode != nil {
		t.Fatalf("DiagnosticCode = %q, want nil", *ev.DiagnosticCode)
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if strings.Contains(string(data), "/Users/example/AuthKey.p8") {
		t.Fatalf("payload leaked unbounded diagnostic code: %s", data)
	}
}

func TestBuildEventWithContextEmitsOnlyLowCardinalityClassifications(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	ev, ok := BuildEventWithContext(
		"asc versions attach-build",
		"1.2.3",
		0,
		2,
		EventContext{
			InvocationShape:  InvocationShapeLeaf,
			ErrorKind:        ErrorKindUnknownFlag,
			FailureStage:     FailureStageParse,
			FailureParameter: "--build-id=BUILD_ID",
		},
	)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.SchemaVersion != 4 {
		t.Fatalf("SchemaVersion = %d, want 4", ev.SchemaVersion)
	}
	if ev.InvocationShape != InvocationShapeLeaf || ev.ErrorKind == nil || *ev.ErrorKind != ErrorKindUnknownFlag ||
		ev.FailureStage == nil || *ev.FailureStage != FailureStageParse {
		t.Fatalf("unexpected classifications: %+v", ev)
	}
	if ev.FailureParameter == nil || *ev.FailureParameter != "--build-id" {
		t.Fatalf("FailureParameter = %v, want --build-id", ev.FailureParameter)
	}
	if ev.OutcomeKind != OutcomeUsageError || ev.HTTPStatus != nil {
		t.Fatalf("unexpected v4 outcome context: %+v", ev)
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	for _, forbidden := range []string{"stderr", "error_message", "args", "argv"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("payload contains forbidden field %q: %s", forbidden, data)
		}
	}
}

func TestBuildEventWithContextCapturesBoundedHTTPContext(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	ev, ok := BuildEventWithContext(
		"asc analytics sales",
		"2.7.0",
		time.Second,
		12,
		EventContext{
			InvocationShape: InvocationShapeLeaf,
			ErrorKind:       ErrorKindOther,
			FailureStage:    FailureStageRequest,
			OutcomeKind:     OutcomeAuthError,
			HTTPStatus:      403,
		},
	)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.HTTPStatus == nil || *ev.HTTPStatus != 403 {
		t.Fatalf("HTTPStatus = %v, want 403", ev.HTTPStatus)
	}
	if ev.OutcomeKind != OutcomeAuthError {
		t.Fatalf("OutcomeKind = %q, want %q", ev.OutcomeKind, OutcomeAuthError)
	}
}

func TestBuildEventWithContextPreservesPublicStorefrontAuthorizationOutcome(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	event, ok := BuildEventWithContext(
		"asc apps public search",
		"1.2.3",
		time.Second,
		1,
		EventContext{
			InvocationShape:  InvocationShapeLeaf,
			ErrorKind:        ErrorKindOther,
			FailureStage:     FailureStageRequest,
			OutcomeKind:      OutcomeAPIClientError,
			HTTPStatus:       403,
			PublicStorefront: true,
		},
	)
	if !ok {
		t.Fatal("expected event")
	}
	if event.OutcomeKind != OutcomeAPIClientError {
		t.Fatalf("OutcomeKind = %q, want %q", event.OutcomeKind, OutcomeAPIClientError)
	}
}

func TestBuildEventWithContextPreservesAPIOutcomesWithoutHTTPStatus(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	tests := []struct {
		name    string
		context EventContext
		want    OutcomeKind
	}{
		{
			name: "explicit client error",
			context: EventContext{
				ErrorKind:    ErrorKindOther,
				FailureStage: FailureStageRequest,
				OutcomeKind:  OutcomeAPIClientError,
			},
			want: OutcomeAPIClientError,
		},
		{
			name: "explicit server error",
			context: EventContext{
				ErrorKind:    ErrorKindOther,
				FailureStage: FailureStageRequest,
				OutcomeKind:  OutcomeAPIServerError,
			},
			want: OutcomeAPIServerError,
		},
		{
			name: "conflict error kind",
			context: EventContext{
				ErrorKind: ErrorKindAPIConflict,
			},
			want: OutcomeConflict,
		},
		{
			name: "server error kind",
			context: EventContext{
				ErrorKind: ErrorKindAPI5xx,
			},
			want: OutcomeAPIServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, ok := BuildEventWithContext("asc builds list", "2.7.0", time.Second, 1, tt.context)
			if !ok {
				t.Fatal("expected event")
			}
			if ev.OutcomeKind != tt.want {
				t.Fatalf("OutcomeKind = %q, want %q", ev.OutcomeKind, tt.want)
			}
			if ev.HTTPStatus != nil {
				t.Fatalf("HTTPStatus = %v, want nil", ev.HTTPStatus)
			}
		})
	}
}

func TestSchemaV3SpoolRecordOmitsSchemaV4Fields(t *testing.T) {
	data, err := json.Marshal(Event{
		SchemaVersion:   3,
		InvocationShape: InvocationShapeLeaf,
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	for _, field := range []string{"outcome_kind", "http_status", "diagnostic_code"} {
		if strings.Contains(string(data), `"`+field+`"`) {
			t.Fatalf("schema-v3 payload contains schema-v4 field %q: %s", field, data)
		}
	}
}

func TestBuildEventWithContextAllowsKnownFailureParameters(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	for _, parameter := range []string{"--id", "--app", "--app-id", "--key-type", "--export-xcodebuild-flag", "--active", "--availability-id", "--base-price", "--batch-id", "--copy-metadata-from", "--copyright", "--country", "--custom-app-name", "--demo-account-password", "--detail-fields", "--end-date", "--external-testing", "--fields", "--group", "--iap-version-fields", "--include", "--input", "--item-fields", "--keywords", "--local", "--name", "--number-of-periods", "--offer-code", "--offer-code-id", "--older-than", "--path", "--pkg", "--price-id", "--price-point-id", "--prices", "--private-key", "--product-id", "--reference-name", "--removed", "--resolved", "--response-fields", "--response-state", "--screenshot-id", "--skip-validation", "--stars", "--state", "--subscription-group-version-fields", "--subscription-version-fields", "--term", "--territories", "--territory", "--tester", "--test-notes", "--treatment-id", "--type", "--upload", "--uses-non-exempt-encryption", "--uses-third-party-content", "--visible-in-app-store", "--whats-new", "--workers"} {
		t.Run(parameter, func(t *testing.T) {
			ev, ok := BuildEventWithContext(
				"asc apps view",
				"1.2.3",
				0,
				2,
				EventContext{
					InvocationShape:  InvocationShapeLeaf,
					ErrorKind:        ErrorKindUnknownFlag,
					FailureStage:     FailureStageParse,
					FailureParameter: parameter,
				},
			)
			if !ok {
				t.Fatal("expected event")
			}
			if ev.FailureParameter == nil || *ev.FailureParameter != parameter {
				t.Fatalf("FailureParameter = %v, want %q", ev.FailureParameter, parameter)
			}
		})
	}
}

func TestBuildEventWithContextStripsKnownFailureParameterValue(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	ev, ok := BuildEventWithContext(
		"asc auth login",
		"1.2.3",
		0,
		2,
		EventContext{
			InvocationShape:  InvocationShapeLeaf,
			ErrorKind:        ErrorKindInvalidValue,
			FailureStage:     FailureStageValidation,
			FailureParameter: "--key-type=individual",
		},
	)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.FailureParameter == nil || *ev.FailureParameter != "--key-type" {
		t.Fatalf("FailureParameter = %v, want --key-type", ev.FailureParameter)
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if strings.Contains(string(data), "individual") {
		t.Fatalf("payload leaked failure parameter value: %s", data)
	}
}

func TestBuildEventWithContextCanonicalizesFailureStage(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	ev, ok := BuildEventWithContext(
		"asc versions attach-build",
		"1.2.3",
		0,
		2,
		EventContext{
			InvocationShape: InvocationShapeLeaf,
			ErrorKind:       ErrorKindUnknownFlag,
			FailureStage:    FailureStageRequest,
		},
	)
	if !ok || ev.FailureStage == nil {
		t.Fatal("expected classified event")
	}
	if *ev.FailureStage != FailureStageParse {
		t.Fatalf("FailureStage = %q, want %q", *ev.FailureStage, FailureStageParse)
	}
}

func TestBuildEventWithContextRejectsUnboundedContextValues(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	ev, ok := BuildEventWithContext(
		"asc builds",
		"1.2.3",
		0,
		2,
		EventContext{
			InvocationShape:  InvocationShape("unknown_child:private-app-name"),
			ErrorKind:        ErrorKind("unknown_flag:--private-value"),
			FailureStage:     FailureStage("stderr:private-error"),
			FailureParameter: "--private_value=SECRET",
		},
	)
	if !ok || ev.ErrorKind == nil || ev.FailureStage == nil {
		t.Fatal("expected canonicalized event")
	}
	if ev.InvocationShape != InvocationShapeLeaf || *ev.ErrorKind != ErrorKindOther || *ev.FailureStage != FailureStageExecution ||
		ev.FailureParameter != nil {
		t.Fatalf("unexpected canonicalized context: %+v", ev)
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	for _, privateValue := range []string{"private-app-name", "private-value", "private-error", "SECRET"} {
		if strings.Contains(string(data), privateValue) {
			t.Fatalf("payload leaked unbounded context %q: %s", privateValue, data)
		}
	}
}

func TestBuildEventWithContextRejectsIdentifierShapedFailureParameters(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	for _, parameter := range []string{
		"--6759231657",
		"--123e4567-e89b-12d3-a456-426614174000",
		"--a1b2c3d4-e5f6-47a8-9012-3456789abcde",
	} {
		t.Run(parameter, func(t *testing.T) {
			ev, ok := BuildEventWithContext(
				"asc builds",
				"1.2.3",
				0,
				2,
				EventContext{
					InvocationShape:  InvocationShapeLeaf,
					ErrorKind:        ErrorKindUnknownFlag,
					FailureStage:     FailureStageParse,
					FailureParameter: parameter,
				},
			)
			if !ok {
				t.Fatal("expected event")
			}
			if ev.FailureParameter != nil {
				t.Fatalf("FailureParameter for %q = %q, want nil", parameter, *ev.FailureParameter)
			}

			data, err := json.Marshal(ev)
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}
			if strings.Contains(string(data), strings.TrimPrefix(parameter, "--")) {
				t.Fatalf("payload leaked identifier-shaped parameter %q: %s", parameter, data)
			}
		})
	}
}

func TestBuildEventWithContextRejectsUnknownFailureParameterNames(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	ev, ok := BuildEventWithContext(
		"asc builds",
		"1.2.3",
		0,
		2,
		EventContext{
			InvocationShape:  InvocationShapeLeaf,
			ErrorKind:        ErrorKindUnknownFlag,
			FailureStage:     FailureStageParse,
			FailureParameter: "--my-secret-project=SECRET",
		},
	)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.FailureParameter != nil {
		t.Fatalf("FailureParameter = %q, want nil", *ev.FailureParameter)
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
		{
			name:       "Claude Code",
			env:        map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1"},
			wantSource: SourceClaudeCode,
		},
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
			env:         map[string]string{"CI": "true", "CLAUDE_CODE_CHILD_SESSION": "1"},
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
