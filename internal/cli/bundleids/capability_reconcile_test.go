package bundleids

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestReconcileUnknownEntitlementIsUsage(t *testing.T) {
	path := writeEntitlements(t, map[string]any{"com.example.unknown": true})
	cmd := capabilityReconcileCommand("plan", false)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{"--bundle", "BUNDLE", "--entitlements", path, "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unknown entitlement key") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveCapabilityBundleIDAcceptsDottedResourceID(t *testing.T) {
	client := newReconcileClient(t, func(req *http.Request) *http.Response {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/bundleIds/id.with.dot" {
			t.Fatalf("unexpected %s %s", req.Method, req.URL.String())
		}
		return reconcileJSON(http.StatusOK, `{"data":{"type":"bundleIds","id":"id.with.dot","attributes":{"identifier":"com.example.app"}}}`)
	})
	id, err := resolveCapabilityBundleID(context.Background(), client, "id.with.dot")
	if err != nil || id != "id.with.dot" {
		t.Fatalf("id=%q error=%v, want dotted resource ID", id, err)
	}
}

func TestResolveCapabilityBundleIDAcceptsIdentifierWithoutDot(t *testing.T) {
	client := newReconcileClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/example":
			return reconcileJSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds" && req.URL.Query().Get("filter[identifier]") == "example":
			return reconcileJSON(http.StatusOK, `{"data":[{"type":"bundleIds","id":"opaque-id","attributes":{"identifier":"example"}}]}`)
		default:
			t.Fatalf("unexpected %s %s", req.Method, req.URL.String())
			return nil
		}
	})
	id, err := resolveCapabilityBundleID(context.Background(), client, "example")
	if err != nil || id != "opaque-id" {
		t.Fatalf("id=%q error=%v, want resolved resource ID", id, err)
	}
}

func TestResolveCapabilityBundleIDDoesNotMaskLookupError(t *testing.T) {
	client := newReconcileClient(t, func(req *http.Request) *http.Response {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/bundleIds/example" {
			t.Fatalf("unexpected %s %s", req.Method, req.URL.String())
		}
		return reconcileJSON(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN"}]}`)
	})
	_, err := resolveCapabilityBundleID(context.Background(), client, "example")
	if err == nil || !strings.Contains(err.Error(), "FORBIDDEN") {
		t.Fatalf("error=%v, want original forbidden error", err)
	}
}

func TestReconcileUbiquityContainerIdentifiersMapsToICloud(t *testing.T) {
	path := writeEntitlements(t, map[string]any{
		"com.apple.developer.ubiquity-container-identifiers": []string{"TEAMID.iCloud.example"},
	})
	desired, err := readDesiredEntitlements(path, false)
	if err != nil {
		t.Fatalf("read entitlements: %v", err)
	}
	if len(desired) != 1 || desired[0].spec.Capability != "ICLOUD" {
		t.Fatalf("desired = %#v, want ICLOUD", desired)
	}
}

func TestReconcileCloudKitSettingsSurviveMultipleICloudEntitlements(t *testing.T) {
	path := writeEntitlements(t, map[string]any{
		"com.apple.developer.icloud-container-identifiers":   []string{"iCloud.example"},
		"com.apple.developer.icloud-services":                []string{"CloudKit"},
		"com.apple.developer.ubiquity-container-identifiers": []string{"iCloud.example"},
	})
	desired, err := readDesiredEntitlements(path, false)
	if err != nil {
		t.Fatalf("read entitlements: %v", err)
	}
	current := []asc.Resource[asc.BundleIDCapabilityAttributes]{{
		ID: "icloud-1",
		Attributes: asc.BundleIDCapabilityAttributes{
			CapabilityType: "ICLOUD",
			Settings:       []asc.CapabilitySetting{{Key: "ICLOUD_VERSION", Options: []asc.CapabilityOption{{Key: "XCODE_5", Enabled: boolPointer(true)}, {Key: "XCODE_6", Enabled: boolPointer(false)}}}},
		},
	}}
	plan := buildCapabilityReconcilePlan("bundle-1", desired, current, false)
	if len(plan.Actions) != 1 || plan.Actions[0].Action != "update" {
		t.Fatalf("actions = %#v, want one ICLOUD update", plan.Actions)
	}
	settings := plan.Actions[0].Settings
	if len(settings) != 1 || settings[0].Key != "ICLOUD_VERSION" || !capabilityOptionEnabled(settings[0].Options, "XCODE_6") || capabilityOptionEnabled(settings[0].Options, "XCODE_5") {
		t.Fatalf("settings = %#v, want XCODE_6 enabled and XCODE_5 disabled", settings)
	}
}

