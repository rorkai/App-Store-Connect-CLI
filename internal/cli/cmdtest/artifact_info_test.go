package cmdtest

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestRunArtifactInfoReceipts(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))
	for _, key := range []string{"ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH", "ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_STRICT_AUTH"} {
		t.Setenv(key, "")
	}
	dir := t.TempDir()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	entry, err := archive.Create("Payload/Demo.app/Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(`<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.demo</string></dict></plist>`)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	unsigned := filepath.Join(dir, "unsigned.ipa")
	if err := os.WriteFile(unsigned, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	oversized := filepath.Join(dir, "oversized.pkg")
	file, err := os.Create(oversized)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate((512 << 20) + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		args       []string
		status     string
		exit       int
		diagnostic string
	}{
		{"missing path", []string{"ipa-info", "--output", "json"}, "", cmd.ExitUsage, "--path is required"},
		{"unsigned", []string{"ipa-info", "--path", unsigned, "--output", "json"}, "unsigned", cmd.ExitError, "embedded profile"},
		{"missing file", []string{"pkg-info", "--path", filepath.Join(dir, "missing.pkg"), "--output", "json"}, "unreadable", cmd.ExitError, "pkg-info:"},
		{"oversized", []string{"pkg-info", "--path", oversized, "--output", "json"}, "unreadable", cmd.ExitError, "not a flat xar package"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var exit int
			stdout, stderr := captureOutput(t, func() { exit = cmd.Run(test.args, "test") })
			if exit != test.exit || !strings.Contains(stderr, test.diagnostic) {
				t.Fatalf("exit=%d stderr=%q", exit, stderr)
			}
			if test.status == "" {
				if stdout != "" {
					t.Fatalf("unexpected stdout %q", stdout)
				}
				return
			}
			var receipt map[string]any
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatalf("stdout=%q: %v", stdout, err)
			}
			if receipt["status"] != test.status || receipt["signatureVerification"] != "not-verified" {
				t.Fatalf("receipt=%v", receipt)
			}
		})
	}
}
