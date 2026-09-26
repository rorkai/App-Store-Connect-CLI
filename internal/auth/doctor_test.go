package auth

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestDoctorConfigPermissionsWarning(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write config error: %v", err)
	}
	t.Setenv("ASC_CONFIG_PATH", configPath)

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Storage")
	if !sectionHasStatus(section, DoctorWarn, "Config file permissions") {
		t.Fatalf("expected config permissions warning, got %#v", section.Checks)
	}

	Doctor(DoctorOptions{Fix: true})
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat config error: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("expected config permissions fixed to 0600, got %#o", info.Mode().Perm())
	}
}

func TestDoctorConfigPathRejectsSymlinkWithoutUnsafeRecommendation(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "config-target.json")
	if err := os.WriteFile(targetPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write config target error: %v", err)
	}
	configPath := filepath.Join(tempDir, "config.json")
	if err := os.Symlink(targetPath, configPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Setenv("ASC_CONFIG_PATH", configPath)

	report := Doctor(DoctorOptions{Fix: true})
	section := findDoctorSection(t, report, "Storage")
	if !sectionHasStatus(section, DoctorFail, "symbolic link") {
		t.Fatalf("expected config symlink rejection, got %#v", section.Checks)
	}
	for _, check := range section.Checks {
		if strings.Contains(check.Recommendation, "chmod") || check.FixApplied {
			t.Fatalf("unexpected symlink remediation: %#v", check)
		}
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat config target error: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("config target mode = %#o, want 0644", got)
	}
}

func TestDoctorConfigPathRejectsParentSymlinkWithoutUnsafeRecommendation(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "config-target")
	if err := os.Mkdir(targetDir, 0o700); err != nil {
		t.Fatalf("make config target dir error: %v", err)
	}
	targetPath := filepath.Join(targetDir, "config.json")
	if err := os.WriteFile(targetPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write config target error: %v", err)
	}
	linkDir := filepath.Join(tempDir, "config-link")
	if err := os.Symlink(targetDir, linkDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(linkDir, "config.json"))

	report := Doctor(DoctorOptions{Fix: true})
	section := findDoctorSection(t, report, "Storage")
	if !sectionHasStatus(section, DoctorFail, "symlink") {
		t.Fatalf("expected config parent symlink rejection, got %#v", section.Checks)
	}
	for _, check := range section.Checks {
		if strings.Contains(check.Recommendation, "chmod") || check.FixApplied {
			t.Fatalf("unexpected symlink remediation: %#v", check)
		}
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat config target error: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("config target mode = %#o, want 0644", got)
	}
}

func TestDoctorConfigPathRejectsSpecialFileWithoutUnsafeRecommendation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/dev/null is not a POSIX special file on Windows")
	}
	info, err := os.Lstat(os.DevNull)
	if err != nil {
		t.Fatalf("Lstat(%q) error: %v", os.DevNull, err)
	}
	if info.Mode().IsRegular() {
		t.Skipf("%s is a regular file on this platform", os.DevNull)
	}

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", os.DevNull)
	report := Doctor(DoctorOptions{Fix: true})
	section := findDoctorSection(t, report, "Storage")
	if !sectionHasStatus(section, DoctorFail, "not a regular file") {
		t.Fatalf("expected config special-file rejection, got %#v", section.Checks)
	}
	for _, check := range section.Checks {
		if strings.Contains(check.Recommendation, "chmod") || check.FixApplied {
			t.Fatalf("unexpected special-file remediation: %#v", check)
		}
	}
}

func TestDoctorStorageBypassMessageSupportsTruthyEnvValues(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "on")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Storage")
	if !sectionHasStatus(section, DoctorInfo, "Keychain is bypassed via ASC_BYPASS_KEYCHAIN") {
		t.Fatalf("expected bypass info message, got %#v", section.Checks)
	}
	for _, check := range section.Checks {
		if strings.Contains(check.Message, "ASC_BYPASS_KEYCHAIN=1") {
			t.Fatalf("expected no hardcoded '=1' in message, got %q", check.Message)
		}
	}
}

func TestDoctorEnvironmentRedactsCredentialIdentifiers(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_KEY_ID", "ABC123SECRET")
	t.Setenv("ASC_ISSUER_ID", "issuer-uuid")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")
	t.Setenv("ASC_PROFILE", "production")

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Environment")
	if !sectionHasStatus(section, DoctorInfo, "ASC_KEY_ID is set") {
		t.Fatalf("expected ASC_KEY_ID presence message, got %#v", section.Checks)
	}
	if !sectionHasStatus(section, DoctorInfo, "ASC_ISSUER_ID is set") {
		t.Fatalf("expected ASC_ISSUER_ID presence message, got %#v", section.Checks)
	}
	if !sectionHasStatus(section, DoctorInfo, "ASC_PROFILE is set (production)") {
		t.Fatalf("expected ASC_PROFILE value in message, got %#v", section.Checks)
	}
	for _, check := range section.Checks {
		if strings.Contains(check.Message, "ABC123SECRET") {
			t.Fatalf("ASC_KEY_ID leaked in message: %q", check.Message)
		}
		if strings.Contains(check.Message, "issuer-uuid") {
			t.Fatalf("ASC_ISSUER_ID leaked in message: %q", check.Message)
		}
	}
}

func TestDoctorEnvironmentValidatesSelectedPrivateKey(t *testing.T) {
	validKeyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, validKeyPath, 0o600, true)
	validKey, err := os.ReadFile(validKeyPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	tests := []struct {
		name            string
		keyPath         string
		privateKey      string
		privateKeyB64   string
		wantStatus      DoctorStatus
		wantMessage     string
		forbiddenSecret string
	}{
		{
			name:        "missing path",
			keyPath:     filepath.Join(t.TempDir(), "missing.p8"),
			wantStatus:  DoctorFail,
			wantMessage: "ASC_PRIVATE_KEY_PATH - file not found",
		},
		{
			name:            "invalid base64",
			privateKeyB64:   "not-base64-secret",
			wantStatus:      DoctorFail,
			wantMessage:     "ASC_PRIVATE_KEY_B64 is not valid base64",
			forbiddenSecret: "not-base64-secret",
		},
		{
			name:            "invalid raw pem",
			privateKey:      "not-a-private-key-secret",
			wantStatus:      DoctorFail,
			wantMessage:     "ASC_PRIVATE_KEY is not a valid private key",
			forbiddenSecret: "not-a-private-key-secret",
		},
		{
			name:        "mixed escaped and real newlines",
			privateKey:  strings.Replace(string(validKey), "\n", `\n`, 1),
			wantStatus:  DoctorOK,
			wantMessage: "ASC_PRIVATE_KEY contains a valid ECDSA private key",
		},
		{
			name:          "valid base64",
			privateKeyB64: base64.StdEncoding.EncodeToString(validKey),
			wantStatus:    DoctorOK,
			wantMessage:   "ASC_PRIVATE_KEY_B64 contains a valid ECDSA private key",
		},
		{
			name:          "path takes precedence",
			keyPath:       validKeyPath,
			privateKey:    "ignored-invalid-raw-key",
			privateKeyB64: "ignored-invalid-base64",
			wantStatus:    DoctorOK,
			wantMessage:   "ASC_PRIVATE_KEY_PATH - valid ECDSA key",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			t.Setenv("ASC_KEY_ID", "ENVKEY")
			t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
			t.Setenv("ASC_PRIVATE_KEY_PATH", test.keyPath)
			t.Setenv("ASC_PRIVATE_KEY", test.privateKey)
			t.Setenv("ASC_PRIVATE_KEY_B64", test.privateKeyB64)

			report := Doctor(DoctorOptions{})
			section := findDoctorSection(t, report, "Environment")
			if !sectionHasStatus(section, test.wantStatus, test.wantMessage) {
				t.Fatalf("expected %s check containing %q, got %#v", test.wantStatus, test.wantMessage, section.Checks)
			}
			if test.wantStatus == DoctorFail && report.Summary.Errors == 0 {
				t.Fatalf("expected failed key validation in summary, got %#v", report.Summary)
			}
			if test.forbiddenSecret != "" {
				for _, check := range section.Checks {
					if strings.Contains(check.Message, test.forbiddenSecret) || strings.Contains(check.Recommendation, test.forbiddenSecret) {
						t.Fatalf("private key material leaked in check: %#v", check)
					}
				}
			}
		})
	}
}

