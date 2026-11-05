package distribution

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.mozilla.org/pkcs7"
	"howett.net/plist"
)

func TestInspectIPAVerifiesEveryMainExecutableArchitectureAgainstProfile(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("codesign verification is macOS-only")
	}
	profile := signedProfile(t, profileFixture{BundleID: "com.example.demo", Devices: []string{"one"}, Expires: time.Now().Add(time.Hour)})
	message, err := pkcs7.Parse(profile)
	if err != nil || len(message.Certificates) == 0 {
		t.Fatalf("parse fixture profile: %v", err)
	}
	leaf := message.Certificates[0].Raw
	var extracted int
	var verified, listed bool
	runCodeSignTool = func(_ context.Context, name string, args ...string) ([]byte, error) {
		switch name {
		case "/usr/bin/lipo":
			if len(args) != 2 || args[0] != "-archs" {
				t.Fatalf("unexpected lipo args: %#v", args)
			}
			listed = true
			return []byte("arm64 x86_64\n"), nil
		case "/usr/bin/codesign":
			if len(args) > 1 && args[0] == "--verify" && args[1] == "--deep" {
				want := []string{"--verify", "--deep", "--strict", "--all-architectures", "--verbose=4"}
				if len(args) != len(want)+1 || strings.Join(args[:len(want)], "\x00") != strings.Join(want, "\x00") {
					t.Fatalf("unexpected codesign verify args: %#v", args)
				}
				verified = true
			}
			if len(args) == 6 && args[0] == "-d" && args[1] == "-a" && args[3] == "--entitlements" && args[4] == ":-" {
				return plist.Marshal(map[string]any{
					"application-identifier":              "TEAM123.com.example.demo",
					"com.apple.developer.team-identifier": "TEAM123",
				}, plist.XMLFormat)
			}
			for _, argument := range args {
				if strings.HasPrefix(argument, "--extract-certificates=") {
					prefix := strings.TrimPrefix(argument, "--extract-certificates=")
					if err := os.WriteFile(prefix+"0", leaf, 0o600); err != nil {
						return nil, err
					}
					extracted++
				}
			}
			return nil, nil
		default:
			t.Fatalf("unexpected tool %q", name)
			return nil, nil
		}
	}
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })
	path := writeIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistBytes(t, map[string]any{
			"CFBundleIdentifier": "com.example.demo", "CFBundleName": "Demo",
			"CFBundleShortVersionString": "1.0", "CFBundleVersion": "1", "CFBundleExecutable": "Demo",
		}),
		"Payload/Demo.app/Demo":                     fakeMachO("main"),
		"Payload/Demo.app/Frameworks/Shared.dylib":  fakeMachO("nested"),
		"Payload/Demo.app/embedded.mobileprovision": profile,
	})
	got := inspectPath(t, path, false)
	if got.Signing.CodeSignatureVerification.Status != CodeSignatureVerified || extracted != 4 || !verified || !listed {
		t.Fatalf("verification=%#v extracted=%d", got.Signing.CodeSignatureVerification, extracted)
	}
	if len(got.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints) != 1 {
		t.Fatalf("fingerprints=%#v", got.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints)
	}
}

func TestInspectIPARejectsEntitlementMismatchInSecondaryArchitecture(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("codesign verification is macOS-only")
	}
	profile := signedProfile(t, profileFixture{BundleID: "com.example.demo", Devices: []string{"one"}, Expires: time.Now().Add(time.Hour)})
	message, err := pkcs7.Parse(profile)
	if err != nil || len(message.Certificates) == 0 {
		t.Fatalf("parse fixture profile: %v", err)
	}
	leaf := message.Certificates[0].Raw
	runCodeSignTool = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "/usr/bin/lipo" {
			return []byte("arm64 x86_64\n"), nil
		}
		if len(args) == 6 && args[0] == "-d" && args[1] == "-a" && args[3] == "--entitlements" && args[4] == ":-" {
			entitlements := map[string]any{
				"application-identifier":              "TEAM123.com.example.demo",
				"com.apple.developer.team-identifier": "TEAM123",
			}
			for index, argument := range args {
				if argument == "-a" && index+1 < len(args) && args[index+1] == "x86_64" {
					entitlements["get-task-allow"] = true
				}
			}
			return plist.Marshal(entitlements, plist.XMLFormat)
		}
		for _, argument := range args {
			if strings.HasPrefix(argument, "--extract-certificates=") {
				return nil, os.WriteFile(strings.TrimPrefix(argument, "--extract-certificates=")+"0", leaf, 0o600)
			}
		}
		return nil, nil
	}
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })
	path := writeIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistBytes(t, map[string]any{
			"CFBundleIdentifier": "com.example.demo", "CFBundleName": "Demo",
			"CFBundleShortVersionString": "1.0", "CFBundleVersion": "1", "CFBundleExecutable": "Demo",
		}),
		"Payload/Demo.app/Demo":                     fakeMachO("main"),
		"Payload/Demo.app/embedded.mobileprovision": profile,
	})
	got := inspectPath(t, path, false)
	if got.Signing.CodeSignatureVerification.Status != CodeSignatureInvalid || !strings.Contains(got.Signing.CodeSignatureVerification.Reason, "get-task-allow") {
		t.Fatalf("verification=%#v", got.Signing.CodeSignatureVerification)
	}
}

