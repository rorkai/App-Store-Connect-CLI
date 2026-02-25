package builds

import (
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestResolveBuildBetaGroupIDsFromList_ByIDAndName(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{
				ID: "group-1",
				Attributes: asc.BetaGroupAttributes{
					Name: "Internal",
				},
			},
			{
				ID: "group-2",
				Attributes: asc.BetaGroupAttributes{
					Name: "External Testers",
				},
			},
		},
	}

	resolved, err := resolveBuildBetaGroupIDsFromList([]string{"group-1", "External Testers"}, groups)
	if err != nil {
		t.Fatalf("resolveBuildBetaGroupIDsFromList() error: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved groups, got %d", len(resolved))
	}
	if resolved[0] != "group-1" || resolved[1] != "group-2" {
		t.Fatalf("unexpected resolved order/content: %v", resolved)
	}
}

func TestResolveBuildBetaGroupIDsFromList_Deduplicates(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "group-1", Attributes: asc.BetaGroupAttributes{Name: "Internal"}},
		},
	}

	resolved, err := resolveBuildBetaGroupIDsFromList([]string{"group-1", "Internal", "group-1"}, groups)
	if err != nil {
		t.Fatalf("resolveBuildBetaGroupIDsFromList() error: %v", err)
	}
	if len(resolved) != 1 || resolved[0] != "group-1" {
		t.Fatalf("expected deduplicated [group-1], got %v", resolved)
	}
}

func TestResolveBuildBetaGroupIDsFromList_AmbiguousName(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "group-1", Attributes: asc.BetaGroupAttributes{Name: "Beta"}},
			{ID: "group-2", Attributes: asc.BetaGroupAttributes{Name: "Beta"}},
		},
	}

	_, err := resolveBuildBetaGroupIDsFromList([]string{"Beta"}, groups)
	if err == nil {
		t.Fatal("expected ambiguous name error")
	}
	if !strings.Contains(err.Error(), "multiple beta groups named") {
		t.Fatalf("expected ambiguous error, got %v", err)
	}
}

func TestResolveBuildBetaGroupIDsFromList_NotFound(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "group-1", Attributes: asc.BetaGroupAttributes{Name: "Internal"}},
		},
	}

	_, err := resolveBuildBetaGroupIDsFromList([]string{"Does Not Exist"}, groups)
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !strings.Contains(err.Error(), `beta group "Does Not Exist" not found`) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestResolveBuildBetaGroupIDsFromList_MixedInputDeduplicates(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{ID: "group-1", Attributes: asc.BetaGroupAttributes{Name: "Internal"}},
			{ID: "group-2", Attributes: asc.BetaGroupAttributes{Name: "External"}},
		},
	}

	resolved, err := resolveBuildBetaGroupIDsFromList([]string{"group-1", "Internal", "group-2", "External", "group-1"}, groups)
	if err != nil {
		t.Fatalf("resolveBuildBetaGroupIDsFromList() error: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("expected 2 deduplicated groups, got %d (%v)", len(resolved), resolved)
	}
	if resolved[0] != "group-1" || resolved[1] != "group-2" {
		t.Fatalf("unexpected resolved order/content: %v", resolved)
	}
}

func TestResolveBuildBetaGroupsFromListIncludesInternalMetadata(t *testing.T) {
	groups := &asc.BetaGroupsResponse{
		Data: []asc.Resource[asc.BetaGroupAttributes]{
			{
				ID: "group-internal",
				Attributes: asc.BetaGroupAttributes{
					Name:            "Internal Crew",
					IsInternalGroup: true,
				},
			},
			{
				ID: "group-external",
				Attributes: asc.BetaGroupAttributes{
					Name:            "External QA",
					IsInternalGroup: false,
				},
			},
		},
	}

	resolved, err := resolveBuildBetaGroupsFromList([]string{"group-internal", "External QA"}, groups)
	if err != nil {
		t.Fatalf("resolveBuildBetaGroupsFromList() error: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved groups, got %d", len(resolved))
	}

	if resolved[0].ID != "group-internal" || !resolved[0].IsInternalGroup {
		t.Fatalf("expected internal group metadata for first entry, got %+v", resolved[0])
	}
	if resolved[1].ID != "group-external" || resolved[1].IsInternalGroup {
		t.Fatalf("expected external group metadata for second entry, got %+v", resolved[1])
	}
}
