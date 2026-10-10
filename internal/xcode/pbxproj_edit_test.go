package xcode

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

func TestStructuredVersionPreservesPBXProjBytes(t *testing.T) {
	for _, name := range []string{"edit", "bump", "shared-configurations"} {
		t.Run(name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, false)
			path := filepath.Join(project, "project.pbxproj")
			oldSettings := `MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42;`
			settings := `
    "MARKETING_VERSION" /* key */ = "1.2.3" /* value */;
    CURRENT_PROJECT_VERSION = 42;
    "CURRENT_PROJECT_VERSION[sdk=iphoneos*]" = "42";
    OTHER = ("escaped\"quote", { nested = (a, b); });
    /* MARKETING_VERSION = 1.2.3; */
   `
			before := strings.Replace(mustReadVersionTestFile(t, path), oldSettings, settings, 1)
			if name == "shared-configurations" {
				before = strings.Replace(before, "buildConfigurationList = 777777777777777777777777;", "buildConfigurationList = 555555555555555555555555;", 1)
			}
			before = strings.ReplaceAll(before, "\n", "\r\n")
			settings = strings.ReplaceAll(settings, "\n", "\r\n")
			writeSigningSettingsTestFile(t, path, before)
			want := strings.Replace(before, settings, strings.ReplaceAll(settings, `= 42;`, `= 43;`), 1)
			want = strings.Replace(want, `= "42";`, `= "43";`, 1)
			if name == "bump" {
				if _, err := BumpVersion(context.Background(), BumpVersionOptions{ProjectDir: project, Target: "App", Configuration: "Debug", BumpType: BumpBuild}); err != nil {
					t.Fatal(err)
				}
			} else {
				want = strings.Replace(want, `= "1.2.3"`, `= "2.0.0"`, 1)
				if _, err := SetVersion(context.Background(), SetVersionOptions{ProjectDir: project, Target: "App", Configuration: "Debug", Version: "2.0.0", BuildNumber: "43"}); err != nil {
					t.Fatal(err)
				}
			}
			if got := mustReadVersionTestFile(t, path); got != want {
				t.Fatalf("pbxproj bytes outside requested values changed:\n%s", got)
			}
			info := mustGetStructuredVersion(t, project, "App", "Debug")
			if info.BuildNumber != "43" {
				t.Fatalf("build = %q", info.BuildNumber)
			}
		})
	}
}

func TestStructuredVersionPreservesPBXProjScopedInsertion(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	path := filepath.Join(project, "project.pbxproj")
	before := mustReadVersionTestFile(t, path)
	marker := `999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = AAAAAAAAAAAAAAAAAAAAAAAA; buildSettings = {  };`
	want := strings.Replace(before, marker, strings.Replace(marker, `{  };`, `{  MARKETING_VERSION = 2.0.0; };`, 1), 1)
	if want == before {
		t.Fatal("missing fixture marker")
	}
	shared := mustReadVersionTestFile(t, filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig"))
	if _, err := SetVersion(context.Background(), SetVersionOptions{ProjectDir: project, Target: "App", Configuration: "Debug", Version: "2.0.0"}); err != nil {
		t.Fatal(err)
	}
	if got := mustReadVersionTestFile(t, path); got != want {
		t.Fatalf("scoped insertion changed unrelated bytes:\n%s", got)
	}
	if got := mustReadVersionTestFile(t, filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")); got != shared {
		t.Fatal("shared xcconfig changed")
	}
	if info := mustGetStructuredVersion(t, project, "App", "Debug"); info.Version != "2.0.0" {
		t.Fatalf("version = %q", info.Version)
	}
}

func TestSigningApplyPreservesPBXProjBytes(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeStructuredVersionProject(t, false)
	path := filepath.Join(project, "project.pbxproj")
	before := mustReadVersionTestFile(t, path)
	old := `MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42;`
	settings := old + ` CODE_SIGN_IDENTITY = "Apple Development"; "CODE_SIGN_IDENTITY[sdk=iphoneos*]" = "Apple Development"; DEVELOPMENT_TEAM /* key */ = OLDTEAM123; OTHER = (";}", (a,b)); /* keep */`
	before = strings.Replace(before, old, settings, 1)
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_IDENTITY":null,"DEVELOPMENT_TEAM":"NEWTEAM123"}}]}]}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state")})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Ready {
		t.Fatalf("blockers = %#v", plan.Blockers)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(before, `CODE_SIGN_IDENTITY = "Apple Development";`, "", 1)
	want = strings.Replace(want, `"CODE_SIGN_IDENTITY[sdk=iphoneos*]" = "Apple Development";`, "", 1)
	want = strings.Replace(want, `= OLDTEAM123;`, `= NEWTEAM123;`, 1)
	if got := mustReadVersionTestFile(t, path); got != want {
		t.Fatalf("signing changed unrelated bytes:\n%s", got)
	}
}

func TestStructuredVersionPreservesPBXProjEncodingAndBareStrings(t *testing.T) {
	for _, tc := range []struct {
		name, bare string
		codec      encoding.Encoding
		bom        bool
	}{
		{name: "utf8-bom", bom: true},
		{name: "utf16-le-bom", codec: unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)},
		{name: "utf16-be-bom", codec: unicode.UTF16(unicode.BigEndian, unicode.UseBOM)},
		{name: "utf16-le", codec: unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)},
		{name: "utf16-be", codec: unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)},
		{name: "slashes", bare: "foo//bar"},
		{name: "comment-like-slashes", bare: "/not*a/*comm*en/t"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, false)
			path := filepath.Join(project, "project.pbxproj")
			before := mustReadVersionTestFile(t, path)
			setting := "CURRENT_PROJECT_VERSION = 42;"
			replacement := setting + ` OTHER = ("Unicode 日本語 😀", ` + tc.bare + `);`
			if tc.bare == "" {
				replacement = setting + ` OTHER = "Unicode 日本語 😀";`
			}
			before = strings.ReplaceAll(strings.Replace(before, setting, replacement, 1), "\n", "\r\n")
			want := strings.Replace(before, setting, "CURRENT_PROJECT_VERSION = 43;", 1)
			encode := func(text string) []byte {
				t.Helper()
				data := []byte(text)
				if tc.codec != nil {
					var err error
					data, err = tc.codec.NewEncoder().Bytes(data)
					if err != nil {
						t.Fatal(err)
					}
				} else if tc.bom {
					data = append([]byte{0xef, 0xbb, 0xbf}, data...)
				}
				return data
			}
			if err := os.WriteFile(path, encode(before), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := SetVersion(context.Background(), SetVersionOptions{ProjectDir: project, Target: "App", Configuration: "Debug", BuildNumber: "43"}); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, encode(want)) {
				t.Fatal("pbxproj encoding or unrelated bytes changed")
			}
			if info := mustGetStructuredVersion(t, project, "App", "Debug"); info.BuildNumber != "43" {
				t.Fatalf("build = %q", info.BuildNumber)
			}
		})
	}
}
