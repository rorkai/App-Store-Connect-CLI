package app_events

import "testing"

func TestAppEventsCommandConstructors(t *testing.T) {
	top := AppEventsCommand()
	if top == nil {
		t.Fatal("expected app-events command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected top-level command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected app-events subcommands")
	}

	if got := Command(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}

	constructors := []func() any{
		func() any { return AppEventLocalizationsCommand() },
		func() any { return AppEventScreenshotsCommand() },
		func() any { return AppEventVideoClipsCommand() },
		func() any { return AppEventsRelationshipsCommand() },
		func() any { return AppEventsSubmitCommand() },
		func() any { return AppEventLocalizationScreenshotsCommand() },
	}
	for _, ctor := range constructors {
		if got := ctor(); got == nil {
			t.Fatal("expected constructor to return command")
		}
	}
}
