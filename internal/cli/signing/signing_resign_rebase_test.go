package signing

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func rebaseTestProfile(bundleID, prefix string, claims map[string]any) signingResignProfile {
	entitlements := map[string]any{
		"application-identifier":              prefix + "." + bundleID,
		"com.apple.application-identifier":    prefix + "." + bundleID,
		"com.apple.developer.team-identifier": "NEWTEAM",
		"get-task-allow":                      false,
	}
	for key, value := range claims {
		entitlements[key] = value
	}
	return signingResignProfile{
		ApplicationIdentifierPrefix: prefix,
		BundleID:                    bundleID,
		TeamID:                      "NEWTEAM",
		Entitlements:                entitlements,
	}
}

func rebaseTestTarget(kind, relativePath, bundleID string, claims map[string]any) signingResignTarget {
	entitlements := map[string]any{
		"application-identifier":              "OLDPREFIX." + bundleID,
		"com.apple.application-identifier":    "OLDPREFIX." + bundleID,
		"com.apple.developer.team-identifier": "OLDTEAM",
		"get-task-allow":                      false,
	}
	for key, value := range claims {
		entitlements[key] = value
	}
	return signingResignTarget{Kind: kind, RelativePath: relativePath, BundleID: bundleID, ExistingEntitlements: entitlements}
}

func TestPlanSigningResignEntitlementsRebasesAllowlistedClaimsWithProfilePrefix(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{
			"OLDPREFIX.com.example.app",
			"OLDPREFIX.com.example.shared",
		},
		signingResignKVStoreEntitlement: "OLDPREFIX.com.example.app",
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*"},
		signingResignKVStoreEntitlement:        "NEWPREFIX.com.example.app",
	})
	plans, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err != nil {
		t.Fatalf("planSigningResignEntitlements() error = %v", err)
	}
	if got := plans[0].Entitlements["application-identifier"]; got != "NEWPREFIX.com.example.app" {
		t.Fatalf("application-identifier = %#v, want profile prefix rather than TeamID", got)
	}
	if got := plans[0].Entitlements[signingResignKeychainGroupsEntitlement]; !signingResignEntitlementValuesEqual(got, []string{
		"NEWPREFIX.com.example.app",
		"NEWPREFIX.com.example.shared",
	}) {
		t.Fatalf("keychain-access-groups = %#v, want ordered rebased values", got)
	}
	if got := plans[0].Entitlements[signingResignKVStoreEntitlement]; got != "NEWPREFIX.com.example.app" {
		t.Fatalf("ubiquity-kvstore-identifier = %#v, want rebased value", got)
	}
	if len(plans[0].Rewrites) != 3 || plans[0].Rewrites[0].Key != signingResignKeychainGroupsEntitlement || plans[0].Rewrites[1].Key != signingResignKeychainGroupsEntitlement || plans[0].Rewrites[2].Key != signingResignKVStoreEntitlement {
		t.Fatalf("rewrites = %#v, want one ordered receipt per keychain element plus KVS scalar", plans[0].Rewrites)
	}
	if plans[0].Rewrites[0].Index == nil || *plans[0].Rewrites[0].Index != 0 || plans[0].Rewrites[1].Index == nil || *plans[0].Rewrites[1].Index != 1 || plans[0].Rewrites[2].Index != nil {
		t.Fatalf("rewrite indexes = %#v, want keychain indexes 0/1 and scalar KVS", plans[0].Rewrites)
	}
	for _, rewrite := range plans[0].Rewrites {
		if signingResignEntitlementContainsWildcard(rewrite.To) {
			t.Fatalf("rewrite %s emitted a wildcard: %#v", rewrite.Key, rewrite.To)
		}
	}
}

func TestPlanSigningResignEntitlementsPreservesDuplicateKeychainElements(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{
			"OLDPREFIX.com.example.shared",
			"OLDPREFIX.com.example.shared",
		},
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*"},
	})
	plans, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err != nil {
		t.Fatalf("planSigningResignEntitlements() error = %v", err)
	}
	if got := plans[0].Entitlements[signingResignKeychainGroupsEntitlement]; !signingResignEntitlementValuesEqual(got, []string{
		"NEWPREFIX.com.example.shared",
		"NEWPREFIX.com.example.shared",
	}) {
		t.Fatalf("keychain-access-groups = %#v, want duplicate order preserved", got)
	}
	if len(plans[0].Rewrites) != 2 || plans[0].Rewrites[0].Index == nil || plans[0].Rewrites[1].Index == nil || *plans[0].Rewrites[0].Index != 0 || *plans[0].Rewrites[1].Index != 1 {
		t.Fatalf("duplicate rewrites = %#v, want one receipt per source position", plans[0].Rewrites)
	}
}

func TestPlanSigningResignEntitlementsPreservesAuthorizedExistingKVS(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKVStoreEntitlement: "OLDPREFIX.com.example.app",
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKVStoreEntitlement: "OLDPREFIX.com.example.app",
	})
	plans, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err != nil {
		t.Fatalf("planSigningResignEntitlements() error = %v", err)
	}
	if got := plans[0].Entitlements[signingResignKVStoreEntitlement]; got != target.ExistingEntitlements[signingResignKVStoreEntitlement] {
		t.Fatalf("KVS = %#v, want exact existing authorized value", got)
	}
	if len(plans[0].Rewrites) != 0 {
		t.Fatalf("KVS rewrites = %#v, want no rewrite for preserved value", plans[0].Rewrites)
	}
}

