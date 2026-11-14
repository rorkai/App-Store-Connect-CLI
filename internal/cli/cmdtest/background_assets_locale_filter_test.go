package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func runBackgroundAssetsCommandWithTransport(
	t *testing.T,
	args []string,
	wantPath string,
	wantQuery []string,
	body string,
) (string, string, error) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET(wantPath, wantQuery...), asctest.JSON(http.StatusOK, body))

	return runCommand(t, args)
}

func TestBackgroundAssetsListsSendOnlyTheFiltersAndSortRequested(t *testing.T) {
	const (
		assetsPath   = "/v1/apps/app-1/backgroundAssets"
		versionsPath = "/v1/backgroundAssets/asset-1/versions"
	)
	assetArgs := []string{"background-assets", "list", "--app", "app-1"}
	versionArgs := []string{"background-assets", "versions", "list", "--background-asset-id", "asset-1"}

	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantQuery []string
		body      string
		wantID    string
	}{
		{
			name:      "asset list sends versions locale",
			args:      append(append([]string{}, assetArgs...), "--versions-locale", "en-US, ja"),
			wantPath:  assetsPath,
			wantQuery: []string{"filter[versions.locale]=en-US,ja"},
			body:      `{"data":[{"type":"backgroundAssets","id":"asset-1"}]}`,
			wantID:    "asset-1",
		},
		{
			name:      "version list sends locale",
			args:      append(append([]string{}, versionArgs...), "--locale", "en-US"),
			wantPath:  versionsPath,
			wantQuery: []string{"filter[locale]=en-US"},
			body:      `{"data":[{"type":"backgroundAssetVersions","id":"version-1"}]}`,
			wantID:    "version-1",
		},
		{
			name:     "asset list omits new parameters by default",
			args:     assetArgs,
			wantPath: assetsPath,
		},
		{
			name:     "asset list sends platforms and sort",
			args:     append(append([]string{}, assetArgs...), "--versions-platform", "ios, MAC_OS", "--sort", "-createdDate"),
			wantPath: assetsPath,
			wantQuery: []string{
				"filter[versions.platforms]=IOS,MAC_OS",
				"sort=-createdDate",
			},
		},
		{
			name:     "version list omits new parameters by default",
			args:     versionArgs,
			wantPath: versionsPath,
		},
		{
			name: "version list sends every filter and sort",
			args: append(append([]string{}, versionArgs...),
				"--platform", "vision_os,TV_OS",
				"--state", "failed,COMPLETE",
				"--version", "1, 2",
				"--internal-beta-state", "ready_for_testing",
				"--external-beta-state", "IN_REVIEW,REJECTED",
				"--app-store-state", "accepted,READY_FOR_DISTRIBUTION",
				"--sort", "-version",
			),
			wantPath: versionsPath,
			wantQuery: []string{
				"filter[platforms]=VISION_OS,TV_OS",
				"filter[state]=FAILED,COMPLETE",
				"filter[version]=1,2",
				"filter[internalBetaRelease.state]=READY_FOR_TESTING",
				"filter[externalBetaRelease.state]=IN_REVIEW,REJECTED",
				"filter[appStoreRelease.state]=ACCEPTED,READY_FOR_DISTRIBUTION",
				"sort=-version",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := test.body
			if body == "" {
				body = `{"data":[]}`
			}
			stdout, stderr, err := runBackgroundAssetsCommandWithTransport(t, test.args, test.wantPath, test.wantQuery, body)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if test.wantID == "" {
				return
			}
			var payload struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if jsonErr := json.Unmarshal([]byte(stdout), &payload); jsonErr != nil {
				t.Fatalf("failed to parse stdout JSON: %v (stdout %q)", jsonErr, stdout)
			}
			if len(payload.Data) != 1 || payload.Data[0].ID != test.wantID {
				t.Fatalf("expected %s in output, got %+v", test.wantID, payload.Data)
			}
		})
	}
}

