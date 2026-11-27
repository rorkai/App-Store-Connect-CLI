package xcode

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// SimulatorInstallOptions targets one already booted iOS simulator.
type SimulatorInstallOptions struct {
	AppPath string
	UDID    string
	Timeout time.Duration
}

var (
	simulatorUUID        = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	simulatorInstallList = SimctlListJSON
	simulatorInstallRun  = runSimulatorInstallCommand
)

// ValidateSimulatorInstallOptions validates flags without files or device tools.
func ValidateSimulatorInstallOptions(options SimulatorInstallOptions) error {
	if strings.TrimSpace(options.AppPath) == "" {
		return fmt.Errorf("--app is required")
	}
	if strings.TrimSpace(options.UDID) == "" {
		return fmt.Errorf("--udid is required")
	}
	if !simulatorUUID.MatchString(options.UDID) {
		return fmt.Errorf("--udid must be an exact simulator UUID")
	}
	if options.Timeout < installMinimumTimeout || options.Timeout > installMaximumTimeout {
		return fmt.Errorf("--timeout must be between 5s and 10m")
	}
	return nil
}

// InstallSimulator installs an app and independently reads its installed bundle metadata.
// It never creates, boots or launches a simulator.
func InstallSimulator(ctx context.Context, options SimulatorInstallOptions) (*asc.XcodeSimulatorInstallResult, error) {
	if err := ValidateSimulatorInstallOptions(options); err != nil {
		return nil, &InstallInputError{message: err.Error()}
	}
	if runtimeGOOS != "darwin" {
		return nil, fmt.Errorf("simulator installation is supported on macOS only")
	}
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	source, err := readSimulatorApp(ctx, options.AppPath)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		return nil, &InstallInputError{message: "--app must be a readable iOS Simulator .app with bundle identifier, version, build and executable metadata"}
	}
	defer source.close()
	inventory, err := simulatorInstallList(ctx)
	if contextErr := ctx.Err(); contextErr != nil {
		return nil, contextErr
	}
	if err != nil {
		return nil, fmt.Errorf("simulator discovery failed")
	}
	var list struct {
		Devices map[string][]struct {
			UDID      string `json:"udid"`
			State     string `json:"state"`
			Available bool   `json:"isAvailable"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(inventory, &list); err != nil {
		return nil, fmt.Errorf("simulator discovery returned invalid JSON")
	}
	matches, usable := 0, false
	for runtime, devices := range list.Devices {
		for _, device := range devices {
			if strings.EqualFold(device.UDID, options.UDID) {
				matches++
				usable = strings.HasPrefix(runtime, "com.apple.CoreSimulator.SimRuntime.iOS-") && device.State == "Booted" && device.Available
			}
		}
	}
	if matches != 1 || !usable {
		return nil, fmt.Errorf("--udid must identify one available, booted iOS simulator")
	}
	result := &asc.XcodeSimulatorInstallResult{BundleID: source.BundleID, Version: source.Version, BuildNumber: source.BuildNumber, SimulatorIdentifierSHA256: hashInstallIdentifier(strings.ToUpper(options.UDID))}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := source.check(ctx); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return result, contextErr
		}
		return result, fmt.Errorf("source simulator app changed before installation")
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return result, contextErr
	}
	if _, err := simulatorInstallRun(ctx, "simctl", "install", options.UDID, options.AppPath); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return result, contextErr
		}
		return result, fmt.Errorf("simulator installation failed")
	}
	result.Installed = true
	if contextErr := ctx.Err(); contextErr != nil {
		return result, contextErr
	}
	output, err := simulatorInstallRun(ctx, "simctl", "get_app_container", options.UDID, source.BundleID, "app")
	if contextErr := ctx.Err(); contextErr != nil {
		return result, contextErr
	}
	if err != nil {
		return result, fmt.Errorf("installed simulator app could not be inspected")
	}
	installed, err := readSimulatorApp(ctx, strings.TrimSpace(string(output)))
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return result, contextErr
		}
		return result, fmt.Errorf("installed simulator app metadata could not be read")
	}
	defer installed.close()
	if installed.BundleID != source.BundleID || installed.Version != source.Version || installed.BuildNumber != source.BuildNumber {
		return result, fmt.Errorf("installed simulator app identity did not match the source bundle, version and build")
	}
	if err := source.check(ctx); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return result, contextErr
		}
		return result, fmt.Errorf("source simulator app changed during installation or verification")
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return result, contextErr
	}
	result.BundleID, result.Version, result.BuildNumber = installed.BundleID, installed.Version, installed.BuildNumber
	result.Verified = true
	return result, nil
}

// simulatorAppSnapshot keeps descriptors alive so a rebuild cannot recycle
// the inspected metadata/executable identity while device discovery runs.
type simulatorAppSnapshot struct {
	asc.XcodeSimulatorInstallResult
	root             rootfs.Root
	plistIdentity    *rootfs.FileIdentity
	executable       *os.File
	executableName   string
	executableInfo   os.FileInfo
	executableSHA256 [sha256.Size]byte
}

func (app *simulatorAppSnapshot) close() {
	if app.executable != nil {
		_ = app.executable.Close()
	}
	_ = app.root.Close()
}

func (app *simulatorAppSnapshot) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := app.root.CheckFileIdentity("Info.plist", app.plistIdentity); err != nil {
		return err
	}
	file, err := app.root.OpenFile(app.executableName)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	retainedInfo, err := app.executable.Stat()
	if err != nil {
		return err
	}
	if !sameSimulatorFile(app.executableInfo, info) || !sameSimulatorFile(app.executableInfo, retainedInfo) {
		return errors.New("bundle executable changed")
	}
	digest, err := simulatorExecutableDigest(ctx, file, info)
	if err != nil {
		return err
	}
	if digest != app.executableSHA256 {
		return errors.New("bundle executable content changed")
	}
	return nil
}

func sameSimulatorFile(expected, actual os.FileInfo) bool {
	return os.SameFile(expected, actual) && expected.Mode() == actual.Mode() && expected.Size() == actual.Size() && expected.ModTime().Equal(actual.ModTime())
}

type simulatorExecutableReader interface {
	io.Reader
	Stat() (os.FileInfo, error)
}

func simulatorExecutableDigest(ctx context.Context, file simulatorExecutableReader, info os.FileInfo) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	hash := sha256.New()
	written, err := copyValidationSnapshotWithContext(ctx, hash, io.LimitReader(file, info.Size()))
	if err != nil {
		return digest, err
	}
	if written != info.Size() {
		return digest, io.ErrUnexpectedEOF
	}
	after, err := file.Stat()
	if err != nil {
		return digest, err
	}
	if !sameSimulatorFile(info, after) {
		return digest, errors.New("bundle executable changed while reading")
	}
	if err := ctx.Err(); err != nil {
		return digest, err
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func readSimulatorApp(ctx context.Context, path string) (*simulatorAppSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(path), ".app") {
		return nil, errors.New("expected .app")
	}
	root, err := rootfs.New(path)
	if err != nil {
		return nil, err
	}
	app := &simulatorAppSnapshot{root: root}
	keep := false
	defer func() {
		if !keep {
			app.close()
		}
	}()
	app.plistIdentity, err = root.CaptureFileLimited("Info.plist", 1<<20)
	if err != nil {
		return nil, err
	}
	var metadata map[string]any
	if _, err := plist.Unmarshal(app.plistIdentity.Data(), &metadata); err != nil {
		return nil, err
	}
	simulator := coercePlistValueToString(metadata["DTPlatformName"]) == "iphonesimulator"
	if platforms, ok := metadata["CFBundleSupportedPlatforms"].([]any); ok {
		for _, platform := range platforms {
			if platform == "iPhoneSimulator" {
				simulator = true
			}
		}
	}
	app.executableName = coercePlistValueToString(metadata["CFBundleExecutable"])
	app.BundleID = coercePlistValueToString(metadata["CFBundleIdentifier"])
	app.Version = coercePlistValueToString(metadata["CFBundleShortVersionString"])
	app.BuildNumber = coercePlistValueToString(metadata["CFBundleVersion"])
	if !simulator || strings.TrimSpace(app.Version) == "" || strings.TrimSpace(app.BuildNumber) == "" || app.BundleID == "" || strings.HasPrefix(app.BundleID, "-") || strings.ContainsAny(app.BundleID, " \t\r\n/") || app.executableName == "" || app.executableName == "." || strings.ContainsAny(app.executableName, "/\\") {
		return nil, errors.New("invalid simulator bundle metadata")
	}
	app.executable, err = root.OpenFile(app.executableName)
	if err != nil {
		return nil, err
	}
	app.executableInfo, err = app.executable.Stat()
	if err != nil {
		return nil, err
	}
	if !app.executableInfo.Mode().IsRegular() || app.executableInfo.Mode()&0o111 == 0 {
		return nil, errors.New("bundle executable is not executable")
	}
	app.executableSHA256, err = simulatorExecutableDigest(ctx, app.executable, app.executableInfo)
	if err != nil {
		return nil, err
	}
	if err := root.CheckFileIdentity("Info.plist", app.plistIdentity); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	keep = true
	return app, nil
}

func runSimulatorInstallCommand(ctx context.Context, args ...string) ([]byte, error) {
	cmd, err := trustedXcodeCommand(ctx, "xcrun", args, nil)
	if err != nil {
		return nil, err
	}
	stop := func() {
		if cmd.Cancel != nil {
			_ = cmd.Cancel()
		} else if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	stdout, stderr := newSimctlListOutput(64<<10, stop), newSimctlListOutput(8<<10, stop)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err = runXcodeCommand(cmd)
	if stdout.exceeded || stderr.exceeded {
		return nil, errors.New("simctl output exceeded limit")
	}
	if err != nil {
		return nil, errors.New("simctl operation failed")
	}
	return stdout.data, nil
}
