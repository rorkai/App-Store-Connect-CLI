package xcode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// syntheticXCProj is hand-written, not Xcode output. Its shape follows Apple's
// xcode-project-format documentation: a JSON5 file at
// <name>.xcodeproj/project.xcproj, flat build settings with [config=...]
// conditions, and xcconfig references through the groups and files tree.
const syntheticXCProj = `// Synthetic fixture.
{
  "default-configuration": "Release",
  "configurations": [
    { "name": "Debug", "file": "Config/Project.xcconfig" },
    "Release",
  ],
  "localizations": { "development": "en" },
  "files": [
    { "kind": "group", "path": "Config", "children": [ { "path": "Project.xcconfig" } ] },
    { "kind": "group", "name": "Products", "children": [ { "path": "<PRODUCTS>/App.app" } ] },
  ],
  "targets": [
    {
      "name": "App",
      "id": "AAAAAAAAAAAAAAAAAAAAAAAA",
      "product": "Products/App.app",
      "product-type": "application",
      "specialized-configurations": [
        { "name": "Release", "file": { "anchor": "Config", "relative-path": "Nested/App.xcconfig" } },
      ],
      "build-settings": {
        "CURRENT_PROJECT_VERSION[config=Debug]": "7",
        "CURRENT_PROJECT_VERSION[config=Release]": "8",
        "MARKETING_VERSION": "1.2.3", /* keep me */
        "PRODUCT_BUNDLE_IDENTIFIER": 'com.example.app',
      },
    },
    {
      "name": "Widget",
      "id": "BBBBBBBBBBBBBBBBBBBBBBBB",
      "product-type": "app-extension",
      "build-settings": {
        "PRODUCT_NAME": "$(TARGET_NAME)",
      },
    },
  ],
  "build-settings": {
    "CURRENT_PROJECT_VERSION": "3",
    "MARKETING_VERSION": "1.0",
    "SDKROOT": "iphoneos",
  },
}
`

func writeXCProjProject(t *testing.T, contents string) (string, string) {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "Demo.xcodeproj")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "project.xcproj")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Config", "Nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Config", "Project.xcconfig"), []byte("PROJECT_ONLY = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Config", "Nested", "App.xcconfig"), []byte("APP_ONLY = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return project, path
}

func TestXCProjVersionView(t *testing.T) {
	for _, test := range []struct {
		name          string
		contents      string
		target        string
		configuration string
		wantVersion   string
		wantBuild     string
	}{
		{name: "default configuration and app target", contents: syntheticXCProj, wantVersion: "1.2.3", wantBuild: "8"},
		{name: "configuration condition", contents: syntheticXCProj, configuration: "Debug", wantVersion: "1.2.3", wantBuild: "7"},
		{name: "project settings are inherited", contents: syntheticXCProj, target: "Widget", wantVersion: "1.0", wantBuild: "3"},
		{
			name: "json5 syntax",
			contents: `{
  default-configuration: 'Release', // unquoted key, single quotes
  configurations: ['Release'],
  files: [],
  targets: [{ name: 'App', id: 'A', 'product-type': "application", 'build-settings': {
    MARKETING_VERSION: '2.0',
    CURRENT_PROJECT_VERSION: "9",
  } }],
}`,
			wantVersion: "2.0",
			wantBuild:   "9",
		},
		{
			name: "configuration wildcard",
			contents: `{ "default-configuration": "Release", "configurations": ["Debug", "Release"], "files": [],
  "targets": [{ "name": "App", "id": "A", "product-type": "application", "build-settings": {
    "CURRENT_PROJECT_VERSION": "1",
    "MARKETING_VERSION[config=De*]": "wild",
    "MARKETING_VERSION[config=Rel*]": "rel",
  } }] }`,
			configuration: "Debug",
			wantVersion:   "wild",
			wantBuild:     "1",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, path := writeXCProjProject(t, test.contents)
			info, err := GetVersionScoped(t.Context(), GetVersionOptions{
				ProjectDir:          project,
				Target:              test.target,
				Configuration:       test.configuration,
				BuildSettingsLookup: BuildSettingsLookupNever,
			})
			if err != nil {
				t.Fatal(err)
			}
			if info.Version != test.wantVersion || info.BuildNumber != test.wantBuild {
				t.Fatalf("got version %q build %q, want %q and %q", info.Version, info.BuildNumber, test.wantVersion, test.wantBuild)
			}
			if test.name == "default configuration and app target" && info.VersionSource != path {
				t.Fatalf("VersionSource = %q, want %q", info.VersionSource, path)
			}
		})
	}
}