func TestVerifyMainExecutableEntitlementsQueriesEveryArchitectureOnce(t *testing.T) {
	profile := entitlementTestProfile()
	profile.ApplicationIdentifierPrefix = []string{"LEGACY123"}
	profile.Entitlements["application-identifier"] = "LEGACY123.com.example.*"
	queries := make(map[string]int)
	runCodeSignTool = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "/usr/bin/lipo" {
			return []byte("arm64 x86_64\n"), nil
		}
		architecture := requireEntitlementDisplayArgs(t, args, "/tmp/Demo")
		queries[architecture]++
		entitlements := validMainEntitlements()
		entitlements["application-identifier"] = "LEGACY123.com.example.demo"
		if architecture == "x86_64" {
			entitlements["application-identifier"] = "LEGACY123.com.example.other"
		}
		return plist.Marshal(entitlements, plist.XMLFormat)
	}
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })

	_, err := verifyMainExecutableEntitlements(context.Background(), "/tmp/Demo", "com.example.demo", profile)
	if err == nil || !strings.Contains(err.Error(), "CFBundleIdentifier") {
		t.Fatalf("verifyMainExecutableEntitlements() error = %v, want secondary-architecture mismatch", err)
	}
	if queries["arm64"] != 1 || queries["x86_64"] != 1 || len(queries) != 2 {
		t.Fatalf("entitlement architecture queries = %#v, want each architecture exactly once", queries)
	}
}

func TestVerifyMainExecutableEntitlementsFailsClosedPerSlice(t *testing.T) {
	for _, test := range []struct {
		name       string
		secondary  func() ([]byte, error)
		wantReason string
	}{
		{
			name:       "extraction failure",
			secondary:  func() ([]byte, error) { return nil, errors.New("codesign failed") },
			wantReason: "could not extract signed main-app entitlements for architecture x86_64",
		},
		{
			name:       "malformed plist",
			secondary:  func() ([]byte, error) { return []byte("not a plist"), nil },
			wantReason: "signed main-app entitlements are invalid",
		},
		{
			name: "missing required entitlement",
			secondary: func() ([]byte, error) {
				return plist.Marshal(map[string]any{
					"com.apple.developer.team-identifier": "TEAM123",
				}, plist.XMLFormat)
			},
			wantReason: "signed main-app application identifier is missing",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var queried []string
			runCodeSignTool = func(_ context.Context, name string, args ...string) ([]byte, error) {
				if name == "/usr/bin/lipo" {
					return []byte("arm64 x86_64\n"), nil
				}
				architecture := requireEntitlementDisplayArgs(t, args, "/tmp/Demo")
				queried = append(queried, architecture)
				if architecture == "x86_64" {
					return test.secondary()
				}
				return plist.Marshal(validMainEntitlements(), plist.XMLFormat)
			}
			t.Cleanup(func() { runCodeSignTool = runBoundedTool })

			_, err := verifyMainExecutableEntitlements(context.Background(), "/tmp/Demo", "com.example.demo", entitlementTestProfile())
			if err == nil || !strings.Contains(err.Error(), test.wantReason) {
				t.Fatalf("verifyMainExecutableEntitlements() error = %v, want %q", err, test.wantReason)
			}
			if strings.Join(queried, ",") != "arm64,x86_64" {
				t.Fatalf("queried architectures = %#v, want both slices once", queried)
			}
		})
	}
}