func TestDoctorEnvironmentRequiresWritablePrivateKeyTempDir(t *testing.T) {
	validKeyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, validKeyPath, 0o600, true)
	validKey, err := os.ReadFile(validKeyPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	tests := []struct {
		name          string
		privateKey    string
		privateKeyB64 string
		wantMessage   string
	}{
		{
			name:        "inline pem",
			privateKey:  string(validKey),
			wantMessage: "ASC_PRIVATE_KEY cannot be materialized as a temporary private key",
		},
		{
			name:          "base64 pem",
			privateKeyB64: base64.StdEncoding.EncodeToString(validKey),
			wantMessage:   "ASC_PRIVATE_KEY_B64 cannot be materialized as a temporary private key",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tempDir := t.TempDir()
			blockedTemp := filepath.Join(tempDir, "not-a-directory")
			if err := os.WriteFile(blockedTemp, []byte("blocked"), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			t.Setenv("TMPDIR", blockedTemp)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
			t.Setenv("ASC_PROFILE", "")
			t.Setenv("ASC_STRICT_AUTH", "")
			t.Setenv("ASC_KEY_TYPE", "")
			t.Setenv("ASC_KEY_ID", "ENVKEY")
			t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
			t.Setenv("ASC_PRIVATE_KEY_PATH", "")
			t.Setenv("ASC_PRIVATE_KEY", test.privateKey)
			t.Setenv("ASC_PRIVATE_KEY_B64", test.privateKeyB64)

			report := Doctor(DoctorOptions{})
			section := findDoctorSection(t, report, "Environment")
			if !sectionHasStatus(section, DoctorFail, test.wantMessage) {
				t.Fatalf("expected temp-key materialization failure, got %#v", section.Checks)
			}
			if report.Summary.Errors == 0 {
				t.Fatalf("expected materialization failure in summary, got %#v", report.Summary)
			}
		})
	}
}

