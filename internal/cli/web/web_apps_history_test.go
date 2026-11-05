package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsHistoryPrintsStatusChanges(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stubWebAppsDistributionSession(t)

	origGet := getWebAppStatusHistoryFn
	t.Cleanup(func() { getWebAppStatusHistoryFn = origGet })

	var gotAppID, gotVersionID string
	getWebAppStatusHistoryFn = func(ctx context.Context, client *webcore.Client, appID, versionID string) (*webcore.AppStatusHistory, error) {
		gotAppID = appID
		gotVersionID = versionID
		return &webcore.AppStatusHistory{
			AppID: appID,
			Versions: []webcore.AppStatusHistoryVersion{{
				VersionID:     "v-2",
				VersionString: "2.0",
				Platform:      "IOS",
				Changes: []webcore.AppStatusChange{
					{ID: "c-2", AppStoreState: "READY_FOR_SALE", Date: "2025-02-01T00:00:00Z", Initiator: "Jane Appleseed"},
					{ID: "c-1", AppVersionState: "IN_REVIEW", Date: "2025-01-01T00:00:00Z"},
				},
			}},
		}, nil
	}

	cmd := WebAppsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if gotAppID != "app-1" || gotVersionID != "" {
		t.Fatalf("appID = %q versionID = %q, want app-1 and empty", gotAppID, gotVersionID)
	}

	var out struct {
		AppID    string `json:"appId"`
		Versions []struct {
			VersionID string `json:"versionId"`
			Changes   []struct {
				AppStoreState string `json:"appStoreState"`
				Date          string `json:"date"`
				Initiator     string `json:"initiator"`
			} `json:"changes"`
		} `json:"versions"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v; stdout=%q", err, stdout)
	}
	if out.AppID != "app-1" || len(out.Versions) != 1 || len(out.Versions[0].Changes) != 2 {
		t.Fatalf("unexpected output: %+v", out)
	}
	if out.Versions[0].Changes[0].Initiator != "Jane Appleseed" {
		t.Fatalf("unexpected initiator: %+v", out.Versions[0].Changes[0])
	}
}

func TestWebAppsHistoryPassesVersionIDFilter(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stubWebAppsDistributionSession(t)

	origGet := getWebAppStatusHistoryFn
	t.Cleanup(func() { getWebAppStatusHistoryFn = origGet })

	var gotVersionID string
	getWebAppStatusHistoryFn = func(ctx context.Context, client *webcore.Client, appID, versionID string) (*webcore.AppStatusHistory, error) {
		gotVersionID = versionID
		return &webcore.AppStatusHistory{AppID: appID}, nil
	}

	cmd := WebAppsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--version-id", "v-7", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if _, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}); stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if gotVersionID != "v-7" {
		t.Fatalf("versionID = %q, want v-7", gotVersionID)
	}
}

func TestWebAppsHistoryTableRendersStatusRows(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	stubWebAppsDistributionSession(t)

	origGet := getWebAppStatusHistoryFn
	t.Cleanup(func() { getWebAppStatusHistoryFn = origGet })

	getWebAppStatusHistoryFn = func(ctx context.Context, client *webcore.Client, appID, versionID string) (*webcore.AppStatusHistory, error) {
		return &webcore.AppStatusHistory{
			AppID: appID,
			Versions: []webcore.AppStatusHistoryVersion{{
				VersionID:     "v-2",
				VersionString: "2.0",
				Platform:      "IOS",
				Changes: []webcore.AppStatusChange{
					{ID: "c-2", AppVersionState: "READY_FOR_DISTRIBUTION", Date: "2025-02-01T00:00:00Z"},
				},
			}},
		}, nil
	}

	cmd := WebAppsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{"version", "status", "date", "initiator", "2.0", "READY_FOR_DISTRIBUTION", "unknown"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q; stdout=%q", want, stdout)
		}
	}
}

func TestWebAppsHistoryRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	cmd := WebAppsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var err error
	_, stderr := captureWebCommandOutput(t, func() {
		err = cmd.Exec(context.Background(), nil)
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if want := "Error: --app is required (or set ASC_APP_ID)\n"; !strings.HasPrefix(stderr, want) {
		t.Fatalf("stderr = %q, want prefix %q", stderr, want)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp usage contract", err)
	}
	if kind := shared.ClassifyUsageError(err); kind != shared.UsageErrorMissingRequired {
		t.Fatalf("usage kind = %q, want %q", kind, shared.UsageErrorMissingRequired)
	}
}

func TestWebAppsHistoryRejectsPositionalArguments(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	cmd := WebAppsHistoryCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{"extra"})
	if err == nil {
		t.Fatal("expected error for positional argument")
	}
	if !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("error = %v, want unexpected argument", err)
	}
}
