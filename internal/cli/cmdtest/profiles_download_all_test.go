package cmdtest

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func registerProfilesDownloadAllPages(reg *asctest.Registry) {
	profile := func(id, uuid, platform, profileType, content string) string {
		return `{"type":"profiles","id":"` + id + `","attributes":{"name":"Name ` + id + `","uuid":"` + uuid + `","platform":"` + platform + `","profileType":"` + profileType + `","profileContent":"` + base64.StdEncoding.EncodeToString([]byte(content)) + `"}}`
	}
	reg.Register(asctest.GET("/v1/profiles", "filter[profileState]=ACTIVE", "filter[profileType]=IOS_APP_DEVELOPMENT,MAC_APP_DEVELOPMENT", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[`+profile("P1", "UUID-1", "IOS", "IOS_APP_DEVELOPMENT", "ios profile")+`],"links":{"next":"https://api.appstoreconnect.apple.com/v1/profiles?cursor=2"}}`))
	reg.Register(asctest.GET("/v1/profiles", "cursor=2"),
		asctest.JSON(http.StatusOK, `{"data":[`+profile("P2", "UUID-2", "MAC_OS", "MAC_APP_DEVELOPMENT", "mac profile")+`],"links":{}}`))
}

func TestProfilesDownloadAllWritesEveryPage(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	dir := filepath.Join(t.TempDir(), "profiles")
	reg := asctest.NewRegistry(t)
	reg.Install()
	registerProfilesDownloadAllPages(reg)

	stdout, _, err := runCommand(t, []string{"profiles", "download", "--all", "--output-dir", dir, "--profile-type", "IOS_APP_DEVELOPMENT,MAC_APP_DEVELOPMENT", "--output-format", "json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls := len(reg.Requests()); calls != 2 {
		t.Fatalf("requests = %d, want 2", calls)
	}
	for name, want := range map[string]string{"UUID-1.mobileprovision": "ios profile", "UUID-2.provisionprofile": "mac profile"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, %v; want %q", name, data, err, want)
		}
	}
	var receipt struct {
		OutputDir string `json:"outputDir"`
		Profiles  []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			UUID        string `json:"uuid"`
			ProfileType string `json:"profileType"`
			OutputPath  string `json:"outputPath"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt %q: %v", stdout, err)
	}
	if receipt.OutputDir != dir || len(receipt.Profiles) != 2 {
		t.Fatalf("receipt = %+v", receipt)
	}
	got := receipt.Profiles[1]
	if got.ID != "P2" || got.Name != "Name P2" || got.UUID != "UUID-2" || got.ProfileType != "MAC_APP_DEVELOPMENT" || got.OutputPath != filepath.Join(dir, "UUID-2.provisionprofile") {
		t.Fatalf("second receipt entry = %+v", got)
	}
}

