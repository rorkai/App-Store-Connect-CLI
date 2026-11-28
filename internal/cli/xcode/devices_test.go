package xcode

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestXcodeDevicesSelectorsAndFormats(t *testing.T) {
	old := runListDevices
	t.Cleanup(func() { runListDevices = old })
	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			runListDevices = func(ctx context.Context, options localxcode.DevicesOptions) (*asc.XcodeDevicesResult, error) {
				if !options.AvailableOnly || options.Timeout != time.Minute {
					t.Fatalf("options=%+v", options)
				}
				return &asc.XcodeDevicesResult{Devices: []asc.XcodeDevice{{DeviceID: "EXACT_SELECTOR", Name: "Fixture Phone", Platform: "iOS", PairingState: "paired", ConnectionState: "connected", Available: true}}}, nil
			}
			d := newXcodeTestDeps(nil)
			cmd := newXcodeDevicesCommand(d)
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{"--available-only", "--timeout", "1m", "--output", format}); err != nil {
				t.Fatal(err)
			}
			stdout, stderr := captureCommandOutput(t, d, func() error { return cmd.Exec(context.Background(), nil) })
			if stderr != "" {
				t.Fatalf("stderr=%q", stderr)
			}
			if format == "json" {
				var result asc.XcodeDevicesResult
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatal(err)
				}
				if len(result.Devices) != 1 || result.Devices[0].DeviceID != "EXACT_SELECTOR" || !result.Devices[0].Available {
					t.Fatalf("result=%+v", result)
				}
			} else if !strings.Contains(stdout, "EXACT_SELECTOR") || !strings.Contains(stdout, "Fixture Phone") || !strings.Contains(stdout, "true") {
				t.Fatalf("output=%q", stdout)
			}
		})
	}
}
