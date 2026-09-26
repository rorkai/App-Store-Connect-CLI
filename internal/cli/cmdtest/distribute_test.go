package cmdtest

import (
	"archive/zip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"go.mozilla.org/pkcs7"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
)

func TestDistributeCommandsAreRegisteredWithAgentOrientedFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	inspect := findSubcommand(root, "distribute", "inspect")
	prepare := findSubcommand(root, "distribute", "prepare")
	plan := findSubcommand(root, "distribute", "plan")
	apply := findSubcommand(root, "distribute", "apply")
	resume := findSubcommand(root, "distribute", "resume")
	status := findSubcommand(root, "distribute", "status")
	verify := findSubcommand(root, "distribute", "verify")
	if inspect == nil || prepare == nil || plan == nil || apply == nil || resume == nil || status == nil || verify == nil {
		t.Fatalf("distribute commands missing: inspect=%v prepare=%v plan=%v apply=%v resume=%v status=%v verify=%v", inspect, prepare, plan, apply, resume, status, verify)
	}
	for _, name := range []string{"ipa", "include-devices", "output"} {
		if inspect.FlagSet.Lookup(name) == nil {
			t.Errorf("inspect --%s missing", name)
		}
	}
	for _, name := range []string{"ipa", "output-dir", "title", "channel", "source-revision", "source-url", "output"} {
		if prepare.FlagSet.Lookup(name) == nil {
			t.Errorf("prepare --%s missing", name)
		}
	}
	for _, name := range []string{"archive-path", "config", "plan", "state-dir", "output"} {
		if plan.FlagSet.Lookup(name) == nil {
			t.Errorf("plan --%s missing", name)
		}
	}
	for _, name := range []string{"plan", "confirm", "output"} {
		if apply.FlagSet.Lookup(name) == nil {
			t.Errorf("apply --%s missing", name)
		}
	}
	for _, command := range []*ffcli.Command{resume, status, verify} {
		for _, name := range []string{"run", "state-dir", "output"} {
			if command.FlagSet.Lookup(name) == nil {
				t.Errorf("%s --%s missing", command.Name, name)
			}
		}
	}
}

func TestDistributeApplyRequiresExactConfirmationBeforePlanRead(t *testing.T) {
	assertUsageExit(t, []string{"distribute", "apply", "--plan", filepath.Join(t.TempDir(), "missing.json")}, "--confirm PLAN_HASH is required")
	assertUsageExit(t, []string{"distribute", "apply", "--plan", filepath.Join(t.TempDir(), "missing.json"), "--confirm", "yes"}, "--confirm must be a 64-character lowercase SHA-256 plan hash")
}

func TestDistributeApplyWellFormedUnequalConfirmationExitsUsage(t *testing.T) {
	planPath := writeBlockedDistributionPlan(t)
	assertUsageExit(t, []string{
		"distribute", "apply",
		"--plan", planPath,
		"--confirm", strings.Repeat("2", 64),
		"--output", "json",
	}, "--confirm must be the exact planHash")
}

func TestDistributeInspectRequiresIPAAndRejectsInvalidOutput(t *testing.T) {
	assertUsageExit(t, []string{"distribute", "inspect"}, "--ipa is required")
	assertUsageExit(t, []string{"distribute", "inspect", "--ipa", "missing.ipa", "--output", "yaml"}, `--output must be one of`)
	assertUsageExit(t, []string{"distribute", "inspect", "unexpected", "--ipa", "missing.ipa"}, `unexpected argument "unexpected"`)
}

func TestDistributePrepareRejectsCredentialSourceURLBeforeFilesystemAccess(t *testing.T) {
	assertUsageExit(t, []string{
		"distribute", "prepare", "--ipa", "missing.ipa", "--source-url", "https://token@example.com/revision",
	}, "user information is not allowed")
	assertUsageExit(t, []string{
		"distribute", "prepare", "--ipa", "missing.ipa", "--source-url", "https://example.com/revision?token=secret",
	}, "query and fragment are not allowed")
	assertUsageExit(t, []string{
		"distribute", "prepare", "--ipa", "missing.ipa", "--source-url", "https://:443/path",
	}, "must be an absolute HTTPS URL")
	assertUsageExit(t, []string{"distribute", "prepare", "unexpected", "--ipa", "missing.ipa"}, `unexpected argument "unexpected"`)
}

