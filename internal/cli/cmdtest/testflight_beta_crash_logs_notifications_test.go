package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightBetaNotificationsCreateOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/build-1/buildBetaDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`))
	reg.Register(asctest.POST("/v1/buildBetaNotifications").JSONBody(`{"data":{"type":"buildBetaNotifications","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"buildBetaNotifications","id":"notif-1"}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "notifications", "send", "--build-id", "build-1"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"notif-1"`) {
		t.Fatalf("expected notification id in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"notificationAction":`) {
		t.Fatalf("expected notificationAction to remain omitted for manual success, got %q", stdout)
	}
}

func TestTestFlightBetaNotificationsCreateReportsPreflightAutoNotify(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/build-1/buildBetaDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "notifications", "send", "--build-id", "build-1"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stdout, `"notificationAction":"auto_notify_enabled"`) {
		t.Fatalf("expected auto-notify action in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"data":`) || strings.Contains(stdout, `"links":`) {
		t.Fatalf("expected action-only output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 1 {
		t.Fatalf("expected one build beta detail request, got %d", requestCount)
	}
}

func TestTestFlightBetaNotificationsCreateReportsRaceAutoNotify(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/build-1/buildBetaDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`))
	reg.Register(asctest.POST("/v1/buildBetaNotifications").JSONBody(`{"data":{"type":"buildBetaNotifications","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"There is a problem with the request entity","detail":"Auto-notify already enabled"}]}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "notifications", "send", "--build-id", "build-1"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stdout, `"notificationAction":"auto_notify_enabled"`) {
		t.Fatalf("expected auto-notify action in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"data":`) || strings.Contains(stdout, `"links":`) {
		t.Fatalf("expected action-only output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 2 {
		t.Fatalf("expected detail preflight and notification create requests, got %d", requestCount)
	}
}

func TestTestFlightBetaNotificationsCreatePreservesUnrelatedConflict(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/build-1/buildBetaDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`))
	reg.Register(asctest.POST("/v1/buildBetaNotifications").JSONBody(`{"data":{"type":"buildBetaNotifications","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"There is a problem with the request entity","detail":"Build is not eligible for notification"}]}`))

	stdout, stderr, runErr := runCommand(t, []string{"testflight", "notifications", "send", "--build-id", "build-1"})

	if runErr == nil {
		t.Fatal("expected unrelated conflict error")
	}
	if !strings.Contains(runErr.Error(), "notifications send: failed to send:") {
		t.Fatalf("expected send error prefix, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "Build is not eligible for notification") {
		t.Fatalf("expected unrelated conflict detail, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 2 {
		t.Fatalf("expected detail preflight and notification create requests, got %d", requestCount)
	}
}
