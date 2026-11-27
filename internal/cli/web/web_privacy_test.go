package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestDeclarationToTupleSetNotCollected(t *testing.T) {
	tuples, err := declarationToTupleSet(privacyDeclarationFile{
		SchemaVersion: privacySchemaVersion,
		DataUsages: []privacyUsage{
			{
				DataProtections: []string{dataProtectionNotCollected},
			},
		},
	})
	if err != nil {
		t.Fatalf("declarationToTupleSet() error = %v", err)
	}
	if len(tuples) != 1 {
		t.Fatalf("expected one tuple, got %d", len(tuples))
	}
	wantKey := privacyTupleKey(privacyTuple{DataProtection: dataProtectionNotCollected})
	if _, ok := tuples[wantKey]; !ok {
		t.Fatalf("expected not-collected tuple key %q, got %#v", wantKey, tuples)
	}
}

func TestDeclarationToTupleSetRejectsCategoryForNotCollected(t *testing.T) {
	_, err := declarationToTupleSet(privacyDeclarationFile{
		SchemaVersion: privacySchemaVersion,
		DataUsages: []privacyUsage{
			{
				Category:        "NAME",
				DataProtections: []string{dataProtectionNotCollected},
			},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "cannot include category") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeclarationToTupleSetRejectsCollectedWithoutPurpose(t *testing.T) {
	_, err := declarationToTupleSet(privacyDeclarationFile{
		SchemaVersion: privacySchemaVersion,
		DataUsages: []privacyUsage{
			{
				Category:        "NAME",
				DataProtections: []string{dataProtectionLinked},
			},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "purposes is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeclarationToTupleSetAllowsTrackingWithoutPurpose(t *testing.T) {
	tuples, err := declarationToTupleSet(privacyDeclarationFile{
		SchemaVersion: privacySchemaVersion,
		DataUsages: []privacyUsage{
			{
				Category:        "PURCHASE_HISTORY",
				DataProtections: []string{dataProtectionTracking},
			},
		},
	})
	if err != nil {
		t.Fatalf("declarationToTupleSet() error = %v", err)
	}
	wantKey := privacyTupleKey(privacyTuple{
		Category:       "PURCHASE_HISTORY",
		Purpose:        "",
		DataProtection: dataProtectionTracking,
	})
	if _, ok := tuples[wantKey]; !ok {
		t.Fatalf("expected tracking tuple key %q, got %#v", wantKey, tuples)
	}
}

func TestDeclarationToTupleSetCanonicalizesTrackingPurposeAway(t *testing.T) {
	tuples, err := declarationToTupleSet(privacyDeclarationFile{
		SchemaVersion: privacySchemaVersion,
		DataUsages: []privacyUsage{
			{
				Category: "PURCHASE_HISTORY",
				Purposes: []string{"APP_FUNCTIONALITY"},
				DataProtections: []string{
					dataProtectionLinked,
					dataProtectionTracking,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("declarationToTupleSet() error = %v", err)
	}
	trackingCanonicalKey := privacyTupleKey(privacyTuple{
		Category:       "PURCHASE_HISTORY",
		Purpose:        "",
		DataProtection: dataProtectionTracking,
	})
	if _, ok := tuples[trackingCanonicalKey]; !ok {
		t.Fatalf("expected canonical tracking tuple key %q, got %#v", trackingCanonicalKey, tuples)
	}
	trackingWithPurposeKey := privacyTupleKey(privacyTuple{
		Category:       "PURCHASE_HISTORY",
		Purpose:        "APP_FUNCTIONALITY",
		DataProtection: dataProtectionTracking,
	})
	if _, ok := tuples[trackingWithPurposeKey]; ok {
		t.Fatalf("tracking tuple should not retain purpose; got %#v", tuples)
	}
}

func TestDeclarationToTupleSetRejectsMixedNotCollectedAndCollected(t *testing.T) {
	cases := []struct {
		name   string
		usages []privacyUsage
	}{
		{
			name: "not_collected_then_collected",
			usages: []privacyUsage{
				{DataProtections: []string{dataProtectionNotCollected}},
				{
					Category:        "NAME",
					Purposes:        []string{"APP_FUNCTIONALITY"},
					DataProtections: []string{dataProtectionLinked},
				},
			},
		},
		{
			name: "collected_then_not_collected",
			usages: []privacyUsage{
				{
					Category:        "NAME",
					Purposes:        []string{"APP_FUNCTIONALITY"},
					DataProtections: []string{dataProtectionLinked},
				},
				{DataProtections: []string{dataProtectionNotCollected}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := declarationToTupleSet(privacyDeclarationFile{
				SchemaVersion: privacySchemaVersion,
				DataUsages:    tc.usages,
			})
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "cannot be combined") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDeclarationFromTupleSetGroupsByCategoryAndPurpose(t *testing.T) {
	declaration := declarationFromTupleSet(map[string]privacyTuple{
		privacyTupleKey(privacyTuple{
			Category:       "NAME",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		}): {
			Category:       "NAME",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		},
		privacyTupleKey(privacyTuple{
			Category:       "NAME",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionTracking,
		}): {
			Category:       "NAME",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionTracking,
		},
	})

	if declaration.SchemaVersion != privacySchemaVersion {
		t.Fatalf("expected schemaVersion=%d, got %d", privacySchemaVersion, declaration.SchemaVersion)
	}
	if len(declaration.DataUsages) != 1 {
		t.Fatalf("expected one usage group, got %d", len(declaration.DataUsages))
	}
	got := declaration.DataUsages[0]
	if got.Category != "NAME" || len(got.Purposes) != 1 || got.Purposes[0] != "APP_FUNCTIONALITY" {
		t.Fatalf("unexpected grouped usage: %#v", got)
	}
	if !reflect.DeepEqual(got.DataProtections, []string{dataProtectionLinked, dataProtectionTracking}) {
		t.Fatalf("unexpected protections: %#v", got.DataProtections)
	}
}

func TestDeclarationFromRemoteDataUsagesEmptyDefaultsNotCollected(t *testing.T) {
	declaration := declarationFromRemoteDataUsages(nil)

	if declaration.SchemaVersion != privacySchemaVersion {
		t.Fatalf("expected schemaVersion=%d, got %d", privacySchemaVersion, declaration.SchemaVersion)
	}
	if len(declaration.DataUsages) != 1 {
		t.Fatalf("expected one default data usage, got %d", len(declaration.DataUsages))
	}
	if !reflect.DeepEqual(declaration.DataUsages[0].DataProtections, []string{dataProtectionNotCollected}) {
		t.Fatalf("unexpected default declaration: %#v", declaration.DataUsages[0])
	}
	if declaration.DataUsages[0].Category != "" || len(declaration.DataUsages[0].Purposes) != 0 {
		t.Fatalf("expected DATA_NOT_COLLECTED declaration with empty category/purposes, got %#v", declaration.DataUsages[0])
	}
}

func TestDeclarationFromRemoteDataUsagesKeepsNotCollectedProtection(t *testing.T) {
	usages := []webcore.AppDataUsage{
		{
			ID:             "u1",
			DataProtection: dataProtectionNotCollected,
		},
	}

	declaration := declarationFromRemoteDataUsages(usages)
	if declaration.SchemaVersion != privacySchemaVersion {
		t.Fatalf("expected schemaVersion=%d, got %d", privacySchemaVersion, declaration.SchemaVersion)
	}
	if len(declaration.DataUsages) != 1 {
		t.Fatalf("expected one data usage, got %#v", declaration.DataUsages)
	}
	got := declaration.DataUsages[0]
	if !reflect.DeepEqual(got.DataProtections, []string{dataProtectionNotCollected}) {
		t.Fatalf("expected DATA_NOT_COLLECTED to remain representable, got %#v", got)
	}
	if got.Category != "" || len(got.Purposes) != 0 {
		t.Fatalf("expected DATA_NOT_COLLECTED declaration with empty category/purposes, got %#v", got)
	}
	if count := countUnrepresentableRemoteUsages(usages); count != 0 {
		t.Fatalf("expected unrepresentableCount=0 for DATA_NOT_COLLECTED, got %d", count)
	}

	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal usage: %v", err)
	}
	if string(raw) != `{"dataProtections":["DATA_NOT_COLLECTED"]}` {
		t.Fatalf("expected canonical not-collected JSON without empty category/purposes, got %s", raw)
	}
}

func TestDeclarationFromRemoteDataUsagesMalformedOnlyPreservesUnrepresentable(t *testing.T) {
	declaration := declarationFromRemoteDataUsages([]webcore.AppDataUsage{
		{
			ID:       "usage-malformed-1",
			Category: "PURCHASE_HISTORY",
			Purpose:  "APP_FUNCTIONALITY",
		},
	})

	if len(declaration.DataUsages) != 1 {
		t.Fatalf("expected one unrepresentable usage, got %#v", declaration.DataUsages)
	}
	got := declaration.DataUsages[0]
	if containsPrivacyProtection(got.DataProtections, dataProtectionNotCollected) {
		t.Fatalf("non-empty malformed remote usages must not collapse to DATA_NOT_COLLECTED: %#v", got)
	}
	if !reflect.DeepEqual(got.DataProtections, []string{dataProtectionUnknown}) {
		t.Fatalf("expected opaque %s marker, got %#v", dataProtectionUnknown, got)
	}
	if got.Category != "PURCHASE_HISTORY" {
		t.Fatalf("expected malformed category to be preserved, got %#v", got)
	}
}

func TestDeclarationFromRemoteDataUsagesPreservesMalformedWhenValidPresent(t *testing.T) {
	declaration := declarationFromRemoteDataUsages([]webcore.AppDataUsage{
		{
			ID:       "usage-malformed-1",
			Category: "PURCHASE_HISTORY",
			Purpose:  "APP_FUNCTIONALITY",
		},
		{
			ID:             "usage-valid-1",
			Category:       "PURCHASE_HISTORY",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		},
	})

	if len(declaration.DataUsages) != 1 {
		t.Fatalf("expected one grouped usage, got %#v", declaration.DataUsages)
	}
	got := declaration.DataUsages[0]
	if got.Category != "PURCHASE_HISTORY" {
		t.Fatalf("unexpected declaration category: %#v", got)
	}
	if containsPrivacyProtection(got.DataProtections, dataProtectionNotCollected) {
		t.Fatalf("mixed remote usages must not collapse malformed entries to DATA_NOT_COLLECTED: %#v", got)
	}
	if !reflect.DeepEqual(got.DataProtections, []string{dataProtectionLinked, dataProtectionUnknown}) {
		t.Fatalf("expected valid and unrepresentable protections, got %#v", got.DataProtections)
	}
}

func TestPlanFromDesiredAndRemoteIncludesDuplicateRemoteDeletes(t *testing.T) {
	desired := map[string]privacyTuple{
		privacyTupleKey(privacyTuple{
			Category:       "NAME",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		}): {
			Category:       "NAME",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		},
	}
	remote := map[string]privacyRemoteState{
		privacyTupleKey(privacyTuple{
			Category:       "NAME",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		}): {
			Tuple: privacyTuple{
				Category:       "NAME",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionLinked,
			},
			UsageIDs: []string{"usage-1", "usage-2"},
		},
	}

	plan := planFromDesiredAndRemote("123", "./privacy.json", desired, remote)
	if len(plan.Adds) != 0 {
		t.Fatalf("expected no adds, got %#v", plan.Adds)
	}
	if len(plan.Deletes) != 1 {
		t.Fatalf("expected one duplicate delete, got %#v", plan.Deletes)
	}
	if plan.Deletes[0].UsageID != "usage-2" {
		t.Fatalf("expected usage-2 delete, got %#v", plan.Deletes[0])
	}
}

func TestPlanFromDesiredAndRemoteSkipsDeletesWithoutUsageID(t *testing.T) {
	desired := map[string]privacyTuple{}
	remote := map[string]privacyRemoteState{
		privacyTupleKey(privacyTuple{
			Category:       "NAME",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		}): {
			Tuple: privacyTuple{
				Category:       "NAME",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionLinked,
			},
			UsageIDs: nil,
		},
	}

	plan := planFromDesiredAndRemote("123", "./privacy.json", desired, remote)
	if len(plan.Deletes) != 0 {
		t.Fatalf("expected no deletes for remote tuples without usage IDs, got %#v", plan.Deletes)
	}
	if len(plan.SkippedDeletes) != 1 {
		t.Fatalf("expected one skipped delete for missing usage id, got %#v", plan.SkippedDeletes)
	}
	if plan.SkippedDeletes[0].Reason != "missing_usage_id" {
		t.Fatalf("expected missing_usage_id reason, got %#v", plan.SkippedDeletes[0])
	}
	if len(plan.APICalls) != 0 {
		t.Fatalf("expected no delete api calls for remote tuples without usage IDs, got %#v", plan.APICalls)
	}
}

func TestPlanFromDesiredAndRemoteKeepsAMatchingIDLessTuple(t *testing.T) {
	key := privacyTupleKey(privacyTuple{
		Category:       "NAME",
		Purpose:        "APP_FUNCTIONALITY",
		DataProtection: dataProtectionLinked,
	})
	tuple := privacyTuple{
		Category:       "NAME",
		Purpose:        "APP_FUNCTIONALITY",
		DataProtection: dataProtectionLinked,
	}
	plan := planFromDesiredAndRemote("123", "./privacy.json", map[string]privacyTuple{key: tuple}, map[string]privacyRemoteState{
		key: {
			Tuple:       tuple,
			IDLessCount: 1,
		},
	})
	if len(plan.Adds) != 0 || len(plan.Deletes) != 0 || len(plan.SkippedDeletes) != 0 {
		t.Fatalf("a single ID-less remote tuple that matches the file is already converged: %#v", plan)
	}
	if !privacyApplyConverged(plan, privacyApplyResult{}) {
		t.Fatal("keeping the matching ID-less tuple must not block convergence")
	}

	extra := planFromDesiredAndRemote("123", "./privacy.json", map[string]privacyTuple{key: tuple}, map[string]privacyRemoteState{
		key: {
			Tuple:       tuple,
			IDLessCount: 2,
		},
	})
	if len(extra.SkippedDeletes) != 1 || extra.SkippedDeletes[0].Reason != "missing_usage_id" {
		t.Fatalf("a second ID-less member of a desired key is still an extra: %#v", extra.SkippedDeletes)
	}
	if privacyApplyConverged(extra, privacyApplyResult{}) {
		t.Fatal("an extra ID-less member of a desired key is not convergence")
	}
}

func TestPlanFromDesiredAndRemoteSkipsIDLessExtrasAlongsideIdentifiedUsages(t *testing.T) {
	key := privacyTupleKey(privacyTuple{
		Category:       "NAME",
		Purpose:        "APP_FUNCTIONALITY",
		DataProtection: dataProtectionLinked,
	})
	tuple := privacyTuple{
		Category:       "NAME",
		Purpose:        "APP_FUNCTIONALITY",
		DataProtection: dataProtectionLinked,
	}
	plan := planFromDesiredAndRemote("123", "./privacy.json", map[string]privacyTuple{key: tuple}, map[string]privacyRemoteState{
		key: {
			Tuple:       tuple,
			UsageIDs:    []string{"usage-1"},
			IDLessCount: 1,
		},
	})
	if len(plan.Deletes) != 0 {
		t.Fatalf("the identified usage matches the file, so it is not deleted: %#v", plan.Deletes)
	}
	if len(plan.SkippedDeletes) != 1 || plan.SkippedDeletes[0].Reason != "missing_usage_id" {
		t.Fatalf("the ID-less extra must remain a skipped delete: %#v", plan.SkippedDeletes)
	}
	if privacyApplyConverged(plan, privacyApplyResult{}) {
		t.Fatal("an ID-less extra next to a matching usage is not convergence")
	}
}

func TestRemoteStateFromDataUsagesCountsIDLessMembersInMixedGroups(t *testing.T) {
	state := remoteStateFromDataUsages([]webcore.AppDataUsage{
		{
			ID:             "usage-1",
			Category:       "NAME",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		},
		{
			Category:       "NAME",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		},
	})
	key := privacyTupleKey(privacyTuple{
		Category:       "NAME",
		Purpose:        "APP_FUNCTIONALITY",
		DataProtection: dataProtectionLinked,
	})
	got := state[key]
	if !reflect.DeepEqual(got.UsageIDs, []string{"usage-1"}) {
		t.Fatalf("expected the identified usage to be kept, got %#v", got)
	}
	if got.IDLessCount != 1 {
		t.Fatalf("expected one ID-less sibling, got %#v", got)
	}
}

func TestPlanFromDesiredAndRemoteIncludesDeleteForMalformedRemoteUsage(t *testing.T) {
	desired := map[string]privacyTuple{
		privacyTupleKey(privacyTuple{
			Category:       "PURCHASE_HISTORY",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		}): {
			Category:       "PURCHASE_HISTORY",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		},
	}
	remote := remoteStateFromDataUsages([]webcore.AppDataUsage{
		{
			ID:             "usage-valid-1",
			Category:       "PURCHASE_HISTORY",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		},
		{
			ID:       "usage-malformed-1",
			Category: "PURCHASE_HISTORY",
			Purpose:  "APP_FUNCTIONALITY",
		},
	})

	plan := planFromDesiredAndRemote("123", "./privacy.json", desired, remote)
	if len(plan.Adds) != 0 || len(plan.Updates) != 0 {
		t.Fatalf("expected no adds/updates, got adds=%#v updates=%#v", plan.Adds, plan.Updates)
	}
	if len(plan.Deletes) != 1 {
		t.Fatalf("expected one delete for malformed remote usage, got %#v", plan.Deletes)
	}
	if plan.Deletes[0].UsageID != "usage-malformed-1" || plan.Deletes[0].DataProtection != dataProtectionUnknown {
		t.Fatalf("unexpected delete for malformed usage: %#v", plan.Deletes[0])
	}
	if len(plan.APICalls) != 1 || plan.APICalls[0].Operation != "delete_data_usage" || plan.APICalls[0].Count != 1 {
		t.Fatalf("unexpected api call summary: %#v", plan.APICalls)
	}
}

func TestPlanFromDesiredAndRemotePairsVerifiedIdentityFlipIntoUpdate(t *testing.T) {
	desiredTuple := privacyTuple{Category: "EMAIL_ADDRESS", Purpose: "APP_FUNCTIONALITY", DataProtection: dataProtectionNotLinked}
	remoteTuple := privacyTuple{Category: "EMAIL_ADDRESS", Purpose: "APP_FUNCTIONALITY", DataProtection: dataProtectionLinked}
	plan := planFromDesiredAndRemote("123", "./privacy.json", map[string]privacyTuple{
		privacyTupleKey(desiredTuple): desiredTuple,
	}, map[string]privacyRemoteState{
		privacyTupleKey(remoteTuple): {Tuple: remoteTuple, UsageIDs: []string{"usage-1"}},
	})
	if len(plan.Updates) != 1 || len(plan.Adds) != 0 || len(plan.Deletes) != 0 {
		t.Fatalf("expected one update and no adds/deletes, got updates=%#v adds=%#v deletes=%#v", plan.Updates, plan.Adds, plan.Deletes)
	}
	if plan.Updates[0].UsageID != "usage-1" || plan.Updates[0].DataProtection != dataProtectionNotLinked {
		t.Fatalf("unexpected update payload: %#v", plan.Updates[0])
	}
	if len(plan.APICalls) != 1 || plan.APICalls[0].Operation != "update_data_usage" || plan.APICalls[0].Count != 1 {
		t.Fatalf("unexpected api calls: %#v", plan.APICalls)
	}
}

func TestCanPairAsUpdateAllowsBothIdentityDirections(t *testing.T) {
	for _, tc := range []struct {
		name, from, to string
	}{
		{name: "linked-to-not-linked", from: dataProtectionLinked, to: dataProtectionNotLinked},
		{name: "not-linked-to-linked", from: dataProtectionNotLinked, to: dataProtectionLinked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			add := privacyPlanChange{
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: tc.to,
			}
			deletion := privacyPlanChange{
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: tc.from,
				UsageID:        "usage-1",
			}
			if !canPairAsUpdate(add, deletion) {
				t.Fatalf("canPairAsUpdate(%s -> %s) = false, want true", tc.from, tc.to)
			}
		})
	}
}

func TestPlanFromDesiredAndRemoteNotCollectedRemainsDeleteCreate(t *testing.T) {
	desired := map[string]privacyTuple{
		privacyTupleKey(privacyTuple{DataProtection: dataProtectionNotCollected}): {
			DataProtection: dataProtectionNotCollected,
		},
	}
	remote := map[string]privacyRemoteState{
		privacyTupleKey(privacyTuple{
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionNotLinked,
		}): {
			Tuple: privacyTuple{
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionNotLinked,
			},
			UsageIDs: []string{"usage-1"},
		},
	}

	plan := planFromDesiredAndRemote("123", "./privacy.json", desired, remote)
	if len(plan.Updates) != 0 {
		t.Fatalf("expected no updates for DATA_NOT_COLLECTED transition, got %#v", plan.Updates)
	}
	if len(plan.Adds) != 1 || len(plan.Deletes) != 1 {
		t.Fatalf("expected one add and one delete, got adds=%#v deletes=%#v", plan.Adds, plan.Deletes)
	}
}

func TestPlanFromDesiredAndRemoteTrackingTransitionYieldsUpdateAndAdd(t *testing.T) {
	desired := map[string]privacyTuple{
		privacyTupleKey(privacyTuple{
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionNotLinked,
		}): {
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionNotLinked,
		},
		privacyTupleKey(privacyTuple{
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionTracking,
		}): {
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionTracking,
		},
	}
	remote := map[string]privacyRemoteState{
		privacyTupleKey(privacyTuple{
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		}): {
			Tuple: privacyTuple{
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionLinked,
			},
			UsageIDs: []string{"usage-1"},
		},
	}

	plan := planFromDesiredAndRemote("123", "./privacy.json", desired, remote)
	if len(plan.Updates) != 1 {
		t.Fatalf("expected one update, got %#v", plan.Updates)
	}
	if len(plan.Adds) != 1 {
		t.Fatalf("expected one add, got %#v", plan.Adds)
	}
	if len(plan.Deletes) != 0 {
		t.Fatalf("expected no deletes, got %#v", plan.Deletes)
	}
	if plan.Updates[0].UsageID != "usage-1" {
		t.Fatalf("expected update to reuse usage-1, got %#v", plan.Updates[0])
	}
}

func TestPlanFromDesiredAndRemoteDoesNotPairTrackingDeleteIntoUpdate(t *testing.T) {
	desired := map[string]privacyTuple{
		privacyTupleKey(privacyTuple{
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		}): {
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		},
		privacyTupleKey(privacyTuple{
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionNotLinked,
		}): {
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionNotLinked,
		},
	}
	remote := map[string]privacyRemoteState{
		privacyTupleKey(privacyTuple{
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionLinked,
		}): {
			Tuple: privacyTuple{
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionLinked,
			},
			UsageIDs: []string{"usage-linked-1"},
		},
		privacyTupleKey(privacyTuple{
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: dataProtectionTracking,
		}): {
			Tuple: privacyTuple{
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionTracking,
			},
			UsageIDs: []string{"usage-tracking-1"},
		},
	}

	plan := planFromDesiredAndRemote("123", "./privacy.json", desired, remote)
	if len(plan.Updates) != 0 {
		t.Fatalf("expected no updates when replacing tracking tuple with identity tuple, got %#v", plan.Updates)
	}
	if len(plan.Adds) != 1 || len(plan.Deletes) != 1 {
		t.Fatalf("expected one add and one delete, got adds=%#v deletes=%#v", plan.Adds, plan.Deletes)
	}
	if plan.Deletes[0].DataProtection != dataProtectionTracking {
		t.Fatalf("expected tracking tuple delete, got %#v", plan.Deletes[0])
	}
}

type permutationCase struct {
	name         string
	protections  []string
	notCollected bool
}

func tupleSetForPermutation(tc permutationCase) map[string]privacyTuple {
	tuples := map[string]privacyTuple{}
	if tc.notCollected {
		tuple := privacyTuple{DataProtection: dataProtectionNotCollected}
		tuples[privacyTupleKey(tuple)] = tuple
		return tuples
	}
	for _, protection := range tc.protections {
		tuple := privacyTuple{
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: protection,
		}
		tuples[privacyTupleKey(tuple)] = tuple
	}
	return tuples
}

func remoteStateForPermutation(tc permutationCase, duplicateFirst bool) map[string]privacyRemoteState {
	state := map[string]privacyRemoteState{}
	if tc.notCollected {
		tuple := privacyTuple{DataProtection: dataProtectionNotCollected}
		usageIDs := []string{"usage-not-collected-1"}
		if duplicateFirst {
			usageIDs = append(usageIDs, "usage-not-collected-2")
		}
		state[privacyTupleKey(tuple)] = privacyRemoteState{
			Tuple:    tuple,
			UsageIDs: usageIDs,
		}
		return state
	}

	for index, protection := range tc.protections {
		tuple := privacyTuple{
			Category:       "EMAIL_ADDRESS",
			Purpose:        "APP_FUNCTIONALITY",
			DataProtection: protection,
		}
		usageIDs := []string{fmt.Sprintf("usage-%s-%d-1", strings.ToLower(protection), index)}
		if duplicateFirst && index == 0 {
			usageIDs = append(usageIDs, fmt.Sprintf("usage-%s-%d-2", strings.ToLower(protection), index))
		}
		state[privacyTupleKey(tuple)] = privacyRemoteState{
			Tuple:    tuple,
			UsageIDs: usageIDs,
		}
	}
	return state
}

func simulatePlanResult(remote map[string]privacyRemoteState, plan privacyPlanOutput) (map[string]privacyTuple, error) {
	byUsageID := map[string]privacyTuple{}
	for _, state := range remote {
		for _, usageID := range state.UsageIDs {
			usageID = strings.TrimSpace(usageID)
			if usageID == "" {
				continue
			}
			byUsageID[usageID] = state.Tuple
		}
	}

	for _, deletion := range plan.Deletes {
		usageID := strings.TrimSpace(deletion.UsageID)
		if usageID == "" {
			return nil, fmt.Errorf("delete operation missing usage id")
		}
		if _, exists := byUsageID[usageID]; !exists {
			return nil, fmt.Errorf("delete operation references unknown usage id %s", usageID)
		}
		delete(byUsageID, usageID)
	}
	for _, update := range plan.Updates {
		usageID := strings.TrimSpace(update.UsageID)
		if usageID == "" {
			return nil, fmt.Errorf("update operation missing usage id")
		}
		if _, exists := byUsageID[usageID]; !exists {
			return nil, fmt.Errorf("update operation references unknown usage id %s", usageID)
		}
		byUsageID[usageID] = privacyTuple{
			Category:       update.Category,
			Purpose:        update.Purpose,
			DataProtection: update.DataProtection,
		}
	}
	nextGeneratedID := 0
	for _, add := range plan.Adds {
		nextGeneratedID++
		byUsageID[fmt.Sprintf("generated-%d", nextGeneratedID)] = privacyTuple{
			Category:       add.Category,
			Purpose:        add.Purpose,
			DataProtection: add.DataProtection,
		}
	}

	result := map[string]privacyTuple{}
	for _, tuple := range byUsageID {
		result[privacyTupleKey(tuple)] = tuple
	}
	return result, nil
}

func TestPlanFromDesiredAndRemotePermutationMatrixProducesDesiredState(t *testing.T) {
	desiredCases := []permutationCase{
		{name: "not_collected", notCollected: true},
		{name: "linked_only", protections: []string{dataProtectionLinked}},
		{name: "not_linked_only", protections: []string{dataProtectionNotLinked}},
		{name: "linked_tracking", protections: []string{dataProtectionLinked, dataProtectionTracking}},
		{name: "not_linked_tracking", protections: []string{dataProtectionNotLinked, dataProtectionTracking}},
		{name: "linked_not_linked", protections: []string{dataProtectionLinked, dataProtectionNotLinked}},
	}

	type remoteCase struct {
		permutationCase
		duplicateFirst bool
	}
	remoteCases := make([]remoteCase, 0, len(desiredCases)*2)
	for _, base := range desiredCases {
		remoteCases = append(remoteCases, remoteCase{
			permutationCase: base,
			duplicateFirst:  false,
		})
		if !base.notCollected {
			remoteCases = append(remoteCases, remoteCase{
				permutationCase: permutationCase{
					name:         base.name + "_dup_first",
					protections:  base.protections,
					notCollected: base.notCollected,
				},
				duplicateFirst: true,
			})
		}
	}

	for _, remoteTC := range remoteCases {
		for _, desiredTC := range desiredCases {
			caseName := remoteTC.name + "->" + desiredTC.name
			t.Run(caseName, func(t *testing.T) {
				desired := tupleSetForPermutation(desiredTC)
				remote := remoteStateForPermutation(remoteTC.permutationCase, remoteTC.duplicateFirst)

				plan := planFromDesiredAndRemote("123", "./privacy.json", desired, remote)

				seenUsageIDs := map[string]string{}
				for _, update := range plan.Updates {
					usageID := strings.TrimSpace(update.UsageID)
					if usageID == "" {
						t.Fatalf("update missing usage id: %#v", update)
					}
					seenUsageIDs[usageID] = "update"
				}
				for _, deletion := range plan.Deletes {
					usageID := strings.TrimSpace(deletion.UsageID)
					if usageID == "" {
						t.Fatalf("delete missing usage id: %#v", deletion)
					}
					if owner, exists := seenUsageIDs[usageID]; exists {
						t.Fatalf("usage id %s appears in both %s and delete operations", usageID, owner)
					}
					seenUsageIDs[usageID] = "delete"
				}

				if remoteTC.notCollected || desiredTC.notCollected {
					if len(plan.Updates) != 0 {
						t.Fatalf("DATA_NOT_COLLECTED transitions must not produce updates, got %#v", plan.Updates)
					}
				}

				gotState, err := simulatePlanResult(remote, plan)
				if err != nil {
					t.Fatalf("simulatePlanResult() error = %v", err)
				}
				if !reflect.DeepEqual(gotState, desired) {
					t.Fatalf("final tuple state mismatch, got=%#v want=%#v plan=%#v", gotState, desired, plan)
				}
			})
		}
	}
}

type fakePrivacyMutationClient struct {
	callOrder     []string
	createCounter int
	createErr     error
}

func (f *fakePrivacyMutationClient) CreateAppDataUsage(_ context.Context, _ string, tuple webcore.DataUsageTuple) (*webcore.AppDataUsage, error) {
	if f.createErr != nil {
		f.callOrder = append(f.callOrder, fmt.Sprintf("create-failed:%s:%s:%s", tuple.Category, tuple.Purpose, tuple.DataProtection))
		return nil, f.createErr
	}
	f.createCounter++
	f.callOrder = append(f.callOrder, fmt.Sprintf("create:%s:%s:%s", tuple.Category, tuple.Purpose, tuple.DataProtection))
	return &webcore.AppDataUsage{
		ID:             fmt.Sprintf("created-%d", f.createCounter),
		Category:       tuple.Category,
		Purpose:        tuple.Purpose,
		DataProtection: tuple.DataProtection,
	}, nil
}

func (f *fakePrivacyMutationClient) UpdateAppDataUsage(_ context.Context, appDataUsageID string, tuple webcore.DataUsageTuple) (*webcore.AppDataUsage, error) {
	f.callOrder = append(f.callOrder, fmt.Sprintf("update:%s:%s", appDataUsageID, tuple.DataProtection))
	return &webcore.AppDataUsage{
		ID:             appDataUsageID,
		Category:       tuple.Category,
		Purpose:        tuple.Purpose,
		DataProtection: tuple.DataProtection,
	}, nil
}

func (f *fakePrivacyMutationClient) DeleteAppDataUsage(_ context.Context, appDataUsageID string) error {
	f.callOrder = append(f.callOrder, "delete:"+appDataUsageID)
	return nil
}

func TestApplyPrivacyPlanExecutesUpdateCreateDeleteOrder(t *testing.T) {
	client := &fakePrivacyMutationClient{}
	plan := privacyPlanOutput{
		Updates: []privacyPlanChange{
			{
				Key:            "EMAIL_ADDRESS|APP_FUNCTIONALITY|DATA_NOT_LINKED_TO_YOU",
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionNotLinked,
				UsageID:        "usage-update-1",
			},
		},
		Adds: []privacyPlanChange{
			{
				Key:            "EMAIL_ADDRESS|ANALYTICS|DATA_NOT_LINKED_TO_YOU",
				Category:       "EMAIL_ADDRESS",
				Purpose:        "ANALYTICS",
				DataProtection: dataProtectionNotLinked,
			},
		},
		Deletes: []privacyPlanChange{
			{
				Key:            "EMAIL_ADDRESS|APP_FUNCTIONALITY|DATA_LINKED_TO_YOU",
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionLinked,
				UsageID:        "usage-delete-1",
			},
		},
	}

	result, err := applyPrivacyPlan(context.Background(), client, "app-123", plan)
	if err != nil {
		t.Fatalf("applyPrivacyPlan() error = %v", err)
	}
	if !reflect.DeepEqual(client.callOrder, []string{
		"update:usage-update-1:DATA_NOT_LINKED_TO_YOU",
		"create:EMAIL_ADDRESS:ANALYTICS:DATA_NOT_LINKED_TO_YOU",
		"delete:usage-delete-1",
	}) {
		t.Fatalf("unexpected call order: %#v", client.callOrder)
	}
	if len(result.Applied) != 3 {
		t.Fatalf("expected 3 applied actions, got %#v", result.Applied)
	}
	if result.Applied[0].Action != "update" || result.Applied[1].Action != "create" || result.Applied[2].Action != "delete" {
		t.Fatalf("unexpected action order: %#v", result.Applied)
	}
	if len(result.Unknown) != 0 || len(result.NotApplied) != 0 {
		t.Fatalf("expected no unknown or not-applied actions: %#v", result)
	}
}

func TestApplyPrivacyPlanRejectsUpdateWithoutUsageID(t *testing.T) {
	client := &fakePrivacyMutationClient{}
	_, err := applyPrivacyPlan(context.Background(), client, "app-123", privacyPlanOutput{
		Updates: []privacyPlanChange{
			{
				Key:            "EMAIL_ADDRESS|APP_FUNCTIONALITY|DATA_NOT_LINKED_TO_YOU",
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionNotLinked,
			},
		},
	})
	if err == nil {
		t.Fatal("expected missing usage id error")
	}
	if !strings.Contains(err.Error(), "missing usage id for update key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyPrivacyPlanRejectsConflictingDeleteAndUpdateUsageID(t *testing.T) {
	client := &fakePrivacyMutationClient{}
	_, err := applyPrivacyPlan(context.Background(), client, "app-123", privacyPlanOutput{
		Updates: []privacyPlanChange{
			{
				Key:            "EMAIL_ADDRESS|APP_FUNCTIONALITY|DATA_NOT_LINKED_TO_YOU",
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionNotLinked,
				UsageID:        "usage-1",
			},
		},
		Deletes: []privacyPlanChange{
			{
				Key:            "EMAIL_ADDRESS|APP_FUNCTIONALITY|DATA_LINKED_TO_YOU",
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionLinked,
				UsageID:        "usage-1",
			},
		},
	})
	if err == nil {
		t.Fatal("expected overlapping usage id error")
	}
	if !strings.Contains(err.Error(), "scheduled for both delete") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyPrivacyPlanRejectsDuplicateUpdateUsageID(t *testing.T) {
	client := &fakePrivacyMutationClient{}
	_, err := applyPrivacyPlan(context.Background(), client, "app-123", privacyPlanOutput{
		Updates: []privacyPlanChange{
			{
				Key:            "EMAIL_ADDRESS|APP_FUNCTIONALITY|DATA_NOT_LINKED_TO_YOU",
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionNotLinked,
				UsageID:        "usage-1",
			},
			{
				Key:            "EMAIL_ADDRESS|ANALYTICS|DATA_NOT_LINKED_TO_YOU",
				Category:       "EMAIL_ADDRESS",
				Purpose:        "ANALYTICS",
				DataProtection: dataProtectionNotLinked,
				UsageID:        "usage-1",
			},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate update usage id error")
	}
	if !strings.Contains(err.Error(), "duplicate update usage id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePrivacyDeclarationFileRejectsUnknownFields(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	path := filepath.Join(dir, "privacy.json")
	if err := os.WriteFile(path, []byte(`{
		"schemaVersion": 1,
		"dataUsages": [
			{
				"category": "NAME",
				"purposes": ["APP_FUNCTIONALITY"],
				"dataProtections": ["DATA_LINKED_TO_YOU"],
				"unknownField": "x"
			}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := parsePrivacyDeclarationFile(d, path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePrivacyDeclarationFileRejectsMultipleJSONValues(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	path := filepath.Join(dir, "privacy.json")
	if err := os.WriteFile(path, []byte(`{
		"schemaVersion": 1,
		"dataUsages": [
			{
				"category": "NAME",
				"purposes": ["APP_FUNCTIONALITY"],
				"dataProtections": ["DATA_LINKED_TO_YOU"]
			}
		]
	}
	{
		"schemaVersion": 1,
		"dataUsages": [
			{
				"dataProtections": ["DATA_NOT_COLLECTED"]
			}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := parsePrivacyDeclarationFile(d, path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "multiple JSON values found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePrivacyDeclarationFileCanonicalizesTrackingPurposeAway(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	path := filepath.Join(dir, "privacy.json")
	if err := os.WriteFile(path, []byte(`{
		"schemaVersion": 1,
		"dataUsages": [
			{
				"category": "PURCHASE_HISTORY",
				"purposes": ["APP_FUNCTIONALITY"],
				"dataProtections": ["DATA_LINKED_TO_YOU", "DATA_USED_TO_TRACK_YOU"]
			}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	declaration, err := parsePrivacyDeclarationFile(d, path)
	if err != nil {
		t.Fatalf("parsePrivacyDeclarationFile() error = %v", err)
	}
	trackingFound := false
	for _, usage := range declaration.DataUsages {
		if len(usage.DataProtections) == 1 && usage.DataProtections[0] == dataProtectionTracking {
			trackingFound = true
			if len(usage.Purposes) != 0 {
				t.Fatalf("expected tracking usage purposes to be empty, got %#v", usage.Purposes)
			}
		}
	}
	if !trackingFound {
		t.Fatalf("expected canonicalized tracking usage in declaration: %#v", declaration.DataUsages)
	}
}

const privacyTestAppID = "123456789"

func TestWebPrivacyPullReportsUnknownWhenPublishedAttributeMissing(t *testing.T) {
	fixture := handlertest.New(t)
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages":
			return privacyJSONResponse(req, `{"data":[]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsagePublishState":
			return privacyJSONResponse(req, `{
				"data": {
					"id": "publish-state-1",
					"type": "appDataUsagesPublishState",
					"attributes": {}
				}
			}`), nil
		default:
			return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
		}
	})

	t.Run("json", func(t *testing.T) {
		d, depsStdout, depsStderr := newWebTestDeps()
		cmd := newWebPrivacyPullCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--app", privacyTestAppID, "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
			if err := cmd.Exec(context.Background(), nil); err != nil {
				t.Fatalf("exec error: %v", err)
			}
		})
		assertNoPrivacySecrets(t, stdout, stderr)

		var payload map[string]any
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
		}
		publishState, ok := payload["publishState"].(map[string]any)
		if !ok {
			t.Fatalf("expected publishState object, got %#v", payload["publishState"])
		}
		known, ok := publishState["publishedKnown"].(bool)
		if !ok {
			t.Fatalf("expected additive publishedKnown bool, got %#v", publishState["publishedKnown"])
		}
		if known {
			t.Fatal("expected publishedKnown=false when Apple omits published")
		}
		if published, ok := publishState["published"].(bool); !ok {
			t.Fatalf("expected published bool to remain present, got %#v", publishState["published"])
		} else if published && !known {
			t.Fatal("published must not report true when publication state is unknown")
		}
	})

	t.Run("table", func(t *testing.T) {
		d, depsStdout, depsStderr := newWebTestDeps()
		cmd := newWebPrivacyPullCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--app", privacyTestAppID, "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
			if err := cmd.Exec(context.Background(), nil); err != nil {
				t.Fatalf("exec error: %v", err)
			}
		})
		assertNoPrivacySecrets(t, stdout, stderr)
		if strings.Contains(stdout, "Published: false") {
			t.Fatalf("pull reported unpublished instead of unknown:\n%s", stdout)
		}
		if !strings.Contains(stdout, "Published: unknown") {
			t.Fatalf("expected Published: unknown in table output, got:\n%s", stdout)
		}
	})
}

func TestWebPrivacyPublishErrorsBeforePatchWhenPublishStateIDEmpty(t *testing.T) {
	fixture := handlertest.New(t)
	var patchCount int
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPatch {
			patchCount++
			return fixture.Response("did not expect PATCH %s", req.URL.Path), nil
		}
		if req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsagePublishState" {
			return privacyJSONResponse(req, `{
				"data": {
					"id": "",
					"type": "appDataUsagesPublishState",
					"attributes": {"published": false}
				}
			}`), nil
		}
		return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyPublishCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", privacyTestAppID, "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	assertNoPrivacySecrets(t, stdout, stderr)
	if execErr == nil {
		t.Fatal("expected publish to fail when publish-state id is empty")
	}
	if !strings.Contains(execErr.Error(), "publish-state id is missing") {
		t.Fatalf("error = %v, want publish-state id is missing", execErr)
	}
	if patchCount != 0 {
		t.Fatalf("expected no PATCH before missing id error, got %d", patchCount)
	}
}

func TestWebPrivacyPublishExitsNonZeroWhenPatchDoesNotConfirmPublished(t *testing.T) {
	tests := []struct {
		name         string
		patchBody    string
		wantFragment string
	}{
		{
			name: "published false",
			patchBody: `{
				"data": {
					"id": "publish-state-1",
					"type": "appDataUsagesPublishState",
					"attributes": {"published": false}
				}
			}`,
			wantFragment: "could not be verified",
		},
		{
			name: "published omitted",
			patchBody: `{
				"data": {
					"id": "publish-state-1",
					"type": "appDataUsagesPublishState",
					"attributes": {}
				}
			}`,
			wantFragment: "could not be verified",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := handlertest.New(t)
			stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsagePublishState":
					return privacyJSONResponse(req, `{
						"data": {
							"id": "publish-state-1",
							"type": "appDataUsagesPublishState",
							"attributes": {"published": false}
						}
					}`), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/iris/v1/appDataUsagesPublishState/publish-state-1":
					return privacyJSONResponse(req, tc.patchBody), nil
				default:
					return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
				}
			})

			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := newWebPrivacyPublishCommand(d)
			if err := cmd.FlagSet.Parse([]string{"--app", privacyTestAppID, "--confirm", "--output", "json"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}

			var execErr error
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				execErr = cmd.Exec(context.Background(), nil)
			})
			assertNoPrivacySecrets(t, stdout, stderr)
			if execErr == nil {
				t.Fatal("expected publish to exit non-zero when PATCH does not confirm published")
			}
			if !strings.Contains(execErr.Error(), tc.wantFragment) {
				t.Fatalf("error = %v, want %q", execErr, tc.wantFragment)
			}
		})
	}
}

func TestWebPrivacyPublishSucceedsWhenPatchConfirmsPublished(t *testing.T) {
	fixture := handlertest.New(t)
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsagePublishState":
			return privacyJSONResponse(req, `{
				"data": {
					"id": "publish-state-1",
					"type": "appDataUsagesPublishState",
					"attributes": {"published": false}
				}
			}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/iris/v1/appDataUsagesPublishState/publish-state-1":
			return privacyJSONResponse(req, `{
				"data": {
					"id": "publish-state-1",
					"type": "appDataUsagesPublishState",
					"attributes": {"published": true}
				}
			}`), nil
		default:
			return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
		}
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyPublishCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", privacyTestAppID, "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	assertNoPrivacySecrets(t, stdout, stderr)

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload["changed"] != true {
		t.Fatalf("expected changed=true, got %#v", payload["changed"])
	}
	publishState, ok := payload["publishState"].(map[string]any)
	if !ok {
		t.Fatalf("expected publishState object, got %#v", payload["publishState"])
	}
	if publishState["published"] != true {
		t.Fatalf("expected published=true, got %#v", publishState["published"])
	}
	if publishState["publishedKnown"] != true {
		t.Fatalf("expected additive publishedKnown=true, got %#v", publishState["publishedKnown"])
	}
}

func stubPrivacyWebSession(t *testing.T, roundTrip func(*http.Request) (*http.Response, error)) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	t.Cleanup(SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(roundTrip)},
		}, "cache", nil
	}))
}

func privacyJSONResponse(req *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func assertNoPrivacySecrets(t *testing.T, stdout, stderr string) {
	t.Helper()
	combined := stdout + stderr
	for _, secret := range []string{"Set-Cookie", "cookie=", "myacinfo", "dqsid"} {
		if strings.Contains(combined, secret) {
			t.Fatalf("output leaked session secret %q", secret)
		}
	}
}

func containsPrivacyProtection(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestWebPrivacyPullDoesNotWriteNotCollectedForMalformedRemoteUsages(t *testing.T) {
	fixture := handlertest.New(t)
	outPath := filepath.Join(t.TempDir(), "privacy.json")
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages":
			return privacyJSONResponse(req, `{
				"data": [{
					"id": "usage-malformed-1",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"PURCHASE_HISTORY"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}}
					}
				}, {
					"id": "usage-malformed-2",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"EMAIL_ADDRESS"}}
					}
				}]
			}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsagePublishState":
			return privacyJSONResponse(req, `{
				"data": {
					"id": "publish-state-1",
					"type": "appDataUsagesPublishState",
					"attributes": {"published": false}
				}
			}`), nil
		default:
			return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
		}
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyPullCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", privacyTestAppID, "--out", outPath, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	assertNoPrivacySecrets(t, stdout, stderr)

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	count, ok := payload["unrepresentableCount"].(float64)
	if !ok || count != 2 {
		t.Fatalf("expected unrepresentableCount=2, got %#v", payload["unrepresentableCount"])
	}
	declaration, ok := payload["declaration"].(map[string]any)
	if !ok {
		t.Fatalf("expected declaration object, got %#v", payload["declaration"])
	}
	assertJSONDeclarationHasNoNotCollected(t, declaration)

	fileData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read --out file: %v", err)
	}
	if strings.Contains(string(fileData), dataProtectionNotCollected) {
		t.Fatalf("--out wrote DATA_NOT_COLLECTED for non-empty malformed remote usages:\n%s", fileData)
	}
	if !strings.Contains(string(fileData), dataProtectionUnknown) {
		t.Fatalf("--out missing opaque %s marker:\n%s", dataProtectionUnknown, fileData)
	}
}

func TestWebPrivacyPullPreservesMalformedUsagesAlongsideValidOnes(t *testing.T) {
	fixture := handlertest.New(t)
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages":
			return privacyJSONResponse(req, `{
				"data": [{
					"id": "usage-malformed-1",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"PURCHASE_HISTORY"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}}
					}
				}, {
					"id": "usage-valid-1",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"PURCHASE_HISTORY"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_LINKED_TO_YOU"}}
					}
				}]
			}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsagePublishState":
			return privacyJSONResponse(req, `{
				"data": {
					"id": "publish-state-1",
					"type": "appDataUsagesPublishState",
					"attributes": {"published": true}
				}
			}`), nil
		default:
			return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
		}
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyPullCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", privacyTestAppID, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	assertNoPrivacySecrets(t, stdout, stderr)

	var payload struct {
		UnrepresentableCount int `json:"unrepresentableCount"`
		Declaration          privacyDeclarationFile
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.UnrepresentableCount != 1 {
		t.Fatalf("expected unrepresentableCount=1, got %d", payload.UnrepresentableCount)
	}
	if len(payload.Declaration.DataUsages) != 1 {
		t.Fatalf("expected one grouped usage, got %#v", payload.Declaration.DataUsages)
	}
	got := payload.Declaration.DataUsages[0]
	if containsPrivacyProtection(got.DataProtections, dataProtectionNotCollected) {
		t.Fatalf("mixed pull collapsed malformed entries: %#v", got)
	}
	if !containsPrivacyProtection(got.DataProtections, dataProtectionLinked) || !containsPrivacyProtection(got.DataProtections, dataProtectionUnknown) {
		t.Fatalf("expected valid and opaque protections, got %#v", got.DataProtections)
	}
}

func TestWebPrivacyPullNotCollectedRoundTripsThroughPlan(t *testing.T) {
	fixture := handlertest.New(t)
	outPath := filepath.Join(t.TempDir(), "privacy.json")
	notCollectedUsages := `{
		"data": [{
			"id": "u1",
			"type": "appDataUsages",
			"relationships": {
				"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_NOT_COLLECTED"}}
			}
		}]
	}`
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		if resp, ok := privacyCatalogRoundTrip(req); ok {
			return resp, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages":
			return privacyJSONResponse(req, notCollectedUsages), nil
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsagePublishState":
			return privacyJSONResponse(req, `{
				"data": {
					"id": "publish-state-1",
					"type": "appDataUsagesPublishState",
					"attributes": {"published": true}
				}
			}`), nil
		default:
			return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
		}
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	pullCmd := newWebPrivacyPullCommand(d)
	if err := pullCmd.FlagSet.Parse([]string{"--app", privacyTestAppID, "--out", outPath, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := pullCmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("pull exec error: %v", err)
		}
	})
	assertNoPrivacySecrets(t, stdout, stderr)

	var payload struct {
		UnrepresentableCount int `json:"unrepresentableCount"`
		Declaration          privacyDeclarationFile
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse pull stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.UnrepresentableCount != 0 {
		t.Fatalf("expected unrepresentableCount=0 for DATA_NOT_COLLECTED, got %d", payload.UnrepresentableCount)
	}
	if len(payload.Declaration.DataUsages) != 1 {
		t.Fatalf("expected one pulled usage, got %#v", payload.Declaration.DataUsages)
	}
	if !reflect.DeepEqual(payload.Declaration.DataUsages[0].DataProtections, []string{dataProtectionNotCollected}) {
		t.Fatalf("expected pulled DATA_NOT_COLLECTED, got %#v", payload.Declaration.DataUsages[0])
	}

	fileData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read --out file: %v", err)
	}
	var written privacyDeclarationFile
	if err := json.Unmarshal(fileData, &written); err != nil {
		t.Fatalf("parse --out file: %v\n%s", err, fileData)
	}
	if _, err := declarationToTupleSet(written); err != nil {
		t.Fatalf("pulled file failed plan validator: %v\n%s", err, fileData)
	}
	if strings.Contains(string(fileData), `"category"`) || strings.Contains(string(fileData), `"purposes"`) {
		t.Fatalf("expected canonical not-collected file without empty category/purposes keys:\n%s", fileData)
	}

	d, depsStdout, depsStderr = newWebTestDeps()
	planCmd := newWebPrivacyPlanCommand(d)
	if err := planCmd.FlagSet.Parse([]string{"--app", privacyTestAppID, "--file", outPath, "--output", "json"}); err != nil {
		t.Fatalf("plan parse error: %v", err)
	}

	planStdout, planStderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := planCmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("plan exec error: %v", err)
		}
	})
	assertNoPrivacySecrets(t, planStdout, planStderr)

	var plan privacyPlanOutput
	if err := json.Unmarshal([]byte(planStdout), &plan); err != nil {
		t.Fatalf("failed to parse plan stdout JSON: %v\nstdout=%s", err, planStdout)
	}
	if len(plan.Adds) != 0 || len(plan.Deletes) != 0 || len(plan.Updates) != 0 {
		t.Fatalf("expected empty pull->plan diff, got adds=%#v deletes=%#v updates=%#v", plan.Adds, plan.Deletes, plan.Updates)
	}
}

func TestWebPrivacyApplyRefusesUnrepresentableDeclarationWithoutDeletes(t *testing.T) {
	fixture := handlertest.New(t)
	var deleteCount int
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodDelete {
			deleteCount++
			return fixture.Response("did not expect DELETE %s", req.URL.Path), nil
		}
		return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
	})

	path := filepath.Join(t.TempDir(), "privacy.json")
	if err := os.WriteFile(path, []byte(`{
		"schemaVersion": 1,
		"dataUsages": [{
			"category": "PURCHASE_HISTORY",
			"purposes": ["APP_FUNCTIONALITY"],
			"dataProtections": ["UNKNOWN_OR_MISSING"]
		}]
	}`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyApplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", privacyTestAppID,
		"--file", path,
		"--allow-deletes",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	assertNoPrivacySecrets(t, stdout, stderr)
	if execErr == nil {
		t.Fatal("expected apply to fail closed on unrepresentable declaration")
	}
	if !strings.Contains(execErr.Error(), "unrepresentable") {
		t.Fatalf("error = %v, want unrepresentable", execErr)
	}
	if !strings.Contains(stderr, "unrepresentable") {
		t.Fatalf("stderr = %q, want unrepresentable", stderr)
	}
	if deleteCount != 0 {
		t.Fatalf("expected no deletes, got %d", deleteCount)
	}
}

func assertJSONDeclarationHasNoNotCollected(t *testing.T, declaration map[string]any) {
	t.Helper()
	usages, ok := declaration["dataUsages"].([]any)
	if !ok {
		t.Fatalf("expected dataUsages array, got %#v", declaration["dataUsages"])
	}
	raw, err := json.Marshal(usages)
	if err != nil {
		t.Fatalf("marshal dataUsages: %v", err)
	}
	if strings.Contains(string(raw), dataProtectionNotCollected) {
		t.Fatalf("declaration contained DATA_NOT_COLLECTED: %s", raw)
	}
}

func privacyCatalogBody(deletedCategories ...string) string {
	deleted := map[string]bool{}
	for _, id := range deletedCategories {
		deleted[id] = true
	}
	categories := ""
	for _, id := range []string{"EMAIL_ADDRESS", "PURCHASE_HISTORY", "PHONE_NUMBER"} {
		if categories != "" {
			categories += ","
		}
		categories += fmt.Sprintf(
			`{"id":%q,"type":"appDataUsageCategories","attributes":{"deleted":%t}}`,
			id,
			deleted[id],
		)
	}
	return "[" + categories + "]"
}

func privacyCatalogRoundTrip(req *http.Request, deletedCategories ...string) (*http.Response, bool) {
	if req.Method != http.MethodGet {
		return nil, false
	}
	switch req.URL.Path {
	case "/iris/v1/appDataUsageCategories":
		return privacyJSONResponse(req, `{"data":`+privacyCatalogBody(deletedCategories...)+`}`), true
	case "/iris/v1/appDataUsagePurposes":
		return privacyJSONResponse(req, `{"data":[
			{"id":"APP_FUNCTIONALITY","type":"appDataUsagePurposes","attributes":{"deleted":false}},
			{"id":"ANALYTICS","type":"appDataUsagePurposes","attributes":{"deleted":false}}
		]}`), true
	case "/iris/v1/appDataUsageDataProtections":
		return privacyJSONResponse(req, `{"data":[
			{"id":"DATA_NOT_COLLECTED","type":"appDataUsageDataProtections","attributes":{"deleted":false}},
			{"id":"DATA_LINKED_TO_YOU","type":"appDataUsageDataProtections","attributes":{"deleted":false}},
			{"id":"DATA_NOT_LINKED_TO_YOU","type":"appDataUsageDataProtections","attributes":{"deleted":false}},
			{"id":"DATA_USED_TO_TRACK_YOU","type":"appDataUsageDataProtections","attributes":{"deleted":false}}
		]}`), true
	}
	return nil, false
}

func writePrivacyDeclarationForTest(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "privacy.json")
	if err := os.WriteFile(path, []byte(privacyTwoUsageDeclaration), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

const privacyTwoUsageDeclaration = `{
	"schemaVersion": 1,
	"dataUsages": [
		{
			"category": "EMAIL_ADDRESS",
			"purposes": ["APP_FUNCTIONALITY"],
			"dataProtections": ["DATA_NOT_LINKED_TO_YOU"]
		},
		{
			"category": "PURCHASE_HISTORY",
			"purposes": ["ANALYTICS"],
			"dataProtections": ["DATA_LINKED_TO_YOU"]
		}
	]
}`

func privacyApplyActionKinds(t *testing.T, payload map[string]any, field string) []string {
	t.Helper()
	raw, ok := payload[field]
	if !ok || raw == nil {
		return nil
	}
	entries, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected %s array, got %#v", field, raw)
	}
	kinds := make([]string, 0, len(entries))
	for _, entry := range entries {
		action, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("expected %s entry object, got %#v", field, entry)
		}
		kinds = append(kinds, fmt.Sprintf("%v:%v", action["action"], action["key"]))
	}
	return kinds
}

type privacyMidSequenceFailureStub struct {
	methodOrder []string
	listCount   int
}

// stubPrivacyMidSequenceFailure serves a remote state that needs one update,
// one create, and one delete, and fails the create with a 500.
func stubPrivacyMidSequenceFailure(t *testing.T) *privacyMidSequenceFailureStub {
	t.Helper()
	stub := &privacyMidSequenceFailureStub{}
	fixture := handlertest.New(t)
	emailProtection := "DATA_LINKED_TO_YOU"
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		if resp, ok := privacyCatalogRoundTrip(req); ok {
			return resp, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages":
			stub.listCount++
			return privacyJSONResponse(req, `{"data":[
				{
					"id": "usage-email",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"EMAIL_ADDRESS"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"`+emailProtection+`"}}
					}
				},
				{
					"id": "usage-phone",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"PHONE_NUMBER"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"ANALYTICS"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_LINKED_TO_YOU"}}
					}
				}
			]}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/iris/v1/appDataUsages/usage-email":
			stub.methodOrder = append(stub.methodOrder, "PATCH")
			emailProtection = "DATA_NOT_LINKED_TO_YOU"
			return privacyJSONResponse(req, `{"data":{
				"id": "usage-email",
				"type": "appDataUsages",
				"relationships": {
					"category": {"data": {"type":"appDataUsageCategories","id":"EMAIL_ADDRESS"}},
					"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}},
					"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_NOT_LINKED_TO_YOU"}}
				}
			}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/appDataUsages":
			stub.methodOrder = append(stub.methodOrder, "POST")
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"code":"ENTITY_ERROR","detail":"apple internal failure"}]}`)),
				Request:    req,
			}, nil
		case req.Method == http.MethodDelete:
			stub.methodOrder = append(stub.methodOrder, "DELETE")
			return fixture.Response("did not expect DELETE %s before creates succeed", req.URL.Path), nil
		default:
			return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
		}
	})
	return stub
}

