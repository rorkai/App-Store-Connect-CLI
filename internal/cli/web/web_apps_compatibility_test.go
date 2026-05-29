package web

import (
	"context"
	"encoding/json"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsCompatibilityEditUpdatesSelectedSettings(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origUpdate := updateWebAppCompatibilityFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		updateWebAppCompatibilityFn = origUpdate
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	var gotAppID string
	var gotMac *bool
	var gotVision *bool
	updateWebAppCompatibilityFn = func(ctx context.Context, client *webcore.Client, appID string, iosAppOnMac, iosAppOnVisionPro *bool) (*webcore.AppCompatibility, error) {
		gotAppID = appID
		gotMac = iosAppOnMac
		gotVision = iosAppOnVisionPro
		return &webcore.AppCompatibility{
			AppID:             appID,
			IOSAppOnMac:       iosAppOnMac,
			IOSAppOnVisionPro: iosAppOnVisionPro,
		}, nil
	}

	cmd := WebAppsCompatibilityEditCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--ios-app-on-mac=true",
		"--ios-app-on-vision-pro=false",
		"--output", "json",
	}); err != nil {
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
	if gotAppID != "app-1" {
		t.Fatalf("appID = %q, want app-1", gotAppID)
	}
	if gotMac == nil || !*gotMac {
		t.Fatalf("expected iosAppOnMac=true, got %+v", gotMac)
	}
	if gotVision == nil || *gotVision {
		t.Fatalf("expected iosAppOnVisionPro=false, got %+v", gotVision)
	}

	var out struct {
		AppID             string `json:"appId"`
		IOSAppOnMac       *bool  `json:"iosAppOnMac"`
		IOSAppOnVisionPro *bool  `json:"iosAppOnVisionPro"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v; stdout=%q", err, stdout)
	}
	if out.AppID != "app-1" {
		t.Fatalf("output appId = %q, want app-1", out.AppID)
	}
	if out.IOSAppOnMac == nil || !*out.IOSAppOnMac {
		t.Fatalf("expected output iosAppOnMac=true, got %+v", out.IOSAppOnMac)
	}
	if out.IOSAppOnVisionPro == nil || *out.IOSAppOnVisionPro {
		t.Fatalf("expected output iosAppOnVisionPro=false, got %+v", out.IOSAppOnVisionPro)
	}
}
