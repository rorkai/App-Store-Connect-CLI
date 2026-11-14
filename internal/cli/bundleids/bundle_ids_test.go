package bundleids

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBundleIDsGetCommand_MissingID(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := newBundleIDsGetCommand(d)

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
	assertUsageOutput(t, stdout, stderr, "Error: --id is required\n")
}

func TestBundleIDsCreateCommand_MissingIdentifier(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := newBundleIDsCreateCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--name", "Example"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --identifier is missing, got %v", err)
	}
	assertUsageOutput(t, stdout, stderr, "Error: --identifier is required\n")
}

func TestBundleIDsCreateCommand_MissingName(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := newBundleIDsCreateCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--identifier", "com.example.app"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --name is missing, got %v", err)
	}
	assertUsageOutput(t, stdout, stderr, "Error: --name is required\n")
}

func TestBundleIDsCreateCommand_UsesBundleIDPlatformContract(t *testing.T) {
	d, _, _ := newTestDeps()
	cmd := newBundleIDsCreateCommand(d)
	platformFlag := cmd.FlagSet.Lookup("platform")
	if platformFlag == nil {
		t.Fatal("expected --platform flag")
	}
	if !strings.Contains(platformFlag.Usage, "IOS, MAC_OS, UNIVERSAL") {
		t.Fatalf("--platform usage = %q, want BundleIdPlatform values", platformFlag.Usage)
	}
	if strings.Contains(platformFlag.Usage, "TV_OS") || strings.Contains(platformFlag.Usage, "VISION_OS") {
		t.Fatalf("--platform usage advertises general app platforms: %q", platformFlag.Usage)
	}

	if err := cmd.FlagSet.Parse([]string{
		"--identifier", "com.example.invalid",
		"--name", "Invalid",
		"--platform", "TV_OS",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil || err.Error() != "bundle-ids create: --platform must be one of: IOS, MAC_OS, UNIVERSAL" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBundleIDsUpdateCommand_MissingID(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := newBundleIDsUpdateCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--name", "New Name"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
	assertUsageOutput(t, stdout, stderr, "Error: --id is required\n")
}

func TestBundleIDsUpdateCommand_MissingName(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := newBundleIDsUpdateCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--id", "BUNDLE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --name is missing, got %v", err)
	}
	assertUsageOutput(t, stdout, stderr, "Error: --name is required\n")
}

func TestBundleIDsDeleteCommand_MissingConfirm(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := newBundleIDsDeleteCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--id", "BUNDLE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
	assertUsageOutput(t, stdout, stderr, "Error: --confirm is required\n")
}

func TestBundleIDsCapabilitiesAddCommand_MissingBundle(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := newBundleIDsCapabilitiesAddCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--capability", "ICLOUD"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --bundle is missing, got %v", err)
	}
	assertUsageOutput(t, stdout, stderr, "Error: --bundle is required\n")
}

func TestBundleIDsCapabilitiesAddCommand_MissingCapability(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := newBundleIDsCapabilitiesAddCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--bundle", "BUNDLE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --capability is missing, got %v", err)
	}
	assertUsageOutput(t, stdout, stderr, "Error: --capability is required\n")
}