func TestXCProjSetVersionPreservesUnrelatedContent(t *testing.T) {
	for _, test := range []struct {
		name    string
		opts    SetVersionOptions
		replace [][2]string
	}{
		{
			name: "shared setting across every configuration",
			opts: SetVersionOptions{Version: "2.0.0"},
			replace: [][2]string{
				{`"MARKETING_VERSION": "1.2.3", /* keep me */`, `"MARKETING_VERSION": "2.0.0", /* keep me */`},
				{`"MARKETING_VERSION": "1.0",`, `"MARKETING_VERSION": "2.0.0",`},
			},
		},
		{
			name:    "configuration condition stays conditional",
			opts:    SetVersionOptions{BuildNumber: "20", Target: "App", Configuration: "Release"},
			replace: [][2]string{{`"CURRENT_PROJECT_VERSION[config=Release]": "8"`, `"CURRENT_PROJECT_VERSION[config=Release]": "20"`}},
		},
		{
			name: "shared setting splits for one configuration",
			opts: SetVersionOptions{Version: "2.0.0", Target: "App", Configuration: "Release"},
			replace: [][2]string{{
				`"MARKETING_VERSION": "1.2.3", /* keep me */`,
				"\"MARKETING_VERSION[config=Debug]\": \"1.2.3\",\n        \"MARKETING_VERSION[config=Release]\": \"2.0.0\", /* keep me */",
			}},
		},
		{
			name: "inherited setting is added to one target",
			opts: SetVersionOptions{Version: "2.0.0", Target: "Widget"},
			replace: [][2]string{{
				"\"PRODUCT_NAME\": \"$(TARGET_NAME)\",\n",
				"\"MARKETING_VERSION\": \"2.0.0\",\n        \"PRODUCT_NAME\": \"$(TARGET_NAME)\",\n",
			}},
		},
		{
			name: "inherited setting is added to one configuration",
			opts: SetVersionOptions{BuildNumber: "20", Target: "Widget", Configuration: "Debug"},
			replace: [][2]string{{
				"\"PRODUCT_NAME\": \"$(TARGET_NAME)\",\n",
				"\"CURRENT_PROJECT_VERSION[config=Debug]\": \"20\",\n        \"PRODUCT_NAME\": \"$(TARGET_NAME)\",\n",
			}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, path := writeXCProjProject(t, syntheticXCProj)
			test.opts.ProjectDir = project
			want := syntheticXCProj
			for _, replacement := range test.replace {
				if !strings.Contains(want, replacement[0]) {
					t.Fatalf("fixture has no %q", replacement[0])
				}
				want = strings.Replace(want, replacement[0], replacement[1], 1)
			}
			result, err := SetVersion(t.Context(), test.opts)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != want {
				t.Fatalf("project.xcproj =\n%s\nwant\n%s", got, want)
			}
			if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != path {
				t.Fatalf("ChangedFiles = %v, want %s", result.ChangedFiles, path)
			}
			for _, change := range result.Changes {
				if change.Source != "xcproj" || change.Path != path {
					t.Fatalf("change = %+v", change)
				}
			}
			if _, err := os.Stat(filepath.Join(project, "project.pbxproj")); !os.IsNotExist(err) {
				t.Fatalf("project.pbxproj must not appear: %v", err)
			}
		})
	}
}