func TestWebPrivacyApplyReportsPartialReceiptWhenCreateFailsMidSequence(t *testing.T) {
	stub := stubPrivacyMidSequenceFailure(t)

	path := writePrivacyDeclarationForTest(t)
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyApplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", privacyTestAppID,
		"--file", path,
		"--allow-deletes",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	assertNoPrivacySecrets(t, stdout, stderr)
	if execErr == nil {
		t.Fatal("expected non-zero exit after a mid-sequence failure")
	}
	methodOrder := stub.methodOrder
	listCount := stub.listCount
	if strings.Contains(stdout+stderr, "apple internal failure") {
		t.Fatalf("output leaked raw Apple response body:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	for _, method := range methodOrder {
		if method == "DELETE" {
			t.Fatalf("deletes must not run before creates succeed: %#v", methodOrder)
		}
	}
	if listCount < 2 {
		t.Fatalf("expected apply to re-read remote state after failure, list count = %d", listCount)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse receipt JSON: %v\nstdout=%s", err, stdout)
	}
	if payload["applied"] != false {
		t.Fatalf("expected applied=false in partial receipt, got %#v", payload["applied"])
	}
	if payload["changed"] != true {
		t.Fatalf("expected changed=true because one action committed, got %#v", payload["changed"])
	}
	applied := privacyApplyActionKinds(t, payload, "actions")
	if len(applied) != 1 || !strings.HasPrefix(applied[0], "update:") {
		t.Fatalf("expected exactly the committed update in actions, got %#v", applied)
	}
	notApplied := privacyApplyActionKinds(t, payload, "notAppliedActions")
	if len(notApplied) != 2 {
		t.Fatalf("expected the failed create and the unattempted delete in notAppliedActions, got %#v", notApplied)
	}
	recheck, ok := payload["recheck"].(map[string]any)
	if !ok {
		t.Fatalf("expected recheck object in partial receipt, got %#v", payload["recheck"])
	}
	if recheck["succeeded"] != true {
		t.Fatalf("expected recheck.succeeded=true, got %#v", recheck["succeeded"])
	}
	if recheck["remainingChanges"] != float64(2) {
		t.Fatalf("expected recheck.remainingChanges=2, got %#v", recheck["remainingChanges"])
	}
	if !strings.Contains(stderr, "partially applied") {
		t.Fatalf("stderr = %q, want a partial-apply diagnostic", stderr)
	}
	if !strings.Contains(stderr, "cause: web privacy apply failed: web api error (status 500)") {
		t.Fatalf("stderr = %q, want the redacted Apple failure cause", stderr)
	}
}

func TestWebPrivacyApplyFailsClosedOnStaleCatalogTokenBeforeAnyMutation(t *testing.T) {
	fixture := handlertest.New(t)
	var mutations int
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		if resp, ok := privacyCatalogRoundTrip(req, "PURCHASE_HISTORY"); ok {
			return resp, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages":
			return privacyJSONResponse(req, `{"data":[]}`), nil
		case req.Method == http.MethodPost || req.Method == http.MethodPatch || req.Method == http.MethodDelete:
			mutations++
			return fixture.Response("did not expect %s %s after a stale catalog token", req.Method, req.URL.Path), nil
		default:
			return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
		}
	})

	path := writePrivacyDeclarationForTest(t)
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyApplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", privacyTestAppID,
		"--file", path,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	assertNoPrivacySecrets(t, stdout, stderr)
	if execErr == nil {
		t.Fatal("expected apply to fail closed on a stale catalog token")
	}
	if mutations != 0 {
		t.Fatalf("expected no mutations, got %d", mutations)
	}
	if !strings.Contains(execErr.Error(), "PURCHASE_HISTORY") {
		t.Fatalf("error = %v, want the stale token named", execErr)
	}
	if !strings.Contains(stderr, "PURCHASE_HISTORY") {
		t.Fatalf("stderr = %q, want the stale token named", stderr)
	}
}