func TestBackgroundAssetsListsRejectInvalidValuesBeforeAnyRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	asctest.NewRegistry(t).Install()

	const (
		assets   = "background-assets list --app app-1"
		versions = "background-assets versions list --background-asset-id asset-1"
	)
	tests := []struct {
		command string
		flag    string
		value   string
		wantErr string
	}{
		{assets, "versions-platform", "WATCH_OS", "background-assets list: --versions-platform must be one of: IOS, MAC_OS, TV_OS, VISION_OS"},
		{assets, "versions-platform", ",", "background-assets list: --versions-platform must be one of: IOS, MAC_OS, TV_OS, VISION_OS"},
		{assets, "sort", "version", "background-assets list: --sort must be one of: assetPackIdentifier, -assetPackIdentifier, createdDate, -createdDate"},
		{versions, "platform", "IOS,WATCH_OS", "background-assets versions list: --platform must be one of: IOS, MAC_OS, TV_OS, VISION_OS"},
		{versions, "state", "IN_REVIEW", "background-assets versions list: --state must be one of: AWAITING_UPLOAD, PROCESSING, FAILED, COMPLETE"},
		{versions, "version", ",", "background-assets versions list: --version must include at least one value"},
		{versions, "internal-beta-state", "IN_REVIEW", "background-assets versions list: --internal-beta-state must be one of: READY_FOR_TESTING, SUPERSEDED"},
		{versions, "external-beta-state", "ACCEPTED", "background-assets versions list: --external-beta-state must be one of: READY_FOR_BETA_SUBMISSION, WAITING_FOR_REVIEW, IN_REVIEW, REJECTED, PROCESSING_FOR_TESTING, READY_FOR_TESTING, SUPERSEDED"},
		{versions, "app-store-state", "READY_FOR_SALE", "background-assets versions list: --app-store-state must be one of: PREPARE_FOR_SUBMISSION, READY_FOR_REVIEW, WAITING_FOR_REVIEW, IN_REVIEW, ACCEPTED, REJECTED, PROCESSING_FOR_DISTRIBUTION, READY_FOR_DISTRIBUTION, SUPERSEDED"},
		{versions, "sort", "createdDate", "background-assets versions list: --sort must be one of: version, -version"},
	}

	for _, test := range tests {
		t.Run(test.command+" --"+test.flag+" "+test.value, func(t *testing.T) {
			args := append(strings.Fields(test.command), "--"+test.flag, test.value)
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(args, "1.2.3")
			})
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			assertUsageDiagnosticFirstLine(t, stderr, test.wantErr)
		})
	}
}

func TestBackgroundAssetsVersionsListShowsLocale(t *testing.T) {
	const body = `{"data":[{"type":"backgroundAssetVersions","id":"version-1","attributes":{"version":"1","locale":"ja"}}]}`
	args := []string{"background-assets", "versions", "list", "--background-asset-id", "asset-1", "--output"}

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/backgroundAssets/asset-1/versions"), asctest.JSON(http.StatusOK, body))
	reg.Register(asctest.GET("/v1/backgroundAssets/asset-1/versions"), asctest.JSON(http.StatusOK, body))

	jsonOut, _, err := runCommand(t, append(args[:len(args):len(args)], "json"))
	if err != nil {
		t.Fatalf("json run error: %v", err)
	}
	var payload struct {
		Data []struct {
			Attributes struct {
				Locale string `json:"locale"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if jsonErr := json.Unmarshal([]byte(jsonOut), &payload); jsonErr != nil {
		t.Fatalf("failed to parse stdout JSON: %v (stdout %q)", jsonErr, jsonOut)
	}
	if len(payload.Data) != 1 || payload.Data[0].Attributes.Locale != "ja" {
		t.Fatalf("expected locale ja in JSON output, got %q", jsonOut)
	}

	tableOut, _, err := runCommand(t, append(args[:len(args):len(args)], "table"))
	if err != nil {
		t.Fatalf("table run error: %v", err)
	}
	for _, want := range []string{"Locale", "ja"} {
		if !strings.Contains(tableOut, want) {
			t.Fatalf("expected %q in table output, got %q", want, tableOut)
		}
	}
}
