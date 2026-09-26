package xcode

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fullsailor/pkcs7"
	"howett.net/plist"
)

func TestSelectSigningProfilePrefersExactOverWildcard(t *testing.T) {
	exact := signingProfile{name: "Exact", uuid: "exact", pattern: "com.example.demo", expires: time.Now()}
	wild := signingProfile{name: "Wild", uuid: "wild", pattern: "com.example.*", wildcard: true, expires: time.Now().Add(time.Hour)}
	selected, _, match := selectSigningProfile([]signingProfile{wild, exact}, "com.example.demo")
	if selected == nil || selected.name != "Exact" || match != "exact" {
		t.Fatalf("selected = %#v match=%s, want exact Exact", selected, match)
	}
	selected, _, match = selectSigningProfile([]signingProfile{wild}, "com.example.other")
	if selected == nil || selected.name != "Wild" || match != "wildcard" {
		t.Fatalf("selected = %#v match=%s, want wildcard Wild", selected, match)
	}
	if selected, _, _ := selectSigningProfile([]signingProfile{wild}, "com.example"); selected != nil {
		t.Fatalf("wildcard matched a non-suffix bundle ID: %#v", selected)
	}
}

func TestInferSigningPlanFromProfilesAppliesUnchanged(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	expires := time.Now().Add(24 * time.Hour).UTC()
	profiles := []string{
		writeSigningTestProfile(t, filepath.Join(root, "App.mobileprovision"), "App Profile", "11111111-1111-1111-1111-111111111111", "ABCDE12345.com.example.demo", expires),
		writeSigningTestProfile(t, filepath.Join(root, "Widget.mobileprovision"), "Widget Profile", "22222222-2222-2222-2222-222222222222", "ABCDE12345.com.example.demo.widget", expires),
		writeSigningTestProfile(t, filepath.Join(root, "Watch.mobileprovision"), "Watch Profile", "33333333-3333-3333-3333-333333333333", "ABCDE12345.com.example.demo.watch", expires),
	}
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:   project,
		ProfilePaths:  profiles,
		Configuration: "Debug",
		StateDir:      filepath.Join(root, "state"),
		ExportMethod:  "development",
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, blockers=%v", plan.Blockers)
	}
	if len(plan.Inferences) != 3 {
		t.Fatalf("inferences = %#v", plan.Inferences)
	}
	for _, inference := range plan.Inferences {
		if inference.ProfileUUID == "" || inference.CertificateSHA256 == "" || inference.ProfilePath == "" {
			t.Fatalf("inference missing provenance: %#v", inference)
		}
		if inference.Match != "exact" {
			t.Fatalf("match = %s, want exact", inference.Match)
		}
	}
	if plan.ExportOptions == nil || plan.ExportOptions.Method != "development" || plan.ExportOptions.ProvisioningProfiles["com.example.demo"] != "App Profile" {
		t.Fatalf("export options = %#v", plan.ExportOptions)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
}

func TestInferSigningPlanSettingsFileOverrideWins(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	profile := writeSigningTestProfile(t, filepath.Join(root, "App.mobileprovision"), "App Profile", "11111111-1111-1111-1111-111111111111", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour))
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{
			"name": "App",
			"configurations": [{
				"name": "Debug",
				"settings": {"CODE_SIGN_IDENTITY": "Apple Development Override"}
			}]
		}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:      project,
		SettingsFilePath: settingsPath,
		ProfilePaths:     []string{profile},
		Configuration:    "Debug",
		SkipTargets:      []string{"Widget", "Watch"},
		StateDir:         filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, blockers=%v", plan.Blockers)
	}
	if !signingPlanSettingEquals(plan, "App", "Debug", "CODE_SIGN_IDENTITY", "Apple Development Override") {
		t.Fatalf("override did not win: %#v", plan.Desired)
	}
	if !signingPlanSettingEquals(plan, "App", "Debug", "PROVISIONING_PROFILE_SPECIFIER", "App Profile") {
		t.Fatalf("inferred specifier missing: %#v", plan.Desired)
	}
}

func signingPlanSettingEquals(plan *SigningPlan, target, configuration, key, want string) bool {
	for _, item := range plan.Desired {
		if item.Target != target {
			continue
		}
		for _, config := range item.Configurations {
			if config.Name != configuration {
				continue
			}
			for _, setting := range config.Settings {
				if setting.Key == key && setting.Value != nil && *setting.Value == want {
					return true
				}
			}
		}
	}
	return false
}

