package signing

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSelectReconcileCertificateWithSHA256(t *testing.T) {
	first, _ := newReconcileTestCertificateForTeam(t, "First", "TEAM1")
	second, _ := newReconcileTestCertificateForTeam(t, "Second", "TEAM1")
	resource := func(id string, certificateContent []byte) asc.Resource[asc.CertificateAttributes] {
		return asc.Resource[asc.CertificateAttributes]{
			ID: id,
			Attributes: asc.CertificateAttributes{
				CertificateType:    "IOS_DISTRIBUTION",
				ExpirationDate:     certificateExpiryRFC3339(t, certificateContent),
				CertificateContent: base64.StdEncoding.EncodeToString(certificateContent),
			},
		}
	}
	resources := []asc.Resource[asc.CertificateAttributes]{
		resource("cert-1", first.Raw),
		resource("cert-2", second.Raw),
	}
	resources = append([]asc.Resource[asc.CertificateAttributes]{{
		ID: "cert-malformed",
		Attributes: asc.CertificateAttributes{
			CertificateType:    "IOS_DISTRIBUTION",
			ExpirationDate:     certificateExpiryRFC3339(t, first.Raw),
			CertificateContent: "not-base64",
		},
	}}, resources...)
	secondDigest := sha256.Sum256(second.Raw)
	secondSHA := hex.EncodeToString(secondDigest[:])

	selected, blockers := selectReconcileCertificateWithFingerprint(resources, "", secondSHA, time.Now(), 7)
	if selected == nil || selected.ID != "cert-2" || selected.SHA256 != secondSHA || len(blockers) != 0 {
		t.Fatalf("selected=%#v blockers=%#v", selected, blockers)
	}
	malformed := resource("cert-malformed", first.Raw)
	malformed.Attributes.CertificateContent = "not-base64"
	selected, blockers = selectReconcileCertificateWithFingerprint(append(resources, malformed), "", secondSHA, time.Now(), 7)
	if selected == nil || selected.ID != "cert-2" || len(blockers) != 0 {
		t.Fatalf("selection with unrelated malformed certificate: selected=%#v blockers=%#v", selected, blockers)
	}

	selected, blockers = selectReconcileCertificateWithFingerprint(resources, "cert-1", secondSHA, time.Now(), 7)
	if selected != nil || !strings.Contains(strings.Join(blockers, "\n"), "does not match") {
		t.Fatalf("ID/fingerprint mismatch selected=%#v blockers=%#v", selected, blockers)
	}

	duplicate := append(resources, resource("cert-3", second.Raw))
	selected, blockers = selectReconcileCertificateWithFingerprint(duplicate, "", secondSHA, time.Now(), 7)
	if selected != nil || !strings.Contains(strings.Join(blockers, "\n"), "eligible iOS distribution certificates match") {
		t.Fatalf("duplicate fingerprint selected=%#v blockers=%#v", selected, blockers)
	}
}

func TestExecuteReconcileApplyRejectsConfirmationAndHashBeforeAuth(t *testing.T) {
	authCalls := 0
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		authCalls++
		return nil, errors.New("auth must not be reached")
	}))

	if _, err := ExecuteReconcileApply(context.Background(), ReconcileApplyOptions{
		PlanPath: "missing.json", ExpectedPlanHash: strings.Repeat("0", 64), Confirm: false,
	}); err == nil || ClassifyReconcileExecutionError(err) != ReconcileExecutionErrorPlanInvalid {
		t.Fatalf("Confirm=false error=%v", err)
	}
	if authCalls != 0 {
		t.Fatalf("auth calls after confirmation rejection=%d", authCalls)
	}

	stateDir := t.TempDir()
	plan := signingReconcilePlanArtifact{
		SchemaVersion: signingReconcileSchemaV1,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Command:       "signing reconcile plan",
		Ready:         true,
		Paths: signingReconcilePaths{
			StateDir: stateDir, PlanPath: filepath.Join(stateDir, "plan.json"),
			ReceiptPath: filepath.Join(stateDir, "receipt.json"), ProfilesDir: filepath.Join(stateDir, "profiles"),
		},
	}
	var err error
	plan.PlanHash, err = hashSigningReconcilePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSigningStateJSON(stateDir, "plan.json", plan, false); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecuteReconcileApply(context.Background(), ReconcileApplyOptions{
		PlanPath: plan.Paths.PlanPath, ExpectedPlanHash: strings.Repeat("f", 64), Confirm: true,
	}); err == nil || ClassifyReconcileExecutionError(err) != ReconcileExecutionErrorPlanInvalid {
		t.Fatalf("hash mismatch error=%v", err)
	}
	if authCalls != 0 {
		t.Fatalf("auth calls after hash rejection=%d", authCalls)
	}
}