func TestWebPrivacyPlanFlagsStaleCatalogTokens(t *testing.T) {
	fixture := handlertest.New(t)
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		if resp, ok := privacyCatalogRoundTrip(req, "PURCHASE_HISTORY"); ok {
			return resp, nil
		}
		if req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages" {
			return privacyJSONResponse(req, `{"data":[]}`), nil
		}
		return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
	})

	path := writePrivacyDeclarationForTest(t)
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyPlanCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", privacyTestAppID,
		"--file", path,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("plan must stay a read-only diagnostic: %v", err)
		}
	})
	assertNoPrivacySecrets(t, stdout, stderr)

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse plan JSON: %v\nstdout=%s", err, stdout)
	}
	stale, ok := payload["staleTokens"].([]any)
	if !ok || len(stale) != 1 {
		t.Fatalf("expected one stale token in plan output, got %#v", payload["staleTokens"])
	}
	entry, ok := stale[0].(map[string]any)
	if !ok {
		t.Fatalf("expected stale token object, got %#v", stale[0])
	}
	if entry["id"] != "PURCHASE_HISTORY" || entry["kind"] != "category" || entry["reason"] != "deleted" {
		t.Fatalf("unexpected stale token entry: %#v", entry)
	}
}

func TestWebPrivacyApplyRerunAfterConvergenceIsANoOp(t *testing.T) {
	fixture := handlertest.New(t)
	var mutations int
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		if resp, ok := privacyCatalogRoundTrip(req); ok {
			return resp, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages":
			return privacyJSONResponse(req, `{"data":[
				{
					"id": "usage-email",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"EMAIL_ADDRESS"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_NOT_LINKED_TO_YOU"}}
					}
				},
				{
					"id": "usage-purchase",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"PURCHASE_HISTORY"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"ANALYTICS"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_LINKED_TO_YOU"}}
					}
				}
			]}`), nil
		case req.Method == http.MethodPost || req.Method == http.MethodPatch || req.Method == http.MethodDelete:
			mutations++
			return fixture.Response("did not expect %s %s on a converged rerun", req.Method, req.URL.Path), nil
		default:
			return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
		}
	})

	path := writePrivacyDeclarationForTest(t)
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyApplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", privacyTestAppID,
		"--file", path,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("converged rerun must succeed: %v", err)
		}
	})
	assertNoPrivacySecrets(t, stdout, stderr)
	if mutations != 0 {
		t.Fatalf("expected no mutations on a converged rerun, got %d", mutations)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse receipt JSON: %v\nstdout=%s", err, stdout)
	}
	if payload["applied"] != true {
		t.Fatalf("expected applied=true, got %#v", payload["applied"])
	}
	if payload["changed"] != false {
		t.Fatalf("expected changed=false on a converged rerun, got %#v", payload["changed"])
	}
}