func TestReconcileRejectsUnsupportedIncreasedMemoryLimit(t *testing.T) {
	path := writeEntitlements(t, map[string]any{
		"com.apple.developer.kernel.increased-memory-limit": true,
	})
	_, err := readDesiredEntitlements(path, false)
	if err == nil || !strings.Contains(err.Error(), "unsupported") || !strings.Contains(err.Error(), "INCREASED_MEMORY_LIMIT") {
		t.Fatalf("error = %v, want explicit unsupported capability", err)
	}
}

func TestReconcilePrivateCloudComputePlansUsableWebCommand(t *testing.T) {
	path := writeEntitlements(t, map[string]any{
		"com.apple.developer.private-cloud-compute": true,
	})
	desired, err := readDesiredEntitlements(path, false)
	if err != nil {
		t.Fatalf("read entitlements: %v", err)
	}
	plan := buildCapabilityReconcilePlan("bundle-1", desired, nil, false)
	if len(plan.Actions) != 1 || plan.Actions[0].Action != "needsWebSession" {
		t.Fatalf("actions = %#v, want needsWebSession", plan.Actions)
	}
	command := plan.Actions[0].Command
	for _, part := range []string{"asc web bundle-ids capabilities enable", "--bundle-id bundle-1", "--capability PRIVATE_CLOUD_COMPUTE", "--confirm"} {
		if !strings.Contains(command, part) {
			t.Fatalf("command = %q, missing %q", command, part)
		}
	}
}

func TestReconcilePrivateCloudComputeFalseDoesNotPlanEnable(t *testing.T) {
	path := writeEntitlements(t, map[string]any{
		"com.apple.developer.private-cloud-compute": false,
	})
	desired, err := readDesiredEntitlements(path, false)
	if err != nil {
		t.Fatalf("read entitlements: %v", err)
	}
	if len(desired) != 0 {
		t.Fatalf("desired = %#v, want no capability request", desired)
	}
}

func TestReconcileFalseBooleanEntitlementsDoNotEnableCapabilities(t *testing.T) {
	path := writeEntitlements(t, map[string]any{
		"com.apple.developer.siri":        false,
		"com.apple.developer.homekit":     false,
		"com.apple.developer.healthkit":   true,
		"com.apple.developer.game-center": false,
	})
	desired, err := readDesiredEntitlements(path, false)
	if err != nil {
		t.Fatalf("read entitlements: %v", err)
	}
	if len(desired) != 1 || desired[0].spec.Capability != "HEALTHKIT" {
		t.Fatalf("desired = %#v, want only HEALTHKIT", desired)
	}
}

func TestReconcileRejectsWrongEntitlementValueKinds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		key   string
		value any
	}{
		{name: "boolean as string", key: "com.apple.developer.healthkit", value: "yes"},
		{name: "APS environment as boolean", key: "aps-environment", value: false},
		{name: "iCloud services as boolean", key: "com.apple.developer.icloud-services", value: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeEntitlements(t, map[string]any{tc.key: tc.value})
			if _, err := readDesiredEntitlements(path, false); err == nil {
				t.Fatalf("expected %q to reject value %#v", tc.key, tc.value)
			}
		})
	}
}

