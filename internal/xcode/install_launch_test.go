package xcode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
)

type launchInstallRunner struct {
	*fakeInstallRunner
	launchPayload string
	launchErr     error
	cancel        context.CancelFunc
}

func (r *launchInstallRunner) Run(ctx context.Context, path string, args, env []string) error {
	if !containsInstallSequence(args, []string{"device", "process", "launch"}) {
		return r.fakeInstallRunner.Run(ctx, path, args, env)
	}
	r.calls = append(r.calls, append([]string(nil), args...))
	r.environments = append(r.environments, append([]string(nil), env...))
	for i, arg := range args {
		if arg == "--json-output" && i+1 < len(args) {
			if r.launchPayload != "" {
				if err := os.WriteFile(args[i+1], []byte(r.launchPayload), 0o600); err != nil {
					return err
				}
			}
		}
	}
	if r.cancel != nil {
		r.cancel()
	}
	return r.launchErr
}

func TestInstallLaunchAfterVerifiedIdentity(t *testing.T) {
	oldGOOS, oldRunner := runtimeGOOS, installRunner
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS, installRunner = oldGOOS, oldRunner })
	const success = `{"info":{"arguments":[],"commandType":"devicectl.device.process.launch","environment":{},"jsonVersion":5,"outcome":"success","version":"test"},"result":{"processIdentifier":123}}`
	for _, tc := range []struct {
		name, payload, verificationBuild string
		runErr                           error
		cancel                           bool
		wantCalls                        int
		wantStarted                      bool
	}{
		{name: "launch accepted", payload: success, wantCalls: 4, wantStarted: true},
		{name: "subprocess failed", payload: success, runErr: errors.New("private path and device"), wantCalls: 4},
		{name: "failed envelope", payload: `{"info":{"commandType":"devicectl.device.process.launch","jsonVersion":5,"outcome":"failure"},"error":{"code":4,"domain":"private"},"result":{}}`, wantCalls: 4},
		{name: "wrong command", payload: `{"info":{"commandType":"devicectl.device.info.apps","jsonVersion":5,"outcome":"success"},"result":{}}`, wantCalls: 4},
		{name: "cancelled", payload: success, cancel: true, wantCalls: 4},
		{name: "verification failed", verificationBuild: "46", wantCalls: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ipa := filepath.Join(t.TempDir(), "Fixture.ipa")
			if err := os.WriteFile(ipa, []byte("mock source"), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fake := &fakeInstallRunner{t: t, verificationBuild: tc.verificationBuild}
			runner := &launchInstallRunner{fakeInstallRunner: fake, launchPayload: tc.payload, launchErr: tc.runErr}
			if tc.cancel {
				runner.cancel = cancel
			}
			installRunner = runner
			stubInstallIPASource(t, &fakeInstallIPASource{inspection: installTestInspection(distribution.ProfileClassAdHoc), appPath: filepath.Join(t.TempDir(), "Fixture.app"), runner: fake})
			result, err := Install(ctx, InstallOptions{IPAPath: ipa, DeviceID: "SELECTOR_CANARY", Timeout: time.Minute, Launch: true})
			if result == nil || !result.Installed || (err == nil) != tc.wantStarted || result.Success != tc.wantStarted {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if len(runner.calls) != tc.wantCalls {
				t.Fatalf("calls=%q", runner.calls)
			}
			if tc.verificationBuild != "" {
				if result.Verified || result.Launch != nil {
					t.Fatalf("unexpected launch after failed verification: %+v", result)
				}
				return
			}
			if !result.Verified || result.Launch == nil || result.Launch.Started != tc.wantStarted {
				t.Fatalf("receipt=%+v", result)
			}
			args := runner.calls[3]
			if !containsInstallSequence(args, []string{"device", "process", "launch", "--device", "SELECTOR_CANARY", "com.example.demo"}) {
				t.Fatalf("launch argv=%q", args)
			}
			for _, forbidden := range []string{"--console", "--terminate-existing", "--environment-variables"} {
				if containsInstallArg(args, forbidden) {
					t.Fatalf("unexpected argv=%q", args)
				}
			}
			if !tc.wantStarted && (result.FailureStage != "launch" || result.FailureCode == "") {
				t.Fatalf("missing launch failure=%+v", result)
			}
		})
	}
}