func writeSigningTestProfile(t *testing.T, path, name, uuid, applicationID string, expires time.Time) string {
	t.Helper()
	return writeSigningTestProfileWith(t, path, name, uuid, applicationID, expires, nil)
}

func writeSigningTestProfileWith(t *testing.T, path, name, uuid, applicationID string, expires time.Time, mutate func(map[string]any)) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Hour)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Apple Development"},
		NotBefore:    now,
		NotAfter:     now.Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"UUID": name + "-uuid", "Name": name, "TeamIdentifier": []string{"ABCDE12345"},
		"ApplicationIdentifierPrefix": []string{"ABCDE12345"},
		"ExpirationDate":              expires,
		"Entitlements": map[string]any{
			"application-identifier":              applicationID,
			"com.apple.developer.team-identifier": "ABCDE12345",
			"get-task-allow":                      true,
		},
		"DeveloperCertificates": [][]byte{der},
		"ProvisionedDevices":    []string{"device"},
	}
	payload["UUID"] = uuid
	if mutate != nil {
		mutate(payload)
	}
	encoded, err := plist.Marshal(payload, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := pkcs7.NewSignedData(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := signed.AddSigner(cert, key, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatal(err)
	}
	result, err := signed.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, result, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeInferredSigningProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	projectPath := filepath.Join(root, "Demo.xcodeproj")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	project := `// !$*UTF8*$!
{
	archiveVersion = 1;
	classes = {};
	objectVersion = 77;
	objects = {
		111111111111111111111111 /* Project object */ = {isa = PBXProject; attributes = {}; buildConfigurationList = 222222222222222222222222; targets = (333333333333333333333333, 444444444444444444444444, AAAAAAAAAAAAAAAAAAAAAAAA); };
		333333333333333333333333 /* App */ = {isa = PBXNativeTarget; buildConfigurationList = 555555555555555555555555; buildPhases = (); dependencies = (); name = App; productName = App; productType = "com.apple.product-type.application"; };
		444444444444444444444444 /* Widget */ = {isa = PBXNativeTarget; buildConfigurationList = 777777777777777777777777; buildPhases = (); dependencies = (); name = Widget; productName = Widget; productType = "com.apple.product-type.app-extension"; };
		AAAAAAAAAAAAAAAAAAAAAAAA /* Watch */ = {isa = PBXNativeTarget; buildConfigurationList = BBBBBBBBBBBBBBBBBBBBBBBB; buildPhases = (); dependencies = (); name = Watch; productName = Watch; productType = "com.apple.product-type.application.watchapp2"; };
		999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Debug; };
		999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Release; };
		999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo; }; name = Debug; };
		999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo; }; name = Release; };
		999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo.widget; }; name = Debug; };
		999999999999999999999996 /* Widget Release */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo.widget; }; name = Release; };
		CCCCCCCCCCCCCCCCCCCCCCCC /* Watch Debug */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo.watch; }; name = Debug; };
		DDDDDDDDDDDDDDDDDDDDDDDD /* Watch Release */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo.watch; }; name = Release; };
		222222222222222222222222 /* Project configuration list */ = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999991, 999999999999999999999992); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
		555555555555555555555555 /* App configuration list */ = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999993, 999999999999999999999994); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
		777777777777777777777777 /* Widget configuration list */ = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999995, 999999999999999999999996); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
		BBBBBBBBBBBBBBBBBBBBBBBB /* Watch configuration list */ = {isa = XCConfigurationList; buildConfigurations = (CCCCCCCCCCCCCCCCCCCCCCCC, DDDDDDDDDDDDDDDDDDDDDDDD); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
	};
	rootObject = 111111111111111111111111 /* Project object */;
}
`
	if err := os.WriteFile(filepath.Join(projectPath, "project.pbxproj"), []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	return projectPath
}

func TestSigningPlanJSONRecordsProfileProvenance(t *testing.T) {
	plan := &SigningPlan{Inferences: []SigningPlanInference{{Target: "App", ProfileUUID: "uuid", CertificateSHA256: "abc", ProfilePath: "App.mobileprovision"}}}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(encoded) {
		t.Fatalf("invalid JSON %s", encoded)
	}
}

func TestParseSigningProfileAcceptsTeamWildcard(t *testing.T) {
	root := t.TempDir()
	path := writeSigningTestProfile(t, filepath.Join(root, "Team.mobileprovision"), "Team Wildcard", "44444444-4444-4444-4444-444444444444", "ABCDE12345.*", time.Now().Add(time.Hour))
	profile, err := parseSigningProfile(path)
	if err != nil {
		t.Fatalf("parseSigningProfile() error = %v", err)
	}
	if !profile.wildcard || profile.pattern != "*" {
		t.Fatalf("profile pattern = %q wildcard=%t, want team wildcard", profile.pattern, profile.wildcard)
	}
	selected, _, match := selectSigningProfile([]signingProfile{profile}, "com.example.demo")
	if selected == nil || match != "wildcard" {
		t.Fatalf("team wildcard did not match: %#v %s", selected, match)
	}
}

func TestSelectSigningProfilePrefersNarrowerWildcard(t *testing.T) {
	later := time.Now().Add(48 * time.Hour)
	team := signingProfile{name: "Team", uuid: "team", pattern: "*", wildcard: true, expires: later}
	narrow := signingProfile{name: "Narrow", uuid: "narrow", pattern: "com.example.*", wildcard: true, expires: time.Now().Add(time.Hour)}
	selected, _, _ := selectSigningProfile([]signingProfile{team, narrow}, "com.example.demo")
	if selected == nil || selected.name != "Narrow" {
		t.Fatalf("selected = %#v, want narrower wildcard", selected)
	}
}

func TestInferSigningPlanIgnoresExpiredProfiles(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	expired := writeSigningTestProfile(t, filepath.Join(root, "Expired.mobileprovision"), "Expired Exact", "55555555-5555-5555-5555-555555555555", "ABCDE12345.com.example.demo", time.Now().Add(-time.Hour))
	wildcard := writeSigningTestProfile(t, filepath.Join(root, "Wild.mobileprovision"), "Live Wildcard", "66666666-6666-6666-6666-666666666666", "ABCDE12345.com.example.*", time.Now().Add(time.Hour))
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:   project,
		ProfilePaths:  []string{expired, wildcard},
		Configuration: "Release",
		StateDir:      filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, blockers=%v", plan.Blockers)
	}
	if !signingPlanSettingEquals(plan, "App", "Release", "PROVISIONING_PROFILE_SPECIFIER", "Live Wildcard") {
		t.Fatalf("expired exact profile was selected: %#v", plan.Inferences)
	}

	onlyExpired, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:   project,
		ProfilePaths:  []string{expired},
		Configuration: "Release",
		SkipTargets:   []string{"Widget", "Watch"},
		StateDir:      filepath.Join(root, "state-expired"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if onlyExpired.Ready {
		t.Fatalf("plan using only an expired profile must be blocked: %#v", onlyExpired.Inferences)
	}
	if !strings.Contains(strings.Join(onlyExpired.Blockers, "\n"), "expired") {
		t.Fatalf("blockers = %v, want an expired-profile explanation", onlyExpired.Blockers)
	}
}

func TestInferSigningPlanSkipsTargetsWithoutProvisioningProfiles(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeSigningProjectFixture(t, "333333333333333333333333, 444444444444444444444444, AAAAAAAAAAAAAAAAAAAAAAAA", `
		333333333333333333333333 /* App */ = {isa = PBXNativeTarget; buildConfigurationList = 555555555555555555555555; buildPhases = (); dependencies = (); name = App; productName = App; productType = "com.apple.product-type.application"; };
		444444444444444444444444 /* Kit */ = {isa = PBXNativeTarget; buildConfigurationList = 777777777777777777777777; buildPhases = (); dependencies = (); name = Kit; productName = Kit; productType = "com.apple.product-type.framework"; };
		AAAAAAAAAAAAAAAAAAAAAAAA /* AppTests */ = {isa = PBXNativeTarget; buildConfigurationList = BBBBBBBBBBBBBBBBBBBBBBBB; buildPhases = (); dependencies = (); name = AppTests; productName = AppTests; productType = "com.apple.product-type.bundle.unit-test"; };
		999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo; }; name = Release; };
		999999999999999999999996 /* Kit Release */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo.kit; }; name = Release; };
		DDDDDDDDDDDDDDDDDDDDDDDD /* AppTests Release */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo.tests; }; name = Release; };
		555555555555555555555555 = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999994); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
		777777777777777777777777 = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999996); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
		BBBBBBBBBBBBBBBBBBBBBBBB = {isa = XCConfigurationList; buildConfigurations = (DDDDDDDDDDDDDDDDDDDDDDDD); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };`)
	root := t.TempDir()
	wildcard := writeSigningTestProfile(t, filepath.Join(root, "Wild.mobileprovision"), "Wildcard", "77777777-7777-7777-7777-777777777777", "ABCDE12345.com.example.*", time.Now().Add(time.Hour))
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:  project,
		ProfilePaths: []string{wildcard},
		StateDir:     filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, blockers=%v", plan.Blockers)
	}
	if len(plan.Inferences) != 1 || plan.Inferences[0].Target != "App" {
		t.Fatalf("inferences = %#v, want only the App target", plan.Inferences)
	}
	for _, target := range plan.Desired {
		if target.Target != "App" {
			t.Fatalf("desired includes %s, which does not embed a provisioning profile", target.Target)
		}
	}
}

func writeSigningProjectFixture(t *testing.T, targetIDs, objects string) string {
	t.Helper()
	root := t.TempDir()
	projectPath := filepath.Join(root, "Demo.xcodeproj")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	project := `// !$*UTF8*$!
{
	archiveVersion = 1;
	classes = {};
	objectVersion = 77;
	objects = {
		111111111111111111111111 /* Project object */ = {isa = PBXProject; attributes = {}; buildConfigurationList = 222222222222222222222222; targets = (` + targetIDs + `); };
		999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Release; };
		222222222222222222222222 /* Project configuration list */ = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999992); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
` + objects + `
	};
	rootObject = 111111111111111111111111 /* Project object */;
}
`
	if err := os.WriteFile(filepath.Join(projectPath, "project.pbxproj"), []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	return projectPath
}

func TestInferSigningPlanMatchesMacOSProfilesByPlatform(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeSigningProjectFixture(t, "333333333333333333333333", `
		333333333333333333333333 /* Mac */ = {isa = PBXNativeTarget; buildConfigurationList = 555555555555555555555555; buildPhases = (); dependencies = (); name = Mac; productName = Mac; productType = "com.apple.product-type.application"; };
		999999999999999999999994 /* Mac Release */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo; SDKROOT = macosx; }; name = Release; };
		555555555555555555555555 = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999994); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };`)
	root := t.TempDir()
	ios := writeSigningTestProfileWith(t, filepath.Join(root, "iOS.mobileprovision"), "iOS Store", "88888888-8888-8888-8888-888888888888", "ABCDE12345.com.example.demo", time.Now().Add(48*time.Hour), func(payload map[string]any) {
		payload["Platform"] = []string{"iOS", "xrOS", "visionOS"}
		delete(payload, "ProvisionedDevices")
	})
	mac := writeSigningTestProfileWith(t, filepath.Join(root, "Mac.provisionprofile"), "Mac Developer ID", "99999999-9999-9999-9999-999999999999", "", time.Now().Add(time.Hour), func(payload map[string]any) {
		payload["Platform"] = []string{"OSX"}
		payload["ProvisionsAllDevices"] = true
		delete(payload, "ProvisionedDevices")
		payload["Entitlements"] = map[string]any{
			"com.apple.application-identifier":    "ABCDE12345.com.example.demo",
			"com.apple.developer.team-identifier": "ABCDE12345",
		}
	})
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:  project,
		ProfilePaths: []string{ios, mac},
		StateDir:     filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, blockers=%v", plan.Blockers)
	}
	if !signingPlanSettingEquals(plan, "Mac", "Release", "PROVISIONING_PROFILE_SPECIFIER", "Mac Developer ID") {
		t.Fatalf("macOS target did not select the macOS profile: %#v", plan.Inferences)
	}
	if plan.ExportOptions == nil || plan.ExportOptions.Method != "developer-id" {
		t.Fatalf("export options = %#v, want developer-id", plan.ExportOptions)
	}
}

func TestInferSigningPlanBlocksMixedExportMethods(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	store := writeSigningTestProfileWith(t, filepath.Join(root, "App.mobileprovision"), "App Store", "12121212-1212-1212-1212-121212121212", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour), func(payload map[string]any) {
		delete(payload, "ProvisionedDevices")
	})
	development := writeSigningTestProfile(t, filepath.Join(root, "Widget.mobileprovision"), "Widget Development", "34343434-3434-3434-3434-343434343434", "ABCDE12345.com.example.demo.widget", time.Now().Add(time.Hour))
	options := SigningPlanOptions{
		ProjectPath:   project,
		ProfilePaths:  []string{store, development},
		Configuration: "Release",
		SkipTargets:   []string{"Watch"},
		StateDir:      filepath.Join(root, "state"),
	}
	plan, err := BuildSigningPlan(options)
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if plan.Ready || !strings.Contains(strings.Join(plan.Blockers, "\n"), "different export methods") {
		t.Fatalf("ready=%t blockers=%v, want a mixed export method blocker", plan.Ready, plan.Blockers)
	}

	// An explicit method only considers profiles of that method, so the App
	// Store profile cannot sign the app for a development export.
	options.ExportMethod = "development"
	options.StateDir = filepath.Join(root, "state-explicit")
	explicit, err := BuildSigningPlan(options)
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if explicit.Ready || !strings.Contains(strings.Join(explicit.Blockers, "\n"), "App/Release") {
		t.Fatalf("explicit method plan ready=%t blockers=%v, want the App Store-only App blocked", explicit.Ready, explicit.Blockers)
	}
}

