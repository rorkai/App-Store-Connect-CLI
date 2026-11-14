package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestDevicesRegisterBatchPaginatesAndSkipsNormalizedDuplicates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	inputPath := filepath.Join(t.TempDir(), "devices.txt")
	contents := strings.Join([]string{
		"# device registration",
		"Device ID\tDevice Name\tDevice Platform",
		"AA-BB-CC\tPhone One\tIOS",
		"aabbcc\tRepeated Phone\tios",
		"DD:EE:FF\tBuild Mac\tMAC_OS",
		"EXISTING-1\tExisting Phone\tIOS",
	}, "\n")
	if err := os.WriteFile(inputPath, []byte(contents), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(devicesListRequest(), asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/devices?cursor=page-2"}}`))
	reg.Register(asctest.GET("/v1/devices", "cursor=page-2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"devices","id":"device-existing","attributes":{"name":"Existing Phone","udid":"EXISTING1","platform":"IOS","status":"ENABLED"}}]}`))
	reg.Register(deviceCreateRequest("Phone One", "AA-BB-CC", "IOS"),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"devices","id":"device-AABBCC","attributes":{"name":"Phone One","udid":"AA-BB-CC","platform":"IOS"}}}`))
	reg.Register(deviceCreateRequest("Build Mac", "DD:EE:FF", "MAC_OS"),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"devices","id":"device-DDEEFF","attributes":{"name":"Build Mac","udid":"DD:EE:FF","platform":"MAC_OS"}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"devices", "register-batch", "--file", inputPath, "--output", "json", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, _ := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	var created []map[string]any
	for _, req := range reg.Requests() {
		if req.Method != http.MethodPost {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(req.Body, &payload); err != nil {
			t.Fatalf("decode create request: %v", err)
		}
		created = append(created, payload["data"].(map[string]any)["attributes"].(map[string]any))
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 registrations, got %d: %+v", len(created), created)
	}
	if created[0]["udid"] != "AA-BB-CC" || created[0]["platform"] != "IOS" {
		t.Fatalf("unexpected first create: %+v", created[0])
	}
	if created[1]["udid"] != "DD:EE:FF" || created[1]["platform"] != "MAC_OS" {
		t.Fatalf("unexpected second create: %+v", created[1])
	}

	var summary struct {
		Total      int `json:"total"`
		Registered int `json:"registered"`
		Skipped    int `json:"skipped"`
		Failed     int `json:"failed"`
		Results    []struct {
			Row    int    `json:"row"`
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode summary %q: %v", stdout, err)
	}
	if summary.Total != 4 || summary.Registered != 2 || summary.Skipped != 2 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Results) != 4 || summary.Results[1].Status != "skipped" || !strings.Contains(summary.Results[1].Reason, "duplicate") {
		t.Fatalf("expected duplicate input result, got %+v", summary.Results)
	}
	if summary.Results[3].Status != "skipped" || !strings.Contains(summary.Results[3].Reason, "already registered") {
		t.Fatalf("expected existing device result, got %+v", summary.Results[3])
	}
}

func TestDevicesRegisterBatchValidatesEveryRowBeforeNetwork(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	inputPath := filepath.Join(t.TempDir(), "devices.txt")
	if err := os.WriteFile(inputPath, []byte("UDID-1\tValid\tIOS\nUDID-2\tInvalid\tTV_OS\n"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	asctest.NewRegistry(t).Install()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"devices", "register-batch", "--file", inputPath, "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "--platform") {
		t.Fatalf("expected line-specific validation error, got %v", err)
	}
}

func TestDevicesRegisterBatchDryRunPlansWithoutCreating(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	inputPath := filepath.Join(t.TempDir(), "devices.txt")
	if err := os.WriteFile(inputPath, []byte("UDID-1\tDevice One\n"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(devicesListRequest(), asctest.JSON(http.StatusOK, `{"data":[]}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"devices", "register-batch", "--file", inputPath, "--platform", "ios", "--dry-run"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"planned":1`) || !strings.Contains(stdout, `"status":"planned"`) {
		t.Fatalf("expected planned dry-run output, got %q", stdout)
	}
}

func TestDevicesRegisterBatchContinuesAfterIsolatedFailure(t *testing.T) {
	for _, test := range []struct {
		name      string
		failFirst func() (*http.Response, error)
	}{
		{
			name: "API failure",
			failFirst: func() (*http.Response, error) {
				return jsonResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","detail":"invalid device"}]}`)
			},
		},
		{
			name:      "request timeout",
			failFirst: func() (*http.Response, error) { return nil, context.DeadlineExceeded },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			inputPath := filepath.Join(t.TempDir(), "devices.txt")
			if err := os.WriteFile(inputPath, []byte("UDID-1\tDevice One\tIOS\nUDID-2\tDevice Two\tIOS\n"), 0o600); err != nil {
				t.Fatalf("write input: %v", err)
			}

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(devicesListRequest(), asctest.JSON(http.StatusOK, `{"data":[]}`))
			reg.Register(deviceCreateRequest("Device One", "UDID-1", "IOS"), func(*http.Request) (*http.Response, error) { return test.failFirst() })
			reg.Register(deviceCreateRequest("Device Two", "UDID-2", "IOS"),
				asctest.JSON(http.StatusCreated, `{"data":{"type":"devices","id":"device-2","attributes":{"name":"Device Two","udid":"UDID-2","platform":"IOS"}}}`))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			if err := root.Parse([]string{"devices", "register-batch", "--file", inputPath, "--confirm"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			var runErr error
			stdout, _ := captureOutput(t, func() {
				runErr = root.Run(context.Background())
			})
			if runErr == nil {
				t.Fatal("expected reported partial failure")
			}
			if createCount := countDeviceCreates(reg); createCount != 2 {
				t.Fatalf("expected processing to continue after one failure, got %d creates", createCount)
			}
			if !strings.Contains(stdout, `"registered":1`) || !strings.Contains(stdout, `"failed":1`) || !strings.Contains(stdout, `"status":"failed"`) {
				t.Fatalf("expected machine-readable partial result, got %q", stdout)
			}
		})
	}
}

