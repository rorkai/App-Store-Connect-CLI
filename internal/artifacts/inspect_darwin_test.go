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
	manifest, err := InspectPKG(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ProductID != "com.example.artifactfixture.pkg" || manifest.Version != "1.2.3" || manifest.InstallLocation != "/" || manifest.Status != "readable" || len(manifest.BundleIDs) != 1 || manifest.BundleIDs[0] != "com.example.artifactfixture" {
		t.Fatalf("manifest=%+v", manifest)
	}
}
