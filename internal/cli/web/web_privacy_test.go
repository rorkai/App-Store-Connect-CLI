package web

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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

func TestDeclarationFromRemoteDataUsagesMalformedOnlyDefaultsNotCollected(t *testing.T) {
	declaration := declarationFromRemoteDataUsages([]webcore.AppDataUsage{
		{
			ID:       "usage-malformed-1",
			Category: "PURCHASE_HISTORY",
			Purpose:  "APP_FUNCTIONALITY",
		},
	})

	if len(declaration.DataUsages) != 1 {
		t.Fatalf("expected one default data usage, got %#v", declaration.DataUsages)
	}
	if !reflect.DeepEqual(declaration.DataUsages[0].DataProtections, []string{dataProtectionNotCollected}) {
		t.Fatalf("unexpected default declaration for malformed-only usages: %#v", declaration.DataUsages[0])
	}
}

func TestDeclarationFromRemoteDataUsagesSkipsMalformedWhenValidPresent(t *testing.T) {
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
		t.Fatalf("expected one valid usage group, got %#v", declaration.DataUsages)
	}
	if declaration.DataUsages[0].Category != "PURCHASE_HISTORY" {
		t.Fatalf("unexpected declaration category: %#v", declaration.DataUsages[0])
	}
	if !reflect.DeepEqual(declaration.DataUsages[0].DataProtections, []string{dataProtectionLinked}) {
		t.Fatalf("unexpected declaration protections: %#v", declaration.DataUsages[0])
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

func TestPlanFromDesiredAndRemotePairsAddDeleteIntoUpdate(t *testing.T) {
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
	if len(plan.Adds) != 0 || len(plan.Deletes) != 0 {
		t.Fatalf("expected no adds/deletes after pairing, got adds=%#v deletes=%#v", plan.Adds, plan.Deletes)
	}
	if plan.Updates[0].UsageID != "usage-1" || plan.Updates[0].DataProtection != dataProtectionNotLinked {
		t.Fatalf("unexpected update payload: %#v", plan.Updates[0])
	}
	if len(plan.APICalls) != 1 || plan.APICalls[0].Operation != "update_data_usage" || plan.APICalls[0].Count != 1 {
		t.Fatalf("unexpected api calls: %#v", plan.APICalls)
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
}

func (f *fakePrivacyMutationClient) CreateAppDataUsage(_ context.Context, _ string, tuple webcore.DataUsageTuple) (*webcore.AppDataUsage, error) {
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

func TestApplyPrivacyPlanExecutesDeleteUpdateCreateOrder(t *testing.T) {
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

	actions, err := applyPrivacyPlan(context.Background(), client, "app-123", plan)
	if err != nil {
		t.Fatalf("applyPrivacyPlan() error = %v", err)
	}
	if !reflect.DeepEqual(client.callOrder, []string{
		"delete:usage-delete-1",
		"update:usage-update-1:DATA_NOT_LINKED_TO_YOU",
		"create:EMAIL_ADDRESS:ANALYTICS:DATA_NOT_LINKED_TO_YOU",
	}) {
		t.Fatalf("unexpected call order: %#v", client.callOrder)
	}
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, got %#v", actions)
	}
	if actions[0].Action != "delete" || actions[1].Action != "update" || actions[2].Action != "create" {
		t.Fatalf("unexpected action order: %#v", actions)
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

	_, err := parsePrivacyDeclarationFile(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePrivacyDeclarationFileRejectsMultipleJSONValues(t *testing.T) {
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

	_, err := parsePrivacyDeclarationFile(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "multiple JSON values found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePrivacyDeclarationFileCanonicalizesTrackingPurposeAway(t *testing.T) {
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

	declaration, err := parsePrivacyDeclarationFile(path)
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