func TestXCProjSplitValueIsReadBackPerConfiguration(t *testing.T) {
	project, _ := writeXCProjProject(t, syntheticXCProj)
	if _, err := SetVersion(t.Context(), SetVersionOptions{ProjectDir: project, Version: "2.0.0", Target: "App", Configuration: "Release"}); err != nil {
		t.Fatal(err)
	}
	for configuration, want := range map[string]string{"Debug": "1.2.3", "Release": "2.0.0"} {
		info, err := GetVersionScoped(t.Context(), GetVersionOptions{ProjectDir: project, Target: "App", Configuration: configuration, BuildSettingsLookup: BuildSettingsLookupNever})
		if err != nil {
			t.Fatal(err)
		}
		if info.Version != want {
			t.Fatalf("%s version = %q, want %q", configuration, info.Version, want)
		}
	}
}

func TestXCProjBumpAcrossSelectedConfigurations(t *testing.T) {
	project, path := writeXCProjProject(t, syntheticXCProj)
	result, err := BumpVersion(t.Context(), BumpVersionOptions{ProjectDir: project, Target: "App", BumpType: BumpMinor})
	if err != nil {
		t.Fatal(err)
	}
	if result.OldVersion != "1.2.3" || result.NewVersion != "1.3.0" {
		t.Fatalf("result = %+v", result)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(syntheticXCProj, `"MARKETING_VERSION": "1.2.3"`, `"MARKETING_VERSION": "1.3.0"`, 1)
	if string(got) != want {
		t.Fatalf("project.xcproj =\n%s\nwant\n%s", got, want)
	}
}

func TestXCProjRefusesWithoutChangingFiles(t *testing.T) {
	unresolved := strings.NewReplacer(`"MARKETING_VERSION": "1.2.3", /* keep me */`, "", `"MARKETING_VERSION": "1.0",`, "").Replace(syntheticXCProj)
	wildcard := strings.Replace(syntheticXCProj, `"MARKETING_VERSION": "1.2.3", /* keep me */`, `"MARKETING_VERSION[config=*]": "1.2.3",`, 1)
	for _, test := range []struct {
		name     string
		contents string
		run      func(context.Context, string) error
		wantErr  string
	}{
		{
			name:     "unknown required capability",
			contents: strings.Replace(syntheticXCProj, `"default-configuration"`, `"required-capabilities": ["glow in the dark files"], "default-configuration"`, 1),
			run:      setVersion20,
			wantErr:  "glow in the dark files",
		},
		{name: "invalid json5", contents: `{ "targets": [ }`, run: setVersion20, wantErr: "project.xcproj"},
		{
			name:     "duplicate key",
			contents: strings.Replace(syntheticXCProj, `"SDKROOT": "iphoneos",`, `"SDKROOT": "iphoneos", "SDKROOT": "macosx",`, 1),
			run:      setVersion20,
			wantErr:  "duplicate key",
		},
		{
			name:     "agvtool edit would create an empty pbxproj",
			contents: unresolved,
			run:      setVersion20,
			wantErr:  "agvtool",
		},
		{
			name:     "agvtool validate",
			contents: unresolved,
			run: func(ctx context.Context, project string) error {
				return ValidateSetVersion(ctx, SetVersionOptions{ProjectDir: project, Version: "2.0"})
			},
			wantErr: "agvtool",
		},
		{
			name:     "agvtool bump",
			contents: unresolved,
			run: func(ctx context.Context, project string) error {
				_, err := BumpVersion(ctx, BumpVersionOptions{ProjectDir: project, BumpType: BumpMinor})
				return err
			},
			wantErr: "agvtool",
		},
		{
			name:     "agvtool bump validate",
			contents: unresolved,
			run: func(ctx context.Context, project string) error {
				return ValidateBumpVersion(ctx, BumpVersionOptions{ProjectDir: project, BumpType: BumpMinor})
			},
			wantErr: "agvtool",
		},
		{
			name:     "wildcard shared by configurations",
			contents: wildcard,
			run: func(ctx context.Context, project string) error {
				_, err := SetVersion(ctx, SetVersionOptions{ProjectDir: project, Version: "2.0.0", Target: "App", Configuration: "Release"})
				return err
			},
			wantErr: "[config=*]",
		},
		{
			name:     "target without build settings",
			contents: strings.Replace(syntheticXCProj, "\"build-settings\": {\n        \"PRODUCT_NAME\": \"$(TARGET_NAME)\",\n      },", "", 1),
			run: func(ctx context.Context, project string) error {
				_, err := SetVersion(ctx, SetVersionOptions{ProjectDir: project, Version: "2.0.0", Target: "Widget"})
				return err
			},
			wantErr: "no build-settings",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, path := writeXCProjProject(t, test.contents)
			err := test.run(t.Context(), project)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, test.wantErr)
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(got) != test.contents {
				t.Fatalf("project.xcproj changed:\n%s", got)
			}
			entries, readErr := os.ReadDir(project)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(entries) != 1 {
				t.Fatalf("project directory gained files: %v", entries)
			}
		})
	}
}

