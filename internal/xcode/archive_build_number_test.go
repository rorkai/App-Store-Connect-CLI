package xcode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadArchiveBuildNumberFloorSelectsAppConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, target, configuration, want, wantErr string
		xcconfig, ambiguous, unresolved            bool
		args                                       []string
	}{
		{name: "inferred app Release", configuration: "Release", want: "42"},
		{name: "explicit app Debug", target: "App", configuration: "Debug", want: "7"},
		{name: "referenced xcconfig", configuration: "Release", xcconfig: true, want: "42"},
		{name: "extension is not app", target: "Widget", configuration: "Release", wantErr: "is not an application"},
		{name: "ambiguous applications", configuration: "Release", ambiguous: true, wantErr: "use --target"},
		{name: "explicit among applications", target: "App", configuration: "Release", ambiguous: true, want: "42"},
		{name: "missing configuration", target: "App", configuration: "Missing", wantErr: "configuration \"Missing\" not found"},
		{name: "unresolved bundle", configuration: "Release", unresolved: true, wantErr: "UNKNOWN_BUNDLE"},
		{name: "raw configuration", args: []string{"-configuration", "Debug"}, wantErr: "cannot override -configuration"},
		{name: "raw build number", args: []string{"CURRENT_PROJECT_VERSION=1"}, wantErr: "cannot override CURRENT_PROJECT_VERSION"},
		{name: "raw xcconfig", args: []string{"-xcconfig", "Other.xcconfig"}, wantErr: "cannot override -xcconfig"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, tc.xcconfig)
			path := filepath.Join(project, "project.pbxproj")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			content := strings.ReplaceAll(string(data), "MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42;", "MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; PRODUCT_BUNDLE_IDENTIFIER = com.example.app;")
			content = strings.Replace(content, "/* App Debug */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42;", "/* App Debug */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 7;", 1)
			content = strings.ReplaceAll(content, "/* Widget Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42;", "/* Widget Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 900;")
			if tc.ambiguous {
				content = strings.ReplaceAll(content, "com.apple.product-type.app-extension", "com.apple.product-type.application")
				content = strings.ReplaceAll(content, "\"wrapper.app-extension\"", "wrapper.application")
				content = strings.ReplaceAll(content, "Widget.appex", "Widget.app")
			}
			if tc.unresolved {
				content = strings.ReplaceAll(content, "com.example.app", "\"$(UNKNOWN_BUNDLE)\"")
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.xcconfig {
				path := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(data, []byte("PRODUCT_BUNDLE_IDENTIFIER = com.example.app\n")...), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := ReadArchiveBuildNumberFloor(project, tc.target, tc.configuration, tc.args)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Target != "App" || got.Configuration != tc.configuration || got.BundleID != "com.example.app" || got.BuildNumber != tc.want {
				t.Fatalf("floor = %+v, want App/%s/com.example.app/%s", got, tc.configuration, tc.want)
			}
		})
	}
}