func TestDoctorEnvironmentSkipsIgnoredPrivateKeys(t *testing.T) {
	withSeparateKeyrings(t)

	t.Run("selected profile", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)
		configPath := filepath.Join(tempDir, "config.json")
		if err := config.SaveAt(configPath, &config.Config{
			DefaultKeyName: "stored",
			Keys: []config.Credential{{
				Name:           "stored",
				KeyID:          "STOREDKEY",
				IssuerID:       "12345678-abcd-1234-abcd-123456789012",
				PrivateKeyPath: storedKeyPath,
			}},
		}); err != nil {
			t.Fatalf("SaveAt() error: %v", err)
		}

		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", configPath)
		t.Setenv("ASC_KEY_ID", "ENVKEY")
		t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
		t.Setenv("ASC_PRIVATE_KEY_PATH", filepath.Join(tempDir, "missing.p8"))

		report := Doctor(DoctorOptions{Profile: "stored"})
		section := findDoctorSection(t, report, "Environment")
		if sectionHasStatus(section, DoctorFail, "ASC_PRIVATE_KEY_PATH") {
			t.Fatalf("expected selected profile to suppress ignored environment key failure, got %#v", section.Checks)
		}
		if !sectionHasStatus(section, DoctorInfo, "ignored because profile \"stored\" provides stored private key material") {
			t.Fatalf("expected ignored environment key note, got %#v", section.Checks)
		}
	})

	t.Run("missing selected profile", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)
		configPath := filepath.Join(tempDir, "config.json")
		if err := config.SaveAt(configPath, &config.Config{
			DefaultKeyName: "other",
			Keys: []config.Credential{{
				Name:           "other",
				KeyID:          "STOREDKEY",
				IssuerID:       "12345678-abcd-1234-abcd-123456789012",
				PrivateKeyPath: storedKeyPath,
			}},
		}); err != nil {
			t.Fatalf("SaveAt() error: %v", err)
		}

		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", configPath)
		t.Setenv("ASC_KEY_ID", "ENVKEY")
		t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
		t.Setenv("ASC_PRIVATE_KEY_PATH", filepath.Join(tempDir, "unused-missing-env.p8"))

		report := Doctor(DoctorOptions{Profile: "missing"})
		section := findDoctorSection(t, report, "Environment")
		if !sectionHasStatus(section, DoctorFail, "Selected profile \"missing\" could not be resolved") {
			t.Fatalf("expected missing selected profile failure, got %#v", section.Checks)
		}
		if sectionHasStatus(section, DoctorFail, "ASC_PRIVATE_KEY_PATH") {
			t.Fatalf("expected unused environment key to remain ignored, got %#v", section.Checks)
		}
		if report.Summary.Errors == 0 {
			t.Fatalf("expected missing selected profile in error summary, got %#v", report.Summary)
		}
	})

	t.Run("incomplete selected profile without environment fallback", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)

		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "")
		t.Setenv("ASC_ISSUER_ID", "")
		t.Setenv("ASC_KEY_TYPE", "")
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", "")
		if err := StoreCredentials("doctor-incomplete-selected", "STOREDKEY", "", storedKeyPath); err != nil {
			t.Fatalf("StoreCredentials() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-incomplete-selected"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		report := Doctor(DoctorOptions{Profile: "doctor-incomplete-selected"})
		section := findDoctorSection(t, report, "Environment")
		if !sectionHasStatus(section, DoctorFail, "Selected profile \"doctor-incomplete-selected\" is incomplete after environment fallback (missing issuer ID)") {
			t.Fatalf("expected incomplete selected profile failure, got %#v", section.Checks)
		}
		if report.Summary.Errors == 0 {
			t.Fatalf("expected incomplete selected profile in error summary, got %#v", report.Summary)
		}
	})

	t.Run("selected profile validates required environment private key", func(t *testing.T) {
		tempDir := t.TempDir()

		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "")
		t.Setenv("ASC_ISSUER_ID", "")
		t.Setenv("ASC_KEY_TYPE", "")
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", filepath.Join(tempDir, "missing-env.p8"))
		if err := StoreCredentials("doctor-selected-needs-key", "STOREDKEY", "12345678-abcd-1234-abcd-123456789012", ""); err != nil {
			t.Fatalf("StoreCredentials() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-selected-needs-key"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		report := Doctor(DoctorOptions{Profile: "doctor-selected-needs-key"})
		section := findDoctorSection(t, report, "Environment")
		if !sectionHasStatus(section, DoctorFail, "ASC_PRIVATE_KEY_PATH - file not found") {
			t.Fatalf("expected required environment key validation, got %#v", section.Checks)
		}
	})

	t.Run("selected profile accepts individual environment key type", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)

		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "")
		t.Setenv("ASC_ISSUER_ID", "")
		t.Setenv("ASC_KEY_TYPE", config.CredentialKeyTypeIndividual)
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", "")
		if err := StoreCredentials("doctor-selected-individual-fallback", "STOREDKEY", "", storedKeyPath); err != nil {
			t.Fatalf("StoreCredentials() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-selected-individual-fallback"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		report := Doctor(DoctorOptions{Profile: "doctor-selected-individual-fallback"})
		section := findDoctorSection(t, report, "Environment")
		if sectionHasStatus(section, DoctorFail, "Selected profile \"doctor-selected-individual-fallback\"") {
			t.Fatalf("expected individual environment key type to remove issuer requirement, got %#v", section.Checks)
		}
		if report.Summary.Errors != 0 {
			t.Fatalf("expected no doctor errors for effective individual credentials, got %#v", report.Summary)
		}
	})

	t.Run("selected profile preserves unsupported stored key type", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)

		if err := StoreCredentialsWithKeyType("doctor-selected-unsupported-key-type", "STOREDKEY", "", storedKeyPath, "personal"); err != nil {
			t.Fatalf("StoreCredentialsWithKeyType() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-selected-unsupported-key-type"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "")
		t.Setenv("ASC_ISSUER_ID", "")
		t.Setenv("ASC_KEY_TYPE", config.CredentialKeyTypeIndividual)
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", "")

		report := Doctor(DoctorOptions{Profile: "doctor-selected-unsupported-key-type"})
		section := findDoctorSection(t, report, "Environment")
		if !sectionHasStatus(section, DoctorFail, "Selected profile \"doctor-selected-unsupported-key-type\" is incomplete after environment fallback (missing issuer ID)") {
			t.Fatalf("expected stored key type to prevent environment type fallback, got %#v", section.Checks)
		}
	})

	t.Run("selected profile rejects invalid environment key type", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)

		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "")
		t.Setenv("ASC_ISSUER_ID", "")
		t.Setenv("ASC_KEY_TYPE", "invalid")
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", "")
		if err := StoreCredentials("doctor-selected-invalid-key-type", "STOREDKEY", "", storedKeyPath); err != nil {
			t.Fatalf("StoreCredentials() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-selected-invalid-key-type"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		report := Doctor(DoctorOptions{Profile: "doctor-selected-invalid-key-type"})
		section := findDoctorSection(t, report, "Environment")
		if !sectionHasStatus(section, DoctorFail, "Selected profile \"doctor-selected-invalid-key-type\" cannot use environment fallback: ASC_KEY_TYPE must be team or individual") {
			t.Fatalf("expected invalid fallback key type failure, got %#v", section.Checks)
		}
	})

	t.Run("selected profile rejects mixed sources in strict auth", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)

		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "")
		t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
		t.Setenv("ASC_KEY_TYPE", "")
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", "")
		if err := StoreCredentials("doctor-selected-strict-mixed", "STOREDKEY", "", storedKeyPath); err != nil {
			t.Fatalf("StoreCredentials() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-selected-strict-mixed"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		report := Doctor(DoctorOptions{Profile: "doctor-selected-strict-mixed", StrictAuth: true})
		section := findDoctorSection(t, report, "Environment")
		if !sectionHasStatus(section, DoctorFail, "Selected profile \"doctor-selected-strict-mixed\" requires mixed stored and environment credential sources while strict authentication is enabled") {
			t.Fatalf("expected strict mixed-source failure, got %#v", section.Checks)
		}
	})

	t.Run("default profile rejects mixed sources in strict auth", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)

		if err := StoreCredentials("doctor-default-strict-mixed", "STOREDKEY", "", storedKeyPath); err != nil {
			t.Fatalf("StoreCredentials() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-default-strict-mixed"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "")
		t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
		t.Setenv("ASC_KEY_TYPE", "")
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", "")

		report := Doctor(DoctorOptions{StrictAuth: true})
		section := findDoctorSection(t, report, "Environment")
		if !sectionHasStatus(section, DoctorFail, "Default stored credentials require mixed stored and environment credential sources while strict authentication is enabled") {
			t.Fatalf("expected strict mixed-source failure for default credentials, got %#v", section.Checks)
		}
	})

	t.Run("default profile preserves unsupported stored key type", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)

		if err := StoreCredentialsWithKeyType("doctor-default-unsupported-key-type", "STOREDKEY", "", storedKeyPath, "personal"); err != nil {
			t.Fatalf("StoreCredentialsWithKeyType() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-default-unsupported-key-type"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "")
		t.Setenv("ASC_ISSUER_ID", "")
		t.Setenv("ASC_KEY_TYPE", config.CredentialKeyTypeIndividual)
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", "")

		report := Doctor(DoctorOptions{})
		section := findDoctorSection(t, report, "Environment")
		if !sectionHasStatus(section, DoctorFail, "Default stored credentials are incomplete after environment fallback (missing issuer ID)") {
			t.Fatalf("expected stored key type to prevent default environment type fallback, got %#v", section.Checks)
		}
	})

	t.Run("default profile rejects invalid environment key type", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)

		if err := StoreCredentials("doctor-default-invalid-env-key-type", "STOREDKEY", "", storedKeyPath); err != nil {
			t.Fatalf("StoreCredentials() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-default-invalid-env-key-type"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "")
		t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
		t.Setenv("ASC_KEY_TYPE", "invalid")
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", "")

		report := Doctor(DoctorOptions{})
		section := findDoctorSection(t, report, "Environment")
		if !sectionHasStatus(section, DoctorFail, "Default stored credentials cannot use environment fallback: ASC_KEY_TYPE must be team or individual") {
			t.Fatalf("expected invalid default fallback key type failure, got %#v", section.Checks)
		}
	})

	t.Run("default strict auth checks failed environment materialization", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)
		rawKey, err := os.ReadFile(storedKeyPath)
		if err != nil {
			t.Fatalf("ReadFile() error: %v", err)
		}
		blockedTemp := filepath.Join(tempDir, "not-a-directory")
		if err := os.WriteFile(blockedTemp, []byte("blocked"), 0o600); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}

		if err := StoreCredentials("doctor-default-materialization-strict", "STOREDKEY", "", storedKeyPath); err != nil {
			t.Fatalf("StoreCredentials() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-default-materialization-strict"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		t.Setenv("TMPDIR", blockedTemp)
		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "ENVKEY")
		t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
		t.Setenv("ASC_KEY_TYPE", "")
		t.Setenv("ASC_PRIVATE_KEY", string(rawKey))
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", "")

		report := Doctor(DoctorOptions{StrictAuth: true})
		section := findDoctorSection(t, report, "Environment")
		if !sectionHasStatus(section, DoctorFail, "Default stored credentials require mixed stored and environment credential sources while strict authentication is enabled") {
			t.Fatalf("expected strict fallback check after environment materialization failure, got %#v", section.Checks)
		}
	})

	t.Run("complete bypass config", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)
		configPath := filepath.Join(tempDir, "config.json")
		if err := config.SaveAt(configPath, &config.Config{
			DefaultKeyName: "stored",
			Keys: []config.Credential{{
				Name:           "stored",
				KeyID:          "STOREDKEY",
				IssuerID:       "12345678-abcd-1234-abcd-123456789012",
				PrivateKeyPath: storedKeyPath,
			}},
		}); err != nil {
			t.Fatalf("SaveAt() error: %v", err)
		}

		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", configPath)
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "ENVKEY")
		t.Setenv("ASC_ISSUER_ID", "87654321-abcd-1234-abcd-123456789012")
		t.Setenv("ASC_PRIVATE_KEY_PATH", filepath.Join(tempDir, "missing-env.p8"))

		report := Doctor(DoctorOptions{})
		section := findDoctorSection(t, report, "Environment")
		if sectionHasStatus(section, DoctorFail, "ASC_PRIVATE_KEY_PATH") {
			t.Fatalf("expected complete bypass config to suppress ignored environment key failure, got %#v", section.Checks)
		}
		if !sectionHasStatus(section, DoctorInfo, "ignored because complete stored config credentials are selected") {
			t.Fatalf("expected bypass config ignore note, got %#v", section.Checks)
		}
	})

	t.Run("complete default keychain credentials", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)

		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "")
		t.Setenv("ASC_ISSUER_ID", "")
		t.Setenv("ASC_KEY_TYPE", "")
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", filepath.Join(tempDir, "missing-env.p8"))
		if err := StoreCredentials("doctor-default-keychain", "STOREDKEY", "12345678-abcd-1234-abcd-123456789012", storedKeyPath); err != nil {
			t.Fatalf("StoreCredentials() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-default-keychain"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		report := Doctor(DoctorOptions{})
		section := findDoctorSection(t, report, "Environment")
		if sectionHasStatus(section, DoctorFail, "ASC_PRIVATE_KEY_PATH") {
			t.Fatalf("expected complete default keychain credentials to suppress ignored environment key failure, got %#v", section.Checks)
		}
		if !sectionHasStatus(section, DoctorInfo, "ignored because complete default stored credentials are selected") {
			t.Fatalf("expected default stored credentials ignore note, got %#v", section.Checks)
		}
	})

	t.Run("default keychain key with environment issuer fallback", func(t *testing.T) {
		tempDir := t.TempDir()
		storedKeyPath := filepath.Join(tempDir, "stored.p8")
		writeECDSAPEM(t, storedKeyPath, 0o600, true)

		t.Setenv("ASC_BYPASS_KEYCHAIN", "")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
		t.Setenv("ASC_PROFILE", "")
		t.Setenv("ASC_KEY_ID", "")
		t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
		t.Setenv("ASC_KEY_TYPE", "")
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")
		t.Setenv("ASC_PRIVATE_KEY_PATH", filepath.Join(tempDir, "unused-missing-env.p8"))
		if err := StoreCredentials("doctor-partial-keychain", "STOREDKEY", "", storedKeyPath); err != nil {
			t.Fatalf("StoreCredentials() error: %v", err)
		}
		t.Cleanup(func() {
			if err := RemoveCredentials("doctor-partial-keychain"); err != nil {
				t.Errorf("RemoveCredentials() error: %v", err)
			}
		})

		report := Doctor(DoctorOptions{})
		section := findDoctorSection(t, report, "Environment")
		if sectionHasStatus(section, DoctorFail, "ASC_PRIVATE_KEY_PATH") {
			t.Fatalf("expected selected default key material to suppress unused environment key failure, got %#v", section.Checks)
		}
		if !sectionHasStatus(section, DoctorInfo, "ignored because default stored private key is selected") {
			t.Fatalf("expected default stored key material ignore note, got %#v", section.Checks)
		}
	})
}