func TestPlanSigningResignEntitlementsUsesExactKVSProfileValueWithoutAppPrefixDerivation(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKVStoreEntitlement: "OLDKVS.com.example.app",
	})
	profile := rebaseTestProfile(target.BundleID, "NEWAPP", map[string]any{
		signingResignKVStoreEntitlement: "NEWKVS.com.example.app",
	})
	plans, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err != nil {
		t.Fatalf("planSigningResignEntitlements() error = %v", err)
	}
	if got := plans[0].Entitlements[signingResignKVStoreEntitlement]; got != "NEWKVS.com.example.app" {
		t.Fatalf("KVS = %#v, want exact replacement-profile value", got)
	}
}

func TestPlanSigningResignEntitlementsRequiresSourceApplicationIdentifierForKVSRebase(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKVStoreEntitlement: "OLDKVS.com.example.app",
	})
	delete(target.ExistingEntitlements, "application-identifier")
	delete(target.ExistingEntitlements, "com.apple.application-identifier")
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKVStoreEntitlement: "NEWKVS.com.example.app",
	})
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err == nil || !strings.Contains(err.Error(), "application-identifier") {
		t.Fatalf("planSigningResignEntitlements() error = %v, want source application-identifier refusal", err)
	}
}

func TestPlanSigningResignEntitlementsRejectsWildcardOnlyKVSReplacement(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKVStoreEntitlement: "OLDPREFIX.com.example.app",
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKVStoreEntitlement: "NEWPREFIX.*",
	})
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err == nil || !strings.Contains(err.Error(), "wildcard cannot choose") {
		t.Fatalf("planSigningResignEntitlements() error = %v, want wildcard-only KVS refusal", err)
	}
}

func TestSigningResignEntitlementValuePermits(t *testing.T) {
	strict := signingResignStrictEntitlementValuePermits
	legacy := signingResignEntitlementValuePermits
	concrete := "NEWPREFIX.com.example.app"
	structured := []any{map[string]any{"name": "one"}, true}
	tests := []struct {
		name    string
		permits func(profileValue, signedValue any) bool
		profile any
		signed  any
		want    bool
	}{
		{name: "strict terminal wildcard", permits: strict, profile: "NEWPREFIX.*", signed: concrete, want: true},
		{name: "strict bare wildcard", permits: strict, profile: "*", signed: "OLD.value"},
		{name: "strict wildcard without dotted prefix", permits: strict, profile: "TEAM*", signed: "TEAM.value"},
		{name: "strict inner wildcard", permits: strict, profile: "NEWPREFIX.*.suffix", signed: concrete},
		{name: "strict wildcard with suffix", permits: strict, profile: "NEWPREFIX.*suffix", signed: concrete},
		{name: "strict double wildcard", permits: strict, profile: "NEWPREFIX.**", signed: concrete},
		{name: "strict list against string", permits: strict, profile: []any{"TEAM.*"}, signed: "TEAM.value"},
		{name: "strict list with non-string entry", permits: strict, profile: []any{"TEAM.*", 42}, signed: "TEAM.value"},
		{name: "strict list with non-string entry against list", permits: strict, profile: []any{"NEWPREFIX.*", 7}, signed: []any{concrete}},
		{name: "strict list with inner wildcard", permits: strict, profile: []any{"NEWPREFIX.*", "NEWPREFIX.*.suffix"}, signed: []any{concrete}},
		{name: "strict list with suffixed wildcard", permits: strict, profile: []any{"NEWPREFIX.*", "NEWPREFIX.*suffix"}, signed: []any{concrete}},
		{name: "strict exact boolean", permits: strict, profile: true, signed: true, want: true},
		{name: "strict different boolean", permits: strict, profile: true, signed: false},
		{name: "strict exact dictionary", permits: strict, profile: map[string]any{"mode": "strict"}, signed: map[string]any{"mode": "strict"}, want: true},
		{name: "strict exact literal-wildcard array", permits: strict, profile: []any{"literal*", "other"}, signed: []any{"literal*", "other"}, want: true},
		{name: "strict exact structured array", permits: strict, profile: structured, signed: []any{map[string]any{"name": "one"}, true}, want: true},
		{name: "strict reordered structured array", permits: strict, profile: structured, signed: []any{true, map[string]any{"name": "one"}}},
		{name: "legacy bare wildcard", permits: legacy, profile: "*", signed: "OLD.value", want: true},
		{name: "legacy double wildcard", permits: legacy, profile: "NEWPREFIX.**", signed: "NEWPREFIX.*suffix", want: true},
		{name: "legacy non-string list entry", permits: legacy, profile: []any{123}, signed: []any{123}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.permits(test.profile, test.signed); got != test.want {
				t.Fatalf("permits(%#v, %#v) = %t, want %t", test.profile, test.signed, got, test.want)
			}
		})
	}
}

func TestSigningResignRebaseRejectsUnsafeClaimStrings(t *testing.T) {
	unsafe := []string{
		"shared",
		"OLDPREFIX.",
		"OLDPREFIX.com.example.app/child",
		"OLDPREFIX.com.example.app\\child",
		"OLDPREFIX.com.example.app\x00child",
		"OLDPREFIX.com.example.app\u2003child",
		"OLDPREFIX.com.example.app\u202echild",
		string([]byte{'O', 'L', 'D', 0xff, '.', 'x'}),
	}
	for _, value := range unsafe {
		if _, err := signingResignConcreteStringList([]any{value}, signingResignKeychainGroupsEntitlement); err == nil {
			t.Fatalf("keychain value %#v was accepted", value)
		}
		if _, _, err := signingResignKVSValueParts(value, signingResignKVStoreEntitlement); err == nil {
			t.Fatalf("KVS value %#v was accepted", value)
		}
		if _, err := signingResignRebasePrefixedValue(value, signingResignKeychainGroupsEntitlement, "OLDPREFIX", "NEWPREFIX"); err == nil {
			t.Fatalf("prefixed value %#v was accepted", value)
		}
	}
}

