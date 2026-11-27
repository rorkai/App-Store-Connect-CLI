package xcode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"howett.net/plist"
)

func simulatorFixtureApp(t *testing.T, platform, version string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("simulator app fixture requires POSIX executable permission bits")
	}
	app := filepath.Join(t.TempDir(), "Fixture.app")
	if err := os.Mkdir(app, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := plist.Marshal(map[string]any{"CFBundleIdentifier": "com.example.fixture", "CFBundleExecutable": "Fixture", "CFBundleShortVersionString": version, "CFBundleVersion": "1", "DTPlatformName": platform}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Info.plist"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Fixture"), []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestInstallSimulatorTargetsAndVerifiesExactApp(t *testing.T) {
	oldGOOS, oldList, oldRun := runtimeGOOS, simulatorInstallList, simulatorInstallRun
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS, simulatorInstallList, simulatorInstallRun = oldGOOS, oldList, oldRun })
	const id = "11111111-1111-1111-1111-111111111111"
	app := simulatorFixtureApp(t, "iphonesimulator", "1.0")
	installed := simulatorFixtureApp(t, "iphonesimulator", "1.0")
	simulatorInstallList = func(context.Context) ([]byte, error) {
		return []byte(`{"devices":{"com.apple.CoreSimulator.SimRuntime.iOS-27-2":[{"udid":"` + id + `","state":"Booted","isAvailable":true}]}}`), nil
	}
	for _, tc := range []struct {
		name, installedPath         string
		installErr                  error
		cancelAfter                 string
		wantInstalled, wantVerified bool
		wantCalls                   int
	}{
		{name: "verified", installedPath: installed, wantInstalled: true, wantVerified: true, wantCalls: 2},
		{name: "different installed version", installedPath: simulatorFixtureApp(t, "iphonesimulator", "2.0"), wantInstalled: true, wantCalls: 2},
		{name: "install failed", installErr: errors.New("private device and path"), wantCalls: 1},
		{name: "cancel after install", installedPath: installed, cancelAfter: "install", wantInstalled: true, wantCalls: 1},
		{name: "cancel after container lookup", installedPath: installed, cancelAfter: "get_app_container", wantInstalled: true, wantCalls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var calls [][]string
			simulatorInstallRun = func(ctx context.Context, args ...string) ([]byte, error) {
				calls = append(calls, append([]string(nil), args...))
				if args[1] == tc.cancelAfter {
					cancel()
				}
				if len(calls) == 1 {
					return nil, tc.installErr
				}
				return []byte(tc.installedPath + "\n"), nil
			}
			result, err := InstallSimulator(ctx, SimulatorInstallOptions{AppPath: app, UDID: id, Timeout: time.Minute})
			if result == nil || result.Installed != tc.wantInstalled || result.Verified != tc.wantVerified || (err == nil) != tc.wantVerified {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			want := [][]string{{"simctl", "install", id, app}}
			if tc.wantCalls == 2 {
				want = append(want, []string{"simctl", "get_app_container", id, "com.example.fixture", "app"})
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls=%q want=%q", calls, want)
			}
			if tc.cancelAfter != "" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation cause lost: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "private device") {
				t.Fatalf("tool diagnostic leaked: %v", err)
			}
			if tc.wantVerified && (result.BundleID != "com.example.fixture" || result.Version != "1.0" || result.BuildNumber != "1") {
				t.Fatalf("installed identity=%+v", result)
			}
		})
	}
}

