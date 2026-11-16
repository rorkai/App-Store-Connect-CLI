package cmdtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXcodeVersionMutationWarnsAboutLiteralInfoPlist(t *testing.T) {
	for _, operation := range []string{"edit", "bump"} {
		t.Run(operation, func(t *testing.T) {
			rootDir := t.TempDir()
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(rootDir, "config.json"))
			project := filepath.Join(rootDir, "Demo.xcodeproj")
			if err := os.Mkdir(project, 0o700); err != nil {
				t.Fatal(err)
			}
			pbx := `// !$*UTF8*$!
 { archiveVersion = 1; classes = {}; objectVersion = 77; objects = {
  AAAAAAAAAAAAAAAAAAAAAAAA = {isa = PBXProject; attributes = {}; buildConfigurationList = BBBBBBBBBBBBBBBBBBBBBBBB; targets = (CCCCCCCCCCCCCCCCCCCCCCCC); };
  BBBBBBBBBBBBBBBBBBBBBBBB = {isa = XCConfigurationList; buildConfigurations = (DDDDDDDDDDDDDDDDDDDDDDDD); defaultConfigurationName = Release; };
  CCCCCCCCCCCCCCCCCCCCCCCC = {isa = PBXNativeTarget; name = App; productName = App; productType = "com.apple.product-type.application"; buildConfigurationList = EEEEEEEEEEEEEEEEEEEEEEEE; buildPhases = (); dependencies = (); };
  DDDDDDDDDDDDDDDDDDDDDDDD = {isa = XCBuildConfiguration; name = Release; buildSettings = {}; };
  EEEEEEEEEEEEEEEEEEEEEEEE = {isa = XCConfigurationList; buildConfigurations = (FFFFFFFFFFFFFFFFFFFFFFFF); defaultConfigurationName = Release; };
  FFFFFFFFFFFFFFFFFFFFFFFF = {isa = XCBuildConfiguration; name = Release; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; INFOPLIST_FILE = Info.plist; }; };
 }; rootObject = AAAAAAAAAAAAAAAAAAAAAAAA; }`
			if err := os.WriteFile(filepath.Join(project, "project.pbxproj"), []byte(pbx), 0o600); err != nil {
				t.Fatal(err)
			}
			plistPath := filepath.Join(rootDir, "Info.plist")
			contents := `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleVersion</key><string>42</string></dict></plist>`
			if err := os.WriteFile(plistPath, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"xcode", "version", operation, "--project", project, "--target", "App", "--configuration", "Release", "--output", "json"}
			if operation == "edit" {
				args = append(args, "--build-number", "50")
			} else {
				args = append(args, "--type", "build")
			}
			stdout, stderr, err := runRootWithDeps(t, nil, args...)
			if err != nil {
				t.Fatal(err)
			}
			var receipt struct {
				BuildNumber      string   `json:"buildNumber"`
				NewBuild         string   `json:"newBuild"`
				ComputedWarnings []string `json:"computedWarnings"`
			}
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatal(err)
			}
			if len(receipt.ComputedWarnings) != 1 {
				t.Fatalf("receipt=%s", stdout)
			}
			warning := receipt.ComputedWarnings[0]
			if !strings.Contains(stderr, "Warning: "+warning+"\n") {
				t.Fatalf("stderr=%q missing receipt warning %q", stderr, warning)
			}
			for _, expected := range []string{`target "App"`, `configuration "Release"`, `CFBundleVersion="42"`, "CURRENT_PROJECT_VERSION", plistPath} {
				if !strings.Contains(warning, expected) {
					t.Errorf("warning=%q missing %q", warning, expected)
				}
			}
			if operation == "edit" && receipt.BuildNumber != "50" || operation == "bump" && receipt.NewBuild != "43" {
				t.Fatalf("receipt=%s", stdout)
			}
			after, err := os.ReadFile(plistPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != contents {
				t.Fatal("source Info.plist changed")
			}
		})
	}
}
