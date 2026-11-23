package cmdtest

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

// writeAPIKeyJSON writes a JSON API key file holding a throwaway P-256 key and
// returns the file path and the key's PEM text.
func writeAPIKeyJSON(t *testing.T, fields map[string]any, base64Key bool) (string, string) {
	t.Helper()

	dir := t.TempDir()
	pemPath := filepath.Join(dir, "key.p8")
	writeECDSAPEM(t, pemPath)
	pemData, err := os.ReadFile(pemPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if _, ok := fields["key"]; !ok {
		fields["key"] = string(pemData)
		if base64Key {
			fields["key"] = base64.StdEncoding.EncodeToString(pemData)
			fields["is_key_content_base64"] = true
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal key json: %v", err)
	}
	jsonPath := filepath.Join(dir, "api_key.json")
	if err := os.WriteFile(jsonPath, data, 0o600); err != nil {
		t.Fatalf("write key json: %v", err)
	}
	return jsonPath, string(pemData)
}

func runAuthLoginKeyJSON(t *testing.T, args ...string) (int, string) {
	t.Helper()

	_, stderr, code := runWithDeps(t, nil, append([]string{"auth", "login", "--skip-validation", "--name", "CI"}, args...)...)
	return code, stderr
}

func TestAuthLoginKeyJSON(t *testing.T) {
	const issuerID = "00000000-0000-0000-0000-000000000000"
	tests := []struct {
		name        string
		fields      map[string]any
		base64Key   bool
		args        []string
		wantCode    int
		wantStderr  string
		wantKeyType string
		// noUsagePage marks a refused file content: the message names the
		// problem, so the command's usage page must not follow it.
		noUsagePage bool
	}{
		{
			name:     "stores key id, issuer id and a 0600 copy of the key",
			fields:   map[string]any{"key_id": "ABC123DEFG", "issuer_id": issuerID},
			wantCode: rootcmd.ExitSuccess,
		},
		{
			name:        "base64 key content and in-house key",
			fields:      map[string]any{"key_id": "ABC123DEFG", "issuer_id": issuerID, "in_house": true, "duration": 1200},
			base64Key:   true,
			wantCode:    rootcmd.ExitSuccess,
			wantStderr:  "Warning: --key-json fields not applied: duration\n",
			wantKeyType: config.CredentialKeyTypeEnterprise,
		},
		{
			name:       "conflicts with --key-id",
			fields:     map[string]any{"key_id": "ABC123DEFG", "issuer_id": issuerID},
			args:       []string{"--key-id", "ABC123DEFG"},
			wantCode:   rootcmd.ExitUsage,
			wantStderr: "--key-json cannot be combined with --key-id",
		},
		{
			name:       "in-house key conflicts with --key-type team",
			fields:     map[string]any{"key_id": "ABC123DEFG", "issuer_id": issuerID, "in_house": true},
			args:       []string{"--key-type", "team"},
			wantCode:   rootcmd.ExitUsage,
			wantStderr: `--key-type team conflicts with "in_house": true in the --key-json file`,
		},
		{
			name:        "missing key",
			fields:      map[string]any{"key_id": "ABC123DEFG", "issuer_id": issuerID, "key": ""},
			wantCode:    rootcmd.ExitUsage,
			wantStderr:  `auth login: --key-json file is missing "key"`,
			noUsagePage: true,
		},
		{
			name:        "invalid base64",
			fields:      map[string]any{"key_id": "ABC123DEFG", "issuer_id": issuerID, "key": "not base64!", "is_key_content_base64": true},
			wantCode:    rootcmd.ExitUsage,
			wantStderr:  `auth login: --key-json field "key" is not valid base64`,
			noUsagePage: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateAuthConfigWrite(t)
			configPath := filepath.Join(t.TempDir(), "config.json")
			t.Setenv("ASC_CONFIG_PATH", configPath)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			jsonPath, pemText := writeAPIKeyJSON(t, test.fields, test.base64Key)

			code, stderr := runAuthLoginKeyJSON(t, append([]string{"--key-json", jsonPath}, test.args...)...)
			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, test.wantCode, stderr)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if strings.Contains(stderr, "PRIVATE KEY") {
				t.Fatalf("stderr leaked key material: %q", stderr)
			}
			if test.noUsagePage && strings.Contains(stderr, "USAGE") {
				t.Fatalf("stderr = %q, want no usage page after the --key-json error", stderr)
			}
			if test.wantCode != rootcmd.ExitSuccess {
				requireNoConfigFile(t, configPath)
				return
			}

			cfg, err := config.LoadAt(configPath)
			if err != nil {
				t.Fatalf("LoadAt() error: %v", err)
			}
			if len(cfg.Keys) != 1 {
				t.Fatalf("stored keys = %+v, want one", cfg.Keys)
			}
			stored := cfg.Keys[0]
			wantPath := filepath.Join(filepath.Dir(configPath), "keys", "AuthKey_CI_ABC123DEFG.p8")
			if stored.KeyID != "ABC123DEFG" || stored.IssuerID != issuerID || stored.KeyType != test.wantKeyType || stored.PrivateKeyPath != wantPath {
				t.Fatalf("stored credential = %+v, want key ABC123DEFG, issuer %s, type %q, path %s", stored, issuerID, test.wantKeyType, wantPath)
			}
			if !strings.Contains(stderr, "Private key from --key-json saved to "+wantPath+"\n") {
				t.Fatalf("stderr = %q, want it to name %s", stderr, wantPath)
			}
			written, err := os.ReadFile(wantPath)
			if err != nil {
				t.Fatalf("read written key: %v", err)
			}
			if string(written) != pemText {
				t.Fatal("written key file does not hold the key from the JSON file")
			}
			info, err := os.Stat(wantPath)
			if err != nil {
				t.Fatalf("stat written key: %v", err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
				t.Fatalf("written key mode = %v, want 0600", info.Mode().Perm())
			}
		})
	}
}

func TestAuthLoginKeyJSONPermissiveFilePrintsErrorThenFix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode permissions are not enforced on Windows")
	}
	isolateAuthConfigWrite(t)
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	jsonPath, _ := writeAPIKeyJSON(t, map[string]any{"key_id": "ABC123DEFG", "issuer_id": "00000000-0000-0000-0000-000000000000"}, false)
	if err := os.Chmod(jsonPath, 0o644); err != nil {
		t.Fatalf("chmod key json: %v", err)
	}

	code, stderr := runAuthLoginKeyJSON(t, "--key-json", jsonPath)

	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
	}
	errorAt := strings.Index(stderr, "Error: auth login: invalid --key-json file: file is too permissive")
	fixAt := strings.Index(stderr, "To fix, run:\n  chmod 600 ")
	if errorAt < 0 || fixAt < errorAt {
		t.Fatalf("stderr = %q, want the error line, then the chmod fix", stderr)
	}
	if strings.Contains(stderr, "USAGE") {
		t.Fatalf("stderr = %q, want no usage page burying the fix", stderr)
	}
	requireNoConfigFile(t, configPath)
}

