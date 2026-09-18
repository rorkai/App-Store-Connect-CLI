package xcode

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
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