func TestPlanSigningResignEntitlementsUsesStrictAuthorizationOnlyWhenRebasing(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		"com.example.claim": "OLD.value",
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		"com.example.claim": "*",
	})
	if _, err := planSigningResignEntitlements(signingResignArchive{MainPath: target.RelativePath, Targets: []signingResignTarget{target}}, map[string]signingResignProfile{target.BundleID: profile}, false); err != nil {
		t.Fatalf("legacy plan rejected historically authorized wildcard: %v", err)
	}
	if _, err := planSigningResignEntitlements(signingResignArchive{MainPath: target.RelativePath, Targets: []signingResignTarget{target}}, map[string]signingResignProfile{target.BundleID: profile}, true); err == nil || !strings.Contains(err.Error(), "com.example.claim") {
		t.Fatalf("rebasing plan error = %v, want strict claim refusal", err)
	}
}

func TestSigningResignNestedAuthorizationStrictRebaseVsLegacy(t *testing.T) {
	entitlements := map[string]any{"com.example.nested": "OLD.value"}
	profile := map[string]any{"com.example.nested": "*"}
	if err := validateSigningResignNestedEntitlements(entitlements, profile); err != nil {
		t.Fatalf("legacy nested authorization rejected wildcard: %v", err)
	}
	if err := validateSigningResignNestedEntitlements(entitlements, profile, true); err == nil {
		t.Fatal("strict nested rebasing authorization accepted bare wildcard")
	}
}

func TestPlanSigningResignEntitlementsRejectsMalformedKeychainProfileArray(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.shared"},
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKeychainGroupsEntitlement: []any{
			"OLDPREFIX.com.example.shared",
			"NEWPREFIX.*.malformed",
		},
	})
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err == nil || !strings.Contains(err.Error(), signingResignKeychainGroupsEntitlement) {
		t.Fatalf("planSigningResignEntitlements() error = %v, want malformed keychain-profile refusal", err)
	}
}

func TestPlanSigningResignEntitlementsRejectsListValuedKVSProfile(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKVStoreEntitlement: "OLDKVS.com.example.app",
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKVStoreEntitlement: []any{"OLDKVS.com.example.app"},
	})
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err == nil || !strings.Contains(err.Error(), signingResignKVStoreEntitlement) {
		t.Fatalf("planSigningResignEntitlements() error = %v, want scalar KVS profile refusal", err)
	}
}

func TestPlanSigningResignEntitlementsDoesNotWildcardApplicationIdentifier(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", nil)
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", nil)
	profile.Entitlements["application-identifier"] = "NEWPREFIX.*"
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err == nil || !strings.Contains(err.Error(), "profile application identifier") {
		t.Fatalf("planSigningResignEntitlements() error = %v, want exact application identifier refusal", err)
	}
}

func TestPlanSigningResignEntitlementsRequiresOneWholeIPAKVSMapping(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKVStoreEntitlement: "OLDPREFIX.com.example.app",
	})
	extension := rebaseTestTarget("app-extension", "Payload/App.app/PlugIns/Feature.appex", "com.example.app.Feature", map[string]any{
		signingResignKVStoreEntitlement: "OLDPREFIX.com.example.app",
	})
	profiles := map[string]signingResignProfile{
		main.BundleID: rebaseTestProfile(main.BundleID, "NEWPREFIX", map[string]any{
			signingResignKVStoreEntitlement: "NEWPREFIX.com.example.app",
		}),
		extension.BundleID: rebaseTestProfile(extension.BundleID, "NEWPREFIX", map[string]any{
			signingResignKVStoreEntitlement: "OTHERKVS.com.example.app",
		}),
	}
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{main, extension},
	}, profiles, true)
	if err == nil || !strings.Contains(err.Error(), "one planned full KVS value") {
		t.Fatalf("planSigningResignEntitlements() error = %v, want one-mapping-per-old-value refusal", err)
	}
}

func TestPlanSigningResignEntitlementsMapsIndependentKVSValuesIndependently(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKVStoreEntitlement: "OLDKVS1.com.example.app",
	})
	other := rebaseTestTarget("app-extension", "Payload/App.app/PlugIns/Other.appex", "com.example.app.Other", map[string]any{
		signingResignKVStoreEntitlement: "OLDKVS2.com.example.other",
	})
	profiles := map[string]signingResignProfile{
		main.BundleID: rebaseTestProfile(main.BundleID, "NEWAPP", map[string]any{
			signingResignKVStoreEntitlement: "NEWKVS1.com.example.app",
		}),
		other.BundleID: rebaseTestProfile(other.BundleID, "NEWAPP", map[string]any{
			signingResignKVStoreEntitlement: "NEWKVS2.com.example.other",
		}),
	}
	plans, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{main, other},
	}, profiles, true)
	if err != nil {
		t.Fatalf("planSigningResignEntitlements() error = %v", err)
	}
	if got := plans[0].Entitlements[signingResignKVStoreEntitlement]; got != "NEWKVS1.com.example.app" {
		t.Fatalf("main KVS = %#v, want independent mapping", got)
	}
	if got := plans[1].Entitlements[signingResignKVStoreEntitlement]; got != "NEWKVS2.com.example.other" {
		t.Fatalf("other KVS = %#v, want independent mapping", got)
	}
}

