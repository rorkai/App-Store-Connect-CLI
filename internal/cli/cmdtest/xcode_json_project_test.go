package cmdtest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Synthetic fixture derived from Apple's xcode-project-format documentation.
const syntheticJSONProject = `{
  "default-configuration": "Release",
  "configurations": [ "Release" ],
  "files": [],
  "targets": [
    {
      "name": "App",
      "id": "AAAAAAAAAAAAAAAAAAAAAAAA",
      "product-type": "application",
      "build-settings": {
        "CURRENT_PROJECT_VERSION": "42",
        "MARKETING_VERSION": "1.2.3",
      },
    },
  ],
}
`

func TestXcodeCommandsWithJSONProject(t *testing.T) {
	for _, test := range []struct {
		name        string
		args        []string
		settings    string
		wantStdout  string
		wantChanged bool
	}{
		{name: "view", args: []string{"xcode", "version", "view"}, wantStdout: `"version":"1.2.3"`},
		{name: "edit", args: []string{"xcode", "version", "edit", "--version", "2.0.0"}, wantStdout: `"source":"xcproj"`, wantChanged: true},
		{name: "bump", args: []string{"xcode", "version", "bump", "--type", "build"}, wantStdout: `"newBuild":"43"`, wantChanged: true},
		{
			name:       "signing plan",
			args:       []string{"xcode", "signing", "plan"},
			settings:   `{"schemaVersion": 1, "targets": [{"name": "App", "configurations": [{"name": "Release", "settings": {"CODE_SIGN_STYLE": "manual"}}]}]}`,
			wantStdout: `"source":"xcproj"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			rootDir := t.TempDir()
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(rootDir, "config.json"))
			project := filepath.Join(rootDir, "Demo.xcodeproj")
			if err := os.Mkdir(project, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(project, "project.xcproj")
			if err := os.WriteFile(path, []byte(syntheticJSONProject), 0o600); err != nil {
				t.Fatal(err)
			}
			args := append(append([]string(nil), test.args...), "--project", project, "--output", "json")
			if test.settings != "" {
				settingsPath := filepath.Join(rootDir, "settings.json")
				if err := os.WriteFile(settingsPath, []byte(test.settings), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--settings-file", settingsPath, "--state-dir", filepath.Join(rootDir, "state"))
			}
			command := RootCommand("test")
			var runErr error
			stdout, _ := captureOutput(t, func() {
				if err := command.Parse(args); err != nil {
					t.Fatal(err)
				}
				runErr = command.Run(context.Background())
			})
			if runErr != nil {
				t.Fatal(runErr)
			}
			if !json.Valid([]byte(stdout)) || !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("stdout = %s, want %s", stdout, test.wantStdout)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if changed := string(got) != syntheticJSONProject; changed != test.wantChanged {
				t.Fatalf("changed = %v, want %v:\n%s", changed, test.wantChanged, got)
			}
		})
	}
}