func TestDoctorEnvironmentPrivateKeyPathRedactsEveryOccurrence(t *testing.T) {
	path := `/private/ci/secret\AuthKey.p8`
	check := DoctorCheck{
		Message:        path + " - failed to read: open " + path + ": permission denied",
		Recommendation: "Run: chmod 600 " + strconv.Quote(path),
	}
	redactEnvironmentPrivateKeyPath(&check, path)
	if strings.Contains(check.Message, path) || strings.Contains(check.Recommendation, path) {
		t.Fatalf("expected every private key path occurrence to be redacted, got %#v", check)
	}
	if strings.Count(check.Message, "ASC_PRIVATE_KEY_PATH") != 2 || check.Recommendation != `Run: chmod 600 "$ASC_PRIVATE_KEY_PATH"` {
		t.Fatalf("expected repeated path occurrences to remain understandable after redaction, got %#v", check)
	}
}

func TestDoctorEnvironmentWarnsForInvalidKeyType(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_KEY_TYPE", "personal")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Environment")
	if !sectionHasStatus(section, DoctorWarn, "ASC_KEY_TYPE is invalid") {
		t.Fatalf("expected invalid ASC_KEY_TYPE warning, got %#v", section.Checks)
	}
	if !sectionHasStatus(section, DoctorWarn, "Environment credentials are incomplete") {
		t.Fatalf("expected incomplete environment warning, got %#v", section.Checks)
	}
}

func TestDoctorEnvironmentWarnsWhenCredentialIdentifiersLookSwapped(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_KEY_ID", "69a6de00-aaaa-bbbb-cccc-123456789abc")
	t.Setenv("ASC_ISSUER_ID", "39MX87M9Y4")
	t.Setenv("ASC_KEY_TYPE", "team")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Environment")
	if !sectionHasStatus(section, DoctorWarn, "ASC_KEY_ID looks like an issuer ID — the values may be swapped") {
		t.Fatalf("expected swapped key ID warning, got %#v", section.Checks)
	}
	if !sectionHasStatus(section, DoctorWarn, "ASC_ISSUER_ID looks like a key ID — the values may be swapped") {
		t.Fatalf("expected swapped issuer ID warning, got %#v", section.Checks)
	}
	for _, check := range section.Checks {
		if strings.Contains(check.Message, "69a6de00") || strings.Contains(check.Message, "39MX87M9Y4") {
			t.Fatalf("credential identifier leaked in message: %q", check.Message)
		}
	}
}