func TestWebPrivacyApplyDoesNotReportSuccessWhenSkippedDeletesRemain(t *testing.T) {
	fixture := handlertest.New(t)
	var mutations int
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		if resp, ok := privacyCatalogRoundTrip(req); ok {
			return resp, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages":
			return privacyJSONResponse(req, `{"data":[
				{
					"id": "usage-email",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"EMAIL_ADDRESS"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_NOT_LINKED_TO_YOU"}}
					}
				},
				{
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"EMAIL_ADDRESS"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_NOT_LINKED_TO_YOU"}}
					}
				},
				{
					"id": "usage-purchase",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"PURCHASE_HISTORY"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"ANALYTICS"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_LINKED_TO_YOU"}}
					}
				}
			]}`), nil
		case req.Method == http.MethodPost || req.Method == http.MethodPatch || req.Method == http.MethodDelete:
			mutations++
			return fixture.Response("did not expect %s %s when only an ID-less leftover remains", req.Method, req.URL.Path), nil
		default:
			return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
		}
	})

	path := writePrivacyDeclarationForTest(t)
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyApplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", privacyTestAppID,
		"--file", path,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	assertNoPrivacySecrets(t, stdout, stderr)
	if execErr == nil {
		t.Fatal("an ID-less leftover is not a successful apply")
	}
	if mutations != 0 {
		t.Fatalf("expected no mutations when the leftover has no usage id, got %d", mutations)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse receipt JSON: %v\nstdout=%s", err, stdout)
	}
	if payload["applied"] != false {
		t.Fatalf("skipped deletes must not report applied=true: %#v", payload["applied"])
	}
	if payload["changed"] != false {
		t.Fatalf("no executable mutation ran, so changed must be false: %#v", payload["changed"])
	}
	skipped, ok := payload["skippedDeletes"].([]any)
	if !ok || len(skipped) != 1 {
		t.Fatalf("expected one skipped delete, got %#v", payload["skippedDeletes"])
	}
	if !strings.Contains(stderr, "without a confirmed change") {
		t.Fatalf("stderr must not call this a successful apply: %q", stderr)
	}
	if !strings.Contains(stderr, "skipped deletes have no usage id, so a rerun cannot remove them") {
		t.Fatalf("stderr must say the leftover cannot be rerun away: %q", stderr)
	}
}

