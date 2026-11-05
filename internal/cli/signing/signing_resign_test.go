package signing

import (
	"archive/zip"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestSigningResignHelpChecksCommandSpecificFlags(t *testing.T) {
	command := SigningResignCommand()
	for _, name := range []string{"ipa", "output", "identity", "identity-password-file", "profiles-manifest", "rebase-team-claims"} {
		flagValue := command.FlagSet.Lookup(name)
		if flagValue == nil {
			t.Fatalf("missing --%s flag", name)
		}
	}
}

func TestSigningResignRebaseTeamClaimsFlagIsOptIn(t *testing.T) {
	command := SigningResignCommand()
	flagValue := command.FlagSet.Lookup("rebase-team-claims")
	if flagValue == nil {
		t.Fatal("missing --rebase-team-claims flag")
	}
	if flagValue.DefValue != "false" {
		t.Fatalf("--rebase-team-claims default = %q, want false", flagValue.DefValue)
	}
	if err := command.FlagSet.Parse([]string{"--rebase-team-claims"}); err != nil {
		t.Fatal(err)
	}
	if getter, ok := flagValue.Value.(flag.Getter); !ok || getter.Get() != true {
		t.Fatal("--rebase-team-claims did not enable the opt-in mode")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	originalExecute := executeSigningResignFn
	t.Cleanup(func() { executeSigningResignFn = originalExecute })
	var captured []signingResignOptions
	executeSigningResignFn = func(_ context.Context, options signingResignOptions) (signingResignResult, error) {
		captured = append(captured, options)
		return signingResignResult{SchemaVersion: 1, Command: "signing resign"}, nil
	}
	for _, enabled := range []bool{false, true} {
		args := []string{
			"--ipa", "input.ipa",
			"--output", "output.ipa",
			"--identity", "identity.p12",
			"--profiles-manifest", "profiles.json",
			"--format", "json",
		}
		if enabled {
			args = append(args, "--rebase-team-claims")
		}
		command := SigningResignCommand()
		if err := command.FlagSet.Parse(args); err != nil {
			t.Fatal(err)
		}
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("SigningResignCommand().Exec(enabled=%t) error = %v", enabled, err)
		}
	}
	if len(captured) != 2 || captured[0].RebaseTeamClaims || !captured[1].RebaseTeamClaims {
		t.Fatalf("executor options = %#v, want absent=false and present=true", captured)
	}
}

func TestSigningResignCommandRejectsInvalidFlagShapes(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "positional argument", args: []string{"unexpected"}},
		{name: "missing required flags", args: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := SigningResignCommand().Exec(context.Background(), test.args); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("SigningResignCommand().Exec() error = %v, want flag.ErrHelp classification", err)
			}
		})
	}
}

func TestSigningResignInvalidManifestDoesNotCreateOutputParent(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	temporary := t.TempDir()
	inputPath := filepath.Join(temporary, "input.ipa")
	writeSigningResignMinimalIPA(t, inputPath)
	manifestPath := filepath.Join(temporary, "profiles.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schemaVersion":1,"profiles":`), 0o600); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(temporary, "not-created", "output.ipa")
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(_ context.Context, _ string, _ ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{}, nil
	}
	_, err := executeSigningResignImplementation(context.Background(), signingResignOptions{
		IPAPath: inputPath, OutputPath: outputPath, IdentityPath: filepath.Join(temporary, "identity.p12"), ProfilesManifestPath: manifestPath,
	})
	if err == nil || !strings.Contains(err.Error(), "profiles manifest") {
		t.Fatalf("executeSigningResignImplementation() error = %v, want invalid manifest", err)
	}
	if !isSigningResignUsageError(err) {
		t.Fatalf("executeSigningResignImplementation() error = %v, want usage classification", err)
	}
	if _, statErr := os.Stat(filepath.Dir(outputPath)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("invalid manifest created output parent: stat error = %v", statErr)
	}
}

func TestSigningResignPreflightErrorsMapToUsageExit(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	for _, test := range []struct {
		name         string
		manifest     string
		platformName string
		supported    string
		wantMessage  string
	}{
		{
			name:     "malformed manifest",
			manifest: `{"schemaVersion":1,"profiles":`,
		},
		{
			name:     "duplicate manifest field",
			manifest: `{"schemaVersion":1,"schemaVersion":1,"profiles":[]}`,
		},
		{
			name:     "unknown manifest field",
			manifest: `{"schemaVersion":1,"profiles":[],"unexpected":true}`,
		},
		{
			name:         "unsupported target platform",
			manifest:     `{"schemaVersion":1,"profiles":[]}`,
			platformName: "macosx",
			supported:    "MacOSX",
			wantMessage:  "target platform",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			temporary := t.TempDir()
			inputPath := filepath.Join(temporary, "input.ipa")
			if test.platformName == "" {
				writeSigningResignMinimalIPA(t, inputPath)
			} else {
				writeSigningResignMinimalIPAForPlatform(t, inputPath, test.platformName, test.supported)
			}
			manifestPath := filepath.Join(temporary, "profiles.json")
			if err := os.WriteFile(manifestPath, []byte(test.manifest), 0o600); err != nil {
				t.Fatal(err)
			}

			originalTool := runSigningResignToolFn
			t.Cleanup(func() { runSigningResignToolFn = originalTool })
			runSigningResignToolFn = func(_ context.Context, _ string, _ ...string) (signingResignToolOutput, error) {
				return signingResignToolOutput{}, nil
			}
			original := executeSigningResignFn
			t.Cleanup(func() { executeSigningResignFn = original })
			executeSigningResignFn = executeSigningResignImplementation
			command := SigningResignCommand()
			if err := command.FlagSet.Parse([]string{
				"--ipa", inputPath,
				"--output", filepath.Join(temporary, "output.ipa"),
				"--identity", filepath.Join(temporary, "identity.p12"),
				"--profiles-manifest", manifestPath,
			}); err != nil {
				t.Fatal(err)
			}
			err := command.Exec(context.Background(), nil)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("SigningResignCommand().Exec() error = %v, want usage/exit-2 classification", err)
			}
			if test.wantMessage != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.wantMessage)) {
				t.Fatalf("SigningResignCommand().Exec() error = %v, want %q", err, test.wantMessage)
			}
		})
	}
}

func TestSigningResignCommandClassifiesPreflightUsageOnly(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	original := executeSigningResignFn
	t.Cleanup(func() { executeSigningResignFn = original })
	makeCommand := func() *ffcli.Command {
		command := SigningResignCommand()
		if err := command.FlagSet.Parse([]string{
			"--ipa", "input.ipa",
			"--output", "output.ipa",
			"--identity", "identity.p12",
			"--profiles-manifest", "profiles.json",
		}); err != nil {
			t.Fatal(err)
		}
		return command
	}

	executeSigningResignFn = func(context.Context, signingResignOptions) (signingResignResult, error) {
		return signingResignResult{}, signingResignUsage(errors.New("profiles manifest contains duplicate fields"))
	}
	if err := makeCommand().Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("preflight usage error = %v, want flag.ErrHelp classification", err)
	}

	executeSigningResignFn = func(context.Context, signingResignOptions) (signingResignResult, error) {
		return signingResignResult{}, errors.New("codesign failed")
	}
	if err := makeCommand().Exec(context.Background(), nil); err == nil || errors.Is(err, flag.ErrHelp) {
		t.Fatalf("operational error = %v, want non-usage error", err)
	}
}

func TestSigningResignOperationalFailuresRemainExecutionErrors(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	original := executeSigningResignFn
	t.Cleanup(func() { executeSigningResignFn = original })
	for _, message := range []string{
		"invalid ZIP archive",
		"unsafe archive entry",
		"missing profile",
		"profile-target mismatch",
		"entitlement preparation failed",
		"codesign failed",
		"verification failed",
	} {
		t.Run(message, func(t *testing.T) {
			executeSigningResignFn = func(context.Context, signingResignOptions) (signingResignResult, error) {
				return signingResignResult{}, errors.New(message)
			}
			command := SigningResignCommand()
			if err := command.FlagSet.Parse([]string{
				"--ipa", "input.ipa",
				"--output", "output.ipa",
				"--identity", "identity.p12",
				"--profiles-manifest", "profiles.json",
			}); err != nil {
				t.Fatal(err)
			}
			err := command.Exec(context.Background(), nil)
			if err == nil || errors.Is(err, flag.ErrHelp) {
				t.Fatalf("SigningResignCommand().Exec() error = %v, want execution failure", err)
			}
		})
	}
}

func TestReadSigningResignManifestFileFailureRemainsExecutionError(t *testing.T) {
	_, err := readSigningResignManifest(filepath.Join(t.TempDir(), "missing-profiles.json"))
	if err == nil || isSigningResignUsageError(err) {
		t.Fatalf("readSigningResignManifest() error = %v, want non-usage file failure", err)
	}
}

func TestSigningResignInvalidFormatDoesNotCreateOutputParent(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	temporary := t.TempDir()
	outputPath := filepath.Join(temporary, "not-created", "output.ipa")
	command := SigningResignCommand()
	if err := command.FlagSet.Parse([]string{
		"--ipa", filepath.Join(temporary, "input.ipa"),
		"--output", outputPath,
		"--identity", filepath.Join(temporary, "identity.p12"),
		"--profiles-manifest", filepath.Join(temporary, "profiles.json"),
		"--format", "yaml",
	}); err != nil {
		t.Fatal(err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "one of") {
		t.Fatalf("SigningResignCommand().Exec() error = %v, want invalid output format", err)
	}
	if _, statErr := os.Stat(filepath.Dir(outputPath)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("invalid format created output parent: stat error = %v", statErr)
	}
}

func writeSigningResignMinimalIPA(t *testing.T, pathValue string) {
	writeSigningResignMinimalIPAForPlatform(t, pathValue, "iphoneos", "iPhoneOS")
}

func writeSigningResignMinimalIPAForPlatform(t *testing.T, pathValue, platformName, supportedPlatform string) {
	t.Helper()
	info, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.app",
		"CFBundleExecutable":         "App",
		"DTPlatformName":             platformName,
		"CFBundleSupportedPlatforms": []string{supportedPlatform},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	executable := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	data := buildSigningResignZip(t, []signingResignZipEntry{
		{name: "Payload/App.app/Info.plist", data: info},
		{name: "Payload/App.app/App", data: executable, mode: 0o755},
	})
	if err := os.WriteFile(pathValue, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeSigningResignIPAWithNestedExtension(t *testing.T, pathValue string) {
	t.Helper()
	platform := map[string]any{
		"DTPlatformName":             "iphoneos",
		"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
	}
	mainInfo := make(map[string]any, len(platform)+2)
	for key, value := range platform {
		mainInfo[key] = value
	}
	mainInfo["CFBundleIdentifier"] = "com.example.app"
	mainInfo["CFBundleExecutable"] = "App"
	extensionInfo := make(map[string]any, len(platform)+2)
	for key, value := range platform {
		extensionInfo[key] = value
	}
	extensionInfo["CFBundleIdentifier"] = "com.example.app.extension"
	extensionInfo["CFBundleExecutable"] = "Extension"
	mainPlist, err := plist.Marshal(mainInfo, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	extensionPlist, err := plist.Marshal(extensionInfo, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	executable := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	data := buildSigningResignZip(t, []signingResignZipEntry{
		{name: "Payload/App.app/Info.plist", data: mainPlist},
		{name: "Payload/App.app/App", data: executable, mode: 0o755},
		{name: "Payload/App.app/PlugIns/Extension.appex/Info.plist", data: extensionPlist},
		{name: "Payload/App.app/PlugIns/Extension.appex/Extension", data: executable, mode: 0o755},
	})
	if err := os.WriteFile(pathValue, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeSigningResignManifestStrict(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "valid",
			data: `{"schemaVersion":1,"profiles":[{"bundleId":"com.example.app","profilePath":"profiles/app.mobileprovision"}]}`,
		},
		{
			name: "unknown field",
			data: `{"schemaVersion":1,"profiles":[],"extra":true}`,
			want: `unknown JSON field "extra"`,
		},
		{
			name: "duplicate field",
			data: `{"schemaVersion":1,"schemaVersion":1,"profiles":[{"bundleId":"com.example.app","profilePath":"app.mobileprovision"}]}`,
			want: "duplicate",
		},
		{
			name: "wildcard bundle",
			data: `{"schemaVersion":1,"profiles":[{"bundleId":"com.example.*","profilePath":"app.mobileprovision"}]}`,
			want: "non-wildcard",
		},
		{
			name: "traversal profile",
			data: `{"schemaVersion":1,"profiles":[{"bundleId":"com.example.app","profilePath":"../app.mobileprovision"}]}`,
			want: "escapes",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest, err := decodeSigningResignManifest([]byte(test.data))
			if test.want == "" {
				if err != nil {
					t.Fatalf("decodeSigningResignManifest() error = %v", err)
				}
				if len(manifest.Profiles) != 1 || manifest.Profiles[0].BundleID != "com.example.app" {
					t.Fatalf("decoded manifest = %#v", manifest)
				}
				return
			}
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.want)) {
				t.Fatalf("decodeSigningResignManifest() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestValidateSigningResignManifestTargetsClassifiesMappingShapeAsUsage(t *testing.T) {
	targetIDs := map[string]struct{}{"com.example.app": {}}
	for _, test := range []struct {
		name     string
		manifest signingResignManifest
	}{
		{
			name: "missing mapping",
			manifest: signingResignManifest{Profiles: []signingResignManifestEntry{
				{BundleID: "com.example.app"},
				{BundleID: "com.example.extension"},
			}},
		},
		{
			name: "extra mapping",
			manifest: signingResignManifest{Profiles: []signingResignManifestEntry{
				{BundleID: "com.example.other"},
			}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateSigningResignManifestTargets(test.manifest, targetIDs)
			if err == nil || !isSigningResignUsageError(err) {
				t.Fatalf("validateSigningResignManifestTargets() error = %v, want usage classification", err)
			}
		})
	}
}

func TestSigningResignCommandClassifiesManifestTargetMappingErrorsAsUsage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	originalTool := runSigningResignToolFn
	originalExecute := executeSigningResignFn
	t.Cleanup(func() {
		runSigningResignToolFn = originalTool
		executeSigningResignFn = originalExecute
	})
	runSigningResignToolFn = func(_ context.Context, _ string, _ ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{}, nil
	}
	executeSigningResignFn = executeSigningResignImplementation
	for _, test := range []struct {
		name     string
		writeIPA func(*testing.T, string)
		manifest string
	}{
		{
			name:     "missing target mapping",
			writeIPA: writeSigningResignIPAWithNestedExtension,
			manifest: `{"schemaVersion":1,"profiles":[{"bundleId":"com.example.app","profilePath":"profile.mobileprovision"}]}`,
		},
		{
			name:     "extra target mapping",
			writeIPA: writeSigningResignMinimalIPA,
			manifest: `{"schemaVersion":1,"profiles":[{"bundleId":"com.example.other","profilePath":"profile.mobileprovision"}]}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			temporary := t.TempDir()
			inputPath := filepath.Join(temporary, "input.ipa")
			test.writeIPA(t, inputPath)
			manifestPath := filepath.Join(temporary, "profiles.json")
			if err := os.WriteFile(manifestPath, []byte(test.manifest), 0o600); err != nil {
				t.Fatal(err)
			}
			command := SigningResignCommand()
			if err := command.FlagSet.Parse([]string{
				"--ipa", inputPath,
				"--output", filepath.Join(temporary, "output.ipa"),
				"--identity", filepath.Join(temporary, "identity.p12"),
				"--profiles-manifest", manifestPath,
			}); err != nil {
				t.Fatal(err)
			}
			err := command.Exec(context.Background(), nil)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("SigningResignCommand().Exec() error = %v, want usage/exit-2 classification", err)
			}
		})
	}
}

func TestReadSigningResignManifestPreservesWhitespacePathBytes(t *testing.T) {
	temporary := t.TempDir()
	manifestPath := filepath.Join(temporary, " profiles manifest.json ")
	profilePath := " profiles/profile.mobileprovision "
	manifestData := []byte(`{"schemaVersion":1,"profiles":[{"bundleId":"com.example.app","profilePath":" profiles/profile.mobileprovision "}]}`)
	if err := os.WriteFile(manifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}

	manifest, err := readSigningResignManifest(manifestPath)
	if err != nil {
		t.Fatalf("readSigningResignManifest() error = %v", err)
	}
	if got := manifest.Profiles[0].ProfilePath; got != profilePath {
		t.Fatalf("manifest profile path = %q, want exact path bytes %q", got, profilePath)
	}
}

