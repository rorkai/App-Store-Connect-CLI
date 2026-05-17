package winbackoffers

import "testing"

func TestWinBackOffersCommandConstructors(t *testing.T) {
	top := WinBackOffersCommand()
	if top == nil {
		t.Fatal("expected win-back-offers command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := WinBackOffersCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
	if got := WinBackOffersPricesCommand(); got == nil {
		t.Fatal("expected prices command")
	}
	if got := WinBackOffersRelationshipsCommand(); got == nil {
		t.Fatal("expected relationships command")
	}
}
