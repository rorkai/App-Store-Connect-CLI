package versions

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestVersionsCommand_PrefersViewAndRemovesLegacyGet(t *testing.T) {
	d, _ := newVersionsTestDeps(nil)
	cmd := NewVersionsCommand(d)

	var viewCmd *ffcli.Command
	for _, sub := range cmd.Subcommands {
		switch sub.Name {
		case "view":
			viewCmd = sub
		}
	}

	if viewCmd == nil {
		t.Fatal("expected canonical view subcommand")
	}
	usage := cmd.UsageFunc(cmd)
	if !strings.Contains(usage, "view") {
		t.Fatalf("expected help to list view subcommand, got %q", usage)
	}
	if strings.Contains(usage, " get ") {
		t.Fatalf("expected help to hide deprecated get alias, got %q", usage)
	}
}

func TestSplitCompatAppStoreVersionIncludes(t *testing.T) {
	apiIncludes, includeAgeRating := splitCompatAppStoreVersionIncludes([]string{
		"ageRatingDeclaration",
		"appStoreReviewDetail",
		"routingAppCoverage",
	})

	if !includeAgeRating {
		t.Fatal("expected age rating include compatibility flag")
	}
	if len(apiIncludes) != 2 {
		t.Fatalf("expected 2 API includes, got %d (%v)", len(apiIncludes), apiIncludes)
	}
	if apiIncludes[0] != "appStoreReviewDetail" || apiIncludes[1] != "routingAppCoverage" {
		t.Fatalf("unexpected API includes: %v", apiIncludes)
	}
}

func TestNormalizeAppStoreVersionReleaseType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: ""},
		{input: "manual", want: "MANUAL"},
		{input: "AFTER_APPROVAL", want: "AFTER_APPROVAL"},
		{input: " scheduled ", want: "SCHEDULED"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := normalizeAppStoreVersionReleaseType(test.input)
			if err != nil {
				t.Fatalf("normalizeAppStoreVersionReleaseType() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("normalizeAppStoreVersionReleaseType() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeAppStoreVersionReleaseTypeRejectsUnknown(t *testing.T) {
	for _, input := range []string{"IMMEDIATE", "   "} {
		t.Run(input, func(t *testing.T) {
			_, err := normalizeAppStoreVersionReleaseType(input)
			if err == nil || !strings.Contains(err.Error(), "--release-type must be one of: MANUAL, AFTER_APPROVAL, SCHEDULED") {
				t.Fatalf("expected release type error, got %v", err)
			}
		})
	}
}

func TestAppendAgeRatingDeclarationInclude(t *testing.T) {
	versionResp := &asc.AppStoreVersionResponse{
		Data: asc.Resource[asc.AppStoreVersionAttributes]{
			Type: asc.ResourceTypeAppStoreVersions,
			ID:   "version-1",
			Relationships: json.RawMessage(`{
				"appStoreReviewDetail":{"data":{"type":"appStoreReviewDetails","id":"review-1"}}
			}`),
		},
		Included: json.RawMessage(`[
			{"type":"appStoreReviewDetails","id":"review-1","attributes":{"contactEmail":"qa@example.com"}}
		]`),
	}
	ageRatingResp := &asc.AgeRatingDeclarationResponse{
		Data: asc.Resource[asc.AgeRatingDeclarationAttributes]{
			Type: asc.ResourceTypeAgeRatingDeclarations,
			ID:   "age-1",
			Attributes: asc.AgeRatingDeclarationAttributes{
				Gambling: boolPtr(true),
			},
		},
	}

	if err := appendAgeRatingDeclarationInclude(versionResp, ageRatingResp); err != nil {
		t.Fatalf("appendAgeRatingDeclarationInclude() error: %v", err)
	}

	var relationships map[string]json.RawMessage
	if err := json.Unmarshal(versionResp.Data.Relationships, &relationships); err != nil {
		t.Fatalf("unmarshal relationships: %v", err)
	}
	if _, ok := relationships["ageRatingDeclaration"]; !ok {
		t.Fatal("expected ageRatingDeclaration relationship to be added")
	}
	if _, ok := relationships["appStoreReviewDetail"]; !ok {
		t.Fatal("expected existing relationship to be preserved")
	}

	var included []map[string]any
	if err := json.Unmarshal(versionResp.Included, &included); err != nil {
		t.Fatalf("unmarshal included: %v", err)
	}
	if len(included) != 2 {
		t.Fatalf("expected 2 included resources, got %d", len(included))
	}

	foundAgeRating := false
	for _, item := range included {
		if item["type"] == string(asc.ResourceTypeAgeRatingDeclarations) && item["id"] == "age-1" {
			foundAgeRating = true
		}
	}
	if !foundAgeRating {
		t.Fatal("expected age rating declaration in included resources")
	}
}

func boolPtr(value bool) *bool {
	return &value
}
