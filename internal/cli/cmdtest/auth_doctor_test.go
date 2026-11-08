package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type doctorMigrationJSON struct {
	DetectedFiles     []string `json:"detectedFiles"`
	DetectedActions   []string `json:"detectedActions"`
	SuggestedCommands []string `json:"suggestedCommands"`
}

type doctorReportJSON struct {
	Migration doctorMigrationJSON `json:"migration"`
	Sections  []struct {
		Title  string `json:"title"`
		Checks []struct {
			Message string `json:"message"`
		} `json:"checks"`
	} `json:"sections"`
	Summary struct {
		Errors int `json:"errors"`
	} `json:"summary"`
}

func TestAuthDoctorJSONIncludesMigrationHints(t *testing.T) {
	withTempRepo(t, func(repo string) {
		fastlaneDir := filepath.Join(repo, "fastlane")
		if err := os.MkdirAll(fastlaneDir, 0o755); err != nil {
			t.Fatalf("mkdir fastlane error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(fastlaneDir, "Appfile"), []byte(`app_identifier "com.example.app"`), 0o644); err != nil {
			t.Fatalf("write Appfile error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(fastlaneDir, "Fastfile"), []byte("deliver\nupload_to_app_store\n"), 0o644); err != nil {
			t.Fatalf("write Fastfile error: %v", err)
		}

		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		stdout, _ := captureOutput(t, func() {
			if err := root.Parse([]string{"auth", "doctor", "--output", "json"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})

		var report doctorReportJSON
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if !sliceContains(report.Migration.DetectedFiles, "fastlane/Appfile") {
			t.Fatalf("expected Appfile in detected files, got %#v", report.Migration.DetectedFiles)
		}
		if !sliceContains(report.Migration.DetectedActions, "deliver") {
			t.Fatalf("expected deliver action, got %#v", report.Migration.DetectedActions)
		}
		if len(report.Migration.SuggestedCommands) == 0 {
			t.Fatalf("expected suggested commands, got %#v", report.Migration.SuggestedCommands)
		}
	})
}

func TestAuthDoctorTextIncludesMigrationHints(t *testing.T) {
	withTempRepo(t, func(repo string) {
		fastlaneDir := filepath.Join(repo, "fastlane")
		if err := os.MkdirAll(fastlaneDir, 0o755); err != nil {
			t.Fatalf("mkdir fastlane error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(fastlaneDir, "Deliverfile"), []byte("app_identifier \"com.example.app\""), 0o644); err != nil {
			t.Fatalf("write Deliverfile error: %v", err)
		}

		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		stdout, _ := captureOutput(t, func() {
			if err := root.Parse([]string{"auth", "doctor"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})

		if !strings.Contains(stdout, "Migration Hints:") {
			t.Fatalf("expected migration section heading, got %q", stdout)
		}
		if !strings.Contains(stdout, "Detected Deliverfile") {
			t.Fatalf("expected deliverfile detection, got %q", stdout)
		}
		if !strings.Contains(stdout, "Suggested:") {
			t.Fatalf("expected suggested commands, got %q", stdout)
		}
	})
}

func TestAuthDoctorTextRedactsCredentialIdentifiers(t *testing.T) {
	withTempRepo(t, func(repo string) {
		keyPath := filepath.Join(repo, "AuthKey.p8")
		writeECDSAPEM(t, keyPath)
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
		t.Setenv("ASC_KEY_ID", "ABC123SECRET")
		t.Setenv("ASC_ISSUER_ID", "issuer-uuid")
		t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		stdout, _ := captureOutput(t, func() {
			if err := root.Parse([]string{"auth", "doctor"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})

		if !strings.Contains(stdout, "ASC_KEY_ID is set") {
			t.Fatalf("expected ASC_KEY_ID presence message, got %q", stdout)
		}
		if !strings.Contains(stdout, "ASC_ISSUER_ID is set") {
			t.Fatalf("expected ASC_ISSUER_ID presence message, got %q", stdout)
		}
		if strings.Contains(stdout, "ABC123SECRET") {
			t.Fatalf("expected ASC_KEY_ID to be redacted, got %q", stdout)
		}
		if strings.Contains(stdout, "issuer-uuid") {
			t.Fatalf("expected ASC_ISSUER_ID to be redacted, got %q", stdout)
		}
	})
}

func TestAuthDoctorJSONFailsForMissingEnvironmentPrivateKey(t *testing.T) {
	withTempRepo(t, func(repo string) {
		missingKeyPath := filepath.Join(repo, "missing.p8")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
		t.Setenv("ASC_KEY_ID", "ENVKEY")
		t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
		t.Setenv("ASC_PRIVATE_KEY_PATH", missingKeyPath)
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		var runErr error
		stdout, _ := captureOutput(t, func() {
			if err := root.Parse([]string{"auth", "doctor", "--output", "json"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})
		if runErr == nil {
			t.Fatal("expected doctor to fail for a missing environment private key")
		}

		var report doctorReportJSON
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if report.Summary.Errors == 0 {
			t.Fatalf("expected a failed check in summary, got %#v", report.Summary)
		}

		found := false
		for _, section := range report.Sections {
			if section.Title != "Environment" {
				continue
			}
			for _, check := range section.Checks {
				if strings.Contains(check.Message, "ASC_PRIVATE_KEY_PATH - file not found") {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("expected environment key failure in JSON output, got %q", stdout)
		}
	})
}

func TestAuthDoctorJSONMigrationHintsUseEmptyArrays(t *testing.T) {
	withTempRepo(t, func(repo string) {
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		stdout, _ := captureOutput(t, func() {
			if err := root.Parse([]string{"auth", "doctor", "--output", "json"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})

		if !strings.Contains(stdout, `"migration":{"detectedFiles":[],"detectedActions":[],"suggestedCommands":[]}`) {
			t.Fatalf("expected migration arrays in JSON output, got %q", stdout)
		}

		var report doctorReportJSON
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if report.Migration.DetectedFiles == nil {
			t.Fatal("expected detectedFiles to decode as an empty array")
		}
		if report.Migration.DetectedActions == nil {
			t.Fatal("expected detectedActions to decode as an empty array")
		}
		if report.Migration.SuggestedCommands == nil {
			t.Fatal("expected suggestedCommands to decode as an empty array")
		}
		if len(report.Migration.DetectedFiles) != 0 || len(report.Migration.DetectedActions) != 0 || len(report.Migration.SuggestedCommands) != 0 {
			t.Fatalf("expected empty migration arrays, got %#v", report.Migration)
		}
	})
}

func TestAuthDoctorJSONPrefillsVersionFromXcodeProject(t *testing.T) {
	withTempRepo(t, func(repo string) {
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

		xcodeprojDir := filepath.Join(repo, "Demo.xcodeproj")
		if err := os.MkdirAll(xcodeprojDir, 0o755); err != nil {
			t.Fatalf("mkdir xcodeproj error: %v", err)
		}
		pbxproj := `
		buildSettings = {
			MARKETING_VERSION = 3.2.1;
		};
		`
		if err := os.WriteFile(filepath.Join(xcodeprojDir, "project.pbxproj"), []byte(pbxproj), 0o644); err != nil {
			t.Fatalf("write pbxproj error: %v", err)
		}

		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
		t.Setenv("ASC_APP_ID", "123456789")

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		stdout, _ := captureOutput(t, func() {
			if err := root.Parse([]string{"auth", "doctor", "--output", "json"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})

		var report doctorReportJSON
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if sliceContains(report.Migration.SuggestedCommands, `asc release run --app "123456789" --version "3.2.1" --build "BUILD_ID" --metadata-dir "./metadata/version/3.2.1" --confirm`) {
			t.Fatalf("expected upload-only migration hints to avoid deprecated release run guidance without metadata, got %#v", report.Migration.SuggestedCommands)
		}
		if !sliceContains(report.Migration.SuggestedCommands, `asc validate --app "123456789" --version-id "VERSION_ID"`) {
			t.Fatalf("expected canonical validate guidance with prefilled app/version, got %#v", report.Migration.SuggestedCommands)
		}
		if !sliceContains(report.Migration.SuggestedCommands, `asc builds upload --app "123456789" --ipa app.ipa --version "3.2.1" --build-number "BUILD_NUMBER" --wait`) {
			t.Fatalf("expected upload guidance for upload-only lanes, got %#v", report.Migration.SuggestedCommands)
		}
		if !sliceContains(report.Migration.SuggestedCommands, `asc builds info --build-id "UPLOADED_BUILD_ID"`) {
			t.Fatalf("expected build lookup guidance for upload-only lanes, got %#v", report.Migration.SuggestedCommands)
		}
		if !sliceContains(report.Migration.SuggestedCommands, `asc versions create --app "123456789" --version "3.2.1"`) {
			t.Fatalf("expected version create guidance for upload-only lanes, got %#v", report.Migration.SuggestedCommands)
		}
		if !sliceContains(report.Migration.SuggestedCommands, `asc review submit --app "123456789" --version-id "VERSION_ID" --build-id "UPLOADED_BUILD_ID" --platform "PLATFORM" --confirm`) {
			t.Fatalf("expected review submit guidance for upload-only lanes, got %#v", report.Migration.SuggestedCommands)
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
		if sliceContains(report.Migration.SuggestedCommands, `asc submit create --app "123456789" --version "3.2.1" --build "BUILD_ID" --confirm`) {
			t.Fatalf("expected upload-only migration hints to avoid deprecated submit create guidance, got %#v", report.Migration.SuggestedCommands)
		}
		if sliceContains(report.Migration.SuggestedCommands, `asc submit preflight --app "123456789" --version "3.2.1"`) {
			t.Fatalf("expected upload-only migration hints to avoid deprecated submit preflight guidance, got %#v", report.Migration.SuggestedCommands)
		}
	})
}

func withTempRepo(t *testing.T, fn func(repo string)) {
	t.Helper()

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
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	fn(repo)
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