func TestDistributeInspectJSONPrivacyAndExplicitDisclosure(t *testing.T) {
	ipa := writeDistributionIPA(t, "private-device-udid")
	stdout, stderr, runErr := runRootCommand(t, []string{"distribute", "inspect", "--output", "json", "--ipa", ipa})
	if runErr != nil {
		t.Fatalf("run error = %v; stderr=%s", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	if strings.Contains(stdout, "private-device-udid") {
		t.Fatalf("default inspect leaked UDID: %s", stdout)
	}
	var result distribution.Inspection
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Preparation.MetadataEligible || result.Signing.DeviceCount != 1 || result.App.BundleID != "com.example.demo" {
		t.Fatalf("unexpected inspection: %#v", result)
	}
	wantCodeSignatureStatus := expectedDistributionFixtureCodeSignatureStatus()
	if result.Signing.CodeSignatureVerification.Status != wantCodeSignatureStatus {
		t.Fatalf("unexpected signer verification: %#v", result.Signing.CodeSignatureVerification)
	}
	if runtime.GOOS != "darwin" && result.Signing.CodeSignatureVerification.Reason != "complete main-app code-signature verification is available only on macOS" {
		t.Fatalf("unexpected portable signer verification: %#v", result.Signing.CodeSignatureVerification)
	}

	stdout, stderr, runErr = runRootCommand(t, []string{"distribute", "inspect", "--ipa", ipa, "--include-devices", "--output", "json"})
	if runErr != nil || stderr != "" {
		t.Fatalf("run error=%v stderr=%q", runErr, stderr)
	}
	if !strings.Contains(stdout, "private-device-udid") {
		t.Fatalf("explicit inspect omitted UDID: %s", stdout)
	}
}

func TestDistributeInspectTableAndMarkdownDeviceDisclosure(t *testing.T) {
	ipa := writeDistributionIPA(t, "private-device-udid")
	for _, format := range []string{"table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			stdout, stderr, runErr := runRootCommand(t, []string{"distribute", "inspect", "--ipa", ipa, "--output", format})
			if runErr != nil || stderr != "" {
				t.Fatalf("run error=%v stderr=%q", runErr, stderr)
			}
			rows := parseDistributeInspectHumanRows(t, format, stdout)
			if rows["Bundle ID"] != "com.example.demo" ||
				rows["Code Signature"] != string(expectedDistributionFixtureCodeSignatureStatus()) ||
				rows["Devices"] != "1" {
				t.Fatalf("unexpected default %s rows: %#v", format, rows)
			}
			if value, exists := rows["Device UDIDs"]; exists {
				t.Fatalf("default %s output disclosed Device UDIDs row %q: %#v", format, value, rows)
			}
			for field, value := range rows {
				if strings.Contains(value, "private-device-udid") {
					t.Fatalf("default %s output leaked UDID in %q row: %#v", format, field, rows)
				}
			}

			stdout, stderr, runErr = runRootCommand(t, []string{
				"distribute", "inspect", "--ipa", ipa, "--include-devices", "--output", format,
			})
			if runErr != nil || stderr != "" {
				t.Fatalf("run with --include-devices error=%v stderr=%q", runErr, stderr)
			}
			rows = parseDistributeInspectHumanRows(t, format, stdout)
			if rows["Devices"] != "1" || rows["Device UDIDs"] != "private-device-udid" {
				t.Fatalf("public %s output omitted exact Device UDIDs row: %#v", format, rows)
			}
		})
	}
}

func expectedDistributionFixtureCodeSignatureStatus() distribution.CodeSignatureVerificationStatus {
	if runtime.GOOS == "darwin" {
		return distribution.CodeSignatureInvalid
	}
	return distribution.CodeSignatureNotVerified
}

func parseDistributeInspectHumanRows(t *testing.T, format, output string) map[string]string {
	t.Helper()
	delimiter := "│"
	if format == "markdown" {
		delimiter = "|"
	}
	rows := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, delimiter) || !strings.HasSuffix(line, delimiter) {
			continue
		}
		cells := strings.Split(strings.Trim(line, delimiter), delimiter)
		if len(cells) != 2 {
			t.Fatalf("malformed %s row %q", format, line)
		}
		field := strings.TrimSpace(cells[0])
		value := strings.TrimSpace(cells[1])
		if field == "Field" || strings.HasPrefix(field, ":-") {
			continue
		}
		rows[field] = value
	}
	return rows
}