func TestDoctorEnvironmentWarnsForNonUUIDIssuerID(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_KEY_ID", "39MX87M9Y4")
	t.Setenv("ASC_ISSUER_ID", "not-a-uuid")
	t.Setenv("ASC_KEY_TYPE", "team")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Environment")
	if !sectionHasStatus(section, DoctorWarn, "ASC_ISSUER_ID is not a UUID") {
		t.Fatalf("expected issuer ID shape warning, got %#v", section.Checks)
	}
	if sectionHasStatus(section, DoctorWarn, "ASC_KEY_ID looks like an issuer ID") {
		t.Fatalf("expected no key ID warning for a plausible key ID, got %#v", section.Checks)
	}
}

func TestDoctorEnvironmentIgnoresIssuerShapeForIndividualKey(t *testing.T) {
	for _, test := range []struct {
		name     string
		keyID    string
		issuerID string
	}{
		{name: "stale issuer is ignored", keyID: "39MX87M9Y4", issuerID: "stale-team-value"},
		{name: "uuid-shaped key without issuer", keyID: "69a6de00-aaaa-bbbb-cccc-123456789abc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			t.Setenv("ASC_KEY_ID", test.keyID)
			t.Setenv("ASC_ISSUER_ID", test.issuerID)
			t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")
			t.Setenv("ASC_KEY_TYPE", "individual")

			report := Doctor(DoctorOptions{})
			section := findDoctorSection(t, report, "Environment")
			for _, check := range section.Checks {
				if check.Status == DoctorWarn && (strings.Contains(check.Message, "ASC_ISSUER_ID") || strings.Contains(check.Message, "issuer ID") || strings.Contains(check.Message, "swapped")) {
					t.Fatalf("unexpected team credential warning for individual key: %q", check.Message)
				}
			}
		})
	}
}

func TestDoctorEnvironmentAcceptsUnusualButValidCredentialShapes(t *testing.T) {
	for _, test := range []struct {
		name     string
		keyID    string
		issuerID string
	}{
		{name: "digits only key id", keyID: "1234567890", issuerID: "69a6de00-aaaa-bbbb-cccc-123456789abc"},
		{name: "mixed case key id", keyID: "39Mx87M9y4", issuerID: "09f4080c-6ee7-4e52-8103-e1241eaaa58a"},
		{name: "uppercase issuer uuid", keyID: "39MX87M9Y4", issuerID: "A7EFEF21-3432-404F-A488-083800B570FF"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			t.Setenv("ASC_KEY_ID", test.keyID)
			t.Setenv("ASC_ISSUER_ID", test.issuerID)
			t.Setenv("ASC_KEY_TYPE", "team")
			t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")

			report := Doctor(DoctorOptions{})
			section := findDoctorSection(t, report, "Environment")
			for _, check := range section.Checks {
				if check.Status == DoctorWarn && (strings.Contains(check.Message, "swapped") || strings.Contains(check.Message, "not a UUID")) {
					t.Fatalf("unexpected credential shape warning: %q", check.Message)
				}
			}
		})
	}
}

func TestDoctorTempFilesWarns(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tempFile, err := os.CreateTemp(os.TempDir(), "asc-key-*.p8")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	tempFile.Close()
	t.Cleanup(func() {
		_ = os.Remove(tempFile.Name())
	})

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Temp Files")
	if !sectionHasStatus(section, DoctorWarn, "orphaned temp key file") {
		t.Fatalf("expected temp file warning, got %#v", section.Checks)
	}
}

func TestDoctorPrivateKeyPathRejectsSpecialFiles(t *testing.T) {
	info, err := os.Stat(os.DevNull)
	if err != nil {
		t.Fatalf("Stat(%q) error: %v", os.DevNull, err)
	}
	if info.Mode().IsRegular() {
		t.Skipf("%s is a regular file on this platform", os.DevNull)
	}

	check := inspectPrivateKeyPath(os.DevNull, DoctorOptions{})
	if check.Status != DoctorFail || !strings.Contains(check.Message, "not a regular file") {
		t.Fatalf("expected special-file rejection, got %#v", check)
	}
}

func TestDoctorPrivateKeyPathRejectsSymlinkWithoutUnsafeRecommendation(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "AuthKey-target.p8")
	writeECDSAPEM(t, targetPath, 0o600, true)
	if err := os.Chmod(targetPath, 0o644); err != nil {
		t.Fatalf("chmod key target error: %v", err)
	}
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	if err := os.Symlink(targetPath, keyPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	check := inspectPrivateKeyPath(keyPath, DoctorOptions{Fix: true})
	if check.Status != DoctorFail || !strings.Contains(check.Message, "symbolic link") {
		t.Fatalf("expected private key symlink rejection, got %#v", check)
	}
	if strings.Contains(check.Recommendation, "chmod") || check.FixApplied {
		t.Fatalf("unexpected symlink remediation: %#v", check)
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat key target error: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("key target mode = %#o, want 0644", got)
	}
}

func TestDoctorPrivateKeyPathRejectsParentSymlinkWithoutUnsafeRecommendation(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "key-target")
	if err := os.Mkdir(targetDir, 0o700); err != nil {
		t.Fatalf("make key target dir error: %v", err)
	}
	targetPath := filepath.Join(targetDir, "AuthKey.p8")
	writeECDSAPEM(t, targetPath, 0o600, true)
	if err := os.Chmod(targetPath, 0o644); err != nil {
		t.Fatalf("chmod key target error: %v", err)
	}
	linkDir := filepath.Join(tempDir, "key-link")
	if err := os.Symlink(targetDir, linkDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	check := inspectPrivateKeyPath(filepath.Join(linkDir, "AuthKey.p8"), DoctorOptions{Fix: true})
	if check.Status != DoctorFail || !strings.Contains(check.Message, "symlink") {
		t.Fatalf("expected private key parent symlink rejection, got %#v", check)
	}
	if strings.Contains(check.Recommendation, "chmod") || check.FixApplied {
		t.Fatalf("unexpected symlink remediation: %#v", check)
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat key target error: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("key target mode = %#o, want 0644", got)
	}
}

func TestDoctorPrivateKeyPermissionsFix(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	if err := os.Chmod(keyPath, 0o644); err != nil {
		t.Fatalf("chmod key error: %v", err)
	}

	cfg := &config.Config{
		DefaultKeyName: "test",
		Keys: []config.Credential{
			{
				Name:           "test",
				KeyID:          "KEY123",
				IssuerID:       "ISS456",
				PrivateKeyPath: keyPath,
			},
		},
	}
	configPath := filepath.Join(tempDir, "config.json")
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("save config error: %v", err)
	}
	t.Setenv("ASC_CONFIG_PATH", configPath)

	report := Doctor(DoctorOptions{Fix: true})
	section := findDoctorSection(t, report, "Private Keys")
	if !sectionHasStatus(section, DoctorOK, "permissions fixed to 0600") {
		t.Fatalf("expected private key permissions fix, got %#v", section.Checks)
	}
}