func TestDevicesRegisterBatchStopsAfterAPIFailureWhenRequested(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	inputPath := filepath.Join(t.TempDir(), "devices.txt")
	if err := os.WriteFile(inputPath, []byte("UDID-1\tDevice One\tIOS\nUDID-2\tDevice Two\tIOS\n"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(devicesListRequest(), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(deviceCreateRequest("Device One", "UDID-1", "IOS"),
		asctest.JSON(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","detail":"invalid device"}]}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"devices", "register-batch", "--file", inputPath, "--continue-on-error=false", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	stdout, _ := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected reported registration failure")
	}
	if createCount := countDeviceCreates(reg); createCount != 1 {
		t.Fatalf("create requests = %d, want 1", createCount)
	}
	var summary struct {
		Total     int `json:"total"`
		Processed int `json:"processed"`
		Failed    int `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode summary %q: %v", stdout, err)
	}
	if summary.Total != 2 || summary.Processed != 1 || summary.Failed != 1 {
		t.Fatalf("unexpected stop-on-error summary: %+v", summary)
	}
}

func TestDevicesRegisterBatchRequiresConfirmBeforeReadingFileOrNetwork(t *testing.T) {
	setupAuth(t)
	missingPath := filepath.Join(t.TempDir(), "missing.txt")

	asctest.NewRegistry(t).Install()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"devices", "register-batch", "--file", missingPath}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) || runErr.Error() != "--confirm is required unless --dry-run is set" {
		t.Fatalf("run error = %v, want exact confirmation usage error", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required unless --dry-run is set") {
		t.Fatalf("stderr = %q, want confirmation error", stderr)
	}
}

func devicesListRequest() asctest.Matcher {
	return asctest.GET("/v1/devices", "fields[devices]=name,udid,platform,status", "limit=200")
}

func deviceCreateRequest(name, udid, platform string) asctest.Matcher {
	return asctest.POST("/v1/devices").JSONBody(`{"data":{"type":"devices","attributes":{"name":"` + name + `","udid":"` + udid + `","platform":"` + platform + `"}}}`)
}

func countDeviceCreates(reg *asctest.Registry) int {
	count := 0
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPost {
			count++
		}
	}
	return count
}