func TestProfilesDownloadAllExistingFileWritesNothing(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	dir := t.TempDir()
	existing := filepath.Join(dir, "UUID-2.provisionprofile")
	if err := os.WriteFile(existing, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := asctest.NewRegistry(t)
	reg.Install()
	registerProfilesDownloadAllPages(reg)

	_, _, err := runCommand(t, []string{"profiles", "download", "--all", "--output-dir", dir, "--profile-type", "IOS_APP_DEVELOPMENT,MAC_APP_DEVELOPMENT"})
	if err == nil || !strings.Contains(err.Error(), "pass --overwrite to replace it") || !strings.Contains(err.Error(), existing) {
		t.Fatalf("error = %v, want existing-file refusal naming %q", err, existing)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("directory has %d entries, want only the existing file", len(entries))
	}
	if data, _ := os.ReadFile(existing); string(data) != "old" {
		t.Fatalf("existing file = %q, want unchanged", data)
	}
}

func TestProfilesDownloadAllUnavailableContent(t *testing.T) {
	for _, mode := range []string{"default", "json", "table", "invalid UUID"} {
		t.Run(mode, func(t *testing.T) {
			setupAuth(t)
			dir := t.TempDir()
			reg := asctest.NewRegistry(t)
			reg.Install()
			uuid := "UUID-2"
			if mode == "invalid UUID" {
				uuid = "../outside"
			}
			reg.Register(asctest.GET("/v1/profiles", "filter[profileState]=ACTIVE", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"profiles","id":"P1","attributes":{"uuid":"UUID-1","profileContent":"cHJvZmlsZQ=="}},{"type":"profiles","id":"P2","attributes":{"uuid":"`+uuid+`"}}],"links":{}}`))
			args := []string{"profiles", "download", "--all", "--output-dir", dir}
			if mode != "default" {
				format := mode
				if mode == "invalid UUID" {
					format = "json"
				}
				args = append(args, "--skip-unavailable", "--output-format", format)
			}
			stdout, _, err := runCommand(t, args)
			if mode == "default" || mode == "invalid UUID" {
				if err == nil || !strings.Contains(err.Error(), `profile "P2" has no usable UUID or downloadable content; nothing was written`) || stdout != "" {
					t.Fatalf("download = %q, %v; want refusal without a receipt", stdout, err)
				}
				entries, readErr := os.ReadDir(dir)
				if readErr != nil || len(entries) != 0 {
					t.Fatalf("refused download wrote files: %v, %v", entries, readErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("partial download: %v", err)
			}
			path := filepath.Join(dir, "UUID-1.mobileprovision")
			if content, err := os.ReadFile(path); err != nil || string(content) != "profile" {
				t.Fatalf("downloaded content = %q, %v", content, err)
			}
			if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
				t.Fatalf("downloaded files = %v, %v; want only P1", entries, err)
			}
			if mode == "json" {
				var receipt struct {
					Profiles []struct{ ID, OutputPath string }
					Skipped  []struct{ ID, Reason string }
				}
				if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
					t.Fatal(err)
				}
				if len(receipt.Profiles) != 1 || receipt.Profiles[0].ID != "P1" || receipt.Profiles[0].OutputPath != path || len(receipt.Skipped) != 1 || receipt.Skipped[0].ID != "P2" || receipt.Skipped[0].Reason != "no downloadable content" {
					t.Fatalf("partial receipt = %s", stdout)
				}
			} else {
				for _, want := range []string{"Status", "Reason", "P1", "downloaded", "P2", "skipped", "no downloadable content"} {
					if !strings.Contains(stdout, want) {
						t.Fatalf("partial table lacks %q: %s", want, stdout)
					}
				}
			}
		})
	}
}

func TestProfilesDownloadAllUsageErrors(t *testing.T) {
	const dir = "./profiles"
	tests := []struct {
		args       []string
		wantStderr string
	}{
		{[]string{"--all"}, "Error: --output-dir is required"},
		{[]string{"--all", "--output-dir", dir, "--id", "PROFILE_ID"}, "Error: profiles download: --all cannot be combined with --id or --output"},
		{[]string{"--all", "--output-dir", dir, "--output", "./profile.mobileprovision"}, "Error: profiles download: --all cannot be combined with --id or --output"},
		{[]string{"--all", "--output-dir", dir, "--id="}, "Error: profiles download: --all cannot be combined with --id or --output"},
		{[]string{"--id", "PROFILE_ID", "--output-dir", dir}, "Error: profiles download: --output-dir, --profile-type, --profile-state, and --name require --all"},
		{[]string{"--id", "PROFILE_ID", "--output", "./p.mobileprovision", "--output-dir="}, "Error: profiles download: --output-dir, --profile-type, --profile-state, and --name require --all"},
		{[]string{"--id", "PROFILE_ID", "--output", "./p.mobileprovision", "--profile-type", "IOS_APP_DEVELOPMENT"}, "Error: profiles download: --output-dir, --profile-type, --profile-state, and --name require --all"},
		{[]string{"--all", "--output-dir", dir, "--profile-state", "EXPIRED"}, "Error: --profile-state must be one of: ACTIVE, INVALID"},
		{[]string{"--id", "PROFILE_ID", "--output", "./p.mobileprovision", "--skip-unavailable"}, "Error: profiles download: --skip-unavailable requires --all"},
		{[]string{"--id", "PROFILE_ID", "--output", "./p.mobileprovision", "--skip-unavailable=false"}, "Error: profiles download: --skip-unavailable requires --all"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			_, stderr, err := runCommand(t, append([]string{"profiles", "download"}, tt.args...))
			if code := cmd.ExitCodeFromError(err); code != cmd.ExitUsage {
				t.Fatalf("exit = %d (%v), want %d", code, err, cmd.ExitUsage)
			}
			if firstLine, _, _ := strings.Cut(stderr, "\n"); firstLine != tt.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, tt.wantStderr)
			}
			if calls := len(reg.Requests()); calls != 0 {
				t.Fatalf("requests = %d, want 0", calls)
			}
		})
	}
}