func TestDoctorPrivateKeys_KeychainPEMWithoutFileStillPasses(t *testing.T) {
	_, _ = withSeparateKeyrings(t)

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	if err := StoreCredentials("keychain-only", "KEY123", "ISS456", keyPath); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}
	credentials, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(credentials) == 0 {
		t.Fatal("expected stored keychain credentials before file removal")
	}
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("Remove(%q) error: %v", keyPath, err)
	}
	credentialsAfterRemove, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() after remove error: %v", err)
	}
	if len(credentialsAfterRemove) == 0 {
		t.Fatal("expected keychain credentials after source key file removal")
	}
	if strings.TrimSpace(credentialsAfterRemove[0].PrivateKeyPEM) == "" {
		t.Fatalf("expected keychain credentials with private key PEM, got %#v", credentialsAfterRemove[0])
	}

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Private Keys")
	if !sectionHasStatus(section, DoctorOK, "valid private key stored in keychain") {
		t.Fatalf("expected keychain PEM success check, got %#v", section.Checks)
	}
	if sectionHasStatus(section, DoctorFail, "file not found") {
		t.Fatalf("expected no file-not-found failure for keychain PEM, got %#v", section.Checks)
	}
}

func TestDoctorMigrationHintsDetected(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("create .git error: %v", err)
	}
	fastlaneDir := filepath.Join(repo, "fastlane")
	if err := os.MkdirAll(fastlaneDir, 0o755); err != nil {
		t.Fatalf("mkdir fastlane error: %v", err)
	}

	secretValue := "SECRET_TOKEN_123"
	appfile := `app_identifier "com.example.app"
apple_id "user@example.com"
team_id "TEAM123"
`
	if err := os.WriteFile(filepath.Join(fastlaneDir, "Appfile"), []byte(appfile), 0o644); err != nil {
		t.Fatalf("write Appfile error: %v", err)
	}
	fastfile := `platform :ios do
  app_store_connect_api_key(
    key_content: "` + secretValue + `"
  )
  deliver
  upload_to_testflight
  app_store_build_number
end
`
	if err := os.WriteFile(filepath.Join(fastlaneDir, "Fastfile"), []byte(fastfile), 0o644); err != nil {
		t.Fatalf("write Fastfile error: %v", err)
	}

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
	clearMigrationTestEnv(t)

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Migration Hints")
	if !sectionHasStatus(section, DoctorInfo, "Detected Appfile") {
		t.Fatalf("expected Appfile detection, got %#v", section.Checks)
	}
	if !sectionHasStatus(section, DoctorInfo, "Detected Fastfile") {
		t.Fatalf("expected Fastfile detection, got %#v", section.Checks)
	}
	if !sectionHasStatus(section, DoctorInfo, "keys: app_identifier") {
		t.Fatalf("expected Appfile keys in output, got %#v", section.Checks)
	}
	if !sectionHasStatus(section, DoctorInfo, "actions: app_store_connect_api_key") {
		t.Fatalf("expected Fastfile actions in output, got %#v", section.Checks)
	}

	if report.Migration == nil {
		t.Fatal("expected migration hints in report")
	}
	expectedActions := []string{
		"app_store_connect_api_key",
		"deliver",
		"upload_to_testflight",
		"app_store_build_number",
	}
	if !reflect.DeepEqual(report.Migration.DetectedActions, expectedActions) {
		t.Fatalf("DetectedActions = %#v, want %#v", report.Migration.DetectedActions, expectedActions)
	}

	expectedCommands := []string{
		`asc auth login --name "MyKey" --key-id "KEY_ID" --issuer-id "ISSUER_ID" --private-key /path/to/AuthKey.p8`,
		"asc migrate validate --fastlane-dir ./fastlane",
		`asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir ./fastlane --confirm`,
		`asc builds info --app "APP_ID" --latest`,
		`asc publish testflight --app "APP_ID" --ipa app.ipa --group "GROUP_ID"`,
	}
	if !reflect.DeepEqual(report.Migration.SuggestedCommands, expectedCommands) {
		t.Fatalf("SuggestedCommands = %#v, want %#v", report.Migration.SuggestedCommands, expectedCommands)
	}

	assertNoSecretInDoctorReport(t, report, secretValue)
}

func TestDoctorMigrationHintsMissingFilesInfoOnly(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("create .git error: %v", err)
	}

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
	clearMigrationTestEnv(t)

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Migration Hints")
	if len(section.Checks) == 0 {
		t.Fatal("expected migration hints checks")
	}
	for _, check := range section.Checks {
		if check.Status != DoctorInfo {
			t.Fatalf("expected info-only checks, got %#v", section.Checks)
		}
	}
	if report.Migration == nil {
		t.Fatal("expected migration hints in report")
	}
	if report.Migration.DetectedFiles == nil {
		t.Fatal("expected detected files to be an empty array, got nil")
	}
	if report.Migration.DetectedActions == nil {
		t.Fatal("expected detected actions to be an empty array, got nil")
	}
	if report.Migration.SuggestedCommands == nil {
		t.Fatal("expected suggested commands to be an empty array, got nil")
	}
	if len(report.Migration.DetectedFiles) != 0 {
		t.Fatalf("expected no detected files, got %#v", report.Migration.DetectedFiles)
	}
	if len(report.Migration.DetectedActions) != 0 {
		t.Fatalf("expected no detected actions, got %#v", report.Migration.DetectedActions)
	}
	if len(report.Migration.SuggestedCommands) != 0 {
		t.Fatalf("expected no suggested commands, got %#v", report.Migration.SuggestedCommands)
	}
}

func TestDoctorMigrationHintsDetectsFromNestedWorktreePath(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: /tmp/worktree\n"), 0o644); err != nil {
		t.Fatalf("write .git marker error: %v", err)
	}
	fastlaneDir := filepath.Join(repo, "fastlane")
	if err := os.MkdirAll(fastlaneDir, 0o755); err != nil {
		t.Fatalf("mkdir fastlane error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fastlaneDir, "Fastfile"), []byte("deliver\n"), 0o644); err != nil {
		t.Fatalf("write Fastfile error: %v", err)
	}

	nestedDir := filepath.Join(repo, "a", "b", "c", "d", "e", "f", "g", "h", "i", "j")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir error: %v", err)
	}

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(nestedDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
	clearMigrationTestEnv(t)

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Migration Hints")
	if !sectionHasStatus(section, DoctorInfo, "Detected Fastfile at fastlane/Fastfile") {
		t.Fatalf("expected Fastfile detection from nested path, got %#v", section.Checks)
	}
	if report.Migration == nil {
		t.Fatal("expected migration hints in report")
	}
	if !reflect.DeepEqual(report.Migration.DetectedFiles, []string{"fastlane/Fastfile"}) {
		t.Fatalf("DetectedFiles = %#v, want %#v", report.Migration.DetectedFiles, []string{"fastlane/Fastfile"})
	}
	if !reflect.DeepEqual(report.Migration.DetectedActions, []string{"deliver"}) {
		t.Fatalf("DetectedActions = %#v, want %#v", report.Migration.DetectedActions, []string{"deliver"})
	}
}