func TestInstallSimulatorRejectsBeforeMutation(t *testing.T) {
	oldGOOS, oldList, oldRun := runtimeGOOS, simulatorInstallList, simulatorInstallRun
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS, simulatorInstallList, simulatorInstallRun = oldGOOS, oldList, oldRun })
	const id = "11111111-1111-1111-1111-111111111111"
	for _, tc := range []struct {
		name, platform, inventory string
		cancelled                 bool
	}{
		{"physical app", "iphoneos", `{"devices":{}}`, false},
		{"shutdown target", "iphonesimulator", `{"devices":{"com.apple.CoreSimulator.SimRuntime.iOS-27-2":[{"udid":"` + id + `","state":"Shutdown","isAvailable":true}]}}`, false},
		{"unavailable target", "iphonesimulator", `{"devices":{"com.apple.CoreSimulator.SimRuntime.iOS-27-2":[{"udid":"` + id + `","state":"Booted","isAvailable":false}]}}`, false},
		{"non ios target", "iphonesimulator", `{"devices":{"com.apple.CoreSimulator.SimRuntime.tvOS-27-2":[{"udid":"` + id + `","state":"Booted","isAvailable":true}]}}`, false},
		{"duplicate target", "iphonesimulator", `{"devices":{"com.apple.CoreSimulator.SimRuntime.iOS-27-2":[{"udid":"` + id + `","state":"Booted","isAvailable":true},{"udid":"` + id + `","state":"Booted","isAvailable":true}]}}`, false},
		{"cancelled", "iphonesimulator", `{"devices":{}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := simulatorFixtureApp(t, tc.platform, "1.0")
			simulatorInstallList = func(ctx context.Context) ([]byte, error) { return []byte(tc.inventory), nil }
			calls := 0
			simulatorInstallRun = func(context.Context, ...string) ([]byte, error) { calls++; return nil, nil }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			_, err := InstallSimulator(ctx, SimulatorInstallOptions{AppPath: app, UDID: id, Timeout: time.Minute})
			if err == nil || calls != 0 {
				t.Fatalf("err=%v mutating calls=%d", err, calls)
			}
		})
	}
}

func TestInstallSimulatorRejectsUnprovenMetadataAndChangedSource(t *testing.T) {
	oldGOOS, oldList, oldRun := runtimeGOOS, simulatorInstallList, simulatorInstallRun
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS, simulatorInstallList, simulatorInstallRun = oldGOOS, oldList, oldRun })
	const id = "11111111-1111-1111-1111-111111111111"
	inventory := []byte(`{"devices":{"com.apple.CoreSimulator.SimRuntime.iOS-27-2":[{"udid":"` + id + `","state":"Booted","isAvailable":true}]}}`)
	for _, tc := range []struct {
		name, changeAt, part     string
		emptyVersion, emptyBuild bool
	}{
		{name: "missing version", emptyVersion: true},
		{name: "missing build", emptyBuild: true},
		{name: "bundle replaced during discovery", changeAt: "discovery", part: "bundle"},
		{name: "plist replaced during discovery", changeAt: "discovery", part: "plist"},
		{name: "executable replaced during discovery", changeAt: "discovery", part: "executable"},
		{name: "bundle replaced during install", changeAt: "install", part: "bundle"},
		{name: "plist replaced during install", changeAt: "install", part: "plist"},
		{name: "executable replaced during install", changeAt: "install", part: "executable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := simulatorFixtureApp(t, "iphonesimulator", "1.0")
			installed := simulatorFixtureApp(t, "iphonesimulator", "1.0")
			if tc.emptyVersion || tc.emptyBuild {
				path := filepath.Join(app, "Info.plist")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var metadata map[string]any
				if _, err := plist.Unmarshal(data, &metadata); err != nil {
					t.Fatal(err)
				}
				if tc.emptyVersion {
					delete(metadata, "CFBundleShortVersionString")
				}
				if tc.emptyBuild {
					delete(metadata, "CFBundleVersion")
				}
				data, err = plist.Marshal(metadata, plist.XMLFormat)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			replace := func() {
				replacement := simulatorFixtureApp(t, "iphonesimulator", "1.0")
				switch tc.part {
				case "bundle":
					if err := os.Rename(app, app+".previous"); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(replacement, app); err != nil {
						t.Fatal(err)
					}
				case "plist":
					if err := os.Rename(filepath.Join(replacement, "Info.plist"), filepath.Join(app, "Info.plist")); err != nil {
						t.Fatal(err)
					}
				case "executable":
					if err := os.Rename(filepath.Join(replacement, "Fixture"), filepath.Join(app, "Fixture")); err != nil {
						t.Fatal(err)
					}
				}
			}
			discoveries, installs := 0, 0
			simulatorInstallList = func(context.Context) ([]byte, error) {
				discoveries++
				if tc.changeAt == "discovery" {
					replace()
				}
				return inventory, nil
			}
			simulatorInstallRun = func(ctx context.Context, args ...string) ([]byte, error) {
				if args[1] == "install" {
					installs++
					if tc.changeAt == "install" {
						replace()
					}
					return nil, nil
				}
				return []byte(installed), nil
			}
			result, err := InstallSimulator(context.Background(), SimulatorInstallOptions{AppPath: app, UDID: id, Timeout: time.Minute})
			if err == nil || result != nil && result.Verified {
				t.Fatalf("unproven verified result=%+v err=%v", result, err)
			}
			if tc.emptyVersion || tc.emptyBuild {
				if discoveries != 0 || installs != 0 {
					t.Fatalf("incomplete metadata reached tools: discovery=%d installs=%d", discoveries, installs)
				}
			}
			if tc.changeAt == "discovery" && installs != 0 {
				t.Fatalf("changed input installed: %d", installs)
			}
			if tc.changeAt == "install" && (result == nil || !result.Installed) {
				t.Fatalf("completed install receipt lost: %+v", result)
			}
		})
	}
}

func TestInstallSimulatorRejectsUnsupportedHostBeforeTools(t *testing.T) {
	oldGOOS, oldList, oldRun := runtimeGOOS, simulatorInstallList, simulatorInstallRun
	t.Cleanup(func() { runtimeGOOS, simulatorInstallList, simulatorInstallRun = oldGOOS, oldList, oldRun })
	calls := 0
	simulatorInstallList = func(context.Context) ([]byte, error) { calls++; return nil, errors.New("discovery reached") }
	simulatorInstallRun = func(context.Context, ...string) ([]byte, error) { calls++; return nil, errors.New("install reached") }
	for _, host := range []string{"windows", "linux"} {
		runtimeGOOS = host
		result, err := InstallSimulator(context.Background(), SimulatorInstallOptions{AppPath: "Fixture.app", UDID: "11111111-1111-1111-1111-111111111111", Timeout: time.Minute})
		if result != nil || err == nil || !strings.Contains(err.Error(), "macOS only") || calls != 0 {
			t.Fatalf("host=%s result=%+v error=%v tool calls=%d", host, result, err, calls)
		}
	}
}

type cancelSimulatorExecutableRead struct {
	*os.File
	cancel    context.CancelFunc
	readBytes int
}

func (file *cancelSimulatorExecutableRead) Read(data []byte) (int, error) {
	n, err := file.File.Read(data)
	file.readBytes += n
	file.cancel()
	return n, err
}

func TestSimulatorExecutableDigestHonorsCancellationDuringRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Executable")
	if err := os.WriteFile(path, make([]byte, 128<<10), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := &cancelSimulatorExecutableRead{File: file, cancel: cancel}
	_, err = simulatorExecutableDigest(ctx, reader, info)
	if !errors.Is(err, context.Canceled) || reader.readBytes <= 0 || int64(reader.readBytes) >= info.Size() {
		t.Fatalf("digest cancellation err=%v read=%d size=%d", err, reader.readBytes, info.Size())
	}
}
