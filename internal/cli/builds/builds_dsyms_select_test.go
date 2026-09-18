package builds

import (
	"strings"
	"testing"
	"time"
)

func TestParseDSYMSelectionExactVersionDoesNotRequireLatest(t *testing.T) {
	selection, err := parseDSYMSelection(dsymFlagInput{
		AppID:   "123",
		Version: "1.2.3",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !selection.selectsByMarketingVersion() || selection.Resolve.Latest || selection.All || selection.Resolve.Version != "1.2.3" {
		t.Fatalf("selection = %+v, want exact version without --latest", selection)
	}
}

func TestParseDSYMSelectionVersionLatestMatchesLatestFlag(t *testing.T) {
	selection, err := parseDSYMSelection(dsymFlagInput{
		AppID:   "123",
		Version: "latest",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !selection.Resolve.Latest || selection.Resolve.Version != "" || selection.Live || selection.Multi {
		t.Fatalf("selection = %+v, want latest without a version filter", selection)
	}
}

func TestParseDSYMSelectionRejectsTimeoutWithoutWait(t *testing.T) {
	_, err := parseDSYMSelection(dsymFlagInput{
		BuildID:    "build-1",
		Timeout:    time.Second,
		TimeoutSet: true,
	})
	if err == nil || !strings.Contains(err.Error(), "--timeout and --poll-interval require --wait") {
		t.Fatalf("error = %v, want timeout requiring --wait", err)
	}
}

func TestChooseLiveVersionRequiresPlatformWhenSeveralAreLive(t *testing.T) {
	_, err := chooseLiveVersion([]liveAppVersion{
		{Version: "2.0", Platform: "IOS", CreatedDate: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
		{Version: "1.4", Platform: "MAC_OS", CreatedDate: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "--platform") || !strings.Contains(err.Error(), "IOS 2.0") || !strings.Contains(err.Error(), "MAC_OS 1.4") {
		t.Fatalf("error = %v, want both live platforms", err)
	}
}

func TestCompareMarketingVersions(t *testing.T) {
	got, err := compareMarketingVersions("1.2", "1.2.0")
	if err != nil || got != 0 {
		t.Fatalf("1.2 vs 1.2.0 = %d, %v", got, err)
	}
	got, err = compareMarketingVersions("1.10", "1.9")
	if err != nil || got <= 0 {
		t.Fatalf("1.10 vs 1.9 = %d, %v", got, err)
	}
	got, err = compareMarketingVersions("1.1.9", "1.2.0")
	if err != nil || got >= 0 {
		t.Fatalf("1.1.9 vs 1.2.0 = %d, %v", got, err)
	}
}