func TestParseCapabilitySettingsRejectsInvalidStructure(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantErr   string
		wantValue string
		wantType  reflect.Type
	}{
		{
			name:    "unknown setting field",
			value:   `[{"key":"ICLOUD_VERSION","unexpected":true}]`,
			wantErr: `unknown field "unexpected"`,
		},
		{
			name:    "unknown option field",
			value:   `[{"key":"ICLOUD_VERSION","options":[{"key":"XCODE_6","unexpected":true}]}]`,
			wantErr: `unknown field "unexpected"`,
		},
		{
			name:    "incorrect setting field casing",
			value:   `[{"KEY":"ICLOUD_VERSION"}]`,
			wantErr: `unknown field "KEY"`,
		},
		{
			name:    "incorrect option field casing",
			value:   `[{"key":"ICLOUD_VERSION","Options":[{"key":"XCODE_6"}]}]`,
			wantErr: `unknown field "Options"`,
		},
		{
			name:    "empty allowed instances",
			value:   `[{"key":"ICLOUD_VERSION","allowedInstances":""}]`,
			wantErr: `allowedInstances at setting index 0 must not be empty`,
		},
		{
			name:    "whitespace allowed instances",
			value:   `[{"key":"ICLOUD_VERSION","allowedInstances":" \t"}]`,
			wantErr: `allowedInstances at setting index 0 must not be blank`,
		},
		{
			name:    "empty setting description",
			value:   `[{"key":"ICLOUD_VERSION","description":""}]`,
			wantErr: `description at setting index 0 must not be empty`,
		},
		{
			name:    "whitespace option name",
			value:   `[{"key":"FUTURE_SETTING","options":[{"key":"FUTURE_OPTION","name":"  "}]}]`,
			wantErr: `name at setting index 0, option index 0 must not be blank`,
		},
		{
			name:    "missing setting key",
			value:   `[{"options":[]}]`,
			wantErr: `capability setting key at index 0 must not be empty`,
		},
		{
			name:    "missing option key",
			value:   `[{"key":"FUTURE_SETTING","options":[{"enabled":true}]}]`,
			wantErr: `capability option key at setting index 0, option index 0 must not be empty`,
		},
		{
			name:      "setting key has wrong type",
			value:     `[{"key":42}]`,
			wantErr:   `--settings must be valid JSON array`,
			wantValue: "number",
			wantType:  reflect.TypeFor[string](),
		},
		{
			name:      "options has wrong shape",
			value:     `[{"key":"FUTURE_SETTING","options":{}}]`,
			wantErr:   `--settings must be valid JSON array`,
			wantValue: "object",
			wantType:  reflect.TypeFor[[]asc.CapabilityOption](),
		},
		{
			name:      "option has wrong shape",
			value:     `[{"key":"FUTURE_SETTING","options":[true]}]`,
			wantErr:   `--settings must be valid JSON array`,
			wantValue: "bool",
			wantType:  reflect.TypeFor[asc.CapabilityOption](),
		},
		{
			name:    "null is not an array",
			value:   `null`,
			wantErr: `must be a JSON array, got null`,
		},
		{
			name:    "null nested field",
			value:   `[{"key":"ICLOUD_VERSION","options":null}]`,
			wantErr: `settings[0].options must not be null`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseCapabilitySettings(tc.value)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
			if tc.wantType != nil {
				// Go releases can change field-path wording; retain the typed cause.
				var typeErr *json.UnmarshalTypeError
				if !errors.As(err, &typeErr) || typeErr.Value != tc.wantValue || typeErr.Type != tc.wantType {
					t.Fatalf("expected JSON %s into %v error, got %#v (%v)", tc.wantValue, tc.wantType, typeErr, err)
				}
			}
		})
	}
}