func TestVerifyMainExecutableEntitlementsPreservesThinBinaryBehavior(t *testing.T) {
	var queried []string
	runCodeSignTool = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "/usr/bin/lipo" {
			return []byte("arm64\n"), nil
		}
		queried = append(queried, requireEntitlementDisplayArgs(t, args, "/tmp/Demo"))
		return plist.Marshal(validMainEntitlements(), plist.XMLFormat)
	}
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })

	architectures, err := verifyMainExecutableEntitlements(context.Background(), "/tmp/Demo", "com.example.demo", entitlementTestProfile())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(architectures, ",") != "arm64" || strings.Join(queried, ",") != "arm64" {
		t.Fatalf("architectures=%#v queried=%#v, want one arm64 slice", architectures, queried)
	}
}

func TestVerifyMainExecutableEntitlementsRejectsInvalidLipoOutput(t *testing.T) {
	for _, output := range []string{"arm64 bad;architecture\n", "arm64 arm64\n"} {
		t.Run(strings.TrimSpace(output), func(t *testing.T) {
			codesignCalled := false
			runCodeSignTool = func(_ context.Context, name string, _ ...string) ([]byte, error) {
				if name == "/usr/bin/lipo" {
					return []byte(output), nil
				}
				codesignCalled = true
				return nil, nil
			}
			t.Cleanup(func() { runCodeSignTool = runBoundedTool })

			if _, err := verifyMainExecutableEntitlements(context.Background(), "/tmp/Demo", "com.example.demo", entitlementTestProfile()); err == nil {
				t.Fatal("verifyMainExecutableEntitlements() error = nil, want invalid architecture rejection")
			}
			if codesignCalled {
				t.Fatal("codesign was called with unvalidated lipo output")
			}
		})
	}
}

func TestValidateSignedMainAppEntitlementsBindsExactBundleIdentifier(t *testing.T) {
	for _, test := range []struct {
		name                        string
		applicationIdentifierPrefix string
		profileApplicationID        string
		signedApplicationID         any
		signedTeamID                string
		wantError                   bool
	}{
		{
			name:                 "exact profile and matching signed identifier",
			profileApplicationID: "TEAM123.com.example.demo",
			signedApplicationID:  "TEAM123.com.example.demo",
			signedTeamID:         "TEAM123",
		},
		{
			name:                        "legacy application prefix differs from team",
			applicationIdentifierPrefix: "LEGACY123",
			profileApplicationID:        "LEGACY123.com.example.demo",
			signedApplicationID:         "LEGACY123.com.example.demo",
			signedTeamID:                "TEAM123",
		},
		{
			name:                        "legacy signed identifier incorrectly uses team",
			applicationIdentifierPrefix: "LEGACY123",
			profileApplicationID:        "LEGACY123.com.example.*",
			signedApplicationID:         "TEAM123.com.example.demo",
			signedTeamID:                "TEAM123",
			wantError:                   true,
		},
		{
			name:                 "wildcard profile and matching signed identifier",
			profileApplicationID: "TEAM123.com.example.*",
			signedApplicationID:  "TEAM123.com.example.demo",
			signedTeamID:         "TEAM123",
		},
		{
			name:                 "wildcard profile different suffix",
			profileApplicationID: "TEAM123.com.example.*",
			signedApplicationID:  "TEAM123.com.example.other",
			signedTeamID:         "TEAM123",
			wantError:            true,
		},
		{
			name:                 "team prefix ambiguity",
			profileApplicationID: "TEAM123.*",
			signedApplicationID:  "TEAM1234.com.example.demo",
			signedTeamID:         "TEAM123",
			wantError:            true,
		},
		{
			name:                 "application identifier injection",
			profileApplicationID: "TEAM123.*",
			signedApplicationID:  `TEAM123.com.example.demo" or true`,
			signedTeamID:         "TEAM123",
			wantError:            true,
		},
		{
			name:                 "missing signed identifier",
			profileApplicationID: "TEAM123.*",
			signedApplicationID:  nil,
			signedTeamID:         "TEAM123",
			wantError:            true,
		},
		{
			name:                 "wrong signed team entitlement",
			profileApplicationID: "TEAM123.com.example.demo",
			signedApplicationID:  "TEAM123.com.example.demo",
			signedTeamID:         "OTHERTEAM",
			wantError:            true,
		},
		{
			name:                 "signed team entitlement has trailing whitespace",
			profileApplicationID: "TEAM123.com.example.demo",
			signedApplicationID:  "TEAM123.com.example.demo",
			signedTeamID:         "TEAM123 ",
			wantError:            true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile := entitlementTestProfile()
			if test.applicationIdentifierPrefix != "" {
				profile.ApplicationIdentifierPrefix = []string{test.applicationIdentifierPrefix}
			}
			profile.Entitlements["application-identifier"] = test.profileApplicationID
			entitlements := map[string]any{
				"com.apple.developer.team-identifier": test.signedTeamID,
			}
			if test.signedApplicationID != nil {
				entitlements["application-identifier"] = test.signedApplicationID
			}
			data, err := plist.Marshal(entitlements, plist.XMLFormat)
			if err != nil {
				t.Fatal(err)
			}
			err = validateSignedMainAppEntitlements(data, "com.example.demo", profile)
			if (err != nil) != test.wantError {
				t.Fatalf("validateSignedMainAppEntitlements() error = %v, wantError=%t", err, test.wantError)
			}
		})
	}
}

