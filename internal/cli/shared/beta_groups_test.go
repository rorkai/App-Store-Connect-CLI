package shared

import (
	"context"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type betaGroupsMutationClientStub struct {
	buildID  string
	groupIDs []string
	notify   bool
	calls    int
}

func (s *betaGroupsMutationClientStub) AddBetaGroupsToBuildWithNotify(_ context.Context, buildID string, groupIDs []string, notify bool) (asc.BuildBetaGroupsNotificationAction, error) {
	s.calls++
	s.buildID = buildID
	s.groupIDs = append([]string(nil), groupIDs...)
	s.notify = notify
	return asc.BuildBetaGroupsNotificationActionNone, nil
}

func TestAddBuildBetaGroupsSkipsInternalGroupsWithAllBuildsWhenRequested(t *testing.T) {
	client := &betaGroupsMutationClientStub{}
	groups := []ResolvedBetaGroup{
		{ID: "group-internal", IsInternalGroup: true, HasAccessToAllBuilds: true},
	}

	result, err := AddBuildBetaGroups(context.Background(), client, "build-1", groups, AddBuildBetaGroupsOptions{
		SkipInternalWithAllBuilds: true,
		Notify:                    true,
	})
	if err != nil {
		t.Fatalf("AddBuildBetaGroups() error = %v", err)
	}

	if client.calls != 0 {
		t.Fatalf("expected no mutation calls, got %d", client.calls)
	}
	if len(result.AddedGroupIDs) != 0 {
		t.Fatalf("expected no added groups, got %v", result.AddedGroupIDs)
	}
	if len(result.SkippedInternalAllBuildsGroups) != 1 {
		t.Fatalf("expected one skipped internal all-builds group, got %d", len(result.SkippedInternalAllBuildsGroups))
	}
	if result.SkippedInternalAllBuildsGroups[0].ID != "group-internal" {
		t.Fatalf("expected skipped group-internal, got %q", result.SkippedInternalAllBuildsGroups[0].ID)
	}
}

func TestAddBuildBetaGroupsAddsInternalGroupsWithAllBuildsWhenSkipDisabled(t *testing.T) {
	client := &betaGroupsMutationClientStub{}
	groups := []ResolvedBetaGroup{
		{ID: "group-internal", IsInternalGroup: true, HasAccessToAllBuilds: true},
		{ID: "group-external", IsInternalGroup: false},
	}

	result, err := AddBuildBetaGroups(context.Background(), client, "build-1", groups, AddBuildBetaGroupsOptions{
		SkipInternalWithAllBuilds: false,
		Notify:                    true,
	})
	if err != nil {
		t.Fatalf("AddBuildBetaGroups() error = %v", err)
	}

	if client.calls != 1 {
		t.Fatalf("expected one mutation call, got %d", client.calls)
	}
	if client.buildID != "build-1" {
		t.Fatalf("expected build-1, got %q", client.buildID)
	}
	if !client.notify {
		t.Fatal("expected notify=true")
	}
	if len(client.groupIDs) != 2 {
		t.Fatalf("expected two group IDs, got %v", client.groupIDs)
	}
	if client.groupIDs[0] != "group-internal" || client.groupIDs[1] != "group-external" {
		t.Fatalf("unexpected group IDs: %v", client.groupIDs)
	}
	if len(result.SkippedInternalAllBuildsGroups) != 0 {
		t.Fatalf("expected no skipped internal all-builds groups, got %v", result.SkippedInternalAllBuildsGroups)
	}
}