func TestInferSigningPlanFiltersProfilesByExportMethod(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	store := writeSigningTestProfileWith(t, filepath.Join(root, "Store.mobileprovision"), "App Store", "57575757-5757-5757-5757-575757575757", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour), func(payload map[string]any) {
		delete(payload, "ProvisionedDevices")
	})
	development := writeSigningTestProfile(t, filepath.Join(root, "Dev.mobileprovision"), "App Development", "68686868-6868-6868-6868-686868686868", "ABCDE12345.com.example.demo", time.Now().Add(48*time.Hour))
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:   project,
		ProfilePaths:  []string{store, development},
		Configuration: "Release",
		ExportMethod:  "app-store",
		SkipTargets:   []string{"Widget", "Watch"},
		StateDir:      filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready || !signingPlanSettingEquals(plan, "App", "Release", "PROVISIONING_PROFILE_SPECIFIER", "App Store") {
		t.Fatalf("ready=%t blockers=%v desired=%#v, want the App Store profile for app-store", plan.Ready, plan.Blockers, plan.Desired)
	}
}

func TestInferSigningPlanExportOptionsFollowSettingsOverrides(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	profile := writeSigningTestProfile(t, filepath.Join(root, "App.mobileprovision"), "App Profile", "56565656-5656-5656-5656-565656565656", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour))
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{
			"name": "App",
			"configurations": [{
				"name": "Release",
				"settings": {"PROVISIONING_PROFILE_SPECIFIER": "Custom App Profile", "DEVELOPMENT_TEAM": "ZYXWV98765"}
			}]
		}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:      project,
		SettingsFilePath: settingsPath,
		ProfilePaths:     []string{profile},
		Configuration:    "Release",
		SkipTargets:      []string{"Widget", "Watch"},
		StateDir:         filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, blockers=%v", plan.Blockers)
	}
	if plan.ExportOptions == nil ||
		plan.ExportOptions.ProvisioningProfiles["com.example.demo"] != "Custom App Profile" ||
		plan.ExportOptions.TeamID != "ZYXWV98765" {
		t.Fatalf("export options = %#v, want the settings-file overrides", plan.ExportOptions)
	}
}