func TestValidateSignedMainAppEntitlementsRejectsUnsafeProfilePrefix(t *testing.T) {
	for _, test := range []struct {
		name     string
		prefixes []string
	}{
		{name: "missing", prefixes: []string{}},
		{name: "ambiguous", prefixes: []string{"TEAM123", "LEGACY123"}},
		{name: "duplicate declarations", prefixes: []string{"TEAM123", "TEAM123"}},
		{name: "unsupported characters", prefixes: []string{`TEAM"123`}},
		{name: "excessive length", prefixes: []string{strings.Repeat("A", 33)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile := entitlementTestProfile()
			profile.ApplicationIdentifierPrefix = test.prefixes
			data, err := plist.Marshal(validMainEntitlements(), plist.XMLFormat)
			if err != nil {
				t.Fatal(err)
			}
			err = validateSignedMainAppEntitlements(data, "com.example.demo", profile)
			if err == nil || !strings.Contains(err.Error(), "application identifier prefix") {
				t.Fatalf("validateSignedMainAppEntitlements() error = %v, want safe single prefix rejection", err)
			}
		})
	}
}

func TestVerifyMainExecutableEntitlementsStopsBetweenArchitecturesOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls []string
	runCodeSignTool = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if name == "/usr/bin/lipo" {
			return []byte("arm64 x86_64 arm64e\n"), nil
		}
		cancel()
		return nil, context.Canceled
	}
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })
	_, err := verifyMainExecutableEntitlements(ctx, "/tmp/Demo", "com.example.demo", parsedProfile{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("verifyMainExecutableEntitlements() error = %v, want context.Canceled", err)
	}
	if len(calls) != 2 {
		t.Fatalf("tool calls = %#v, want lipo and one architecture only", calls)
	}
}

func TestMaterializeMainAppContextCancellationStopsBeforeLaterFiles(t *testing.T) {
	path := writeOrderedIPA(t, []orderedZipEntry{
		{Name: "Payload/Demo.app/Demo", Data: fakeMachO("main")},
		{Name: "Payload/Demo.app/Frameworks/Later.dylib", Data: fakeMachO("later")},
	})
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(file, info.Size())
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ctx, cancel := context.WithCancel(context.Background())
	duringMaterializedCopyForTest = cancel
	t.Cleanup(func() { duringMaterializedCopyForTest = nil })
	err = materializeMainAppContext(ctx, root, reader.File, "Payload/Demo.app")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("materializeMainAppContext() error = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "Frameworks", "Later.dylib")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("later nested code stat error = %v, want not found", err)
	}
}

func TestRunCodeSignInvocationUsesIndependentDeadlines(t *testing.T) {
	var deadlines []time.Time
	runCodeSignTool = func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("codesign invocation context has no deadline")
		}
		deadlines = append(deadlines, deadline)
		return nil, nil
	}
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })

	if _, err := runCodeSignInvocation(context.Background(), "codesign", "--verify"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := runCodeSignInvocation(context.Background(), "lipo", "-archs"); err != nil {
		t.Fatal(err)
	}
	if len(deadlines) != 2 || !deadlines[1].After(deadlines[0]) {
		t.Fatalf("invocation deadlines = %#v, want a fresh deadline for every tool call", deadlines)
	}
}

func TestRunCodeSignInvocationPropagatesParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runCodeSignTool = func(toolCtx context.Context, _ string, _ ...string) ([]byte, error) {
		if !errors.Is(toolCtx.Err(), context.Canceled) {
			t.Fatalf("tool context error = %v, want context.Canceled", toolCtx.Err())
		}
		return nil, toolCtx.Err()
	}
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })

	if _, err := runCodeSignInvocation(ctx, "codesign", "--verify"); !errors.Is(err, context.Canceled) {
		t.Fatalf("runCodeSignInvocation() error = %v, want context.Canceled", err)
	}
}

