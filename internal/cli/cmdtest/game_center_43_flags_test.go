package cmdtest

import (
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func resetAuthEnvForUsageTests(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")
}

func TestRun_GameCenter43Flags_InvalidValuesReturnUsage(t *testing.T) {
	resetAuthEnvForUsageTests(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "achievement submit invalid pre-released",
			args:    []string{"game-center", "achievements", "submit", "--vendor-id", "ach", "--percentage", "100", "--bundle-id", "com.example.app", "--scoped-player-id", "player-1", "--pre-released", "maybe"},
			wantErr: "--pre-released must be true or false",
		},
		{
			name:    "leaderboard submit invalid pre-released",
			args:    []string{"game-center", "leaderboards", "submit", "--vendor-id", "lb", "--score", "100", "--bundle-id", "com.example.app", "--scoped-player-id", "player-1", "--pre-released", "maybe"},
			wantErr: "--pre-released must be true or false",
		},
		{
			name:    "activity create invalid create-initial-version",
			args:    []string{"game-center", "activities", "create", "--app", "APP_ID", "--reference-name", "Seasonal", "--vendor-id", "com.example.seasonal", "--create-initial-version", "maybe"},
			wantErr: "--create-initial-version must be true or false",
		},
		{
			name:    "challenge create invalid create-initial-version",
			args:    []string{"game-center", "challenges", "create", "--app", "APP_ID", "--reference-name", "Weekly", "--vendor-id", "com.example.weekly", "--leaderboard-id", "LB_ID", "--create-initial-version", "maybe"},
			wantErr: "--create-initial-version must be true or false",
		},
		{
			name:    "activity fallback requires create-initial-version",
			args:    []string{"game-center", "activities", "create", "--app", "APP_ID", "--reference-name", "Seasonal", "--vendor-id", "com.example.seasonal", "--initial-fallback-url", "https://example.com/fallback"},
			wantErr: "--initial-fallback-url requires --create-initial-version true",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, stderr := captureOutput(t, func() {
				code := cmd.Run(test.args, "1.0.0")
				if code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestRun_GameCenter43Flags_ValidValuesReachAuth(t *testing.T) {
	resetAuthEnvForUsageTests(t)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "achievement submit valid pre-released",
			args: []string{"game-center", "achievements", "submit", "--vendor-id", "ach", "--percentage", "100", "--bundle-id", "com.example.app", "--scoped-player-id", "player-1", "--pre-released", "true"},
		},
		{
			name: "leaderboard submit valid pre-released",
			args: []string{"game-center", "leaderboards", "submit", "--vendor-id", "lb", "--score", "100", "--bundle-id", "com.example.app", "--scoped-player-id", "player-1", "--pre-released", "false"},
		},
		{
			name: "activity create valid initial version flags",
			args: []string{"game-center", "activities", "create", "--app", "APP_ID", "--reference-name", "Seasonal", "--vendor-id", "com.example.seasonal", "--create-initial-version", "true", "--initial-fallback-url", "https://example.com/fallback"},
		},
		{
			name: "challenge create valid initial version flag",
			args: []string{"game-center", "challenges", "create", "--app", "APP_ID", "--reference-name", "Weekly", "--vendor-id", "com.example.weekly", "--leaderboard-id", "LB_ID", "--create-initial-version", "true"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, stderr := captureOutput(t, func() {
				code := cmd.Run(test.args, "1.0.0")
				if code != cmd.ExitAuth {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitAuth, code)
				}
			})
			if !strings.Contains(strings.ToLower(stderr), "auth") && !strings.Contains(strings.ToLower(stderr), "authentication") {
				t.Fatalf("expected auth-related stderr, got %q", stderr)
			}
		})
	}
}