func TestReconcileRejectsHallucinatedInAppPurchaseEntitlement(t *testing.T) {
	path := writeEntitlements(t, map[string]any{"com.apple.InAppPurchase": true})
	_, err := readDesiredEntitlements(path, false)
	if err == nil || !strings.Contains(err.Error(), "unknown entitlement key") {
		t.Fatalf("error = %v, want unknown entitlement key", err)
	}
}

func TestReconcileUsesActualInterAppAudioEntitlementKey(t *testing.T) {
	path := writeEntitlements(t, map[string]any{"inter-app-audio": true})
	desired, err := readDesiredEntitlements(path, false)
	if err != nil || len(desired) != 1 || desired[0].spec.Capability != "INTER_APP_AUDIO" {
		t.Fatalf("desired = %#v, error = %v; want INTER_APP_AUDIO", desired, err)
	}
}

func TestReconcileRejectsSymlinkedEntitlementsParent(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(realDir, "App.entitlements")
	data, err := plist.Marshal(map[string]any{"com.apple.developer.siri": true}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	linkedDir := filepath.Join(dir, "linked")
	if err := os.Symlink(realDir, linkedDir); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	if _, err := readDesiredEntitlements(filepath.Join(linkedDir, "App.entitlements"), false); err == nil {
		t.Fatal("expected symlinked parent to be rejected")
	}
}

func TestReconcilePlanDoesNotAcceptConfirm(t *testing.T) {
	if flag := capabilityReconcileCommand("plan", false).FlagSet.Lookup("confirm"); flag != nil {
		t.Fatal("plan must not advertise an ignored --confirm flag")
	}
	if flag := capabilityReconcileCommand("apply", true).FlagSet.Lookup("confirm"); flag == nil {
		t.Fatal("apply must require --confirm")
	}
}

func TestReconcileApplyAddsAndPreservesPushSettings(t *testing.T) {
	enabled := true
	var posts, patches int
	client := newReconcileClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-1":
			return reconcileJSON(http.StatusOK, `{"data":{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app"}}}`)
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/bundleIdCapabilities"):
			body := `{"data":[{"type":"bundleIdCapabilities","id":"push-1","attributes":{"capabilityType":"PUSH_NOTIFICATIONS","settings":[{"key":"BROADCAST","options":[{"key":"BROADCAST_ENABLED","enabled":true}]}]}}]}`
			return reconcileJSON(http.StatusOK, body)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/bundleIdCapabilities":
			posts++
			payload, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(payload), `"key":"COMPLETE_PROTECTION"`) || strings.Contains(string(payload), "NSFileProtectionComplete") {
				t.Fatalf("add payload = %s", payload)
			}
			return reconcileJSON(http.StatusCreated, `{"data":{"type":"bundleIdCapabilities","id":"dp-1","attributes":{"capabilityType":"DATA_PROTECTION"}}}`)
		case req.Method == http.MethodPatch:
			patches++
			return reconcileJSON(http.StatusOK, `{"data":{"type":"bundleIdCapabilities","id":"push-1","attributes":{}}}`)
		default:
			t.Fatalf("unexpected %s %s", req.Method, req.URL.Path)
			return nil
		}
	})
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))
	path := writeEntitlements(t, map[string]any{
		"aps-environment": "production",
		"com.apple.developer.default-data-protection": "NSFileProtectionComplete",
	})
	cmd := capabilityReconcileCommand("apply", true)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{"--bundle", "bundle-1", "--entitlements", path, "--confirm", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	stdout, _ := captureReconcile(t, func() { runErr = cmd.Run(context.Background()) })
	if runErr != nil {
		t.Fatalf("apply: %v\n%s", runErr, stdout)
	}
	if posts != 1 || patches != 0 {
		t.Fatalf("posts=%d patches=%d; push settings must be kept, not replaced", posts, patches)
	}
	if !strings.Contains(stdout, `"action":"keep"`) || !strings.Contains(stdout, `"action":"add"`) {
		t.Fatalf("stdout = %s", stdout)
	}
	_ = enabled
}

