package cmdtest

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

var productPageScreenshotSetUploadCommands = []struct {
	name    string
	args    []string
	setPath string
}{
	{
		name:    "experiment treatment upload",
		args:    []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "upload", "--localization-id", "LOC_ID"},
		setPath: "/v1/appStoreVersionExperimentTreatmentLocalizations/LOC_ID/appScreenshotSets",
	},
	{
		name:    "experiment treatment sync",
		args:    []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "sync", "--localization-id", "LOC_ID", "--confirm"},
		setPath: "/v1/appStoreVersionExperimentTreatmentLocalizations/LOC_ID/appScreenshotSets",
	},
	{
		name:    "custom page upload",
		args:    []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "upload", "--localization-id", "LOC_ID"},
		setPath: "/v1/appCustomProductPageLocalizations/LOC_ID/appScreenshotSets",
	},
	{
		name:    "custom page sync",
		args:    []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "sync", "--localization-id", "LOC_ID", "--confirm"},
		setPath: "/v1/appCustomProductPageLocalizations/LOC_ID/appScreenshotSets",
	},
}

// Product page screenshot sets take the same --concurrency bounds as
// asc screenshots upload; an out-of-range value must fail before any request.
func TestProductPageScreenshotSetUploadsRejectConcurrencyOutOfRange(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	for _, command := range productPageScreenshotSetUploadCommands {
		for _, value := range []string{"0", "11"} {
			t.Run(command.name+" "+value, func(t *testing.T) {
				asctest.NewRegistry(t).Install()
				args := append(append([]string{}, command.args...), "--path", "./shots", "--device-type", "IPHONE_65", "--concurrency", value)
				_, stderr, runErr := runRootCommand(t, args)
				if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
				}
				if want := "--concurrency must be between 1 and 10"; !strings.Contains(stderr, want) {
					t.Fatalf("expected %q in stderr %q", want, stderr)
				}
			})
		}
	}
}

// --no-wait must reach the shared upload flow of every product page
// screenshot-set command: each committed screenshot is checked once instead of
// polled until App Store Connect finishes processing it.
func TestProductPageScreenshotSetUploadsNoWaitChecksDeliveryOnce(t *testing.T) {
	for _, command := range productPageScreenshotSetUploadCommands {
		t.Run(command.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")

			filePath := writePPOCmdtestPNG(t, t.TempDir(), "01-home.png", 1242, 2688)
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("Stat() error: %v", err)
			}

			var mu sync.Mutex
			stateChecks := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == command.setPath:
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`), nil
				case req.Method == http.MethodGet && (req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots" ||
					req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots"):
					return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
					return jsonHTTPResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/shot-1","length":%d,"offset":0}]}}}`, fileInfo.Size())), nil
				case req.Method == http.MethodPut && req.URL.Host == "upload.example":
					return jsonHTTPResponse(http.StatusOK, `{}`), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/shot-1":
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"uploaded":true,"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/shot-1":
					mu.Lock()
					stateChecks++
					check := stateChecks
					mu.Unlock()
					if check > 1 {
						return jsonHTTPResponse(http.StatusBadRequest, `{"errors":[{"detail":"unexpected extra state check"}]}`), nil
					}
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
					return jsonHTTPResponse(http.StatusNoContent, ""), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}))

			args := append(append([]string{}, command.args...), "--path", filePath, "--device-type", "IPHONE_65", "--no-wait", "--output", "json")
			stdout, stderr, runErr := runRootCommand(t, args)
			if runErr != nil {
				t.Fatalf("Run() error: %v\nstderr=%s", runErr, stderr)
			}
			if stateChecks != 1 {
				t.Fatalf("state checks = %d, want exactly 1", stateChecks)
			}
			if !strings.Contains(stdout, `"state":"UPLOAD_COMPLETE"`) {
				t.Fatalf("expected the committed screenshot in the receipt, got %s", stdout)
			}
		})
	}
}
