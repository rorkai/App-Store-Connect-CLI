package signing

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type signingResignBundleIDFixtureTarget struct {
	kind         string
	relativePath string
	info         map[string]any
}

func TestMapSigningResignBundleIDs(t *testing.T) {
	const mainPath = "Payload/App.app"
	main := func(extra map[string]any) signingResignBundleIDFixtureTarget {
		info := map[string]any{
			"CFBundleIdentifier": "com.example.app",
			"CFBundleURLTypes":   []any{map[string]any{"CFBundleURLSchemes": []any{"com.example.app"}}},
		}
		for key, value := range extra {
			info[key] = value
		}
		return signingResignBundleIDFixtureTarget{kind: "application", relativePath: mainPath, info: info}
	}
	nested := func(kind, relativePath, bundleID string, extra map[string]any) signingResignBundleIDFixtureTarget {
		info := map[string]any{"CFBundleIdentifier": bundleID}
		for key, value := range extra {
			info[key] = value
		}
		return signingResignBundleIDFixtureTarget{kind: kind, relativePath: relativePath, info: info}
	}
	extension := nested("app-extension", mainPath+"/PlugIns/Widget.appex", "com.example.app.widget", nil)
	watchApp := func(companion string) signingResignBundleIDFixtureTarget {
		return nested("watch-application", mainPath+"/Watch/Watch.app", "com.example.app.watchkitapp", map[string]any{"WKCompanionAppBundleIdentifier": companion})
	}
	watchExtension := func(watchApp string) signingResignBundleIDFixtureTarget {
		return nested("watch-extension", mainPath+"/Watch/Watch.app/PlugIns/WatchExtension.appex", "com.example.app.watchkitapp.watchkitextension", map[string]any{
			"NSExtension": map[string]any{"NSExtensionAttributes": map[string]any{"WKAppBundleIdentifier": watchApp}},
		})
	}

	for _, test := range []struct {
		name    string
		newID   string
		format  int
		targets []signingResignBundleIDFixtureTarget
		want    map[string]string
		wantErr string
		cancel  bool
	}{
		{
			name:    "cancelled context is not a usage error",
			newID:   "com.example.white",
			targets: []signingResignBundleIDFixtureTarget{main(nil)},
			wantErr: context.Canceled.Error(),
			cancel:  true,
		},
		{
			name:    "app with one extension keeps XML",
			newID:   "com.example.white",
			format:  plist.XMLFormat,
			targets: []signingResignBundleIDFixtureTarget{extension, main(nil)},
			want:    map[string]string{mainPath: "com.example.white", extension.relativePath: "com.example.white.widget"},
		},
		{
			name:    "binary Info.plist stays binary",
			newID:   "com.example.white",
			format:  plist.BinaryFormat,
			targets: []signingResignBundleIDFixtureTarget{extension, main(nil)},
			want:    map[string]string{mainPath: "com.example.white", extension.relativePath: "com.example.white.widget"},
		},
		{
			name:   "watch keys point at the new identifiers",
			newID:  "com.example.white",
			format: plist.XMLFormat,
			targets: []signingResignBundleIDFixtureTarget{
				watchExtension("com.example.app.watchkitapp"),
				watchApp("com.example.app"),
				main(nil),
			},
			want: map[string]string{
				mainPath:                        "com.example.white",
				mainPath + "/Watch/Watch.app":   "com.example.white.watchkitapp",
				watchExtension("").relativePath: "com.example.white.watchkitapp.watchkitextension",
			},
		},
		{
			name:    "watch companion key naming another app",
			newID:   "com.example.white",
			targets: []signingResignBundleIDFixtureTarget{watchApp("com.example.other"), main(nil)},
			wantErr: `WKCompanionAppBundleIdentifier "com.example.other"`,
		},
		{
			name:    "watch extension key naming another app",
			newID:   "com.example.white",
			targets: []signingResignBundleIDFixtureTarget{watchExtension("com.example.app"), watchApp("com.example.app"), main(nil)},
			wantErr: `WKAppBundleIdentifier "com.example.app"`,
		},
		{
			name:    "nested identifier outside the main prefix",
			newID:   "com.example.white",
			targets: []signingResignBundleIDFixtureTarget{nested("app-extension", mainPath+"/PlugIns/Other.appex", "com.vendor.share", nil), main(nil)},
			wantErr: "com.vendor.share",
		},
		{
			name:    "App Clip target",
			newID:   "com.example.white",
			targets: []signingResignBundleIDFixtureTarget{nested("app-clip", mainPath+"/AppClips/Clip.app", "com.example.app.Clip", nil), main(nil)},
			wantErr: "App Clip",
		},
		{
			name:    "same identifier",
			newID:   "COM.example.app",
			targets: []signingResignBundleIDFixtureTarget{main(nil)},
			wantErr: "equals the current bundle identifier",
		},
		{
			name:    "invalid identifier",
			newID:   "com.example.*",
			targets: []signingResignBundleIDFixtureTarget{main(nil)},
			wantErr: "not a valid bundle identifier",
		},
		{
			name:    "new identifier equal to a source-only nested identifier",
			newID:   "com.example.app.widget",
			format:  plist.XMLFormat,
			targets: []signingResignBundleIDFixtureTarget{extension, main(nil)},
			want:    map[string]string{mainPath: "com.example.app.widget", extension.relativePath: "com.example.app.widget.widget"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			archive := signingResignArchive{MainPath: mainPath}
			original := make(map[string][]byte, len(test.targets))
			for _, fixture := range test.targets {
				data, err := plist.Marshal(fixture.info, test.format)
				if err != nil {
					t.Fatal(err)
				}
				directory := filepath.Join(root, filepath.FromSlash(fixture.relativePath))
				if err := os.MkdirAll(directory, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "Info.plist"), data, 0o600); err != nil {
					t.Fatal(err)
				}
				original[fixture.relativePath] = data
				archive.Targets = append(archive.Targets, signingResignTarget{
					Kind: fixture.kind, RelativePath: fixture.relativePath, BundleID: fixture.info["CFBundleIdentifier"].(string),
				})
			}
			tree, err := rootfs.New(root)
			if err != nil {
				t.Fatal(err)
			}
			defer tree.Close()

			ctx, cancel := context.WithCancel(context.Background())
			if test.cancel {
				cancel()
			}
			defer cancel()
			mapped, err := mapSigningResignBundleIDs(ctx, tree, archive, test.newID)
			for relativePath, data := range original {
				if got := mustReadFile(t, filepath.Join(root, filepath.FromSlash(relativePath), "Info.plist")); !bytes.Equal(got, data) {
					t.Fatalf("planning changed %s/Info.plist before the plan was applied", relativePath)
				}
			}
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("mapSigningResignBundleIDs() error = %v, want %q", err, test.wantErr)
				}
				if errors.Is(err, context.Canceled) == isSigningResignUsageError(err) {
					t.Fatalf("mapSigningResignBundleIDs() error = %v: usage=%t, want usage only for refusals", err, isSigningResignUsageError(err))
				}
				return
			}
			if err != nil {
				t.Fatalf("mapSigningResignBundleIDs() error = %v", err)
			}
			for index, target := range mapped.Targets {
				source := archive.Targets[index].BundleID
				if target.BundleID != test.want[target.RelativePath] || target.SourceBundleID != source {
					t.Fatalf("target %s mapped %s -> %s, want %s -> %s", target.RelativePath, target.SourceBundleID, target.BundleID, source, test.want[target.RelativePath])
				}
				var info map[string]any
				format, err := plist.Unmarshal(target.InfoPlistData, &info)
				if err != nil {
					t.Fatal(err)
				}
				if format != test.format {
					t.Fatalf("target %s Info.plist format = %d, want %d", target.RelativePath, format, test.format)
				}
				if info["CFBundleIdentifier"] != target.BundleID {
					t.Fatalf("target %s CFBundleIdentifier = %v, want %s", target.RelativePath, info["CFBundleIdentifier"], target.BundleID)
				}
				if companion, ok := info["WKCompanionAppBundleIdentifier"]; ok && companion != "com.example.white" {
					t.Fatalf("WKCompanionAppBundleIdentifier = %v, want the new main identifier", companion)
				}
				if attributes, ok := info["NSExtension"].(map[string]any); ok {
					if got := attributes["NSExtensionAttributes"].(map[string]any)["WKAppBundleIdentifier"]; got != "com.example.white.watchkitapp" {
						t.Fatalf("WKAppBundleIdentifier = %v, want the new watch app identifier", got)
					}
				}
				if schemes, ok := info["CFBundleURLTypes"]; ok && !strings.Contains(signingResignRewriteValueSortKey(schemes), `"com.example.app"`) {
					t.Fatalf("URL schemes changed: %v", schemes)
				}
			}
		})
	}
}

