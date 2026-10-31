package cmdtest

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts/artifactstest"
)

func isolateArtifactCommandEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "no-config.json"))
	for _, key := range []string{"ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH", "ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_STRICT_AUTH"} {
		t.Setenv(key, "")
	}
}

func writeArtifactZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, name := range []string{
		"Payload/Demo.app/Info.plist",
		"Payload/Demo.app/Demo",
		"Payload/Demo.app/embedded.mobileprovision",
		"Payload/Demo.app/PlugIns/Widget.appex/Info.plist",
	} {
		data, ok := files[name]
		if !ok {
			continue
		}
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func signerJSON(chain artifactstest.Chain) map[string]any {
	return map[string]any{
		"commonName":        artifactstest.SignerCommonName,
		"teamId":            artifactstest.TeamID,
		"organization":      "Example Corp",
		"issuerCommonName":  artifactstest.IssuerCommonName,
		"serialNumber":      "1234ABCD",
		"notBefore":         "2026-01-02T03:04:05Z",
		"notAfter":          "2027-01-02T03:04:05Z",
		"sha1Fingerprint":   fmt.Sprintf("%X", sha1.Sum(chain.Leaf.Raw)),
		"sha256Fingerprint": fmt.Sprintf("%X", sha256.Sum256(chain.Leaf.Raw)),
	}
}

func assertJSONEqual(t *testing.T, got string, want map[string]any) {
	t.Helper()
	var decoded any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("stdout=%q: %v", got, err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var normalized any
	if err := json.Unmarshal(wantJSON, &normalized); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.MarshalIndent(decoded, "", "  ")
	wantIndented, _ := json.MarshalIndent(normalized, "", "  ")
	if string(gotJSON) != string(wantIndented) {
		t.Fatalf("receipt mismatch\n got: %s\nwant: %s", gotJSON, wantIndented)
	}
}

func TestRunIPAInfoReportsSignerIdentity(t *testing.T) {
	isolateArtifactCommandEnv(t)
	chain := artifactstest.NewChain(t)
	path := filepath.Join(t.TempDir(), "signed.ipa")
	writeArtifactZip(t, path, map[string]string{
		"Payload/Demo.app/Info.plist":                      `<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.demo</string><key>CFBundleExecutable</key><string>Demo</string><key>CFBundleShortVersionString</key><string>1.2.3</string><key>CFBundleVersion</key><string>9</string></dict></plist>`,
		"Payload/Demo.app/Demo":                            string(chain.SignedExecutable(t)),
		"Payload/Demo.app/embedded.mobileprovision":        `<plist version="1.0"><dict><key>Name</key><string>Demo Profile</string><key>UUID</key><string>PROFILE-UUID</string><key>Entitlements</key><dict><key>com.apple.developer.team-identifier</key><string>` + artifactstest.TeamID + `</string></dict></dict></plist>`,
		"Payload/Demo.app/PlugIns/Widget.appex/Info.plist": `<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.demo.widget</string></dict></plist>`,
	})
	var exit int
	stdout, stderr := captureOutput(t, func() { exit = cmd.Run([]string{"ipa-info", "--path", path, "--output", "json"}, "test") })
	if exit != cmd.ExitSuccess || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	assertJSONEqual(t, stdout, map[string]any{
		"signatureVerification": "not-verified",
		"path":                  path,
		"bundleId":              "com.example.demo",
		"version":               "1.2.3",
		"buildNumber":           "9",
		"teamId":                artifactstest.TeamID,
		"signerCommonName":      artifactstest.SignerCommonName,
		"status":                "readable",
		"codeSignature":         "signed",
		"signer":                signerJSON(chain),
		"architectures": []any{
			map[string]any{"cpuType": artifactstest.CPUTypeARM64, "cpuSubtype": 0, "arch": "arm64", "codeSignature": "signed", "signer": signerJSON(chain)},
		},
		"signerConsistent": true,
		"nestedBundles": []any{map[string]any{
			"bundleId": "com.example.demo.widget",
			"path":     "Payload/Demo.app/PlugIns/Widget.appex/Info.plist",
		}},
	})

	stdout, _ = captureOutput(t, func() { exit = cmd.Run([]string{"ipa-info", "--path", path, "--output", "table"}, "test") })
	if exit != cmd.ExitSuccess || !strings.Contains(stdout, artifactstest.SignerCommonName) || !strings.Contains(stdout, "Team ID") {
		t.Fatalf("exit=%d table=%q", exit, stdout)
	}
}

func TestRunIPAInfoReportsAdHocAndUnreadableSignaturesWithoutSigner(t *testing.T) {
	isolateArtifactCommandEnv(t)
	dir := t.TempDir()
	info := `<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.demo</string><key>CFBundleExecutable</key><string>Demo</string></dict></plist>`
	profile := `<plist version="1.0"><dict><key>Entitlements</key><dict/></dict></plist>`
	tests := []struct {
		name       string
		executable string
		want       string
		warning    string
	}{
		{"ad-hoc", string(artifactstest.MachO(artifactstest.Superblob(artifactstest.CodeDirectorySlot(), artifactstest.CMSSlot(nil)))), "ad-hoc", ""},
		{"unsigned", string(artifactstest.MachO(nil)), "unsigned", ""},
		{"unreadable", "not a Mach-O file", "unreadable", "Warning: ipa-info: code signature is unreadable: main executable is not a Mach-O file"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, test.name+".ipa")
			writeArtifactZip(t, path, map[string]string{
				"Payload/Demo.app/Info.plist":               info,
				"Payload/Demo.app/Demo":                     test.executable,
				"Payload/Demo.app/embedded.mobileprovision": profile,
			})
			var exit int
			stdout, stderr := captureOutput(t, func() { exit = cmd.Run([]string{"ipa-info", "--path", path, "--output", "json"}, "test") })
			if exit != cmd.ExitSuccess || strings.TrimSpace(stderr) != test.warning {
				t.Fatalf("exit=%d stderr=%q", exit, stderr)
			}
			var receipt map[string]any
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatal(err)
			}
			signer, present := receipt["signer"]
			if receipt["codeSignature"] != test.want || !present || signer != nil || receipt["signerCommonName"] != nil || receipt["teamId"] != nil {
				t.Fatalf("receipt=%v", receipt)
			}
		})
	}
}