func TestSigningResignCommandPreservesPathBytes(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	original := executeSigningResignFn
	t.Cleanup(func() { executeSigningResignFn = original })
	var got signingResignOptions
	executeSigningResignFn = func(_ context.Context, options signingResignOptions) (signingResignResult, error) {
		got = options
		return signingResignResult{SchemaVersion: 1, Command: "signing resign"}, nil
	}

	command := SigningResignCommand()
	values := []string{
		"--ipa", " input.ipa ",
		"--output", " output.ipa ",
		"--identity", " identity.p12 ",
		"--identity-password-file", " password file ",
		"--profiles-manifest", " profiles manifest.json ",
		"--format", "json",
	}
	if err := command.FlagSet.Parse(values); err != nil {
		t.Fatal(err)
	}
	if err := command.Exec(context.Background(), nil); err != nil {
		t.Fatalf("SigningResignCommand().Exec() error = %v", err)
	}
	want := signingResignOptions{
		IPAPath:              " input.ipa ",
		OutputPath:           " output.ipa ",
		IdentityPath:         " identity.p12 ",
		IdentityPasswordPath: " password file ",
		ProfilesManifestPath: " profiles manifest.json ",
	}
	if got != want {
		t.Fatalf("command options = %#v, want exact path bytes %#v", got, want)
	}
	if got := signingResignPathOrEmpty("   \t"); got != "" {
		t.Fatalf("whitespace-only optional path = %q, want empty", got)
	}
}

func TestExtractSigningResignCertificateMatchesOnlyLeaf(t *testing.T) {
	leafKey := mustRSAKey(t)
	chainKey := mustRSAKey(t)
	leaf := mustSigningCertificate(t, leafKey, 101)
	chain := mustSigningCertificate(t, chainKey, 102)
	original := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = original })
	runSigningResignToolFn = func(_ context.Context, _ string, args ...string) (signingResignToolOutput, error) {
		for _, arg := range args {
			if prefix, ok := strings.CutPrefix(arg, "--extract-certificates="); ok {
				if err := os.WriteFile(prefix+"0", leaf.Raw, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(prefix+"1", chain.Raw, 0o600); err != nil {
					t.Fatal(err)
				}
				return signingResignToolOutput{}, nil
			}
		}
		return signingResignToolOutput{}, nil
	}

	if err := extractSigningResignCertificate(context.Background(), "/tmp/app", certificateSHA256(leaf)); err != nil {
		t.Fatalf("extractSigningResignCertificate() leaf error = %v", err)
	}
	if err := extractSigningResignCertificate(context.Background(), "/tmp/app", certificateSHA256(chain)); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("extractSigningResignCertificate() chain error = %v, want leaf-only mismatch", err)
	}
}

func TestBuildSigningResignEntitlementsReplacesIdentity(t *testing.T) {
	existing := map[string]any{
		"application-identifier":                "OLDTEAM.com.example.app",
		"com.apple.application-identifier":      "OLDTEAM.com.example.app",
		"com.apple.developer.team-identifier":   "OLDTEAM",
		"get-task-allow":                        true,
		"com.apple.security.application-groups": []string{"group.com.example"},
	}
	profile := map[string]any{
		"application-identifier":                "NEWTEAM.com.example.app",
		"com.apple.application-identifier":      "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier":   "NEWTEAM",
		"get-task-allow":                        false,
		"com.apple.security.application-groups": []any{"group.com.example", "group.other"},
	}
	got, err := buildSigningResignEntitlements(existing, profile)
	if err != nil {
		t.Fatalf("buildSigningResignEntitlements() error = %v", err)
	}
	if got["application-identifier"] != "NEWTEAM.com.example.app" || got["com.apple.developer.team-identifier"] != "NEWTEAM" {
		t.Fatalf("identity entitlements = %#v", got)
	}
	if got["get-task-allow"] != false || len(got) != len(existing) {
		t.Fatalf("rewritten entitlements = %#v", got)
	}
	if !signingResignEntitlementValuePermits(profile["com.apple.security.application-groups"], got["com.apple.security.application-groups"]) {
		t.Fatalf("capability entitlement was not preserved: %#v", got)
	}
}

func TestBuildSigningResignEntitlementsDerivesPushEnvironmentFromProfileClass(t *testing.T) {
	existing := map[string]any{
		"application-identifier":              "OLDTEAM.com.example.app",
		"com.apple.developer.team-identifier": "OLDTEAM",
		"get-task-allow":                      true,
		"aps-environment":                     "development",
	}
	profile := signingResignProfile{
		Class: signingResignProfileClassAdHoc,
		Entitlements: map[string]any{
			"application-identifier":              "NEWTEAM.com.example.app",
			"com.apple.developer.team-identifier": "NEWTEAM",
			"get-task-allow":                      false,
			"aps-environment":                     "production",
		},
	}

	got, err := buildSigningResignEntitlementsForProfile(existing, profile)
	if err != nil {
		t.Fatalf("buildSigningResignEntitlementsForProfile() error = %v", err)
	}
	if got["aps-environment"] != "production" {
		t.Fatalf("aps-environment = %#v, want replacement profile class value", got["aps-environment"])
	}
}

func TestBuildSigningResignEntitlementsRebindsAppAttestByProfileClass(t *testing.T) {
	const key = "com.apple.developer.devicecheck.appattest-environment"
	existing := map[string]any{
		"application-identifier":              "OLDTEAM.com.example.app",
		"com.apple.developer.team-identifier": "OLDTEAM",
		"get-task-allow":                      true,
		key:                                   "development",
	}
	profile := signingResignProfile{
		Class: signingResignProfileClassAdHoc,
		Entitlements: map[string]any{
			"application-identifier":              "NEWTEAM.com.example.app",
			"com.apple.developer.team-identifier": "NEWTEAM",
			"get-task-allow":                      false,
			key:                                   "production",
		},
	}
	got, err := buildSigningResignEntitlementsForProfile(existing, profile)
	if err != nil {
		t.Fatalf("buildSigningResignEntitlementsForProfile() error = %v", err)
	}
	if got[key] != "production" {
		t.Fatalf("app attest environment = %#v, want replacement profile class value", got[key])
	}
	delete(existing, key)
	got, err = buildSigningResignEntitlementsForProfile(existing, profile)
	if err != nil {
		t.Fatalf("buildSigningResignEntitlementsForProfile() error = %v", err)
	}
	if _, exists := got[key]; exists {
		t.Fatal("app attest environment granted without an existing claim")
	}
}

func TestBuildSigningResignEntitlementsRebindsBetaReportsByProfileClass(t *testing.T) {
	baseProfile := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.application-identifier":    "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
	}
	tests := []struct {
		name            string
		class           string
		existingBeta    any
		existingPresent bool
		profileBeta     any
		profilePresent  bool
		wantBeta        any
		wantPresent     bool
		wantErr         bool
	}{
		{
			name:            "app store keeps authorized claim",
			class:           signingResignProfileClassAppStore,
			existingBeta:    true,
			existingPresent: true,
			profileBeta:     true,
			profilePresent:  true,
			wantBeta:        true,
			wantPresent:     true,
		},
		{
			name:            "app store rebinds false source",
			class:           signingResignProfileClassAppStore,
			existingBeta:    false,
			existingPresent: true,
			profileBeta:     true,
			profilePresent:  true,
			wantBeta:        true,
			wantPresent:     true,
		},
		{
			name:            "development removes store claim",
			class:           signingResignProfileClassDevelopment,
			existingBeta:    true,
			existingPresent: true,
			wantPresent:     false,
		},
		{
			name:            "ad hoc removes store claim",
			class:           signingResignProfileClassAdHoc,
			existingBeta:    true,
			existingPresent: true,
			profileBeta:     false,
			profilePresent:  true,
			wantPresent:     false,
		},
		{
			name:           "development profile cannot authorize store claim",
			class:          signingResignProfileClassDevelopment,
			profileBeta:    true,
			profilePresent: true,
			wantErr:        true,
		},
		{
			name:           "invalid app store profile value",
			class:          signingResignProfileClassAppStore,
			profileBeta:    "yes",
			profilePresent: true,
			wantErr:        true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			existing := map[string]any{
				"application-identifier":              "OLDTEAM.com.example.app",
				"com.apple.developer.team-identifier": "OLDTEAM",
				"get-task-allow":                      false,
			}
			if test.existingPresent {
				existing["beta-reports-active"] = test.existingBeta
			}
			profile := make(map[string]any, len(baseProfile)+1)
			for key, value := range baseProfile {
				profile[key] = value
			}
			if test.profilePresent {
				profile["beta-reports-active"] = test.profileBeta
			}
			got, err := buildSigningResignEntitlementsForProfile(existing, signingResignProfile{
				Class:        test.class,
				Entitlements: profile,
			})
			if (err != nil) != test.wantErr {
				t.Fatalf("buildSigningResignEntitlementsForProfile() error = %v, wantErr %v", err, test.wantErr)
			}
			if err != nil {
				return
			}
			gotBeta, gotPresent := got["beta-reports-active"]
			if gotPresent != test.wantPresent || (gotPresent && gotBeta != test.wantBeta) {
				t.Fatalf("beta-reports-active = %#v (present %v), want %#v (present %v); entitlements = %#v", gotBeta, gotPresent, test.wantBeta, test.wantPresent, got)
			}
		})
	}
}

func TestBuildSigningResignEntitlementsRebindsICloudEnvironmentByProfileClass(t *testing.T) {
	baseProfile := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.application-identifier":    "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
	}
	tests := []struct {
		name            string
		class           string
		existingEnv     any
		existingPresent bool
		profileEnv      any
		profilePresent  bool
		wantEnv         any
		wantPresent     bool
		wantErr         bool
	}{
		{
			name:            "development to distribution",
			class:           signingResignProfileClassAdHoc,
			existingEnv:     "Development",
			existingPresent: true,
			profileEnv:      "Production",
			profilePresent:  true,
			wantEnv:         "Production",
			wantPresent:     true,
		},
		{
			name:            "distribution to development",
			class:           signingResignProfileClassDevelopment,
			existingEnv:     "Production",
			existingPresent: true,
			profileEnv:      "Development",
			profilePresent:  true,
			wantEnv:         "Development",
			wantPresent:     true,
		},
		{
			name:           "absent capability is not granted",
			class:          signingResignProfileClassAppStore,
			profileEnv:     "Production",
			profilePresent: true,
			wantPresent:    false,
		},
		{
			name:            "missing replacement authorization remains blocked",
			class:           signingResignProfileClassAppStore,
			existingEnv:     "Development",
			existingPresent: true,
			wantErr:         true,
		},
		{
			name:            "profile value must match class",
			class:           signingResignProfileClassDevelopment,
			existingEnv:     "Development",
			existingPresent: true,
			profileEnv:      "Production",
			profilePresent:  true,
			wantErr:         true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			existing := map[string]any{
				"application-identifier":              "OLDTEAM.com.example.app",
				"com.apple.developer.team-identifier": "OLDTEAM",
				"get-task-allow":                      false,
			}
			if test.existingPresent {
				existing["com.apple.developer.icloud-container-environment"] = test.existingEnv
			}
			profile := make(map[string]any, len(baseProfile)+1)
			for key, value := range baseProfile {
				profile[key] = value
			}
			if test.profilePresent {
				profile["com.apple.developer.icloud-container-environment"] = test.profileEnv
			}
			got, err := buildSigningResignEntitlementsForProfile(existing, signingResignProfile{
				Class:        test.class,
				Entitlements: profile,
			})
			if (err != nil) != test.wantErr {
				t.Fatalf("buildSigningResignEntitlementsForProfile() error = %v, wantErr %v", err, test.wantErr)
			}
			if err != nil {
				return
			}
			gotEnv, gotPresent := got["com.apple.developer.icloud-container-environment"]
			if gotPresent != test.wantPresent || (gotPresent && gotEnv != test.wantEnv) {
				t.Fatalf("iCloud environment = %#v (present %v), want %#v (present %v); entitlements = %#v", gotEnv, gotPresent, test.wantEnv, test.wantPresent, got)
			}
		})
	}
}

func TestBuildSigningResignEntitlementsKeepsConcreteValuesForWildcardProfileClaims(t *testing.T) {
	existing := map[string]any{
		"application-identifier":                             "OLDTEAM.com.example.app",
		"com.apple.application-identifier":                   "OLDTEAM.com.example.app",
		"com.apple.developer.team-identifier":                "OLDTEAM",
		"get-task-allow":                                     true,
		"keychain-access-groups":                             []string{"NEWTEAM.com.example.shared"},
		"com.apple.developer.ubiquity-kvstore-identifier":    "NEWTEAM.com.example.app",
		"com.apple.developer.parent-application-identifiers": []string{"NEWTEAM.com.example.parent"},
	}
	profile := map[string]any{
		"application-identifier":                             "NEWTEAM.com.example.app",
		"com.apple.application-identifier":                   "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier":                "NEWTEAM",
		"get-task-allow":                                     false,
		"keychain-access-groups":                             []any{"NEWTEAM.*"},
		"com.apple.developer.ubiquity-kvstore-identifier":    "NEWTEAM.*",
		"com.apple.developer.parent-application-identifiers": []any{"NEWTEAM.*"},
	}

	got, err := buildSigningResignEntitlements(existing, profile)
	if err != nil {
		t.Fatalf("buildSigningResignEntitlements() error = %v", err)
	}
	for _, key := range []string{
		"keychain-access-groups",
		"com.apple.developer.ubiquity-kvstore-identifier",
		"com.apple.developer.parent-application-identifiers",
	} {
		if !signingResignEntitlementValuesEqual(got[key], existing[key]) {
			t.Fatalf("identity entitlement %s = %#v, want concrete existing value %#v", key, got[key], existing[key])
		}
		if signingResignEntitlementContainsWildcard(got[key]) {
			t.Fatalf("identity entitlement %s retained a wildcard: %#v", key, got[key])
		}
	}
}

func TestValidateSigningResignExistingEntitlementsRequiresCoherentIdentity(t *testing.T) {
	base := map[string]any{
		"application-identifier":              "OLDTEAM.com.example.app",
		"com.apple.developer.team-identifier": "OLDTEAM",
	}
	for _, test := range []struct {
		name       string
		mutate     func(map[string]any)
		wantErr    bool
		wantDetail string
	}{
		{
			name: "coherent old identity",
		},
		{
			name: "contradictory application identifiers",
			mutate: func(values map[string]any) {
				values["com.apple.application-identifier"] = "DIFFERENT.com.example.app"
			},
			wantErr:    true,
			wantDetail: "contradictory",
		},
		{
			name: "application identifier suffix mismatch",
			mutate: func(values map[string]any) {
				values["application-identifier"] = "OLDTEAM.com.example.other"
			},
			wantErr:    true,
			wantDetail: "target bundle identifier",
		},
		{
			name: "legacy prefix differs from old team",
			mutate: func(values map[string]any) {
				values["com.apple.developer.team-identifier"] = "DIFFERENT"
			},
		},
		{
			name: "missing optional synonym",
			mutate: func(values map[string]any) {
				delete(values, "com.apple.application-identifier")
			},
		},
		{
			name: "wildcard application identifier",
			mutate: func(values map[string]any) {
				values["application-identifier"] = "OLDTEAM.com.example.*"
			},
			wantErr:    true,
			wantDetail: "invalid",
		},
		{
			name: "team-only concrete claim",
			mutate: func(values map[string]any) {
				delete(values, "application-identifier")
			},
		},
		{
			name: "wildcard team identifier",
			mutate: func(values map[string]any) {
				values["com.apple.developer.team-identifier"] = "OLD*TEAM"
			},
			wantErr:    true,
			wantDetail: "invalid",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := make(map[string]any, len(base)+1)
			for key, value := range base {
				values[key] = value
			}
			if test.mutate != nil {
				test.mutate(values)
			}
			err := validateSigningResignExistingEntitlements(values, "com.example.app")
			if (err != nil) != test.wantErr {
				t.Fatalf("validateSigningResignExistingEntitlements() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.wantDetail != "" && (err == nil || !strings.Contains(err.Error(), test.wantDetail)) {
				t.Fatalf("validateSigningResignExistingEntitlements() error = %v, want %q", err, test.wantDetail)
			}
		})
	}
}

