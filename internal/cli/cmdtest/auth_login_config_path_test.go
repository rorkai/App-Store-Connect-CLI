package cmdtest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

// isolateAuthConfigWrite gives the test its own home directory and working
// directory, clears inherited credential selection, and returns the global
// config path under that home. Callers choose ASC_CONFIG_PATH themselves.
func isolateAuthConfigWrite(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, name := range []string{
		"ASC_PROFILE",
		"ASC_KEY_ID",
		"ASC_ISSUER_ID",
		"ASC_PRIVATE_KEY_PATH",
		"ASC_PRIVATE_KEY",
		"ASC_PRIVATE_KEY_B64",
		"ASC_KEY_TYPE",
		"ASC_STRICT_AUTH",
	} {
		t.Setenv(name, "")
	}

	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir(.git) error: %v", err)
	}
	t.Chdir(workDir)

	return filepath.Join(home, ".asc", "config.json")
}

func runAuthConfigCommand(t *testing.T, args ...string) (string, string) {
	t.Helper()

	stdout, stderr, code := runWithDeps(t, nil, args...)
	if code != cmd.ExitSuccess {
		t.Fatalf("%v exit code = %d, want %d; stderr=%q", args, code, cmd.ExitSuccess, stderr)
	}
	return stdout, stderr
}

func authLoginConfigArgs(t *testing.T, extra ...string) []string {
	t.Helper()

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	args := []string{
		"auth", "login",
		"--skip-validation",
		"--name", "Isolated",
		"--key-id", "KEY123",
		"--issuer-id", "ISS456",
		"--private-key", keyPath,
	}
	return append(args, extra...)
}

func requireConfigProfiles(t *testing.T, path string, want ...string) {
	t.Helper()

	cfg, err := config.LoadAt(path)
	if err != nil {
		t.Fatalf("LoadAt(%s) error: %v", path, err)
	}
	got := make([]string, 0, len(cfg.Keys))
	for _, key := range cfg.Keys {
		got = append(got, key.Name)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("profiles in %s = %v, want %v", path, got, want)
	}
}

func requireNoConfigFile(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no config at %s, got stat error %v", path, err)
	}
}

func TestAuthLoginBypassWritesToConfigPathOverride(t *testing.T) {
	tests := []struct {
		name      string
		bypassEnv string
		extra     []string
	}{
		{name: "ASC_BYPASS_KEYCHAIN", bypassEnv: "1"},
		{name: "--bypass-keychain", bypassEnv: "0", extra: []string{"--bypass-keychain"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			globalPath := isolateAuthConfigWrite(t)
			overridePath := filepath.Join(t.TempDir(), "isolated", "config.json")
			t.Setenv("ASC_CONFIG_PATH", overridePath)
			t.Setenv("ASC_BYPASS_KEYCHAIN", test.bypassEnv)

			stdout, _ := runAuthConfigCommand(t, authLoginConfigArgs(t, test.extra...)...)
			if !strings.Contains(stdout, "Storing credentials in config file at "+overridePath) {
				t.Fatalf("expected the storage message to name %s, got %q", overridePath, stdout)
			}
			requireConfigProfiles(t, overridePath, "Isolated")
			requireNoConfigFile(t, globalPath)

			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			statusOut, _ := runAuthConfigCommand(t, "auth", "status", "--output", "json")
			var status struct {
				Credentials []struct {
					Name     string `json:"name"`
					KeyID    string `json:"keyId"`
					StoredIn string `json:"storedIn"`
				} `json:"credentials"`
			}
			if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
				t.Fatalf("unmarshal auth status: %v; stdout=%q", err, statusOut)
			}
			if len(status.Credentials) != 1 ||
				status.Credentials[0].Name != "Isolated" ||
				status.Credentials[0].KeyID != "KEY123" ||
				status.Credentials[0].StoredIn != "config: "+overridePath {
				t.Fatalf("auth status did not read back the saved profile from %s: %+v", overridePath, status.Credentials)
			}
		})
	}
}

func TestAuthLoginBypassWithoutConfigPathOverrideWritesGlobalConfig(t *testing.T) {
	globalPath := isolateAuthConfigWrite(t)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	stdout, _ := runAuthConfigCommand(t, authLoginConfigArgs(t)...)
	if !strings.Contains(stdout, "Storing credentials in config file at "+globalPath) {
		t.Fatalf("expected the storage message to name %s, got %q", globalPath, stdout)
	}
	requireConfigProfiles(t, globalPath, "Isolated")
	requireNoConfigFile(t, filepath.Join(".", ".asc", "config.json"))
}