func TestInferSigningPlanRejectsUnknownSkipTarget(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	profile := writeSigningTestProfile(t, filepath.Join(root, "Wild.mobileprovision"), "Wildcard", "78787878-7878-7878-7878-787878787878", "ABCDE12345.com.example.*", time.Now().Add(time.Hour))
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:  project,
		ProfilePaths: []string{profile},
		SkipTargets:  []string{"Widgte"},
		StateDir:     filepath.Join(root, "state"),
	})
	if err == nil || !IsSigningInputError(err) || !strings.Contains(err.Error(), `"Widgte"`) {
		t.Fatalf("error = %v, want an input error naming the unknown skip target", err)
	}
}

func TestInferSigningPlanExportOptionsIncludeSettingsOnlyTargets(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	profile := writeSigningTestProfileWith(t, filepath.Join(root, "App.mobileprovision"), "App Store", "90909090-9090-9090-9090-909090909090", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour), func(payload map[string]any) {
		delete(payload, "ProvisionedDevices")
	})
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{
			"name": "Widget",
			"configurations": [{
				"name": "Release",
				"settings": {"CODE_SIGN_STYLE": "Manual", "PROVISIONING_PROFILE_SPECIFIER": "Widget Store", "DEVELOPMENT_TEAM": "ABCDE12345"}
			}]
		}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:      project,
		SettingsFilePath: settingsPath,
		ProfilePaths:     []string{profile},
		Configuration:    "Release",
		SkipTargets:      []string{"Watch"},
		StateDir:         filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, blockers=%v", plan.Blockers)
	}
	if plan.ExportOptions == nil || plan.ExportOptions.ProvisioningProfiles["com.example.demo.widget"] != "Widget Store" {
		t.Fatalf("export options = %#v, want the settings-only Widget profile", plan.ExportOptions)
	}
}

