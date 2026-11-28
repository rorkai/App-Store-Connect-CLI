package xcode

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestListDevicesUsesInstallSelectorsAndAvailability(t *testing.T) {
	oldGOOS, oldRunner := runtimeGOOS, installRunner
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS, installRunner = oldGOOS, oldRunner })
	for _, version := range []int{4, 5, 6} {
		t.Run(map[int]string{4: "legacy", 5: "modern", 6: "native"}[version], func(t *testing.T) {
			makeEntry := func(id, udid, name, reality, state, pairing string) map[string]any {
				hardware := map[string]any{"udid": udid, "platform": "iOS", "reality": reality}
				if version == 6 {
					hardware = map[string]any{"udid": udid, "osPlatform": "iOS", "fidelity": reality}
				}
				entry := map[string]any{"identifier": id}
				if version == 4 {
					entry["hardwareProperties"] = hardware
					entry["connectionProperties"] = map[string]any{"pairingState": pairing, "tunnelState": state}
					entry["deviceProperties"] = map[string]any{"name": name}
				} else {
					entry["properties"] = map[string]any{"hardware": hardware, "connection": map[string]any{"pairingState": pairing, "state": state}, "state": map[string]any{"name": name}}
				}
				return entry
			}
			payload, err := json.Marshal(map[string]any{"info": map[string]any{"commandType": "devicectl.list.devices", "jsonVersion": version, "outcome": "success"}, "result": map[string]any{"devices": []any{
				makeEntry("EXACT_SELECTOR", "HARDWARE_UDID", "Fixture Phone", "physical", "connected", "paired"),
				makeEntry("OFFLINE_SELECTOR", "OFFLINE_UDID", "Offline Phone", "physical", "disconnected", "paired"),
				makeEntry("MISSING_UDID", "", "Unknown Membership", "physical", "connected", "paired"),
				makeEntry("SIMULATOR_SELECTOR", "SIM_UDID", "Simulator", "simulated", "connected", "paired"),
			}}})
			if err != nil {
				t.Fatal(err)
			}
			runner := &fakeInstallRunner{t: t, deviceListPayload: string(payload)}
			installRunner = runner
			for _, availableOnly := range []bool{false, true} {
				result, err := ListDevices(context.Background(), DevicesOptions{AvailableOnly: availableOnly, Timeout: time.Minute})
				if err != nil {
					t.Fatal(err)
				}
				want := 3
				if availableOnly {
					want = 1
				}
				if len(result.Devices) != want {
					t.Fatalf("availableOnly=%v result=%+v", availableOnly, result)
				}
				found := false
				for _, device := range result.Devices {
					if device.DeviceID == "EXACT_SELECTOR" {
						found = true
						if !device.Available || device.Name != "Fixture Phone" {
							t.Fatalf("usable device=%+v", device)
						}
					} else if device.Available {
						t.Fatalf("unusable device offered available=%+v", device)
					}
				}
				if !found {
					t.Fatalf("missing installer selector: %+v", result)
				}
				encoded, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(encoded), "HARDWARE_UDID") || strings.Contains(string(encoded), "SIMULATOR_SELECTOR") {
					t.Fatalf("unintended private/nonphysical output: %s", encoded)
				}
			}
			device, err := discoverInstallDevice(context.Background(), runner, "/private/tmp/fake-devicectl", nil, t.TempDir(), time.Minute, "EXACT_SELECTOR", []string{"HARDWARE_UDID"})
			if err != nil || device.Identifier != "EXACT_SELECTOR" {
				t.Fatalf("listed selector rejected by install discovery: %+v err=%v", device, err)
			}
			if len(runner.calls) != 3 {
				t.Fatalf("calls=%q", runner.calls)
			}
			for _, args := range runner.calls {
				if !containsInstallSequence(args, []string{"list", "devices"}) || containsInstallArg(args, "install") {
					t.Fatalf("non-read discovery=%q", args)
				}
			}
		})
	}
}

type cancellingDevicesRunner struct {
	*fakeInstallRunner
	cancel   context.CancelFunc
	resolves int
}

func (r *cancellingDevicesRunner) ResolveDevicectl(ctx context.Context, _ []string) (string, error) {
	r.resolves++
	r.cancel()
	return "", ctx.Err()
}

func TestListDevicesPreservesCancellationBeforeDiscovery(t *testing.T) {
	oldGOOS, oldRunner := runtimeGOOS, installRunner
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS, installRunner = oldGOOS, oldRunner })
	for _, alreadyCancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "during tool resolution", true: "before tool resolution"}[alreadyCancelled], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runner := &cancellingDevicesRunner{fakeInstallRunner: &fakeInstallRunner{t: t}, cancel: cancel}
			installRunner = runner
			if alreadyCancelled {
				cancel()
			}
			result, err := ListDevices(ctx, DevicesOptions{Timeout: time.Minute})
			wantResolves := 1
			if alreadyCancelled {
				wantResolves = 0
			}
			if result != nil || !errors.Is(err, context.Canceled) || len(runner.calls) != 0 || runner.resolves != wantResolves {
				t.Fatalf("result=%+v err=%v resolves=%d discoveryCalls=%d", result, err, runner.resolves, len(runner.calls))
			}
		})
	}
}