func TestPrivacyApplyStepsRunPrerequisiteDeletesBeforeCreates(t *testing.T) {
	steps := privacyApplySteps(privacyPlanOutput{
		Adds: []privacyPlanChange{
			{Key: "||DATA_NOT_COLLECTED", DataProtection: dataProtectionNotCollected},
		},
		Deletes: []privacyPlanChange{
			{
				Key:            "EMAIL_ADDRESS|APP_FUNCTIONALITY|DATA_LINKED_TO_YOU",
				Category:       "EMAIL_ADDRESS",
				Purpose:        "APP_FUNCTIONALITY",
				DataProtection: dataProtectionLinked,
				UsageID:        "usage-1",
			},
		},
	})

	if len(steps) != 2 {
		t.Fatalf("expected two steps, got %#v", steps)
	}
	if steps[0].Action != "delete" || steps[1].Action != "create" {
		t.Fatalf("switching to DATA_NOT_COLLECTED must delete collected tuples first: %#v", steps)
	}
}

func TestPrivacyApplyStepsRunNotCollectedDeleteBeforeCollectedCreates(t *testing.T) {
	steps := privacyApplySteps(privacyPlanOutput{
		Adds: []privacyPlanChange{
			{Key: "A|P|DATA_LINKED_TO_YOU", Category: "A", Purpose: "P", DataProtection: dataProtectionLinked},
		},
		Deletes: []privacyPlanChange{
			{Key: "||DATA_NOT_COLLECTED", DataProtection: dataProtectionNotCollected, UsageID: "usage-not-collected"},
		},
	})

	if len(steps) != 2 || steps[0].Action != "delete" || steps[1].Action != "create" {
		t.Fatalf("a DATA_NOT_COLLECTED delete must precede collected creates: %#v", steps)
	}
}