func TestInspectIPARejectsNestedCodeSignedByDifferentLeafEvenWhenDeepVerifyPasses(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("codesign verification is macOS-only")
	}
	profile := signedProfile(t, profileFixture{BundleID: "com.example.demo", Devices: []string{"one"}, Expires: time.Now().Add(time.Hour)})
	message, err := pkcs7.Parse(profile)
	if err != nil || len(message.Certificates) == 0 {
		t.Fatalf("parse fixture profile: %v", err)
	}
	mainLeaf := message.Certificates[0].Raw
	otherProfile := signedProfile(t, profileFixture{BundleID: "com.example.other", Devices: []string{"one"}, Expires: time.Now().Add(time.Hour)})
	otherMessage, err := pkcs7.Parse(otherProfile)
	if err != nil || len(otherMessage.Certificates) == 0 {
		t.Fatalf("parse other profile: %v", err)
	}
	otherLeaf := otherMessage.Certificates[0].Raw
	runCodeSignTool = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "/usr/bin/lipo" {
			return []byte("arm64\n"), nil
		}
		if len(args) == 6 && args[0] == "-d" && args[1] == "-a" && args[3] == "--entitlements" && args[4] == ":-" {
			return plist.Marshal(map[string]any{
				"application-identifier":              "TEAM123.com.example.demo",
				"com.apple.developer.team-identifier": "TEAM123",
			}, plist.XMLFormat)
		}
		for _, argument := range args {
			if strings.HasPrefix(argument, "--extract-certificates=") {
				leaf := mainLeaf
				if strings.Contains(args[len(args)-1], "Shared.dylib") {
					leaf = otherLeaf
				}
				return nil, os.WriteFile(strings.TrimPrefix(argument, "--extract-certificates=")+"0", leaf, 0o600)
			}
		}
		return nil, nil
	}
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })
	path := writeIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistBytes(t, map[string]any{
			"CFBundleIdentifier": "com.example.demo", "CFBundleName": "Demo",
			"CFBundleShortVersionString": "1.0", "CFBundleVersion": "1", "CFBundleExecutable": "Demo",
		}),
		"Payload/Demo.app/Demo":                     fakeMachO("main"),
		"Payload/Demo.app/Frameworks/Shared.dylib":  fakeMachO("nested"),
		"Payload/Demo.app/embedded.mobileprovision": profile,
	})
	got := inspectPath(t, path, false)
	if got.Signing.CodeSignatureVerification.Status != CodeSignatureInvalid || !strings.Contains(got.Signing.CodeSignatureVerification.Reason, "nested signed code signer") {
		t.Fatalf("verification=%#v", got.Signing.CodeSignatureVerification)
	}
}

func TestInspectIPACodeSignToolAbsenceIsExplicitlyNotVerified(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("codesign verification is macOS-only")
	}
	runCodeSignTool = func(context.Context, string, ...string) ([]byte, error) { return nil, exec.ErrNotFound }
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })
	path := validExecutableIPA(t)
	got := inspectPath(t, path, false)
	if got.Signing.CodeSignatureVerification.Status != CodeSignatureNotVerified {
		t.Fatalf("verification=%#v", got.Signing.CodeSignatureVerification)
	}
}

func TestInspectIPARejectsSignerOutsideEmbeddedProfile(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("codesign verification is macOS-only")
	}
	otherProfile := signedProfile(t, profileFixture{BundleID: "com.example.other", Devices: []string{"one"}, Expires: time.Now().Add(time.Hour)})
	message, err := pkcs7.Parse(otherProfile)
	if err != nil || len(message.Certificates) == 0 {
		t.Fatalf("parse other profile: %v", err)
	}
	otherLeaf := message.Certificates[0].Raw
	runCodeSignTool = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "/usr/bin/lipo" {
			return []byte("arm64\n"), nil
		}
		for _, argument := range args {
			if strings.HasPrefix(argument, "--extract-certificates=") {
				return nil, os.WriteFile(strings.TrimPrefix(argument, "--extract-certificates=")+"0", otherLeaf, 0o600)
			}
		}
		if len(args) == 6 && args[0] == "-d" && args[1] == "-a" && args[3] == "--entitlements" && args[4] == ":-" {
			return plist.Marshal(map[string]any{
				"application-identifier":              "TEAM123.com.example.demo",
				"com.apple.developer.team-identifier": "TEAM123",
			}, plist.XMLFormat)
		}
		return nil, nil
	}
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })
	got := inspectPath(t, validExecutableIPA(t), false)
	if got.Signing.CodeSignatureVerification.Status != CodeSignatureInvalid {
		t.Fatalf("verification=%#v", got.Signing.CodeSignatureVerification)
	}
}