func TestPlanSigningResignEntitlementsAllowsWildcardKVSAuthorizationForChosenExactValue(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKVStoreEntitlement: "OLDKVS.com.example.app",
	})
	extension := rebaseTestTarget("app-extension", "Payload/App.app/PlugIns/Feature.appex", "com.example.app.Feature", map[string]any{
		signingResignKVStoreEntitlement: "OLDKVS.com.example.app",
	})
	profiles := map[string]signingResignProfile{
		main.BundleID: rebaseTestProfile(main.BundleID, "NEWAPP", map[string]any{
			signingResignKVStoreEntitlement: "NEWKVS.com.example.app",
		}),
		extension.BundleID: rebaseTestProfile(extension.BundleID, "NEWAPP", map[string]any{
			signingResignKVStoreEntitlement: "NEWKVS.*",
		}),
	}
	plans, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{main, extension},
	}, profiles, true)
	if err != nil {
		t.Fatalf("planSigningResignEntitlements() error = %v", err)
	}
	for _, plan := range plans {
		if got := plan.Entitlements[signingResignKVStoreEntitlement]; got != "NEWKVS.com.example.app" {
			t.Fatalf("KVS = %#v, want the one exact candidate authorized by wildcard", got)
		}
	}
}

func TestPlanSigningResignEntitlementsKeepsDefaultCrossTeamRefusal(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.shared"},
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*"},
	})
	_, err := buildSigningResignEntitlements(target.ExistingEntitlements, profile.Entitlements)
	if err == nil || !strings.Contains(err.Error(), "not authorized by the replacement profile") {
		t.Fatalf("buildSigningResignEntitlements() error = %v, want unchanged fail-closed refusal", err)
	}
}

func TestPlanSigningResignEntitlementsPreservesAuthorizedUnrelatedKeychainValues(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{
			"OLDPREFIX.com.example.shared",
			"UNRELATED.com.example.other",
		},
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*", "UNRELATED.com.example.other"},
	})
	plans, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err != nil {
		t.Fatalf("planSigningResignEntitlements() error = %v, want authorized unrelated value preserved", err)
	}
	if got := plans[0].Entitlements[signingResignKeychainGroupsEntitlement]; !signingResignEntitlementValuesEqual(got, []string{
		"NEWPREFIX.com.example.shared",
		"UNRELATED.com.example.other",
	}) {
		t.Fatalf("keychain-access-groups = %#v, want mixed rebase/preserve result", got)
	}
	if len(plans[0].Rewrites) != 1 || plans[0].Rewrites[0].Index == nil || *plans[0].Rewrites[0].Index != 0 {
		t.Fatalf("keychain rewrites = %#v, want only source-prefixed element rewritten", plans[0].Rewrites)
	}
}

func TestPlanSigningResignEntitlementsPreservesWildcardAuthorizedUnrelatedKeychainValues(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{
			"OLDPREFIX.com.example.shared",
			"SHARED.com.example.other",
		},
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*", "SHARED.*"},
	})
	plans, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err != nil {
		t.Fatalf("planSigningResignEntitlements() error = %v, want wildcard-authorized unrelated value preserved", err)
	}
	if got := plans[0].Entitlements[signingResignKeychainGroupsEntitlement]; !signingResignEntitlementValuesEqual(got, []string{
		"NEWPREFIX.com.example.shared",
		"SHARED.com.example.other",
	}) {
		t.Fatalf("keychain-access-groups = %#v, want rebased and wildcard-preserved values", got)
	}
	if len(plans[0].Rewrites) != 1 || plans[0].Rewrites[0].Index == nil || *plans[0].Rewrites[0].Index != 0 {
		t.Fatalf("keychain rewrites = %#v, want only source-prefixed element rewritten", plans[0].Rewrites)
	}
}

func TestPlanSigningResignEntitlementsAuthorizesOnlyTheTransformedValue(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.shared"},
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.com.example.allowed"},
	})
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err == nil || !strings.Contains(err.Error(), "NEWPREFIX.com.example.shared") {
		t.Fatalf("planSigningResignEntitlements() error = %v, want authorization of transformed value", err)
	}
}

func TestPlanSigningResignEntitlementsRejectsUnanchoredProfileWildcard(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.shared"},
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKeychainGroupsEntitlement: []any{"*"},
	})
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err == nil || !strings.Contains(err.Error(), "not authorized by the replacement profile") {
		t.Fatalf("planSigningResignEntitlements() error = %v, want destination-prefix wildcard refusal", err)
	}
}

func TestPlanSigningResignEntitlementsDoesNotGenericRewriteICloudContainers(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		"com.apple.developer.ubiquity-container-identifiers": []string{"OLDPREFIX.iCloud.com.example.app"},
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		"com.apple.developer.ubiquity-container-identifiers": []any{"NEWPREFIX.*"},
	})
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err == nil || !strings.Contains(err.Error(), "not authorized by the replacement profile") {
		t.Fatalf("planSigningResignEntitlements() error = %v, want exact-only refusal for iCloud containers", err)
	}
}