func TestSigningResignExistingOtherTeamCanBeReplaced(t *testing.T) {
	existing := map[string]any{
		"application-identifier":              "OTHERPREFIX.com.example.app",
		"com.apple.developer.team-identifier": "OTHERTEAM",
	}
	if err := validateSigningResignExistingEntitlements(existing, "com.example.app"); err != nil {
		t.Fatalf("validateSigningResignExistingEntitlements() error = %v", err)
	}
	profile := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.application-identifier":    "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
	}
	got, err := buildSigningResignEntitlements(existing, profile)
	if err != nil {
		t.Fatalf("buildSigningResignEntitlements() error = %v", err)
	}
	if got["application-identifier"] != "NEWTEAM.com.example.app" || got["com.apple.developer.team-identifier"] != "NEWTEAM" {
		t.Fatalf("rewritten identity entitlements = %#v", got)
	}
}

func TestPrepareSigningResignTreeRejectsIncoherentInputBeforeMutation(t *testing.T) {
	stagePath := t.TempDir()
	treePath := filepath.Join(stagePath, "tree")
	if err := os.Mkdir(treePath, 0o700); err != nil {
		t.Fatal(err)
	}
	stageRoot, err := rootfs.New(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()
	treeRoot, err := rootfs.New(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer treeRoot.Close()
	profile := signingResignProfile{Data: []byte("profile"), Entitlements: map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.application-identifier":    "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
	}}
	_, err = prepareSigningResignTree(context.Background(), stageRoot, treeRoot, signingResignArchive{Targets: []signingResignTarget{{
		RelativePath: "Payload/App.app", BundleID: "com.example.app", ExistingEntitlements: map[string]any{
			"application-identifier":              "OLDTEAM.com.example.other",
			"com.apple.developer.team-identifier": "OLDTEAM",
		},
	}}}, map[string]signingResignProfile{"com.example.app": profile})
	if err == nil {
		t.Fatal("prepareSigningResignTree() accepted incoherent input entitlements")
	}
	if _, statErr := os.Stat(filepath.Join(stagePath, "entitlements")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("incoherent input created private entitlements directory: %v", statErr)
	}
}

func TestPrepareSigningResignTreePlansAllTargetsBeforeLaterValidationFailure(t *testing.T) {
	stagePath := t.TempDir()
	treePath := filepath.Join(stagePath, "tree")
	if err := os.MkdirAll(filepath.Join(treePath, "Payload", "App.app"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(treePath, "Payload", "App.app", "PlugIns", "Feature.appex"), 0o700); err != nil {
		t.Fatal(err)
	}
	// This failure occurs after target/profile planning in the legacy flow,
	// making it a regression test for whole-IPA no-mutation preflight.
	if err := os.WriteFile(filepath.Join(treePath, "SwiftSupport"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	stageRoot, err := rootfs.New(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()
	treeRoot, err := rootfs.New(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer treeRoot.Close()
	profile := func(bundleID string) signingResignProfile {
		return signingResignProfile{Data: []byte("replacement profile"), Entitlements: map[string]any{
			"application-identifier":              "NEWPREFIX." + bundleID,
			"com.apple.application-identifier":    "NEWPREFIX." + bundleID,
			"com.apple.developer.team-identifier": "NEWTEAM",
			"get-task-allow":                      false,
		}}
	}
	archive := signingResignArchive{
		MainPath: "Payload/App.app",
		Targets: []signingResignTarget{
			{Kind: "application", RelativePath: "Payload/App.app", BundleID: "com.example.app"},
			{Kind: "app-extension", RelativePath: "Payload/App.app/PlugIns/Feature.appex", BundleID: "com.example.app.Feature"},
		},
	}
	_, err = prepareSigningResignTree(context.Background(), stageRoot, treeRoot, archive, map[string]signingResignProfile{
		"com.example.app":         profile("com.example.app"),
		"com.example.app.Feature": profile("com.example.app.Feature"),
	})
	if err == nil || !strings.Contains(err.Error(), "SwiftSupport") {
		t.Fatalf("prepareSigningResignTree() error = %v, want later preserved-directory failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(stagePath, "entitlements")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("later preflight failure left entitlement staging behind: %v", statErr)
	}
	for _, target := range archive.Targets {
		if _, statErr := os.Stat(filepath.Join(treePath, filepath.FromSlash(target.RelativePath), "embedded.mobileprovision")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("later preflight failure embedded profile for %s: %v", target.BundleID, statErr)
		}
	}
}

func TestValidateSigningResignProfileForTarget(t *testing.T) {
	profile := signingResignProfile{
		BundleID: "com.example.app", TeamID: "NEWTEAM", ApplicationIdentifierPrefix: "NEWTEAM",
		Entitlements: map[string]any{
			"application-identifier":              "NEWTEAM.com.example.app",
			"com.apple.application-identifier":    "NEWTEAM.com.example.app",
			"com.apple.developer.team-identifier": "NEWTEAM",
		},
	}
	if err := validateSigningResignProfileForTarget(profile, "com.example.app"); err != nil {
		t.Fatalf("validateSigningResignProfileForTarget() error = %v", err)
	}
	profile.Entitlements["application-identifier"] = "NEWTEAM.com.example.other"
	if err := validateSigningResignProfileForTarget(profile, "com.example.app"); err == nil {
		t.Fatal("validateSigningResignProfileForTarget() accepted mismatched profile identifier")
	}
}

func TestBuildSigningResignEntitlementsRejectsUnpermittedCapability(t *testing.T) {
	_, err := buildSigningResignEntitlements(
		map[string]any{"com.apple.security.application-groups": []string{"group.not-permitted"}},
		map[string]any{
			"application-identifier":                "TEAM.com.example.app",
			"com.apple.developer.team-identifier":   "TEAM",
			"get-task-allow":                        false,
			"com.apple.security.application-groups": []any{"group.allowed"},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "not authorized by the replacement profile") {
		t.Fatalf("buildSigningResignEntitlements() error = %v, want unpermitted capability", err)
	}
}

func TestValidateSigningResignArchiveRejectsPathCollisions(t *testing.T) {
	data := buildSigningResignZip(t, []signingResignZipEntry{
		{name: "Payload/App.app/a/file", data: []byte("x")},
		{name: "Payload/App.app/a", data: []byte("not-a-directory")},
	})
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSigningResignArchive(context.Background(), reader); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("validateSigningResignArchive() error = %v, want collision", err)
	}
}

func TestSnapshotSigningResignIPAComputesDigestAndDoesNotRewriteSource(t *testing.T) {
	directory := t.TempDir()
	inputPath := filepath.Join(directory, "input.ipa")
	contents := []byte("deterministic ipa bytes")
	if err := os.WriteFile(inputPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	stagePath := filepath.Join(directory, "stage")
	if err := os.Mkdir(stagePath, 0o700); err != nil {
		t.Fatal(err)
	}
	stage, err := os.OpenRoot(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	snapshot, digest, err := snapshotSigningResignIPA(context.Background(), source, int64(len(contents)), stage)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	if digest != signingResignSHA256(contents) {
		t.Fatalf("snapshot digest = %q, want %q", digest, signingResignSHA256(contents))
	}
	snapshotData, err := io.ReadAll(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(snapshotData, contents) {
		t.Fatalf("snapshot data = %q, want %q", snapshotData, contents)
	}
	sourceData, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sourceData, contents) {
		t.Fatalf("source changed to %q", sourceData)
	}
}

func TestSigningResignToolInvocationsUseExplicitKeychainAndNoDeepMutation(t *testing.T) {
	original := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = original })
	var calls [][]string
	runSigningResignToolFn = func(_ context.Context, executable string, args ...string) (signingResignToolOutput, error) {
		calls = append(calls, append([]string{executable}, args...))
		return signingResignToolOutput{}, nil
	}
	if err := signSigningResignObject(context.Background(), "/tmp/object", "ABC123", "/tmp/keychain", "/tmp/entitlements.plist"); err != nil {
		t.Fatal(err)
	}
	if err := verifySigningResignObject(context.Background(), "/tmp/object", "TEAM", false); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("tool calls = %#v", calls)
	}
	if strings.Contains(strings.Join(calls[0], " "), "--deep") || !strings.Contains(strings.Join(calls[0], " "), "--keychain /tmp/keychain") {
		t.Fatalf("sign invocation = %#v", calls[0])
	}
	if strings.Contains(strings.Join(calls[1], " "), "--deep") {
		t.Fatalf("leaf verify invocation unexpectedly deep = %#v", calls[1])
	}
}

func TestSigningResignResultRedactsInputPath(t *testing.T) {
	result := signingResignResult{
		SchemaVersion: 1,
		Command:       "signing resign",
		Input: signingResignInputResult{
			SizeBytes: 42,
			SHA256:    strings.Repeat("A", 64),
		},
		Output: signingResignArtifactResult{
			Path:      "/safe/output/resigned.ipa",
			SizeBytes: 42,
			SHA256:    strings.Repeat("B", 64),
		},
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "source-private-input") {
		t.Fatalf("result unexpectedly contains a source path: %s", encoded)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.Unmarshal(decoded["input"], &input); err != nil {
		t.Fatal(err)
	}
	if _, exists := input["path"]; exists {
		t.Fatalf("input result unexpectedly exposes path: %s", encoded)
	}
	if _, exists := decoded["output"]; !exists {
		t.Fatalf("result lost output artifact: %s", encoded)
	}
}

func TestValidateSigningResignPlatformAcceptsWatchOSForWatchTargets(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		info    map[string]any
		wantErr bool
		usage   bool
	}{
		{
			name: "watch application",
			kind: "watch-application",
			info: map[string]any{
				"DTPlatformName":             "watchos",
				"CFBundleSupportedPlatforms": []string{"WatchOS"},
			},
		},
		{
			name: "watch extension",
			kind: "watch-extension",
			info: map[string]any{
				"DTPlatformName":             "watchos",
				"CFBundleSupportedPlatforms": []string{"WatchOS"},
			},
		},
		{
			name: "watch application platform only",
			kind: "watch-application",
			info: map[string]any{"DTPlatformName": "watchos"},
		},
		{
			name: "iOS application",
			kind: "application",
			info: map[string]any{
				"DTPlatformName":             "iphoneos",
				"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
			},
		},
		{
			name:    "watch metadata on iOS target",
			kind:    "application",
			info:    map[string]any{"DTPlatformName": "watchos"},
			wantErr: true,
			usage:   true,
		},
		{
			name:    "iOS metadata on watch target",
			kind:    "watch-application",
			info:    map[string]any{"DTPlatformName": "iphoneos"},
			wantErr: true,
			usage:   true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateSigningResignPlatform(test.info, test.kind)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateSigningResignPlatform() error = %v, wantErr %v", err, test.wantErr)
			}
			if isSigningResignUsageError(err) != test.usage {
				t.Fatalf("validateSigningResignPlatform() usage = %v, want %v (error = %v)", isSigningResignUsageError(err), test.usage, err)
			}
		})
	}
}

func TestSortSigningResignCodePlansIsLeafFirst(t *testing.T) {
	plans := []signingResignCodePlan{
		{Path: filepath.Join("tree", "App.app", "Frameworks", "Outer.framework", "Outer")},
		{Path: filepath.Join("tree", "App.app", "Frameworks", "Outer.framework", "Versions", "A", "Inner")},
		{Path: filepath.Join("tree", "App.app", "Frameworks", "Other.framework", "Other")},
	}
	sortSigningResignCodePlans(plans)
	innerIndex, outerIndex := -1, -1
	for index, plan := range plans {
		if strings.HasSuffix(plan.Path, filepath.Join("Versions", "A", "Inner")) {
			innerIndex = index
		}
		if strings.HasSuffix(plan.Path, filepath.Join("Outer.framework", "Outer")) {
			outerIndex = index
		}
	}
	if innerIndex < 0 || outerIndex < 0 || innerIndex >= outerIndex {
		t.Fatalf("outer framework executable was scheduled before its nested code: %#v", plans)
	}
}

func TestPrepareSigningResignTreePreservesSwiftSupportDylibs(t *testing.T) {
	stagePath := t.TempDir()
	treePath := filepath.Join(stagePath, "tree")
	if err := os.Mkdir(treePath, 0o700); err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(treePath, "Payload", "App.app")
	if err := os.MkdirAll(appPath, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "App"), data, 0o755); err != nil {
		t.Fatal(err)
	}
	swiftPath := filepath.Join(treePath, "SwiftSupport", "iphoneos")
	if err := os.MkdirAll(swiftPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(swiftPath, "libswiftCore.dylib"), data, 0o755); err != nil {
		t.Fatal(err)
	}
	stageRoot, err := rootfs.New(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()
	treeRoot, err := rootfs.New(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer treeRoot.Close()
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(_ context.Context, _ string, _ ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{}, nil
	}
	profile := signingResignProfile{
		Data: []byte("profile"),
		Entitlements: map[string]any{
			"application-identifier":              "TEAM.com.example.app",
			"com.apple.application-identifier":    "TEAM.com.example.app",
			"com.apple.developer.team-identifier": "TEAM",
			"get-task-allow":                      false,
		},
	}
	prepared, err := prepareSigningResignTree(context.Background(), stageRoot, treeRoot, signingResignArchive{
		MainPath: "Payload/App.app",
		Targets: []signingResignTarget{{
			Kind: "application", RelativePath: "Payload/App.app", BundleID: "com.example.app", Executable: "App", ProfileMode: 0o644,
		}},
	}, map[string]signingResignProfile{"com.example.app": profile})
	if err != nil {
		t.Fatalf("prepareSigningResignTree() error = %v", err)
	}
	if len(prepared.CodePlans) != 0 {
		t.Fatalf("SwiftSupport dylib was scheduled for app signing: %#v", prepared.CodePlans)
	}
	if got := mustReadFile(t, filepath.Join(swiftPath, "libswiftCore.dylib")); !bytes.Equal(got, data) {
		t.Fatal("SwiftSupport dylib changed during preparation")
	}
}

func TestSigningResignPreservedExternalCodeRequiresAppleVerification(t *testing.T) {
	if !isSigningResignPreservedExternalCodePath("/tmp/tree", "/tmp/tree/SwiftSupport/iphoneos/libswiftCore.dylib") {
		t.Fatal("exact SwiftSupport path was not recognized")
	}
	for _, pathValue := range []string{
		"/tmp/tree/SwiftSupport/iphoneos/nested/libswiftCore.dylib",
		"/tmp/tree/SwiftSupport/iphoneos/libswiftCore.DYLIB",
		"/tmp/tree/SwiftSupport/iphoneos/.dylib",
	} {
		if isSigningResignPreservedExternalCodePath("/tmp/tree", pathValue) {
			t.Fatalf("unsafe SwiftSupport path was accepted: %q", pathValue)
		}
	}
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	var calls [][]string
	runSigningResignToolFn = func(_ context.Context, executable string, args ...string) (signingResignToolOutput, error) {
		calls = append(calls, append([]string{executable}, args...))
		return signingResignToolOutput{}, errors.New("code signature is invalid")
	}
	err := verifySigningResignPreservedExternalCode(context.Background(), "/tmp/tree/SwiftSupport/iphoneos/libswiftCore.dylib")
	if err == nil {
		t.Fatal("verifySigningResignPreservedExternalCode() accepted an unverified artifact")
	}
	if len(calls) != 1 || len(calls[0]) < 6 || calls[0][1] != "--verify" || calls[0][2] != "--strict" || calls[0][4] != "-R=anchor apple generic" {
		t.Fatalf("SwiftSupport verification calls = %#v", calls)
	}
}

func TestSigningResignPreservedExternalCodeRejectsTemporaryPathReplacement(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(sourcePath, []byte("trusted code"), 0o755); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()

	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(_ context.Context, _ string, args ...string) (signingResignToolOutput, error) {
		verificationPath := args[len(args)-1]
		replacedPath := verificationPath + ".replaced"
		if err := os.Rename(verificationPath, replacedPath); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(verificationPath, []byte("attacker replacement"), 0o755); err != nil {
			t.Fatal(err)
		}
		return signingResignToolOutput{}, nil
	}

	err = verifySigningResignPreservedExternalCodeOpen(context.Background(), source, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "changed during verification") {
		t.Fatalf("verifySigningResignPreservedExternalCodeOpen() error = %v, want replacement rejection", err)
	}
}

func TestSigningResignPreservedExternalCodeRejectsSameLengthMutation(t *testing.T) {
	trusted := []byte("trusted code")
	mutated := []byte("mutated code")
	if len(trusted) != len(mutated) {
		t.Fatal("test mutation must preserve file length")
	}
	sourcePath := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(sourcePath, trusted, 0o755); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()

	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(_ context.Context, _ string, args ...string) (signingResignToolOutput, error) {
		verificationPath := args[len(args)-1]
		if err := os.WriteFile(verificationPath, mutated, 0o600); err != nil {
			t.Fatal(err)
		}
		return signingResignToolOutput{}, nil
	}

	err = verifySigningResignPreservedExternalCodeOpen(context.Background(), source, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "changed during verification") {
		t.Fatalf("verifySigningResignPreservedExternalCodeOpen() error = %v, want same-length mutation rejection", err)
	}
}

func TestSigningResignPreservedExternalCodeRejectsTemporaryRootReplacement(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(sourcePath, []byte("trusted code"), 0o755); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	parent := t.TempDir()
	verificationRoot := filepath.Join(parent, "verification-root")
	movedRoot := filepath.Join(parent, "original-root")
	if err := os.Mkdir(verificationRoot, 0o700); err != nil {
		t.Fatal(err)
	}

	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(_ context.Context, _ string, _ ...string) (signingResignToolOutput, error) {
		if err := os.Rename(verificationRoot, movedRoot); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(verificationRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		return signingResignToolOutput{}, nil
	}

	err = verifySigningResignPreservedExternalCodeOpen(context.Background(), source, verificationRoot)
	if err == nil || !strings.Contains(err.Error(), "changed during verification") {
		t.Fatalf("verifySigningResignPreservedExternalCodeOpen() error = %v, want root replacement rejection", err)
	}
}

func TestValidateSigningResignSwiftSupportRejectsTamperedAndNestedEntries(t *testing.T) {
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	for _, test := range []struct {
		name  string
		setup func(t *testing.T, root string)
		want  string
	}{
		{
			name: "unsigned dylib",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, "libswiftCore.dylib"), []byte("tampered"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "verify preserved SwiftSupport code",
		},
		{
			name: "nested entry",
			setup: func(t *testing.T, root string) {
				t.Helper()
				nested := filepath.Join(root, "nested")
				if err := os.Mkdir(nested, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(nested, "libswiftCore.dylib"), []byte("nested"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "nested",
		},
		{
			name: "direct non-dylib entry",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, "README"), []byte("unexpected"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "unsupported",
		},
		{
			name: "symbolic link",
			setup: func(t *testing.T, root string) {
				t.Helper()
				target := filepath.Join(root, "z-libswiftCore-real.dylib")
				if err := os.WriteFile(target, []byte("runtime"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(root, "libswiftCore.dylib")); err != nil {
					t.Skipf("symlink creation unavailable: %v", err)
				}
			},
			want: "nested or symbolic-link",
		},
		{
			name: "root file",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(filepath.Dir(root), "README"), []byte("unexpected"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "only the iphoneos directory",
		},
		{
			name: "other platform directory",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Mkdir(filepath.Join(filepath.Dir(root), "watchos"), 0o700); err != nil {
					t.Fatal(err)
				}
			},
			want: "only the iphoneos directory",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			temporary := t.TempDir()
			root := filepath.Join(temporary, "SwiftSupport", "iphoneos")
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			test.setup(t, root)
			runSigningResignToolFn = func(_ context.Context, _ string, _ ...string) (signingResignToolOutput, error) {
				return signingResignToolOutput{}, errors.New("code object is not signed")
			}
			err := validateSigningResignSwiftSupport(context.Background(), temporary)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateSigningResignSwiftSupport() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateSigningResignSwiftSupportRejectsLaterSymlinkBeforeCodeVerification(t *testing.T) {
	temporary := t.TempDir()
	root := filepath.Join(temporary, "SwiftSupport", "iphoneos")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "a-libswiftCore-real.dylib")
	if err := os.WriteFile(target, []byte("runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "z-libswiftCore.dylib")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}

	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	toolCalls := 0
	runSigningResignToolFn = func(_ context.Context, _ string, _ ...string) (signingResignToolOutput, error) {
		toolCalls++
		return signingResignToolOutput{}, errors.New("code object is not signed")
	}

	err := validateSigningResignSwiftSupport(context.Background(), temporary)
	if err == nil || !strings.Contains(err.Error(), "nested or symbolic-link") {
		t.Fatalf("validateSigningResignSwiftSupport() error = %v, want symbolic-link rejection", err)
	}
	if toolCalls != 0 {
		t.Fatalf("SwiftSupport verification tool calls = %d, want structural rejection before code verification", toolCalls)
	}
}

func TestValidateSigningResignSwiftSupportAcceptsCanonicalLayout(t *testing.T) {
	temporary := t.TempDir()
	root := filepath.Join(temporary, "SwiftSupport", "iphoneos")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "libswiftCore.dylib"), []byte("signed runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	var verified []string
	runSigningResignToolFn = func(_ context.Context, executable string, args ...string) (signingResignToolOutput, error) {
		verified = append(verified, executable+" "+strings.Join(args, " "))
		return signingResignToolOutput{}, nil
	}
	if err := validateSigningResignSwiftSupport(context.Background(), temporary); err != nil {
		t.Fatalf("validateSigningResignSwiftSupport() error = %v", err)
	}
	if len(verified) != 1 || !strings.Contains(verified[0], "--verify") || !strings.Contains(verified[0], ".signing-resign-verify-") {
		t.Fatalf("SwiftSupport verification calls = %#v", verified)
	}
}

func TestVerifyPackedSigningResignIPARejectsTamperedSwiftSupportAfterRepack(t *testing.T) {
	info, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.app",
		"CFBundleExecutable":         "App",
		"DTPlatformName":             "iphoneos",
		"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	packedData := buildSigningResignZip(t, []signingResignZipEntry{
		{name: "Payload/App.app/Info.plist", data: info},
		{name: "Payload/App.app/App", data: []byte("macho"), mode: 0o755},
		{name: "SwiftSupport/iphoneos/libswiftCore.dylib", data: []byte("tampered runtime"), mode: 0o755},
	})
	temporary := t.TempDir()
	packedPath := filepath.Join(temporary, "packed.ipa")
	if err := os.WriteFile(packedPath, packedData, 0o600); err != nil {
		t.Fatal(err)
	}
	stageRoot, err := rootfs.New(temporary)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(_ context.Context, _ string, args ...string) (signingResignToolOutput, error) {
		if len(args) > 0 && args[0] == "--verify" {
			return signingResignToolOutput{}, errors.New("tampered SwiftSupport runtime is not Apple-signed")
		}
		return signingResignToolOutput{}, nil
	}
	fileInfo, err := os.Stat(packedPath)
	if err != nil {
		t.Fatal(err)
	}
	err = verifyPackedSigningResignIPA(context.Background(), packedPath, fileInfo.Size(), stageRoot, filepath.Join(temporary, "tree"), signingResignPreparedTree{}, "TEAM", strings.Repeat("A", 64))
	if err == nil || !strings.Contains(err.Error(), "SwiftSupport") {
		t.Fatalf("verifyPackedSigningResignIPA() error = %v, want final SwiftSupport provenance failure", err)
	}
}

func TestSigningResignToolContextHonorsCallerDeadline(t *testing.T) {
	deadline := time.Now().Add(time.Hour)
	caller, cancelCaller := context.WithDeadline(context.Background(), deadline)
	deferred, cancelDeferred := signingResignToolContext(caller, signingResignToolTimeout)
	deferredDeadline, ok := deferred.Deadline()
	if !ok || !deferredDeadline.Equal(deadline) {
		t.Fatalf("caller deadline = %v, want %v", deferredDeadline, deadline)
	}
	cancelDeferred()
	cancelCaller()

	fallback, cancelFallback := signingResignToolContext(context.Background(), signingResignToolTimeout)
	fallbackDeadline, ok := fallback.Deadline()
	if !ok || time.Until(fallbackDeadline) < 4*time.Minute {
		t.Fatalf("fallback deadline = %v, want a realistic multi-minute phase timeout", fallbackDeadline)
	}
	cancelFallback()
}

func TestValidateSigningResignOptionsUsesDeterministicRequiredOrder(t *testing.T) {
	tests := []struct {
		name    string
		options signingResignOptions
		want    string
	}{
		{name: "input", options: signingResignOptions{}, want: "IPA input"},
		{name: "output", options: signingResignOptions{IPAPath: "input.ipa"}, want: "IPA output"},
		{name: "identity", options: signingResignOptions{IPAPath: "input.ipa", OutputPath: "output.ipa"}, want: "signing identity"},
		{name: "manifest", options: signingResignOptions{IPAPath: "input.ipa", OutputPath: "output.ipa", IdentityPath: "identity.p12"}, want: "profiles manifest"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateSigningResignOptions(test.options)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateSigningResignOptions() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateSigningResignEntitlementsAgainstDocumentRequiresExactGeneratedDocument(t *testing.T) {
	existing := map[string]any{
		"application-identifier":              "OLDTEAM.com.example.app",
		"com.apple.application-identifier":    "OLDTEAM.com.example.app",
		"com.apple.developer.team-identifier": "OLDTEAM",
		"get-task-allow":                      false,
		"keychain-access-groups":              []string{"NEWTEAM.com.example.shared"},
	}
	profile := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.application-identifier":    "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
		"keychain-access-groups":              []any{"NEWTEAM.*"},
	}
	want, err := buildSigningResignEntitlements(existing, profile)
	if err != nil {
		t.Fatal(err)
	}
	documentData, err := marshalSigningResignEntitlements(want)
	if err != nil {
		t.Fatal(err)
	}
	documentPath := filepath.Join(t.TempDir(), "target-000.plist")
	if err := os.WriteFile(documentPath, documentData, 0o600); err != nil {
		t.Fatal(err)
	}
	subject := "target com.example.app signed entitlements"
	if err := validateSigningResignEntitlementsAgainstDocument(want, documentPath, subject); err != nil {
		t.Fatalf("validateSigningResignEntitlementsAgainstDocument() error = %v, want exact document accepted", err)
	}
	actual := make(map[string]any, len(want))
	for key, value := range want {
		actual[key] = value
	}
	actual["keychain-access-groups"] = []string{"NEWTEAM.other"}
	if !signingResignEntitlementValuePermits(profile["keychain-access-groups"], actual["keychain-access-groups"]) {
		t.Fatal("test setup does not exercise the profile wildcard subset case")
	}
	err = validateSigningResignEntitlementsAgainstDocument(actual, documentPath, subject)
	if err == nil || !strings.Contains(err.Error(), "exactly match the generated document") {
		t.Fatalf("validateSigningResignEntitlementsAgainstDocument() error = %v, want exact-document rejection", err)
	}
}

func TestRebaseSigningResignPreparedTreeKeepsOriginalInventoryAndDocuments(t *testing.T) {
	originalRoot := t.TempDir()
	packedRoot := t.TempDir()
	originalExecutable := filepath.Join(originalRoot, "Payload", "App.app", "Frameworks", "Feature.framework", "Feature")
	nestedEntitlements := filepath.Join(originalRoot, "entitlements", "code-000001.plist")
	targetEntitlements := filepath.Join(originalRoot, "entitlements", "target-000.plist")
	original := signingResignPreparedTree{
		Archive: signingResignArchive{
			MainPath: "Payload/App.app",
			Targets: []signingResignTarget{{
				Kind:                 "application",
				RelativePath:         "Payload/App.app",
				BundleID:             "com.example.app",
				Executable:           "App",
				ProfileMode:          0o644,
				EntitlementsPath:     targetEntitlements,
				ExistingEntitlements: map[string]any{"com.example.capability": "enabled"},
			}},
		},
		CodePlans: []signingResignCodePlan{{Path: originalExecutable, EntitlementsPath: nestedEntitlements}},
	}
	rebased, err := rebaseSigningResignPreparedTree(original, originalRoot, packedRoot)
	if err != nil {
		t.Fatalf("rebaseSigningResignPreparedTree() error = %v", err)
	}
	wantExecutable := filepath.Join(packedRoot, "Payload", "App.app", "Frameworks", "Feature.framework", "Feature")
	if got := rebased.CodePlans[0].Path; got != wantExecutable {
		t.Fatalf("rebased code path = %q, want %q", got, wantExecutable)
	}
	if got := rebased.CodePlans[0].EntitlementsPath; got != nestedEntitlements {
		t.Fatalf("rebased nested entitlements path = %q, want original %q", got, nestedEntitlements)
	}
	if got := rebased.Archive.Targets[0].EntitlementsPath; got != targetEntitlements {
		t.Fatalf("rebased target entitlements path = %q, want original %q", got, targetEntitlements)
	}
	if got := original.CodePlans[0].Path; got != originalExecutable {
		t.Fatalf("original code path mutated to %q", got)
	}
	if got := original.Archive.Targets[0].EntitlementsPath; got != targetEntitlements {
		t.Fatalf("original target entitlements path mutated to %q", got)
	}
}

func TestVerifyPackedSigningResignIPARejectsInventoryChanges(t *testing.T) {
	for _, test := range []struct {
		name           string
		executableName string
		profileData    []byte
		want           string
	}{
		{
			name:           "executable",
			executableName: "Other",
			profileData:    []byte("replacement profile"),
			want:           "Mach-O executable inventory changed",
		},
		{
			name:           "embedded profile",
			executableName: "App",
			profileData:    []byte("different replacement profile"),
			want:           "target profile changed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			baselineProfile := []byte("replacement profile")
			info, err := plist.Marshal(map[string]any{
				"CFBundleIdentifier":         "com.example.app",
				"CFBundleExecutable":         test.executableName,
				"DTPlatformName":             "iphoneos",
				"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
			}, plist.XMLFormat)
			if err != nil {
				t.Fatal(err)
			}
			executable := []byte{
				0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
				0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			}
			packedData := buildSigningResignZip(t, []signingResignZipEntry{
				{name: "Payload/App.app/Info.plist", data: info},
				{name: "Payload/App.app/" + test.executableName, data: executable, mode: 0o755},
				{name: "Payload/App.app/embedded.mobileprovision", data: test.profileData},
			})
			temporary := t.TempDir()
			packedPath := filepath.Join(temporary, "packed.ipa")
			if err := os.WriteFile(packedPath, packedData, 0o600); err != nil {
				t.Fatal(err)
			}
			stageRoot, err := rootfs.New(temporary)
			if err != nil {
				t.Fatal(err)
			}
			defer stageRoot.Close()

			original := signingResignPreparedTree{Archive: signingResignArchive{
				MainPath: "Payload/App.app",
				Targets: []signingResignTarget{{
					Kind:         "application",
					RelativePath: "Payload/App.app",
					BundleID:     "com.example.app",
					Executable:   "App",
					ProfileMode:  0o644,
					Profile: signingResignProfile{
						Data:   baselineProfile,
						SHA256: signingResignSHA256(baselineProfile),
					},
				}},
			}}
			originalTool := runSigningResignToolFn
			t.Cleanup(func() { runSigningResignToolFn = originalTool })
			runSigningResignToolFn = func(_ context.Context, _ string, _ ...string) (signingResignToolOutput, error) {
				return signingResignToolOutput{}, nil
			}
			fileInfo, err := os.Stat(packedPath)
			if err != nil {
				t.Fatal(err)
			}
			err = verifyPackedSigningResignIPA(context.Background(), packedPath, fileInfo.Size(), stageRoot, temporary, original, "TEAM", strings.Repeat("A", 64))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("verifyPackedSigningResignIPA() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateSigningResignPackedCodeInventory(t *testing.T) {
	const targetPath = "Payload/App.app/App"
	machoData := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	for _, test := range []struct {
		name       string
		setup      func(t *testing.T, packedRoot, originalRoot string)
		wantErr    bool
		wantPasses bool
	}{
		{
			name: "unchanged inventory",
			setup: func(t *testing.T, packedRoot, _ string) {
				t.Helper()
				writeSigningResignTestMachO(t, packedRoot, targetPath, machoData)
			},
			wantPasses: true,
		},
		{
			name: "extra Mach-O in existing bundle",
			setup: func(t *testing.T, packedRoot, _ string) {
				t.Helper()
				writeSigningResignTestMachO(t, packedRoot, targetPath, machoData)
				writeSigningResignTestMachO(t, packedRoot, "Payload/App.app/Extra", machoData)
			},
			wantErr: true,
		},
		{
			name: "new framework executable",
			setup: func(t *testing.T, packedRoot, _ string) {
				t.Helper()
				writeSigningResignTestMachO(t, packedRoot, targetPath, machoData)
				writeSigningResignTestMachO(t, packedRoot, "Payload/App.app/Frameworks/New.framework/New", machoData)
			},
			wantErr: true,
		},
		{
			name: "missing original nested code plan",
			setup: func(t *testing.T, packedRoot, _ string) {
				t.Helper()
				writeSigningResignTestMachO(t, packedRoot, targetPath, machoData)
			},
			wantErr: true,
		},
		{
			name: "SwiftSupport is separately excluded",
			setup: func(t *testing.T, packedRoot, _ string) {
				t.Helper()
				writeSigningResignTestMachO(t, packedRoot, targetPath, machoData)
				writeSigningResignTestMachO(t, packedRoot, "SwiftSupport/iphoneos/libswiftCore.dylib", machoData)
			},
			wantPasses: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			temporary := t.TempDir()
			packedRoot := filepath.Join(temporary, "packed")
			originalRoot := filepath.Join(temporary, "original")
			if err := os.MkdirAll(packedRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(originalRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			test.setup(t, packedRoot, originalRoot)
			original := signingResignPreparedTree{Archive: signingResignArchive{
				Targets: []signingResignTarget{{RelativePath: "Payload/App.app", Executable: "App"}},
			}}
			if test.name == "missing original nested code plan" {
				original.CodePlans = []signingResignCodePlan{{Path: filepath.Join(originalRoot, "Payload/App.app/Frameworks/Feature.framework/Feature")}}
			}
			err := validateSigningResignPackedCodeInventory(context.Background(), packedRoot, originalRoot, original)
			if (err != nil) != test.wantErr || (err == nil) != test.wantPasses {
				t.Fatalf("validateSigningResignPackedCodeInventory() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestEnumerateSigningResignMachOFilesRootStaysOnOpenedTree(t *testing.T) {
	parent := t.TempDir()
	treePath := filepath.Join(parent, "packed-tree")
	originalPath := filepath.Join(parent, "original-tree")
	if err := os.Mkdir(treePath, 0o700); err != nil {
		t.Fatal(err)
	}
	machoData := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	writeSigningResignTestMachO(t, treePath, "Payload/App.app/App", machoData)
	opened, err := os.OpenRoot(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if err := os.Rename(treePath, originalPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(treePath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeSigningResignTestMachO(t, treePath, "Payload/Evil.app/Evil", machoData)

	paths, err := enumerateSigningResignMachOFilesRoot(context.Background(), opened)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(paths, []string{"Payload/App.app/App"}) {
		t.Fatalf("enumerateSigningResignMachOFilesRoot() = %v, want original opened tree", paths)
	}
}

func TestVerifySigningResignTreeRejectsRootReplacementDuringCodesign(t *testing.T) {
	parent := t.TempDir()
	treePath := filepath.Join(parent, "packed-tree")
	movedPath := filepath.Join(parent, "original-tree")
	if err := os.MkdirAll(filepath.Join(treePath, "Payload", "App.app"), 0o700); err != nil {
		t.Fatal(err)
	}
	tree, err := rootfs.New(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()

	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(_ context.Context, _ string, _ ...string) (signingResignToolOutput, error) {
		if err := os.Rename(treePath, movedPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(treePath, 0o700); err != nil {
			t.Fatal(err)
		}
		return signingResignToolOutput{}, nil
	}

	err = verifySigningResignTree(context.Background(), tree, signingResignPreparedTree{
		Archive: signingResignArchive{MainPath: "Payload/App.app"},
	}, "TEAM", "")
	if err == nil {
		t.Fatal("verifySigningResignTree() accepted a replaced verification root")
	}
	var operational *signingResignOperationalError
	if !errors.As(err, &operational) || operational.stage != signingResignStageVerification || operational.code != signingResignCodeVerification {
		t.Fatalf("verifySigningResignTree() error = %v, want closed verification failure", err)
	}
}

func writeSigningResignTestMachO(t *testing.T, root, relative string, data []byte) {
	t.Helper()
	pathValue := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(pathValue), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathValue, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyPackedSigningResignIPARejectsDroppedEntitlement(t *testing.T) {
	profileData := []byte("replacement profile")
	profileDigest := signingResignSHA256(profileData)
	info, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.app",
		"CFBundleExecutable":         "App",
		"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	executable := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	packedData := buildSigningResignZip(t, []signingResignZipEntry{
		{name: "Payload/App.app/Info.plist", data: info},
		{name: "Payload/App.app/App", data: executable, mode: 0o755},
		{name: "Payload/App.app/embedded.mobileprovision", data: profileData},
	})
	stagePath := t.TempDir()
	if err := os.Mkdir(filepath.Join(stagePath, "tree"), 0o700); err != nil {
		t.Fatal(err)
	}
	packedPath := filepath.Join(stagePath, "packed.ipa")
	if err := os.WriteFile(packedPath, packedData, 0o600); err != nil {
		t.Fatal(err)
	}
	stageRoot, err := rootfs.New(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()

	original := signingResignPreparedTree{
		Archive: signingResignArchive{
			MainPath: "Payload/App.app",
			Targets: []signingResignTarget{{
				Kind:         "application",
				RelativePath: "Payload/App.app",
				BundleID:     "com.example.app",
				Executable:   "App",
				ProfileMode:  0o644,
				ExistingEntitlements: map[string]any{
					"application-identifier":              "TEAM.com.example.app",
					"com.apple.application-identifier":    "TEAM.com.example.app",
					"com.apple.developer.team-identifier": "TEAM",
					"get-task-allow":                      false,
					"com.example.capability":              "enabled",
				},
				Profile: signingResignProfile{
					Data:   profileData,
					SHA256: profileDigest,
					Entitlements: map[string]any{
						"application-identifier":              "TEAM.com.example.app",
						"com.apple.application-identifier":    "TEAM.com.example.app",
						"com.apple.developer.team-identifier": "TEAM",
						"get-task-allow":                      false,
						"com.example.capability":              "enabled",
					},
				},
			}},
		},
	}
	expectedEntitlements, err := plist.Marshal(original.Archive.Targets[0].ExistingEntitlements, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := stageRoot.WriteFile("expected-entitlements.plist", expectedEntitlements, 0o600); err != nil {
		t.Fatal(err)
	}
	original.Archive.Targets[0].EntitlementsPath = filepath.Join(stagePath, "expected-entitlements.plist")

	droppedEntitlements, err := plist.Marshal(map[string]any{
		"application-identifier":              "TEAM.com.example.app",
		"com.apple.application-identifier":    "TEAM.com.example.app",
		"com.apple.developer.team-identifier": "TEAM",
		"get-task-allow":                      false,
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	originalTool := runSigningResignToolFn
	originalCertificate := extractSigningResignCertificateFn
	t.Cleanup(func() {
		runSigningResignToolFn = originalTool
		extractSigningResignCertificateFn = originalCertificate
	})
	runSigningResignToolFn = func(_ context.Context, _ string, args ...string) (signingResignToolOutput, error) {
		if len(args) > 0 && args[0] == "-d" {
			return signingResignToolOutput{Stdout: droppedEntitlements}, nil
		}
		return signingResignToolOutput{}, nil
	}
	extractSigningResignCertificateFn = func(context.Context, string, string) error { return nil }

	fileInfo, err := os.Stat(packedPath)
	if err != nil {
		t.Fatal(err)
	}
	err = verifyPackedSigningResignIPA(context.Background(), packedPath, fileInfo.Size(), stageRoot, filepath.Join(stagePath, "tree"), original, "TEAM", strings.Repeat("A", 64))
	if err == nil {
		t.Fatal("verifyPackedSigningResignIPA() returned nil for dropped entitlement")
	}
	var operational *signingResignOperationalError
	if !errors.As(err, &operational) || operational.stage != signingResignStageVerification || operational.code != signingResignCodeVerification {
		t.Fatalf("verifyPackedSigningResignIPA() error = %v, want closed verification failure", err)
	}
}

type signingResignZipEntry struct {
	name      string
	data      []byte
	mode      os.FileMode
	modeSet   bool
	directory bool
}

func buildSigningResignZip(t *testing.T, entries []signingResignZipEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, item := range entries {
		if item.directory {
			header := &zip.FileHeader{Name: item.name, Method: zip.Store}
			mode := item.mode
			if mode == 0 && !item.modeSet {
				mode = 0o755
			}
			header.SetMode(os.ModeDir | mode)
			if _, err := writer.CreateHeader(header); err != nil {
				t.Fatal(err)
			}
			continue
		}
		mode := item.mode
		if mode != 0 || item.modeSet {
			header := &zip.FileHeader{Name: item.name, Method: zip.Deflate}
			header.SetMode(mode)
			file, err := writer.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.Write(item.data); err != nil {
				t.Fatal(err)
			}
			continue
		}
		file, err := writer.Create(item.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(item.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestMaterializeSigningResignArchivePreservesSafeFileModesThroughRepack(t *testing.T) {
	data := buildSigningResignZip(t, []signingResignZipEntry{
		{name: "Payload/App.app/Resources/data.txt", data: []byte("data"), mode: 0o644},
		{name: "Payload/App.app/Resources/tool", data: []byte("tool"), mode: 0o755},
		{name: "Payload/App.app/Resources/private.txt", data: []byte("private"), mode: 0o640},
		{name: "Payload/App.app/Resources/private-tool", data: []byte("private tool"), mode: 0o750},
	})
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSigningResignArchive(context.Background(), reader); err != nil {
		t.Fatal(err)
	}
	stagePath := t.TempDir()
	treePath := filepath.Join(stagePath, "tree")
	if err := os.Mkdir(treePath, 0o700); err != nil {
		t.Fatal(err)
	}
	stageRoot, err := rootfs.New(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()
	stageOS, err := stageRoot.OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer stageOS.Close()
	treeOS, err := stageOS.OpenRoot("tree")
	if err != nil {
		t.Fatal(err)
	}
	defer treeOS.Close()
	if err := materializeSigningResignArchive(context.Background(), reader, treeOS); err != nil {
		t.Fatal(err)
	}
	treeRoot, err := rootfs.New(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer treeRoot.Close()
	for _, test := range []struct {
		name string
		mode os.FileMode
	}{
		{name: "Payload/App.app/Resources/data.txt", mode: 0o644},
		{name: "Payload/App.app/Resources/tool", mode: 0o755},
		{name: "Payload/App.app/Resources/private.txt", mode: 0o640},
		{name: "Payload/App.app/Resources/private-tool", mode: 0o750},
	} {
		file, err := treeRoot.OpenFile(test.name)
		if err != nil {
			t.Fatal(err)
		}
		info, err := file.Stat()
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != test.mode {
			t.Fatalf("materialized %s mode = %#o, want %#o", test.name, info.Mode().Perm(), test.mode)
		}
	}
	packedPath, _, _, err := repackSigningResignTree(context.Background(), stageRoot, treeRoot)
	if err != nil {
		t.Fatal(err)
	}
	packed, err := os.Open(packedPath)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()
	packedInfo, err := packed.Stat()
	if err != nil {
		t.Fatal(err)
	}
	packedReader, err := zip.NewReader(packed, packedInfo.Size())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		mode os.FileMode
	}{
		{name: "Payload/App.app/Resources/data.txt", mode: 0o644},
		{name: "Payload/App.app/Resources/tool", mode: 0o755},
		{name: "Payload/App.app/Resources/private.txt", mode: 0o640},
		{name: "Payload/App.app/Resources/private-tool", mode: 0o750},
	} {
		var found *zip.File
		for _, member := range packedReader.File {
			if member.Name == test.name {
				found = member
				break
			}
		}
		if found == nil {
			t.Fatalf("packed archive is missing %s", test.name)
		}
		if found.Mode().Perm() != test.mode {
			t.Fatalf("packed %s mode = %#o, want %#o", test.name, found.Mode().Perm(), test.mode)
		}
	}
}

func TestValidateSigningResignArchiveRejectsExplicitWorldWritableMode(t *testing.T) {
	data := buildSigningResignZip(t, []signingResignZipEntry{
		{name: "Payload/App.app/Resources/data.txt", data: []byte("data"), mode: 0o666},
	})
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSigningResignArchive(context.Background(), reader); err == nil || !strings.Contains(err.Error(), "permission mode") {
		t.Fatalf("validateSigningResignArchive() error = %v, want unsafe permission rejection", err)
	}
}

func TestValidateSigningResignArchiveRejectsUnreadableExplicitModes(t *testing.T) {
	for _, test := range []struct {
		name  string
		entry signingResignZipEntry
		want  string
	}{
		{
			name:  "file mode zero",
			entry: signingResignZipEntry{name: "Payload/App.app/Resources/data.txt", data: []byte("data"), modeSet: true},
			want:  "unreadable archive file mode",
		},
		{
			name:  "directory mode zero",
			entry: signingResignZipEntry{name: "Payload/App.app/Resources/", modeSet: true, directory: true},
			want:  "unreadable or untraversable directory mode",
		},
		{
			name:  "setuid file",
			entry: signingResignZipEntry{name: "Payload/App.app/Resources/tool", data: []byte("tool"), mode: os.ModeSetuid | 0o644},
			want:  "special mode",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := buildSigningResignZip(t, []signingResignZipEntry{test.entry})
			reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			if err := validateSigningResignArchive(context.Background(), reader); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateSigningResignArchive() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestMaterializeSigningResignArchiveDefaultsDOSModeOnly(t *testing.T) {
	data := buildSigningResignZip(t, []signingResignZipEntry{
		{name: "Payload/App.app/Resources/data.txt", data: []byte("data")},
	})
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if reader.File[0].CreatorVersion>>8 != 0 {
		t.Fatalf("test ZIP unexpectedly declares a Unix creator: %#x", reader.File[0].CreatorVersion)
	}
	stagePath := t.TempDir()
	treePath := filepath.Join(stagePath, "tree")
	if err := os.Mkdir(treePath, 0o700); err != nil {
		t.Fatal(err)
	}
	stageRoot, err := rootfs.New(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()
	stageOS, err := stageRoot.OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer stageOS.Close()
	treeOS, err := stageOS.OpenRoot("tree")
	if err != nil {
		t.Fatal(err)
	}
	defer treeOS.Close()
	if err := materializeSigningResignArchive(context.Background(), reader, treeOS); err != nil {
		t.Fatal(err)
	}
	treeRoot, err := rootfs.New(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer treeRoot.Close()
	file, err := treeRoot.OpenFile("Payload/App.app/Resources/data.txt")
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("DOS-mode materialized permission = %#o, want 0644", got)
	}
}

func TestRepackSigningResignTreeIsDeterministicAndBounded(t *testing.T) {
	stagePath := t.TempDir()
	treePath := filepath.Join(stagePath, "tree")
	if err := os.Mkdir(treePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(treePath, "b.txt"), []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(treePath, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	stageRoot, err := rootfs.New(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()
	treeRoot, err := rootfs.New(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer treeRoot.Close()
	packedPath, size, digest, err := repackSigningResignTree(context.Background(), stageRoot, treeRoot)
	if err != nil {
		t.Fatal(err)
	}
	if size <= 0 || digest != signingResignSHA256(mustReadFile(t, packedPath)) {
		t.Fatalf("packed artifact size=%d digest=%q", size, digest)
	}
	if _, err := os.Stat(packedPath); err != nil {
		t.Fatal(err)
	}
}

func TestSigningResignRepackEntryLimitError(t *testing.T) {
	if err := signingResignRepackEntryLimitError(signingResignMaxArchiveEntries); err != nil {
		t.Fatalf("signingResignRepackEntryLimitError(%d) = %v, want accepted", signingResignMaxArchiveEntries, err)
	}
	if err := signingResignRepackEntryLimitError(signingResignMaxArchiveEntries + 1); err == nil || !strings.Contains(err.Error(), "archive entry limit") {
		t.Fatalf("signingResignRepackEntryLimitError(%d) = %v, want limit rejection", signingResignMaxArchiveEntries+1, err)
	}
}

func TestPublishSigningResignOutputReportsAmbiguousPublication(t *testing.T) {
	contents := []byte("packed IPA")
	for _, test := range []struct {
		name       string
		packedSize int64
		digest     string
	}{
		{name: "size mismatch", packedSize: int64(len(contents)) + 1, digest: signingResignSHA256(contents)},
		{name: "digest mismatch", packedSize: int64(len(contents)), digest: strings.Repeat("0", 64)},
	} {
		t.Run(test.name, func(t *testing.T) {
			stagePath := t.TempDir()
			packedPath := filepath.Join(stagePath, "packed.ipa")
			if err := os.WriteFile(packedPath, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			outputPath := filepath.Join(t.TempDir(), "output.ipa")
			outputRoot, err := rootfs.New(filepath.Dir(outputPath))
			if err != nil {
				t.Fatal(err)
			}
			defer outputRoot.Close()
			_, err = publishSigningResignOutput(context.Background(), outputRoot, filepath.Base(outputPath), packedPath, test.packedSize, test.digest)
			if err == nil || !errors.Is(err, ErrSigningResignPublicationAmbiguous) {
				t.Fatalf("publishSigningResignOutput() error = %v, want ambiguous publication", err)
			}
			published, readErr := os.ReadFile(outputPath)
			if readErr != nil {
				t.Fatalf("read ambiguous published artifact: %v", readErr)
			}
			if !bytes.Equal(published, contents) {
				t.Fatalf("ambiguous published artifact = %q, want %q", published, contents)
			}
		})
	}
}

func TestSigningResignHashHonorsCancellation(t *testing.T) {
	contents := []byte("large enough staged artifact")
	pathValue := filepath.Join(t.TempDir(), "packed.ipa")
	if err := os.WriteFile(pathValue, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := hashSigningResignFile(ctx, pathValue, int64(len(contents))); !errors.Is(err, context.Canceled) {
		t.Fatalf("hashSigningResignFile() error = %v, want cancellation", err)
	}
}

func TestPublishSigningResignOutputCancellationAfterPublicationIsAmbiguous(t *testing.T) {
	contents := []byte("packed IPA")
	stagePath := t.TempDir()
	packedPath := filepath.Join(stagePath, "packed.ipa")
	if err := os.WriteFile(packedPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(t.TempDir(), "output.ipa")
	outputRoot, err := rootfs.New(filepath.Dir(outputPath))
	if err != nil {
		t.Fatal(err)
	}
	defer outputRoot.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	originalHook := signingResignBeforePublishedHashFn
	t.Cleanup(func() { signingResignBeforePublishedHashFn = originalHook })
	signingResignBeforePublishedHashFn = cancel
	artifact, err := publishSigningResignOutput(ctx, outputRoot, filepath.Base(outputPath), packedPath, int64(len(contents)), signingResignSHA256(contents))
	if err == nil || !errors.Is(err, ErrSigningResignPublicationAmbiguous) {
		t.Fatalf("publishSigningResignOutput() error = %v, want ambiguous cancellation", err)
	}
	if artifact.Path != "" {
		t.Fatalf("ambiguous publication returned a success artifact: %#v", artifact)
	}
	if published, readErr := os.ReadFile(outputPath); readErr != nil || !bytes.Equal(published, contents) {
		t.Fatalf("published artifact after cancellation = %q, read error = %v", published, readErr)
	}
}

func TestPublishSigningResignOutputCancellationRetainsContextCause(t *testing.T) {
	contents := []byte("packed IPA")
	stagePath := t.TempDir()
	packedPath := filepath.Join(stagePath, "packed.ipa")
	if err := os.WriteFile(packedPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(t.TempDir(), "output.ipa")
	outputRoot, err := rootfs.New(filepath.Dir(outputPath))
	if err != nil {
		t.Fatal(err)
	}
	defer outputRoot.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	originalHook := signingResignBeforePublishedHashFn
	t.Cleanup(func() { signingResignBeforePublishedHashFn = originalHook })
	signingResignBeforePublishedHashFn = cancel
	_, err = publishSigningResignOutput(ctx, outputRoot, filepath.Base(outputPath), packedPath, int64(len(contents)), signingResignSHA256(contents))
	if err == nil || !errors.Is(err, ErrSigningResignPublicationAmbiguous) {
		t.Fatalf("publishSigningResignOutput() error = %v, want ambiguous publication", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("publishSigningResignOutput() error = %v, want the cancellation cause retained", err)
	}
}

func TestSigningResignVerificationEntitlementReadUsesVerificationStage(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-entitlements.plist")
	err := validateSigningResignEntitlementsAgainstDocumentAtStage(
		map[string]any{},
		missing,
		"verified target entitlements",
		signingResignStageVerification,
	)
	if err == nil {
		t.Fatal("validateSigningResignEntitlementsAgainstDocumentAtStage() error = nil, want missing document failure")
	}
	var operation *signingResignOperationalError
	if !errors.As(err, &operation) {
		t.Fatalf("error = %v, want typed verification error", err)
	}
	if operation.stage != signingResignStageVerification || operation.code != signingResignCodeGeneratedEntitlements {
		t.Fatalf("error = %v, want verification/generated-entitlements classification", err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRunSigningResignEnvironmentNeverInstallsProfiles(t *testing.T) {
	original := signingResignPlatformDepsFn
	t.Cleanup(func() { signingResignPlatformDepsFn = original })
	temporary := t.TempDir()
	var events []string
	signingResignPlatformDepsFn = func() signingRunDeps {
		return signingRunDeps{
			GOOS:        "darwin",
			RandomBytes: func(size int) ([]byte, error) { return bytes.Repeat([]byte{0x42}, size), nil },
			TempDir: func() (string, error) {
				return filepath.Join(temporary, "session"), os.Mkdir(filepath.Join(temporary, "session"), 0o700)
			},
			RemoveTempDir: func(path string) error { events = append(events, "remove-temp"); return os.RemoveAll(path) },
			AcquireLock: func(context.Context) (func() error, error) {
				events = append(events, "lock")
				return func() error { events = append(events, "unlock"); return nil }, nil
			},
			Recover:       func(context.Context) error { events = append(events, "recover"); return nil },
			WriteJournal:  func(signingRunJournal, bool) error { events = append(events, "journal-write"); return nil },
			RemoveJournal: func() error { events = append(events, "journal-remove"); return nil },
			KeychainSearchList: func(context.Context) ([]string, error) {
				events = append(events, "list")
				return []string{"login.keychain-db"}, nil
			},
			CreateKeychain: func(context.Context, string, []byte) error { events = append(events, "create-keychain"); return nil },
			ImportIdentity: func(context.Context, string, []byte, []byte, []byte, string) error {
				events = append(events, "import")
				return nil
			},
			SetKeychainSearchList:     func(context.Context, []string) error { events = append(events, "activate"); return nil },
			RemoveKeychainSearchEntry: func(context.Context, string) error { events = append(events, "remove-search"); return nil },
			DeleteKeychain:            func(context.Context, string) error { events = append(events, "delete-keychain"); return nil },
		}
	}
	identity := testSigningResignIdentity(t)
	if err := runSigningResignEnvironment(context.Background(), identity, func(_ context.Context, keychainPath string) error {
		if keychainPath == "" {
			t.Fatal("callback received an empty keychain path")
		}
		events = append(events, "operation")
		return nil
	}); err != nil {
		t.Fatalf("runSigningResignEnvironment() error = %v", err)
	}
	joined := strings.Join(events, ",")
	if strings.Contains(joined, "install") || strings.Contains(joined, "profile") {
		t.Fatalf("environment unexpectedly touched a profile: %s", joined)
	}
	if !strings.Contains(joined, "create-keychain,remove-search,import") || !strings.Contains(joined, "activate,operation,remove-search,delete-keychain") {
		t.Fatalf("environment events = %s", joined)
	}
}

func testSigningResignIdentity(t *testing.T) *signingRunIdentity {
	t.Helper()
	key, err := rsa.GenerateKey(cryptorand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test", OrganizationalUnit: []string{"TEAM"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(cryptorand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &signingRunIdentity{Certificate: certificate, PrivateKey: key, CertificateSHA1: strings.Repeat("A", 40), CertificateSHA256: strings.Repeat("B", 64)}
}

func TestDiscoverSigningResignArchiveRejectsNonLocalEntriesWithoutPriorValidation(t *testing.T) {
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)
	for _, name := range []string{"Payload/App.app/Info.plist", "../escape"} {
		if _, err := writer.Create(name); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := rootfs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()
	_, err = discoverSigningResignArchive(context.Background(), reader, tree)
	if err == nil || !strings.Contains(err.Error(), "non-local archive path") {
		t.Fatalf("discoverSigningResignArchive() error = %v, want non-local entry rejection", err)
	}
}

func TestBuildSigningResignEntitlementsPreservesConcreteIdentitySubsets(t *testing.T) {
	existing := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
		"keychain-access-groups":              []string{"NEWTEAM.com.example.app"},
	}
	profile := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.application-identifier":    "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
		"keychain-access-groups":              []any{"NEWTEAM.com.example.app", "NEWTEAM.com.example.shared"},
	}
	got, err := buildSigningResignEntitlements(existing, profile)
	if err != nil {
		t.Fatalf("buildSigningResignEntitlements() error = %v", err)
	}
	if !signingResignEntitlementValuesEqual(got["keychain-access-groups"], existing["keychain-access-groups"]) {
		t.Fatalf(
			"keychain-access-groups = %#v, want the app's existing subset %#v without profile-widened groups",
			got["keychain-access-groups"], existing["keychain-access-groups"],
		)
	}
}

func TestBuildSigningResignEntitlementsOmitsOptionalWildcardOnlyProfileClaims(t *testing.T) {
	existing := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
	}
	profile := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.application-identifier":    "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
		"keychain-access-groups":              []any{"NEWTEAM.*"},
	}
	got, err := buildSigningResignEntitlements(existing, profile)
	if err != nil {
		t.Fatalf("buildSigningResignEntitlements() error = %v, want wildcard-only optional claim omitted", err)
	}
	if value, exists := got["keychain-access-groups"]; exists {
		t.Fatalf("keychain-access-groups = %#v, want the unclaimed optional capability omitted", value)
	}
	wildcardProfile := map[string]any{
		"application-identifier":              "NEWTEAM.*",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
	}
	if _, err := buildSigningResignEntitlements(map[string]any{}, wildcardProfile); err == nil {
		t.Fatal("buildSigningResignEntitlements() accepted a wildcard-only required identity claim with no existing concrete value")
	}
}

func TestInspectSigningResignTargetRequiresOwnerExecutableBinary(t *testing.T) {
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(context.Context, string, ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{}, nil
	}
	treePath := t.TempDir()
	appDir := filepath.Join(treePath, "Payload", "App.app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	info, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.app",
		"CFBundleExecutable":         "App",
		"DTPlatformName":             "iphoneos",
		"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "Info.plist"), info, 0o644); err != nil {
		t.Fatal(err)
	}
	executable := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	if err := os.WriteFile(filepath.Join(appDir, "App"), executable, 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := rootfs.New(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()
	if _, err := inspectSigningResignTarget(context.Background(), tree, "Payload/App.app", "application"); err == nil || !strings.Contains(err.Error(), "owner-execute") {
		t.Fatalf("inspectSigningResignTarget() error = %v, want owner-execute rejection for a 0644 executable", err)
	}
	if err := os.Chmod(filepath.Join(appDir, "App"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectSigningResignTarget(context.Background(), tree, "Payload/App.app", "application"); err != nil {
		t.Fatalf("inspectSigningResignTarget() error = %v, want executable target accepted", err)
	}
}

func TestSigningResignAppExecutableClassificationRequiresMHExecute(t *testing.T) {
	base := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	for _, test := range []struct {
		name     string
		fileType []byte
		want     bool
	}{
		{name: "MH_EXECUTE", fileType: []byte{2, 0, 0, 0}, want: true},
		{name: "MH_DYLIB", fileType: []byte{6, 0, 0, 0}},
		{name: "MH_BUNDLE", fileType: []byte{8, 0, 0, 0}},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := append([]byte(nil), base...)
			copy(data[12:16], test.fileType)
			path := filepath.Join(t.TempDir(), "executable")
			if err := os.WriteFile(path, data, 0o755); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if got := isSigningResignAppExecutableFile(file, info.Size()); got != test.want {
				t.Fatalf("isSigningResignAppExecutableFile() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestPrepareSigningResignTreeRequiresOwnerExecutableNestedCode(t *testing.T) {
	treePath := t.TempDir()
	appPath := filepath.Join(treePath, "Payload", "App.app")
	nestedPath := filepath.Join(appPath, "PlugIns", "Service.xpc", "Service")
	if err := os.MkdirAll(filepath.Dir(nestedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	machoData := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	if err := os.WriteFile(filepath.Join(appPath, "App"), machoData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nestedPath, machoData, 0o644); err != nil {
		t.Fatal(err)
	}
	stagePath := t.TempDir()
	stageRoot, err := rootfs.New(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()
	treeRoot, err := rootfs.New(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer treeRoot.Close()
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(context.Context, string, ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{}, nil
	}
	profile := signingResignProfile{
		Data: []byte("profile"),
		Entitlements: map[string]any{
			"application-identifier":              "TEAM.com.example.app",
			"com.apple.application-identifier":    "TEAM.com.example.app",
			"com.apple.developer.team-identifier": "TEAM",
			"get-task-allow":                      false,
		},
	}
	_, err = prepareSigningResignTree(context.Background(), stageRoot, treeRoot, signingResignArchive{
		MainPath: "Payload/App.app",
		Targets: []signingResignTarget{{
			Kind: "application", RelativePath: "Payload/App.app", BundleID: "com.example.app", Executable: "App",
		}},
	}, map[string]signingResignProfile{"com.example.app": profile})
	if err == nil || !strings.Contains(err.Error(), "owner-execute") {
		t.Fatalf("prepareSigningResignTree() error = %v, want nested owner-execute rejection", err)
	}
}

func TestSigningResignArchiveMemberModeUsesNonExecutableDOSDefault(t *testing.T) {
	machoData := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	data := buildSigningResignZip(t, []signingResignZipEntry{{
		name: "Payload/App.app/PlugIns/Service.xpc/Service", data: machoData,
	}})
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 1 || reader.File[0].CreatorVersion>>8 != 0 {
		t.Fatalf("test ZIP unexpectedly carries Unix mode metadata: %#v", reader.File)
	}
	mode, err := signingResignArchiveMemberMode(reader.File[0])
	if err != nil {
		t.Fatalf("signingResignArchiveMemberMode() error = %v", err)
	}
	if mode != 0o644 {
		t.Fatalf("DOS-created nested executable default mode = %#o, want %#o", mode, 0o644)
	}
}

func TestSigningResignSafeFileModeRequiresOwnerWritableDirectories(t *testing.T) {
	if _, err := signingResignSafeFileMode(0o555, true); err == nil || !strings.Contains(err.Error(), "owner-writable") {
		t.Fatalf("signingResignSafeFileMode(0o555, dir) error = %v, want owner-writable rejection", err)
	}
	mode, err := signingResignSafeFileMode(0o755, true)
	if err != nil || mode != 0o755 {
		t.Fatalf("signingResignSafeFileMode(0o755, dir) = %#o, %v, want accepted", mode, err)
	}
}

func TestRepackSigningResignTreePreservesDirectoryEntries(t *testing.T) {
	stagePath := t.TempDir()
	treePath := filepath.Join(stagePath, "tree")
	emptyDir := filepath.Join(treePath, "Payload", "App.app", "Empty.lproj")
	if err := os.MkdirAll(emptyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(emptyDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(treePath, "Payload", "App.app", "Info.plist"), []byte("plist"), 0o644); err != nil {
		t.Fatal(err)
	}
	stageRoot, err := rootfs.New(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()
	treeRoot, err := rootfs.New(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer treeRoot.Close()
	packedPath, size, _, err := repackSigningResignTree(context.Background(), stageRoot, treeRoot)
	if err != nil {
		t.Fatal(err)
	}
	packed, err := os.Open(packedPath)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()
	reader, err := zip.NewReader(packed, size)
	if err != nil {
		t.Fatal(err)
	}
	var emptyEntry *zip.File
	for _, member := range reader.File {
		if member.Name == "Payload/App.app/Empty.lproj/" {
			emptyEntry = member
		}
	}
	if emptyEntry == nil {
		t.Fatalf("re-signed IPA dropped the empty directory entry; members = %v", memberNamesForTest(reader))
	}
	if !emptyEntry.FileInfo().IsDir() || emptyEntry.Mode().Perm() != 0o750 {
		t.Fatalf("empty directory entry mode = %v, want preserved directory mode 0o750", emptyEntry.Mode())
	}
}

func memberNamesForTest(reader *zip.Reader) []string {
	names := make([]string, 0, len(reader.File))
	for _, member := range reader.File {
		names = append(names, member.Name)
	}
	return names
}

func TestBuildSigningResignEntitlementsOmitsAbsentOptionalConcreteClaims(t *testing.T) {
	existing := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
	}
	profile := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.application-identifier":    "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
		"keychain-access-groups":              []any{"NEWTEAM.com.example.shared"},
	}
	got, err := buildSigningResignEntitlements(existing, profile)
	if err != nil {
		t.Fatalf("buildSigningResignEntitlements() error = %v", err)
	}
	if value, exists := got["keychain-access-groups"]; exists {
		t.Fatalf("keychain-access-groups = %#v, want the unclaimed optional capability omitted even for concrete profile values", value)
	}
	if value, exists := got["com.apple.application-identifier"]; exists {
		t.Fatalf("com.apple.application-identifier = %#v, want the unclaimed alternate identifier omitted", value)
	}
	if got["get-task-allow"] != false {
		t.Fatalf("get-task-allow = %#v, want the required claim adopted from the profile", got["get-task-allow"])
	}
}

func TestBuildSigningResignEntitlementsListsEveryUnauthorizedCrossTeamClaim(t *testing.T) {
	existing := map[string]any{
		"application-identifier":                          "OLDTEAM.com.example.app",
		"com.apple.developer.team-identifier":             "OLDTEAM",
		"get-task-allow":                                  false,
		"keychain-access-groups":                          []string{"OLDTEAM.com.example.shared"},
		"com.apple.developer.ubiquity-kvstore-identifier": "OLDTEAM.com.example.app",
	}
	profile := map[string]any{
		"application-identifier":                          "NEWTEAM.com.example.app",
		"com.apple.application-identifier":                "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier":             "NEWTEAM",
		"get-task-allow":                                  false,
		"keychain-access-groups":                          []any{"NEWTEAM.*"},
		"com.apple.developer.ubiquity-kvstore-identifier": "NEWTEAM.*",
	}
	_, err := buildSigningResignEntitlements(existing, profile)
	if err == nil {
		t.Fatal("buildSigningResignEntitlements() accepted cross-team claims, want fail-closed refusal")
	}
	message := err.Error()
	for _, want := range []string{
		"keychain-access-groups",
		`"OLDTEAM.com.example.shared"`,
		`"NEWTEAM.com.example.shared"`,
		"com.apple.developer.ubiquity-kvstore-identifier",
		`"OLDTEAM.com.example.app"`,
		`"NEWTEAM.com.example.app"`,
		"then re-run",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("refusal %q is missing %q", message, want)
		}
	}
	if got := strings.Count(message, "then re-run"); got != 2 {
		t.Fatalf("refusal %q lists %d remediations, want one per blocked claim", message, got)
	}
}

func TestSigningResignUnauthorizedClaimsEscapeAndBoundDetails(t *testing.T) {
	rawKey := "unauthorized\n\x1b[31mclaim"
	rawValue := strings.Repeat("\x1b", 512)
	claims := make([]signingResignUnauthorizedClaim, 32)
	for index := range claims {
		claims[index] = signingResignUnauthorizedClaim{
			Key:      rawKey,
			Existing: rawValue,
			Profile:  "authorized",
		}
	}
	err := signingResignUnauthorizedClaimsError(claims)
	message := err.Error()
	if strings.ContainsAny(message, "\r\n\x1b") {
		t.Fatalf("unauthorized-claim detail contains raw control characters: %q", message)
	}
	if strings.Contains(message, rawKey) || strings.Contains(message, rawValue) {
		t.Fatal("unauthorized-claim detail exposed unescaped or unbounded input")
	}
	if len(message) > signingResignPublicDetailMaxBytes {
		t.Fatalf("unauthorized-claim detail length = %d, want bounded output", len(message))
	}
	suggestion, ok := signingResignClaimRebaseSuggestion("OLDTEAM."+rawValue, "NEWTEAM.*")
	if !ok {
		t.Fatal("signingResignClaimRebaseSuggestion() did not derive a wildcard remediation")
	}
	if strings.ContainsAny(suggestion, "\r\n\x1b") || len(suggestion) > 160 {
		t.Fatalf("wildcard remediation = %q, want escaped bounded output", suggestion)
	}
}

func TestDecodeSigningResignManifestRejectsStandaloneCaseVariantFields(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
	}{
		{name: "top-level SchemaVersion", data: `{"SchemaVersion":1,"profiles":[{"bundleId":"com.example.app","profilePath":"p/app.mobileprovision"}]}`},
		{name: "top-level Profiles", data: `{"schemaVersion":1,"Profiles":[{"bundleId":"com.example.app","profilePath":"p/app.mobileprovision"}]}`},
		{name: "entry BundleID", data: `{"schemaVersion":1,"profiles":[{"BundleID":"com.example.app","profilePath":"p/app.mobileprovision"}]}`},
		{name: "entry ProfilePath", data: `{"schemaVersion":1,"profiles":[{"bundleId":"com.example.app","ProfilePath":"p/app.mobileprovision"}]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeSigningResignManifest([]byte(test.data)); err == nil {
				t.Fatal("decodeSigningResignManifest() accepted a case-variant schema field")
			}
		})
	}
	valid := `{"schemaVersion":1,"profiles":[{"bundleId":"com.example.app","profilePath":"p/app.mobileprovision"}]}`
	if _, err := decodeSigningResignManifest([]byte(valid)); err != nil {
		t.Fatalf("decodeSigningResignManifest() error = %v, want exact schema accepted", err)
	}
}

func TestBuildSigningResignEntitlementsDerivesProfileRequiredDistributionClaims(t *testing.T) {
	existing := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
	}
	profile := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
		"beta-reports-active":                 true,
	}
	got, err := buildSigningResignEntitlements(existing, profile)
	if err != nil {
		t.Fatalf("buildSigningResignEntitlements() error = %v", err)
	}
	if got["beta-reports-active"] != true {
		t.Fatalf("beta-reports-active = %#v, want the profile-required distribution claim derived", got["beta-reports-active"])
	}

	delete(profile, "beta-reports-active")
	got, err = buildSigningResignEntitlements(existing, profile)
	if err != nil {
		t.Fatalf("buildSigningResignEntitlements() error = %v", err)
	}
	if _, exists := got["beta-reports-active"]; exists {
		t.Fatal("beta-reports-active granted without a profile claim")
	}

	profile["beta-reports-active"] = "yes"
	if _, err := buildSigningResignEntitlements(existing, profile); err == nil {
		t.Fatal("buildSigningResignEntitlements() accepted a non-boolean profile-required claim")
	}
}

func TestSigningResignUnauthorizedClaimsRefusalSurvivesPublicBoundary(t *testing.T) {
	existing := map[string]any{
		"application-identifier":              "OLDTEAM.com.example.app",
		"com.apple.developer.team-identifier": "OLDTEAM",
		"get-task-allow":                      false,
		"keychain-access-groups":              []string{"OLDTEAM.com.example.shared"},
	}
	profile := map[string]any{
		"application-identifier":              "NEWTEAM.com.example.app",
		"com.apple.application-identifier":    "NEWTEAM.com.example.app",
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
		"keychain-access-groups":              []any{"NEWTEAM.*"},
	}
	_, refusal := buildSigningResignEntitlements(existing, profile)
	if refusal == nil {
		t.Fatal("buildSigningResignEntitlements() accepted an unauthorized claim")
	}
	if !signingResignOperationalErrorTree(refusal) {
		t.Fatalf("refusal %q is not public-safe, so the boundary would flatten its remediation", refusal)
	}
	wrapped := wrapSigningResignOperationalError(signingResignStagePreparation, signingResignCodeFilesystem, refusal)
	if !strings.Contains(wrapped.Error(), "then re-run") || !strings.Contains(wrapped.Error(), "keychain-access-groups") {
		t.Fatalf("boundary-wrapped refusal = %q, want the per-claim remediation preserved verbatim", wrapped.Error())
	}
	contextual := wrapSigningResignPublicDetail("target com.example.app entitlements", refusal)
	rewrapped := wrapSigningResignOperationalError(signingResignStagePreparation, signingResignCodeFilesystem, contextual)
	if !strings.Contains(rewrapped.Error(), "target com.example.app entitlements") || !strings.Contains(rewrapped.Error(), "then re-run") {
		t.Fatalf("contextual refusal = %q, want context and remediation preserved", rewrapped.Error())
	}
}

func TestSigningResignCommandSurfacesUnauthorizedClaimRemediation(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	original := executeSigningResignFn
	t.Cleanup(func() { executeSigningResignFn = original })
	refusal := signingResignUnauthorizedClaimsError([]signingResignUnauthorizedClaim{{
		Key:      "keychain-access-groups",
		Existing: []string{"OLDTEAM.com.example.shared"},
		Profile:  []any{"NEWTEAM.*"},
	}})
	executeSigningResignFn = func(context.Context, signingResignOptions) (signingResignResult, error) {
		return signingResignResult{}, refusal
	}
	command := SigningResignCommand()
	if err := command.FlagSet.Parse([]string{
		"--ipa", "input.ipa", "--output", "output.ipa",
		"--identity", "identity.p12", "--profiles-manifest", "profiles.json",
	}); err != nil {
		t.Fatal(err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("SigningResignCommand().Exec() error = nil, want surfaced refusal")
	}
	for _, want := range []string{
		"keychain-access-groups",
		`"OLDTEAM.com.example.shared"`,
		`"NEWTEAM.com.example.shared"`,
		"then re-run",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("command error %q is missing %q, so stderr would omit the remediation", err, want)
		}
	}
}

func TestSigningResignCommandPublicationAmbiguityRetainsCauseAndRedactsIt(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	const secret = "/private/tmp/secret-published-ipa"
	underlying := errors.New("hash failed at " + secret)
	publication := signingResignPublicationAmbiguousError("hash published re-signed IPA failed", underlying)
	original := executeSigningResignFn
	t.Cleanup(func() { executeSigningResignFn = original })
	executeSigningResignFn = func(context.Context, signingResignOptions) (signingResignResult, error) {
		return signingResignResult{}, wrapSigningResignOperationalError(
			signingResignStageArtifact,
			signingResignCodeArtifactHash,
			publication,
		)
	}
	command := SigningResignCommand()
	if err := command.FlagSet.Parse([]string{
		"--ipa", "input.ipa", "--output", "output.ipa",
		"--identity", "identity.p12", "--profiles-manifest", "profiles.json",
	}); err != nil {
		t.Fatal(err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("SigningResignCommand().Exec() error = nil, want publication uncertainty")
	}
	if !errors.Is(err, ErrSigningResignPublicationAmbiguous) || !errors.Is(err, underlying) {
		t.Fatalf("command error = %v, want ambiguity and underlying causes retained", err)
	}
	if strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "artifact verification (artifact-hash)") {
		t.Fatalf("command error = %q, want closed artifact stage/code without private path", err)
	}
}

func TestSigningResignCommandReceiptRenderFailureRetainsPublicationAmbiguity(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("signing resign is macOS-only")
	}
	const secret = "/private/tmp/secret-receipt-renderer"
	renderErr := errors.New("stdout closed at " + secret)
	outputPath := filepath.Join(t.TempDir(), "published.ipa")
	originalExecute := executeSigningResignFn
	originalPrint := printSigningResignResultFn
	t.Cleanup(func() {
		executeSigningResignFn = originalExecute
		printSigningResignResultFn = originalPrint
	})
	executeSigningResignFn = func(context.Context, signingResignOptions) (signingResignResult, error) {
		return signingResignResult{
			Output: signingResignArtifactResult{Path: outputPath},
		}, nil
	}
	printSigningResignResultFn = func(signingResignResult, string, bool) error {
		return renderErr
	}

	command := SigningResignCommand()
	if err := command.FlagSet.Parse([]string{
		"--ipa", "input.ipa", "--output", outputPath,
		"--identity", "identity.p12", "--profiles-manifest", "profiles.json",
	}); err != nil {
		t.Fatal(err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("SigningResignCommand().Exec() error = nil, want post-publication uncertainty")
	}
	if !errors.Is(err, ErrSigningResignPublicationAmbiguous) || !errors.Is(err, renderErr) {
		t.Fatalf("command error = %v, want publication ambiguity and renderer cause", err)
	}
	if strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "artifact verification (artifact-publish)") {
		t.Fatalf("command error = %q, want closed artifact publication stage/code", err)
	}
}

func TestSigningResignCodeContainersIncludeBundleAndXPC(t *testing.T) {
	treePath := t.TempDir()
	plans := []signingResignCodePlan{
		{Path: filepath.Join(treePath, "Payload", "App.app", "Frameworks", "Feature.framework", "Feature")},
		{Path: filepath.Join(treePath, "Payload", "App.app", "PlugIns", "Loadable.bundle", "Loadable")},
		{Path: filepath.Join(treePath, "Payload", "App.app", "Helpers", "Agent.xpc", "Agent")},
	}
	containers := signingResignFrameworkContainers(treePath, plans)
	want := map[string]bool{
		filepath.Join(treePath, "Payload", "App.app", "Frameworks", "Feature.framework"): false,
		filepath.Join(treePath, "Payload", "App.app", "PlugIns", "Loadable.bundle"):      false,
		filepath.Join(treePath, "Payload", "App.app", "Helpers", "Agent.xpc"):            false,
	}
	for _, container := range containers {
		if _, expected := want[container]; !expected {
			t.Fatalf("unexpected container %q in %v", container, containers)
		}
		want[container] = true
	}
	for container, seen := range want {
		if !seen {
			t.Fatalf("containers %v are missing %q", containers, container)
		}
	}
}

func TestSigningResignContainerEntitlementsFollowMainExecutable(t *testing.T) {
	treePath := t.TempDir()
	container := filepath.Join(treePath, "Payload", "App.app", "Frameworks", "Feature.framework")
	info, err := plist.Marshal(map[string]any{"CFBundleExecutable": "Feature"}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(container, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(container, "Info.plist"), info, 0o600); err != nil {
		t.Fatal(err)
	}
	plans := []signingResignCodePlan{
		{Path: filepath.Join(container, "Versions", "A", "Feature"), EntitlementsPath: "/stage/entitlements/version.plist"},
		{Path: filepath.Join(container, "Feature"), EntitlementsPath: filepath.Join(t.TempDir(), "feature.plist")},
	}
	if got := signingResignContainerEntitlementsPath(treePath, container, plans); got != plans[1].EntitlementsPath {
		t.Fatalf("container entitlements path = %q, want main executable document", got)
	}
	if got := signingResignContainerEntitlementsPath(treePath, filepath.Join(treePath, "Payload", "App.app", "PlugIns", "Empty.bundle"), plans); got != "" {
		t.Fatalf("unplanned container entitlements path = %q, want empty", got)
	}
	versioned := filepath.Join(treePath, "Payload", "App.app", "Frameworks", "Versioned.framework")
	if err := os.MkdirAll(filepath.Join(versioned, "Versions", "A"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versioned, "Info.plist"), info, 0o600); err != nil {
		t.Fatal(err)
	}
	versionEntitlements := filepath.Join(t.TempDir(), "versioned.plist")
	versionPlans := []signingResignCodePlan{
		{Path: filepath.Join(versioned, "Versions", "A", "Feature"), EntitlementsPath: ""},
		{Path: filepath.Join(versioned, "Versions", "B", "Feature"), EntitlementsPath: versionEntitlements},
	}
	if got := signingResignContainerEntitlementsPath(treePath, versioned, versionPlans); got != "" {
		t.Fatalf("empty first version entitlements path = %q, want empty", got)
	}
	versionPlans[0].EntitlementsPath = versionEntitlements
	if got := signingResignContainerEntitlementsPath(treePath, versioned, versionPlans); got != versionEntitlements {
		t.Fatalf("versioned container entitlements path = %q, want %q", got, versionEntitlements)
	}
}

func TestSigningResignContainerEntitlementsRejectAmplifiedInfoPlist(t *testing.T) {
	treePath := t.TempDir()
	container := filepath.Join(treePath, "Payload", "App.app", "Frameworks", "Feature.framework")
	if err := os.MkdirAll(container, 0o700); err != nil {
		t.Fatal(err)
	}
	var info strings.Builder
	info.WriteString(`<?xml version="1.0"?><plist><dict><key>CFBundleExecutable</key><string>Feature</string><key>Padding</key><array>`)
	for range infoplist.MaxObjects {
		info.WriteString(`<true/>`)
	}
	info.WriteString(`</array></dict></plist>`)
	if err := os.WriteFile(filepath.Join(container, "Info.plist"), []byte(info.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	entitlements := filepath.Join(t.TempDir(), "feature.plist")
	plans := []signingResignCodePlan{{Path: filepath.Join(container, "Feature"), EntitlementsPath: entitlements}}
	if got := signingResignContainerEntitlementsPath(treePath, container, plans); got != "" {
		t.Fatalf("amplified container Info.plist selected entitlements path %q, want empty", got)
	}
}

func TestSignSigningResignTreePreservesContainerMainEntitlements(t *testing.T) {
	treePath := t.TempDir()
	container := filepath.Join(treePath, "Payload", "App.app", "Frameworks", "Feature.framework")
	if err := os.MkdirAll(container, 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := plist.Marshal(map[string]any{"CFBundleExecutable": "Feature"}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(container, "Info.plist"), info, 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(container, "Feature")
	if err := os.WriteFile(executable, []byte("nested executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	entitlements := filepath.Join(t.TempDir(), "feature.entitlements")
	if err := os.WriteFile(entitlements, []byte("permitted non-identity claim"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = original })
	var calls [][]string
	runSigningResignToolFn = func(_ context.Context, executable string, args ...string) (signingResignToolOutput, error) {
		calls = append(calls, append([]string{executable}, args...))
		return signingResignToolOutput{}, nil
	}
	prepared := signingResignPreparedTree{CodePlans: []signingResignCodePlan{{Path: executable, EntitlementsPath: entitlements}}}
	if err := signSigningResignTree(context.Background(), treePath, prepared, "IDENTITY", "/tmp/keychain"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("codesign calls = %#v, want leaf and container passes", calls)
	}
	if got := strings.Join(calls[1], " "); !strings.Contains(got, "--entitlements "+entitlements) {
		t.Fatalf("container signing call = %q, want prepared entitlements", got)
	}
	calls = nil
	prepared.CodePlans[0].EntitlementsPath = ""
	if err := signSigningResignTree(context.Background(), treePath, prepared, "IDENTITY", "/tmp/keychain"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || strings.Contains(strings.Join(calls[1], " "), "--entitlements") {
		t.Fatalf("empty container entitlements signing call = %#v, want no entitlements flag", calls)
	}
}

func TestDiscoverSigningResignArchiveIgnoresRegularFilesNamedLikeBundles(t *testing.T) {
	info, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.app",
		"CFBundleExecutable":         "App",
		"DTPlatformName":             "iphoneos",
		"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	executable := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	data := buildSigningResignZip(t, []signingResignZipEntry{
		{name: "Payload/App.app/Info.plist", data: info},
		{name: "Payload/App.app/App", data: executable, mode: 0o755},
		{name: "Payload/App.app/Resources/example.app", data: []byte("resource named like a bundle")},
		{name: "Payload/App.app/Resources/example.appex", data: []byte("resource named like an extension")},
	})
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSigningResignArchive(context.Background(), reader); err != nil {
		t.Fatalf("validateSigningResignArchive() error = %v", err)
	}
	tree, err := rootfs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()
	treeOS, err := tree.OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer treeOS.Close()
	if err := materializeSigningResignArchive(context.Background(), reader, treeOS); err != nil {
		t.Fatalf("materializeSigningResignArchive() error = %v", err)
	}
	originalTool := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = originalTool })
	runSigningResignToolFn = func(context.Context, string, ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{
			Stderr: []byte("staged executable: code object is not signed at all"),
		}, errors.New("codesign failed")
	}
	archive, err := discoverSigningResignArchive(context.Background(), reader, tree)
	if err != nil {
		t.Fatalf("discoverSigningResignArchive() error = %v, want regular files named like bundles to be ignored", err)
	}
	if archive.MainPath != "Payload/App.app" {
		t.Fatalf("archive.MainPath = %q, want Payload/App.app", archive.MainPath)
	}
	if len(archive.Targets) != 1 || archive.Targets[0].RelativePath != "Payload/App.app" {
		t.Fatalf("archive.Targets = %+v, want only the main application", archive.Targets)
	}
}

func TestDiscoverSigningResignArchiveStillRejectsNestedBundleDirectories(t *testing.T) {
	info, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.app",
		"CFBundleExecutable":         "App",
		"DTPlatformName":             "iphoneos",
		"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	executable := []byte{
		0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01,
		0x03, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	for _, test := range []struct {
		name    string
		entries []signingResignZipEntry
	}{
		{
			name: "implied directory",
			entries: []signingResignZipEntry{
				{name: "Payload/App.app/Frameworks/Helper.app/Info.plist", data: info},
			},
		},
		{
			name: "explicit directory entry",
			entries: []signingResignZipEntry{
				{name: "Payload/App.app/Frameworks/Helper.app/", directory: true},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			entries := append([]signingResignZipEntry{
				{name: "Payload/App.app/Info.plist", data: info},
				{name: "Payload/App.app/App", data: executable, mode: 0o755},
			}, test.entries...)
			data := buildSigningResignZip(t, entries)
			reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			tree, err := rootfs.New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer tree.Close()
			_, err = discoverSigningResignArchive(context.Background(), reader, tree)
			if err == nil || !strings.Contains(err.Error(), "unsupported nested app target") {
				t.Fatalf("discoverSigningResignArchive() error = %v, want unsupported nested target rejection", err)
			}
		})
	}
}

func TestPreflightSigningResignArchiveRejectsOversizedCentralDirectoryInventory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.ipa")
	data := make([]byte, 22)
	binary.LittleEndian.PutUint32(data[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(data[10:12], signingResignMaxArchiveEntries+1)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := preflightSigningResignArchive(context.Background(), file, int64(len(data))); err == nil || !strings.Contains(err.Error(), "too many archive entries") {
		t.Fatalf("error = %v", err)
	}
}

func TestPreflightSigningResignArchiveRejectsOversizedDirectoryBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.ipa")
	data := make([]byte, 22)
	binary.LittleEndian.PutUint32(data[0:4], 0x06054b50)
	binary.LittleEndian.PutUint32(data[12:16], uint32(signingResignMaxCentralDirectoryBytes+1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := preflightSigningResignArchive(context.Background(), file, int64(len(data))); err == nil || !strings.Contains(err.Error(), "central directory exceeds") {
		t.Fatalf("error = %v", err)
	}
}

func TestPreflightSigningResignArchiveIgnoresFakeEOCDInComment(t *testing.T) {
	data := make([]byte, 22+4)
	binary.LittleEndian.PutUint32(data[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(data[20:22], 4)
	binary.LittleEndian.PutUint32(data[22:26], 0x06054b50)
	path := filepath.Join(t.TempDir(), "fake-comment.ipa")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := preflightSigningResignArchive(context.Background(), file, int64(len(data))); err != nil {
		t.Fatalf("valid EOCD with comment rejected: %v", err)
	}
}

func TestPreflightSigningResignArchiveRejectsZIP64DirectoryOffsetSentinel(t *testing.T) {
	data := make([]byte, 22)
	binary.LittleEndian.PutUint32(data[0:4], 0x06054b50)
	binary.LittleEndian.PutUint32(data[16:20], 0xffffffff)
	path := filepath.Join(t.TempDir(), "zip64-offset.ipa")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := preflightSigningResignArchive(context.Background(), file, int64(len(data))); err == nil || !strings.Contains(err.Error(), "ZIP64") {
		t.Fatalf("error = %v, want ZIP64 rejection", err)
	}
}

func TestPreflightSigningResignArchiveAcceptsBoundedZIP64Directory(t *testing.T) {
	data := make([]byte, 56+20+22)
	binary.LittleEndian.PutUint32(data[0:4], 0x06064b50)
	binary.LittleEndian.PutUint64(data[4:12], 44)
	binary.LittleEndian.PutUint32(data[56:60], 0x07064b50)
	binary.LittleEndian.PutUint64(data[64:72], 0)
	binary.LittleEndian.PutUint32(data[72:76], 1)
	eocd := data[76:]
	binary.LittleEndian.PutUint32(eocd[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(eocd[8:10], 0xffff)
	binary.LittleEndian.PutUint16(eocd[10:12], 0xffff)
	binary.LittleEndian.PutUint32(eocd[12:16], 0xffffffff)
	binary.LittleEndian.PutUint32(eocd[16:20], 0xffffffff)

	path := filepath.Join(t.TempDir(), "bounded-zip64.ipa")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := preflightSigningResignArchive(context.Background(), file, int64(len(data))); err != nil {
		t.Fatalf("bounded ZIP64 directory rejected: %v", err)
	}
}

func TestPreflightSigningResignArchiveRejectsWrappedClassicEntryCount(t *testing.T) {
	const count = 65_536
	data := make([]byte, count*46+22)
	for offset := 0; offset < count*46; offset += 46 {
		binary.LittleEndian.PutUint32(data[offset:], 0x02014b50)
	}
	eocd := data[count*46:]
	binary.LittleEndian.PutUint32(eocd, 0x06054b50)
	binary.LittleEndian.PutUint32(eocd[12:16], uint32(count*46))
	binary.LittleEndian.PutUint32(eocd[16:20], 0)
	path := filepath.Join(t.TempDir(), "wrapped-count.ipa")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := preflightSigningResignArchive(context.Background(), file, int64(len(data))); err == nil || !strings.Contains(err.Error(), "too many archive entries") {
		t.Fatalf("error = %v", err)
	}
}

func TestPreflightSigningResignArchiveHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := preflightSigningResignArchive(ctx, nil, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
}

func TestPreflightSigningResignArchiveRejectsOversizedZIP64Inventory(t *testing.T) {
	data := make([]byte, 56+20+22)
	binary.LittleEndian.PutUint32(data[0:4], 0x06064b50)
	binary.LittleEndian.PutUint64(data[4:12], 44)
	binary.LittleEndian.PutUint64(data[24:32], signingResignMaxArchiveEntries+1)
	binary.LittleEndian.PutUint64(data[32:40], signingResignMaxArchiveEntries+1)
	binary.LittleEndian.PutUint32(data[56:60], 0x07064b50)
	binary.LittleEndian.PutUint32(data[72:76], 1)
	eocd := data[76:]
	binary.LittleEndian.PutUint32(eocd[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(eocd[8:10], 0xffff)
	binary.LittleEndian.PutUint16(eocd[10:12], 0xffff)
	binary.LittleEndian.PutUint32(eocd[12:16], 0xffffffff)
	binary.LittleEndian.PutUint32(eocd[16:20], 0xffffffff)

	path := filepath.Join(t.TempDir(), "oversized-zip64.ipa")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := preflightSigningResignArchive(context.Background(), file, int64(len(data))); err == nil || !strings.Contains(err.Error(), "too many archive entries") {
		t.Fatalf("error = %v, want ZIP64 entry-count rejection", err)
	}
}

func TestPreflightSigningResignArchiveAcceptsPrefixedClassicDirectory(t *testing.T) {
	data := buildSigningResignZip(t, []signingResignZipEntry{{name: "Payload/App.app/Info.plist", data: []byte("plist")}})
	data = append([]byte("prefix"), data...)
	path := filepath.Join(t.TempDir(), "prefixed.ipa")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := preflightSigningResignArchive(context.Background(), file, int64(len(data))); err != nil {
		t.Fatalf("prefixed classic directory rejected: %v", err)
	}
}

func TestPreflightSigningResignArchiveRejectsUnderstatedDirectoryBytes(t *testing.T) {
	const actualEntries = signingResignMaxArchiveEntries + 1
	directoryBytes := actualEntries * 46
	data := make([]byte, directoryBytes+22)
	for offset := 0; offset < directoryBytes; offset += 46 {
		binary.LittleEndian.PutUint32(data[offset:], 0x02014b50)
	}
	eocd := data[directoryBytes:]
	binary.LittleEndian.PutUint32(eocd[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(eocd[10:12], 1)
	binary.LittleEndian.PutUint32(eocd[12:16], 46)

	path := filepath.Join(t.TempDir(), "understated-directory.ipa")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := preflightSigningResignArchive(context.Background(), file, int64(len(data))); err == nil || !strings.Contains(err.Error(), "too many archive entries") {
		t.Fatalf("error = %v, want understated directory entry-count rejection", err)
	}
}

func TestPreflightSigningResignArchiveRejectsExcessCentralDirectoryMetadata(t *testing.T) {
	const extraBytes = int64(65535)
	const recordBytes = int64(46) + extraBytes
	records := int64(signingResignMaxCentralDirectoryMetadataBytes)/extraBytes + 1
	directoryBytes := records * recordBytes

	path := filepath.Join(t.TempDir(), "oversized-metadata.ipa")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := file.Truncate(directoryBytes + 22); err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 46)
	binary.LittleEndian.PutUint32(header[0:4], 0x02014b50)
	binary.LittleEndian.PutUint16(header[30:32], uint16(extraBytes))
	for offset := int64(0); offset < directoryBytes; offset += recordBytes {
		if written, err := file.WriteAt(header, offset); err != nil || written != len(header) {
			t.Fatalf("write central directory header at %d: n=%d err=%v", offset, written, err)
		}
	}
	eocd := make([]byte, 22)
	binary.LittleEndian.PutUint32(eocd[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(eocd[8:10], uint16(records))
	binary.LittleEndian.PutUint16(eocd[10:12], uint16(records))
	binary.LittleEndian.PutUint32(eocd[12:16], uint32(directoryBytes))
	if written, err := file.WriteAt(eocd, directoryBytes); err != nil || written != len(eocd) {
		t.Fatalf("write EOCD: n=%d err=%v", written, err)
	}

	if err := preflightSigningResignArchive(context.Background(), file, directoryBytes+22); err == nil || !strings.Contains(err.Error(), "metadata exceeds") {
		t.Fatalf("error = %v, want central-directory metadata rejection", err)
	}
}

func TestPreflightSigningResignArchiveAcceptsPrefixedZIP64Directory(t *testing.T) {
	const prefixBytes = 7
	const directoryBytes = 46
	const zip64RecordBytes = 56
	const locatorBytes = 20
	const eocdBytes = 22
	zip64Offset := prefixBytes + directoryBytes
	locatorOffset := zip64Offset + zip64RecordBytes
	eocdOffset := locatorOffset + locatorBytes
	data := make([]byte, prefixBytes+directoryBytes+zip64RecordBytes+locatorBytes+eocdBytes)
	for index := 0; index < prefixBytes; index++ {
		data[index] = 0xA5
	}
	binary.LittleEndian.PutUint32(data[prefixBytes:prefixBytes+4], 0x02014b50)
	zip64 := data[zip64Offset:]
	binary.LittleEndian.PutUint32(zip64[0:4], 0x06064b50)
	binary.LittleEndian.PutUint64(zip64[4:12], 44)
	binary.LittleEndian.PutUint64(zip64[32:40], 1)
	binary.LittleEndian.PutUint64(zip64[40:48], directoryBytes)
	binary.LittleEndian.PutUint64(zip64[48:56], 0)
	locator := data[locatorOffset:]
	binary.LittleEndian.PutUint32(locator[0:4], 0x07064b50)
	binary.LittleEndian.PutUint64(locator[8:16], uint64(zip64Offset))
	binary.LittleEndian.PutUint32(locator[16:20], 1)
	eocd := data[eocdOffset:]
	binary.LittleEndian.PutUint32(eocd[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(eocd[8:10], 0xffff)
	binary.LittleEndian.PutUint16(eocd[10:12], 0xffff)
	binary.LittleEndian.PutUint32(eocd[12:16], 0xffffffff)
	binary.LittleEndian.PutUint32(eocd[16:20], 0xffffffff)

	path := filepath.Join(t.TempDir(), "prefixed-zip64.ipa")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := preflightSigningResignArchive(context.Background(), file, int64(len(data))); err != nil {
		t.Fatalf("prefixed ZIP64 directory rejected: %v", err)
	}
	reader, err := zip.NewReader(file, int64(len(data)))
	if err != nil {
		t.Fatalf("archive/zip rejected prefixed ZIP64 directory: %v", err)
	}
	if len(reader.File) != 1 {
		t.Fatalf("archive/zip returned %d files, want one", len(reader.File))
	}
}