func TestApplyPrivacyPlanRecordsUnknownAndUnattemptedStepsOnFailure(t *testing.T) {
	client := &fakePrivacyMutationClient{createErr: fmt.Errorf("web api error (status 500)")}
	plan := privacyPlanOutput{
		Updates: []privacyPlanChange{
			{Key: "A|P|DATA_NOT_LINKED_TO_YOU", Category: "A", Purpose: "P", DataProtection: dataProtectionNotLinked, UsageID: "usage-update"},
		},
		Adds: []privacyPlanChange{
			{Key: "B|P|DATA_LINKED_TO_YOU", Category: "B", Purpose: "P", DataProtection: dataProtectionLinked},
		},
		Deletes: []privacyPlanChange{
			{Key: "C|P|DATA_LINKED_TO_YOU", Category: "C", Purpose: "P", DataProtection: dataProtectionLinked, UsageID: "usage-delete"},
		},
	}

	result, err := applyPrivacyPlan(context.Background(), client, "app-123", plan)
	if err == nil {
		t.Fatal("expected applyPrivacyPlan to return the create failure")
	}
	if len(result.Applied) != 1 || result.Applied[0].Action != "update" {
		t.Fatalf("expected only the committed update: %#v", result.Applied)
	}
	if len(result.Unknown) != 1 || result.Unknown[0].Action != "create" {
		t.Fatalf("expected the failed create to be unknown: %#v", result.Unknown)
	}
	if len(result.NotApplied) != 1 || result.NotApplied[0].Action != "delete" {
		t.Fatalf("expected the unattempted delete to be reported: %#v", result.NotApplied)
	}
	for _, call := range client.callOrder {
		if strings.HasPrefix(call, "delete:") {
			t.Fatalf("delete must not run after a failed create: %#v", client.callOrder)
		}
	}
}

func TestPrivacyStaleTokensFlagsDeletedAndUnknownTokens(t *testing.T) {
	desired := map[string]privacyTuple{
		"a": {Category: "EMAIL_ADDRESS", Purpose: "APP_FUNCTIONALITY", DataProtection: dataProtectionLinked},
		"b": {Category: "RETIRED_CATEGORY", Purpose: "GONE_PURPOSE", DataProtection: dataProtectionLinked},
	}
	catalog := privacyCatalogTokens{
		Categories: map[string]bool{
			"EMAIL_ADDRESS":    false,
			"RETIRED_CATEGORY": true,
		},
		Purposes: map[string]bool{
			"APP_FUNCTIONALITY": false,
		},
		DataProtections: map[string]bool{
			dataProtectionLinked: false,
		},
	}

	stale := privacyStaleTokens(desired, catalog)
	if !reflect.DeepEqual(stale, []privacyStaleToken{
		{Kind: "category", ID: "RETIRED_CATEGORY", Reason: "deleted"},
		{Kind: "purpose", ID: "GONE_PURPOSE", Reason: "unknown"},
	}) {
		t.Fatalf("unexpected stale tokens: %#v", stale)
	}
}

func TestPrivacyStaleTokensSkipsDimensionsAppleReturnedEmpty(t *testing.T) {
	desired := map[string]privacyTuple{
		"a": {Category: "EMAIL_ADDRESS", Purpose: "APP_FUNCTIONALITY", DataProtection: dataProtectionLinked},
	}
	stale := privacyStaleTokens(desired, privacyCatalogTokens{})
	if len(stale) != 0 {
		t.Fatalf("an empty catalog proves nothing and must not flag tokens: %#v", stale)
	}
}

func TestResolvePrivacyApplyResultUsesRemoteEvidence(t *testing.T) {
	result := privacyApplyResult{
		Applied: []privacyApplyAction{},
		Unknown: []privacyApplyAction{
			{Action: "create", Key: "A|P|DATA_LINKED_TO_YOU"},
			{Action: "create", Key: "MISSING|P|DATA_LINKED_TO_YOU"},
			{Action: "delete", Key: "B|P|DATA_LINKED_TO_YOU", UsageID: "usage-b"},
			{Action: "delete", Key: "C|P|DATA_LINKED_TO_YOU", UsageID: "usage-gone"},
			{Action: "update", Key: "D|P|DATA_NOT_LINKED_TO_YOU", UsageID: "usage-d"},
		},
		NotApplied: []privacyApplyAction{},
	}
	remote := map[string]privacyRemoteState{
		"A|P|DATA_LINKED_TO_YOU":     {UsageIDs: []string{"usage-a"}},
		"B|P|DATA_LINKED_TO_YOU":     {UsageIDs: []string{"usage-b"}},
		"D|P|DATA_NOT_LINKED_TO_YOU": {UsageIDs: []string{"usage-d"}},
	}

	resolved := resolvePrivacyApplyResult(result, remote)
	appliedKeys := make([]string, 0, len(resolved.Applied))
	for _, action := range resolved.Applied {
		appliedKeys = append(appliedKeys, action.Action+":"+action.Key)
	}
	if !reflect.DeepEqual(appliedKeys, []string{
		"create:A|P|DATA_LINKED_TO_YOU",
		"delete:C|P|DATA_LINKED_TO_YOU",
		"update:D|P|DATA_NOT_LINKED_TO_YOU",
	}) {
		t.Fatalf("unexpected applied actions: %#v", appliedKeys)
	}
	if len(resolved.Applied) > 0 && resolved.Applied[0].UsageID != "usage-a" {
		t.Fatalf("expected the confirmed create to adopt the remote usage id: %#v", resolved.Applied[0])
	}
	notAppliedKeys := make([]string, 0, len(resolved.NotApplied))
	for _, action := range resolved.NotApplied {
		notAppliedKeys = append(notAppliedKeys, action.Action+":"+action.Key)
	}
	if !reflect.DeepEqual(notAppliedKeys, []string{
		"create:MISSING|P|DATA_LINKED_TO_YOU",
		"delete:B|P|DATA_LINKED_TO_YOU",
	}) {
		t.Fatalf("unexpected not-applied actions: %#v", notAppliedKeys)
	}
	if len(resolved.Unknown) != 0 {
		t.Fatalf("expected every unknown action to be resolved: %#v", resolved.Unknown)
	}
}

