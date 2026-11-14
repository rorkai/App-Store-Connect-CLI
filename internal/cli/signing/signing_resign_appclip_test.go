package signing

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// TestSigningResignAppClipClaimsSurvivePostSignNestedFixture is a macOS
// ad-hoc-signing canary for planned claims in nested bundles. It proves the
// local entitlement boundary; it does not prove iOS-device or distribution
// signing behavior.
func TestSigningResignAppClipClaimsSurvivePostSignNestedFixture(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("codesign fixture is available only on macOS")
	}
	if _, err := os.Stat("/usr/bin/codesign"); err != nil {
		t.Skip("codesign is unavailable")
	}

	root := t.TempDir()
	mainPath := filepath.Join(root, "Payload", "App.app")
	clipPath := filepath.Join(mainPath, "AppClips", "Clip.app")
	writeSigningResignAppClipFixtureBundle(t, mainPath, "App", "com.example.app")
	writeSigningResignAppClipFixtureBundle(t, clipPath, "Clip", "com.example.app.Clip")

	mainEntitlements := map[string]any{
		"application-identifier":              "OLDPREFIX.com.example.app",
		"com.apple.application-identifier":    "OLDPREFIX.com.example.app",
		"com.apple.developer.team-identifier": "OLDTEAM",
		"get-task-allow":                      false,
		signingResignAssociatedAppClipEntitlement: []string{
			"OLDPREFIX.com.example.app.Clip",
		},
	}
	clipEntitlements := map[string]any{
		"application-identifier":              "OLDPREFIX.com.example.app.Clip",
		"com.apple.application-identifier":    "OLDPREFIX.com.example.app.Clip",
		"com.apple.developer.team-identifier": "OLDTEAM",
		"get-task-allow":                      false,
		signingResignParentEntitlement: []string{
			"OLDPREFIX.com.example.app",
		},
	}
	mainEntitlementsPath := filepath.Join(root, "source-main.entitlements.plist")
	clipEntitlementsPath := filepath.Join(root, "source-clip.entitlements.plist")
	writeSigningResignFixturePlist(t, mainEntitlementsPath, mainEntitlements)
	writeSigningResignFixturePlist(t, clipEntitlementsPath, clipEntitlements)

	// Sign the nested target before its enclosing app, matching the production
	// leaf-first order and making the source archive a genuinely signed pair.
	signSigningResignAdHocFixture(t, clipPath, clipEntitlementsPath)
	signSigningResignAdHocFixture(t, mainPath, mainEntitlementsPath)
	verifySigningResignAdHocFixture(t, mainPath)

	tree, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()

	mainTarget, err := inspectSigningResignTargetWithEntitlements(context.Background(), tree, "Payload/App.app", "application", true)
	if err != nil {
		t.Fatalf("inspect signed main target: %v", err)
	}
	clipTarget, err := inspectSigningResignTargetWithEntitlements(context.Background(), tree, "Payload/App.app/AppClips/Clip.app", "app-clip", true)
	if err != nil {
		t.Fatalf("inspect signed App Clip target: %v", err)
	}
	if got := mainTarget.ExistingEntitlements[signingResignAssociatedAppClipEntitlement]; !signingResignEntitlementValuesEqual(got, mainEntitlements[signingResignAssociatedAppClipEntitlement]) {
		t.Fatalf("signed main associated claim = %#v, want %#v", got, mainEntitlements[signingResignAssociatedAppClipEntitlement])
	}
	if got := clipTarget.ExistingEntitlements[signingResignParentEntitlement]; !signingResignEntitlementValuesEqual(got, clipEntitlements[signingResignParentEntitlement]) {
		t.Fatalf("signed App Clip parent claim = %#v, want %#v", got, clipEntitlements[signingResignParentEntitlement])
	}

	archive := signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{mainTarget, clipTarget},
	}
	profiles := map[string]signingResignProfile{
		mainTarget.BundleID: rebaseTestProfile(mainTarget.BundleID, "NEWMAIN", map[string]any{
			signingResignAssociatedAppClipEntitlement: []any{"NEWCLIP.com.example.app.Clip"},
		}),
		clipTarget.BundleID: rebaseTestProfile(clipTarget.BundleID, "NEWCLIP", map[string]any{
			signingResignParentEntitlement: []any{"NEWMAIN.com.example.app"},
		}),
	}
	plans, err := planSigningResignEntitlements(archive, profiles, true)
	if err != nil {
		t.Fatalf("plan signed App Clip relationship rebasing: %v", err)
	}
	if got := plans[0].Entitlements[signingResignAssociatedAppClipEntitlement]; !signingResignEntitlementValuesEqual(got, []string{"NEWCLIP.com.example.app.Clip"}) {
		t.Fatalf("planned main associated claim = %#v, want new App Clip identifier", got)
	}
	if got := plans[1].Entitlements[signingResignParentEntitlement]; !signingResignEntitlementValuesEqual(got, []string{"NEWMAIN.com.example.app"}) {
		t.Fatalf("planned App Clip parent claim = %#v, want new main identifier", got)
	}

	postSignMainEntitlementsPath := filepath.Join(root, "post-sign-main.entitlements.plist")
	postSignClipEntitlementsPath := filepath.Join(root, "post-sign-clip.entitlements.plist")
	writeSigningResignFixturePlist(t, postSignMainEntitlementsPath, plans[0].Entitlements)
	writeSigningResignFixturePlist(t, postSignClipEntitlementsPath, plans[1].Entitlements)
	signSigningResignAdHocFixture(t, clipPath, postSignClipEntitlementsPath)
	signSigningResignAdHocFixture(t, mainPath, postSignMainEntitlementsPath)
	verifySigningResignAdHocFixture(t, mainPath)

	mainPostSign, err := readSigningResignEntitlements(context.Background(), filepath.Join(mainPath, "App"))
	if err != nil {
		t.Fatalf("read post-sign main entitlements: %v", err)
	}
	clipPostSign, err := readSigningResignEntitlements(context.Background(), filepath.Join(clipPath, "Clip"))
	if err != nil {
		t.Fatalf("read post-sign App Clip entitlements: %v", err)
	}
	if got := mainPostSign[signingResignAssociatedAppClipEntitlement]; !signingResignEntitlementValuesEqual(got, []string{"NEWCLIP.com.example.app.Clip"}) {
		t.Fatalf("post-sign main associated claim = %#v, want paired destination claim", got)
	}
	if got := clipPostSign[signingResignParentEntitlement]; !signingResignEntitlementValuesEqual(got, []string{"NEWMAIN.com.example.app"}) {
		t.Fatalf("post-sign App Clip parent claim = %#v, want paired destination claim", got)
	}
}