func TestReconcileProductionAdaptersDoNotPrintOrReturnCanaryDetails(t *testing.T) {
	canary := "PROVIDER-DETAIL-LEAK-CANARY"
	missingPlan := filepath.Join(t.TempDir(), canary+".json")
	options := ReconcileApplyOptions{PlanPath: missingPlan, ExpectedPlanHash: strings.Repeat("a", 64), Confirm: true}

	for name, execute := range map[string]func() error{
		"apply": func() error {
			_, err := ExecuteReconcileApply(context.Background(), options)
			return err
		},
		"verify": func() error {
			_, err := VerifyReconcileCompletion(context.Background(), options)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				err := execute()
				if err == nil {
					t.Fatal("adapter unexpectedly succeeded")
				}
				if strings.Contains(err.Error(), canary) || strings.Contains(err.Error(), missingPlan) {
					t.Fatalf("adapter error leaked protected detail: %v", err)
				}
				if got := ClassifyReconcileExecutionError(err); got != ReconcileExecutionErrorPlanInvalid {
					t.Fatalf("error kind = %q, want %q", got, ReconcileExecutionErrorPlanInvalid)
				}
			})
			if stdout != "" || stderr != "" {
				t.Fatalf("adapter wrote output: stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

func TestExecuteReconcilePlanCollapsesOperationalCanaryWithoutOutput(t *testing.T) {
	secrets := []string{
		"RECONCILE-PLAN-PROVIDER-CANARY",
		"https://objects.example.test/app.ipa?X-Amz-Credential=RAW-CREDENTIAL&X-Amz-Signature=SECRET",
		"AABBCCDD",
		"device-resource-private-1",
		"/private/operator/signing/reconcile.json",
	}
	providerDetail := strings.Join(secrets, " ")
	stateDir := t.TempDir()
	devicesPath := filepath.Join(stateDir, "devices.json")
	if err := os.WriteFile(devicesPath, []byte(`{"schemaVersion":1,"devices":[{"name":"Phone","udid":"AABBCCDD","platform":"IOS"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	originalArchiveReader := readSigningArchiveRequirements
	readSigningArchiveRequirements = func(string) (signingArchiveRequirements, error) {
		return signingArchiveRequirements{TeamID: "TEAM1", Targets: []signingTarget{{Kind: "application", BundleID: "com.example.app"}}}, nil
	}
	t.Cleanup(func() { readSigningArchiveRequirements = originalArchiveReader })
	originalFactory := sharedASCClient
	sharedASCClient = func() (*asc.Client, error) { return nil, errors.New(providerDetail) }
	t.Cleanup(func() { sharedASCClient = originalFactory })

	stdout, stderr := captureOutput(t, func() {
		_, err := ExecuteReconcilePlan(context.Background(), ReconcilePlanOptions{
			ArchivePath: "App.xcarchive", DevicesFile: devicesPath, CertificateSHA256: strings.Repeat("a", 64),
			MinimumValidityDays: 7, MaxMutations: 32, StateDir: stateDir,
		})
		if err == nil || ClassifyReconcileExecutionError(err) != ReconcileExecutionErrorRetryable {
			t.Fatalf("planning adapter error = %v", err)
		}
		for _, secret := range secrets {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("planning adapter error leaked %q: %v", secret, err)
			}
		}
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("planning adapter wrote output: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestExecuteReconcilePlanClassifiesTerminalArchiveFailureAsPlanInvalid(t *testing.T) {
	stateDir := t.TempDir()
	devicesPath := filepath.Join(stateDir, "devices.json")
	if err := os.WriteFile(devicesPath, []byte(`{"schemaVersion":1,"devices":[{"name":"Phone","udid":"AABBCCDD","platform":"IOS"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	originalArchiveReader := readSigningArchiveRequirements
	readSigningArchiveRequirements = func(string) (signingArchiveRequirements, error) {
		return signingArchiveRequirements{TeamID: "TEAM1"}, nil
	}
	t.Cleanup(func() { readSigningArchiveRequirements = originalArchiveReader })
	authCalls := 0
	originalFactory := sharedASCClient
	sharedASCClient = func() (*asc.Client, error) {
		authCalls++
		return nil, errors.New("auth must not be reached")
	}
	t.Cleanup(func() { sharedASCClient = originalFactory })

	_, err := ExecuteReconcilePlan(context.Background(), ReconcilePlanOptions{
		ArchivePath: "App.xcarchive", DevicesFile: devicesPath, CertificateSHA256: strings.Repeat("a", 64),
		MinimumValidityDays: 7, MaxMutations: 32, StateDir: stateDir,
	})
	if err == nil || ClassifyReconcileExecutionError(err) != ReconcileExecutionErrorPlanInvalid {
		t.Fatalf("terminal planning error = %v", err)
	}
	if authCalls != 0 {
		t.Fatalf("terminal archive failure reached auth %d times", authCalls)
	}
}

func TestReconcileRetryClassificationTreatsAuthAsRetryableAndNotFoundAsDrift(t *testing.T) {
	for _, status := range []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	} {
		if !isRetryableReconcileFailure(&asc.APIError{StatusCode: status, Detail: "private provider detail"}) {
			t.Fatalf("status %d should be retryable", status)
		}
	}
	if isRetryableReconcileFailure(&asc.APIError{StatusCode: http.StatusNotFound, Detail: "private provider detail"}) {
		t.Fatal("404 should be semantic drift")
	}
}

func TestExecuteReconcileApplyCollapsesOperationalCanaryError(t *testing.T) {
	canary := "RAW-AUTH-PROVIDER-CANARY"
	stateDir := t.TempDir()
	devicesPath := filepath.Join(stateDir, "devices.json")
	devicesBody := `{"schemaVersion":1,"devices":[{"name":"Private Phone","udid":"SECRET-UDID","platform":"IOS"}]}`
	if err := os.WriteFile(devicesPath, []byte(devicesBody), 0o600); err != nil {
		t.Fatal(err)
	}
	devices, err := decodeSigningDevicesFile([]byte(devicesBody))
	if err != nil {
		t.Fatal(err)
	}
	target := signingTarget{Kind: "application", RelativePath: "Products/Applications/App.app", BundleID: "com.example.app", Executable: "App"}
	plan := signingReconcilePlanArtifact{
		SchemaVersion: signingReconcileSchemaV1, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Command: "signing reconcile plan", Ready: true,
		TeamID: "TEAM1", MinimumValidityDays: 0, MaxMutations: 1, MutationCount: 0,
		DeviceSetSHA256: digestSigningDeviceInputs(devices.Devices).SHA256,
		Paths:           signingReconcilePaths{ArchivePath: "App.xcarchive", DevicesFile: devicesPath, StateDir: stateDir, PlanPath: filepath.Join(stateDir, "plan.json"), ReceiptPath: filepath.Join(stateDir, "receipt.json"), ProfilesDir: filepath.Join(stateDir, "profiles")},
		Certificate:     &signingCertificateRef{ID: "cert-1", SHA256: strings.Repeat("a", 64), TeamID: "TEAM1", ExpirationDate: "2100-01-01T00:00:00Z"},
		Targets:         []signingTarget{target}, Devices: []signingDesiredDevice{{Platform: "IOS", Fingerprint: devices.Devices[0].Fingerprint, NameSHA256: fingerprintReconcileName("Private Phone")}},
		Actions: []signingAction{{ID: "download:com.example.app", Kind: actionDownloadProfile, BundleID: "com.example.app", ProfileID: "profile-1"}},
	}
	plan.PlanHash, err = hashSigningReconcilePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSigningStateJSON(stateDir, "plan.json", plan, false); err != nil {
		t.Fatal(err)
	}
	originalArchiveReader := readSigningArchiveRequirements
	archiveDrift := false
	readSigningArchiveRequirements = func(string) (signingArchiveRequirements, error) {
		if archiveDrift {
			return signingArchiveRequirements{TeamID: "OTHERTEAM", Targets: []signingTarget{target}}, nil
		}
		return signingArchiveRequirements{TeamID: plan.TeamID, Targets: plan.Targets}, nil
	}
	t.Cleanup(func() { readSigningArchiveRequirements = originalArchiveReader })
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return nil, &asc.APIError{StatusCode: http.StatusServiceUnavailable, Detail: canary}
	}))

	archiveDrift = true
	stdout, stderr := captureOutput(t, func() {
		_, err := ExecuteReconcileApply(context.Background(), ReconcileApplyOptions{PlanPath: plan.Paths.PlanPath, ExpectedPlanHash: plan.PlanHash, Confirm: true})
		if err == nil || !errors.Is(err, ErrReconcilePlanDrift) || ClassifyReconcileExecutionError(err) != ReconcileExecutionErrorPlanDrift {
			t.Fatalf("drift error = %v", err)
		}
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("drift adapter wrote output: stdout=%q stderr=%q", stdout, stderr)
	}
	archiveDrift = false

	stdout, stderr = captureOutput(t, func() {
		_, err := ExecuteReconcileApply(context.Background(), ReconcileApplyOptions{PlanPath: plan.Paths.PlanPath, ExpectedPlanHash: plan.PlanHash, Confirm: true})
		if err == nil || ClassifyReconcileExecutionError(err) != ReconcileExecutionErrorRetryable {
			t.Fatalf("adapter error = %v", err)
		}
		for _, secret := range []string{canary, "SECRET-UDID", "Private Phone", plan.Paths.PlanPath} {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("adapter error leaked %q: %v", secret, err)
			}
		}
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("adapter wrote output: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestExecuteReconcileApplySilentlyClassifiesExistingReceiptConflictAsDrift(t *testing.T) {
	canary := "RECEIPT-CONFLICT-CANARY"
	stateDir := t.TempDir()
	devicesPath := filepath.Join(stateDir, "devices.json")
	if err := os.WriteFile(devicesPath, []byte(`{"schemaVersion":1,"devices":[{"name":"Phone","udid":"AABBCCDD","platform":"IOS"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	devices, err := decodeSigningDevicesFile([]byte(`{"schemaVersion":1,"devices":[{"name":"Phone","udid":"AABBCCDD","platform":"IOS"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	target := signingTarget{Kind: "application", BundleID: "com.example.app"}
	plan := signingReconcilePlanArtifact{
		SchemaVersion: signingReconcileSchemaV1, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Command: "signing reconcile plan", Ready: true,
		TeamID: "TEAM1", MinimumValidityDays: 0, MaxMutations: 1, DeviceSetSHA256: digestSigningDeviceInputs(devices.Devices).SHA256,
		Paths:       signingReconcilePaths{ArchivePath: "App.xcarchive", DevicesFile: devicesPath, StateDir: stateDir, PlanPath: filepath.Join(stateDir, "plan.json"), ReceiptPath: filepath.Join(stateDir, "receipt.json"), ProfilesDir: filepath.Join(stateDir, "profiles")},
		Certificate: &signingCertificateRef{ID: "cert-1", SHA256: strings.Repeat("a", 64), TeamID: "TEAM1", ExpirationDate: "2100-01-01T00:00:00Z"},
		Targets:     []signingTarget{target}, Devices: []signingDesiredDevice{{Platform: "IOS", Fingerprint: devices.Devices[0].Fingerprint, NameSHA256: fingerprintReconcileName("Phone")}},
	}
	plan.PlanHash, err = hashSigningReconcilePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSigningStateJSON(stateDir, "plan.json", plan, false); err != nil {
		t.Fatal(err)
	}
	conflict := signingReconcileReceipt{SchemaVersion: signingReconcileSchemaV1, PlanHash: strings.Repeat("b", 64), StateDir: canary, ReceiptPath: filepath.Join(stateDir, canary+".json")}
	if err := writeSigningStateJSON(stateDir, "receipt.json", conflict, false); err != nil {
		t.Fatal(err)
	}
	originalArchiveReader := readSigningArchiveRequirements
	readSigningArchiveRequirements = func(string) (signingArchiveRequirements, error) {
		return signingArchiveRequirements{TeamID: plan.TeamID, Targets: plan.Targets}, nil
	}
	t.Cleanup(func() { readSigningArchiveRequirements = originalArchiveReader })

	stdout, stderr := captureOutput(t, func() {
		_, err := ExecuteReconcileApply(context.Background(), ReconcileApplyOptions{PlanPath: plan.Paths.PlanPath, ExpectedPlanHash: plan.PlanHash, Confirm: true})
		if err == nil || !errors.Is(err, ErrReconcilePlanDrift) || ClassifyReconcileExecutionError(err) != ReconcileExecutionErrorPlanDrift || strings.Contains(err.Error(), canary) {
			t.Fatalf("receipt conflict error = %v", err)
		}
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("receipt conflict wrote output: stdout=%q stderr=%q", stdout, stderr)
	}

	if err := os.Remove(plan.Paths.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	malformedCanary := "MALFORMED-RECEIPT-PRIVATE-CANARY"
	if err := os.WriteFile(plan.Paths.ReceiptPath, []byte(`{"schemaVersion":1,"private":"`+malformedCanary+`"`), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr = captureOutput(t, func() {
		_, err := ExecuteReconcileApply(context.Background(), ReconcileApplyOptions{PlanPath: plan.Paths.PlanPath, ExpectedPlanHash: plan.PlanHash, Confirm: true})
		if err == nil || !errors.Is(err, ErrReconcilePlanDrift) || ClassifyReconcileExecutionError(err) != ReconcileExecutionErrorPlanDrift || strings.Contains(err.Error(), malformedCanary) {
			t.Fatalf("malformed receipt error = %v", err)
		}
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("malformed receipt wrote output: stdout=%q stderr=%q", stdout, stderr)
	}

	if err := os.Remove(plan.Paths.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	pathCanary := filepath.Join(t.TempDir(), "WRONG-RECEIPT-PATH-CANARY")
	wrongPath := signingReconcileReceipt{
		SchemaVersion: signingReconcileSchemaV1, PlanHash: plan.PlanHash,
		StateDir: pathCanary, ReceiptPath: filepath.Join(pathCanary, "receipt.json"),
	}
	data, err := json.Marshal(wrongPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.Paths.ReceiptPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr = captureOutput(t, func() {
		_, err := ExecuteReconcileApply(context.Background(), ReconcileApplyOptions{PlanPath: plan.Paths.PlanPath, ExpectedPlanHash: plan.PlanHash, Confirm: true})
		if err == nil || !errors.Is(err, ErrReconcilePlanDrift) || ClassifyReconcileExecutionError(err) != ReconcileExecutionErrorPlanDrift || strings.Contains(err.Error(), pathCanary) {
			t.Fatalf("wrong-path receipt error = %v", err)
		}
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("wrong-path receipt wrote output: stdout=%q stderr=%q", stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(pathCanary, "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("receipt escaped protected state dir: %v", err)
	}
}

func TestSigningReconcileCLIKeepsDetailedUsageDiagnostic(t *testing.T) {
	canary := "PUBLIC-CLI-PLAN-CANARY"
	path := filepath.Join(t.TempDir(), canary+".json")
	stdout, stderr := captureOutput(t, func() {
		if _, err := executeSigningReconcileApply(context.Background(), path); err == nil {
			t.Fatal("public executor unexpectedly succeeded")
		}
	})
	if stdout != "" || !strings.Contains(stderr, canary) {
		t.Fatalf("public CLI diagnostic changed: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestReconcileReceiptViewSelectsExactMainProfile(t *testing.T) {
	certificate, key := newReconcileTestCertificate(t, "Profile Signer")
	stateDir := t.TempDir()
	mainUUID := "11111111-1111-1111-1111-111111111111"
	extensionUUID := "22222222-2222-2222-2222-222222222222"
	writeProfile := func(uuid string) (string, string) {
		t.Helper()
		content := buildReconcileTestMobileProvision(t, map[string]any{
			"UUID": uuid, "ExpirationDate": time.Now().Add(30 * 24 * time.Hour), "Entitlements": map[string]any{},
			"DeveloperCertificates": [][]byte{certificate.Raw},
		}, certificate, key)
		path, err := writeVerifiedProfile(stateDir, content)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(content)
		return path, hex.EncodeToString(digest[:])
	}
	mainPath, mainSHA := writeProfile(mainUUID)
	extensionPath, extensionSHA := writeProfile(extensionUUID)
	certificateDigest := sha256.Sum256(certificate.Raw)

	plan := signingReconcilePlanArtifact{
		SchemaVersion: signingReconcileSchemaV1,
		PlanHash:      strings.Repeat("a", 64),
		Certificate:   &signingCertificateRef{SHA256: hex.EncodeToString(certificateDigest[:])},
		Paths: signingReconcilePaths{
			StateDir: stateDir, ReceiptPath: filepath.Join(stateDir, "receipt.json"), ProfilesDir: filepath.Join(stateDir, "profiles"),
		},
		Targets: []signingTarget{
			{Kind: "application", BundleID: "com.example.app"},
			{Kind: "extension", BundleID: "com.example.app.share"},
		},
		Actions: []signingAction{
			{ID: "main", Kind: actionDownloadProfile, BundleID: "com.example.app", ProfileID: "profile-main"},
			{ID: "extension", Kind: actionCreateProfile, BundleID: "com.example.app.share"},
		},
	}
	receipt := signingReconcileReceipt{
		SchemaVersion: signingReconcileSchemaV1, PlanHash: plan.PlanHash, Complete: true,
		ReceiptPath: plan.Paths.ReceiptPath,
		Actions: []signingActionReceipt{
			{ID: "extension", Kind: actionCreateProfile, Status: "completed", ResourceID: "profile-extension", OutputPath: extensionPath},
			{ID: "main", Kind: actionDownloadProfile, Status: "completed", ResourceID: "profile-main", OutputPath: mainPath},
		},
	}

	view, err := newReconcileReceiptView(plan, receipt)
	if err != nil {
		t.Fatalf("newReconcileReceiptView() error=%v", err)
	}
	if view.MainProfile == nil {
		t.Fatal("MainProfile=nil")
	}
	if got := *view.MainProfile; got.ResourceID != "profile-main" || got.UUID != mainUUID || got.Path != mainPath || got.SHA256 != mainSHA {
		t.Fatalf("MainProfile=%#v", got)
	}
	if len(view.Profiles) != 2 || view.Profiles[1].SHA256 != extensionSHA {
		t.Fatalf("Profiles=%#v", view.Profiles)
	}
	encoded := mustMarshalJSON(t, view)
	for _, secret := range []string{"serialNumber", "deviceFingerprint", "Entitlements"} {
		if strings.Contains(encoded, secret) {
			t.Fatalf("redacted receipt contains %q: %s", secret, encoded)
		}
	}
}

func TestReconcilePlanViewExposesCertificateWithoutProtectedInputs(t *testing.T) {
	plan := signingReconcilePlanArtifact{
		SchemaVersion: signingReconcileSchemaV1, PlanHash: strings.Repeat("a", 64), Ready: true,
		TeamID:          "TEAM1",
		DeviceSetSHA256: "EXPECTED-SEMANTIC-DIGEST",
		Paths:           signingReconcilePaths{PlanPath: "/state/plan.json", ReceiptPath: "/state/receipt.json"},
		Certificate: &signingCertificateRef{
			ID: "cert-id", SHA256: strings.Repeat("b", 64), TeamID: "TEAM1",
			ExpirationDate: "2030-01-01T00:00:00Z", SerialNumber: "SECRET-SERIAL",
		},
		Targets: []signingTarget{{
			Kind: "application", BundleID: "com.example.app", RelativePath: "SECRET-PATH",
			Entitlements: map[string]any{"secret-entitlement": "SECRET-ENTITLEMENT"},
		}},
		Devices: []signingDesiredDevice{
			{Fingerprint: "bbbb", NameSHA256: "SECRET-NAME-HASH", ResourceID: "SECRET-DEVICE-ID"},
			{Fingerprint: "aaaa"},
		},
		Blockers: []string{"device bbbb SECRET-NAME-HASH SECRET-DEVICE-ID blocked"},
		Actions: []signingAction{{
			ID: "SECRET-ACTION-ID", Kind: actionCreateProfile, BundleID: "com.example.app",
			DeviceResourceIDs: []string{"SECRET-DEVICE-ID"},
		}},
	}
	view := newReconcilePlanView(plan)
	if view.Certificate == nil || view.Certificate.ResourceID != "cert-id" || view.Certificate.SHA256 != strings.Repeat("b", 64) || view.Certificate.TeamID != "TEAM1" || view.Certificate.ExpirationDate == "" {
		t.Fatalf("Certificate=%#v", view.Certificate)
	}
	if view.TeamID != "TEAM1" || view.DeviceSetSHA256 != "EXPECTED-SEMANTIC-DIGEST" {
		t.Fatalf("TeamID=%q DeviceSetSHA256=%q", view.TeamID, view.DeviceSetSHA256)
	}
	encoded := mustMarshalJSON(t, view)
	for _, secret := range []string{"SECRET-SERIAL", "SECRET-PATH", "SECRET-ENTITLEMENT", "SECRET-NAME-HASH", "SECRET-DEVICE-ID", "SECRET-ACTION-ID", `"aaaa"`, `"bbbb"`} {
		if strings.Contains(encoded, secret) {
			t.Fatalf("redacted plan contains %q: %s", secret, encoded)
		}
	}
}

func TestExecuteReconcilePlanRequiresCertificateSHA256BeforeAuth(t *testing.T) {
	authCalls := 0
	originalFactory := sharedASCClient
	sharedASCClient = func() (*asc.Client, error) {
		authCalls++
		return nil, errors.New("auth must not be reached")
	}
	t.Cleanup(func() { sharedASCClient = originalFactory })

	_, err := ExecuteReconcilePlan(context.Background(), ReconcilePlanOptions{
		ArchivePath: "App.xcarchive", DevicesFile: "devices.json",
		MinimumValidityDays: 7, MaxMutations: 32, StateDir: t.TempDir(),
	})
	if err == nil || ClassifyReconcileExecutionError(err) != ReconcileExecutionErrorPlanInvalid {
		t.Fatalf("missing certificate fingerprint error=%v", err)
	}
	if authCalls != 0 {
		t.Fatalf("auth calls=%d", authCalls)
	}
}

func TestDigestSigningDeviceInputsUsesSemanticUDIDs(t *testing.T) {
	formatted := digestSigningDeviceInputs([]signingDeviceInput{
		{UDID: "0000-1111:aaaa"}, {UDID: "2222-bbbb"}, {UDID: "00001111AAAA"},
	})
	canonical := digestSigningDeviceInputs([]signingDeviceInput{
		{UDID: "00001111AAAA"}, {UDID: "2222BBBB"},
	})
	different := digestSigningDeviceInputs([]signingDeviceInput{
		{UDID: "00001111AAAA"}, {UDID: "3333CCCC"},
	})
	if formatted.Count != 2 || formatted != canonical {
		t.Fatalf("formatted=%#v canonical=%#v", formatted, canonical)
	}
	if formatted.SHA256 == different.SHA256 {
		t.Fatalf("different sets share digest %q", formatted.SHA256)
	}
}

func certificateExpiryRFC3339(t *testing.T, der []byte) string {
	t.Helper()
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate.NotAfter.UTC().Format(time.RFC3339)
}

func mustMarshalJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestReconcileReceiptViewRejectsProfilePathOutsideState(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "outside.mobileprovision")
	if err := os.WriteFile(outside, []byte("not a profile"), 0o600); err != nil {
		t.Fatal(err)
	}
	stateDir := t.TempDir()
	plan := signingReconcilePlanArtifact{
		PlanHash: strings.Repeat("a", 64),
		Paths:    signingReconcilePaths{StateDir: stateDir, ProfilesDir: filepath.Join(stateDir, "profiles")},
		Targets:  []signingTarget{{Kind: "application", BundleID: "com.example.app"}},
		Actions:  []signingAction{{ID: "main", Kind: actionDownloadProfile, BundleID: "com.example.app", ProfileID: "profile-main"}},
	}
	receipt := signingReconcileReceipt{Complete: true, Actions: []signingActionReceipt{{
		ID: "main", Kind: actionDownloadProfile, Status: "completed", ResourceID: "profile-main", OutputPath: outside,
	}}}
	if _, err := newReconcileReceiptView(plan, receipt); err == nil || !strings.Contains(err.Error(), "profiles directory") {
		t.Fatalf("outside path error=%v", err)
	}
}

func TestReconcileReceiptViewRejectsDownloadResourceIDDifferentFromPlan(t *testing.T) {
	plan := signingReconcilePlanArtifact{
		PlanHash: strings.Repeat("a", 64),
		Targets:  []signingTarget{{Kind: "application", BundleID: "com.example.app"}},
		Actions: []signingAction{{
			ID: "main", Kind: actionDownloadProfile, BundleID: "com.example.app", ProfileID: "profile-planned",
		}},
	}
	receipt := signingReconcileReceipt{Actions: []signingActionReceipt{{
		ID: "main", Kind: actionDownloadProfile, Status: "completed", ResourceID: "profile-other",
	}}}
	if _, err := newReconcileReceiptView(plan, receipt); err == nil || !strings.Contains(err.Error(), "differs from the exact plan") {
		t.Fatalf("mismatched download resource error=%v", err)
	}
}

func TestFetchVerifiedProfileContentRejectsResponseResourceIDDifferentFromRequest(t *testing.T) {
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		if request.Method != http.MethodGet || request.URL.Path != "/v1/profiles/profile-planned" {
			t.Fatalf("request=%s %s", request.Method, request.URL.Path)
		}
		return signingFetchJSONResponse(http.StatusOK, `{
			"data":{"type":"profiles","id":"profile-other","attributes":{
				"profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z"
			}}
		}`)
	})
	plan := signingReconcilePlanArtifact{
		MinimumValidityDays: 7,
		Certificate:         &signingCertificateRef{ID: "cert-1", SHA256: strings.Repeat("a", 64)},
	}
	_, _, err := fetchVerifiedProfileContent(
		context.Background(), client, "profile-planned", plan, signingDevicesFile{}, signingTarget{BundleID: "com.example.app"},
	)
	if err == nil || !strings.Contains(err.Error(), "returned resource ID") {
		t.Fatalf("mismatched profile response error=%v", err)
	}
}

func TestReadProtectedFileRejectsMultipleHardLinks(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "devices.json")
	alias := filepath.Join(directory, "devices-alias.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, alias); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	if _, err := readProtectedFile(path); err == nil || !strings.Contains(err.Error(), "multiple hard links") {
		t.Fatalf("hard-linked protected input error=%v", err)
	}
}