func TestReconcileApplyUpdatesProtectionWithoutDroppingAlternatives(t *testing.T) {
	var patches int
	client := newReconcileClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-1":
			return reconcileJSON(http.StatusOK, `{"data":{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app"}}}`)
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/bundleIdCapabilities"):
			return reconcileJSON(http.StatusOK, `{"data":[{"type":"bundleIdCapabilities","id":"dp-1","attributes":{"capabilityType":"DATA_PROTECTION","settings":[{"key":"DATA_PROTECTION_PERMISSION_LEVEL","options":[{"key":"COMPLETE_PROTECTION","enabled":false},{"key":"PROTECTED_UNLESS_OPEN","enabled":true},{"key":"PROTECTED_UNTIL_FIRST_USER_AUTH","enabled":false}]}]}}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/bundleIdCapabilities/dp-1":
			patches++
			var payload struct {
				Data struct {
					Attributes struct {
						Settings []asc.CapabilitySetting `json:"settings"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			options := payload.Data.Attributes.Settings[0].Options
			if len(options) != 3 || !capabilityOptionEnabled(options, "COMPLETE_PROTECTION") || capabilityOptionEnabled(options, "PROTECTED_UNLESS_OPEN") {
				t.Fatalf("PATCH options = %#v", options)
			}
			return reconcileJSON(http.StatusOK, `{"data":{"type":"bundleIdCapabilities","id":"dp-1","attributes":{"capabilityType":"DATA_PROTECTION"}}}`)
		default:
			t.Fatalf("unexpected %s %s", req.Method, req.URL.Path)
			return nil
		}
	})
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))
	path := writeEntitlements(t, map[string]any{"com.apple.developer.default-data-protection": "NSFileProtectionComplete"})
	cmd := capabilityReconcileCommand("apply", true)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{"--bundle", "bundle-1", "--entitlements", path, "--confirm", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	stdout, _ := captureReconcile(t, func() { runErr = cmd.Run(context.Background()) })
	if runErr != nil || patches != 1 || !strings.Contains(stdout, `"status":"applied"`) {
		t.Fatalf("error=%v patches=%d stdout=%s", runErr, patches, stdout)
	}
}

func TestEntitlementCapabilitiesUsePublishedTypes(t *testing.T) {
	data, err := os.ReadFile("../../../docs/openapi/latest.json")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(data), `"CapabilityType"`)
	if start < 0 {
		t.Fatal("CapabilityType schema missing")
	}
	section := string(data)[start : start+2500]
	for _, item := range entitlementCapabilityCatalog() {
		if item.Capability == "" {
			continue
		}
		if !strings.Contains(section, `"`+item.Capability+`"`) {
			t.Fatalf("%s is not in the published CapabilityType enum", item.Capability)
		}
	}
}

func TestMergeCapabilitySettingsKeepsBroadcast(t *testing.T) {
	enabled := true
	existing := []asc.CapabilitySetting{{Key: "BROADCAST", Options: []asc.CapabilityOption{{Key: "BROADCAST_ENABLED", Enabled: &enabled}}}}
	desired := dataProtectionSettings("NSFileProtectionComplete")
	merged, changed := mergeCapabilitySettings(existing, desired)
	if !changed || !capabilityOptionPresent(merged[0].Options, "BROADCAST_ENABLED") {
		t.Fatalf("merged = %#v changed=%v", merged, changed)
	}
}

