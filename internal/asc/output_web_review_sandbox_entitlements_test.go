package asc

import (
	"strings"
	"testing"
)

func TestWebReviewSandboxEntitlementResultUsesRegisteredRenderers(t *testing.T) {
	result := &WebReviewSandboxEntitlementResult{
		VersionID:      "version-1",
		ReviewDetailID: "detail-1",
		Entitlement:    "com.apple.security.network.server",
		UsageID:        "usage-1",
		Justification:  "Listens for paired devices.",
		Changed:        true,
		Status:         "created",
	}
	ensureOutputRegistryPopulated()
	if !isRegistryTypeRegistered(typeForPtr[WebReviewSandboxEntitlementResult]()) {
		t.Fatal("WebReviewSandboxEntitlementResult is not registered with the output renderer")
	}
	for name, render := range map[string]func(any) error{"table": PrintTable, "markdown": PrintMarkdown} {
		output := captureStdout(t, func() error { return render(result) })
		for _, want := range []string{"version-1", "detail-1", "com.apple.security.network.server", "usage-1", "Listens for paired devices.", "created"} {
			if !strings.Contains(output, want) {
				t.Fatalf("%s output missing %q: %q", name, want, output)
			}
		}
	}
}

func TestWebReviewSandboxEntitlementsApplyResultUsesRegisteredRenderers(t *testing.T) {
	result := &WebReviewSandboxEntitlementsApplyResult{
		VersionID:      "version-1",
		ReviewDetailID: "detail-1",
		Changed:        true,
		Rows: []WebReviewSandboxEntitlementApplyRow{
			{Entitlement: "com.apple.security.network.server", UsageID: "usage-1", Justification: "Listens for paired devices.", Status: "updated"},
		},
	}
	ensureOutputRegistryPopulated()
	if !isRegistryTypeRegistered(typeForPtr[WebReviewSandboxEntitlementsApplyResult]()) {
		t.Fatal("WebReviewSandboxEntitlementsApplyResult is not registered with the output renderer")
	}
	for name, render := range map[string]func(any) error{"table": PrintTable, "markdown": PrintMarkdown} {
		output := captureStdout(t, func() error { return render(result) })
		for _, want := range []string{"com.apple.security.network.server", "updated", "usage-1", "Listens for paired devices."} {
			if !strings.Contains(output, want) {
				t.Fatalf("%s output missing %q: %q", name, want, output)
			}
		}
	}
}
