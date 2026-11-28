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

func TestXcodeInstallLaunchReceipts(t *testing.T) {
	old := runInstall
	t.Cleanup(func() { runInstall = old })
	for _, format := range []string{"json", "table", "markdown"} {
		for _, started := range []bool{true, false} {
			t.Run(format+"/"+map[bool]string{true: "started", false: "failed"}[started], func(t *testing.T) {
				runInstall = func(ctx context.Context, options localxcode.InstallOptions) (*asc.XcodeInstallResult, error) {
					if !options.Launch {
						t.Fatal("--launch not forwarded")
					}
					result := &asc.XcodeInstallResult{Success: started, Installed: true, Verified: true, Launch: &asc.XcodeInstallLaunch{Started: started}}
					if !started {
						result.FailureStage = "launch"
						result.FailureCode = "launch_failed"
						return result, errors.New("PRIVATE_LAUNCH_DETAIL")
					}
					return result, nil
				}
				d := newXcodeTestDeps(nil)
				cmd := newXcodeInstallCommand(d)
				cmd.FlagSet.SetOutput(io.Discard)
				if err := cmd.FlagSet.Parse([]string{"--ipa", "Fixture.ipa", "--device-id", "SELECTOR_CANARY", "--launch", "--output", format}); err != nil {
					t.Fatal(err)
				}
				var runErr error
				stdout, stderr := captureCommandOutput(t, d, func() error { runErr = cmd.Exec(context.Background(), nil); return runErr })
				if (runErr == nil) != started {
					t.Fatalf("err=%v", runErr)
				}
				if started && stderr != "" || !started && !strings.Contains(stderr, "xcode install failed at launch (launch_failed)") {
					t.Fatalf("stderr=%q", stderr)
				}
				if strings.Contains(stdout+stderr, "PRIVATE_LAUNCH_DETAIL") || strings.Contains(stdout+stderr, "SELECTOR_CANARY") {
					t.Fatalf("private details leaked: %q/%q", stdout, stderr)
				}
				if format == "json" {
					var result asc.XcodeInstallResult
					if err := json.Unmarshal([]byte(stdout), &result); err != nil {
						t.Fatal(err)
					}
					if !result.Installed || !result.Verified || result.Launch == nil || result.Launch.Started != started || result.Success != started {
						t.Fatalf("receipt=%+v", result)
					}
				} else if !strings.Contains(stdout, "launch_started") || !strings.Contains(stdout, "verified") {
					t.Fatalf("receipt=%q", stdout)
				}
			})
		}
	}
}