func setVersion20(ctx context.Context, project string) error {
	_, err := SetVersion(ctx, SetVersionOptions{ProjectDir: project, Version: "2.0"})
	return err
}

func TestProjectWithBothDefinitionFilesFails(t *testing.T) {
	project, _ := writeXCProjProject(t, syntheticXCProj)
	if err := os.WriteFile(filepath.Join(project, "project.pbxproj"), []byte("// !$*UTF8*$!\n{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := GetVersionScoped(t.Context(), GetVersionOptions{ProjectDir: project, BuildSettingsLookup: BuildSettingsLookupNever})
	if err == nil || !strings.Contains(err.Error(), "both project.pbxproj and project.xcproj") {
		t.Fatalf("error = %v", err)
	}
}

func TestXCProjViewNeverRunsAgvtool(t *testing.T) {
	unresolved := strings.NewReplacer(`"MARKETING_VERSION": "1.2.3", /* keep me */`, "", `"MARKETING_VERSION": "1.0",`, "").Replace(syntheticXCProj)
	project, _ := writeXCProjProject(t, unresolved)
	restore := overrideTestEnvironment(t)
	t.Cleanup(restore)
	runtimeGOOS = "darwin"
	trustedXcodeToolPathFn = func(_ context.Context, tool string, _ []string) (string, error) {
		t.Fatalf("unexpected tool %q", tool)
		return "", nil
	}
	_, err := GetVersionScoped(t.Context(), GetVersionOptions{ProjectDir: project, BuildSettingsLookup: BuildSettingsLookupNever})
	if err == nil || !strings.Contains(err.Error(), "--xcodebuild-settings-lookup never") {
		t.Fatalf("error = %v", err)
	}
}

const signingXCProj = `{
  "default-configuration": "Release",
  "configurations": ["Debug", "Release"],
  "files": [],
  "targets": [
    {
      "name": "App",
      "id": "AAAAAAAAAAAAAAAAAAAAAAAA",
      "product-type": "application",
      "build-settings": {
        "CODE_SIGN_STYLE": "Automatic",
        "DEVELOPMENT_TEAM[config=Debug]": "TEAMDEBUG1",
        "DEVELOPMENT_TEAM[config=Release]": "TEAMOLD123",
        "PRODUCT_BUNDLE_IDENTIFIER": "com.example.app",
      },
    },
  ],
  "build-settings": {
    "SDKROOT": "iphoneos",
  },
}
`

func TestXCProjSigningPlanAndApply(t *testing.T) {
	requireStrictSigningPlatform(t)
	for _, test := range []struct {
		name        string
		contents    string
		settings    string
		wantBlocker string
		replace     [][2]string
	}{
		{
			name:     "per-configuration signing settings",
			contents: signingXCProj,
			settings: `{"CODE_SIGN_STYLE": "manual", "DEVELOPMENT_TEAM": "ABCDE12345", "PROVISIONING_PROFILE_SPECIFIER": "App Store Profile"}`,
			replace: [][2]string{
				{`"CODE_SIGN_STYLE": "Automatic",`, "\"CODE_SIGN_STYLE[config=Debug]\": \"Automatic\",\n        \"CODE_SIGN_STYLE[config=Release]\": \"Manual\","},
				{`"DEVELOPMENT_TEAM[config=Release]": "TEAMOLD123"`, `"DEVELOPMENT_TEAM[config=Release]": "ABCDE12345"`},
				{`"PRODUCT_BUNDLE_IDENTIFIER": "com.example.app",`, "\"PRODUCT_BUNDLE_IDENTIFIER\": \"com.example.app\",\n        \"PROVISIONING_PROFILE_SPECIFIER[config=Release]\": \"App Store Profile\","},
			},
		},
		{
			name:        "unsupported removal blocks the plan",
			contents:    strings.Replace(signingXCProj, `"PRODUCT_BUNDLE_IDENTIFIER": "com.example.app",`, `"PRODUCT_BUNDLE_IDENTIFIER": "com.example.app", "PROVISIONING_PROFILE_SPECIFIER": "Old",`, 1),
			settings:    `{"PROVISIONING_PROFILE_SPECIFIER": null}`,
			wantBlocker: "removing build setting",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			contents := test.contents
			project, path := writeXCProjProject(t, contents)
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{"schemaVersion": 1, "targets": [{"name": "App", "configurations": [{"name": "Release", "settings": `+test.settings+`}]}]}`)
			plan, err := BuildSigningPlan(SigningPlanOptions{ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state")})
			if err != nil {
				t.Fatal(err)
			}
			if test.wantBlocker != "" {
				if plan.Ready || !strings.Contains(strings.Join(plan.Blockers, "\n"), test.wantBlocker) {
					t.Fatalf("ready = %v, blockers = %v, want a blocker containing %q", plan.Ready, plan.Blockers, test.wantBlocker)
				}
				if got := mustReadVersionTestFile(t, path); got != contents {
					t.Fatalf("project.xcproj changed:\n%s", got)
				}
				return
			}
			if !plan.Ready {
				t.Fatalf("blockers = %v", plan.Blockers)
			}
			for _, change := range plan.Changes {
				if change.Source != "xcproj" || change.Path != path {
					t.Fatalf("change = %+v", change)
				}
			}
			if err := WriteSigningPlanArtifact(plan, false); err != nil {
				t.Fatal(err)
			}
			result, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
			if err != nil {
				t.Fatal(err)
			}
			want := contents
			for _, replacement := range test.replace {
				if !strings.Contains(want, replacement[0]) {
					t.Fatalf("fixture has no %q", replacement[0])
				}
				want = strings.Replace(want, replacement[0], replacement[1], 1)
			}
			if got := mustReadVersionTestFile(t, path); got != want {
				t.Fatalf("project.xcproj =\n%s\nwant\n%s", got, want)
			}
			if len(result.Files) != 1 || result.Files[0].Path != path || result.Files[0].Source != "xcproj" {
				t.Fatalf("receipt files = %+v", result.Files)
			}
		})
	}
}

func TestXCProjSigningRejectsSymlinkedProjectFile(t *testing.T) {
	project, path := writeXCProjProject(t, signingXCProj)
	moved := filepath.Join(filepath.Dir(project), "elsewhere.xcproj")
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := openSigningStructuredVersionProject(project); !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("error = %v, want a symlink refusal", err)
	}
}
