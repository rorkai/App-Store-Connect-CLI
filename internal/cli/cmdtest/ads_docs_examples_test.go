package cmdtest

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kballard/go-shellquote"
)

func TestAdsGuideExamplesParseAgainstCurrentCLI(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "apple-ads-private-key.pem")
	writeECDSAPEM(t, keyPath)
	payloads := writeAdsGuidePayloads(t, tempDir)

	commands := adsGuideCommands(t)
	if len(commands) == 0 {
		t.Fatal("expected Apple Ads guide commands")
	}
	for _, commandLine := range commands {
		args := adsGuideArgs(t, commandLine, keyPath, payloads)
		if adsArgsContainHelp(args) {
			continue
		}
		t.Run(commandLine, func(t *testing.T) {
			root := RootCommand("dev")
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse %q: %v", commandLine, err)
			}
		})
	}
}

func TestAdsGuideExamplesDispatchRepresentativeCommands(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "apple-ads-private-key.pem")
	writeECDSAPEM(t, keyPath)
	payloads := writeAdsGuidePayloads(t, tempDir)

	isolateAdsGuideEnv(t)
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "123456")

	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "appleid.apple.com":
			return adsJSONResponse(http.StatusOK, `{"access_token":"ACCESS","token_type":"Bearer","expires_in":3600,"scope":"searchadsorg"}`), nil
		case "api.searchads.apple.com":
			if req.Method == http.MethodDelete {
				return adsJSONResponse(http.StatusNoContent, ``), nil
			}
			if strings.Contains(req.URL.RawQuery, "limit=") || strings.Contains(req.URL.Path, "/search/") || strings.Contains(req.URL.Path, "/creatives") || strings.Contains(req.URL.Path, "/campaigns") {
				return adsJSONResponse(http.StatusOK, `{"data":[{"id":123456789}],"pagination":{"itemsPerPage":1,"startIndex":0,"totalResults":1}}`), nil
			}
			return adsJSONResponse(http.StatusOK, `{"data":{"id":123456789}}`), nil
		default:
			t.Fatalf("unexpected host %s for %s %s", req.URL.Host, req.Method, req.URL.String())
			return nil, nil
		}
	}))

	for _, commandLine := range []string{
		`asc ads auth login --name "Marketing" --client-id "SEARCHADS_CLIENT_ID" --team-id "SEARCHADS_TEAM_ID" --key-id "KEY_ID" --private-key ./apple-ads-private-key.pem --org "123456"`,
		"asc ads auth status --validate",
		"asc ads auth doctor",
		"asc ads auth token --confirm --output json",
		"asc ads campaigns --limit 10 --output json",
		`asc ads acls --output json`,
		`asc ads campaigns list --org "123456" --output json`,
		`asc ads me view`,
		`asc ads campaigns delete --org "123456" --campaign 987654321 --confirm`,
		`asc ads apps search --org "123456" --query "My App" --limit 10 --output json`,
		`asc ads product-pages list --org "123456" --adam-id 1234567890 --states VISIBLE`,
		`asc ads targeting-keywords create-bulk --org "123456" --campaign 987654321 --ad-group 123456789 --file keywords.json`,
		`asc ads targeting-keywords delete-bulk --org "123456" --campaign 987654321 --ad-group 123456789 --file keyword-ids.json --confirm`,
		`asc ads reports campaigns --org "123456" --file reporting-request.json --output json`,
		`asc ads impression-share-reports --org "123456" --limit 50 --output json`,
		`asc ads api request --method POST --path v5/campaigns/find --org "123456" --file selector.json --output json`,
	} {
		t.Run(commandLine, func(t *testing.T) {
			args := adsGuideArgs(t, commandLine, keyPath, payloads)
			stdout, stderr, err := runAdsGuideCommand(t, args)
			if err != nil {
				t.Fatalf("run %q: %v\nstderr: %s", commandLine, err, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if strings.Contains(commandLine, "--output json") || strings.Contains(commandLine, " auth token ") {
				var parsed any
				if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
					t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
				}
			}
		})
	}
}

