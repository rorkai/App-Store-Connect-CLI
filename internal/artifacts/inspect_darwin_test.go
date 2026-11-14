package artifacts

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInspectPKGNativeComponentPackage(t *testing.T) {
	pkgbuild, err := exec.LookPath("pkgbuild")
	if err != nil {
		t.Skip("macOS pkgbuild is not installed")
	}
	root := filepath.Join(t.TempDir(), "root")
	contents := filepath.Join(root, "Applications", "Demo.app", "Contents")
	if err := os.MkdirAll(contents, 0o755); err != nil {
		t.Fatal(err)
	}
	info := plistXML(t, map[string]any{
		"CFBundleIdentifier":         "com.example.artifactfixture",
		"CFBundleName":               "Demo",
		"CFBundleShortVersionString": "1.2.3",
		"CFBundleVersion":            "9",
		"CFBundlePackageType":        "APPL",
	})
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), info, 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(t.TempDir(), "Demo.pkg")
	cmd := exec.Command(pkgbuild, "--root", root, "--identifier", "com.example.artifactfixture.pkg", "--version", "1.2.3", "--install-location", "/", pkg)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pkgbuild: %v: %s", err, output)
	}
	data, err := os.ReadFile(pkg)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := inspectPKG(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ProductID != "com.example.artifactfixture.pkg" || manifest.Version != "1.2.3" || manifest.InstallLocation != "/" || manifest.Status != "readable" || len(manifest.BundleIDs) != 1 || manifest.BundleIDs[0] != "com.example.artifactfixture" {
		t.Fatalf("manifest=%+v", manifest)
	}
}

func TestInspectPKGNativeProductArchive(t *testing.T) {
	productbuild, err := exec.LookPath("productbuild")
	if err != nil {
		t.Skip("macOS productbuild is not installed")
	}
	app := filepath.Join(t.TempDir(), "Demo.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	info := plistXML(t, map[string]any{
		"CFBundleIdentifier":         "com.example.productfixture",
		"CFBundleName":               "Demo",
		"CFBundleExecutable":         "Demo",
		"CFBundleShortVersionString": "2.3.4",
		"CFBundleVersion":            "56",
		"CFBundlePackageType":        "APPL",
		"LSMinimumSystemVersion":     "13.0",
		"CFBundleSupportedPlatforms": []any{"MacOSX"},
	})
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), info, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "MacOS", "Demo"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(t.TempDir(), "Demo.pkg")
	if output, err := exec.Command(productbuild, "--component", app, "/Applications", pkg).CombinedOutput(); err != nil {
		t.Fatalf("productbuild: %v: %s", err, output)
	}
	data, err := os.ReadFile(pkg)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := inspectPKG(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatalf("manifest=%+v err=%v", manifest, err)
	}
	if manifest.Status != "readable" || manifest.ProductID != "com.example.productfixture" || manifest.Version != "2.3.4" || manifest.BundleID != "com.example.productfixture" || manifest.BuildNumber != "56" || manifest.MinimumOSVersion != "13.0" || manifest.InstallLocation != "/Applications" || len(manifest.Warnings) != 0 {
		t.Fatalf("manifest=%+v", manifest)
	}
	if len(manifest.Components) != 1 || manifest.Components[0].App == nil || manifest.Components[0].App.Name != "Demo" || !manifest.Components[0].Primary {
		t.Fatalf("components=%+v", manifest.Components)
	}

	// Cross-check the component layout and metadata with Apple's tools.
	if xar, err := exec.LookPath("xar"); err == nil {
		listing, err := exec.Command(xar, "-tf", pkg).Output()
		if err != nil {
			t.Fatalf("xar -tf: %v", err)
		}
		for _, member := range []string{"Distribution", manifest.Components[0].Path + "/PackageInfo", manifest.Components[0].Path + "/Payload"} {
			if !bytes.Contains(listing, []byte(member+"\n")) {
				t.Fatalf("xar listing lacks %s:\n%s", member, listing)
			}
		}
	}
	pkgutil, err := exec.LookPath("pkgutil")
	if err != nil {
		return
	}
	expanded := filepath.Join(t.TempDir(), "expanded")
	if output, err := exec.Command(pkgutil, "--expand", pkg, expanded).CombinedOutput(); err != nil {
		t.Fatalf("pkgutil --expand: %v: %s", err, output)
	}
	packageInfo, err := os.ReadFile(filepath.Join(expanded, manifest.Components[0].Path, "PackageInfo"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := decodePackageInfo(packageInfo)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Components[0].Identifier != expected.Identifier || manifest.Components[0].Version != expected.Version || manifest.Components[0].InstallLocation != expected.InstallLocation {
		t.Fatalf("component=%+v pkgutil PackageInfo=%+v", manifest.Components[0], expected)
	}
}
