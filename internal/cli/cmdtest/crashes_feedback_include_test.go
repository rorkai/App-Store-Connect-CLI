package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const feedbackScreenshotFields = "createdDate,comment,email,deviceModel,osVersion,locale,timeZone,architecture,connectionType,pairedAppleWatch,appUptimeInMilliseconds,diskBytesAvailable,diskBytesTotal,batteryPercentage,screenWidthInPoints,screenHeightInPoints,appPlatform,devicePlatform,deviceFamily,buildBundleId,screenshots,build,tester"

type betaSubmissionListOutput struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	Included []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Version string `json:"version"`
		} `json:"attributes"`
	} `json:"included"`
}

func decodeBetaSubmissionListOutput(t *testing.T, stdout string) betaSubmissionListOutput {
	t.Helper()

	var output betaSubmissionListOutput
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout)
	}
	return output
}

func installDefaultTransportForServer(t *testing.T, server *httptest.Server) {
	t.Helper()

	installDefaultTransport(t, serverTransport(t, server))
}

// serverTransport sends every request to server, keeping its path and query.
func serverTransport(t *testing.T, server *httptest.Server) http.RoundTripper {
	t.Helper()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
}

func TestBetaFeedbackListIncludeBuildSendsBuildRelationship(t *testing.T) {
	for _, tc := range []struct {
		command  string
		resource string
	}{
		{command: "crashes", resource: "betaFeedbackCrashSubmissions"},
		{command: "feedback", resource: "betaFeedbackScreenshotSubmissions"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps/123/"+tc.resource, "include=build", "fields[builds]=version,preReleaseVersion"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"`+tc.resource+`","id":"submission-1"}],`+
					`"included":[{"type":"builds","id":"b1","attributes":{"version":"532621"}}]}`))

			stdout, stderr, err := runCommand(t, []string{"testflight", tc.command, "list", "--app", "123", "--include", "build", "--output", "json"})
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			output := decodeBetaSubmissionListOutput(t, stdout)
			if len(output.Data) != 1 || output.Data[0].ID != "submission-1" {
				t.Fatalf("unexpected %s data: %+v", tc.command, output.Data)
			}
			if len(output.Included) != 1 || output.Included[0].Type != "builds" || output.Included[0].ID != "b1" || output.Included[0].Attributes.Version != "532621" {
				t.Fatalf("unexpected included build: %+v", output.Included)
			}
		})
	}
}

func TestBetaFeedbackListInvalidIncludeReturnsUsageError(t *testing.T) {
	for _, command := range []string{"crashes", "feedback"} {
		t.Run(command, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			asctest.NewRegistry(t).Install()

			stdout, stderr, runErr := runCommand(t, []string{"testflight", command, "list", "--app", "123", "--include", "bogus"})

			if rootcmd.ExitCodeFromError(runErr) != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", rootcmd.ExitCodeFromError(runErr), rootcmd.ExitUsage, runErr)
			}
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "--include must be a comma-separated list of: build, tester") {
				t.Fatalf("stderr = %q, want --include usage error", stderr)
			}
		})
	}
}