func runAdsGuideCommand(t *testing.T, args []string) (string, string, error) {
	t.Helper()

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		root := RootCommand("dev")
		if err := root.Parse(args); err != nil {
			runErr = err
			return
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func isolateAdsGuideEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"ASC_KEY_ID",
		"ASC_ISSUER_ID",
		"ASC_PRIVATE_KEY_PATH",
		"ASC_PRIVATE_KEY",
		"ASC_PRIVATE_KEY_B64",
		"ASC_PROFILE",
		"ASC_CONFIG_PATH",
		"ASC_BYPASS_KEYCHAIN",
		"ASC_STRICT_AUTH",
		"ASC_APP_ID",
		"ASC_ADS_ACCESS_TOKEN",
		"ASC_ADS_CLIENT_ID",
		"ASC_ADS_TEAM_ID",
		"ASC_ADS_KEY_ID",
		"ASC_ADS_PRIVATE_KEY_PATH",
		"ASC_ADS_PRIVATE_KEY",
		"ASC_ADS_PRIVATE_KEY_B64",
		"ASC_ADS_ORG_ID",
		"ASC_ADS_PROFILE",
		"ASC_ADS_BYPASS_KEYCHAIN",
		"ASC_ADS_STRICT_AUTH",
	} {
		t.Setenv(key, "")
	}
}

func adsGuideCommands(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "commands", "ads.mdx"))
	if err != nil {
		t.Fatalf("read commands/ads.mdx: %v", err)
	}

	var commands []string
	var pending string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if pending == "" && !strings.HasPrefix(trimmed, "asc ads ") {
			continue
		}
		if pending == "" {
			pending = trimmed
		} else {
			pending += " " + trimmed
		}
		pending = strings.TrimSpace(strings.TrimSuffix(pending, "\\"))
		if strings.HasSuffix(trimmed, "\\") {
			continue
		}
		commands = append(commands, pending)
		pending = ""
	}
	return commands
}

func adsGuideArgs(t *testing.T, commandLine string, keyPath string, payloads map[string]string) []string {
	t.Helper()

	fields, err := shellquote.Split(commandLine)
	if err != nil {
		t.Fatalf("split %q: %v", commandLine, err)
	}
	if len(fields) < 2 || fields[0] != "asc" || fields[1] != "ads" {
		t.Fatalf("expected asc ads command, got %#v", fields)
	}
	args := append([]string(nil), fields[1:]...)
	for i, arg := range args {
		switch arg {
		case "./apple-ads-private-key.pem":
			args[i] = keyPath
		case "campaign.json", "campaign-update.json", "ad-group.json", "ad.json", "selector.json", "keywords.json", "keyword-ids.json", "negative-keywords.json", "reporting-request.json", "custom-report-request.json":
			args[i] = payloads[arg]
		}
	}
	return args
}

func adsArgsContainHelp(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func writeAdsGuidePayloads(t *testing.T, dir string) map[string]string {
	t.Helper()

	files := map[string]string{
		"campaign.json":              `{"name":"Brand Campaign","status":"PAUSED"}`,
		"campaign-update.json":       `{"campaign":{"status":"PAUSED"}}`,
		"ad-group.json":              `{"name":"Brand Ad Group","status":"PAUSED"}`,
		"ad.json":                    `{"name":"Brand Ad","status":"PAUSED"}`,
		"selector.json":              `{"conditions":[{"field":"deleted","operator":"EQUALS","values":["false"]}],"pagination":{"offset":0,"limit":100}}`,
		"keywords.json":              `[{"text":"example keyword","matchType":"EXACT","status":"PAUSED"}]`,
		"keyword-ids.json":           `[111111111,222222222]`,
		"negative-keywords.json":     `[{"text":"free","matchType":"BROAD","status":"ACTIVE"}]`,
		"reporting-request.json":     `{"startTime":"2026-05-01","endTime":"2026-05-31","returnRowTotals":true,"selector":{"pagination":{"offset":0,"limit":100}}}`,
		"custom-report-request.json": `{"startTime":"2026-05-01","endTime":"2026-05-31"}`,
	}
	paths := make(map[string]string, len(files))
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		paths[name] = path
	}
	return paths
}
