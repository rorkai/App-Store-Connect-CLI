package cmdtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func clearAuthEnvWithConfig(t *testing.T) string {
	t.Helper()
	home := setCmdtestHome(t)
	for _, name := range []string{"ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH", "ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64"} {
		t.Setenv(name, "")
	}
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	configPath := filepath.Join(home, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	return configPath
}

func TestLocalCredentialFailuresAreMissingAuth(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) []string
		want  func(configPath string) string
	}{
		{
			name: "malformed config",
			setup: func(t *testing.T) []string {
				configPath := clearAuthEnvWithConfig(t)
				if err := os.WriteFile(configPath, []byte("bad"), 0o600); err != nil {
					t.Fatalf("write config: %v", err)
				}
				return []string{"apps", "list"}
			},
			want: func(configPath string) string { return "failed to parse config " + configPath + ": invalid character" },
		},
		{
			name: "profile without config",
			setup: func(t *testing.T) []string {
				clearAuthEnvWithConfig(t)
				return []string{"--profile", "ghost", "apps", "list"}
			},
			want: func(configPath string) string {
				return `credentials not found for profile "ghost"; no profiles are configured (config file: ` + configPath + ")"
			},
		},
		{
			name: "unknown profile",
			setup: func(t *testing.T) []string {
				writeProfileFixtureConfig(t)
				return []string{"--profile", "ghost", "apps", "list"}
			},
			want: func(string) string {
				return `credentials not found for profile "ghost"; available profiles: client, prod, staging`
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := test.setup(t)
			var code int
			stdout, stderr := captureOutput(t, func() { code = rootcmd.Run(args, "1.2.3") })
			if code != rootcmd.ExitAuth {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitAuth, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if want := test.want(os.Getenv("ASC_CONFIG_PATH")); !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if !strings.Contains(stderr, "Hint: Run `asc auth status`") {
				t.Fatalf("stderr = %q, want the auth status hint", stderr)
			}
		})
	}
}

func TestSymlinkedConfigIsRefusedWithGuidance(t *testing.T) {
	tests := []struct {
		name string
		link func(t *testing.T, home, realDir string)
		args []string
	}{
		{
			name: "symlinked config file",
			link: func(t *testing.T, home, realDir string) {
				if err := os.Mkdir(filepath.Join(home, ".asc"), 0o700); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.Symlink(filepath.Join(realDir, "config.json"), filepath.Join(home, ".asc", "config.json")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			},
			args: []string{"apps", "list"},
		},
		{
			name: "symlinked config directory",
			link: func(t *testing.T, home, realDir string) {
				if err := os.Symlink(realDir, filepath.Join(home, ".asc")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			},
			args: []string{"auth", "status"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := setCmdtestHome(t)
			t.Chdir(t.TempDir())
			for _, name := range []string{"ASC_CONFIG_PATH", "ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH", "ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64"} {
				t.Setenv(name, "")
			}
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			realDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(realDir, "config.json"), []byte(`{"key_id":"ABC123DEFG","issuer_id":"00000000-0000-0000-0000-000000000000","private_key_path":"/nonexistent/AuthKey.p8"}`), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			test.link(t, home, realDir)

			var code int
			stdout, stderr := captureOutput(t, func() { code = rootcmd.Run(test.args, "1.2.3") })
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			configPath := filepath.Join(home, ".asc", "config.json")
			wantError := "config " + configPath + " is a symlink or is inside a symlinked directory; symlinked config files and directories are not followed\n"
			wantHint := "Hint: Replace the symlink with the real file, or set ASC_CONFIG_PATH to the absolute path of a config file whose path contains no symlinks.\n"
			if strings.Count(stderr, "Error:") != 1 || !strings.Contains(stderr, wantError) || !strings.HasSuffix(stderr, wantHint) {
				t.Fatalf("stderr = %q, want one error ending %q and hint %q", stderr, wantError, wantHint)
			}
		})
	}
}

func TestAuthLoginMissingPrivateKeyNamesGivenPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unexpanded tilde",
			args: []string{"--private-key", "~/AuthKey.p8"},
			want: `private key file not found: "~/AuthKey.p8" (the shell did not expand ~; use an absolute path)`,
		},
		{
			name: "fix permissions",
			args: []string{"--fix-permissions", "--private-key", "/nonexistent/AuthKey.p8"},
			want: `private key file not found: "/nonexistent/AuthKey.p8"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearAuthEnvWithConfig(t)
			args := append([]string{"auth", "login", "--bypass-keychain", "--name", "demo", "--key-id", "ABC123DEFG", "--issuer-id", "00000000-0000-0000-0000-000000000000"}, test.args...)
			var code int
			_, stderr := captureOutput(t, func() { code = rootcmd.Run(args, "1.2.3") })
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
			}
			if !strings.Contains(stderr, "Error: auth login: invalid private key: "+test.want+"\n") {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
		})
	}
}

func TestAuthLoginKeyIDFromKeyFileName(t *testing.T) {
	tests := []struct {
		name       string
		keyFile    string
		args       []string
		wantCode   int
		wantStderr string
		wantKeyID  string
	}{
		{
			name:       "omitted key id is read from the file name",
			keyFile:    "AuthKey_39MX87M9Y4.p8",
			wantCode:   rootcmd.ExitSuccess,
			wantStderr: "Using --key-id 39MX87M9Y4 from the private key file name\n",
			wantKeyID:  "39MX87M9Y4",
		},
		{
			name:      "explicit key id wins",
			keyFile:   "AuthKey_39MX87M9Y4.p8",
			args:      []string{"--key-id", "ABC123DEFG"},
			wantCode:  rootcmd.ExitSuccess,
			wantKeyID: "ABC123DEFG",
		},
		{
			name:       "other file names still require the flag",
			keyFile:    "key.p8",
			wantCode:   rootcmd.ExitUsage,
			wantStderr: "Error: --key-id is required\nHint: the key ID is listed next to the key on the API keys page and in the downloaded file name, AuthKey_<KEY_ID>.p8.\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := clearAuthEnvWithConfig(t)
			keyPath := filepath.Join(t.TempDir(), test.keyFile)
			writeECDSAPEM(t, keyPath)
			args := append([]string{"auth", "login", "--bypass-keychain", "--skip-validation", "--name", "demo", "--issuer-id", "00000000-0000-0000-0000-000000000000", "--private-key", keyPath}, test.args...)
			var code int
			_, stderr := captureOutput(t, func() { code = rootcmd.Run(args, "1.2.3") })
			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, test.wantCode, stderr)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if test.wantStderr == "" && strings.Contains(stderr, "Using --key-id") {
				t.Fatalf("stderr = %q, want no inferred key ID", stderr)
			}
			if test.wantKeyID == "" {
				return
			}
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("read config: %v", err)
			}
			var stored struct {
				KeyID string `json:"key_id"`
			}
			if err := json.Unmarshal(data, &stored); err != nil {
				t.Fatalf("parse config: %v", err)
			}
			if stored.KeyID != test.wantKeyID {
				t.Fatalf("stored key_id = %q, want %q", stored.KeyID, test.wantKeyID)
			}
		})
	}
}

func TestAPIPathWithoutMethodSuggestsGET(t *testing.T) {
	clearAuthEnvWithConfig(t)
	var code int
	_, stderr := captureOutput(t, func() { code = rootcmd.Run([]string{"api", "/v1/apps"}, "1.2.3") })
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if !strings.Contains(stderr, "Error: api: METHOD and PATH are required\nHint: pass the method first: asc api GET /v1/apps\n") {
		t.Fatalf("stderr = %q, want a GET suggestion", stderr)
	}
}