func TestMergeCapabilitySettingsConvergesProtectionLevel(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []asc.CapabilityOption
		wantKey string
	}{
		{name: "disabled desired option", options: []asc.CapabilityOption{{Key: "COMPLETE_PROTECTION", Enabled: boolPointer(false)}}, wantKey: "COMPLETE_PROTECTION"},
		{name: "different enabled level", options: []asc.CapabilityOption{{Key: "PROTECTED_UNTIL_FIRST_USER_AUTH", Enabled: boolPointer(true)}}, wantKey: "COMPLETE_PROTECTION"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			existing := []asc.CapabilitySetting{
				{Key: "BROADCAST", Options: []asc.CapabilityOption{{Key: "BROADCAST_ENABLED", Enabled: boolPointer(true)}}},
				{Key: "DATA_PROTECTION_PERMISSION_LEVEL", Options: tc.options},
			}
			merged, changed := mergeCapabilitySettings(existing, dataProtectionSettings("NSFileProtectionComplete"))
			if !changed || len(merged) != 2 || !capabilityOptionEnabled(merged[1].Options, tc.wantKey) {
				t.Fatalf("merged = %#v changed=%v", merged, changed)
			}
			if tc.name == "different enabled level" && capabilityOptionEnabled(merged[1].Options, "PROTECTED_UNTIL_FIRST_USER_AUTH") {
				t.Fatalf("old protection level remained enabled: %#v", merged)
			}
			_, changed = mergeCapabilitySettings(merged, dataProtectionSettings("NSFileProtectionComplete"))
			if changed {
				t.Fatalf("second reconciliation should keep: %#v", merged)
			}
			if !capabilityOptionPresent(merged[0].Options, "BROADCAST_ENABLED") {
				t.Fatalf("broadcast setting lost: %#v", merged)
			}
		})
	}
}

func TestMergeCapabilitySettingsKeepsDisabledAlternatives(t *testing.T) {
	existing := []asc.CapabilitySetting{{
		Key: "DATA_PROTECTION_PERMISSION_LEVEL",
		Options: []asc.CapabilityOption{
			{Key: "COMPLETE_PROTECTION", Enabled: boolPointer(true)},
			{Key: "PROTECTED_UNLESS_OPEN", Enabled: boolPointer(false)},
			{Key: "PROTECTED_UNTIL_FIRST_USER_AUTH", Enabled: boolPointer(false)},
		},
	}}
	merged, changed := mergeCapabilitySettings(existing, dataProtectionSettings("NSFileProtectionComplete"))
	if changed || len(merged[0].Options) != 3 {
		t.Fatalf("merged=%#v changed=%v", merged, changed)
	}
}

func capabilityOptionEnabled(options []asc.CapabilityOption, key string) bool {
	for _, option := range options {
		if option.Key == key && option.Enabled != nil {
			return *option.Enabled
		}
	}
	return false
}

func TestMergeCapabilitySettingsIgnoresReadOnlyOptionMetadata(t *testing.T) {
	existing := []asc.CapabilitySetting{{
		Key:     "DATA_PROTECTION_PERMISSION_LEVEL",
		Options: []asc.CapabilityOption{{Key: "COMPLETE_PROTECTION", Name: "Complete Protection", Enabled: boolPointer(true)}},
	}}
	merged, changed := mergeCapabilitySettings(existing, dataProtectionSettings("NSFileProtectionComplete"))
	if changed || len(merged) != 1 || merged[0].Options[0].Name != "Complete Protection" {
		t.Fatalf("merged=%#v changed=%v", merged, changed)
	}
}

func TestDataProtectionSettingsUsesASCOptionKeys(t *testing.T) {
	for entitlement, want := range map[string]string{
		"NSFileProtectionComplete":                             "COMPLETE_PROTECTION",
		"NSFileProtectionCompleteUnlessOpen":                   "PROTECTED_UNLESS_OPEN",
		"NSFileProtectionCompleteUntilFirstUserAuthentication": "PROTECTED_UNTIL_FIRST_USER_AUTH",
	} {
		settings := dataProtectionSettings(entitlement)
		if len(settings) != 1 || len(settings[0].Options) != 1 || settings[0].Options[0].Key != want {
			t.Fatalf("%s: settings = %#v, want %s", entitlement, settings, want)
		}
	}
}