func TestPlanSigningResignEntitlementsPreservesAuthorizedAppClipParentWithoutRewriting(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", nil)
	clip := rebaseTestTarget("app-clip", "Payload/App.app/AppClips/Clip.app", "com.example.app.Clip", map[string]any{
		signingResignParentEntitlement: []string{"OLDPREFIX.com.example.app"},
	})
	profiles := map[string]signingResignProfile{
		main.BundleID: rebaseTestProfile(main.BundleID, "NEWPREFIX", nil),
		clip.BundleID: rebaseTestProfile(clip.BundleID, "NEWPREFIX", map[string]any{
			signingResignParentEntitlement: []any{"OLDPREFIX.com.example.app"},
		}),
	}
	plans, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{main, clip},
	}, profiles, true)
	if err != nil {
		t.Fatalf("planSigningResignEntitlements() error = %v", err)
	}
	if got := plans[1].Entitlements[signingResignParentEntitlement]; !signingResignEntitlementValuesEqual(got, []string{"OLDPREFIX.com.example.app"}) {
		t.Fatalf("parent claim = %#v, want authorized unchanged value", got)
	}
	if len(plans[1].Rewrites) != 0 {
		t.Fatalf("rewrites = %#v, want none", plans[1].Rewrites)
	}
}

func TestPlanSigningResignEntitlementsDoesNotRebaseUnauthorizedAppClipParent(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", nil)
	clip := rebaseTestTarget("app-clip", "Payload/App.app/AppClips/Clip.app", "com.example.app.Clip", map[string]any{
		signingResignParentEntitlement: []string{"OLDPREFIX.com.example.app"},
	})
	profiles := map[string]signingResignProfile{
		main.BundleID: rebaseTestProfile(main.BundleID, "NEWPREFIX", nil),
		clip.BundleID: rebaseTestProfile(clip.BundleID, "NEWPREFIX", map[string]any{
			signingResignParentEntitlement: []any{"NEWPREFIX.com.example.app"},
		}),
	}
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{main, clip},
	}, profiles, true)
	if err == nil || !strings.Contains(err.Error(), signingResignParentEntitlement) {
		t.Fatalf("planSigningResignEntitlements() error = %v, want unauthorized unchanged claim refusal", err)
	}
}

func TestPlanSigningResignEntitlementsRebasesPairedAppClipClaimsTogether(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignAssociatedAppClipEntitlement: []string{"OLDPREFIX.com.example.app.Clip"},
	})
	clip := rebaseTestTarget("app-clip", "Payload/App.app/AppClips/Clip.app", "com.example.app.Clip", map[string]any{
		signingResignParentEntitlement: []string{"OLDPREFIX.com.example.app"},
	})
	profiles := map[string]signingResignProfile{
		main.BundleID: rebaseTestProfile(main.BundleID, "NEWMAIN", map[string]any{
			signingResignAssociatedAppClipEntitlement: []any{"NEWCLIP.com.example.app.Clip"},
		}),
		clip.BundleID: rebaseTestProfile(clip.BundleID, "NEWCLIP", map[string]any{
			signingResignParentEntitlement: []any{"NEWMAIN.com.example.app"},
		}),
	}
	plans, err := planSigningResignEntitlements(signingResignArchive{MainPath: main.RelativePath, Targets: []signingResignTarget{main, clip}}, profiles, true)
	if err != nil {
		t.Fatalf("plan paired App Clip claims: %v", err)
	}
	if got := plans[0].Entitlements[signingResignAssociatedAppClipEntitlement]; !signingResignEntitlementValuesEqual(got, []string{"NEWCLIP.com.example.app.Clip"}) {
		t.Fatalf("associated claim = %#v", got)
	}
	if got := plans[1].Entitlements[signingResignParentEntitlement]; !signingResignEntitlementValuesEqual(got, []string{"NEWMAIN.com.example.app"}) {
		t.Fatalf("parent claim = %#v", got)
	}
	if len(plans[0].Rewrites) != 1 || len(plans[1].Rewrites) != 1 {
		t.Fatalf("rewrites = %#v, %#v", plans[0].Rewrites, plans[1].Rewrites)
	}
}

func TestPlanSigningResignEntitlementsPreservesDuplicatePairedAppClipArrayEntries(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{signingResignAssociatedAppClipEntitlement: []string{"OLDPREFIX.com.example.app.Clip", "OLDPREFIX.com.example.app.Clip"}})
	clip := rebaseTestTarget("app-clip", "Payload/App.app/AppClips/Clip.app", "com.example.app.Clip", map[string]any{signingResignParentEntitlement: []string{"OLDPREFIX.com.example.app", "OLDPREFIX.com.example.app"}})
	profiles := map[string]signingResignProfile{main.BundleID: rebaseTestProfile(main.BundleID, "NEWMAIN", map[string]any{signingResignAssociatedAppClipEntitlement: []any{"NEWCLIP.com.example.app.Clip"}}), clip.BundleID: rebaseTestProfile(clip.BundleID, "NEWCLIP", map[string]any{signingResignParentEntitlement: []any{"NEWMAIN.com.example.app"}})}
	plans, err := planSigningResignEntitlements(signingResignArchive{MainPath: main.RelativePath, Targets: []signingResignTarget{main, clip}}, profiles, true)
	if err != nil {
		t.Fatalf("plan duplicate paired claims: %v", err)
	}
	if got := plans[0].Entitlements[signingResignAssociatedAppClipEntitlement]; !signingResignEntitlementValuesEqual(got, []string{"NEWCLIP.com.example.app.Clip", "NEWCLIP.com.example.app.Clip"}) {
		t.Fatalf("associated claim = %#v", got)
	}
	if got := plans[1].Entitlements[signingResignParentEntitlement]; !signingResignEntitlementValuesEqual(got, []string{"NEWMAIN.com.example.app", "NEWMAIN.com.example.app"}) {
		t.Fatalf("parent claim = %#v", got)
	}
	if len(plans[0].Rewrites) != 2 || len(plans[1].Rewrites) != 2 {
		t.Fatalf("rewrites = %#v %#v, want two per side", plans[0].Rewrites, plans[1].Rewrites)
	}
}