func TestAuthLoginKeyJSONRefusesToReplaceADifferentKey(t *testing.T) {
	isolateAuthConfigWrite(t)
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	fields := func() map[string]any {
		return map[string]any{"key_id": "ABC123DEFG", "issuer_id": "00000000-0000-0000-0000-000000000000"}
	}

	firstJSON, firstPEM := writeAPIKeyJSON(t, fields(), false)
	if code, stderr := runAuthLoginKeyJSON(t, "--key-json", firstJSON); code != rootcmd.ExitSuccess {
		t.Fatalf("first login exit code = %d; stderr = %q", code, stderr)
	}
	secondJSON, _ := writeAPIKeyJSON(t, fields(), false)
	code, stderr := runAuthLoginKeyJSON(t, "--key-json", secondJSON)
	if code == rootcmd.ExitSuccess {
		t.Fatalf("second login with a different key succeeded; stderr = %q", stderr)
	}
	if !strings.Contains(stderr, "refusing to overwrite existing private key file") {
		t.Fatalf("stderr = %q, want the overwrite refusal", stderr)
	}
	keyPath := filepath.Join(filepath.Dir(configPath), "keys", "AuthKey_CI_ABC123DEFG.p8")
	kept, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key file: %v", err)
	}
	if string(kept) != firstPEM {
		t.Fatal("second login replaced the first key file")
	}
}
