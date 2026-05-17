package androidiosmapping

import "testing"

func TestAndroidIosMappingCommandConstructors(t *testing.T) {
	cmd := AndroidIosMappingCommand()
	if cmd == nil {
		t.Fatal("expected android-ios-mapping command")
		return
	}
	if cmd.Name == "" {
		t.Fatal("expected command name")
	}
	if len(cmd.Subcommands) != 5 {
		t.Fatalf("expected 5 subcommands, got %d", len(cmd.Subcommands))
	}

	constructors := []func() any{
		func() any { return AndroidIosMappingListCommand() },
		func() any { return AndroidIosMappingGetCommand() },
		func() any { return AndroidIosMappingCreateCommand() },
		func() any { return AndroidIosMappingUpdateCommand() },
		func() any { return AndroidIosMappingDeleteCommand() },
	}
	for _, ctor := range constructors {
		if got := ctor(); got == nil {
			t.Fatal("expected non-nil subcommand constructor")
		}
	}
}
