package reviews

import "testing"

func TestReviewsCommandConstructors(t *testing.T) {
	top := ReviewsCommand()
	if top == nil {
		t.Fatal("expected reviews command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected reviews subcommands")
	}

	if got := ReviewsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}

	constructors := []func() any{
		func() any { return ReviewCommand() },
		func() any { return ReviewStatusCommand() },
		func() any { return ReviewDoctorCommand() },
		func() any { return ReviewsGetCommand() },
		func() any { return ReviewsRatingsCommand() },
		func() any { return ReviewsRespondBatchCommand() },
		func() any { return ReviewsResponseCommand() },
		func() any { return ReviewDetailsAttachmentsListCommand() },
	}
	for _, ctor := range constructors {
		if got := ctor(); got == nil {
			t.Fatal("expected constructor to return command")
		}
	}
}
