package appclips

import "testing"

func TestAppClipsCommandConstructors(t *testing.T) {
	top := AppClipsCommand()
	if top == nil {
		t.Fatal("expected app-clips command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected top-level command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected app-clips subcommands")
	}

	if got := AppClipsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}

	constructors := []func() any{
		func() any { return AppClipDefaultExperiencesCommand() },
		func() any { return AppClipAdvancedExperiencesCommand() },
		func() any { return AppClipHeaderImagesCommand() },
		func() any { return AppClipInvocationsCommand() },
		func() any { return AppClipDomainStatusCommand() },
	}
	for _, ctor := range constructors {
		if got := ctor(); got == nil {
			t.Fatal("expected constructor to return command")
		}
	}
}
