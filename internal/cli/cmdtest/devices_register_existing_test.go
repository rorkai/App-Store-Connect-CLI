package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestDevicesRegisterReusesExistingDeviceWithNormalizedUDID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/devices", "fields[devices]=name,udid,platform,status", "filter[platform]=IOS", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"devices","id":"device-existing","attributes":{"name":"Existing iPhone","platform":"IOS","udid":"ABC-123-DEF","status":"ENABLED"}}],"links":{"next":""}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"devices", "register", "--name", "Existing iPhone", "--udid", "ABC123DEF", "--platform", "IOS"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"device-existing"`) {
		t.Fatalf("expected existing device output, got %q", stdout)
	}
}

func TestDevicesRegisterStopsScanningAfterExistingUDIDMatch(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/devices", "fields[devices]=name,udid,platform,status", "filter[platform]=IOS", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"devices","id":"device-existing","attributes":{"name":"Existing iPhone","platform":"IOS","udid":"ABC-123-DEF","status":"ENABLED"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/devices?cursor=next"}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"devices", "register", "--name", "Existing iPhone", "--udid", "ABC123DEF", "--platform", "IOS"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"device-existing"`) {
		t.Fatalf("expected existing device output, got %q", stdout)
	}
	if requestCount := len(reg.Requests()); requestCount != 1 {
		t.Fatalf("expected one preflight request, got %d", requestCount)
	}
}

func TestDevicesRegisterCreatesDeviceWhenNoExistingUDIDMatches(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/devices", "fields[devices]=name,udid,platform,status", "filter[platform]=UNIVERSAL", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.POST("/v1/devices").JSONBody(`{"data":{"type":"devices","attributes":{"name":"New iPhone","udid":"NEW-UDID","platform":"UNIVERSAL"}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"devices","id":"device-new","attributes":{"name":"New iPhone","platform":"UNIVERSAL","udid":"NEW-UDID","status":"ENABLED"}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"devices", "register", "--name", "New iPhone", "--udid", "NEW-UDID", "--platform", "universal"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"device-new"`) {
		t.Fatalf("expected created device output, got %q", stdout)
	}
	if requestCount := len(reg.Requests()); requestCount != 2 {
		t.Fatalf("expected GET and POST requests, got %d", requestCount)
	}
}
