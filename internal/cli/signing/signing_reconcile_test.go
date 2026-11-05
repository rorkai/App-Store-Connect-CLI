package signing

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"go.mozilla.org/pkcs7"
	"howett.net/plist"
)

func TestSigningReconcilePlanValidatesBeforeAuth(t *testing.T) {
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return nil, errors.New("client must not be created")
	}))

	tests := []struct {
		name      string
		args      []string
		want      string
		wantCode  shared.DiagnosticCode
		wantParam string
	}{
		{name: "archive", args: []string{"--devices-file", "devices.json"}, want: "Error: --archive-path is required", wantCode: shared.DiagnosticRequiredInputMissing, wantParam: "--archive-path"},
		{name: "devices", args: []string{"--archive-path", "App.xcarchive"}, want: "Error: --devices-file is required", wantCode: shared.DiagnosticRequiredInputMissing, wantParam: "--devices-file"},
		{name: "validity", args: []string{"--archive-path", "App.xcarchive", "--devices-file", "devices.json", "--minimum-validity-days", "-1"}, want: "Error: --minimum-validity-days must be at least 0", wantCode: shared.DiagnosticInvalidInput, wantParam: "--minimum-validity-days"},
		{name: "validity max", args: []string{"--archive-path", "App.xcarchive", "--devices-file", "devices.json", "--minimum-validity-days", "3651"}, want: "Error: --minimum-validity-days must be at most 3650", wantCode: shared.DiagnosticInvalidInput, wantParam: "--minimum-validity-days"},
		{name: "mutations", args: []string{"--archive-path", "App.xcarchive", "--devices-file", "devices.json", "--max-mutations", "0"}, want: "Error: --max-mutations must be at least 1", wantCode: shared.DiagnosticInvalidInput, wantParam: "--max-mutations"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := SigningReconcilePlanCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.Parse(test.args); err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				runErr = cmd.Run(context.Background())
				if !errors.Is(runErr, flag.ErrHelp) {
					t.Fatalf("Run() error = %v, want usage error", runErr)
				}
			})
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok || diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("DiagnosticFromError() = %+v, %t; want code %q parameter %q", diagnostic, ok, test.wantCode, test.wantParam)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
		})
	}
}

func TestExecuteSigningReconcilePlanIsReadOnlyAndPlansAdditiveActions(t *testing.T) {
	certificate, _ := newReconcileTestCertificate(t, "Distribution")
	certificateContent := base64.StdEncoding.EncodeToString(certificate.Raw)
	stateDir := t.TempDir()
	devicesPath := filepath.Join(t.TempDir(), "devices.json")
	if err := os.WriteFile(devicesPath, []byte(`{"schemaVersion":1,"devices":[{"name":"Phone","udid":"SECRET-UDID","platform":"IOS"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	originalArchiveReader := readSigningArchiveRequirements
	readSigningArchiveRequirements = func(string) (signingArchiveRequirements, error) {
		return signingArchiveRequirements{TeamID: "TEAM1", Targets: []signingTarget{{
			Kind: "application", RelativePath: "Products/Applications/App.app", BundleID: "com.example.app", Executable: "App",
			Entitlements: map[string]any{"com.apple.developer.team-identifier": "TEAM1"},
		}}}, nil
	}
	t.Cleanup(func() { readSigningArchiveRequirements = originalArchiveReader })

	var methods []string
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		methods = append(methods, request.Method+" "+request.URL.Path)
		switch request.URL.Path {
		case "/v1/devices":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"123","activated":true,"expirationDate":"2100-01-01T00:00:00Z","certificateContent":%q}}]}`, certificateContent))
		case "/v1/bundleIds":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		default:
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
	originalFactory := sharedASCClient
	sharedASCClient = func() (*asc.Client, error) { return client, nil }
	t.Cleanup(func() { sharedASCClient = originalFactory })

	plan, err := executeSigningReconcilePlan(context.Background(), signingReconcilePlanOptions{
		ArchivePath: "App.xcarchive", DevicesFile: devicesPath, MinimumValidityDays: 7,
		MaxMutations: 32, StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("executeSigningReconcilePlan() error = %v", err)
	}
	if !plan.Ready || plan.MutationCount != 3 {
		t.Fatalf("plan ready=%v mutations=%d blockers=%v", plan.Ready, plan.MutationCount, plan.Blockers)
	}
	wantKinds := []string{actionRegisterDevice, actionCreateBundleID, actionCreateProfile}
	for index, kind := range wantKinds {
		if plan.Actions[index].Kind != kind {
			t.Fatalf("actions[%d].Kind=%q, want %q", index, plan.Actions[index].Kind, kind)
		}
	}
	for _, method := range methods {
		if !strings.HasPrefix(method, http.MethodGet+" ") {
			t.Fatalf("planning mutated remote state: %s", method)
		}
	}
	encoded, err := os.ReadFile(filepath.Join(stateDir, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "SECRET-UDID") {
		t.Fatalf("plan leaked raw UDID: %s", encoded)
	}
	if strings.Contains(string(encoded), `"name":"Phone"`) || strings.Contains(string(encoded), `"deviceName"`) {
		t.Fatalf("plan leaked device name: %s", encoded)
	}
}

func TestExecuteSigningReconcilePlanBlocksCertificateFromAnotherTeam(t *testing.T) {
	certificate, _ := newReconcileTestCertificateForTeam(t, "Distribution", "OTHERTEAM")
	certificateContent := base64.StdEncoding.EncodeToString(certificate.Raw)
	devicesPath := filepath.Join(t.TempDir(), "devices.json")
	if err := os.WriteFile(devicesPath, []byte(`{"schemaVersion":1,"devices":[{"name":"Phone","udid":"SECRET-UDID","platform":"IOS"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	originalArchiveReader := readSigningArchiveRequirements
	readSigningArchiveRequirements = func(string) (signingArchiveRequirements, error) {
		return signingArchiveRequirements{TeamID: "TEAM1", Targets: []signingTarget{{
			Kind: "application", BundleID: "com.example.app",
			Entitlements: map[string]any{"com.apple.developer.team-identifier": "TEAM1"},
		}}}, nil
	}
	t.Cleanup(func() { readSigningArchiveRequirements = originalArchiveReader })
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		switch request.URL.Path {
		case "/v1/devices":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","activated":true,"expirationDate":"2100-01-01T00:00:00Z","certificateContent":%q}}]}`, certificateContent))
		case "/v1/bundleIds":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		default:
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
	originalFactory := sharedASCClient
	sharedASCClient = func() (*asc.Client, error) { return client, nil }
	t.Cleanup(func() { sharedASCClient = originalFactory })
	plan, err := executeSigningReconcilePlan(context.Background(), signingReconcilePlanOptions{
		ArchivePath: "App.xcarchive", DevicesFile: devicesPath, MinimumValidityDays: 7,
		MaxMutations: 32, StateDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("executeSigningReconcilePlan() error = %v", err)
	}
	if plan.Ready || !strings.Contains(strings.Join(plan.Blockers, "\n"), "belongs to team OTHERTEAM") {
		t.Fatalf("plan ready=%v blockers=%#v", plan.Ready, plan.Blockers)
	}
}

func TestSelectReconcileCertificateAcceptsLiveMissingActivationAndRequiresValidityWindow(t *testing.T) {
	certificate, _ := newReconcileTestCertificate(t, "Distribution")
	resource := asc.Resource[asc.CertificateAttributes]{
		ID: "cert-1",
		Attributes: asc.CertificateAttributes{
			CertificateType: "IOS_DISTRIBUTION", ExpirationDate: "2100-01-01T00:00:00Z",
			CertificateContent: base64.StdEncoding.EncodeToString(certificate.Raw),
		},
	}
	if selected, blockers := selectReconcileCertificate([]asc.Resource[asc.CertificateAttributes]{resource}, "cert-1", time.Now(), 7); selected == nil || len(blockers) != 0 {
		t.Fatalf("live-shaped certificate without activated selected=%#v blockers=%#v", selected, blockers)
	}
	inactive := false
	resource.Attributes.Activated = &inactive
	if selected, blockers := selectReconcileCertificate([]asc.Resource[asc.CertificateAttributes]{resource}, "cert-1", time.Now(), 7); selected != nil || len(blockers) == 0 {
		t.Fatalf("explicitly inactive certificate selected=%#v blockers=%#v", selected, blockers)
	}
	active := true
	resource.Attributes.Activated = &active
	if selected, blockers := selectReconcileCertificate([]asc.Resource[asc.CertificateAttributes]{resource}, "cert-1", time.Date(2099, 12, 30, 0, 0, 0, 0, time.UTC), 7); selected != nil || len(blockers) == 0 {
		t.Fatalf("certificate shorter than validity window selected=%#v blockers=%#v", selected, blockers)
	}
}

func TestSigningCapabilitiesForEntitlements(t *testing.T) {
	tests := []struct {
		name             string
		entitlements     map[string]any
		wantCapabilities []string
		wantUnverified   []string // substrings, one per unverified entry
	}{
		{
			name:             "live increased memory limit",
			entitlements:     map[string]any{"com.apple.developer.kernel.increased-memory-limit": true},
			wantCapabilities: []string{"INCREASED_MEMORY_LIMIT"},
		},
		{
			name: "type is not proof of entitlement settings",
			entitlements: map[string]any{
				"com.apple.security.application-groups":           []string{"group.com.example.shared"},
				"com.apple.developer.in-app-payments":             []string{"merchant.com.example"},
				"com.apple.developer.networking.networkextension": []string{"packet-tunnel-provider"},
				"com.apple.developer.pass-type-identifiers":       []string{"TEAM1.com.example.pass"},
			},
			wantUnverified: []string{
				"com.apple.developer.in-app-payments (capability settings)",
				"com.apple.developer.networking.networkextension (capability settings)",
				"com.apple.developer.pass-type-identifiers (capability settings)",
				"com.apple.security.application-groups (capability settings)",
			},
		},
		{name: "development task entitlement", entitlements: map[string]any{"get-task-allow": true}, wantUnverified: []string{"get-task-allow"}},
		{name: "false get-task-allow", entitlements: map[string]any{"get-task-allow": false}},
		{name: "development push environment", entitlements: map[string]any{"aps-environment": "development"}, wantUnverified: []string{"aps-environment"}},
		{name: "production push environment", entitlements: map[string]any{"aps-environment": "production"}, wantCapabilities: []string{"PUSH_NOTIFICATIONS"}},
		{name: "TestFlight entitlement", entitlements: map[string]any{"beta-reports-active": true}, wantUnverified: []string{"beta-reports-active"}},
		{name: "false beta reports", entitlements: map[string]any{"beta-reports-active": false}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			capabilities, unverified := signingCapabilitiesForEntitlements(test.entitlements)
			matches := len(capabilities) == len(test.wantCapabilities) && len(unverified) == len(test.wantUnverified)
			for index := 0; matches && index < len(capabilities); index++ {
				matches = capabilities[index] == test.wantCapabilities[index]
			}
			for index := 0; matches && index < len(unverified); index++ {
				matches = strings.Contains(unverified[index], test.wantUnverified[index])
			}
			if !matches {
				t.Fatalf("capabilities=%#v unverified=%#v, want %#v and %#v", capabilities, unverified, test.wantCapabilities, test.wantUnverified)
			}
		})
	}
}

func TestSigningReconcileRequestContextUsesWorkflowTimeout(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "")
	ctx, cancel := signingRequestContext(context.Background())
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("signingRequestContext() has no deadline")
	}
	if remaining := time.Until(deadline); remaining < 10*time.Minute {
		t.Fatalf("signingRequestContext() remaining timeout = %v, want workflow-sized timeout", remaining)
	}
}