// TestSigningResignBundleIDAdHocRoundTrip ad-hoc signs a planned tree on macOS.
// It proves the local signing identifier and claims; it does not prove device
// install or distribution behavior.
func TestSigningResignBundleIDAdHocRoundTrip(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("codesign fixture is available only on macOS")
	}
	if _, err := os.Stat("/usr/bin/codesign"); err != nil {
		t.Skip("codesign is unavailable")
	}
	stagePath := t.TempDir()
	treePath := filepath.Join(stagePath, "tree")
	mainPath := filepath.Join(treePath, "Payload", "App.app")
	extensionPath := filepath.Join(mainPath, "PlugIns", "Widget.appex")
	writeSigningResignAppClipFixtureBundle(t, mainPath, "App", "com.example.app")
	writeSigningResignAppClipFixtureBundle(t, extensionPath, "Widget", "com.example.app.widget")
	for _, bundle := range []struct{ path, bundleID string }{{extensionPath, "com.example.app.widget"}, {mainPath, "com.example.app"}} {
		entitlementsPath := filepath.Join(stagePath, filepath.Base(bundle.path)+".entitlements")
		writeSigningResignFixturePlist(t, entitlementsPath, map[string]any{
			"application-identifier":               "OLDPREFIX." + bundle.bundleID,
			"com.apple.developer.team-identifier":  "OLDTEAM",
			"get-task-allow":                       false,
			signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.shared"},
		})
		signSigningResignAdHocFixture(t, bundle.path, entitlementsPath)
	}

	stageRoot, err := rootfs.New(stagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stageRoot.Close()
	tree, err := rootfs.New(treePath)
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()
	archive := signingResignArchive{MainPath: "Payload/App.app"}
	for _, target := range []struct{ relativePath, kind string }{{"Payload/App.app/PlugIns/Widget.appex", "app-extension"}, {"Payload/App.app", "application"}} {
		inspected, err := inspectSigningResignTargetWithEntitlements(context.Background(), tree, target.relativePath, target.kind, true)
		if err != nil {
			t.Fatal(err)
		}
		archive.Targets = append(archive.Targets, inspected)
	}
	mapped, err := mapSigningResignBundleIDs(context.Background(), tree, archive, "com.example.white")
	if err != nil {
		t.Fatalf("mapSigningResignBundleIDs() error = %v", err)
	}
	keychain := map[string]any{signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*"}}
	profiles := map[string]signingResignProfile{
		"com.example.white":        rebaseTestProfile("com.example.white", "NEWPREFIX", keychain),
		"com.example.white.widget": rebaseTestProfile("com.example.white.widget", "NEWPREFIX", keychain),
	}
	for bundleID, profile := range profiles {
		profile.Data = []byte("profile " + bundleID)
		profiles[bundleID] = profile
	}
	prepared, err := prepareSigningResignTree(context.Background(), stageRoot, tree, mapped, profiles, true)
	if err != nil {
		t.Fatalf("prepareSigningResignTree() error = %v", err)
	}
	for _, target := range prepared.Archive.Targets {
		signSigningResignAdHocFixture(t, filepath.Join(treePath, filepath.FromSlash(target.RelativePath)), target.EntitlementsPath)
	}
	verifySigningResignAdHocFixture(t, mainPath)

	for _, want := range prepared.Archive.Targets {
		output, err := exec.Command("/usr/bin/codesign", "-d", "--verbose=2", filepath.Join(treePath, filepath.FromSlash(want.RelativePath))).CombinedOutput()
		if err != nil {
			t.Fatalf("codesign -d %s: %v: %s", want.RelativePath, err, output)
		}
		if !strings.Contains(string(output), "Identifier="+want.BundleID+"\n") {
			t.Fatalf("signing identifier for %s is not %s:\n%s", want.RelativePath, want.BundleID, output)
		}
		rediscovered, err := inspectSigningResignTargetWithEntitlements(context.Background(), tree, want.RelativePath, want.Kind, true)
		if err != nil {
			t.Fatalf("rediscover %s: %v", want.RelativePath, err)
		}
		if rediscovered.BundleID != want.BundleID {
			t.Fatalf("rediscovered bundle identifier = %s, want %s", rediscovered.BundleID, want.BundleID)
		}
		if got := rediscovered.ExistingEntitlements["application-identifier"]; got != "NEWPREFIX."+want.BundleID {
			t.Fatalf("signed application-identifier = %v, want the new profile's identifier", got)
		}
		if got := rediscovered.ExistingEntitlements[signingResignKeychainGroupsEntitlement]; !signingResignEntitlementValuesEqual(got, []string{"NEWPREFIX.shared"}) {
			t.Fatalf("signed keychain groups = %#v, want the rebased group", got)
		}
	}
}