func TestPlanSigningResignEntitlementsPreservesPairedClaimsOnMismatchedPrefix(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{signingResignAssociatedAppClipEntitlement: []string{"OLDPREFIX.com.example.app.Clip"}})
	clip := rebaseTestTarget("app-clip", "Payload/App.app/AppClips/Clip.app", "com.example.app.Clip", map[string]any{signingResignParentEntitlement: []string{"WRONGPREFIX.com.example.app"}})
	profiles := map[string]signingResignProfile{main.BundleID: rebaseTestProfile(main.BundleID, "NEWMAIN", map[string]any{signingResignAssociatedAppClipEntitlement: []any{"OLDPREFIX.com.example.app.Clip"}}), clip.BundleID: rebaseTestProfile(clip.BundleID, "NEWCLIP", map[string]any{signingResignParentEntitlement: []any{"WRONGPREFIX.com.example.app"}})}
	plans, err := planSigningResignEntitlements(signingResignArchive{MainPath: main.RelativePath, Targets: []signingResignTarget{main, clip}}, profiles, true)
	if err != nil {
		t.Fatalf("plan mismatched prefix: %v", err)
	}
	if len(plans[0].Rewrites) != 0 || len(plans[1].Rewrites) != 0 {
		t.Fatalf("one-sided rewrites: %#v %#v", plans[0].Rewrites, plans[1].Rewrites)
	}
}

func TestPlanSigningResignEntitlementsRejectsMixedAppClipArrayWhenUnauthorized(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{signingResignAssociatedAppClipEntitlement: []string{"OLDPREFIX.com.example.app.Clip", "OLDPREFIX.com.example.missing"}})
	clip := rebaseTestTarget("app-clip", "Payload/App.app/AppClips/Clip.app", "com.example.app.Clip", map[string]any{signingResignParentEntitlement: []string{"OLDPREFIX.com.example.app"}})
	profiles := map[string]signingResignProfile{main.BundleID: rebaseTestProfile(main.BundleID, "NEWMAIN", map[string]any{signingResignAssociatedAppClipEntitlement: []any{"NEWCLIP.com.example.app.Clip", "NEWCLIP.com.example.missing"}}), clip.BundleID: rebaseTestProfile(clip.BundleID, "NEWCLIP", map[string]any{signingResignParentEntitlement: []any{"NEWMAIN.com.example.app"}})}
	_, err := planSigningResignEntitlements(signingResignArchive{MainPath: main.RelativePath, Targets: []signingResignTarget{main, clip}}, profiles, true)
	if err == nil || !strings.Contains(err.Error(), signingResignAssociatedAppClipEntitlement) {
		t.Fatalf("expected mixed-array refusal, got %v", err)
	}
}

func TestPlanSigningResignEntitlementsRejectsMultipleAppClipEdges(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{signingResignAssociatedAppClipEntitlement: []string{"OLDPREFIX.com.example.one", "OLDPREFIX.com.example.two"}})
	one := rebaseTestTarget("app-clip", "Payload/App.app/AppClips/One.app", "com.example.one", map[string]any{signingResignParentEntitlement: []string{"OLDPREFIX.com.example.app"}})
	two := rebaseTestTarget("app-clip", "Payload/App.app/AppClips/Two.app", "com.example.two", map[string]any{signingResignParentEntitlement: []string{"OLDPREFIX.com.example.app"}})
	profiles := map[string]signingResignProfile{main.BundleID: rebaseTestProfile(main.BundleID, "NEWMAIN", map[string]any{signingResignAssociatedAppClipEntitlement: []any{"NEWONE.com.example.one", "NEWTWO.com.example.two"}}), one.BundleID: rebaseTestProfile(one.BundleID, "NEWONE", map[string]any{signingResignParentEntitlement: []any{"NEWMAIN.com.example.app"}}), two.BundleID: rebaseTestProfile(two.BundleID, "NEWTWO", map[string]any{signingResignParentEntitlement: []any{"NEWMAIN.com.example.app"}})}
	_, err := planSigningResignEntitlements(signingResignArchive{MainPath: main.RelativePath, Targets: []signingResignTarget{main, one, two}}, profiles, true)
	if err == nil || !strings.Contains(err.Error(), "not authorized") {
		t.Fatalf("expected multiple-edge refusal, got %v", err)
	}
}

func TestPlanSigningResignEntitlementsDoesNotRewriteAssociatedAppClipClaim(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		"com.apple.developer.associated-appclip-app-identifiers": []string{"OLDPREFIX.com.example.app.Clip"},
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		"com.apple.developer.associated-appclip-app-identifiers": []any{"OLDPREFIX.com.example.app.Clip"},
	})
	plans, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err != nil {
		t.Fatalf("planSigningResignEntitlements() error = %v", err)
	}
	if got := plans[0].Entitlements[signingResignAssociatedAppClipEntitlement]; !signingResignEntitlementValuesEqual(got, []string{"OLDPREFIX.com.example.app.Clip"}) {
		t.Fatalf("associated claim = %#v, want authorized unchanged value", got)
	}
	if len(plans[0].Rewrites) != 0 {
		t.Fatalf("rewrites = %#v, want none", plans[0].Rewrites)
	}
}

