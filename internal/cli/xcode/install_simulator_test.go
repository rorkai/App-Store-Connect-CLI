package xcode

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestXcodeInstallSimulatorReceipts(t *testing.T) {
	old := runInstallSimulator
	t.Cleanup(func() { runInstallSimulator = old })
	for _, format := range []string{"json", "table", "markdown"} {
		for _, verified := range []bool{true, false} {
			t.Run(format+"/"+map[bool]string{true: "verified", false: "verification failed"}[verified], func(t *testing.T) {
				runInstallSimulator = func(ctx context.Context, options localxcode.SimulatorInstallOptions) (*asc.XcodeSimulatorInstallResult, error) {
					if options.UDID != "11111111-1111-1111-1111-111111111111" || !strings.HasSuffix(options.AppPath, "Fixture.app") {
						t.Fatalf("options=%+v", options)
					}
					result := &asc.XcodeSimulatorInstallResult{BundleID: "com.example.fixture", Version: "1.0", BuildNumber: "1", Installed: true, Verified: verified}
					if !verified {
						return result, errors.New("installed simulator app identity did not match")
					}
					return result, nil
				}
				d := newXcodeTestDeps(nil)
				cmd := newXcodeInstallSimulatorCommand(d)
				cmd.FlagSet.SetOutput(io.Discard)
				if err := cmd.FlagSet.Parse([]string{"--app", "Fixture.app", "--udid", "11111111-1111-1111-1111-111111111111", "--output", format}); err != nil {
					t.Fatal(err)
				}
				var resultErr error
				stdout, stderr := captureCommandOutput(t, d, func() error { resultErr = cmd.Exec(context.Background(), nil); return resultErr })
				if (resultErr == nil) != verified || stderr != "" {
					t.Fatalf("err=%v stderr=%q", resultErr, stderr)
				}
				if format == "json" {
					var result asc.XcodeSimulatorInstallResult
					if err := json.Unmarshal([]byte(stdout), &result); err != nil {
						t.Fatal(err)
					}
					if !result.Installed || result.Verified != verified || result.BundleID != "com.example.fixture" {
						t.Fatalf("receipt=%+v", result)
					}
				} else if !strings.Contains(stdout, "com.example.fixture") || !strings.Contains(stdout, "installed") || !strings.Contains(stdout, "verified") {
					t.Fatalf("receipt=%q", stdout)
				}
			})
		}
	}
}
