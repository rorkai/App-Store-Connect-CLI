package xcode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
)

func TestValidateInstallOptions(t *testing.T) {
	valid := InstallOptions{IPAPath: "App.ipa", DeviceID: "SELECTOR_CANARY", Timeout: 5 * time.Minute}
	if err := ValidateInstallOptions(valid); err != nil {
		t.Fatalf("ValidateInstallOptions(valid) error = %v", err)
	}
	for _, test := range []struct {
		name string
		edit func(*InstallOptions)
		want string
	}{
		{name: "missing IPA", edit: func(options *InstallOptions) { options.IPAPath = "" }, want: "--ipa is required"},
		{name: "wrong extension", edit: func(options *InstallOptions) { options.IPAPath = "App.zip" }, want: "must end with .ipa"},
		{name: "missing device", edit: func(options *InstallOptions) { options.DeviceID = "" }, want: "--device-id is required"},
		{name: "device whitespace", edit: func(options *InstallOptions) { options.DeviceID = " SELECTOR_CANARY" }, want: "leading or trailing whitespace"},
		{name: "short timeout", edit: func(options *InstallOptions) { options.Timeout = time.Second }, want: "between"},
		{name: "long timeout", edit: func(options *InstallOptions) { options.Timeout = 11 * time.Minute }, want: "between"},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := valid
			test.edit(&options)
			if err := ValidateInstallOptions(options); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateInstallOptions() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestInstallRejectsUnreadableIPAAsUsageInputWithoutResult(t *testing.T) {
	previousGOOS := runtimeGOOS
	t.Cleanup(func() { runtimeGOOS = previousGOOS })
	runtimeGOOS = "darwin"

	result, err := Install(context.Background(), InstallOptions{
		IPAPath:  filepath.Join(t.TempDir(), "missing.ipa"),
		DeviceID: "SELECTOR_CANARY",
		Timeout:  5 * time.Minute,
	})
	if result != nil {
		t.Fatalf("Install() result = %#v, want nil for usage input", result)
	}
	var inputErr *InstallInputError
	if !errors.As(err, &inputErr) {
		t.Fatalf("Install() error = %v, want InstallInputError", err)
	}
	if strings.Contains(err.Error(), "missing.ipa") {
		t.Fatalf("Install() error leaked source path: %v", err)
	}
}

func TestValidateInstallInspectionAllowsDevelopmentAndAdHocProfiles(t *testing.T) {
	for _, profileClass := range []distribution.ProfileClass{distribution.ProfileClassDevelopment, distribution.ProfileClassAdHoc} {
		t.Run(string(profileClass), func(t *testing.T) {
			inspection := installTestInspection(profileClass)
			if err := validateInstallInspection(inspection); err != nil {
				t.Fatalf("validateInstallInspection() error = %v", err)
			}
		})
	}
	for _, profileClass := range []distribution.ProfileClass{distribution.ProfileClassAppStore, distribution.ProfileClassEnterprise, distribution.ProfileClassUnknown} {
		t.Run(string(profileClass), func(t *testing.T) {
			inspection := installTestInspection(profileClass)
			if err := validateInstallInspection(inspection); err == nil || !strings.Contains(err.Error(), "development or ad-hoc") {
				t.Fatalf("validateInstallInspection() error = %v, want profile-class rejection", err)
			}
		})
	}
}

func TestValidateInstallInspectionAllowsGenericPreparationWarnings(t *testing.T) {
	inspection := installTestInspection(distribution.ProfileClassAdHoc)
	inspection.Preparation.Issues = []string{
		"embedded targets require target-by-target signing validation before preparation",
		"app title is missing",
	}
	if err := validateInstallInspection(inspection); err != nil {
		t.Fatalf("validateInstallInspection() error = %v, want installable main app despite preparation warnings", err)
	}
}

func TestValidateInstallInspectionRequiresInstallIdentity(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*distribution.Inspection)
		want   string
	}{
		{name: "bundle identifier", mutate: func(inspection *distribution.Inspection) { inspection.App.BundleID = "" }, want: "bundle identifier"},
		{name: "version", mutate: func(inspection *distribution.Inspection) { inspection.App.Version = "" }, want: "version"},
		{name: "build", mutate: func(inspection *distribution.Inspection) { inspection.App.BuildNumber = "" }, want: "build"},
		{name: "profile UUID", mutate: func(inspection *distribution.Inspection) { inspection.Signing.ProfileUUID = "" }, want: "profile UUID"},
		{name: "profile expiration", mutate: func(inspection *distribution.Inspection) { inspection.Signing.ExpiresAt = "" }, want: "expiration"},
		{name: "profile certificates", mutate: func(inspection *distribution.Inspection) {
			inspection.Signing.ProfileCertificateSHA256Fingerprints = nil
		}, want: "certificates"},
	} {
		t.Run(test.name, func(t *testing.T) {
			inspection := installTestInspection(distribution.ProfileClassAdHoc)
			test.mutate(&inspection)
			if err := validateInstallInspection(inspection); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateInstallInspection() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateInstallInspectionRejectsInstallBlockingPreparationIssue(t *testing.T) {
	for _, issue := range []string{
		"provisioning profile bundle identifier does not match the app",
		"provisioning profile does not include the iOS platform",
	} {
		t.Run(issue, func(t *testing.T) {
			inspection := installTestInspection(distribution.ProfileClassAdHoc)
			inspection.Preparation.Issues = []string{issue}
			if err := validateInstallInspection(inspection); err == nil {
				t.Fatal("validateInstallInspection() accepted install-blocking preparation issue")
			}
		})
	}
}

func TestInstallUsesAppPathAndVerifiesInstalledBuild(t *testing.T) {
	previousGOOS := runtimeGOOS
	previousRunner := installRunner
	previousNow := installNow
	t.Cleanup(func() {
		runtimeGOOS = previousGOOS
		installRunner = previousRunner
		installNow = previousNow
	})
	runtimeGOOS = "darwin"
	installNow = time.Now

	ipaPath := filepath.Join(t.TempDir(), "Demo.ipa")
	if err := os.WriteFile(ipaPath, []byte("not-read-by-fake-materializer"), 0o600); err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(t.TempDir(), "Demo.app")
	if err := os.MkdirAll(appPath, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &fakeInstallRunner{t: t}
	installRunner = runner
	source := &fakeInstallIPASource{inspection: installTestInspection(distribution.ProfileClassAdHoc), appPath: appPath, runner: runner}
	stubInstallIPASource(t, source)

	result, err := Install(context.Background(), InstallOptions{
		IPAPath:  ipaPath,
		DeviceID: "SELECTOR_CANARY",
		Timeout:  5 * time.Minute,
		Environment: []string{
			"DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer",
			"ASC_API_KEY=must-not-leak",
		},
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if result == nil || !result.Success || !result.Installed || !result.Verified {
		t.Fatalf("Install() result = %#v, want successful verified result", result)
	}
	if result.Device == nil || result.Device.IdentifierSHA256 == "SELECTOR_CANARY" || result.Device.IdentifierSHA256 == "" {
		t.Fatalf("result leaked or omitted device digest: %#v", result)
	}
	if result.IPA.SHA256 == "" || result.IPA.BundleID != "com.example.demo" || result.IPA.BuildNumber != "45" {
		t.Fatalf("result omitted artifact identity: %#v", result)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("devicectl calls = %d, want discovery/install/verification", len(runner.calls))
	}
	if source.materializeCalls != 1 || source.runnerCallsAtMaterialize != 1 {
		t.Fatalf("materialize calls = %d after %d devicectl calls, want exactly one extraction after device discovery", source.materializeCalls, source.runnerCallsAtMaterialize)
	}
	if source.cleanupCalls == 0 {
		t.Fatal("IPA app source was not cleaned up")
	}
	installArgs := runner.calls[1]
	if containsInstallArg(installArgs, ipaPath) || !containsInstallArg(installArgs, appPath) {
		t.Fatalf("install args = %#v, want materialized app path only", installArgs)
	}
	if !containsInstallSequence(installArgs, []string{"device", "install", "app", "--device", "SELECTOR_CANARY", appPath}) {
		t.Fatalf("install args = %#v, want exact device install sequence", installArgs)
	}
	if containsInstallArg(installArgs, "--ipa") || containsInstallArg(installArgs, "--all-devices") {
		t.Fatalf("install args = %#v, want no source-IPA or batch selector", installArgs)
	}
	verificationArgs := runner.calls[2]
	if !containsInstallSequence(verificationArgs, []string{"device", "info", "apps", "--device", "SELECTOR_CANARY", "--bundle-id", "com.example.demo"}) {
		t.Fatalf("verification args = %#v, want exact bundle observation", verificationArgs)
	}
	for _, environment := range runner.environments {
		if containsInstallArg(environment, "ASC_API_KEY=must-not-leak") {
			t.Fatalf("secret environment leaked to devicectl: %#v", runner.environments)
		}
	}
}

func TestInstallProceedsToDevicectlForIPAWithEmbeddedTargets(t *testing.T) {
	previousGOOS := runtimeGOOS
	previousRunner := installRunner
	t.Cleanup(func() {
		runtimeGOOS = previousGOOS
		installRunner = previousRunner
	})
	runtimeGOOS = "darwin"
	ipaPath := filepath.Join(t.TempDir(), "Demo.ipa")
	if err := os.WriteFile(ipaPath, []byte("not-read-by-fake-materializer"), 0o600); err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(t.TempDir(), "Demo.app")
	if err := os.MkdirAll(appPath, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &fakeInstallRunner{t: t}
	installRunner = runner
	inspection := installTestInspection(distribution.ProfileClassAdHoc)
	inspection.EmbeddedTargets = []string{"Payload/Demo.app/PlugIns/Widget.appex/Info.plist"}
	inspection.Preparation.MetadataEligible = false
	inspection.Preparation.Issues = []string{"embedded targets require target-by-target signing validation before preparation"}
	stubInstallIPASource(t, &fakeInstallIPASource{inspection: inspection, appPath: appPath, runner: runner})

	result, err := Install(context.Background(), InstallOptions{IPAPath: ipaPath, DeviceID: "SELECTOR_CANARY", Timeout: 5 * time.Minute})
	if err != nil {
		t.Fatalf("Install() error = %v, want embedded-target IPA to remain installable", err)
	}
	if result == nil || !result.Success || !result.Installed || !result.Verified {
		t.Fatalf("Install() result = %#v, want successful verified result", result)
	}
	if result.FailureStage != "" || result.FailureCode != "" {
		t.Fatalf("Install() result reported failure %q/%q, want none", result.FailureStage, result.FailureCode)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("devicectl calls = %d, want discovery/install/verification despite embedded-target preparation warning", len(runner.calls))
	}
	if !containsInstallSequence(runner.calls[0], []string{"list", "devices"}) {
		t.Fatalf("discovery args = %#v, want device listing first", runner.calls[0])
	}
	if !containsInstallSequence(runner.calls[1], []string{"device", "install", "app", "--device", "SELECTOR_CANARY", appPath}) {
		t.Fatalf("install args = %#v, want exact install sequence with the materialized app path second", runner.calls[1])
	}
	if containsInstallArg(runner.calls[1], ipaPath) {
		t.Fatalf("install args = %#v, want no source IPA path", runner.calls[1])
	}
	if !containsInstallSequence(runner.calls[2], []string{"device", "info", "apps", "--device", "SELECTOR_CANARY", "--bundle-id", "com.example.demo"}) {
		t.Fatalf("verification args = %#v, want exact bundle observation last", runner.calls[2])
	}
}

func TestInstallDoesNotMaterializeIneligibleProfile(t *testing.T) {
	previousGOOS := runtimeGOOS
	previousRunner := installRunner
	t.Cleanup(func() {
		runtimeGOOS = previousGOOS
		installRunner = previousRunner
	})
	runtimeGOOS = "darwin"
	ipaPath := filepath.Join(t.TempDir(), "Demo.ipa")
	if err := os.WriteFile(ipaPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeInstallRunner{t: t}
	installRunner = runner
	inspection := installTestInspection(distribution.ProfileClassAdHoc)
	inspection.Signing.ExpiresAt = installNow().Add(-time.Hour).UTC().Format(time.RFC3339)
	source := &fakeInstallIPASource{inspection: inspection, runner: runner}
	stubInstallIPASource(t, source)

	result, err := Install(context.Background(), InstallOptions{IPAPath: ipaPath, DeviceID: "SELECTOR_CANARY", Timeout: 5 * time.Minute})
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("Install() error = %v, want expired-profile rejection", err)
	}
	if result == nil || result.Success || result.FailureStage != "profile-preflight" || result.FailureCode != "profile_not_installable" {
		t.Fatalf("Install() result = %#v, want profile-preflight failure", result)
	}
	if source.materializeCalls != 0 {
		t.Fatalf("materialize calls = %d, want no extraction for an ineligible profile", source.materializeCalls)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("devicectl calls = %d, want none before profile eligibility passes", len(runner.calls))
	}
	if source.cleanupCalls == 0 {
		t.Fatal("IPA app source was not cleaned up")
	}
}

func TestInstallDoesNotMaterializeWhenRequestedDeviceMissing(t *testing.T) {
	previousGOOS := runtimeGOOS
	previousRunner := installRunner
	t.Cleanup(func() {
		runtimeGOOS = previousGOOS
		installRunner = previousRunner
	})
	runtimeGOOS = "darwin"
	ipaPath := filepath.Join(t.TempDir(), "Demo.ipa")
	if err := os.WriteFile(ipaPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeInstallRunner{t: t}
	installRunner = runner
	source := &fakeInstallIPASource{inspection: installTestInspection(distribution.ProfileClassAdHoc), runner: runner}
	stubInstallIPASource(t, source)

	result, err := Install(context.Background(), InstallOptions{IPAPath: ipaPath, DeviceID: "MISSING_CANARY", Timeout: 5 * time.Minute})
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("Install() error = %v, want missing-device rejection", err)
	}
	if result == nil || result.Success || result.FailureStage != "device-discovery" || result.FailureCode != "device_not_found" {
		t.Fatalf("Install() result = %#v, want device-discovery failure", result)
	}
	if source.materializeCalls != 0 {
		t.Fatalf("materialize calls = %d, want no extraction for a missing device", source.materializeCalls)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("devicectl calls = %d, want device discovery only", len(runner.calls))
	}
}

func TestInstallReportsMaterializationFailureAfterDeviceDiscovery(t *testing.T) {
	previousGOOS := runtimeGOOS
	previousRunner := installRunner
	t.Cleanup(func() {
		runtimeGOOS = previousGOOS
		installRunner = previousRunner
	})
	runtimeGOOS = "darwin"
	ipaPath := filepath.Join(t.TempDir(), "Demo.ipa")
	if err := os.WriteFile(ipaPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeInstallRunner{t: t}
	installRunner = runner
	source := &fakeInstallIPASource{
		inspection:     installTestInspection(distribution.ProfileClassAdHoc),
		materializeErr: errors.New("extraction denied"),
		runner:         runner,
	}
	stubInstallIPASource(t, source)

	result, err := Install(context.Background(), InstallOptions{IPAPath: ipaPath, DeviceID: "SELECTOR_CANARY", Timeout: 5 * time.Minute})
	if err == nil || !strings.Contains(err.Error(), "materialize IPA app") {
		t.Fatalf("Install() error = %v, want materialization failure", err)
	}
	if result == nil || result.Success || result.FailureStage != "materialization" || result.FailureCode != "materialization_failed" {
		t.Fatalf("Install() result = %#v, want materialization failure result", result)
	}
	if result.Device == nil {
		t.Fatalf("Install() result = %#v, want discovered device identity in materialization failure", result)
	}
	if source.materializeCalls != 1 || source.runnerCallsAtMaterialize != 1 {
		t.Fatalf("materialize calls = %d after %d devicectl calls, want one extraction attempt after device discovery", source.materializeCalls, source.runnerCallsAtMaterialize)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("devicectl calls = %d, want no install attempt without a materialized app", len(runner.calls))
	}
}

func TestInstallReturnsUnverifiedResultWhenPostInstallDoesNotMatch(t *testing.T) {
	previousGOOS := runtimeGOOS
	previousRunner := installRunner
	t.Cleanup(func() {
		runtimeGOOS = previousGOOS
		installRunner = previousRunner
	})
	runtimeGOOS = "darwin"
	ipaPath := filepath.Join(t.TempDir(), "Demo.ipa")
	if err := os.WriteFile(ipaPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(t.TempDir(), "Demo.app")
	if err := os.MkdirAll(appPath, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &fakeInstallRunner{t: t, verificationBuild: "different"}
	installRunner = runner
	stubInstallIPASource(t, &fakeInstallIPASource{inspection: installTestInspection(distribution.ProfileClassAdHoc), appPath: appPath, runner: runner})

	result, err := Install(context.Background(), InstallOptions{IPAPath: ipaPath, DeviceID: "SELECTOR_CANARY", Timeout: 5 * time.Minute})
	if err == nil || !strings.Contains(err.Error(), "version or build") {
		t.Fatalf("Install() error = %v, want post-install mismatch", err)
	}
	if result == nil || !result.Installed || result.Verified || result.Success {
		t.Fatalf("Install() result = %#v, want installed but unverified", result)
	}
	if result.FailureStage != "verification" || result.FailureCode != "verification_failed" {
		t.Fatalf("Install() failure = %#v, want verification failure classification", result)
	}
}

func TestInstallReturnsRedactedDeviceDiscoveryFailureResult(t *testing.T) {
	previousGOOS := runtimeGOOS
	previousRunner := installRunner
	t.Cleanup(func() {
		runtimeGOOS = previousGOOS
		installRunner = previousRunner
	})
	runtimeGOOS = "darwin"
	ipaPath := filepath.Join(t.TempDir(), "Demo.ipa")
	if err := os.WriteFile(ipaPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(t.TempDir(), "Demo.app")
	if err := os.MkdirAll(appPath, 0o700); err != nil {
		t.Fatal(err)
	}
	installRunner = &failingInstallRunner{}
	source := &fakeInstallIPASource{inspection: installTestInspection(distribution.ProfileClassAdHoc), appPath: appPath}
	stubInstallIPASource(t, source)

	result, err := Install(context.Background(), InstallOptions{IPAPath: ipaPath, DeviceID: "SELECTOR_CANARY", Timeout: 5 * time.Minute})
	if err == nil {
		t.Fatal("Install() error = nil, want discovery failure")
	}
	if result == nil || result.Success || result.FailureStage != "device-discovery" || result.FailureCode != "devicectl_unavailable" {
		t.Fatalf("Install() result = %#v, want redacted device-discovery failure", result)
	}
	data, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	output := string(data)
	for _, secret := range []string{"SELECTOR_CANARY", "UDID_CANARY", ipaPath, appPath} {
		if strings.Contains(output, secret) {
			t.Fatalf("result JSON leaked %q: %s", secret, output)
		}
	}
	if source.materializeCalls != 0 {
		t.Fatalf("materialize calls = %d, want no extraction when devicectl is unavailable", source.materializeCalls)
	}
}

func TestReadInstallJSONRejectsHardLink(t *testing.T) {
	directory := t.TempDir()
	pathValue := filepath.Join(directory, "devices.json")
	if err := os.WriteFile(pathValue, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(directory, "alias.json")
	if err := os.Link(pathValue, linkPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	if _, err := readInstallJSON(directory, "devices.json"); err == nil || !strings.Contains(err.Error(), "single-link") {
		t.Fatalf("readInstallJSON() error = %v, want hard-link rejection", err)
	}
}

func TestReadInstallJSONRejectsDestinationReplacement(t *testing.T) {
	directory := t.TempDir()
	pathValue := filepath.Join(directory, "devices.json")
	replacement := filepath.Join(directory, "replacement.json")
	if err := os.WriteFile(pathValue, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacement, []byte(`{"replacement":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := installAfterJSONLstatForTest
	t.Cleanup(func() { installAfterJSONLstatForTest = previous })
	installAfterJSONLstatForTest = func() {
		_ = os.Remove(pathValue)
		_ = os.Rename(replacement, pathValue)
	}
	if _, err := readInstallJSON(directory, "devices.json"); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("readInstallJSON() error = %v, want replacement rejection", err)
	}
}

func TestParseDevicectlEnvelopeRejectsDuplicateKeys(t *testing.T) {
	payload := []byte(`{"info":{"commandType":"devicectl.list.devices","commandType":"devicectl.list.devices","jsonVersion":5,"outcome":"success"},"result":{"devices":[]}}`)
	if _, err := parseDevicectlEnvelope(payload, "devicectl.list.devices"); err == nil {
		t.Fatal("parseDevicectlEnvelope() accepted duplicate JSON key")
	}
}

func TestParseDevicectlEnvelopeAcceptsLegacyJSONVersion(t *testing.T) {
	payload := []byte(`{"info":{"arguments":[],"commandType":"devicectl.list.devices","environment":{},"jsonVersion":4,"outcome":"success","version":"legacy"},"result":{"devices":[]}}`)
	if _, err := parseDevicectlEnvelope(payload, "devicectl.list.devices"); err != nil {
		t.Fatalf("parseDevicectlEnvelope() error = %v, want legacy schema accepted", err)
	}
}

func TestParseDevicectlEnvelopeAcceptsV5DeprecationNotice(t *testing.T) {
	payload := []byte(`{"info":{"arguments":[],"commandType":"devicectl.list.devices","environment":{},"jsonVersion":5,"outcome":"success","version":"current"},"result":{"_deprecationNotice":{"deprecatedFields":["hardwareProperties"],"message":"diagnostic-only","replacement":"properties"},"devices":[]}}`)
	envelope, err := parseDevicectlEnvelope(payload, "devicectl.list.devices")
	if err != nil {
		t.Fatalf("parseDevicectlEnvelope() error = %v, want v5 deprecation notice accepted", err)
	}
	var result installDeviceListResult
	if err := decodeStrictDevicectlResult(envelope.Result, envelope.Info.JSONVersion, &result); err != nil {
		t.Fatalf("decodeStrictDevicectlResult() error = %v, want v5 deprecation notice accepted", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "_deprecationNotice") || strings.Contains(string(encoded), "diagnostic-only") {
		t.Fatalf("decoded result exposed the deprecation notice: %s", encoded)
	}
}

func TestDecodeStrictDevicectlResultRejectsLegacyDeprecationNotice(t *testing.T) {
	payload := json.RawMessage(`{"_deprecationNotice":{"message":"diagnostic-only"},"devices":[]}`)
	var result installDeviceListResult
	if err := decodeStrictDevicectlResult(payload, installLegacyJSONVersion, &result); err == nil {
		t.Fatal("decodeStrictDevicectlResult() accepted a legacy deprecation notice")
	}
}

func TestParseDevicectlEnvelopeRejectsFutureJSONVersion(t *testing.T) {
	payload := []byte(`{"info":{"arguments":[],"commandType":"devicectl.list.devices","environment":{},"jsonVersion":7,"outcome":"success","version":"future"},"result":{"devices":[]}}`)
	if _, err := parseDevicectlEnvelope(payload, "devicectl.list.devices"); err == nil {
		t.Fatal("parseDevicectlEnvelope() accepted unsupported future schema")
	}
}

func TestNormalizeInstallDeviceFailsClosedWithoutHardwareUDID(t *testing.T) {
	entry := installDeviceEntry{
		Identifier: "SELECTOR_CANARY",
		Properties: json.RawMessage(`{"connection":{"pairingState":"paired","state":"connected"},"hardware":{"platform":"iOS","reality":"physical"}}`),
	}
	_, err := normalizeInstallDevice(entry)
	var membershipErr *installDeviceMembershipUnavailableError
	if !errors.As(err, &membershipErr) {
		t.Fatalf("normalizeInstallDevice() error = %v, want membership-unavailable error", err)
	}
	if strings.Contains(err.Error(), "SELECTOR_CANARY") {
		t.Fatalf("normalizeInstallDevice() error leaked selector: %v", err)
	}
}

func installTestInspection(profileClass distribution.ProfileClass) distribution.Inspection {
	issues := []string{}
	if profileClass == distribution.ProfileClassDevelopment {
		issues = []string{"provisioning profile class is development; expected ad-hoc"}
	}
	expiresAt := installNow().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	return distribution.Inspection{
		SchemaVersion:      "1",
		Platform:           "IOS",
		DistributionMethod: "release-testing",
		App: distribution.App{
			BundleID:    "com.example.demo",
			Title:       "Demo",
			Version:     "1.2.3",
			BuildNumber: "45",
		},
		Artifact: distribution.Artifact{SizeBytes: 4, SHA256: strings.Repeat("a", 64)},
		Signing: distribution.Signing{
			ProfileClass:                         profileClass,
			ProfileUUID:                          "PROFILE_CANARY",
			ExpiresAt:                            expiresAt,
			DeviceCount:                          1,
			Devices:                              []string{"UDID_CANARY"},
			ProfileCertificateSHA256Fingerprints: []string{strings.Repeat("c", 64)},
			ProfileIntegrityVerification:         distribution.CodeSignatureVerification{Status: distribution.CodeSignatureVerified},
			ProfileTrustVerification:             distribution.CodeSignatureVerification{Status: distribution.CodeSignatureVerified},
			CodeSignatureVerification:            distribution.CodeSignatureVerification{Status: distribution.CodeSignatureVerified},
		},
		Preparation: distribution.Preparation{MetadataEligible: profileClass == distribution.ProfileClassAdHoc, Issues: issues},
	}
}

type fakeInstallIPASource struct {
	inspection               distribution.Inspection
	appPath                  string
	materializeErr           error
	materializeCalls         int
	cleanupCalls             int
	runner                   *fakeInstallRunner
	runnerCallsAtMaterialize int
}

func (s *fakeInstallIPASource) Inspection() distribution.Inspection { return s.inspection }

func (s *fakeInstallIPASource) MaterializeApp(context.Context) (*distribution.MaterializedApp, error) {
	s.materializeCalls++
	if s.runner != nil {
		s.runnerCallsAtMaterialize = len(s.runner.calls)
	}
	if s.materializeErr != nil {
		return nil, s.materializeErr
	}
	return &distribution.MaterializedApp{Inspection: s.inspection, Path: s.appPath}, nil
}

func (s *fakeInstallIPASource) Cleanup() { s.cleanupCalls++ }

func stubInstallIPASource(t *testing.T, source *fakeInstallIPASource) {
	t.Helper()
	previous := openInstallIPASource
	t.Cleanup(func() { openInstallIPASource = previous })
	openInstallIPASource = func(context.Context, *os.File, int64, distribution.InspectOptions) (installIPASource, error) {
		return source, nil
	}
}

type fakeInstallRunner struct {
	deviceListPayload string
	t                 *testing.T
	calls             [][]string
	environments      [][]string
	verificationBuild string
}

type failingInstallRunner struct{}

func (*failingInstallRunner) ResolveDevicectl(context.Context, []string) (string, error) {
	return "", errors.New("tool lookup failed")
}

func (*failingInstallRunner) Run(context.Context, string, []string, []string) error {
	return errors.New("run should not be reached")
}

func (runner *fakeInstallRunner) ResolveDevicectl(context.Context, []string) (string, error) {
	return "/private/tmp/fake-devicectl", nil
}

func (runner *fakeInstallRunner) Run(_ context.Context, _ string, args, environment []string) error {
	runner.calls = append(runner.calls, append([]string(nil), args...))
	runner.environments = append(runner.environments, append([]string(nil), environment...))
	index := -1
	for i, arg := range args {
		if arg == "--json-output" && i+1 < len(args) {
			index = i + 1
			break
		}
	}
	if index < 0 {
		runner.t.Fatal("fake devicectl call did not provide --json-output")
	}
	pathValue := args[index]
	var payload string
	switch {
	case containsInstallArg(args, "list") && containsInstallArg(args, "devices"):
		payload = fakeDeviceListPayload()
		if runner.deviceListPayload != "" {
			payload = runner.deviceListPayload
		}
	case containsInstallArg(args, "install"):
		payload = fakeInstallPayload()
	case containsInstallArg(args, "info"):
		build := "45"
		if runner.verificationBuild != "" {
			build = runner.verificationBuild
		}
		payload = fakeAppsPayload(build)
	default:
		runner.t.Fatalf("unexpected fake devicectl args: %#v", args)
	}
	return os.WriteFile(pathValue, []byte(payload), 0o600)
}

func fakeDeviceListPayload() string {
	return `{"info":{"arguments":[],"commandType":"devicectl.list.devices","environment":{},"jsonVersion":5,"outcome":"success","version":"642.15"},"result":{"devices":[{"identifier":"SELECTOR_CANARY","properties":{"connection":{"pairingState":"paired","state":"connected"},"hardware":{"platform":"iOS","reality":"physical","udid":"UDID_CANARY"},"state":{"visibilityClass":"default"}}}]}}`
}

func fakeInstallPayload() string {
	return `{"info":{"arguments":[],"commandType":"devicectl.device.install.app","environment":{},"jsonVersion":5,"outcome":"success","version":"642.15"},"result":{"deviceIdentifier":"SELECTOR_CANARY","installedApplications":[{"bundleID":"com.example.demo"}]}}`
}

func fakeAppsPayload(build string) string {
	value, _ := json.Marshal(build)
	return `{"info":{"arguments":[],"commandType":"devicectl.device.info.apps","environment":{},"jsonVersion":5,"outcome":"success","version":"642.15"},"result":{"apps":[{"bundleIdentifier":"com.example.demo","version":"1.2.3","bundleVersion":` + string(value) + `}]}}`
}

func containsInstallArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func containsInstallSequence(args, want []string) bool {
	if len(want) == 0 {
		return true
	}
	for start := 0; start+len(want) <= len(args); start++ {
		matched := true
		for index, value := range want {
			if args[start+index] != value {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func TestDiscoverInstallDeviceSchemaVersions(t *testing.T) {
	for _, version := range []int{4, 5, 6} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			payload := strings.Replace(fakeDeviceListPayload(), `"jsonVersion":5`, fmt.Sprintf(`"jsonVersion":%d`, version), 1)
			if version == 4 {
				payload = `{"info":{"arguments":[],"commandType":"devicectl.list.devices","environment":{},"jsonVersion":4,"outcome":"success","version":"4"},"result":{"devices":[{"identifier":"SELECTOR_CANARY","hardwareProperties":{"platform":"iOS","reality":"physical","udid":"UDID_CANARY"},"connectionProperties":{"pairingState":"paired","tunnelState":"connected"},"deviceProperties":{"visibilityClass":"default"}}]}}`
			}
			if version == 6 {
				payload = strings.ReplaceAll(payload, `"platform":`, `"osPlatform":`)
				payload = strings.ReplaceAll(payload, `"reality":`, `"fidelity":`)
			}
			runner := &fakeInstallRunner{t: t, deviceListPayload: payload}
			device, err := discoverInstallDevice(context.Background(), runner, "/private/tmp/fake-devicectl", nil, t.TempDir(), 30*time.Second, "SELECTOR_CANARY", []string{"UDID_CANARY"})
			if err != nil {
				t.Fatalf("schema %d discovery failed: %v", version, err)
			}
			if device.Identifier != "SELECTOR_CANARY" || device.UDID != "UDID_CANARY" || device.Platform != "iOS" || device.Reality != "physical" || !isUsableInstallDevice(device) {
				t.Fatalf("schema %d discovery normalized incorrectly: %+v", version, device)
			}
			if len(runner.calls) != 1 || !containsInstallArg(runner.calls[0], "list") || !containsInstallArg(runner.calls[0], "devices") {
				t.Fatalf("discovery performed another operation: %v", runner.calls)
			}
		})
	}
}
