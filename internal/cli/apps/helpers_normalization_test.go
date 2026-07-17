package apps

import (
	"reflect"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestNormalizeInclude(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		allowed []string
		wantLen int
		wantErr bool
	}{
		{
			name:    "empty",
			input:   "",
			allowed: appInfoIncludeList(),
			wantLen: 0,
		},
		{
			name:    "valid",
			input:   "ageRatingDeclaration,appInfoLocalizations",
			allowed: appInfoIncludeList(),
			wantLen: 2,
		},
		{
			name:    "unsupported territory age ratings",
			input:   "territoryAgeRatings",
			allowed: appInfoIncludeList(),
			wantErr: true,
		},
		{
			name:    "invalid option",
			input:   "badValue",
			allowed: appInfoIncludeList(),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := shared.NormalizeSelection(test.input, test.allowed, "--include")
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != test.wantLen {
				t.Fatalf("expected %d values, got %d", test.wantLen, len(got))
			}
		})
	}
}

func TestAppInfoIncludeListMatchesOpenAPI441(t *testing.T) {
	want := []string{
		"app",
		"ageRatingDeclaration",
		"appInfoLocalizations",
		"primaryCategory",
		"primarySubcategoryOne",
		"primarySubcategoryTwo",
		"secondaryCategory",
		"secondarySubcategoryOne",
		"secondarySubcategoryTwo",
	}
	if got := appInfoIncludeList(); !reflect.DeepEqual(got, want) {
		t.Fatalf("appInfoIncludeList() = %v, want exact OpenAPI 4.4.1 enum %v", got, want)
	}
}

func TestNormalizeAppTagHelpers(t *testing.T) {
	visibility, err := normalizeAppTagVisibilityFilter("TRUE,false")
	if err != nil {
		t.Fatalf("unexpected visibility error: %v", err)
	}
	if len(visibility) != 2 || visibility[0] != "true" || visibility[1] != "false" {
		t.Fatalf("unexpected visibility values: %v", visibility)
	}
	if _, err := normalizeAppTagVisibilityFilter("maybe"); err == nil {
		t.Fatal("expected visibility validation error")
	}

	if _, err := normalizeAppTagFields("name,visibleInAppStore"); err != nil {
		t.Fatalf("unexpected app tag fields error: %v", err)
	}
	if _, err := normalizeAppTagFields("invalidField"); err == nil {
		t.Fatal("expected app tag fields validation error")
	}

	if _, err := normalizeAppTagInclude("territories"); err != nil {
		t.Fatalf("unexpected include error: %v", err)
	}
	if _, err := normalizeAppTagInclude("invalidInclude"); err == nil {
		t.Fatal("expected include validation error")
	}

	if _, err := normalizeTerritoryFields("currency"); err != nil {
		t.Fatalf("unexpected territory field error: %v", err)
	}
	if _, err := normalizeTerritoryFields("name"); err == nil {
		t.Fatal("expected territory fields validation error")
	}
}

func TestNormalizeAppEncryptionDeclarationHelpers(t *testing.T) {
	if _, err := normalizeAppEncryptionDeclarationFields("usesEncryption,platform"); err != nil {
		t.Fatalf("unexpected declaration fields error: %v", err)
	}
	if _, err := normalizeAppEncryptionDeclarationFields("bad"); err == nil {
		t.Fatal("expected declaration fields validation error")
	}

	if _, err := normalizeAppEncryptionDeclarationDocumentFields("fileName,fileSize"); err != nil {
		t.Fatalf("unexpected document fields error: %v", err)
	}
	if _, err := normalizeAppEncryptionDeclarationDocumentFields("bad"); err == nil {
		t.Fatal("expected document fields validation error")
	}

	if _, err := normalizeAppEncryptionDeclarationInclude("app,builds"); err != nil {
		t.Fatalf("unexpected include error: %v", err)
	}
	if _, err := normalizeAppEncryptionDeclarationInclude("bad"); err == nil {
		t.Fatal("expected include validation error")
	}
}

func TestNormalizeTerritoryAgeRatingHelpers(t *testing.T) {
	if _, err := normalizeTerritoryAgeRatingFields("appStoreAgeRating"); err != nil {
		t.Fatalf("unexpected territory age rating fields error: %v", err)
	}
	if _, err := normalizeTerritoryAgeRatingFields("bad"); err == nil {
		t.Fatal("expected territory age rating fields validation error")
	}

	include, err := normalizeTerritoryAgeRatingInclude("territory")
	if err != nil {
		t.Fatalf("unexpected include error: %v", err)
	}
	if !contains(include, "territory") {
		t.Fatalf("expected include to contain territory, got %v", include)
	}
	if _, err := normalizeTerritoryAgeRatingInclude("bad"); err == nil {
		t.Fatal("expected territory include validation error")
	}
}