func TestRunPKGInfoReportsSignerIdentity(t *testing.T) {
	isolateArtifactCommandEnv(t)
	chain := artifactstest.NewChain(t)
	dir := t.TempDir()
	info := map[string][]byte{"PackageInfo": []byte(`<pkg-info version="1.0" install-location="/Applications" identifier="com.example.pkg"><bundle id="com.example.demo"/></pkg-info>`)}
	signed := filepath.Join(dir, "signed.pkg")
	if err := os.WriteFile(signed, artifactstest.Xar(t, info, chain.XarSignature("signature")), 0o600); err != nil {
		t.Fatal(err)
	}
	unsigned := filepath.Join(dir, "unsigned.pkg")
	if err := os.WriteFile(unsigned, artifactstest.Xar(t, info, ""), 0o600); err != nil {
		t.Fatal(err)
	}
	var exit int
	stdout, stderr := captureOutput(t, func() { exit = cmd.Run([]string{"pkg-info", "--path", signed, "--output", "json"}, "test") })
	if exit != cmd.ExitSuccess || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	assertJSONEqual(t, stdout, map[string]any{
		"signatureVerification": "not-verified",
		"path":                  signed,
		"productId":             "com.example.pkg",
		"version":               "1.0",
		"installLocation":       "/Applications",
		"bundleIds":             []any{"com.example.demo"},
		"signerCommonName":      artifactstest.SignerCommonName,
		"teamId":                artifactstest.TeamID,
		"status":                "readable",
		"packageSignature":      "signed",
		"signer":                signerJSON(chain),
	})

	stdout, stderr = captureOutput(t, func() { exit = cmd.Run([]string{"pkg-info", "--path", unsigned, "--output", "json"}, "test") })
	if exit != cmd.ExitSuccess || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	assertJSONEqual(t, stdout, map[string]any{
		"signatureVerification": "not-verified",
		"path":                  unsigned,
		"productId":             "com.example.pkg",
		"version":               "1.0",
		"installLocation":       "/Applications",
		"bundleIds":             []any{"com.example.demo"},
		"status":                "readable",
		"packageSignature":      "unsigned",
		"signer":                nil,
	})
}

func TestRunPKGInfoWarnsAboutTheExpiringDeveloperIDAuthority(t *testing.T) {
	isolateArtifactCommandEnv(t)
	info := map[string][]byte{"PackageInfo": []byte(`<pkg-info version="1.0" identifier="com.example.pkg"/>`)}
	for _, test := range []struct {
		name        string
		unit        string
		wantStatus  string
		wantWarning bool
	}{
		{name: "previous authority", unit: "Apple Certification Authority", wantStatus: "affected", wantWarning: true},
		{name: "G2 authority", unit: "G2", wantStatus: "not-affected"},
	} {
		t.Run(test.name, func(t *testing.T) {
			chain := artifactstest.NewChainWithIssuer(t, pkix.Name{CommonName: "Developer ID Certification Authority", OrganizationalUnit: []string{test.unit}})
			path := filepath.Join(t.TempDir(), "signed.pkg")
			if err := os.WriteFile(path, artifactstest.Xar(t, info, chain.XarSignature("signature")), 0o600); err != nil {
				t.Fatal(err)
			}
			var exit int
			stdout, stderr := captureOutput(t, func() { exit = cmd.Run([]string{"pkg-info", "--path", path, "--output", "json"}, "test") })
			if exit != cmd.ExitSuccess {
				t.Fatalf("exit=%d stderr=%q", exit, stderr)
			}
			var receipt struct {
				Signer struct {
					IssuerOrganizationalUnit   string `json:"issuerOrganizationalUnit"`
					DeveloperIDAuthorityStatus string `json:"developerIdAuthorityStatus"`
				} `json:"signer"`
			}
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatalf("parse stdout %q: %v", stdout, err)
			}
			if receipt.Signer.IssuerOrganizationalUnit != test.unit || receipt.Signer.DeveloperIDAuthorityStatus != test.wantStatus {
				t.Fatalf("unexpected signer: %+v", receipt.Signer)
			}
			if got := strings.Contains(stderr, "will no longer install"); got != test.wantWarning {
				t.Fatalf("stderr=%q, wantWarning %t", stderr, test.wantWarning)
			}
		})
	}
}