func TestInferSigningPlanIgnoresProfilesWithExpiredCertificates(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Apple Distribution: Expired"},
		NotBefore:    time.Now().Add(-48 * time.Hour),
		NotAfter:     time.Now().Add(-time.Hour),
	}
	expiredCert, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	profile := writeSigningTestProfileWith(t, filepath.Join(root, "App.mobileprovision"), "Stale Cert", "abababab-abab-abab-abab-abababababab", "ABCDE12345.com.example.demo", time.Now().Add(24*time.Hour), func(payload map[string]any) {
		payload["DeveloperCertificates"] = [][]byte{expiredCert}
	})
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:   project,
		ProfilePaths:  []string{profile},
		Configuration: "Release",
		SkipTargets:   []string{"Widget", "Watch"},
		StateDir:      filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if plan.Ready || !strings.Contains(strings.Join(plan.Blockers, "\n"), "expired") {
		t.Fatalf("ready=%t blockers=%v, want an expired certificate blocker", plan.Ready, plan.Blockers)
	}
}

func TestInferSigningPlanIgnoresProfilesWithNotYetValidCertificates(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "Apple Distribution: Future"},
		NotBefore:    time.Now().Add(24 * time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
	}
	futureCert, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	profile := writeSigningTestProfileWith(t, filepath.Join(root, "App.mobileprovision"), "Future Cert", "cdcdcdcd-cdcd-cdcd-cdcd-cdcdcdcdcdcd", "ABCDE12345.com.example.demo", time.Now().Add(48*time.Hour), func(payload map[string]any) {
		payload["DeveloperCertificates"] = [][]byte{futureCert}
	})
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:   project,
		ProfilePaths:  []string{profile},
		Configuration: "Release",
		SkipTargets:   []string{"Widget", "Watch"},
		StateDir:      filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if plan.Ready || !strings.Contains(strings.Join(plan.Blockers, "\n"), "no currently valid signing certificate") {
		t.Fatalf("ready=%t blockers=%v, want a certificate validity blocker", plan.Ready, plan.Blockers)
	}
}