func TestReconcileDataProtectionNoneDoesNotAddCapability(t *testing.T) {
	path := writeEntitlements(t, map[string]any{"com.apple.developer.default-data-protection": "NSFileProtectionNone"})
	desired, err := readDesiredEntitlements(path, false)
	if err != nil || len(desired) != 0 {
		t.Fatalf("desired=%#v error=%v", desired, err)
	}
	existing := []asc.Resource[asc.BundleIDCapabilityAttributes]{{ID: "dp-1", Attributes: asc.BundleIDCapabilityAttributes{CapabilityType: "DATA_PROTECTION"}}}
	plan := buildCapabilityReconcilePlan("bundle-1", desired, existing, false)
	if len(plan.Actions) != 1 || plan.Actions[0].Action != "unmanaged" {
		t.Fatalf("plan without --allow-remove = %#v", plan)
	}
	plan = buildCapabilityReconcilePlan("bundle-1", desired, existing, true)
	if len(plan.Actions) != 1 || plan.Actions[0].Action != "remove" {
		t.Fatalf("plan with --allow-remove = %#v", plan)
	}
}

func TestReconcileRejectsUnknownDataProtectionValue(t *testing.T) {
	path := writeEntitlements(t, map[string]any{"com.apple.developer.default-data-protection": "not-a-protection-level"})
	_, err := readDesiredEntitlements(path, false)
	if err == nil || !strings.Contains(err.Error(), "not-a-protection-level") {
		t.Fatalf("error = %v", err)
	}
}

func boolPointer(value bool) *bool { return &value }