func TestPlanSigningResignEntitlementsDoesNotRebaseUnauthorizedAssociatedAppClipClaim(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignAssociatedAppClipEntitlement: []string{"OLDPREFIX.com.example.app.Clip"},
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignAssociatedAppClipEntitlement: []any{"NEWPREFIX.com.example.app.Clip"},
	})
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err == nil || !strings.Contains(err.Error(), signingResignAssociatedAppClipEntitlement) {
		t.Fatalf("planSigningResignEntitlements() error = %v, want unchanged associated authorization refusal", err)
	}
}

func TestPlanSigningResignEntitlementsRejectsMalformedAssociatedAppClipProfile(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignAssociatedAppClipEntitlement: []string{"OLDPREFIX.com.example.app.Clip"},
	})
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignAssociatedAppClipEntitlement: []any{
			"OLDPREFIX.com.example.app.Clip",
			"OLDPREFIX.*.malformed",
		},
	})
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err == nil || !strings.Contains(err.Error(), signingResignAssociatedAppClipEntitlement) {
		t.Fatalf("planSigningResignEntitlements() error = %v, want malformed associated-profile refusal", err)
	}
}

func TestPlanSigningResignEntitlementsRejectsMalformedAppClipRelationshipSuffix(t *testing.T) {
	for _, profileClaim := range []any{
		[]any{"OLDPREFIX.com..example"},
		[]any{"OLDPREFIX.*"},
	} {
		t.Run(fmt.Sprintf("profile-%v", profileClaim), func(t *testing.T) {
			target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
				signingResignAssociatedAppClipEntitlement: []string{"OLDPREFIX.com..example"},
			})
			profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
				signingResignAssociatedAppClipEntitlement: profileClaim,
			})
			_, err := planSigningResignEntitlements(signingResignArchive{
				MainPath: "Payload/App.app",
				Targets:  []signingResignTarget{target},
			}, map[string]signingResignProfile{target.BundleID: profile}, true)
			if err == nil || !strings.Contains(err.Error(), "invalid relationship suffix") {
				t.Fatalf("planSigningResignEntitlements() error = %v, want malformed relationship refusal", err)
			}
		})
	}
}

func TestPlanSigningResignEntitlementsAllowsDifferentPrefixesWithoutRebase(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", nil)
	extension := rebaseTestTarget("app-extension", "Payload/App.app/PlugIns/Feature.appex", "com.example.app.Feature", nil)
	profiles := map[string]signingResignProfile{
		main.BundleID:      rebaseTestProfile(main.BundleID, "NEWPREFIX", nil),
		extension.BundleID: rebaseTestProfile(extension.BundleID, "OTHERPREFIX", nil),
	}
	if _, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{main, extension},
	}, profiles, false); err != nil {
		t.Fatalf("planSigningResignEntitlements() without rebase error = %v, want legacy behavior", err)
	}
}

func TestBuildSigningResignRebaseGraphAllowsIndependentDestinationPrefixes(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.main"},
	})
	other := rebaseTestTarget("app-extension", "Payload/App.app/PlugIns/Other.appex", "com.example.other", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.other"},
	})
	profiles := map[string]signingResignProfile{
		main.BundleID: rebaseTestProfile(main.BundleID, "NEWPREFIX", map[string]any{
			signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*"},
		}),
		other.BundleID: rebaseTestProfile(other.BundleID, "OTHERPREFIX", map[string]any{
			signingResignKeychainGroupsEntitlement: []any{"OTHERPREFIX.*"},
		}),
	}
	graph, err := buildSigningResignRebaseGraph(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{main, other},
	}, profiles)
	if err != nil {
		t.Fatalf("buildSigningResignRebaseGraph() error = %v, want independent prefixes accepted", err)
	}
	if graph.KeychainMapping["OLDPREFIX.com.example.main"] != "NEWPREFIX.com.example.main" || graph.KeychainMapping["OLDPREFIX.com.example.other"] != "OTHERPREFIX.com.example.other" {
		t.Fatalf("keychain mapping = %#v, want independent destination prefixes", graph.KeychainMapping)
	}
}

func TestPlanSigningResignEntitlementsRejectsKeychainMappingCollision(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.shared"},
	})
	other := rebaseTestTarget("app-extension", "Payload/App.app/PlugIns/Other.appex", "com.example.other", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OTHEROLD.com.example.shared"},
	})
	other.ExistingEntitlements["application-identifier"] = "OTHEROLD." + other.BundleID
	other.ExistingEntitlements["com.apple.application-identifier"] = "OTHEROLD." + other.BundleID
	profiles := map[string]signingResignProfile{
		main.BundleID: rebaseTestProfile(main.BundleID, "NEWPREFIX", map[string]any{
			signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*"},
		}),
		other.BundleID: rebaseTestProfile(other.BundleID, "NEWPREFIX", map[string]any{
			signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*"},
		}),
	}
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{main, other},
	}, profiles, true)
	if err == nil || !strings.Contains(err.Error(), "distinct full keychain values one-to-one") {
		t.Fatalf("planSigningResignEntitlements() error = %v, want keychain one-to-one collision refusal", err)
	}
}

