package cmd

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestRun_VersionFlag(t *testing.T) {
	resetReportFlags(t)

	stdout, _ := captureCommandOutput(t, func() {
		code := Run([]string{"--version"}, "9.9.9")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if !strings.Contains(stdout, "9.9.9") {
		t.Fatalf("expected version in stdout, got %q", stdout)
	}
}

func TestRun_ReportFlagValidationError(t *testing.T) {
	resetReportFlags(t)

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"--report-file", filepath.Join(t.TempDir(), "junit.xml"), "completion", "--shell", "bash"}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "--report is required") {
		t.Fatalf("expected report validation error, got %q", stderr)
	}
}

func TestRun_ReportWriteFailureReturnsExitError(t *testing.T) {
	resetReportFlags(t)

	reportPath := filepath.Join(t.TempDir(), "junit.xml")
	if err := os.WriteFile(reportPath, []byte("existing"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"--report", "junit",
			"--report-file", reportPath,
			"completion", "--shell", "bash",
		}, "1.0.0")
		if code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	if !strings.Contains(stderr, "failed to write JUnit report") {
		t.Fatalf("expected JUnit write failure in stderr, got %q", stderr)
	}
}

func TestRun_UnknownCommandReturnsUsage(t *testing.T) {
	resetReportFlags(t)

	code := Run([]string{"unknown-command"}, "1.0.0")
	if code != ExitUsage {
		t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
	}
}

func TestRun_RemovedTopLevelCommandsReturnUnknown(t *testing.T) {
	resetReportFlags(t)

	tests := []struct {
		name string
		arg  string
	}{
		{name: "assets removed", arg: "assets"},
		{name: "shots removed", arg: "shots"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, stderr := captureCommandOutput(t, func() {
				code := Run([]string{test.arg}, "1.0.0")
				if code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})
			if !strings.Contains(stderr, "Unknown command: "+test.arg) {
				t.Fatalf("expected unknown command in stderr, got %q", stderr)
			}
		})
	}
}