func TestParseCapabilitySettingsAcceptsForwardCompatibleKeysAndExactFields(t *testing.T) {
	settings, err := parseCapabilitySettings(`[
		{"key":"ICLOUD_VERSION","name":"iCloud","description":"version","enabledByDefault":true,"visible":true,"allowedInstances":"SINGLE","minInstances":1,"options":[
			{"key":"XCODE_5","name":"Xcode 5","description":"legacy","enabledByDefault":false,"enabled":false,"supportsWildcard":true},
			{"key":"XCODE_6","enabled":true}
		]},
		{"key":"ENABLED_FOR_MAC_APP_SETUP","options":[{"key":"USE_IOS_APPID","enabled":true}]},
		{"key":"APP_GROUP_IDENTIFIERS","options":[{"key":"group.com.example.shared","enabled":true}]},
		{"key":"FUTURE_SETTING","allowedInstances":"FUTURE_INSTANCE_MODE","options":[{"key":"FUTURE_OPTION"}]}
	]`)
	if err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	if len(settings) != 4 {
		t.Fatalf("settings count = %d, want 4", len(settings))
	}
	if settings[0].MinInstances == nil || *settings[0].MinInstances != 1 {
		t.Fatalf("minInstances = %v, want 1", settings[0].MinInstances)
	}
	if settings[0].Options[0].Enabled == nil || *settings[0].Options[0].Enabled {
		t.Fatalf("explicit enabled=false was not preserved: %+v", settings[0].Options[0].Enabled)
	}
	if settings[0].Options[0].SupportsWildcard == nil || !*settings[0].Options[0].SupportsWildcard {
		t.Fatalf("supportsWildcard=true was not preserved: %+v", settings[0].Options[0].SupportsWildcard)
	}
	if settings[1].Key != "ENABLED_FOR_MAC_APP_SETUP" || settings[1].Options[0].Key != "USE_IOS_APPID" {
		t.Fatalf("live Apple setting/option pair was not preserved: %+v", settings[1])
	}
	if settings[2].Key != "APP_GROUP_IDENTIFIERS" || settings[2].Options[0].Key != "group.com.example.shared" {
		t.Fatalf("app group setting/option pair was not preserved: %+v", settings[2])
	}
	if settings[3].Key != "FUTURE_SETTING" || settings[3].AllowedInstances != "FUTURE_INSTANCE_MODE" || settings[3].Options[0].Key != "FUTURE_OPTION" {
		t.Fatalf("future setting values were not preserved: %+v", settings[3])
	}
}

func TestBundleIDsCapabilitiesRemoveCommand_MissingID(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := newBundleIDsCapabilitiesRemoveCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
	assertUsageOutput(t, stdout, stderr, "Error: --id is required\n")
}

func TestBundleIDsCapabilitiesRemoveCommand_MissingConfirm(t *testing.T) {
	d, stdout, stderr := newTestDeps()
	cmd := newBundleIDsCapabilitiesRemoveCommand(d)

	if err := cmd.FlagSet.Parse([]string{"--id", "CAPABILITY_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
	assertUsageOutput(t, stdout, stderr, "Error: --confirm is required\n")
}

func TestExtractBundleIDFromNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/bundleIds/bundle-123/profiles?cursor=abc"
	got, err := extractBundleIDFromNextURL(next)
	if err != nil {
		t.Fatalf("extractBundleIDFromNextURL() error: %v", err)
	}
	if got != "bundle-123" {
		t.Fatalf("expected bundle-123, got %q", got)
	}
}

func TestExtractBundleIDFromNextURLRelationships(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/bundleIds/bundle-123/relationships/profiles?cursor=abc"
	got, err := extractBundleIDFromNextURL(next)
	if err != nil {
		t.Fatalf("extractBundleIDFromNextURL() error: %v", err)
	}
	if got != "bundle-123" {
		t.Fatalf("expected bundle-123, got %q", got)
	}
}

func TestExtractBundleIDFromNextURL_Invalid(t *testing.T) {
	_, err := extractBundleIDFromNextURL("https://api.appstoreconnect.apple.com/v1/bundleIds")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExtractBundleIDFromNextURL_RejectsMalformedHost(t *testing.T) {
	tests := []string{
		"http://localhost:80:80/v1/bundleIds/bundle-123/profiles?cursor=abc",
		"http://::1/v1/bundleIds/bundle-123/profiles?cursor=abc",
	}

	for _, next := range tests {
		t.Run(next, func(t *testing.T) {
			if _, err := extractBundleIDFromNextURL(next); err == nil {
				t.Fatalf("expected error for malformed URL %q", next)
			}
		})
	}
}

func newTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{IO: &shared.IO{Out: &stdout, ErrOut: &stderr}}, &stdout, &stderr
}

func assertUsageOutput(t *testing.T, stdout, stderr *bytes.Buffer, wantStderr string) {
	t.Helper()
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); got != wantStderr {
		t.Fatalf("stderr = %q, want %q", got, wantStderr)
	}
}
