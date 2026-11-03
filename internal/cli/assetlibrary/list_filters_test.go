package assetlibrary

import "testing"

func TestLibraryFiltersOnlyOnMediaLists(t *testing.T) {
	for _, group := range Command().Subcommands {
		for _, child := range group.Subcommands {
			for _, name := range libraryListFilterFlags {
				// Instance --id selectors predate the new collection ID filter.
				if name == "id" && child.Name != "list" {
					continue
				}
				// Upload --category and --reference-name set attributes of the new
				// asset; they are not list filters.
				if (name == "category" || name == "reference-name") && child.Name == "upload" {
					continue
				}
				// Placement reads have their own --state and --sort values.
				if (name == "state" || name == "sort") && child.Name == "placements" {
					continue
				}
				got := child.FlagSet.Lookup(name) != nil
				want := (group.Name == "images" || group.Name == "videos") && child.Name == "list"
				if got != want {
					t.Fatalf("%s %s --%s exposure=%v want%v", group.Name, child.Name, name, got, want)
				}
			}
		}
	}
}
