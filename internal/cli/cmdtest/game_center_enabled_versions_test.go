package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupAuth(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
}

func TestGameCenterEnabledVersionsValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "enabled-versions", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "enabled-versions", "compatible-versions"}, "--id is required"},
		{[]string{"game-center", "enabled-versions", "list", "--app", "APP_ID", "--limit", "300"}, "game-center enabled-versions list: --limit must be between 1 and 200"},
	})
}

func TestGameCenterEnabledVersionsListSuccess(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/APP_ID/gameCenterEnabledVersions", "limit=50"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"gameCenterEnabledVersions","id":"gc-1"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"game-center", "enabled-versions", "list", "--app", "APP_ID", "--limit", "50"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "\"gc-1\"") {
		t.Fatalf("expected enabled version ID in output, got %q", stdout)
	}
}

func TestGameCenterEnabledVersionsListNextURL(t *testing.T) {
	setupAuth(t)

	nextURL := "https://api.appstoreconnect.apple.com/v1/apps/app-123/gameCenterEnabledVersions?limit=2"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-123/gameCenterEnabledVersions", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"gameCenterEnabledVersions","id":"gc-2"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"game-center", "enabled-versions", "list", "--next", nextURL})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "\"gc-2\"") {
		t.Fatalf("expected enabled version ID in output, got %q", stdout)
	}
}

func TestGameCenterEnabledVersionsListPaginate(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/APP_ID/gameCenterEnabledVersions", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"gameCenterEnabledVersions","id":"gc-3"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/APP_ID/gameCenterEnabledVersions?page=2"}}`))
	reg.Register(asctest.GET("/v1/apps/APP_ID/gameCenterEnabledVersions", "page=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"gameCenterEnabledVersions","id":"gc-4"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"game-center", "enabled-versions", "list", "--app", "APP_ID", "--paginate"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "\"gc-3\"") || !strings.Contains(stdout, "\"gc-4\"") {
		t.Fatalf("expected paginated ids in output, got %q", stdout)
	}
}

func TestGameCenterEnabledVersionsCompatibleVersionsSuccess(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/gameCenterEnabledVersions/ENABLED_VERSION_ID/compatibleVersions", "limit=50"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"gameCenterEnabledVersions","id":"gc-5"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"game-center", "enabled-versions", "compatible-versions", "--id", "ENABLED_VERSION_ID", "--limit", "50"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "\"gc-5\"") {
		t.Fatalf("expected compatible version ID in output, got %q", stdout)
	}
}

func TestGameCenterEnabledVersionsCompatibleVersionsPaginate(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/gameCenterEnabledVersions/ENABLED_VERSION_ID/compatibleVersions", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"gameCenterEnabledVersions","id":"gc-6"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/gameCenterEnabledVersions/ENABLED_VERSION_ID/compatibleVersions?page=2"}}`))
	reg.Register(asctest.GET("/v1/gameCenterEnabledVersions/ENABLED_VERSION_ID/compatibleVersions", "page=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"gameCenterEnabledVersions","id":"gc-7"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"game-center", "enabled-versions", "compatible-versions", "--id", "ENABLED_VERSION_ID", "--paginate"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "\"gc-6\"") || !strings.Contains(stdout, "\"gc-7\"") {
		t.Fatalf("expected paginated ids in output, got %q", stdout)
	}
}
