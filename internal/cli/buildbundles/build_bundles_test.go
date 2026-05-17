package buildbundles

import "testing"

func TestBuildBundlesCommandConstructors(t *testing.T) {
	top := BuildBundlesCommand()
	if top == nil {
		t.Fatal("expected build-bundles command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := BuildBundlesCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
	if got := BuildBundleFileSizesCommand(); got == nil {
		t.Fatal("expected file sizes command")
	}
	if got := BuildBundlesAppClipCommand(); got == nil {
		t.Fatal("expected app clip command")
	}
}
