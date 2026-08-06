package registry

import "testing"

func TestAssetLibraryIsNotRegistered(t *testing.T) {
	for _, cmd := range NewCatalog("dev").MetadataCommands() {
		if cmd.Name == "asset-library" {
			t.Fatal("asset-library must remain absent from the public root registry")
		}
	}
}
