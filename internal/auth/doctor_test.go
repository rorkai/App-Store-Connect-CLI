package auth

import (
	"os"
	"path/filepath"
	"reflect"
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
	fastfile := `lane :beta do
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
		`asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir ./fastlane`,
		`asc builds latest --app "APP_ID"`,
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
	if !sliceContains(report.Migration.SuggestedCommands, `asc builds latest --app "123456789"`) {
		t.Fatalf("expected personalized app id in builds latest, got %#v", report.Migration.SuggestedCommands)
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc publish appstore --app "123456789" --ipa app.ipa --version "2.3.4" --submit --confirm`) {
		t.Fatalf("expected personalized publish command, got %#v", report.Migration.SuggestedCommands)
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc submit create --app "123456789" --version "2.3.4" --build "BUILD_ID" --confirm`) {
		t.Fatalf("expected personalized submit command, got %#v", report.Migration.SuggestedCommands)
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
	if !sliceContains(report.Migration.SuggestedCommands, `asc migrate import --app "987654321" --version-id "version-id-123" --fastlane-dir ./fastlane`) {
		t.Fatalf("expected personalized migrate import command, got %#v", report.Migration.SuggestedCommands)
	}
	if !sliceContains(report.Migration.SuggestedCommands, `asc submit create --app "987654321" --version "4.5.6" --build "build-id-456" --confirm`) {
		t.Fatalf("expected personalized submit command with resolved build ID, got %#v", report.Migration.SuggestedCommands)
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

func clearMigrationTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
}