func TestFeedbackListIncludeScreenshotsPreservesAllFeedbackFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requestErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/123/betaFeedbackScreenshotSubmissions" {
			requestErr <- fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		query := req.URL.Query()
		if len(query) != 1 {
			requestErr <- fmt.Errorf("expected only the feedback sparse fieldset, got %q", query.Encode())
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		if got := query.Get("fields[betaFeedbackScreenshotSubmissions]"); got != feedbackScreenshotFields {
			requestErr <- fmt.Errorf("feedback fieldset = %q, want %q", got, feedbackScreenshotFields)
			http.Error(w, "unexpected fieldset", http.StatusBadRequest)
			return
		}
		requestErr <- nil
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"betaFeedbackScreenshotSubmissions","id":"fb-1","attributes":{"createdDate":"2026-01-20T00:00:00Z","comment":"Nice","email":"tester@example.com","deviceModel":"iPhone17,1","osVersion":"18.0","locale":"en-US","timeZone":"America/Los_Angeles","architecture":"arm64","connectionType":"WIFI","pairedAppleWatch":"Watch7,1","appUptimeInMilliseconds":1234,"diskBytesAvailable":2000,"diskBytesTotal":4000,"batteryPercentage":85,"screenWidthInPoints":430,"screenHeightInPoints":932,"appPlatform":"IOS","devicePlatform":"IOS","deviceFamily":"IPHONE","buildBundleId":"com.example.app","screenshots":[{"url":"https://example.com/shot.png","width":320,"height":640,"expirationDate":"2026-01-21T00:00:00Z"}]},"relationships":{"build":{"data":{"type":"builds","id":"build-1"}},"tester":{"data":{"type":"betaTesters","id":"tester-1"}}}}]}`)
	}))
	t.Cleanup(server.Close)
	installDefaultTransportForServer(t, server)

	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run([]string{"testflight", "feedback", "list", "--app", "123", "--include-screenshots", "--output", "json"}, "1.2.3")
	})

	if exitCode != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr: %s", exitCode, rootcmd.ExitSuccess, stderr)
	}
	if err := <-requestErr; err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var output struct {
		Data []struct {
			Attributes    map[string]any `json:"attributes"`
			Relationships map[string]struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout)
	}
	if len(output.Data) != 1 {
		t.Fatalf("feedback count = %d, want 1", len(output.Data))
	}
	expectedAttributeNames := []string{
		"createdDate", "comment", "email", "deviceModel", "osVersion", "locale", "timeZone",
		"architecture", "connectionType", "pairedAppleWatch", "appUptimeInMilliseconds",
		"diskBytesAvailable", "diskBytesTotal", "batteryPercentage", "screenWidthInPoints",
		"screenHeightInPoints", "appPlatform", "devicePlatform", "deviceFamily", "buildBundleId",
		"screenshots",
	}
	for _, name := range expectedAttributeNames {
		if _, ok := output.Data[0].Attributes[name]; !ok {
			t.Errorf("JSON output omitted feedback attribute %q", name)
		}
	}
	for name, want := range map[string]any{
		"createdDate":             "2026-01-20T00:00:00Z",
		"comment":                 "Nice",
		"appUptimeInMilliseconds": float64(1234),
		"batteryPercentage":       float64(85),
		"buildBundleId":           "com.example.app",
	} {
		if got := output.Data[0].Attributes[name]; got != want {
			t.Errorf("feedback attribute %q = %#v, want %#v", name, got, want)
		}
	}
	screenshots, ok := output.Data[0].Attributes["screenshots"].([]any)
	if !ok || len(screenshots) != 1 {
		t.Fatalf("feedback screenshots = %#v, want one screenshot", output.Data[0].Attributes["screenshots"])
	}
	screenshot, ok := screenshots[0].(map[string]any)
	if !ok {
		t.Fatalf("feedback screenshot = %#v, want an object", screenshots[0])
	}
	for name, want := range map[string]any{
		"url":            "https://example.com/shot.png",
		"width":          float64(320),
		"height":         float64(640),
		"expirationDate": "2026-01-21T00:00:00Z",
	} {
		if got := screenshot[name]; got != want {
			t.Errorf("feedback screenshot %q = %#v, want %#v", name, got, want)
		}
	}
	if output.Data[0].Relationships["build"].Data.ID != "build-1" || output.Data[0].Relationships["tester"].Data.ID != "tester-1" {
		t.Fatalf("JSON output omitted feedback relationships: %+v", output.Data[0].Relationships)
	}
}

func TestFeedbackListIncludeScreenshotsWithIncludedRelationships(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requestErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/123/betaFeedbackScreenshotSubmissions" {
			requestErr <- fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		query := req.URL.Query()
		if got := query.Get("include"); got != "build,tester" {
			requestErr <- fmt.Errorf("include query = %q, want %q", got, "build,tester")
			http.Error(w, "unexpected include", http.StatusBadRequest)
			return
		}
		if got := query.Get("fields[betaFeedbackScreenshotSubmissions]"); got != feedbackScreenshotFields {
			requestErr <- fmt.Errorf("feedback fieldset = %q, want %q", got, feedbackScreenshotFields)
			http.Error(w, "unexpected fieldset", http.StatusBadRequest)
			return
		}
		requestErr <- nil
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"betaFeedbackScreenshotSubmissions","id":"fb-1","attributes":{"screenshots":[{"url":"https://example.com/shot.png","width":320,"height":640,"expirationDate":"2026-01-21T00:00:00Z"}]}}],"included":[{"type":"builds","id":"build-1","attributes":{"version":"1.0"}},{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}]}`)
	}))
	t.Cleanup(server.Close)
	installDefaultTransportForServer(t, server)

	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run([]string{"testflight", "feedback", "list", "--app", "123", "--include-screenshots", "--include", "build,tester", "--output", "json"}, "1.2.3")
	})

	if exitCode != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr: %s", exitCode, rootcmd.ExitSuccess, stderr)
	}
	if err := <-requestErr; err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	output := decodeBetaSubmissionListOutput(t, stdout)
	if len(output.Data) != 1 || output.Data[0].ID != "fb-1" {
		t.Fatalf("unexpected feedback data: %+v", output.Data)
	}
	gotIncluded := make(map[string]bool, len(output.Included))
	for _, resource := range output.Included {
		gotIncluded[resource.Type+"/"+resource.ID] = true
	}
	for _, identity := range []string{"builds/build-1", "betaTesters/tester-1"} {
		if !gotIncluded[identity] {
			t.Fatalf("missing included resource %q: %+v", identity, output.Included)
		}
	}
}