func TestEnumerateMachOFilesIgnoresJavaClassMagicCollision(t *testing.T) {
	appPath := t.TempDir()
	mainPath := filepath.Join(appPath, "Demo")
	fatPath := filepath.Join(appPath, "Universal")
	classPath := filepath.Join(appPath, "Resource.class")
	malformedFatPath := filepath.Join(appPath, "MalformedUniversal")
	thinMagicResourcePath := filepath.Join(appPath, "ThinMagicResource")
	if err := os.WriteFile(mainPath, fakeMachO("main"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fatPath, validFatMachO(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(classPath, []byte{0xca, 0xfe, 0xba, 0xbe, 0x00, 0x00, 0x00, 0x3d}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(malformedFatPath, []byte{0xca, 0xfe, 0xba, 0xbe, 0x00, 0x00, 0x00, 0x01}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(thinMagicResourcePath, []byte{0xcf, 0xfa, 0xed, 0xfe, 'n', 'o', 't', '-', 'm', 'a', 'c', 'h', 'o'}, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := enumerateMachOFilesContext(context.Background(), appPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{mainPath, fatPath}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("enumerateMachOFilesContext() = %#v, want %#v", got, want)
	}
}

func TestEnumerateMachOFilesIgnoresNonLoadableObjectFile(t *testing.T) {
	appPath := t.TempDir()
	mainPath := filepath.Join(appPath, "Demo")
	objectPath := filepath.Join(appPath, "Resource.o")
	if err := os.WriteFile(mainPath, fakeMachO("main"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(objectPath, fakeMachOType("resource", 1), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := enumerateMachOFilesContext(context.Background(), appPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != mainPath {
		t.Fatalf("enumerateMachOFilesContext() = %#v, want only %q", got, mainPath)
	}
}

func TestEnumerateMachOFilesIncludesOnlyLoadableThinImageTypes(t *testing.T) {
	appPath := t.TempDir()
	want := []string{
		filepath.Join(appPath, "Bundle"),
		filepath.Join(appPath, "Dylib"),
		filepath.Join(appPath, "Executable"),
	}
	for _, fixture := range []struct {
		name     string
		fileType byte
	}{
		{name: "Executable", fileType: 2},
		{name: "Dylib", fileType: 6},
		{name: "Bundle", fileType: 8},
		{name: "ObjectResource", fileType: 1},
	} {
		if err := os.WriteFile(filepath.Join(appPath, fixture.name), fakeMachOType(fixture.name, fixture.fileType), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := enumerateMachOFilesContext(context.Background(), appPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("enumerateMachOFilesContext() = %#v, want %#v", got, want)
	}
}

func TestEnumerateMachOFilesClassifiesFatImageTypesSafely(t *testing.T) {
	t.Run("fat object is a resource", func(t *testing.T) {
		appPath := t.TempDir()
		mainPath := filepath.Join(appPath, "Demo")
		if err := os.WriteFile(mainPath, fakeMachO("main"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appPath, "Resource.o"), fatMachOWithTypes(1), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := enumerateMachOFilesContext(context.Background(), appPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0] != mainPath {
			t.Fatalf("enumerateMachOFilesContext() = %#v, want only %q", got, mainPath)
		}
	})

	t.Run("mixed loadable and object slices are rejected", func(t *testing.T) {
		appPath := t.TempDir()
		if err := os.WriteFile(filepath.Join(appPath, "Mixed"), fatMachOWithTypes(2, 1), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := enumerateMachOFilesContext(context.Background(), appPath); err == nil || !strings.Contains(err.Error(), "mixes loadable and non-loadable") {
			t.Fatalf("enumerateMachOFilesContext() error = %v, want mixed-type rejection", err)
		}
	})

	t.Run("malformed slice is rejected after valid fat header", func(t *testing.T) {
		appPath := t.TempDir()
		data := fatMachOWithTypes(2, 6)
		secondSliceOffset := 8 + 2*20 + 32
		clear(data[secondSliceOffset : secondSliceOffset+4])
		if err := os.WriteFile(filepath.Join(appPath, "Malformed"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := enumerateMachOFilesContext(context.Background(), appPath); err == nil || !strings.Contains(err.Error(), "malformed architecture slice") {
			t.Fatalf("enumerateMachOFilesContext() error = %v, want malformed-slice rejection", err)
		}
	})
}

func TestValidateTeamIdentifierRejectsRequirementInjection(t *testing.T) {
	for _, value := range []string{"TEAM123", "abc123", strings.Repeat("A", 32)} {
		if err := validateTeamIdentifier(value); err != nil {
			t.Fatalf("validateTeamIdentifier(%q) = %v", value, err)
		}
	}
	for _, value := range []string{"", `TEAM" or true`, "TEAM 123", "TÉAM123", strings.Repeat("A", 33)} {
		if err := validateTeamIdentifier(value); err == nil {
			t.Fatalf("validateTeamIdentifier(%q) unexpectedly succeeded", value)
		}
	}
}

func TestVerifyMainAppCodeSignatureRejectsUnsafeTeamBeforeTools(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("codesign verification is macOS-only")
	}
	called := false
	runCodeSignTool = func(context.Context, string, ...string) ([]byte, error) {
		called = true
		return nil, errors.New("unexpected")
	}
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })

	got := verifyMainAppCodeSignatureContext(context.Background(), nil, "Payload/Demo.app", "Demo", "com.example.demo", parsedProfile{
		embeddedProfile: embeddedProfile{TeamIdentifier: []string{`TEAM" or true`}},
	})
	if got.Status != CodeSignatureInvalid || !strings.Contains(got.Reason, "unsupported characters") || called {
		t.Fatalf("verification=%#v called=%t", got, called)
	}
}

func TestVerifyMainAppCodeSignatureRejectsExpandedExecutableBeforeTools(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("codesign verification is macOS-only")
	}
	member := &zip.File{FileHeader: zip.FileHeader{Name: "Payload/Demo.app/Demo", UncompressedSize64: uint64(maxMainAppExpandedBytes + 1)}}
	called := false
	runCodeSignTool = func(context.Context, string, ...string) ([]byte, error) {
		called = true
		return nil, errors.New("unexpected")
	}
	t.Cleanup(func() { runCodeSignTool = runBoundedTool })
	got := verifyMainAppCodeSignatureContext(context.Background(), []*zip.File{member}, "Payload/Demo.app", "Demo", "com.example.demo", parsedProfile{})
	if got.Status != CodeSignatureInvalid || called {
		t.Fatalf("verification=%#v called=%t", got, called)
	}
}

func validExecutableIPA(t *testing.T) string {
	t.Helper()
	return writeIPA(t, map[string][]byte{
		"Payload/Demo.app/Info.plist": plistBytes(t, map[string]any{
			"CFBundleIdentifier": "com.example.demo", "CFBundleName": "Demo",
			"CFBundleShortVersionString": "1.0", "CFBundleVersion": "1", "CFBundleExecutable": "Demo",
		}),
		"Payload/Demo.app/Demo": fakeMachO("main"),
		"Payload/Demo.app/embedded.mobileprovision": signedProfile(t, profileFixture{
			BundleID: "com.example.demo", Devices: []string{"one"}, Expires: time.Now().Add(time.Hour),
		}),
	})
}

func entitlementTestProfile() parsedProfile {
	return parsedProfile{embeddedProfile: embeddedProfile{
		TeamIdentifier:              []string{"TEAM123"},
		ApplicationIdentifierPrefix: []string{"TEAM123"},
		Entitlements: map[string]any{
			"application-identifier":              "TEAM123.com.example.*",
			"com.apple.developer.team-identifier": "TEAM123",
			"get-task-allow":                      false,
		},
	}}
}

func validMainEntitlements() map[string]any {
	return map[string]any{
		"application-identifier":              "TEAM123.com.example.demo",
		"com.apple.developer.team-identifier": "TEAM123",
	}
}

func requireEntitlementDisplayArgs(t *testing.T, args []string, codePath string) string {
	t.Helper()
	if len(args) != 6 ||
		args[0] != "-d" ||
		args[1] != "-a" ||
		args[3] != "--entitlements" ||
		args[4] != ":-" ||
		args[5] != codePath {
		t.Fatalf("unexpected codesign entitlement args: %#v", args)
	}
	if err := validateArchitecture(args[2]); err != nil {
		t.Fatalf("unvalidated architecture argument %q: %v", args[2], err)
	}
	return args[2]
}

func fakeMachO(payload string) []byte {
	return fakeMachOType(payload, 2)
}

func fakeMachOType(payload string, fileType byte) []byte {
	header := []byte{
		0xcf, 0xfa, 0xed, 0xfe, // MH_MAGIC_64
		0x0c, 0x00, 0x00, 0x01, // CPU_TYPE_ARM64
		0x00, 0x00, 0x00, 0x00, // CPU subtype
		fileType, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, // no load commands
		0x00, 0x00, 0x00, 0x00, // load command size
		0x00, 0x00, 0x00, 0x00, // flags
		0x00, 0x00, 0x00, 0x00, // reserved
	}
	return append(header, []byte(payload)...)
}

func validFatMachO() []byte {
	return fatMachOWithTypes(2)
}

func fatMachOWithTypes(fileTypes ...byte) []byte {
	const (
		architectureHeaderSize = 20
		sliceSize              = 32
	)
	tableEnd := 8 + len(fileTypes)*architectureHeaderSize
	data := make([]byte, tableEnd+len(fileTypes)*sliceSize)
	copy(data[:4], []byte{0xca, 0xfe, 0xba, 0xbe})
	binary.BigEndian.PutUint32(data[4:8], uint32(len(fileTypes)))
	for index, fileType := range fileTypes {
		headerOffset := 8 + index*architectureHeaderSize
		sliceOffset := tableEnd + index*sliceSize
		binary.BigEndian.PutUint32(data[headerOffset:headerOffset+4], 0x0100000c)
		binary.BigEndian.PutUint32(data[headerOffset+8:headerOffset+12], uint32(sliceOffset))
		binary.BigEndian.PutUint32(data[headerOffset+12:headerOffset+16], sliceSize)
		binary.BigEndian.PutUint32(data[headerOffset+16:headerOffset+20], 2)
		copy(data[sliceOffset:sliceOffset+sliceSize], fakeMachOType("", fileType))
	}
	return data
}

func TestValidateExecutableNameRejectsTraversalAndFormatting(t *testing.T) {
	for _, value := range []string{"../Demo", "PlugIns/Demo", "Demo\u202Eipa", strings.Repeat("a", 256)} {
		if err := validateExecutableName(value); err == nil {
			t.Fatalf("validateExecutableName(%q) unexpectedly succeeded", value)
		}
	}
}

func TestEntitlementValuePermitsExactWildcardAndSubsetOnly(t *testing.T) {
	tests := []struct {
		name    string
		profile any
		signed  any
		want    bool
	}{
		{name: "exact", profile: "TEAM.com.example.app", signed: "TEAM.com.example.app", want: true},
		{name: "terminal wildcard", profile: "TEAM.com.example.*", signed: "TEAM.com.example.app", want: true},
		{name: "wildcard needs suffix", profile: "TEAM.com.example.*", signed: "TEAM.com.example.", want: false},
		{name: "mismatched prefix", profile: "TEAM.com.example.*", signed: "OTHER.com.example.app", want: false},
		{name: "signed exact leading whitespace", profile: "TEAM.com.example.app", signed: " TEAM.com.example.app", want: false},
		{name: "signed exact trailing whitespace", profile: "TEAM.com.example.app", signed: "TEAM.com.example.app ", want: false},
		{name: "signed wildcard leading whitespace", profile: "TEAM.com.example.*", signed: " TEAM.com.example.app", want: false},
		{name: "profile exact whitespace is not normalized", profile: " TEAM.com.example.app", signed: "TEAM.com.example.app", want: false},
		{name: "profile wildcard whitespace is not normalized", profile: " TEAM.com.example.*", signed: "TEAM.com.example.app", want: false},
		{name: "list subset", profile: []any{"group.one", "group.two"}, signed: []any{"group.two"}, want: true},
		{name: "list signed whitespace", profile: []any{"group.one"}, signed: []any{" group.one"}, want: false},
		{name: "list extra", profile: []any{"group.one"}, signed: []any{"group.two"}, want: false},
		{name: "bool mismatch", profile: false, signed: true, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := entitlementValuePermits(test.profile, test.signed); got != test.want {
				t.Fatalf("entitlementValuePermits(%#v, %#v) = %t, want %t", test.profile, test.signed, got, test.want)
			}
		})
	}
}