func TestResolvePrivacyApplyResultKeepsUpdateUnknownWithoutEvidence(t *testing.T) {
	resolved := resolvePrivacyApplyResult(privacyApplyResult{
		Unknown: []privacyApplyAction{
			{Action: "update", Key: "A|P|DATA_LINKED_TO_YOU", UsageID: "usage-vanished"},
		},
	}, map[string]privacyRemoteState{})

	if len(resolved.Unknown) != 1 {
		t.Fatalf("an update whose usage id vanished stays unknown: %#v", resolved)
	}
}

func TestResolvePrivacyApplyResultKeepsDeleteUnknownWhenRemoteTupleLacksUsageID(t *testing.T) {
	resolved := resolvePrivacyApplyResult(privacyApplyResult{
		Unknown: []privacyApplyAction{
			{
				Action:  "delete",
				Key:     "PHONE_NUMBER|ANALYTICS|DATA_LINKED_TO_YOU",
				UsageID: "usage-phone",
			},
		},
	}, map[string]privacyRemoteState{
		"PHONE_NUMBER|ANALYTICS|DATA_LINKED_TO_YOU": {},
	})

	if len(resolved.Unknown) != 1 || resolved.Unknown[0].Action != "delete" {
		t.Fatalf("an ID-less leftover tuple cannot prove the delete committed: %#v", resolved)
	}
	if len(resolved.Applied) != 0 {
		t.Fatalf("the delete must not be reported as committed: %#v", resolved.Applied)
	}
	if len(resolved.NotApplied) != 0 {
		t.Fatalf("the original usage id is gone, so the delete is not proven not-applied either: %#v", resolved.NotApplied)
	}
}

func TestResolvePrivacyApplyResultKeepsDeleteUnknownWhenIDLessSiblingRemains(t *testing.T) {
	resolved := resolvePrivacyApplyResult(privacyApplyResult{
		Unknown: []privacyApplyAction{
			{
				Action:  "delete",
				Key:     "PHONE_NUMBER|ANALYTICS|DATA_LINKED_TO_YOU",
				UsageID: "usage-phone",
			},
		},
	}, map[string]privacyRemoteState{
		"PHONE_NUMBER|ANALYTICS|DATA_LINKED_TO_YOU": {
			UsageIDs:    []string{"usage-other"},
			IDLessCount: 1,
		},
	})

	if len(resolved.Unknown) != 1 || resolved.Unknown[0].Action != "delete" {
		t.Fatalf("an ID-less sibling could be the deleted resource: %#v", resolved)
	}
	if len(resolved.Applied) != 0 {
		t.Fatalf("the delete must not be reported as committed: %#v", resolved.Applied)
	}
	if len(resolved.NotApplied) != 0 {
		t.Fatalf("the original usage id is gone, so the delete is not proven not-applied: %#v", resolved.NotApplied)
	}
}

func TestWebPrivacyApplyTableReceiptSeparatesAppliedAndNotAppliedActions(t *testing.T) {
	stubPrivacyMidSequenceFailure(t)

	path := writePrivacyDeclarationForTest(t)
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyApplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", privacyTestAppID,
		"--file", path,
		"--allow-deletes",
		"--confirm",
		"--output", "table",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	assertNoPrivacySecrets(t, stdout, stderr)
	if execErr == nil {
		t.Fatal("expected non-zero exit after a mid-sequence failure")
	}
	for _, want := range []string{
		"Applied: false",
		"Changed: true",
		"Applied Actions",
		"Not Applied Actions",
		"Remaining Changes",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table receipt missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "Unknown Actions") {
		t.Fatalf("every attempted action was resolved, so no unknown section belongs in:\n%s", stdout)
	}
}

func TestPrivacyApplyFailureMessageDistinguishesNoConfirmedChange(t *testing.T) {
	partial := privacyApplyFailureMessage("123456789", privacyApplyOutput{
		Actions:           []privacyApplyAction{{Action: "update"}},
		NotAppliedActions: []privacyApplyAction{{Action: "create"}},
	}, fmt.Errorf("web privacy apply failed: web api error (status 500), codes=[ENTITY_ERROR]"), nil, "")
	if !strings.Contains(partial, "partially applied changes for app 123456789") {
		t.Fatalf("unexpected partial message: %q", partial)
	}
	if !strings.Contains(partial, "1 committed, 0 unknown, 1 not applied") {
		t.Fatalf("partial message must carry the receipt counts: %q", partial)
	}

	if !strings.Contains(partial, "cause: web privacy apply failed: web api error (status 500), codes=[ENTITY_ERROR]") {
		t.Fatalf("partial message must carry the reported cause: %q", partial)
	}

	nothing := privacyApplyFailureMessage("123456789", privacyApplyOutput{
		NotAppliedActions: []privacyApplyAction{{Action: "create"}, {Action: "delete"}},
	}, nil, fmt.Errorf("web privacy apply recheck failed: web api error (status 503)"), "")
	if !strings.Contains(nothing, "recheck failed: web privacy apply recheck failed: web api error (status 503)") {
		t.Fatalf("a failed recheck must be reported: %q", nothing)
	}
	if strings.Contains(nothing, "partially applied") {
		t.Fatalf("an apply that committed nothing is not a partial apply: %q", nothing)
	}
	if !strings.Contains(nothing, "without a confirmed change") {
		t.Fatalf("unexpected no-change message: %q", nothing)
	}
	if !strings.Contains(nothing, "0 committed, 0 unknown, 2 not applied") {
		t.Fatalf("no-change message must carry the receipt counts: %q", nothing)
	}
}

type recordingPrivacyUsageReader struct {
	ctxErr      error
	hadDeadline bool
	called      bool
}

func (r *recordingPrivacyUsageReader) ListAppDataUsages(ctx context.Context, _ string) ([]webcore.AppDataUsage, error) {
	r.called = true
	r.ctxErr = ctx.Err()
	_, r.hadDeadline = ctx.Deadline()
	return []webcore.AppDataUsage{}, nil
}

func TestRecheckPrivacyRemoteUsagesUsesAFreshTimeoutBudget(t *testing.T) {
	d, _, _ := newWebTestDeps()

	// The apply request context can already be past its deadline when the
	// mutation fails, so the recheck derives its own budget from the command
	// context instead of inheriting the exhausted one.
	reader := &recordingPrivacyUsageReader{}
	if _, err := recheckPrivacyRemoteUsages(context.Background(), d, reader, "123456789"); err != nil {
		t.Fatalf("recheckPrivacyRemoteUsages() error = %v", err)
	}
	if !reader.called {
		t.Fatal("expected the recheck to reach the client")
	}
	if reader.ctxErr != nil {
		t.Fatalf("recheck context must still be live, got %v", reader.ctxErr)
	}
	if !reader.hadDeadline {
		t.Fatal("recheck context must carry its own deadline")
	}
}

func TestRecheckPrivacyRemoteUsagesHonoursParentCancellation(t *testing.T) {
	d, _, _ := newWebTestDeps()

	parent, cancel := context.WithCancel(context.Background())
	cancel()

	reader := &recordingPrivacyUsageReader{}
	if _, err := recheckPrivacyRemoteUsages(parent, d, reader, "123456789"); err != nil {
		t.Fatalf("recheckPrivacyRemoteUsages() error = %v", err)
	}
	if reader.ctxErr == nil {
		t.Fatal("a cancelled command context must still cancel the recheck")
	}
}

func TestWebPrivacyApplyReportsConvergenceWhenTheFailedStepActuallyCommitted(t *testing.T) {
	fixture := handlertest.New(t)
	deleted := false
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		if resp, ok := privacyCatalogRoundTrip(req); ok {
			return resp, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages":
			usages := `{
				"id": "usage-phone",
				"type": "appDataUsages",
				"relationships": {
					"category": {"data": {"type":"appDataUsageCategories","id":"PHONE_NUMBER"}},
					"purpose": {"data": {"type":"appDataUsagePurposes","id":"ANALYTICS"}},
					"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_LINKED_TO_YOU"}}
				}
			},`
			if deleted {
				usages = ""
			}
			return privacyJSONResponse(req, `{"data":[`+usages+`
				{
					"id": "usage-email",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"EMAIL_ADDRESS"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_NOT_LINKED_TO_YOU"}}
					}
				},
				{
					"id": "usage-purchase",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"PURCHASE_HISTORY"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"ANALYTICS"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_LINKED_TO_YOU"}}
					}
				}
			]}`), nil
		case req.Method == http.MethodDelete && req.URL.Path == "/iris/v1/appDataUsages/usage-phone":
			// Apple commits the delete and then fails the response.
			deleted = true
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"code":"GATEWAY","detail":"upstream hiccup"}]}`)),
				Request:    req,
			}, nil
		default:
			return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
		}
	})

	path := writePrivacyDeclarationForTest(t)
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyApplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", privacyTestAppID,
		"--file", path,
		"--allow-deletes",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	assertNoPrivacySecrets(t, stdout, stderr)
	if execErr == nil {
		t.Fatal("a failed mutation response must still exit non-zero")
	}
	if strings.Contains(stdout+stderr, "upstream hiccup") {
		t.Fatalf("output leaked raw Apple response body:\nstdout=%s\nstderr=%s", stdout, stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse receipt JSON: %v\nstdout=%s", err, stdout)
	}
	if payload["applied"] != true {
		t.Fatalf("the re-read proved every change committed, so applied must be true: %#v", payload["applied"])
	}
	if len(privacyApplyActionKinds(t, payload, "notAppliedActions")) != 0 {
		t.Fatalf("expected no not-applied actions, got %#v", payload["notAppliedActions"])
	}
	if len(privacyApplyActionKinds(t, payload, "unknownActions")) != 0 {
		t.Fatalf("expected no unknown actions, got %#v", payload["unknownActions"])
	}
	recheck, ok := payload["recheck"].(map[string]any)
	if !ok || recheck["remainingChanges"] != float64(0) {
		t.Fatalf("expected recheck.remainingChanges=0, got %#v", payload["recheck"])
	}
	if strings.Contains(stderr, "partially applied") {
		t.Fatalf("a fully converged plan is not a partial apply: %q", stderr)
	}
	if !strings.Contains(stderr, "after every planned change committed") {
		t.Fatalf("stderr = %q, want the convergence diagnostic", stderr)
	}
}

func TestWebPrivacyApplyTrackingDeleteRefusal(t *testing.T) {
	const trackingCode = "STATE_ERROR.BINARY_INDICATES_APP_TRACKS_USERS"
	for _, tc := range []struct {
		name            string
		status          int
		code            string
		protection      string
		committed       bool
		recheckFails    bool
		pendingTracking bool
		wantGuidance    bool
	}{
		{name: "tracking refusal", status: 409, code: trackingCode, protection: dataProtectionTracking, wantGuidance: true},
		{name: "tracking delete committed", status: 409, code: trackingCode, protection: dataProtectionTracking, committed: true},
		{name: "nontracking delete committed", status: 409, code: trackingCode, protection: dataProtectionNotLinked, committed: true},
		{name: "tracking delete never attempted", status: 409, code: trackingCode, protection: dataProtectionNotLinked, pendingTracking: true},
		{name: "ordinary conflict", status: 409, code: "ENTITY_ERROR", protection: dataProtectionTracking},
		{name: "forbidden", status: 403, code: "FORBIDDEN", protection: dataProtectionTracking},
		{name: "recheck failed", status: 409, code: trackingCode, protection: dataProtectionTracking, recheckFails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := handlertest.New(t)
			reads, deletes := 0, 0
			usage := func(id, category, protection string) string {
				return fmt.Sprintf(`{"id":%q,"type":"appDataUsages","relationships":{
					"category":{"data":{"id":%q,"type":"appDataUsageCategories"}},
					"purpose":{"data":{"id":"ANALYTICS","type":"appDataUsagePurposes"}},
					"dataProtection":{"data":{"id":%q,"type":"appDataUsageDataProtections"}}}}`, id, category, protection)
			}
			stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
				if resp, ok := privacyCatalogRoundTrip(req); ok {
					return resp, nil
				}
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages":
					reads++
					if reads > 1 && tc.recheckFails {
						resp := privacyJSONResponse(req, `{"errors":[{"code":"UNAVAILABLE"}]}`)
						resp.StatusCode = http.StatusServiceUnavailable
						return resp, nil
					}
					usages := []string{usage("usage-keep", "EMAIL_ADDRESS", dataProtectionLinked)}
					if reads == 1 || !tc.committed {
						usages = append(usages, usage("usage-target", "ADVERTISING_DATA", tc.protection))
					}
					if tc.pendingTracking {
						usages = append(usages, usage("usage-pending", "ADVERTISING_DATA", dataProtectionTracking))
					}
					return privacyJSONResponse(req, `{"data":[`+strings.Join(usages, ",")+`]}`), nil
				case req.Method == http.MethodDelete && req.URL.Path == "/iris/v1/appDataUsages/usage-target":
					deletes++
					resp := privacyJSONResponse(req, fmt.Sprintf(`{"errors":[{"code":%q,"detail":"cookie=private"}]}`, tc.code))
					resp.StatusCode = tc.status
					return resp, nil
				default:
					return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
				}
			})
			path := filepath.Join(t.TempDir(), "privacy.json")
			if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"dataUsages":[{"category":"EMAIL_ADDRESS","purposes":["ANALYTICS"],"dataProtections":["DATA_LINKED_TO_YOU"]}]}`), 0o600); err != nil {
				t.Fatal(err)
			}
			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := newWebPrivacyApplyCommand(d)
			if err := cmd.FlagSet.Parse([]string{"--app", privacyTestAppID, "--file", path, "--allow-deletes", "--confirm", "--output", "json"}); err != nil {
				t.Fatal(err)
			}
			var execErr error
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { execErr = cmd.Exec(context.Background(), nil) })
			assertNoPrivacySecrets(t, stdout, stderr)
			var apiErr *webcore.APIError
			if !errors.As(execErr, &apiErr) || apiErr.Status != tc.status || !reflect.DeepEqual(apiErr.AllCodes(), []string{tc.code}) {
				t.Fatalf("original provider error not preserved: %v", execErr)
			}
			if reads != 2 || deletes != 1 {
				t.Fatalf("got %d reads and %d deletes; want one attempted delete followed by one recheck", reads, deletes)
			}
			var receipt privacyApplyOutput
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatalf("invalid receipt: %v\n%s", err, stdout)
			}
			if receipt.Applied != tc.committed || receipt.Recheck == nil || receipt.Recheck.Succeeded == tc.recheckFails {
				t.Fatalf("incorrect reconciliation: %+v", receipt)
			}
			if tc.recheckFails {
				if len(receipt.UnknownActions) != 1 || receipt.UnknownActions[0].UsageID != "usage-target" {
					t.Fatalf("failed readback must retain unknown outcome: %+v", receipt)
				}
			} else if tc.committed {
				if len(receipt.Actions) != 1 || receipt.Actions[0].UsageID != "usage-target" || !strings.Contains(stderr, "a rerun is a no-op") {
					t.Fatalf("committed delete must retain convergence receipt: %+v\n%s", receipt, stderr)
				}
			} else if len(receipt.NotAppliedActions) == 0 || receipt.NotAppliedActions[len(receipt.NotAppliedActions)-1].UsageID != "usage-target" {
				t.Fatalf("readback must identify the refused deletion: %+v", receipt)
			}
			if got := strings.Contains(stderr, "retrying alone does not address this constraint"); got != tc.wantGuidance {
				t.Fatalf("tracking guidance = %t, want %t; stderr = %s", got, tc.wantGuidance, stderr)
			}
			if tc.wantGuidance && (!strings.Contains(stderr, "usage-target") || !strings.Contains(stderr, "App Privacy") || strings.Contains(stderr, "converges")) {
				t.Fatalf("refusal must identify the usage and recovery without promising convergence: %s", stderr)
			}
		})
	}
}

