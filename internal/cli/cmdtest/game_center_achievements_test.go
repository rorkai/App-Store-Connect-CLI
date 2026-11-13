package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestGameCenterAchievementsValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "achievements", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "achievements", "create", "--reference-name", "Test", "--vendor-id", "com.test", "--points", "10"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "achievements", "create", "--app", "APP_ID", "--group-id", "GROUP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--points", "10"}, "--app cannot be used with --group-id"},
		{[]string{"game-center", "achievements", "create", "--group-id", "GROUP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--points", "10"}, "--vendor-id must start with \"grp.\" when using --group-id"},
		{[]string{"game-center", "achievements", "create", "--app", "APP_ID", "--vendor-id", "com.test", "--points", "10"}, "--reference-name is required"},
		{[]string{"game-center", "achievements", "create", "--app", "APP_ID", "--reference-name", "Test", "--points", "10"}, "--vendor-id is required"},
		{[]string{"game-center", "achievements", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test"}, "--points must be between 1 and 100"},
		{[]string{"game-center", "achievements", "submit", "--percentage", "100", "--bundle-id", "BUNDLE_ID", "--scoped-player-id", "PLAYER_ID"}, "--vendor-id is required"},
		{[]string{"game-center", "achievements", "submit", "--vendor-id", "com.example.achievement", "--bundle-id", "BUNDLE_ID", "--scoped-player-id", "PLAYER_ID"}, "--percentage is required"},
		{[]string{"game-center", "achievements", "submit", "--vendor-id", "com.example.achievement", "--percentage", "100", "--scoped-player-id", "PLAYER_ID"}, "--bundle-id is required"},
		{[]string{"game-center", "achievements", "submit", "--vendor-id", "com.example.achievement", "--percentage", "100", "--bundle-id", "BUNDLE_ID"}, "--scoped-player-id is required"},
		{[]string{"game-center", "achievements", "update"}, "--id is required"},
		{[]string{"game-center", "achievements", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "achievements", "delete", "--id", "ACH_ID"}, "--confirm is required"},
		{[]string{"game-center", "achievements", "localizations", "list"}, "--achievement-id is required"},
		{[]string{"game-center", "achievements", "localizations", "create", "--locale", "en-US", "--name", "Test", "--before-earned-description", "Before", "--after-earned-description", "After"}, "--achievement-id is required"},
		{[]string{"game-center", "achievements", "localizations", "create", "--achievement-id", "ACH_ID", "--name", "Test", "--before-earned-description", "Before", "--after-earned-description", "After"}, "--locale is required"},
		{[]string{"game-center", "achievements", "localizations", "create", "--achievement-id", "ACH_ID", "--locale", "en-US", "--before-earned-description", "Before", "--after-earned-description", "After"}, "--name is required"},
		{[]string{"game-center", "achievements", "localizations", "create", "--achievement-id", "ACH_ID", "--locale", "en-US", "--name", "Test", "--after-earned-description", "After"}, "--before-earned-description is required"},
		{[]string{"game-center", "achievements", "localizations", "create", "--achievement-id", "ACH_ID", "--locale", "en-US", "--name", "Test", "--before-earned-description", "Before"}, "--after-earned-description is required"},
		{[]string{"game-center", "achievements", "images", "upload", "--file", "test.png"}, "--localization-id is required"},
		{[]string{"game-center", "achievements", "images", "upload", "--localization-id", "LOC_ID"}, "--file is required"},
		{[]string{"game-center", "achievements", "releases", "list"}, "--achievement-id is required"},
	})
}

func TestGameCenterAchievementsListNoDetailReturnsEmptyList(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	expectedURL := "https://api.appstoreconnect.apple.com/v1/apps/APP_ID/gameCenterDetail"

	callCount := &lockedCounter{}
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if callCount.Inc() > 1 {
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
		}
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != expectedURL {
			t.Fatalf("expected URL %s, got %s", expectedURL, req.URL.String())
		}

		body := `{"data":{"type":"gameCenterDetails","id":"","attributes":{}},"links":{"self":"` + expectedURL + `"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "list", "--app", "APP_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Warning: no Game Center detail exists for this app") {
		t.Fatalf("expected warning in stderr, got %q", stderr)
	}

	var resp struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %q", err, stdout)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected empty data array, got %d items", len(resp.Data))
	}
}
