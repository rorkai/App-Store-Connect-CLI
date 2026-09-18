package bundleids

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestReconcileApplyAddsAndPreservesPushSettings(t *testing.T) {
	enabled := true
	var posts, patches int
	client := newReconcileClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/bundleIdCapabilities"):
			body := `{"data":[{"type":"bundleIdCapabilities","id":"push-1","attributes":{"capabilityType":"PUSH_NOTIFICATIONS","settings":[{"key":"BROADCAST","options":[{"key":"BROADCAST_ENABLED","enabled":true}]}]}}]}`
			return reconcileJSON(http.StatusOK, body)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/bundleIdCapabilities":
			posts++
			payload, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(payload), "DATA_PROTECTION") {
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

func writeEntitlements(t *testing.T, values map[string]any) string {
	t.Helper()
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "App.entitlements")
	if err := os.WriteFile(path, []byte(plistXML(values)), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = data
	return path
}

func plistXML(values map[string]any) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>`)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for _, key := range keys {
		b.WriteString("<key>" + key + "</key>")
		switch value := values[key].(type) {
		case string:
			b.WriteString("<string>" + value + "</string>")
		default:
			b.WriteString("<true/>")
		}
	}
	b.WriteString("</dict></plist>")
	return b.String()
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
	old := os.Stdout
	os.Stdout = stdout
	defer func() { os.Stdout = old }()
	fn()
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data), ""
}
