package cmdtest

import "testing"

func TestCategoriesEncryptionRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"categories", "subcategories"}, wantErr: "categories subcategories: --next"},
		{args: []string{"encryption", "declarations", "list"}, wantErr: "encryption declarations list: --next"},
	})
}

func TestCategoriesEncryptionPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"categories", "subcategories"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appCategories/GAMES/subcategories?cursor=AQ&limit=200",
			dataType: "appCategories",
		},
		{
			args:     []string{"encryption", "declarations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEncryptionDeclarations?cursor=AQ&limit=200",
			dataType: "appEncryptionDeclarations",
		},
	})
}
