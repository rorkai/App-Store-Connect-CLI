package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestPublishTestflightExistingBuildIDSkipsUpload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-1"}]}`),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-1",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"buildId":"build-1"`) {
		t.Fatalf("expected build ID in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"uploaded":false`) {
		t.Fatalf("expected uploaded=false in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"notified":`) {
		t.Fatalf("expected notified to be omitted when --notify is not set, got %q", stdout)
	}
	if strings.Contains(stdout, `"notificationAction":`) {
		t.Fatalf("expected notificationAction to be omitted when --notify is not set, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildDoesNotRetryPostUploadPropagationError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-1"}]}`),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'builds' with id 'build-1'"}]}`))

	stdout, stderr, runErr := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-1",
	})

	if runErr == nil || !strings.Contains(runErr.Error(), "failed to add groups") {
		t.Fatalf("expected existing-build relationship failure, got %v", runErr)
	}
	if requestCount := len(reg.Requests()); requestCount != 3 {
		t.Fatalf("request count = %d, want 3 with no confirming GET or retry", requestCount)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("existing-build failure must not emit uploaded-build recovery output: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestPublishTestflightExistingBuildIDAllowsInternalGroup(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-internal"}]}`),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-internal",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildIDNotifyUsesBuildBetaNotificationsEndpoint(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-internal"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/builds/build-1/buildBetaDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`))
	reg.Register(asctest.POST("/v1/buildBetaNotifications").JSONBody(`{"data":{"type":"buildBetaNotifications","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"buildBetaNotifications","id":"notif-1"}}`))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-internal",
		"--notify",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 5 {
		t.Fatalf("expected group lookup, build fetch, group assignment, build beta detail fetch, and beta notification; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notified":true`) {
		t.Fatalf("expected notified=true in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notificationAction":"manual"`) {
		t.Fatalf("expected notificationAction=manual in output, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildIDAddsInternalGroupWithAccessToAllBuilds(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-internal"}]}`),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-internal",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 3 {
		t.Fatalf("expected group lookup, build fetch, and group assignment; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"buildId":"build-1"`) {
		t.Fatalf("expected build ID in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"uploaded":false`) {
		t.Fatalf("expected uploaded=false in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"notified":`) {
		t.Fatalf("expected notified to be omitted when --notify is not set, got %q", stdout)
	}
	if strings.Contains(stdout, `"notificationAction":`) {
		t.Fatalf("expected notificationAction to be omitted when --notify is not set, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestPublishTestflightExistingBuildIDWithInternalAllBuildsGroupStillNotifies(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-internal"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/builds/build-1/buildBetaDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`))
	reg.Register(asctest.POST("/v1/buildBetaNotifications").JSONBody(`{"data":{"type":"buildBetaNotifications","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"buildBetaNotifications","id":"notif-1"}}`))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-internal",
		"--notify",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 5 {
		t.Fatalf("expected group lookup, build fetch, group assignment, build beta detail fetch, and beta notification; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notified":true`) {
		t.Fatalf("expected notified=true in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notificationAction":"manual"`) {
		t.Fatalf("expected notificationAction=manual in output, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildIDNotifySkipsManualNotificationWhenAutoNotifyEnabled(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-internal"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/builds/build-1/buildBetaDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-internal",
		"--notify",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 4 {
		t.Fatalf("expected group lookup, build fetch, group assignment, and build beta detail fetch; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notified":false`) {
		t.Fatalf("expected notified=false in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notificationAction":"auto_notify_enabled"`) {
		t.Fatalf("expected notificationAction=auto_notify_enabled in output, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildIDAddsInternalAndExternalGroupsWhenInternalHasAllBuilds(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}},{"type":"betaGroups","id":"group-external","attributes":{"name":"External","isInternalGroup":false}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-internal"},{"type":"betaGroups","id":"group-external"}]}`),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-internal,group-external",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 3 {
		t.Fatalf("expected group lookup, build fetch, and group assignment; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal","group-external"]`) {
		t.Fatalf("expected internal and external groups in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"notified":`) {
		t.Fatalf("expected notified to be omitted when --notify is not set, got %q", stdout)
	}
	if strings.Contains(stdout, `"notificationAction":`) {
		t.Fatalf("expected notificationAction to be omitted when --notify is not set, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestPublishTestflightExistingBuildIDNotifyTreatsAutoNotifyConflictAsAlreadyEnabled(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-internal"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/builds/build-1/buildBetaDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`))
	reg.Register(asctest.POST("/v1/buildBetaNotifications").JSONBody(`{"data":{"type":"buildBetaNotifications","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"There is a problem with the request entity","detail":"Auto-notify already enabled"}]}`))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-internal",
		"--notify",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notified":false`) {
		t.Fatalf("expected notified=false in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notificationAction":"auto_notify_enabled"`) {
		t.Fatalf("expected notificationAction=auto_notify_enabled in output, got %q", stdout)
	}
	if requestCount := len(reg.Requests()); requestCount != 5 {
		t.Fatalf("expected 5 requests, got %d", requestCount)
	}
}

func TestPublishTestflightExistingBuildIDNotifyPreservesPartialSuccessMessageWhenNotificationFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Internal","isInternalGroup":true,"hasAccessToAllBuilds":true}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-internal"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/builds/build-1/buildBetaDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`))
	reg.Register(asctest.POST("/v1/buildBetaNotifications").JSONBody(`{"data":{"type":"buildBetaNotifications","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusServiceUnavailable, `{"errors":[{"code":"INTERNAL_ERROR","title":"Service unavailable","detail":"email delivery temporarily unavailable"}]}`))

	stdout, stderr, runErr := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-internal",
		"--notify",
	})

	if runErr == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), `publish testflight: beta groups were added to build "build-1", but notifying testers failed`) {
		t.Fatalf("expected partial-success publish error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "failed to add groups") {
		t.Fatalf("did not expect misleading add-groups wrapper, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "Service unavailable: email delivery temporarily unavailable") {
		t.Fatalf("expected underlying notification error, got %v", runErr)
	}
	if requestCount := len(reg.Requests()); requestCount != 5 {
		t.Fatalf("expected 5 requests, got %d", requestCount)
	}
}

func TestPublishTestflightExistingBuildNumberResolvesAndWaits(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123", "filter[preReleaseVersion.platform]=IOS", "filter[processingState]=PROCESSING,FAILED,INVALID,VALID", "filter[version]=42", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"PROCESSING"}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-42"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"PROCESSING"}}}`))
	reg.Register(asctest.GET("/v1/builds/build-42"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-42/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-1"}]}`),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-number", "42",
		"--group", "group-1",
		"--wait",
		"--poll-interval", "1ms",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"buildId":"build-42"`) {
		t.Fatalf("expected build ID in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"uploaded":false`) {
		t.Fatalf("expected uploaded=false in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"processingState":"VALID"`) {
		t.Fatalf("expected processingState VALID in output, got %q", stdout)
	}
}

func TestPublishTestflightExistingBuildNumberNotifyUsesBuildBetaNotificationsEndpoint(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123", "filter[preReleaseVersion.platform]=IOS", "filter[processingState]=PROCESSING,FAILED,INVALID,VALID", "filter[version]=42", "limit=1", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}]}`))
	reg.Register(asctest.POST("/v1/builds/build-42/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-1"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/builds/build-42/buildBetaDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-42","attributes":{"autoNotifyEnabled":false}}}`))
	reg.Register(asctest.POST("/v1/buildBetaNotifications").JSONBody(`{"data":{"type":"buildBetaNotifications","relationships":{"build":{"data":{"type":"builds","id":"build-42"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"buildBetaNotifications","id":"notif-42"}}`))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-number", "42",
		"--group", "group-1",
		"--notify",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 5 {
		t.Fatalf("expected build lookup, group assignment, build beta detail fetch, and beta notification; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"buildId":"build-42"`) {
		t.Fatalf("expected build ID in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notified":true`) {
		t.Fatalf("expected notified=true in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"notificationAction":"manual"`) {
		t.Fatalf("expected notificationAction=manual in output, got %q", stdout)
	}
}