func TestAuthLoginBypassWithoutNameReadsConfigPathOverride(t *testing.T) {
	globalPath := isolateAuthConfigWrite(t)
	overridePath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", overridePath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	if err := config.SaveAt(globalPath, &config.Config{
		Keys: []config.Credential{{Name: "GlobalOnly", KeyID: "GLOBAL1", IssuerID: "ISS", PrivateKeyPath: "/tmp/global.p8"}},
	}); err != nil {
		t.Fatalf("SaveAt(global) error: %v", err)
	}
	globalBefore, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("ReadFile(global) error: %v", err)
	}

	args := authLoginConfigArgs(t)
	withoutName := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		if args[index] == "--name" {
			index++
			continue
		}
		withoutName = append(withoutName, args[index])
	}
	runAuthConfigCommand(t, withoutName...)

	requireConfigProfiles(t, overridePath, "default")
	globalAfter, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("ReadFile(global) error: %v", err)
	}
	if string(globalAfter) != string(globalBefore) {
		t.Fatalf("expected the global config to be unchanged")
	}
}

func TestAuthLoginBypassLocalWinsOverConfigPathOverride(t *testing.T) {
	globalPath := isolateAuthConfigWrite(t)
	overridePath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", overridePath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	_, stderr := runAuthConfigCommand(t, authLoginConfigArgs(t, "--local")...)

	localPath, err := config.LocalPath()
	if err != nil {
		t.Fatalf("LocalPath() error: %v", err)
	}
	requireConfigProfiles(t, localPath, "Isolated")
	requireNoConfigFile(t, overridePath)
	requireNoConfigFile(t, globalPath)
	want := "Warning: ASC_CONFIG_PATH is set, so other commands read " + overridePath + " instead of " + localPath
	if !strings.Contains(stderr, want) {
		t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
	}
}

func TestAuthInitWritesToConfigPathOverride(t *testing.T) {
	globalPath := isolateAuthConfigWrite(t)
	overridePath := filepath.Join(t.TempDir(), "isolated", "config.json")
	t.Setenv("ASC_CONFIG_PATH", overridePath)

	stdout, _ := runAuthConfigCommand(t, "auth", "init")
	var result struct {
		ConfigPath string `json:"config_path"`
		Created    bool   `json:"created"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal auth init: %v; stdout=%q", err, stdout)
	}
	if result.ConfigPath != overridePath || !result.Created {
		t.Fatalf("auth init result = %+v, want config_path %q", result, overridePath)
	}
	if _, err := os.Stat(overridePath); err != nil {
		t.Fatalf("expected config at %s: %v", overridePath, err)
	}
	requireNoConfigFile(t, globalPath)
}

func TestAuthInitWithoutConfigPathOverrideWritesGlobalConfig(t *testing.T) {
	globalPath := isolateAuthConfigWrite(t)
	t.Setenv("ASC_CONFIG_PATH", "")

	stdout, _ := runAuthConfigCommand(t, "auth", "init")
	var result struct {
		ConfigPath string `json:"config_path"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal auth init: %v; stdout=%q", err, stdout)
	}
	if result.ConfigPath != globalPath {
		t.Fatalf("auth init config_path = %q, want %q", result.ConfigPath, globalPath)
	}
	if _, err := os.Stat(globalPath); err != nil {
		t.Fatalf("expected config at %s: %v", globalPath, err)
	}
}

func TestAuthLoginBypassLocalWarnsWhenConfigPathOverrideIsInvalid(t *testing.T) {
	isolateAuthConfigWrite(t)
	t.Setenv("ASC_CONFIG_PATH", "relative/config.json")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	_, stderr := runAuthConfigCommand(t, authLoginConfigArgs(t, "--local")...)

	localPath, err := config.LocalPath()
	if err != nil {
		t.Fatalf("LocalPath() error: %v", err)
	}
	requireConfigProfiles(t, localPath, "Isolated")
	want := "Warning: other commands cannot read " + localPath + " while ASC_CONFIG_PATH is invalid"
	if !strings.Contains(stderr, want) {
		t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
	}
}