func TestPlanSigningTargetBlocksMismatchedAppIDSeedBeforeProfileCreation(t *testing.T) {
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		switch request.URL.Path {
		case "/v1/bundleIds":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app","platform":"IOS","seedId":"OTHERTEAM"}}]}`)
		case "/v1/bundleIds/bundle-1/bundleIdCapabilities", "/v1/bundleIds/bundle-1/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		default:
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
	_, actions, blockers, err := planSigningTarget(
		context.Background(),
		client,
		signingTarget{
			BundleID:    "com.example.app",
			AppIDPrefix: "TEAM1",
			Entitlements: map[string]any{
				"com.apple.developer.team-identifier": "TEAM1",
			},
		},
		[]signingDesiredDevice{{ResourceID: "device-1"}},
		&signingCertificateRef{ID: "cert-1"},
		7,
	)
	if err != nil {
		t.Fatalf("planSigningTarget() error=%v", err)
	}
	if !strings.Contains(strings.Join(blockers, "\n"), "seed ID OTHERTEAM") {
		t.Fatalf("blockers=%#v, want seed-prefix mismatch", blockers)
	}
	for _, action := range actions {
		if action.Kind == actionCreateProfile {
			t.Fatalf("actions=%#v include profile creation despite seed-prefix mismatch", actions)
		}
	}
}

func TestEnsureReconcileProfileRechecksAppIDSeedBeforeMutation(t *testing.T) {
	mutations := 0
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		if request.Method != http.MethodGet {
			mutations++
		}
		switch request.URL.Path {
		case "/v1/bundleIds":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app","platform":"IOS","seedId":"OTHERTEAM"}}]}`)
		default:
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	_, _, err := ensureReconcileProfile(
		context.Background(),
		client,
		signingReconcilePlanArtifact{},
		signingDevicesFile{},
		signingAction{BundleID: "com.example.app"},
		signingTarget{BundleID: "com.example.app", AppIDPrefix: "TEAM1"},
	)
	if err == nil || !strings.Contains(err.Error(), "seed ID OTHERTEAM") {
		t.Fatalf("ensureReconcileProfile() error=%v, want seed-prefix mismatch", err)
	}
	if mutations != 0 {
		t.Fatalf("ensureReconcileProfile() made %d mutations despite seed-prefix mismatch", mutations)
	}
}

func TestPlanSigningTargetBlocksUnverifiableCapabilityValues(t *testing.T) {
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		switch request.URL.Path {
		case "/v1/bundleIds":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app","platform":"IOS","seedId":"TEAM1"}}]}`)
		case "/v1/bundleIds/bundle-1/bundleIdCapabilities":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIdCapabilities","id":"cap-1","attributes":{"capabilityType":"APP_GROUPS"}}]}`)
		case "/v1/bundleIds/bundle-1/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		default:
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})

	_, _, blockers, err := planSigningTarget(
		context.Background(),
		client,
		signingTarget{
			BundleID:    "com.example.app",
			AppIDPrefix: "TEAM1",
			Entitlements: map[string]any{
				"com.apple.developer.team-identifier": "TEAM1",
				"com.apple.security.application-groups": []string{
					"group.com.example.shared",
				},
			},
		},
		[]signingDesiredDevice{{ResourceID: "device-1"}},
		&signingCertificateRef{ID: "cert-1"},
		7,
	)
	if err != nil {
		t.Fatalf("planSigningTarget() error = %v", err)
	}
	got := strings.Join(blockers, "\n")
	if !strings.Contains(got, "com.apple.security.application-groups") || !strings.Contains(got, "cannot be verified safely") {
		t.Fatalf("blockers = %#v, want unverifiable capability values", blockers)
	}
}

func TestExecuteSigningReconcileApplyCreatesAndVerifiesWithoutPatchOrDelete(t *testing.T) {
	distributionCertificate, distributionKey := newReconcileTestCertificate(t, "Distribution")
	certificateContent := base64.StdEncoding.EncodeToString(distributionCertificate.Raw)
	certificateFingerprint := reconcileTestCertificateFingerprint(distributionCertificate)
	stateDir := t.TempDir()
	devicesPath := filepath.Join(t.TempDir(), "devices.json")
	devicesBody := `{"schemaVersion":1,"devices":[{"name":"Phone","udid":"SECRET-UDID","platform":"IOS"}]}`
	if err := os.WriteFile(devicesPath, []byte(devicesBody), 0o600); err != nil {
		t.Fatal(err)
	}
	devices, err := decodeSigningDevicesFile([]byte(devicesBody))
	if err != nil {
		t.Fatal(err)
	}
	target := signingTarget{Kind: "application", RelativePath: "Products/Applications/App.app", BundleID: "com.example.app", Executable: "App", Entitlements: map[string]any{"com.apple.developer.team-identifier": "TEAM1"}}
	originalArchiveReader := readSigningArchiveRequirements
	readSigningArchiveRequirements = func(string) (signingArchiveRequirements, error) {
		return signingArchiveRequirements{TeamID: "TEAM1", Targets: []signingTarget{target}}, nil
	}
	t.Cleanup(func() { readSigningArchiveRequirements = originalArchiveReader })

	profileContentBytes := buildReconcileTestMobileProvision(t, map[string]any{
		"UUID": "00000000-0000-0000-0000-0000000000AD", "ExpirationDate": time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
		"ProvisionedDevices": []string{"SECRET-UDID"}, "Entitlements": map[string]any{"com.apple.developer.team-identifier": "TEAM1"},
		"DeveloperCertificates": [][]byte{distributionCertificate.Raw},
	}, distributionCertificate, distributionKey)
	profileContent := base64.StdEncoding.EncodeToString(profileContentBytes)

	plan := signingReconcilePlanArtifact{
		SchemaVersion: signingReconcileSchemaV1, GeneratedAt: "2026-01-01T00:00:00Z", Command: "signing reconcile plan", Ready: true,
		TeamID: "TEAM1", MinimumValidityDays: 7, MaxMutations: 32, MutationCount: 3,
		DeviceSetSHA256: digestSigningDeviceInputs(devices.Devices).SHA256,
		Paths:           signingReconcilePaths{ArchivePath: "App.xcarchive", DevicesFile: devicesPath, StateDir: stateDir, PlanPath: filepath.Join(stateDir, "plan.json"), ReceiptPath: filepath.Join(stateDir, "receipt.json"), ProfilesDir: filepath.Join(stateDir, "profiles")},
		Certificate:     &signingCertificateRef{ID: "cert-1", CertificateType: "IOS_DISTRIBUTION", SerialNumber: "123", ExpirationDate: "2100-01-01T00:00:00Z", SHA256: certificateFingerprint, TeamID: "TEAM1"},
		Targets:         []signingTarget{target}, Devices: []signingDesiredDevice{{Platform: "IOS", Fingerprint: devices.Devices[0].Fingerprint, NameSHA256: fingerprintReconcileName("Phone")}},
		Actions: []signingAction{
			{ID: "device:" + devices.Devices[0].Fingerprint, Kind: actionRegisterDevice, DeviceFingerprint: devices.Devices[0].Fingerprint, Platform: "IOS"},
			{ID: "bundle:com.example.app", Kind: actionCreateBundleID, BundleID: "com.example.app", Platform: "IOS"},
			{ID: "profile:com.example.app", Kind: actionCreateProfile, BundleID: "com.example.app", CertificateID: "cert-1", ProfileName: "ASC Ad Hoc com.example.app abc"},
		},
	}
	plan.PlanHash, err = hashSigningReconcilePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSigningStateJSON(stateDir, "plan.json", plan, false); err != nil {
		t.Fatal(err)
	}

	deviceExists, bundleExists, profileExists := false, false, false
	var methods []string
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		methods = append(methods, request.Method+" "+request.URL.Path)
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1/certificates":
			return signingFetchJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"123","activated":true,"expirationDate":"2100-01-01T00:00:00Z","certificateContent":%q}}]}`, certificateContent))
		case request.Method == http.MethodGet && request.URL.Path == "/v1/devices":
			if deviceExists {
				return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-1","attributes":{"name":"Phone","udid":"SECRET-UDID","platform":"IOS","status":"ENABLED"}}]}`)
			}
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v1/devices":
			var payload asc.DeviceCreateRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode device create request: %v", err)
			}
			if payload.Data.Attributes.Name != "Phone" || payload.Data.Attributes.UDID != "SECRET-UDID" || payload.Data.Attributes.Platform != asc.DevicePlatformIOS {
				t.Errorf("device create payload = %#v", payload)
			}
			deviceExists = true
			return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"devices","id":"device-1","attributes":{"name":"Phone","udid":"SECRET-UDID","platform":"IOS","status":"ENABLED"}}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v1/bundleIds":
			if bundleExists {
				return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"name":"App","identifier":"com.example.app","platform":"IOS"}}]}`)
			}
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v1/bundleIds":
			var payload asc.BundleIDCreateRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode bundle ID create request: %v", err)
			}
			if payload.Data.Attributes.Identifier != "com.example.app" || payload.Data.Attributes.Platform != asc.BundleIDPlatformIOS {
				t.Errorf("bundle ID create payload = %#v", payload)
			}
			bundleExists = true
			return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"bundleIds","id":"bundle-1","attributes":{"name":"App","identifier":"com.example.app","platform":"IOS"}}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v1/bundleIds/bundle-1/profiles":
			if profileExists {
				return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"profile-1","attributes":{"name":"ASC Ad Hoc com.example.app abc","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z"}}]}`)
			}
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v1/bundleIds/bundle-1/bundleIdCapabilities":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v1/profiles":
			var payload asc.ProfileCreateRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode profile create request: %v", err)
			}
			if payload.Data.Attributes.ProfileType != reconcileProfileType || payload.Data.Relationships.BundleID.Data.ID != "bundle-1" {
				t.Errorf("profile create payload = %#v", payload)
			}
			if got := payload.Data.Relationships.Certificates.Data; len(got) != 1 || got[0].ID != "cert-1" {
				t.Errorf("profile certificates = %#v", got)
			}
			if got := payload.Data.Relationships.Devices.Data; len(got) != 1 || got[0].ID != "device-1" {
				t.Errorf("profile devices = %#v", got)
			}
			profileExists = true
			return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"profiles","id":"profile-1","attributes":{"name":"ASC Ad Hoc com.example.app abc","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE"}}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v1/profiles/profile-1":
			return signingFetchJSONResponse(http.StatusOK, `{"data":{"type":"profiles","id":"profile-1","attributes":{"name":"ASC Ad Hoc com.example.app abc","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","uuid":"00000000-0000-0000-0000-0000000000AD","expirationDate":"2100-01-01T00:00:00Z","profileContent":"`+profileContent+`"}}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v1/profiles/profile-1/certificates":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{}}]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v1/profiles/profile-1/devices":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-1","attributes":{"name":"Phone","udid":"SECRET-UDID","platform":"IOS","status":"ENABLED"}}]}`)
		default:
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))
	receipt, err := executeSigningReconcileApply(context.Background(), filepath.Join(stateDir, "plan.json"))
	if err != nil {
		t.Fatalf("executeSigningReconcileApply() error=%v", err)
	}
	if !receipt.Complete || len(receipt.Actions) != 3 {
		t.Fatalf("receipt=%#v", receipt)
	}
	for _, method := range methods {
		if strings.HasPrefix(method, http.MethodPatch+" ") || strings.HasPrefix(method, http.MethodDelete+" ") {
			t.Fatalf("forbidden mutation: %s", method)
		}
	}
	if _, err := os.Stat(filepath.Join(stateDir, "profiles", "00000000-0000-0000-0000-0000000000AD.mobileprovision")); err != nil {
		t.Fatal(err)
	}
	posts := 0
	for _, method := range methods {
		if strings.HasPrefix(method, http.MethodPost+" ") {
			posts++
		}
	}
	second, err := executeSigningReconcileApply(context.Background(), filepath.Join(stateDir, "plan.json"))
	if err != nil || !second.Complete {
		t.Fatalf("resume receipt=%#v error=%v", second, err)
	}
	postsAfterResume := 0
	for _, method := range methods {
		if strings.HasPrefix(method, http.MethodPost+" ") {
			postsAfterResume++
		}
	}
	if postsAfterResume != posts {
		t.Fatalf("resume repeated mutations: before=%d after=%d", posts, postsAfterResume)
	}
	receiptBytes, err := os.ReadFile(filepath.Join(stateDir, "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(receiptBytes), "SECRET-UDID") {
		t.Fatalf("receipt leaked raw UDID: %s", receiptBytes)
	}
}

