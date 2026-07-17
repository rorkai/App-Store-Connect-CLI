package apps

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"

func normalizeAppInfoInclude(value string) ([]string, error) {
	return shared.NormalizeSelection(value, appInfoIncludeList(), "--include")
}

func appInfoIncludeList() []string {
	return []string{
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
}