func TestDistributePrepareFailsClosedForUnverifiedFixture(t *testing.T) {
	ipa := writeDistributionIPA(t, "private-device-udid")
	outputDir := filepath.Join(t.TempDir(), "bundle")
	args := []string{
		"distribute", "prepare", "--ipa", ipa, "--output-dir", outputDir,
		"--title", "Preview", "--channel", "pull-request-7", "--source-revision", "abcdef", "--output", "json",
	}
	stdout, stderr, runErr := runRootCommand(t, args)
	if runErr == nil || !strings.Contains(runErr.Error(), "complete main-app signature verification") {
		t.Fatalf("run error=%v stderr=%q", runErr, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout=%q", stdout)
	}
	if _, err := os.Stat(outputDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unverified prepare created output: %v", err)
	}
}

func TestDistributeInspectRejectsSymlinkIPA(t *testing.T) {
	ipa := writeDistributionIPA(t, "device")
	link := filepath.Join(t.TempDir(), "linked.ipa")
	if err := os.Symlink(ipa, link); err != nil {
		t.Fatal(err)
	}
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"distribute", "inspect", "--ipa", link, "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected runtime symlink error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q", stdout)
	}
}

func writeDistributionIPA(t *testing.T, device string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Hour)
	template := &x509.Certificate{SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "Fixture"}, NotBefore: now, NotAfter: now.Add(365 * 24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID": "fixture-profile", "TeamIdentifier": []string{"TEAM123"}, "ApplicationIdentifierPrefix": []string{"SEED123"},
		"Platform":           []string{"iOS", "visionOS"},
		"ProvisionedDevices": []string{device}, "ExpirationDate": now.Add(48 * time.Hour), "DeveloperCertificates": [][]byte{der},
		"Entitlements": map[string]any{"application-identifier": "SEED123.com.example.demo", "com.apple.developer.team-identifier": "TEAM123", "get-task-allow": false},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := pkcs7.NewSignedData(profilePlist)
	if err != nil {
		t.Fatal(err)
	}
	if err := signed.AddSigner(cert, key, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatal(err)
	}
	profile, err := signed.Finish()
	if err != nil {
		t.Fatal(err)
	}
	infoPlist, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier": "com.example.demo", "CFBundleDisplayName": "Demo", "CFBundleShortVersionString": "1.0", "CFBundleVersion": "7", "MinimumOSVersion": "17.0",
		"DTPlatformName": "iphoneos", "CFBundleSupportedPlatforms": []string{"iPhoneOS"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "App.ipa")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, data := range map[string][]byte{
		"Payload/Demo.app/Info.plist":               infoPlist,
		"Payload/Demo.app/embedded.mobileprovision": profile,
	} {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

type blackboxDistributionPlan struct {
	SchemaVersion int                             `json:"schemaVersion"`
	PlanID        string                          `json:"planId"`
	PlanHash      string                          `json:"planHash"`
	CreatedAt     string                          `json:"createdAt"`
	Ready         bool                            `json:"ready"`
	ConfigPath    string                          `json:"configPath"`
	ConfigSHA256  string                          `json:"configSha256"`
	Archive       blackboxDistributionArchive     `json:"archive"`
	DeviceSet     blackboxDistributionDeviceSet   `json:"deviceSet"`
	Identity      blackboxDistributionIdentity    `json:"identity"`
	Publication   blackboxDistributionPublication `json:"publication"`
	Reconcile     blackboxDistributionReconcile   `json:"reconcile"`
	Effects       []blackboxDistributionEffect    `json:"effects"`
	Blockers      []blackboxDistributionBlocker   `json:"blockers,omitempty"`
	Paths         blackboxDistributionPlanPaths   `json:"paths"`
}

type blackboxDistributionArchive struct {
	Path             string `json:"path"`
	TreeSHA256       string `json:"treeSha256"`
	SizeBytes        int64  `json:"sizeBytes"`
	FileCount        int    `json:"fileCount"`
	BundleID         string `json:"bundleId"`
	Title            string `json:"title"`
	PublishedTitle   string `json:"publishedTitle"`
	Version          string `json:"version"`
	BuildNumber      string `json:"buildNumber"`
	MinimumOSVersion string `json:"minimumOSVersion,omitempty"`
	TeamID           string `json:"teamId"`
	TargetCount      int    `json:"targetCount"`
}

type blackboxDistributionDeviceSet struct {
	SHA256     string `json:"sha256"`
	FileSHA256 string `json:"fileSha256"`
	Count      int    `json:"count"`
}

type blackboxDistributionIdentity struct {
	CertificateResourceID string `json:"certificateResourceId"`
	CertificateSHA256     string `json:"certificateSha256"`
	TeamID                string `json:"teamId"`
	ExpirationDate        string `json:"expirationDate"`
	MinimumValidUntil     string `json:"minimumValidUntil"`
}

type blackboxDistributionPublication struct {
	Endpoint         string `json:"endpoint"`
	DownloadEndpoint string `json:"downloadEndpoint,omitempty"`
	Region           string `json:"region"`
	Bucket           string `json:"bucket"`
	Prefix           string `json:"prefix"`
	AddressingStyle  string `json:"addressingStyle"`
	URLTTL           string `json:"urlTtl"`
	DownloadGrace    string `json:"downloadGrace"`
	VerifyTimeout    string `json:"verifyTimeout"`
}

type blackboxDistributionReconcile struct {
	PlanPath            string `json:"planPath"`
	PlanHash            string `json:"planHash"`
	ReceiptPath         string `json:"receiptPath"`
	MinimumValidityDays int    `json:"minimumValidityDays"`
	MutationCount       int    `json:"mutationCount"`
	MaxMutations        int    `json:"maxMutations"`
}

type blackboxDistributionEffect struct {
	Stage    string `json:"stage"`
	Kind     string `json:"kind"`
	BundleID string `json:"bundleId,omitempty"`
	Count    int    `json:"count,omitempty"`
}

type blackboxDistributionBlocker struct {
	Code    string `json:"code"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

type blackboxDistributionPlanPaths struct {
	StateDir string `json:"stateDir"`
}

func writeBlockedDistributionPlan(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	digest := strings.Repeat("1", 64)
	plan := blackboxDistributionPlan{
		SchemaVersion: 1,
		PlanID:        "dplan_11111111111111111111111111111111",
		CreatedAt:     "2026-08-13T08:00:00Z",
		Ready:         false,
		ConfigPath:    filepath.Join(root, "distribution.json"),
		ConfigSHA256:  digest,
		Archive: blackboxDistributionArchive{
			Path: filepath.Join(root, "Demo.xcarchive"), TreeSHA256: digest,
			SizeBytes: 1, FileCount: 1, BundleID: "com.example.demo", Title: "Demo", PublishedTitle: "Demo",
			Version: "1.2.3", BuildNumber: "42", MinimumOSVersion: "17.0",
			TeamID: "TEAM123", TargetCount: 2,
		},
		DeviceSet: blackboxDistributionDeviceSet{SHA256: digest, FileSHA256: digest, Count: 1},
		Identity: blackboxDistributionIdentity{
			CertificateSHA256: digest, TeamID: "TEAM123",
			ExpirationDate: "2027-08-13T08:00:00Z", MinimumValidUntil: "2026-08-13T08:00:00Z",
		},
		Publication: blackboxDistributionPublication{
			Endpoint: "https://storage.example.com", Region: "us-east-1", Bucket: "private-previews",
			Prefix: "previews", AddressingStyle: "path", URLTTL: "1h", DownloadGrace: "1h", VerifyTimeout: "30s",
		},
		Reconcile: blackboxDistributionReconcile{
			PlanPath: filepath.Join(root, "reconcile-plan.json"), PlanHash: digest,
			ReceiptPath: filepath.Join(root, "reconcile-receipt.json"), MinimumValidityDays: 1, MaxMutations: 1,
		},
		Effects: []blackboxDistributionEffect{},
		Blockers: []blackboxDistributionBlocker{{
			Code: "embedded_targets_unsupported", Stage: "preflight", Message: "One main application target is required.",
		}},
		Paths: blackboxDistributionPlanPaths{StateDir: filepath.Join(root, "runs")},
	}
	canonical := plan
	canonical.PlanHash = ""
	canonical.CreatedAt = ""
	encoded, err := json.Marshal(canonical)
	if err != nil {
		t.Fatal(err)
	}
	plan.PlanHash = fmt.Sprintf("%x", sha256.Sum256(encoded))
	encoded, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "plan.json")
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