func TestGetProfileCandidatesPaginatesAndRejectsExtraDevices(t *testing.T) {
	distributionCertificate, distributionKey := newReconcileTestCertificate(t, "Distribution")
	profileBytes := buildReconcileTestMobileProvision(t, map[string]any{
		"UUID": "00000000-0000-0000-0000-0000000000AD", "ExpirationDate": time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
		"ProvisionedDevices": []string{"SECRET-UDID"}, "Entitlements": map[string]any{"com.apple.developer.team-identifier": "TEAM1"},
		"DeveloperCertificates": [][]byte{distributionCertificate.Raw},
	}, distributionCertificate, distributionKey)
	content := base64.StdEncoding.EncodeToString(profileBytes)

	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		switch {
		case request.URL.Path == "/v1/bundleIds/bundle-1/profiles" && request.URL.Query().Get("cursor") == "":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"p-extra","attributes":{"name":"Extra","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/bundleIds/bundle-1/profiles?cursor=2"}}`)
		case request.URL.Path == "/v1/bundleIds/bundle-1/profiles" && request.URL.Query().Get("cursor") == "2":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"p-exact","attributes":{"name":"Exact","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","expirationDate":"2099-01-01T00:00:00Z"}}]}`)
		case strings.HasSuffix(request.URL.Path, "/certificates"):
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{}}]}`)
		case request.URL.Path == "/v1/profiles/p-extra/devices" && request.URL.Query().Get("cursor") == "":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-1","attributes":{}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/profiles/p-extra/devices?cursor=2"}}`)
		case request.URL.Path == "/v1/profiles/p-extra/devices" && request.URL.Query().Get("cursor") == "2":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-2","attributes":{}}]}`)
		case request.URL.Path == "/v1/profiles/p-exact/devices":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-1","attributes":{}}]}`)
		case request.URL.Path == "/v1/profiles/p-extra" || request.URL.Path == "/v1/profiles/p-exact":
			id := strings.TrimPrefix(request.URL.Path, "/v1/profiles/")
			return signingFetchJSONResponse(http.StatusOK, `{"data":{"type":"profiles","id":"`+id+`","attributes":{"name":"Profile","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","uuid":"00000000-0000-0000-0000-0000000000AD","expirationDate":"2100-01-01T00:00:00Z","profileContent":"`+content+`"}}}`)
		default:
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
	candidates, err := getProfileCandidates(
		context.Background(), client,
		asc.Resource[asc.BundleIDAttributes]{ID: "bundle-1", Attributes: asc.BundleIDAttributes{Identifier: "com.example.app", Platform: asc.BundleIDPlatformIOS}},
		signingTarget{BundleID: "com.example.app", Entitlements: map[string]any{"com.apple.developer.team-identifier": "TEAM1"}},
		[]signingDesiredDevice{{Fingerprint: fingerprintDevice(normalizeReconcileUDID("SECRET-UDID")), ResourceID: "device-1", Status: "ENABLED"}},
		signingCertificateRef{ID: "cert-1", SHA256: reconcileTestCertificateFingerprint(distributionCertificate)}, 7,
	)
	if err != nil {
		t.Fatalf("getProfileCandidates() error=%v", err)
	}
	if len(candidates) != 2 || !candidates[0].Suitable || candidates[0].Profile.ID != "p-exact" || candidates[0].ExtraDevices != 0 {
		t.Fatalf("candidates=%#v", candidates)
	}
	if candidates[1].Profile.ID != "p-extra" || candidates[1].Suitable {
		t.Fatalf("extra-device candidate was considered suitable: %#v", candidates[1])
	}
}