func TestReconcileApplyPrintsPartialReceiptOnAPIFailure(t *testing.T) {
	var posts int
	client := newReconcileClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-1":
			return reconcileJSON(http.StatusOK, `{"data":{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app"}}}`)
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/bundleIdCapabilities"):
			return reconcileJSON(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/bundleIdCapabilities":
			posts++
			if posts == 1 {
				return reconcileJSON(http.StatusCreated, `{"data":{"type":"bundleIdCapabilities","id":"created-1","attributes":{"capabilityType":"DATA_PROTECTION"}}}`)
			}
			return reconcileJSON(http.StatusInternalServerError, `{"errors":[{"status":"500","title":"test failure"}]}`)
		default:
			t.Fatalf("unexpected %s %s", req.Method, req.URL.Path)
			return nil
		}
	})
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))
	path := writeEntitlements(t, map[string]any{
		"aps-environment": "production",
		"com.apple.developer.default-data-protection": "NSFileProtectionComplete",
	})
	cmd := capabilityReconcileCommand("apply", true)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{"--bundle", "bundle-1", "--entitlements", path, "--confirm", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	stdout, _ := captureReconcile(t, func() { runErr = cmd.Run(context.Background()) })
	if runErr == nil || posts != 2 {
		t.Fatalf("error=%v posts=%d stdout=%s", runErr, posts, stdout)
	}
	var reportedErr shared.ReportedError
	if !errors.As(runErr, &reportedErr) {
		t.Fatalf("error type = %T, want reported error after printing receipt", runErr)
	}
	var receipt struct {
		Actions []struct {
			Capability   string `json:"capability"`
			CapabilityID string `json:"capabilityId"`
			Status       string `json:"status"`
			Error        string `json:"error"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("partial receipt is not JSON: %v; stdout=%s", err, stdout)
	}
	if len(receipt.Actions) != 2 || receipt.Actions[0].Status != "applied" || receipt.Actions[0].CapabilityID != "created-1" || receipt.Actions[1].Status != "failed" || receipt.Actions[1].Error == "" {
		t.Fatalf("partial receipt = %+v", receipt)
	}
}

func TestReconcileApplyRefusesWebOnlyCapabilityBeforeWrites(t *testing.T) {
	var writes int
	client := newReconcileClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-1":
			return reconcileJSON(http.StatusOK, `{"data":{"type":"bundleIds","id":"bundle-1","attributes":{"identifier":"com.example.app"}}}`)
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/bundleIdCapabilities"):
			return reconcileJSON(http.StatusOK, `{"data":[]}`)
		default:
			writes++
			return reconcileJSON(http.StatusCreated, `{"data":{"type":"bundleIdCapabilities","id":"created-1","attributes":{"capabilityType":"HEALTHKIT"}}}`)
		}
	})
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))
	path := writeEntitlements(t, map[string]any{
		"com.apple.developer.private-cloud-compute": true,
		"com.apple.developer.healthkit":             true,
	})
	cmd := capabilityReconcileCommand("apply", true)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{"--bundle", "bundle-1", "--entitlements", path, "--confirm", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	stdout, _ := captureReconcile(t, func() { runErr = cmd.Run(context.Background()) })
	var reportedErr shared.ReportedError
	if runErr == nil || !errors.As(runErr, &reportedErr) || writes != 0 {
		t.Fatalf("error=%v writes=%d stdout=%s, want nonzero reported result with no writes", runErr, writes, stdout)
	}
	var receipt struct {
		Actions []struct {
			Action string `json:"action"`
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("receipt JSON: %v; stdout=%s", err, stdout)
	}
	if len(receipt.Actions) != 2 || receipt.Actions[1].Action != "needsWebSession" || receipt.Actions[1].Status != "failed" || receipt.Actions[1].Error == "" {
		t.Fatalf("receipt = %+v, want explicit failed web-only action", receipt.Actions)
	}
}

func TestReconcileApplyMissingConfirmPrintsOnce(t *testing.T) {
	cmd := capabilityReconcileCommand("apply", true)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{"--bundle", "bundle-1", "--entitlements", "unused.plist"}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	_, stderr := captureReconcile(t, func() { runErr = cmd.Run(context.Background()) })
	if runErr == nil || strings.Count(stderr, "--confirm is required") != 1 {
		t.Fatalf("error=%v stderr=%q", runErr, stderr)
	}
}

func TestReconcileApplyRejectsInvalidOutputBeforeAPI(t *testing.T) {
	path := writeEntitlements(t, map[string]any{"aps-environment": "production"})
	var clientCalls int
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalls++
		return nil, io.EOF
	}))
	cmd := capabilityReconcileCommand("apply", true)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{"--bundle", "bundle-1", "--entitlements", path, "--confirm", "--output", "bogus"}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	_, _ = captureReconcile(t, func() { runErr = cmd.Run(context.Background()) })
	if runErr == nil || !strings.Contains(runErr.Error(), "--output") || clientCalls != 0 {
		t.Fatalf("error=%v clientCalls=%d", runErr, clientCalls)
	}
}

func writeEntitlements(t *testing.T, values map[string]any) string {
	t.Helper()
	data, err := plist.Marshal(values, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "App.entitlements")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func newReconcileClient(t *testing.T, fn func(*http.Request) *http.Response) *asc.Client {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "key.p8")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := asc.NewClientWithHTTPClient("KEY", "ISSUER", path, &http.Client{Transport: reconcileTrip(fn)})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

type reconcileTrip func(*http.Request) *http.Response

func (fn reconcileTrip) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req), nil }

func reconcileJSON(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func captureReconcile(t *testing.T, fn func()) (string, string) {
	t.Helper()
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	old, oldErr := os.Stdout, os.Stderr
	os.Stdout = stdout
	os.Stderr = stderr
	defer func() { os.Stdout, os.Stderr = old, oldErr }()
	fn()
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}
	if err := stderr.Close(); err != nil {
		t.Fatal(err)
	}
	errData, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data), string(errData)
}