func TestInferSigningPlanUsesGenericIdentityForMultiCertificateProfiles(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	certificates := make([][]byte, 0, 2)
	for index, name := range []string{"Apple Development: Alice (AAAAAAAAAA)", "Apple Development: Bob (BBBBBBBBBB)"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		template := &x509.Certificate{
			SerialNumber: big.NewInt(int64(10 + index)),
			Subject:      pkix.Name{CommonName: name},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Duration(index+1) * 24 * time.Hour),
		}
		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		certificates = append(certificates, der)
	}
	profile := writeSigningTestProfileWith(t, filepath.Join(root, "App.mobileprovision"), "Team Development", "efefefef-efef-efef-efef-efefefefefef", "ABCDE12345.com.example.demo", time.Now().Add(48*time.Hour), func(payload map[string]any) {
		payload["DeveloperCertificates"] = certificates
	})
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:   project,
		ProfilePaths:  []string{profile},
		Configuration: "Debug",
		SkipTargets:   []string{"Widget", "Watch"},
		StateDir:      filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, blockers=%v", plan.Blockers)
	}
	if !signingPlanSettingEquals(plan, "App", "Debug", "CODE_SIGN_IDENTITY", "Apple Development") {
		t.Fatalf("identity is not the generic certificate kind: %#v", plan.Desired)
	}
}

