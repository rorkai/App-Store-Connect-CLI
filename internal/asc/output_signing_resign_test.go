package asc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSigningResignResultRegisteredTableAndMarkdownOutput(t *testing.T) {
	result := &SigningResignResult{
		SchemaVersion: 1,
		Command:       "signing resign",
		Input: SigningResignInputResult{
			SizeBytes: 42,
			SHA256:    strings.Repeat("A", 64),
		},
		Output: SigningResignArtifactResult{
			Path:      "/safe/output/resigned.ipa",
			SizeBytes: 43,
			SHA256:    strings.Repeat("B", 64),
		},
		Identity: SigningResignIdentityResult{
			CertificateSHA256: strings.Repeat("C", 64),
			TeamID:            "TEAMID",
		},
		Targets: []SigningResignTargetResult{{
			Kind:          "application",
			RelativePath:  "Payload/App.app",
			BundleID:      "com.example.app",
			ProfileClass:  "app-store",
			ProfileUUID:   "PROFILE-UUID",
			ProfileSHA256: strings.Repeat("D", 64),
			Status:        "verified",
		}},
		Verification: SigningResignVerification{Status: "verified", Scope: "complete"},
	}

	ensureOutputRegistryPopulated()
	if !isRegistryTypeRegistered(typeForPtr[SigningResignResult]()) {
		t.Fatal("SigningResignResult is not registered with the output renderer")
	}

	table := captureStdout(t, func() error { return PrintTable(result) })
	for _, want := range []string{"schemaVersion", "1", "input.sizeBytes", "42", "output.path", "com.example.app", "verified"} {
		if !strings.Contains(table, want) {
			t.Fatalf("expected table to contain %q, got %q", want, table)
		}
	}
	markdown := captureStdout(t, func() error { return PrintMarkdown(result) })
	for _, want := range []string{"| schemaVersion", "| input.sizeBytes", "| output.path", "| com.example.app"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("expected markdown to contain %q, got %q", want, markdown)
		}
	}
}

func TestSigningResignResultEntitlementRewritesAreAdditiveAndOrdered(t *testing.T) {
	firstIndex := 0
	result := &SigningResignResult{
		EntitlementRewrites: func() *[]SigningResignEntitlementRewrite {
			values := []SigningResignEntitlementRewrite{
				{
					TargetRelativePath: "Payload/App.app",
					BundleID:           "com.example.app",
					Key:                "keychain-access-groups",
					ElementIndex:       &firstIndex,
					From:               "OLDPREFIX.com.example.app",
					To:                 "NEWPREFIX.com.example.app",
				},
				{
					TargetRelativePath: "Payload/App.app",
					BundleID:           "com.example.app",
					Key:                "com.apple.developer.ubiquity-kvstore-identifier",
					From:               "OLDPREFIX.com.example.app",
					To:                 "NEWPREFIX.com.example.app",
				},
			}
			return &values
		}(),
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"entitlementRewrites"`) || strings.Index(text, "keychain-access-groups") > strings.Index(text, "ubiquity-kvstore-identifier") {
		t.Fatalf("entitlement rewrites JSON is missing or nondeterministic: %s", text)
	}
	if !strings.Contains(text, `"targetRelativePath":"Payload/App.app"`) || !strings.Contains(text, `"elementIndex":0`) {
		t.Fatalf("entitlement rewrite JSON uses the wrong field names: %s", text)
	}
	var decoded struct {
		EntitlementRewrites []SigningResignEntitlementRewrite `json:"entitlementRewrites"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.EntitlementRewrites) != 2 || decoded.EntitlementRewrites[0].To == nil || decoded.EntitlementRewrites[0].ElementIndex == nil || *decoded.EntitlementRewrites[0].ElementIndex != 0 {
		t.Fatalf("decoded entitlement rewrites = %#v, want every additive rewrite", decoded.EntitlementRewrites)
	}
	rows, values := signingResignResultRows(result)
	foundRewriteRow := false
	for _, row := range values {
		if len(row) > 0 && row[0] == "entitlementRewrite.000.key" {
			foundRewriteRow = true
			break
		}
	}
	if len(rows) == 0 || len(values) == 0 || !strings.Contains(rows[0], "field") || !foundRewriteRow {
		t.Fatalf("table rows omitted entitlement rewrites: headers=%#v rows=%#v", rows, values)
	}
}

func TestSigningResignResultEntitlementRewritesOmitDisabledAndEmitEnabledEmpty(t *testing.T) {
	withoutRewrites := &SigningResignResult{}
	encoded, err := json.Marshal(withoutRewrites)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "entitlementRewrites") {
		t.Fatalf("disabled result unexpectedly emitted entitlementRewrites: %s", encoded)
	}
	for _, target := range []SigningResignTargetResult{{BundleID: "com.example.app"}, {BundleID: "com.example.new", SourceBundleID: "com.example.app"}} {
		encoded, err := json.Marshal(target)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(string(encoded), `"sourceBundleId":"com.example.app"`); got != (target.SourceBundleID != "") || strings.Count(string(encoded), "sourceBundleId") > 1 || (!got && strings.Contains(string(encoded), "sourceBundleId")) {
			t.Fatalf("target JSON = %s, want sourceBundleId only for a mapped target", encoded)
		}
	}
	empty := []SigningResignEntitlementRewrite{}
	withEmptyRewrites := &SigningResignResult{EntitlementRewrites: &empty}
	encoded, err = json.Marshal(withEmptyRewrites)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"entitlementRewrites":[]`) {
		t.Fatalf("enabled result did not emit empty entitlementRewrites array: %s", encoded)
	}
}