func TestPlanSigningResignEntitlementsRejectsSharedKeychainPreserveAndRebase(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.shared"},
	})
	other := rebaseTestTarget("app-extension", "Payload/App.app/PlugIns/Other.appex", "com.example.other", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.shared"},
	})
	profiles := map[string]signingResignProfile{
		main.BundleID: rebaseTestProfile(main.BundleID, "NEWPREFIX", map[string]any{
			signingResignKeychainGroupsEntitlement: []any{"OLDPREFIX.com.example.shared"},
		}),
		other.BundleID: rebaseTestProfile(other.BundleID, "NEWPREFIX", map[string]any{
			signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*"},
		}),
	}
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{main, other},
	}, profiles, true)
	if err == nil || !strings.Contains(err.Error(), "one planned full keychain value") {
		t.Fatalf("planSigningResignEntitlements() error = %v, want shared keychain mapping conflict", err)
	}
}

func TestPlanSigningResignEntitlementsRejectsSharedKeychainDifferentDestinations(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.shared"},
	})
	other := rebaseTestTarget("app-extension", "Payload/App.app/PlugIns/Other.appex", "com.example.other", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.shared"},
	})
	profiles := map[string]signingResignProfile{
		main.BundleID: rebaseTestProfile(main.BundleID, "NEWPREFIX", map[string]any{
			signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*"},
		}),
		other.BundleID: rebaseTestProfile(other.BundleID, "OTHERPREFIX", map[string]any{
			signingResignKeychainGroupsEntitlement: []any{"OTHERPREFIX.*"},
		}),
	}
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{main, other},
	}, profiles, true)
	if err == nil || !strings.Contains(err.Error(), "one planned full keychain value") {
		t.Fatalf("planSigningResignEntitlements() error = %v, want differing shared keychain destinations refused", err)
	}
}

func TestPlanSigningResignEntitlementsRejectsKVStoreMappingCollision(t *testing.T) {
	main := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKVStoreEntitlement: "OLDKVS1.com.example.shared",
	})
	other := rebaseTestTarget("app-extension", "Payload/App.app/PlugIns/Other.appex", "com.example.other", map[string]any{
		signingResignKVStoreEntitlement: "OLDKVS2.com.example.shared",
	})
	profiles := map[string]signingResignProfile{
		main.BundleID: rebaseTestProfile(main.BundleID, "NEWAPP", map[string]any{
			signingResignKVStoreEntitlement: "NEWKVS.com.example.shared",
		}),
		other.BundleID: rebaseTestProfile(other.BundleID, "NEWAPP", map[string]any{
			signingResignKVStoreEntitlement: "NEWKVS.com.example.shared",
		}),
	}
	_, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{main, other},
	}, profiles, true)
	if err == nil || !strings.Contains(err.Error(), "distinct full KVS values one-to-one") {
		t.Fatalf("planSigningResignEntitlements() error = %v, want KVS one-to-one collision refusal", err)
	}
}

func TestPlanSigningResignEntitlementsDoesNotMutateInputClaims(t *testing.T) {
	target := rebaseTestTarget("application", "Payload/App.app", "com.example.app", map[string]any{
		signingResignKeychainGroupsEntitlement: []string{"OLDPREFIX.com.example.shared"},
	})
	original := append([]string(nil), target.ExistingEntitlements[signingResignKeychainGroupsEntitlement].([]string)...)
	profile := rebaseTestProfile(target.BundleID, "NEWPREFIX", map[string]any{
		signingResignKeychainGroupsEntitlement: []any{"NEWPREFIX.*"},
	})
	plans, err := planSigningResignEntitlements(signingResignArchive{
		MainPath: "Payload/App.app",
		Targets:  []signingResignTarget{target},
	}, map[string]signingResignProfile{target.BundleID: profile}, true)
	if err != nil {
		t.Fatalf("planSigningResignEntitlements() error = %v", err)
	}
	planned := plans[0].Entitlements[signingResignKeychainGroupsEntitlement].([]string)
	planned[0] = "MUTATED-PLAN-VALUE"
	if got := target.ExistingEntitlements[signingResignKeychainGroupsEntitlement].([]string); !signingResignEntitlementValuesEqual(got, original) {
		t.Fatalf("input entitlement changed through plan = %#v, want %#v", got, original)
	}
}

func TestSigningResignEntitlementRewriteOrderIsCanonical(t *testing.T) {
	arrayIndex := 1
	scalar := signingResignOutputEntitlementRewrite{
		TargetRelativePath: "Payload/App.app",
		BundleID:           "com.example.app",
		Key:                signingResignKeychainGroupsEntitlement,
		From:               "OLDPREFIX.scalar",
		To:                 "NEWPREFIX.scalar",
	}
	array := signingResignOutputEntitlementRewrite{
		TargetRelativePath: "Payload/App.app",
		BundleID:           "com.example.app",
		Key:                signingResignKeychainGroupsEntitlement,
		ElementIndex:       &arrayIndex,
		From:               "OLDPREFIX.array",
		To:                 "NEWPREFIX.array",
	}
	otherTarget := scalar
	otherTarget.TargetRelativePath = "Payload/App.app/PlugIns/Feature.appex"
	values := []signingResignOutputEntitlementRewrite{array, otherTarget, scalar}
	sort.SliceStable(values, func(left, right int) bool {
		return signingResignOutputEntitlementRewriteLess(values[left], values[right])
	})
	if values[0].ElementIndex != nil || values[1].ElementIndex == nil || values[2].TargetRelativePath != otherTarget.TargetRelativePath {
		t.Fatalf("canonical rewrite order = %#v, want scalar, array, then later target", values)
	}
}