func TestWebPrivacyApplyOmitsRemainingChangesWhenTheRecheckFails(t *testing.T) {
	fixture := handlertest.New(t)
	listCount := 0
	stubPrivacyWebSession(t, func(req *http.Request) (*http.Response, error) {
		if resp, ok := privacyCatalogRoundTrip(req); ok {
			return resp, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/"+privacyTestAppID+"/dataUsages":
			listCount++
			if listCount > 1 {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[{"code":"UNAVAILABLE"}]}`)),
					Request:    req,
				}, nil
			}
			return privacyJSONResponse(req, `{"data":[]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/appDataUsages":
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"code":"ENTITY_ERROR"}]}`)),
				Request:    req,
			}, nil
		default:
			return fixture.Response("unexpected request: %s %s", req.Method, req.URL.Path), nil
		}
	})

	path := writePrivacyDeclarationForTest(t)
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebPrivacyApplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", privacyTestAppID,
		"--file", path,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var execErr error
	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	assertNoPrivacySecrets(t, stdout, stderr)
	if execErr == nil {
		t.Fatal("expected a non-zero exit")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse receipt JSON: %v\nstdout=%s", err, stdout)
	}
	if payload["applied"] != false {
		t.Fatalf("an unverified failure must not report applied=true: %#v", payload["applied"])
	}
	recheck, ok := payload["recheck"].(map[string]any)
	if !ok {
		t.Fatalf("expected recheck object, got %#v", payload["recheck"])
	}
	if recheck["succeeded"] != false {
		t.Fatalf("expected recheck.succeeded=false, got %#v", recheck["succeeded"])
	}
	if _, present := recheck["remainingChanges"]; present {
		t.Fatalf("a failed re-read computed no count, so remainingChanges must be absent: %#v", recheck)
	}
	if _, present := payload["changed"]; present {
		t.Fatalf("an unresolved action must not serialize changed as false: %#v", payload["changed"])
	}
	if len(privacyApplyActionKinds(t, payload, "unknownActions")) == 0 {
		t.Fatalf("the attempted create must remain unknown: %#v", payload)
	}
	if !strings.Contains(stderr, "recheck failed: web privacy apply recheck failed: web api error (status 500)") {
		t.Fatalf("stderr = %q, want the redacted re-read failure reported alongside the mutation cause", stderr)
	}
	if !strings.Contains(stderr, "cause: web privacy apply failed: web api error (status 500)") {
		t.Fatalf("stderr = %q, want the original mutation cause", stderr)
	}
}

func TestBuildPrivacyRecheckRowsRendersUnknownRemainingChanges(t *testing.T) {
	rows := buildPrivacyRecheckRows(privacyApplyRecheck{Performed: true})
	if !reflect.DeepEqual(rows[2], []string{"Remaining Changes", "unknown"}) {
		t.Fatalf("unexpected rows for a failed recheck: %#v", rows)
	}

	remaining := 3
	rows = buildPrivacyRecheckRows(privacyApplyRecheck{Performed: true, Succeeded: true, RemainingChanges: &remaining})
	if !reflect.DeepEqual(rows[2], []string{"Remaining Changes", "3"}) {
		t.Fatalf("unexpected rows for a successful recheck: %#v", rows)
	}
}

func TestPrivacyApplyFailureMessageReportsConvergence(t *testing.T) {
	message := privacyApplyFailureMessage("123456789", privacyApplyOutput{
		Applied: true,
		Actions: []privacyApplyAction{{Action: "delete"}},
	}, fmt.Errorf("web privacy apply failed: web api error (status 502)"), nil, "")
	if strings.Contains(message, "partially applied") {
		t.Fatalf("a converged plan is not a partial apply: %q", message)
	}
	if !strings.Contains(message, "after every planned change committed") {
		t.Fatalf("unexpected convergence message: %q", message)
	}
	if !strings.Contains(message, "a rerun is a no-op") {
		t.Fatalf("convergence message must say a rerun is a no-op: %q", message)
	}
}

func TestPrivacyApplyFailureMessageDoesNotPromiseConvergenceWhenSkippedDeletesRemain(t *testing.T) {
	message := privacyApplyFailureMessage("123456789", privacyApplyOutput{
		Actions: []privacyApplyAction{{Action: "update"}},
		SkippedDeletes: []privacySkippedDelete{
			{
				Key:            "PHONE_NUMBER|ANALYTICS|DATA_LINKED_TO_YOU",
				Category:       "PHONE_NUMBER",
				Purpose:        "ANALYTICS",
				DataProtection: dataProtectionLinked,
				Reason:         "missing_usage_id",
			},
		},
	}, fmt.Errorf("web privacy apply failed: web api error (status 500)"), nil, "")
	if strings.Contains(message, "converges") {
		t.Fatalf("skipped deletes cannot be cleared by a rerun: %q", message)
	}
	if strings.Contains(message, "a rerun is a no-op") {
		t.Fatalf("an undeletable leftover is not a match: %q", message)
	}
	if !strings.Contains(message, "skipped deletes have no usage id, so a rerun cannot remove them") {
		t.Fatalf("message must name the undeletable leftover: %q", message)
	}
	if !strings.Contains(message, "partially applied") {
		t.Fatalf("a committed update with a leftover skipped delete is still a partial apply: %q", message)
	}
}

func TestApplyPrivacyPlanReportsPlannedStepsAsNotAppliedWhenValidationFails(t *testing.T) {
	client := &fakePrivacyMutationClient{}
	plan := privacyPlanOutput{
		Updates: []privacyPlanChange{
			{Key: "A|P|DATA_NOT_LINKED_TO_YOU", Category: "A", Purpose: "P", DataProtection: dataProtectionNotLinked, UsageID: "usage-1"},
			{Key: "B|P|DATA_NOT_LINKED_TO_YOU", Category: "B", Purpose: "P", DataProtection: dataProtectionNotLinked, UsageID: "usage-1"},
		},
		Adds: []privacyPlanChange{
			{Key: "C|P|DATA_LINKED_TO_YOU", Category: "C", Purpose: "P", DataProtection: dataProtectionLinked},
		},
	}

	result, err := applyPrivacyPlan(context.Background(), client, "app-123", plan)
	if err == nil {
		t.Fatal("expected the duplicate usage id validation error")
	}
	if len(client.callOrder) != 0 {
		t.Fatalf("validation must abort before any mutation: %#v", client.callOrder)
	}
	if len(result.Applied) != 0 || len(result.Unknown) != 0 {
		t.Fatalf("nothing was attempted: %#v", result)
	}
	if len(result.NotApplied) != len(privacyApplySteps(plan)) {
		t.Fatalf("every planned step must be reported as not applied: %#v", result.NotApplied)
	}
	actions := map[string]int{}
	for _, action := range result.NotApplied {
		actions[action.Action]++
	}
	if actions["update"] != 2 || actions["create"] != 1 {
		t.Fatalf("unexpected not-applied buckets: %#v", result.NotApplied)
	}
}

func TestPrivacyApplyChangedOmitsWhenOnlyUnknownActionsRemain(t *testing.T) {
	if got := privacyApplyChanged(privacyApplyResult{
		Unknown: []privacyApplyAction{{Action: "create"}},
	}); got != nil {
		t.Fatalf("unresolved actions must omit changed, got %#v", *got)
	}

	got := privacyApplyChanged(privacyApplyResult{
		Applied: []privacyApplyAction{{Action: "update"}},
		Unknown: []privacyApplyAction{{Action: "create"}},
	})
	if got == nil || !*got {
		t.Fatalf("a confirmed mutation still reports changed=true: %#v", got)
	}

	got = privacyApplyChanged(privacyApplyResult{})
	if got == nil || *got {
		t.Fatalf("a fully resolved no-op reports changed=false: %#v", got)
	}
}

func TestPrivacyApplyConvergedRejectsResidualSkippedDeletes(t *testing.T) {
	clean := privacyApplyResult{}
	converged := privacyPlanOutput{}
	if !privacyApplyConverged(converged, clean) {
		t.Fatal("an empty residual plan with a fully resolved result is converged")
	}

	withSkipped := privacyPlanOutput{
		SkippedDeletes: []privacySkippedDelete{
			{
				Key:            "PHONE_NUMBER|ANALYTICS|DATA_LINKED_TO_YOU",
				Category:       "PHONE_NUMBER",
				Purpose:        "ANALYTICS",
				DataProtection: dataProtectionLinked,
				Reason:         "missing_usage_id",
			},
		},
	}
	if privacyApplyConverged(withSkipped, clean) {
		t.Fatal("an undeletable extra tuple still on the remote is not convergence")
	}

	if privacyApplyConverged(converged, privacyApplyResult{
		Unknown: []privacyApplyAction{{Action: "update", Key: "A|P|DATA_LINKED_TO_YOU"}},
	}) {
		t.Fatal("an unresolved action is not convergence")
	}
	if privacyApplyConverged(converged, privacyApplyResult{
		NotApplied: []privacyApplyAction{{Action: "create", Key: "A|P|DATA_LINKED_TO_YOU"}},
	}) {
		t.Fatal("a not-applied action is not convergence")
	}
	if privacyApplyConverged(privacyPlanOutput{
		Adds: []privacyPlanChange{{Key: "A|P|DATA_LINKED_TO_YOU"}},
	}, clean) {
		t.Fatal("a residual executable change is not convergence")
	}
}