func TestGetAllReconcileDevicesPaginatesWithoutDuplicatingExistingDevice(t *testing.T) {
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		switch request.URL.Query().Get("cursor") {
		case "":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-1","attributes":{"name":"Phone","udid":"SECRET-UDID","platform":"IOS","status":"ENABLED"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/devices?cursor=2"}}`)
		case "2":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-2","attributes":{"name":"Other","udid":"OTHER-UDID","platform":"IOS","status":"ENABLED"}}]}`)
		default:
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
	devices, err := getAllReconcileDevices(context.Background(), client)
	if err != nil {
		t.Fatalf("getAllReconcileDevices() error = %v", err)
	}
	if len(devices) != 2 || devices[0].ID != "device-1" || devices[1].ID != "device-2" {
		t.Fatalf("devices = %#v", devices)
	}
	input, err := decodeSigningDevicesFile([]byte(`{"schemaVersion":1,"devices":[{"name":"Phone","udid":"SECRET-UDID","platform":"IOS"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	resolved, actions, blockers := planDesiredDevices(input.Devices, devices)
	if len(blockers) != 0 || len(actions) != 0 || len(resolved) != 1 || resolved[0].ResourceID != "device-1" {
		t.Fatalf("resolved=%#v actions=%#v blockers=%#v", resolved, actions, blockers)
	}
}

func TestGetAllBundleIDCapabilitiesUsesLiveUnparameterizedFirstPage(t *testing.T) {
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		if request.URL.Query().Get("cursor") == "" {
			if request.URL.RawQuery != "" {
				t.Fatalf("first capability request query = %q, want empty", request.URL.RawQuery)
			}
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIdCapabilities","id":"cap-1","attributes":{"capabilityType":"PUSH_NOTIFICATIONS"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/bundleIds/bundle-1/bundleIdCapabilities?cursor=2"}}`)
		}
		return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIdCapabilities","id":"cap-2","attributes":{"capabilityType":"APP_GROUPS"}}]}`)
	})
	capabilities, err := getAllBundleIDCapabilities(context.Background(), client, "bundle-1")
	if err != nil {
		t.Fatalf("getAllBundleIDCapabilities() error = %v", err)
	}
	if len(capabilities) != 2 || capabilities[0].ID != "cap-1" || capabilities[1].ID != "cap-2" {
		t.Fatalf("capabilities = %#v", capabilities)
	}
}

func TestPreflightSigningApplyFindsLaterTargetBlockerBeforeMutation(t *testing.T) {
	devices, err := decodeSigningDevicesFile([]byte(`{"schemaVersion":1,"devices":[{"name":"Phone","udid":"SECRET-UDID","platform":"IOS"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	certificate, _ := newReconcileTestCertificate(t, "Distribution")
	plan := signingReconcilePlanArtifact{
		Certificate:         &signingCertificateRef{ID: "cert-1", SHA256: reconcileTestCertificateFingerprint(certificate)},
		MinimumValidityDays: 7,
		Targets: []signingTarget{
			{BundleID: "com.example.one", Entitlements: map[string]any{"com.apple.developer.team-identifier": "TEAM1"}},
			{BundleID: "com.example.two", Entitlements: map[string]any{"com.apple.developer.team-identifier": "TEAM1"}},
		},
		Actions: []signingAction{
			{ID: "profile:com.example.one", Kind: actionCreateProfile},
			{ID: "profile:com.example.two", Kind: actionCreateProfile},
		},
	}
	var methods []string
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		methods = append(methods, request.Method)
		switch request.URL.Path {
		case "/v1/devices":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-1","attributes":{"name":"Phone","udid":"SECRET-UDID","platform":"IOS","status":"ENABLED"}}]}`)
		case "/v1/bundleIds":
			identifier := request.URL.Query().Get("filter[identifier]")
			platform := "IOS"
			id := "bundle-one"
			if identifier == "com.example.two" {
				platform = "MAC_OS"
				id = "bundle-two"
			}
			return signingFetchJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"bundleIds","id":%q,"attributes":{"identifier":%q,"platform":%q}}]}`, id, identifier, platform))
		case "/v1/bundleIds/bundle-one/bundleIdCapabilities", "/v1/bundleIds/bundle-two/bundleIdCapabilities":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case "/v1/bundleIds/bundle-one/profiles", "/v1/bundleIds/bundle-two/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		default:
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
	err = preflightSigningApply(context.Background(), client, plan, devices)
	if err == nil || !strings.Contains(err.Error(), "incompatible platform") {
		t.Fatalf("preflightSigningApply() error = %v", err)
	}
	for _, method := range methods {
		if method != http.MethodGet {
			t.Fatalf("preflight sent %s", method)
		}
	}
}

func TestEnsureReconcileDeviceConvergesAfterAmbiguousPOST(t *testing.T) {
	input, err := decodeSigningDevicesFile([]byte(`{"schemaVersion":1,"devices":[{"name":"Phone","udid":"SECRET-UDID","platform":"IOS"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	created := false
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		if request.Method == http.MethodPost {
			created = true
			return signingFetchJSONResponse(http.StatusInternalServerError, `{"errors":[{"detail":"response lost"}]}`)
		}
		if created {
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-1","attributes":{"name":"Phone","udid":"SECRET-UDID","platform":"IOS","status":"ENABLED"}}]}`)
		}
		return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
	})
	device, err := ensureReconcileDevice(context.Background(), client, input.Devices[0])
	if err != nil || device.ID != "device-1" {
		t.Fatalf("ensureReconcileDevice() device=%#v error=%v", device, err)
	}
}