func TestRun_NoArgsShowsHelpReturnsSuccess(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		code := Run([]string{}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if !strings.Contains(stdout, "USAGE") || !strings.Contains(stdout, "GETTING STARTED COMMANDS") {
		t.Fatalf("expected root help in stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestRun_InvokesSkillsUpdateCheckForSubcommand(t *testing.T) {
	resetReportFlags(t)

	origCheck := maybeCheckForSkillUpdates
	t.Cleanup(func() { maybeCheckForSkillUpdates = origCheck })

	called := make(chan struct{}, 1)
	maybeCheckForSkillUpdates = func(ctx context.Context) {
		select {
		case called <- struct{}{}:
		default:
		}
	}

	_, _ = captureCommandOutput(t, func() {
		code := Run([]string{"completion", "--shell", "bash"}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	select {
	case <-called:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected skills update check to be invoked")
	}
}

func TestRun_SkipsSkillsUpdateCheckForRootInvocation(t *testing.T) {
	resetReportFlags(t)

	origCheck := maybeCheckForSkillUpdates
	t.Cleanup(func() { maybeCheckForSkillUpdates = origCheck })

	called := false
	maybeCheckForSkillUpdates = func(ctx context.Context) {
		called = true
	}

	_, _ = captureCommandOutput(t, func() {
		code := Run([]string{}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if called {
		t.Fatal("expected skills update check to be skipped for root invocation")
	}
}

func TestRun_HelpSkipsAuthResolution(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()

	tests := []struct {
		name          string
		args          []string
		wantHelpText  string
		avoidMessages []string
	}{
		{
			name:          "apps list help",
			args:          []string{"apps", "list", "--help"},
			wantHelpText:  "List apps from App Store Connect.",
			avoidMessages: []string{"missing authentication", "keychain access denied"},
		},
		{
			name:          "auth token help",
			args:          []string{"auth", "token", "--help"},
			wantHelpText:  "Print a signed JWT for direct App Store Connect API calls.",
			avoidMessages: []string{"missing authentication", "--confirm is required", "keychain access denied"},
		},
		{
			name:          "auth issuer-id help",
			args:          []string{"auth", "issuer-id", "--help"},
			wantHelpText:  "Print the active App Store Connect issuer ID.",
			avoidMessages: []string{"missing authentication", "keychain access denied"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, exitCode := runHelpSubprocess(t, tempDir, test.args...)
			combined := stdout + stderr

			if exitCode != 0 {
				t.Fatalf("help exit code = %d, want 0; stdout=%q stderr=%q", exitCode, stdout, stderr)
			}
			if !strings.Contains(combined, test.wantHelpText) {
				t.Fatalf("expected help text %q in output, got stdout=%q stderr=%q", test.wantHelpText, stdout, stderr)
			}
			for _, avoid := range test.avoidMessages {
				if strings.Contains(combined, avoid) {
					t.Fatalf("expected help path to avoid %q, got stdout=%q stderr=%q", avoid, stdout, stderr)
				}
			}
		})
	}
}

func TestHasPositionalArgs_EndOfFlagsSeparator(t *testing.T) {
	root := RootCommand("1.0.0")

	if got := hasPositionalArgs(root.FlagSet, []string{"--", "--version"}); !got {
		t.Fatalf("hasPositionalArgs() = %v, want true", got)
	}
}

func TestRootCommand_UnknownCommandPrintsHelpError(t *testing.T) {
	root := RootCommand("1.2.3")
	if err := root.Parse([]string{"unknown-subcommand"}); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	_, stderr := captureCommandOutput(t, func() {
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("Run() error = %v, want %v", err, flag.ErrHelp)
		}
	})

	if !strings.Contains(stderr, "Unknown command: unknown-subcommand") {
		t.Fatalf("expected unknown command output, got %q", stderr)
	}
}

func TestRootCommand_UsageGroupsSubcommands(t *testing.T) {
	root := RootCommand("1.2.3")
	usage := root.UsageFunc(root)

	if strings.Contains(usage, "SUBCOMMANDS") {
		t.Fatalf("usage should not use a single SUBCOMMANDS section, got %q", usage)
	}

	if !strings.Contains(usage, "GETTING STARTED COMMANDS") {
		t.Fatalf("expected GETTING STARTED group header, got %q", usage)
	}

	if !strings.Contains(usage, "  auth:") || !strings.Contains(usage, "  doctor:") || !strings.Contains(usage, "  install-skills:") || !strings.Contains(usage, "  init:") {
		t.Fatalf("expected grouped getting started commands with gh-style spacing, got %q", usage)
	}

	if !strings.Contains(usage, "ANALYTICS & FINANCE COMMANDS") {
		t.Fatalf("expected analytics group header, got %q", usage)
	}

	if !strings.Contains(usage, "  analytics:") || !strings.Contains(usage, "  finance:") {
		t.Fatalf("expected grouped analytics/finance commands, got %q", usage)
	}

	if strings.Contains(usage, "  offer-codes:") || strings.Contains(usage, "  win-back-offers:") || strings.Contains(usage, "  promoted-purchases:") {
		t.Fatalf("expected deprecated monetization shims to be hidden from primary root usage, got %q", usage)
	}

	if !strings.Contains(usage, "  subscriptions:") {
		t.Fatalf("expected subscriptions command to remain visible in root usage, got %q", usage)
	}

	if !strings.Contains(usage, "  screenshots:") || !strings.Contains(usage, "  video-previews:") {
		t.Fatalf("expected screenshots and video-previews commands in root usage, got %q", usage)
	}

	if strings.Contains(usage, "  assets:") || strings.Contains(usage, "  shots:") {
		t.Fatalf("expected old assets/shots commands to be removed from root usage, got %q", usage)
	}

	releaseIdx := strings.Index(usage, "  release:")
	reviewIdx := strings.Index(usage, "  review:")
	submitIdx := strings.Index(usage, "  submit:")
	if releaseIdx == -1 || reviewIdx == -1 || submitIdx == -1 {
		t.Fatalf("expected release, review, and submit commands in root usage, got %q", usage)
	}
	if releaseIdx > reviewIdx || releaseIdx > submitIdx {
		t.Fatalf("expected release to lead the review and release group, got %q", usage)
	}
}

func TestRootCommand_ReleaseHelpMentionsCanonicalPathAndStatus(t *testing.T) {
	root := RootCommand("1.2.3")

	var releaseCmd *ffcli.Command
	for _, subcommand := range root.Subcommands {
		if subcommand.Name == "release" {
			releaseCmd = subcommand
			break
		}
	}
	if releaseCmd == nil {
		t.Fatal("expected release subcommand to be registered")
	}

	usage := releaseCmd.UsageFunc(releaseCmd)
	if !strings.Contains(usage, "canonical path") {
		t.Fatalf("expected release help to describe the canonical path, got %q", usage)
	}
	if !strings.Contains(usage, `asc status --app "APP_ID"`) {
		t.Fatalf("expected release help to mention status monitoring, got %q", usage)
	}
	if !strings.Contains(usage, `asc submit create --app "APP_ID" --version "VERSION" --build "BUILD_ID" --confirm`) {
		t.Fatalf("expected release help to keep low-level submit guidance discoverable, got %q", usage)
	}
}

func TestRootCommand_WorkflowHelpMentionsReleaseAndStatusMonitoring(t *testing.T) {
	root := RootCommand("1.2.3")

	var workflowCmd *ffcli.Command
	for _, subcommand := range root.Subcommands {
		if subcommand.Name == "workflow" {
			workflowCmd = subcommand
			break
		}
	}
	if workflowCmd == nil {
		t.Fatal("expected workflow subcommand to be registered")
	}

	usage := workflowCmd.UsageFunc(workflowCmd)
	if !strings.Contains(usage, `asc release run --app $APP_ID --version $VERSION --build $BUILD_ID --metadata-dir ./metadata/version/$VERSION --confirm`) {
		t.Fatalf("expected workflow help to show the high-level release step, got %q", usage)
	}
	if !strings.Contains(usage, `asc status --app "APP_ID"`) {
		t.Fatalf("expected workflow help to mention post-release status monitoring, got %q", usage)
	}
}

func TestRun_InvalidOutputReturnsUsageBeforeAuth(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"devices", "register",
			"--name", "My Device",
			"--udid", "UDID",
			"--platform", "IOS",
			"--output", "yaml",
		}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "unsupported format: yaml") {
		t.Fatalf("expected output validation error, got %q", stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected output validation before auth resolution, got %q", stderr)
	}
}

func TestRun_InvalidPrettyReturnsUsageBeforeAuth(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"devices", "update",
			"--id", "DEVICE_ID",
			"--status", "ENABLED",
			"--output", "table",
			"--pretty",
		}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "--pretty is only valid with JSON output") {
		t.Fatalf("expected pretty/output validation error, got %q", stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected output validation before auth resolution, got %q", stderr)
	}
}

func TestRun_InvalidParentOutputReturnsUsageBeforeLeafExec(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"reviews",
			"--output", "yaml",
			"respond",
			"--review-id", "REVIEW_ID",
			"--response", "Thanks!",
		}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "unsupported format: yaml") {
		t.Fatalf("expected output validation error, got %q", stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected parent output validation before leaf execution, got %q", stderr)
	}
}

func TestRun_InvalidParentPrettyReturnsUsageBeforeLeafExec(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{
			"reviews",
			"--output", "table",
			"--pretty",
			"respond",
			"--review-id", "REVIEW_ID",
			"--response", "Thanks!",
		}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "--pretty is only valid with JSON output") {
		t.Fatalf("expected pretty/output validation error, got %q", stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected parent pretty validation before leaf execution, got %q", stderr)
	}
}

func TestRun_AuthTokenRequiresConfirm(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)

	cfg := &config.Config{
		DefaultKeyName: "default",
		Keys: []config.Credential{
			{
				Name:           "default",
				KeyID:          "KEY123",
				IssuerID:       "ISS456",
				PrivateKeyPath: keyPath,
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"auth", "token"}, "1.0.0")
		if code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected confirm required error, got %q", stderr)
	}
}

func TestRun_AuthTokenEnvOnlyCredentials(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	stdout, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"auth", "token", "--confirm", "--output", "json"}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Token   string `json:"token"`
		KeyID   string `json:"keyId"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.KeyID != "ENVKEY" {
		t.Fatalf("expected keyId ENVKEY, got %q", payload.KeyID)
	}
	if payload.Profile != "" {
		t.Fatalf("expected empty profile for env credentials, got %q", payload.Profile)
	}
	if parts := strings.Split(payload.Token, "."); len(parts) != 3 {
		t.Fatalf("expected JWT token, got %q", payload.Token)
	}
}

func TestRun_AuthTokenHonorsRootProfileFlag(t *testing.T) {
	resetReportFlags(t)

	configPath := writeAuthTokenProfilesConfig(t)
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	stdout, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"--profile", "second", "auth", "token", "--confirm", "--output", "json"}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		KeyID   string `json:"keyId"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.KeyID != "KEY_B" || payload.Profile != "second" {
		t.Fatalf("expected second profile payload, got %+v", payload)
	}
}

func TestRun_AuthTokenHonorsASCProfileEnv(t *testing.T) {
	resetReportFlags(t)

	configPath := writeAuthTokenProfilesConfig(t)
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "second")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	stdout, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"auth", "token", "--confirm", "--output", "json"}, "1.0.0")
		if code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		KeyID   string `json:"keyId"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.KeyID != "KEY_B" || payload.Profile != "second" {
		t.Fatalf("expected second profile payload, got %+v", payload)
	}
}

func TestRun_AuthTokenAmbiguousProfilesReturnAuthExit(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)

	cfg := &config.Config{
		DefaultKeyName: "",
		Keys: []config.Credential{
			{
				Name:           "first",
				KeyID:          "KEY_A",
				IssuerID:       "ISS_A",
				PrivateKeyPath: keyPath,
			},
			{
				Name:           "second",
				KeyID:          "KEY_B",
				IssuerID:       "ISS_B",
				PrivateKeyPath: keyPath,
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"auth", "token", "--confirm"}, "1.0.0")
		if code != ExitAuth {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitAuth)
		}
	})

	if !strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected missing authentication error, got %q", stderr)
	}
}

func TestRun_AuthTokenRejectsPermissiveKeyFile(t *testing.T) {
	resetReportFlags(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)
	if err := os.Chmod(keyPath, 0o644); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}

	cfg := &config.Config{
		DefaultKeyName: "default",
		Keys: []config.Credential{
			{
				Name:           "default",
				KeyID:          "KEY123",
				IssuerID:       "ISS456",
				PrivateKeyPath: keyPath,
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	resetSelectedProfile(t)

	_, stderr := captureCommandOutput(t, func() {
		code := Run([]string{"auth", "token", "--confirm"}, "1.0.0")
		if code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	if !strings.Contains(stderr, "private key file is too permissive") {
		t.Fatalf("expected permissive key file error, got %q", stderr)
	}
}

func TestWriteJUnitReport(t *testing.T) {
	resetReportFlags(t)

	reportPath := filepath.Join(t.TempDir(), "junit.xml")
	shared.SetReportFile(reportPath)
	t.Cleanup(func() {
		shared.SetReportFile("")
	})

	runErr := errors.New("boom")
	if err := writeJUnitReport("asc builds list", runErr, 2*time.Second); err != nil {
		t.Fatalf("writeJUnitReport() error: %v", err)
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var suite struct {
		XMLName   xml.Name `xml:"testsuite"`
		Failures  int      `xml:"failures,attr"`
		TestCases []struct {
			Name    string `xml:"name,attr"`
			Failure *struct {
				Type string `xml:"type,attr"`
			} `xml:"failure"`
		} `xml:"testcase"`
	}
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}
	if suite.Failures != 1 {
		t.Fatalf("failures = %d, want 1", suite.Failures)
	}
	if len(suite.TestCases) != 1 || suite.TestCases[0].Name != "asc builds list" {
		t.Fatalf("unexpected testcase payload: %+v", suite.TestCases)
	}
	if suite.TestCases[0].Failure == nil || suite.TestCases[0].Failure.Type != "ERROR" {
		t.Fatalf("expected failure type ERROR, got %+v", suite.TestCases[0].Failure)
	}
}

func resetReportFlags(t *testing.T) {
	t.Helper()
	shared.SetReportFormat("")
	shared.SetReportFile("")
}

func resetSelectedProfile(t *testing.T) {
	t.Helper()
	previousProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(previousProfile)
	})
}

func writeAuthTokenProfilesConfig(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)

	cfg := &config.Config{
		DefaultKeyName: "first",
		Keys: []config.Credential{
			{
				Name:           "first",
				KeyID:          "KEY_A",
				IssuerID:       "ISS_A",
				PrivateKeyPath: keyPath,
			},
			{
				Name:           "second",
				KeyID:          "KEY_B",
				IssuerID:       "ISS_B",
				PrivateKeyPath: keyPath,
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}
	return configPath
}

func writeRunTestECDSAPEM(t *testing.T, path string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
}

func captureCommandOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() stdout error: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() stderr error: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutR)
		_ = stdoutR.Close()
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrR)
		_ = stderrR.Close()
		errC <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = stdoutW.Close()
		_ = stderrW.Close()
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	return <-outC, <-errC
}

func runHelpSubprocess(t *testing.T, tempDir string, args ...string) (string, string, int) {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error: %v", err)
	}

	cmdArgs := []string{"-test.run=TestRunHelpHelperProcess", "--"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(exe, cmdArgs...)
	cmd.Env = append(os.Environ(),
		"GO_WANT_HELP_PROCESS=1",
		"ASC_BYPASS_KEYCHAIN=",
		"ASC_CONFIG_PATH="+filepath.Join(tempDir, "missing.json"),
		"ASC_PROFILE=",
		"ASC_KEY_ID=",
		"ASC_ISSUER_ID=",
		"ASC_PRIVATE_KEY_PATH=",
		"ASC_PRIVATE_KEY=",
		"ASC_PRIVATE_KEY_B64=",
		"ASC_STRICT_AUTH=",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("help subprocess error: %v", err)
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func TestRunHelpHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELP_PROCESS") != "1" {
		return
	}

	sep := -1
	for i, arg := range os.Args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep == -1 || sep+1 >= len(os.Args) {
		os.Exit(2)
	}

	code := Run(os.Args[sep+1:], "1.0.0")
	os.Exit(code)
}