func TestInferSigningPlanKeepsBlockerForIncompleteOverride(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	profile := writeSigningTestProfile(t, filepath.Join(root, "App.mobileprovision"), "App Profile", "13131313-1313-1313-1313-131313131313", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour))
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{
			"name": "Widget",
			"configurations": [{"name": "Release", "settings": {"CODE_SIGN_IDENTITY": "Apple Distribution"}}]
		}, {
			"name": "Watch",
			"configurations": [{"name": "Release", "settings": {"CODE_SIGN_STYLE": "Automatic"}}]
		}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:      project,
		SettingsFilePath: settingsPath,
		ProfilePaths:     []string{profile},
		Configuration:    "Release",
		StateDir:         filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	blockers := strings.Join(plan.Blockers, "\n")
	if plan.Ready || !strings.Contains(blockers, "Widget/Release") {
		t.Fatalf("ready=%t blockers=%v, want the Widget blocker kept", plan.Ready, plan.Blockers)
	}
	if strings.Contains(blockers, "Watch/Release") {
		t.Fatalf("blockers=%v, an Automatic signing override should cover Watch", plan.Blockers)
	}
}

func TestInferSigningPlanAutomaticOverrideClearsManualSettings(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	profile := writeSigningTestProfile(t, filepath.Join(root, "Wild.mobileprovision"), "Wildcard", "24242424-2424-2424-2424-242424242424", "ABCDE12345.com.example.*", time.Now().Add(time.Hour))
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{
			"name": "Widget",
			"configurations": [{"name": "Release", "settings": {"CODE_SIGN_STYLE": "Automatic"}}]
		}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:      project,
		SettingsFilePath: settingsPath,
		ProfilePaths:     []string{profile},
		Configuration:    "Release",
		SkipTargets:      []string{"Watch"},
		StateDir:         filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, blockers=%v", plan.Blockers)
	}
	if signingPlanSettingEquals(plan, "Widget", "Release", "PROVISIONING_PROFILE_SPECIFIER", "Wildcard") ||
		signingPlanSettingEquals(plan, "Widget", "Release", "CODE_SIGN_IDENTITY", "Apple Development") {
		t.Fatalf("automatic override kept inferred manual settings: %#v", plan.Desired)
	}
	if plan.ExportOptions == nil || plan.ExportOptions.ProvisioningProfiles["com.example.demo.widget"] != "" {
		t.Fatalf("export options = %#v, want no manual profile for the automatic Widget", plan.ExportOptions)
	}
}