func writeSigningResignAppClipFixtureBundle(t *testing.T, bundlePath, executable, bundleID string) {
	t.Helper()
	if err := os.MkdirAll(bundlePath, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	executablePath := filepath.Join(bundlePath, executable)
	if err := os.WriteFile(executablePath, data, 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         bundleID,
		"CFBundleExecutable":         executable,
		"CFBundlePackageType":        "APPL",
		"CFBundleShortVersionString": "1.0",
		"CFBundleVersion":            "1",
		"DTPlatformName":             "iphoneos",
		"CFBundleSupportedPlatforms": []string{"iPhoneOS"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundlePath, "Info.plist"), info, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeSigningResignFixturePlist(t *testing.T, pathValue string, value map[string]any) {
	t.Helper()
	data, err := plist.Marshal(value, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathValue, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func signSigningResignAdHocFixture(t *testing.T, bundlePath, entitlementsPath string) {
	t.Helper()
	command := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", "--entitlements", entitlementsPath, bundlePath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("ad-hoc sign %s: %v: %s", filepath.Base(bundlePath), err, strings.TrimSpace(string(output)))
	}
}

func verifySigningResignAdHocFixture(t *testing.T, bundlePath string) {
	t.Helper()
	command := exec.Command("/usr/bin/codesign", "--verify", "--strict", "--deep", bundlePath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("verify ad-hoc signature %s: %v: %s", filepath.Base(bundlePath), err, strings.TrimSpace(string(output)))
	}
}