func TestEnsureReconcileBundleIDConvergesAfterAmbiguousPOST(t *testing.T) {
	created := false
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		if request.Method == http.MethodPost {
			created = true
			return signingFetchJSONResponse(http.StatusInternalServerError, `{"errors":[{"detail":"response lost"}]}`)
		}
		if created {
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app","platform":"IOS"}}]}`)
		}
		return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
	})
	bundle, err := ensureReconcileBundleID(context.Background(), client, "com.example.app")
	if err != nil || bundle.ID != "bundle-1" {
		t.Fatalf("ensureReconcileBundleID() bundle=%#v error=%v", bundle, err)
	}
}

func TestApplyRejectsConcurrentBundleSeedMismatchBeforeProfilePOST(t *testing.T) {
	bundleLookups := 0
	profilePosts := 0
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1/bundleIds":
			bundleLookups++
			if bundleLookups == 1 {
				return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app","platform":"IOS","seedId":"OTHERTEAM"}}]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v1/bundleIds":
			return signingFetchJSONResponse(http.StatusConflict, `{"errors":[{"detail":"created concurrently"}]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v1/devices":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v1/bundleIds/bundle-1/profiles":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v1/profiles":
			profilePosts++
			return signingFetchJSONResponse(http.StatusCreated, `{"data":{"type":"profiles","id":"profile-1","attributes":{}}}`)
		default:
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
	target := signingTarget{
		BundleID:    "com.example.app",
		AppIDPrefix: "TEAM1",
		Entitlements: map[string]any{
			"com.apple.developer.team-identifier": "TEAM1",
		},
	}
	plan := signingReconcilePlanArtifact{
		Certificate: &signingCertificateRef{ID: "cert-1"},
		Targets:     []signingTarget{target},
	}

	_, _, err := applySigningAction(
		context.Background(),
		client,
		plan,
		signingDevicesFile{},
		signingAction{Kind: actionCreateBundleID, BundleID: target.BundleID},
		map[string]string{},
	)
	if err == nil || !strings.Contains(err.Error(), "seed ID OTHERTEAM") {
		t.Fatalf("create Bundle ID action error = %v, want seed mismatch", err)
	}
	if profilePosts != 0 {
		t.Fatalf("profile POST count = %d, want 0", profilePosts)
	}
}

func TestEnsureReconcileProfileConvergesAfterAmbiguousPOSTOnlyWhenExact(t *testing.T) {
	certificate, key := newReconcileTestCertificate(t, "Distribution")
	profileBytes := buildReconcileTestMobileProvision(t, map[string]any{
		"UUID": "00000000-0000-0000-0000-0000000000AD", "ExpirationDate": time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
		"ProvisionedDevices": []string{"SECRET-UDID"}, "Entitlements": map[string]any{"com.apple.developer.team-identifier": "TEAM1"},
		"DeveloperCertificates": [][]byte{certificate.Raw},
	}, certificate, key)
	content := base64.StdEncoding.EncodeToString(profileBytes)
	devices, err := decodeSigningDevicesFile([]byte(`{"schemaVersion":1,"devices":[{"name":"Phone","udid":"SECRET-UDID","platform":"IOS"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	plan := signingReconcilePlanArtifact{
		Certificate:         &signingCertificateRef{ID: "cert-1", SHA256: reconcileTestCertificateFingerprint(certificate)},
		MinimumValidityDays: 7,
	}
	target := signingTarget{BundleID: "com.example.app", Entitlements: map[string]any{"com.apple.developer.team-identifier": "TEAM1"}}
	action := signingAction{BundleID: target.BundleID, ProfileName: "ASC Ad Hoc exact"}
	created := false
	client := newSigningFetchTestClient(t, func(request *http.Request) *http.Response {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v1/profiles":
			created = true
			return signingFetchJSONResponse(http.StatusConflict, `{"errors":[{"detail":"response lost"}]}`)
		case request.URL.Path == "/v1/bundleIds":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app","platform":"IOS"}}]}`)
		case request.URL.Path == "/v1/devices":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-1","attributes":{"name":"Phone","udid":"SECRET-UDID","platform":"IOS","status":"ENABLED"}}]}`)
		case request.URL.Path == "/v1/bundleIds/bundle-1/profiles":
			if created {
				return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"profile-1","attributes":{"name":"ASC Ad Hoc exact","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z"}}]}`)
			}
			return signingFetchJSONResponse(http.StatusOK, `{"data":[]}`)
		case request.URL.Path == "/v1/profiles/profile-1":
			return signingFetchJSONResponse(http.StatusOK, `{"data":{"type":"profiles","id":"profile-1","attributes":{"name":"ASC Ad Hoc exact","profileType":"IOS_APP_ADHOC","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z","profileContent":"`+content+`"}}}`)
		case request.URL.Path == "/v1/profiles/profile-1/certificates":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"cert-1","attributes":{}}]}`)
		case request.URL.Path == "/v1/profiles/profile-1/devices":
			return signingFetchJSONResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-1","attributes":{}}]}`)
		default:
			return signingFetchJSONResponse(http.StatusInternalServerError, `{}`)
		}
	})
	profile, gotContent, err := ensureReconcileProfile(context.Background(), client, plan, devices, action, target)
	if err != nil || profile.ID != "profile-1" || string(gotContent) != string(profileBytes) {
		t.Fatalf("ensureReconcileProfile() profile=%#v content=%d error=%v", profile, len(gotContent), err)
	}
}

func TestDecodeReconcileMobileProvisionRejectsForgedCMS(t *testing.T) {
	certificate, key := newReconcileTestCertificate(t, "Signer")
	signed := buildReconcileTestMobileProvision(t, map[string]any{
		"UUID": "00000000-0000-0000-0000-0000000000AD",
	}, certificate, key)
	forged := append([]byte(nil), signed...)
	forged[len(forged)-1] ^= 0xff
	if _, err := decodeReconcileMobileProvision(forged); err == nil {
		t.Fatal("decodeReconcileMobileProvision() accepted forged CMS")
	}

	raw, err := plist.Marshal(map[string]any{"UUID": "00000000-0000-0000-0000-0000000000AD"}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeReconcileMobileProvision(raw); err == nil {
		t.Fatal("decodeReconcileMobileProvision() accepted unsigned plist")
	}
}

func TestProfileContentMatchesTargetRejectsWrongEmbeddedCertificate(t *testing.T) {
	selectedCertificate, _ := newReconcileTestCertificate(t, "Selected")
	otherCertificate, otherKey := newReconcileTestCertificate(t, "Other")
	profileBytes := buildReconcileTestMobileProvision(t, map[string]any{
		"UUID":                  "00000000-0000-0000-0000-0000000000AD",
		"ProvisionedDevices":    []string{"SECRET-UDID"},
		"Entitlements":          map[string]any{"com.apple.developer.team-identifier": "TEAM1"},
		"DeveloperCertificates": [][]byte{otherCertificate.Raw},
	}, otherCertificate, otherKey)
	if profileContentMatchesTarget(
		base64.StdEncoding.EncodeToString(profileBytes),
		signingTarget{Entitlements: map[string]any{"com.apple.developer.team-identifier": "TEAM1"}},
		[]signingDesiredDevice{{Fingerprint: fingerprintDevice(normalizeReconcileUDID("SECRET-UDID")), ResourceID: "device-1"}},
		reconcileTestCertificateFingerprint(selectedCertificate),
		time.Time{},
	) {
		t.Fatal("profileContentMatchesTarget() accepted the wrong embedded certificate")
	}
}

func TestProfileContentMatchesTargetRejectsWrongDeviceAndExpiredContent(t *testing.T) {
	certificate, key := newReconcileTestCertificate(t, "Distribution")
	build := func(udid string, expires time.Time) string {
		return base64.StdEncoding.EncodeToString(buildReconcileTestMobileProvision(t, map[string]any{
			"UUID":                  "00000000-0000-0000-0000-0000000000AD",
			"ExpirationDate":        expires,
			"ProvisionedDevices":    []string{udid},
			"Entitlements":          map[string]any{"com.apple.developer.team-identifier": "TEAM1"},
			"DeveloperCertificates": [][]byte{certificate.Raw},
		}, certificate, key))
	}
	desired := []signingDesiredDevice{{Fingerprint: fingerprintDevice(normalizeReconcileUDID("SECRET-UDID"))}}
	target := signingTarget{Entitlements: map[string]any{"com.apple.developer.team-identifier": "TEAM1"}}
	fingerprint := reconcileTestCertificateFingerprint(certificate)
	if profileContentMatchesTarget(build("OTHER-UDID", time.Now().Add(30*24*time.Hour)), target, desired, fingerprint, time.Now()) {
		t.Fatal("profile with different embedded UDID was accepted")
	}
	if profileContentMatchesTarget(build("SECRET-UDID", time.Now().Add(-time.Hour)), target, desired, fingerprint, time.Now()) {
		t.Fatal("profile with expired embedded content was accepted")
	}
}

func TestWriteVerifiedProfileRejectsDifferentContentForExistingUUID(t *testing.T) {
	certificate, key := newReconcileTestCertificate(t, "Distribution")
	stateDir := t.TempDir()
	first := buildReconcileTestMobileProvision(t, map[string]any{
		"UUID": "00000000-0000-0000-0000-0000000000AD",
		"Name": "First",
	}, certificate, key)
	path, err := writeVerifiedProfile(stateDir, first)
	if err != nil {
		t.Fatalf("writeVerifiedProfile(first) error = %v", err)
	}
	if _, err := writeVerifiedProfile(stateDir, first); err != nil {
		t.Fatalf("writeVerifiedProfile(identical) error = %v", err)
	}
	second := buildReconcileTestMobileProvision(t, map[string]any{
		"UUID": "00000000-0000-0000-0000-0000000000AD",
		"Name": "Second",
	}, certificate, key)
	if _, err := writeVerifiedProfile(stateDir, second); err == nil {
		t.Fatal("writeVerifiedProfile() overwrote different content with the same UUID")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(first) {
		t.Fatal("existing profile content changed")
	}
}

func TestPrepareReconcileProfileOutputRejectsUnusablePath(t *testing.T) {
	stateDir := t.TempDir()
	profilesPath := filepath.Join(stateDir, "profiles")
	if err := os.WriteFile(profilesPath, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareReconcileProfileOutput(stateDir); err == nil {
		t.Fatal("prepareReconcileProfileOutput() accepted a non-directory profiles path")
	}
	if err := os.Remove(profilesPath); err != nil {
		t.Fatal(err)
	}
	if err := prepareReconcileProfileOutput(stateDir); err != nil {
		t.Fatalf("prepareReconcileProfileOutput() error = %v", err)
	}
	info, err := os.Stat(profilesPath)
	if err != nil || !info.IsDir() {
		t.Fatalf("profiles directory info=%v error=%v", info, err)
	}
	entries, err := os.ReadDir(profilesPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("profile writability probe left files behind: %v", entries)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(profilesPath, 0o500); err != nil {
			t.Fatal(err)
		}
		if err := prepareReconcileProfileOutput(stateDir); err == nil {
			t.Fatal("prepareReconcileProfileOutput() accepted an unwritable profiles directory")
		}
		if err := os.Chmod(profilesPath, 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSigningReconcileApplyRequiresConfirmBeforeReadingPlan(t *testing.T) {
	cmd := SigningReconcileApplyCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{"--plan", filepath.Join(t.TempDir(), "missing.json")}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		err := cmd.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("Run() error = %v, want usage error", err)
		}
	})
	if stdout != "" || !strings.Contains(stderr, "Error: --confirm is required") {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestSigningReconcilePlatformRequiresDarwin(t *testing.T) {
	if err := validateSigningReconcilePlatform("darwin"); err != nil {
		t.Fatalf("validateSigningReconcilePlatform(darwin) error = %v", err)
	}
	for _, goos := range []string{"linux", "windows"} {
		err := validateSigningReconcilePlatform(goos)
		if !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "macOS") {
			t.Fatalf("validateSigningReconcilePlatform(%q) error = %v, want macOS usage error", goos, err)
		}
	}
}

func TestReadSigningPlanArtifactClassifiesMalformedJSONAsUsage(t *testing.T) {
	for name, body := range map[string]string{
		"truncated":     `{"schemaVersion":1`,
		"unknown field": `{"schemaVersion":1,"unexpected":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "plan.json")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := readSigningPlanArtifact(path)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("readSigningPlanArtifact() error = %v, want usage classification", err)
			}
		})
	}
}

func TestSigningReconcilePlanClassifiesInvalidDevicesFileAsUsageErrorBeforeSideEffects(t *testing.T) {
	devicesPath := filepath.Join(t.TempDir(), "devices.json")
	if err := os.WriteFile(devicesPath, []byte(`{"schemaVersion":1,"devices":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(t.TempDir(), "signing-state")
	originalArchiveReader := readSigningArchiveRequirements
	readSigningArchiveRequirements = func(string) (signingArchiveRequirements, error) {
		t.Fatal("archive must not be read for an invalid devices file")
		return signingArchiveRequirements{}, nil
	}
	t.Cleanup(func() { readSigningArchiveRequirements = originalArchiveReader })
	originalFactory := sharedASCClient
	sharedASCClient = func() (*asc.Client, error) {
		t.Fatal("ASC client must not be created for an invalid devices file")
		return nil, nil
	}
	t.Cleanup(func() { sharedASCClient = originalFactory })

	cmd := SigningReconcilePlanCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{
		"--archive-path", "App.xcarchive",
		"--devices-file", devicesPath,
		"--state-dir", stateDir,
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		err := cmd.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("Run() error = %v, want usage error", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "contains invalid JSON") {
		t.Fatalf("stderr = %q, want devices validation error", stderr)
	}
	if _, err := os.Stat(stateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("state directory exists after invalid input: %v", err)
	}
}

func TestSigningReconcilePlanRejectsDevicesPlanPathCollisionBeforeSideEffects(t *testing.T) {
	stateDir := t.TempDir()
	devicesPath := filepath.Join(stateDir, "plan.json")
	originalDevices := []byte(`{"schemaVersion":1,"devices":[{"name":"Phone","udid":"ABC123","platform":"IOS"}]}`)
	if err := os.WriteFile(devicesPath, originalDevices, 0o600); err != nil {
		t.Fatal(err)
	}
	originalArchiveReader := readSigningArchiveRequirements
	readSigningArchiveRequirements = func(string) (signingArchiveRequirements, error) {
		t.Fatal("archive must not be read when devices and plan paths collide")
		return signingArchiveRequirements{}, nil
	}
	t.Cleanup(func() { readSigningArchiveRequirements = originalArchiveReader })

	_, err := executeSigningReconcilePlan(context.Background(), signingReconcilePlanOptions{
		ArchivePath: "App.xcarchive",
		DevicesFile: devicesPath,
		StateDir:    stateDir,
		Overwrite:   true,
	})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("executeSigningReconcilePlan() error = %v, want usage error", err)
	}
	got, readErr := os.ReadFile(devicesPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, originalDevices) {
		t.Fatalf("devices file was overwritten: got %q", got)
	}
}

func TestDecodeSigningDevicesFileStrict(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "unknown", body: `{"schemaVersion":1,"devices":[{"name":"Phone","udid":"ABC","platform":"IOS","extra":true}]}`, want: "unknown field"},
		{name: "version", body: `{"schemaVersion":2,"devices":[{"name":"Phone","udid":"ABC","platform":"IOS"}]}`, want: "schemaVersion must be 1"},
		{name: "empty", body: `{"schemaVersion":1,"devices":[]}`, want: "devices must contain at least one device"},
		{name: "platform", body: `{"schemaVersion":1,"devices":[{"name":"Phone","udid":"ABC","platform":"MAC_OS"}]}`, want: "platform must be IOS"},
		{name: "duplicate", body: `{"schemaVersion":1,"devices":[{"name":"One","udid":"00-aa-11-bb","platform":"IOS"},{"name":"Two","udid":"00aa11bb","platform":"IOS"}]}`, want: "duplicate device UDID"},
		{name: "control name", body: "{\"schemaVersion\":1,\"devices\":[{\"name\":\"Phone\\nInjected\",\"udid\":\"ABCD1234\",\"platform\":\"IOS\"}]}", want: "without control or bidi"},
		{name: "bidi name", body: "{\"schemaVersion\":1,\"devices\":[{\"name\":\"Phone\\u202e\",\"udid\":\"ABCD1234\",\"platform\":\"IOS\"}]}", want: "without control or bidi"},
		{name: "udid punctuation", body: `{"schemaVersion":1,"devices":[{"name":"Phone","udid":"ABC/DEF?123","platform":"IOS"}]}`, want: "udid has invalid format"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeSigningDevicesFile([]byte(test.body))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}

	got, err := decodeSigningDevicesFile([]byte(`{"schemaVersion":1,"devices":[{"name":" Phone ","udid":"00-aa-11-bb","platform":"ios"}]}`))
	if err != nil {
		t.Fatalf("decodeSigningDevicesFile() error = %v", err)
	}
	if got.Devices[0].Name != "Phone" || got.Devices[0].Platform != "IOS" || got.Devices[0].Fingerprint == "" {
		t.Fatalf("decoded = %#v", got)
	}
}

func TestSigningReconcilePlanClassifiesInvalidDevicesAsUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"schemaVersion":1`},
		{name: "unknown field", body: `{"schemaVersion":1,"devices":[{"name":"Phone","udid":"ABCD1234","platform":"IOS","extra":true}]}`},
		{name: "invalid UDID", body: `{"schemaVersion":1,"devices":[{"name":"Phone","udid":"ABC/DEF","platform":"IOS"}]}`},
		{name: "duplicate device", body: `{"schemaVersion":1,"devices":[{"name":"One","udid":"00-aa-11-bb","platform":"IOS"},{"name":"Two","udid":"00aa11bb","platform":"IOS"}]}`},
		{name: "unsupported platform", body: `{"schemaVersion":1,"devices":[{"name":"Phone","udid":"ABCD1234","platform":"MAC_OS"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			devicesPath := filepath.Join(t.TempDir(), "devices.json")
			if err := os.WriteFile(devicesPath, []byte(test.body), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := SigningReconcilePlanCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.Parse([]string{
				"--archive-path", filepath.Join(t.TempDir(), "App.xcarchive"),
				"--devices-file", devicesPath,
				"--state-dir", t.TempDir(),
			}); err != nil {
				t.Fatalf("Parse() error=%v", err)
			}
			_, _ = captureOutput(t, func() {
				err := cmd.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("Run() error=%v, want usage classification", err)
				}
			})
		})
	}
}

func TestReadProtectedFileRequiresPrivateBoundedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readProtectedFile(path); runtime.GOOS != "windows" && (err == nil || !strings.Contains(err.Error(), "0600 or stricter")) {
		t.Fatalf("readProtectedFile() error = %v", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), reconcileProtectedFileMaxBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readProtectedFile(path); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("readProtectedFile(oversized) error = %v", err)
	}
}

func TestProtectedDevicesAndPlanInputsRejectSymlinkedParents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"devices.json", "plan.json"} {
		if err := os.WriteFile(filepath.Join(realDir, name), []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(realDir, filepath.Join(dir, "linked")); err != nil {
		t.Fatal(err)
	}

	if _, err := readProtectedFile(filepath.Join(dir, "linked", "devices.json")); !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("readProtectedFile() error = %v, want rootfs.ErrSymlink", err)
	}
	if _, err := readSigningPlanArtifact(filepath.Join(dir, "linked", "plan.json")); !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("readSigningPlanArtifact() error = %v, want rootfs.ErrSymlink", err)
	}
}

func TestSanitizeReconcileErrorRedactsDeviceSecrets(t *testing.T) {
	devices, err := decodeSigningDevicesFile([]byte(`{"schemaVersion":1,"devices":[{"name":"Rudrank Phone","udid":"SECRET-UDID","platform":"IOS"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	sanitized := sanitizeReconcileError(shared.UsageErrorf("API rejected SECRET-UDID for Rudrank Phone, Rudrank+Phone, Rudrank%%20Phone, and SECRETUDID"), devices)
	if !errors.Is(sanitized, flag.ErrHelp) {
		t.Fatalf("sanitized error lost usage classification: %v", sanitized)
	}
	got := sanitized.Error()
	for _, secret := range []string{"SECRET-UDID", "SECRETUDID", "Rudrank Phone", "Rudrank+Phone", "Rudrank%20Phone"} {
		if strings.Contains(got, secret) {
			t.Fatalf("sanitized error leaked %q: %s", secret, got)
		}
	}
}

func TestPreflightSigningApplySanitizesRemoteDeviceErrors(t *testing.T) {
	devices, err := decodeSigningDevicesFile([]byte(`{"schemaVersion":1,"devices":[{"name":"Rudrank Phone","udid":"SECRET-UDID","platform":"IOS"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	client := newSigningFetchTestClient(t, func(*http.Request) *http.Response {
		return signingFetchJSONResponse(http.StatusBadRequest, `{"errors":[{"detail":"Rudrank Phone Rudrank+Phone Rudrank%20Phone SECRET-UDID SECRETUDID"}]}`)
	})

	err = preflightSigningApplySanitized(context.Background(), client, signingReconcilePlanArtifact{}, devices)
	if err == nil {
		t.Fatal("preflightSigningApplySanitized() error = nil")
	}
	for _, secret := range []string{"Rudrank Phone", "Rudrank+Phone", "Rudrank%20Phone", "SECRET-UDID", "SECRETUDID"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("preflight error leaked %q: %v", secret, err)
		}
	}
}

func TestSigningDevicesUsageErrorsDoNotLeakUntrustedPathsOrFields(t *testing.T) {
	const secret = "SECRET-UDID-Rudrank-Phone"
	tests := []struct {
		name string
		err  error
		call func(error) error
	}{
		{
			name: "protected read",
			err:  &os.PathError{Op: "open", Path: filepath.Join(t.TempDir(), secret, "devices.json"), Err: os.ErrNotExist},
			call: protectedDevicesFileUsageError,
		},
		{
			name: "parse",
			err:  fmt.Errorf("decode devices file: json: unknown field %q", secret),
			call: invalidDevicesFileUsageError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, stderr := captureOutput(t, func() {
				err := test.call(test.err)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v, want usage classification", err)
				}
			})
			if strings.Contains(stderr, secret) {
				t.Fatalf("stderr leaked %q: %s", secret, stderr)
			}
			if !strings.Contains(stderr, "invalid devices file") {
				t.Fatalf("stderr = %q, want useful devices-file diagnostic", stderr)
			}
		})
	}
}

func TestSigningTargetsEqualNormalizesExactNestedNumbers(t *testing.T) {
	archive := []signingTarget{{
		BundleID: "com.example.app",
		Entitlements: map[string]any{
			"integer": uint64(7),
			"nested": []any{
				map[string]any{"negative": int64(-4), "fraction": float64(1.5)},
			},
		},
	}}
	plan := []signingTarget{{
		BundleID: "com.example.app",
		Entitlements: map[string]any{
			"integer": json.Number("7"),
			"nested": []any{
				map[string]any{"negative": json.Number("-4"), "fraction": json.Number("1.5")},
			},
		},
	}}
	if !signingTargetsEqual(archive, plan) {
		t.Fatal("signingTargetsEqual() rejected semantically equal nested numbers")
	}
	plan[0].Entitlements["integer"] = json.Number("8")
	if signingTargetsEqual(archive, plan) {
		t.Fatal("signingTargetsEqual() accepted a different integer")
	}
}

func TestSigningTargetsEqualPreservesLargeIntegerExactness(t *testing.T) {
	const large = uint64(9007199254740993)
	archive := []signingTarget{{BundleID: "com.example.app", Entitlements: map[string]any{"large": large}}}
	exact := []signingTarget{{BundleID: "com.example.app", Entitlements: map[string]any{"large": json.Number("9007199254740993")}}}
	if !signingTargetsEqual(archive, exact) {
		t.Fatal("exact json.Number representation of large integer was rejected")
	}
	different := []signingTarget{{BundleID: "com.example.app", Entitlements: map[string]any{"large": json.Number("9007199254740992")}}}
	if signingTargetsEqual(archive, different) {
		t.Fatal("neighboring large integers were treated as equal")
	}
	lossy := []signingTarget{{BundleID: "com.example.app", Entitlements: map[string]any{"large": float64(large)}}}
	if signingTargetsEqual(archive, lossy) {
		t.Fatal("lossy float64 representation beyond JSON's safe integer range was accepted")
	}
}

func TestReadSigningPlanPreservesLargeEntitlementInteger(t *testing.T) {
	const large = uint64(9007199254740993)
	stateDir := t.TempDir()
	plan := signingReconcilePlanArtifact{
		SchemaVersion: signingReconcileSchemaV1,
		Targets: []signingTarget{{
			BundleID:     "com.example.app",
			Entitlements: map[string]any{"large": large},
		}},
	}
	var err error
	plan.PlanHash, err = hashSigningReconcilePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSigningStateJSON(stateDir, "plan.json", plan, false); err != nil {
		t.Fatal(err)
	}
	decoded, err := readSigningPlanArtifact(filepath.Join(stateDir, "plan.json"))
	if err != nil {
		t.Fatalf("readSigningPlanArtifact() error = %v", err)
	}
	if !signingTargetsEqual(plan.Targets, decoded.Targets) {
		t.Fatalf("decoded targets lost numeric exactness: %#v", decoded.Targets)
	}
}

func TestLoadSigningReceiptRejectsPathsOutsidePlan(t *testing.T) {
	stateDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideReceipt := filepath.Join(outsideDir, "receipt.json")
	if err := os.WriteFile(outsideReceipt, []byte("outside sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := signingReconcilePlanArtifact{
		SchemaVersion: signingReconcileSchemaV1,
		PlanHash:      "plan-hash",
		Paths: signingReconcilePaths{
			StateDir:    stateDir,
			ReceiptPath: filepath.Join(stateDir, "receipt.json"),
		},
	}
	edited := signingReconcileReceipt{
		SchemaVersion: signingReconcileSchemaV1,
		PlanHash:      plan.PlanHash,
		StateDir:      outsideDir,
		ReceiptPath:   outsideReceipt,
		Complete:      true,
	}
	data, err := json.Marshal(edited)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "receipt.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := loadOrStartSigningReceipt(plan); err == nil {
		t.Fatal("loadOrStartSigningReceipt() error = nil, want path-binding rejection")
	}
	outside, err := os.ReadFile(outsideReceipt)
	if err != nil {
		t.Fatal(err)
	}
	if string(outside) != "outside sentinel" {
		t.Fatalf("outside receipt was overwritten: %q", outside)
	}
}

func TestSigningReconcilePlanHashExcludesGeneratedAtAndSelf(t *testing.T) {
	plan := signingReconcilePlanArtifact{
		SchemaVersion: signingReconcileSchemaV1,
		GeneratedAt:   "2026-01-01T00:00:00Z",
		PlanHash:      "old",
		Ready:         true,
		Targets:       []signingTarget{{BundleID: "com.example.app", Kind: "application"}},
		Actions:       []signingAction{{ID: "profile:com.example.app", Kind: actionCreateProfile}},
	}
	first, err := hashSigningReconcilePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.GeneratedAt = "2026-02-02T00:00:00Z"
	plan.PlanHash = "different"
	second, err := hashSigningReconcilePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("hash changed: %s != %s", first, second)
	}
}

func TestWriteSigningStateJSONUses0600AndOverwriteGate(t *testing.T) {
	dir := t.TempDir()
	if err := writeSigningStateJSON(dir, "plan.json", map[string]any{"ready": true}, false); err != nil {
		t.Fatalf("writeSigningStateJSON() error = %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
	if err := writeSigningStateJSON(dir, "plan.json", map[string]any{"ready": false}, false); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second write error = %v, want os.ErrExist", err)
	}
	if err := os.Chmod(filepath.Join(dir, "plan.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeSigningStateJSON(dir, "plan.json", map[string]any{"ready": false}, true); err != nil {
		t.Fatalf("overwrite error = %v", err)
	}
	info, err = os.Stat(filepath.Join(dir, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("overwritten mode=%o, want 600", got)
	}

	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "plan.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "plan.json")); err != nil {
		t.Fatal(err)
	}
	if err := writeSigningStateJSON(dir, "plan.json", map[string]any{"ready": true}, true); err == nil {
		t.Fatal("expected symlink overwrite rejection")
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "sentinel" {
		t.Fatalf("outside=%q err=%v", got, err)
	}
}

func TestValidateSigningApplyPlanRejectsMismatchedReceiptPath(t *testing.T) {
	stateDir := t.TempDir()
	err := validateSigningApplyPlan(signingReconcilePlanArtifact{
		Paths: signingReconcilePaths{
			StateDir:    stateDir,
			ReceiptPath: filepath.Join(stateDir, "alternate-receipt.json"),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "receipt path") {
		t.Fatalf("mismatched receipt path error = %v", err)
	}
}

func TestWriteSigningStateJSONRejectsPlanLargerThanApplyCanRead(t *testing.T) {
	dir := t.TempDir()
	value := map[string]string{"padding": string(bytes.Repeat([]byte("x"), reconcilePlanFileMaxBytes))}
	err := writeSigningStateJSON(dir, "plan.json", value, false)
	if err == nil || !strings.Contains(err.Error(), "plan exceeds") {
		t.Fatalf("writeSigningStateJSON() error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "plan.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("oversized plan artifact exists: %v", statErr)
	}
}

func newReconcileTestCertificate(t *testing.T, commonName string) (*x509.Certificate, crypto.PrivateKey) {
	return newReconcileTestCertificateForTeam(t, commonName, "TEAM1")
}

func newReconcileTestCertificateForTeam(t *testing.T, commonName, teamID string) (*x509.Certificate, crypto.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("rand.Int() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName, OrganizationalUnit: []string{teamID}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("x509.CreateCertificate() error = %v", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("x509.ParseCertificate() error = %v", err)
	}
	return certificate, key
}

func buildReconcileTestMobileProvision(t *testing.T, payload map[string]any, signer *x509.Certificate, key crypto.PrivateKey) []byte {
	t.Helper()
	plistBytes, err := plist.Marshal(payload, plist.XMLFormat)
	if err != nil {
		t.Fatalf("plist.Marshal() error = %v", err)
	}
	signed, err := pkcs7.NewSignedData(plistBytes)
	if err != nil {
		t.Fatalf("pkcs7.NewSignedData() error = %v", err)
	}
	if err := signed.AddSigner(signer, key, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatalf("SignedData.AddSigner() error = %v", err)
	}
	result, err := signed.Finish()
	if err != nil {
		t.Fatalf("SignedData.Finish() error = %v", err)
	}
	return result
}

func reconcileTestCertificateFingerprint(certificate *x509.Certificate) string {
	digest := sha256.Sum256(certificate.Raw)
	return hex.EncodeToString(digest[:])
}