func TestInferSigningPlanExportOptionsFollowFinalSigningStyle(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	unrelated := writeSigningTestProfile(t, filepath.Join(root, "Other.mobileprovision"), "Other", "35353535-3535-3535-3535-353535353535", "ABCDE12345.org.other.app", time.Now().Add(time.Hour))
	settingsOnlyPath := filepath.Join(root, "settings-only.json")
	writeSigningSettingsTestFile(t, settingsOnlyPath, `{
		"schemaVersion": 1,
		"targets": [{
			"name": "App",
			"configurations": [{"name": "Release", "settings": {"CODE_SIGN_STYLE": "Manual", "PROVISIONING_PROFILE_SPECIFIER": "App Store", "DEVELOPMENT_TEAM": "ABCDE12345"}}]
		}]
	}`)
	settingsOnly, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:      project,
		SettingsFilePath: settingsOnlyPath,
		ProfilePaths:     []string{unrelated},
		Configuration:    "Release",
		ExportMethod:     "app-store",
		SkipTargets:      []string{"Widget", "Watch"},
		StateDir:         filepath.Join(root, "state-settings"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !settingsOnly.Ready || settingsOnly.ExportOptions == nil || settingsOnly.ExportOptions.ProvisioningProfiles["com.example.demo"] != "App Store" {
		t.Fatalf("ready=%t blockers=%v export=%#v, want settings-only export options", settingsOnly.Ready, settingsOnly.Blockers, settingsOnly.ExportOptions)
	}

	matching := writeSigningTestProfile(t, filepath.Join(root, "App.mobileprovision"), "App Profile", "46464646-4646-4646-4646-464646464646", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour))
	automaticPath := filepath.Join(root, "automatic.json")
	writeSigningSettingsTestFile(t, automaticPath, `{
		"schemaVersion": 1,
		"targets": [{
			"name": "App",
			"configurations": [{"name": "Release", "settings": {"CODE_SIGN_STYLE": "Automatic"}}]
		}]
	}`)
	automatic, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:      project,
		SettingsFilePath: automaticPath,
		ProfilePaths:     []string{matching},
		Configuration:    "Release",
		SkipTargets:      []string{"Widget", "Watch"},
		StateDir:         filepath.Join(root, "state-automatic"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if automatic.ExportOptions != nil {
		t.Fatalf("export options = %#v, want none when every target signs automatically", automatic.ExportOptions)
	}
}

func TestInferSigningPlanExportOptionsAcceptProfileUUIDOverrides(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	unrelated := writeSigningTestProfile(t, filepath.Join(root, "Other.mobileprovision"), "Other", "79797979-7979-7979-7979-797979797979", "ABCDE12345.org.other.app", time.Now().Add(time.Hour))
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{
			"name": "App",
			"configurations": [{"name": "Release", "settings": {"CODE_SIGN_STYLE": "Manual", "PROVISIONING_PROFILE": "8A8A8A8A-8A8A-8A8A-8A8A-8A8A8A8A8A8A", "DEVELOPMENT_TEAM": "ABCDE12345"}}]
		}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:      project,
		SettingsFilePath: settingsPath,
		ProfilePaths:     []string{unrelated},
		Configuration:    "Release",
		ExportMethod:     "app-store",
		SkipTargets:      []string{"Widget", "Watch"},
		StateDir:         filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready || plan.ExportOptions == nil || plan.ExportOptions.ProvisioningProfiles["com.example.demo"] != "8A8A8A8A-8A8A-8A8A-8A8A-8A8A8A8A8A8A" {
		t.Fatalf("ready=%t blockers=%v export=%#v, want the UUID mapping", plan.Ready, plan.Blockers, plan.ExportOptions)
	}
}

func TestInferSigningPlanMatchesProfilesByTargetPlatform(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeSigningProjectFixture(t, "333333333333333333333333, 444444444444444444444444", `
		333333333333333333333333 /* Phone */ = {isa = PBXNativeTarget; buildConfigurationList = 555555555555555555555555; buildPhases = (); dependencies = (); name = Phone; productName = Phone; productType = "com.apple.product-type.application"; };
		444444444444444444444444 /* TV */ = {isa = PBXNativeTarget; buildConfigurationList = 777777777777777777777777; buildPhases = (); dependencies = (); name = TV; productName = TV; productType = "com.apple.product-type.application"; };
		999999999999999999999994 /* Phone Release */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo; SDKROOT = iphoneos; }; name = Release; };
		999999999999999999999996 /* TV Release */ = {isa = XCBuildConfiguration; buildSettings = { PRODUCT_BUNDLE_IDENTIFIER = com.example.demo; SDKROOT = appletvos; }; name = Release; };
		555555555555555555555555 = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999994); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
		777777777777777777777777 = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999996); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };`)
	root := t.TempDir()
	ios := writeSigningTestProfileWith(t, filepath.Join(root, "iOS.mobileprovision"), "iOS Profile", "9b9b9b9b-9b9b-9b9b-9b9b-9b9b9b9b9b9b", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour), func(payload map[string]any) {
		payload["Platform"] = []string{"iOS", "xrOS", "visionOS"}
	})
	tvos := writeSigningTestProfileWith(t, filepath.Join(root, "tvOS.mobileprovision"), "tvOS Profile", "acacacac-acac-acac-acac-acacacacacac", "ABCDE12345.com.example.demo", time.Now().Add(48*time.Hour), func(payload map[string]any) {
		payload["Platform"] = []string{"tvOS"}
	})
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:  project,
		ProfilePaths: []string{ios, tvos},
		StateDir:     filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !signingPlanSettingEquals(plan, "Phone", "Release", "PROVISIONING_PROFILE_SPECIFIER", "iOS Profile") ||
		!signingPlanSettingEquals(plan, "TV", "Release", "PROVISIONING_PROFILE_SPECIFIER", "tvOS Profile") {
		t.Fatalf("profiles were not matched by platform: %#v", plan.Inferences)
	}
}

func TestParseSigningProfileRejectsTamperedContent(t *testing.T) {
	root := t.TempDir()
	path := writeSigningTestProfile(t, filepath.Join(root, "App.mobileprovision"), "App Profile", "bdbdbdbd-bdbd-bdbd-bdbd-bdbdbdbdbdbd", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(data, []byte("com.example.demo"), []byte("com.example.evil"), 1)
	if bytes.Equal(tampered, data) {
		t.Fatal("fixture did not contain the application identifier")
	}
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseSigningProfile(path); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("parseSigningProfile() error = %v, want a signature verification failure", err)
	}
}
