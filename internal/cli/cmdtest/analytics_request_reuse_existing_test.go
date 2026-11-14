package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAnalyticsRequestReuseExistingRejectsInvalidAccessType(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "request",
			"--app", "app-1",
			"--access-type", "DAILY",
			"--reuse-existing",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--access-type must be ONGOING or ONE_TIME_SNAPSHOT") {
		t.Fatalf("expected invalid access type error, got %q", stderr)
	}
}

func TestAnalyticsRequestReuseExistingUsesExistingRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/analyticsReportRequests", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[`+
		`{"type":"analyticsReportRequests","id":"req-snapshot","attributes":{"accessType":"ONE_TIME_SNAPSHOT","stoppedDueToInactivity":false}},`+
		`{"type":"analyticsReportRequests","id":"req-existing","attributes":{"accessType":"ONGOING","stoppedDueToInactivity":false}}`+
		`],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"analytics", "request",
		"--app", "app-1",
		"--access-type", "ONGOING",
		"--reuse-existing",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
	}
	if result["requestId"] != "req-existing" {
		t.Fatalf("expected existing request id, got %#v", result["requestId"])
	}
	if result["created"] != false {
		t.Fatalf("expected created=false, got %#v", result["created"])
	}
}

func TestAnalyticsRequestReuseExistingScansEveryPage(t *testing.T) {
	const (
		snapshot = `{"type":"analyticsReportRequests","id":"req-snapshot","attributes":{"accessType":"ONE_TIME_SNAPSHOT","stoppedDueToInactivity":false}}`
		ongoing  = `{"type":"analyticsReportRequests","id":"req-ongoing","attributes":{"accessType":"ONGOING","stoppedDueToInactivity":false}}`
	)
	tests := []struct {
		name  string
		pages [2]string
	}{
		{name: "match on later page", pages: [2]string{snapshot, ongoing}},
		{name: "match on first of multiple pages", pages: [2]string{ongoing, snapshot}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps/app-1/analyticsReportRequests", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[`+test.pages[0]+`],"links":{"next":"/v1/apps/app-1/analyticsReportRequests?cursor=2"}}`))
			reg.Register(asctest.GET("/v1/apps/app-1/analyticsReportRequests", "cursor=2"),
				asctest.JSON(http.StatusOK, `{"data":[`+test.pages[1]+`],"links":{"next":""}}`))

			stdout, stderr, err := runCommand(t, []string{
				"analytics", "request",
				"--app", "app-1",
				"--access-type", "ONGOING",
				"--reuse-existing",
				"--output", "json",
			})
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
			}
			if result["requestId"] != "req-ongoing" {
				t.Fatalf("expected existing ONGOING request id, got %#v", result["requestId"])
			}
			if result["created"] != false {
				t.Fatalf("expected created=false, got %#v", result["created"])
			}
			if got := len(reg.Requests()); got != 2 {
				t.Fatalf("request count = %d, want 2", got)
			}
		})
	}
}

func TestAnalyticsRequestReuseExistingCreatesMissingRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/analyticsReportRequests", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.POST("/v1/analyticsReportRequests").JSONBody(`{"data":{"type":"analyticsReportRequests","attributes":{"accessType":"ONGOING"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"analyticsReportRequests","id":"req-created","attributes":{"accessType":"ONGOING","stoppedDueToInactivity":false}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"analytics", "request",
		"--app", "app-1",
		"--access-type", "ONGOING",
		"--reuse-existing",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
	}
	if result["requestId"] != "req-created" {
		t.Fatalf("expected created request id, got %#v", result["requestId"])
	}
	if result["created"] != true {
		t.Fatalf("expected created=true, got %#v", result["created"])
	}
	for _, removed := range []string{"state", "createdDate"} {
		if _, ok := result[removed]; ok {
			t.Fatalf("removed output field %q is still present: %#v", removed, result)
		}
	}
}

func TestAnalyticsRequestReuseExistingRefetchesAfterCreateConflict(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/analyticsReportRequests", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.POST("/v1/analyticsReportRequests").JSONBody(`{"data":{"type":"analyticsReportRequests","attributes":{"accessType":"ONGOING"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR","title":"Conflict","detail":"duplicate request"}]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/analyticsReportRequests", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[`+
		`{"type":"analyticsReportRequests","id":"req-created-by-race","attributes":{"accessType":"ONGOING","stoppedDueToInactivity":false}}`+
		`],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"analytics", "request",
		"--app", "app-1",
		"--access-type", "ONGOING",
		"--reuse-existing",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
	}
	if result["requestId"] != "req-created-by-race" {
		t.Fatalf("expected raced request id, got %#v", result["requestId"])
	}
	if result["created"] != false {
		t.Fatalf("expected created=false, got %#v", result["created"])
	}
}