func TestDoctorMigrationHintsPrefillsVersionFromXcodeAndAppID(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("create .git error: %v", err)
	}
	fastlaneDir := filepath.Join(repo, "fastlane")
	if err := os.MkdirAll(fastlaneDir, 0o755); err != nil {
		t.Fatalf("mkdir fastlane error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fastlaneDir, "Appfile"), []byte(`app_identifier "com.example.app"`), 0o644); err != nil {
		t.Fatalf("write Appfile error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fastlaneDir, "Fastfile"), []byte("upload_to_app_store\napp_store_build_number\n"), 0o644); err != nil {
		t.Fatalf("write Fastfile error: %v", err)
	}

	xcodeprojDir := filepath.Join(repo, "Sample.xcodeproj")
	if err := os.MkdirAll(xcodeprojDir, 0o755); err != nil {
		t.Fatalf("mkdir xcodeproj error: %v", err)
	}
	pbxproj := `
		buildSettings = {
			MARKETING_VERSION = 2.3.4;
		};
	`
	if err := os.WriteFile(filepath.Join(xcodeprojDir, "project.pbxproj"), []byte(pbxproj), 0o644); err != nil {
		t.Fatalf("write pbxproj error: %v", err)
	}

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
	t.Setenv("ASC_APP_ID", "123456789")
	clearMigrationTestEnv(t)
	t.Setenv("ASC_APP_ID", "123456789")

	report := Doctor(DoctorOptions{})
	section := findDoctorSection(t, report, "Migration Hints")
	if !sectionHasStatus(section, DoctorInfo, `Detected MARKETING_VERSION "2.3.4"`) {
		t.Fatalf("expected MARKETING_VERSION detection, got %#v", section.Checks)
	}
	if report.Migration == nil {
		t.Fatal("expected migration hints in report")
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc builds info --app "123456789" --latest`) {
		t.Fatalf("expected personalized app id in builds info latest suggestion, got %#v", report.Migration.SuggestedCommands)
	}
	if sliceContains(report.Migration.SuggestedCommands, `asc release run --app "123456789" --version "2.3.4" --build "BUILD_ID" --metadata-dir "./metadata/version/2.3.4" --confirm`) {
		t.Fatalf("expected upload-only migration hints to avoid non-actionable release run guidance, got %#v", report.Migration.SuggestedCommands)
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc validate --app "123456789" --version-id "VERSION_ID"`) {
		t.Fatalf("expected personalized validate command, got %#v", report.Migration.SuggestedCommands)
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc builds upload --app "123456789" --ipa app.ipa --version "2.3.4" --build-number "BUILD_NUMBER" --wait`) {
		t.Fatalf("expected upload step for upload-only migration hints, got %#v", report.Migration.SuggestedCommands)
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc builds info --app "123456789" --build-number "BUILD_NUMBER" --version "2.3.4"`) {
		t.Fatalf("expected build lookup step for upload-only migration hints, got %#v", report.Migration.SuggestedCommands)
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc versions create --app "123456789" --version "2.3.4"`) {
		t.Fatalf("expected personalized version create command, got %#v", report.Migration.SuggestedCommands)
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc review submit --app "123456789" --version-id "VERSION_ID" --build-id "UPLOADED_BUILD_ID" --platform "PLATFORM" --confirm`) {
		t.Fatalf("expected review submit step for upload-only migration hints, got %#v", report.Migration.SuggestedCommands)
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc versions attach-build --version-id "VERSION_ID" --build-id "UPLOADED_BUILD_ID"`) {
		t.Fatalf("expected attach-build guidance before validate, got %#v", report.Migration.SuggestedCommands)
	}
	attachIdx := sliceIndex(report.Migration.SuggestedCommands, `asc versions attach-build --version-id "VERSION_ID" --build-id "UPLOADED_BUILD_ID"`)
	validateIdx := sliceIndex(report.Migration.SuggestedCommands, `asc validate --app "123456789" --version-id "VERSION_ID"`)
	reviewSubmitIdx := sliceIndex(report.Migration.SuggestedCommands, `asc review submit --app "123456789" --version-id "VERSION_ID" --build-id "UPLOADED_BUILD_ID" --platform "PLATFORM" --confirm`)
	if attachIdx < 0 || validateIdx <= attachIdx || reviewSubmitIdx <= validateIdx {
		t.Fatalf("expected attach-build -> validate -> review submit ordering, got %#v", report.Migration.SuggestedCommands)
	}
	if sliceContains(report.Migration.SuggestedCommands, `asc review submissions-create --app "123456789" --platform "PLATFORM"`) {
		t.Fatalf("expected upload-only migration hints to avoid the old multi-step review submission guidance, got %#v", report.Migration.SuggestedCommands)
	}
	if sliceContains(report.Migration.SuggestedCommands, `asc review items-add --submission "REVIEW_SUBMISSION_ID" --item-type appStoreVersions --item-id "VERSION_ID"`) {
		t.Fatalf("expected upload-only migration hints to avoid the old multi-step review submission guidance, got %#v", report.Migration.SuggestedCommands)
	}
	if sliceContains(report.Migration.SuggestedCommands, `asc review submissions-submit --id "REVIEW_SUBMISSION_ID" --confirm`) {
		t.Fatalf("expected upload-only migration hints to avoid the old multi-step review submission guidance, got %#v", report.Migration.SuggestedCommands)
	}
	if sliceContains(report.Migration.SuggestedCommands, `asc submit create --app "123456789" --version "2.3.4" --build "BUILD_ID" --confirm`) {
		t.Fatalf("expected upload-only migration hints to avoid deprecated submit create guidance, got %#v", report.Migration.SuggestedCommands)
	}
	if sliceContains(report.Migration.SuggestedCommands, `asc submit preflight --app "123456789" --version "2.3.4"`) {
		t.Fatalf("expected upload-only migration hints to avoid deprecated submit preflight guidance, got %#v", report.Migration.SuggestedCommands)
	}
}

func TestDoctorMigrationHintsUsesResolvedIDsWhenLookupSucceeds(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("create .git error: %v", err)
	}
	fastlaneDir := filepath.Join(repo, "fastlane")
	if err := os.MkdirAll(fastlaneDir, 0o755); err != nil {
		t.Fatalf("mkdir fastlane error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fastlaneDir, "Appfile"), []byte(`app_identifier "com.example.app"`), 0o644); err != nil {
		t.Fatalf("write Appfile error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fastlaneDir, "Fastfile"), []byte("deliver\nupload_to_app_store\napp_store_build_number\n"), 0o644); err != nil {
		t.Fatalf("write Fastfile error: %v", err)
	}

	xcodeprojDir := filepath.Join(repo, "Sample.xcodeproj")
	if err := os.MkdirAll(xcodeprojDir, 0o755); err != nil {
		t.Fatalf("mkdir xcodeproj error: %v", err)
	}
	pbxproj := `
		buildSettings = {
			MARKETING_VERSION = 4.5.6;
		};
	`
	if err := os.WriteFile(filepath.Join(xcodeprojDir, "project.pbxproj"), []byte(pbxproj), 0o644); err != nil {
		t.Fatalf("write pbxproj error: %v", err)
	}

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
	clearMigrationTestEnv(t)

	called := false
	resolver := func(input MigrationSuggestionResolverInput) MigrationSuggestionResolverOutput {
		called = true
		return MigrationSuggestionResolverOutput{
			AppID:     "987654321",
			VersionID: "version-id-123",
			BuildID:   "build-id-456",
		}
	}

	report := DoctorWithMigrationResolver(DoctorOptions{}, resolver)
	if !called {
		t.Fatal("expected migration remote resolver to be called")
	}
	if report.Migration == nil {
		t.Fatal("expected migration hints in report")
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc migrate import --app "987654321" --version-id "version-id-123" --fastlane-dir ./fastlane --confirm`) {
		t.Fatalf("expected personalized migrate import command, got %#v", report.Migration.SuggestedCommands)
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc publish appstore --app "987654321" --ipa app.ipa --version "4.5.6" --submit --confirm`) {
		t.Fatalf("expected personalized canonical publish command, got %#v", report.Migration.SuggestedCommands)
	}
}

func TestBuildSuggestedCommandsUploadOnlyUsesUploadedBuildPlaceholder(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	commands := buildSuggestedCommands(migrationSignals{
		detectedActions:  []string{"upload_to_app_store"},
		marketingVersion: "1.2.3",
	}, func(MigrationSuggestionResolverInput) MigrationSuggestionResolverOutput {
		return MigrationSuggestionResolverOutput{
			AppID:     "123456789",
			VersionID: "version-id-123",
			BuildID:   "build-id-456",
		}
	})

	if !sliceContains(commands, `asc review submit --app "123456789" --version-id "VERSION_ID" --build-id "UPLOADED_BUILD_ID" --platform "PLATFORM" --confirm`) {
		t.Fatalf("expected review submit guidance to use placeholder IDs, got %#v", commands)
	}
	if !sliceContains(commands, `asc versions attach-build --version-id "VERSION_ID" --build-id "UPLOADED_BUILD_ID"`) {
		t.Fatalf("expected attach-build guidance to use uploaded build placeholder, got %#v", commands)
	}
	attachIdx := sliceIndex(commands, `asc versions attach-build --version-id "VERSION_ID" --build-id "UPLOADED_BUILD_ID"`)
	validateIdx := sliceIndex(commands, `asc validate --app "123456789" --version-id "VERSION_ID"`)
	reviewSubmitIdx := sliceIndex(commands, `asc review submit --app "123456789" --version-id "VERSION_ID" --build-id "UPLOADED_BUILD_ID" --platform "PLATFORM" --confirm`)
	if attachIdx < 0 || validateIdx <= attachIdx || reviewSubmitIdx <= validateIdx {
		t.Fatalf("expected attach-build -> validate -> review submit ordering, got %#v", commands)
	}
	if sliceContains(commands, `asc review submit --app "123456789" --version-id "version-id-123" --build-id "UPLOADED_BUILD_ID" --platform "PLATFORM" --confirm`) {
		t.Fatalf("expected upload-only guidance to avoid a platform-agnostic resolved version ID, got %#v", commands)
	}
	if !sliceContains(commands, `asc versions create --app "123456789" --version "1.2.3"`) {
		t.Fatalf("expected upload-only guidance to keep version creation when no platform-aware version ID is available, got %#v", commands)
	}
}

func TestBuildSuggestedCommandsUploadOnlyDoesNotRequestResolvedBuildID(t *testing.T) {
	t.Setenv("ASC_APP_ID", "123456789")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	var resolverInput MigrationSuggestionResolverInput
	buildSuggestedCommands(migrationSignals{
		detectedActions:  []string{"upload_to_app_store"},
		marketingVersion: "1.2.3",
	}, func(input MigrationSuggestionResolverInput) MigrationSuggestionResolverOutput {
		resolverInput = input
		return MigrationSuggestionResolverOutput{VersionID: "version-id-123"}
	})

	if resolverInput.NeedVersionID {
		t.Fatalf("expected upload-only migration hints to avoid requesting a platform-agnostic version ID, got %+v", resolverInput)
	}
	if resolverInput.NeedBuildID {
		t.Fatalf("expected upload-only migration hints to avoid requesting a resolved build ID, got %+v", resolverInput)
	}
}

func TestBuildSuggestedCommandsQuotesDerivedPublishVersion(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	commands := buildSuggestedCommands(migrationSignals{
		detectedActions:  []string{"deliver", "upload_to_app_store"},
		marketingVersion: `1.2.3 beta "1"`,
	}, nil)

	if !sliceContains(commands, `asc publish appstore --app "APP_ID" --ipa app.ipa --version "1.2.3 beta \"1\"" --submit --confirm`) {
		t.Fatalf("expected quoted canonical publish command derived from version string, got %#v", commands)
	}
}

func assertNoSecretInDoctorReport(t *testing.T, report DoctorReport, secret string) {
	t.Helper()
	for _, section := range report.Sections {
		for _, check := range section.Checks {
			if strings.Contains(check.Message, secret) {
				t.Fatalf("secret leaked in message: %q", check.Message)
			}
			if strings.Contains(check.Recommendation, secret) {
				t.Fatalf("secret leaked in recommendation: %q", check.Recommendation)
			}
		}
	}
	if report.Migration != nil {
		for _, cmd := range report.Migration.SuggestedCommands {
			if strings.Contains(cmd, secret) {
				t.Fatalf("secret leaked in suggested command: %q", cmd)
			}
		}
		for _, file := range report.Migration.DetectedFiles {
			if strings.Contains(file, secret) {
				t.Fatalf("secret leaked in detected file: %q", file)
			}
		}
	}
}

func findDoctorSection(t *testing.T, report DoctorReport, title string) DoctorSection {
	t.Helper()
	for _, section := range report.Sections {
		if section.Title == title {
			return section
		}
	}
	t.Fatalf("expected section %q, got %#v", title, report.Sections)
	return DoctorSection{}
}

func sectionHasStatus(section DoctorSection, status DoctorStatus, contains string) bool {
	for _, check := range section.Checks {
		if check.Status == status && strings.Contains(check.Message, contains) {
			return true
		}
	}
	return false
}

func sliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sliceIndex(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}

func clearMigrationTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
}
