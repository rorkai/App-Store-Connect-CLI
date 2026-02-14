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
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
		t.Setenv("ASC_KEY_ID", "ABC123SECRET")
		t.Setenv("ASC_ISSUER_ID", "issuer-uuid")
		t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")

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
		if !sliceContains(report.Migration.SuggestedCommands, `asc publish appstore --app "123456789" --ipa app.ipa --version "3.2.1" --submit --confirm`) {
			t.Fatalf("expected publish command with prefilled app/version, got %#v", report.Migration.SuggestedCommands)
		}
		if !sliceContains(report.Migration.SuggestedCommands, `asc submit create --app "123456789" --version "3.2.1" --build "BUILD_ID" --confirm`) {
			t.Fatalf("expected submit command with prefilled app/version, got %#v", report.Migration.SuggestedCommands)
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
